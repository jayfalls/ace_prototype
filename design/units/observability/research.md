# Research Document

## Research Questions from Problem Space

This document addresses the open research questions identified in the problem_space.md for the observability unit.

---

## 1. Frontend Telemetry Stack

### Question
What frontend telemetry stack to use? Constraint is trace context propagation compatibility with the backend OTel setup.

### Research Findings

#### Option A: OpenTelemetry Browser SDK
- **Pros:**
  - Native W3C Trace Context support for end-to-end tracing
  - Works seamlessly with backend OTel SDK using OTLP protocol
  - Industry standard, vendor-neutral
  - Automatic injection of trace context into fetch/XHR requests
  - Can send to any OTLP-compatible backend (Tempo, Jaeger, etc.)

- **Cons:**
  - Larger bundle size compared to alternatives
  - More complex configuration
  - Some features require additional setup

- **Status:** Actively maintained by OTel community

#### Option B: Sentry
- **Pros:**
  - Excellent error tracking and debugging
  - Easy to set up, great DX
  - Has started supporting OpenTelemetry
  - Rich UI for error investigation

- **Cons:**
  - Originally proprietary SDK (now supports OTel but not fully native)
  - Can use OTel as protocol but loses some Sentry-specific features
  - Vendor lock-in if using full feature set

#### Option C: Hybrid Approach (OTel + Sentry)
- Use OTel browser SDK for trace context propagation
- Use Sentry for error tracking (which can correlate with OTel traces)
- This provides best of both worlds

**Decision: Not recommended** - Sentry introduces vendor lock-in without providing additional value over OTel's native capabilities for this project. Use Option A (OTel Browser SDK) only.

### Recommendation
**Use OpenTelemetry Browser SDK** - It provides the critical trace context propagation needed for end-to-end tracing from browser to backend. The native W3C Trace Context support is essential for the "same trace IDs must flow end-to-end" requirement.

---

## 2. OTel Go SDK and NATS Context Propagation

### Question
Are there any known issues with NATS context propagation in OTel Go SDK?

### Research Findings

NATS is not natively supported by OTel SDKs for automatic context propagation. However, the OTel Go SDK provides the TextMapPropagator interface that allows custom carriers.

#### Implementation Pattern
```go
// Custom NATS message carrier for OTel context propagation
type NATSMessageCarrier struct {
    msg *nats.Msg
}

// Inject trace context into NATS message headers
func (c NATSMessageCarrier) Set(key, value string) {
    c.msg.Header.Set(key, value)
}

// Extract trace context from NATS message headers
func (c NATSMessageCarrier) Get(key string) string {
    return c.msg.Header.Get(key)
}

func (c NATSMessageCarrier) Keys() []string {
    // Return header keys
}
```

#### Resources Found
- OneUptime blog provides working example of NATS + OTel integration
- Tracetest article confirms NATS context propagation requires custom carrier
- OTel propagation package provides TextMapPropagator interface

### Recommendation
**Custom NATS carrier implementation required** - This is a known pattern, not an OTel limitation. The SDK provides the necessary interfaces. The implementation should:
1. Use W3C Trace Context propagator
2. Inject/extract from NATS message headers
3. Follow OTel semantic conventions for messaging spans

---

## 3. Prometheus High-Cardinality Label Strategy

### Question
How to handle agentId in Prometheus labels without killing performance at scale?

### Research Findings

#### The Problem
- Every unique label value combination creates a new time series
- High-cardinality labels (like agentId with thousands of unique agents) can create millions of time series
- This degrades Prometheus query performance and storage

#### Best Practices
1. **Avoid agentId as a label** on standard metrics
2. **Use UsageEvents** for agent-level cost attribution (stored in PostgreSQL)
3. **Keep low-cardinality labels** on Prometheus metrics:
   - service_name
   - method
   - path
   - status_code
   - operation

#### Alternative Approaches
- **Recording rules**: Pre-aggregate agent metrics with downsampled cardinality
- **Remote write to external system**: Use Thanos, Cortex, or TimescaleDB for high-cardinality metrics
- **Metric relabeling**: Drop high-cardinality labels at scrape time

### Recommendation
**Do NOT add agentId as a Prometheus label** - Store agent-level usage data in PostgreSQL (via UsageEvents through NATS) and query there. Keep Prometheus metrics focused on service-level observability with low-cardinality labels.

---

## 4. Loki Log Format Requirements

### Question
What format does Loki expect? JSON with specific fields? OTel Collector vs direct export?

### Research Findings

#### OTel Collector vs Direct Export

**Option A: OTel Collector with Loki Exporter (Recommended)**
- **Pros:**
  - Batching: Aggregates multiple log lines before sending, reducing HTTP overhead
  - Retry: Built-in retry logic with backoff for failed exports
  - Fan-out: Can send to multiple backends simultaneously (Loki + any other)
  - Format translation: Handles conversion from OTLP to Loki's format automatically
  - Pipeline: Can process, filter, and transform logs before export
- **Cons:**
  - Additional container/process to operate
  - Additional configuration in compose/K8s
  - More resources (CPU/memory) required

**Option B: Direct Export (Alternative)**
- Use Loki OTLP HTTP endpoint directly without Collector
- **Pros:** Simpler infrastructure, fewer components
- **Cons:** No batching, no retry, no fan-out, no pipeline processing
- This is a valid alternative for simpler deployments

#### Recommended Fields for JSON Logs
```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "info",
  "message": "Request processed",
  "service_name": "api",
  "trace_id": "abc123",
  "span_id": "def456",
  "agent_id": "agent-001",
  "correlation_id": "corr-789"
}
```

#### Loki Label Strategy
- **Index labels** (high cost): service.name, environment
- **Structured metadata** (low cost): trace_id, span_id, agent_id
- Avoid indexing high-cardinality fields

### Recommendation
**Use OTel Collector with Loki exporter** - The batching, retry, and fan-out capabilities justify the additional operational complexity. For a system emitting thousands of events per minute, the Collector provides resilience that direct exporters lack.

---

## 6. Usage Event NATS to PostgreSQL

### Question
How do usage events get from NATS into PostgreSQL? What component consumes the NATS subject and writes rows?

### Research Findings

Three architectural options:

**Option A: Dedicated Consumer in API Service**
- A goroutine in the API service subscribes to the usage event NATS subject
- Writes directly to PostgreSQL using existing repository pattern
- **Pros:** No new services, uses existing DB connection pool
- **Cons:** Couples usage event processing to API service lifecycle

**Option B: Standalone Consumer Process**
- Separate Go service that subscribes to NATS and writes to PostgreSQL
- Independent scaling and lifecycle
- **Pros:** Independent deployment, can scale horizontally
- **Cons:** New service to deploy, maintain, monitor

**Option C: OTel Collector NATS Receiver**
- Use OTel Collector's NATS receiver to consume events
- Write to PostgreSQL using OTel Collector's database exporter
- **Pros:** OTel-native, handles batching and retries
- **Cons:** OTel Collector's DB exporter support is limited, less flexibility

### Recommendation
**Option A: Dedicated Consumer in API service** - For MVP, the simplest approach is a goroutine in the API service that subscribes to `ace.usage.event` and writes to PostgreSQL. This avoids introducing a new service. At scale, this can be refactored to Option B (standalone consumer) without changing the NATS contract.

### Note
This is an architectural decision that affects implementation. The FSD should specify which approach is used.

### Recommendation
**Install latest stable versions** - Use the latest stable releases for all four components. Pin versions in Docker Compose and Kubernetes manifests at deployment time.

---

## Summary of Recommendations

| Area | Recommendation | Priority |
|------|---------------|----------|
| Frontend Telemetry | OTel Browser SDK | Must Have |
| NATS Integration | Custom TextMapPropagator carrier | Must Have |
| Prometheus Labels | No agentId in labels | Must Have |
| Log Format | OTel Collector → Loki | Must Have |
| OTel Go SDK | Latest stable | Must Have |
| Grafana Stack | Latest stable | Must Have |
| Usage Events | Dedicated consumer in API service | Must Have |

---

## Dependencies to Add

```go
// Backend - OpenTelemetry (use latest stable)
go.opentelemetry.io/otel
go.opentelemetry.io/otel/sdk
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
go.opentelemetry.io/otel/exporters/prometheus

// Frontend - OpenTelemetry (use latest stable)
npm install @opentelemetry/api @opentelemetry/sdk-trace-web @opentelemetry/exporter-trace-otlp-http
```

---

## Next Steps

1. Proceed to FSD (Functional Specification Document) with these technology decisions
2. Define the UsageEvent schema in detail
3. Design the shared/telemetry package API
4. Plan the NATS message carrier implementation
