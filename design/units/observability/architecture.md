# Architecture Document

## Overview

This document describes how the observability primitives integrate with the ACE Framework architecture, the data flow for traces/metrics/logs, and how usage events propagate through the system.

## System Integration

### High-Level Observability Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           ACE Framework                                      │
│                                                                             │
│  ┌──────────────┐      ┌──────────────┐      ┌──────────────────────┐     │
│  │   Frontend   │      │    Core      │      │   Cognitive Engine  │     │
│  │  SvelteKit   │◄────►│  API (Go)    │◄────►│        (Future)     │     │
│  │              │      │    Chi        │      │                      │     │
│  └──────┬───────┘      └──────┬───────┘      └──────────┬───────────┘     │
│         │                      │                         │                 │
│         │              ┌───────┴───────┐                 │                 │
│         │              │   Auth (JWT)  │                 │                 │
│         │              │  WebSocket    │                 │                 │
│         └──────────────┼───────────────┼─────────────────┘                 │
│                        │               │                                   │
│         ┌──────────────┼───────────────┼─────────────────────────────────┐ │
│         │         Telemetry/Senses                                             │ │
│         │  Inputs: Chat | Sensors | Metrics | Webhooks                    │ │
│         │  + Observability: shared/telemetry                               │ │
│         └──────────────┬───────────────┬─────────────────────────────────┘ │
│                        │               │                                   │
│                        ▼               ▼                                   │
│                 ┌───────────┐   ┌───────────┐                              │
│                 │PostgreSQL │   │   NATS    │                              │
│                 │  + SQLC   │   │(Pub/Sub)  │                              │
│                 │           │   │           │                              │
│                 │ +Usage    │   │ +Trace    │                              │
│                 │ Events   │   │ Context   │                              │
│                 └───────────┘   └───────────┘                              │
│                        │               │                                   │
│                        ▼               ▼                                   │
│                 ┌─────────────────────────────────────────────────────┐    │
│                 │              OTel Collector                           │    │
│                 │  ┌─────────┐  ┌─────────┐  ┌─────────┐              │    │
│                 │  │ filelog │  │   OTLP  │  │  OTLP   │              │    │
│                 │  │ Receiver│  │ (grpc)  │  │ (http)  │              │    │
│                 │  └────┬────┘  └────┬────┘  └────┬────┘              │    │
│                 │       │            │            │                     │    │
│                 │       ▼            ▼            ▼                     │    │
│                 │  ┌─────────────────────────────────────────────┐     │    │
│                 │  │           Processors                        │     │    │
│                 │  │  batch | memory_limiter | resource         │     │    │
│                 │  └────────────────────┬────────────────────────┘     │    │
│                 │                       │                               │    │
│                 │                       ▼                               │    │
│                 │  ┌────────────┐ ┌──────────┐ ┌────────────┐        │    │
│                 │  │    Loki    │ │  Tempo   │ │ Prometheus │        │    │
│                 │  │ (Logs)     │ │ (Traces) │ │ (Metrics)  │        │    │
│                 │  └────────────┘ └──────────┘ └────────────┘        │    │
│                 └─────────────────────────────────────────────────────┘    │
│                                      │                                     │
│                                      ▼                                     │
│                         ┌────────────────────────┐                         │
│                         │       Grafana         │                         │
│                         │  Loki | Tempo | Query │                         │
│                         └────────────────────────┘                         │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Component Architecture

### shared/telemetry Package

The `shared/telemetry` package provides observability primitives consumed by all services:

```
┌─────────────────────────────────────────────────────────────┐
│                    shared/telemetry                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐   │
│  │  telemetry  │  │   tracer    │  │     metrics     │   │
│  │    Init()   │  │   Start()   │  │   Histogram    │   │
│  │   Config    │  │  Extract()  │  │    Counter     │   │
│  │   Shutdown  │  │  Inject()   │  │     Gauge      │   │
│  └──────┬──────┘  └──────┬──────┘  └────────┬────────┘   │
│         │                │                   │              │
│         └────────────────┼───────────────────┘              │
│                          │                                  │
│  ┌─────────────┐  ┌──────┴──────┐  ┌─────────────────┐   │
│  │   logger    │  │     nats    │  │     usage       │   │
│  │ NewLogger() │  │   Carrier   │  │    Publisher    │   │
│  │  *zap.Logger│  │   TextMap   │  │    Consumer    │   │
│  └─────────────┘  └─────────────┘  └─────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Trace Context Propagation

Traces flow across service boundaries using W3C Trace Context:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Trace Propagation Flow                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Frontend                                                                  │
│     │                                                                        │
│     │ Creates root span with trace_id                                        │
│     ▼                                                                        │
│  ┌───────────────────┐                                                       │
│  │ traceparent hdr  │ ──► 00-abc123-def456-01-abc1234567890ab              │
│  └───────────────────┘                                                       │
│     │                                                                        │
│     │ HTTP Request                                                           │
│     ▼                                                                        │
│  API Service (shared/telemetry)                                             │
│     │                                                                        │
│     ├─► Extract trace from HTTP headers                                     │
│     │    propagator.ExtractHTTP(ctx, req.Header)                            │
│     │                                                                        │
│     ├─► Start span with extracted context                                  │
│     │    tracer.Start(ctx, "api.handle")                                    │
│     │                                                                        │
│     │ Add attributes: service_name, agent_id, cycle_id                      │
│     │                                                                        │
│     │ Publish to NATS                                                       │
│     ▼                                                                        │
│  ┌───────────────────┐                                                       │
│  │ traceparent hdr  │ ──► Injected into NATS headers                        │
│  │ (trace context)  │                                                       │
│  └───────────────────┘                                                       │
│     │                                                                        │
│     │ NATS Message                                                          │
│     ▼                                                                        │
│  Cognitive Engine / Layer                                                   │
│     │                                                                        │
│     ├─► Extract trace from NATS headers                                     │
│     │    propagator.ExtractNATS(ctx, msg.Header)                           │
│     │                                                                        │
│     ├─► Continue span with extracted context                                │
│     │    tracer.Start(ctx, "layer.process")                                 │
│     │                                                                        │
│     │ LLM Call                                                              │
│     │    tracer.Start(ctx, "llm.call")                                      │
│     │                                                                        │
│     │ Export spans to OTel Collector                                        │
│     ▼                                                                        │
│  OTel Collector ──► Tempo (traces)                                         │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Usage Event Flow

Usage events track resource consumption and flow from services to PostgreSQL:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                       Usage Event Flow                                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Service (API, Cognitive Engine, etc.)                                      │
│     │                                                                        │
│     │ Call LLM / Read Memory / Execute Tool                                 │
│     ▼                                                                        │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │ UsagePublisher.Publish() or LLMCall() convenience method              │  │
│  │                                                                       │  │
│  │ - Creates UsageEvent with:                                            │  │
│  │   * timestamp, agent_id, cycle_id, session_id                        │  │
│  │   * service_name, operation_type, resource_type                       │  │
│  │   * cost_usd, duration_ms, token_count, metadata                    │  │
│  │                                                                       │  │
│  │ - Publishes to NATS subject: ace.usage.event                         │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│     │                                                                        │
│     │ NATS Pub/Sub                                                         │
│     ▼                                                                        │
│  API Service (UsageConsumer)                                                │
│     │                                                                        │
│     ├─► Subscribes to ace.usage.event                                      │
│     │                                                                        │
│     ├─► On message received:                                               │
│     │    - Parse UsageEvent from JSON                                      │
│     │    - Insert into PostgreSQL usage_events table                      │
│     │    - Index: agent_id, cycle_id, session_id, timestamp              │  │
│     │                                                                        │
│     ▼                                                                        │
│  PostgreSQL (usage_events table)                                            │
│     │                                                                        │
│     │ Queryable by:                                                        │
│     │ - Agent (cost attribution, billing)                                  │
│     │ - Cycle (Layer Inspector integration)                                │
│     │ - Session (conversation-level cost)                                 │
│     │ - Service (cost per service)                                        │
│     │ - Operation type (cost breakdown)                                   │
│     │ - Time window (trends)                                              │
│     │                                                                        │
│     ▼                                                                        │
│  Product Features (Layer Inspector, Cost Dashboard)                         │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Log Aggregation Flow

Logs flow from services through OTel Collector to Loki:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Log Aggregation Flow                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Service                                                                     │
│     │                                                                        │
│     │ Write JSON to stdout (via *zap.Logger)                               │
│     │                                                                        │
│     │ {                                                                     │
│     │   "timestamp": "2024-01-15T10:30:00Z",                              │
│     │   "level": "info",                                                   │
│     │   "message": "Request processed",                                    │
│     │   "service_name": "api",                                             │
│     │   "trace_id": "abc123",                                              │
│     │   "agent_id": "agent-001",                                           │
│     │   "cycle_id": "cycle-001"                                            │
│     │ }                                                                     │
│     │                                                                        │
│     ▼                                                                        │
│  Docker Container Log (stdout/stderr)                                        │
│     │                                                                        │
│     │ Captured to: /var/lib/docker/containers/<id>/<id>-json.log          │
│     │                                                                        │
│     ▼                                                                        │
│  OTel Collector (filelog receiver)                                          │
│     │                                                                        │
│     │ - Reads JSON logs from container log files                           │
│     │ - Parses JSON, moves fields to attributes                           │
│     │ - Adds resource attributes (service.name, etc.)                     │
│     │                                                                        │
│     ▼                                                                        │
│  OTel Collector (processors)                                                │
│     │                                                                        │
│     │ - batch: aggregates before sending                                   │
│     │ - memory_limiter: prevents OOM                                       │
│     │                                                                        │
│     ▼                                                                        │
│  Loki (log storage)                                                         │
│     │                                                                        │
│     │ - Indexed by: service.name, level                                    │
│     │ - Structured metadata: trace_id, span_id, agent_id, cycle_id        │
│     │                                                                        │
│     ▼                                                                        │
│  Grafana (visualization)                                                    │
│     │                                                                        │
│     │ - LogQL queries                                                      │
│     │ - Correlate logs with traces                                         │
│     │                                                                        │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Data Flow Summary

| Data Type | Source | Transport | Storage | Visualization |
|-----------|--------|-----------|---------|---------------|
| Traces | All services | OTLP (HTTP/gRPC) | Tempo | Grafana |
| Metrics | All services | OTLP (HTTP/gRPC) | Prometheus | Grafana |
| Logs | All services | stdout → filelog | Loki | Grafana |
| Usage Events | All services | NATS | PostgreSQL | Grafana (custom) |

## Scaling Considerations

### High-Volume Scenarios

- **Trace sampling**: OTel Collector supports head-based and tail-based sampling
- **Log filtering**: Processors can filter before sending to reduce volume
- **Metrics aggregation**: Prometheus handles high-cardinality with relabeling

### Multi-Agent Scenarios

- **Trace isolation**: Each agent has unique trace_id; filter by agent_id in Tempo
- **Usage attribution**: UsageEvent includes agent_id for per-agent cost
- **Layer Inspector**: Cycle-level traces enable per-cycle analysis

## Security Considerations

- **No sensitive data in traces**: Avoid logging PII, passwords, tokens
- **Metric cardinality**: agentId not used as Prometheus label (privacy + performance)
- **Usage event retention**: Consider TTL policies for PostgreSQL usage_events
- **OTel Collector**: Runs internal network; no external exposure needed

## Deployment

### Development (Docker Compose)

```yaml
services:
  otel-collector:
    image: otel/opentelemetry-collector-contrib
    volumes:
      - ./otel-collector-config.yaml:/etc/otelcol-contrib/config.yaml
    ports:
      - "4317:4317"   # OTLP gRPC
      - "4318:4318"   # OTLP HTTP
      - "8889:8889"   # Prometheus metrics
    depends_on:
      - loki
      - tempo

  loki:
    image: grafana/loki
    ports:
      - "3100:3100"

  tempo:
    image: grafana/tempo
    ports:
      - "4316:4316"   # OTLP gRPC
      - "4315:4315"   # OTLP HTTP
```

### Production (Kubernetes)

- OTel Collector as DaemonSet or sidecar
- Loki, Tempo, Prometheus as managed services or StatefulSets
- Grafana as centralized dashboard
