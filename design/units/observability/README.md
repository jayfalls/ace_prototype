# Observability

This unit establishes the shared observability primitives in `shared/telemetry` before any feature service is built. Like the messaging unit established NATS communication contracts, this unit establishes the contracts for how every service reports what it is doing, what it is consuming, and what it is costing.

## Status

- **Problem Space**: In Progress
- **BSD**: Pending
- **User Stories**: Pending
- **Research**: Pending
- **FSD**: Pending
- **Architecture**: Pending
- **Implementation**: Pending

## Six Concerns

1. **Usage Event Envelope** - Canonical Go type for resource consumption events (LLM calls, memory reads, tool executions, DB queries, NATS messages)
2. **OpenTelemetry Trace Propagation** - Trace context across HTTP and NATS boundaries with agentId/cycleId as mandatory span attributes
3. **Structured Logging Standards** - Shared logger initialization with mandatory fields and JSON format for Loki
4. **Prometheus Metric Conventions** - Naming schemes, label cardinality rules, standard metrics per service
5. **Observability as Product Feature** - User-facing features (cost dashboards, layer inspector) designed here
6. **Frontend Observability** - Browser-side telemetry with trace context propagation to backend

## Documents

- [Problem Space](problem_space.md)
