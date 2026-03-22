# Query Patterns: Usage

**FSD Requirement**: FR-1.4

---

## Overview

Query patterns for the `usage_events` table. Covers CRUD operations, filtering, pagination, and aggregation. All examples use SQLC annotation syntax.

**Note**: No SQLC queries are currently defined for `usage_events`. These patterns document the recommended approach when queries are created.

---

## CRUD Operations

### Create (Insert)

```sql
-- name: CreateUsageEvent :one
INSERT INTO usage_events (
    timestamp, agent_id, cycle_id, session_id,
    service_name, operation_type, resource_type,
    cost_usd, duration_ms, token_count, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;
```

**Go usage**:
```go
event, err := queries.CreateUsageEvent(ctx, db.CreateUsageEventParams{
    Timestamp:     time.Now(),
    AgentID:       agentID,
    CycleID:       cycleID,
    SessionID:     sessionID,
    ServiceName:   "openai",
    OperationType: "completion",
    ResourceType:  "llm",
    CostUsd:       pgtype.Numeric{...},
    DurationMs:    pgtype.Int8{Int64: 1500, Valid: true},
    TokenCount:    pgtype.Int8{Int64: 250, Valid: true},
    Metadata:      json.RawMessage(`{"model":"gpt-4"}`),
})
```

### Read (Single)

```sql
-- name: GetUsageEventByID :one
SELECT * FROM usage_events WHERE id = $1;
```

### Read (List)

```sql
-- name: ListUsageEventsByAgentID :many
SELECT * FROM usage_events
WHERE agent_id = $1
ORDER BY timestamp DESC
LIMIT $2 OFFSET $3;
```

### Delete

Usage events are immutable — hard delete is not recommended. For data retention, use archiving:

```sql
-- name: ArchiveOldUsageEvents :exec
DELETE FROM usage_events WHERE timestamp < $1;
```

---

## Filtering Patterns

### Filter by Agent

```sql
WHERE agent_id = $1
```

### Filter by Time Range

```sql
WHERE timestamp BETWEEN $1 AND $2
ORDER BY timestamp DESC
```

Supported by `idx_usage_events_timestamp`.

### Filter by Operation Type

```sql
WHERE operation_type = $1
```

Supported by `idx_usage_events_operation_type`.

### Filter by Service Name

```sql
WHERE service_name = $1
```

Supported by `idx_usage_events_service_name`.

### Combined Filters

```sql
WHERE agent_id = $1
  AND timestamp BETWEEN $2 AND $3
  AND operation_type = $4
```

**Recommended composite index**: `idx_usage_events_agent_timestamp` on `(agent_id, timestamp DESC)`.

### JSONB Metadata Filter

```sql
WHERE metadata @> '{"model": "gpt-4"}'::jsonb
```

Requires GIN index on `metadata` for performance.

---

## Pagination Patterns

### Cursor-Based Pagination (Recommended)

Cursor-based pagination provides O(1) performance regardless of position in the dataset, unlike offset-based pagination which degrades linearly.

```sql
-- First page
-- name: ListUsageEventsFirstPage :many
SELECT * FROM usage_events
ORDER BY timestamp DESC, id DESC
LIMIT $1;

-- Next page (cursor is the last row's timestamp and id)
-- name: ListUsageEventsAfter :many
SELECT * FROM usage_events
WHERE (timestamp, id) < ($1, $2)
ORDER BY timestamp DESC, id DESC
LIMIT $3;
```

**Cursor encoding**: Encode `(timestamp, id)` as base64url of JSON:
```json
{"timestamp": "2026-03-21T12:00:00Z", "id": "550e8400-e29b-41d4-a716-446655440000"}
```

**Recommended index**: `(timestamp DESC, id DESC)` composite for efficient cursor lookups.

**Performance**: Constant ~50ms regardless of page number. Compare to offset at page 1000: 5+ seconds.

### Offset-Based Pagination (Small Datasets Only)

Use only for admin dashboards with page numbers on small datasets (<10K rows):

```sql
-- name: ListUsageEventsPage :many
SELECT * FROM usage_events
ORDER BY timestamp DESC
LIMIT $1 OFFSET $2;
```

**Warning**: Offset pagination degrades linearly. At `OFFSET 99000`, PostgreSQL must scan and discard 99,100 rows. Use cursor-based for large datasets.

| Page | Offset Latency | Cursor Latency |
|------|---------------|----------------|
| 1 | ~50ms | ~50ms |
| 100 | ~500ms | ~50ms |
| 1000 | 5+ seconds | ~50ms |

---

## Aggregation Patterns

### Cost by Agent

```sql
-- name: AggregateCostByAgent :many
SELECT
    agent_id,
    SUM(cost_usd) AS total_cost,
    COUNT(*) AS event_count,
    SUM(token_count) AS total_tokens
FROM usage_events
WHERE timestamp BETWEEN $1 AND $2
GROUP BY agent_id
ORDER BY total_cost DESC;
```

### Cost by Service

```sql
-- name: AggregateCostByService :many
SELECT
    service_name,
    SUM(cost_usd) AS total_cost,
    COUNT(*) AS event_count,
    AVG(duration_ms) AS avg_duration_ms
FROM usage_events
WHERE timestamp BETWEEN $1 AND $2
GROUP BY service_name
ORDER BY total_cost DESC;
```

### Cost by Operation Type

```sql
-- name: AggregateCostByOperation :many
SELECT
    operation_type,
    COUNT(*) AS event_count,
    SUM(cost_usd) AS total_cost,
    AVG(cost_usd) AS avg_cost
FROM usage_events
WHERE agent_id = $1
  AND timestamp BETWEEN $2 AND $3
GROUP BY operation_type
ORDER BY total_cost DESC;
```

### Window Function: Running Total

```sql
SELECT
    timestamp,
    cost_usd,
    SUM(cost_usd) OVER (ORDER BY timestamp) AS running_total
FROM usage_events
WHERE agent_id = $1
ORDER BY timestamp;
```

---

## PostgreSQL-Specific Features

### JSONB Containment

```sql
-- Events with specific metadata
SELECT * FROM usage_events
WHERE metadata @> '{"model": "gpt-4"}'::jsonb;
```

### JSONB Field Extraction

```sql
-- Extract specific field from metadata
SELECT
    metadata->>'model' AS model,
    COUNT(*) AS count
FROM usage_events
WHERE metadata IS NOT NULL
GROUP BY metadata->>'model';
```

### CTE for Complex Analytics

```sql
WITH agent_costs AS (
    SELECT
        agent_id,
        DATE_TRUNC('day', timestamp) AS day,
        SUM(cost_usd) AS daily_cost
    FROM usage_events
    WHERE timestamp > NOW() - INTERVAL '30 days'
    GROUP BY agent_id, DATE_TRUNC('day', timestamp)
)
SELECT
    agent_id,
    AVG(daily_cost) AS avg_daily_cost,
    MAX(daily_cost) AS peak_daily_cost
FROM agent_costs
GROUP BY agent_id;
```

---

## Performance Considerations

1. **Always include `ORDER BY`** — without it, PostgreSQL returns rows in arbitrary order, breaking pagination consistency.
2. **Use cursor-based pagination** for any list endpoint that may exceed 1000 rows.
3. **Index on filter columns** — all common filters should have supporting indexes (see `indexes.md`).
4. **Avoid `SELECT *`** in production queries — select only needed columns to reduce I/O.
5. **Batch inserts** — for high-volume event ingestion, use `COPY` or batch `INSERT` instead of single-row inserts.
