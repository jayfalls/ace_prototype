# Messaging Paradigm - Research

## Overview

This document evaluates technical approaches for implementing the messaging paradigm, focusing on the NATS Go client, subject naming patterns, and connection management strategies.

## NATS Go Client

### Option A: Official github.com/nats-io/nats.go

**Pros:**
- Official NATS client, actively maintained by Synadia
- Supports both core NATS and JetStream
- Modern v2 API with context support
- Well-documented with examples

**Cons:**
- API surface is large; need to wrap for our use cases
- Requires understanding of both `nats` and `jetstream` packages

**Recommendation:** Use official client. It's the standard and well-supported.

### Option B: Third-party wrappers

**Analysis:** Not recommended. The official client is comprehensive, and wrapping it further would add unnecessary abstraction.

### Version

Use latest stable: `github.com/nats-io/nats.go/v2` (as of 2024, v2 is the current stable API)

## Connection Management

### Option A: Service-managed connections

Each service creates and manages its own NATS connection via the shared wrapper.

**Pros:**
- Simple to understand
- No shared state between services

**Cons:**
- Each service needs configuration

**Recommendation:** Services create their own connection via `shared/messaging.NewClient()`. Wrapper handles connection lifecycle.

### Option B: Central connection pool

**Analysis:** Not needed. NATS multiplexes efficiently over a single connection per process.

## Subject Naming Implementation

### Option A: String constants package

```go
package subjects

const (
    EngineLayerInput  = "ace.engine.%s.layer.%s.input"
    MemoryStore      = "ace.memory.%s.store"
)
```

**Pros:**
- Simple
- Easy to use with fmt.Sprintf

**Cons:**
- Typos not caught at compile time
- No validation

### Option B: Typed constants with validation

```go
type Subject string

const (
    EngineLayerInput Subject = "ace.engine.%s.layer.%s.input"
)

func (s Subject) Format(args ...interface{}) string {
    return fmt.Sprintf(string(s), args...)
}

func (s Subject) Validate() error {
    // validate format matches pattern
}
```

**Pros:**
- Compile-time type safety
- Can add validation

**Cons:**
- More code

**Recommendation:** Use Option B with validation test. Prevents silent routing failures from typos.

## Message Envelope

### Option A: Struct with JSON serialization

```go
type Envelope struct {
    MessageID    string `json:"message_id"`
    CorrelationID string `json:"correlation_id"`
    AgentID      string `json:"agent_id,omitempty"`
    CycleID      string `json:"cycle_id,omitempty"`
    SourceService string `json:"source_service"`
    Timestamp    time.Time `json:"timestamp"`
    SchemaVersion string `json:"schema_version"`
    Payload      json.RawMessage `json:"payload"`
}
```

**Pros:**
- Simple
- JSON is human-readable
- Easy to debug

**Cons:**
- Slightly more verbose than headers-only

**Recommendation:** Envelope struct that serializes to NATS headers. Payload separate in message body.

## Health Check Approach

### Option A: Connection ping only

```go
func (c *Client) HealthCheck() error {
    return c.nc.Ping()
}
```

**Pros:**
- Simple

**Cons:**
- Doesn't verify JetStream

### Option B: Connection + JetStream verification

```go
func (c *Client) HealthCheck() error {
    if err := c.nc.Ping(); err != nil {
        return err
    }
    // Verify JetStream is responsive
    _, err := c.js.AccountInfo()
    return err
}
```

**Pros:**
- More thorough
- Catches JetStream issues

**Cons:**
- Slightly more expensive

**Recommendation:** Option B. Health check should verify both connection and JetStream.

## Testing Strategy

### Option A: Interface mocking

```go
type Publisher interface {
    Publish(subject string, data []byte) error
    Subscribe(subject string, handler func(msg *Msg)) (*Subscription, error)
}
```

**Pros:**
- Easy to mock
- Services can test without NATS

**Cons:**
- Interface must cover all needed functionality

### Option B: Embedded NATS server

Use `github.com/nats-io/nats-server/v2/server` for integration tests.

**Pros:**
- Real NATS behavior
- Tests actual serialization

**Cons:**
- Heavier tests

**Recommendation:** Option A for unit tests, Option B for integration tests if needed.

## JetStream Stream Configuration

### Stream Ownership

**Option A: Service self-provisioning**

Each service creates its streams on startup using idempotent `CreateOrUpdateStream`.

**Pros:**
- Services are self-contained
- No central coordination

**Cons:**
- Race condition if multiple instances start simultaneously

**Option B: Central provisioning

Separate provisioning step creates all streams before services start.

**Pros:**
- No race conditions
- Clear ownership

**Cons:**
- Additional deployment step

**Recommendation:** Option A with idempotent `CreateOrUpdateStream`. Handle "stream already exists" gracefully.

### Retention Categories

Define stream categories:
1. **Cognitive streams**: `ace.engine.>`, `ace.memory.>`, `ace.tools.>`, `ace.senses.>`, `ace.llm.>`
2. **Usage streams**: `ace.llm.usage`
3. **System streams**: `ace.system.>`

Each category can have different retention policies.

## Error Handling

### Dead Letter Queues

JetStream supports dead letter streams:
- Configure `MaxDeliver` retries
- Set `DeadLetterSubject` for failed messages

**Recommendation:** Configure dead letter streams per consumer, not globally.

### Retry Policy

- Cognitive messages: 3 retries with exponential backoff
- Tool invocations: 5 retries (more critical)
- Usage events: 1 retry (best effort)

## Conclusion

**Recommended Stack:**
- Client: `github.com/nats-io/nats.go/v2`
- Wrapper: Service-managed connections with health checks
- Subjects: Typed constants with validation tests
- Envelope: Struct with NATS headers
- Testing: Interface mocking for unit tests
- Streams: Self-provisioning with idempotent CreateOrUpdateStream
