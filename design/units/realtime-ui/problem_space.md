# Problem Space: Real-time UI Updates & Retry Mechanisms

## Core Conflict

Users need to see system state as it evolves **without refreshing**, even during network instability or backend pressure. The frontend must maintain an accurate view of backend state through **any combination** of transport failures, reconnections, and concurrent modifications.

## Constraints

### Functional Constraints
- **All data that benefits from real-time should have it** — not just chat messages, but agent status, cognitive cycle progress, logs, system health, resource changes
- **Multi-user collaborative environment** — many users may watch the same resource simultaneously
- **History + forward** — reconnecting clients receive missed history then continue with live updates
- **Last-write-wins** — simple conflict resolution, no complex merge logic required
- **Single binary architecture** — WebSocket server lives in the backend service alongside API, agents, providers

### Transport Constraints
- **WebSockets + Polling as equals** — both are first-class citizens that work in tandem
- **WebSockets for efficiency** when available, **polling for resilience** when blocked (corporate proxies, mobile networks)
- **Automatic degradation** — system functions fully even if WebSocket connection fails entirely
- **Exactly-once delivery** is acceptable — no need for complex ordering guarantees if messages are designed correctly

### Scale Constraints
- **Many agents → single user** — one user may monitor many agents
- **Many users → same resource** — broadcast capability required for shared views
- **Current scale:** 10s-100s concurrent connections per node (design should not preclude horizontal scaling)

## Success Metrics

1. **Zero refresh required** — Users never need to manually refresh to see current state
2. **Sub-second latency** — Updates visible within 1 second under normal conditions
3. **Seamless reconnection** — After any network interruption, state synchronizes automatically without user action
4. **Graceful degradation** — System remains functional (via polling) even if WebSockets completely unavailable
5. **Observable health** — Clear visibility into connection state, retry attempts, and sync status

## Key Questions Resolved

| Question | Answer |
|----------|--------|
| What data gets real-time updates? | Anything that benefits — agent status, cycles, logs, health, resources |
| User model? | Multi-user collaborative — shared agents and resources |
| Reconnection behavior? | Receive history (missed updates) + continue live stream |
| Conflict resolution? | Last-write-wins (simple, predictable) |
| Architecture? | Single binary — backend service houses API + WebSockets |
| Polling role? | First-class citizen, works in tandem with WebSockets |
| Delivery guarantees? | Exactly-once acceptable, ordering not critical |
| Connection patterns? | Many agents per user, many users per resource (broadcast) |

## Related Units

- **Frontend Design** — Provides the SvelteKit foundation this builds upon
- **Messaging Paradigm** — NATS patterns may inform pub/sub architecture
- **Cognitive Engine** — Generates many of the events that need real-time distribution
- **Observability** — Connection health and sync status must be observable
