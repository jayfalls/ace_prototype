# Functional Specification Document

## Overview
This document outlines the technical implementation for the Core API patterns, structure, tools, and libraries. The goal is to establish foundational patterns that make development easy, maintainable, scalable, reviewable, and debuggable.

## Technical Stack

| Component | Technology | Version |
|-----------|------------|---------|
| Web Framework | Chi | v5 |
| Database | PostgreSQL | 18 |
| DB Type Safety | SQLC | Latest |
| Migrations | Goose | Latest |
| Validation | go-playground/validator | v10 |
| Config | Standard lib + godotenv | N/A |

## Project Structure

### Directory Layout
```
api/
├── cmd/
│   └── api/
│       └── main.go              # Entry point
├── internal/
│   ├── config/
│   │   └── config.go            # Configuration loading
│   ├── handler/                 # HTTP handlers (controller layer)
│   │   └── example/
│   │       └── handler.go
│   ├── service/                 # Business logic (service layer)
│   │   └── example/
│   │       └── service.go
│   ├── repository/              # Data access (repository layer)
│   │   ├── db.go               # Database connection
│   │   └── example/
│   │       └── repo.go
│   ├── model/                   # Domain models
│   │   └── example/
│   │       └── model.go
│   ├── middleware/              # HTTP middleware
│   │   └── logger.go
│   └── response/                # Response helpers
│       └── response.go
├── migrations/                   # Database migrations
│   └── 001_create_example.sql
├── sqlc.yaml                    # SQLC configuration
├── go.mod
├── go.sum
└── .env                         # Local development (not committed)
```

### Layer Responsibilities

**Handler Layer (internal/handler/)**
- Receives HTTP requests
- Validates input
- Calls service layer
- Returns HTTP responses

**Service Layer (internal/service/)**
- Contains business logic
- Coordinates between handlers and repositories
- Transaction management

**Repository Layer (internal/repository/)**
- Database operations
- Uses SQLC-generated code
- Repository interfaces for testability

**Model Layer (internal/model/)**
- Domain models
- Request/response DTOs

## Database Setup

### SQLC Configuration (sqlc.yaml)
```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "internal/repository/query"
    schema: "migrations"
    gen:
      go:
        package: "db"
        out: "internal/repository/db"
        sql_package: "pgx/v5"
```

### Migration Setup (migrations/)
- SQL migration files with version prefixes: `001_`, `002_`, etc.
- Goose for running migrations
- Migration command: `goose postgres ${DATABASE_URL} up`

### Connection Pool
- Use `pgx/v5` for database connections
- Configure max connections, timeouts via config
- Environment variables for all connection settings

## API Implementation

### Router Setup (Chi)
```go
r := chi.NewRouter()

// Middleware
r.Use(middleware.Logger)
r.Use(middleware.Recoverer)

// Routes
r.Route("/api/v1", func(r chi.Router) {
    // v1 routes
})

// Health check
r.Get("/health", handler.Health)
```

### Request/Response Pattern
```go
// Request DTO
type CreateExampleRequest struct {
    Name string `json:"name" validate:"required,min=1,max=100"`
}

// Response helper
type APIResponse struct {
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
    Error   *APIError   `json:"error,omitempty"`
}

type APIError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}
```

### Error Handling
- All errors return standardized JSON response
- HTTP status codes: 200, 400, 401, 403, 404, 500
- Validation errors return field-level details

## Validation

### Using go-playground/validator
```go
type CreateRequest struct {
    Name  string `json:"name" validate:"required,min=1,max=100"`
    Email string `json:"email" validate:"required,email"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
    var req CreateRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        response.Error(w, "invalid_request", "Invalid request body", http.StatusBadRequest)
        return
    }

    if err := validate.Struct(req); err != nil {
        response.ValidationError(w, err)
        return
    }
    
    // Process request...
}
```

## Configuration

### Environment Variables
| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| POSTGRES_HOST | Database host | Yes | localhost |
| POSTGRES_PORT | Database port | Yes | 5432 |
| POSTGRES_USER | Database user | Yes | postgres |
| POSTGRES_PASSWORD | Database password | Yes | - |
| POSTGRES_DB | Database name | Yes | ace |
| API_PORT | HTTP server port | No | 8080 |
| LOG_LEVEL | Logging level | No | info |

### Config Package
```go
type Config struct {
    Postgres PostgresConfig `envprefix:"POSTGRES_"`
    API      APIConfig
}

type PostgresConfig struct {
    Host     string `env:"HOST"`
    Port     int    `env:"PORT"`
    User     string `env:"USER"`
    Password string `env:"PASSWORD"`
    DB       string `env:"DB"`
}
```

## Frontend Integration

### CORS Configuration
- Allow frontend origin (configurable via env)
- Standard headers: Content-Type, Authorization

### API Base URL
- Configurable via environment
- Frontend uses: `VITE_API_URL`

## Middleware

### Required Middleware
1. **Logger** - Request/response logging
2. **Recoverer** - Panic recovery
3. **CORS** - Cross-origin requests
4. **Timeout** - Request timeout

### Middleware Order
```
Logger → Recoverer → CORS → Timeout → Routes
```

## Acceptance Criteria

- [ ] Project structure follows layered architecture
- [ ] Chi router is configured and working
- [ ] SQLC generates type-safe database code
- [ ] Migrations can be created and run
- [ ] Request validation works with go-playground/validator
- [ ] Error responses are standardized
- [ ] Configuration loads from environment
- [ ] Health check endpoint exists
- [ ] CORS is configured for frontend
- [ ] All code follows consistent patterns
