# User Stories

## User Story 1: Developer Sets Up Local Environment
**As a** developer joining the project  
**I want to** start the development environment with a single command  
**So that** I can begin coding immediately without manual setup

**Acceptance Criteria:**
- [ ] Running `docker compose up` starts all services
- [ ] No additional tools need to be installed on host machine
- [ ] All services are accessible and healthy
- [ ] Documentation provides clear instructions for first-time setup

## User Story 2: Backend Code Changes Reflect Immediately
**As a** backend developer  
**I want to** see code changes reflected in running application without restart  
**So that** I can iterate quickly during development

**Acceptance Criteria:**
- [ ] Go source file changes trigger automatic rebuild
- [ ] Rebuild completes in less than 5 seconds
- [ ] No manual refresh or restart required
- [ ] Logs show compilation progress

## User Story 3: Frontend Code Changes Reflect Immediately
**As a** frontend developer  
**I want to** see UI changes reflected in browser without refresh  
**So that** I can iterate quickly on UI development

**Acceptance Criteria:**
- [ ] Svelte/TypeScript file changes trigger automatic rebuild
- [ ] Browser automatically refreshes with new content
- [ ] No manual refresh required
- [ ] Hot Module Replacement (HMR) works correctly

## User Story 4: Backend Connects to Database
**As a** backend developer  
**I want to** connect to PostgreSQL from my backend code  
**So that** I can start implementing data models

**Acceptance Criteria:**
- [ ] ace_db (PostgreSQL) container is running and accessible
- [ ] Backend can establish connection using environment variables
- [ ] Connection string is documented
- [ ] Database is initialized with schema if needed (auto migrations)

## User Story 5: Backend Connects to NATS
**As a** backend developer  
**I want to** connect to NATS messaging from my backend code  
**So that** I can implement event-driven architecture

**Acceptance Criteria:**
- [ ] ace_broker (NATS) container is running and accessible
- [ ] Backend can establish connection using environment variables
- [ ] Connection string is documented
- [ ] Test message can be published and subscribed

## User Story 6: Developer Can Run Common Commands
**As a** developer  
**I want to** have simple commands for common operations  
**So that** I don't need to remember complex docker commands

**Acceptance Criteria:**
- [ ] `make up` starts all services
- [ ] `make down` stops all services
- [ ] `make logs` shows aggregated logs
- [ ] `make clean` removes all containers and volumes

## User Story 7: Clean Startup Works from Scratch
**As a** new developer  
**I want to** clone the repo and start developing in under 10 minutes  
**So that** I can get started quickly

**Acceptance Criteria:**
- [ ] Clone repo on fresh machine
- [ ] Run `docker compose up`
- [ ] All services start without errors
- [ ] Complete process takes less than 10 minutes

## User Story 8: Data Persists Between Restarts
**As a** developer  
**I want my data to persist between container restarts  
**So that** I don't lose work when restarting services

**Acceptance Criteria:**
- [ ] PostgreSQL data persists via named volumes
- [ ] Data survives `make down` and `make up`
- [ ] Data survives container recreation

## User Story 9: Choose Between Docker and Podman
**As a** developer  
**I want to** choose my container runtime  
**So that** I can use my preferred tool

**Acceptance Criteria:**
- [ ] Set CONTAINER_ORCHESTRATOR env var to "docker" or "podman"
- [ ] Make commands use the specified orchestrator
- [ ] Works on Linux, macOS, Windows

## User Story 10: AI Agent Can Run Tasks
**As an** AI agent  
**I want to** execute specific tasks easily  
**So that** I can contribute to development

**Acceptance Criteria:**
- [ ] Make targets have clear, machine-readable names
- [ ] Logs are readable and informative
- [ ] Can run individual services for testing
- [ ] Code structure is agent-friendly (clear files, documentation)

## User Story 11: Dev Environment Mirrors Production
**As a** developer  
**I want** dev to use the same base images as production  
**So that** what I develop works the same in production

**Acceptance Criteria:**
- [ ] Same base Docker images in dev and prod
- [ ] Same environment variable structure
- [ ] Same service names and ports
