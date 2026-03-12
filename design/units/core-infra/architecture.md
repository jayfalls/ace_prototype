# Architecture Document

## Overview
This document outlines the technical architecture for the core infrastructure development environment.

## Design Principles

### Single Compose File
- **Single `docker-compose.yml`** used for both dev and prod
- Environment-driven configuration (env vars, env files)
- Dev vs Prod differentiation via environment variables

### Seamless Service Discovery
- Use Docker Compose built-in DNS (service names)
- This pattern translates directly to Kubernetes (service DNS)
- No additional service mesh needed for dev

### Environment Configuration
- `.env` file for local development
- Environment variables drive all configuration
- Structure that translates to K8s ConfigMaps/Secrets

## Architecture Components

### Services
```
ace_api    - Go API service (port 8080)
ace_fe     - SvelteKit frontend (port 5173)
ace_db     - PostgreSQL database (port 5432)
ace_broker - NATS messaging (port 4222)
```

### Network
- Single Docker network for all services
- Service discovery via service names
- Ports exposed to host for development

## Technical Decisions

### Docker Compose
- Single `docker-compose.yml` file
- Environment-specific overrides via `.env` files
- Service names: ace_api, ace_fe, ace_db, ace_broker
- Named volumes for data persistence

### Container Orchestration
- CONTAINER_ORCHESTRATOR env var for docker/podman selection
- Makefile wraps compose commands
- Consistent command interface regardless of runtime

### API Service Structure
- Placeholder structure (to be defined in API unit)
- Single `main.go` entry point
- Standard Go module structure

### Frontend Service Structure
- Standard SvelteKit project structure
- Vite for development server and HMR

### Database
- PostgreSQL official image
- Named volume for persistence
- Environment-driven configuration

### Messaging
- NATS official image
- No persistence in dev (in-memory)

## Future-Ready Scaffolding

### Observability (for observability unit)
- Structured logging format (JSON)
- Configurable log levels via env
- Request/response logging middleware

### Migrations (for API unit)
- Migration directory structure
- Entry point for running migrations
- Environment-driven migration path

### Kubernetes Readiness
- Service names match K8s service names
- Environment-based configuration
- Single image for dev and prod
- Non-root container user

## File Structure

```
.
├── docker-compose.yml    # Single compose file for all environments
├── Makefile              # Orchestrator-agnostic commands
├── .env                  # Local development overrides
├── .env.example          # Template for environment variables
├── .gitignore
├── api/
│   ├── Dockerfile
│   ├── air.toml
│   ├── go.mod
│   ├── go.sum
│   └── main.go
└── frontend/
    ├── Dockerfile
    ├── package.json
    ├── svelte.config.js
    ├── vite.config.ts
    └── src/
```

## Configuration Approach

### Environment Variables
All configuration via environment variables:
- `CONTAINER_ORCHESTRATOR` - docker or podman
- `POSTGRES_*` - database configuration
- `NATS_URL` - messaging configuration
- `VITE_*` - frontend configuration

### Compose File Structure
- Use `env_file` for default values
- Environment section for overrides
- No hardcoded values

## Security Considerations
- Non-root users in containers
- No secrets committed to git
- .env in .gitignore
- Development credentials only (not production)

## Dependencies

### Runtime
- Docker or Podman
- Docker Compose

### Services
- Go 1.26 (api service)
- Node.js 25 (frontend service)
- PostgreSQL 18
- NATS 2.12
