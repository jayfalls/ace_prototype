# Communication Patterns — Slice 9

**Unit:** agents-study  
**Output:** `design/units/agents-study/study/communication.md`  
**Research conducted:** 2026-04-23

---

## 1. Overview

This document cross-cuts the 13 studied agent systems along communication architecture dimensions: message passing patterns, context propagation, broadcast patterns, surface independence, and protocol choices. It concludes with an ACE Recommendation table validating ACE's existing NATS decision with comparative evidence.

---

## 2. OpenClaw — Gateway Hub with Channel Adapters

### Architecture

OpenClaw uses a **hub-and-spoke gateway** as the central message router. All channel adapters (Slack, Telegram, Discord, WhatsApp, Matrix, iOS, Android, etc.) connect to the single Gateway process, which normalizes messages into a unified session model.

### Communication Details

| Dimension | Implementation |
|-----------|----------------|
| **Message Routing** | `src/gateway/server-channels.ts` — ChannelManager orchestrates lifecycle of each channel plugin. Per-channel `startChannel` / `stopChannel` with restart backoff policy. |
| **Session Routing** | `src/gateway/server-node-events.ts` — `handleNodeEvent` routes inbound events (voice, agent.request, notifications, exec.*) to sessions keyed by `sessionKey`. |
| **Broadcast** | `src/gateway/server-broadcast.ts` — `createGatewayBroadcaster` fans out to connected WebSocket clients. Per-event scope guards (`READ_SCOPE`, `WRITE_SCOPE`, `ADMIN_SCOPE`) filter who receives what. |
| **Normalization** | `src/channels/registry.ts` — `normalizeChannelId` / `normalizeAnyChannelId` resolve channel aliases to canonical IDs. |
| **Protocol** | WebSocket for real-time client connections; HTTP/REST for control plane; channel adapters use platform-native protocols (Slack RTM, Telegram Bot API, etc.). |
| **Context Propagation** | `sessionKey` threads through all event types. Voice transcripts deduplicated via fingerprinting (callId + sequence/timestamp). |
| **Surface Independence** | Core gateway knows nothing about channel semantics — only normalized session events. Channel adapters live in `extensions/*` and register via plugin manifest. |

### Key Files

- `src/gateway/server-channels.ts` — Channel lifecycle manager
- `src/gateway/server-broadcast.ts` — WebSocket broadcast with scope guards
- `src/gateway/server-node-events.ts` — Event routing by sessionKey
- `src/channels/registry.ts` — Channel ID normalization

### Assessment

OpenClaw's gateway is a **bottleneck risk** — single Gateway process handling all 24+ channel adapters. Session serialization is per-`sessionKey` in the Gateway process. The architecture explicitly acknowledges this with restart policies and health monitoring. For ACE, this validates the NATS decision: a message bus avoids single-process bottlenecks.

---

## 3. OpenCode — Effect/PubSub Bus + JSON-RPC over stdio

### Architecture

OpenCode uses an **internal event bus** (`Effect`'s `PubSub`) for in-process communication, with a JSON-RPC server for external clients (TUI, CLI, web). The bus is typed with Zod schemas — events are defined as `BusEvent.define(name, schema)`.

### Communication Details

| Dimension | Implementation |
|-----------|----------------|
| **Message Routing** | `src/bus/index.ts` — `Service` interface with `publish`, `subscribe`, `subscribeAll`. Uses `Effect.PubSub` internally. |
| **Bus Topology** | In-process PubSub with wildcard subscriber. `GlobalBus.emit` bridges to cross-instance events via `InstanceState`. |
| **Protocol** | JSON-RPC over stdio for TUI; HTTP/WebSocket for web client. `src/server/server.ts` — Hono-based HTTP server with WebSocket upgrade. |
| **Context Propagation** | `agentId` is implicit in `InstanceState`. Events carry typed `payload` with Zod-validated properties. |
| **Surface Independence** | Bus is in-process; surfaces (TUI, CLI, web) are separate entry points sharing the same bus instance. |

### Key Files

- `src/bus/index.ts` — Typed event bus with Effect PubSub
- `src/server/server.ts` — Hono HTTP/WebSocket server

### Assessment

OpenCode's bus is **in-process only**, suitable for single-process deployments. The typed event definition pattern (Zod schemas for events) is a strong pattern ACE should adopt for NATS subject contracts. The JSON-RPC over stdio for TUI is pragmatic but limits remote operation.

---

## 4. Goose — Tokio Broadcast Channel + Gateway Trait

### Architecture

Goose uses **tokio broadcast channels** for in-process event distribution, with a `Gateway` trait abstracting platform adapters (currently Telegram only). The `SessionEventBus` provides replay buffers for SSE clients.

### Communication Details

| Dimension | Implementation |
|-----------|----------------|
| **Message Routing** | `crates/goose-server/src/session_event_bus.rs` — `SessionEventBus` with `broadcast::channel(256)` and `VecDeque` replay buffer of 512 events. |
| **Replay Mechanism** | `subscribe(last_event_id)` replays buffered events with seq > last_event_id. Returns `Err(ClientTooFarBehind)` if client drifted too far. |
| **Broadcast** | Tokio `broadcast::Sender<SessionEvent>` fans out to all subscribers. |
| **Protocol** | SSE (Server-Sent Events) for real-time; HTTP/REST for control. `crates/goose-server/src/routes/` has explicit reply routes. |
| **Context Propagation** | `SessionEvent { seq, request_id, event }` — monotonic sequence per session for ordering guarantees. |
| **Gateway Pattern** | `crates/goose/src/gateway/mod.rs` — `Gateway` trait with `start`, `send_message`, `validate_config`. Pluggable: Telegram implemented. |

### Key Files

- `crates/goose-server/src/session_event_bus.rs` — Broadcast channel + replay buffer
- `crates/goose/src/gateway/mod.rs` — Gateway trait abstraction
- `crates/goose/src/gateway/manager.rs` — Gateway lifecycle management

### Assessment

Goose's `SessionEventBus` with **monotonic sequencing and replay buffers** is the most robust SSE/live event pattern in the study. The `ClientTooFarBehind` error handling is explicit about client drift. ACE's JetStream persistence for replay aligns with this pattern.

---

## 5. Hermes Agent — JSON-RPC over stdio + Gateway Platform Adapters

### Architecture

Hermes uses a **bifurcated communication model**: the TUI gateway communicates with the main process over **stdio JSON-RPC** (newline-delimited JSON), while messaging platform adapters (Telegram, Discord, Slack, WhatsApp, etc.) communicate via **HTTP webhooks + long-polling**.

### Communication Details

| Dimension | Implementation |
|-----------|----------------|
| **Message Routing** | `tui_gateway/server.py` — JSON-RPC dispatcher with thread pool for long handlers (`slash.exec`, `session.resume`, `shell.exec`). |
| **Long Handler Pattern** | `_LONG_HANDLERS` frozenset routed to `ThreadPoolExecutor` to avoid blocking the dispatcher loop. |
| **Platform Routing** | `gateway/run.py` — main loop + slash commands + message dispatch. Platform adapters in `gateway/platforms/`. |
| **Protocol** | stdio JSON-RPC for TUI; HTTP for messaging platforms; WebSocket possible via platform hooks. |
| **Context Propagation** | `session_key` as the primary routing key. `_set_session_context` / `_clear_session_context` manage async-local storage per session. |
| **ACP Adapter** | `acp_adapter/entry.py` — ACP (Agent Client Protocol) stdio adapter for VS Code / Zed / JetBrains integration. |
| **Surface Independence** | Core `AIAgent` in `run_agent.py` is invoked by CLI, TUI gateway, ACP adapter, and messaging gateways — all surface-agnostic. |

### Key Files

- `tui_gateway/server.py` — JSON-RPC over stdio with thread pool dispatcher
- `gateway/run.py` — Main gateway loop with platform routing
- `acp_adapter/entry.py` — ACP stdio adapter

### Assessment

Hermes has the **most mature surface separation** — the same `AIAgent` powers CLI, TUI, ACP, and 15+ messaging platforms. The stdio JSON-RPC pattern for TUI is simple but effective. The thread pool for long handlers prevents dispatcher starvation — a pattern ACE should consider for blocking operations on the NATS bus.

---

## 6. pi-mono — Surface-Agnostic Core with File-Based IPC

### Architecture

pi-mono's `pi-agent-core` (in `@mariozechner/pi-agent-core`) is surface-agnostic. The `mom` package implements a Slack bot by importing the core agent. Communication between the harness (Slack Socket Mode) and the agent is **file-based**: `log.jsonl` for message history, `context.jsonl` for LLM context.

### Communication Details

| Dimension | Implementation |
|-----------|----------------|
| **Message Routing** | `packages/mom/src/agent.ts` — `AgentRunner` cached per channel. Messages read from `log.jsonl`, context built from `context.jsonl`. |
| **Event System** | `packages/mom/src/events.ts` — `EventsWatcher` polls filesystem for JSON event files (`immediate`, `one-shot`, `periodic`). Triggers via synthetic Slack events. |
| **IPC Mechanism** | File-based: `log.jsonl` (append-only history), `context.jsonl` (LLM context), `MEMORY.md` (persistent memory). File watching via `FSWatcher`. |
| **Sandbox Isolation** | `packages/mom/src/sandbox.ts` — Docker container or host execution. Path translation between container and host paths. |
| **Protocol** | Slack Socket Mode (WebSocket) for inbound; filesystem for agent communication. |

### Key Files

- `packages/mom/src/agent.ts` — Agent runner with file-based session context
- `packages/mom/src/events.ts` — Filesystem-based event scheduling
- `packages/mom/src/sandbox.ts` — Docker/host sandbox abstraction

### Assessment

pi-mono's **file-based IPC** is the simplest in the study — no network protocols between harness and agent, just filesystem operations. This is extremely reliable but limits to single-machine deployments. ACE's NATS bus provides the same reliability guarantees with distribution support.

---

## 7. Oh My OpenAgent — 52 Lifecycle Hooks with Event Propagation

### Architecture

Oh My OpenAgent communicates via **lifecycle hooks** — 52 hooks across 5 tiers (Session, Tool Guard, Transform, Continuation, Skill). Hooks are registered at startup and propagate through the agent loop.

### Communication Details

| Dimension | Implementation |
|-----------|----------------|
| **Hook Architecture** | `src/hooks/index.ts` — exports all hook factories. Hooks receive `(event, ctx)` and return values based on event type. |
| **Hook Tiers** | Session (24) → Tool Guard (14) → Transform (5) → Continuation (7) → Skill (2). |
| **Inter-Layer Communication** | Hooks communicate via shared `ctx` object. No explicit message passing — hooks read/write context fields. |
| **Event Propagation** | Events: `session.created`, `session.idle`, `session.error`, `tool.execute.before`, `tool.execute.after`, `chat.message`, `chat.params`, etc. |
| **Examples** | `atlasHook` orchestrates boulder sessions; `todoContinuationEnforcer` injects continuation prompts; `ralphLoop` implements self-referential dev loop. |
| **Plugin Interface** | `src/plugin/hooks/create-*-hooks.ts` compose hook tiers. OpenCode plugin interface at `src/plugin-interface.ts`. |

### Key Files

- `src/hooks/index.ts` — Hook exports and registration
- `src/hooks/atlas/` — Master orchestrator hook (17 files, ~1976 LOC)
- `src/hooks/ralph-loop/` — Self-referential dev loop (14 files, ~1687 LOC)

### Assessment

Oh My OpenAgent's hook system is **event-driven but tightly coupled** — hooks share mutable `ctx`. The 52 hooks + 160k+ LOC codebase is complex. For ACE, the hook pattern is sound but should be combined with typed NATS subjects rather than shared mutable state.

---

## 8. Devin — VM Isolation + WebSocket to Cloud Brain

### Architecture

Devin uses **full VM isolation** per session, with a cloud-hosted "Brain" that communicates with the DevBox VM via WebSocket. The manager coordinates worker VMs via the same Brain infrastructure.

### Communication Details

| Dimension | Implementation |
|-----------|----------------|
| **VM Isolation** | Each Devin session (including managed child Devins) runs in its own VM. Isolation is at the hypervisor level, not process level. |
| **Manager→Worker Communication** | Parent Devin coordinates via Devin's internal infrastructure — same messaging as human→Devin. Task assignment via structured prompts, result compilation via context merging. |
| **Brain↔DevBox Link** | WebSocket connection from DevBox VM to isolated Brain container. On DevBox startup, WebSocket opens and connects to Cognition tenant. |
| **Protocol** | WebSocket (primary), HTTPS/443 (control plane). All exchanges over WebSocket once DevBox is bootstrapped. |
| **Context Propagation** | Playbooks provide structured context to child Devins. Knowledge management stores cross-session context. |
| **Parallel Execution** | Up to 10 managed Devins in parallel. Each in own VM with independent state. Coordinator scopes work, monitors progress, resolves conflicts, compiles results. |

### Evidence Sources

- Devin VPC docs: "On DevBox startup, a websocket opens and connects to an isolated container in the Devin tenant. All subsequent exchanges happen over this connection."
- Devin Advanced Capabilities: "Devin can delegate to a team of managed Devins that work in parallel. Each managed Devin is a full Devin with its own isolated virtual machine."
- Devin MCP server: `devin_session_events` for event inspection; `devin_session_gather` to wait for multiple sessions.

### Assessment

Devin's VM isolation is the **heaviest isolation model** in the study — appropriate for cloud-native SaaS where isolation = security = trust. ACE should not adopt full VM isolation for local-first, but the **manager/worker coordination pattern** (task decomposition, parallel execution, result compilation) maps to ACE pods with NATS inter-pod messaging.

---

## 9. Cross-Cutting Comparison

### 9.1 Message Passing Patterns

| System | Pattern | Evidence |
|--------|---------|----------|
| OpenClaw | Hub-and-spoke gateway | Single `Gateway` process normalizes all channels |
| OpenCode | In-process PubSub | `Effect.PubSub` with typed events |
| Goose | Tokio broadcast + replay | `SessionEventBus` with monotonic seq |
| Hermes | JSON-RPC + platform adapters | stdio RPC for TUI; HTTP for messaging |
| pi-mono | File-based IPC | `log.jsonl` / `context.jsonl` |
| Oh My OpenAgent | Hook event propagation | 52 hooks across 5 tiers |
| Devin | WebSocket + VM isolation | Brain↔DevBox WebSocket; manager coordinates via Brain |
| **ACE** | **NATS with typed subjects** | `shared/messaging` with typed `Subject` values |

### 9.2 Context Propagation

| System | Mechanism | Scope |
|--------|-----------|-------|
| OpenClaw | `sessionKey` threading | Per-session in Gateway |
| OpenCode | `InstanceState` + Zod payloads | Per-instance |
| Goose | `SessionEvent.seq` monotonic | Per-session |
| Hermes | `session_key` + async-local storage | Per-session via `_set_session_context` |
| pi-mono | File-based: `log.jsonl` → `context.jsonl` | Per-channel filesystem |
| Oh My OpenAgent | Shared `ctx` object in hooks | Per-invocation |
| Devin | Playbooks + Knowledge API | Per-session + cross-session |
| **ACE** | **`agentId` in all messages, spans, DB rows** | **Per-agent + per-pod + per-swarm** |

### 9.3 Broadcast Patterns

| System | Broadcast Mechanism | Fan-out |
|--------|---------------------|---------|
| OpenClaw | WebSocket broadcaster with scope guards | Per-connection filtering |
| OpenCode | Wildcard PubSub subscription | All subscribers |
| Goose | Tokio broadcast channel | All receivers |
| Hermes | SSE replay + live receiver | All SSE clients |
| pi-mono | Filesystem polling | Single consumer per channel |
| Oh My OpenAgent | Hook chaining | Sequential per tier |
| Devin | Session event aggregation via MCP | `devin_session_gather` for multi-session |
| **ACE** | **NATS subject fan-out + JetStream** | **Reference-counted TopicReg, per-user auth** |

### 9.4 Surface Independence

| System | Surface-Agnostic Core | Evidence |
|--------|----------------------|----------|
| OpenClaw | Partial — Gateway knows channel semantics | 24+ channel adapters but unified session model |
| OpenCode | Yes — Bus + server separation | TUI, CLI, web share bus |
| Goose | Partial — `Gateway` trait, currently Telegram only | `crates/goose/src/gateway/mod.rs` |
| Hermes | **Yes — AIAgent invoked by CLI, TUI, ACP, 15+ platforms** | `run_agent.py` is surface-agnostic |
| pi-mono | **Yes — `pi-agent-core` imported by CLI, TUI, Slack** | `packages/mom/src/agent.ts` imports core |
| Oh My OpenAgent | Yes — Hook interface abstracts host | OpenCode plugin interface |
| Devin | No — Brain is cloud-only | VPC deployment with Azure-hosted Brain |
| **ACE** | **Yes — cognitive layers communicate via NATS, not surface** | **Chat Interface is the only surface-aware component** |

### 9.5 Protocol Choices

| System | Primary Protocol | Secondary |
|--------|------------------|-----------|
| OpenClaw | WebSocket (clients), HTTP (control) | Channel-native (Slack RTM, Telegram API) |
| OpenCode | JSON-RPC (stdio), HTTP (web) | WebSocket upgrade |
| Goose | SSE (server-sent events) | HTTP/REST |
| Hermes | stdio JSON-RPC (TUI), HTTP (messaging platforms) | Webhooks |
| pi-mono | Slack Socket Mode (WebSocket) | Filesystem |
| Oh My OpenAgent | Hook callbacks (in-process) | OpenCode plugin protocol |
| Devin | WebSocket (Brain↔DevBox) | HTTPS (control plane) |
| **ACE** | **NATS (internal), WebSocket (clients)** | **HTTP/REST for control plane** |

---

## 10. ACE NATS Decision Validation

### Evidence Synthesis

| Pattern | Supporting Systems | Contradicting Systems | ACE Validation |
|---------|-------------------|----------------------|----------------|
| **Hub-and-spoke with message bus** | OpenClaw (gateway), Hermes (gateway) | pi-mono (file-based), Devin (VM) | **ADOPTED** — NATS as central bus avoids single-process bottlenecks of OpenClaw's Gateway |
| **Typed event definitions** | OpenCode (Zod schemas), Hermes (JSON-RPC) | None | **ADOPTED** — ACE's typed `Subject` with `.Format()` method prevents silent routing failures |
| **Replay buffers for consumers** | Goose (`SessionEventBus` 512-event replay), Hermes (SSE replay) | None | **ADOPTED** — JetStream provides durable replay beyond in-memory buffers |
| **Monotonic sequencing** | Goose (seq per session) | None | **ADOPTED** — JetStream `SeqManager` or message sequence numbers |
| **Reference-counted subscriptions** | OpenCode (PubSub subscribers) | None | **ADOPTED** — `TopicReg` reference counts avoid redundant NATS subscriptions |
| **Scope/permission filtering on broadcast** | OpenClaw (READ_SCOPE, WRITE_SCOPE guards) | None | **ADOPTED** — Hub dispatches only to authorized clients per topic |
| **Async-local session context** | Hermes (`_set_session_context`) | None | **ADOPTED** — Go context.Context propagates trace + auth |
| **Long-running handler thread pool** | Hermes (ThreadPoolExecutor for slash.exec) | None | **ADOPTED** — NATS subscription handlers should not block; use worker pools for heavy processing |
| **Manager/worker coordination** | Devin (parent Devin → child Devins) | None | **ADOPTED** — ACE pods coordinate via NATS; parent pod scopes work, monitors, compiles |
| **Surface-agnostic core** | Hermes, pi-mono | OpenClaw (Gateway-centric) | **ADOPTED** — ACE cognitive layers are NATS-only; Chat Interface translates external protocols |

### What ACE Does Better

1. **Embedded NATS vs. external services**: OpenClaw, Hermes, and Devin all require external infrastructure (Gateway daemon, platform APIs, cloud tenants). ACE's embedded NATS enables single-binary deployment with zero external dependencies.

2. **Typed subjects vs. string-based routing**: OpenCode uses Zod schemas for event payloads but string-based bus topics. ACE's typed `Subject` values with `.Format()` compile-time-check subject construction.

3. **JetStream persistence vs. in-memory buffers**: Goose's 512-event replay buffer and Hermes's SSE replay are in-memory. ACE's JetStream provides durable persistence and replay across restarts.

4. **Reference-counted TopicReg vs. manual subscription management**: OpenClaw manages per-channel subscriptions in a `Map`. ACE's `TopicReg` automatically creates/releases NATS subscriptions based on client interest.

### What ACE Should Adopt from Others

1. **Hermes's thread pool for blocking handlers**: ACE NATS handlers that perform heavy work (file I/O, LLM calls) should be routed to a worker pool, not block the subscription callback.

2. **Goose's `ClientTooFarBehind` error for replay drift**: When JetStream consumers fall behind, ACE should detect and surface this explicitly rather than silently dropping events.

3. **pi-mono's event file pattern for scheduling**: ACE's Swarm Coordinator could use a filesystem-based event scheduling (cron events, immediate events) as an alternative to pure NATS scheduling for persistent workflows.

---

## 11. ACE Recommendation Table

| Communication Pattern | Evidence | ACE Recommendation | Implementation |
|----------------------|----------|-------------------|----------------|
| **NATS as central message bus** | OpenClaw Gateway bottleneck; Hermes 15+ surface routing; ACE already uses NATS | **ADOPT** (existing) | Continue embedding NATS; do not add Gateway-style central process |
| **Typed subject constants** | OpenCode Zod event schemas; ACE already has `Subject` type | **ADOPT** (existing) | Keep `shared/messaging/subjects.go` typed; never construct subjects by string concatenation |
| **JetStream for durable replay** | Goose 512-event replay buffer; Hermes SSE replay | **ADOPT** (existing) | JetStream streams for all event types requiring replay |
| **Reference-counted TopicReg** | OpenCode PubSub; ACE already has TopicReg | **ADOPT** (existing) | TopicReg maps public topics → NATS subjects with refcounting |
| **Per-user auth on broadcast** | OpenClaw scope guards on WebSocket | **ADOPT** (existing) | Hub checks user permissions before dispatching to client |
| **Async context propagation** | Hermes `_set_session_context` | **ADOPT** (existing) | Go `context.Context` carries trace + auth through NATS handlers |
| **Blocking handler thread pool** | Hermes `ThreadPoolExecutor` for slash.exec | **ADOPT** | Add worker pool for NATS handlers doing heavy work |
| **Monotonic session sequencing** | Goose `SessionEvent.seq` | **ADOPT** | JetStream `SeqManager` or `StreamInfo` sequence tracking |
| **Hook event system** | Oh My OpenAgent 52 hooks | **ADAPT** | ACE event system uses typed NATS subjects, not shared mutable `ctx` |
| **File-based IPC for sandbox** | pi-mono `log.jsonl` / `context.jsonl` | **ADAPT** | ACE sandbox could use file-based IPC for extreme isolation scenarios |
| **Manager/worker VM isolation** | Devin (full VM per session) | **AVOID** (local-first) | Full VM isolation is cloud-only; process isolation is ACE baseline |
| **Hub-and-spoke gateway** | OpenClaw single Gateway process | **AVOID** | NATS bus avoids single-process bottleneck; no Gateway needed |
| **Cloud-only Brain** | Devin Brain in Cognition tenant | **AVOID** (local-first) | ACE brain (cognitive engine) must run locally; cloud is opt-in |

---

## 12. Key Findings

### Convergence Points

1. **Typed event definitions**: OpenCode (Zod), Hermes (JSON-RPC schemas), and ACE (Go typed structs) all converge on defining event shapes explicitly. This prevents routing errors and enables documentation.

2. **Broadcast with scope filtering**: OpenClaw and ACE both filter broadcasts by client permissions. This is essential for multi-user deployments.

3. **Reference-counted subscription management**: OpenCode's PubSub and ACE's TopicReg both avoid redundant subscriptions. This optimization matters at scale.

4. **Replay for disconnected clients**: Goose and Hermes both provide replay mechanisms. JetStream makes this durable.

### Divergence Points

1. **In-process vs. distributed**: OpenCode and pi-mono use in-process communication (PubSub, filesystem). Hermes, OpenClaw, and ACE use network-based communication (NATS, WebSocket). The choice depends on deployment model.

2. **Tight vs. loose coupling**: Oh My OpenAgent's hooks are tightly coupled (shared `ctx`). ACE's NATS messages are loosely coupled (producer/consumer share nothing but the subject). Loose coupling scales better.

3. **Centralized vs. federated**: OpenClaw's Gateway is a single central process. ACE's NATS bus is decentralized — any client can publish/subscribe without a central coordinator.

---

*Document Complete.*
