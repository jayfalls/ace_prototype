# usage_events

**FSD Requirement**: FR-1.1

---

## Purpose

The `usage_events` table tracks per-operation usage data for billing, cost attribution, and analytics. Each row represents a single API or service operation with associated cost, duration, and token consumption metrics. Events are linked to agents, cycles, and sessions for multi-level attribution.

---

## Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | UUID | NO | `gen_random_uuid()` | PRIMARY KEY |
| `timestamp` | TIMESTAMPTZ | NO | — | — |
| `agent_id` | UUID | NO | — | — |
| `cycle_id` | UUID | NO | — | — |
| `session_id` | UUID | NO | — | — |
| `service_name` | VARCHAR(255) | NO | — | — |
| `operation_type` | VARCHAR(50) | NO | — | — |
| `resource_type` | VARCHAR(50) | NO | — | — |
| `cost_usd` | DECIMAL(10,6) | YES | — | — |
| `duration_ms` | BIGINT | YES | — | — |
| `token_count` | BIGINT | YES | — | — |
| `metadata` | JSONB | YES | — | — |
| `created_at` | TIMESTAMPTZ | YES | `NOW()` | — |

---

## Primary Key

| Name | Type | Column | Generation |
|------|------|--------|------------|
| `usage_events_pkey` | PRIMARY KEY | `id` | `gen_random_uuid()` (UUID v4) |

---

## Indexes

| Index Name | Columns | Type | Purpose |
|------------|---------|------|---------|
| `idx_usage_events_agent_id` | `agent_id` | B-tree | Filter events by agent for attribution |
| `idx_usage_events_cycle_id` | `cycle_id` | B-tree | Filter events by cycle for grouping |
| `idx_usage_events_session_id` | `session_id` | B-tree | Filter events by session |
| `idx_usage_events_timestamp` | `timestamp DESC` | B-tree | Time-range queries (newest first) |
| `idx_usage_events_operation_type` | `operation_type` | B-tree | Filter by operation type |
| `idx_usage_events_service_name` | `service_name` | B-tree | Filter by service name |

See `indexes.md` for detailed index analysis.

---

## Relationships

| Column | References | Relationship |
|--------|-----------|--------------|
| `agent_id` | `agents(id)` (future) | Many usage_events belong to one agent |

**Note**: Foreign key constraints on `agent_id`, `cycle_id`, and `session_id` are not yet defined in the current migration. They should be added when the referenced tables (`agents`, `cycles`, `sessions`) are created.

---

## Triggers

None currently defined. When the `update_updated_at()` trigger function is created, a trigger should be added:

```sql
CREATE TRIGGER set_usage_events_updated_at
    BEFORE UPDATE ON usage_events
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();
```

Note: The current `created_at` column lacks an `updated_at` companion. Future migrations should add `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` and the corresponding trigger.

---

## SQLC Queries

No SQLC queries are currently defined for this table. When created, they should be placed in:

```
backend/services/api/internal/repository/queries/usage.sql
```

Expected queries:
- `CreateUsageEvent :one` — Insert a new usage event
- `GetUsageEventByID :one` — Retrieve a single event by ID
- `ListUsageEventsByAgentID :many` — List events for a specific agent
- `ListUsageEventsByTimeRange :many` — List events within a time window
- `AggregateUsageByAgent :many` — Aggregate costs/tokens by agent

---

## Migration Source

**File**: `backend/shared/telemetry/migrations/20260321000000_create_usage_events.go`

Created in migration `20260321000000_create_usage_events` using Goose v3 Go function pattern.
