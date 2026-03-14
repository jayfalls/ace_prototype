# Problem Space

## Initial Discovery

### Core Questions

**Q: What problem are we trying to solve?**
A: The ACE Framework will run as a distributed multi-service system with autonomous agents executing cognitive cycles, making LLM calls, querying memory, invoking tools, and emitting thousands of events per minute. Without a shared observability foundation established before those services are built, each service will instrument itself independently — inventing its own log formats, its own metric names, its own trace span conventions, and its own usage event shapes. The result is a system that is technically observable in isolation but impossible to reason about as a whole: you cannot trace a user message through the API, into the cognitive engine, across six layers, through an LLM call, and back out again if every service speaks a different observability dialect.

This unit establishes the shared observability primitives in shared/telemetry before any feature service is built. Like the messaging unit established NATS communication contracts, this unit establishes the contracts for how every service reports what it is doing, what it is consuming, and what it is costing. Every subsequent service inherits these primitives rather than inventing its own.

**Q: Who are the users?**
A: 
- Backend services (consuming shared/telemetry package)
- Frontend (SvelteKit app consuming frontend telemetry module)
- Operators/Devs (using Grafana dashboards, Tempo traces, Loki logs)
- End users (consuming product features built on observability data: cost dashboards, layer inspector, reliability indicators)

**Q: What are the success criteria?**
A: 
1. Every subsequent service uses shared/telemetry for all observability needs (no custom instrumentation)
2. Distributed traces can be followed from API through cognitive engine, through all layers, through LLM calls, and back
3. Usage events flow through NATS to PostgreSQL and are queryable by agent, service, operation, and time window
4. Frontend and backend traces share the same trace IDs for end-to-end correlation
5. All logs follow the same structured JSON format with mandatory fields

**Q: What constraints exist (budget, timeline, tech stack)?**
A:
- **Tech Stack**: Go backend, SvelteKit/TypeScript frontend, PostgreSQL, NATS, Grafana stack (Loki, Tempo, Prometheus)
- **No cloud dependencies**: Self-hosted Tempo for traces, self-hosted Grafana for visualization
- **PostgreSQL for MVP**: Usage events stored in PostgreSQL initially, with schema designed for potential migration to ClickHouse/TimescaleDB at scale
- **Health endpoints out of scope**: Already handled by existing pattern in API handler layer

## Iterative Exploration

### Technology Decisions Already Made

#### 1. Backend Telemetry: OpenTelemetry SDK
**Q: OpenTelemetry vs Custom?**
A: **OTel SDK** - It's the industry standard, every tool in the stack (Tempo, Prometheus, Loki, Grafana) speaks it natively, and building custom instrumentation is solving a solved problem. The dependency complexity cost is worth paying once here so every service gets it for free.

#### 2. Usage Event Storage: PostgreSQL
**Q: What persistent store for NATS-based usage events?**
A: **PostgreSQL** - It's already in the stack, usage event volume at MVP scale doesn't justify a specialist time-series store, and the data model is well-understood. The schema should be designed with migration to ClickHouse or TimescaleDB in mind if query performance degrades at scale — that's a future concern, not an MVP one.

#### 3. Trace Backend: Self-hosted Tempo
**Q: What trace backend?**
A: **Self-hosted Tempo** - Keeps everything in the existing Grafana stack (Loki for logs, Tempo for traces, Prometheus for metrics, Grafana for dashboards — all unified). No cloud dependency, works in both Docker Compose and Kubernetes, and Grafana's unified querying across logs/traces/metrics is genuinely valuable for the kind of debugging this system needs.

#### 4. Scope: NATS Subject Constants
**Q: Should shared/telemetry include NATS subject definitions?**
A: **Yes** - Include the usage event type and its NATS subject constant. The subject constant for usage events is a telemetry concern, not a general messaging concern — it would be strange for the messaging package to define a subject that only the telemetry system uses.

#### 5. Health Endpoints
**Q: Should telemetry package provide health check endpoints?**
A: **No** - Health check endpoints are out of scope. They already exist in the API handler layer and the pattern is established. The telemetry package should expose a `HealthCheck()` error method that the readiness handler can call to verify the OTel exporter connection, same pattern as the NATS wrapper.

### Research Questions (Open)

#### 6. Frontend Telemetry Stack
**Q: What frontend telemetry stack to use?**
A: **Research required** - No existing tools in use. Open question for research - the constraint is trace context propagation compatibility with the backend OTel setup. Research should evaluate OpenTelemetry browser SDK, Sentry, and any other actively maintained options against that constraint.

#### 7. OTel Go SDK Specifics
**Q: Are there any known issues with NATS context propagation in OTel Go SDK?**
A: **Research required** - Validate current OTel Go SDK version and any known issues with NATS context propagation specifically, since that's a less common integration pattern than HTTP propagation.

### Priority Clarification

**Q: How should the six concerns be prioritized?**
A: **Concerns 1-4 are blockers** — nothing gets built until they're done. **Concerns 5 and 6 are designed here but some implementation may be deferred** to the units that actually surface them (the Layer Inspector unit will implement the cost dashboard, the frontend unit will implement the browser telemetry module).

## Six Concerns to Address

### 1. Usage Event Envelope
The canonical Go type that any service emits when it consumes a resource. An LLM call, a memory read, a tool execution, a database query, a NATS message published — all of these are resource consumption events and all need to be tracked consistently for cost attribution, billing, and capacity planning. The shape of this envelope (which fields are mandatory, which are optional, what the allowed values for operation type and resource type are) must be defined once here and used everywhere. This is not a logging concern — it is a structured business event that flows through NATS to a persistent store and is queryable by agent, by service, by operation, and by time window.

### 2. OpenTelemetry Trace Propagation
How distributed traces are initialised, how trace context is passed across service boundaries (both HTTP and NATS), and how agentId and cycleId are attached as mandatory span attributes on every span that touches agent-related work. This is where "multi-agent as first class" gets operationalised in the observability layer: a trace that starts at the API when a user sends a message must carry the agentId through the NATS message to the cognitive engine, through each layer, through the LLM gateway call, and back. Without this threading established as a shared primitive, it will be implemented differently by each service or not at all.

### 3. Structured Logging Standards
The shared logger initialisation, the mandatory fields every log line must carry (service name, agentId where applicable, correlationId, cycleId where applicable), the log level conventions, and the JSON output format that the log aggregation stack (Loki) expects. Services should not configure their own loggers — they should call a bootstrap function from shared/telemetry and get a correctly configured logger back.

### 4. Prometheus Metric Conventions
The naming scheme, the label cardinality rules (agentId as a label requires careful thought since high-cardinality labels can destroy Prometheus performance at scale), and the standard metrics that every service exposes (request latency, error rate, active connections). The metric bootstrap should be a shared function so every service exposes a /metrics endpoint in the same way with the same baseline instrumentation.

### 5. Observability as a Product Feature
Usage data, trace data, and metrics are not only for operators — they drive user-facing functionality. The Layer Inspector showing real-time cognitive cycle execution, cost dashboards showing per-agent LLM spend, reliability indicators showing agent uptime and error rates, and debugging tools showing why a specific cycle produced an unexpected result — all of these are product features that depend entirely on the observability primitives defined in this unit. The problem space must therefore consider the frontend consumption of observability data as a first-class requirement, not an afterthought. What gets stored, how it gets queried, and how it gets surfaced to users must be designed with these product surfaces in mind from the start.

### 6. Frontend Observability
The browser-side counterpart to the backend instrumentation. The SvelteKit frontend needs its own observability story covering: JavaScript error tracking and reporting (unhandled exceptions, failed API calls, WebSocket disconnections), real user monitoring (page load times, time-to-interactive, WebSocket latency), and frontend trace context propagation (when the frontend initiates an action that produces a backend trace, the frontend should contribute the root span so the full trace from user click to layer output is captured in a single trace). Frontend telemetry must use the same trace IDs and correlation IDs as the backend so a user-reported bug ("my agent stopped responding") can be correlated immediately with backend traces without manual cross-referencing.

## Key Insights

1. **OTel SDK is the right choice** - Industry standard, native integration with Tempo/Loki/Prometheus/Grafana, avoids reinventing solved problems
2. **PostgreSQL is sufficient for MVP usage events** - Schema design should consider future migration to ClickHouse/TimescaleDB
3. **Self-hosted Tempo keeps stack unified** - No cloud dependencies, works in Docker and Kubernetes, Grafana's unified querying is valuable
4. **NATS subject constants belong in telemetry** - Usage events are a telemetry concern, not general messaging
5. **HealthCheck() method pattern** - Follow existing NATS wrapper pattern for exporter health verification
6. **Concerns 1-4 are blockers** - These must be fully implemented before any feature services
7. **Concerns 5-6 can be deferred** - Design now, implement in Layer Inspector and frontend units later
8. **Frontend/backend trace correlation is critical** - Same trace IDs must flow end-to-end

## Dependencies Identified

- **OpenTelemetry Go SDK** - Core instrumentation
- **OpenTelemetry exporters** - OTLP for Tempo, Prometheus for metrics, potentially Loki-compatible logger
- **NATS Go client** - Already in use, need OTel context propagation integration
- **PostgreSQL** - Already in use, schema for usage events
- **Grafana Stack (Tempo, Loki, Prometheus)** - Already planned, need OTel integration
- **Frontend OTel SDK or Sentry** - Research needed

## Assumptions Made

1. The shared/telemetry package will be imported by all backend services
2. The frontend will have a separate telemetry module (not Go)
3. NATS message context propagation requires custom OTel integration (not out-of-the-box)
4. Usage event schema will include: timestamp, agent_id, service_name, operation_type, resource_type, cost/duration metadata
5. Trace context will use W3C Trace Context standard for propagation
6. Prometheus metrics will use histogram for latency, counter for errors, gauge for active connections

## Open Questions (For Research)

1. **Frontend telemetry stack**: OpenTelemetry browser SDK vs Sentry vs other - evaluate based on backend trace context compatibility
2. **OTel Go SDK version**: Validate current stable version and any breaking changes
3. **NATS OTel integration**: Any known issues with OTel context propagation over NATS? Custom carrier implementation needed?
4. **High-cardinality label strategy**: How to handle agentId in Prometheus labels without killing performance at scale?
5. **Log aggregation format**: What format does Loki expect? JSON with specific fields?

## Next Steps

1. Proceed to BSD (Business Specification Document) with the problem space clarified
2. Research phase should evaluate:
   - Frontend telemetry options (OTel browser SDK, Sentry, others)
   - OTel Go SDK current version and NATS integration patterns
   - Prometheus label cardinality strategies
   - Loki log format requirements
3. All six concerns will be designed in this unit; concerns 1-4 fully implemented, 5-6 designed but implementation deferred
