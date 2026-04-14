# users

Schema: `main`

## Columns

| Column | Type | Default | Nullable |
|--------|------|---------|----------|
| `id` | TEXT | - | YES |
| `username` | TEXT | - | NO |
| `password_hash` | TEXT | - | NO |
| `pin_hash` | TEXT | - | YES |
| `role` | TEXT | 'user' | NO |
| `status` | TEXT | 'pending' | NO |
| `suspended_at` | TEXT | - | YES |
| `suspended_reason` | TEXT | - | YES |
| `deleted_at` | TEXT | - | YES |
| `created_at` | TEXT | - | NO |
| `updated_at` | TEXT | - | NO |

## Indexes

| Name | Unique | Definition |
|------|--------|------------|
| `idx_users_username` | No | `CREATE INDEX idx_users_username ON users (1)` |

