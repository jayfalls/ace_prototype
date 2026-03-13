# Problem Space

## Overview
Establish the API & DB patterns, structure, tools, and libraries to make development easy, maintainable, scalable, reviewable, and debuggable.

## What Are We Building?
- Foundation patterns for the Go API layer
- Database access patterns using SQLC
- Project structure and package organization
- Core middleware patterns (logging, error handling)
- Frontend API interface patterns

## What Are We NOT Building?
- No authentication/authorization (separate auth unit)
- No full observability (separate observability unit)
- No testing/CICD infrastructure (separate unit)
- No business features (agents, memories, sessions, etc.)

## Current State
- `api/main.go`: Minimal placeholder with Gin, health check, root endpoint
- `api/go.mod`: Only Gin dependency
- No database setup, no structure, no patterns

## Constraints
1. **AI-first architecture**: Code/tooling must be agent-friendly from the ground up
2. **Hot reload**: Core requirement for fast iteration (handled by core-infra)
3. **Services run together**: Not in isolation locally
4. **Dev = Prod approach**: Same base images, similar config (implementation uses separate dev/prod Dockerfiles)
5. **.env for dev**: Simple secrets management

## Success Criteria
A developer (or AI agent) can:
- Easily understand where to add new API endpoints
- Easily add new database tables and queries
- Follow consistent patterns across all code
- Debug issues easily
- Write tests following established patterns
- Scale the codebase as more features are added

## Dependencies
- Core Infrastructure unit (dev environment, docker compose)
- Architecture unit (overall system design)
- Future: Auth unit, Observability unit, CICD unit

## Open Questions for Research/Architecture
1. Which Go web framework (Chi, raw net/http, Gin)?
2. Database patterns (repository vs active record vs pure SQLC)?
3. Migration tool (golang-migrate, goose, dbmate)?
4. Validation approach?
5. Project structure (layered vs domain-driven)?
