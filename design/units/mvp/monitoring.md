# Monitoring

## Logging

### Backend
- Structured JSON logs using zap/zerolog
- Log levels: DEBUG, INFO, WARN, ERROR
- Request ID correlation for tracing

### Frontend
- Browser console for development
- Error boundary components for production errors

## Metrics

### API Metrics
- Request latency (histogram)
- Request count (counter)
- Error rate (counter)

### Agent Metrics
- Active sessions (gauge)
- Messages processed (counter)
- Thought cycles (counter)

### Health Checks
- GET /health - Basic health check
- GET /ready - Readiness probe

## Alerting

For MVP, alerts are logged to console:
- Agent start/stop events
- Authentication failures
- API errors (5xx)

## Dashboards

Not included in MVP - deferred to production with Grafana.
