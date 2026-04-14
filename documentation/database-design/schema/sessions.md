# sessions

Schema: `main`

## Columns

| Column | Type | Default | Nullable |
|--------|------|---------|----------|
| `id` | TEXT | - | YES |
| `user_id` | TEXT | - | NO |
| `refresh_token_hash` | TEXT | - | NO |
| `user_agent` | TEXT | - | YES |
| `ip_address` | TEXT | - | YES |
| `last_used_at` | TEXT | - | NO |
| `expires_at` | TEXT | - | NO |
| `created_at` | TEXT | - | NO |

## Constraints

| Name | Type | Definition |
|------|------|------------|
| `fk_sessions_0` | FOREIGN KEY | `FOREIGN KEY (user_id) REFERENCES users(id)` |

## Indexes

| Name | Unique | Definition |
|------|--------|------------|
| `idx_sessions_expires_at` | No | `CREATE INDEX idx_sessions_expires_at ON sessions (6)` |
| `idx_sessions_user_id` | No | `CREATE INDEX idx_sessions_user_id ON sessions (1)` |

