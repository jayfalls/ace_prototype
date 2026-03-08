# Functional Specification Document

## Overview
Define the structural components and their interactions for the ACE Framework MVP.

## Components

### Core Components

#### 1. API Service
- **Responsibility**: Entry point for all external requests, authentication, rate limiting
- **Technology**: FastAPI (built into Python backend)
- **Interfaces**: REST/HTTP endpoints, OpenAPI docs

#### 2. Frontend
- **Responsibility**: User interface for interacting with the system
- **Technology**: HTMX + Alpine.js + Tailwind (served by API or static)
- **Interfaces**: Web UI, HTMX for dynamic content

#### 3. Cognitive Engine
- **Responsibility**: Implements the 6 ACE Framework layers
- **Components**:
  - Moral Reasoning Layer
  - High-Level Planning Layer
  - Low-Level Planning Layer
  - Strategic Layer
  - Tactical Layer
  - Operational Layer
- **Technology**: Python + LangChain/LlamaIndex
- **Interfaces**: Internal API, message-based communication

#### 4. Persistence Layer
- **Responsibility**: Data storage and retrieval
- **Technology**: PostgreSQL (all environments)
- **Interfaces**: SQL via ORM (SQLAlchemy or similar)

#### 5. Message Layer
- **Responsibility**: Asynchronous communication between components
- **Technology**: NATS
- **Interfaces**: Publish/subscribe, request/reply patterns

### Component Boundaries

| Component | Responsibility | Owns |
|-----------|---------------|------|
| API Service | Request ingress, auth, routing | HTTP endpoints, auth, OpenAPI |
| Frontend | UI rendering | HTMX templates, Tailwind styles |
| Cognitive Engine | Decision making | ACE layer implementations, LangChain |
| Persistence | Data storage | Database, SQLAlchemy models |
| Message Layer | Event routing | NATS connections, pub/sub |

## Communication Patterns

### Synchronous (Request-Response)
- Client → API Service → Cognitive Engine → Persistence → Response

### Asynchronous (Event-Based)
- Cognitive Engine → NATS → [any component]
- Components communicate via NATS for loose coupling

### Data Flow

```
Client Request
    ↓
API Gateway (auth, validate)
    ↓
Cognitive Engine (process)
    ↓
[optional] Message Layer (async tasks)
    ↓
Persistence (store/retrieve)
    ↓
Response
```

## Container Strategy

### Containers
- **api**: FastAPI service (serves both API + frontend)
- **cognitive-engine**: Core ACE processing
- **nats**: Message broker
- **postgres**: Database

### Development Mode (Docker Compose)
- All services in docker-compose.yml
- PostgreSQL for persistence
- Hot reload enabled

### Production Mode (Kubernetes)
- Each ACE cognitive engine as a pod
- PostgreSQL as managed service or statefulset
- NATS for inter-agent communication

## Technical Decisions

| Component | Technology | Rationale |
|-----------|------------|-----------|
| Language | Python + UV | AI ecosystem (LangChain, LlamaIndex), UV for fast deps |
| Database | PostgreSQL | Robust, scalable, K8s-native |
| Message Broker | NATS | Lightweight, native K8s support, perfect for agent swarm |
| Frontend | HTMX + Alpine.js + Tailwind | Minimal custom code, server-side rendered |
| API | Built into Python (FastAPI) | Lightweight, auto-docs, works with Python ecosystem |

### Development Mode (Single Machine)
- Run all components via Docker Compose
- PostgreSQL in container

### Production Mode (Kubernetes)
- Each ACE runs as a pod
- PostgreSQL (managed or self-hosted)
- NATS for inter-agent communication

## Out of Scope for This FSD
- Database schemas (deferred to unit-specific FSDs)
- API endpoint definitions (deferred to API unit)
- Deployment configuration (deferred to deployment unit)
