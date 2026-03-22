# SQLC Workflow

**FSD Requirement**: FR-2.3

---

## Overview

SQLC generates type-safe Go code from SQL queries. This document covers the configuration, query organization, annotation syntax, and generation workflow for the ACE Framework.

---

## Configuration Reference

Source: `backend/services/api/sqlc.yaml`

| Setting | Value | Description |
|---------|-------|-------------|
| `version` | `"2"` | SQLC config version |
| `engine` | `postgresql` | Database engine — PostgreSQL-specific features supported |
| `queries` | `internal/repository/queries` | Directory containing `.sql` query files |
| `schema` | `migrations` | Schema source — Goose migration directory |
| `gen.go.package` | `db` | Go package name for generated code |
| `gen.go.out` | `internal/repository/generated` | Output directory for generated Go files |
| `gen.go.sql_package` | `pgx/v5` | SQL driver — PostgreSQL-native pgx |
| `gen.go.emit_interface` | `false` | Do not generate `Querier` interface |
| `gen.go.emit_exact_table_names` | `false` | Use singularized table names for structs |
| `gen.go.emit_empty_slices` | `false` | Return `nil` instead of empty slices |
| `gen.go.emit_exported_queries` | `false` | Do not export query functions |
| `gen.go.emit_json_tags` | `true` | Generate JSON struct tags |
| `gen.go.emit_result_struct_pointers` | `true` | Nullable fields use `*Type` pointers |
| `gen.go.emit_params_struct_pointers` | `false` | Parameter structs use value types |
| `gen.go.json_tags_case_style` | `snake` | JSON tags use `snake_case` |
| `gen.go.output_db_file_name` | `db.go` | Generated DB connection file |
| `gen.go.output_models_file_name` | `models.go` | Generated model structs file |
| `gen.go.output_querier_file_name` | `querier.go` | Generated querier file |

---

## Query Organization

Queries are organized **one file per domain entity**:

```
backend/services/api/internal/repository/queries/
├── agents.sql          # Agent CRUD queries (planned)
├── memory.sql          # Memory node queries (planned)
├── tools.sql           # Tool invocation queries (planned)
├── usage.sql           # Usage event queries (planned)
└── system.sql          # System/pod queries (planned)
```

**Currently**: No `.sql` query files exist. They are created as the codebase grows.

Each `.sql` file contains all SQLC-annotated queries for that domain entity.

---

## Annotation Syntax

### Basic Pattern

```sql
-- name: FunctionName :return_type
SQL QUERY $1, $2, ...;
```

### Return Types

| Type | Description | Example |
|------|-------------|---------|
| `:one` | Returns single row | `GetAgentByID` |
| `:many` | Returns multiple rows | `ListAgents` |
| `:exec` | Executes without return | `DeleteAgent` |
| `:execrows` | Executes, returns affected row count | `UpdateAgentStatus` |

### Examples

#### Single Row Return (`:one`)

```sql
-- name: GetAgentByID :one
SELECT * FROM agents
WHERE id = $1 AND deleted_at IS NULL;
```

Generates:
```go
func (q *Queries) GetAgentByID(ctx context.Context, id uuid.UUID) (*Agent, error)
```

#### Multiple Rows Return (`:many`)

```sql
-- name: ListAgentsByUserID :many
SELECT * FROM agents
WHERE user_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
```

Generates:
```go
func (q *Queries) ListAgentsByUserID(ctx context.Context, arg ListAgentsByUserIDParams) ([]*Agent, error)
```

#### Execute Without Return (`:exec`)

```sql
-- name: DeleteAgent :exec
UPDATE agents SET deleted_at = NOW() WHERE id = $1;
```

Generates:
```go
func (q *Queries) DeleteAgent(ctx context.Context, id uuid.UUID) error
```

#### Execute With Row Count (`:execrows`)

```sql
-- name: UpdateAgentStatus :execrows
UPDATE agents SET status = $2, updated_at = NOW() WHERE id = $1;
```

Generates:
```go
func (q *Queries) UpdateAgentStatus(ctx context.Context, arg UpdateAgentStatusParams) (int64, error)
```

### Named Parameters

SQLC uses positional parameters (`$1`, `$2`, etc.). Named Go parameters are derived from the SQLC config or inferred from column names in the generated struct:

```sql
-- Parameters: $1 = id (uuid), $2 = status (string)
-- name: UpdateAgentStatus :execrows
UPDATE agents SET status = $2, updated_at = NOW() WHERE id = $1;
```

Generated params struct:
```go
type UpdateAgentStatusParams struct {
    ID     uuid.UUID `json:"id"`
    Status string    `json:"status"`
}
```

### RETURNING Clause

Use `RETURNING *` or `RETURNING column_list` to get the inserted/updated row back:

```sql
-- name: CreateAgent :one
INSERT INTO agents (user_id, name, status)
VALUES ($1, $2, $3)
RETURNING *;
```

---

## Generation Workflow

```
1.  Edit or create .sql query file
    in internal/repository/queries/

2.  Run: sqlc generate

3.  Generated Go code appears in
    internal/repository/generated/

4.  Use generated types and functions
    in repository layer

5.  NEVER hand-edit files in
    internal/repository/generated/
```

### Commands

| Command | Purpose | When |
|---------|---------|------|
| `sqlc generate` | Generate Go code from SQL | After editing `.sql` files |
| `sqlc diff` | Show diff between current and generated | CI/CD verification |
| `sqlc verify` | Verify generated code matches SQL | Pre-commit check |
| `sqlc init` | Create `sqlc.yaml` template | New project setup |

---

## Generated Files

| File | Contents | Hand-Editable |
|------|----------|---------------|
| `db.go` | `DBTX` interface, `Queries` struct, `New()`, `WithTx()` | **No** |
| `models.go` | Go structs for each table | **No** |
| `querier.go` | Contains `DBTX` interface only (when `emit_interface: false`) | **No** |
| `*.sql.go` | One file per `.sql` query file with function implementations | **No** |

### emit_interface Behavior

With `emit_interface: false` (current setting):
- `querier.go` contains only the `DBTX` interface (for dependency injection)
- No `Querier` interface is generated
- Repository code uses `*Queries` struct directly

With `emit_interface: true` (if changed):
- `querier.go` contains a `Querier` interface abstracting all query methods
- Useful for mocking in unit tests
- Repository code can accept the `Querier` interface instead of `*Queries`

---

## Common Patterns

### Named Queries

Group related queries in the same `.sql` file:

```sql
-- Agent CRUD
-- name: GetAgentByID :one
-- name: ListAgentsByUserID :many
-- name: CreateAgent :one
-- name: UpdateAgent :execrows
-- name: DeleteAgent :exec
```

### JSONB Handling

SQLC generates `json.RawMessage` for JSONB columns:

```sql
-- name: CreateUsageEvent :one
INSERT INTO usage_events (metadata) VALUES ($1) RETURNING *;
```

Go side:
```go
metadata, _ := json.Marshal(map[string]string{"model": "gpt-4"})
event, err := queries.CreateUsageEvent(ctx, metadata)
```

### Transaction Support

Use `WithTx()` to run queries within a transaction:

```go
tx, err := db.BeginTx(ctx, nil)
if err != nil { return err }
defer tx.Rollback()

qtx := queries.WithTx(tx)
// run queries with qtx...
err = tx.Commit()
```

### Upsert (INSERT ... ON CONFLICT)

```sql
-- name: UpsertAgent :one
INSERT INTO agents (user_id, name, status)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, name) DO UPDATE
SET status = EXCLUDED.status, updated_at = NOW()
RETURNING *;
```

---

## CI/CD Integration

Add `sqlc diff` to CI pipeline to detect generated code drift:

```yaml
- name: Verify SQLC
  run: sqlc diff
  # Fails if .sql files produce different output than committed generated code
```

---

## Constraints

1. **NEVER** hand-edit files in `internal/repository/generated/`
2. **ALWAYS** run `sqlc generate` after modifying `.sql` query files
3. **ALWAYS** commit generated files alongside `.sql` changes
4. **ONE** query file per domain entity
5. **ANNOTATIONS** must use correct return type (`:one`, `:many`, `:exec`, `:execrows`)
