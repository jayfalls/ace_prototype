# resource_permissions

Schema: `main`

## Columns

| Column | Type | Default | Nullable |
|--------|------|---------|----------|
| `id` | TEXT | - | YES |
| `user_id` | TEXT | - | NO |
| `resource_type` | TEXT | - | NO |
| `resource_id` | TEXT | - | NO |
| `permission_level` | TEXT | - | NO |
| `granted_by` | TEXT | - | YES |
| `created_at` | TEXT | datetime('now') | NO |

## Constraints

| Name | Type | Definition |
|------|------|------------|
| `fk_resource_permissions_0` | FOREIGN KEY | `FOREIGN KEY (user_id) REFERENCES users(id)` |

## Indexes

| Name | Unique | Definition |
|------|--------|------------|
| `idx_resource_permissions_resource` | No | `CREATE INDEX idx_resource_permissions_resource ON resource_permissions (2, 3)` |
| `idx_resource_permissions_user_id` | No | `CREATE INDEX idx_resource_permissions_user_id ON resource_permissions (1)` |

