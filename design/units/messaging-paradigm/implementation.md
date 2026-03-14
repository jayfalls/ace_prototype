# Messaging Paradigm - Implementation

## Overview

This document breaks the Messaging Paradigm unit into implementable micro-PRs. Each issue/PR should be independently testable and mergeable.

## Implementation Strategy

1. Start with foundational pieces (envelope, subjects) that other pieces depend on
2. Build the client wrapper next as it provides the infrastructure
3. Add patterns on top of the client
4. Configure streams last as they're the highest-level abstraction
5. Testing throughout

## Micro-PRs

### PR-1: Message Envelope

**Files to create/modify:**
- `shared/messaging/go.mod` - New module
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

### PR-2: Subject Constants

**Files to create/modify:**
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

### PR-3: NATS Client Interface

**Files to create/modify:**
- `shared/messaging/client.go` - Client interface

**Implementation:**
- Define `Client` interface with all methods from FSD
- Add `Config` struct
- Add mock client for testing

**Acceptance Criteria:**
- Interface covers all communication patterns
- Mock client implements interface for testing

---

### PR-4: NATS Client Implementation

**Files to create/modify:**
- `shared/messaging/client.go` - Client implementation

**Implementation:**
- Implement `natsClient` struct
- Implement `NewClient` function
- Implement all Client interface methods
- Add connection management (reconnect, drain)
- Add health check

**Acceptance Criteria:**
- Connects to NATS server
- Reconnects on disconnect
- Health check verifies connection and JetStream
- Drain gracefully closes connection

---

### PR-5: Communication Patterns

**Files to create/modify:**
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

**Files to create/modify:**
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

### PR-7: Error Types

**Files to create/modify:**
- `shared/messaging/errors.go` - Error types and handling

**Implementation:**
- Define error types from FSD
- Add error wrapping helpers

**Acceptance Criteria:**
- All error types from FSD are defined

---

### PR-8: Integration Tests

**Files to create/modify:**
- `shared/messaging/*_test.go` - Integration tests

**Implementation:**
- Add tests using embedded NATS if available
- Test all communication patterns
- Test stream creation

**Acceptance Criteria:**
- Tests pass against real NATS
- Error cases are covered

---

## Dependencies Between PRs

```
PR-1 (Envelope)
    │
    ▼
PR-2 (Subjects) ◄────────┐
    │                    │
    ▼                    │
PR-3 (Client Interface)  │
    │                    │
    ▼                    │
PR-4 (Client Impl)      │
    │                    │
    ▼                    │
PR-5 (Patterns) ─────────┤
    │                    │
    ▼                    │
PR-6 (Streams) ──────────┤
    │                    │
    ▼                    │
PR-7 (Errors) ───────────┤
    │                    │
    ▼                    │
PR-8 (Tests) ───────────┘
```

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
- Update `shared/go.mod` to include messaging module as dependency
- Document usage in package comments
- Ensure backward compatibility for schema version
