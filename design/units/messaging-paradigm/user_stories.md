# Messaging Paradigm - User Stories

## User Categories

1. **Service Developers** - Build services that communicate via NATS
2. **System Architects** - Define communication contracts
3. **DevOps/Platform** - Monitor and debug the system
4. **Frontend Developers** - Receive thought trace streams

## User Stories

### US-1: Message Envelope Standardization

**As a** Service Developer  
**I want** every message to have a standard envelope with message_id, correlation_id, agent_id, cycle_id, source_service, timestamp, and schema_version  
**So that** I can trace messages end-to-end and understand the context of any message without parsing its payload

**Acceptance Criteria:**
- [ ] Go struct for Envelope exists in shared/messaging
- [ ] Envelope fields are documented
- [ ] Helper functions exist to read/write envelope to NATS headers
- [ ] schema_version is included for backward compatibility

### US-2: NATS Subject Naming Convention

**As a** System Architect  
**I want** all NATS subjects to follow a consistent naming convention  
**So that** services can subscribe to specific patterns using wildcards

**Acceptance Criteria:**
- [ ] Subject template constants defined: `ace.<domain>.<agentId>.<subsystem>.<action>`
- [ ] System subject template: `ace.system.<subsystem>.<action>`
- [ ] All concrete subjects derived from templates are documented
- [ ] Wildcard patterns documented for common subscribers

### US-3: Agent ID First-Class Routing

**As a** Service Developer  
**I want** agentId to be a mandatory segment in subjects for all cognitive messages  
**So that** I can subscribe to everything for a specific agent without deserializing payloads

**Acceptance Criteria:**
- [ ] All agent-scoped subjects include agentId as third segment
- [ ] System messages (spawn, shutdown, health) use ace.system.* pattern
- [ ] Documentation shows examples for each domain

### US-4: Request-Reply Pattern

**As a** Service Developer  
**I want** to be able to send a message and wait for a synchronous response  
**So that** I can implement RPC-style calls (e.g., memory retrieval, LLM inference)

**Acceptance Criteria:**
- [ ] Helper function for request-reply exists
- [ ] Timeout configuration is supported
- [ ] Example usage documented

### US-5: Fire-and-Forget Pattern

**As a** Service Developer  
**I want** to publish events without waiting for a response  
**So that** I can emit layer outputs, tool results, and usage events efficiently

**Acceptance Criteria:**
- [ ] Helper function for publish exists
- [ ] Async publishing works without blocking
- [ ] Example usage documented

### US-6: Streaming Pattern

**As a** Frontend Developer  
**I want** to receive a stream of thought events for an agent's cognitive cycle  
**So that** I can display real-time thought traces in the UI

**Acceptance Criteria:**
- [ ] Helper function for subscribing to JetStream push consumer exists
- [ ] Subscription can be filtered by agentId
- [ ] Example shows how to integrate with WebSocket for frontend

### US-7: Shared NATS Wrapper

**As a** Service Developer  
**I want** to use a shared NATS client wrapper that handles connection management  
**So that** I don't need to implement reconnection, drain, and health checks myself

**Acceptance Criteria:**
- [ ] NewClient function creates configured client
- [ ] Automatic reconnection with backoff
- [ ] Graceful drain on shutdown
- [ ] Health check method compatible with readiness handler

### US-8: Health Check Integration

**As a** DevOps Engineer  
**I want** the NATS wrapper to expose a health check method  
**So that** the existing readiness handler can include NATS in health checks

**Acceptance Criteria:**
- [ ] HealthCheck method returns error if NATS or JetStream unavailable
- [ ] Can be called by readiness handler in same pattern as database Ping
- [ ] Check verifies connection alive AND JetStream responsive

### US-9: Testability Without NATS

**As a** Service Developer  
**I want** to be able to mock the messaging interface in unit tests  
**So that** tests don't require a running NATS server

**Acceptance Criteria:**
- [ ] NATS interface is defined that can be mocked
- [ ] Unit tests can use mock implementation
- [ ] Integration tests can use embedded NATS or test NATS

### US-10: Subject Constant Validation

**As a** System Architect  
**I want** subject names to be validated at build/test time  
**So that** typos in subject names cause build failures, not silent routing failures

**Acceptance Criteria:**
- [ ] Subject constants are typed (not raw strings)
- [ ] Test verifies all subjects match documented convention
- [ ] Code generation step (if used) is documented

### US-11: Error Handling Strategy

**As a** Service Developer  
**I want** to understand what happens when messages fail  
**So that** I can design appropriate error handling in my service

**Acceptance Criteria:**
- [ ] Dead letter stream configuration documented
- [ ] Retry policy documented
- [ ] Failure modes (service down, crash, retry exhausted) documented

### US-12: JetStream Configuration

**As a** Platform Engineer  
**I want** JetStream streams to be configured for persistence and durability  
**So that** messages are not lost and can be replayed for debugging

**Acceptance Criteria:**
- [ ] Stream configurations defined for each domain
- [ ] Retention strategy defined (categories, not exact durations)
- [ ] Consumer groups configured where needed for scaling

### US-13: Stream Ownership

**As a** Platform Engineer  
**I want** to know who creates JetStream streams on service startup  
**So that** I can ensure idempotent creation in production

**Acceptance Criteria:**
- [ ] Stream creation strategy documented (self, central, or migration)
- [ ] Idempotent creation approach defined
- [ ] Failure handling if stream already exists

### US-14: Message Ordering Within Cycle

**As a** Service Developer  
**I want** to understand how message ordering within a cycle is handled  
**So that** I can design appropriate aggregation logic

**Acceptance Criteria:**
- [ ] Aggregation responsibility documented
- [ ] Out-of-order handling documented
- [ ] Sequence number approach defined (if needed)
