# Implementation Plan: Deployment & Developer Experience

**Unit:** deployment-dx
**Date:** 2026-04-12
**Status:** Design

---

## Vertical Slice Strategy

Each slice is a complete, independently testable vertical that cuts through multiple layers. Slices are ordered by risk (highest risk first) and dependency graph. Every slice produces a testable artifact — either a passing test suite or a runnable binary capability.

**Principles:**
- Each slice is one PR
- Each slice touches DB → Service → Handler → (sometimes) UI
- Each slice must compile and pass `make test` independently
- Later slices depend on earlier slices but never vice versa
- Deletions (devops/, go.work, etc.) happen in a dedicated cleanup slice at the end

---

## Dependency Graph

```
Slice 1 (Module consolidation)
  └→ Slice 2 (XDG paths + CLI)
       └→ Slice 3 (Embedded database)
            └→ Slice 4 (Embedded NATS)
                 └→ Slice 5 (In-process cache)
                      └→ Slice 6 (Custom telemetry)
                           └→ Slice 8 (App lifecycle + server)
                                └→ Slice 9 (Frontend embedding)
                                     └→ Slice 10 (Telemetry Inspector API)
                                          └→ Slice 11 (Build + Makefile)
                                               └→ Slice 12 (Install + verify scripts)
                                                    └→ Slice 13 (Cleanup: delete devops/, go.work, changelogs/)
```

Slices 3-6 are independent of each other after Slice 2, but sequencing them avoids config wiring conflicts. Slice 7 (git hook) is independent after Slice 11.

---

## Slice Definitions

### Slice 1: Module Consolidation — Single `go.mod`

**Risk:** High — everything depends on import paths

**Backend:**
- Create `backend/go.mod` with `module ace`
- Merge all dependency `go.mod` files (shared, messaging, telemetry, api, docs-gen)
- Move `backend/shared/` → `backend/internal/` (caching, messaging, telemetry)
- Move `backend/services/api/internal/` → `backend/internal/api/`
- Move `backend/services/api/cmd/main.go` → `backend/cmd/ace/main.go` (stub: just prints "ace starting" and exits)
- Move `backend/services/api/migrations/` → `backend/migrations/`
- Move `backend/services/api/sqlc.yaml` → `backend/sqlc.yaml`
- Update all import paths from `ace/shared/*` → `ace/internal/*`, `ace/api/internal/*` → `ace/internal/api/*`
- Run `go mod tidy && go mod vendor`
- Delete `backend/go.work`, `backend/go.work.sum`, per-module `go.mod`/`go.sum` files

**Frontend:** None

**Test:** `go build ./...` compiles. All existing tests pass with new import paths. `make test` green.

**Definition of Done:** Single `backend/go.mod`, all imports resolved, binary builds, tests pass.

---

### Slice 2: XDG Paths, Config Resolution, CLI Entry Point

**Risk:** Medium — foundation for all subsequent slices

**Backend:**
- Create `internal/platform/paths.go`: `ResolvePaths()`, `Paths` struct, `EnsureDirs()`, `PrintPaths()`
- Create `internal/app/config.go`: `Config` struct with all fields from FSD §1.2–1.3, `ResolveConfig()` implementing priority (CLI > env > config.yaml > XDG defaults)
- Create `internal/app/app.go`: `App` struct with `New()`, `Serve()`, `Shutdown()` stubs (no subsystem wiring yet — just config + paths + HTTP server)
- Update `cmd/ace/main.go`: parse flags, call `app.New(cfg)`, call `app.Serve()`, handle SIGINT/SIGTERM
- Add `adrg/xdg` and `golang.org/x/term` to go.mod
- Add `ace paths`, `ace version`, `ace migrate` (stub), `ace help` subcommands
- Create `$HOME/.local/share/ace/` on first run with 0700 permissions

**Frontend:** None

**Test:**
- Unit: `TestResolvePaths_Defaults`, `TestResolvePaths_EnvOverride`, `TestResolvePaths_CLIFlag`, `TestEnsureDirs_CreatesTree`
- Unit: `TestResolveConfig_Defaults`, `TestResolveConfig_EnvOverrides`, `TestResolveConfig_CLIOverridesEnv`, `TestResolveConfig_Validation_Errors`
- Integration: `TestApp_New_CreatesDataDirs`, `TestApp_Paths_Command`
- Manual: `go run ./cmd/ace paths` prints correct XDG paths; `go run ./cmd/ace --data-dir=/tmp/ace-test paths` shows override

**Definition of Done:** `ace paths` prints resolved paths. `ace --help` shows all flags. Config resolution has full test coverage. Data directories created with correct permissions.

---

### Slice 3: Embedded Database (SQLite)

**Risk:** High — data layer is foundational

**Backend:**
- Add `modernc.org/sqlite` to go.mod
- Add `github.com/lib/pq` to go.mod (for future external mode)
- Create `internal/platform/database/database.go`: `Open()`, `Migrate()`, `Config` struct
- Create `internal/platform/database/database_embed.go` (default build): SQLite driver, WAL pragmas, `file:{datadir}/ace.db?_pragma=...`
- Create `internal/platform/database/database_ext.go` (`//go:build external`): PostgreSQL driver
- Create `backend/migrations/` with SQLite-adapted migration files from existing PostgreSQL migrations
  - Convert `SERIAL`/`BIGSERIAL` → `INTEGER PRIMARY KEY AUTOINCREMENT`
  - Convert `BOOLEAN` → `INTEGER`
  - Convert `TIMESTAMPTZ` → `TEXT`
  - Convert `gen_random_uuid()` → Go-generated UUIDs
- Add new migration: `20260412000000_create_telemetry.sql` (ott_spans, ott_metrics)
- Add new migration: `20260412000001_create_usage_events.sql`
- Update `sqlc.yaml` engine to `sqlite`
- Update all SQL query files in `internal/repository/queries/` for SQLite dialect
- Regenerate SQLC code
- Wire `database.Open(cfg)` into `app.New()`
- `ace migrate` subcommand now actually runs migrations

**Frontend:** None

**Test:**
- Unit: `TestOpen_EmbeddedDB_CreatesFile`, `TestOpen_EmbeddedDB_WALMode`, `TestOpen_EmbeddedDB_ForeignKeys`
- Unit: `TestMigrate_RunsAllMigrations`, `TestMigrate_Idempotent`
- Unit: `TestSQLite_Dialect_Adaptations` (test that existing queries work with SQLite)
- Integration: `TestApp_New_OpensDatabase`, `TestApp_Migrate_Command`

**Definition of Done:** `ace` starts, creates `ace.db` with WAL, applies all migrations. Existing repository tests pass against SQLite in-memory. `ace migrate` applies migrations and reports count.

---

### Slice 4: Embedded NATS Messaging

**Risk:** Medium — NATS embedding is well-documented

**Backend:**
- Add `github.com/nats-io/nats-server/v2` and `github.com/nats-io/nats.go` to go.mod
- Create `internal/platform/messaging/messaging.go`: `Init()`, `Config`, cleanup function
- Create `internal/platform/messaging/server_embed.go` (default build): `startEmbeddedNATS()` with `DontListen: true`, JetStream enabled, `StoreDir` from `paths.NATSPath`
- Create `internal/platform/messaging/server_ext.go` (`//go:build external`): connect to remote URL
- Wire `messaging.Init(cfg)` into `app.New()`
- Wire cleanup (drain client → shutdown server) into `app.Shutdown()`
- Ensure existing `internal/messaging/Client` interface works with `InProcessServer` connection

**Frontend:** None

**Test:**
- Integration: `TestInit_Embedded_StartsAndStops`, `TestInit_Embedded_PublishSubscribe`, `TestInit_Embedded_JetStream`
- Unit: `TestMessagingConfig_Defaults`
- Test that existing messaging patterns (publish, subscribe, request-reply) work over InProcessServer

**Definition of Done:** App starts with embedded NATS, no TCP ports opened. Existing messaging tests pass over in-process connection. Graceful shutdown drains and stops server cleanly.

---

### Slice 5: In-Process Cache (Ristretto)

**Risk:** Medium — new backend must match CacheBackend interface

**Backend:**
- Add `github.com/dgraph-io/ristretto` to go.mod
- Create `internal/platform/cache/cache.go`: `Init()`, `Config`
- Create `internal/platform/cache/inprocess.go`: `InProcessBackend` implementing `caching.CacheBackend`
  - `Get`, `Set`, `Delete`, `GetMany`, `SetMany`, `DeleteMany` → Ristretto calls
  - `DeleteByTag` → tag index lookup + Ristretto deletes
  - `DeletePattern` → iterate + glob match
  - `SAdd`, `SMembers`, `SRem` → tag index operations
  - `Exists`, `TTL` → Ristretto lookup with metadata
  - `Close` → Ristretto close
- Create `internal/platform/cache/valkey.go` (`//go:build external`): existing ValkeyBackend
- Wire `cache.Init(cfg)` into `app.New()`
- Update `internal/caching/constructors.go`: add `NewInProcessBackend()` option

**Frontend:** None

**Test:**
- Unit: `TestInProcessBackend_SetGet`, `TestInProcessBackend_DeleteByTag`, `TestInProcessBackend_DeletePattern`, `TestInProcessBackend_TagIndex`
- Unit: `TestInProcessBackend_MaxCost_Eviction`
- Integration: `TestCache_Init_Embedded_Ristretto`
- Existing `cache_test.go` and `cache_integration_test.go` pass with InProcessBackend

**Definition of Done:** InProcessBackend passes all CacheBackend interface tests. Tag invalidation works. Memory budget enforced. App starts with Ristretto cache, no Valkey dependency.

---

### Slice 6: Custom Telemetry (SQLite Exporters)

**Risk:** Medium — OTel SDK customization required

**Backend:**
- Add `go.opentelemetry.io/otel` SDK dependencies to go.mod
- Add `go.uber.org/zap` and `gopkg.in/natefinch/lumberjack.v2` to go.mod
- Create `internal/platform/telemetry/telemetry.go`: `Init()`, `Telemetry` struct, `Config`
- Create `internal/platform/telemetry/sqlite_exporter.go`: `SpanExporter` implementation writing to `ott_spans` table
- Create `internal/platform/telemetry/sqlite_reader.go`: `MetricReader` implementation writing to `ott_metrics` table
- Create `internal/platform/telemetry/sqlite_queries.sql`: SQLC queries for telemetry tables
- Create `internal/platform/telemetry/logger.go`: dual-output logger (stdout JSON + file via lumberjack)
- Update `internal/telemetry/` to use Init results instead of hardcoded OTLP
- Update `internal/telemetry/usage.go`: direct DB write instead of NATS publish (in embedded mode)
- Add telemetry pruning goroutine (7d spans, 24h metrics, 90d usage events)
- Wire `telemetry.Init(cfg)` into `app.New()`
- Create `internal/platform/telemetry/server_ext.go` (`//go:build external`): OTLP exporters

**Frontend:** None

**Test:**
- Unit: `TestSpanExporter_WritesToDB`, `TestMetricReader_WritesToDB`
- Unit: `TestLogger_DualOutput_StdoutAndFile`
- Unit: `TestPruning_DeletesExpiredSpans`, `TestPruning_DeletesExpiredMetrics`
- Integration: `TestTelemetry_Init_Embedded`, `TestTelemetry_Init_External_Stub`

**Definition of Done:** OTel traces and metrics write to SQLite. Log file created with rotation. Usage events write directly to DB. Pruning runs on schedule. App starts with embedded telemetry, zero external collectors.

---

### Slice 7: Git Hook & Test Optimization

**Risk:** Low — Makefile refactoring

**Backend:**
- Update `Makefile` root targets: `ace`, `ui`, `test`
- `make ace`: `go build -o bin/ace ./cmd/ace/`
- `make ui`: `cd frontend && npm run build`
- `make test`: sequential pipeline (go build → go vet → go test -short → sqlc generate → fe lint → fe test → git add)
- Remove `-count=1` from test flags (enable caching)
- Delete `.pre-commit-config` references to Docker/Podman
- Update pre-commit hook to call `make test` only

**Frontend:** None

**Test:**
- Manual: `make test` completes in <30s (after in-memory SQLite, no containers)
- Manual: `make ace` produces `bin/ace` binary
- Manual: Pre-commit hook runs `make test`

**Definition of Done:** `make test` is the single validation command. No Docker/Podman required. Git hook is `make test`. Target <30s execution.

---

### Slice 8: App Lifecycle Wiring + HTTP Server

**Risk:** High — integration point for all subsystems

**Backend:**
- Complete `internal/app/app.go`: wire all subsystems (database → NATS → cache → telemetry → services → router → server)
- Complete `cmd/ace/main.go`: full startup sequence with rollback on failure
- Implement graceful shutdown in `App.Shutdown()`: HTTP drain → telemetry flush → NATS drain → NATS server shutdown → cache close → database close
- Update `internal/api/router/router.go`: add SPA handler parameter, add `/telemetry/*` route group (stubs for now)
- Create `internal/platform/frontend/frontend.go`: default dev mode (returns "frontend not available in dev mode" placeholder)
- Structured startup/shutdown logging per FSD §8
- Implement `/health/live` and `/health/ready` endpoints checking all subsystems
- Wire configuration validation (FSD §2.4)

**Frontend:** None

**Test:**
- Integration: `TestApp_FullStartup_AllEmbedded`, `TestApp_FullStartup_ShutdownSequence`
- Integration: `TestApp_StartupFailure_RollsBackInitialized` (test that failed NATS init rolls back database)
- Integration: `TestHealthEndpoints_ReturnsOK`
- Manual: `go run ./cmd/ace` starts all subsystems, logs startup sequence, serves on :8080, responds to SIGINT

**Definition of Done:** `ace` binary starts, initializes all 4 embedded subsystems, serves HTTP on configured port, responds to `/health/live` and `/health/ready`, shuts down gracefully on signal.

---

### Slice 9: Frontend Embedding + SPA Handler

**Risk:** Medium — SPA routing and build tag handling

**Backend:**
- Create `internal/platform/frontend/spa.go`: `SPAHandler(fs.FS)` with routing rules from FSD §3.5
- Create `internal/platform/frontend/frontend_embed.go` (`//go:build embed`): `//go:embed all:../../../frontend/build` + `Handler()` returning SPA handler
- Create `internal/platform/frontend/frontend_dev.go` (default build): `Handler()` reverse-proxying to `http://localhost:5173`
- Wire SPA handler into router (`route/* → SPAHandler`)
- Configure build tags in Makefile and GoReleaser

**Frontend:**
- Update `svelte.config.js`: switch from `adapter-node` to `adapter-static` with `fallback: 'index.html'`
- Add `@sveltejs/adapter-static` to `package.json` dependencies
- Verify `npm run build` produces `frontend/build/` with static assets

**Test:**
- Unit: `TestSPAHandler_AssetRequest_ServesFile`, `TestSPAHandler_NonAssetRequest_ServesIndexHTML`, `TestSPAHandler_ViteInternal_ServesFile`
- Integration: `TestApp_EmbedMode_ServesFrontend`, `TestApp_DevMode_ProxiesToVite`
- Manual: `make ace` (without embed tag) → starts with dev proxy. `go build -tags embed ./cmd/ace/` → starts serving embedded assets at :8080

**Definition of Done:** Production binary serves SPA from embedded FS. Dev binary proxies to Vite dev server. SPA routing works (direct URL access returns index.html). Asset caching headers set correctly.

---

### Slice 10: Telemetry Inspector API Endpoints

**Risk:** Low — standard CRUD endpoints

**Backend:**
- Create `internal/api/handler/telemetry_handler.go`: `TelemetryHandler` struct with `Spans()`, `Metrics()`, `Usage()`, `Health()` methods
- Create `internal/repository/queries/telemetry_spans.sql`, `telemetry_metrics.sql`, `usage_events.sql`: SQLC query definitions
- Regenerate SQLC code
- Wire `/telemetry/*` routes in router with JWT auth middleware
- Implement query parameter validation per FSD §3.3
- Implement `/telemetry/health` returning subsystem status (database, NATS, cache)

**Frontend:** None (future UI slice)

**Test:**
- Unit: `TestSpansHandler_Pagination`, `TestSpansHandler_TimeRange`, `TestMetricsHandler_Window`, `TestUsageHandler_Filters`
- Unit: `TestHealthHandler_AllHealthy`, `TestHealthHandler_Degraded`
- Integration: `TestTelemetryRoutes_RequireAuth`

**Definition of Done:** All 4 Inspector endpoints return correct JSON payloads. Authentication enforced. Pagination and filtering work. Health endpoint reflects real subsystem status.

---

### Slice 11: Build System — Makefile, GoReleaser, SQLC

**Risk:** Low — build tooling

**Backend:**
- Finalize `Makefile`: `ace`, `ui`, `test`, `generate`, `migrate` targets
- Create `.goreleaser.yml` per FSD §7.2: multi-arch builds (linux/darwin, amd64/arm64), embed tag, ldflags for version/commit/date
- Update `sqlc.yaml` in `backend/`: engine `sqlite`, correct paths for new directory structure
- Verify `sqlc generate` works with SQLite dialect
- Verify `go build -tags embed ./cmd/ace/` produces working binary
- Add `scripts/docs-gen/` logic to Makefile (or integrate into `internal/` if kept)

**Frontend:** Verify `npm run build` configuration is correct for `adapter-static`.

**Test:**
- Manual: `make ace` builds dev binary. `go build -tags embed ./cmd/ace/` builds production binary.
- Manual: `make test` passes full pipeline.
- Manual: Binary size check (<150MB with embedded frontend).
- Manual: `make generate` (sqlc) succeeds with SQLite engine.

**Definition of Done:** `make ace`, `make ui`, `make test` all work. GoReleaser config builds for all 4 targets. SQLC generates from SQLite queries. Production binary includes embedded frontend.

---

### Slice 12: Installation Scripts

**Risk:** Low — shell scripting

**Scripts:**
- Create `scripts/install.sh`: detect OS/arch → fetch latest GitHub release → download binary → download checksums.txt → verify SHA256 → install to `$HOME/.local/bin/ace` → PATH check → success message
- Create `scripts/verify.sh`: standalone checksum verification for binaries already downloaded
- Both scripts follow Anchore/Syft pattern: `-b` flag for custom install dir, HTTPS-only, graceful error handling

**Backend:** None (scripts are repo-level)

**Frontend:** None

**Test:**
- Manual: `curl -fsSL https://ace.dev/install.sh | sh` (or local test with `bash scripts/install.sh`)
- Manual: Verify checksum validation works (tampered binary fails)
- Manual: Verify PATH detection and guidance
- Manual: `scripts/verify.sh` validates an existing binary

**Definition of Done:** `install.sh` correctly detects OS/arch, downloads, verifies, and installs binary. `verify.sh` validates checksums. Error cases handled (unsupported OS, download failure, checksum mismatch).

---

### Slice 13: Cleanup — Delete Dead Code & Configs

**Risk:** Low — removal only

**Deletions:**
- Delete `devops/` directory (Docker Compose, container configs)
- Delete `changelogs/` directory
- Delete `backend/go.work`, `backend/go.work.sum`
- Delete per-module `go.mod`/`go.sum` (shared, messaging, telemetry, api, docs-gen)
- Delete `backend/services/` (code already moved to `internal/`)
- Delete `backend/shared/` (code already moved to `internal/`)
- Delete `backend/scripts/docs-gen/` (if converted to Makefile target)
- Delete `backend/caching.test` (test binary artifact)
- Delete `backend/tests/` (merged into `internal/` tests)
- Delete `backend/vendor/` (will regenerate from single go.mod)
- Remove Docker/Podman dependencies from all configuration
- Remove Valkey/Redis dependencies from go.mod (if no longer imported)
- Remove Prometheus/Grafana/Loki/Tempo/OTEL Collector configs
- Clean up `.env.example` → remove all external service URLs
- Run `go mod tidy && go mod vendor`

**Test:**
- `make test` passes
- `make ace` builds successfully
- No import references to deleted packages
- No references to Docker/Podman/Valkey/Prometheus in codebase

**Definition of Done:** Dead code removed. Binary builds. Tests pass. No references to deleted infrastructure.

---

## Risk Ordering Summary

| Priority | Slice | Risk | Dependency |
|----------|-------|------|------------|
| 1 | Slice 1: Module consolidation | 🔴 High | None (foundation) |
| 2 | Slice 2: Paths + Config + CLI | 🟡 Medium | Slice 1 |
| 3 | Slice 3: Embedded database | 🔴 High | Slice 2 |
| 4 | Slice 4: Embedded NATS | 🟡 Medium | Slice 2 |
| 5 | Slice 5: In-process cache | 🟡 Medium | Slice 2 |
| 6 | Slice 6: Custom telemetry | 🟡 Medium | Slice 3 (DB) |
| 7 | Slice 7: Git hook + test opt | 🟢 Low | Independent (parallel with 3-6) |
| 8 | Slice 8: App lifecycle + server | 🔴 High | Slices 3-6 |
| 9 | Slice 9: Frontend embedding | 🟡 Medium | Slice 8 |
| 10 | Slice 10: Inspector API | 🟢 Low | Slice 8 |
| 11 | Slice 11: Build system | 🟢 Low | Slice 9 |
| 12 | Slice 12: Install scripts | 🟢 Low | Slice 11 |
| 13 | Slice 13: Cleanup | 🟢 Low | All prior |

---

## Total: 13 Vertical Slices