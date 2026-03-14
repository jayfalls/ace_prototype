# Messaging Paradigm - Functional Specification

## Overview

This document defines the functional specifications for the messaging paradigm, establishing the communication contract that all ACE Framework services will use.

## Package Structure

```
shared/messaging/
├── go.mod
├── envelope.go      # Message envelope struct and helpers
├── subjects.go      # Subject name constants and templates
├── client.go        # NATS client wrapper
├── patterns.go      # Request-reply, publish, subscribe helpers
├── stream.go        # JetStream configuration
└── errors.go        # Error types and handling
```

## Message Envelope

### Envelope Struct

```go
package messaging

import (
    "time"
)

const SchemaVersion = "1.0"

type Envelope struct {
    MessageID     string          `json:"message_id"`
    CorrelationID string          `json:"correlation_id"`
    AgentID       string          `json:"agent_id,omitempty"`
    CycleID       string          `json:"cycle_id,omitempty"`
    SourceService string          `json:"source_service"`
    Timestamp     time.Time       `json:"timestamp"`
    SchemaVersion string          `json:"schema_version"`
    Payload       json.RawMessage `json:"payload,omitempty"`
}
```

### Header Mapping

Envelope fields map to NATS headers:

| Envelope Field | Header Key |
|----------------|------------|
| MessageID | X-Message-ID |
| CorrelationID | X-Correlation-ID |
| AgentID | X-Agent-ID |
| CycleID | X-Cycle-ID |
| SourceService | X-Source-Service |
| Timestamp | X-Timestamp |
| SchemaVersion | X-Schema-Version |

### Helper Functions

```go
// NewEnvelope creates a new envelope with generated ID and timestamp
func NewEnvelope(correlationID, agentID, cycleID, sourceService string) *Envelope

// EnvelopeFromHeaders creates envelope from NATS message headers
func EnvelopeFromHeaders(msg *nats.Msg) (*Envelope, error)

// SetHeaders sets envelope fields as NATS headers on a message
func SetHeaders(msg *nats.Msg, env *Envelope)

// GenerateMessageID returns a new UUID v4
func GenerateMessageID() string
```

## Subject Naming

### Subject Type

```go
type Subject string

const (
    // Engine subjects
    SubjectEngineLayerInput  Subject = "ace.engine.%s.layer.%s.input"
    SubjectEngineLayerOutput Subject = "ace.engine.%s.layer.%s.output"
    SubjectEngineLoopStatus Subject = "ace.engine.%s.loop.%s.status"
    
    // Memory subjects
    SubjectMemoryStore  Subject = "ace.memory.%s.store"
    SubjectMemoryQuery  Subject = "ace.memory.%s.query"
    SubjectMemoryResult Subject = "ace.memory.%s.result"
    
    // Tools subjects
    SubjectToolsInvoke Subject = "ace.tools.%s.%s.invoke"
    SubjectToolsResult Subject = "ace.tools.%s.%s.result"
    
    // Senses subjects
    SubjectSensesEvent Subject = "ace.senses.%s.%s.event"
    
    // LLM subjects
    SubjectLLMRequest  Subject = "ace.llm.%s.request"
    SubjectLLMResponse Subject = "ace.llm.%s.response"
    SubjectLLMUsage   Subject = "ace.llm.%s.usage"
    
    // System subjects
    SubjectSystemAgentsSpawn   Subject = "ace.system.agents.spawn"
    SubjectSystemAgentsShutdown Subject = "ace.system.agents.shutdown"
    SubjectSystemHealth        Subject = "ace.system.health.%s"
)
```

### Subject Methods

```go
// Format returns the subject with interpolated values
func (s Subject) Format(args ...interface{}) string

// Validate checks if the subject matches expected patterns
func (s Subject) Validate() error
```

### Subject Categories

| Category | Pattern | Example |
|----------|---------|---------|
| Engine | `ace.engine.{agentId}.{subsystem}.{action}` | `ace.engine.agent-1.layer.2.input` |
| Memory | `ace.memory.{agentId}.{action}` | `ace.memory.agent-1.store` |
| Tools | `ace.tools.{agentId}.{toolName}.{action}` | `ace.tools.agent-1.browse.invoke` |
| Senses | `ace.senses.{agentId}.{senseType}.event` | `ace.agent-1.chat.event` |
| LLM | `ace.llm.{agentId}.{action}` | `ace.llm.agent-1.request` |
| System | `ace.system.{subsystem}.{action}` | `ace.system.agents.spawn` |

### Wildcard Patterns

| Subscriber | Wildcard Pattern | Purpose |
|------------|------------------|---------|
| Safety Monitor | `ace.*.*.*.>` | Watch all messages |
| Layer Inspector | `ace.engine.{agentId}.layer.>` | All layer activity for agent |
| Swarm Coordinator | `ace.system.swarm.>` | Coordination messages |
| Per-agent watcher | `ace.*.{agentId}.>` | All messages for agent |

## NATS Client

### Client Config

```go
type Config struct {
    URLs        string        // NATS server URLs (comma-separated)
    Name        string        // Client name
    Username    string        // Optional credentials
    Password    string        // Optional credentials
    Timeout     time.Duration // Connection timeout
    MaxReconnect int          // Max reconnection attempts
    ReconnectWait time.Duration // Wait between reconnects
}
```

### Client Interface

```go
type Client interface {
    // Publish sends a message without waiting for response
    Publish(subject string, data []byte, headers nats.Header) error
    
    // Request sends a message and waits for response
    Request(subject string, data []byte, timeout time.Duration) (*nats.Msg, error)
    
    // Subscribe creates a subscription
    Subscribe(subject string, handler MsgHandler) (*Subscription, error)
    
    // SubscribeToStream creates a JetStream consumer
    SubscribeToStream(ctx context.Context, stream, consumer, subject string, handler MsgHandler) error
    
    // HealthCheck verifies connection and JetStream
    HealthCheck() error
    
    // Drain gracefully closes connection
    Drain() error
    
    // Close closes connection
    Close()
}
```

### Implementation

```go
type natsClient struct {
    nc *nats.Conn
    js nats.JetStream
    cfg Config
}

func NewClient(cfg Config) (Client, error)
func (c *natsClient) Publish(subject string, data []byte, headers nats.Header) error
func (c *natsClient) Request(subject string, data []byte, timeout time.Duration) (*nats.Msg, error)
func (c *natsClient) Subscribe(subject string, handler MsgHandler) (*Subscription, error)
func (c *natsClient) SubscribeToStream(ctx context.Context, stream, consumer, subject string, handler MsgHandler) error
func (c *natsClient) HealthCheck() error
func (c *natsClient) Drain() error
func (c *natsClient) Close()
```

### Health Check

```go
func (c *natsClient) HealthCheck() error {
    // Check connection
    if err := c.nc.Ping(); err != nil {
        return fmt.Errorf("nats connection unhealthy: %w", err)
    }
    
    // Check JetStream
    _, err := c.js.AccountInfo()
    if err != nil {
        return fmt.Errorf("jetstream unhealthy: %w", err)
    }
    
    return nil
}
```

## Communication Patterns

### Fire-and-Forget (Publish)

```go
func Publish(client Client, subject Subject, agentID, cycleID, sourceService string, payload []byte) error {
    env := NewEnvelope(
        uuid.New().String(),
        agentID,
        cycleID,
        sourceService,
    )
    
    headers := make(nats.Header)
    SetHeadersFromEnvelope(headers, env)
    
    return client.Publish(subject.Format(agentID), payload, headers)
}
```

### Request-Reply

```go
func RequestReply(client Client, subject Subject, agentID, cycleID, sourceService string, payload []byte, timeout time.Duration) ([]byte, error) {
    env := NewEnvelope(
        uuid.New().String(),
        agentID,
        cycleID,
        sourceService,
    )
    
    headers := make(nats.Header)
    SetHeadersFromEnvelope(headers, env)
    
    reply, err := client.Request(subject.Format(agentID), payload, timeout)
    if err != nil {
        return nil, err
    }
    
    return reply.Data, nil
}
```

### Streaming (JetStream Push Consumer)

```go
func StreamThoughts(client Client, agentID string, handler func([]byte) error) error {
    ctx := context.Background()
    
    subject := Subject("ace.engine." + agentID + ".layer.>")
    
    return client.SubscribeToStream(
        ctx,
        "THOUGHTS",        // stream name
        "api-frontend",    // consumer name  
        string(subject),
        func(msg *nats.Msg) {
            // Process message
            handler(msg.Data)
            msg.Ack()
        },
    )
}
```

## JetStream Configuration

### Stream Definitions

```go
var Streams = []StreamConfig{
    {
        Name:       "COGNITIVE",
        Description: "Cognitive engine messages",
        Subjects:   []string{
            "ace.engine.>",
            "ace.memory.>",
            "ace.tools.>",
            "ace.senses.>",
            "ace.llm.>",
        },
        Retention:  nats.KeyValuePolicy, // Retain by key
        MaxBytes:   1 * 1024 * 1024 * 1024, // 1GB
        MaxAge:     24 * time.Hour,
        Storage:    nats.FileStorage,
        Replicas:   1,
    },
    {
        Name:       "USAGE",
        Description: "LLM usage events",
        Subjects:   []string{
            "ace.llm.usage",
        },
        Retention:  nats.LimitsPolicy,
        MaxBytes:   100 * 1024 * 1024, // 100MB
        MaxAge:     30 * 24 * time.Hour, // 30 days
        Storage:    nats.FileStorage,
        Replicas:   1,
    },
    {
        Name:       "SYSTEM",
        Description: "System events",
        Subjects:   []string{
            "ace.system.>",
        },
        Retention:  nats.WorkQueuePolicy,
        MaxBytes:   10 * 1024 * 1024, // 10MB
        Storage:    nats.MemoryStorage,
        Replicas:   1,
    },
}
```

### Stream Creation

```go
func EnsureStreams(ctx context.Context, js nats.JetStream) error {
    for _, cfg := range Streams {
        _, err := js.CreateStream(ctx, cfg)
        if err != nil && !errors.Is(err, nats.ErrStreamNameConflict) {
            return err
        }
    }
    return nil
}
```

### Consumer Groups

For horizontal scaling, use durable consumers with queue groups:

```go
func CreateConsumer(ctx context.Context, js nats.JetStream, stream, consumer, durable string) error {
    _, err := js.CreateConsumer(ctx, stream, nats.ConsumerConfig{
        Durable:        durable,
        DeliverPolicy: nats.DeliverNewPolicy,
        AckPolicy:     nats.AckExplicitPolicy,
        AckWait:       30 * time.Second,
        MaxDeliver:    3,
        FilterSubject: stream + ".>",
    })
    return err
}
```

## Error Handling

### Error Types

```go
var (
    ErrConnectionFailed = errors.New("nats connection failed")
    ErrJetStreamDown    = errors.New("jetstream unavailable")
    ErrTimeout          = errors.New("request timeout")
    ErrNoResponders     = errors.New("no responders for request")
)
```

### Dead Letter Configuration

```go
type DeadLetterConfig struct {
    StreamName      string
    MaxDeliverAttempts int
    Backoff          time.Duration
}

func ConfigureDeadLetters(js nats.JetStream, consumerCfg *nats.ConsumerConfig) error {
    consumerCfg.DeadLetterSubject = "DLQ." + consumerCfg.FilterSubject
    consumerCfg.MaxDeliver = 3
    
    // Create DLQ stream if needed
    dlqStream := "DLQ"
    _, err := js.CreateStream(context.Background(), nats.StreamConfig{
        Name:    dlqStream,
        Subjects: []string{"DLQ.>"},
    })
    return err
}
```

### Retry Policy

| Message Type | Max Retries | Backoff |
|--------------|--------------|---------|
| Tool invocation | 5 | Exponential (1s, 2s, 4s, 8s, 16s) |
| Memory query | 3 | Exponential (500ms, 1s, 2s) |
| LLM request | 3 | Exponential (1s, 2s, 4s) |
| Usage events | 1 | None |
| Layer output | 3 | Exponential (1s, 2s, 4s) |

## Testing

### Interface for Mocking

```go
type MockClient struct {
    PublishedMsgs   []*MockMsg
    Subscriptions  []*MockSubscription
    RequestResp    []byte
    RequestErr     error
}

type MockMsg struct {
    Subject string
    Data    []byte
    Headers nats.Header
}

func (m *MockClient) Publish(subject string, data []byte, headers nats.Header) error {
    m.PublishedMsgs = append(m.PublishedMsgs, &MockMsg{Subject: subject, Data: data, Headers: headers})
    return nil
}

// ... implement all Client interface methods
```

### Usage in Tests

```go
func TestHandlerProcessMessage(t *testing.T) {
    mockClient := &messaging.MockClient{}
    handler := NewHandler(mockClient)
    
    // Test with mock
    err := handler.Process(context.Background(), []byte(`{"test": true}`))
    assert.NoError(t, err)
    assert.Len(t, mockClient.PublishedMsgs, 1)
}
```

## Open Questions Resolved

### Subject Structure Variability

The FSD acknowledges that subject depth varies:
- Standard: 4 segments (`ace.domain.agentId.action`)
- Extended: 5+ segments (`ace.engine.agentId.layer.2.input`)
- System: 3 segments (`ace.system.health.service`)

The subject templates handle this via `fmt.Sprintf` interpolation.

### Stream Ownership

Each service creates its own streams on startup using `CreateOrUpdateStream`. This is idempotent - if the stream already exists, it succeeds without error.

### Session Context

The FSD places `session_id` in the message payload, not the envelope. The envelope focuses on routing and correlation. Session reconstruction is done by correlating cycle_ids within a session.

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| NATS_URLS | NATS server URLs | nats://localhost:4222 |
| NATS_CLIENT_NAME | Client name | ace-service |
| NATS_USERNAME | Optional username | (none) |
| NATS_PASSWORD | Optional password | (none) |
| NATS_TIMEOUT | Connection timeout | 10s |
| NATS_MAX_RECONNECT | Max reconnect attempts | 10 |
| NATS_RECONNECT_WAIT | Wait between reconnects | 1s |

### Example Usage

```go
cfg := messaging.Config{
    URLs:        getenv("NATS_URLS", "nats://localhost:4222"),
    Name:        getenv("NATS_CLIENT_NAME", "ace-api"),
    Timeout:     10 * time.Second,
    MaxReconnect: 10,
    ReconnectWait: 1 * time.Second,
}

client, err := messaging.NewClient(cfg)
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```
