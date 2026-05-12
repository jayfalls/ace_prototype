# provider_models

Schema: `main`

## Columns

| Column | Type | Default | Nullable |
|--------|------|---------|----------|
| `id` | TEXT | - | YES |
| `provider_id` | TEXT | - | NO |
| `model_id` | TEXT | - | NO |
| `display_name` | TEXT | - | NO |
| `context_limit` | INTEGER | - | YES |
| `features_json` | TEXT | '{}' | NO |
| `pricing_json` | TEXT | '{}' | NO |
| `parameters_json` | TEXT | '{}' | NO |
| `is_user_edited` | INTEGER | 0 | NO |
| `created_at` | TEXT | - | NO |
| `updated_at` | TEXT | - | NO |

## Constraints

| Name | Type | Definition |
|------|------|------------|
| `fk_provider_models_0` | FOREIGN KEY | `FOREIGN KEY (provider_id) REFERENCES providers(id)` |

## Indexes

| Name | Unique | Definition |
|------|--------|------------|
| `idx_provider_models_provider_id` | No | `CREATE INDEX idx_provider_models_provider_id ON provider_models (1)` |

