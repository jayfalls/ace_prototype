# Messaging Paradigm - Implementation

## Overview

This document breaks the Messaging Paradigm unit into implementable micro-PRs. Each issue/PR should be independently testable and mergeable.

## Implementation Strategy

1. Start with foundational types (errors, subjects) that have no dependencies
2. Build the envelope (depends on errors for error types)
3. Implement the client (interface + mock + implementation together)
4. Add patterns on top of the client
5. Configure streams last as they're the highest-level abstraction
6. Add integration tests with embedded NATS

## Micro-PRs

### PR-1: Error Types

**Status:** Merged
**PR:** [#118](https://github.com/jayfalls/ace_prototype/pull/118)

**Files to create:**
- `shared/messaging/go.mod` - New module
- `shared/messaging/errors.go` - Error types and handling

**Implementation:**
- Define error types from FSD
- Add error wrapping helpers

**Acceptance Criteria:**
- [x] All error types from FSD are defined

---

### PR-2: Subject Constants

**Files to create:**
- `shared/messaging/subjects.go` - Subject type and constants

**Implementation:**
- Define `Subject` type
- Add all subject constants from FSD
- Add `Format` method for interpolation
- Add `Validate` method

**Acceptance Criteria:**
- All subjects from FSD are defined
- Format method works with variadic args
- Validation test passes

---

### PR-3: Message Envelope

**Files to create:**
- `shared/messaging/envelope.go` - Envelope struct and helpers

**Implementation:**
- Define `Envelope` struct with all required fields
- Add header mapping functions
- Add `NewEnvelope` constructor
- Add `GenerateMessageID` function

**Acceptance Criteria:**
- Envelope struct has all fields from FSD
- Header mapping works bidirectionally
- Unit tests for envelope creation and header mapping

---

### PR-4: NATS Client (Interface + Implementation)

**Files to create:**
- `shared/messaging/client.go` - Client interface and implementation

**Implementation:**
- Define `Client` interface with all methods from FSD
- Add `Config` struct
- Add mock client for testing
- Implement `natsClient` struct
- Implement `NewClient` function
- Implement all Client interface methods
- Add connection management (reconnect, drain)
- Add health check

**Acceptance Criteria:**
- Interface covers all communication patterns
- Mock client implements interface for testing
- Connects to NATS server
- Reconnects on disconnect
- Health check verifies connection and JetStream
- Drain gracefully closes connection

---

### PR-5: Communication Patterns

**Files to create:**
- `shared/messaging/patterns.go` - Publish, Request, Subscribe helpers

**Implementation:**
- Add `Publish` helper function
- Add `RequestReply` helper function
- Add stream subscription helpers

**Acceptance Criteria:**
- Publish sends messages with envelope headers
- RequestReply waits for response with timeout
- Correlation ID is preserved through the chain

---

### PR-6: JetStream Configuration

**Files to create:**
- `shared/messaging/stream.go` - Stream configuration

**Implementation:**
- Define stream configs from FSD
- Add `EnsureStreams` function
- Add consumer creation helpers
- Add DLQ helpers

**Acceptance Criteria:**
- Streams can be created idempotently
- Consumer configuration is correct
- DLQ stream can be created

---

### PR-7: Integration Tests

**Files to create:**
- `shared/messaging/*_test.go` - Integration tests

**Implementation:**
- Use embedded NATS server via `github.com/nats-io/nats-server/v2/server`
- Create `TestMain` to spin up embedded server
- Run all integration tests against the embedded server
- Tear down server after tests

**Testing Strategy:**
```go
import (
    "testing"
    "github.com/nats-io/nats-server/v2/server"
    "github.com/nats-io/nats.go"
)

var srv *server.Server

func TestMain(m *testing.M) {
    opts := &server.Options{
        Port:     -1, // random port
        JetStream: true,
    }
    var err error
    srv, err = server.NewServer(opts)
    if err != nil {
        panic(err)
    }
    go srv.Start()
    if !srv.ReadyForConnections(5 * time.Second) {
        panic("NATS server not ready")
    }
    code := m.Run()
    srv.Shutdown()
    os.Exit(code)
}

func TestPublish(t *testing.T) {
    nc, err := nats.Connect(srv.ClientURL())
    // test publish
}
```

**Acceptance Criteria:**
- Tests pass against embedded NATS
- Error cases are covered
- DLQ stream can be created

---

## Dependencies Between PRs

The dependency chain is linear:

```
PR-1 (Errors) ──► PR-2 (Subjects) ──► PR-3 (Envelope) ──► PR-4 (Client) ──► PR-5 (Patterns) ──► PR-6 (Streams) ──► PR-7 (Tests)
```

- Errors have no dependencies (foundational types)
- Subjects have no dependencies (string constants)
- Envelope uses error types for validation
- Client uses subjects and envelope
- Patterns use client
- Streams use client
- Tests verify everything


## Repository Structure After Implementation

```
shared/
└── messaging/
    ├── go.mod
    ├── envelope.go
    ├── subjects.go
    ├── client.go
    ├── patterns.go
    ├── stream.go
    ├── errors.go
    ├── envelope_test.go
    ├── subjects_test.go
    ├── client_test.go
    └── patterns_test.go
```

## Notes

- Each PR should include corresponding tests
- Add `shared/messaging` to `backend/go.work` for workspace resolution
- Services that depend on messaging add it to their go.mod require
- Document usage in package comments
- Ensure backward compatibility for schema version
