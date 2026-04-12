# BSD: Business/System Design — Deployment & Developer Experience

**Unit:** deployment-dx
**Date:** 2026-04-12
**Status:** Design

---

## 1. System Context

### 1.1 Purpose

ACE transforms from a multi-container orchestration (10+ containers, 30+ second cold start, 1GB+ RAM) into a single-binary system installable via `curl | sh` and runnable as `ace`. The system preserves all functional capabilities while eliminating operational complexity.

### 1.2 Actors

| Actor | Interaction |
|-------|-------------|
| **Developer** | Builds, tests, and runs ACE locally. Uses `make ace`, `make test`. Expects fast feedback loops. |
| **End User** | Installs ACE via `curl \| sh`, runs `ace` command, interacts via browser at `localhost:8080`. |
| **Enterprise Operator** | Deploys ACE with external dependencies (PostgreSQL, NATS cluster, OTLP collector) via configuration flags. |

### 1.3 External Systems

| System | Default Mode | Enterprise Mode | Interface |
|--------|-------------|-----------------|-----------|
| **SQLite** | Embedded, in-process | Replaced by external PostgreSQL | `database/sql` driver switch |
| **NATS** | Embedded, in-process | Replaced by remote NATS cluster | Connection URL switch |
| **Ristretto cache** | In-process memory | Replaced by external Valkey | Backend interface switch |
| **OTel SDK** | Custom SQLite exporters | OTLP export to collectors | Exporter configuration |
| **GitHub Releases** | Source of binary downloads | N/A | `install.sh` fetches binaries |
| **LLM Providers** | External API calls | Same | Unchanged from current |

### 1.4 System Boundary

```
┌──────────────────────────────────────────────────────────┐
│                      ACE Binary                          │
│                                                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────┐ │
│  │  HTTP    │  │  NATS    │  │  Cache   │  │ SQLite  │ │
│  │  Server  │  │  Server  │  │ (Ristr.) │  │ (embedded)│
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬────┘ │
│       │             │             │              │      │
│  ┌────┴─────────────┴─────────────┴──────────────┴────┐│
│  │              Service Layer (business logic)         ││
│  └──────────────────────┬──────────────────────────────┘│
│                         │                               │
│  ┌──────────────────────┴──────────────────────────────┐│
│  │           Embedded Frontend (static assets)          ││
│  └─────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────┘
       │              │              │
   Browser       LLM APIs      GitHub Releases
   (user)        (external)     (installation)
```

---

## 2. Business Rules

### 2.1 Single-Binary Mandate

- **BR-01**: The default deployment is a single binary with zero external dependencies. No daemon, no container, no separate process.
- **BR-02**: The binary must be self-contained: database, messaging, caching, telemetry, and frontend assets all run in-process.
- **BR-03**: External dependencies (PostgreSQL, remote NATS, Valkey, OTLP) are available via configuration flags for enterprise deployments but are never required for default operation.

### 2.2 Installation Contract

- **BR-04**: Installation must work with a single `curl -fsSL https://ace.dev/install.sh | sh` command.
- **BR-05**: The installed binary must be verifiable via a standalone `verify.sh` script for users who reject pipe-to-shell.
- **BR-06**: Binary checksums (SHA256) must be published alongside every GitHub release.
- **BR-07**: The default install target is `$HOME/.local/bin/ace` (XDG convention). A `-b` flag overrides the target directory.
- **BR-08**: Install script must detect OS (linux/darwin) and architecture (amd64/arm64) automatically.

### 2.3 Data Ownership

- **BR-09**: All persistent data lives under a single root: `$XDG_DATA_HOME/ace/` by default.
- **BR-10**: Configuration lives at `$XDG_CONFIG_HOME/ace/config.yaml` by default.
- **BR-11**: Log output goes to both stdout (JSON) and `$XDG_STATE_HOME/ace/logs/ace.log`.
- **BR-12**: The `--data-dir` CLI flag and `$ACE_DATA_DIR` environment variable override the default data root, in that priority order.
- **BR-13**: The `ace paths` command prints all configured paths for transparency.

### 2.4 Performance Budgets

- **BR-14**: Cold start (from binary launch to serving HTTP) must complete in under 5 seconds.
- **BR-15**: The total memory footprint for default operation must stay under 200MB.
- **BR-16**: The total disk footprint (binary + data) must stay under 200MB for the initial empty state.
- **BR-17**: `make test` (the full validation pipeline) must complete in under 30 seconds.

### 2.5 Interface Preservation

- **BR-18**: The existing `Cache` interface (`shared/caching.Cache`) is preserved. Only the backend implementation changes (ValkeyBackend → InProcessBackend).
- **BR-19**: The existing messaging contract (`shared/messaging`) is preserved. The NATS client library is unchanged; only the server connection switches from TCP to in-process.
- **BR-20**: The existing `shared/telemetry` interfaces are preserved. Custom SQLite-backed exporters replace the OTLP collector, but the SDK instrumentation APIs remain identical.
- **BR-21**: All existing HTTP API routes (`/api/v1/*`) are preserved without breaking changes.

### 2.6 Development Flow

- **BR-23**: `make ace` produces the single binary with embedded frontend. No Docker/Podman required.
- **BR-24**: `make test` is the single source of truth for validation. It runs Go build, Go lint, Go tests (with `-short`), SQLC generate, docs generation, frontend lint, and frontend test.
- **BR-25**: The git pre-commit hook calls `make test` exclusively.
- **BR-26**: In development mode, the Go binary proxies frontend requests to `vite dev` server for HMR. In production mode, it serves embedded static assets.

---

## 3. Data Model

### 3.1 Storage Architecture

ACE stores all persistent data in a single directory tree rooted at the data directory:

```
$ACE_DATA_DIR/
├── ace.db                    # Primary SQLite database (WAL mode)
├── ace.db-wal                # SQLite write-ahead log
├── ace.db-shm                # SQLite shared memory
├── nats/                     # JetStream storage for embedded NATS
├── logs/                     # Structured application logs
│   └── ace.log
├── config.yaml               # Configuration override (optional)
└── telemetry/                # Telemetry data (same SQLite instance)
    └── metrics.db            # (future: consolidated into ace.db)
```

### 3.2 Key Entities

```
┌─────────────┐     ┌─────────────┐     ┌──────────────────┐
│  ace.db      │     │  NATS Store  │     │  Telemetry        │
│  (SQLite)    │     │  (JetStream) │     │  (SQLite tables)   │
├─────────────┤     ├─────────────┤     ├──────────────────┤
│ sessions     │     │ streams      │     │ ott_spans         │
│ agents       │     │ consumers    │     │ ott_metrics       │
│ permissions  │     │ messages     │     │ usage_events       │
│ auth_tokens  │     │              │     │                    │
│ migrations   │     │              │     │                    │
└─────────────┘     └─────────────┘     └──────────────────┘
```

### 3.3 SQLite Configuration

The primary database uses the following mandatory pragmas at connection time:

| Pragma | Value | Purpose |
|--------|-------|---------|
| `journal_mode` | `WAL` | Concurrent readers with single writer |
| `foreign_keys` | `ON` | Enforce referential integrity |
| `busy_timeout` | `5000` | Retry on lock contention (5s) |
| `synchronous` | `NORMAL` | Balance safety and speed |

Connection string format:
```
file:{data_dir}/ace.db?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)
```

### 3.4 Database Switching

The system supports two database modes:

| Mode | Driver | Connection | Use Case |
|------|--------|------------|----------|
| **Embedded** (default) | `modernc.org/sqlite` | `file:ace.db?_pragma=...` | Single binary, local dev |
| **External** (enterprise) | `github.com/lib/pq` | `postgres://host:5432/ace` | HA, multi-node |

The switch is driven by a single configuration key. SQLC interfaces remain identical; only the driver and connection string change. Goose migrations run against both dialects.

---

## 4. System Behavior

### 4.1 Startup Sequence

```
ace [flags]
  │
  ├─ 1. Parse config (CLI flags > env vars > config.yaml > defaults)
  ├─ 2. Initialize data directory (mkdir -p, permissions 0700)
  ├─ 3. Initialize SQLite (open conn, pragmas, run migrations)
  ├─ 4. Initialize NATS (embedded server, DontListen: true, JetStream)
  ├─ 5. Initialize Cache (Ristretto backend, memory budget)
  ├─ 6. Initialize Telemetry (OTel SDK → SQLite exporters + zap logger)
  ├─ 7. Initialize Services (wire dependencies via constructor injection)
  ├─ 8. Initialize HTTP Server
│     ├─ /api/v1/*     → API handlers
│     ├─ /telemetry/* → Telemetry Inspector (spans, metrics, usage)
│     └─ /*            → SPA handler (embedded assets or vite proxy)
  └─ 9. Serve until shutdown signal
```

**Startup invariants:**
- Each subsystem initializes sequentially. No subsystem starts until its dependencies are ready.
- NATS server must be ready before any service attempts to publish or subscribe.
- Database migrations run on every startup. Unknown migrations fail the startup.
- If any subsystem fails to initialize, the process exits with a non-zero code and a descriptive error.

### 4.2 Shutdown Sequence

```
SIGINT/SIGTERM received
  │
  ├─ 1. Stop accepting new HTTP requests (drain connections, 10s timeout)
  ├─ 2. Flush telemetry (span/metric exporters → SQLite)
  ├─ 3. Drain NATS (graceful unsubscribe, 5s timeout)
  ├─ 4. Close cache (Ristretto doesn't need explicit close)
  ├─ 5. Close SQLite (checkpoint WAL, close connections)
  └─ 6. Exit 0
```

### 4.3 Configuration Resolution

Priority order (highest wins):

| Priority | Source | Example |
|----------|--------|---------|
| 1 | CLI flag | `--data-dir=/custom/path` |
| 2 | Environment variable | `ACE_DATA_DIR=/custom/path` |
| 3 | Config file | `$XDG_CONFIG_HOME/ace/config.yaml` |
| 4 | XDG default | `$XDG_DATA_HOME/ace/` |
| 5 | Hardcoded fallback | `~/.local/share/ace/` |

Key configuration flags:

| Flag | Env Var | Default | Purpose |
|------|---------|---------|---------|
| `--data-dir` | `ACE_DATA_DIR` | `$XDG_DATA_HOME/ace/` | Data root directory |
| `--config` | `ACE_CONFIG` | `$XDG_CONFIG_HOME/ace/config.yaml` | Config file path |
| `--port` | `ACE_PORT` | `8080` | HTTP listen port |
| `--db-mode` | `ACE_DB_MODE` | `embedded` | `embedded` or `external` |
| `--db-url` | `ACE_DB_URL` | (auto) | Database connection string |
| `--nats-mode` | `ACE_NATS_MODE` | `embedded` | `embedded` or `external` |
| `--nats-url` | `ACE_NATS_URL` | (in-process) | NATS connection URL |
| `--cache-mode` | `ACE_CACHE_MODE` | `embedded` | `embedded` or `external` |
| `--cache-url` | `ACE_CACHE_URL` | (in-process) | Valkey/Redis connection URL |
| `--telemetry-mode` | `ACE_TELEMETRY_MODE` | `embedded` | `embedded` or `external` |
| `--dev` | `ACE_DEV` | `false` | Enable Vite dev proxy for frontend |

### 4.4 Messaging Behavior

The embedded NATS server operates in a specific configuration:

- **`DontListen: true`**: No TCP listener. The server does not accept external connections.
- **In-process connections**: All clients connect via `nats.InProcessServer()`, which uses `net.Pipe` for zero-copy in-process communication.
- **JetStream**: Enabled for durable message storage. Streams persist to `$ACE_DATA_DIR/nats/`.
- **Subject hierarchy**: Unchanged from current system. All existing NATS subject patterns continue to work.
- **Graceful drain**: On shutdown, in-process clients drain connections before the server stops.

Switching to external mode:
- `--nats-mode=external` changes the client connection from in-process to a network URL.
- The embedded server is not started. The client connects to the remote cluster.
- All subject patterns, message envelopes, and subscription logic remain identical.

### 4.5 Caching Behavior

The in-process cache operates under memory constraints:

- **Memory budget**: 50MB hard limit (configurable via `$ACE_CACHE_MAX_COST`).
- **Eviction**: TinyLFU admission policy (Ristretto). When budget is exceeded, least-cost entries are evicted.
- **Tag invalidation**: Implemented at the application layer using a secondary index. Tags are stored as sorted sets mapping tag → set of cache keys. `DeleteByTag` looks up the tag set, deletes all member keys, then removes the tag set.
- **Stampede protection**: Uses `golang.org/x/sync/singleflight` wrapped in the existing `SingleFlight` interface.
- **Version invalidation**: Each cache entry stores its version stamp. `InvalidateByVersion` compares the expected version and invalidates on mismatch.

The `CacheBackend` interface is preserved. A new `InProcessBackend` struct implements this interface using Ristretto as the underlying store, plus application-managed tag indexes.

### 4.6 Telemetry Behavior

The embedded telemetry subsystem replaces the external observability stack:

| Aspect | Current (External) | New (Embedded) |
|--------|--------------------|-----------------|
| **Traces** | OTLP → Tempo | OTel SDK → SQLite `ott_spans` table |
| **Metrics** | OTLP → Prometheus | OTel SDK → SQLite `ott_metrics` table |
| **Logs** | Structured → Loki | Structured → stdout (JSON) + local file |
| **Usage Events** | NATS → processed | Direct write → SQLite `usage_events` table |
| **Dashboards** | Grafana | `/telemetry/*` JSON endpoints |

**Inspector endpoints** (product-facing, on main API port):

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/telemetry/spans` | Query recent traces |
| `GET` | `/telemetry/metrics` | Query metric summaries |
| `GET` | `/telemetry/usage` | Query cost attribution data |
| `GET` | `/telemetry/health` | System health (DB, NATS, cache status) |

**Data retention**: SQLite tables for telemetry data use time-based cleanup. Spans older than 7 days and metrics older than 24 hours are pruned on startup and every 6 hours.

**Enterprise export**: When `--telemetry-mode=external`, the custom SQLite exporters are replaced by standard OTLP exporters pointing to configured collector endpoints.

### 4.7 Frontend Serving Behavior

Two modes, controlled by build tags:

| Mode | Build Tag | Behavior |
|------|-----------|----------|
| **Production** | default (no tag) | `go:embed all:build` serves static assets. SPA fallback routes all non-asset requests to `index.html`. |
| **Development** | `dev` | No embedded assets. HTTP handler proxies `/` requests to Vite dev server at `http://localhost:5173`. API routes still served by Go. |

**SPA routing rules** (production mode):
1. Request path has a file extension → serve from embedded FS (with appropriate `Content-Type`).
2. Request path starts with `/_app/` or `/@vite/` → serve from embedded FS (Vite-internal assets).
3. All other paths → serve `index.html` (client-side routing handles the rest).

**API routing**: All `/api/v1/*` requests are handled by the Go HTTP router before the SPA handler.

### 4.8 Installation Workflow

```
User runs: curl -fsSL https://ace.dev/install.sh | sh
  │
  ├─ 1. Detect OS (linux/darwin) and arch (amd64/arm64)
  ├─ 2. Fetch latest release tag from GitHub API
  ├─ 3. Download binary: github.com/ace-org/ace/releases/download/{tag}/ace_{os}_{arch}
  ├─ 4. Download checksums.txt from same release
  ├─ 5. Verify SHA256 of binary against checksums.txt
  ├─ 6. Install binary to $HOME/.local/bin/ace (or -b override)
  ├─ 7. Make binary executable
  ├─ 8. Check if install dir is in $PATH
  │     └─ If not: print shell-appropriate export command
  └─ 9. Print success message with next steps
```

**Verification script** (`verify.sh`): Standalone script that downloads checksums.txt and verifies an already-downloaded binary. For users who reject pipe-to-shell.

---

## 5. Interface Boundaries

### 5.1 CLI Interface

```
ace [global flags] [command]

Global Flags:
  --data-dir string       Data root directory ($ACE_DATA_DIR)
  --config string         Config file path ($ACE_CONFIG)
  --port int              HTTP listen port (default 8080)
  --db-mode string        Database mode: embedded|external (default "embedded")
  --db-url string         Database connection URL (auto-constructed in embedded mode)
  --nats-mode string      NATS mode: embedded|external (default "embedded")
  --nats-url string       NATS connection URL (in-process in embedded mode)
  --cache-mode string     Cache mode: embedded|external (default "embedded")
  --cache-url string      Cache connection URL (unused in embedded mode)
  --telemetry-mode string  Telemetry mode: embedded|external (default "embedded")
  --dev                   Enable development mode (Vite proxy)

Commands:
  ace              Start the ACE server (default if no command)
  ace paths        Print all configured paths (data, config, logs, cache)
  ace version      Print version information
  ace migrate      Run database migrations manually
  ace help         Print help
```

### 5.2 HTTP API Boundaries

The Go binary exposes a single HTTP server on `--port` (default 8080):

| Route Group | Prefix | Handler | Mode |
|-------------|--------|---------|------|
| **API** | `/api/v1/*` | Business logic handlers | Always |
| **Telemetry API** | `/telemetry/*` | Inspector handlers | Always |
| **SPA** | `/*` (catch-all) | Embedded FS or Vite proxy | Production: embedded / Dev: proxy |

### 5.3 Build System Interface

The Makefile is simplified to three primary targets:

```makefile
ace:    Build the Go binary (includes embedded frontend if built)
ui:     Build the SvelteKit frontend (produces build/ directory)
test:   Full validation pipeline
```

**`make test` pipeline** (single source of truth):

```
┌─────────────────┐
│   make test      │
├─────────────────┤
│ Go build        │
│ Go lint/vet     │
│ Go test -short  │
│ SQLC generate   │
│ Docs generation │
│ FE lint         │
│ FE test         │
│ Makefile check  │
└─────────────────┘
     target: <30s
```

### 5.4 Package Interface Boundaries

The transition from multi-module workspace to single module with `internal/` packages preserves these public surfaces:

| Current Package | New Package | Interface Preserved |
|----------------|-------------|-------------------|
| `ace/shared/caching` | `ace/internal/caching` | `Cache`, `CacheBackend`, `CacheObserver` interfaces |
| `ace/shared/messaging` | `ace/internal/messaging` | `Client`, `Subscription` types, all publish/subscribe methods |
| `ace/shared/telemetry` | `ace/internal/telemetry` | `Logger`, `Tracer`, `Meter`, `Usage` interfaces |
| `ace/api/internal/handler` | `ace/internal/api/handler` | HTTP handler signatures |
| `ace/api/internal/service` | `ace/internal/api/service` | Service interface signatures |
| `ace/api/internal/repository` | `ace/internal/api/repository` | Repository interface signatures |

**Key invariant**: Every `internal/` package exposes the same interface its `shared/` predecessor did. Only the package path changes. Callers update imports; behavior is identical.

### 5.5 Build Tag Interface

Two build tags control binary behavior:

| Tag | Files Affected | Behavior |
|-----|---------------|----------|
| (none, default) | `frontend_embed.go` | Embeds `build/` directory. Serves static assets via `go:embed`. |
| `dev` | `frontend_dev.go` | No embedded assets. Proxies `/` to `http://localhost:5173`. API routes still served by Go. |

Build commands:
- Production: `go build ./cmd/ace/` (includes `frontend_embed.go`)
- Development: `go build -tags dev ./cmd/ace/` (includes `frontend_dev.go`)

---

## 6. Migration & Cleanup Rules

### 6.1 What Gets Deleted

The following are removed entirely as part of this unit:

| Item | Reason |
|------|--------|
| `devops/` directory | No containers needed. All dependencies are embedded. |
| `changelogs/` directory | Consolidated into git history or removed per problem space scope. |
| `backend/go.work` | Single `go.mod` replaces workspace. |
| `backend/shared/go.mod` | Merged into single module. |
| `backend/shared/messaging/go.mod` | Merged into single module. |
| `backend/shared/telemetry/go.mod` | Merged into single module. |
| `backend/scripts/docs-gen/go.mod` | Merged into single module as Makefile target. |
| Docker/Podman configs | No containers needed. |

### 6.2 What Gets Replaced

| Current | Replacement | Interface Change |
|---------|-------------|-----------------|
| PostgreSQL container | Embedded SQLite (modernc.org/sqlite) | Driver switch in config. SQLC dialect changes. |
| NATS container | Embedded NATS server | No client changes. Connection switches to in-process. |
| Valkey container | Ristretto in-process backend | New `InProcessBackend` implements existing `CacheBackend` interface. |
| Grafana/Prometheus/Loki/Tempo stack | SQLite-backed OTel exporters + `/telemetry/` endpoints | New telemetry Inspector replaces dashboard access. |
| SvelteKit adapter-node | SvelteKit adapter-static | Build output changes to static files. No runtime Node.js. |
| `go.work` multi-module | Single `go.mod` with `internal/` | Import paths change. No functional changes. |

### 6.3 Enterprise Extension Points

Enterprise mode activates via configuration flags. Each `-mode=external` flag switches a subsystem from embedded to external:

| Subsystem | Embedded Mode | External Mode |
|-----------|--------------|----------------|
| Database | SQLite in-process | PostgreSQL URL |
| Messaging | NATS in-process | NATS cluster URL |
| Cache | Ristretto in-process | Valkey URL |
| Telemetry | SQLite exporters | OTLP collector URLs |

Switching is per-subsystem. An operator can use embedded database with external NATS, for example. No subsystem requires any other to be in external mode.