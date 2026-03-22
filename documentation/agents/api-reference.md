# Agent API Reference

**FSD Requirement**: FR-6.1

---

## Role

This document helps OpenCode agents understand and generate code for the ACE API. It provides machine-parseable endpoint summaries, request/response schemas, and code generation templates.

---

## Project Knowledge

- The ACE API uses Chi router with a standard response envelope: `{ "success": true, "data": {...} }`
- All responses are JSON with `Content-Type: application/json`
- Error responses use: `{ "success": false, "error": { "code": "...", "message": "...", "details": [...] } }`
- The codebase uses `pgx/v5` for PostgreSQL, `sqlc` for type-safe queries, `goose/v3` for migrations
- Currently all endpoints are public (no JWT auth middleware)
- The `agentId` attribute threads through every message, row, span, and log line for attribution

---

## Endpoints

### System

| Method | Path | Purpose | Auth |
|--------|------|---------|------|
| GET | `/` | Server identifier | Public |
| GET | `/metrics` | Prometheus metrics | Public |

### Health

| Method | Path | Purpose | Auth |
|--------|------|---------|------|
| GET | `/health/live` | Liveness probe | Public |
| GET | `/health/ready` | Readiness probe (DB + NATS) | Public |
| GET | `/health/exporters` | Exporter health (OTLP) | Public |

### Examples (Placeholder)

| Method | Path | Purpose | Auth |
|--------|------|---------|------|
| POST | `/examples/` | Create example | Public |
| GET | `/examples/{id}` | Get example by ID | Public |

---

## Commands

```bash
# Run the API server
make up

# Run tests
make test

# Generate documentation
make docs

# Run migrations
goose -dir backend/shared/telemetry/migrations up

# Generate SQLC code
sqlc generate -f backend/services/api/sqlc.yaml
```

---

## Code Style

### Response Pattern

Always use the `response` package for HTTP responses:

```go
response.Success(w, data)        // 200
response.Created(w, data)        // 201
response.BadRequest(w, code, msg) // 400
response.ValidationError(w, err)  // 400 with field details
response.NotFound(w, msg)         // 404
response.InternalError(w, msg)    // 500
```

### Handler Pattern

```go
func (h *Handler) Action(w http.ResponseWriter, r *http.Request) {
    // 1. Decode request
    var req RequestType
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        response.BadRequest(w, "invalid_request", "Invalid request body")
        return
    }

    // 2. Validate
    if err := validator.ValidateStruct(req); err != nil {
        response.ValidationError(w, err)
        return
    }

    // 3. Business logic (service layer)
    result, err := h.service.Method(r.Context(), req)
    if err != nil {
        // Map error type to appropriate response
        response.InternalError(w, "Operation failed")
        return
    }

    // 4. Return response
    response.Success(w, result)
}
```

---

## Boundaries

### ALWAYS

- Use `response.*` helpers for all HTTP responses
- Include `agent_id` in agent-attributed entities
- Validate request bodies with `validator.ValidateStruct`
- Propagate `context.Context` through all layers
- Handle errors (never use `_` for error returns)

### NEVER

- Return raw HTTP responses without the `response` package
- Expose internal error details to clients
- Use `interface{}` or `any` in new types
- Skip validation on incoming data
- Ignore `context.Context` cancellation

---

## Code Generation Templates

### New Endpoint Template

```go
// @Summary      {Action} {resource}
// @Description  {Description}
// @Tags         {tag}
// @Accept       json
// @Produce      json
// @Param        request body {RequestType} true "{Description}"
// @Success      201 {object} response.APIResponse
// @Failure      400 {object} response.APIResponse
// @Router       /{path} [post]
func (h *Handler) Action(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

### Request Type Template

```go
type Create{Resource}Request struct {
    Name  string `json:"name" validate:"required,min=1,max=255"`
    AgentID uuid.UUID `json:"agent_id" validate:"required"`
    // Add fields
}
```

---

## Validation

- Verify endpoint returns correct HTTP status code
- Verify response envelope format (`success`, `data` or `error`)
- Verify validation errors include field-level `details`
- Verify `agent_id` is present in agent-attributed responses
