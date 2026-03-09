# Architecture

## High-Level Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              ACE Framework                               │
│                                                                          │
│  ┌──────────────┐      ┌──────────────┐      ┌──────────────────────┐  │
│  │   Frontend   │      │    API       │      │   Cognitive Engine  │  │
│  │  SvelteKit   │◄────►│     Go       │◄────►│        Go           │  │
│  │   (Web UI)   │      │    (Gin)     │      │   (6 ACE Layers)   │  │
│  └──────┬───────┘      └──────┬───────┘      └──────────┬───────────┘  │
│         │                      │                         │              │
│         │              ┌───────┴───────┐                 │              │
│         │              │               │                 │              │
│         │         ┌────▼────┐    ┌─────▼─────┐   ┌─────▼─────┐      │
│         │         │   Auth   │    │  WebSocket │   │   NATS    │      │
│         │         │   (JWT)  │    │  Handler   │   │  (Async)  │      │
│         │         └──────────┘    └────────────┘   └─────┬─────┘      │
│         │                                               │              │
│         │                                               ▼              │
│         │                                      ┌──────────────────┐     │
│         │                                      │  Python Workers  │     │
│         │                                      │  (LLM/AI)       │     │
│         │                                      └──────────────────┘     │
│         │                                              │              │
│         └──────────────────────────────────────────────┼──────────────┘
│                                                    ┌──▼──────────────┐
│                                                    │  PostgreSQL     │
│                                                    │  + SQLC         │
│                                                    └─────────────────┘
└─────────────────────────────────────────────────────────────────────────┘
```

## Component Diagram

### Core Components

| Component | Responsibility | Public API |
|-----------|---------------|------------|
| **Frontend** | User interface, real-time updates | Static + WebSocket |
| **API (Gin)** | HTTP routes, auth, websocket upgrade, orchestration | REST + WS |
| **Cognitive Engine** | 6 ACE layer processing, LLM calls | Internal |
| **WebSocket Handler** | Real-time thought trace streaming | WebSocket |
| **Auth** | JWT validation, session management | Middleware |
| **Persistence** | Data storage via SQLC | SQL queries |
| **Message Layer** | Async communication | NATS pub/sub |

### Container Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Development (Docker Compose)                   │
│                                                                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐   │
│  │   frontend  │  │     api     │  │    nats     │  │   postgres  │   │
│  │  :5173      │  │   :8080     │  │  :4222      │  │   :5432     │   │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘   │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│                      Production (Kubernetes)                           │
│                                                                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐   │
│  │   frontend  │  │     api     │  │    nats     │  │  postgres   │   │
│  │  (Deployment)│ │  (Deployment)│ │  (StatefulSet)│ │(Managed)   │   │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘   │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                     cognitive-engine                            │    │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐           │    │
│  │  │ ace-pod │  │ ace-pod │  │ ace-pod │  │ ace-pod │  ...     │    │
│  │  │ :8081   │  │ :8081   │  │ :8081   │  │ :8081   │           │    │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘           │    │
│  └─────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────┘
```

## Data Flow

### Primary Flow (REST)

```
User → Frontend → API (Gin) → Cognitive Engine → PostgreSQL → Response
```

### Real-Time Flow (WebSocket)

```
User → Frontend → WebSocket → Cognitive Engine → Thought Stream → User
                                      ↓
                               PostgreSQL (persist)
```

### Async Flow (NATS)

```
Cognitive Engine → NATS → Worker (LLM) → NATS → Cognitive Engine → User
```

## Sequence Diagram

```mermaid
sequenceDiagram
    participant User
    participant Frontend
    participant API
    participant CognitiveEngine
    participant Database
    participant NATS
    participant Worker

    Note over User,Worker: REST Flow
    User->>Frontend: Open page
    Frontend-->>User: Render UI
    
    User->>Frontend: Send message
    Frontend->>API: POST /api/chat
    API->>CognitiveEngine: Process request
    CognitiveEngine->>Database: Save conversation
    Database-->>CognitiveEngine: Confirm
    CognitiveEngine-->>API: Response
    API-->>Frontend: JSON response
    Frontend-->>User: Update UI

    Note over User,Worker: WebSocket Flow
    User->>Frontend: Connect ws://api/ws
    Frontend->>API: WS Upgrade
    API->>Frontend: WS Connected
    
    User->>Frontend: Send message
    Frontend->>API: WS message
    API->>CognitiveEngine: Stream request
    
    loop Thought Processing
        CognitiveEngine->>NATS: Publish thought event
        NATS->>Worker: Process with LLM
        Worker-->>NATS: LLM Response
        NATS->>CognitiveEngine: Response
        CognitiveEngine->>API: Thought update
        API->>Frontend: WS frame (thought)
        Frontend-->>User: Real-time update
    end
    
    CognitiveEngine->>Database: Save final response
    Database-->>CognitiveEngine: Confirm
```

## Integration Points

### External Integrations

| Service | Integration Type | Purpose |
|---------|-----------------|---------|
| LLM Providers (OpenAI, Anthropic, Ollama) | HTTP API | LLM inference |
| OAuth Providers (future) | OAuth2 | User authentication |

### Internal Integrations

| Component | Interface | Data Exchanged |
|-----------|-----------|----------------|
| Frontend ↔ API | REST + WebSocket | JSON, text stream |
| API ↔ Database | SQLC queries | Structured data |
| API ↔ NATS | NATS client | Events, async tasks |
| Cognitive Engine ↔ Workers | NATS pub/sub | Thought events, LLM requests |

## Event Flow

| Event | Producer | Consumer | Payload |
|-------|----------|----------|---------|
| `thought.start` | Cognitive Engine | Frontend (WS) | `{ agent_id, request_id, layer }` |
| `thought.update` | Cognitive Engine | Frontend (WS) | `{ request_id, thought, layer, metadata }` |
| `thought.complete` | Cognitive Engine | Frontend (WS) | `{ request_id, final, metrics }` |
| `llm.request` | Cognitive Engine | Worker | `{ request_id, prompt, model, options }` |
| `llm.response` | Worker | Cognitive Engine | `{ request_id, response, tokens }` |
| `agent.created` | API | Database | `{ agent_id, config }` |
| `agent.message` | API | Cognitive Engine | `{ agent_id, message, session_id }` |

## System Boundaries

- **Trusted Zone**: API, Cognitive Engine, Database
  - Internal communication within the cluster
  - JWT-authenticated requests
  
- **Untrusted Zone**: Frontend, External LLM Providers
  - Client-side code (browser)
  - External API calls (LLM providers)

## Security Architecture

### Authentication
- JWT tokens for API authentication
- Token validation middleware on all protected routes
- Future: oauth2-proxy for OAuth integration

### Authorization
- Role-based access (future)
- Agent ownership validation
- Session-based authorization

### Data Protection
- HTTPS in production
- SQL injection prevention via SQLC (parameterized queries)
- Input validation on all API endpoints
- Rate limiting (future)

## Network Architecture

### Development
```
localhost:5173 (Frontend) 
    ↓ 
localhost:8080 (API) 
    ↓ 
localhost:5432 (PostgreSQL)
localhost:4222 (NATS)
```

### Production (K8s)
```
Internet → LoadBalancer → frontend (443)
                       → api (443)
                       → nats (443)
                       → postgres (managed)
```
