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

#### Option C: Hybrid Approach (Recommended)
- Use OTel browser SDK for trace context propagation
- Use Sentry for error tracking (which can correlate with OTel traces)
- This provides best of both worlds

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
What format does Loki expect? JSON with specific fields?

### Research Findings

#### OTel Collector Integration
When using OTel Collector with Loki exporter:
- Logs should be sent in OTLP format
- OTel Collector handles conversion to Loki format
- Resource attributes become Loki labels
- Log attributes become structured metadata

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
**Use OTel Collector with Loki exporter** - Send structured JSON logs via OTLP to OTel Collector, which forwards to Loki. Use structured metadata for trace/correlation IDs rather than Loki labels.

---

## 5. OTel Go SDK Version

### Question
Validate current stable version and any breaking changes.

### Research Findings

#### Current Status (2025)
- OTel Go SDK is actively maintained
- Regular releases with backward compatibility
- 1.x versions are stable for production use

#### Key Packages
- `go.opentelemetry.io/otel` - Core SDK
- `go.opentelemetry.io/otel/sdk` - SDK implementation
- `go.opentelemetry.io/otel/exporters/otlp` - OTLP exporters
- `go.opentelemetry.io/otel/contrib` - Third-party integrations

### Recommendation
**Use latest stable 1.x version** - Check go.mod for current recommendations. Avoid pre-1.0 versions.

---

## Summary of Recommendations

| Area | Recommendation | Priority |
|------|---------------|----------|
| Frontend Telemetry | OTel Browser SDK | Must Have |
| NATS Integration | Custom TextMapPropagator carrier | Must Have |
| Prometheus Labels | No agentId in labels | Must Have |
| Log Format | OTel Collector → Loki | Must Have |
| OTel Go SDK | Latest stable 1.x | Must Have |

---

## Dependencies to Add

```go
// Backend - OpenTelemetry
go.opentelemetry.io/otel@v1.x.x
go.opentelemetry.io/otel/sdk@v1.x.x
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@v1.x.x
go.opentelemetry.io/otel/exporters/prometheus@v1.x.x

// Frontend - OpenTelemetry
npm install @opentelemetry/api @opentelemetry/sdk-trace-web @opentelemetry/exporter-trace-otlp-http
```

---

## Next Steps

1. Proceed to FSD (Functional Specification Document) with these technology decisions
2. Define the UsageEvent schema in detail
3. Design the shared/telemetry package API
4. Plan the NATS message carrier implementation
