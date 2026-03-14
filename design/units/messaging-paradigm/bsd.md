# Messaging Paradigm - Business Specification

## Business Case

The ACE Framework is a distributed multi-service system where the cognitive engine, API, memory, senses, tools, and future services all need to communicate with each other. Currently there is no defined contract for how that communication happens — no agreed message format, no subject naming convention, no rules about which services publish and which subscribe, and no shared Go types that encode any of this.

### Why This Matters Now

Every service that gets built after this point will need to make communication decisions independently. Without a prior contract they will inevitably make incompatible ones:
- Different message envelope formats
- Inconsistent subject naming
- Agent context as optional afterthought vs first-class
- No way to trace messages end-to-end
- System becomes impossible to debug or reason about

This unit exists to establish that contract before any service beyond the API is built. The deliverable is not a running service — it is the shared communication primitives that every future service will depend on.

## Business Value

### Consistency
All services communicate using the same patterns and formats. Any developer can look at any service's NATS interactions and understand them immediately.

### Observability
correlation_id and cycle_id enable end-to-end tracing. The Layer Inspector can reconstruct any cognitive cycle's thought flow. The Safety Monitor can watch all messages cheaply via headers.

### Debuggability
Standardized subjects with wildcards allow monitoring at any level:
- All messages: `ace.*.*.*.>`
- Per-agent: `ace.*.{agentId}.>`
- Per-layer: `ace.engine.{agentId}.layer.>`
- System events: `ace.system.*`

### Scalability
JetStream provides persistence and replay for debugging past cognitive cycles. Consumer groups enable horizontal scaling of the cognitive engine.

### Maintainability
Single source of truth in `shared/messaging/` — fix once, all services benefit. New services inherit best practices automatically.

## Risk if Not Done

- Incompatible message formats between services
- Silent routing failures (typos in subjects)
- No way to trace a thought from input to output across layers
- Cannot debug by replaying past cognitive cycles
- Every service needs to make independent decisions, leading to fragmentation
- Observability and auth units cannot publish events correctly

## Scope

### In Scope
- Message envelope definition with mandatory fields (message_id, correlation_id, agent_id, cycle_id, source_service, timestamp, schema_version)
- NATS subject naming convention (`ace.<domain>.<agentId>.<subsystem>.<action>`, `ace.system.<subsystem>.<action>`)
- Shared NATS client wrapper with connection management, reconnection, drain, health checks
- Support for request-reply, fire-and-forget, and streaming patterns
- JetStream configuration for persistence and durability
- Subject constant validation

### Out of Scope
- Multi-tenancy (enterprise organization isolation) — not in MVP scope
- Message signing — TLS sufficient for internal communication
- Protobuf payloads — JSON acceptable for MVP
- Specific retention durations — tuned by individual units based on their needs

### In Scope (Refined)
- JetStream stream retention strategy — categories for cognitive messages, usage events, system events
- FSD defines retention strategy; individual units tune exact durations

## Open Questions for FSD

1. **Subject Structure Variability** — Pattern `ace.<domain>.<agentId>.<subsystem>.<action>` doesn't fit all examples. FSD needs to tighten or acknowledge variable depth.

2. **Stream Ownership** — Who creates JetStream streams on service startup? Need idempotent creation or central management.

3. **Session Context** — Should `session_id` be on envelope or in payload? Affects Layer Inspector trace reconstruction.

## Priority

**Critical Foundation** — This must be completed before any feature service is built. It is a prerequisite for observability, auth, and all future units.

All subsequent units will need to publish events, and they cannot do so correctly without these primitives already existing.
