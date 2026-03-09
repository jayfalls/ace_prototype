# Monitoring

<!--
Intent: Define observability requirements for the feature.
Scope: Logging, metrics, alerting, and dashboards needed to monitor the feature.
Used by: AI agents to implement proper observability and respond to issues.
-->

## Overview

The core-infra unit implements comprehensive monitoring for the ACE Framework MVP, including structured logging, Prometheus metrics, health checks, and alerting for all core services.

## Logging

### Log Levels

| Level | Usage |
|-------|-------|
| DEBUG | Detailed debugging information (SQL queries, request bodies) |
| INFO | General informational logs (requests, auth events) |
| WARNING | Warning conditions (deprecated usage, slow queries) |
| ERROR | Error conditions (validation failures, not found) |
| CRITICAL | Critical conditions (panic, service unavailable) |

### Key Log Events

| Event | Level | Fields | Description |
|-------|-------|--------|-------------|
| request_received | INFO | method, path, request_id, user_id | Incoming HTTP request |
| request_completed | INFO | method, path, request_id, user_id, status, duration_ms | Completed HTTP request |
| auth_login_success | INFO | user_id, method | Successful authentication |
| auth_login_failure | WARNING | email, reason | Failed authentication attempt |
| auth_token_refresh | INFO | user_id | JWT token refresh |
| user_created | INFO | user_id, email | New user registration |
| agent_created | INFO | agent_id, owner_id | New agent created |
| agent_started | INFO | agent_id, session_id | Agent session started |
| agent_stopped | INFO | agent_id, session_id | Agent session ended |
| thought_recorded | DEBUG | session_id, layer | Agent thought recorded |
| memory_stored | DEBUG | memory_id, owner_id | Memory stored |
| websocket_connected | INFO | user_id, session_id | WebSocket connection |
| websocket_disconnected | INFO | user_id, session_id | WebSocket disconnected |
| database_query | DEBUG | query, duration_ms | Database query executed |
| rate_limit_exceeded | WARNING | user_id, endpoint | Rate limit triggered |

### Log Format

```json
{
  "timestamp": "2024-01-01T00:00:00Z",
  "level": "INFO",
  "event": "request_completed",
  "component": "api",
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "request_id": "550e8400-e29b-41d4-a716-446655440001",
  "message": "GET /api/agents completed",
  "data": {
    "method": "GET",
    "path": "/api/agents",
    "status": 200,
    "duration_ms": 45
  }
}
```

### Logging Best Practices

- Use structured JSON logging (zerolog)
- Include correlation IDs (request_id) for tracing
- Log sensitive data appropriately (masked emails, no passwords)
- Include context in error logs (stack traces, user info)
- Use context-aware logging (request-scoped loggers)

## Metrics

### Key Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| http_requests_total | Counter | Total HTTP requests | method, path, status |
| http_request_duration_seconds | Histogram | Request latency | method, path |
| auth_login_total | Counter | Login attempts | result (success/failure) |
| active_websocket_connections | Gauge | WebSocket connections | - |
| agents_active | Gauge | Active agents | owner_id |
| sessions_active | Gauge | Active sessions | agent_id |
| thoughts_recorded_total | Counter | Total thoughts recorded | layer |
| memories_stored_total | Counter | Total memories stored | memory_type |
| database_query_duration_seconds | Histogram | DB query latency | query_type |
| rate_limit_exceeded_total | Counter | Rate limit hits | endpoint |

### Custom Metrics

```go
// Example metric definitions
import "github.com/prometheus/client_golang/prometheus"

var (
    HTTPRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total number of HTTP requests",
        },
        []string{"method", "path", "status"},
    )
    
    HTTPRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP request latency in seconds",
            Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1},
        },
        []string{"method", "path"},
    )
    
    ActiveWebSocketConnections = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "active_websocket_connections",
            Help: "Number of active WebSocket connections",
        },
    )
)
```

### Metric Collection

- **Backend**: Prometheus client library (prometheus/client_golang)
- **Scrape Interval**: 15 seconds
- **Endpoint**: `/metrics`
- **Retention**: 15 days

## Dashboards

### Dashboard: ACE Framework Overview

| Panel | Metric | Visualization |
|-------|--------|---------------|
| Request Rate | sum(rate(http_requests_total[5m])) | Line chart |
| Error Rate | sum(rate(http_requests_total{status=~"5.."}[5m])) | Line chart |
| Latency P99 | histogram_quantile(0.99, http_request_duration_seconds) | Line chart |
| Active Users | active_websocket_connections | Stat |
| Active Agents | agents_active | Stat |
| Active Sessions | sessions_active | Stat |

### Dashboard: System Health

| Panel | Metric | Visualization |
|-------|--------|---------------|
| Database Connections | pg_stat_activity | Stat |
| Memory Usage | process_resident_memory_bytes | Stat |
| CPU Usage | process_cpu_seconds_total | Graph |
| Goroutines | go_goroutines | Stat |

## Alerting

### Alert Rules

| Alert Name | Condition | Severity | Description |
|------------|-----------|----------|-------------|
| HighErrorRate | rate(http_requests_total{status=~"5.."}[5m]) > 0.05 | critical | More than 5% errors |
| HighLatency | histogram_quantile(0.99, http_request_duration_seconds) > 5 | warning | P99 > 5 seconds |
| WebSocketDisconnections | rate(websocket_disconnected_total[5m]) > 10/min | warning | Many disconnections |
| DatabaseConnectionExhaustion | pg_stat_activity > 80 | critical | DB pool near capacity |
| ServiceDown | up{job="ace-api"} == 0 | critical | API not responding |

### Alert Channels

| Channel | Type | Destination |
|---------|------|-------------|
| PagerDuty | PagerDuty | Production incidents |
| Slack | Webhook | #ace-alerts channel |
| Email | SMTP | on-call team |

### Alert Response Runbook

| Alert | Response Steps |
|-------|----------------|
| HighErrorRate | 1. Check logs for errors 2. Identify failing endpoint 3. Check database connectivity 4. Rollback recent changes if needed |
| HighLatency | 1. Check database queries 2. Review slow logs 3. Check resource utilization 4. Scale if needed |
| ServiceDown | 1. Check deployment status 2. Review recent deployments 3. Check health endpoint 4. Restart service |

## Tracing

### Distributed Tracing
- **Tool**: OpenTelemetry with Jaeger backend
- **Sampling Rate**: 10% (100% for errors)
- **Trace Context**: `X-Request-ID` header
- **B3 Propagation**: Support for Zipkin-style headers

### Key Spans

| Span | Description | Attributes |
|------|-------------|------------|
| http.request | Full HTTP request | method, path, headers |
| db.query | Database query | query, duration, rows |
| ws.message | WebSocket message | direction, size |
| auth.token_validation | JWT validation | user_id, valid |

## Health Checks

### Endpoint: /health

```json
{
  "status": "healthy",
  "version": "1.0.0",
  "timestamp": "2024-01-01T00:00:00Z",
  "checks": {
    "database": "ok",
    "redis": "ok"
  }
}
```

### Endpoint: /health/ready

```json
{
  "ready": true,
  "checks": {
    "database": true,
    "migrations": true
  }
}
```

### Readiness vs Liveness

- **Readiness**: Can serve traffic (dependencies healthy, migrations complete)
- **Liveness**: Is running (not stuck in infinite loop, not deadlocked)

### Kubernetes Probes

- **Liveness Probe**: HTTP GET /health every 10s, timeout 5s
- **Readiness Probe**: HTTP GET /health/ready every 5s, timeout 3s
- **Startup Probe**: HTTP GET /health every 5s, failure threshold 30
