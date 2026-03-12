# Implementation Plan

<!--
Intent: Define the step-by-step execution plan for building the feature.
Scope: All implementation tasks, their order, dependencies, and checkpoints.
Used by: AI agents to execute the feature implementation in a logical order.

Guidelines:
- Be highly verbose, break down into smallest possible tasks
- Document WHAT needs to be created, not HOW (implementer figures that out)
- Include verification step for EACH task
- Include final integration verification
- Order tasks logically (dependencies first)
-->

## Implementation Overview

This document defines the step-by-step implementation plan for the core infrastructure development environment. The implementation establishes a containerized development environment with Go backend, SvelteKit frontend, PostgreSQL database, and NATS messaging.

## Implementation Phases

### Phase 1: Project Foundation

This phase creates the base configuration files and directory structure that other components depend on.

#### Tasks

| Task | Description | Dependencies |
|------|-------------|--------------|
| 1.1 | Create root directory structure | None |
| 1.2 | Create docker-compose.yml with all services | None |
| 1.3 | Create .env.example template | None |
| 1.4 | Create .gitignore | None |
| 1.5 | Create Makefile with orchestrator support | None |

#### Deliverables
- docker-compose.yml defining all four services
- .env.example with all required environment variables
- .gitignore for development environment
- Makefile with CONTAINER_ORCHESTRATOR support

#### Verification
- [ ] docker-compose.yml validates (docker compose config)
- [ ] .env.example contains all required variables
- [ ] Makefile shows help target

---

### Phase 2: API Service

This phase creates the Go backend service with hot reload capability.

#### Tasks

| Task | Description | Dependencies |
|------|-------------|--------------|
| 2.1 | Create api/ directory | Phase 1 |
| 2.2 | Create api/Dockerfile | Phase 1 |
| 2.3 | Create api/go.mod | Phase 1 |
| 2.4 | Create api/main.go (minimal HTTP server) | 2.3 |
| 2.5 | Create api/air.toml for hot reload | 2.2 |
| 2.6 | Add api service to docker-compose.yml | 1.2, 2.2 |

#### Deliverables
- api/Dockerfile - single Dockerfile for dev and prod
- api/go.mod - Go module definition
- api/main.go - minimal HTTP server on port 8080
- api/air.toml - hot reload configuration
- docker-compose.yml updated with ace_api service

#### Verification
- [ ] Dockerfile builds successfully
- [ ] air.toml is valid
- [ ] docker-compose.yml includes ace_api service

---

### Phase 3: Frontend Service

This phase creates the SvelteKit frontend service with hot reload capability.

#### Tasks

| Task | Description | Dependencies |
|------|-------------|--------------|
| 3.1 | Create frontend/ directory | Phase 1 |
| 3.2 | Create frontend/Dockerfile | Phase 1 |
| 3.3 | Create frontend/package.json | Phase 1 |
| 3.4 | Create frontend/svelte.config.js | Phase 1 |
| 3.5 | Create frontend/vite.config.ts | Phase 1 |
| 3.6 | Create frontend/src/app.html (minimal SvelteKit) | 3.3 |
| 3.7 | Create frontend/src/routes/+page.svelte | 3.6 |
| 3.8 | Add frontend service to docker-compose.yml | 1.2, 3.2 |

#### Deliverables
- frontend/Dockerfile - SvelteKit container
- frontend/package.json - Node dependencies
- frontend/svelte.config.js - SvelteKit configuration
- frontend/vite.config.ts - Vite configuration with HMR
- frontend/src/ - minimal SvelteKit app structure
- docker-compose.yml updated with ace_fe service

#### Verification
- [ ] Dockerfile builds successfully
- [ ] package.json has valid dependencies
- [ ] docker-compose.yml includes ace_fe service

---

### Phase 4: Database Service

This phase configures the PostgreSQL database service.

#### Tasks

| Task | Description | Dependencies |
|------|-------------|--------------|
| 4.1 | Add ace_db service to docker-compose.yml | 1.2 |
| 4.2 | Configure PostgreSQL volume for persistence | 4.1 |
| 4.3 | Configure environment variables for ace_db | 4.1 |

#### Deliverables
- docker-compose.yml with ace_db PostgreSQL service
- Named volume for data persistence

#### Verification
- [ ] docker-compose.yml includes ace_db with volume
- [ ] Environment variables defined

---

### Phase 5: Messaging Service

This phase configures the NATS messaging service.

#### Tasks

| Task | Description | Dependencies |
|------|-------------|--------------|
| 5.1 | Add ace_broker service to docker-compose.yml | 1.2 |
| 5.2 | Configure NATS persistence (to mimic prod) | 5.1 |
| 5.3 | Configure environment variables for ace_broker | 5.1 |

#### Deliverables
- docker-compose.yml with ace_broker NATS service
- NATS persistence enabled

#### Verification
- [ ] docker-compose.yml includes ace_broker with persistence
- [ ] Environment variables defined

---

### Phase 6: Integration

This phase ensures all services work together correctly.

#### Tasks

| Task | Description | Dependencies |
|------|-------------|--------------|
| 6.1 | Verify docker-compose.yml has all services | 2.6, 3.8, 4.3, 5.3 |
| 6.2 | Verify networking between services | 6.1 |
| 6.3 | Test CONTAINER_ORCHESTRATOR in Makefile | 1.5 |
| 6.4 | Split Dockerfiles into dev/prod variants | 2.2, 3.2 |
| 6.5 | Split docker-compose into dev/prod | 6.1 |
| 6.6 | Add ENVIRONMENT support to Makefile | 1.5 |
| 6.7 | Fix container startup issues (permissions, volumes) | 6.1 |

#### Deliverables
- docker-compose.dev.yml for development (hot reload, volume mounts)
- docker-compose.prod.yml for production (no volumes, optimized images)
- api/Dockerfile.dev (hot reload with air, root user for volume permissions)
- api/Dockerfile.prod (pre-built binary, non-root user)
- frontend/Dockerfile.dev (npm run dev)
- frontend/Dockerfile.prod (production build, non-root user)
- Makefile with ENVIRONMENT=dev|prod support

#### Verification
- [x] All four services defined in docker-compose.dev.yml
- [x] All four services defined in docker-compose.prod.yml
- [x] Services on same network
- [x] Makefile respects CONTAINER_ORCHESTRATOR
- [x] Makefile respects ENVIRONMENT
- [x] `make up` starts all services in dev mode
- [x] `make up ENVIRONMENT=prod` starts all services in prod mode
- [x] `make down` stops all services

---

## Implementation Checklist

### Foundation
- [x] docker-compose.dev.yml created
- [x] docker-compose.prod.yml created
- [x] .env.example created
- [x] .gitignore created
- [x] Makefile created

### API Service
- [x] api/Dockerfile.dev created (hot reload)
- [x] api/Dockerfile.prod created (production)
- [x] api/go.mod created
- [x] api/main.go created
- [x] api/air.toml created
- [x] ace_api service in docker-compose.dev.yml
- [x] ace_api service in docker-compose.prod.yml

### Frontend Service
- [x] frontend/Dockerfile.dev created (hot reload)
- [x] frontend/Dockerfile.prod created (production)
- [x] frontend/package.json created
- [x] frontend/svelte.config.js created
- [x] frontend/vite.config.ts created
- [x] frontend/src/ structure created
- [x] ace_fe service in docker-compose.dev.yml
- [x] ace_fe service in docker-compose.prod.yml

### Database Service
- [x] ace_db service in docker-compose.dev.yml
- [x] ace_db service in docker-compose.prod.yml
- [x] Named volume configured

### Messaging Service
- [x] ace_broker service in docker-compose.dev.yml
- [x] ace_broker service in docker-compose.prod.yml
- [x] Persistence configured

### Integration
- [x] All services present in docker-compose.dev.yml
- [x] All services present in docker-compose.prod.yml
- [x] Services can communicate
- [x] Makefile commands work
- [x] ENVIRONMENT variable supported

## Verification Commands

Run these commands to verify the implementation:

```bash
# Verify docker-compose configuration
docker compose -f docker-compose.dev.yml config
docker compose -f docker-compose.prod.yml config

# Development mode (default)
make up              # Start all services in dev mode
make down            # Stop all services
make build           # Build dev images
make logs            # View logs

# Production mode
make up ENVIRONMENT=prod     # Start all services in prod mode
make down ENVIRONMENT=prod   # Stop all services
make build ENVIRONMENT=prod # Build prod images

# Check service status
docker compose -f docker-compose.dev.yml ps
docker compose -f docker-compose.prod.yml ps

# Verify networking
docker network ls
docker network inspect ace_prototype_ace_network

# Verify volumes
docker volume ls

# Clean up
make clean
make clean ENVIRONMENT=prod
```

## Rollback Plan

To rollback the implementation:

1. Run `make clean` to remove all containers and volumes
2. Remove created files:
   - docker-compose.dev.yml
   - docker-compose.prod.yml
   - Makefile
   - .env.example
   - api/ directory
   - frontend/ directory
3. Restore from git if needed
