# User Stories

## Gherkin Syntax Guide
- **Feature**: A logical grouping of scenarios
- **Scenario**: A specific example of the feature working
- **Given**: Preconditions (what must be true before)
- **When**: The action being performed
- **Then**: Expected outcome (assertions)
- **And/But**: Chain multiple conditions or steps

## Feature: Shared Observability Primitives

### Background
```gherkin
Background: Telemetry package is available
  Given the shared/telemetry package is imported
  And the service calls telemetry.Init() at startup
```

### Scenario: Service initializes observability stack
```gherkin
Scenario: Service initializes observability with shared telemetry
  Given a Go service that imports shared/telemetry
  When the service calls telemetry.Init(config)
  Then a tracer is configured and ready
  And a logger is configured with service name
  And metrics are exposed on /metrics endpoint
  And trace context can be extracted from HTTP requests
```

### Scenario: Usage event emission
```gherkin
Scenario: Service emits usage event for LLM call
  Given a service that has called telemetry.Init()
  When the service makes an LLM API call
  And the call completes with token usage
  Then a UsageEvent is published to NATS subject "ace.usage.event"
  And the event contains: timestamp, agent_id, service_name, operation_type="llm_call"
  And the event contains: resource_type="api", token_count, cost_usd
```

### Scenario: Usage event for memory query
```gherkin
Scenario: Service emits usage event for memory read
  Given a service that has called telemetry.Init()
  When the service reads from memory store
  And the read operation completes
  Then a UsageEvent is published to NATS
  And the event contains: operation_type="memory_read"
  And the event contains: resource_type="memory", duration_ms
```

### Scenario: Usage event for tool execution
```gherkin
Scenario: Service emits usage event for tool invocation
  Given a service that has called telemetry.Init()
  When the service invokes a tool
  And the tool execution completes
  Then a UsageEvent is published to NATS
  And the event contains: operation_type="tool_execute"
  And the event contains: resource_type="tool", tool_name
```

### Scenario: HTTP trace context propagation
```gherkin
Scenario: Trace context propagates through HTTP
  Given a service that has called telemetry.Init()
  When an HTTP request arrives with traceparent header
  Then the trace context is extracted from the header
  And a span is created with the incoming trace_id
  And the span has attribute agentId when applicable
  And the span has attribute cycleId when applicable
```

### Scenario: NATS trace context propagation
```gherkin
Scenario: Trace context propagates through NATS
  Given a service that has called telemetry.Init()
  And the service has an active trace span
  When the service publishes a message to NATS
  Then trace context is injected into NATS headers
  And the receiving service can extract the trace context
  And the full trace flows from API through cognitive engine to LLM
```

### Scenario: Structured logging with mandatory fields
```gherkin
Scenario: Service logs with structured format
  Given a service that has called telemetry.Init()
  When the service logs a message
  Then the log output is JSON format
  And the log contains field: service_name
  And the log contains field: timestamp
  And the log contains field: level (debug/info/warn/error)
  And the log contains field: message
  And when agentId is available, log contains: agentId
  And when correlationId is available, log contains: correlationId
  And when cycleId is available, log contains: cycleId
```

### Scenario: Prometheus metrics exposed
```gherkin
Scenario: Service exposes standard metrics
  Given a service that has called telemetry.Init()
  When a GET request is made to /metrics
  Then the response contains metric: http_request_duration_seconds (histogram)
  And the response contains metric: http_requests_total (counter)
  And the response contains metric: http_active_requests (gauge)
  And metrics have label: service_name
  And metrics have label: method (for http metrics)
  And metrics have label: path (for http metrics)
  And metrics have label: status_code (for http metrics)
```

### Scenario: High-cardinality label handling
```gherkin
Scenario: Metrics avoid high-cardinality labels
  Given a service that has called telemetry.Init()
  When the service records metrics with agentId
  Then agentId is NOT added as a label to standard metrics
  But agentId is added to UsageEvent for billing/cost queries
  And metrics use low-cardinality labels only (service, method, status)
```

### Scenario: Frontend trace context creation
```gherkin
Scenario: Frontend creates trace context
  Given the SvelteKit frontend has telemetry module initialized
  When a user triggers an action (click, form submit)
  Then a trace context is created with trace_id and span_id
  And the trace context is attached to the outgoing API request
  And the same trace_id appears in backend traces
  And the frontend trace is the root span of the full trace
```

### Scenario: Frontend error tracking
```gherkin
Scenario: Frontend reports JavaScript errors
  Given the SvelteKit frontend has telemetry module initialized
  When an uncaught exception occurs in the browser
  Then the error is reported to the error tracking service
  And the error includes: message, stack trace, URL
  And the error includes the current trace_id if available
  And the error is correlated with backend traces by trace_id
```

### Scenario: Frontend performance monitoring
```gherkin
Scenario: Frontend monitors page performance
  Given the SvelteKit frontend has telemetry module initialized
  When a page loads or user interacts
  Then performance metrics are recorded
  And metrics include: page_load_time, time_to_interactive
  And metrics include: api_request_duration, websocket_latency
```

## Acceptance Criteria Mapping

| Scenario | Acceptance Criteria | Test Priority |
|----------|---------------------|---------------|
| Service initializes observability stack | All backend services use shared/telemetry | Must |
| Usage event emission (LLM/memory/tool) | Events queryable by agent_id, service, operation | Must |
| HTTP trace context propagation | Trace flows from API → cognitive engine → LLM | Must |
| NATS trace context propagation | Full end-to-end trace correlation | Must |
| Structured logging | All logs contain mandatory fields | Must |
| Prometheus metrics exposed | All services expose standard metrics on /metrics | Must |
| High-cardinality label handling | agentId NOT in Prometheus labels | Must |
| Frontend trace context creation | Same trace IDs flow from browser to backend | Should |
| Frontend error tracking | Errors correlated with backend traces | Should |
| Frontend performance monitoring | Performance metrics recorded | Could |