# provider_groups

Schema: `main`

## Columns

| Column | Type | Default | Nullable |
|--------|------|---------|----------|
| `id` | TEXT | - | YES |
| `name` | TEXT | - | NO |
| `strategy` | TEXT | - | NO |
| `config_json` | TEXT | '{}' | NO |
| `is_default` | INTEGER | 0 | NO |
| `created_at` | TEXT | - | NO |
| `updated_at` | TEXT | - | NO |

## Indexes

| Name | Unique | Definition |
|------|--------|------------|
| `idx_provider_groups_default` | No | `CREATE INDEX idx_provider_groups_default ON provider_groups (4)` |

