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
│         └──────────────┬───────────────┴─────────────────────────────────┘ │
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
│  │ HealthCheck │  │             │  │  NewMeter()    │   │
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

**HealthCheck()** - Returns error if OTel exporter connection is down. Called by the readiness handler to verify observability pipeline connectivity.

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
│     ├─► Extract trace from NATS headers                                    │
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
│     │ Call LLM / Read Memory / Execute Tool                                  │
│     ▼                                                                        │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │ UsagePublisher.Publish() or LLMCall() convenience method              │  │
│  │                                                                       │  │
│  │ - Creates UsageEvent with:                                            │  │
│  │   * timestamp, agent_id, cycle_id, session_id                        │  │
│  │   * service_name, operation_type, resource_type                       │  │
│  │   * cost_usd, duration_ms, token_count, metadata                     │  │
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
│     │    - Parse UsageEvent from JSON                                       │
│     │    - Insert into PostgreSQL usage_events table                       │
│     │    - Index: agent_id, cycle_id, session_id, timestamp               │
│     │                                                                        │
│     ▼                                                                        │
│  PostgreSQL (usage_events table)                                            │
│     │                                                                        │
│     │ Queryable by:                                                        │
│     │ - Agent (cost attribution, billing)                                  │
│     │ - Cycle (Layer Inspector integration)                                │
│     │ - Session (conversation-level cost)                                 │
│     │ - Service (cost per service)                                       │
│     │ - Operation type (cost breakdown)                                   │
│     │ - Time window (trends)                                              │
│     │                                                                        │
│     ▼                                                                        │
│  Product Features (Layer Inspector, Cost Dashboard)                          │
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
│     │   "timestamp": "2024-01-15T10:30:00Z",                               │
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
│     │ - Structured metadata: trace_id, span_id, agent_id, cycle_id       │
│     │                                                                        │
│     ▼                                                                        │
│  Grafana (visualization)                                                    │
│     │                                                                        │
│     │ - LogQL queries                                                     │
│     │ - Correlate logs with traces                                         │
│     │                                                                        │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Metrics Flow (Prometheus Pull Model)

Services expose a `/metrics` endpoint using the Prometheus client. The OTel Collector scrapes these endpoints and pushes to Prometheus:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      Metrics Flow (Prometheus Pull)                         │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Service                                                                     │
│     │                                                                        │
│     │ Register standard metrics (via shared/telemetry)                     │
│     │ - http_request_duration_seconds (histogram)                         │
│     │ - http_requests_total (counter)                                     │
│     │ - http_active_requests (gauge)                                      │
│     │                                                                        │
│     │ Expose /metrics endpoint                                             │
│     ▼                                                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ # HELP http_request_duration_seconds HTTP request duration          │   │
│  │ # TYPE http_request_duration_seconds histogram                     │   │
│  │ http_request_duration_seconds_bucket{le="0.1"} 123              │   │
│  │ http_request_duration_seconds_bucket{le="0.5"} 456              │   │
│  │ ...                                                                  │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│     │                                                                        │
│     │ Prometheus scrape (pull)                                            │
│     ▼                                                                        │
│  OTel Collector (prometheus receiver)                                        │
│     │                                                                        │
│     │ - Scrapes /metrics from each service                               │
│     │ - Converts to OTLP internal format                                  │
│     │                                                                        │
│     ▼                                                                        │
│  OTel Collector (processors)                                                │
│     │                                                                        │
│     │ - batch: aggregates before sending                                   │
│     │ - memory_limiter: prevents OIM                                     │
│     │                                                                        │
│     ▼                                                                        │
│  Prometheus                                                                 │
│     │                                                                        │
│     │ - Stores time series data                                           │
│     │ - service_name, method, path, status as labels                     │
│     │                                                                        │
│     ▼                                                                        │
│  Grafana (visualization)                                                    │
│     │                                                                        │
│     │ - PromQL queries                                                   │
│     │ - Dashboards for latency, error rates, utilization                  │
│     │                                                                        │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Frontend Observability Flows

The SvelteKit frontend uses OpenTelemetry browser SDK for trace context propagation, JavaScript error tracking, and performance monitoring:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    Frontend Observability Flows                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    Trace Context Creation                            │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│     │                                                                        │
│     │ User triggers action (click, form submit)                           │
│     ▼                                                                        │
│  Frontend Telemetry Module (OTel browser SDK)                              │
│     │                                                                        │
│     │ - Creates root span with new trace_id                              │
│     │ - Injects traceparent header into fetch/XHR requests               │
│     │                                                                        │
│     ▼                                                                        │
│  HTTP Request with traceparent header                                       │
│     │                                                                        │
│     │ ──► traceparent: 00-abc123-def456-01-abc1234567890ab              │
│     │                                                                        │
│     ▼                                                                        │
│  Backend (continues existing trace)                                        │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    JavaScript Error Tracking                        │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│     │                                                                        │
│     │ Uncaught exception in browser                                      │
│     ▼                                                                        │
│  Frontend Error Handler                                                    │
│     │                                                                        │
│     │ - Captures: message, stack trace, URL, user agent                  │
│     │ - Attaches current trace_id (if available)                         │
│     │                                                                        │
│     ▼                                                                        │
│  Error Tracking Service (deferred - Sentry or OTel backend)              │
│     │                                                                        │
│     │ - Stores error with trace correlation                              │
│     │                                                                        │
│     ▼                                                                        │
│  Grafana (correlated with backend traces)                                  │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                 Performance Monitoring                                │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│     │                                                                        │
│     │ Page load, user interactions                                       │
│     ▼                                                                        │
│  Performance APIs (Navigation Timing, Resource Timing)                    │
│     │                                                                        │
│     │ - page_load_time, time_to_interactive                            │
│     │ - api_request_duration, websocket_latency                         │
│     │                                                                        │
│     ▼                                                                        │
│  Frontend Metrics Store                                                    │
│     │                                                                        │
│     │ - Aggregated and exposed via /metrics endpoint                    │
│     │                                                                        │
│     ▼                                                                        │
│  Prometheus (scraped by OTel Collector)                                   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Data Flow Summary

| Data Type | Source | Transport | Storage | Visualization |
|-----------|--------|-----------|---------|---------------|
| Traces | All services | OTLP (HTTP/gRPC) push | Tempo | Grafana |
| Metrics | All services | Prometheus scrape (pull) | Prometheus | Grafana |
| Logs | All services | stdout → filelog | Loki | Grafana |
| Usage Events | All services | NATS | PostgreSQL | Grafana (custom) |
| Frontend Traces | Browser | HTTP header injection | Tempo | Grafana |
| Frontend Errors | Browser | HTTP POST | Custom | Grafana |
| Frontend Metrics | Browser | Prometheus scrape | Prometheus | Grafana |

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
  prometheus:
    image: prom/prometheus
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:9090/-/healthy"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s

  otel-collector:
    image: otel/opentelemetry-collector-contrib
    volumes:
      - ./otel-collector-config.yaml:/etc/otelcol-contrib/config.yaml
    ports:
      - "4317:4317"   # OTLP gRPC
      - "4318:4318"   # OTLP HTTP
      - "8889:8889"   # Prometheus metrics
      - "8888:8888"   # OTel Collector health check
    depends_on:
      loki:
        condition: service_healthy
      tempo:
        condition: service_healthy
      prometheus:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8888/healthz"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s

  loki:
    image: grafana/loki
    ports:
      - "3100:3100"
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:3100/ready"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s

  tempo:
    image: grafana/tempo
    ports:
      - "4316:4316"   # OTLP gRPC
      - "4315:4315"   # OTLP HTTP
      - "4314:4314"   # Tempo health check
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:4314/ready"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s

  grafana:
    image: grafana/grafana
    ports:
      - "3000:3000"
    environment:
      - GF_AUTH_ANONYMOUS_ENABLED=true
    depends_on:
      - prometheus
      - loki
      - tempo
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:3000/api/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s
```

### Production (Kubernetes)

- OTel Collector as DaemonSet or sidecar
- Loki, Tempo, Prometheus as managed services or StatefulSets
- Grafana as centralized dashboard
