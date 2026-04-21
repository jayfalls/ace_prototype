# Real-time UI Updates & Retry Mechanisms

**Unit**: realtime-ui  
**Status**: ✅ Complete  
**Stack**: Go (coder/websocket, NATS) + Svelte 5 (runes)

---

## Overview

The realtime-ui unit delivers WebSocket-based real-time updates with automatic polling fallback, NATS-to-client event bridging, reconnection with replay, and connection status UI. Users see system state as it evolves without refreshing, even during network instability.

---

## Architecture

### Backend (Go)

```
Browser ←→ WebSocket/Polling ←→ Handler ←→ Hub ←→ TopicReg ←→ NATS
                                    ↓
                                SeqBuffer (replay)
```

| Component | File | Purpose |
|-----------|------|---------|
| **Hub** | `backend/internal/api/realtime/hub.go` | Central registry: manages clients, fans out NATS events, enforces authorization |
| **Client** | `backend/internal/api/realtime/client.go` | Per-connection state: read/write pumps, rate limiting (100 msg/s), metrics |
| **TopicReg** | `backend/internal/api/realtime/topic.go` | NATS subscription registry with reference counting |
| **SeqBuffer** | `backend/internal/api/realtime/seq.go` | Per-topic ring buffer for replay on reconnect |
| **Handler** | `backend/internal/api/realtime/handler.go` | WebSocket upgrade + polling endpoint, OTel spans, auth via first message |
| **Config** | `backend/internal/api/realtime/config.go` | Constants: timeouts, limits, metric names |

### Frontend (Svelte 5)

| Component | File | Purpose |
|-----------|------|---------|
| **RealtimeManager** | `frontend/src/lib/realtime/manager.svelte.ts` | Singleton: connection lifecycle, subscribe/unsubscribe, event routing |
| **WebSocketConnection** | `frontend/src/lib/realtime/connection.svelte.ts` | Low-level WS wrapper with auth flow |
| **ReconnectManager** | `frontend/src/lib/realtime/reconnect.ts` | Exponential backoff (1s→30s), max 5 attempts |
| **PollingClient** | `frontend/src/lib/realtime/polling.ts` | Adaptive polling (1s active / 10s idle), activity detection |
| **AgentStore** | `frontend/src/lib/stores/agents.svelte.ts` | Real-time agent status updates |
| **UsageStore** | `frontend/src/lib/stores/usage.svelte.ts` | Real-time usage event feed |
| **ConnectionIndicator** | `frontend/src/lib/components/realtime/ConnectionIndicator.svelte` | Status badge in sidebar |
| **LiveBadge** | `frontend/src/lib/components/realtime/LiveBadge.svelte` | Pulsing green dot |

---

## Message Protocol

### Client → Server

| Type | Fields | Purpose |
|------|--------|---------|
| `auth` | `token` | Authenticate connection (first message) |
| `subscribe` | `topics[]` | Subscribe to topics (max 50) |
| `unsubscribe` | `topics[]` | Unsubscribe from topics |
| `replay` | `topics[]`, `since_seq` | Request missed events |
| `ping` | — | Heartbeat |

### Server → Client

| Type | Fields | Purpose |
|------|--------|---------|
| `auth_ok` | `connection_id` | Authentication success |
| `auth_error` | `reason` | Authentication failure |
| `subscribed` | `topics[]` | Subscription confirmed |
| `unsubscribed` | `topics[]` | Unsubscription confirmed |
| `event` | `topic`, `seq`, `data` | Real-time event |
| `resync_required` | `topics[]` | Buffer exceeded, fetch via REST |
| `pong` | — | Heartbeat response |
| `error` | `message` | General error |

---

## Topic Format

```
{resourceType}:{resourceId}:{eventType}
```

| Topic | NATS Subject | Description |
|-------|-------------|-------------|
| `agent:{id}:status` | `ace.engine.{id}.layer.>` | Agent status changes |
| `agent:{id}:logs` | `ace.engine.{id}.loop.>` | Agent log output |
| `agent:{id}:cycles` | `ace.engine.{id}.layer.6.output` | Cognitive cycle results |
| `system:health` | `ace.system.health.>` | System health events |
| `usage:{id}` | `ace.usage.{id}.>` | Usage/cost events |

---

## Connection Lifecycle

```
disconnected → connecting → connected
                  ↓              ↓
             reconnecting    (WS close)
                  ↓              ↓
             polling ←──── reconnecting (periodic retry)
                  ↓
             disconnected (manual)
```

- **WebSocket primary**: Connects to `/api/ws`, authenticates via first message with JWT
- **Polling fallback**: `GET /api/realtime/updates?topics=...&since_seq=...` (existing auth middleware)
- **Reconnection**: Exponential backoff 1s, 2s, 4s, 8s, 16s (capped at 30s), max 5 attempts
- **Replay**: On reconnect, sends `replay` with `lastSeq` per topic; if buffer exceeded, receives `resync_required` and fetches via REST
- **Token refresh**: `realtimeManager.refreshAuth(newToken)` sends new auth without reconnecting

---

## Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/ws` | First message (JWT) | WebSocket upgrade |
| `GET` | `/api/realtime/updates` | Bearer token | Polling endpoint |

---

## Observability

### OTel Spans

| Span | Attributes |
|------|-----------|
| `realtime.ws.upgrade` | `user_id`, `connection_id` |
| `realtime.ws.auth` | `success` |
| `realtime.ws.subscribe` | `user_id`, `topics` |
| `realtime.ws.disconnect` | `user_id`, `connection_id`, `duration_ms` |
| `realtime.poll` | `user_id`, `topics`, `since_seq` |

### OTel Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `ace.realtime.ws.connections.active` | Gauge | Active WebSocket connections |
| `ace.realtime.ws.messages.sent` | Counter | Messages sent to clients |
| `ace.realtime.ws.messages.received` | Counter | Messages received from clients |
| `ace.realtime.ws.errors` | Counter | Write errors, auth failures |
| `ace.realtime.poll.requests` | Counter | Polling requests |
| `ace.realtime.poll.events.delivered` | Counter | Events delivered via polling |
| `ace.realtime.buffer.replay.events` | Counter | Replay events sent |
| `ace.realtime.buffer.resync.required` | Counter | Resync-required responses |

### Rate Limits

| Limit | Value | Config |
|-------|-------|--------|
| WS messages/second | 100 | `WS_RATE_LIMIT` |
| WS subscriptions | 50 | `WS_MAX_SUBSCRIPTIONS` |
| WS message size | 64KB | `WS_MAX_MESSAGE_SIZE` |
| Polling requests/min | 60 | `POLL_RATE_LIMIT` |

---

## Usage

### Frontend — RealtimeManager

```typescript
import { realtimeManager } from '$lib/realtime/manager.svelte';

// Connect (call after login)
realtimeManager.connect(token);

// Subscribe to topics
realtimeManager.subscribe(['agent:123:status', 'system:health']);

// Listen for events
realtimeManager.on('agent:123:status', (data) => {
  console.log('Agent status changed:', data);
});

// Disconnect (call on logout)
realtimeManager.disconnect();

// Refresh auth token (no reconnect needed)
realtimeManager.refreshAuth(newToken);

// Reactive status
$derived(realtimeManager.status); // 'connected' | 'connecting' | 'polling' | 'disconnected' | 'reconnecting'
```

### Frontend — AgentStore

```typescript
import { agentStore } from '$lib/stores/agents';

// Initialize (call on page mount)
await agentStore.init();

// Reactive agent list
$derived(agentStore.agents);
$derived(agentStore.loading);
$derived(agentStore.error);

// Cleanup (call on page unmount)
agentStore.destroy();
```

### Frontend — UsageStore

```typescript
import { usageStore } from '$lib/stores/usage';

// Initialize with user ID
await usageStore.init(userId);

// Reactive usage events
$derived(usageStore.usageEvents);

// Cleanup
usageStore.destroy();
```

### Backend — Hub

```go
import "ace/internal/api/realtime"

// Create hub on startup
hub := realtime.NewHub(natsConn, logger, meter)

// Pass to router config
cfg := router.Config{
    Hub: hub,
    // ...
}

// Close on shutdown
defer hub.Close()
```

---

## Testing

### Test Coverage

- **331 tests** across 30 test files
- Backend: SeqBuffer, TopicReg, Hub, Client, Handler, Integration (11 tests)
- Frontend: RealtimeManager, Connection, Reconnect, Polling, Topics, AgentStore, UsageStore, Notifications, ConnectionIndicator, LiveBadge, Integration (10 tests)

### Run Tests

```bash
make test              # Full suite
make test-realtime     # Backend realtime integration tests only
```

---

## Related Documentation

- [design/units/realtime-ui/problem_space.md](../design/units/realtime-ui/problem_space.md)
- [design/units/realtime-ui/research.md](../design/units/realtime-ui/research.md)
- [design/units/realtime-ui/architecture.md](../design/units/realtime-ui/architecture.md)
- [design/units/realtime-ui/implementation_plan.md](../design/units/realtime-ui/implementation_plan.md)
