# Messaging Paradigm - Problem Space

## Overview

The ACE Framework is a distributed multi-service system where the cognitive engine, API, memory, senses, tools, and future services all need to communicate with each other. Currently there is no defined contract for how that communication happens — no agreed message format, no subject naming convention, no rules about which services publish and which subscribe, and no shared Go types that encode any of this.

This unit establishes the communication contract that every future service will depend on. These primitives live in `shared/messaging/` and become the single source of truth for how the system's components talk to each other.

## Core Problems

### 1. Message Envelope (Solved)

Every message needs:
- `message_id` - UUID, generated at publish time
- `correlation_id` - Traces a chain of causally related messages (when Layer 3 triggers Layer 4 which triggers a tool call, all three share the same correlation_id)
- `agent_id` - Mandatory on everything except system-level infrastructure messages
- `cycle_id` - The cognitive cycle that produced this message (critical for Layer Inspector and aggregating multiple messages from the same cycle)
- `source_service` - Which service published it
- `timestamp` - When the message was published
- `schema_version` - Version of the message shape

**Solution**: Envelope fields go in NATS JetStream headers. Payload is separate, allowing subscribers to make routing/filtering decisions by reading headers without deserializing the full payload.

### 2. NATS Subject Structure (Solved)

```
ace.<domain>.<agentId>.<subsystem>.<action>
```

- `ace.` prefix namespaces everything and prevents collisions
- System-level messages (not agent-scoped): `ace.system.<subsystem>.<action>`
- Wildcards are essential:
  - Safety Monitor: `ace.*.*.*.>` (watches everything)
  - Layer Inspector: `ace.engine.<agentId>.layer.>` (all layer activity for one agent)
  - Swarm Coordinator: `ace.system.swarm.>` (coordination messages)

No tenant namespacing at this stage. Multi-tenancy in the enterprise sense is not in scope for MVP.

### 3. Agent ID Enforcement (Solved)

Both routing and traceability - inseparable. When API spawns an agent it publishes to `ace.system.agents.spawn` with agentId in payload. From that point on, every message related to that agent's cognition, memory, tools, and senses carries agentId as a subject segment.

**Concrete subject examples:**
```
ace.engine.{agentId}.layer.{layerId}.input
ace.engine.{agentId}.layer.{layerId}.output
ace.engine.{agentId}.loop.{loopId}.status
ace.memory.{agentId}.store
ace.memory.{agentId}.query
ace.memory.{agentId}.result
ace.tools.{agentId}.{toolName}.invoke
ace.tools.{agentId}.{toolName}.result
ace.senses.{agentId}.{senseType}.event
ace.llm.{agentId}.request
ace.llm.{agentId}.response
ace.llm.{agentId}.usage
ace.system.agents.spawn
ace.system.agents.shutdown
ace.system.health.{serviceId}
```

System lifecycle events don't have agentId segment because they are infrastructure, not cognitive.

### 4. Communication Patterns (Solved)

**Request-reply** (synchronous result needed):
- Cognitive engine asking memory service for long-term retrieval
- Layer asking LLM gateway for inference result
- API asking engine for agent's current status
- Use NATS request-reply with inbox subjects

**Fire-and-forget** (no result needed):
- Layer publishing output to northbound bus
- Tool publishing result back to engine
- Sense publishing incoming event
- Usage tracking events
- Vast majority of cognitive cycle messages

**Streaming** (continuous flow):
- Thought trace pipeline to frontend
- Engine publishes sequence of thought events for a cycle
- API subscribes via JetStream push consumer, forwards over WebSocket

### 5. Service Connection Management (Solved)

Shared NATS client wrapper in `shared/messaging/` handles:
- Connection establishment with retry and backoff
- Reconnection with same retry logic
- Graceful drain on shutdown (use NATS drain concept)
- Health exposure for readiness check

All services are both publishers and subscribers. Connection pooling not needed - NATS multiplexes efficiently over single connection.

### 6. Additional Decisions (Solved)

- **JetStream from day one**: Gives at-least-once delivery, message replay, consumer groups, KV store
- **KV store** for agent configuration: Subscribe to KV watch instead of polling database
- **JSON for payloads**: Operational simplicity, human-readable, directly injectable into LLM prompts
- **No message signing**: TLS for NATS connection (handled in security unit) is sufficient
- **Low throughput**: Design for correctness and observability, not throughput

---

## Additional Problems Identified

### 7. Error Handling and Dead Letters

What happens when:
- Message published but consuming service is down?
- Service crashes mid-processing?
- Message exceeds retry limit?

JetStream has configurable retry policies and can route failed messages to a dead letter stream. Need defined strategy:
- Cognitive engine layer publishes tool invocation, never gets result - retry? fail cycle? escalate to Safety Monitor?

### 8. Message Ordering Within a Cycle

Multiple messages sharing same `cycle_id` are aggregated at cycle boundary. Who is responsible for aggregation? What if messages arrive out of order (possible when layers run at different speeds)?

NATS JetStream guarantees ordering per subject but not across subjects.

### 9. Competing Consumers for Horizontal Scaling

When multiple cognitive engine pods run (Kubernetes multi-agent scenario), they cannot all consume from same stream naively. Need JetStream durable consumers with queue group strategy:
- Which streams use queue groups?
- Which don't?
- How does engine know which agent instances it is responsible for?

### 10. Stream Retention Policy

JetStream streams need retention configuration:
- Cognitive messages: how long?
- Usage events: how long (for cost attribution)?
- System events: how long?
- Layer Inspector replay depends on messages being retained long enough for debugging

### 11. Testing Without Live NATS

`shared/messaging` will be imported by every service. Unit tests should not require running NATS server.
- Option A: Wrapper exposes interface that can be mocked
- Option B: Include embedded NATS server helper for integration tests
- This decision shapes the entire package's API design

### 12. Subject Constant Validation

Subject names will be string constants. A typo (`ace.engine.{agentId}.layre.output`) causes silent routing failure - publisher sends, nothing consumes.

How to enforce correctness:
- Typed constants?
- Code generation step?
- At minimum: test that verifies all defined subjects match documented convention

### 13. Health Check Integration

Existing readiness handler checks database pool via `Ping`. When messaging unit delivers shared NATS wrapper:
- Wrapper must expose health check method compatible with readiness handler pattern
- "NATS healthy" = TCP connection open AND JetStream reachable AND service's streams/consumers exist

---

## Open Questions for FSD

### Subject Structure Variability

The defined pattern is `ace.<domain>.<agentId>.<subsystem>.<action>` but some concrete examples don't fit cleanly:
- `ace.memory.{agentId}.store` - no subsystem segment
- `ace.engine.{agentId}.layer.{layerId}.input` - more than five segments

The FSD needs to either tighten the pattern or explicitly acknowledge variable depth subjects.

### Stream Ownership

When a new service starts up, who creates the JetStream streams it needs?
- Option A: Service creates its own streams on startup
- Option B: Central provisioning step
- Option C: Migration-style setup script

This matters because if two cognitive engine pods start simultaneously they'll both try to create the same streams. Need idempotent stream creation or central management.

### Session Context

The data model has sessions - a user-agent interaction context above the cycle level. Should `session_id` belong on the envelope or purely in the payload? This affects Layer Inspector's ability to reconstruct a full session's thought trace, not just a single cycle's.

---

## Success Criteria

1. Every service uses shared NATS wrapper from `shared/messaging/`
2. All inter-service messages follow envelope format with headers for metadata
3. All subjects follow naming convention with agentId as mandatory segment
4. All three communication patterns are supported and documented
5. Health check integration works with existing readiness handler
6. Unit tests can run without NATS server (mockable interface)
7. Subject constants are validated at build/test time

---

## Dependencies

- **Before**: Core API (must exist to receive messages)
- **After**: Observability, Auth, all feature services (all need these primitives to publish events correctly)

This unit must be completed before any service beyond the API is built.
