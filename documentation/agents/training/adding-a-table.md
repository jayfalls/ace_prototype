# Training: Adding a New Table

**FSD Requirement**: FR-6.4

---

## Role

Step-by-step guide for agents creating a new table in the ACE Framework. Follow this workflow from migration to handler to test.

---

## Prerequisites

- Understanding of conventions in `documentation/database-design/conventions.md`
- Understanding of SQLC workflow in `documentation/database-design/sqlc.md`
- Understanding of migration patterns in `documentation/database-design/migrations.md`

---

## Step-by-Step Workflow

### Step 1: Create the Migration

```bash
# Generate timestamp
TIMESTAMP=$(date +%Y%m%d%H%M%S)
echo $TIMESTAMP

# Create migration file
goose -dir backend/shared/telemetry/migrations create create_{table_name} go
```

Or create manually:

```
backend/shared/telemetry/migrations/{YYYYMMDDHHMMSS}_create_{table_name}.go
```

### Step 2: Implement the Migration

```go
package migrations

import (
    "database/sql"
    "github.com/pressly/goose/v3"
)

func init() {
    goose.AddMigration(upCreate{TableName}, downCreate{TableName})
}

func upCreate{TableName}(tx *sql.Tx) error {
    _, err := tx.Exec(`
        CREATE TABLE {table_name} (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            agent_id UUID NOT NULL,
            -- Add columns here --
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );

        CREATE INDEX idx_{table_name}_agent_id ON {table_name}(agent_id);
        -- Add more indexes as needed --

        CREATE TRIGGER set_{table_name}_updated_at
            BEFORE UPDATE ON {table_name}
            FOR EACH ROW
            EXECUTE FUNCTION update_updated_at();
    `)
    return err
}

func downCreate{TableName}(tx *sql.Tx) error {
    _, err := tx.Exec("DROP TABLE IF EXISTS {table_name}")
    return err
}
```

### Step 3: Create SQLC Queries

Create `backend/services/api/internal/repository/queries/{table_name}.sql`:

```sql
-- name: Create{Entity} :one
INSERT INTO {table_name} (agent_id, {columns})
VALUES ($1, {$2...})
RETURNING *;

-- name: Get{Entity}ByID :one
SELECT * FROM {table_name} WHERE id = $1;

-- name: List{Entities}ByAgentID :many
SELECT * FROM {table_name}
WHERE agent_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: Update{Entity} :execrows
UPDATE {table_name} SET {columns} = ${n}, updated_at = NOW()
WHERE id = $1;

-- name: Delete{Entity} :exec
DELETE FROM {table_name} WHERE id = $1;
```

### Step 4: Generate SQLC Code

```bash
sqlc generate -f backend/services/api/sqlc.yaml
```

### Step 5: Create Repository

Create `backend/services/api/internal/repository/{table_name}_repo.go`:

```go
package repository

import (
    "context"
    "github.com/jackc/pgx/v5/pgxpool"
    "ace/api/internal/repository/generated"
)

type {Entity}Repo struct {
    pool    *pgxpool.Pool
    queries *db.Queries
}

func New{Entity}Repo(pool *pgxpool.Pool) *{Entity}Repo {
    return &{Entity}Repo{
        pool:    pool,
        queries: db.New(pool),
    }
}

func (r *{Entity}Repo) Create(ctx context.Context, params db.Create{Entity}Params) (*db.{Entity}, error) {
    return r.queries.Create{Entity}(ctx, params)
}
```

### Step 6: Create Handler

Create `backend/services/api/internal/handler/{table_name}.go`:

```go
package handler

import (
    "encoding/json"
    "net/http"
    "ace/api/internal/response"
    "ace/api/internal/validator"
)

type {Entity}Handler struct {
    repo *repository.{Entity}Repo
}

func New{Entity}Handler(repo *repository.{Entity}Repo) *{Entity}Handler {
    return &{Entity}Handler{repo: repo}
}

func (h *{Entity}Handler) Create(w http.ResponseWriter, r *http.Request) {
    var req Create{Entity}Request
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        response.BadRequest(w, "invalid_request", "Invalid request body")
        return
    }

    if err := validator.ValidateStruct(req); err != nil {
        response.ValidationError(w, err)
        return
    }

    result, err := h.repo.Create(r.Context(), db.Create{Entity}Params{
        AgentID: req.AgentID,
        // Map fields --
    })
    if err != nil {
        response.InternalError(w, "Failed to create {entity}")
        return
    }

    response.Created(w, result)
}
```

### Step 7: Register Route

In `backend/services/api/cmd/main.go`:

```go
{table}Repo := repository.New{Entity}Repo(pool)
{table}Handler := handler.New{Entity}Handler({table}Repo)

r.Route("/{table_name}s", func(r chi.Router) {
    r.Post("/", {table}Handler.Create)
    r.Get("/{id}", {table}Handler.Get)
})
```

### Step 8: Write Tests

Create tests for the migration, repository, and handler. Verify:
- Migration up/down roundtrip works
- CRUD operations succeed
- Validation errors return 400 with field details
- Not found returns 404

### Step 9: Update Documentation

- Add schema doc: `documentation/database-design/schema/{group}/{table_name}.md`
- Update ERD: `documentation/database-design/erd/{group}.md`
- Update index catalog: `documentation/database-design/indexes.md`
- Update endpoint map: `documentation/api/endpoint-map.md`

---

## Conventions Checklist

Before committing, verify:

- [ ] Table name is `snake_case`, plural
- [ ] Columns are `snake_case`, singular
- [ ] Primary key is `UUID DEFAULT gen_random_uuid()`
- [ ] `agent_id UUID NOT NULL` included (if agent-attributed)
- [ ] `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` included
- [ ] `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` included (if mutable)
- [ ] `set_{table}_updated_at` trigger created (if mutable)
- [ ] `idx_{table}_agent_id` index created (if agent-attributed)
- [ ] Migration has descriptive `up`/`down` function names
- [ ] Migration file uses timestamp prefix
- [ ] SQLC queries use correct annotations
- [ ] SQLC code generated and committed
- [ ] Documentation files created/updated

---

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Using `CREATE TABLE IF NOT EXISTS` | Use `CREATE TABLE` (Goose manages existence) |
| Bare `up`/`down` function names | Use `upCreate{TableName}`/`downCreate{TableName}` |
| Missing `down` function | Always implement `down` |
| Missing `agent_id` column | Include for agent-attributed tables |
| Forgetting `sqlc generate` | Run after every `.sql` change |
| Hand-editing generated files | Never — edit `.sql` and regenerate |
