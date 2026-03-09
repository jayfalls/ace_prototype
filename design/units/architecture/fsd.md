# Functional Specification Document

## Overview
Define the structural components and their interactions for the ACE Framework MVP.

## Technology Stack

| Component | Technology | Rationale |
|-----------|------------|-----------|
| Language | Go (all components) | Full control, no library constraints, best for custom cognitive layers |
| API | Gin | Fast, minimal, great for websockets |
| Frontend | SvelteKit + TypeScript | Best developer experience, minimal code, easiest to maintain quality |
| Database | PostgreSQL + SQLC | Type-safe SQL, no ORM overhead |
| Real-time | WebSockets | Native to Go, for cognitive trace updates |
| Auth | JWT (now) / oauth2-proxy (later) | Simple start, scales to OAuth |
| Message | NATS | For scaling to agent swarms |
| Migrations | golang-migrate | Schema evolution |

## Components

### Core Components

#### 1. API Service
- **Responsibility**: Entry point for all requests, authentication, websocket handling, cognitive orchestration
- **Technology**: Go + Gin
- **Interfaces**: REST/HTTP endpoints, WebSockets

#### 2. Frontend
- **Responsibility**: User interface for interacting with agents
- **Technology**: SvelteKit + TypeScript
- **Interfaces**: Web UI, WebSocket client

#### 3. Cognitive Engine
- **Responsibility**: Implements the 6 ACE Framework layers
- **Components**:
  - Moral Reasoning Layer
  - High-Level Planning Layer
  - Low-Level Planning Layer
  - Strategic Layer
  - Tactical Layer
  - Operational Layer
- **Technology**: Go (no external AI libraries - full custom control)
- **Interfaces**: WebSocket for real-time updates, NATS for async

#### 4. Persistence Layer
- **Responsibility**: Data storage and retrieval
- **Technology**: PostgreSQL + SQLC
- **Interfaces**: Type-safe SQL queries

#### 5. Message Layer
- **Responsibility**: Asynchronous communication between components
- **Technology**: NATS
- **Interfaces**: Publish/subscribe, request/reply

### Component Boundaries

| Component | Responsibility | Owns |
|-----------|---------------|------|
| API Service | Request ingress, auth, websocket, orchestration | HTTP/WebSocket endpoints, auth, routing |
| Frontend | UI rendering | Svelte components, stores |
| Cognitive Engine | Decision making | ACE layer implementations |
| Persistence | Data storage | PostgreSQL, SQLC queries |
| Message Layer | Event routing | NATS connections, pub/sub |

## Communication Patterns

### Synchronous (Request-Response)
- Client → API → Cognitive Engine → Persistence → Response

### Asynchronous (Real-time)
- Client ↔ WebSocket ↔ Cognitive Engine (for thought traces)

### Event-Based (Scaling)
- Cognitive Engine → NATS → [any component]

### Data Flow

```
Client Request
    ↓
API (auth, validate)
    ↓
Cognitive Engine (process)
    ↓
[optional] NATS (async tasks)
    ↓
Persistence (store/retrieve)
    ↓
Response
```

## Container Strategy

### Containers
- **api**: Go + Gin service
- **frontend**: SvelteKit (served by API or static)
- **cognitive-engine**: Go cognitive processing
- **nats**: Message broker
- **postgres**: Database

### Development Mode (Docker Compose)
- All services in docker-compose.yml
- Hot reload enabled

### Production Mode (Kubernetes)
- Each component as a pod
- PostgreSQL as managed service or statefulset
- NATS for inter-agent communication

## Out of Scope for This FSD
- Database schemas (deferred to unit-specific FSDs)
- API endpoint definitions (deferred to API unit)
- Deployment configuration (deferred to deployment unit)
