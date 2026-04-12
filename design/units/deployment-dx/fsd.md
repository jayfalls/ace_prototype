# FSD: Functional Specification — Deployment & Developer Experience

**Unit:** deployment-dx
**Date:** 2026-04-12
**Status:** Design

---

## 1. CLI Interface

### 1.1 Command Structure

```
ace [global flags] [command]
```

Default command (no subcommand): `ace` starts the server.

### 1.2 Global Flags

| Flag | Type | Default | Env Var | Description |
|------|------|---------|---------|-------------|
| `--data-dir` | string | `$XDG_DATA_HOME/ace/` | `ACE_DATA_DIR` | Root directory for all persistent data |
| `--config` | string | `$XDG_CONFIG_HOME/ace/config.yaml` | `ACE_CONFIG` | Path to configuration file |
| `--port` | int | `8080` | `ACE_PORT` | HTTP listen port |
| `--host` | string | `0.0.0.0` | `ACE_HOST` | HTTP listen address |
| `--db-mode` | string | `embedded` | `ACE_DB_MODE` | Database driver: `embedded` (SQLite) or `external` (PostgreSQL) |
| `--db-url` | string | (auto-constructed in embedded mode) | `ACE_DB_URL` | Database connection URL (required when `--db-mode=external`) |
| `--nats-mode` | string | `embedded` | `ACE_NATS_MODE` | Messaging mode: `embedded` or `external` |
| `--nats-url` | string | (in-process) | `ACE_NATS_URL` | NATS connection URL (required when `--nats-mode=external`) |
| `--cache-mode` | string | `embedded` | `ACE_CACHE_MODE` | Cache mode: `embedded` (Ristretto) or `external` (Valkey) |
| `--cache-url` | string | (unused) | `ACE_CACHE_URL` | Valkey/Redis connection URL (required when `--cache-mode=external`) |
| `--cache-max-cost` | int | `52428800` | `ACE_CACHE_MAX_COST` | Ristretto max cost in bytes (default 50MB) |
| `--telemetry-mode` | string | `embedded` | `ACE_TELEMETRY_MODE` | Telemetry mode: `embedded` (SQLite) or `external` (OTLP) |
| `--otlp-endpoint` | string | (unused) | `ACE_OTLP_ENDPOINT` | OTLP collector URL (required when `--telemetry-mode=external`) |
| `--dev` | bool | `false` | `ACE_DEV` | Enable development mode (Vite proxy for frontend) |

### 1.3 Subcommands

#### `ace` (default — no subcommand)

Starts the ACE server. Parses configuration, initializes all subsystems, and serves HTTP.

**Startup output (stdout, JSON):**

```json
{"level":"info","ts":"2026-04-12T10:00:00Z","msg":"starting ace","version":"0.1.0","data_dir":"/home/user/.local/share/ace","port":8080}
{"level":"info","ts":"2026-04-12T10:00:00Z","msg":"database initialized","mode":"embedded","path":"/home/user/.local/share/ace/ace.db"}
{"level":"info","ts":"2026-04-12T10:00:00Z","msg":"messaging initialized","mode":"embedded"}
{"level":"info","ts":"2026-04-12T10:00:00Z","msg":"cache initialized","mode":"embedded","max_cost_mb":50}
{"level":"info","ts":"2026-04-12T10:00:00Z","msg":"telemetry initialized","mode":"embedded"}
{"level":"info","ts":"2026-04-12T10:00:00Z","msg":"server listening","addr":"0.0.0.0:8080"}
```

**Startup failure output (stderr):**

```
Error: database: open failed: unable to create data directory: mkdir /home/user/.local/share/ace: permission denied
```

Each subsystem failure produces a specific error wrapped with the subsystem name. The process exits with code 1.

**Shutdown (SIGINT/SIGTERM):**

```json
{"level":"info","ts":"2026-04-12T10:05:00Z","msg":"shutdown initiated","signal":"SIGINT"}
{"level":"info","ts":"2026-04-12T10:05:00Z","msg":"draining HTTP connections","timeout":"10s"}
{"level":"info","ts":"2026-04-12T10:05:00Z","msg":"flushing telemetry"}
{"level":"info","ts":"2026-04-12T10:05:00Z","msg":"draining NATS connections","timeout":"5s"}
{"level":"info","ts":"2026-04-12T10:05:00Z","msg":"closing database","wal_checkpoint":"ok"}
{"level":"info","ts":"2026-04-12T10:05:01Z","msg":"shutdown complete"}
```

Process exits with code 0 on clean shutdown.

#### `ace paths`

Prints all resolved filesystem paths. Useful for debugging configuration.

**Output (stdout):**

```
Data Dir:     /home/user/.local/share/ace
Config Dir:   /home/user/.local/config/ace
Log Dir:      /home/user/.local/state/ace/logs
Database:     /home/user/.local/share/ace/ace.db
NATS Store:   /home/user/.local/share/ace/nats
Telemetry:    /home/user/.local/share/ace/telemetry
Cache:        (in-process, 50MB max)
```

Environment overrides are reflected in the output. For example:

```bash
$ ACE_DATA_DIR=/opt/ace ace paths
Data Dir:     /opt/ace
Config Dir:   /opt/ace
Log Dir:      /opt/ace/logs
Database:     /opt/ace/ace.db
NATS Store:   /opt/ace/nats
Telemetry:    /opt/ace/telemetry
Cache:        (in-process, 50MB max)
```

#### `ace version`

Prints version information.

**Output (stdout):**

```
ace v0.1.0
go1.26.0
build: 2026-04-12T10:00:00Z
commit: abc1234
```

#### `ace migrate`

Runs all pending database migrations manually and exits. Useful for pre-deployment checks.

**Output on success (stdout):**

```
Running migrations...
  20260321000000_create_users.sql ............ ok
  20260321000001_create_sessions.sql ........ ok
  20260412000000_create_telemetry.sql ....... ok
  20260412000001_create_usage_events.sql .... ok
Migrations complete: 4 applied, 0 pending.
```

**Output on failure (stderr):**

```
Error: migration failed at 20260412000000_create_telemetry.sql: table ott_spans already exists
```

#### `ace help`

Prints usage information including all flags and subcommands.

**Output (stdout):**

```
ACE — Agent Configuration Engine

Usage:
  ace [flags] [command]

Commands:
  ace           Start the ACE server (default)
  ace paths     Print configured filesystem paths
  ace version   Print version information
  ace migrate   Run database migrations and exit
  ace help      Print this help message

Flags:
  --data-dir string         Data root directory (env: ACE_DATA_DIR)
  --config string           Config file path (env: ACE_CONFIG)
  --port int                HTTP listen port (env: ACE_PORT, default: 8080)
  --host string             HTTP listen address (env: ACE_HOST, default: 0.0.0.0)
  --db-mode string          Database mode: embedded|external (env: ACE_DB_MODE, default: embedded)
  --db-url string           Database URL (env: ACE_DB_URL, required when db-mode=external)
  --nats-mode string        NATS mode: embedded|external (env: ACE_NATS_MODE, default: embedded)
  --nats-url string         NATS URL (env: ACE_NATS_URL, required when nats-mode=external)
  --cache-mode string       Cache mode: embedded|external (env: ACE_CACHE_MODE, default: embedded)
  --cache-url string        Cache URL (env: ACE_CACHE_URL, required when cache-mode=external)
  --cache-max-cost int      Max cache cost in bytes (env: ACE_CACHE_MAX_COST, default: 52428800)
  --telemetry-mode string   Telemetry mode: embedded|external (env: ACE_TELEMETRY_MODE, default: embedded)
  --otlp-endpoint string    OTLP collector URL (env: ACE_OTLP_ENDPOINT, required when telemetry-mode=external)
  --dev                     Enable development mode: proxy frontend to Vite dev server (env: ACE_DEV)

Configuration priority: CLI flags > environment variables > config file > XDG defaults
```

---

## 2. Configuration File Format

### 2.1 File Location

Default: `$XDG_CONFIG_HOME/ace/config.yaml` (typically `~/.config/ace/config.yaml`).

Override: `--config /path/to/config.yaml` or `ACE_CONFIG=/path/to/config.yaml`.

The file is optional. If absent, all values use defaults or environment overrides.

### 2.2 Full Configuration Schema

```yaml
# ACE Configuration File
# All values are optional; defaults are used for missing keys.
# Priority: CLI flags > env vars > this file > XDG defaults.

server:
  host: "0.0.0.0"         # --host / ACE_HOST
  port: 8080               # --port / ACE_PORT

data:
  dir: ""                  # --data-dir / ACE_DATA_DIR
                           # Empty string = XDG default ($XDG_DATA_HOME/ace/)

database:
  mode: "embedded"         # --db-mode / ACE_DB_MODE ("embedded" or "external")
  url: ""                  # --db-url / ACE_DB_URL
                           # When embedded: auto-constructed as file:{data_dir}/ace.db?_pragma=...
                           # When external: required, e.g. postgres://user:pass@host:5432/ace

messaging:
  mode: "embedded"         # --nats-mode / ACE_NATS_MODE ("embedded" or "external")
  url: ""                  # --nats-url / ACE_NATS_URL
                           # When embedded: ignored (in-process connection)
                           # When external: required, e.g. nats://host:4222

cache:
  mode: "embedded"         # --cache-mode / ACE_CACHE_MODE ("embedded" or "external")
  url: ""                  # --cache-url / ACE_CACHE_URL
                           # When embedded: ignored (in-process Ristretto)
                           # When external: required, e.g. valkey://host:6379
  max_cost: 52428800        # --cache-max-cost / ACE_CACHE_MAX_COST (bytes, default 50MB)

telemetry:
  mode: "embedded"         # --telemetry-mode / ACE_TELEMETRY_MODE ("embedded" or "external")
  otlp_endpoint: ""         # --otlp-endpoint / ACE_OTLP_ENDPOINT
                           # When embedded: ignored (SQLite exporters)
                           # When external: required, e.g. localhost:4317

auth:
  jwt_secret: ""            # Required. Must be ≥32 characters.
  access_token_ttl: "15m"  # Duration string
  refresh_token_ttl: "168h" # Duration string (7 days)
  jwt_audience: ["ace-api"]
  jwt_issuer: "ace-auth"
  rate_limit_per_ip: 100
  rate_limit_per_email: 10
  rate_limit_window: "1m"
  login_lockout_threshold: 5
  login_lockout_duration: "15m"
  password_min_length: 8
  password_require_upper: true
  password_require_lower: true
  password_require_number: true
  password_require_symbol: false

logging:
  level: "info"            # debug, info, warn, error
  format: "json"           # json or text

development:
  dev: false               # --dev / ACE_DEV (proxy frontend to Vite)
```

### 2.3 Value Resolution Rules

For each configuration key:

1. If CLI flag is provided → use CLI value
2. Else if environment variable is set → use env value
3. Else if config.yaml key is present and non-empty → use config value
4. Else → use hardcoded default

Type conversions:
- Durations: parsed via `time.ParseDuration` (e.g., `"15m"`, `"168h"`)
- Integers: parsed via `strconv.Atoi`
- Booleans: `"true"`, `"1"`, `"yes"` → true; `"false"`, `"0"`, `"no"` → false
- Lists: comma-separated strings from env vars, YAML arrays from config file

### 2.4 Validation Rules

The following are validated at startup after resolution:

| Rule | Condition | Error |
|------|-----------|-------|
| Data dir writable | `--data-dir` must be creatable or writable | `config: data dir is not writable: {path}` |
| Port range | `1 ≤ port ≤ 65535` | `config: invalid port: {port}` |
| External DB URL required | If `db-mode=external`, `db-url` must be non-empty | `config: db-url is required when db-mode is external` |
| External NATS URL required | If `nats-mode=external`, `nats-url` must be non-empty | `config: nats-url is required when nats-mode is external` |
| External cache URL required | If `cache-mode=external`, `cache-url` must be non-empty | `config: cache-url is required when cache-mode is external` |
| External OTLP endpoint required | If `telemetry-mode=external`, `otlp-endpoint` must be non-empty | `config: otlp-endpoint is required when telemetry-mode is external` |
| JWT secret length | `jwt_secret` must be ≥32 characters | `config: jwt_secret must be at least 32 characters` |
| JWT secret present | `jwt_secret` must be non-empty | `config: jwt_secret is required` |
| Mode values | `db-mode`, `nats-mode`, `cache-mode`, `telemetry-mode` must be `embedded` or `external` | `config: invalid {field}: {value}, must be embedded or external` |

### 2.5 Data Directory Creation

On startup, ACE creates the following directory tree with `0700` permissions:

```
{data_dir}/
├── ace.db              # Created by SQLite driver on first open
├── nats/               # Created by embedded NATS for JetStream storage
├── logs/               # Created for structured log file output
│   └── ace.log
└── telemetry/          # Created for telemetry SQLite database
    └── metrics.db
```

If any directory creation fails (permissions, disk full), startup aborts with a descriptive error.

---

## 3. API Endpoints

### 3.1 Route Map

The binary exposes a single HTTP server. Routes are matched in this order:

| Priority | Prefix | Handler | Auth |
|----------|--------|---------|------|
| 1 | `/health/live` | Health live check | None |
| 2 | `/health/ready` | Health ready check | None |
| 3 | `/auth/*` | Auth routes (register, login, logout, refresh, password reset, magic link) | None (public) or JWT (session routes) |
| 4 | `/admin/*` | Admin routes (user management) | JWT + admin role |
| 5 | `/api/v1/*` | Business logic routes | JWT |
| 6 | `/telemetry/*` | Telemetry Inspector routes | JWT |
| 7 | `/*` | SPA handler (embedded assets or Vite proxy) | None |

### 3.2 Existing API Routes (Unchanged)

All existing `/auth/*`, `/admin/*`, and `/health/*` routes continue with the same request/response contracts. No breaking changes to any endpoint.

### 3.3 New Telemetry Inspector Endpoints

#### `GET /telemetry/spans`

Query recent trace spans.

**Request:**

| Parameter | Location | Type | Required | Default | Description |
|-----------|----------|------|----------|---------|-------------|
| `limit` | query | int | no | 50 | Maximum spans to return (1-1000) |
| `offset` | query | int | no | 0 | Pagination offset |
| `service` | query | string | no | "" | Filter by service name |
| `operation` | query | string | no | "" | Filter by operation name |
| `start_time` | query | string (RFC3339) | no | 24h ago | Start of time range |
| `end_time` | query | string (RFC3339) | no | now | End of time range |

**Response (200 OK):**

```json
{
  "spans": [
    {
      "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
      "span_id": "00f067aa0ba902b7",
      "operation": "auth.login",
      "service": "ace-api",
      "start_time": "2026-04-12T10:00:00Z",
      "end_time": "2026-04-12T10:00:00.150Z",
      "duration_ms": 150,
      "status": "ok",
      "attributes": {"http.method": "POST", "http.status_code": 200}
    }
  ],
  "total": 142,
  "limit": 50,
  "offset": 0
}
```

**Error Responses:**

| Status | Condition | Body |
|--------|-----------|------|
| 400 | Invalid `limit`, `offset`, or time format | `{"error": "invalid limit: must be between 1 and 1000"}` |
| 401 | Missing or invalid JWT | `{"error": "unauthorized"}` |

#### `GET /telemetry/metrics`

Query metric summaries.

**Request:**

| Parameter | Location | Type | Required | Default | Description |
|-----------|----------|------|----------|---------|-------------|
| `name` | query | string | no | "" | Filter by metric name |
| `window` | query | string | no | `1h` | Aggregation window: `5m`, `15m`, `1h`, `6h`, `24h` |
| `limit` | query | int | no | 50 | Maximum metric series to return (1-200) |

**Response (200 OK):**

```json
{
  "metrics": [
    {
      "name": "http_requests_total",
      "type": "counter",
      "labels": {"method": "POST", "path": "/auth/login"},
      "value": 142.0,
      "timestamp": "2026-04-12T10:00:00Z",
      "window": "1h"
    }
  ],
  "total": 8,
  "limit": 50
}
```

**Error Responses:**

| Status | Condition | Body |
|--------|-----------|------|
| 400 | Invalid `window` value | `{"error": "invalid window: must be one of 5m, 15m, 1h, 6h, 24h"}` |
| 401 | Missing or invalid JWT | `{"error": "unauthorized"}` |

#### `GET /telemetry/usage`

Query cost attribution data (usage events).

**Request:**

| Parameter | Location | Type | Required | Default | Description |
|-----------|----------|------|----------|---------|-------------|
| `agent_id` | query | string (UUID) | no | "" | Filter by agent ID |
| `event_type` | query | string | no | "" | Filter by event type (llm_call, memory_read, tool_exec) |
| `from` | query | string (RFC3339) | no | 7d ago | Start of time range |
| `to` | query | string (RFC3339) | no | now | End of time range |
| `limit` | query | int | no | 100 | Maximum events to return (1-500) |
| `offset` | query | int | no | 0 | Pagination offset |

**Response (200 OK):**

```json
{
  "events": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "agent_id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
      "session_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "event_type": "llm_call",
      "model": "gpt-4",
      "input_tokens": 1500,
      "output_tokens": 800,
      "cost_usd": 0.045,
      "duration_ms": 2300,
      "timestamp": "2026-04-12T10:30:00Z"
    }
  ],
  "total": 85,
  "limit": 100,
  "offset": 0
}
```

**Error Responses:**

| Status | Condition | Body |
|--------|-----------|------|
| 400 | Invalid `agent_id` format or time format | `{"error": "invalid agent_id: must be a valid UUID"}` |
| 401 | Missing or invalid JWT | `{"error": "unauthorized"}` |

#### `GET /telemetry/health`

Returns subsystem health status.

**Request:** No parameters.

**Response (200 OK):**

```json
{
  "status": "healthy",
  "checks": {
    "database": {
      "status": "ok",
      "mode": "embedded",
      "path": "/home/user/.local/share/ace/ace.db",
      "size_bytes": 1048576
    },
    "messaging": {
      "status": "ok",
      "mode": "embedded",
      "connections": 0
    },
    "cache": {
      "status": "ok",
      "mode": "embedded",
      "max_cost_bytes": 52428800,
      "current_cost_bytes": 1048576,
      "hit_rate": 0.87
    },
    "telemetry": {
      "status": "ok",
      "mode": "embedded",
      "spans_last_hour": 142,
      "metrics_last_hour": 8
    }
  }
}
```

**Degraded response (503 Service Unavailable):**

```json
{
  "status": "degraded",
  "checks": {
    "database": {
      "status": "ok",
      "mode": "embedded",
      "path": "/home/user/.local/share/ace/ace.db",
      "size_bytes": 1048576
    },
    "messaging": {
      "status": "error",
      "error": "embedded NATS: JetStream not available"
    },
    "cache": {
      "status": "ok",
      "mode": "embedded",
      "max_cost_bytes": 52428800,
      "current_cost_bytes": 1048576,
      "hit_rate": 0.87
    },
    "telemetry": {
      "status": "ok",
      "mode": "embedded",
      "spans_last_hour": 142,
      "metrics_last_hour": 8
    }
  }
}
```

### 3.4 Health Endpoints

#### `GET /health/live`

Liveness check. Returns 200 if the process is running.

```json
{"status": "ok"}
```

#### `GET /health/ready`

Readiness check. Returns 200 if all subsystems can serve requests.

```json
{"status": "ok", "checks": {"database": {"status": "ok"}, "nats": {"status": "ok"}}}
```

If any subsystem is unhealthy, returns 503:

```json
{"status": "degraded", "checks": {"database": {"status": "ok"}, "nats": {"status": "error", "error": "connection refused"}}}
```

### 3.5 SPA Serving (Production Mode)

When built with the `embed` build tag (production default), the catch-all `/*` route serves the embedded SvelteKit SPA.

**Routing rules (evaluated in order):**

1. Request path has a file extension (`.js`, `.css`, `.svg`, `.png`, `.ico`, `.woff2`, etc.) → serve file from embedded FS with appropriate `Content-Type` and cache headers
2. Request path starts with `/_app/` or `/@vite/` → serve from embedded FS (Vite internal assets)
3. All other paths → serve `index.html` with `Content-Type: text/html; charset=utf-8`

**Asset caching:**

| Pattern | Cache-Control |
|---------|--------------|
| `/_app/immutable/` | `public, max-age=31536000, immutable` |
| Other files with extension | `public, max-age=3600` |
| `index.html` | `no-cache` |

### 3.6 Vite Dev Proxy (Development Mode)

When `--dev` flag is set (or `ACE_DEV=true`):

1. `/api/v1/*`, `/auth/*`, `/admin/*`, `/health/*` → served by Go handlers as normal
2. All other `/*` requests → reverse-proxied to `http://localhost:5173` (Vite dev server)
3. WebSocket upgrade requests → also proxied (for HMR)
4. If Vite is unreachable → return 502:
   ```json
   {"error": "vite dev server not reachable at localhost:5173. Run 'npm run dev' in the frontend directory."}
   ```

---

## 4. Error Messages and Handling

### 4.1 Error Formatting Convention

All errors follow a structured JSON format for API responses and a prefixed format for CLI output.

**API error response body:**

```json
{
  "error": "human-readable message",
  "code": "MACHINE_READABLE_CODE"
}
```

**CLI error output (stderr):**

```
{subsystem}: {action} failed: {root cause}
```

Examples:
```
database: open failed: unable to create data directory: mkdir /opt/ace: permission denied
messaging: server start failed: embedded NATS did not become ready within 10s
cache: initialization failed: ristretto: max cost must be positive
telemetry: initialization failed: unable to create telemetry directory: permission denied
config: invalid port: 99999 (must be between 1 and 65535)
migration: failed at 20260412000000_create_telemetry.sql: table ott_spans already exists
```

### 4.2 Startup Errors

Each subsystem initializes sequentially. On failure, all previously-initialized subsystems are rolled back, and the process exits with code 1.

| Phase | Error Condition | Exit Message | Exit Code |
|-------|----------------|--------------|-----------|
| Config | Invalid flag value | `config: invalid {field}: {reason}` | 1 |
| Config | Missing required field | `config: {field} is required when {condition}` | 1 |
| Paths | Data dir not writable | `config: data dir is not writable: {path}` | 1 |
| Database | SQLite/PG open failure | `database: open failed: {driver error}` | 1 |
| Database | Migration failure | `migration: failed at {file}: {sql error}` | 1 |
| NATS | Embedded server start timeout | `messaging: server start failed: embedded NATS did not become ready within 10s` | 1 |
| NATS | Remote connection failure | `messaging: connection failed: nats: {remote error}` | 1 |
| Cache | Ristretto init failure | `cache: initialization failed: ristretto: {error}` | 1 |
| Cache | Valkey connection failure | `cache: connection failed: valkey: {remote error}` | 1 |
| Telemetry | SQLite exporter failure | `telemetry: initialization failed: unable to create exporter: {error}` | 1 |
| Telemetry | OTLP connection failure | `telemetry: initialization failed: unable to connect to collector: {error}` | 1 |
| Server | Port already bound | `server: listen failed: bind: address already in use` | 1 |

### 4.3 Runtime Errors

| Subsystem | Error | Handling |
|-----------|-------|----------|
| Database | `SQLITE_BUSY` (lock contention) | Automatically retried by `busy_timeout=5000ms` pragma. If 5s timeout exceeded, return HTTP 503 to client. |
| Database | `SQLITE_CORRUPT` | Log critical error. Return HTTP 500. Suggest `ace migrate` or data dir recreation. |
| Database | Disk full | Log critical error. Return HTTP 503. |
| NATS (embedded) | Internal publish failure | Log error. Return HTTP 500 for the request. Embedded mode should never see connection failures. |
| NATS (external) | Connection lost | Auto-reconnect via `nats.go` built-in reconnection (2s intervals, max 10 attempts). After exhausting, return HTTP 503. |
| Cache | Ristretto admission rejection | Silent. Item is not cached; subsequent reads fall through to source. Logged at debug level. |
| Cache | Valkey connection lost | Auto-reconnect with exponential backoff. During outage, cache operations return `ErrCacheUnavailable` and the service layer proceeds without cache (cache-aside pattern). |
| Telemetry | SQLite write failure | Log warning. Telemetry data is best-effort; never blocks request path. |
| Frontend SPA | Embedded asset not found | Serve `index.html` (SPA fallback). If `index.html` missing, return HTTP 500. |
| Frontend SPA | Vite dev server unreachable | Return HTTP 502 with JSON error body. |

### 4.4 Shutdown Error Handling

Shutdown proceeds in reverse startup order regardless of individual failures:

1. HTTP server drain (10s deadline)
2. Telemetry flush (exporters push pending spans/metrics to SQLite)
3. NATS client drain (5s timeout)
4. NATS server shutdown (embedded only)
5. Cache close (Ristretto close)
6. Database close (WAL checkpoint, close connections)

Each step logs errors but does not abort. The final shutdown aggregates errors via `errors.Join()` and returns them from `App.Shutdown()`.

### 4.5 Configuration Validation Errors

All validation occurs before subsystem initialization. Errors are precise:

| Condition | Error Message |
|-----------|---------------|
| Port out of range | `config: invalid port: {port} (must be between 1 and 65535)` |
| Invalid mode value | `config: invalid db-mode: "{value}" (must be "embedded" or "external")` |
| Missing URL for external mode | `config: db-url is required when db-mode is "external"` |
| JWT secret missing | `config: jwt_secret is required` |
| JWT secret too short | `config: jwt_secret must be at least 32 characters (got {n})` |
| Invalid duration format | `config: invalid duration: "{value}" (use format like "15m", "168h")` |
| Data dir not writable | `config: data dir is not writable: {path}: {os error}` |
| Config file parse error | `config: failed to parse config file: {yaml error}` |
| Config file not found (explicit) | `config: config file not found: {path}` (only when `--config` is explicitly provided; missing default is silently ignored) |

---

## 5. Installation and Upgrade Process

### 5.1 Fresh Installation

**Method 1: `curl | sh` (primary)**

```bash
curl -fsSL https://ace.dev/install.sh | sh
```

Script behavior:
1. Detect OS: `uname -s` → `linux` or `darwin`
2. Detect arch: `uname -m` → `x86_64` (→ `amd64`), `aarch64` (→ `arm64`), `arm64`
3. Fetch latest release tag: `curl -fsSL https://api.github.com/repos/ace-org/ace/releases/latest | grep tag_name`
4. Download binary: `curl -fsSL https://github.com/ace-org/ace/releases/download/{tag}/ace_{os}_{arch} -o /tmp/ace`
5. Download checksums: `curl -fsSL https://github.com/ace-org/ace/releases/download/{tag}/checksums.txt -o /tmp/ace_checksums.txt`
6. Verify checksum: `sha256sum -c /tmp/ace_checksums.txt --ignore-missing`
7. Install: `install /tmp/ace $HOME/.local/bin/ace`
8. Make executable: `chmod +x $HOME/.local/bin/ace`
9. Check PATH: if `$HOME/.local/bin` not in `$PATH`, print shell-appropriate export:
   - bash: `echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc`
   - zsh: `echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc`
   - fish: `echo 'fish_add_path $HOME/.local/bin' >> ~/.config/fish/config.fish`
10. Print success message:
    ```
    ACE v0.1.0 installed successfully!

    Installed to: /home/user/.local/bin/ace
    Data directory: /home/user/.local/share/ace/
    Configuration: /home/user/.local/config/ace/

    Quick start:
      ace                    Start the server
      ace --help             Show all options
      ace paths               Show configured paths

    Documentation: https://ace.dev/docs
    ```

**Custom install directory:**

```bash
curl -fsSL https://ace.dev/install.sh | sh -s -- -b /usr/local/bin
```

The `-b` flag overrides the default `$HOME/.local/bin` installation target.

**Method 2: Verification-only (`verify.sh`)**

For users who reject pipe-to-shell:

```bash
# Download manually
curl -fsSL https://github.com/ace-org/ace/releases/download/v0.1.0/ace_linux_amd64 -o ace
curl -fsSL https://github.com/ace-org/ace/releases/download/v0.1.0/checksums.txt -o checksums.txt

# Verify
curl -fsSL https://ace.dev/verify.sh | sh
# Or: sh verify.sh
```

`verify.sh` checks the SHA256 checksum of `ace` binary against `checksums.txt`.

### 5.2 Installation Script Error Handling

| Condition | Error Message | Exit Code |
|-----------|--------------|-----------|
| Unsupported OS | `Error: unsupported operating system: {os}. ACE supports linux and darwin.` | 1 |
| Unsupported architecture | `Error: unsupported architecture: {arch}. ACE supports amd64 and arm64.` | 1 |
| GitHub API unreachable | `Error: unable to fetch latest release from GitHub: {http error}` | 1 |
| Binary download failure | `Error: failed to download binary: {http error}` | 1 |
| Checksum download failure | `Error: failed to download checksums: {http error}` | 1 |
| Checksum mismatch | `Error: checksum mismatch! Expected: {expected}, Got: {actual}. The binary may have been tampered with. Aborting.` | 1 |
| Install directory not writable | `Error: cannot write to {dir}: {os error}. Try with -b flag to specify a different directory.` | 1 |
| `ace` binary already exists | Warning only: `Warning: replacing existing installation at {path}` | 0 (continues) |

### 5.3 First Run

After installation, running `ace` for the first time:

1. Resolves configuration (all defaults apply)
2. Creates data directory tree at `$XDG_DATA_HOME/ace/` with `0700` permissions
3. Creates config directory at `$XDG_CONFIG_HOME/ace/` with `0700` permissions (no config file created — defaults used)
4. Opens SQLite database at `{data_dir}/ace.db` with WAL pragmas applied
5. Runs all Goose migrations against the database
6. Starts embedded NATS server (`DontListen: true`)
7. Initializes Ristretto cache (50MB budget)
8. Initializes telemetry (SQLite exporters, zap logger)
9. Starts HTTP server on `:8080`
10. Prints startup log lines (see section 1.3)

### 5.4 Upgrade Process

```bash
# Re-run install script — replaces the binary
curl -fsSL https://ace.dev/install.sh | sh
```

On next `ace` restart:
1. New binary starts up
2. Goose checks migration version in database
3. Any new migrations are applied automatically
4. Existing data is preserved (SQLite file and NATS store remain)

**Migration strategy:**

- Database schema changes ship as new Goose migration files in the binary
- On startup, `database.Migrate()` runs `goose.Up(db, migrationsDir)` — this applies only pending migrations
- Migrations are always forward-only (no down migrations in production)
- If a migration fails, startup aborts with the migration error

**Data compatibility:**

- SQLite file format is forward-compatible within the same major version
- NATS JetStream data in `{data_dir}/nats/` is recreated on startup if missing
- Cache data (Ristretto) is in-memory only — no persistence concern on upgrade

### 5.5 Downgrade

Downgrading to an older version is not officially supported. If necessary:
1. Back up data directory: `cp -r ~/.local/share/ace ~/.local/share/ace.bak`
2. Replace binary with older version
3. If schema incompatibility occurs, the older migration set will fail to apply
4. Restore from backup: `rm -rf ~/.local/share/ace && mv ~/.local/share/ace.bak ~/.local/share/ace`

### 5.6 Uninstall

```bash
rm $HOME/.local/bin/ace
rm -rf $HOME/.local/share/ace
rm -rf $HOME/.config/ace
rm -rf $HOME/.local/state/ace
```

No system-level changes are made during installation or uninstallation.

---

## 6. Database Schema (Migrations)

### 6.1 Migration Files

Goose migrations are embedded in the binary and applied on startup. Each file follows the naming convention `{timestamp}_{description}.sql`.

**New migrations for deployment-dx:**

| File | Description |
|------|-------------|
| `20260412000000_create_telemetry.sql` | Creates `ott_spans`, `ott_metrics` tables for embedded telemetry |
| `20260412000001_create_usage_events.sql` | Creates `usage_events` table for cost attribution |

**Existing migrations** (adapted from PostgreSQL to SQLite dialect):

| Change | PostgreSQL | SQLite |
|--------|-----------|--------|
| Auto IDs | `SERIAL` / `BIGSERIAL` | `INTEGER PRIMARY KEY AUTOINCREMENT` |
| Booleans | `BOOLEAN` | `INTEGER` (0/1) |
| UUIDs | `gen_random_uuid()` | Go-generated UUIDs (no SQL function) |
| Timestamps | `TIMESTAMPTZ` | `TEXT` (RFC3339 format) |
| RETURNING | `RETURNING *` | `lastInsertId` + select query |

### 6.2 Telemetry Schema

**`ott_spans` table:**

```sql
CREATE TABLE IF NOT EXISTS ott_spans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    trace_id TEXT NOT NULL,
    span_id TEXT NOT NULL,
    parent_span_id TEXT,
    operation_name TEXT NOT NULL,
    service_name TEXT NOT NULL,
    start_time TEXT NOT NULL,
    end_time TEXT NOT NULL,
    duration_ms INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'ok',
    attributes TEXT,  -- JSON-encoded
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_ott_spans_trace_id ON ott_spans(trace_id);
CREATE INDEX IF NOT EXISTS idx_ott_spans_service ON ott_spans(service_name);
CREATE INDEX IF NOT EXISTS idx_ott_spans_created_at ON ott_spans(created_at);
```

**`ott_metrics` table:**

```sql
CREATE TABLE IF NOT EXISTS ott_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'counter',
    labels TEXT,  -- JSON-encoded
    value REAL NOT NULL,
    timestamp TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_ott_metrics_name ON ott_metrics(name);
CREATE INDEX IF NOT EXISTS idx_ott_metrics_created_at ON ott_metrics(created_at);
```

**`usage_events` table:**

```sql
CREATE TABLE IF NOT EXISTS usage_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    model TEXT,
    input_tokens INTEGER,
    output_tokens INTEGER,
    cost_usd REAL,
    duration_ms INTEGER,
    metadata TEXT,  -- JSON-encoded
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_usage_events_agent_id ON usage_events(agent_id);
CREATE INDEX IF NOT EXISTS idx_usage_events_event_type ON usage_events(event_type);
CREATE INDEX IF NOT EXISTS idx_usage_events_created_at ON usage_events(created_at);
```

### 6.3 Data Retention

Telemetry data is pruned on startup and every 6 hours:

| Table | Retention | Prune Method |
|-------|-----------|-------------|
| `ott_spans` | 7 days | `DELETE FROM ott_spans WHERE created_at < datetime('now', '-7 days')` |
| `ott_metrics` | 24 hours | `DELETE FROM ott_metrics WHERE created_at < datetime('now', '-1 day')` |
| `usage_events` | 90 days | `DELETE FROM usage_events WHERE created_at < datetime('now', '-90 days')` |

---

## 7. Build System

### 7.1 Makefile Targets

```makefile
# Primary targets
ace:          Build the Go binary (development mode with Vite proxy)
test:         Full validation pipeline (build, lint, test, docs, git add)
```

**`make ace`** (development build):
```bash
go build -o bin/ace ./cmd/ace/
# Default build (no tags) → frontend.go is compiled
# Frontend requests proxy to http://localhost:5173 (Vite dev server)
# Hot reload works via Vite HMR
```

**`make test`** (full validation):
```bash
go build ./...
go vet ./...
go test -short ./...
sqlc generate
cd frontend && npm run lint && npm run test:run
go run ./scripts/docs-gen/...
git add .
```

**Production builds** are handled by GoReleaser (see 7.2), not Makefile.

### 7.2 GoReleaser Configuration

`.goreleaser.yml` produces multi-arch binaries:

```yaml
builds:
  - main: ./cmd/ace/
    ldflags:
      - -s -w -X main.version={{.Version}} -X main.commit={{.ShortCommit}} -X main.date={{.Date}}
    goos:
      - linux
      - darwin
    goarch:
      - amd64
      - arm64
    tags:
      - embed
archives:
  - format: binary
    name_template: ace_{{ .Os }}_{{ .Arch }}
checksum:
  name_template: checksums.txt
release:
  github:
    owner: ace-org
    name: ace
```

### 7.3 Build Tags

| Tag | Files Included | Behavior |
|-----|---------------|----------|
| (none, default) | `frontend.go` | Proxies frontend to `http://localhost:5173`. Development mode with HMR. |
| `embed` | `frontend_embed.go` | Embedded static assets. Production mode serves from `go:embed`. |

### 7.4 SQLC Dialect Change

`sqlc.yaml` changes from `postgresql` to `sqlite`:

```yaml
version: "2"
sql:
  - schema: "migrations/"
    queries: "internal/repository/queries/"
    engine: "sqlite"
    gen:
      go:
        package: "generated"
        out: "internal/repository/generated"
```

All SQL query files must use SQLite-compatible syntax (see section 6.1 for dialect mapping).

---

## 8. Logging Specification

### 8.1 Log Format

All logs are structured JSON written to stdout. In addition, logs are written to `{data_dir}/logs/ace.log` via `lumberjack` rotation.

**Log entry structure:**

```json
{
  "level": "info",
  "ts": "2026-04-12T10:00:00.000Z",
  "msg": "database initialized",
  "mode": "embedded",
  "path": "/home/user/.local/share/ace/ace.db"
}
```

### 8.2 Log Rotation

Rotating file output to `{data_dir}/logs/ace.log`:

| Setting | Value |
|---------|-------|
| Max size | 100 MB |
| Max backups | 3 |
| Max age | 28 days |
| Compress | true |

### 8.3 Startup/Shutdown Log Sequence

**Startup:**
```
INFO  starting ace  version=0.1.0 data_dir=/home/user/.local/share/ace port=8080
INFO  database initialized  mode=embedded path=/home/user/.local/share/ace/ace.db
INFO  migrations applied  count=4
INFO  messaging initialized  mode=embedded
INFO  cache initialized  mode=embedded max_cost_mb=50
INFO  telemetry initialized  mode=embedded
INFO  server listening  addr=0.0.0.0:8080
```

**Shutdown:**
```
INFO  shutdown initiated  signal=SIGINT
INFO  draining HTTP connections  timeout=10s
INFO  HTTP server drained
INFO  flushing telemetry
INFO  draining NATS connections  timeout=5s
INFO  NATS connections drained
INFO  closing database  wal_checkpoint=ok
INFO  shutdown complete
```

### 8.4 Error Log Examples

```
ERROR database: open failed  error="unable to open database file: permission denied" path=/opt/ace/ace.db
ERROR messaging: connection failed  error="nats: connection refused" url=nats://prod:4222
ERROR cache: operation failed  error="ristretto: item rejected" key=agent:123:memory
WARN  telemetry: span write failed  error="disk full" spans_dropped=12
```