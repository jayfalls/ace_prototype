# Implementation Plan

## Overview

This document breaks down the observability unit into implementable micro-PRs. Each issue/PR should be small enough to implement in a single session.

## Implementation Order

The implementation should proceed in dependency order:

1. **Shared Package Setup** - Create the shared/telemetry module
2. **Dependencies** - Add OTel, zap, NATS, goose dependencies
3. **Types** - Define UsageEvent type
4. **NATS Constants** - Add usage event subject
5. **Logger** - Structured logger with zap
6. **Tracer** - OpenTelemetry setup
7. **Metrics** - Prometheus metrics
8. **NATS Carrier** - Custom propagator
9. **Usage Publisher** - NATS publisher
10. **Usage Consumer** - PostgreSQL consumer
11. **Goose Migration** - usage_events table + main.go update
12. **Middleware** - HTTP middleware
13. **OTel Collector Config** - Docker Compose config
14. **Frontend Telemetry** - SvelteKit telemetry module
15. **Integration Tests** - End-to-end tests

## Micro-PRs

### PR 1: Create shared/telemetry Module Structure

**Goal:** Set up the shared/telemetry module with basic structure

**Files:**
- `backend/shared/telemetry/telemetry.go` - Main init with Config
- `backend/shared/telemetry/go.mod` - Module definition

**Acceptance Criteria:**
- Module compiles
- Basic Init function that returns Telemetry struct
- Config struct with ServiceName, Environment, OTLPEndpoint

---

### PR 2: Add Dependencies

**Goal:** Add all required OTel and supporting dependencies

**Files:**
- `backend/shared/telemetry/go.mod` - Updated with dependencies

**Dependencies to Add:**
```
go.opentelemetry.io/otel
go.opentelemetry.io/otel/sdk
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
go.opentelemetry.io/otel/exporters/prometheus
go.opentelemetry.io/otel/propagation
go.uber.org/zap
github.com/jackc/pgx/v5
github.com/nats-io/nats.go
github.com/pressly/goose/v3
```

**Acceptance Criteria:**
- `go mod tidy` succeeds
- All dependencies resolve without conflicts

---

### PR 3: Define UsageEvent Type

**Goal:** Create the UsageEvent type and constants

**Files:**
- `backend/shared/telemetry/usage.go` - UsageEvent type
- `backend/shared/telemetry/constants.go` - NATS subject constants

**Code:**
```go
type UsageEvent struct {
    Timestamp     time.Time `json:"timestamp"`
    AgentID       string    `json:"agent_id"`
    CycleID       string    `json:"cycle_id"`
    SessionID     string    `json:"session_id"`
    ServiceName   string    `json:"service_name"`
    OperationType string    `json:"operation_type"`
    ResourceType  string    `json:"resource_type"`
    CostUSD       float64   `json:"cost_usd,omitempty"`
    DurationMs    int64     `json:"duration_ms,omitempty"`
    TokenCount    int64     `json:"token_count,omitempty"`
    Metadata      map[string]string `json:"metadata,omitempty"`
}

const SubjectUsageEvent = "ace.usage.event"
```

**Acceptance Criteria:**
- Type compiles
- JSON serialization works
- Tests pass

---

### PR 4: Structured Logger

**Goal:** Implement logger initialization with zap

**Files:**
- `backend/shared/telemetry/logger.go` - Logger setup

**Code:**
```go
func NewLogger(serviceName, environment string) (*zap.Logger, error)
```

**Requirements:**
- JSON output format
- Mandatory fields: timestamp, level, message, service_name
- Optional fields: trace_id, span_id, agent_id, cycle_id, session_id, correlation_id
- Output to stdout only

**Acceptance Criteria:**
- Logger outputs valid JSON
- All mandatory fields present
- Tests pass

---

### PR 5: OpenTelemetry Tracer

**Goal:** Implement trace provider setup

**Files:**
- `backend/shared/telemetry/tracer.go` - Tracer setup

**Code:**
```go
type Telemetry struct {
    Tracer    *tracesdk.Tracer
    // ...
}

func (t *Telemetry) HealthCheck() error
```

**Requirements:**
- W3C Trace Context propagator
- Batch span exporter to OTLP endpoint
- Resource attributes: service.name, deployment.environment
- HealthCheck returns error if exporter connection is down

**Acceptance Criteria:**
- Tracer creates spans
- Trace context extracts from HTTP headers
- Tests pass

---

### PR 6: Prometheus Metrics

**Goal:** Implement metrics bootstrap

**Files:**
- `backend/shared/telemetry/metrics.go` - Metrics setup

**Code:**
```go
func RegisterMetrics() http.Handler  // Returns /metrics endpoint
```

**Standard Metrics:**
- `http_request_duration_seconds` (histogram)
- `http_requests_total` (counter)
- `http_active_requests` (gauge)

**Labels (low cardinality only):**
- service_name
- method
- path
- status_code

**Acceptance Criteria:**
- Metrics endpoint serves Prometheus format
- No high-cardinality labels (no agentId)
- Tests pass

---

### PR 7: NATS Carrier for Trace Context

**Goal:** Implement custom OTel propagator for NATS

**Files:**
- `backend/shared/telemetry/natsCarrier.go` - NATS carrier

**Code:**
```go
type NATSCarrier struct {
    msg *nats.Msg
}

func InjectTraceContext(ctx context.Context, msg *nats.Msg)
func ExtractTraceContext(ctx context.Context, msg *nats.Msg) context.Context
```

**Requirements:**
- Implements TextMapPropagator interface
- Injects trace context into NATS headers
- Extracts trace context from NATS headers

**Acceptance Criteria:**
- Trace context propagates through NATS messages
- Tests pass

---

### PR 8: Usage Event Publisher

**Goal:** Implement UsagePublisher for emitting usage events

**Files:**
- `backend/shared/telemetry/usage.go` - Add publisher code

**Code:**
```go
type UsagePublisher struct {
    nc *nats.Conn
}

func NewUsagePublisher(nc *nats.Conn) *UsagePublisher

func (p *UsagePublisher) Publish(ctx context.Context, event UsageEvent) error
func (p *UsagePublisher) LLMCall(ctx context.Context, agentID, cycleID, sessionID, service string, tokens int64, costUSD float64, durationMs int64) error
func (p *UsagePublisher) MemoryRead(ctx context.Context, agentID, cycleID, sessionID, service string, durationMs int64) error
func (p *UsagePublisher) ToolExecute(ctx context.Context, agentID, cycleID, sessionID, service, toolName string, durationMs int64) error
```

**Requirements:**
- Publishes to `ace.usage.event` subject
- All convenience methods return error

**Acceptance Criteria:**
- Events published to NATS
- Error handling works
- Tests pass

---

### PR 9: Usage Event Consumer

**Goal:** Implement UsageConsumer for persisting to PostgreSQL

**Files:**
- `backend/shared/telemetry/consumer.go` - Consumer implementation

**Code:**
```go
type UsageConsumer struct {
    sub  *nats.Subscription
    pool *pgxpool.Pool
}

func NewUsageConsumer(nc *nats.Conn, pool *pgxpool.Pool) *UsageConsumer

func (c *UsageConsumer) Start(ctx context.Context) error
```

**Requirements:**
- Subscribes to `ace.usage.event`
- Parses UsageEvent from JSON
- Inserts into PostgreSQL

**Acceptance Criteria:**
- Consumes messages from NATS
- Writes to database
- Tests pass

---

### PR 10: Goose Migration + Main Update

**Goal:** Create usage_events table using goose and wire into main.go

**Files:**
- `backend/shared/telemetry/migrations/*.go` - Goose migrations
- `backend/services/api/cmd/api/main.go` - Add goose registration and migration runner

**Migration Code:**
```go
// migrations/001_create_usage_events.go
package migrations

import (
    "github.com/pressly/goose/v3"
)

func init() {
    goose.AddMigration(up, down)
}

func up(tx *sql.Tx) error {
    _, err := tx.Exec(`
        CREATE TABLE IF NOT EXISTS usage_events (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            timestamp TIMESTAMPTZ NOT NULL,
            agent_id UUID NOT NULL,
            cycle_id UUID NOT NULL,
            session_id UUID NOT NULL,
            service_name VARCHAR(255) NOT NULL,
            operation_type VARCHAR(50) NOT NULL,
            resource_type VARCHAR(50) NOT NULL,
            cost_usd DECIMAL(10, 6),
            duration_ms BIGINT,
            token_count BIGINT,
            metadata JSONB,
            created_at TIMESTAMPTZ DEFAULT NOW()
        );
        CREATE INDEX IF NOT EXISTS idx_usage_events_agent_id ON usage_events(agent_id);
        CREATE INDEX IF NOT EXISTS idx_usage_events_cycle_id ON usage_events(cycle_id);
        CREATE INDEX IF NOT EXISTS idx_usage_events_session_id ON usage_events(session_id);
        CREATE INDEX IF NOT EXISTS idx_usage_events_timestamp ON usage_events(timestamp DESC);
        CREATE INDEX IF NOT EXISTS idx_usage_events_operation_type ON usage_events(operation_type);
        CREATE INDEX IF NOT EXISTS idx_usage_events_service_name ON usage_events(service_name);
    `)
    return err
}

func down(tx *sql.Tx) error {
    _, err := tx.Exec("DROP TABLE IF EXISTS usage_events")
    return err
}
```

**Main.go Update:**
```go
import "github.com/pressly/goose/v3"

// In main(), after DB connection:
if err := goose.RunMigrationsOnDB(db, "shared/telemetry/migrations", "up"); err != nil {
    log.Fatal(err)
}
```

**Acceptance Criteria:**
- Migration runs on startup
- Table and indexes created
- Tests pass

---

### PR 11: HTTP Middleware

**Goal:** Implement HTTP middleware for trace/metrics

**Files:**
- `backend/shared/telemetry/middleware.go` - HTTP middleware

**Code:**
```go
func TraceMiddleware(next http.Handler) http.Handler
func MetricsMiddleware(next http.Handler) http.Handler
func LoggerMiddleware(next http.Handler) http.Handler
```

**Requirements:**
- TraceMiddleware extracts trace context from HTTP headers
- MetricsMiddleware tracks request duration
- LoggerMiddleware logs requests

**Acceptance Criteria:**
- Middleware chains work with chi router
- Trace context continues through requests
- Tests pass

---

### PR 12: OTel Collector Configuration

**Goal:** Create OTel Collector config for Docker Compose

**Files:**
- `devops/otel-collector-config.yaml` - OTel Collector configuration

**Config Requirements:**
- filelog receiver for Loki logs
- OTLP receiver for traces/metrics
- Loki exporter
- Tempo exporter  
- Prometheus exporter
- Processors: batch, memory_limiter, resource

**Docker Compose Update:**
Add OTel Collector service with health check to compose file

**Acceptance Criteria:**
- Config is valid YAML
- All receivers/exporters configured correctly
- Health check endpoint exposed

---

### PR 13: Frontend Telemetry Module

**Goal:** Implement SvelteKit frontend telemetry module

**Files:**
- `frontend/src/lib/telemetry/index.ts` - Main telemetry module
- `frontend/src/lib/telemetry/trace.ts` - Trace context
- `frontend/src/lib/telemetry/error.ts` - Error tracking
- `frontend/src/lib/telemetry/metrics.ts` - Performance metrics

**Requirements:**
- OpenTelemetry browser SDK integration
- Trace context injection into fetch/XHR
- JavaScript error tracking (unhandled exceptions)
- Performance monitoring (page load, time-to-interactive)
- Exports trace_id for backend correlation

**Code:**
```typescript
// frontend/src/lib/telemetry/index.ts
import { init, getTraceId, setGlobalTraceId } from './trace';
import { initErrorTracking } from './error';
import { initPerformanceMonitoring } from './metrics';

export function initTelemetry(serviceName: string, otelCollectorUrl: string) {
    init(serviceName, otelCollectorUrl);
    initErrorTracking();
    initPerformanceMonitoring();
}

export { getTraceId, setGlobalTraceId };
```

**Acceptance Criteria:**
- Module compiles
- Error tracking captures exceptions
- Performance metrics captured
- Trace context propagates to backend

---

### PR 14: Integration Tests

**Goal:** End-to-end tests for the telemetry package

**Files:**
- `backend/shared/telemetry/integration_test.go`

**Tests:**
- Full trace: HTTP → NATS → Consumer → Database
- Metrics endpoint scrape
- Logger JSON output
- Usage event round-trip

**Acceptance Criteria:**
- All integration tests pass
- NATS and PostgreSQL containers available

---

### PR 15: Grafana Provisioning (Issue #163)

**Goal:** Add Grafana provisioning for auto-configured datasources and dashboards

**Files:**
- `devops/provisioning/datasources/datasources.yml` - Datasource configuration
- `devops/provisioning/dashboards/dashboards.yml` - Dashboard imports
- `devops/dev/compose.yml` - Mount provisioning files (dev only)
- `devops/prod/compose.yml` - No provisioning (manual config in prod)

**Requirements:**
1. Create datasource provisioning for:
   - Loki: `http://loki:3100`
   - Tempo: `http://tempo:3200`
   - Prometheus: `http://prometheus:9090`

2. Create dashboard provisioning to import pre-built dashboards

3. Add pre-built dashboards from Grafana.com for Loki, Tempo, Prometheus, NATS, PostgreSQL

4. Update compose files to mount provisioning into Grafana container (dev only)

**Acceptance Criteria:**
- Datasources auto-configured on Grafana first load (dev)
- Dashboards available without manual import (dev)
- Works for dev compose files only (prod uses manual config)

---

## Summary

| PR | Name | Files | Description |
|----|------|-------|-------------|
| 1 | Module Setup | 2 | Create telemetry module |
| 2 | Dependencies | 1 | Add OTel, zap, NATS, goose deps |
| 3 | Types | 2 | UsageEvent, constants |
| 4 | Logger | 1 | Structured logging |
| 5 | Tracer | 1 | OTel setup |
| 6 | Metrics | 1 | Prometheus metrics |
| 7 | NATS Carrier | 1 | Trace propagation |
| 8 | Publisher | 1 | Usage event publisher |
| 9 | Consumer | 1 | Usage event consumer |
| 10 | Goose Migration | 3+ | Database schema + main.go |
| 11 | Middleware | 1 | HTTP middleware |
| 12 | OTel Collector | 1 | Docker Compose config |
| 13 | Frontend | 4+ | SvelteKit telemetry |
| 14 | Integration Tests | 1 | E2E tests |
| 15 | Grafana Provisioning | 4+ | Auto-configure datasources/dashboards (Issue #163) |

**Total: 15 PRs**
