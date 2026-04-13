# usage_events

Schema: `main`

## Columns

| Column | Type | Default | Nullable |
|--------|------|---------|----------|
| `id` | INTEGER | - | YES |
| `agent_id` | TEXT | - | NO |
| `session_id` | TEXT | - | NO |
| `event_type` | TEXT | - | NO |
| `model` | TEXT | - | YES |
| `input_tokens` | INTEGER | - | YES |
| `output_tokens` | INTEGER | - | YES |
| `cost_usd` | REAL | - | YES |
| `duration_ms` | INTEGER | - | YES |
| `metadata` | TEXT | - | YES |
| `created_at` | TEXT | datetime('now') | NO |

## Indexes

| Name | Unique | Definition |
|------|--------|------------|
| `idx_usage_events_created_at` | No | `CREATE INDEX idx_usage_events_created_at ON usage_events (10)` |
| `idx_usage_events_event_type` | No | `CREATE INDEX idx_usage_events_event_type ON usage_events (3)` |
| `idx_usage_events_agent_id` | No | `CREATE INDEX idx_usage_events_agent_id ON usage_events (1)` |

