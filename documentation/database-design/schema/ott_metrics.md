# ott_metrics

Schema: `main`

## Columns

| Column | Type | Default | Nullable |
|--------|------|---------|----------|
| `id` | INTEGER | - | YES |
| `name` | TEXT | - | NO |
| `type` | TEXT | 'counter' | NO |
| `labels` | TEXT | - | YES |
| `value` | REAL | - | NO |
| `timestamp` | TEXT | - | NO |
| `created_at` | TEXT | - | NO |

## Indexes

| Name | Unique | Definition |
|------|--------|------------|
| `idx_ott_metrics_created_at` | No | `CREATE INDEX idx_ott_metrics_created_at ON ott_metrics (6)` |
| `idx_ott_metrics_name` | No | `CREATE INDEX idx_ott_metrics_name ON ott_metrics (1)` |

