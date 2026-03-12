# Functional Specification Document

## Overview
This document outlines the technical implementation for the Core Infrastructure development environment. The goal is to establish a containerized development environment with hot reloading for both Go backend and SvelteKit frontend.

## Architecture

### Components
1. **Go Backend** - Containerized with hot reloading via air
2. **SvelteKit Frontend** - Containerized with Vite HMR
3. **PostgreSQL** - Official Docker image for data persistence
4. **NATS** - Official Docker image for messaging

### Network Architecture
```
┌─────────────────────────────────────────────┐
│           Docker/Podman Network             │
│                                             │
│  ┌──────────┐    ┌──────────┐              │
│  │  ace_api │    │  ace_fe  │              │
│  │   Go     │    │ SvelteKit│              │
│  │ Backend  │    │ Frontend │              │
│  └────┬─────┘    └────┬─────┘              │
│       │                │                     │
│       └────────┬───────┘                     │
│                │                             │
│    ┌───────────┴───────────┐                │
│    │                       │                │
│ ┌──┴───┐             ┌────┴────┐          │
│ │ ace_db│           │ace_broker│          │
│ │PostgreSQL│          │   NATS  │          │
│ └────────┘            └─────────┘          │
└─────────────────────────────────────────────┘
```

## Technical Specifications

### Docker Compose Configuration

**Services:**
- `ace_api` - Go application with air for hot reload
- `ace_fe` - SvelteKit application with Vite HMR
- `ace_db` - PostgreSQL database
- `ace_broker` - NATS messaging server

**Environment Variables:**
| Variable | Description | Default |
|----------|-------------|---------|
| POSTGRES_HOST | PostgreSQL host | ace_db |
| POSTGRES_PORT | PostgreSQL port | 5432 |
| POSTGRES_USER | PostgreSQL user | postgres |
| POSTGRES_PASSWORD | PostgreSQL password | postgres |
| POSTGRES_DB | Database name | ace |
| NATS_URL | NATS server URL | nats://ace_broker:4222 |

### Backend Implementation

**Dockerfile.dev:**
- Based on golang:1.26 (latest stable)
- Install air for hot reloading
- Mount source code as volume
- Expose port 8080

**air Configuration:**
- Watch entire backend directory
- Exclude vendor, .git, node_modules
- Build command: go build -o /app/main
- Run command: /app/main

### Frontend Implementation

**Dockerfile.dev:**
- Based on node:25 (latest stable)
- Install dependencies with npm install
- Run with npm run dev
- Expose port 5173 (Vite default)

**Vite Configuration:**
- Enable HMR via WebSocket
- Configure proxy to backend on port 8080

### Database Configuration

**PostgreSQL:**
- Image: postgres:18 (latest stable)
- Volume: postgres_data for persistence
- Environment variables configured via .env
- Initialize with empty database

**NATS:**
- Image: nats:2.12 (latest stable)
- No persistence needed (dev environment)
- Expose port 4222 for client connections

## File Structure

```
├── docker-compose.yml
├── Makefile
├── backend/
│   ├── Dockerfile
│   ├── Dockerfile.dev
│   ├── air.toml
│   ├── go.mod
│   ├── go.sum
│   └── main.go
├── frontend/
│   ├── Dockerfile
│   ├── Dockerfile.dev
│   ├── package.json
│   ├── svelte.config.js
│   ├── vite.config.ts
│   └── src/
│       └── ...
└── .env.example
```

## Makefile Commands

| Command | Description |
|---------|-------------|
| `make up` | Start all services |
| `make down` | Stop all services |
| `make logs` | View aggregated logs |
| `make logs-backend` | View backend logs only |
| `make logs-frontend` | View frontend logs only |
| `make clean` | Remove all containers and volumes |
| `make re` | Restart all services |

## Development Workflow

1. **Start Development:**
   ```bash
   make up
   ```

2. **View Logs:**
   ```bash
   make logs
   ```

3. **Stop Development:**
   ```bash
   make down
   ```

4. **Clean Up:**
   ```bash
   make clean
   ```

## Hot Reloading Behavior

### Backend
- Source code changes in `./backend` trigger automatic rebuild
- air watches for file changes and rebuilds
- Binary restarts automatically
- Build output visible in logs

### Frontend
- Source code changes in `./frontend` trigger HMR
- Vite updates modules without full reload
- Browser automatically reflects changes (via HMR)

## Environment Configuration

### .env.example
```env
# Backend
POSTGRES_HOST=ace_db
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=ace
NATS_URL=nats://ace_broker:4222

# Frontend
VITE_API_URL=http://localhost:8080
```

## Security Considerations
- Development credentials in .env (not committed to git)
- .env.example committed for reference
- .gitignore excludes .env files

## Dependencies

### Backend
- Go 1.26+ (latest stable)
- air (hot reload)
- Standard library only for initial setup

### Frontend
- Node.js 25+ (latest stable)
- SvelteKit
- Vite

### Infrastructure
- Docker/Podman
- Docker Compose
- PostgreSQL 18
- NATS 2.12
