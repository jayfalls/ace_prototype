# Research: Deployment & Developer Experience

**Unit:** deployment-dx
**Date:** 2026-04-12
**Status:** Research

---

## 1. Embedded Database

### Candidates

| | SQLite (modernc.org/sqlite) | bbolt (etcd-io/bbolt) | BadgerDB (dgraph-io/badger) | DQLite (canonical/dqlite) |
|---|---|---|---|---|
| **Type** | SQL (full RDBMS) | KV (B+tree) | KV (LSM tree) | SQL (distributed SQLite) |
| **ACID** | Full | Full | SSI transactions | Full |
| **SQLC Compatible** | Yes | No | No | Yes (SQLite dialect) |
| **CGo Required** | No (modernc) | No | No | Yes |
| **Concurrent Reads** | Excellent (WAL mode) | Good (read txns) | Excellent | Good |
| **Write Performance** | Good (WAL + immediate) | Moderate | Excellent | Good |
| **Migration Path** | SQL nearly identical to PG | Rewrite all queries | Rewrite all queries | SQL similar to PG |
| **Maturity** | Battle-tested (25+ years) | Production (etcd) | Production (dgraph) | Production (LXD) |
| **SQL Compatibility** | ~95% of PG dialect | N/A | N/A | ~95% of PG dialect |

### Analysis

**modernc.org/sqlite** is the clear choice. It is a pure-Go (no CGo) SQLite implementation that exposes a `database/sql` driver. This is critical because:

1. **SQLC compatibility**: The existing `sqlc.yaml` pipeline generates Go code from SQL queries. SQLite understands standard SQL, so most PostgreSQL queries port with minor syntax changes (e.g., `BOOLEAN` → `INTEGER`, `gen_random_uuid()` → auto-increment or explicit UUID generation). SQLC has a SQLite engine, so the code generation pipeline works without rearchitecting.

2. **Migration preservation**: The Goose migration system works with `database/sql`, which SQLite supports. Existing migration files can be adapted with minimal changes rather than rewritten.

3. **WAL mode**: Enables concurrent readers with a single writer, matching PostgreSQL's read concurrency model. With `PRAGMA journal_mode=WAL` and `PRAGMA busy_timeout=5000`, SQLite handles the ACE workload (many reads, moderate writes) well.

4. **No CGo**: The `modernc.org/sqlite` driver is a pure Go transpilation of SQLite C. This means the binary remains statically compilable with no C toolchain dependency.

5. **In-process**: Runs within the Go process. No daemon, no network, no sockets. Data stored as a single file in the data directory.

**Key trade-offs**: SQLite has a single-writer concurrency model. For ACE's workload (agent reads/writes, not high-concurrency OLTP), this is acceptable. WAL mode allows concurrent reads during writes. If enterprise deployments need PostgreSQL, the configuration flag can switch the driver while preserving SQLC interfaces.

### Recommendation

**modernc.org/sqlite** with WAL mode, foreign keys enabled, and `database/sql` driver. Open connection with URI pragmas:
```
file:data/ace.db?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)
```

---

## 2. Internal Messaging

### Candidates

| | Embedded NATS Server | In-Process Channels (Go chans) | Watermill | Asynq (hibiken/asynq) |
|---|---|---|---|---|
| **Type** | Full broker, in-process | Primitive | Abstraction layer | Redis-backed task queue |
| **Pub/Sub** | Yes (with wildcards) | Manual | Yes | No (task queue only) |
| **Request/Reply** | Yes | Manual | Via middleware | No |
| **JetStream (durability)** | Yes | No | Via Redis | Via Redis |
| **Subject Namespacing** | Yes (hierarchical) | Manual | Via topic | N/A |
| **Envelope/Metadata** | Built-in (headers) | Manual | Via middleware | Built-in |
| **Existing Contract Compatibility** | Direct (shared/messaging) | Rewrite | Rewrite | Rewrite |
| **CGo Required** | No | No | No | No |
| **Binary Size Impact** | ~15-20MB | 0 | ~2MB | ~5MB + Redis |

### Analysis

**Embedded NATS Server** is the correct choice. The existing `shared/messaging` package uses NATS client libraries (`nats.go`) with JetStream, subject-based routing, envelopes, and request-reply patterns. Embedding the NATS server eliminates the external daemon while preserving the entire messaging contract.

Key capabilities of embedded NATS:

1. **In-process connection**: `nats.InProcessServer(ns)` bypasses the network stack entirely. Messages travel via `net.Pipe` (in-process go channels), eliminating TCP overhead, serialization cost, and network latency.

2. **`DontListen: true`**: Configures the embedded server to skip the TCP accept loop. No ports opened, no network exposure. The server is invisible to external processes.

3. **Full feature parity**: JetStream, subject wildcards, request-reply, queues — all work identically in embedded mode. The existing `shared/messaging` package requires zero changes to its public API.

4. **Startup sequence**: The NATS server starts in-process before application services initialize. Client connections are established in-process, no reconnect logic needed.

5. **Graceful shutdown**: `ns.Shutdown()` followed by `ns.WaitForShutdown()` provides clean teardown.

6. **Enterprise option**: For deployments needing external NATS (multi-node, persistence), a configuration flag switches from embedded to remote mode. The client code is identical — only the connection URL changes.

### Recommendation

**Embedded NATS Server** with `DontListen: true` and `InProcessServer` connection. The server starts in `main()` before any service initialization. Existing `shared/messaging` contracts are preserved.

---

## 3. Internal Caching

### Candidates

| | Ristretto (dgraph-io/ristretto) | BigCache (allegro/bigcache) | FreeCache (coocood/freecache) | Otter (maypok86/otter) | In-Process sync.Map |
|---|---|---|---|---|---|
| **TTL Support** | Per-item cost-based | Global LifeWindow | Per-item | Per-item | Manual |
| **Eviction Policy** | TinyLFU | Time-based | LRU | S3-FIFO | None |
| **Memory Limit** | Yes (MaxCost) | Yes (HardMaxSize) | Yes (size in bytes) | Yes (capacity) | No |
| **GC Pressure** | Low (probabilistic) | Low (byte slices) | Low (zero-GC design) | Low | High |
| **Tag Invalidation** | No | No | No | No | No |
| **Stampede Protection** | No | No | No | No | No |
| **Throughput** | ~800K ops/sec | ~2M ops/sec | ~1M ops/sec | ~2M+ ops/sec | ~3M ops/sec |
| **Maturity** | High (production dgraph) | High (production Allegro) | Medium | New (2024) | Stdlib |

### Analysis

The current `shared/caching` package provides:

- Tag-based invalidation (`DeleteByTag`)
- Version-based invalidation (`InvalidateByVersion`)
- Cache-aside with stampede protection (`GetOrFetch`)
- Namespace + agentID key scoping
- Stats (hit/miss rates)
- Valkey backend

None of the standalone Go cache libraries provide **tag-based invalidation** or **stampede protection** out of the box. The current Valkey backend implements these features at the application layer (in `shared/caching`). This means we need to port the application logic, not just swap backends.

**Ristretto** is the best foundation because:

1. **Cost-based admission**: Ristretto uses TinyLFU admission policy which provides high hit rates with memory bounds. This is superior to simple LRU for ACE's workload where cache entries vary in size (agent memory vs. configuration data).

2. **Low GC pressure**: Ristretto stores values as `interface{}` but uses probabilistic admission that limits the working set. Combined with the existing key-scoping scheme (namespace:agentID:entityType:entityID), the GC impact is manageable.

3. **MaxCost memory limit**: Sets a hard ceiling on memory usage. For a single binary with a target of <200MB total memory, we can allocate ~50MB to the cache and let Ristretto enforce it.

4. **The missing features** (tag invalidation, stampede protection, versioned keys) already exist in `shared/caching` as application-level logic. We port these to work on top of Ristretto the same way they currently work on top of Valkey.

5. **Enterprise fallback**: The `shared/caching` package already uses an interface (`Backend`) with `ValkeyBackend` as the implementation. We add an `InProcessBackend` implementing the same interface. Configuration switches between them.

### Recommendation

**Ristretto** as the in-process backend, wrapped in a new `InProcessBackend` that implements the existing `Backend` interface from `shared/caching`. Tag invalidation, version management, and stampede protection remain application-level concerns ported from the current Valkey implementation.

---

## 4. CLI Installation Standards

### Industry Patterns (curl | sh)

| Project | Install Command | Binary Location | Checksum | Auto-Update |
|---|---|---|---|---|
| Rust (rustup) | `curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \| sh` | `~/.cargo/bin` | Yes (SHA256) | Yes (rustup update) |
| Deno | `curl -fsSL https://deno.land/install.sh \| sh` | `~/.deno/bin` | Yes (SHA256) | Yes (deno upgrade) |
| GoReleaser | `curl -sSfL https://raw.githubusercontent.com/goreleaser/goreleaser/main/install.sh \| sh` | `./bin` (cwd) or `-b` flag | Yes (checksums.txt) | No |
| Helm | `curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 \| bash` | `/usr/local/bin` | Yes (SHA256) | No |
| Anchore/Syft | `curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh \| sh -s -- -b $HOME/.local/bin` | Configurable | Yes (checksums.txt) | No |

### Best Practice Analysis

**Security best practices** ([bettercli.org](https://bettercli.org)):

1. **Checksum verification**: Download `checksums.txt` from the GitHub release and verify the binary SHA256 before installing. Never execute an unverified binary.
2. **HTTPS with TLS**: Use `curl -fsSL` with `--proto '=https'` to enforce TLS 1.2+.
3. **User-level install by default**: Install to `$HOME/.local/bin` (XDG standard), not `/usr/local/bin` (requires root). Provide `-b` flag for custom location.
4. **PATH guidance**: Detect if install dir is in `$PATH`. If not, print the shell-appropriate `export` command.
5. **Versioned releases**: Download from `https://github.com/{org}/{repo}/releases/download/{tag}/{binary}` with detected OS/arch.
6. **Standalone verification script**: Provide a separate `verify.sh` for users who distrust pipe-to-shell.

**Install directory convention**:

- **Default**: `$HOME/.local/bin` (XDG standard for user executables)
- **System-wide**: `/usr/local/bin` (with `sudo`)
- **Custom**: `-b` flag to override

### Recommendation

Create `scripts/install.sh` following the Anchore/Syft pattern:
- Detect OS (linux/darwin) and arch (amd64/arm64)
- Fetch latest version from GitHub releases API
- Download binary + checksums.txt
- Verify SHA256 checksum
- Install to `$HOME/.local/bin` by default (or `-b $DIR`)
- Print PATH instructions if directory not in `$PATH`
- Provide `scripts/verify.sh` for standalone checksum verification

---

## 5. Data Directory Standards

### XDG Base Directory Specification

The [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir/latest/) defines standard locations for application data:

| Purpose | XDG Variable | Linux Default | macOS Default |
|---|---|---|---|
| Config | `$XDG_CONFIG_HOME` | `~/.config` | `~/Library/Application Support` |
| Data | `$XDG_DATA_HOME` | `~/.local/share` | `~/Library/Application Support` |
| Cache | `$XDG_CACHE_HOME` | `~/.cache` | `~/Library/Caches` |
| State | `$XDG_STATE_HOME` | `~/.local/state` | `~/Library/Application Support` |
| Runtime | `$XDG_RUNTIME_DIR` | `/run/user/{uid}` | `$TMPDIR` |
| Bin | — | `~/.local/bin` | `~/.local/bin` |

### Go XDG Libraries

| Library | Stars | Approach | App-Scoped | macOS Support |
|---|---|---|---|---|
| `adrg/xdg` | 970+ | Global singleton | Manual (prepend app name) | Yes |
| `muesli/go-app-paths` | 100+ | Scoped per app name | Yes (built-in) | Yes |
| `tzrikka/xdg` | New | Simple, XDG-first | Manual | Yes |

### ACE Data Directory Layout

ACE needs a single data root that contains the database, messaging state, logs, and cache. Using XDG conventions:

```
$XDG_DATA_HOME/ace/          # Data root (default: ~/.local/share/ace/)
├── ace.db                    # SQLite database
├── ace.db-wal                # SQLite WAL file
├── ace.db-shm                # SQLite shared memory
├── nats/                     # JetStream storage directory
├── logs/                     # Application logs
│   └── ace.log
├── config.yaml               # User configuration override
└── telemetry/                 # Local telemetry data
    └── metrics.db             # SQLite metrics store
```

The configuration file can also be found at `$XDG_CONFIG_HOME/ace/config.yaml` (default: `~/.config/ace/config.yaml`), allowing config/data separation.

### Command-Line Override

The `ace` command accepts `--data-dir` flag that overrides the default. Priority:
1. `--data-dir` flag (highest)
2. `$ACE_DATA_DIR` environment variable
3. `$XDG_DATA_HOME/ace/` (XDG default)
4. `~/.local/share/ace/` (fallback)

### Recommendation

Use `adrg/xdg` for path resolution (mature, well-maintained, respects XDG env vars). Create directories on first run with `0700` permissions. Document all paths in help output (`ace paths`).

---

## 6. Frontend Embedding

### Approaches

| | adapter-static + go:embed | adapter-node (keep separate) | adapter-static + custom handler |
|---|---|---|---|
| **Build Output** | Static HTML/JS/CSS | Node.js server | Static HTML/JS/CSS |
| **Binary Embedding** | `//go:embed all:build` | N/A | `//go:embed all:build` |
| **Binary Size Impact** | +5-15MB (gzipped) | 0 | +5-15MB (gzipped) |
| **SPA Routing** | Custom handler (fallback to index.html) | Node.js handles | Custom handler |
| **Dev Mode** | Vite dev server (separate) | Same as prod | Vite dev server (separate) |
| **API Proxy** | Go server serves both | Separate processes | Go server serves both |
| **HMR in Dev** | Vite dev server (no embed) | Same | Vite dev server (no embed) |

### SPA Embedding Pattern

The proven pattern for embedding a SvelteKit SPA in a Go binary:

1. **SvelteKit config**: Switch from `adapter-node` to `adapter-static` with `fallback: 'index.html'` and `prerender.entries` set to generate all static pages.

2. **Build**: `npm run build` produces a `build/` directory with all static assets.

3. **Go embed**: A `frontend.go` file uses `//go:embed all:build` to embed the entire build output into the binary.

4. **Handler**: A custom `http.Handler` serves embedded assets:
   - Requests with file extensions (`.js`, `.css`, `.svg`, etc.) → serve from embedded FS
   - Requests starting with `/_app/` → serve from embedded FS (Vite assets)
   - All other requests → serve `index.html` (SPA fallback for client-side routing)

5. **Dev mode**: Build tag `//go:build !dev` for embedded assets, `//go:build dev` for proxying to `vite dev` server.

6. **API routing**: The Go server handles `/api/*` routes, then falls through to the SPA handler for everything else.

### SvelteKit Configuration Changes

```javascript
// svelte.config.js - current
import adapter from '@sveltejs/adapter-node';

// svelte.config.js - new
import adapter from '@sveltejs/adapter-static';
// ...
adapter: adapter({
  pages: 'build',
  assets: 'build',
  fallback: 'index.html',
  precompress: true,
})
```

### Recommendation

**adapter-static + go:embed** with build-tag-based dev/prod switching. In production, all frontend assets are embedded in the binary. In development, requests proxy to `vite dev` server for HMR. A custom Chi middleware handles SPA routing fallback.

---

## 7. Custom Telemetry

### Current Stack (to be replaced)

| Component | Purpose | Resource Cost |
|---|---|---|
| Prometheus | Metrics scraping & storage | ~100MB RAM |
| Loki | Log aggregation | ~200MB RAM |
| Tempo | Distributed tracing | ~100MB RAM |
| Grafana | Dashboards & visualization | ~50MB RAM |
| OTEL Collector | Telemetry pipeline | ~100MB RAM |
| **Total** | | **~550MB RAM** |

### Approach: Embedded Observability

The goal is to replace the entire external observability stack with a lightweight, embedded solution. The key principle: **telemetry data stays in-process** and is queryable via API endpoints.

### Architecture

```
shared/telemetry (existing package)
├── Logger (zap)              — stays, now structured to local file + stdout
├── Tracer (OTel)             — stays, exports to local SQLite spans table
├── Meter (OTel)              — stays, exports to local SQLite metrics table
├── Usage (existing)          — stays, writes to SQLite usage_events table
└── Inspector (new)           — HTTP handlers for querying telemetry data
    ├── GET /telemetry/spans      — trace inspection
    ├── GET /telemetry/metrics    — metric introspection
    ├── GET /telemetry/usage      — cost attribution
    └── GET /telemetry/health     — system health
```

### Key Design Decisions

1. **SQLite for telemetry storage**: Use the same embedded SQLite instance with separate tables for spans, metrics, and usage events. Retention via SQLite TTL (delete old rows periodically).

2. **OTel SDK remains**: The `shared/telemetry` package keeps the OpenTelemetry SDK for instrumentation. Instead of exporting to an external collector, a custom `SpanExporter` and `MetricReader` write to SQLite tables.

3. **Structured logging**: Zap logs go to both stdout (JSON) and a local log file (`$XDG_STATE_HOME/ace/logs/ace.log`). Log rotation via `lumberjack` or `natefinch/lumberjack`.

4. **Telemetry API endpoints**: A new Inspector component exposes HTTP endpoints on the same port as the API. These endpoints provide the observability data that Grafana dashboards previously visualized, but through simple JSON API responses. These are product-facing endpoints that drive user features.

5. **Enterprise export**: Configuration flag to re-enable OTLP export to external collectors. In embedded mode, telemetry stays local. In enterprise mode, it can forward to Prometheus/Loki/Tempo.

6. **Memory budget**: The telemetry subsystem should use <10MB RAM. SQLite handles the storage, OTel SDK handles in-flight buffering, and periodic flushes keep memory bounded.

### Cost Attribution Preservation

The existing `UsageEvent` system (LLM calls, memory reads, tool executions) is preserved exactly. Events are written to the `usage_events` table in SQLite instead of published to NATS. The Inspector exposes query endpoints for cost dashboards.

### Recommendation

Replace the entire external observability stack with embedded SQLite-backed OTel SDK exporters + telemetry HTTP endpoints. Preserve `shared/telemetry` interfaces. Add `Inspector` component for product-facing observability queries.

---

## 8. Build System Consolidation

### Current State (Multi-Module Workspace)

```
backend/
├── go.work                     # Workspace root
├── go.work.sum
├── services/
│   └── api/
│       ├── go.mod   (ace/api)
│       └── cmd/main.go
├── shared/
│   ├── go.mod       (ace/shared)
│   ├── caching/
│   ├── messaging/   (separate go.mod: ace/shared/messaging)
│   └── telemetry/   (separate go.mod: ace/shared/telemetry)
└── scripts/
    └── docs-gen/
        └── go.mod    (scripts/docs-gen)
```

### Problem

Multi-module workspaces add complexity for a single binary target:
- `go work sync` is a development convenience that shouldn't exist in production
- Each module has its own `go.mod`/`go.sum` with duplicated dependency tracking
- Cross-module changes require `go work sync` to propagate
- The `go.work` file is typically not committed, causing CI divergence

### Options

| | Single Module (flat) | Single Module (internal/) | Keep Workspace |
|---|---|---|---|
| **Import Path** | `ace/...` flat | `ace/internal/...` encapsulated | `ace/api`, `ace/shared/...` |
| **Visibility** | All packages public | `internal/` enforces encapsulation | Public per module |
| **Binary Count** | 1 (`cmd/ace/main.go`) | 1 (`cmd/ace/main.go`) | 1 (but complex) |
| **Dependency Mgmt** | Single `go.mod` | Single `go.mod` | 4+ separate `go.mod` |
| **Build Simplicity** | `go build ./cmd/ace` | `go build ./cmd/ace` | `go work sync && go build` |
| **refactoring** | Easy (same module) | Easy (same module) | Complex (multi-module) |

### Recommendation

**Consolidate to a single Go module** with `internal/` packages. The workspace was necessary when there were multiple deployable services. With a single binary, it creates unnecessary friction.

```
backend/
├── go.mod                  # Single module: module ace
├── go.sum
├── cmd/
│   └── ace/
│       └── main.go         # Single entry point
├── internal/
│   ├── api/                # HTTP handler layer
│   │   ├── handler/
│   │   ├── middleware/
│   │   ├── router/
│   │   └── model/
│   ├── service/            # Business logic layer
│   ├── repository/         # Data access layer (SQLC generated)
│   ├── messaging/           # In-process messaging (was shared/messaging)
│   ├── caching/            # In-process caching (was shared/caching)
│   ├── telemetry/          # Observability (was shared/telemetry)
│   ├── config/             # Configuration
│   └── embedding/          # Frontend asset serving
├── migrations/              # Goose migrations
├── sql/                     # SQLC query files
└── frontend/                # SvelteKit source (build output embedded)
    └── build/               # Static assets for go:embed
```

Key changes:
- `ace/shared/messaging` becomes `ace/internal/messaging` — same code, simpler imports
- `ace/shared/telemetry` becomes `ace/internal/telemetry`
- `ace/shared/caching` becomes `ace/internal/caching`
- `ace/api` handler/service/repository move to `ace/internal/api/...`
- `scripts/docs-gen` becomes a `Makefile` target or standalone tool, not a workspace module

---

## 9. Test Optimization

### Current Problem

The pre-commit hook runs 9 sequential quality gates (build, lint, test, SQLC generate, docs validation, frontend lint, frontend test, compose validate, Makefile validate). Integration tests require running containers. Total time exceeds 60 seconds.

### Optimization Strategies

| Strategy | Approach | Estimated Impact |
|---|---|---|
| **Test Categorization** | `//go:build unit` vs `//go:build integration` tags | Hook uses `-short` flag, CI runs everything |
| **Parallel Execution** | `go test -parallel=NumCPU ./...` within packages | 2-4x speedup on multi-core |
| **Package-Level Parallelism** | `go test ./...` runs packages in parallel by default | Already happens |
| **Short Flag** | `go test -short` skips long-running tests | Fast filter for hook while testing everything |
| **Test Caching** | `go test -count=1` disables; remove flag to enable | Free speedup on re-runs |
| **Build Tag Separation** | Separate unit from integration in build tags | Clean separation, hook uses `-short` |

### Recommended Test Categorization

```go
// Unit test (runs in git hook)
//go:build unit

// Integration test (requires containers/external deps)
//go:build integration

// No tag = always runs (default unit tests)
```

**Makefile targets**:

| Target | Commands | When |
|---|---|---|
| `make test` | Full pipeline: build, lint, test (all), docs-gen, fe-lint, fe-test | Git hook and CI |

`make test` is the single command that validates everything. It uses `go test -short` for the hook to skip long-running tests while still testing all packages. CI runs the same command but may set additional flags.

### Recommendation

1. `make test` tests everything - all Go tests (with `-short` for speed), all linting, all frontend tests, and doc generation.
2. Git hook simply calls `make test`.
3. Enable test caching for faster re-runs (remove `-count=1`).
4. Target: hook completes in <30 seconds by optimizing the pipeline, not by skipping tests.

---

## 10. Git Hook Efficiency

### Current Pre-Commit Hook

The hook runs 9 sequential gates. Analysis of time cost:

| Gate | Current Time | Notes |
|---|---|---|
| 1. Go Build | ~5s | Always needed |
| 2. Go Lint (fmt + vet) | ~3s | Always needed |
| 3. Go Test | ~30-60s | Uses `-short` flag to skip long tests |
| 4. SQLC Generate | ~3s | Always needed |
| 5. Docs Validation | ~5s | Requires DB (now embedded, no containers) |
| 6. Frontend Lint | ~5s | Always needed |
| 7. Frontend Test | ~5s | Always needed |
| 8. Docker Compose Validate | ~2s | **REMOVED** - devops/ folder deleted |
| 9. Makefile Validate | ~1s | Always needed |

**Total current: ~59-89 seconds**

### New Hook Design

The hook delegates entirely to `make test`:

```bash
#!/bin/bash
make test
```

`make test` handles everything:
- Go build and lint
- Go tests (all packages, with `-short` flag for speed)
- SQLC generate
- Docs generation (now works with embedded SQLite)
- Frontend lint and test
- Makefile validation

### Optimization Strategies

1. **Parallel execution**: Run Go and Frontend pipelines in parallel within `make test`
2. **Test caching**: Enable Go test caching (remove `-count=1`)
3. **Embedded database**: Docs generation no longer requires external PostgreSQL
4. **Remove Docker dependency**: No containers needed for tests

### devops/ Folder Deletion

The entire `devops/` folder (Docker Compose files, container configs) will be deleted as part of this unit. The application will use:
- Embedded SQLite instead of PostgreSQL container
- Embedded NATS instead of NATS container  
- In-process caching instead of Valkey/Redis
- Custom telemetry instead of Prometheus/Grafana/Loki/Tempo/OTEL containers

**Target total: <30 seconds** (achieved via parallelization and removing container overhead)

### Recommendation

1. Git hook calls `make test` exclusively
2. `make test` is the single source of truth for validation
3. Delete `devops/` folder entirely
4. Remove all Docker/Podman dependencies from the development workflow

---

## Summary of Recommendations

| Area | Recommendation | Key Benefit |
|---|---|---|
| **Embedded Database** | modernc.org/sqlite (pure Go, WAL mode) | Single file DB, SQLC compatible, no CGo |
| **Internal Messaging** | Embedded NATS (DontListen + InProcessServer) | Preserves `shared/messaging` contract, zero network overhead |
| **Internal Caching** | Ristretto as InProcessBackend (implementing `Backend` interface) | Preserves `shared/caching` contract, bounded memory |
| **CLI Installation** | `curl \| sh` script with checksums, XDG paths, GitHub releases | One-command install, verified binaries |
| **Data Directory** | XDG spec via `adrg/xdg`, `$XDG_DATA_HOME/ace/` as root | Platform-native, discoverable, documented |
| **Frontend Embedding** | adapter-static + go:embed + build-tag dev switch | Single binary, dev HMR preserved |
| **Custom Telemetry** | SQLite-backed OTel exporters + product-facing HTTP endpoints | Replaces 5 containers, <10MB RAM, same OTel SDK |
| **Build Consolidation** | Single Go module with `internal/` packages | Single `go.mod`, simpler imports, single binary build |
| **Test Optimization** | `//go:build unit` / `//go:build integration` tags, `make test` tests everything | Single command for full validation |
| **Git Hook Efficiency** | Hook calls `make test`, parallel execution, no Docker | <30s vs current >60s, no containers |

### Risk Mitigations

| Risk | Mitigation |
|---|---|
| SQLite single-writer bottleneck | WAL mode + busy_timeout; enterprise flag for PostgreSQL |
| Loss of Grafana dashboards | Debug HTTP endpoints (`/debug/`); lightweight JSON API |
| Binary size >200MB target | Compress frontend assets; measure early; strip symbols |
| NATS feature regression | Identical `shared/messaging` API; integration tests cover parity |
| Frontend SPA routing edge cases | Comprehensive routing middleware with fallback logic |
| Migration complexity | Incremental slices; each layer swapped independently |