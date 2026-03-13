# Implementation Plan

## Overview
This document breaks down the implementation of core-api foundational patterns into micro-PRs. Each PR represents the smallest divisible unit of work that can be independently implemented and tested.

## Prerequisites
Before starting implementation, ensure you have read:
- `design/README.md` - System architecture
- `design/units/core-api/` - All design documents for this unit

## Implementation Order

### Phase 1: Foundation

#### PR 1: Go Workspace Setup
**Goal**: Establish the Go workspace structure
**Files**:
- `backend/go.work`
- `backend/shared/go.mod` (placeholder)
- `backend/services/api/go.mod`

**Acceptance Criteria**:
- [ ] `go work ls` shows both modules
- [ ] `go build ./...` succeeds in workspace
- [ ] Modules can import each other

#### PR 2: Database Connection
**Goal**: Establish PostgreSQL connection using pgx
**Files**:
- `backend/services/api/internal/repository/db.go`
- `backend/services/api/internal/config/config.go`

**Acceptance Criteria**:
- [ ] Can connect to PostgreSQL using environment variables
- [ ] Connection pool is configured
- [ ] Graceful shutdown works

### Phase 2: HTTP Server

#### PR 3: Basic HTTP Server with Chi
**Goal**: Set up Chi router with minimal server
**Files**:
- `backend/services/api/cmd/main.go`
- `backend/services/api/internal/handler/health.go`

**Acceptance Criteria**:
- [ ] Server starts on configured port
- [ ] GET /health returns 200 OK
- [ ] Graceful shutdown works

#### PR 4: Middleware Stack
**Goal**: Add standard middleware
**Files**:
- `backend/services/api/internal/middleware/logger.go`
- `backend/services/api/internal/middleware/recovery.go`
- `backend/services/api/internal/middleware/cors.go`

**Acceptance Criteria**:
- [ ] Requests are logged
- [ ] Panics are recovered
- [ ] CORS is configurable

### Phase 3: Database Patterns

#### PR 5: SQLC Setup
**Goal**: Configure SQLC for type-safe queries
**Files**:
- `backend/services/api/sqlc.yaml`
- `backend/services/api/internal/repository/queries/.keep`
- `backend/services/api/migrations/.keep`

**Acceptance Criteria**:
- [ ] SQLC generates Go code from queries
- [ ] Generated types are used in handlers

#### PR 6: Health Check Migration
**Goal**: Add health check table for SQLC demo
**Files**:
- `backend/services/api/migrations/001_create_health_check.sql`
- `backend/services/api/internal/repository/queries/health.sql`

**Acceptance Criteria**:
- [ ] Migration runs successfully
- [ ] SQLC generates queries
- [ ] Health handler uses generated query

### Phase 4: API Patterns

#### PR 7: Response Helpers
**Goal**: Standardize API responses
**Files**:
- `backend/services/api/internal/response/response.go`

**Acceptance Criteria**:
- [ ] Success responses follow format
- [ ] Error responses follow format
- [ ] Validation errors return field details

#### PR 8: Input Validation
**Goal**: Add request validation
**Files**:
- `backend/services/api/internal/middleware/validation.go` (optional)

**Acceptance Criteria**:
- [ ] Invalid requests return 400 with details
- [ ] Validation happens before business logic

### Phase 5: Configuration

#### PR 9: Configuration Package
**Goal**: Complete config loading
**Files**:
- `backend/services/api/internal/config/config.go`

**Acceptance Criteria**:
- [ ] All config from environment variables
- [ ] Validation on startup
- [ ] .env.example is complete

## Implementation Notes

### Shared Module
The `shared/` module is a placeholder for future units. For now:
- Create empty `shared/go.mod` with module name `github.com/ace/shared`
- This allows imports to resolve even without actual code

### Package Naming
- Use singular names: `handler`, `service`, `repository`
- Group by domain as features are added: `handler/users`, `handler/sessions`

### Error Handling
- Return errors from lower layers
- Handle at handler level
- Log with context before returning

### Testing
- Unit tests for handlers and services
- Integration tests for repository
- Keep tests adjacent to code: `handler_test.go`

## Workflow

1. Create branch from `main`
2. Implement one PR at a time
3. Ensure all acceptance criteria are met
4. Create PR and link to this issue
5. After merge, update changelog

## Future Work
After core-api patterns are established:
- Add authentication (core-auth unit)
- Add observability (core-observability unit)
- Add business features
