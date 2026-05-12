# providers

Schema: `main`

## Columns

| Column | Type | Default | Nullable |
|--------|------|---------|----------|
| `id` | TEXT | - | YES |
| `name` | TEXT | - | NO |
| `provider_type` | TEXT | - | NO |
| `base_url` | TEXT | - | NO |
| `encrypted_api_key` | BLOB | - | NO |
| `api_key_nonce` | BLOB | - | NO |
| `encrypted_dek` | BLOB | - | NO |
| `dek_nonce` | BLOB | - | NO |
| `encryption_version` | INTEGER | 1 | NO |
| `config_json` | TEXT | '{}' | NO |
| `is_enabled` | INTEGER | 1 | NO |
| `created_at` | TEXT | - | NO |
| `updated_at` | TEXT | - | NO |

## Indexes

| Name | Unique | Definition |
|------|--------|------------|
| `idx_providers_enabled` | No | `CREATE INDEX idx_providers_enabled ON providers (10)` |
| `idx_providers_type` | No | `CREATE INDEX idx_providers_type ON providers (2)` |

