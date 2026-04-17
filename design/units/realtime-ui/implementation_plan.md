# Implementation Plan: Real-time UI Updates & Retry Mechanisms

[unit: realtime-ui]

## Execution Strategy

Each slice is a vertical, testable increment. A slice delivers one complete end-to-end capability: backend message types → Hub/Client/TopicReg → WebSocket handler → frontend RealtimeManager → Svelte component → test. No horizontal "build all X" phases.

Dependencies between slices are explicit. A slice that depends on a prior slice lists it.

---

## Slice 1: Message Types & Sequence Buffer (Backend Foundation)

- **Backend:**
  - Create `internal/api/realtime/message.go` — `ClientMessage` and `ServerMessage` types with discriminated union `Type` field
  - ClientMessage types: `auth`, `subscribe`, `unsubscribe`, `replay`, `ping`
  - ServerMessage types: `auth_ok`, `auth_error`, `subscribed`, `unsubscribed`, `event`, `resync_required`, `pong`, `error`
  - Each message carries `topic`, `seq`, `data` fields as appropriate
  - Define topic string format: `agent:{id}:status`, `agent:{id}:logs`, `agent:{id}:cycles`, `system:health`, `usage:{id}`
  - Define topic validation regex: `^[a-z0-9]+:[a-z0-9-]+:[a-z0-9_]+$`
  - Create `internal/api/realtime/seq.go` — `SeqBuffer` with per-topic ring buffers, `Append()`, `Replay()` returning `ErrBufferExceeded`
  - Create `internal/api/realtime/seq_test.go` — unit tests for SeqBuffer: append, replay within buffer, replay beyond buffer (ErrBufferExceeded), concurrent access, buffer expiry
- **Frontend:** N/A
- **Test:** `go test ./internal/api/realtime/...` — all message type JSON round-trips pass, SeqBuffer append/replay/expiry/Concurrency tests pass

---

## Slice 2: TopicReg — NATS Subscription Registry (Backend)

Depends on: Slice 1 (message types for topic validation)

- **Backend:**
  - Create `internal/api/realtime/topic.go` — `TopicReg` struct with `refs map[string]int`, `subs map[string]*nats.Subscription`, `topicToSubject` mapping
  - `NewTopicReg(natsConn, hub, logger)` constructor
  - `Add(topic string) error` — validate topic format, increment ref count, create NATS subscription on first reference using `messaging.SubscribeWithEnvelope`, dispatch callback to `hub.dispatchNATSEvent`
  - `Remove(topic string) error` — decrement ref count, unsubscribe from NATS when count reaches zero
  - `natsToTopic(subject string) string` — reverse mapping from NATS subject back to public topic
  - Topic-to-NATS mapping table:
    - `agent:{id}:status` → `ace.engine.{id}.layer.>`
    - `agent:{id}:logs` → `ace.engine.{id}.loop.>`
    - `agent:{id}:cycles` → `ace.engine.{id}.layer.6.output`
    - `system:health` → `ace.system.health.>`
    - `usage:{id}` → `ace.usage.{id}.>`
  - Create `internal/api/realtime/topic_test.go` — unit tests with mock NATS: Add creates sub, Add again increments ref, Remove decrements, Remove to zero unsubscribes, invalid topic rejected, concurrent Add/Remove
- **Frontend:** N/A
- **Test:** `go test ./internal/api/realtime/...` — TopicReg ref counting, NATS sub lifecycle, topic validation, concurrent access tests pass

---

## Slice 3: Client & Hub — Connection Registry & Fan-Out (Backend)

Depends on: Slice 1 (message types), Slice 2 (TopicReg)

- **Backend:**
  - Create `internal/api/realtime/client.go` — `Client` struct with `id`, `userID`, `role`, `conn (*websocket.Conn)`, `topics map[string]struct{}`, `send chan []byte` (buffered 128), `hub *Hub`, `seq uint64`, `done chan struct{}`
  - `NewClient(conn, userID, role, hub)` constructor (UUID for id)
  - `Send(msg ServerMessage)` — marshals to JSON, non-blocking send to `send` channel (drops on full for non-critical, blocks-critical for auth/control)
  - `writePump()` — goroutine reading from `send` channel, writing to WebSocket via `wsjson.Write`, exits on `done` or context cancellation
  - `readPump(ctx)` — goroutine reading from WebSocket via `wsjson.Read`, dispatching to `handleMessage`, calls `hub.Unregister` on exit
  - `handleMessage(ctx, msg)` — switch on message type: `subscribe` → `hub.Subscribe`, `unsubscribe` → `hub.Unsubscribe`, `replay` → `hub.Replay`, `ping` → send `pong`, `auth` → token refresh
  - Create `internal/api/realtime/hub.go` — `Hub` struct with `mu sync.RWMutex`, `clients map[string][]*Client` (userID → connections), `topics *TopicReg`, `nats *nats.Conn`, `buffer *SeqBuffer`, `logger`, `meter`
  - `NewHub(natsConn, logger, meter)` constructor
  - `Run()` — starts core NATS subscription listeners (system topics)
  - `Register(client)` — adds to clients map, sends `auth_ok`
  - `Unregister(client)` — removes from clients map, removes all topic subscriptions via TopicReg, closes `send` channel, logs disconnect with duration
  - `Subscribe(client, topics)` — validates max 50 topics, checks authorization per topic, adds topics to client, calls `TopicReg.Add` for each new topic, sends `subscribed` response
  - `Unsubscribe(client, topics)` — removes topics from client, calls `TopicReg.Remove` for each removed topic, sends `unsubscribed` response
  - `dispatchNATSEvent(topic, data)` — acquires read lock, appends to SeqBuffer, iterates clients subscribed to topic, checks authorization per-client, sends to authorized clients
  - `PollEvents(userID, role, topics, sinceSeq)` — returns buffered events for authorized topics, returns `resync_required` list for topics where buffer was exceeded
  - `Close()` — drains all client connections, unsubscribes all NATS subs, releases resources
  - `isAuthorized(userID, role, topic)` — checks if user can view the topic's resource (admin sees all, regular user sees own agents)
  - Create `internal/api/realtime/hub_test.go` — integration tests using `net/http/httptest` + `coder/websocket` test tools: Register/Unregister lifecycle, Subscribe/Unsubscribe topic flow, fan-out dispatches to authorized clients only, authorization filters unauthorized topics, multiple clients per user, close drains all clients
- **Frontend:** N/A
- **Test:** `go test ./internal/api/realtime/...` — hub register/unregister, subscribe/unsubscribe, fan-out with auth filter, multiple clients, graceful close tests pass

---

## Slice 4: WebSocket Handler & Router Integration (Backend)

Depends on: Slice 3 (Hub, Client)

- **Backend:**
  - Create `internal/api/realtime/handler.go` — `HandleWebSocket(hub, tokenService) http.HandlerFunc`
  - WebSocket upgrade via `websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})`
  - 5-second auth timeout using `context.WithTimeout`
  - Read first message: must be `auth` type with valid JWT token
  - Validate JWT via `tokenService.ValidateToken(token)`
  - On auth success: create `Client`, `hub.Register(client)`, send `auth_ok` with `connection_id`, start `writePump` goroutine, run `readPump` (blocks)
  - On auth failure: close with `websocket.StatusPolicyViolation`, log reason
  - CORS: validate `Origin` header during upgrade (same policy as REST API)
  - `HandlePolling(hub) http.HandlerFunc` — authenticated endpoint (uses existing auth middleware)
  - Parse `topics` and `since_seq` from query params, call `hub.PollEvents(userID, role, topics, sinceSeq)`, return JSON envelope
  - Wire routes into `router.New()`:
    - `r.Get("/api/ws", realtime.HandleWebSocket(hub, tokenService))` — no auth middleware (auth via first message)
    - Inside auth-required group: `r.Get("/api/realtime/updates", realtime.HandlePolling(hub))`
  - Update `router.Config` to include `Hub *realtime.Hub`
  - Update main application startup: create `realtime.NewHub(natsConn, logger, meter)`, pass to router config
  - Update `App.Shutdown()` to call `hub.Close()` for graceful WebSocket drain
  - Add `coder/websocket` to `go.mod`
  - Add OTel metrics for realtime: `ace.realtime.ws.connections.active`, `ace.realtime.ws.messages.sent`, `ace.realtime.ws.messages.received`, `ace.realtime.ws.errors`, `ace.realtime.poll.requests`, `ace.realtime.poll.events.delivered`
- **Frontend:** N/A
- **Test:** Manual integration test: `wscat` connects to `/api/ws`, sends auth, subscribes to topic, receives event. `curl` hits `/api/realtime/updates` with auth header. Hub shuts down cleanly. `make test` passes including existing tests.

---

## Slice 5: Frontend RealtimeManager — Connection & Auth (Frontend Foundation)

Depends on: Slice 4 (WebSocket endpoint available)

- **Backend:** N/A (uses endpoint from Slice 4)
- **Frontend:**
  - Create `frontend/src/lib/realtime/types.ts` — TypeScript discriminated unions mirroring backend message types:
    - `ClientMessage` union: `AuthMessage`, `SubscribeMessage`, `UnsubscribeMessage`, `ReplayMessage`, `PingMessage`
    - `ServerMessage` union: `AuthOkMessage`, `AuthErrorMessage`, `SubscribedMessage`, `UnsubscribedMessage`, `EventMessage`, `ResyncRequiredMessage`, `PongMessage`, `ErrorMessage`
    - `ConnectionStatus` type: `'connecting' | 'connected' | 'polling' | 'disconnected'`
    - `TopicEvent` type with `type`, `topic`, `seq`, `data` fields
    - `PollingResponse` type with `events`, `current_seq`, `has_more`, `resync_required` fields
  - Create `frontend/src/lib/realtime/connection.ts` — `WebSocketConnection` class:
    - `connect(url: string): Promise<void>` — opens WebSocket, resolves on `auth_ok`, rejects on `auth_error` or timeout
    - `send(message: ClientMessage): void` — sends JSON over WebSocket (queues if not connected)
    - `onMessage(callback: (msg: ServerMessage) => void): () => void` — returns unsubscribe function
    - `close(): void` — clean close with `websocket.StatusNormalClosure`
    - `status: ConnectionStatus` — reactive `$state`
    - Heartbeat: send `ping` every 30s, close if no `pong` within 10s
  - Create `frontend/src/lib/realtime/manager.svelte.ts` — `RealtimeManager` class:
    - Reactive state: `status = $state<ConnectionStatus>('disconnected')`, `lastSeq = $state<Record<string, number>>({})`, `reconnectAttempts = $state(0)`
    - `connect(token: string): void` — create WebSocketConnection, send auth message, transition to `connected` on success
    - `disconnect(): void` — clean close, clear subscriptions state (keep subscription set for reconnect)
    - `subscribe(topics: string[]): void` — add to subscription set, send `subscribe` message if connected
    - `unsubscribe(topics: string[]): void` — remove from subscription set, send `unsubscribe` message if connected
    - `on(eventType: string, handler: (data: unknown) => void): () => void` — register handler, return unsubscribe function
    - Private: `handlers = new Map<string, Set<(data: unknown) => void>>()`
    - Private: `dispatchEvent(message: ServerMessage): void` — route by type to registered handlers
    - Private: `sendQueue: ClientMessage[]` — queue outbound messages until connected
  - Export singleton `realtimeManager` from `manager.svelte.ts`
  - Wire `AuthStore` → `RealtimeManager`: add `realtimeManager.connect(token)` call on login, `realtimeManager.disconnect()` on logout
- **Test:** Unit tests for RealtimeManager: connect/disconnect state transitions, subscribe/unsubscribe message queuing, event handler registration and dispatch, auth success/failure flows. Mock WebSocket in tests.

---

## Slice 6: Reconnection & Polling Fallback (Frontend Resilience)

Depends on: Slice 5 (RealtimeManager with connection)

- **Backend:** N/A (polling endpoint from Slice 4)
- **Frontend:**
  - Create `frontend/src/lib/realtime/reconnect.ts` — `ReconnectManager` class:
    - Exponential backoff: `RECONNECT_BASE_MS * 2^(attempt-1)`, capped at `RECONNECT_MAX_MS` (30s)
    - `shouldRetry(attempt: number): boolean` — returns false after `RECONNECT_MAX_ATTEMPTS` (5)
    - `getDelay(attempt: number): number` — calculates backoff delay
    - `reset(): void` — resets attempt counter on successful connect
  - Create `frontend/src/lib/realtime/polling.ts` — `PollingClient` class:
    - `start(topics: string[], sinceSeq: Record<string, number>, onEvents: (events: TopicEvent[]) => void, onResync: (topics: string[]) => void): void`
    - `stop(): void`
    - Adaptive interval: `POLL_INTERVAL_ACTIVE_MS` (1s) when user active, `POLL_INTERVAL_IDLE_MS` (10s) when idle
    - Activity detection: listen for `click`, `scroll`, `keydown`, `focus` events on `document`; if activity within last 30s, consider user active
    - Uses `apiClient.request<PollingResponse>` to call `GET /api/realtime/updates?topics=...&since_seq=...`
    - Updates `lastSeq` from response `current_seq`
    - On `resync_required` in response, calls `onResync` callback
  - Update `frontend/src/lib/api/client.ts` — add polling request method: `pollEvents(topics: string[], sinceSeq: number): Promise<PollingResponse>`
  - Update `frontend/src/lib/realtime/manager.svelte.ts`:
    - Integrate `ReconnectManager`: on WebSocket close/error, attempt reconnect with backoff. After `RECONNECT_MAX_ATTEMPTS` failures, fall back to `PollingClient`
    - Integrate `PollingClient`: when `status = 'polling'`, start polling. Periodically attempt WebSocket reconnect (every 30s). On WebSocket success, stop polling and switch to `status = 'connected'`
    - Reconnection flow: on reconnect, send `replay` message with `lastSeq` for each topic. Handle `resync_required` by fetching full state via REST API for each topic
    - `resyncTopic(topic: string): Promise<void>` — fetch full current state via REST for the topic's resource
  - Create `frontend/src/lib/realtime/topic.ts` — topic utility functions:
    - `parseTopic(topic: string): { resourceType: string; resourceId: string; subType: string }` — parse `agent:123:status` → `{ resourceType: 'agent', resourceId: '123', subType: 'status' }`
    - `buildTopic(resourceType: string, resourceId: string, subType: string): string` — compose topic string
    - `getResyncEndpoint(topic: string): string` — map topic to REST endpoint (e.g., `agent:123:status` → `/agents/123`)
- **Test:**
  - `ReconnectManager` tests: backoff calculation, max attempts, reset on success
  - `PollingClient` tests: start/stop lifecycle, activity detection, adaptive interval, event dispatch, resync callback
  - `topic.ts` tests: parse and build topic strings, validate format, map to REST endpoints

---

## Slice 7: Connection Indicator & Live Badge (Frontend UI)

Depends on: Slice 5 (RealtimeManager reactive state)

- **Backend:** N/A
- **Frontend:**
  - Create `frontend/src/lib/components/realtime/ConnectionIndicator.svelte`:
    - Reads `realtimeManager.status` via `$derived`
    - Status config: `connected` → green dot + "Connected", `connecting` → yellow dot + "Connecting...", `polling` → yellow dot + "Polling", `disconnected` → red dot + "Disconnected"
    - Shows `LiveBadge` when connected
    - Positioned in `Header.svelte` alongside user menu
    - Click handler: shows tooltip with reconnect attempts count on non-connected states
  - Create `frontend/src/lib/components/realtime/LiveBadge.svelte`:
    - Pulsing green dot animation (CSS `@keyframes pulse`)
    - Shown only when `realtimeManager.status === 'connected'`
    - Small, unobtrusive visual indicator
  - Update `frontend/src/lib/components/layout/Header.svelte` — add `ConnectionIndicator` component next to `UserMenu`
  - Create `frontend/src/lib/components/realtime/index.ts` — barrel export for `ConnectionIndicator` and `LiveBadge`
- **Test:**
  - `ConnectionIndicator` tests: renders correct color/label for each status, shows LiveBadge only when connected, hidden when disconnected
  - `LiveBadge` tests: renders pulsing dot, has correct animation class
  - Visual: ConnectionIndicator visible in header on all pages when authenticated

---

## Slice 8: Store Integration — Agent Status Real-time Updates

Depends on: Slice 5 (RealtimeManager), Slice 6 (polling fallback)

- **Backend:** N/A (events will flow from NATS through Hub once cognitive engine is wired)
- **Frontend:**
  - Create `frontend/src/lib/stores/agents.svelte.ts` — `AgentStore` class:
    - `agents = $state<Agent[]>([])` — reactive agent list
    - `loading = $state(false)`, `error = $state<string | null>(null)`
    - `init(): Promise<void>` — fetch initial agent list via REST (`GET /agents`), then subscribe to real-time updates
    - Subscribe to topics: `agent:{id}:status` for each agent the user can see (or wildcard approach)
    - Register handlers: `agent.status_change`, `agent.cycle_start`, `agent.cycle_complete`
    - `handleStatusChange(data)` — find agent by ID, update status in-place (last-write-wins)
    - `handleCycleStart(data)` — find agent, update cycle info
    - `handleCycleComplete(data)` — find agent, update cycle completion
    - `destroy(): void` — unsubscribe all handlers, unsubscribe from topics
    - Export singleton `agentStore`
  - Update `frontend/src/lib/api/client.ts` — add agent API methods if not present: `listAgents()`, `getAgent(id)`
  - Update dashboard page (`(app)/+page.svelte`) — initialize `agentStore` on mount, show agent status changes in real-time
  - Wire token refresh: when `authStore` refreshes the access token, call `realtimeManager.connect(newToken)` to send new auth message on existing connection (no reconnect needed)
- **Test:**
  - `AgentStore` tests: init fetches agents via REST, subscribe registers handlers, status change updates agent in list, cycle start/completion events update agent, destroy cleans up
  - Manual: open two browser tabs, observe agent status changes propagate in real-time between tabs

---

## Slice 9: Backend Observability & Rate Limiting

Depends on: Slice 3 (Hub), Slice 4 (Handler)

- **Backend:**
  - Add OTel spans to `realtime/handler.go`:
    - `realtime.ws.upgrade` — WebSocket upgrade with `user_id`, `connection_id` attributes
    - `realtime.ws.auth` — Auth handshake with `success` attribute
    - `realtime.ws.subscribe` — Topic subscription with `user_id`, `topics` attributes
    - `realtime.ws.disconnect` — Connection close with `user_id`, `connection_id`, `duration_ms` attributes
    - `realtime.poll` — Polling request with `user_id`, `topics`, `since_seq` attributes
  - Add OTel metrics (counter gauges already defined in Slice 4, now instrument them):
    - Increment `ace.realtime.ws.connections.active` on Register, decrement on Unregister
    - Increment `ace.realtime.ws.messages.sent` on each client send
    - Increment `ace.realtime.ws.messages.received` on each client message received
    - Increment `ace.realtime.ws.errors` on write errors, auth failures, unexpected disconnects
    - Increment `ace.realtime.poll.requests` on each polling request
    - Increment `ace.realtime.poll.events.delivered` on each event in polling response
    - Increment `ace.realtime.buffer.replay.events` on each replay event sent
    - Increment `ace.realtime.buffer.resync.required` on each `resync_required` response
  - Add structured logging with correlation: `connection_id`, `user_id`, `topic` in all log entries
  - Add usage events for connection time:
    - On disconnect, emit `telemetry.UsageEvent` with `OperationTypeNATSPublish`, `ResourceTypeMessaging`, duration_ms, `user_id`, `connection_id`, `transport` ("websocket" or "polling"), `topics_count`
  - Add rate limiting to WebSocket handler:
    - Max 100 messages/second per connection (configurable via `WS_RATE_LIMIT`)
    - Max 50 topics per subscription request (enforced in `Hub.Subscribe`)
    - Max 64KB per message (enforced in `websocket.Accept` read limit)
  - Add rate limiting to polling endpoint using existing `rate_limit_middleware.go` at 60/min per user (configurable via `POLL_RATE_LIMIT`)
  - Add WebSocket configuration constants: `WS_AUTH_TIMEOUT`, `WS_MAX_SUBSCRIPTIONS`, `WS_MAX_MESSAGE_SIZE`, `WS_HEARTBEAT_INTERVAL`, `WS_SEND_CHANNEL_SIZE`, `BUFFER_MAX_SIZE`, `BUFFER_MAX_AGE`, `POLL_MAX_TOPICS`
  - Wire health check: add `realtime` subsystem to `/health/ready` — check Hub is running and NATS connection is alive
- **Frontend:** N/A
- **Test:** `go test ./internal/api/realtime/...` — metrics increment on connect/disconnect, rate limit enforcement rejects oversize messages and over-limit subscriptions, health check includes realtime status

---

## Slice 10: System Health & Usage Real-time Topics (End-to-End Demo)

Depends on: Slice 4 (backend routes), Slice 5 (frontend manager), Slice 6 (polling), Slice 8 (store pattern)

- **Backend:**
  - Wire `system:health` topic to existing NATS `ace.system.health.>` subject in TopicReg
  - Wire `usage:{id}` topic to existing NATS `ace.usage.{id}.>` subject in TopicReg
  - Create a simple integration test that publishes to NATS and verifies event arrives on WebSocket client
  - Ensure `dispatchNATSEvent` correctly translates NATS subject to public topic and sequences the event
- **Frontend:**
  - Update `frontend/src/lib/components/telemetry/HealthCards.svelte` (existing) — subscribe to `system:health` topic via `realtimeManager`, update health cards in real-time when events arrive instead of polling
  - Create `frontend/src/lib/stores/usage.svelte.ts` — `UsageStore` class (similar pattern to AgentStore):
    - `usageEvents = $state<UsageEvent[]>([])` — reactive usage event list
    - Subscribe to `usage:{userId}` topic
    - Register handler for `usage.cost` events
    - Update usage table in real-time
  - Update `frontend/src/lib/stores/notifications.svelte.ts` (existing) — add real-time notifications:
    - When `realtimeManager.status` transitions to `disconnected`, add warning toast "Connection lost. Reconnecting..."
    - When `realtimeManager.status` transitions to `connected` from `polling` or `disconnected`, add success toast "Connected"
    - When `realtimeManager.status` transitions to `polling`, add info toast "Using polling mode"
- **Test:**
  - Backend integration test: publish to NATS `ace.system.health.ok`, verify connected WebSocket client receives event with correct topic and seq
  - Frontend: HealthCards update without manual refresh when NATS events arrive
  - Frontend: Notification toasts appear on connection status changes

---

## Slice 11: Integration & Stress Testing

Depends on: All prior slices

- **Backend:**
  - Create `internal/api/realtime/integration_test.go`:
    - Test: WebSocket connect → auth → subscribe → receive event → unsubscribe → disconnect (full lifecycle)
    - Test: Multiple clients subscribe to same topic → one NATS event fans out to all authorized clients
    - Test: Client subscribes to unauthorized topic → receives auth error
    - Test: WebSocket close → reconnect → replay → receive live events (reconnection flow)
    - Test: Buffer exceeded → client receives `resync_required` → falls back to REST resync
    - Test: Polling endpoint returns events since `since_seq`, handles `resync_required`
    - Test: Token refresh via auth message → connection continues without reconnect
    - Test: Hub graceful shutdown closes all client connections cleanly
    - Test: Concurrent fan-out (100 events/second) — verify no drops for critical messages
  - Add `make test-realtime` target for running just the realtime tests
- **Frontend:**
  - Create `frontend/src/lib/realtime/integration.test.ts`:
    - Test: Full lifecycle — connect → subscribe → receive events → disconnect → reconnect → replay
    - Test: Polling fallback — WebSocket fails → starts polling → WebSocket recovers → stops polling → resumes WebSocket
    - Test: Adaptive polling interval — active user polls every 1s, idle user polls every 10s
    - Test: Token refresh flow — new token sent as auth message without reconnect
    - Test: Resync flow — `resync_required` received → REST API calls → state updated
  - Manual end-to-end test:
    - Open two browser tabs, subscribe to same agent in both
    - Trigger agent status change via API or NATS publish
    - Verify both tabs update within 1 second
    - Disconnect one tab's network (Chrome DevTools offline)
    - Verify other tab continues receiving events
    - Reconnect first tab
    - Verify first tab catches up with replayed events
    - Block WebSocket (DevTools → block WS URL)
    - Verify tab falls back to polling within 3 reconnect attempts
    - Unblock WebSocket
    - Verify tab transitions back to WebSocket connection
- **Test:** All integration tests pass. Manual end-to-end test passes with sub-second latency, seamless reconnection, and graceful polling fallback.

---

## Dependency Graph

```
Slice 1 (Message Types + SeqBuffer)
  └─→ Slice 2 (TopicReg)
       └─→ Slice 3 (Client + Hub)
            └─→ Slice 4 (WS Handler + Router)
            │    └─→ Slice 9  (Observability + Rate Limiting)
            └─→ Slice 5 (Frontend RealtimeManager)
                 ├─→ Slice 6 (Reconnection + Polling)
                 │    └─→ Slice 8 (Agent Store Integration)
                 │         └─→ Slice 10 (System Health + Usage Topics)
                 │              └─→ Slice 11 (Integration + Stress Testing)
                 └─→ Slice 7 (Connection Indicator UI)
                      └─→ Slice 8 (Agent Store Integration)
```

Slices 6 and 7 are independent of each other (both depend on Slice 5). Slice 9 is independent of frontend slices (depends on Slice 4 backend). Slices 7 and 9 can be developed in parallel.

---

## Notable Cross-Cutting Concerns (applied in every slice)

- **No `any`:** All TypeScript types are explicit discriminated unions. All Go types are explicit structs. No `interface{}` or `any`.
- **No `else`:** Early returns only. Guard clauses for error cases.
- **Auth first:** Every WebSocket message after auth is processed. Unauthenticated connections closed within 5 seconds.
- **Last-write-wins:** Incoming events update state in-place. No merge logic.
- **Observability:** Every backend operation emits OTel spans, metrics, and structured logs with `connection_id`, `user_id`, `topic` correlation.
- **Error handling:** Non-blocking sends on full channel drop non-critical events. Critical events (auth, control) never dropped.
- **Security:** Topic validation regex enforced. Max 50 subscriptions per connection. Max 64KB per message. Rate limiting on polling endpoint.