# provider_group_members

Schema: `main`

## Columns

| Column | Type | Default | Nullable |
|--------|------|---------|----------|
| `id` | TEXT | - | YES |
| `group_id` | TEXT | - | NO |
| `provider_id` | TEXT | - | NO |
| `priority` | INTEGER | 0 | NO |
| `created_at` | TEXT | - | NO |

## Constraints

| Name | Type | Definition |
|------|------|------------|
| `fk_provider_group_members_0` | FOREIGN KEY | `FOREIGN KEY (provider_id) REFERENCES providers(id)` |
| `fk_provider_group_members_1` | FOREIGN KEY | `FOREIGN KEY (group_id) REFERENCES provider_groups(id)` |

## Indexes

| Name | Unique | Definition |
|------|--------|------------|
| `idx_provider_group_members_priority` | No | `CREATE INDEX idx_provider_group_members_priority ON provider_group_members (1, 3)` |
| `idx_provider_group_members_group_id` | No | `CREATE INDEX idx_provider_group_members_group_id ON provider_group_members (1)` |

