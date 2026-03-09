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

<!--
NOTE: Document the technology stack and tools used in the project.
Should include: Python version, package manager (uv), databases, Docker, message brokers, etc.
Depends on: Architecture (to know what's needed)
-->

## 3. Data Model

<!--
NOTE: Document the database schema, entities, and relationships.
Should include: Schema diagrams, entity relationships, data flow.
-->

## 4. API

<!--
NOTE: High-level API overview - detailed endpoints are defined by each unit.
-->

## 5. Frontend

<!--
NOTE: Document the UI/UX design.
Should include: Component library, styling approach, state management.
-->

## 6. Deployment

<!--
NOTE: Document the deployment strategy.
Should include: Docker configuration, CI/CD, environments.
-->

## 7. Security

<!--
NOTE: Document security considerations.
Should include: Authentication, authorization, data protection.
-->

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
