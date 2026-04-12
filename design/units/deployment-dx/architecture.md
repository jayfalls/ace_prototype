# Architecture: Deployment & Developer Experience

**Unit:** deployment-dx
**Date:** 2026-04-12
**Status:** Design

---

## 1. Component Overview

The architecture transforms ACE from a multi-container orchestration into a single Go binary with four embedded subsystems. Each subsystem has a default (embedded) mode and an enterprise (external) mode controlled by configuration flags.

```
┌───────────────────────────────────────────────────────────────────┐
│                         cmd/ace/main.go                          │
│                      (CLI entry point, wiring)                  │
├───────────────────────────────────────────────────────────────────┤
│  internal/app                                                    │
│  ├── app.go                    ── orchestrates startup/shutdown   │
│  └── config.go                 ── resolves config from all srcs  │
├───────────────────────────────────────────────────────────────────┤
│  internal/platform                                               │
│  ├── database                  ── SQLite + PostgreSQL switch     │
│  ├── messaging                 ── Embedded NATS + remote switch  │
│  ├── cache                     ── Ristretto + Valkey switch       │
│  ├── telemetry                 ── SQLite exporters + OTLP switch  │
│  ├── frontend                  ── go:embed + Vite proxy switch   │
│  └── paths.go                  ── XDG path resolution            │
├───────────────────────────────────────────────────────────────────┤
│  internal/api                                                    │
│  ├── handler/                  ── HTTP handlers (unchanged)      │
│  ├── middleware/               ── HTTP middleware (unchanged)     │
│  ├── router/                   ── Chi router + SPA catch-all     │
│  ├── service/                  ── Business logic (unchanged)      │
│  ├── repository/               ── SQLC data access (dialect sw.) │
│  ├── model/                    ── Domain types (unchanged)       │
│  ├── response/                ── HTTP response helpers           │
│  └── validator/                ── Input validation                │
├───────────────────────────────────────────────────────────────────┤
│  internal/caching             ── Cache interface + backends      │
│  internal/messaging           ── NATS client + embedded server    │
│  internal/telemetry           ── OTel SDK + SQLite exporters     │
└───────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Package | Responsibility |
|-----------|---------|---------------|
| **app** | `internal/app` | Lifecycle orchestration: startup sequence, dependency wiring, graceful shutdown |
| **config** | `internal/app` | Configuration resolution: CLI flags > env vars > config.yaml > XDG defaults |
| **database** | `internal/platform/database` | Open `database/sql` with appropriate driver; run Goose migrations |
| **messaging** | `internal/platform/messaging` | Start embedded NATS server; create in-process or remote client |
| **cache** | `internal/platform/cache` | Construct `CacheBackend` from mode (Ristretto or Valkey) |
| **telemetry** | `internal/platform/telemetry` | Initialize OTel SDK with SQLite or OTLP exporters; configure zap logger |
| **frontend** | `internal/platform/frontend` | Serve embedded static assets (prod) or proxy to Vite dev server (dev) |
| **paths** | `internal/platform` | Resolve data, config, log, and cache directories via XDG + CLI overrides |
| **api** | `internal/api/*` | HTTP routing, handlers, services, repositories — business logic unchanged |

---

## 2. Data Flow

### 2.1 Startup Data Flow

```
ace main.go
  │
  ├── Parse CLI flags & resolve config
  │     └── config.Resolve() → Config struct
  │
  ├── Initialize paths (XDG resolution)
  │     └── paths.Resolve() → Paths{DataDir, ConfigDir, LogDir, ...}
  │
  ├── Initialize database
  │     └── database.Open(cfg) → *sql.DB
  │           ├── embedded: modernc.org/sqlite driver, file:ace.db?_pragma=...
  │           └── external: lib/pq driver, postgres://...
  │
  ├── Run Goose migrations
  │     └── goose.Up(db, "migrations/") → error | nil
  │
  ├── Initialize NATS
  │     └── messaging.Init(cfg) → (nats.Client, func() error)
  │           ├── embedded: start server (DontListen:true), InProcessServer conn
  │           └── external: connect to remote URL
  │
  ├── Initialize cache
  │     └── cache.Init(cfg) → caching.CacheBackend
  │           ├── embedded: Ristretto InProcessBackend
  │           └── external: Valkey ValkeyBackend
  │
  ├── Initialize telemetry
  │     └── telemetry.Init(cfg) → (*TracerProvider, *MeterProvider, zap.Logger)
  │           ├── embedded: SQLite SpanExporter + MetricReader + file logger
  │           └── external: OTLP exporters + structured logger
  │
  ├── Wire services (dependency injection)
  │     └── Constructor injection: each service receives its deps as interfaces
  │
  ├── Initialize HTTP server
  │     └── router.New(cfg) → *chi.Mux
  │           ├── /api/v1/*      → API handlers
  │           ├── /api/v1/telemetry/* → Inspector handlers
  │           └── /*              → SPA handler
  │
  └── Listen on :8080 (or configured port)
```

### 2.2 Request Data Flow (Production)

```
Browser Request
  │
  ▼
HTTP Server (chi.Mux)
  │
  ├── /api/v1/* ─────────────────────── API Route
  │     │
  │     ▼
  │   Middleware Chain (Recovery → Logger → CORS → RateLimit → Auth)
  │     │
  │     ▼
  │   Handler (handler.AuthHandler, etc.)
  │     │
  │     ▼
  │   Service (service.AuthService, etc.)
  │     │
  │     ▼
  │   Repository (SQLC-generated queries against *sql.DB)
  │     │
  │     ▼
  │   SQLite (in-process) or PostgreSQL (external)
  │
  └── /* ─────────────────────────────── SPA Route
        │
        ▼
      SPA Handler
        │
        ├── Has file extension? → Serve from go:embed FS
        ├── Starts with /_app/ or /@vite/? → Serve from go:embed FS
        └── Otherwise? → Serve index.html (client-side routing)
```

### 2.3 Request Data Flow (Development)

```
Browser Request
  │
  ▼
HTTP Server (chi.Mux)
  │
  ├── /api/v1/* ─────────────────────── Same as production
  │
  └── /* ─────────────────────────────── Vite Proxy
        │
        ▼
      Reverse Proxy → http://localhost:5173
        │
        ▼
      Vite Dev Server (HMR enabled)
```

### 2.4 Internal Message Flow

```
Service publishes message
  │
  ▼
caching.Cache.Set() / messaging.Client.Publish()
  │
  ├── Cache path:
  │     Cache.Set() → cacheImpl → InProcessBackend
  │       └── Ristretto .Set() with cost-based admission
  │       └── Tag index update (tag → set of cache keys)
  │
  └── Messaging path:
        Client.Publish(subject, data, headers)
          │
          ▼
        Embedded NATS (in-process net.Pipe)
          │
          ▼
        Subscriber receives via nats.Subscribe()
          │
          ▼
        Handler processes message
```

### 2.5 Telemetry Data Flow

```
Service operation
  │
  ├── OTel SDK → SpanExporter
  │     └── embedded: SQLiteExporter → INSERT INTO ott_spans
  │     └── external: OTLPExporter → HTTP/gRPC to collector
  │
  ├── OTel SDK → MetricReader
  │     └── embedded: SQLiteMetricReader → INSERT INTO ott_metrics
  │     └── external: OTLPExporter → HTTP/gRPC to collector
  │
  ├── Usage event
  │     └── embedded: Direct write → INSERT INTO usage_events
  │     └── external: Publish to NATS → consumer writes to usage_events
  │
  └── Structured log
        └── zap.Logger → stdout (JSON) + file (ace.log)

---

## 3. Interface Contracts

### 3.1 Platform Interfaces

The `internal/platform` packages define switchable backends. Each subsystem uses a mode-based factory function that returns the correct implementation.

#### 3.1.1 Database (`internal/platform/database`)

```go
package database

// Open returns a *sql.DB configured for the specified mode.
// embedded: uses modernc.org/sqlite with WAL pragmas.
// external: uses lib/pq with the provided URL.
// The caller is responsible for running migrations after Open.
func Open(cfg *Config) (*sql.DB, error)

// Migrate runs all Goose migrations against the database.
func Migrate(db *sql.DB, migrationsDir string) error

// Config holds database connection parameters.
type Config struct {
    Mode           string // "embedded" or "external"
    DataDir        string // Used to construct SQLite path in embedded mode
    ExternalURL    string // PostgreSQL URL for external mode
}
```

Key behavior:
- In embedded mode, the connection string is constructed as: `file:{DataDir}/ace.db?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)`
- In external mode, the URL is passed directly to `lib/pq`
- Both modes return `*sql.DB`, making SQLC queries compatible across dialects

#### 3.1.2 Messaging (`internal/platform/messaging`)

```go
package messaging

// Init starts the messaging subsystem and returns a Client.
// embedded: starts an in-process NATS server (DontListen:true),
//   creates an InProcessServer connection, and returns a Client.
// external: connects to the remote NATS URL and returns a Client.
// The returned cleanup function must be called on shutdown
// (drain client → shutdown server).
func Init(cfg *Config) (messaging.Client, func() error, error)

// Config holds messaging connection parameters.
type Config struct {
    Mode      string // "embedded" or "external"
    RemoteURL string // NATS URL for external mode
    DataDir    string // JetStream storage directory for embedded mode
}
```

The returned `messaging.Client` is identical to the current `shared/messaging.Client` interface — no API changes required. The only difference is how the underlying `*nats.Conn` is created.

#### 3.1.3 Cache (`internal/platform/cache`)

```go
package cache

// Init creates the appropriate CacheBackend based on mode.
// embedded: returns an InProcessBackend wrapping Ristretto.
// external: returns a ValkeyBackend (existing implementation).
func Init(cfg *Config) (caching.CacheBackend, error)

// Config holds cache connection parameters.
type Config struct {
    Mode      string // "embedded" or "external"
    MaxCost   int64  // Memory budget in bytes for Ristretto (default 50MB)
    ValkeyURL string // Valkey URL for external mode
}
```

The `InProcessBackend` implements the existing `caching.CacheBackend` interface. Tag invalidation, version management, and stampede protection remain in `internal/caching` (the `Cache` wrapper) — they are not `InProcessBackend` concerns.

#### 3.1.4 Telemetry (`internal/platform/telemetry`)

```go
package telemetry

// Init sets up the complete telemetry subsystem.
// embedded: creates SQLite-backed OTel exporters + zap logger to file.
// external: creates OTLP exporters + zap logger to stdout.
// Returns cleanup function that flushes and shuts down providers.
func Init(cfg *Config) (*Telemetry, func(context.Context) error, error)

// Telemetry holds all telemetry providers.
type Telemetry struct {
    Logger         *zap.Logger
    TracerProvider *sdktrace.TracerProvider
    MeterProvider  *sdkmetric.MeterProvider
}

// Config holds telemetry configuration.
type Config struct {
    Mode      string // "embedded" or "external"
    DataDir   string // SQLite file directory for embedded mode
    ServiceName string
    OTLPEndpoint string // OTLP collector URL for external mode
}
```

#### 3.1.5 Frontend (`internal/platform/frontend`)

Two files, selected by build tags:

```go
// File: internal/platform/frontend/frontend.go
// Default build: development mode with Vite proxy

package frontend

import "net/http"

// Handler returns an http.Handler that proxies to http://localhost:5173
// for frontend requests, falling through to the Vite dev server.
func Handler() http.Handler { /* ... */ }
```

```go
// File: internal/platform/frontend/frontend_embed.go
//go:build embed

package frontend

import "embed"
import "net/http"

//go:embed all:../../../frontend/build
var assets embed.FS

// Handler returns an http.Handler that serves embedded assets
// with SPA fallback routing.
func Handler() http.Handler { /* ... */ }
```

```go
// File: internal/platform/frontend/frontend_dev.go
//go:build dev

package frontend

import "net/handler"

// Handler returns an http.Handler that proxies to http://localhost:5173
// for frontend requests, falling through to the Vite dev server.
func Handler() http.Handler { /* ... */ }
```

#### 3.1.6 Paths (`internal/platform`)

```go
package platform

// Paths holds all resolved filesystem paths for the application.
type Paths struct {
    DataDir   string // $XDG_DATA_HOME/ace/ or --data-dir
    ConfigDir string // $XDG_CONFIG_HOME/ace/
    LogDir    string // $XDG_STATE_HOME/ace/logs/
    DBPath    string // DataDir/ace.db
    NATSPath  string // DataDir/nats/
    TelemetryPath string // DataDir/telemetry/
}

// ResolvePaths resolves all paths using XDG conventions and CLI overrides.
// Priority: --data-dir flag > $ACE_DATA_DIR > $XDG_DATA_HOME/ace > ~/.local/share/ace/
func ResolvePaths(dataDirOverride string) (*Paths, error)

// EnsureDirs creates all required directories with 0700 permissions.
func (p *Paths) EnsureDirs() error

// PrintPaths outputs all resolved paths (used by `ace paths` command).
func (p *Paths) PrintPaths() string
```

### 3.2 Application Configuration (`internal/app`)

```go
package app

// Config holds all resolved configuration for the application.
type Config struct {
    // Paths
    Paths *platform.Paths

    // Server
    Port int    // default 8080
    Host string // default "0.0.0.0"
    Dev  bool   // enables Vite proxy mode

    // Database
    DBMode string // "embedded" or "external"
    DBURL  string // PostgreSQL URL (required when DBMode="external")

    // Messaging
    NATSMode string // "embedded" or "external"
    NATSURL  string // NATS URL (required when NATSMode="external")

    // Cache
    CacheMode    string // "embedded" or "external"
    CacheURL     string // Valkey URL (required when CacheMode="external")
    CacheMaxCost int64  // Ristretto max cost in bytes (default 50MB)

    // Telemetry
    TelemetryMode string // "embedded" or "external"
    OTLPEndpoint  string // OTLP collector URL (required when TelemetryMode="external")

    // Auth (unchanged from current)
    JWTSecret          string
    AccessTokenTTL     time.Duration
    RefreshTokenTTL    time.Duration
    JWTAudience        []string
    JWTIssuer          string
    RateLimitPerIP     int
    RateLimitPerEmail  int
    RateLimitWindow    time.Duration
    CORSAllowedOrigins []string
    // ... (existing auth config fields)
}

// ResolveConfig loads configuration with priority:
// CLI flags > env vars > config.yaml > XDG defaults
func ResolveConfig() (*Config, error)
```

### 3.3 App Lifecycle (`internal/app`)

```go
package app

// App represents the running application with all subsystems.
type App struct {
    cfg     *Config
    db      *sql.DB
    nats    messaging.Client
    cache   caching.Cache
    tel     *telemetry.Telemetry
    server  *http.Server
    cleanup func(context.Context) error
}

// New creates and wires all subsystems based on configuration.
// Returns an App ready to serve, or an error if any subsystem fails.
func New(cfg *Config) (*App, error)

// Serve starts the HTTP server and blocks until shutdown signal.
func (a *App) Serve() error

// Shutdown gracefully stops all subsystems in reverse startup order.
func (a *App) Shutdown(ctx context.Context) error
```

### 3.4 Router Changes (`internal/api/router`)

The router's `New` function gains a `SPAHandler` dependency:

```go
// Config holds all dependencies needed to create the router.
type Config struct {
    App         *app.Config
    Queries     *db.Queries
    AuthService *service.AuthService
    TokenService *service.TokenService
    MagicLinkService *service.MagicLinkService
    SPAHandler  http.Handler  // NEW: from platform/frontend
    Cache       caching.Cache // NEW: for telemetry Inspector
}

// New creates a chi.Mux with all routes.
// Route groups:
//   /health/*           → liveness/readiness
//   /auth/*             → authentication endpoints
//   /api/v1/*           → business logic (existing)
//   /api/v1/telemetry/* → Inspector endpoints (NEW)
//   /*                  → SPA handler (NEW)
```

### 3.5 Telemetry Inspector (`internal/api/handler`)

```go
package handler

// TelemetryHandler serves product-facing observability endpoints.
type TelemetryHandler struct {
    db *sql.DB  // queries against telemetry tables
}

// Spans returns recent trace spans.
// GET /api/v1/telemetry/spans?limit=50&service=ace
func (h *TelemetryHandler) Spans(w http.ResponseWriter, r *http.Request)

// Metrics returns metric summaries.
// GET /api/v1/telemetry/metrics?name=http_requests&window=1h
func (h *TelemetryHandler) Metrics(w http.ResponseWriter, r *http.Request)

// Usage returns cost attribution data.
// GET /api/v1/telemetry/usage?agent_id=xxx&from=2026-01-01
func (h *TelemetryHandler) Usage(w http.ResponseWriter, r *http.Request)

// Health returns subsystem health status.
// GET /api/v1/telemetry/health
func (h *TelemetryHandler) Health(w http.ResponseWriter, r *http.Request)
```

### 3.6 Cache Interface Preservation (`internal/caching`)

The `Cache` and `CacheBackend` interfaces from `shared/caching` are preserved unchanged. The `InProcessBackend` is a new implementation:

```go
package caching

// InProcessBackend implements CacheBackend using Ristretto
// plus application-managed tag and version indexes.
type InProcessBackend struct {
    cache       *ristretto.Cache
    tagIndex    map[string]map[string]struct{}  // tag → set of keys
    tagIndexMu  sync.RWMutex
    versionMu   sync.Mutex
}

func NewInProcessBackend(cfg InProcessConfig) (CacheBackend, error)

type InProcessConfig struct {
    MaxCost   int64         // Memory budget in bytes (default 50MB)
    BufferItems int64       // Number of keys per Get buffer (default 64)
}
```

The `InProcessBackend` implements all `CacheBackend` methods:
- `Get`, `Set`, `Delete`, `GetMany`, `SetMany`, `DeleteMany` → direct Ristretto calls
- `DeletePattern` → iterate Ristretto keys, filter by glob pattern
- `DeleteByTag` → look up tag index, delete member keys, remove tag entry
- `SAdd`, `SMembers`, `SRem` → tag index operations (the "sets" in InProcessBackend are the tag index, not Redis sets)
- `Exists`, `TTL` → Ristretto lookup with metadata tracking
- `Close` → Ristretto close

---

## 4. Directory Structure

### 4.1 New Backend Layout (Single Module)

```
backend/
├── go.mod                          # module ace (single module, no go.work)
├── go.sum
├── cmd/
│   └── ace/
│       └── main.go                 # CLI entry point: parse flags, call app.New(), app.Serve()
├── internal/
│   ├── app/
│   │   ├── app.go                  # App struct, New(), Serve(), Shutdown()
│   │   └── config.go               # Config struct, ResolveConfig(), flag parsing
│   ├── platform/
│   │   ├── database/
│   │   │   ├── database.go         # Open(), Migrate(), Config
│   │   │   ├── database_embed.go   # //go:build !external — SQLite-specific logic
│   │   │   └── database_ext.go     # //go:build external — PostgreSQL-specific logic
│   │   ├── messaging/
│   │   │   ├── messaging.go        # Init(), Config, cleanup
│   │   │   ├── server_embed.go      # //go:build !external — embedded NATS server start
│   │   │   └── server_ext.go        # //go:build external — remote NATS client
│   │   ├── cache/
│   │   │   ├── cache.go            # Init(), Config
│   │   │   ├── inprocess.go         # InProcessBackend (Ristretto) — always compiled
│   │   │   └── valkey.go           # ValkeyBackend — always compiled (or build-tagged)
│   │   ├── telemetry/
│   │   │   ├── telemetry.go        # Init(), Telemetry struct, Config
│   │   │   ├── sqlite_exporter.go   # SQLite SpanExporter + MetricReader
│   │   │   ├── sqlite_queries.sql   # SQLC queries for telemetry tables
│   │   │   └── inspector.go         # TelemetryHandler HTTP handlers
│   │   ├── frontend/
│   │   │   ├── frontend.go         # Default: proxy to Vite dev server
│   │   │   ├── frontend_embed.go   # //go:build embed — go:embed static assets
│   │   │   └── spa.go              # SPA routing logic (shared by both modes)
│   │   └── paths.go                # XDG path resolution, Paths struct
│   ├── api/
│   │   ├── handler/
│   │   │   ├── admin_handler.go
│   │   │   ├── auth_handler.go
│   │   │   ├── health.go
│   │   │   ├── session_handler.go
│   │   │   └── telemetry_handler.go # NEW: Inspector endpoints
│   │   ├── middleware/
│   │   │   ├── auth_middleware.go
│   │   │   ├── cors.go
│   │   │   ├── logger.go
│   │   │   ├── rate_limit_middleware.go
│   │   │   ├── rbac_middleware.go
│   │   │   └── recovery.go
│   │   ├── router/
│   │   │   └── router.go           # Updated: SPA catch-all + telemetry routes
│   │   ├── service/
│   │   │   ├── auth_service.go
│   │   │   ├── event_service.go
│   │   │   ├── magic_link_service.go
│   │   │   ├── password_service.go
│   │   │   ├── permission_service.go
│   │   │   └── token_service.go
│   │   ├── repository/
│   │   │   ├── db.go               # Updated: accept *sql.DB from platform/database
│   │   │   ├── generated/           # SQLC-generated code (dialect: sqlite or postgresql)
│   │   │   ├── queries/
│   │   │   │   ├── auth_tokens.sql
│   │   │   │   ├── permissions.sql
│   │   │   │   ├── sessions.sql
│   │   │   │   ├── users.sql
│   │   │   │   ├── version_stamps.sql
│   │   │   │   ├── telemetry_spans.sql    # NEW
│   │   │   │   ├── telemetry_metrics.sql  # NEW
│   │   │   │   └── usage_events.sql        # NEW
│   │   │   └── schema.sql
│   │   ├── model/
│   │   ├── response/
│   │   └── validator/
│   ├── caching/
│   │   ├── cache.go              # Unchanged: Cache implementation
│   │   ├── constructors.go       # Updated: add NewInProcessBackend
│   │   ├── inprocess_backend.go  # NEW: InProcessBackend implementation
│   │   ├── valkey_backend.go     # Unchanged: ValkeyBackend
│   │   ├── key_builder.go
│   │   ├── singleflight.go
│   │   ├── version_store.go
│   │   ├── warming.go
│   │   ├── errors.go
│   │   ├── noop_observer.go
│   │   ├── options.go
│   │   └── types.go              # Unchanged: Cache, CacheBackend interfaces
│   ├── messaging/
│   │   ├── client.go             # Largely unchanged: Client interface
│   │   ├── envelope.go
│   │   ├── errors.go
│   │   ├── patterns.go
│   │   ├── stream.go
│   │   ├── subjects.go
│   │   └── server_embed.go       # NEW: embedded NATS server lifecycle
│   └── telemetry/
│       ├── telemetry.go          # Updated: Init() returns embedded-configured providers
│       ├── sqlite_exporter.go     # NEW: OTel SpanExporter → SQLite
│       ├── sqlite_reader.go       # NEW: OTel MetricReader → SQLite
│       ├── logger.go              # Updated: dual-output (stdout + file)
│       ├── tracer.go              # Updated: uses Init result, not OTLP
│       ├── metrics.go             # Updated: uses Init result, not OTLP
│       ├── consumer.go            # Unchanged
│       ├── middleware.go           # Unchanged
│       ├── types.go               # Unchanged
│       ├── constants.go           # Unchanged
│       └── usage.go               # Updated: direct DB write instead of NATS publish
├── migrations/
│   ├── 20260321000000_create_users.sql
│   ├── 20260321000001_create_sessions.sql
│   ├── ... (existing migrations, adapted for SQLite)
│   ├── 20260412000000_create_telemetry.sql    # NEW
│   └── 20260412000001_create_usage_events.sql # NEW
├── sql/
│   └── (SQLC query definitions, adapted for SQLite dialect)
└── Makefile                        # Simplified: ace, ui, test targets
```

### 4.2 New Frontend Layout

```
frontend/
├── svelte.config.js   # Changed: adapter-static with fallback: 'index.html'
├── package.json
├── vite.config.ts
├── src/
│   └── (unchanged SvelteKit source)
└── build/              # Produced by npm run build, consumed by go:embed
    ├── index.html
    ├── _app/
    └── ...
```

### 4.3 Repository Root Additions

```
ace_prototype/
├── scripts/
│   ├── install.sh       # curl | sh installation script
│   └── verify.sh       # Standalone checksum verification
├── backend/            # (described above)
├── frontend/           # (described above)
├── design/
│   └── units/deployment-dx/
├── Makefile            # Simplified root Makefile
└── .goreleaser.yml     # NEW: GoReleaser configuration for multi-arch builds
```

### 4.4 Deleted Items

| Item | Reason |
|------|--------|
| `backend/go.work` | Replaced by single `go.mod` |
| `backend/go.work.sum` | No longer needed |
| `backend/shared/go.mod` | Merged into single module |
| `backend/shared/messaging/go.mod` | Merged into single module |
| `backend/shared/messaging/go.sum` | No longer needed |
| `backend/shared/telemetry/go.mod` | Merged into single module |
| `backend/shared/telemetry/go.sum` | No longer needed |
| `backend/scripts/docs-gen/go.mod` | Merged as Makefile target |
| `backend/scripts/docs-gen/` | Moved to Makefile target |
| `backend/services/api/go.mod` | Merged into single module |
| `backend/services/api/go.sum` | No longer needed |
| `backend/vendor/` | Replaced by `go mod vendor` on single module |
| `backend/caching.test` | Test binary artifact, regenerate |
| `backend/tests/` | Merge into `internal/` tests |
| `devops/` | All container orchestrations removed |
| `changelogs/` | Consolidated into git history |
| `backend/services/api/cmd/` | Replaced by `backend/cmd/ace/` |
```

---

## 5. Key Algorithms & Approaches

### 5.1 Configuration Resolution Algorithm

```
ResolveConfig():
    1. Create Config with hardcoded defaults
    2. Overlay with XDG-derived defaults (paths, ports)
    3. Overlay with config.yaml values (if file exists)
    4. Overlay with environment variables (ACE_DATA_DIR, ACE_PORT, etc.)
    5. Overlay with CLI flags (--data-dir, --port, etc.)
    6. Validate:
       - Required fields must be present after all layers
       - Mode-specific fields must match (external mode requires URL)
       - Port must be valid (1-65535)
       - Data dir must be writable
    7. Construct derived values:
       - DBPath = DataDir/ace.db
       - NATSPath = DataDir/nats/
       - LogPath = LogDir/ace.log
    8. Return Config or error
```

Implementation uses `adrg/xdg` for base path resolution, `flag` package for CLI parsing, and manual YAML parsing for config file. Priority escalation ensures CLI flags always win.

### 5.2 Embedded NATS Server Lifecycle

```go
func startEmbeddedNATS(dataDir string) (*nats.Server, error) {
    ns, err := nats.NewServer(&nats.ServerConfig{
        DontListen: true,
        StoreDir:   filepath.Join(dataDir, "nats"),
        JetStream:  true,
        NoLog:      true,
    })
    if err != nil {
        return nil, fmt.Errorf("start embedded NATS: %w", err)
    }
    go ns.Start()
    if !ns.ReadyForConnections(10 * time.Second) {
        return nil, fmt.Errorf("embedded NATS server did not start within timeout")
    }
    return ns, nil
}
```

Key invariants:
- `DontListen: true` — zero TCP ports
- `InProcessServer(ns)` — `net.Pipe` connection, no network
- No reconnect logic needed — server is in-process
- JetStream persists to `$ACE_DATA_DIR/nats/`
- Cleanup: drain client → shutdown server

### 5.3 Ristretto InProcessBackend Tag Invalidation

The `CacheBackend` interface requires `DeleteByTag()` which currently uses Redis sets. The `InProcessBackend` implements this via a secondary index:

```go
type InProcessBackend struct {
    cache      *ristretto.Cache
    tagIndex   map[string]map[string]struct{}  // tag → set of cache keys
    tagIndexMu sync.RWMutex
}
```

- `SAdd(ctx, key, members, ttl)` — registers tags: for each member, `tagIndex[member]` gets `key` added
- `DeleteByTag(ctx, tag)` — looks up `tagIndex[tag]`, deletes all member keys from Ristretto, removes the tag entry
- `SMembers(ctx, key)` — returns `tagIndex[key]` as a string slice
- Memory overhead is negligible vs the 50MB Ristretto budget

### 5.4 SPA Routing Middleware

```go
func SPAHandler(fs fs.FS) http.Handler {
    fileServer := http.FileServer(http.FS(fs))
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        path := r.URL.Path
        if hasExtension(path) {
            fileServer.ServeHTTP(w, r)
            return
        }
        if strings.HasPrefix(path, "/_app/") || strings.HasPrefix(path, "/@vite/") {
            fileServer.ServeHTTP(w, r)
            return
        }
        r.URL.Path = "/index.html"
        fileServer.ServeHTTP(w, r)
    })
}
```

In dev mode, `frontend_dev.go` provides a handler that reverse-proxies to `http://localhost:5173`.

### 5.5 SQLite Telemetry Exporter

Custom OTel `SpanExporter` and `MetricReader` write to SQLite tables (`ott_spans`, `ott_metrics`). A background goroutine prunes: spans >7 days, metrics >24 hours.

### 5.6 Module Consolidation

One-time restructuring from multi-module workspace to single `go.mod`:
1. Create `backend/go.mod` with `module ace`
2. Move `shared/` → `internal/`, `services/api/internal/` → `internal/api/`
3. Update all import paths (e.g., `ace/shared/caching` → `ace/internal/caching`)
4. Move `services/api/cmd/main.go` → `cmd/ace/main.go`
5. Run `go mod tidy && go mod vendor`
6. Delete `go.work`, per-module `go.mod`/`go.sum` files

SQLC dialect changes from `postgresql` to `sqlite`.

### 5.7 Migration Dialect Adaptation

| PostgreSQL | SQLite | Notes |
|-----------|--------|-------|
| `SERIAL`/`BIGSERIAL` | `INTEGER PRIMARY KEY AUTOINCREMENT` | Auto-ID |
| `BOOLEAN` | `INTEGER` (0/1) | No native boolean |
| `gen_random_uuid()` | Go-generated UUIDs | Not a SQL function |
| `TIMESTAMPTZ` | `TEXT` (RFC3339) | Store as strings |
| `RETURNING *` | `lastInsertId` + query | Limited RETURNING support |
| `ON CONFLICT` | `INSERT OR REPLACE` / `ON CONFLICT` | SQLite 3.24+ |

Strategy: create new SQLite-specific migration files. `database.Open()` detects driver and runs appropriate migration directory.

---

## 6. Error Handling Strategy

### 6.1 Error Wrapping Convention

All errors use `fmt.Errorf("context: %w", err)`. Each package defines sentinel errors:

```go
// internal/platform/database
var (
    ErrOpenFailed      = errors.New("database: open failed")
    ErrMigrationFailed = errors.New("database: migration failed")
)

// internal/platform/messaging
var (
    ErrServerStartFailed = errors.New("messaging: server start failed")
    ErrConnectFailed     = errors.New("messaging: connection failed")
)
```

### 6.2 Startup Failure Handling

`app.New()` initializes subsystems sequentially. On failure, it rolls back all successfully-initialized subsystems:

```go
func New(cfg *Config) (*App, error) {
    var rollback []func(context.Context) error
    // Database, NATS, Cache, Telemetry — each appends rollback on success
    // On any error: rollbackAll(ctx, rollback); return nil, err
}
```

### 6.3 Runtime Error Propagation

- **Database**: SQLite `SQLITE_BUSY` retried at driver level via `busy_timeout=5000`
- **NATS**: Existing typed errors unchanged; far less likely in embedded mode
- **Cache**: Ristretto silently drops under memory pressure (TinyLFU admission); logged via `CacheObserver`
- **Telemetry**: Best-effort writes; errors logged, never block request path

### 6.4 Shutdown Error Handling

Shutdown follows reverse startup order. Each step logs errors but continues. Final return aggregates via `errors.Join()`.

```
1. HTTP server drain (10s deadline)
2. Telemetry flush (spans/metrics → SQLite)
3. NATS client drain (5s timeout)
4. NATS server shutdown (embedded only)
5. Cache close (Ristretto cleanup)
6. Database close (WAL checkpoint)
```

---

## 7. Testing Strategy

### 7.1 Test Categories

| Category | Build Tag | Scope | Runs In |
|----------|-----------|-------|---------|
| **Unit** | (none) | Pure logic, no external deps | `make test` (git hook) |
| **Integration** | `//go:build integration` | Embedded subsystems | CI only |

### 7.2 Test File Organization

Each `internal/platform/*` package has `_test.go` (unit) and `_integration_test.go` (integration) files. Unit tests use mocks at interface boundaries. Integration tests use in-process embedded subsystems.

### 7.3 Database Testing

In-memory SQLite replaces Docker PostgreSQL:

```go
func setupTestDB(t *testing.T) *sql.DB {
    db, err := sql.Open("sqlite", "file::memory:?cache=shared")
    // ... error handling, migrations, t.Cleanup
    return db
}
```

### 7.4 Integration Test: Full Startup

```go
//go:build integration
func TestAppStartupAndShutdown(t *testing.T) {
    cfg := &app.Config{ /* all embedded modes */ }
    a, err := app.New(cfg)
    // assert healthy → graceful shutdown
}
```

### 7.5 Makefile Targets

```makefile
# Dev mode (default): Vite proxy for HMR
ace:       go build -o bin/ace ./cmd/ace/

# Production build: embedded frontend (used by GoReleaser)
ace-prod:  cd frontend && npm run build && go build -tags embed -o bin/ace ./cmd/ace/

# Full validation pipeline
test:      go build ./... && go vet ./... && go test -short ./... && sqlc generate && (cd frontend && npm run lint && npm run test:run)
```

### 7.6 Test Performance Targets

| Type | Target | Method |
|------|--------|--------|
| `make test` | <30s | In-memory SQLite, embedded NATS, `-short`, no Docker |
| Package unit test | <5s | In-memory everything |
| Git hook | <30s | Calls `make test` |

---

## 8. Build & Release

### 8.1 Build Tags

| Tag | Behavior |
|-----|----------|
| (default) | Vite proxy, HMR, development |
| `embed` | Embedded frontend, production mode |
| `external_db` | PostgreSQL (future enterprise builds) |

### 8.2 GoReleaser

Multi-arch builds (linux/darwin, amd64/arm64), stripped binaries, SHA256 checksums published to GitHub Releases.

### 8.3 Install Script

`scripts/install.sh`: detect OS/arch → fetch latest release → verify SHA256 → install to `$HOME/.local/bin/ace` → check PATH. `scripts/verify.sh` for standalone checksum verification.

### 8.4 Binary Size Budget

Target: <150MB (Go ~30MB + frontend ~15MB + SQLite driver ~15MB + NATS ~20MB + other ~10MB). Well within 200MB disk budget.

---

## 9. Security Considerations

- **Data directory**: All dirs created with `0700` permissions
- **Install script**: HTTPS-only, SHA256 verification, user-level install (no root)
- **Embedded NATS**: `DontListen:true` — zero network exposure
- **SQLite**: `0600` file permissions, parameterized queries via SQLC