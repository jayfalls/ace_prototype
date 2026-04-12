# Problem Space: Deployment & Developer Experience

**Unit:** deployment-dx  
**Status:** Discovery  
**Date:** 2026-04-12  

---

## 1. Problem Statement

ACE currently requires 10+ containerized services (PostgreSQL, NATS, Valkey, Prometheus, Grafana, Loki, Tempo, OTEL Collector, plus app containers) orchestrated via Docker/Podman Compose, with a multi-module Go workspace (`go.work` with 4 modules), Dockerfiles for API and frontend, and a complex Makefile that orchestrates container lifecycle. This creates a hostile developer experience: 30+ second cold starts, multi-gigabyte resource footprints, and an insurmountable barrier for anyone who just wants to try ACE.

The goal is to transform ACE from a complex multi-container orchestration into a **single Go binary** that can be installed with `curl | sh` and run immediately. The binary must embed the frontend, use internal systems for persistence/messaging/caching/telemetry, and still support optional enterprise hooks for external observability stacks.

**Core Conflict:** External dependencies give ACE battle-tested reliability (PostgreSQL's ACID, NATS's JetStream, Valkey's speed) but at the cost of operational complexity that kills adoption. Going internal removes that complexity but requires us to build or embed alternatives that are "good enough" for the single-agent/hobbyist deployment while preserving the ability to scale to enterprise later.

---

## 2. Constraints

- **Cross-platform:** Linux primary, macOS secondary. Binary must compile for both `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`.
- **Single binary deployment model.** No external daemons, no Docker, no containers required for the default case.
- **Preserve existing API contracts.** All current HTTP endpoints, NATS subject patterns, and telemetry shapes must continue to function. Internal replacements must implement the same interfaces.
- **Optional enterprise hooks.** When configured, the system must be able to export OTLP traces/metrics to external collectors, connect to external PostgreSQL/NATS/Valkey for scale-out deployments. The default is embedded; the enterprise path is pluggable.
- **Hot reload for development.** The dev experience must preserve or improve on current `air`-based hot reload for backend and Vite HMR for frontend.
- **Preserve `shared/` package interfaces.** The `messaging`, `telemetry`, and `caching` packages define contracts that internal replacements must satisfy. No breaking changes to the interface layer without a migration path.
- **Go 1.26 baseline.** All solutions must work with the current toolchain.
- **SvelteKit frontend.** Currently uses `adapter-node`. Must migrate to `adapter-static` for embedding, with SPA catch-all routing.

---

## 3. Research Findings

### 3.1 Database Options

The current system uses PostgreSQL 18 via `pgx/v5` with SQLC-generated queries, Goose Go-migrations, and 5 migration files. Any embedded database must support ACID transactions and allow a migration path from current PostgreSQL schemas.

| Criterion | SQLite (modernc.org/sqlite) | BadgerDB (dgraph-io/badger/v4) | Pebble (cockroachdb/pebble) | BoltDB (go.etcd.io/bbolt) |
|---|---|---|---|---|
| **Data Model** | Relational (SQL) | KV store (LSM-tree) | KV store (LSM-tree) | KV store (B+ tree) |
| **ACID** | Full ACID (WAL mode) | ACID with SSI transactions | Limited (single-range transactions) | Full ACID (serializable) |
| **CGo Required** | No (pure Go transpilation) | No (pure Go) | No (pure Go) | No (pure Go) |
| **Query Language** | SQL (migration from PostgreSQL is conceptually direct) | Go API only | Go API only | Go API only |
| **Write Performance** | Single-writer, good read concurrency | High write throughput | Optimized for CockroachDB internals | Moderate (B+ tree) |
| **Read Performance** | Good with WAL mode, indexes | Good with iterators | Excellent for range scans | Good for point lookups |
| **Migration from PostgreSQL** | Most compatible: SQL→SQL, schema changes mostly portable | Complete rewrite: schema must be reimagined as KV | Complete rewrite | Complete rewrite |
| **Cross-Compilation** | Full support (pure Go) | Full support (pure Go) | Full support (pure Go) | Full support (pure Go) |
| **Concurrency** | WAL mode: concurrent reads, single writer | Concurrent reads and writes with transactions | Concurrent, designed for high write loads | Single writer via mutex, concurrent readers |
| **Maturity** | Production-ready, used in many Go projects | Production-ready (Dgraph, BadgerDB itself) | Production-ready (CockroachDB backbone) | Production-ready (etcd backbone) |
| **WAL/Recovery** | Built-in WAL mode | Built-in value log + LSM | Manifest + WAL | mmap-based, requires fsync |
| **Database Size** | Single file + WAL file | Directory of SST files | Directory of SST files | Single file |
| **Key Feature for ACE** | Direct SQL migration path, SQLC compatibility | High write throughput for messaging/events | Range scans for telemetry time-series | Simple API for config/state |

**Analysis:**

- **modernc.org/sqlite** is the strongest candidate for replacing PostgreSQL. It provides pure Go (no CGo), full SQL support, WAL mode for concurrent reads, and the most straightforward migration path. SQLC can generate queries for SQLite with minor dialect adjustments. The main concern is single-writer limitation, but ACE's current schema (agents, sessions, auth tokens, permissions) is moderate-write, not high-write.

- **BadgerDB** is excellent for the messaging/event store replacement (NATS JetStream data). Its LSM-tree architecture handles high-throughput sequential writes well, supports TTL natively, and provides ACID transactions. Not suitable as a direct PostgreSQL replacement because it would require reimplementing all SQL queries as KV operations.

- **Pebble** is primarily CockroachDB's internal storage engine. While extremely capable, it has a narrow API surface tailored to CockroachDB's needs. Using it outside that context means fighting its design assumptions. Not recommended.

- **BoltDB/bbolt** is simple and reliable (powers etcd). Best suited for small config/state stores, not for the primary data layer. Its single-writer B+ tree model limits write throughput.

**Recommendation:** Use **modernc.org/sqlite** as the primary embedded database (replacing PostgreSQL), with **BadgerDB** as the internal message/event store (replacing NATS JetStream persistence). See §3.2 for messaging details.

#### SQLite Migration Considerations

| PostgreSQL Feature | SQLite Equivalent | Migration Effort |
|---|---|---|
| `gen_random_uuid()` | Use Go-side UUID generation (`google/uuid`) | Low |
| `TIMESTAMPTZ` | `TEXT` with ISO 8601 format, or INTEGER epoch | Medium — ensure all queries use `datetime()` |
| `CREATE TRIGGER ... EXECUTE FUNCTION` | `CREATE TRIGGER ... FOR EACH ROW` with inline SQL | Medium — rewrite trigger functions |
| Row-level security | Not available | Low — not currently used |
| `pgx/v5` driver | `modernc.org/sqlite` driver with `database/sql` | Low — interface is `database/sql` compatible |
| SQLC compatibility | SQLC supports SQLite dialect | Low — regenerate with `engine: ["sqlite"]` |
| Concurrent writes | WAL mode + single connection pool | Medium — use `SetMaxOpenConns(1)` for writer, separate reader pool |
| `JSONB` columns | JSON1 extension (available by default) | Low — similar functionality |

### 3.2 Messaging Options

The current system uses NATS 2.12 with JetStream for:
- Cognitive engine inter-layer communication (subjects: `ace.engine.{agentID}.layer.{layerID}.{input|output}`)
- Usage events with 30-day retention (`ace.usage.{agentID}.{token|cost}`)
- System events with work queue policy (`ace.system.agents.{spawn|shutdown}`, `ace.system.health.{agentID}`)
- Memory operations (`ace.memory.{agentID}.{store|query|result}`)
- LLM request/reply (`ace.llm.{agentID}.{request|response}`)
- Tool invocation (`ace.tools.{agentID}.{toolID}.{invoke|result}`)
- Senses events (`ace.senses.{agentID}.{senseID}.event`)
- Dead letter queue for failed messages

The current code uses SubscribeToStream (JetStream consumers), RequestReply, and Publish patterns with envelope-based message tracking.

| Criterion | Embedded NATS Server | In-Memory Go Bus (custom) | Watermill + In-Memory | BadgerDB-backed Event Log |
|---|---|---|---|---|
| **JetStream Parity** | Full parity (same server, in-process) | Must rebuild persistence, ACK, replay | Must rebuild persistence, ACK, replay | Event log + pub/sub; no JetStream features |
| **Persistence** | Built-in (file-based JetStream) | None (process death = message loss) | None by default | BadgerDB provides durability |
| **API Compatibility** | 100% — same Go client, same subjects | New API surface needed | New API surface needed | New API surface needed |
| **Binary Size** | +8-12MB (NATS server embedded) | Negligible | Negligible | +5-7MB (BadgerDB) |
| **Memory Footprint** | Moderate (runs embedded, no TCP overhead possible) | Minimal | Minimal | Moderate (LSM compaction) |
| **Message Replay** | JetStream replay | No | No | BadgerDB iterators |
| **At-Least-Once Delivery** | JetStream consumers | Must implement | Must implement | Must implement on top of BadgerDB |
| **Request-Reply** | Built-in | Must implement | Must implement | Must implement |
| **Startup Time** | ~100ms (embedded server init) | Instant | Instant | ~50ms (BadgerDB open) |
| **Complexity** | Low (reuse existing code) | High (rebuild everything) | Medium (Watermill abstraction) | High (build custom event system) |

**Analysis:**

- **Embedded NATS** (`github.com/nats-io/nats-server/v2/server`) is the lowest-risk, highest-compatibility option. It runs NATS in-process, supports all JetStream features, and requires zero changes to messaging code. The NATS Go client already supports connecting to an embedded server via `nats.Connect()` with an in-process connector. Binary size increase is acceptable (~8-12MB).

- **Pure Go in-memory bus** would require rebuilding message persistence, at-least-once delivery, request-reply patterns, dead letter queues, and consumer acknowledgment — essentially reimplementing what NATS JetStream already provides. This is months of work with high risk.

- **Watermill** is a Go messaging abstraction that supports multiple pub/sub implementations. It could provide an in-memory adapter, but lacks persistence and JetStream-like features out of the box.

- **BadgerDB-backed event log** would give persistence but still requires building the pub/sub layer, consumer management, replay, and ACK systems on top.

**Recommendation:** Use **embedded NATS server** for the default single-binary mode. The `shared/messaging` package already wraps NATS client calls — all that changes is the server running in-process instead of as a separate container. For enterprise deployments that need scale-out, the same code connects to an external NATS cluster. This is a configuration switch, not a code rewrite.

### 3.3 Caching Options

The current system uses Valkey (Redis-compatible) via `valkey-go` client with a `shared/caching` package that provides:
- Namespace-scoped keys with agent ID prefix
- TTL-based expiration
- Tag-based invalidation (`DeleteByTag`)
- Stampede protection (`GetOrFetch`)
- Version-based invalidation
- Cache stats (hit/miss rates)

| Criterion | Ristretto (dgraph-io/ristretto/v2) | BigCache (allegro/bigcache/v3) | go-cache (patrickmn/go-cache) | golang-lru (hashicorp/golang-lru) |
|---|---|---|---|---|
| **TTL Support** | Per-item TTL via `SetWithTTL` | Per-item `LifeWindow` | Per-item TTL | No native TTL (LRU eviction by size) |
| **Eviction Policy** | TinyLFU admission + SampledLFU eviction | Time-based expiration + size limit | TTL expiration, no size eviction | LRU by size |
| **Tag-based Invalidation** | No (must build on top) | No (must build on top) | No (must build on top) | No (must build on top) |
| **Memory Limit** | Yes (`MaxCost` cost-based) | Yes (`HardMaxCacheSize` in MB) | No unbounded by default | Yes (fixed size) |
| **Concurrency** | High (sharded, contention-free) | High (sharded, lock-free) | Moderate (`sync.RWMutex`) | High (sharded locks in v2) |
| **Hit Ratio** | Best-in-class (TinyLFU admission) | Good (time-based) | Basic (no admission policy) | LRU — decent for scan-resistant workloads |
| **Stats/Metrics** | Built-in `Metrics` collection | `Stats()` with hits/misses/collisions | No built-in stats | No built-in stats |
| **Generics** | Yes (v2: `Cache[K, V]`) | No (values are `[]byte`) | No (`map[string]Item`) | Yes (v2: `LRY[K, V]` recently) |
| **Binary Size** | Small (~200KB) | Small (~150KB) | Tiny (~30KB) | Tiny (~20KB) |
| **Stampede Protection** | No (must build on top) | No (must build on top) | No (must build on top) | No |

**Analysis:**

None of the in-memory caches provide tag-based invalidation or stampede protection out of the box — these are custom features in the current `shared/caching` layer. The replacement must either:
1. Rebuild the `Cache` interface using an in-memory backend and implementing tag/stampede on top, or
2. Use embedded Valkey (Valkey can be embedded via its module system, but this is complex).

**Ristretto v2** is the strongest candidate:
- Generics support (`Cache[string, []byte]`) eliminates `interface{}`/`any`
- Cost-based admission is better than simple LRU for ACE's mixed-size cached values
- Built-in metrics align with current caching stats needs
- Production-proven (powers Dgraph)
- TinyLFU admission provides superior hit ratios for the access patterns ACE's caching layer sees (many agents reading the same configs/prompts)

**Tag-based invalidation** can be built on top of Ristretto by maintaining a reverse index (tag → set of cache keys) in a separate BadgerDB bucket or in-memory map. This is exactly how Valkey implements tag-based invalidation internally with sets.

**Stampede protection** (`GetOrFetch`/singleflight pattern) can be implemented using Go's `golang.org/x/sync/singleflight`, which is already an indirect dependency.

**Recommendation:** Use **Ristretto v2** as the in-memory cache backend, with custom layers for tag-based invalidation (reverse index map) and stampede protection (`singleflight` wrapper). This stays within pure Go and covers all current `shared/caching` features.

### 3.4 Installation Best Practices

Research into successful Go CLI tools (OpenCode, Rustup, Homebrew) reveals consistent patterns:

#### Binary Installation Path

| Tool | Install Location | Method | Data Directory |
|---|---|---|---|
| OpenCode | `~/.opencode/bin/` | `curl \| sh`, Homebrew, npm | `~/.config/opencode/` |
| Rustup | `~/.cargo/bin/` | `curl \| sh` | `~/.cargo/`, `~/.rustup/` |
| Homebrew | `/opt/homebrew/bin/` (macOS), `/usr/local/bin/` (Linux) | Native package | `/opt/homebrew/` |
| Docker | `/usr/local/bin/docker` | Package manager, `curl \| sh` | `/var/lib/docker/` |

**XDG Base Directory Specification** provides the platform-standard approach:

| Purpose | Linux | macOS |
|---|---|---|
| Config | `~/.config/ace/` | `~/Library/Application Support/ace/` |
| Data | `~/.local/share/ace/` | `~/Library/Application Support/ace/` |
| Cache | `~/.cache/ace/` | `~/Library/Caches/ace/` |
| State | `~/.local/state/ace/` | `~/Library/Application Support/ace/` |
| Runtime | `$XDG_RUNTIME_DIR/ace/` | `$TMPDIR/ace/` |

**OpenCode's install script** (`https://opencode.ai/install`) provides a reference pattern:
1. Detect OS/architecture
2. Download binary from GitHub releases to temp dir
3. Extract and move to `$HOME/.opencode/bin/`
4. Add to PATH via shell profile modification (`.bashrc`, `.zshrc`)
5. Support `--version`, `--binary`, `--no-modify-path` flags
6. Verify checksums

**Security considerations for `curl | sh`:**
- Always use `curl -fsSL` (fail silently, show errors, follow redirects, SSL)
- Verify binary checksums against published SHA256
- Use HTTPS exclusively
- Support `--dry-run` for inspection before execution
- Provide a `VERIFY` script that audits the install script before running
- Consider separate install/verify steps: `curl -fsSL https://ace.dev/install.sh | sh -s -- --verify`

**Recommendation for ACE:**

```
Binary:      ~/.local/bin/ace                    (XDG bin home, user-local, no sudo needed)
Config:      ~/.config/ace/ace.yaml             (XDG_CONFIG_HOME)
Data:        ~/.local/share/ace/               (XDG_DATA_HOME — SQLite DB, BadgerDB data)
Cache:       ~/.cache/ace/                     (XDG_CACHE_HOME — ephemeral data)
State:       ~/.local/state/ace/               (XDG_STATE_HOME — logs, telemetry ring buffers)
```

The Go library `adrg/xdg` (970+ GitHub stars, actively maintained) provides cross-platform XDG directory resolution with macOS and Windows fallbacks, eliminating the need to handle platform differences manually.

### 3.5 Frontend Embedding

The current frontend uses `@sveltejs/adapter-node`, which builds a Node.js server. For embedding, we need `@sveltejs/adapter-static` to produce static HTML/JS/CSS that Go's `embed` package can bundle into the binary.

#### Embedding Pattern

```go
// frontend.go — conditional embedding via build tags

//go:build embed

package frontend

import (
    "embed"
    "io/fs"
)

//go:embed all:dist
var distFS embed.FS

var Frontend fs.FS

func init() {
    sub, err := fs.Sub(distFS, "dist")
    if err != nil {
        panic(err)
    }
    Frontend = sub
}

var IsEmbedded = true
```

```go
// frontend_noembed.go — development mode

//go:build !embed

package frontend

import (
    "os"
    "io/fs"
)

var Frontend fs.FS

func init() {
    // In dev mode, serve from live Vite dev server,
    // or read from local filesystem
    Frontend = os.DirFS("frontend/dist")
}

var IsEmbedded = false
```

#### SPA Catch-All Routing

For an SPA, all non-API routes must serve `index.html`:

```go
func spaHandler(frontend fs.FS) http.HandlerFunc {
    indexHTML, _ := fs.ReadFile(frontend, "index.html")
    fileServer := http.FileServer(http.FS(frontend))
    
    return func(w http.ResponseWriter, r *http.Request) {
        path := r.URL.Path
        // Try serving the static file first
        if f, err := frontend.Open(path[1:]); err == nil {
            f.Close()
            fileServer.ServeHTTP(w, r)
            return
        }
        // Fall back to index.html for SPA routing
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.Write(indexHTML)
    }
}
```

#### Dev vs Prod Asset Handling

| Mode | Build Tag | Frontend Source | Hot Reload |
|---|---|---|---|
| Development | `!embed` | Vite dev server at `:5173` (proxy) | Vite HMR |
| Production | `embed` | Embedded `dist/` via `go:embed` | N/A |

**Build commands:**
- `make ace` → `cd frontend && npm run build && cd .. && go build -tags embed -o ace ./cmd/ace`
- `make ui` → `cd frontend && npm run dev` (standalone frontend development)
- `make dev` → `air -tags !embed` with Vite proxy for frontend

**Current adapter migration:** Change `svelte.config.js` from `adapter-node` to `adapter-static` with SPA fallback:

```javascript
import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/kit/vite-plugin-svelte';

export default {
    preprocess: vitePreprocess(),
    kit: {
        adapter: adapter({
            fallback: 'index.html'  // SPA mode
        })
    }
};
```

SvelteKit routes using `+page.ts` or `+layout.ts` with `load` functions that use server-side features need migration to `+page.js` with client-only data fetching, or prerendering enabled per-route.

### 3.6 Custom Telemetry Architecture

The current system deploys Prometheus, Grafana, Loki, Tempo, and an OTEL Collector — 5 observability containers — to provide:
- Distributed traces with span attributes (`agentId`, `cycleId`, `serviceName`)
- Metrics (request latency, error rates)
- Structured logs with context
- Usage events (`ace.usage.{agentID}.{token|cost}`)
- Health checks

For a single-binary deployment, this external stack must be replaced with an internal telemetry system while preserving the `shared/telemetry` interface.

#### Requirements for Internal Telemetry

1. **Usage tracking:** Token counts, cost attribution per agent/cycle/session. Must persist to the embedded database for querying.
2. **Cost analysis:** Aggregate spend per agent, per model, per time window.
3. **Self-improvement loops:** Cognitive engine needs trace/span data to evaluate its own performance.
4. **Layer Inspector reconstruction:** Must be able to replay any agent's cognitive cycle by trace ID.
5. **Optional enterprise export:** When `OTEL_ENDPOINT` is configured, export traces/metrics via OTLP.

#### Architecture: Ring Buffer + SQLite Persistence

```
┌─────────────────────────────────────────────────┐
│                 ACE Binary                        │
│                                                   │
│  ┌──────────┐   ┌────────────┐   ┌────────────┐ │
│  │ Traces   │   │  Metrics   │   │   Logs     │ │
│  │ (Ring    │   │  (Ring     │   │  (Ring     │ │
│  │  Buffer) │   │   Buffer)  │   │   Buffer)  │ │
│  └────┬─────┘   └─────┬──────┘   └─────┬──────┘ │
│       │               │                │         │
│       └───────────────┼────────────────┘         │
│                       │                           │
│              ┌────────▼────────┐                  │
│              │  Telemetry       │                  │
│              │  Aggregator      │                  │
│              │  (batch writer)  │                  │
│              └────────┬────────┘                  │
│                       │                           │
│       ┌───────────────┼───────────────┐           │
│       │               │               │           │
│  ┌────▼─────┐  ┌─────▼──────┐  ┌─────▼──────┐   │
│  │ SQLite   │  │ API        │  │ OTLP       │   │
│  │ Persist   │  │ Endpoints  │  │ Export     │   │
│  │ (usage,  │  │ (/debug/   │  │ (optional, │   │
│  │  traces, │  │  traces,   │  │  when      │   │
│  │  metrics)│  │  metrics)  │  │  enabled)  │   │
│  └──────────┘  └────────────┘  └────────────┘   │
└─────────────────────────────────────────────────┘
```

#### Ring Buffer Specifications

| Component | Size | Eviction | Persistence |
|---|---|---|---|
| Trace Ring Buffer | Configurable, default 10K spans | Oldest evicted when full | Batched to SQLite every 5s |
| Metrics Ring Buffer | Configurable, default 1K data points | Oldest evicted when full | Batched to SQLite every 10s |
| Log Ring Buffer | Configurable, default 5K entries | Oldest evicted when full | Batched to SQLite every 5s |
| Usage Events | No buffer — direct write | N/A (critical path) | Synchronous SQLite write |

#### SQLite Telemetry Schema (New Tables)

```sql
CREATE TABLE IF NOT EXISTS telemetry_spans (
    trace_id    TEXT NOT NULL,
    span_id     TEXT NOT NULL,
    parent_id   TEXT,
    agent_id   TEXT NOT NULL,
    cycle_id    TEXT,
    operation   TEXT NOT NULL,
    start_time  INTEGER NOT NULL,  -- nanoseconds since epoch
    end_time    INTEGER NOT NULL,
    attributes  TEXT,               -- JSON
    status      TEXT NOT NULL DEFAULT 'ok',
    PRIMARY KEY (trace_id, span_id)
);

CREATE TABLE IF NOT EXISTS telemetry_metrics (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL,  -- counter, gauge, histogram
    value       REAL NOT NULL,
    labels      TEXT,           -- JSON
    timestamp   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS usage_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id    TEXT NOT NULL,
    cycle_id    TEXT,
    session_id  TEXT,
    service     TEXT NOT NULL,
    operation   TEXT NOT NULL,
    resource    TEXT NOT NULL,
    tokens_in   INTEGER DEFAULT 0,
    tokens_out  INTEGER DEFAULT 0,
    cost_usd    REAL DEFAULT 0.0,
    duration_ms INTEGER DEFAULT 0,
    metadata    TEXT,           -- JSON
    created_at  INTEGER NOT NULL
);

CREATE INDEX idx_spans_agent ON telemetry_spans(agent_id, start_time);
CREATE INDEX idx_spans_trace ON telemetry_spans(trace_id);
CREATE INDEX idx_metrics_name ON telemetry_metrics(name, timestamp);
CREATE INDEX idx_usage_agent ON usage_events(agent_id, created_at);
```

#### API Endpoints for Self-Inspection

When running in single-binary mode, expose debug endpoints:
- `GET /debug/traces?agent_id=&limit=` — recent traces
- `GET /debug/spans/{trace_id}` — full trace reconstruction
- `GET /debug/metrics?name=&window=` — metric time series
- `GET /debug/usage?agent_id=&from=&to=` — cost analysis
- `GET /debug/health` — extended health with telemetry status

These replace Grafana dashboards for the single-binary use case and power the Layer Inspector's UI.

#### `shared/telemetry` Interface Preservation

The current `telemetry.Init()` returns a struct with `Tracer`, `Meter`, `Logger`, `Usage` fields. The internal telemetry implementation will:
- Provide a `noop.Meter` and `noop.Tracer` when no OTLP endpoint is configured (ring buffers collect data instead)
- Provide real `otel.Tracer` and `otel.Meter` when OTLP endpoint is configured
- Always provide `zap.Logger` (writes to ring buffer + stderr)
- Always provide `UsagePublisher` (writes to SQLite usage_events table)

### 3.7 Build System Options

#### Current State
- `go.work` with 4 modules: `services/api`, `shared`, `shared/messaging`, `shared/telemetry`
- Docker Compose for all services
- `air` for hot reload (in container)
- Vendor directories per module

#### Proposed: Single Go Module

Migrate from `go.work` multi-module to a single `go.mod` at `backend/`:

```
backend/
├── go.mod              # module ace
├── go.sum
├── cmd/
│   └── ace/
│       └── main.go     # single entry point
├── internal/
│   ├── api/            # handler, service, repository (from services/api)
│   ├── messaging/      # from shared/messaging (internal now)
│   ├── telemetry/      # from shared/telemetry (internal now)
│   ├── caching/        # from shared/caching (internal now)
│   ├── config/         # configuration loading
│   ├── embedded/       # embedded NATS server, frontend FS
│   └── db/             # SQLite connection, migrations
├── migrations/         # Goose Go migrations (from services/api/migrations)
└── frontend/           # embedded frontend (build tag conditional)
    ├── frontend_embed.go
    └── frontend_noembed.go
```

**Rationale for single module:**
- Eliminates `go.work` complexity and version skew
- All `shared/` packages become `internal/` — they were never meant to be imported externally
- Simplifies `go build`, `go test`, `go mod tidy`
- Single `vendor/` directory
- SQLC can still generate into `internal/repository/generated/`

**Build commands:**
```makefile
ace:           # Build production binary
               cd frontend && npm run build
               cd backend && go build -tags embed -o ../ace ./cmd/ace

ace-dev:       # Development with hot reload
               cd backend && air -c air.toml

ui:            # Frontend development server  
               cd frontend && npm run dev

test:          # Run all tests
               cd backend && go test ./...

test-integration:  # Integration tests (needs SQLite, not PostgreSQL)
               cd backend && go test -tags integration ./...
```

#### Cross-Compilation Targets

```makefile
RELEASES = linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

release:     # Build all platform binaries
             for os arch in $(RELEASES); do \
               GOOS=$$os GOARCH=$$arch go build -tags embed -o dist/ace-$$os-$$arch ./cmd/ace; \
             done
```

The `modernc.org/sqlite` pure-Go library cross-compiles without CGo, making cross-compilation trivial. This would not be true with `mattn/go-sqlite3` (CGo dependency).

---

## 4. Risk Analysis

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| SQLite single-writer bottleneck under high concurrent writes | M | H | Use WAL mode, separate read/write connection pools (`MaxOpenConns(1)` for writer). For enterprise deployments, PostgreSQL remains available. |
| Embedded NATS server increases binary size (~8-12MB) | H | L | Acceptable trade-off for feature parity. LZMA compression reduces to ~3-4MB. UPX can further compress. |
| SQLite migration from PostgreSQL schemas requires dialect changes | M | M | Use SQLC's multi-engine support to generate both dialects. Maintain PostgreSQL兼容 drivers behind a config flag. |
| Tag-based cache invalidation on Ristretto requires custom reverse index | M | M | Build reverse index as in-memory `map[string]map[string]struct{}` with periodic cleanup. Validated pattern used by other caching systems. |
| Frontend adapter-static migration breaks SSR pages | M | M | Migrate SSR logic to client-side data fetching. Use `+page.js` instead of `+page.ts`. Enable prerendering for static pages. |
| `curl \| sh` install security concerns | L | M | Pin releases to SHA256 checksums. Provide standalone verification script. Support checksum-first install: Download, verify, then execute. |
| Loss of observability UX (Grafana dashboards) | H | M | Build `/debug/*` endpoints with JSON API. Future: lightweight web dashboard embedded in binary. |
| BadgerDB LSM compaction pauses under load | L | M | Tune `NumVersionsToKeep`, `NumLevelZeroTables`, `NumLevelZeroTablesStall`. Run compaction in background goroutine with rate limiting. |
| Enterprise users need to revert to external dependencies | L | H | Configuration-driven backend selection (`ACE_DB=sqlite\|postgres`, `ACE_MQ=embedded\|nats`, etc.). Same `shared/` interfaces, different implementations. |
| Go embed increases binary size by frontend asset size | H | L | Frontend builds are typically 500KB-2MB gzipped. Acceptable trade-off. UPX compression as optional release step. |

---

## 5. Recommendations

Preliminary recommendations for BSD phase detailed design:

1. **Database:** modernc.org/sqlite as primary embedded DB. BadgerDB for message/event persistence dual-use. Configuration flag to switch to PostgreSQL for enterprise.

2. **Messaging:** Embedded NATS server (`nats-io/nats-server`) in-process. Same NATS Go client code, different connection method (in-process vs TCP). Configuration flag for external NATS cluster.

3. **Caching:** Ristretto v2 with custom tag invalidation layer and singleflight stampede protection. In-process only. Configuration flag to switch to Valkey for distributed deployments.

4. **Telemetry:** Custom ring buffer system with SQLite persistence. No external collectors in default mode. OTLP export as opt-in configuration. `shared/telemetry` interface preserved with backend-agnostic implementation.

5. **Frontend:** Switch to `@sveltejs/adapter-static` with SPA fallback. Go embed with build tags for conditional embedding. Vite dev server proxied in development mode.

6. **Build System:** Consolidate to single Go module. `internal/` for all shared packages. `cmd/ace` as entry point. Three make commands: `ace`, `ui`, `test`.

7. **Installation:** `curl -fsSL https://ace.dev/install.sh | sh` script following OpenCode pattern. Binary at `~/.local/bin/ace`, data at `~/.local/share/ace/`, config at `~/.config/ace/`. Use `adrg/xdg` for cross-platform path resolution.

8. **Data Directory:** XDG Base Directory specification with `adrg/xdg` library. Database files in `$XDG_DATA_HOME/ace/` (`~/.local/share/ace/`). Config in `$XDG_CONFIG_HOME/ace/` (`~/.config/ace/`).

---

## 6. References

- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — Pure Go SQLite driver
- [SQLite in Go, with and without cgo](https://datastation.multiprocess.io/blog/2022-05-12-sqlite-in-go-with-and-without-cgo.html) — Performance benchmarks
- [PostgreSQL vs SQLite: scaling path](https://cr0x.net/en/sqlite-to-postgresql-no-downtime/) — Migration considerations
- [BadgerDB](https://github.com/dgraph-io/badger) — Embedded key-value database
- [Pebble](https://github.com/cockroachdb/pebble) — LSM-tree storage engine
- [bbolt](https://github.com/etcd-io/bbolt) — Embedded B+ tree database
- [NATS Server](https://github.com/nats-io/nats-server) — Embedded NATS server
- [NATS Go Client](https://github.com/nats-io/nats.go) — NATS client library
- [Ristretto](https://github.com/dgraph-io/ristretto) — High-performance Go cache
- [BigCache](https://github.com/allegro/bigcache) — Fast concurrent cache
- [Go embed package](https://pkg.go.dev/embed) — Go standard library embedding
- [Conditional embedding in Go](https://shallowbrooksoftware.com/posts/conditional-embedding-in-go/) — Build tag pattern for dev/prod
- [SvelteKit adapter-static](https://kit.svelte.dev/docs/adapter-static) — Static site generation
- [adrg/xdg](https://github.com/adrg/xdg) — Cross-platform XDG directory specification
- [OpenCode install script](https://github.com/sst/opencode/blob/dev/install) — Reference for `curl | sh` install pattern