# Messaging Paradigm - Architecture

## Overview

This document describes how the messaging paradigm integrates with the ACE Framework architecture and how services communicate through NATS.

## Architecture Context

The Messaging Paradigm is foundational infrastructure that enables all inter-service communication in the ACE Framework. It sits between the Core API and future services (Cognitive Engine, Memory, Tools, Senses, LLM).

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              ACE Framework                               │
│                                                                          │
│  ┌──────────────┐      ┌──────────────┐      ┌──────────────────────┐  │
│  │   Frontend   │      │    Core      │      │   Cognitive Engine  │  │
│  │  SvelteKit   │◄────►│  API (Go)    │◄────►│        (Future)     │  │
│  │              │      │    Chi        │      │                      │  │
│  └──────┬───────┘      └──────┬───────┘      └──────────┬───────────┘  │
│         │                      │                         │              │
│         │              ┌───────┴───────┐                 │              │
│         │              │   Auth (JWT)  │                 │              │
│         │              │  WebSocket    │                 │              │
│         └──────────────┼───────────────┼─────────────────┘              │
│                        │               │                                  │
│         ┌──────────────┼───────────────┼───────────────────────────┐    │
│         │         Telemetry/Senses                            │    │
│         │  Inputs: Chat | Sensors | Metrics | Webhooks       │    │
│         └──────────────┬───────────────┬───────────────────────────┘    │
│                        │               │                                  │
│                        ▼               ▼                                  │
│                 ┌───────────┐   ┌───────────┐                          │
│                 │PostgreSQL │   │   NATS    │◄──── shared/messaging    │
│                 │  + SQLC   │   │+JetStream │                          │
│                 └───────────┘   └───────────┘                          │
│                        │               │                                  │
│                        ▼               ▼                                  │
│                 ┌─────────────────────────────────────────────────────┐   │
│                 │              Actuators (Outputs)                    │   │
│                 │  Chat | Tools | Signals | Export                   │   │
│                 └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘

                        │               │
                        │   Messaging   │
                        │    Layer      │
                        └───────────────┘
```

## Integration Points

### Core API → NATS

The Core API uses the messaging layer to:
- Publish agent lifecycle events (spawn, shutdown)
- Subscribe to agent thought streams
- Request-reply with Memory service
- Publish usage events for observability

### Cognitive Engine → NATS

The Cognitive Engine (when implemented) will:
- Publish layer inputs and outputs to NATS
- Subscribe to tool invocations
- Request-reply with Memory service
- Stream thoughts to frontend via WebSocket

### Memory → NATS

The Memory service will:
- Subscribe to store requests
- Publish query results
- Handle retries with dead letter queue

### Tools → NATS

Tool services will:
- Subscribe to invoke subjects
- Publish results
- Handle async tool execution

### Senses → NATS

Senses services will:
- Publish events from external inputs
- Route to appropriate cognitive engine agents

## Message Flow Examples

### Agent Spawn Flow

```
┌──────────┐    ┌─────────┐    ┌──────────┐    ┌──────────────┐
│  Frontend │───►│ Core API│───►│   NATS   │───►│ Cognitive    │
│          │    │         │    │         │    │ Engine       │
│ POST     │    │ Publish │    │ Subject:│    │ Subscribe    │
│ /agents  │    │         │    │ ace.system.agents.spawn│    │              │
└──────────┘    └─────────┘    └──────────┘    └──────────────┘
```

1. Frontend POSTs to `/api/v1/agents`
2. Core API validates and publishes to `ace.system.agents.spawn`
3. Cognitive Engine receives spawn event
4. Cognitive Engine creates agent and publishes status

### Thought Stream Flow

```
┌──────────────┐    ┌──────────────┐    ┌─────────┐    ┌──────────┐
│  Cognitive   │───►│    NATS      │───►│ Core API│───►│ Frontend │
│  Engine      │    │  (JetStream) │    │ (WS)   │    │          │
│              │    │              │    │         │    │          │
│ Publish      │    │ Subscribe    │    │ Stream  │    │ Receive  │
│ ace.engine.>│    │              │    │         │    │ thoughts  │
└──────────────┘    └──────────────┘    └─────────┘    └──────────┘
```

1. Cognitive Engine publishes layer outputs to `ace.engine.{agentId}.layer.{n}.output`
2. JetStream persists messages for replay
3. Core API subscribes to agent's thought stream
4. Frontend receives real-time updates via WebSocket

### Tool Invocation Flow

```
┌──────────────┐    ┌─────────┐    ┌──────────┐    ┌──────────────┐
│  Cognitive   │───►│   NATS  │───►│  Tools   │───►│  External   │
│  Engine      │    │         │    │ Service  │    │  Service     │
│              │    │ Request │    │         │    │              │
│ ace.tools.X  │    │ Reply  │    │ Invoke  │    │ HTTP call    │
└──────────────┘    └─────────┘    └──────────┘    └──────────────┘
```

1. Cognitive Engine publishes tool invocation to `ace.tools.{agentId}.{tool}.invoke`
2. Tools service receives and processes
3. Tools service publishes result to `ace.tools.{agentId}.{tool}.result`
4. Cognitive Engine receives result

### Memory Query Flow

```
┌──────────────┐    ┌─────────┐    ┌──────────┐    ┌──────────────┐
│  Cognitive   │───►│   NATS  │───►│  Memory  │───►│  PostgreSQL  │
│  Engine      │    │         │    │ Service  │    │              │
│              │    │ Request │    │         │    │              │
│ ace.memory.X │    │ Reply   │    │ Query   │    │ SELECT       │
└──────────────┘    └─────────┘    └──────────┘    └──────────────┘
```

1. Cognitive Engine requests memory via `ace.memory.{agentId}.query`
2. Memory service processes request
3. Memory service responds via NATS reply
4. Cognitive Engine receives query result

## Component Architecture

### shared/messaging Package

```
shared/messaging/
├── envelope.go      # Message envelope struct, header mapping
├── subjects.go      # Subject constants, templates, validation
├── client.go       # NATS client wrapper (interface + impl)
├── patterns.go      # Publish, RequestReply, Stream helpers
├── stream.go        # JetStream configuration
└── errors.go       # Error types
```

### Service Integration Pattern

```go
// 1. Initialize client
client, err := messaging.NewClient(messaging.Config{
    URLs:        os.Getenv("NATS_URLS"),
    Name:        "ace-cognitive-engine",
    Timeout:     10 * time.Second,
})
if err != nil {
    return err
}
defer client.Close()

// 2. Ensure streams exist (EnsureStreams uses Client interface directly)
ctx := context.Background()
if err := messaging.EnsureStreams(ctx, client); err != nil {
    return err
}

// 3. Subscribe to subjects
sub, err := client.Subscribe("ace.tools.agent-1.browse.invoke", handler)
if err != nil {
    return err
}
defer sub.Unsubscribe()

// 4. Publish events
if err := messaging.Publish(client, 
    "ace.engine.agent-1.layer.2.output",
    correlationID,  // propagate from incoming
    "agent-1",
    "cycle-123",
    "ace-cognitive-engine",
    payload,
); err != nil {
    return err
}
```

## Scaling Considerations

### Horizontal Scaling

Multiple instances of the Cognitive Engine can run with:
- **Shared JetStream consumers**: Use durable consumers with same name for load balancing
- **Queue groups**: `nats.Durable("consumer"), nats.Queue("group")`

```
Instance 1 ──┐
             ├──► JetStream ──► Messages distributed round-robin
Instance 2 ──┘     (Queue Group)
```

### Connection Pooling

NATS connections are multiplexed - a single connection can handle thousands of concurrent publishers/subscribers. No additional connection pooling needed.

### Consumer Groups

Consumer groups are per-service-type, not per-agent. Agent routing is handled by subject filtering:

| Service | Consumer Group | Filter Subject | Purpose |
|---------|---------------|----------------|---------|
| Cognitive Engine | `cognitive-engine` | `ace.engine.>` | All layer activity |
| Memory | `memory` | `ace.memory.>` | All memory operations |
| Tools | `tools` | `ace.tools.>` | All tool invocations |
| Layer Inspector | `inspector` | `ace.engine.{agentId}.layer.>` | Debug/replay |
| Usage Tracker | `usage` | `ace.usage.>` | Aggregate usage metrics |
| Safety Monitor | `safety` | `ace.*.*.*.>` | Security monitoring |

This approach avoids unbounded consumer accumulation � consumers are created per service type, not per agent.

## Observability

### Tracing

Each message includes:
- `X-Correlation-ID`: Links related messages across services
- `X-Cycle-ID`: Identifies cognitive cycle
- `X-Agent-ID`: Identifies agent
- `X-Source-Service`: Origin service

### Metrics

> **Note**: Metric instrumentation follows patterns defined in the observability unit. The specific metric names and labels will be defined there to ensure consistency across all services.

High-level categories for metrics:
- Message throughput (published/consumed)
- Request latency
- Connection health
- Consumer lag

### Logging

Log at key points:
- Message published
- Message consumed
- Request timeout
- Consumer error
- Stream creation

## Security

### Network Isolation

- NATS server runs on internal network
- Services connect via TLS
- No direct external access to NATS

### Authentication

- NATS credentials via environment variables
- Service identity in client name
- Optional: JWT authentication for NATS

### Message Validation

- Schema version in envelope for backward compatibility
- Optional: JSON Schema validation for payloads
- Envelope fields validated before publish

## Deployment

### Development

```yaml
# docker-compose.yaml
services:
  nats:
    image: nats:2.12
    ports:
      - "4222:4222"
    command: ["--js"]
    volumes:
      - nats-data:/data

volumes:
  nats-data:
```

### Production

```yaml
# production-config.yaml
cluster:
  name: ace-prod
  size: 3
  
streams:
  cognitive:
    retention: 24h
    storage: file
  usage:
    retention: 30d
    storage: file
  system:
    retention: 7d
    storage: file  # NOT memory - persist system events across restarts
```

## Failure Modes

| Scenario | Impact | Recovery |
|----------|--------|----------|
| NATS down | Services cannot communicate | Queue messages, retry on reconnect |
| JetStream down | No persistence - messages would be lost | **Fail loudly** - surface error, halt affected operations, alert via health check |
| Consumer crash | Messages unacknowledged | Redelivery after timeout |
| Producer flood | Backpressure | Flow control, circuit breaker |
