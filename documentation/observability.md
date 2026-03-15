# Observability

The ACE Framework includes a comprehensive observability stack for monitoring traces, logs, and metrics. This document explains how to set up and use the observability features.

## Overview

The observability stack consists of:

| Component | Purpose | Port |
|-----------|---------|------|
| **Prometheus** | Metrics collection and storage | 9090 |
| **Grafana** | Visualization and dashboards | 3000 |
| **Loki** | Log aggregation | 3100 |
| **Tempo** | Distributed tracing | 4314, 4315, 4316 |
| **OTel Collector** | Telemetry collection and processing | 4317, 4318, 8888, 8889 |

## Quick Start

### Starting the Observability Stack

Run the following command to start all observability services:

```bash
cd devops/dev
docker compose up -d prometheus otel-collector loki tempo grafana
```

Or start the entire stack:

```bash
cd devops/dev
docker compose up -d
```

### Accessing the UIs

| Service | URL | Credentials |
|---------|-----|-------------|
| Grafana | http://localhost:3000 | Anonymous (Admin) |
| Prometheus | http://localhost:9090 | N/A |
| Loki | http://localhost:3100 | N/A |
| Tempo | http://localhost:4314 | N/A |

## Configuring Services

### OTel Collector Configuration

The OTel Collector configuration is located at `devops/otel-collector-config.yaml`. It includes:

- **Receivers**: OTLP (gRPC/HTTP), Filelog
- **Exporters**: Loki (logs), Tempo (traces), Prometheus (metrics)
- **Processors**: batch, memory_limiter, resource

To modify the configuration, edit `devops/otel-collector-config.yaml` and restart the OTel Collector:

```bash
docker compose restart otel-collector
```

### Prometheus Configuration

The Prometheus configuration is located at `devops/dev/prometheus.yml`. It includes scrape targets for:

- Prometheus itself
- OTel Collector
- ACE API service
- Loki
- Tempo

## Viewing Logs

### Using Grafana

1. Open Grafana at http://localhost:3000
2. Navigate to **Explore** (compass icon)
3. Select **Loki** as the data source
4. Run a log query, for example:
   - `{job="ace_api"}` - All logs from the API service
   - `{container_name="ace_api"}` - Container-specific logs
   - `level="error"` - Error logs only

### Using Loki Directly

```bash
# Query logs using Loki API
curl -G "http://localhost:3100/loki/api/v1/query" \
  --data-urlencode 'query={job="ace_api"}'
```

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

## Viewing Traces

### Using Grafana

1. Open Grafana at http://localhost:3000
2. Navigate to **Explore** (compass icon)
3. Select **Tempo** as the data source
4. Search for traces by:
   - Service name
   - Trace ID
   - Span name
   - Time range

### Using Tempo Directly

```bash
# Search traces
curl -G "http://localhost:4315/api/search" \
  --data-urlencode 'q={service.name="api"}'

# Get trace by ID
curl "http://localhost:4315/api/traces/<trace_id>"
```

### Trace Context Propagation

Traces automatically propagate across services using W3C Trace Context. To view a complete trace:

1. Find a trace ID from logs (look for `trace_id` field)
2. Search in Tempo to see the full trace
3. Click on spans to see detailed timing and attributes

## Viewing Metrics

### Using Prometheus

1. Open Prometheus at http://localhost:9090
2. Navigate to **Graph** or use the table view
3. Enter a metric name, for example:
   - `http_request_duration_seconds` - Request latency
   - `http_requests_total` - Request count
   - `http_active_requests` - Active requests

### Using Grafana

1. Open Grafana at http://localhost:3000
2. Navigate to **Dashboards**
3. Import or create dashboards using Prometheus as the data source

### Available Metrics

#### HTTP Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `http_request_duration_seconds` | Histogram | Request latency in seconds |
| `http_requests_total` | Counter | Total number of requests |
| `http_active_requests` | Gauge | Current number of active requests |

Labels: `service_name`, `method`, `path`, `status_code`

#### OTel Collector Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `otelcol_exporter_sent_spans` | Counter | Number of spans exported |
| `otelcol_exporter_sent_metric_points` | Counter | Number of metric points exported |
| `otelcol_receiver_accepted_spans` | Counter | Number of spans received |

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
