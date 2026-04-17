# Architecture: Real-time UI Updates & Retry Mechanisms

[unit: realtime-ui]

## 1. Architectural Overview

The real-time system bridges NATS internal events to browser clients through a custom Go bridge inside the API service. It uses WebSocket as the primary transport with HTTP polling as an equal-class fallback. The architecture has two halves: a **Go backend bridge** (Hub + Client + TopicReg) that subscribes to NATS, filters events per-user, and fans out to WebSocket connections; and a **Svelte 5 frontend manager** (RealtimeManager) that maintains the connection, manages subscriptions, and feeds events into existing rune stores.

```
┌────────────────────────────────────────────────────────────────────────────┐
│                              Browser                                       │
│                                                                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                       │
│  │ AgentStore   │  │ StatusStore │  │  (future)    │                       │
│  │ (runes)      │  │ (runes)     │  │  stores      │                       │
│  └──────┬───────┘  └──────┬──────┘  └──────┬───────┘                       │
│         │                 │                │                               │
│  ┌──────▼─────────────────▼────────────────▼────────────────────────────┐   │
│  │                    RealtimeManager (runes)                         │   │
│  │  status: connected | polling | disconnected                        │   │
│  │  subscribe(topics) / unsubscribe(topics)                          │   │
│  │  seq tracking, reconnect, adaptive polling                        │   │
│  └────────────┬──────────────────────────────────────────────────────┘   │
│               │ WebSocket (primary) | HTTP polling (fallback)            │
└───────────────┼──────────────────────────────────────────────────────────┘
                │
                ▼
┌───────────────────────────────────────────────────────────────────────────┐
│                         Go Backend (single binary)                        │
│                                                                           │
│   Chi Router                                                              │
│   ├── /api/ws          → WebSocket upgrade + Hub.Register()              │
│   ├── /api/realtime    → HTTP polling endpoint                            │
│   └── /api/*           → Existing REST handlers                           │
│                                                                           │
│   ┌──────────────────────────────────────────────────────────────┐       │
│   │                         Hub                                    │       │
│   │  • Manages Client connections (map[userID]→[]*Client)        │       │
│   │  • Receives NATS events → TopicReg dispatch                  │       │
│   │  • Reference-counts NATS subscriptions per topic             │       │
│   └──────────┬───────────────────────────┬──────────────────────┘       │
│              │                           │                               │
│   ┌──────────▼──────────┐   ┌────────────▼─────────────────┐            │
│   │    TopicReg         │   │         Client                │            │
│   │  • topic → []sub    │   │  • userID, conn, topics       │            │
│   │  • ref-count NATS   │   │  • send channel, seq counter  │            │
│   │  • subscribes NATS  │   │  • write goroutine            │            │
│   └──────────┬──────────┘   └───────────────────────────────┘            │
│              │                                                           │
│              ▼                                                           │
│   ┌──────────────────────┐                                              │
│   │  NATS (embedded)      │                                              │
│   │  ace.engine.>         │                                              │
│   │  ace.tools.>          │                                              │
│   │  ace.usage.>          │                                              │
│   │  ace.system.>         │                                              │
│   └──────────────────────┘                                              │
└───────────────────────────────────────────────────────────────────────────┘
```

### Key Constraints

1. **Single binary**: WebSocket server lives in the same process as the REST API, sharing Chi router, auth middleware, and NATS connection.
2. **Last-write-wins**: No complex merge logic. If two events update the same resource, the later one wins.
3. **Auth first**: Unauthenticated WebSocket connections are closed within 5 seconds.
4. **topic namespace is public-facing**, not raw NATS subjects. The bridge translates between them.
5. **Existing patterns**: Follows handler/service/repository layering. Hub is a long-lived service, Client is per-connection.

---

## 2. Backend Architecture

### 2.1 Package Structure

The real-time system lives in `internal/api/realtime/`, parallel to the existing `handler/`, `service/`, `middleware/` packages:

```
internal/api/realtime/
├── hub.go           # Central registry: manages clients, fans out events
├── client.go        # Per-connection state: WebSocket conn, topics, send buffer
├── topic.go         # TopicReg: reference-counted NATS subscriptions
├── message.go       # Message types: ClientMessage, ServerMessage, TopicEvent
├── handler.go       # WebSocket upgrade handler + HTTP polling handler
├── seq.go           # Sequence ID generator + bounded event buffer
└── hub_test.go      # Integration tests for Hub fan-out
```

This mirrors the existing `handler/`, `service/`, `model/` pattern. The `realtime` package is the service layer. The `handler.go` bridges HTTP to the service (Hub), just as `auth_handler.go` bridges HTTP to `AuthService`.

### 2.2 Hub

The Hub is the central coordinator. It is created once at startup, injected into the router, and runs for the lifetime of the process.

```go
// Hub manages all real-time client connections and event distribution.
// It subscribes to NATS subjects, applies per-user authorization filters,
// and fans out authorized events to connected clients.
type Hub struct {
    mu       sync.RWMutex
    clients  map[string][]*Client   // userID → connections (one user may have multiple tabs)
    topics   *TopicReg              // reference-counted NATS subscriptions
    nats     *nats.Conn             // shared NATS connection
    buffer   *SeqBuffer             // bounded replay buffer per topic
    logger   *zap.Logger
    meter    metric.Meter
}

// Register adds a new WebSocket client connection to the Hub.
// Called after successful auth handshake.
func (h *Hub) Register(client *Client)

// Unregister removes a client. Cleans up empty topic subscriptions.
func (h *Hub) Unregister(client *Client)

// Subscribe adds topics to a client and registers NATS subs if needed.
func (h *Hub) Subscribe(client *Client, topics []string) error

// Unsubscribe removes topics from a client and decrements NATS sub ref-counts.
func (h *Hub) Unsubscribe(client *Client, topics []string)

// dispatchNATSEvent receives a NATS message, determines which clients
// are authorized to receive it, and sends it to their write goroutines.
func (h *Hub) dispatchNATSEvent(topic string, data []byte)
```

**Key behaviors:**
- `Register` and `Unregister` are called from the WebSocket handler goroutine. Both acquire the write lock.
- `Subscribe` and `Unsubscribe` delegate to `TopicReg` for NATS subscription management.
- `dispatchNATSEvent` is the NATS callback. It acquires a read lock on clients, filters by authorization (does user have access to this agent/resource?), and non-blocking-sends to each client's channel.

### 2.3 Client

Each Client represents a single WebSocket connection (one browser tab).

```go
// Client represents a single WebSocket connection with per-connection state.
type Client struct {
    id       string                    // unique connection ID (UUID)
    userID   string                    // authenticated user ID
    role     string                    // user role for authorization filtering
    conn     *websocket.Conn           // coder/websocket connection
    topics   map[string]struct{}       // subscribed topic set
    send     chan []byte               // buffered outbound channel (128 messages)
    hub      *Hub                      // back-reference for unregistration
    seq      uint64                    // per-client sequence counter
    done     chan struct{}             // signals write goroutine to stop
    logger   *zap.Logger
}
```

**Write goroutine pattern:**

```go
func (c *Client) writePump() {
    defer c.conn.Close(websocket.StatusNormalClosure, "")
    for {
        select {
        case msg, ok := <-c.send:
            if !ok {
                // channel closed — hub unregistered us
                return
            }
            c.seq++
            if err := wsjson.Write(r.Context(), c.conn, msg); err != nil {
                c.logger.Warn("write error, disconnecting", zap.Error(err))
                return
            }
        case <-c.done:
            return
        case <-r.Context().Done():
            return
        }
    }
}
```

**Read goroutine pattern:**

```go
func (c *Client) readPump(ctx context.Context) {
    defer c.hub.Unregister(c)
    for {
        var msg ClientMessage
        if err := wsjson.Read(ctx, c.conn, &msg); err != nil {
            // connection closed or error
            return
        }
        if err := c.handleMessage(ctx, msg); err != nil {
            c.logger.Warn("client message error", zap.Error(err))
            // send error response, don't disconnect
        }
    }
}
```

The Client never directly writes to the WebSocket. All writes go through the `send` channel → write goroutine. This avoids the concurrent-write mutex problem entirely (required by `coder/websocket`).

### 2.4 TopicReg (Topic Registry)

The TopicReg manages the mapping between public-facing client topics and internal NATS subjects, with reference counting to avoid redundant subscriptions.

```go
// TopicReg tracks which NATS subjects are actively subscribed and
// reference-counts per-topic NATS subscriptions so multiple clients
// watching the same topic share one NATS subscription.
type TopicReg struct {
    mu       sync.Mutex
    refs     map[string]int            // topic → reference count
    subs     map[string]*nats.Subscription // topic → NATS subscription
    hub      *Hub                      // back-reference for dispatch callbacks
    nats     *nats.Conn
    logger   *zap.Logger

    // topicToSubject maps public topic strings to NATS subject patterns.
    // e.g., "agent:abc123:status" → "ace.engine.abc123.layer.>"
    topicToSubject map[string]string
}

// Add increments the reference count for a topic.
// If this is the first reference, creates a NATS subscription.
func (t *TopicReg) Add(topic string) error

// Remove decrements the reference count for a topic.
// If count reaches zero, unsubscribes from NATS.
func (t *TopicReg) Remove(topic string) error
```

**Topic → NATS subject mapping:**

| Client Topic Pattern | NATS Subject | Authorization |
|---|---|---|
| `agent:{id}:status` | `ace.engine.{id}.layer.>` | Users can only subscribe to agents they own (or all for admins) |
| `agent:{id}:logs` | `ace.engine.{id}.loop.>` | Same as above |
| `agent:{id}:cycles` | `ace.engine.{id}.layer.6.output` | Same as above |
| `system:health` | `ace.system.health.>` | All authenticated users |
| `usage:{id}` | `ace.usage.{id}.>` | Own usage only, admin sees all |

Authorization is enforced in the Hub's `dispatchNATSEvent`: when a NATS message arrives, the Hub checks each subscribed client's user permissions before sending.

### 2.5 Sequence IDs and Bounded Buffer

Every event sent to a client gets a monotonically increasing sequence ID scoped to that topic. The server maintains a bounded in-memory buffer per topic for reconnection replay.

```go
// SeqBuffer stores recent events per topic for reconnection replay.
// Each entry carries a global sequence number and the event payload.
// Buffer is bounded per topic (default: 1000 events or 5 minutes).
type SeqBuffer struct {
    mu      sync.RWMutex
    perTopic map[string]*ringBuffer  // topic → circular buffer
    maxSize  int
    maxAge   time.Duration
}

// Append adds an event to the topic's buffer.
func (s *SeqBuffer) Append(topic string, seq uint64, data []byte)

// Replay returns all events after the given sequence number for a topic.
// Returns ErrBufferExceeded if the requested seq is no longer in buffer,
// signaling that the client should do a full state resync via REST.
func (s *SeqBuffer) Replay(topic string, afterSeq uint64) ([]BufferedEvent, error)
```

**Reconnection flow:**

1. Client reconnects and sends `{"type": "replay", "topics": {"agent:123:status": 42}}`
2. Hub calls `SeqBuffer.Replay("agent:123:status", 42)`
3. If seq 43+ events are buffered, sends them all, then switches to live stream
4. If seq 42 is no longer in buffer (disconnected too long), returns `ErrBufferExceeded`
5. Client receives `{"type": "resync_required", "topic": "agent:123:status"}` and fetches full state via REST API

### 2.6 WebSocket Handler and Auth Flow

The WebSocket handler lives on the existing Chi router alongside REST endpoints. It follows the same pattern as existing handlers.

```go
// HandleWebSocket upgrades an HTTP connection to WebSocket and manages
// the connection lifecycle: auth handshake, read/write pumps, cleanup.
func HandleWebSocket(hub *Hub, tokenService *service.TokenService) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Step 1: Upgrade HTTP to WebSocket using coder/websocket
        conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
            CompressionMode: websocket.CompressionDisabled,
        })
        if err != nil {
            response.InternalError(w, "WebSocket upgrade failed")
            return
        }

        // Step 2: Set read timeout for auth handshake (5 seconds)
        ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
        defer cancel()

        // Step 3: Read first message — must be auth message
        var authMsg ClientMessage
        if err := wsjson.Read(ctx, conn, &authMsg); err != nil {
            conn.Close(websocket.StatusPolicyViolation, "auth timeout")
            return
        }
        if authMsg.Type != "auth" {
            conn.Close(websocket.StatusPolicyViolation, "first message must be auth")
            return
        }

        // Step 4: Validate JWT token
        claims, err := tokenService.ValidateToken(authMsg.Token)
        if err != nil {
            conn.Close(websocket.StatusPolicyViolation, "invalid token")
            return
        }

        // Step 5: Create Client and register with Hub
        client := NewClient(conn, claims.UserID, claims.Role, hub)

        // Step 6: Send auth acknowledgment
        hub.Register(client)
        client.Send(ServerMessage{
            Type: "auth_ok",
            Data: map[string]any{"connection_id": client.ID()},
        })

        // Step 7: Start read and write pumps
        go client.writePump()
        client.readPump(r.Context()) // blocks until disconnect
    }
}
```

**Auth flow summary:**

```
Client                              Server
   │                                   │
   │──── WebSocket upgrade ────────────>│
   │                                   │
   │──── {"type":"auth","token":"jwt"}>│  (5s timeout starts)
   │                                   │
   │<─── {"type":"auth_ok","data":{}} ──│  (valid token)
   │                                   │
   │──── {"type":"subscribe",           │
   │      "topics":["agent:123:status"]}│
   │                                   │
   │<─── {"type":"subscribed",         │
   │      "topics":["agent:123:status"]}│
   │                                   │
   │<─── {"type":"event",              │
   │      "topic":"agent:123:status",   │
   │      "seq":1,                      │
   │      "data":{...}}                 │
```

### 2.7 HTTP Polling Endpoint

The polling endpoint is a standard REST endpoint on the same Chi router. It reuses the existing auth middleware and returns events since a given sequence number.

```go
// HandlePolling returns events for subscribed topics since the given sequence numbers.
// GET /api/realtime/updates?topics=agent:123:status&since_seq=42
func HandlePolling(hub *Hub) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Auth middleware has already validated the JWT and set userID in context
        userID := mw.GetUserIDFromContext(r.Context())
        role := mw.GetUserRoleFromContext(r.Context())

        topics := parseTopics(r.URL.Query().Get("topics"))
        sinceSeq := parseSinceSeq(r.URL.Query().Get("since_seq"))

        // Fetch buffered events for authorized topics
        events, err := hub.PollEvents(userID, role, topics, sinceSeq)
        if err != nil {
            response.InternalError(w, "polling failed")
            return
        }

        response.OK(w, events)
    }
}
```

**Polling request/response:**

```
GET /api/realtime/updates?topics=agent:123:status,system:health&since_seq=42

Response (200 OK):
{
  "success": true,
  "data": {
    "events": [
      {
        "type": "agent.status_change",
        "topic": "agent:123:status",
        "seq": 43,
        "data": { "agent_id": "123", "status": "running" }
      }
    ],
    "current_seq": 45,
    "has_more": false
  }
}

Response when buffer exceeded (200 OK):
{
  "success": true,
  "data": {
    "events": [],
    "current_seq": 45,
    "resync_required": ["agent:123:status"]
  }
}
```

### 2.9 NATS → Client Bridge Flow

The complete flow from an internal NATS event to a client receiving it:

```
1. Cognitive Engine publishes to NATS:
   messaging.Publish(client, messaging.SubjectEngineLayerOutput, 
       corID, agentID, cycleID, "cognitive-engine", payload)

2. NATS delivers to TopicReg's subscription callback:
   func (t *TopicReg) onNATSMessage(msg *nats.Msg) {
       topic := t.natsToTopic(msg.Subject)
       t.hub.dispatchNATSEvent(topic, msg.Data)
   }

3. Hub dispatches to authorized clients:
   func (h *Hub) dispatchNATSEvent(topic string, data []byte) {
       h.mu.RLock()
       defer h.mu.RUnlock()
       
       seq := h.buffer.Append(topic, data)  // increment per-topic seq
       
       for _, client := range h.clientsByTopic(topic) {
           if h.isAuthorized(client.userID, client.role, topic) {
               msg := ServerMessage{Type: eventType(topic), Topic: topic, Seq: seq, Data: data}
               client.Send(msg)  // non-blocking send to channel
           }
       }
   }

4. Client's write goroutine sends to WebSocket.

5. Frontend RealtimeManager receives and dispatches to registered store callbacks.
```

### 2.10 Router Integration

The WebSocket and polling endpoints integrate into the existing Chi router alongside REST endpoints:

```go
// In router.New(), add these routes:
r.Route("/api", func(r chi.Router) {
    // ... existing auth, admin, telemetry routes ...

    // Real-time routes
    r.Get("/ws", realtime.HandleWebSocket(hub, tokenService))
    
    r.Group(func(r chi.Router) {
        r.Use(authMw.RequireAuth())
        r.Get("/realtime/updates", realtime.HandlePolling(hub))
    })
})
```

The WebSocket endpoint (`/api/ws`) does not use the auth middleware — auth is handled via the first-message JWT handshake. The polling endpoint (`/api/realtime/updates`) uses the existing auth middleware since it's a standard HTTP request.

### 2.11 Hub Lifecycle

The Hub is initialized at application startup and shut down gracefully:

```go
// In main.go or app initialization:
hub := realtime.NewHub(natsConn, logger, meter)
go hub.Run()  // starts NATS subscription listeners

// Wire into router:
routerCfg := &router.Config{
    // ... existing fields ...
    Hub: hub,
}

// Graceful shutdown:
func (a *App) Shutdown(ctx context.Context) error {
    a.hub.Close()  // closes all client connections, unsubscribes from NATS
    // ... existing shutdown logic ...
}
```

`Hub.Run()` starts the NATS subscription listeners on core topics. When a NATS message arrives, it's dispatched to the relevant TopicReg callback. `Hub.Close()` drains all client connections, unsubscribes from NATS, and releases resources.

---

## 3. Frontend Architecture

### 3.1 RealtimeManager

The `RealtimeManager` is a Svelte 5 rune-based class that manages the WebSocket connection, polling fallback, and event dispatch. It follows the same pattern as `AuthStore`, `UIStore`, and `NotificationStore`.

```typescript
// $lib/realtime/manager.svelte.ts

export class RealtimeManager {
  // Reactive state — components can bind to these
  status = $state<'connecting' | 'connected' | 'polling' | 'disconnected'>('disconnected');
  lastSeq = $state<Record<string, number>>({});
  reconnectAttempts = $state(0);

  // Private internals
  private ws: WebSocket | null = null;
  private pollInterval: ReturnType<typeof setInterval> | null = null;
  private subscriptions = new Set<string>();
  private handlers = new Map<string, Set<(data: unknown) => void>>();
  private sendQueue: ClientMessage[] = [];

  // Configuration
  private readonly WS_URL: string;
  private readonly POLL_URL: string;
  private readonly AUTH_TIMEOUT_MS = 5000;
  private readonly RECONNECT_BASE_MS = 1000;
  private readonly RECONNECT_MAX_MS = 30000;
  private readonly POLL_INTERVAL_ACTIVE_MS = 1000;
  private readonly POLL_INTERVAL_IDLE_MS = 10000;
  private readonly HEARTBEAT_INTERVAL_MS = 30000;

  connect(token: string): void;
  disconnect(): void;
  subscribe(topics: string[]): void;
  unsubscribe(topics: string[]): void;
  on(eventType: string, handler: (data: unknown) => void): () => void;
}
```

### 3.2 Connection Lifecycle

```
┌─────────────────────────────────────────────────────────────────┐
│                      Connection State Machine                    │
│                                                                  │
│  disconnected                                                     │
│       │                                                          │
│       ├── connect(token) ──────────► connecting                  │
│       │                                 │                        │
│       │                                 ├── WebSocket open ──► connected
│       │                                 │                        │
│       │        ┌─── auth_ok ─────────────┘                        │
│       │        │                                                 │
│       │        │    ┌─── auth_error / timeout ──► disconnected   │
│       │        │                                                 │
│       │    connected                                             │
│       │        ├── ws close / error ──► reconnecting ──► connecting
│       │        │                                                 │
│       │        │   ┌── WebSocket unreachable?
│       │        │   │                                             │
│       │        │   ▼                                             │
│       │        │  polling                                        │
│       │        │   ├── poll fails ──► reconnecting               │
│       │        │   └── WebSocket available again ──► connecting  │
│       │        │                                                 │
│       │        │   (adaptive: 1s active, 10s idle)              │
│       │        │                                                 │
│       └────────┘                                                 │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**Key behaviors:**

1. **connect(token)**: Opens a WebSocket to `/api/ws`, sends auth message, starts 5s auth timeout.
2. **Auth success**: Transitions to `connected`, sends any queued subscribe messages, starts heartbeat.
3. **Auth failure**: Closes WebSocket, transitions to `disconnected`.
4. **WebSocket close/error**: Attempts reconnect with exponential backoff (1s, 2s, 4s, 8s, 16s, 30s cap). After 3 failed reconnects, falls back to polling mode.
5. **Polling mode**: Uses `GET /api/realtime/updates?topics=...&since_seq=...`. Adapts interval based on user activity. Transitions back to WebSocket when available.
6. **Heartbeat**: Client sends `ping` every 30s. Server responds with `pong`. If no `pong` within 10s, considers connection dead and triggers reconnect.
7. **disconnect()**: Cleanly closes WebSocket, stops polling, clears state. Does not remove subscriptions (they persist for reconnect).

### 3.3 Store Integration Pattern

Stores subscribe to the RealtimeManager for specific event types. The manager dispatches events to registered handlers.

```typescript
// $lib/stores/agents.svelte.ts
import { realtimeManager } from '$lib/realtime/manager.svelte';
import { apiClient } from '$lib/api/client';

export class AgentStore {
  agents = $state<Agent[]>([]);
  loading = $state(false);
  error = $state<string | null>(null);

  private unsubscribers: (() => void)[] = [];

  async init(): Promise<void> {
    // 1. Fetch initial state via REST
    this.agents = await apiClient.request<Agent[]>({ method: 'GET', path: '/agents' });
    
    // 2. Subscribe to real-time updates
    this.unsubscribers.push(
      realtimeManager.on('agent.status_change', (data) => this.handleStatusChange(data)),
      realtimeManager.on('agent.cycle_start', (data) => this.handleCycleStart(data)),
      realtimeManager.on('agent.cycle_complete', (data) => this.handleCycleComplete(data)),
    );
    
    // 3. Subscribe to relevant topics
    const topic = `agent:*:status`;  // or specific agent IDs
    realtimeManager.subscribe([topic]);
  }

  private handleStatusChange(data: unknown): void {
    const event = data as { agent_id: string; status: string };
    const idx = this.agents.findIndex(a => a.id === event.agent_id);
    if (idx >= 0) {
      this.agents[idx] = { ...this.agents[idx], status: event.status };
    }
  }

  destroy(): void {
    this.unsubscribers.forEach(fn => fn());
  }
}

export const agentStore = new AgentStore();
```

The pattern is:
1. **REST for initial state** — always fetch current state from the API on page load.
2. **RealtimeManager for deltas** — subscribe to events that update the already-loaded state.
3. **Last-write-wins** — incoming events update in-place. If two events update the same agent, the later seq wins.

### 3.4 Reconnection and State Resync

When the RealtimeManager reconnects (after WebSocket close or switching from polling to WebSocket):

```typescript
private async reconnect(): Promise<void> {
  this.status = 'connecting';
  this.reconnectAttempts++;
  
  const delay = Math.min(
    this.RECONNECT_BASE_MS * Math.pow(2, this.reconnectAttempts - 1),
    this.RECONNECT_MAX_MS
  );
  
  await new Promise(resolve => setTimeout(resolve, delay));
  
  try {
    await this.connect(this.currentToken);
  } catch {
    if (this.reconnectAttempts >= 3) {
      // Fall back to polling mode
      this.startPolling();
    }
  }
}

private async onConnect(): Promise<void> {
  if (Object.keys(this.lastSeq).length > 0) {
    // Send replay request for missed events
    this.send({ type: 'replay', seqs: this.lastSeq });
  } else {
    // Full state resync via REST for all subscribed topics
    await this.resyncAll();
  }
}

private async handleResyncRequired(event: { topics: string[] }): Promise<void> {
  // Buffer exceeded — fetch full state via REST for each topic
  for (const topic of event.topics) {
    await this.resyncTopic(topic);
  }
}
```

### 3.5 Adaptive Polling

When WebSocket is unavailable, the manager switches to HTTP polling with adaptive intervals:

```typescript
private startPolling(): void {
  this.status = 'polling';
  const poll = async () => {
    if (this.status !== 'polling') return;
    
    const topics = Array.from(this.subscriptions);
    if (topics.length === 0) {
      this.scheduleNextPoll(this.POLL_INTERVAL_IDLE_MS);
      return;
    }
    
    const maxSeq = Math.max(...Object.values(this.lastSeq), 0);
    try {
      const response = await apiClient.request<PollingResponse>({
        method: 'GET',
        path: '/realtime/updates',
        params: { topics: topics.join(','), since_seq: String(maxSeq) }
      });
      
      for (const event of response.events) {
        this.dispatchEvent(event);
      }
      
      if (response.resync_required?.length) {
        for (const topic of response.resync_required) {
          await this.resyncTopic(topic);
        }
      }
      
      this.scheduleNextPoll(this.USER_ACTIVE ? this.POLL_INTERVAL_ACTIVE_MS : this.POLL_INTERVAL_IDLE_MS);
    } catch {
      this.scheduleNextPoll(this.POLL_INTERVAL_IDLE_MS);
    }
  };
  
  poll();
}

private scheduleNextPoll(intervalMs: number): void {
  this.pollInterval = setTimeout(this.poll, intervalMs);
}
```

**Activity detection**: User activity (click, scroll, keypress, focus) resets a timer. If activity occurs within the last 30 seconds, `USER_ACTIVE` is true and polling uses 1s interval. Otherwise, 10s interval.

### 3.6 Frontend File Structure

```
frontend/src/lib/
├── realtime/
│   ├── manager.svelte.ts     # RealtimeManager class (connection, fallback, dispatch)
│   ├── types.ts              # ClientMessage, ServerMessage, EventType discriminated unions
│   ├── connection.ts         # WebSocket wrapper (connect, close, send queue)
│   ├── polling.ts            # HTTP polling client with adaptive interval
│   ├── reconnect.ts          # Exponential backoff reconnect logic
│   └── topic.ts               # Topic string utilities (parse, validate, auth check)
│
├── stores/
│   ├── auth.svelte.ts         # (existing — provides JWT token to RealtimeManager)
│   ├── ui.svelte.ts           # (existing — may show connection status indicator)
│   ├── notifications.svelte.ts # (existing — may show connection lost/restored toasts)
│   └── agents.svelte.ts       # (new — subscribes to agent.* events)
│
├── components/
│   ├── realtime/
│   │   ├── ConnectionIndicator.svelte  # Green/yellow/red dot for connection state
│   │   └── LiveBadge.svelte           # Pulsing dot for "live" data
│   └── (existing components unchanged)
│
└── api/
    ├── client.ts              # (existing — add polling endpoint method)
    └── (existing unchanged)
```

### 3.7 Connection Indicator Component

```svelte
<!-- $lib/components/realtime/ConnectionIndicator.svelte -->
<script lang="ts">
  import { realtimeManager } from '$lib/realtime/manager.svelte';
  import { LiveBadge } from './LiveBadge.svelte';

  const statusConfig = {
    connected: { color: 'bg-green-500', label: 'Connected' },
    connecting: { color: 'bg-yellow-500', label: 'Connecting...' },
    polling: { color: 'bg-yellow-500', label: 'Polling' },
    disconnected: { color: 'bg-red-500', label: 'Disconnected' },
  } as const;

  let config = $derived(statusConfig[realtimeManager.status]);
</script>

<div class="flex items-center gap-1.5" title={config.label}>
  <div class="h-2 w-2 rounded-full {config.color}"></div>
  <span class="text-xs text-muted-foreground">{config.label}</span>
  {#if realtimeManager.status === 'connected'}
    <LiveBadge />
  {/if}
</div>
```

This component sits in the `Header.svelte` alongside the user menu, providing constant visibility into connection state.

---

## 4. Observability Integration

The real-time system extends the existing `shared/telemetry` package. No new observability infrastructure is introduced.

### 4.1 Backend Metrics (OTel)

```go
// Metrics emitted by Hub and Client:
var (
    wsConnectionsActive = tel.Meter.NewInt64Gauge("ace.realtime.ws.connections.active")
    wsMessagesSent       = tel.Meter.NewInt64Counter("ace.realtime.ws.messages.sent")
    wsMessagesReceived   = tel.Meter.NewInt64Counter("ace.realtime.ws.messages.received")
    wsErrors             = tel.Meter.NewInt64Counter("ace.realtime.ws.errors")
    pollRequests         = tel.Meter.NewInt64Counter("ace.realtime.poll.requests")
    pollEventsDelivered  = tel.Meter.NewInt64Counter("ace.realtime.poll.events.delivered")
    bufferReplayEvents   = tel.Meter.NewInt64Counter("ace.realtime.buffer.replay.events")
    bufferResyncRequired = tel.Meter.NewInt64Counter("ace.realtime.buffer.resync.required")
)
```

### 4.2 Backend Spans (OTel)

Key operations get trace spans:

| Span Name | Attributes | When |
|-----------|-----------|------|
| `realtime.ws.upgrade` | `user_id`, `connection_id` | WebSocket upgrade |
| `realtime.ws.auth` | `user_id`, `success` | Auth handshake |
| `realtime.ws.subscribe` | `user_id`, `topics` | Topic subscription |
| `realtime.ws.disconnect` | `user_id`, `connection_id`, `duration_ms` | Connection close |
| `realtime.nats.dispatch` | `topic`, `recipients` | NATS event fan-out |
| `realtime.poll` | `user_id`, `topics`, `since_seq` | Polling request |

### 4.3 Backend Usage Events

```go
// Usage events for connection time (cost attribution per user):
err = tel.Usage.Publish(ctx, telemetry.UsageEvent{
    AgentID:       "",  // not agent-specific
    OperationType: telemetry.OperationTypeNATSPublish,
    ResourceType:  telemetry.ResourceTypeMessaging,
    DurationMs:    connectionDurationMs,
    Metadata:      map[string]string{
        "user_id":        client.userID,
        "connection_id":  client.id,
        "transport":      "websocket",  // or "polling"
        "topics_count":   strconv.Itoa(len(client.topics)),
    },
})
```

### 4.4 Frontend Observability

The RealtimeManager exposes reactive state that components can bind to:

```typescript
// Components can observe connection state:
realtimeManager.status          // 'connecting' | 'connected' | 'polling' | 'disconnected'
realtimeManager.reconnectAttempts  // number of reconnect attempts
```

No separate frontend telemetry is needed. The existing `$lib/telemetry` module can log connection state changes as structured events. The `ConnectionIndicator` component renders the state visually.

---

## 5. Security

### 5.1 Authentication

- **WebSocket**: JWT in first message (research decision D7). The unauthenticated window is 5 seconds — connections without a valid auth message are closed with `websocket.StatusPolicyViolation`.
- **Polling**: Standard `Authorization: Bearer <token>` header. Reuses existing `auth_middleware.go`.
- **Token refresh on WebSocket**: The RealtimeManager sends a new `{"type": "auth", "token": "<new_token>"}` message when the access token is refreshed. The Hub validates the new token and updates the client's claims. No reconnection needed.

### 5.2 Authorization

- **Topic-level filtering**: When a client subscribes to `agent:{id}:status`, the Hub checks whether the user has permission to view that agent. Regular users can only subscribe to their own agents; admins can subscribe to any.
- **Event-level filtering**: In `dispatchNATSEvent`, each event is checked against the recipient's permissions before sending. A user never receives events for agents they don't have access to.
- **Polling endpoint**: The `GET /api/realtime/updates` endpoint filters results by the authenticated user's permissions.

### 5.3 Input Validation

- **Topic strings**: Validated against a whitelist pattern (`^[a-z0-9]+:[a-z0-9-]+:[a-z0-9_]+$`). Reject malformed topics immediately.
- **Max subscriptions**: Capped at 50 topics per connection. Prevents resource exhaustion.
- **Message size**: Max 64KB per message. Messages exceeding this are dropped.
- **Rate limiting**: The existing rate limiter applies to the polling endpoint. WebSocket messages are rate-limited per connection (max 100 messages/second).

### 5.4 Transport Security

- **WSS in production**: WebSocket connections use `wss://` in production (handled by reverse proxy/TLS termination).
- **CORS**: WebSocket upgrade uses the same CORS policy as the REST API. The `Origin` header is validated during upgrade.
- **Compression**: `permessage-deflate` is available but disabled by default. Can be enabled for bandwidth-constrained clients.

---

## 6. Failure Modes

| Scenario | Impact | Detection | Recovery |
|----------|--------|-----------|----------|
| Client WebSocket disconnects | Events buffered on server | Hub detects close | Client reconnects, sends `replay` with `last_seq`, server replays missed events |
| Server restarts | All clients disconnected | WebSocket close frame sent | Clients reconnect with exponential backoff, then full resync via REST |
| NATS connection lost | No internal events delivered | Hub NATS subscription error | Events stop flowing. Clients see no new data. On NATS reconnect, Hub re-subscribes to all topics |
| Hub buffer overflow | Short disconnections lose events | `resync_required` sent to client | Client calls REST endpoint for full state fetch |
| Auth token expires during WebSocket | Client can't re-auth after server restart | Client detects 401 on polling or server closes WS | RealtimeManager refreshes token via AuthStore, sends new `auth` message |
| Client network blocked (no WS, no HTTP) | Complete isolation | All requests fail, `status = 'disconnected'` | Retry with max backoff (30s). Show disconnected indicator. No data loss — resync on reconnect |
| Razor-thin network (WS blocked, HTTP works) | WebSocket fails, polling works | WebSocket upgrade fails or times out | Fall back to polling with adaptive interval (1s active, 10s idle) |
| Message burst (100 events/sec) | Client send channel fills | Channel buffer full | Non-blocking send: if channel full, drop oldest non-critical event. Critical events (auth, control) never dropped |

---

## 7. Configuration

### 7.1 Backend Configuration (Environment Variables)

| Variable | Default | Description |
|----------|---------|-------------|
| `WS_AUTH_TIMEOUT` | `5s` | Time to wait for auth message after WebSocket upgrade |
| `WS_MAX_SUBSCRIPTIONS` | `50` | Max topics per WebSocket connection |
| `WS_MAX_MESSAGE_SIZE` | `65536` | Max message size in bytes (64KB) |
| `WS_HEARTBEAT_INTERVAL` | `30s` | Server heartbeat interval |
| `WS_SEND_CHANNEL_SIZE` | `128` | Buffered channel size per client |
| `BUFFER_MAX_SIZE` | `1000` | Max events per topic in replay buffer |
| `BUFFER_MAX_AGE` | `5m` | Max age of events in replay buffer |
| `POLL_MAX_TOPICS` | `20` | Max topics per polling request |
| `POLL_RATE_LIMIT` | `60/min` | Rate limit for polling endpoint per user |

### 7.2 Frontend Configuration (Constants)

| Constant | Default | Description |
|----------|---------|-------------|
| `WS_URL` | `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.host}/api/ws` | WebSocket URL |
| `POLL_URL` | `/api/realtime/updates` | Polling endpoint |
| `AUTH_TIMEOUT_MS` | `5000` | Time to wait for auth_ok after connect |
| `RECONNECT_BASE_MS` | `1000` | Initial reconnect delay |
| `RECONNECT_MAX_MS` | `30000` | Maximum reconnect delay |
| `RECONNECT_MAX_ATTEMPTS` | `5` | Attempts before falling back to polling |
| `POLL_INTERVAL_ACTIVE_MS` | `1000` | Polling interval when user is active |
| `POLL_INTERVAL_IDLE_MS` | `10000` | Polling interval when user is idle |
| `HEARTBEAT_INTERVAL_MS` | `30000` | Client heartbeat interval |
| `ACTIVITY_TIMEOUT_MS` | `30000` | Time since last activity before considering idle |

---

## 8. Architectural Decisions Record

| # | Decision | Choice | Alternatives Considered | Rationale |
|---|----------|-------|------------------------|-----------|
| ADR-1 | WebSocket library | coder/websocket | gorilla/websocket, gobwas/ws | Research D1: context support, zero deps, modern API |
| ADR-2 | Transport strategy | WebSocket + polling hybrid | WebSocket-only, SSE, WS+SSE | Research D2: matches problem_space constraint, polling works everywhere |
| ADR-3 | NATS bridging | Custom Go Hub bridge | NATS WS gateway, direct NATS | Research D3: auth filtering, message enrichment, namespace privacy |
| ADR-4 | Subscription model | Topic-based on single connection | Per-resource connections, broadcast-all | Research D4: efficient, dynamic, maps to NATS |
| ADR-5 | Reconnection replay | Seq IDs + bounded buffer + REST resync fallback | Timestamp replay, full state resync only | Research D5: gap-free for short disconnects, graceful for long ones |
| ADR-6 | Polling design | Adaptive interval (1s/10s) | Fixed interval, long polling | Research D6: efficient when idle, responsive when active |
| ADR-7 | WebSocket auth | JWT in first message | JWT in query param, Sec-WebSocket-Protocol | Research D7: token not in URLs, refreshable without reconnect |
| ADR-8 | Frontend manager | Custom Svelte 5 runes class | svelte-realtime, sveltekit-websockets | Research D8: compatible with adapter-static, consistent with existing stores |
| ADR-9 | Message format | Typed per-event with discriminated union | Generic envelope with opaque payload | Research D9: type safety, extensible, matches "no any" constraint |
| ADR-10 | Backend pattern | Hub + Client + TopicReg | Flat connection map | Research D10: NATS sub sharing, per-client filtering, scales well |
| ADR-11 | Observability | Extend shared/telemetry | New observability module | Research D11: reuse existing spans, metrics, logs, usage events |
| ADR-12 | Auth enforcement | Topic-level + event-level dual filter | Topic-level only, event-level only | Dual filtering prevents both unauthorized subscriptions and data leakage from shared NATS subs |
| ADR-13 | Send pattern | Buffered channel + write goroutine | Mutex-protected direct writes | Eliminates concurrent write risk, non-blocking send to slow clients |
| ADR-14 | Topic namespace | Public-facing topics mapped to NATS subjects | Exposing raw NATS subjects | Security: internal NATS structure is private; can change without client changes |