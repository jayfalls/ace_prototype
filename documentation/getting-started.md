# Getting Started

## Quick Start

### Clone and Run

```bash
# Clone the repository
git clone https://github.com/jayfalls/ace_prototype.git
cd ace_prototype

# Copy the example environment file
cp .env.example .env

# Start all services in development mode
make up
```

## Configuration

All configuration is done through the `.env` file. Copy `.env.example` to `.env` and customize as needed:

```bash
# Example .env file
CONTAINER_ORCHESTRATOR=docker
ENVIRONMENT=dev

# Database
POSTGRES_HOST=ace_db
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=ace

# NATS Broker
NATS_URL=nats://ace_broker:4222

# Frontend
VITE_API_URL=http://localhost:8080
```

### Using Podman

To use podman instead of docker, update your `.env` file:

```bash
CONTAINER_ORCHESTRATOR=podman
```

Then run:
```bash
make up
```

## Development vs Production

The project supports two environments:

| Environment | Compose File | Description |
|-------------|--------------|-------------|
| `dev` (default) | `docker-compose.dev.yml` | Hot reload, volume mounts for live coding |
| `prod` | `docker-compose.prod.yml` | Optimized images, no volumes, production-ready |

### Development Mode (Default)

```bash
make up              # Start all services
make down            # Stop all services
make build           # Build dev images
make logs            # View all logs
make logs-api        # View API logs
```

### Production Mode

```bash
make up ENVIRONMENT=prod              # Start all services in production mode
make down ENVIRONMENT=prod           # Stop all services
make build ENVIRONMENT=prod          # Build production images
make clean ENVIRONMENT=prod          # Clean up production containers
```

## Make Commands

| Command | Description |
|---------|-------------|
| `make up` | Start all services (dev mode) |
| `make up ENVIRONMENT=prod` | Start all services (prod mode) |
| `make down` | Stop all services |
| `make restart` | Restart all services |
| `make logs` | View logs |
| `make logs-api` | View API logs |
| `make logs-fe` | View frontend logs |
| `make logs-db` | View database logs |
| `make logs-broker` | View broker logs |
| `make clean` | Remove all containers and volumes (dev) |
| `make clean ENVIRONMENT=prod` | Remove all containers and volumes (prod) |
| `make build` | Build all images (dev) |
| `make build ENVIRONMENT=prod` | Build all images (prod) |
| `make ps` | Show running containers |
| `make help` | Show available commands |

## Environment Variables

| Variable | Values | Default | Description |
|----------|--------|---------|-------------|
| `ENVIRONMENT` | `dev`, `prod` | `dev` | Which environment to use |
| `CONTAINER_ORCHESTRATOR` | `docker`, `podman` | `docker` | Container runtime |
