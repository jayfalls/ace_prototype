# Core Infrastructure Development Environment

This document provides instructions for setting up and using the ACE Prototype development environment.

## Overview

The core infrastructure provides a containerized development environment with:
- **API Service** - Go backend with Gin web framework
- **Frontend Service** - SvelteKit frontend (Phase 3)
- **Database** - PostgreSQL
- **Messaging** - NATS message broker

## Prerequisites

- Docker or Podman
- Docker Compose
- Git

## Quick Start

### 1. Clone the Repository

```bash
git clone https://github.com/jayfalls/ace_prototype.git
cd ace_prototype
```

### 2. Configure Environment Variables

Copy the example environment file:

```bash
cp .env.example .env
```

The default values work for local development:
- `POSTGRES_HOST=ace_db`
- `POSTGRES_PORT=5432`
- `POSTGRES_USER=postgres`
- `POSTGRES_PASSWORD=postgres`
- `POSTGRES_DB=ace`
- `NATS_URL=nats://ace_broker:4222`
- `CONTAINER_ORCHESTRATOR=docker`

### 3. Start All Services

Using the Makefile:

```bash
make up
```

Or directly with Docker Compose:

```bash
docker compose up -d
```

### 4. Verify Services

Check service status:

```bash
docker compose ps
```

Expected output:
```
NAME       IMAGE       COMMAND               SERVICE    CREATED   STATUS
ace_api    ace_api     "./ace-api"           api        ...       Up
ace_broker nats:2.12  "nats-server -js"     broker     ...       Up
ace_db     postgres:18 "docker-entrypoint.s…" db        ...       Up
```

### 5. Test the API

The API server should be running on port 8080:

```bash
# Test root endpoint
curl http://localhost:8080/
# Response: {"message":"ACE API Server"}

# Test health check
curl http://localhost:8080/health
# Response: {"status":"OK"}
```

## Service Details

### API Service (Port 8080)

The API service is built with Go and uses the Gin web framework.

**Endpoints:**
- `GET /` - Root endpoint
- `GET /health` - Health check

**Development:**
- Uses [air](https://github.com/air-verse/air) for hot reload
- Source code is mounted from `./api` to `/app` in the container
- Any changes to the source code will trigger an automatic rebuild

**Run in development mode:**
```bash
docker compose run --rm -it api air
```

**Production build:**
```bash
docker build -t ace_api ./api
docker run -d -p 8080:8080 ace_api
```

### Frontend Service (Port 5173)

See [Phase 3: Frontend Service Documentation](#phase-3-frontend-service)

### Database Service (Port 5432)

PostgreSQL 18 with persistent storage.

**Connection:**
- Host: `localhost` (from host) or `ace_db` (from containers)
- Port: `5432`
- Database: `ace`
- User: `postgres`
- Password: `postgres`

### Messaging Service (Port 4222)

NATS 2.12 with JetStream persistence.

**Connection:**
- Host: `localhost` (from host) or `ace_broker` (from containers)
- Port: `4222`

## Development Workflow

### Making Changes to API

1. Edit files in the `api/` directory
2. The container will automatically rebuild (if using air)
3. Test your changes

### Running Specific Services

```bash
# Start only API and Database
docker compose up -d api db

# Start only API
docker compose up -d api

# View logs
docker compose logs -f api
```

### Running Tests

```bash
# Run API tests
docker compose exec api go test ./...

# Run with coverage
docker compose exec api go test -coverprofile=coverage.out ./...
```

### Stopping Services

```bash
# Stop all services
make down

# Or with Docker Compose
docker compose down
```

### Cleaning Up

```bash
# Remove all containers, networks, and volumes
make clean

# Or with Docker Compose
docker compose down -v
```

## Troubleshooting

### Container Won't Start

Check logs:
```bash
docker compose logs <service-name>
```

### Port Already in Use

Stop other services using the same port:
```bash
# Find what's using port 8080
sudo lsof -i :8080
```

### Database Connection Issues

Ensure the database is ready before the API starts:
```bash
docker compose up -d db
# Wait a few seconds
docker compose up -d api
```

### Rebuilding from Scratch

```bash
docker compose down -v
docker compose build --no-cache
docker compose up -d
```

## Project Structure

```
ace_prototype/
├── api/                    # Go API service
│   ├── Dockerfile         # Container definition
│   ├── air.toml          # Hot reload configuration
│   ├── go.mod            # Go module definition
│   ├── go.sum            # Go dependencies
│   └── main.go           # Application entry point
├── frontend/              # SvelteKit frontend (Phase 3)
├── documentation/         # Project documentation
│   └── changelogs/       # Daily changelog entries
├── docker-compose.yml    # Container orchestration
├── Makefile             # Development commands
├── .env.example         # Environment template
└── .gitignore          # Git ignore patterns
```

## Available Makefile Commands

```bash
make help              # Show available commands
make up               # Start all services
make down             # Stop all services
make restart          # Restart all services
make logs             # View logs
make logs-api         # View API logs
make logs-fe          # View frontend logs
make logs-db          # View database logs
make logs-broker      # View broker logs
make clean            # Remove all containers and volumes
make build            # Build all images
make rebuild          # Rebuild all images (no cache)
```

## Security

- The API container runs as a non-root user (`appuser`)
- No secrets are committed to the repository
- Use `.env` file for sensitive data (already in `.gitignore`)
- Use strong passwords in production

## Next Steps

- **Phase 3**: Implement Frontend Service with SvelteKit
- **Phase 4-6**: Integration testing and verification

## Related Documentation

- [Design README](../design/README.md)
- [Core Infrastructure Implementation](../design/units/core-infra/implementation.md)
- [Core Infrastructure Architecture](../design/units/core-infra/architecture.md)
