# Research: Real-time UI Updates & Retry Mechanisms

[unit: realtime-ui]

## Overview

This document evaluates technical approaches for delivering real-time UI updates in ACE's single-binary architecture. The core decision space: WebSocket library, transport protocol strategy, NATS-to-client bridging pattern, frontend connection management, reconnection/retry strategy, and polling fallback design.

---

## 1. Go WebSocket Library

### Option A: coder/websocket (née nhooyr.io/websocket)

**Pros:**
- First-class context support — `websocket.Accept` takes `context.Context`, cancellation propagates cleanly
- Zero external dependencies
- Modern Go idioms: `wsjson.Read`/`wsjson.Write` handle JSON with buffer reuse
- Built-in compression (permessage-deflate) with `CompressionMode` config
- `websocket.CloseStatus(err)` extracts close codes — clean error classification
- Actively maintained by Coder (production-grade usage)
- Benchmark score 93.45 (Context7), highest among Go WebSocket libs
- Safer defaults: timeouts, proper close handshake

**Cons:**
- Smaller community than gorilla/websocket
- API slightly different from gorilla's familiar pattern

### Option B: gorilla/websocket

**Pros:**
- Industry standard, largest community
- Extensive documentation and examples
- Battle-tested in production across thousands of projects
- Familiar `io.Reader`/`io.Writer` interfaces

**Cons:**
- No context support — must manage cancellation manually
- Concurrent writes require external mutex (common footgun)
- Archive status (no longer actively maintained as of 2022; community fork exists)
- More boilerplate for close handling

### Option C: gobwas/ws

**Pros:**
- Zero allocations on hot path — best raw throughput
- Event-driven API for maximum control
- Handles 3M+ connections in benchmarks

**Cons:**
- Low-level API — must handle frame parsing manually
- Not compatible with `net/http` (uses its own upgrade path)
- Steeper learning curve
- Overkill for 10s-100s of connections

### Recommendation

**coder/websocket.** Context support is non-negotiable for a production system that needs clean shutdown. Zero dependencies keeps the single binary lean. At our scale (10s-100s connections), throughput differences between libraries are irrelevant — safety and API ergonomics win. The `wsjson` helpers eliminate JSON boilerplate. Compression support is a bonus for bandwidth-constrained clients.

---

## 2. Transport Protocol Strategy

### Option A: WebSocket Only

All real-time data flows over a persistent WebSocket connection.

**Pros:**
- Single transport to implement and debug
- Bidirectional — client can send subscription commands
- Lowest latency once connected

**Cons:**
- No fallback if WebSocket blocked (corporate proxies, some mobile networks)
- Must implement own reconnection, heartbeat, and history replay
- Initial HTTP upgrade adds latency on reconnect

### Option B: SSE Only

Server-Sent Events over HTTP. Client→Server via regular POST requests.

**Pros:**
- Built-in `EventSource` API with auto-reconnect
- `Last-Event-ID` header for history replay
- Works through most proxies and firewalls
- Simpler server implementation (standard HTTP handler)

**Cons:**
- Unidirectional only — client→server requires separate HTTP calls
- No binary support
- 6 connections per origin limit in older browsers (HTTP/1.1)
- No compression
- Problem space requires bidirectional communication (subscription commands, agent interactions)

### Option C: WebSocket Primary + SSE Fallback

WebSocket when available, SSE when WebSocket fails.

**Pros:**
- Best of both worlds in theory

**Cons:**
- Two complete transport implementations to maintain
- SSE fallback still doesn't solve bidirectional need
- Increased complexity for marginal gain over polling

### Option D: WebSocket Primary + Polling Fallback (Hybrid)

WebSocket for live updates when available. HTTP polling on a configurable interval as automatic fallback when WebSocket is unavailable or degraded.

**Pros:**
- Matches problem_space.md constraint: "WebSockets + Polling as equals — both are first-class citizens"
- Polling works everywhere HTTP works
- Simpler than SSE fallback — no second streaming protocol
- Polling doubles as health check (connection state observable)
- Single subscription command protocol: client tells server what it wants regardless of transport

**Cons:**
- Polling has higher latency than WebSocket (acceptable per problem_space: sub-second under normal conditions)
- More HTTP load during polling periods (mitigated by adaptive intervals)

### Recommendation

**Option D: WebSocket Primary + Polling Fallback.** This directly satisfies the problem_space constraint. The key insight: the system has one subscription model (client subscribes to topics), and two transports that deliver updates for those topics. The client automatically selects the best available transport. Polling isn't a degraded mode — it's a co-equal citizen with different latency characteristics.

---

## 3. NATS-to-Client Bridging Pattern

The backend already uses NATS internally. How do backend events reach the client?

### Option A: NATS WebSocket Gateway (nats-websocket-gw or NATS leafnode WS)

Expose NATS directly to the browser via a WebSocket gateway. Clients speak the NATS protocol over WebSocket.

**Pros:**
- Zero bridging code — NATS handles routing
- Leverages existing NATS subject namespace

**Cons:**
- Exposes internal NATS subject namespace to clients (security concern)
- No server-side filtering or authorization — any connected client subscribes to any subject
- NATS protocol in browser requires a JS NATS client (~50KB)
- Cannot transform/enrich messages before delivery
- Single binary must expose NATS WS port separately from API
- No fine-grained auth: can't filter subscriptions per user role
- orus-io/nats-websocket-gw is unmaintained (last update 2023)

### Option B: Custom Go Bridge (NATS Subscribe → WebSocket/Polling Publish)

Backend service subscribes to NATS subjects, applies authorization and filtering, then pushes authorized events to connected clients over WebSocket or HTTP polling responses.

**Pros:**
- Full control over authorization — filter events by user role, agent ownership
- Transform/enrich messages before delivery (add display names, resolve references)
- Reuse existing auth middleware (JWT validation on WebSocket upgrade)
- No new external dependency
- Consistent with existing handler/service/repository pattern
- NATS subject namespace stays internal — client sees curated topics only
- Single port — WebSocket upgrade and REST API on same Chi router

**Cons:**
- Bridge code to write and maintain
- Backend must track per-client subscriptions and fan-out

**Cons:**
- Must manage connection registry (map of user→connections)
- Broadcast fan-out is application logic

### Recommendation

**Option B: Custom Go Bridge.** ACE is a multi-user system where users have different visibility (admin sees all agents, user sees their own). Exposing raw NATS to clients bypasses this entirely. The bridge pattern lets us reuse the existing auth middleware on WebSocket upgrade, filter events per-user, and keep the internal messaging namespace private. The fan-out logic is straightforward: a `Hub` type maintains a map of user ID → set of connections, and a NATS subscriber dispatches to relevant connections.

---

## 4. Client-Side Subscription Model

### Option A: Global Connection, Topic Subscriptions

Single WebSocket connection. Client sends subscribe/unsubscribe messages for specific topics.

```
Client → Server: { "type": "subscribe", "topics": ["agent:123:status", "agent:123:logs"] }
Client → Server: { "type": "unsubscribe", "topics": ["agent:123:logs"] }
Server → Client: { "topic": "agent:123:status", "data": {...}, "seq": 42 }
```

**Pros:**
- Single connection — efficient for many subscriptions
- Server controls what each client receives
- Dynamic: subscribe/unsubscribe without reconnecting
- Natural mapping to NATS: each topic subscription maps to a NATS subject subscription

**Cons:**
- Server must track per-client topic subscriptions
- More complex protocol

### Option B: Per-Resource Connection

Each resource (agent, dashboard, etc.) gets its own WebSocket connection.

**Pros:**
- Simple — no subscription protocol needed

**Cons:**
- N connections per page — resource-heavy
- Browser connection limits
- Doesn't match problem_space: "many agents per user, many users per resource"

### Option C: Broadcast Everything, Client Filters

Server pushes all events the user is authorized for; client filters in JavaScript.

**Pros:**
- Simple server — no per-client tracking

**Cons:**
- Bandwidth waste — client receives events it doesn't display
- Doesn't scale with many agents/users

### Recommendation

**Option A: Global Connection, Topic Subscriptions.** One persistent WebSocket per client with a topic subscription protocol. This is the standard pattern for real-time systems (Socket.IO rooms, Phoenix channels, Discord gateway). It maps naturally to NATS: when a client subscribes to topic `agent:123:status`, the bridge subscribes to NATS subject `ace.engine.123.layer.*.output` (or equivalent) if not already subscribed. The topic namespace is public-facing and curated (not raw NATS subjects).

---

## 5. Reconnection & History Replay

### Option A: Sequence IDs + Server-Side Buffer

Every event carries a monotonically increasing sequence ID. Server maintains a bounded buffer of recent events per topic. On reconnect, client sends last received seq ID; server replays missed events.

**Pros:**
- Gap-free replay — no missed events during brief disconnections
- Simple protocol: `{"type": "replay", "last_seq": 42}` → server sends seq 43..latest
- Sequence IDs are cheap: atomic counter per topic

**Cons:**
- Server must buffer events (bounded, e.g., last 1000 per topic or 5 minutes)
- Memory cost scales with number of active topics
- Very long disconnections may exceed buffer — graceful fallback needed

### Option B: Timestamp-Based Replay

Client sends last event timestamp on reconnect. Server queries DB for events after that time.

**Pros:**
- No in-memory buffer — always complete replay

**Cons:**
- DB query on every reconnect — adds latency
- Requires all events persisted to DB (not all events are — e.g., progress ticks)
- Timestamp skew between client and server causes gaps or duplicates

### Option C: Full State Resync on Reconnect

On reconnect, client fetches full current state via REST API, then receives live updates.

**Pros:**
- Simplest — no replay protocol needed
- Always consistent — full state snapshot

**Cons:**
- Expensive for large states (full agent list + all statuses)
- Doesn't preserve intermediate events user may have missed
- REST fetch + WebSocket resume race condition

### Recommendation

**Option A with C as fallback.** Sequence IDs for short disconnections (under buffer limit). If the client's `last_seq` is older than the buffer, fall back to full state resync via REST. This gives the best experience for brief network hiccups (the common case) while handling extended outages gracefully. The buffer is bounded (configurable, default 1000 events or 5 minutes per topic) so memory is predictable.

---

## 6. Polling Fallback Design

### Option A: Fixed-Interval Polling

Client polls a REST endpoint at a fixed interval (e.g., every 2 seconds).

**Pros:**
- Simplest implementation
- Predictable load

**Cons:**
- Wastes bandwidth when nothing changed
- Latency equals polling interval

### Option B: Adaptive-Interval Polling

Client adjusts polling interval based on activity: fast (1s) when active, slow (10s) when idle.

**Pros:**
- Efficient — fewer requests when idle
- Responsive when active

**Cons:**
- More complex client logic
- Heuristics for "active" vs "idle" may misjudge

### Option C: Long Polling

Server holds the HTTP request open until new data is available or timeout (e.g., 30s).

**Pros:**
- Near-real-time latency
- Works through proxies

**Cons:**
- Ties up server resources (goroutine per held request)
- More complex server implementation than regular polling
- WebSocket upgrade already handles the low-latency case

### Recommendation

**Option B: Adaptive-Interval Polling.** Fixed-interval is wasteful; long polling is redundant when WebSocket exists. Adaptive polling gives responsiveness when the user is actively watching (1s interval) and backs off when idle (10s interval). The "active" signal is simple: any user interaction (click, scroll, focus) resets the fast interval. The polling endpoint is a standard REST endpoint (`GET /api/realtime/updates?topics=...&since_seq=...`) that returns events since the given sequence ID — reusing the same seq-based protocol as WebSocket replay.

---

## 7. Authentication on WebSocket

### Option A: JWT in Query Parameter

`ws://host/api/ws?token=<jwt>`

**Pros:**
- Simple — works with standard WebSocket API
- No custom headers needed

**Cons:**
- Token appears in server logs and browser history
- Cannot rotate token without reconnecting

### Option B: JWT in First Message

Client connects, then sends `{"type": "auth", "token": "<jwt>"}` as first message. Server closes connection if auth fails within timeout.

**Pros:**
- Token not in URL — not logged or cached
- Can refresh token by sending new auth message without reconnecting

**Cons:**
- Brief window where unauthenticated connection exists
- Must implement auth timeout

### Option C: JWT in Sec-WebSocket-Protocol Header

Encode token into the `Sec-WebSocket-Protocol` header during upgrade.

**Pros:**
- Token not in URL

**Cons:**
- Abuses the protocol header
- Token visible in upgrade request headers
- Awkward and non-standard

### Recommendation

**Option B: JWT in First Message.** Token stays out of URLs and server logs. The unauthenticated window is mitigated by a short timeout (5 seconds — close connection if no valid auth message received). Token refresh is a clean auth message — no reconnection needed. This pattern is used by Slack, Discord, and most production WebSocket systems.

---

## 8. Frontend Connection Manager Pattern

### Option A: Custom Runes-Based Manager

A Svelte 5 class using `$state` for connection state, `$effect` for lifecycle, no external dependencies.

```typescript
class RealtimeManager {
  status = $state<'connecting' | 'connected' | 'polling' | 'disconnected'>('disconnected');
  seq = $state<number>(0);
  private ws: WebSocket | null = null;
  private pollInterval: number | null = null;

  connect(token: string): void;
  subscribe(topics: string[]): void;
  unsubscribe(topics: string[]): void;
  disconnect(): void;
}
```

**Pros:**
- Zero dependencies
- Full control over reconnection, backoff, fallback logic
- Native Svelte 5 runes — reactive state updates flow to UI automatically
- Testable as a plain TypeScript class (like existing stores)
- Consistent with frontend-design architecture: rune class stores

**Cons:**
- Must write and maintain reconnection logic
- No built-in history replay

### Option B: svelte-realtime Library

Full-featured real-time Svelte library with delta sync, replay, optimistic updates, rooms.

**Pros:**
- Feature-complete out of the box
- Handles reconnection, replay, schema evolution

**Cons:**
- Requires `adapter-node` — incompatible with our `adapter-static` SPA
- Opinionated about server architecture (expects Node.js backend)
- Large dependency for our use case
- Doesn't align with our single-binary Go backend

### Option C: sveltekit-websockets Library

SvelteKit-specific WebSocket infrastructure with typed connections.

**Pros:**
- Typed WebSocket infrastructure for SvelteKit
- Svelte component for reactive streaming

**Cons:**
- Requires `adapter-node` — incompatible with our `adapter-static`
- Designed for Node.js backend, not Go

### Recommendation

**Option A: Custom Runes-Based Manager.** Our SPA architecture with `adapter-static` excludes libraries that require server-side SvelteKit rendering. The custom manager is consistent with the existing pattern (AuthStore, UIStore, NotificationStore are all rune-based classes). It gives full control over the WebSocket→polling fallback transition. The manager is a store that other stores consume: `AgentStore` subscribes to topics via the `RealtimeManager`, receives updates, and updates its reactive state.

---

## 9. Message Envelope Design

### Option A: Per-Event Typed Messages

Each event type has its own message shape. Client dispatches based on `type` field.

```json
{ "type": "agent.status", "topic": "agent:123:status", "seq": 42, "data": { "agent_id": "123", "status": "running" } }
```

**Pros:**
- Type-safe on both ends
- Easy to add new event types
- Natural mapping to TypeScript discriminated unions

**Cons:**
- More types to define and maintain

### Option B: Generic Envelope with Raw Payload

One message shape for everything. Payload is opaque.

```json
{ "topic": "agent:123:status", "seq": 42, "data": { ... } }
```

**Pros:**
- Simpler — one message type

**Cons:**
- No type safety on payload
- Client must infer structure from topic name

### Recommendation

**Option A: Per-Event Typed Messages.** Type safety matches the existing codebase constraint ("no `any`, explicit types throughout"). The `type` field enables discriminated unions in TypeScript. New event types are additive — no breaking changes. The topic field provides routing; the type field provides parsing.

---

## 10. Backend Architecture Pattern

### Option A: Hub + Client + Topic Registry

Three types in a dedicated `realtime` package within the API service:

```
Hub      — Central registry. Manages Client connections. Subscribes to NATS. Fans out events.
Client   — Represents one WebSocket connection. Tracks subscribed topics. Writes events to connection.
TopicReg — Tracks which NATS subjects are actively subscribed. Reference-counts per-topic NATS subs.
```

**Pros:**
- Clear separation of concerns
- NATS subscriptions are shared — multiple clients watching same topic use one NATS sub
- Reference counting prevents NATS subscription leaks
- Fits within existing API service alongside REST handlers

**Cons:**
- Three types to implement
- Reference counting adds complexity

### Option B: Flat Connection Map

Simple map of connection ID → WebSocket connection. Broadcast to all.

**Pros:**
- Simplest possible implementation

**Cons:**
- No per-client filtering — everyone gets everything
- No NATS subscription sharing
- Doesn't support multi-user authorization

### Recommendation

**Option A: Hub + Client + Topic Registry.** The reference-counted NATS subscription sharing is essential for efficiency. With many users watching the same agent, we want one NATS subscription for `ace.engine.123.>`, not one per connected client. The Hub is the bridge between NATS and WebSocket. The TopicReg is the deduplication layer. This scales well: adding a new client to an already-subscribed topic is a map lookup, not a new NATS subscription.

---

## 11. Connection Health & Observability

### Approach

Reuse existing `shared/telemetry` patterns. The real-time system emits:

- **OTel spans** for WebSocket upgrade, message send/receive, reconnection events
- **OTel metrics** for active connections, messages per second, polling frequency
- **Usage events** for WebSocket connection time (cost attribution per user)
- **Structured logs** with agent_id, user_id, connection_id correlation

Connection state is exposed to the frontend via the manager's `$state` properties. The UI displays a connection indicator (green/yellow/red) derived from the manager's `status` reactive state.

No new observability infrastructure — extend what exists.

---

## 12. Decision Summary

| # | Decision | Choice | Key Rationale |
|---|----------|-------|---------------|
| D1 | WebSocket library | coder/websocket | Context support, zero deps, modern API, actively maintained |
| D2 | Transport strategy | WebSocket primary + polling fallback | Matches problem_space constraint; polling works everywhere |
| D3 | NATS bridging | Custom Go bridge (Hub pattern) | Auth filtering, message enrichment, internal namespace privacy |
| D4 | Client subscription | Topic-based on single connection | Efficient, dynamic, maps to NATS, standard pattern |
| D5 | Reconnection replay | Sequence IDs + bounded buffer, REST resync fallback | Gap-free for brief disconnects; graceful for extended outages |
| D6 | Polling fallback | Adaptive interval (1s active → 10s idle) | Efficient when idle, responsive when active |
| D7 | WS authentication | JWT in first message | Token not in URLs; refreshable without reconnect |
| D8 | Frontend manager | Custom runes-based class | Consistent with existing stores; adapter-static compatible |
| D9 | Message format | Typed per-event messages with `type` field | Type safety, discriminated unions, extensible |
| D10 | Backend pattern | Hub + Client + TopicReg | NATS sub sharing, per-client filtering, scales well |
| D11 | Observability | Extend shared/telemetry | Reuse existing spans, metrics, logs, usage events |

---

## References

- coder/websocket: https://github.com/coder/websocket (Context7 ID: /coder/websocket)
- gorilla/websocket: https://github.com/gorilla/websocket (archived, community fork)
- gobwas/ws: https://github.com/gobwas/ws
- Benchmark comparison: https://github.com/lesismal/go-websocket-benchmark
- NATS WebSocket gateway: https://github.com/orus-io/nats-websocket-gw (unmaintained)
- svelte-realtime: https://github.com/lanteanio/svelte-realtime (requires adapter-node)
- sveltekit-websockets: https://github.com/SourceRegistry/sveltekit-websockets (requires adapter-node)
