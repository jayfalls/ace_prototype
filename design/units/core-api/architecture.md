# Architecture

## Overview
This document outlines the technical architecture for the Core API service. It builds upon the FSD and research documents to define the specific architecture decisions.

## System Context

### Position in ACE Framework
The Core API service is the central HTTP API entry point for the ACE Framework. It coordinates:
- **Frontend** → **API** → **Database** (PostgreSQL)
- **Frontend** → **API** → **Broker** → (future: Cognitive Engine, Memory Service)

```
┌──────────────────────────────────────────────────────────────────┐
│                         ACE Framework                             │
│                                                                   │
│  ┌─────────┐     ┌─────────────┐     ┌────────────────────┐   │
│  │Frontend │────▶│  Core API   │────▶│   PostgreSQL        │   │
│  │(Svelte) │     │  (Go/Chi)   │     │   (ace_db)          │   │
│  └─────────┘     └──────┬──────┘     └────────────────────┘   │
│                          │                                        │
│                          ▼                                        │
│                   ┌─────────────┐                                │
│                   │    NATS     │                                │
│                   │ (Broker)    │                                │
│                   └─────────────┘                                │
│                          │                                        │
│            ┌─────────────┼─────────────┐                        │
│            ▼             ▼             ▼                        │
│     ┌──────────┐  ┌──────────┐  ┌──────────┐                  │
│     │Cognitive │  │  Memory  │  │  Future  │                  │
│     │ Engine   │  │  Service │  │ Services │                  │
│     └──────────┘  └──────────┘  └──────────┘                  │
└──────────────────────────────────────────────────────────────────┘
```

## Architecture Patterns

### Layered Architecture
The API follows a layered (vertical slice) architecture:

```
┌─────────────────────────────────────────┐
│           Handler Layer                  │
│    (HTTP Request/Response)              │
├─────────────────────────────────────────┤
│           Service Layer                  │
│      (Business Logic)                   │
├─────────────────────────────────────────┤
│          Repository Layer                │
│    (Data Access / SQLC)                 │
├─────────────────────────────────────────┤
│           Database                      │
│      (PostgreSQL)                       │
└─────────────────────────────────────────┘
```

### Dependency Flow
- Dependencies point inward: Handler → Service → Repository → Database
- Each layer only knows about the layer directly below it
- Repository interfaces live in the service layer

## Component Design

### HTTP Server

**Router (Chi)**
- Uses chi/v5 for routing
- Routes organized by version and resource
- Example: `GET /api/v1/health`

**Middleware Stack** (applied in order)
1. `Recoverer` - Panic recovery
2. `Logger` - Request logging
3. `CORS` - Cross-origin requests
4. `Timeout` - Request timeout

**Handler Pattern**
```go
type Handler struct {
    service ServiceInterface
}

type ServiceInterface interface {
    // Service methods
}

func (h *Handler) HandleMethod(w http.ResponseWriter, r *http.Request) {
    // 1. Parse request
    // 2. Validate input
    // 3. Call service
    // 4. Write response
}
```

### Database Layer

**Connection Management**
- Uses `pgx/v5` for PostgreSQL connections
- Connection pool with configurable limits
- Single database instance: `ace_db`

**SQLC Workflow**
```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│  migrations │───▶│   SQLC      │───▶│  Generated  │
│   (.sql)    │    │   generate  │    │    Go       │
└─────────────┘    └─────────────┘    └─────────────┘
     │                                       │
     │                                       │
     ▼                                       ▼
  Schema                               Type-safe
  Definition                           Queries
```

**Repository Pattern**
- Repository interfaces defined in service layer
- Concrete implementations in repository layer
- Uses SQLC generated queries

### Configuration

**Environment-Based Config**
- All configuration via environment variables
- No hardcoded values
- `.env` file for local development
- Environment variables for production

**Config Structure**
```go
type Config struct {
    Server   ServerConfig
    Database DatabaseConfig
    NATS     NATSConfig
}
```

### Error Handling

**Error Response Format**
```go
type ErrorResponse struct {
    Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
    Code    string   `json:"code"`
    Message string   `json:"message"`
    Details []FieldError `json:"details,omitempty"`
}

type FieldError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}
```

**HTTP Status Codes**
| Code | Usage |
|------|-------|
| 200  | Success |
| 400  | Bad Request / Validation Error |
| 401  | Unauthorized |
| 403  | Forbidden |
| 404  | Not Found |
| 500  | Internal Server Error |

### Validation

**Request Validation Flow**
```
HTTP Request → Handler → Validator → Service → Repository
                  │            │
                  │            ▼
                  │      ValidationError
                  │            │
                  ▼            ▼
            400 Response   Retry
```

**Validation Library**
- `go-playground/validator/v10`
- Struct tags for declarative validation
- Custom validators for domain-specific rules

## Data Flow

### Typical Request Flow
```
1. Client sends HTTP request
2. Middleware processes (log, CORS, timeout)
3. Router dispatches to handler
4. Handler parses request body
5. Handler validates input
6. Handler calls service method
7. Service coordinates business logic
8. Service calls repository
9. Repository executes SQLC query
10. Repository returns result
11. Service returns to handler
12. Handler writes JSON response
13. Response sent to client
```

## Module Structure

### Go Workspace
```
backend/
├── go.work              # Workspace file
├── shared/              # Shared library (future)
└── services/
    └── api/             # API service module
        ├── cmd/         # Entry points
        ├── internal/   # Private code
        ├── migrations/  # DB migrations
        └── sqlc.yaml    # SQLC config
```

### Package Responsibilities

| Package | Responsibility | Public API |
|---------|---------------|------------|
| `cmd` | Application entry | main.go |
| `internal/config` | Configuration loading | Config struct |
| `internal/middleware` | HTTP middleware | Middleware functions |
| `internal/handler` | HTTP handlers | Handler methods |
| `internal/service` | Business logic | Service interfaces |
| `internal/repository` | Data access | Repository interfaces |
| `internal/response` | Response helpers | WriteJSON, Error functions |

## Security Considerations

### Input Validation
- All user input validated before processing
- Sanitize strings to prevent injection
- Use parameterized queries (SQLC handles this)

### Error Messages
- Don't expose internal details in errors
- Log errors with full context server-side
- Return generic messages to clients

### CORS
- Configure allowed origins via environment
- Restrict HTTP methods and headers
- Don't allow credentials if not needed

## Performance Considerations

### Database
- Connection pooling (pgx)
- Prepared statements via SQLC
- Query optimization in migrations

### HTTP
- Request timeout middleware
- Keep-alive connections
- Gzip compression (future)

## Future Architecture Considerations

When adding new services (Cognitive Engine, Memory):
- Each service gets own module under `services/`
- Shared code moves to `shared/`
- Services communicate via NATS
- Each service has own database schema or database

## Acceptance Criteria

- [ ] Layered architecture is clear and followable
- [ ] Each package has single, documented responsibility
- [ ] Dependencies flow inward only
- [ ] Repository pattern enables testability
- [ ] Error handling is consistent across all handlers
- [ ] Configuration is environment-based
- [ ] Workspace structure supports future services
