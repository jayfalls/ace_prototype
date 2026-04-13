# Observability

The ACE Framework includes built-in observability features for tracing, logging, and metrics. This document explains the observability capabilities and optional external stack.

## Built-in Observability

ACE includes telemetry built into the binary:

- **Structured Logging**: JSON logs to stdout
- **Metrics**: Prometheus-compatible `/metrics` endpoint
- **Tracing**: OpenTelemetry traces via OTLP

### Quick Start

Run ACE and access built-in endpoints:

```bash
make ace
```

### Built-in Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /healthz` | Health check |
| `GET /metrics` | Prometheus metrics |

### Log Format

All services output JSON logs with the following structure:

```json
{
  "timestamp": "2026-03-15T10:30:00Z",
  "level": "info",
  "message": "Request processed",
  "service_name": "api",
  "trace_id": "abc123",
  "span_id": "def456"
}
```

### Available Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `http_request_duration_seconds` | Histogram | Request latency in seconds |
| `http_requests_total` | Counter | Total number of requests |
| `http_active_requests` | Gauge | Current number of active requests |

Labels: `service_name`, `method`, `path`, `status_code`

## Optional Observability Stack

For advanced monitoring, an optional external stack can be deployed separately:

```bash
cd devops/dev
docker compose up -d prometheus otel-collector loki tempo grafana
```

| Component | Purpose | Port |
|-----------|---------|------|
| **Prometheus** | Metrics collection and storage | 9090 |
| **Grafana** | Visualization and dashboards | 3000 |
| **Loki** | Log aggregation | 3100 |
| **Tempo** | Distributed tracing | 4314, 4315, 4316 |
| **OTel Collector** | Telemetry collection and processing | 4317, 4318, 8888, 8889 |

### Accessing UIs

| Service | URL | Credentials |
|---------|-----|-------------|
| Grafana | http://localhost:3000 | Anonymous (Admin) |
| Prometheus | http://localhost:9090 | N/A |
| Loki | http://localhost:3100 | N/A |
| Tempo | http://localhost:4314 | N/A |

## Application Integration

### Go Services

To send telemetry from your Go service:

```go
import "ace/shared/telemetry"

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

### Environment Variables

Configure your services with these environment variables:

```bash
# OTel Collector endpoint
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
OTEL_EXPORTER_OTLP_PROTOCOL=grpc

# Service identification
OTEL_SERVICE_NAME=api
OTEL_RESOURCE_ATTRIBUTES=service.name=api,deployment.environment=development
```

### Structured Logging

Use the telemetry logger for structured JSON output:

```go
logger := telemetry.NewLogger("api", "development")
logger.Info("request processed",
    zap.String("trace_id", traceID),
    zap.String("span_id", spanID),
)
```

## Troubleshooting

### OTel Collector Not Receiving Data

1. Check if the OTel Collector is running:
   ```bash
   docker compose ps
   ```

2. Check the health endpoint:
   ```bash
   curl http://localhost:8888/healthz
   ```

3. Check OTel Collector logs:
   ```bash
   docker compose logs otel-collector
   ```

### No Metrics in Prometheus

1. Verify Prometheus can scrape the OTel Collector:
   ```bash
   curl http://localhost:8889/metrics
   ```

2. Check Prometheus targets:
   - Open http://localhost:9090/targets
   - Look for OTel Collector target status

### No Logs in Loki

1. Verify Loki is running:
   ```bash
   curl http://localhost:3100/ready
   ```

2. Check that services are writing to stdout (required for filelog receiver)

### No Traces in Tempo

1. Verify Tempo is running:
   ```bash
   curl http://localhost:4314/ready
   ```

2. Check that services are sending OTLP data to the collector

## Additional Resources

- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)
- [Prometheus Documentation](https://prometheus.io/docs/)
- [Loki Documentation](https://grafana.com/docs/loki/latest/)
- [Tempo Documentation](https://grafana.com/docs/tempo/latest/)
