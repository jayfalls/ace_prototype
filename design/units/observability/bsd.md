# Business Specification Document

## Feature Name

Shared Observability Primitives

## Problem Statement

The ACE Framework will run as a distributed multi-service system with autonomous agents executing cognitive cycles, making LLM calls, querying memory, invoking tools, and emitting thousands of events per minute. Without a shared observability foundation established before those services are built, each service will instrument itself independently — inventing its own log formats, its own metric names, its own trace span conventions, and its own usage event shapes. The result is a system that is technically observable in isolation but impossible to reason about as a whole: you cannot trace a user message through the API, into the cognitive engine, across six layers, through an LLM call, and back out again if every service speaks a different observability dialect.

## Solution

Establish the shared observability primitives in `shared/telemetry` before any feature service is built. Like the messaging unit established NATS communication contracts, this unit establishes the contracts for how every service reports what it is doing, what it is consuming, and what it is costing. Every subsequent service inherits these primitives rather than inventing its own.

The solution includes:
1. **Usage Event Envelope** - Canonical Go type for resource consumption events (LLM calls, memory reads, tool executions, DB queries, NATS messages) that flows through NATS to PostgreSQL
2. **OpenTelemetry Trace Propagation** - Trace context initialization and propagation across HTTP and NATS boundaries with agentId and cycleId as mandatory span attributes
3. **Structured Logging Standards** - Shared logger initialization with mandatory fields (service name, agentId, correlationId, cycleId) in JSON format for Loki
4. **Prometheus Metric Conventions** - Naming schemes, label cardinality rules, and standard metrics (request latency, error rate, active connections)
5. **Frontend Observability Module** - Browser-side telemetry design with trace context propagation to backend

## In Scope

- Usage event type definition and NATS subject constant
- OpenTelemetry SDK initialization and configuration
- Trace context propagation across HTTP and NATS
- Structured logger bootstrap function with mandatory fields (stdout/stderr only, no files)
- OTel Collector configuration for stdout/stderr ingestion
- Prometheus metrics bootstrap with standard instrumentations
- Frontend telemetry module design (implementation deferred to frontend unit)
- Database schema for usage events in PostgreSQL

## Out of Scope

- Product features built on observability data (cost dashboards, layer inspector) - designed but implemented in respective units
- Health check endpoints - already handled in API handler layer
- Trace/metric storage infrastructure (Tempo, Prometheus, Loki) - configured separately in Docker Compose/Kubernetes

## Value Proposition

1. **Consistency** - Every service uses the same observability primitives, eliminating fragmentation
2. **Debuggability** - Full end-to-end trace correlation from user click through all backend layers to LLM calls
3. **Cost Attribution** - Queryable usage data for billing, capacity planning, and cost optimization per agent/service
4. **Operator Experience** - Unified Grafana stack (Loki, Tempo, Prometheus) for all observability data
5. **Product Features** - Foundation for user-facing observability (cost dashboards, reliability indicators)

## Success Criteria

| Criterion | Metric | Target |
|-----------|--------|--------|
| All backend services use shared/telemetry | Code review | 100% of services import shared/telemetry |
| End-to-end trace correlation | Manual trace verification | Trace flows from API → cognitive engine → LLM |
| Usage event queryability | Query test | Events queryable by agent_id, service, operation, time |
| Log consistency | Log format review | All logs contain mandatory fields |
| Metrics consistency | Metrics review | All services expose standard metrics on /metrics |
| Frontend/backend trace correlation | Manual verification | Same trace IDs flow from browser to backend |

## Dependencies

- OpenTelemetry Go SDK
- NATS Go client (for trace context propagation)
- PostgreSQL (for usage event storage)
- Grafana Stack (Tempo, Loki, Prometheus) - external infrastructure
