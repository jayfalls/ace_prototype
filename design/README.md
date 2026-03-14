# Design

This is the main living design document for the ACE Framework MVP.

## Overview

The ACE Framework is a conceptual cognitive architecture for building ethical autonomous agents.

- [Source](source.md) - ACE Framework research and theory
- [Units](units.md) - Feature/component definitions and templates

## 1. Architecture

### High-Level Overview

The ACE Framework consists of:
- **Frontend** - SvelteKit web UI
- **Core API** - Go HTTP API with Chi
- **Message Broker (NATS)** - Inter-service communication
- **Database (PostgreSQL)** - Persistence with SQLC

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              ACE Framework                               │
│                                                                          │
│  ┌──────────────┐      ┌──────────────┐      ┌──────────────────────┐  │
│  │   Frontend   │◄────►│    Core      │      │   Cognitive Engine  │  │
│  │  SvelteKit   │      │  API (Go)    │◄────►│        (Future)     │  │
│  │              │      │    Chi       │      │                      │  │
│  └──────┬───────┘      └──────┬───────┘      └──────────┬───────────┘  │
│         │                     │                         │              │
│         │              ┌──────┴──────┐                  │              │
│         │              │             │                  │              │
│         └──────────────┼─────────────┼──────────────────┘              │
│                        ▼             ▼                                  │
│                 ┌───────────┐   ┌───────────┐                          │
│                 │PostgreSQL │   │   NATS    │                          │
│                 │  + SQLC   │   │(Broker)   │                          │
│                 └───────────┘   └───────────┘                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Core Components

| Component | Responsibility | Status |
|-----------|---------------|--------|
| **Frontend** | User interface (SvelteKit) | Future |
| **Core API** | HTTP API, orchestration (Go/Chi) | In Progress |
| **Message Broker (NATS)** | Inter-service communication | Future |
| **Persistence** | PostgreSQL + SQLC + Goose | In Progress |
| **Cognitive Engine** | 6 ACE layers | Future |
| **Memory Service** | Per-layer + global memory | Future |

### Development Environment

**Docker Compose** (local development):
- `frontend` - SvelteKit dev server (:5173)
- `api` - Go API server (:8080)
- `postgres` - Database (:5432)
- `nats` - Message broker (:4222)

### Project Structure

```
backend/
├── go.work                      # Go workspace
├── shared/                      # Shared code (future units)
└── services/
    └── api/                    # Core API service
        ├── cmd/                # Entry points
        ├── internal/           # Private code
        │   ├── config/         # Configuration
        │   ├── handler/        # HTTP handlers
        │   ├── middleware/    # HTTP middleware
        │   ├── repository/    # Database layer
        │   └── response/      # Response helpers
        ├── migrations/        # DB migrations
        └── sqlc.yaml          # SQLC config
```

## 2. Technologies

### Backend
- **Go 1.26** - Primary language
- **Chi v5** - HTTP router
- **pgx/v5** - PostgreSQL driver
- **SQLC** - Type-safe SQL
- **Goose** - Database migrations
- **go-playground/validator** - Input validation
- **NATS** - Message broker

### Frontend (Future)
- **SvelteKit** - Full-stack web framework
- **TypeScript** - Type-safe code

### Database
- **PostgreSQL** - Primary data store
- **SQLC** - Compile-time SQL type checking
- **Goose** - SQL migrations

### Infrastructure
- **Docker Compose** - Local development
- **WebSocket** - Real-time communication (future)

## 3. API

### Core API Patterns

The Core API establishes patterns for all future services:

- **Layered Architecture**: Handler → Service → Repository → Database
- **Repository Pattern**: Interfaces in service layer, implementations in repository
- **SQLC**: Type-safe queries generated from SQL
- **Migrations**: Goose for versioned schema changes
- **Validation**: Struct tags with go-playground/validator
- **Response Format**: Consistent JSON success/error responses

### REST Endpoints

Base: `/api/v1`

| Endpoint | Description |
|----------|-------------|
| `GET /health` | Health check |

Additional endpoints will be defined in future units.

## 4. Database

### Schema Management

- **SQLC**: Write SQL queries → Generate type-safe Go code
- **Goose**: Versioned migrations in `migrations/` directory
- **pgx/v5**: Connection pooling and prepared statements

### Key Patterns

- Generated types live in `repository/generated/`
- Query files in `repository/queries/`
- Migrations numbered: `001_`, `002_`, etc.

## 5. Configuration

All configuration via environment variables:
- `DATABASE_URL` - PostgreSQL connection
- `NATS_URL` - NATS broker URL
- `PORT` - HTTP server port
- `LOG_LEVEL` - Logging level

No hardcoded values. `.env.example` documents required variables.

## 6. Security

### Authentication (Future)
- JWT-based stateless authentication
- Token includes: `user_id`, `exp`, `roles`

### Protected Routes
All routes require auth except:
- `GET /health` - Health check

### Input Validation
- All input validated with go-playground/validator
- SQL injection prevented via SQLC (parameterized queries)

## 7. Testing

### Test Types
- **Unit tests**: Handlers and services
- **Integration tests**: Repository and database
- **E2E tests**: Critical user flows

### Tools
- **Go**: Built-in testing framework
- **Vitest** - Frontend (future)

## 8. Monitoring

### Logging
- Structured JSON logs
- Request ID middleware for tracing

### Health Checks
- `GET /health` - Basic liveness check
