# Training: Creating a New API Endpoint with Database Access

**FSD Requirement**: FR-6.4

---

## Role

Full workflow for agents creating a new API endpoint that reads/writes to the database: handler, service, repository, SQLC query, and OpenAPI annotation.

---

## Prerequisites

- Understanding of conventions in `documentation/database-design/conventions.md`
- Understanding of SQLC workflow in `documentation/database-design/sqlc.md`
- Understanding of the endpoint-to-DB mapping in `documentation/api/endpoint-map.md`
- Understanding of the Layered Architecture: Handler → Service → Repository → SQLC

---

## Step-by-Step Workflow

### Step 1: Define the SQLC Query

Create or update `backend/services/api/internal/repository/queries/{domain}.sql`:

```sql
-- name: {FunctionName} :one/:many/:exec/:execrows
{SQL QUERY}
```

Annotation types:
- `:one` — returns a single row
- `:many` — returns multiple rows
- `:exec` — no return value (INSERT/UPDATE/DELETE)
- `:execrows` — returns number of affected rows

### Step 2: Generate SQLC Code

```bash
sqlc generate -f backend/services/api/sqlc.yaml
```

### Step 3: Create Repository Method

Create or update `backend/services/api/internal/repository/{domain}_repo.go`:

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

func (r *{Entity}Repo) {Method}(ctx context.Context, params db.{FunctionName}Params) (db.{ReturnType}, error) {
    return r.queries.{FunctionName}(ctx, params)
}
```

### Step 4: Create Service (if business logic needed)

Create `backend/services/api/internal/service/{domain}_service.go`:

```go
package service

import (
    "context"
    "ace/api/internal/repository"
)

type {Entity}Service struct {
    repo *repository.{Entity}Repo
}

func New{Entity}Service(repo *repository.{Entity}Repo) *{Entity}Service {
    return &{Entity}Service{repo: repo}
}

func (s *{Entity}Service) {Method}(ctx context.Context, ...) ({ReturnType}, error) {
    // Business logic here
    return s.repo.{Method}(ctx, ...)
}
```

### Step 5: Create Handler

Create `backend/services/api/internal/handler/{domain}.go`:

```go
package handler

import (
    "encoding/json"
    "net/http"
    "ace/api/internal/response"
    "ace/api/internal/validator"
)

type {Entity}Handler struct {
    service *service.{Entity}Service
}

func New{Entity}Handler(svc *service.{Entity}Service) *{Entity}Handler {
    return &{Entity}Handler{service: svc}
}

func (h *{Entity}Handler) {Action}(w http.ResponseWriter, r *http.Request) {
    // Decode request
    // Validate
    // Call service
    // Return response
}
```

### Step 6: Register Route

In `backend/services/api/cmd/main.go`:

```go
{domain}Repo := repository.New{Entity}Repo(pool)
{domain}Service := service.New{Entity}Service({domain}Repo)
{domain}Handler := handler.New{Entity}Handler({domain}Service)

r.Route("/{domain}s", func(r chi.Router) {
    r.Post("/", {domain}Handler.Create)
    r.Get("/{id}", {domain}Handler.Get)
})
```

### Step 7: Update OpenAPI Annotations

Add Annot8 annotations to the handler for OpenAPI generation:

```go
// @Summary Create {entity}
// @Tags {domain}
// @Accept json
// @Produce json
// @Param request body Create{Entity}Request true "Request"
// @Success 201 {object} response.APIResponse{data={Entity}}
// @Failure 400 {object} response.APIResponse{error=response.APIError}
// @Router /{domain}s [post]
```

### Step 8: Write Tests

Test the full chain:
- Repository: verify SQLC query works against test DB
- Service: verify business logic
- Handler: verify HTTP request/response

### Step 9: Update Documentation

- Update endpoint map: `documentation/api/endpoint-map.md`
- Update schema docs if new columns/tables involved
- Update ERD if relationships changed

---

## Conventions Checklist

Before committing, verify:

- [ ] SQLC annotation syntax correct (`:one`, `:many`, `:exec`, `:execrows`)
- [ ] Repository follows pattern: `repo.{Method}(ctx, params)`
- [ ] Handler uses `response.*` helpers for all responses
- [ ] Request validation via `validator.ValidateStruct`
- [ ] Route registered in `main.go`
- [ ] OpenAPI annotations on handler
- [ ] `agent_id` included in queries for agent-attributed tables
- [ ] Endpoint map documentation updated

---

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Business logic in handler | Move to service layer |
| Skipping service layer for simple CRUD | Always use service layer for consistency |
| Missing validation | Use `validator.ValidateStruct` on all request bodies |
| Not regenerating SQLC | Run `sqlc generate` after every `.sql` change |
| Missing error response codes | Use `response.BadRequest`, `response.NotFound`, `response.InternalError` |
