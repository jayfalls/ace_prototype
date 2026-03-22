# Index Strategy

**FSD Requirement**: FR-1.3

---

## Overview

This document catalogs all database indexes, their purpose, and performance rationale. Currently only the `usage_events` table has indexes defined.

---

## Index Catalog: usage_events

All indexes are B-tree (PostgreSQL default). No partial, GIN, or GiST indexes are currently defined.

| # | Index Name | Table | Columns | Type | Query Pattern |
|---|-----------|-------|---------|------|---------------|
| 1 | `idx_usage_events_agent_id` | usage_events | `agent_id` | B-tree | Filter events by agent for attribution and billing |
| 2 | `idx_usage_events_cycle_id` | usage_events | `cycle_id` | B-tree | Filter events by cycle for grouping and analysis |
| 3 | `idx_usage_events_session_id` | usage_events | `session_id` | B-tree | Filter events by session for debugging and tracing |
| 4 | `idx_usage_events_timestamp` | usage_events | `timestamp DESC` | B-tree | Time-range queries with newest-first ordering |
| 5 | `idx_usage_events_operation_type` | usage_events | `operation_type` | B-tree | Filter by operation type for analytics |
| 6 | `idx_usage_events_service_name` | usage_events | `service_name` | B-tree | Filter by service for per-service cost tracking |

---

## Index Analysis

### 1. `idx_usage_events_agent_id`

- **Columns**: `agent_id`
- **Type**: B-tree (equality)
- **Supports**: `WHERE agent_id = $1`, `ORDER BY agent_id`, `GROUP BY agent_id`
- **Rationale**: Primary attribution filter — most usage queries filter by agent
- **Usage frequency**: High

### 2. `idx_usage_events_cycle_id`

- **Columns**: `cycle_id`
- **Type**: B-tree (equality)
- **Supports**: `WHERE cycle_id = $1`, `GROUP BY cycle_id`
- **Rationale**: Cycle-level grouping for cost aggregation
- **Usage frequency**: Medium

### 3. `idx_usage_events_session_id`

- **Columns**: `session_id`
- **Type**: B-tree (equality)
- **Supports**: `WHERE session_id = $1`
- **Rationale**: Session-level debugging and tracing
- **Usage frequency**: Medium

### 4. `idx_usage_events_timestamp`

- **Columns**: `timestamp DESC`
- **Type**: B-tree (range, ordered)
- **Supports**: `WHERE timestamp BETWEEN $1 AND $2`, `ORDER BY timestamp DESC`, cursor-based pagination
- **Rationale**: Time-range queries are the primary access pattern for usage data; DESC ordering supports newest-first display
- **Usage frequency**: Very high

### 5. `idx_usage_events_operation_type`

- **Columns**: `operation_type`
- **Type**: B-tree (equality)
- **Supports**: `WHERE operation_type = $1`, `GROUP BY operation_type`
- **Rationale**: Analytics queries grouping by operation type
- **Usage frequency**: Medium

### 6. `idx_usage_events_service_name`

- **Columns**: `service_name`
- **Type**: B-tree (equality)
- **Supports**: `WHERE service_name = $1`, `GROUP BY service_name`
- **Rationale**: Per-service cost tracking and analytics
- **Usage frequency**: Medium

---

## Composite Index Recommendations

The following composite indexes would improve performance for common multi-column queries. Add as query patterns are established:

| Proposed Index | Columns | Rationale |
|---------------|---------|-----------|
| `idx_usage_events_agent_timestamp` | `(agent_id, timestamp DESC)` | Agent-filtered time-range queries (get recent events for agent) |
| `idx_usage_events_agent_op` | `(agent_id, operation_type)` | Agent-filtered operation-type queries |
| `idx_usage_events_cursor` | `(timestamp DESC, id DESC)` | Cursor-based pagination with tie-breaker |

---

## Missing Index Recommendations

| Gap | Recommendation | Priority |
|-----|---------------|----------|
| Cursor pagination | Add `(timestamp DESC, id DESC)` composite for deterministic cursor pagination | High (when pagination is implemented) |
| JSONB metadata queries | Add GIN index on `metadata` if JSONB containment queries are needed | Low (defer until needed) |
| Soft delete filter | Add partial index `WHERE deleted_at IS NULL` if soft delete is added to usage_events | Low (usage events are immutable) |

---

## Index Naming Conventions

All indexes follow the convention defined in `conventions.md`:

- **Format**: `idx_{table}_{columns}`
- **Example**: `idx_usage_events_agent_id` → index on `usage_events.agent_id`
- **Composite**: `idx_{table}_{col1}_{col2}` → composite index
- **Unique**: `uq_{table}_{columns}` → unique constraint index

---

## Notes

- The current schema has 6 single-column B-tree indexes, one per column. As the codebase grows, consider consolidating into composite indexes for the most common query patterns to reduce write amplification.
- PostgreSQL automatically creates an index on the primary key (`usage_events_pkey` on `id`).
- Index maintenance cost: each index adds overhead to INSERT/UPDATE operations. Monitor write performance as table grows.
