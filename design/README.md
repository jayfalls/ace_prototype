# ACE Framework — Design

## Purpose and Maintenance

This document is the single source of truth for understanding the ACE Framework as a system. Its audience is any agent or human who needs to build features without breaking existing architecture, disrespecting prior research, or going in the wrong direction.

**What belongs here:** the cognitive model, the high-level architecture, the key decisions that have already been made and why, and the interfaces of the shared packages that everything depends on. If a new unit introduces a shared interface, a new architectural pattern, or a fundamental constraint, this document must be updated when that unit merges.

**What does not belong here:** unit plans, implementation details, historical rationale for discarded approaches, or descriptions of the future that do not yet exist in code. Those live in `design/units/`.

**How to keep it lean:** before adding anything, ask whether an agent could build a feature incorrectly without knowing this. If the answer is no, it does not belong here. When updating, ask whether anything already here has been superseded and should be removed or rewritten. This document must never grow monotonically — it should be revised, not appended to.

---

## The Cognitive Model

This is the most important section. Misunderstanding it will cause you to build the wrong thing.

**ACE is an autonomous entity, not a chatbot.** The six cognitive layers process, plan, and act continuously regardless of whether any outside party is communicating with the agent. The layers are almost always processing — as soon as there is a message on either the northbound or southbound bus, the relevant layers will act on it. The cycle model is not event-driven in the traditional sense; it is closer to a continuous loop where layers respond to whatever is on the bus. When we reach the cognitive engine unit this will be tightened with optimisations, but the fundamental posture is continuous processing, not triggered processing.

**The Chat Interface is a broker between the agent's internals and the outside world.** It is not a layer. It handles all communication with anything external — humans, other systems, bots, APIs that push information — and translates that into terms the agent's Senses component understands. Critically, it is bidirectional: the outside world can push into the agent via the Chat Interface, and the cognitive layers can push information out through it when they need to surface something externally. Think of it as the agent's external-facing nervous system. Because it sits at this boundary, it is the only part of the system that needs to understand external communication protocols.

**Layer memory isolation exists to preserve cognitive independence.** Each layer has access to its own memory module and the global memory module only. This is a deliberate constraint: L1 (Aspirational) needs to maintain ethical judgment that is not contaminated by L6's operational details, and L6 (Task Prosecution) needs task focus that is not distracted by strategic deliberation.

**agentId threads through every message, row, span, and log line because attribution is the foundation of everything.** Cost must be traceable to the agent that incurred it. The Layer Inspector must be able to reconstruct any cognitive cycle for any agent. The Safety Monitor must be able to audit any agent's behaviour at any point in time. In a multi-agent swarm, debugging requires knowing which agent in which pod produced which output. If agentId is missing from any part of the pipeline, all of these capabilities silently break. There is no retrofitting attribution after the fact — it must be present from the first message.

---

## Architecture

### System Topology

```
┌────────────┐    REST / WebSocket    ┌──────────────────────────┐
│   ace_fe   │◄──────────────────────►│         ace_api          │
│  SvelteKit │                        │     Go / Chi  :8080      │
│   :5173    │                        └────────────┬─────────────┘
└────────────┘                                     │
                                        ┌──────────┴──────────┐
                                        │                     │
                                ┌───────▼──────┐   ┌──────────▼──────┐
                                │    ace_db    │   │   ace_broker    │
                                │ Postgres 18  │   │ NATS 2.12 + JS  │
                                │   :5432      │   │    :4222        │
                                └──────────────┘   └─────────────────┘
```

The API is the only service today. All future services communicate exclusively via NATS — never through a shared database connection or direct HTTP calls to each other. This constraint exists because services must be independently deployable and independently scalable. In a Kubernetes swarm, cognitive engine pods scale horizontally while the API remains a single entry point, and that independence requires a clean boundary.

The observability pipeline runs alongside all services: OTel Collector ingests traces via OTLP gRPC, metrics via Prometheus scrape, and logs via filelog from Docker stdout, routing them to Tempo, Prometheus, and Loki. Observability here is not just ops tooling — it directly powers user-facing product features. The Layer Inspector, cost dashboards, and reliability indicators all query the same pipeline. This is why the observability contracts (UsageEvent shape, span attribute names, log field names) are as stable as the API contracts.

### Agent Swarm Architecture

ACE is designed to scale from a single agent (N=1, hobbyist Docker Compose) to a hierarchical swarm (N=1000+, Kubernetes) without changing any code. The architecture mirrors hierarchical agent swarm thinking: just as a single ACE agent has six cognitive layers in a chain of command from Aspirational to Task Prosecution, a swarm of agents is organised into a **pod tree** where each pod is itself a chain of command.

A **pod** is a collection of agents with a configurable hierarchy and a defined upper bound on the number of agents it contains. The upper bound matters for two reasons: it prevents too many agents from competing on the same resources, and it enforces intentional role assignment within a pod to avoid redundancy. The **root pod** is always a single pod — it is the top of the swarm's chain of command. Every layer of pods below the root can **horizontally scale** by having sibling pods, each with their own children. This forms a pod tree where the root is singular but any branch can fan out. A strategic pod at the top of the tree delegates to coordinator pods in the middle, which delegate to worker pods at the leaves, intentionally mirroring the single-agent cognitive layer structure at macro scale so the same reasoning principles apply at both levels.

Each pod's chain of command is **fully configurable** — there is no hardcoded hierarchy. A deployment can have flat pods of parallel workers, deep hierarchies, or arbitrary trees. Because a single-agent deployment is just a swarm of size one with a degenerate pod topology, there is no separate codepath.

### Memory Architecture

Memory is tiered because different timescales of context require different retrieval strategies. Injecting everything always would overflow context windows and drown signal in noise; querying everything on demand would miss the persistent background context that makes a cognitive agent coherent over time. Every token budget in the memory system is configurable — and this applies to any component that contributes tokens to a context window, not just memory tiers.

A single agent's memory unit has four tiers:

**L1 — Immediate term (default 200 tokens).** No summarisation. Verbatim content covering the last few minutes. Updated every iteration. This is the highest-fidelity, shortest-horizon tier — think of it as a scratchpad of what just happened. It is always injected.

**L2 — Short term (default 500 tokens).** Summaries covering hours-to-days time horizons. Updated every few iterations. Always injected.

**L3 — Medium term (default 1500 tokens).** Summaries covering weeks-to-months time horizons. Updated every few dozen iterations. Always injected.

**L4 — Long term (default 3000 tokens).** Pull-based. A tree of nodes in PostgreSQL, each with content, a compressed summary, tags, an importance score, and a parent reference. Retrieved via forward tree traversal from abstract root nodes to concrete leaf nodes, followed by a tag-based search. Results are merged, deduplicated, and truncated to the configured token budget. Updates from active processing are pushed every iteration and batched for consolidation by the Memory Manager global loop, which generates summaries, computes importance scores, and prunes low-importance nodes.

**The memory system scales with the swarm.** A complete memory unit exists for each layer of a single agent, and then for the agent as a whole (cross-layer), and then for each pod, and then for each layer of pods in the tree, and then a global memory module for the entire swarm. The same four-tier structure applies at each level. This means that a worker pod's L1 captures recent tactical decisions, a coordinator pod's L3 captures strategic context over weeks, and the global swarm memory holds the mission and constraints that govern all pods. The hierarchy ensures that higher-level context propagates downward while lower-level operational detail does not pollute strategic memory.

### Layer Loops

Each cognitive layer runs **N configurable concurrent loops**, not one. The exact upper bound and usage pattern of loops for each layer is set in the agent configuration — some layers may only need a single loop, while others benefit from many running concurrently. This is intentionally granular so that deployments can tune the balance between capability, cost, and resource consumption for each layer independently.

Task Prosecution (L6) can run up to a configured maximum of concurrent loops simultaneously because it is the action layer — parallelism here means the agent can pursue multiple tasks or subtasks at once. Higher layers benefit from fewer concurrent loops because they are deliberative rather than executive. Each loop tracks its own `max_iterations` and `max_time_seconds` limits as configured. A loop monitor tracks cumulative token consumption per layer to detect and terminate runaway loops that exceed configured budgets.

### Global Loops

Global loops exist at three levels: **per-agent** (running for each individual agent), **per-pod** (running for the pod as a whole), and **per-swarm** (running across all pods). Each level has its own instance of each loop type, allowing concerns to be addressed at the appropriate scope.

The **Chat Interface** handles all communication between the agent's internals and the outside world — humans, bots, and external systems alike. It is bidirectional: the outside world pushes information in via senses, and the cognitive layers can push information out through it when they need to surface something externally. It acts as a broker, translating between the agent's internal representations and whatever external protocol is in use.

The **Safety Monitor** batch-processes messages for efficiency rather than evaluating every message individually as it arrives. The default mode is reactive — it processes batches and curves issues after detecting a pattern — because proactive evaluation of every message would be prohibitively expensive. Whether the monitor runs in reactive batch mode or proactive per-message mode is configurable, and the configuration can be granular: different behaviours can be set for different subject areas, different agent types, or different risk levels. This allows deployments to apply expensive proactive checking only where the stakes warrant it.

The **Memory Manager** consolidates memory writes, generates summaries, deduplicates overlapping memories, recomputes importance scores, and maintains the tree structure. At the agent level it manages the individual agent's four-tier memory. At the pod level it consolidates shared pod memory. At the swarm level it maintains the global memory module.

The **Swarm Coordinator** manages pod topology at the swarm level: routing tasks between pods, allocating shared resources such as LLM rate limit budgets across agents, and coordinating inter-pod communication. At the pod level it manages agent assignments and the pod's internal chain of command.

The **Learning Loop** processes feedback signals from completed cycles — explicit ratings, implicit signals like task completion, and evaluation suite results — to improve future performance. It is also responsible for maintaining each agent's tools and skills: adding new capabilities when the agent's context warrants them, retiring tools that are no longer useful, and updating skill configurations based on observed performance.

---

## Shared Package Interfaces

### `shared/messaging`

```go
// Create a client on service startup
client, err := messaging.NewClient(messaging.Config{
    URLs:          "nats://ace_broker:4222",
    Name:          "my-service",
    Timeout:       10 * time.Second,
    MaxReconnect:  5,
    ReconnectWait: 2 * time.Second,
})

// Publish — always use this wrapper, never client.Publish directly.
// The wrapper creates the envelope and sets all mandatory headers
// (message_id, correlation_id, agent_id, cycle_id, source_service,
// timestamp, schema_version). Calling client.Publish directly bypasses
// this and produces incomplete envelopes.
err = messaging.Publish(client, subject, correlationID, agentID, cycleID, "my-service", payload)

// Publish with Subject type (recommended for type safety)
err = messaging.PublishWithSubject(client, messaging.SubjectEngineLayerInput, correlationID, agentID, cycleID, "my-service", payload, agentID, layerID)

// Request-reply pattern — use when you need a response
reply, err := messaging.RequestReply(client, subject, correlationID, agentID, cycleID, "my-service", payload, 30*time.Second)

// Request-reply with Subject type
reply, err := messaging.RequestReplyWithSubject(client, messaging.SubjectLLMRequest, correlationID, agentID, cycleID, "my-service", payload, 30*time.Second, agentID)

// Subscribe with automatic envelope parsing
sub, err := messaging.SubscribeWithEnvelope(client, subject, func(env *messaging.Envelope, data []byte) error {
    // env.AgentID, env.CycleID, env.CorrelationID are populated from headers
    return nil
})

// JetStream consumer — use this when at-least-once delivery matters
err = messaging.SubscribeToStream(ctx, client, "COGNITIVE", "consumer-name", subject, handler)

// JetStream with envelope parsing
err = messaging.SubscribeToStreamWithEnvelope(ctx, client, "COGNITIVE", "consumer-name", subject, func(env *messaging.Envelope, data []byte) error {
    return nil
})

// Health check — include in /health/ready alongside the database ping
err = client.HealthCheck() // verifies TCP connection AND JetStream AccountInfo

// Create or update all streams idempotently on startup
err = messaging.EnsureStreams(ctx, js)

// Reply to an incoming request (preserves correlation ID)
err = messaging.ReplyTo(client, incomingMsg, payload)

// Forward a message to a new subject (preserves envelope)
err = messaging.ForwardMessage(client, incomingMsg, newSubject)

// Create request envelope from incoming message (preserves correlation ID)
env := messaging.CreateRequestEnvelope(incomingMsg, agentID, cycleID, "my-service")
```

Subject constants are typed `Subject` values in `shared/messaging/subjects.go` with a `.Format(args...)` method for interpolation — never construct subject strings by hand, because typos produce silent routing failures with no compile-time detection.

**Available Subjects:**
- `SubjectEngineLayerInput` — `ace.engine.%s.layer.%s.input`
- `SubjectEngineLayerOutput` — `ace.engine.%s.layer.%s.output`
- `SubjectEngineLoopStatus` — `ace.engine.%s.loop.%s.status`
- `SubjectMemoryStore` — `ace.memory.%s.store`
- `SubjectMemoryQuery` — `ace.memory.%s.query`
- `SubjectMemoryResult` — `ace.memory.%s.result`
- `SubjectToolsInvoke` — `ace.tools.%s.%s.invoke`
- `SubjectToolsResult` — `ace.tools.%s.%s.result`
- `SubjectSensesEvent` — `ace.senses.%s.%s.event`
- `SubjectLLMRequest` — `ace.llm.%s.request`
- `SubjectLLMResponse` — `ace.llm.%s.response`
- `SubjectUsageToken` — `ace.usage.%s.token`
- `SubjectUsageCost` — `ace.usage.%s.cost`
- `SubjectSystemAgentsSpawn` — `ace.system.agents.spawn`
- `SubjectSystemAgentsShutdown` — `ace.system.agents.shutdown`
- `SubjectSystemHealth` — `ace.system.health.%s`

**JetStream Streams:**
- `COGNITIVE` — Cognitive engine messages (1GB, 24h retention)
- `USAGE` — LLM usage events (100MB, 30 days retention)
- `SYSTEM` — System events (10MB, work queue policy)
- `DLQ` — Dead letter queue for failed messages

### `shared/telemetry`

```go
// Initialise once on service startup
tel, err := telemetry.Init(ctx, telemetry.Config{
    ServiceName:  "api",
    Environment:  cfg.Environment,
    OTLPEndpoint: cfg.OTLPEndpoint,
})
defer tel.Shutdown(ctx)

// tel.Tracer  — trace.Tracer
// tel.Meter   — metric.Meter
// tel.Logger  — *zap.Logger pre-configured with service_name
// tel.Usage   — *UsagePublisher

// Health check — include in /health/ready alongside the database ping
err = telemetry.HealthCheck() // verifies OTLP exporter connection

// Attach correlation context to the logger for a request or cycle scope
logger := telemetry.LogFields{
    AgentID: agentID,
    CycleID: cycleID,
}.AddFields(tel.Logger)

// Spans must include agentId and cycleId so the Layer Inspector
// can correlate traces with cognitive cycles
ctx, span := tel.Tracer.Start(ctx, "operation.name")
telemetry.AddSpanAttributes(span, telemetry.SpanAttributes{
    AgentID:     agentID,
    CycleID:     cycleID,
    ServiceName: "api",
})
defer span.End()

// Propagate trace context across NATS so distributed traces
// connect across service boundaries
telemetry.InjectTraceContext(ctx, natsMsg)
ctx = telemetry.ExtractTraceContext(ctx, natsMsg)

// Extract trace context from HTTP headers
ctx = telemetry.ExtractHTTP(r.Context(), r.Header)

// Usage events feed cost attribution, the Layer Inspector, and billing.
// All return error — log failures, never propagate them.
err = tel.Usage.LLMCall(ctx, agentID, cycleID, sessionID, "api", tokens, costUSD, durationMs)
err = tel.Usage.MemoryRead(ctx, agentID, cycleID, sessionID, "api", durationMs)
err = tel.Usage.ToolExecute(ctx, agentID, cycleID, sessionID, "api", toolName, durationMs)
err = tel.Usage.Publish(ctx, telemetry.UsageEvent{
    AgentID:       agentID,
    OperationType: telemetry.OperationTypeDBQuery,
    ResourceType:  telemetry.ResourceTypeDatabase,
    DurationMs:    durationMs,
    Metadata:      map[string]string{"query": "select_agents"},
})

// Attach middleware to the Chi router
router.Use(telemetry.TraceMiddleware()) // extracts trace context from HTTP headers
router.Use(telemetry.MetricsMiddleware(cfg.ServiceName)) // records request metrics
router.Use(telemetry.LoggerMiddleware(cfg.ServiceName)) // logs HTTP requests
```

**Middleware Stack (attach in this order):**
1. `TraceMiddleware()` — Extracts W3C Trace Context from HTTP headers
2. `MetricsMiddleware(serviceName)` — Records request latency and error rates
3. `LoggerMiddleware(serviceName)` — Logs HTTP requests with trace context

**UsageEvent Operation Types:**
- `OperationTypeLLMCall` — LLM API calls
- `OperationTypeMemoryRead` — Memory reads
- `OperationTypeMemoryWrite` — Memory writes
- `OperationTypeToolExecute` — Tool executions
- `OperationTypeDBQuery` — Database queries
- `OperationTypeNATSPublish` — NATS publish operations
- `OperationTypeNATSSubscribe` — NATS subscribe operations

**UsageEvent Resource Types:**
- `ResourceTypeAPI` — API resources
- `ResourceTypeMemory` — Memory resources
- `ResourceTypeTool` — Tool resources
- `ResourceTypeDatabase` — Database resources
- `ResourceTypeMessaging` — Messaging resources

### `services/api` Handler Pattern

```go
func (h *AgentHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req CreateAgentRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        response.BadRequest(w, "invalid_request", "Invalid request body")
        return
    }
    if err := validator.ValidateStruct(req); err != nil {
        response.ValidationError(w, err)
        return
    }
    // All business logic lives in the service layer — never in the handler
    agent, err := h.service.CreateAgent(r.Context(), req)
    if err != nil {
        response.InternalError(w, "Failed to create agent")
        return
    }
    response.Created(w, agent)
}

// All responses use the standard envelope:
// success: { "success": true, "data": {...} }
// error:   { "success": false, "error": { "code": "...", "message": "...", "details": [...] } }
```

### Database Migrations

Migrations are Goose Go functions — never SQL files. Each file registers itself via `init()`.

```go
// backend/services/api/migrations/20240315120000_create_agents.go
package migrations

import (
    "database/sql"
    "github.com/pressly/goose/v3"
)

func init() {
    goose.AddMigration(upCreateAgents, downCreateAgents)
}

func upCreateAgents(tx *sql.Tx) error {
    _, err := tx.Exec(`
        CREATE TABLE agents (
            id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id    UUID         NOT NULL,
            name       VARCHAR(255) NOT NULL,
            status     VARCHAR(50)  NOT NULL DEFAULT 'idle',
            created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
        );
        CREATE TRIGGER set_agents_updated_at
            BEFORE UPDATE ON agents
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
        CREATE INDEX idx_agents_user_id ON agents(user_id);
    `)
    return err
}

func downCreateAgents(tx *sql.Tx) error {
    _, err := tx.Exec(`DROP TABLE IF EXISTS agents;`)
    return err
}
```

---

## Development Workflow

All development tasks are driven through the Makefile. The Makefile is the single entry point for all operations.

### Makefile Targets

| Target | Description | Usage |
|--------|-------------|-------|
| `make dev` | Full dev setup: clone agency-agents, setup distrobox, install deps | First-time setup |
| `make agent` | Enter distrobox and run OpenCode interactively | Start AI agent |
| `make agent-stop` | Stop OpenCode in distrobox | Stop AI agent |
| `make up` | Start all services in development mode | Start development |
| `make down` | Stop all services | Stop development |
| `make re` | Restart all services (down + up) | Restart services |
| `make build` | Build all service images | Build containers |
| `make test` | Run all tests in API and frontend containers | Run tests |
| `make logs` | View aggregated logs for all services | View logs |
| `make logs-api` | View logs for ace_api service | View API logs |
| `make logs-fe` | View logs for ace_fe service | View frontend logs |
| `make logs-db` | View logs for ace_db service | View database logs |
| `make logs-broker` | View logs for ace_broker service | View NATS logs |
| `make clean` | Remove all containers and volumes | Clean up |
| `make ps` | Show running containers | Check status |
| `make help` | Show help message | Get help |

### Environment Variables

The Makefile supports two environment variables:

- **ENVIRONMENT**: `dev` or `prod` (default: `dev`)
- **CONTAINER_ORCHESTRATOR**: `docker` or `podman` (default: `docker`)

Example usage:
```bash
make up ENVIRONMENT=prod CONTAINER_ORCHESTRATOR=podman
```

### Development Environment

The development environment uses:
- **Docker Compose**: Local development with all services
- **Distrobox**: Isolated development environment for OpenCode
- **Pre-commit hooks**: Automated quality gates before commits

Services accessible after `make up`:
- Frontend: http://localhost:5173
- API: http://localhost:8080
- PostgreSQL: localhost:5432
- NATS: localhost:4222
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000
- Loki: http://localhost:3100
- Tempo: http://localhost:3200
- OTel Collector: http://localhost:4317 (gRPC), http://localhost:4318 (HTTP)

### Testing

Tests are run via `make test`, which executes:
- Go integration tests in the API container
- Frontend tests in the Frontend container

The pre-commit hook runs quality gates including:
- Go build, lint, and unit tests
- Frontend lint and tests
- Docker Compose validation
- Makefile validation

---

## Constraints That Cannot Be Broken

**agentId belongs everywhere on the agent processing path** — NATS envelope headers, database rows, OTel span attributes, and log lines. Without it, cost attribution, the Layer Inspector, swarm debugging, and the Safety Monitor's audit trail all silently break.

**Usage events are non-negotiable.** Every LLM call, memory read, tool execution, and database query on the agent processing path emits a UsageEvent to `ace.usage.event`. This is not monitoring overhead — it is the data layer that powers user-facing features.

**Services communicate via NATS, not HTTP or shared databases.** This keeps services independently deployable and independently scalable.

**`shared/` packages are transport-agnostic.** They never import `net/http`, NATS, or any transport. HTTP adapters live in `services/api/internal/middleware/`. This allows shared packages to be imported by any future service without dragging in irrelevant dependencies.

**No `interface{}` or `any`.** Explicit types throughout — `map[string]string` not `map[string]interface{}`.

**No else chains.** Early returns only.

**Immutable records for configs and prompts.** Agent configurations and layer prompts are never updated in place — new versioned rows are inserted. Understanding why an agent behaved a particular way on a particular day requires knowing exactly what prompt was active at that moment. Without immutability, that history is lost.

**SQLC generated files are never hand-edited.** Modify the `.sql` query file and re-run `sqlc generate`.

**All operations go through the Makefile.** Never run docker commands, go commands, or npm commands directly — use the Makefile targets. This ensures consistency across environments and prevents configuration drift.

**Pre-commit hooks are mandatory.** The pre-commit hook runs quality gates before every commit. Never bypass with `--no-verify` unless the failure is unrelated to your changes and you understand the consequences.

---

## Unit Status

This section tracks the completion status of each design unit. Units are completed in order of dependencies.

### Completed Units

| Unit | Status | Description |
|------|--------|-------------|
| **Architecture Planning** | ✅ Complete | System architecture and design patterns |
| **Core Infrastructure** | ✅ Complete | Foundation services and infrastructure |
| **API Design** | ✅ Complete | API patterns, structure, tools, and libraries |
| **Integrate Agent Tools (Agency Agents)** | ✅ Complete | Agent tool integration patterns |
| **Messaging Paradigm** | ✅ Complete | NATS communication contracts (`shared/messaging`) |
| **OpenCode Migration** | ✅ Complete | OpenCode integration and migration |
| **Observability** | ✅ Complete | Observability primitives (`shared/telemetry`) |

### In Progress Units

| Unit | Status | Description |
|------|--------|-------------|
| **Database Design & API/DB Documentation** | 🔄 In Progress | Database design documentation and API/DB specification |

### Planned Units — Foundation

| Unit | Status | Description |
|------|--------|-------------|
| **Caching Strategies** | 📋 Planned | Caching patterns and implementation strategies |
| **Users & Auth (JWT, SSO)** | 📋 Planned | Authentication and authorization system |
| **Auditing** | 📋 Planned | Audit logging and compliance tracking |
| **Security (Certs, TLS, HTTPS, etc)** | 📋 Planned | Security infrastructure and encryption |
| **CI/CD (PromptFoo)** | 📋 Planned | Continuous integration and deployment pipeline |
| **Frontend Design (Impeccable)** | 📋 Planned | SvelteKit user interface design and implementation |
| **Production Deployment (End User Experience)** | 📋 Planned | Production deployment and end-user setup |

### Planned Units — Research

| Unit | Status | Description |
|------|--------|-------------|
| **Existing Agents Study** | 📋 Planned | Study of OpenClaw, Claude Code, Open Code, Oh My OpenAgent, Devin |

### Planned Units — Core Cognitive

| Unit | Status | Description |
|------|--------|-------------|
| **Providers** | 📋 Planned | Testing, sequencing, groups, rate limits, usage & cost tracking |
| **Cognitive Engine** | 📋 Planned | 6 ACE layers with NATS inter-layer communication, start up sequence and validation, shut down, failure cases, Loops |
| **Layer Inspector** | 📋 Planned | Cognitive cycle inspection and debugging |
| **Senses (Inputs)** | 📋 Planned | Input handling and sanitisation layer |
| **Tools (Outputs & Interfaces)** | 📋 Planned | Output handling and tool interfaces |
| **Memory** | 📋 Planned | Per-layer + global memory modules (L1, L2, L3, L4), context poisoning mitigations, RLM integration |
| **Skills** | 📋 Planned | Agent skills system (agentskills.io specification) |

### Planned Units — Advanced

| Unit | Status | Description |
|------|--------|-------------|
| **Global Layers** | 📋 Planned | Global cognitive layer coordination |
| **Agent Switching** | 📋 Planned | Dynamic agent switching and context transfer |
| **Multi Agent** | 📋 Planned | Multi-agent orchestration and pod management |
| **Env Set Up** | 📋 Planned | LLM managed installer script for spot instances |
| **Integrate Auto Research into Agent** | 📋 Planned | Auto-research integration (self-improvement, actions, fine-tuning) |

---

## References

- [Units](units/README.md) - Individual unit documentation
- [Source](source.md) - ACE Framework research and theory
- [AGENTS.md](../AGENTS.md) - Agent instructions and coding standards
