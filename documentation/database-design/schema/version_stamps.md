# version_stamps

Schema: `public`

## Columns

| Column | Type | Default | Nullable |
|--------|------|---------|----------|
| `key` | character varying | - | NO |
| `version` | character varying | - | NO |
| `source_hash` | character varying | - | YES |
| `updated_at` | timestamp with time zone | now() | NO |
| `updated_by` | character varying | - | YES |

## Constraints

| Name | Type | Definition |
|------|------|------------|
| `version_stamps_key_not_null` | CHECK | `NOT NULL key` |
| `version_stamps_pkey` | PRIMARY KEY | `PRIMARY KEY (key)` |
| `version_stamps_updated_at_not_null` | CHECK | `NOT NULL updated_at` |
| `version_stamps_version_not_null` | CHECK | `NOT NULL version` |

