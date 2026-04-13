# auth_tokens

Schema: `main`

## Columns

| Column | Type | Default | Nullable |
|--------|------|---------|----------|
| `id` | TEXT | - | YES |
| `user_id` | TEXT | - | NO |
| `token_type` | TEXT | - | NO |
| `token_hash` | TEXT | - | NO |
| `expires_at` | TEXT | - | NO |
| `used_at` | TEXT | - | YES |
| `created_at` | TEXT | datetime('now') | NO |

## Constraints

| Name | Type | Definition |
|------|------|------------|
| `fk_auth_tokens_0` | FOREIGN KEY | `FOREIGN KEY (user_id) REFERENCES users(id)` |

## Indexes

| Name | Unique | Definition |
|------|--------|------------|
| `idx_auth_tokens_expires_at` | No | `CREATE INDEX idx_auth_tokens_expires_at ON auth_tokens (4)` |
| `idx_auth_tokens_token_hash` | No | `CREATE INDEX idx_auth_tokens_token_hash ON auth_tokens (3)` |
| `idx_auth_tokens_user_id` | No | `CREATE INDEX idx_auth_tokens_user_id ON auth_tokens (1)` |

