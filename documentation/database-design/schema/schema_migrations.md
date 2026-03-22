# schema_migrations

Schema: `public`

## Columns

| Column | Type | Default | Nullable |
|--------|------|---------|----------|
| `id` | integer | - | NO |
| `version_id` | bigint | - | NO |
| `is_applied` | boolean | - | NO |
| `tstamp` | timestamp without time zone | now() | NO |

## Constraints

| Name | Type | Definition |
|------|------|------------|
| `schema_migrations_id_not_null` | CHECK | `NOT NULL id` |
| `schema_migrations_is_applied_not_null` | CHECK | `NOT NULL is_applied` |
| `schema_migrations_pkey` | PRIMARY KEY | `PRIMARY KEY (id)` |
| `schema_migrations_tstamp_not_null` | CHECK | `NOT NULL tstamp` |
| `schema_migrations_version_id_not_null` | CHECK | `NOT NULL version_id` |

