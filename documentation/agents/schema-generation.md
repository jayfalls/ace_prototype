# Schema-Aware Code Generation

**FSD Requirement**: FR-6.2

---

## Role

This document helps OpenCode agents generate database code with schema awareness. It covers Goose migrations, SQLC query generation, and the agentId attribution pattern.

---

## Project Knowledge

- Schema source of truth: Goose migrations in `backend/shared/telemetry/migrations/`
- SQLC config: `backend/services/api/sqlc.yaml` (engine: postgresql, driver: pgx/v5)
- Current tables: `usage_events` (one table, will expand)
- The `agentId` attribute must be included in all agent-attributed tables

---

## Commands

```bash
# Create a new migration
goose -dir backend/shared/telemetry/migrations create {description} go

# Generate SQLC code
sqlc generate -f backend/services/api/sqlc.yaml

# Run migrations locally
goose -dir backend/shared/telemetry/migrations up

# Rollback last migration (dev only)
goose -dir backend/shared/telemetry/migrations down

# Check migration status
goose -dir backend/shared/telemetry/migrations status
```

---

## Migration Generation Template

When creating a new table migration:

```go
// backend/shared/telemetry/migrations/{YYYYMMDDHHMMSS}_{description}.go
package migrations

import (
    "database/sql"
    "github.com/pressly/goose/v3"
)

func init() {
    goose.AddMigration(up{Description}, down{Description})
}

func up{Description}(tx *sql.Tx) error {
    _, err := tx.Exec(`
        CREATE TABLE {table_name} (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            agent_id UUID NOT NULL,
            -- columns --
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );

        CREATE INDEX idx_{table_name}_agent_id ON {table_name}(agent_id);

        CREATE TRIGGER set_{table_name}_updated_at
            BEFORE UPDATE ON {table_name}
            FOR EACH ROW
            EXECUTE FUNCTION update_updated_at();
    `)
    return err
}

func down{Description}(tx *sql.Tx) error {
    _, err := tx.Exec("DROP TABLE IF EXISTS {table_name}")
    return err
}
```

### agentId Inclusion Rule

ALWAYS include `agent_id UUID NOT NULL` in tables that represent agent-owned or agent-produced data:
- Usage tracking tables
- Memory tables
- Tool invocation tables
- Session tables

ALWAYS create an index: `CREATE INDEX idx_{table}_agent_id ON {table}(agent_id);`

---

## SQLC Query Template

When creating SQLC queries for a new table:

```sql
-- backend/services/api/internal/repository/queries/{table}.sql

-- name: Create{Entity} :one
INSERT INTO {table} (agent_id, {columns})
VALUES ($1, {$2...})
RETURNING *;

-- name: Get{Entity}ByID :one
SELECT * FROM {table} WHERE id = $1;

-- name: List{Entities}ByAgentID :many
SELECT * FROM {table}
WHERE agent_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: Update{Entity} :execrows
UPDATE {table} SET {columns} = ${n}, updated_at = NOW()
WHERE id = $1;

-- name: Delete{Entity} :exec
DELETE FROM {table} WHERE id = $1;
```

After creating the `.sql` file, run: `sqlc generate -f backend/services/api/sqlc.yaml`

---

## AgentId Attribution Rules

agentId is the foundation of attribution in the ACE Framework:

1. **Always include `agent_id`** in tables that represent agent activity
2. **Always create a foreign key** to `agents(id)` when the `agents` table exists
3. **Always create an index** on `agent_id` for query performance
4. **Always include `agent_id` in SQLC queries** that filter by agent
5. **Document the attribution purpose** in migration comments

The `agent_id` enables:
- Cost attribution per agent
- Layer Inspector tracing
- Swarm debugging
- Billing breakdown by agent

---

## Code Style

### Go Migration Functions

- Function names: `up{Description}`, `down{Description}` (descriptive, not bare `up`/`down`)
- File names: `{YYYYMMDDHHMMSS}_{description}.go` (timestamp prefix)
- Package: `migrations`
- Registration: `init()` → `goose.AddMigration(up, down)`

### SQLC Queries

- One `.sql` file per domain entity
- Annotations: `-- name: FunctionName :one/:many/:exec/:execrows`
- Positional parameters: `$1`, `$2`, etc.
- Always use `RETURNING *` for INSERT/UPDATE

---

## Boundaries

### ALWAYS

- Include `agent_id UUID NOT NULL` in agent-attributed tables
- Create `idx_{table}_agent_id` index
- Use `gen_random_uuid()` for primary keys
- Include `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` in all tables
- Include `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` in mutable tables
- Create `set_{table}_updated_at` trigger for mutable tables
- Use snake_case for all identifiers

### NEVER

- Hand-edit files in `internal/repository/generated/`
- Use `CREATE TABLE IF NOT EXISTS` (Goose manages existence)
- Use `timestamp without time zone` (always `TIMESTAMPTZ`)
- Use bare `up`/`down` function names
- Use numeric filename prefixes (`001_`)
- Skip the `down` function in migrations

---

## Validation

After generating code, verify:
- [ ] Migration file follows naming convention
- [ ] `down` function properly reverses `up`
- [ ] `agent_id` column included with index
- [ ] SQLC queries use correct annotation syntax
- [ ] Generated code compiles: `go build ./...`
- [ ] Tests pass: `make test`
