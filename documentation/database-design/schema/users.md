# users

Schema: `main`

## Columns

| Column | Type | Default | Nullable |
|--------|------|---------|----------|
| `id` | TEXT | - | YES |
| `email` | TEXT | - | NO |
| `password_hash` | TEXT | - | NO |
| `role` | TEXT | 'user' | NO |
| `status` | TEXT | 'pending' | NO |
| `suspended_at` | TEXT | - | YES |
| `suspended_reason` | TEXT | - | YES |
| `deleted_at` | TEXT | - | YES |
| `created_at` | TEXT | datetime('now') | NO |
| `updated_at` | TEXT | datetime('now') | NO |

## Indexes

| Name | Unique | Definition |
|------|--------|------------|
| `idx_users_status` | No | `CREATE INDEX idx_users_status ON users (4)` |
| `idx_users_email` | No | `CREATE INDEX idx_users_email ON users (1)` |

