# Endpoint-to-Database Mapping

**FSD Requirement**: FR-2.2

---

## Overview

This document maps each API endpoint to its database operations, tracing the call chain: Handler → Service → Repository → SQLC.

---

## Call Chain Pattern

Every endpoint follows this pattern:

```
Client → HTTP Request → Handler → Service → Repository → SQLC Generated → PostgreSQL
                                                              ↓
Client ← HTTP Response ← Handler ← Service ← Repository ← Typed Go Structs
```

Currently, most endpoints are inline handlers without service/repository layers. As the codebase grows, the full pattern will be adopted.

---

## Endpoint Mapping Table

### Current Endpoints

| Method | Path | Handler | Service | Repository | SQLC Query | Transaction | Notes |
|--------|------|---------|---------|------------|------------|-------------|-------|
| GET | `/` | inline | — | — | — | No | Returns server identifier |
| GET | `/health/live` | `HealthHandler.Live` | — | — | — | No | Always returns 200 |
| GET | `/health/ready` | `HealthHandler.Ready` | — | — | — | No | Direct `pool.Ping()` + NATS check |
| GET | `/health/exporters` | `HealthHandler.Exporters` | — | — | — | No | OTLP health check |
| GET | `/metrics` | `telemetry.RegisterMetrics()` | — | — | — | No | Prometheus metrics endpoint |
| POST | `/examples/` | `ExampleHandler.Create` | — | — | — | No | Validation demo, no DB |
| GET | `/examples/{id}` | `ExampleHandler.Get` | — | — | — | No | Returns hardcoded response |

### Planned Endpoints (Future)

| Method | Path | Handler | Service | Repository | SQLC Query | Transaction |
|--------|------|---------|---------|------------|------------|-------------|
| POST | `/agents/` | `AgentHandler.Create` | `AgentService.Create` | `AgentRepo.Create` | `CreateAgent :one` | Yes |
| GET | `/agents/{id}` | `AgentHandler.Get` | `AgentService.GetByID` | `AgentRepo.GetByID` | `GetAgentByID :one` | No |
| GET | `/agents/` | `AgentHandler.List` | `AgentService.List` | `AgentRepo.List` | `ListAgentsByUserID :many` | No |
| POST | `/usage/` | `UsageHandler.Create` | `UsageService.Create` | `UsageRepo.Create` | `CreateUsageEvent :one` | No |
| GET | `/usage/` | `UsageHandler.List` | `UsageService.List` | `UsageRepo.List` | `ListUsageEventsByAgentID :many` | No |

---

## Detailed Mappings

### GET /health/ready

**Handler**: `HealthHandler.Ready` in `backend/services/api/internal/handler/health.go`

```
Handler
  ├─ h.pool.Ping(ctx)           → Direct DB connectivity check
  ├─ h.nats.HealthCheck()       → NATS connectivity check
  └─ response.JSON(w, ...)      → Return check results
```

**Database operations**: `pool.Ping()` — no SQLC queries, direct connection check.

### POST /examples/

**Handler**: `ExampleHandler.Create` in `backend/services/api/internal/handler/example.go`

```
Handler
  ├─ Decode JSON body           → CreateExampleRequest
  ├─ validator.ValidateStruct() → Validate request fields
  ├─ Build ExampleResponse      → Placeholder (no DB)
  └─ response.Created(w, ...)   → Return 201
```

**Database operations**: None — placeholder endpoint demonstrating validation pattern.

### GET /examples/{id}

**Handler**: `ExampleHandler.Get` in `backend/services/api/internal/handler/example.go`

```
Handler
  ├─ Build ExampleResponse      → Hardcoded placeholder
  └─ response.Success(w, ...)   → Return 200
```

**Database operations**: None — placeholder endpoint.

---

## Data Transformation Flow

### Request Path (for future endpoints)

```
Request JSON
  → Go struct (handler: json.Decode)
    → Validated struct (validator.ValidateStruct)
      → Domain type (service layer)
        → SQLC params (repository layer)
          → SQL parameters (SQLC generated)
```

### Response Path

```
SQL result rows
  → SQLC model (generated)
    → Domain type (repository layer)
      → Response type (service layer)
        → JSON envelope (response.Success / response.Created)
```

---

## SQLC File References

| SQLC File | Generated File | Status |
|-----------|---------------|--------|
| `internal/repository/queries/usage.sql` | `internal/repository/generated/usage.sql.go` | Planned |
| `internal/repository/queries/agents.sql` | `internal/repository/generated/agents.sql.go` | Planned |
| `internal/repository/queries/memory.sql` | `internal/repository/generated/memory.sql.go` | Planned |

**Current**: No `.sql` query files exist yet.

---

## Notes

- Current endpoints are inline handlers (no service/repository layers) — this is acceptable for the prototype stage
- As the codebase grows, all business logic should move to the service layer, following the Handler → Service → Repository → SQLC pattern
- The mapping table will expand as new endpoints and SQLC queries are added
- Transaction boundaries should be documented when multi-table operations are introduced
