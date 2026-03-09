# Design Decisions

<!--
Intent: Document the "why" behind technical choices made during implementation.
Scope: All significant design decisions, alternatives considered, and rationale.
Used by: AI agents to understand trade-offs and make informed decisions about future changes.
-->

## Overview

This document outlines the key design decisions for the core-infra unit, which provides foundational backend services for the ACE Framework MVP. These decisions prioritize simplicity, type safety, and developer productivity.

## UI/UX Design Decisions

Not applicable - core-infra is a backend-only unit.

## API Design Decisions

### REST vs GraphQL

- **Choice**: REST
- **Rationale**: Simpler to implement, better tooling, easier to cache, well-understood pattern. GraphQL adds complexity that isn't needed for MVP.

### Resource Naming

| Resource | Naming Convention | Rationale |
|----------|------------------|-----------|
| Users | /api/v1/users | Standard REST pluralization |
| Agents | /api/v1/agents | Standard REST pluralization |
| Sessions | Nested under agents | Sessions belong to agents |
| Thoughts | Nested under sessions | Thoughts belong to sessions |
| Memories | /api/v1/memories | Top-level - users own memories |
| LLM Providers | /api/v1/llm-providers | Standard REST pluralization |
| Settings | Nested under agents | Settings belong to agents |

### Versioning Strategy

- **Approach**: URL path versioning (/v1/)
- **Rationale**: Most straightforward, easy to understand, works with all HTTP clients

### Error Response Format

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Format | Consistent JSON structure | All responses follow {data: ...} or {error: {code, message, details}} |
| HTTP Status Codes | RESTful semantics | 200=success, 201=created, 400=bad request, 401=unauthorized, 404=not found, 500=error |

## Data Model Decisions

### Schema Design

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Primary Keys | UUID (gen_random_uuid) | No central ID service needed, better security |
| Timestamps | UTC, DEFAULT NOW() | Consistent time zone, automatic population |
| JSON Storage | JSONB | PostgreSQL native, indexed, queryable |
| Password Storage | bcrypt hash | Industry standard, configurable cost |
| API Key Storage | Encrypted VARCHAR | Security best practice |

### Indexing Strategy

- **User email/username**: Unique indexes for fast lookups during auth
- **Agent owner_id**: Fast filtering by user
- **Session agent_id/status**: Fast session queries
- **Memory tags**: GIN index for array search
- **Memory owner_id**: Fast user memory queries

### Caching Strategy

- **Approach**: Minimal caching for MVP
- **Rationale**: Keep it simple, add caching when profiling shows need
- **Future**: Redis for session data, rate limiting

## Architecture Decisions

### Project Structure

| Component | Pattern | Rationale |
|-----------|---------|-----------|
| Handler → Service → Repository | Clean Architecture | Separation of concerns, testable |
| SQLC for DB access | Type-safe queries | Eliminates runtime SQL errors |
| Middleware for cross-cutting | HTTP middleware | Auth, logging, rate limiting |

### API Layer

- **Framework**: Gin
- **Rationale**: Fast, lightweight, great middleware ecosystem, easy to learn

### Database Layer

- **Driver**: pgx
- **Query Generator**: SQLC
- **Rationale**: pgx is the standard Go PostgreSQL driver, SQLC provides type safety

### Authentication

- **JWT with RS256**
- **Access Token**: 15 minutes
- **Refresh Token**: 7 days
- **Rationale**: Industry standard, stateless, scales well

### Real-time Communication

- **Technology**: WebSocket (gorilla/websocket)
- **Rationale**: Full-duplex, lower latency than polling, works with Gin

## Trade-offs

### Known Trade-offs

| Trade-off | Impact | Mitigation |
|-----------|--------|------------|
| No ORM | More SQL knowledge required | SQLC generates type-safe code |
| PostgreSQL only | Lock-in | Document migration path if needed |
| JWT in localStorage (frontend) | XSS risk | HttpOnly cookies preferred, but harder with WebSocket |
| Synchronous processing | Latency | Future: async task queue |

### Decisions Postponed

- **Caching layer**: Add Redis when needed for performance
- **Multi-tenancy**: MVP supports single-tenant (user-owned resources)
- **API rate limiting**: Add when abuse is observed
- **Webhook events**: Add when external integrations needed

## Alternatives Considered

### Alternative 1: GORM (ORM)
- **Description**: Use GORM for database access
- **Why Rejected**: Runtime errors from string queries, performance overhead
- **Pros**: Familiar to developers, auto-migrations
- **Cons**: Runtime errors, slower, complex relationships

### Alternative 2: GraphQL (gqlgen)
- **Description**: Use GraphQL API
- **Why Rejected**: Overkill for MVP, added complexity
- **Pros**: Flexible queries, fewer endpoints
- **Cons**: Learning curve, more complex client code, debugging harder

### Alternative 3: gRPC
- **Description**: Use gRPC for API
- **Why Rejected**: Browser support issues, overkill for MVP
- **Pros**: Fast, type-safe, streaming
- **Cons**: Not browser-native, more complex setup

### Alternative 4: SQLite
- **Description**: Use SQLite for development
- **Why Rejected**: Different features than production PostgreSQL
- **Pros**: No setup, embedded
- **Cons**: Feature differences (JSON, arrays), migration needed for production

### Alternative 5: MongoDB
- **Description**: Use MongoDB for flexibility
- **Why Rejected**: SQL preferred for relational data
- **Pros**: Flexible schema
- **Cons**: Different query language, eventual consistency

## Conventions Used

### Go Code
- **Package naming**: lowercase, short (e.g., `handlers`, `services`)
- **Types**: PascalCase (e.g., `UserService`)
- **Variables**: camelCase (e.g., `userID`)
- **Constants**: PascalCase or SCREAMING_SNAKE_CASE
- **Interfaces**: Reader, Writer, Handler suffix

### Database
- **Tables**: snake_case, plural (e.g., `users`, `agents`)
- **Columns**: snake_case (e.g., `created_at`, `owner_id`)
- **Indexes**: `idx_<table>_<column>` format
- **Foreign Keys**: `<table>_id` suffix

### HTTP API
- **Endpoints**: kebab-case where needed (e.g., `llm-providers`)
- **JSON fields**: camelCase (e.g., `createdAt`)
- **Response wrapping**: `{data: ...}` for success
