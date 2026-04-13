# ott_spans

Schema: `main`

## Columns

| Column | Type | Default | Nullable |
|--------|------|---------|----------|
| `id` | INTEGER | - | YES |
| `trace_id` | TEXT | - | NO |
| `span_id` | TEXT | - | NO |
| `parent_span_id` | TEXT | - | YES |
| `operation_name` | TEXT | - | NO |
| `service_name` | TEXT | - | NO |
| `start_time` | TEXT | - | NO |
| `end_time` | TEXT | - | NO |
| `duration_ms` | INTEGER | - | NO |
| `status` | TEXT | 'ok' | NO |
| `attributes` | TEXT | - | YES |
| `created_at` | TEXT | datetime('now') | NO |

## Indexes

| Name | Unique | Definition |
|------|--------|------------|
| `idx_ott_spans_created_at` | No | `CREATE INDEX idx_ott_spans_created_at ON ott_spans (11)` |
| `idx_ott_spans_service` | No | `CREATE INDEX idx_ott_spans_service ON ott_spans (5)` |
| `idx_ott_spans_trace_id` | No | `CREATE INDEX idx_ott_spans_trace_id ON ott_spans (1)` |

