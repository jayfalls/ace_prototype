# Design

This is the main living design document for the ACE Framework MVP.

## Overview

The ACE Framework is a conceptual cognitive architecture for building ethical autonomous agents.

- [Source](source.md) - ACE Framework research and theory
- [Units](units.md) - Feature/component definitions and templates

## 1. Architecture

### High-Level Overview

The ACE Framework consists of:
- **Telemetry (Senses)** - Input handling (chat, sensors, metrics, webhooks)
- **Cognitive Engine** - 6 ACE layers with NATS for northbound/southbound communication
- **Actuators (Outputs)** - Output handling (chat, tools, signals, export)
- **Memory** - Per-layer + global memory modules

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
│         │              │   Auth (JWT)  │                 │              │
│         │              │  WebSocket    │                 │              │
│         └──────────────┼───────────────┼─────────────────┘              │
│                        │               │                                  │
│         ┌──────────────┼───────────────┼───────────────────────────┐    │
│         │         Telemetry/Senses                            │    │
│         │  Inputs: Chat | Sensors | Metrics | Webhooks       │    │
│         └──────────────┬───────────────┬───────────────────────────┘    │
│                        │               │                                  │
│                        ▼               ▼                                  │
│                 ┌───────────┐   ┌───────────┐                          │
│                 │PostgreSQL │   │   NATS    │                          │
│                 │  + SQLC   │   │(Pub/Sub)  │                          │
│                 └───────────┘   └───────────┘                          │
│                        │               │                                  │
│                        ▼               ▼                                  │
│                 ┌─────────────────────────────────────────────────────┐   │
│                 │              Actuators (Outputs)                    │   │
│                 │  Chat | Tools | Signals | Export                   │   │
│                 └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
```

### Core Components

| Component | Responsibility |
|-----------|---------------|
| **Frontend** | User interface, real-time updates (SvelteKit) |
| **API (Gin)** | HTTP routes, auth, websocket, orchestration |
| **Cognitive Engine** | 6 ACE layers with NATS inter-layer communication |
| **Telemetry (Senses)** | Input handling: chat, sensors, metrics, webhooks |
| **Actuators (Outputs)** | Output handling: chat, tools, signals, export |
| **Memory** | Per-layer + global modules (long/medium/short term) |
| **Message Broker (NATS)** | Inter-layer communication (northbound/southbound) |
| **Persistence** | PostgreSQL + SQLC |

### Layer Communication

- **NATS** for inter-layer (northbound/southbound buses)
- Each message includes: `timestamp`, `cycle_id`, `layer_id`
- Multiple messages per cycle aggregated at cycle boundary
- Variable layer speeds handled via async NATS messaging

### Loops

**Within Layers:**
- Configurable loops (task prosecution: infinite, planning: finite)
- Max loops, max cycles, max time per loop defined in config
- Pull-based status updates, output on completion

**Global Loops (HRM):**
- Chat Interface (fast) - human interaction
- Safety Monitor (fast) - threat detection
- Swarm Coordinator (medium) - multi-agent
- Memory Manager (slow) - consolidation
- Learning Loop (medium) - feedback integration

### Memory Architecture

Each layer has its own memory module + global module:
- **Long-term**: Tree structure with tags, query via tree traversal + tag search
- **Medium-term**: Always injected
- **Short-term**: Always injected
- **Isolation**: Layer only accesses own module + global module

### Container Architecture

**Single Agent Mode:**
- frontend (:5173), api (:8080), telemetry (:8081), nats (:4222), postgres (:5432)

**Kubernetes (Multi-Agent):**
- Frontend, API, Telemetry (Deployments)
- NATS (StatefulSet)
- PostgreSQL (Managed)
- Cognitive Engine pods

See [units/architecture/architecture.md](units/architecture/architecture.md) for detailed diagrams and specifications.

## 2. Technologies

### Backend
- **Go** - Primary language for API and Cognitive Engine
- **Gin** - HTTP web framework
- **SQLC** - Type-safe SQL access to PostgreSQL
- **NATS** - Message broker for inter-layer communication

### Frontend
- **SvelteKit** - Full-stack web framework
- **TypeScript** - Type-safe frontend code

### Database
- **PostgreSQL** - Primary data store
- **SQLC** - Compile-time SQL type checking

### Authentication
- **JWT** - Token-based authentication
- **oauth2-proxy** - OAuth integration (future)

### Infrastructure
- **Docker** - Containerization
- **Kubernetes** - Orchestration for multi-agent deployments
- **WebSocket** - Real-time communication

## 3. Data Model

The data model is defined in the Core Infrastructure unit.

### Core Entities
- **Agent**: Autonomous cognitive entity (id, name, description, config, status)
- **Memory**: Long-term memory with tree structure (id, parent_id, content, tags, memory_type, importance)
- **Session**: User-agent interaction sessions (id, agent_id, user_id, status, context)
- **Thought**: Individual thought records for debugging/traceability (id, session_id, layer, cycle, content)
- **User**: User accounts (id, email, name, password_hash)
- **LLMProvider**: LLM configurations (id, name, api_key, base_url, default_model)
- **LLMAttachment**: LLM to layer/component mapping (id, agent_id, provider_id, target_type, target_id, model)
- **AgentSetting**: Agent-specific settings (id, agent_id, key, value)
- **SystemSetting**: Global system settings (id, key, value, is_secret)
- **AgentToolWhitelist**: Per-agent tool whitelist (id, agent_id, tool_source, tool_name, enabled)

### Relationships
- Agent 1:N Memories
- Agent 1:N Sessions
- Session 1:N Thoughts
- Agent N:N LLMProvider (via LLMAttachment)
- Agent N:N Tools (via AgentToolWhitelist)

## 4. API

The API structure is defined in the Core Infrastructure unit.

### REST API
- **Agents**: CRUD + lifecycle (start/stop)
- **Memories**: CRUD + search
- **Sessions**: CRUD + thought traces
- **LLM Providers**: CRUD
- **Settings**: Agent + system level
- **Tools**: Whitelist management

### WebSocket
- Real-time thought streaming
- Agent status updates

## 5. Frontend

### Tech Stack
- **SvelteKit**: Full-stack framework
- **TypeScript**: Type-safe code

### Features
- User authentication UI (login/register)
- Agent management (create, configure, start, stop, delete)
- Real-time chat interface
- Live thought trace visualization
- Memory browser and search
- Settings management

## 6. Deployment

<!--
NOTE: Document the deployment strategy.
Should include: Docker configuration, CI/CD, environments.
-->

## 7. Security

Security is defined in the Core Infrastructure unit.

### Authentication
- JWT-based stateless authentication
- Token includes: `user_id`, `exp`, `roles`
- Token refresh before expiration

### Protected Routes
All routes require authentication except:
- `POST /api/auth/register` - Registration
- `POST /api/auth/login` - Login
- `GET /api/tools/sources` - List available tool sources

### Data Protection
- SQL injection prevention via SQLC (parameterized queries)
- XSS prevention on user inputs
- API keys encrypted in database

## 8. Testing

<!--
NOTE: Document the testing strategy.
Should include: Test types, coverage targets, testing tools.
-->

## 9. Monitoring

<!--
NOTE: Document observability.
Should include: Logging, metrics, alerting, dashboards.
-->
