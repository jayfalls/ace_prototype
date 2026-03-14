# Functional Specification Document

## Overview

Defines the technical implementation details for the shared observability primitives in `shared/telemetry`. This document specifies the Go package API, data structures, and integration patterns for traces, metrics, logs, and usage events.

## Technology Stack

| Component | Technology | Rationale |
|-----------|------------|-----------|
| Backend Telemetry | OpenTelemetry Go SDK | Industry standard, native Tempo/Loki/Prometheus integration |
| Trace Backend | Tempo (self-hosted) | Part of Grafana stack, no cloud dependency |
| Log Aggregation | Loki (self-hosted) | Part of Grafana stack, OTel Collector integration |
| Metrics | Prometheus | Industry standard, /metrics endpoint |
| Usage Events | PostgreSQL | MVP storage, schema designed for ClickHouse migration |
| NATS Integration | Custom TextMapPropagator | OTel context propagation over NATS headers |

## Package Structure

```
shared/telemetry/
├── telemetry.go          # Main initialization and bootstrap
├── tracer.go             # Trace provider and span helpers
├── metrics.go            # Prometheus metrics bootstrap
├── logger.go             # Structured logger initialization
├── usage.go              # UsageEvent type and publisher
├── natsCarrier.go        # NATS message carrier for OTel
├── constants.go          # NATS subject constants
└── types.go              # Shared type definitions
```

## Core Components

### 1. UsageEvent Type

The canonical type for tracking resource consumption across all services.

```go
type UsageEvent struct {
    Timestamp     time.Time `json:"timestamp"`
    AgentID       string    `json:"agent_id"`
    ServiceName   string    `json:"service_name"`
    OperationType string    `json:"operation_type"`  // llm_call, memory_read, tool_execute, db_query, nats_publish
    ResourceType  string    `json:"resource_type"`  // api, memory, tool, database, messaging
    CostUSD       float64   `json:"cost_usd,omitempty"`
    DurationMs    int64     `json:"duration_ms,omitempty"`
    TokenCount    int64     `json:"token_count,omitempty"`
    Metadata      map[string]interface{} `json:"metadata,omitempty"`
}
```

**Operation Types (Enum)**
- `llm_call` - LLM API invocation
- `memory_read` - Memory store read
- `memory_write` - Memory store write
- `tool_execute` - Tool invocation
- `db_query` - Database query
- `nats_publish` - NATS message published
- `nats_subscribe` - NATS message consumed

**NATS Subject Constant**
```go
const SubjectUsageEvent = "ace.usage.event"
```

### 2. Telemetry Initialization

All services initialize telemetry at startup with a single config struct.

```go
type Config struct {
    ServiceName string
    Environment string  // "development", "staging", "production"
    OTLPEndpoint string // OTel Collector endpoint for traces/metrics
    LokiEndpoint string // Loki endpoint for logs (optional, uses OTel Collector)
}
```

**Bootstrap Function**
```go
func Init(ctx context.Context, config Config) (*Telemetry, error)
```

**Returned Telemetry Object**
```go
type Telemetry struct {
    Tracer    *tracing.Tracer
    Meter     *metrics.Meter
    Logger    *zap.Logger
    Usage     *UsagePublisher
    Shutdown  func(context.Context) error
}
```

### 3. Trace Configuration

**Trace Provider Setup**
- W3C Trace Context propagator for HTTP headers
- Custom NATS carrier for message propagation
- Batch span exporter to OTel Collector
- Resource attributes: service.name, service.version, deployment.environment

**Mandatory Span Attributes**
```go
// Every span related to agent work must include:
SpanAttributes{
    "agent_id":  string,  // Required when agent context exists
    "cycle_id":  string,  // Required when cycle context exists
    "service_name": string, // Always required
}
```

**Trace Context Extraction**
```go
// HTTP middleware extracts trace context
func TraceMiddleware(next http.Handler) http.Handler

// For manual extraction in other contexts
func ExtractHTTP(ctx context.Context, headers http.Header) context.Context
func ExtractNATS(ctx context.Context, msg *nats.Msg) context.Context
```

### 4. NATS Carrier Implementation

Custom OTel TextMapPropagator for NATS message headers.

```go
type NATSCarrier struct {
    msg *nats.Msg
}

func (c NATSCarrier) Set(key, value string) {
    c.msg.Header.Set(key, value)
}

func (c NATSCarrier) Get(key string) string {
    return c.msg.Header.Get(key)
}

func (c NATSCarrier) Keys() []string {
    // Return header keys
}

// Usage
propagator := propagation.NewCompositeTextMapPropagator(
    propagation.TraceContext{},
    propagation.Baggage{},
)

func InjectTraceContext(ctx context.Context, msg *nats.Msg) {
    propagator.Inject(ctx, NATSCarrier{msg: msg})
}

func ExtractTraceContext(ctx context.Context, msg *nats.Msg) context.Context {
    return propagator.Extract(ctx, NATSCarrier{msg: msg})
}
```

### 5. Metrics Configuration

**Standard Metrics Exposed**

| Metric Name | Type | Labels | Description |
|-------------|------|--------|-------------|
| `http_request_duration_seconds` | Histogram | service, method, path, status | Request latency |
| `http_requests_total` | Counter | service, method, path, status | Total requests |
| `http_active_requests` | Gauge | service | Active requests |
| `otelcol_exporter_sent_spans` | Counter | service, exporter | Spans exported |

**Labels Strategy**
- **Low-cardinality only**: service, method, path, status
- **NO agentId as label**: Use UsageEvent for agent-level attribution
- **NO userId as label**: Privacy and cardinality concerns

**Metrics Endpoint**
```go
// Automatic /metrics endpoint registration
func RegisterMetrics mux.Handler
```

### 6. Logging Configuration

**Logger Bootstrap**
```go
func NewLogger(serviceName, environment string) (*zap.Logger, error)
```

**Output Requirements**
- Write to stdout/stderr only (no log files)
- JSON format for Loki ingestion
- OTel Collector filelog receiver reads from Docker container logs

**Mandatory Fields**
```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "info",
  "message": "Request processed",
  "service_name": "api",
  "trace_id": "abc123",
  "span_id": "def456",
  "agent_id": "agent-001",
  "correlation_id": "corr-789",
  "cycle_id": "cycle-001"
}
```

**Log Level Conventions**
- `debug`: Detailed flow information
- `info`: Normal operation events
- `warn`: Potential issues requiring attention
- `error`: Failures requiring investigation

### 7. Usage Event Publisher

```go
type UsagePublisher struct {
    nc *nats.Conn
}

func NewUsagePublisher(nc *nats.Conn) *UsagePublisher

// Publishes to SubjectUsageEvent ("ace.usage.event")
func (p *UsagePublisher) Publish(ctx context.Context, event UsageEvent) error

// Convenience methods for common operations
func (p *UsagePublisher) LLMCall(ctx context.Context, agentID, service string, tokens int64, costUSD float64, durationMs int64)
func (p *UsagePublisher) MemoryRead(ctx context.Context, agentID, service string, durationMs int64)
func (p *UsagePublisher) ToolExecute(ctx context.Context, agentID, service, toolName string, durationMs int64)
```

### 8. Usage Event Consumer (in API Service)

```go
type UsageConsumer struct {
    sub *nats.Subscription
    db  *sql.DB
}

func NewUsageConsumer(nc *nats.Conn, db *sql.DB) *UsageConsumer

func (c *UsageConsumer) Start(ctx context.Context) error
```

**Database Schema**
```sql
CREATE TABLE usage_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp TIMESTAMPTZ NOT NULL,
    agent_id UUID REFERENCES agents(id),
    service_name VARCHAR(255) NOT NULL,
    operation_type VARCHAR(50) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    cost_usd DECIMAL(10, 6),
    duration_ms BIGINT,
    token_count BIGINT,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for common query patterns
CREATE INDEX idx_usage_events_agent_id ON usage_events(agent_id);
CREATE INDEX idx_usage_events_timestamp ON usage_events(timestamp DESC);
CREATE INDEX idx_usage_events_operation_type ON usage_events(operation_type);
CREATE INDEX idx_usage_events_service_name ON usage_events(service_name);
```

## Integration Patterns

### Service Initialization

```go
package main

import (
    "context"
    "log"
    
    "github.com/ace/framework/shared/telemetry"
)

func main() {
    ctx := context.Background()
    
    t, err := telemetry.Init(ctx, telemetry.Config{
        ServiceName:  "api",
        Environment:  "development",
        OTLPEndpoint: "localhost:4317",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer t.Shutdown(ctx)
    
    // Use t.Tracer, t.Meter, t.Logger, t.Usage
}
```

### HTTP Middleware Usage

```go
router := chi.NewRouter()
router.Use(telemetry.TraceMiddleware)
router.Use(telemetry.MetricsMiddleware)
router.Use(telemetry.LoggerMiddleware)
```

### LLM Call Instrumentation

```go
func (s *Service) CallLLM(ctx context.Context, prompt string) (string, error) {
    ctx, span := s.telemetry.Tracer.Start(ctx, "llm.call")
    defer span.End()
    
    start := time.Now()
    response, err := s.llmClient.Complete(ctx, prompt)
    durationMs := time.Since(start).Milliseconds()
    
    if err != nil {
        span.RecordError(err)
        return "", err
    }
    
    // Emit usage event
    s.telemetry.Usage.LLMCall(ctx, agentID, "api", response.Tokens, calculateCost(response), durationMs)
    
    span.SetAttributes(
        attribute.Int64("llm.tokens", response.Tokens),
        attribute.String("llm.model", response.Model),
    )
    
    return response.Text, nil
}
```

### NATS Message Publishing

```go
func (s *Service) PublishEvent(ctx context.Context, event Event) error {
    msg, _ := json.Marshal(event)
    natsMsg := &nats.Msg{
        Data: msg,
        Header: nats.Header{},
    }
    
    // Inject current trace context into NATS message
    telemetry.InjectTraceContext(ctx, natsMsg)
    
    return s.nc.PublishMsg(natsMsg)
}
```

### NATS Message Subscribing

```go
func (s *Service) SubscribeEvents(subject string) error {
    _, err := s.nc.Subscribe(subject, func(msg *nats.Msg) {
        // Extract trace context from incoming message
        ctx := telemetry.ExtractTraceContext(context.Background(), msg)
        
        ctx, span := s.telemetry.Tracer.Start(ctx, "event.process")
        defer span.End()
        
        // Process message with tracing
        s.processEvent(ctx, msg.Data)
    })
    return err
}
```

## OTel Collector Configuration

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
  
  filelog:
    include: ["/var/lib/docker/containers/*/*-json.log"]
    operators:
      - type: json_parser
        timestamp:
          parse_from: timestamp
          layout: '2006-01-02T15:04:05Z07:00'
      - type: move
        from: level
        to: attributes.level
      - type: move
        from: message
        to: body.message

processors:
  batch:
    timeout: 10s
    send_batch_size: 1000
  
  memory_limiter:
    check_interval: 1s
    limit_mib: 1000

exporters:
  otlp/tempo:
    endpoint: tempo:4317
    tls:
      insecure: true
  
  loki:
    endpoint: http://loki:3100/loki/api/v1/push
    labels:
      attributes:
        service.name: "service_name"

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch, memory_limiter]
      exporters: [otlp/tempo]
    logs:
      receivers: [filelog]
      processors: [batch, memory_limiter]
      exporters: [loki]
    metrics:
      receivers: [otlp]
      processors: [batch, memory_limiter]
      exporters: [prometheus]
```

## Out of Scope

- Frontend telemetry module (deferred to frontend unit)
- Product features using observability data (cost dashboards, layer inspector)
- Health check endpoints (already handled in API layer)

## Dependencies

```go
// go.mod additions
require (
    go.opentelemetry.io/otel v1.21.0
    go.opentelemetry.io/otel/sdk v1.21.0
    go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.21.0
    go.opentelemetry.io/otel/exporters/prometheus v0.44.0
    go.opentelemetry.io/otel/propagation v1.21.0
    go.uber.org/zap v1.26.0
    github.com/nats-io/nats.go v1.31.0
)
```

## API Surface Summary

| Function | Purpose |
|----------|---------|
| `telemetry.Init()` | Initialize all observability components |
| `telemetry.TraceMiddleware()` | HTTP middleware for trace context |
| `telemetry.MetricsMiddleware()` | HTTP middleware for metrics |
| `telemetry.ExtractHTTP()` | Manual trace extraction from HTTP |
| `telemetry.ExtractNATS()` | Manual trace extraction from NATS |
| `telemetry.InjectTraceContext()` | Inject trace into NATS message |
| `telemetry.NewLogger()` | Create structured logger |
| `UsagePublisher.Publish()` | Emit usage event to NATS |
| `UsageConsumer.Start()` | Consume usage events from NATS |
