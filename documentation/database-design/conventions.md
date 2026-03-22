# Naming Conventions & Data Types

**FSD Requirements**: FR-3.1, FR-3.2

---

## Overview

This document defines the authoritative naming conventions and standard data types for all PostgreSQL objects in the ACE Framework. All new schema objects **MUST** follow these conventions. Legacy exceptions are documented in `legacy-patterns.md`.

---

## Naming Conventions

### General Rules

1. **Lowercase only** — PostgreSQL folds unquoted identifiers to lowercase. Never use PascalCase or camelCase in SQL.
2. **snake_case** — Separate words with underscores (`_`). No hyphens, no spaces.
3. **Letters, numbers, underscores only** — No special characters in identifiers.
4. **No reserved words** — Do not use PostgreSQL reserved keywords as names.

### Tables

| Rule | Convention | Examples |
|------|-----------|----------|
| Format | `snake_case` | `agents`, `memory_nodes`, `tool_invocations` |
| Plural nouns | Tables represent collections | `agents` (not `agent`), `sessions` (not `session`) |
| Entity group prefix | Optional, for disambiguation | `agent_configs`, `memory_summaries` |

### Columns

| Rule | Convention | Examples |
|------|-----------|----------|
| Format | `snake_case` | `created_at`, `user_id`, `is_active` |
| Singular | Columns represent a single value | `name` (not `names`), `status` (not `statuses`) |
| Foreign keys | `{referenced_table}_id` | `agent_id`, `session_id` |
| Timestamps | `{action}_at` | `created_at`, `updated_at`, `deleted_at` |
| Booleans | `is_{adjective}` | `is_active`, `is_deleted`, `is_enabled` |
| Counts | `{noun}_count` | `token_count`, `error_count` |
| Amounts | `{noun}_{unit}` | `cost_usd`, `duration_ms` |

### Indexes

| Rule | Convention | Examples |
|------|-----------|----------|
| Format | `idx_{table}_{columns}` | `idx_agents_user_id`, `idx_usage_events_timestamp` |
| Composite | `idx_{table}_{col1}_{col2}` | `idx_agents_user_id_status` |
| Unique | `uq_{table}_{columns}` | `uq_agents_name`, `uq_users_email` |
| Partial | `idx_{table}_{columns}_where_{condition}` | `idx_agents_active_where_deleted_null` |

### Constraints

| Constraint Type | Convention | Examples |
|----------------|-----------|----------|
| Primary key | `{table}_pkey` (PostgreSQL default) | `usage_events_pkey` |
| Foreign key | `fk_{table}_{column}` | `fk_usage_events_agent_id` |
| Unique | `uq_{table}_{columns}` | `uq_agents_name` |
| Check | `ck_{table}_{column}_{condition}` | `ck_agents_status_valid` |
| Exclusion | `ex_{table}_{columns}` | `ex_reservations_overlap` |

### Triggers

| Rule | Convention | Examples |
|------|-----------|----------|
| Updated at | `set_{table}_updated_at` | `set_agents_updated_at` |
| Audit | `audit_{table}_{action}` | `audit_agents_on_delete` |

### Functions

| Rule | Convention | Examples |
|------|-----------|----------|
| Format | `snake_case`, verb-noun | `update_updated_at`, `calculate_cost` |
| Trigger functions | `trigger_{action}_{table}` | `trigger_update_updated_at` |

---

## Standard Data Types

### Primary Keys

```sql
id UUID PRIMARY KEY DEFAULT gen_random_uuid()
```

- **UUID** — Universally unique, no central coordination needed.
- **gen_random_uuid()** — PostgreSQL 14+ built-in, cryptographically random.
- Do NOT use SERIAL/BIGSERIAL — UUIDs are preferred for distributed systems.

### Timestamps

```sql
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at TIMESTAMPTZ  -- nullable for soft delete
```

- **ALWAYS** use `TIMESTAMPTZ` — never `TIMESTAMP` (without timezone).
- **ALWAYS** use `NOW()` as default for `created_at`/`updated_at`.
- Add `updated_at` trigger to auto-update on row changes.

### Status & Enum-like Fields

```sql
status VARCHAR(50) NOT NULL DEFAULT 'active'
operation_type VARCHAR(50) NOT NULL
```

- **VARCHAR(50)** — sufficient for most status/enum values.
- Use `CHECK` constraints to validate allowed values when the set is fixed.
- Avoid PostgreSQL ENUM types — ALTER TYPE is painful; VARCHAR + CHECK is more flexible.

### Names & Labels

```sql
name VARCHAR(255) NOT NULL
service_name VARCHAR(255) NOT NULL
```

- **VARCHAR(255)** — standard for names, titles, labels.
- Use `NOT NULL` unless the field is truly optional.

### Descriptions & Text

```sql
description TEXT
```

- **TEXT** — unlimited length, no performance difference from VARCHAR in PostgreSQL.

### JSON Data

```sql
metadata JSONB
```

- **JSONB** — binary JSON, indexable with GIN, supports containment operators (`@>`).
- Use JSONB over JSON — JSONB is faster for queries and supports indexing.
- Add `metadata IS NOT NULL` default or `CHECK` constraint if empty objects are undesirable.

### Boolean Flags

```sql
is_active BOOLEAN NOT NULL DEFAULT false
```

- **ALWAYS** provide an explicit `DEFAULT`.
- Prefer `DEFAULT false` over `DEFAULT true` — safer for new features.

### Numeric Amounts

```sql
cost_usd DECIMAL(10, 6)    -- 10 total digits, 6 after decimal
duration_ms BIGINT          -- milliseconds as integer
token_count BIGINT          -- count as integer
```

- **DECIMAL** — exact precision for money. Never use FLOAT/DOUBLE PRECISION for currency.
- **BIGINT** — for counts, durations in milliseconds, byte sizes.
- **INTEGER** — for small counts (< 2.1 billion).

### Foreign Keys

```sql
agent_id UUID NOT NULL
-- with constraint:
CONSTRAINT fk_usage_events_agent_id FOREIGN KEY (agent_id) REFERENCES agents(id)
```

- **UUID** — match the referenced table's primary key type.
- **NOT NULL** — unless the relationship is optional.

---

## PostgreSQL Feature Guidance

### JSONB vs Normalized Tables

| Use JSONB When | Normalize When |
|----------------|----------------|
| Schema varies by record | Schema is stable and well-defined |
| Metadata/tags/attributes | Aggregate queries (SUM, AVG, GROUP BY) are core |
| User preferences/settings | Referential integrity needed (foreign keys) |
| Third-party API responses | Frequent filtering on specific fields |

**Hybrid approach** (recommended): Normalize core fields, use JSONB for flexible attributes.

### Arrays

Use PostgreSQL arrays for simple, fixed-type lists:

```sql
tags TEXT[]
```

- Avoid arrays of complex types — use JSONB or a separate table instead.
- Use `ANY()` for querying array elements.

### CTEs (Common Table Expressions)

Use CTEs for complex multi-step queries:

```sql
WITH active_agents AS (
    SELECT * FROM agents WHERE status = 'active'
)
SELECT * FROM active_agents WHERE user_id = $1;
```

- Improves readability for complex queries.
- PostgreSQL 12+ inlines CTEs that are referenced once — no performance penalty.

### Partial Indexes

Use partial indexes for filtered queries:

```sql
CREATE INDEX idx_agents_active ON agents(user_id) WHERE deleted_at IS NULL;
```

- Smaller index, faster writes, perfect for WHERE clauses that always include the same filter.

### COMMENT ON

Add database-level comments for documentation:

```sql
COMMENT ON TABLE usage_events IS 'Tracks per-operation usage data for billing and attribution';
COMMENT ON COLUMN usage_events.agent_id IS 'References the agent that produced this usage event';
```

---

## Validation

New migrations and schema changes should be validated against these conventions:

1. **Naming**: All identifiers follow snake_case
2. **Primary keys**: UUID with gen_random_uuid() default
3. **Timestamps**: created_at and updated_at with TIMESTAMPTZ
4. **Indexes**: idx_{table}_{columns} naming
5. **Constraints**: {type}_{table}_{columns} naming

CI/CD validation is enforced via `make test` (see `architecture.md` §Makefile Integration).
