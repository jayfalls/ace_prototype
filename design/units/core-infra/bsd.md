# Business Specification Document

## Unit Name
Core Infrastructure - Development Environment Setup

## Problem Statement
The project lacks a standardized, containerized development environment. Developers must manually install and configure Go, Node.js, PostgreSQL, NATS, and other dependencies on their host machines, leading to:
- Inconsistent environments across team members
- Difficulty onboarding new developers
- Version conflicts between projects
- Complex setup instructions

Additionally, with AI agents as first-class citizens, the development environment must be:
- Agent-friendly (easy to run specific tasks, readable outputs)
- Seamless from dev to production

## Solution
Establish a complete containerized development environment using Docker/Podman that provides:
- Go backend (ace_api) running in a container with hot reloading
- SvelteKit frontend (ace_fe) running in a container with hot reloading
- PostgreSQL database (ace_db) container
- NATS messaging server (ace_broker) container
- All services orchestrated via Docker Compose
- Single command setup (`git clone && docker compose up`)
- Maximum automation (auto migrations, auto seeding)

## Core Principles (from Problem Space)
1. **Single command setup** - `git clone && docker compose up`
2. **Hot reload** - core requirement for fast iteration
3. **Always run together** - services not isolated locally (tests handle isolation)
4. **AI-first** - code and tooling must be agent-friendly
5. **Dev = Prod** - same base images, compose for single agent, K8s for multi-agent
6. **Maximum automation** - migrations and seeding auto-run
7. **Named volumes** - persist data between restarts
8. **.env files** - simple dev secrets management

## Resource Expectations
- Reported minimum: 4 vCPU, 8 GB RAM
- Actual minimum: 2 vCPU, 2 GB RAM

## In Scope
- Docker Compose configuration for all services (ace_ prefix)
- Go backend container with hot reloading (air)
- SvelteKit frontend container with hot reloading (Vite)
- PostgreSQL database container
- NATS server container
- CONTAINER_ORCHESTRATOR env var for docker/podman selection
- Development workflow documentation
- Makefile for common operations
- Single Dockerfile per service (not separate dev/prod Dockerfiles)

## Out of Scope
- Production deployment configurations (K8s manifests in separate unit)
- Database migrations or schema (will be handled in later units)
- Application functionality (nothing needs to "work")
- Authentication or authorization
- CI/CD pipelines
- Monitoring or observability setup

## Value Proposition
- Developers can start developing in minutes without installing any tools locally
- Consistent environment across all team members and AI agents
- Easy cleanup and recreation of environment
- Standardized development workflow
- Seamless dev-to-prod experience

## Success Criteria
| Criterion | Metric | Target |
|-----------|--------|--------|
| Single command setup | Fresh clone + docker compose up | Complete in <10 minutes |
| All services start | `docker compose up` completes without errors | All 4 containers running |
| Backend hot reload | Source code changes reflect in <5 seconds | < 5 seconds |
| Frontend hot reload | Source code changes reflect in <5 seconds | < 5 seconds |
| Database accessible | Can connect to PostgreSQL from backend | Connection successful |
| NATS accessible | Can connect to NATS from backend | Connection successful |
| Data persistence | Data survives container restart | Named volumes used |
| Container orchestrator | Can switch between docker/podman | CONTAINER_ORCHESTRATOR env works |
