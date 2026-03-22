# Connection Pooling

**FSD Requirement**: FR-3.3

---

## Overview

The ACE Framework uses `pgxpool` (from `pgx/v5`) for PostgreSQL connection pooling. This document covers pool configuration, tuning, health checks, and environment-specific settings.

---

## pgxpool Configuration

`pgxpool` is the PostgreSQL-native connection pool from the `pgx/v5` library. It bypasses the `database/sql` abstraction for better performance and PostgreSQL-specific features.

### Pool Parameters

| Parameter | Description | Development | Production (Single) | Production (Multi) |
|-----------|-------------|-------------|---------------------|-------------------|
| `MaxConns` | Maximum open connections | 10 | 25–50 | 10–20 per instance |
| `MinConns` | Minimum idle connections kept warm | 2 | 5–10 | 2–5 per instance |
| `MaxConnLifetime` | Max time a connection is reused | 30 min | 1 hour | 30 min |
| `MaxConnIdleTime` | Max time idle before closing | 10 min | 30 min | 5–10 min |
| `HealthCheckPeriod` | Connection health check interval | 1 min | 1 min | 30 sec |

### Configuration Example

```go
config, err := pgxpool.ParseConfig(databaseURL)
if err != nil {
    return nil, fmt.Errorf("failed to parse database config: %w", err)
}

config.MaxConns = 25
config.MinConns = 5
config.MaxConnLifetime = 1 * time.Hour
config.MaxConnIdleTime = 30 * time.Minute
config.HealthCheckPeriod = 1 * time.Minute

pool, err := pgxpool.NewWithConfig(ctx, config)
if err != nil {
    return nil, fmt.Errorf("failed to create connection pool: %w", err)
}
```

---

## Tuning Guidelines

### MaxConns Baseline

Rule of thumb: `MaxConns = (CPU cores * 2) + 1`

Adjust based on:
- PostgreSQL's `max_connections` setting (default 100)
- Number of application instances
- Expected concurrent request load

### PostgreSQL max_connections Budget

```
PostgreSQL max_connections = 100
├── Reserve 10–20 for admin/monitoring tools
├── Remaining 80–90 available for applications
└── Per instance (3 API instances): ~25–30 max connections
```

### Connection Lifetime

- **Shorter** when using PgBouncer or load-balanced replicas (avoids stale connections to removed replicas)
- **Longer** in single-instance deployments (reduces connection creation overhead)
- Rule: Set `MaxConnLifetime` shorter than any load balancer's idle timeout

### Idle Connections

- In production: Set `MinConns = MaxConns` to avoid connection creation latency under load
- In development: Use smaller `MinConns` to save resources
- Monitor `IdleConns()` — if consistently zero, the pool may be undersized

---

## Health Check Pattern

The `/health/ready` endpoint checks database connectivity using `pool.Ping()`:

```go
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
    checks := map[string]checkResult{}
    overallStatus := "ok"
    httpStatus := http.StatusOK

    checks["database"] = checkResult{Status: "ok"}
    if err := h.pool.Ping(r.Context()); err != nil {
        overallStatus = "degraded"
        checks["database"] = checkResult{Status: "fail", Reason: "ping failed"}
        httpStatus = http.StatusServiceUnavailable
    }

    response.JSON(w, httpStatus, map[string]any{
        "status": overallStatus,
        "checks": checks,
    })
}
```

This is implemented in `backend/services/api/internal/handler/health.go`.

---

## Pool Monitoring

Track pool stats via `pool.Stat()`:

| Method | Description | Alert Threshold |
|--------|-------------|-----------------|
| `AcquireCount()` | Total connections acquired | — |
| `AcquireDuration()` | Total time waiting for connections | Increasing trend = pool exhaustion |
| `IdleConns()` | Current idle connections | Consistently zero = pool undersized |
| `TotalConns()` | Total connections in pool | At `MaxConns` = pool at capacity |
| `MaxConns()` | Configured maximum | — |
| `ConstructingConns()` | Connections being created | Stuck non-zero = connection issues |

### Monitoring Example

```go
stat := pool.Stat()
log.Info("pool stats",
    "total_conns", stat.TotalConns(),
    "idle_conns", stat.IdleConns(),
    "acquire_count", stat.AcquireCount(),
    "acquire_duration", stat.AcquireDuration(),
)
```

### Alerting

- **Pool exhaustion**: `AcquireDuration` increases consistently → increase `MaxConns` or add instances
- **Connection leak**: `TotalConns` stays at `MaxConns` with `IdleConns = 0` → check for missing `pool.Release()` calls
- **Connection churn**: High `AcquireCount` relative to request rate → increase `MinConns`

---

## Connection Leak Detection

pgxpool does not have built-in leak detection. Mitigation strategies:

1. **Always use `defer conn.Release()`** after `pool.Acquire()`
2. **Set `MaxConnLifetime`** — leaked connections eventually expire
3. **Monitor `TotalConns()`** — if it hits `MaxConns` and stays there, connections may be leaking
4. **Use context timeouts** — `context.WithTimeout` prevents connections from being held indefinitely

---

## PgBouncer (Supplementary)

For production deployments with multiple application instances, consider PgBouncer:

| Mode | Use Case | Notes |
|------|----------|-------|
| Transaction | Most applications | Connection returned after transaction completes |
| Session | Applications using prepared statements | Connection held for entire session |
| Statement | Simple queries only | Each statement gets a fresh connection |

PgBouncer reduces PostgreSQL connection overhead and allows higher instance counts. However, it may interfere with prepared statements — use `pgx`'s simple protocol mode if needed.

---

## Environment-Specific Summary

| Setting | Development | Staging | Production |
|---------|-------------|---------|------------|
| MaxConns | 10 | 15 | 25–50 |
| MinConns | 2 | 3 | 5–10 |
| MaxConnLifetime | 30 min | 45 min | 1 hour |
| MaxConnIdleTime | 10 min | 20 min | 30 min |
| HealthCheckPeriod | 1 min | 1 min | 1 min |

---

## Notes

- `pgxpool` uses `database/sql` under the hood for the connection pool but exposes a pgx-native API
- Connection parameters can be set via the `DATABASE_URL` environment variable or `pgxpool.ParseConfig()`
- For local development, consider using a smaller `MaxConns` to avoid exhausting PostgreSQL's `max_connections` with multiple services
