# Multi-Agent Delegation & Orchestration — Cross-System Comparison

**Slice:** 5 of 14  
**Systems Analyzed:** Devin, OpenCode, Oh My OpenAgent, Goose, Hermes Agent, OpenClaw, Autoresearch  
**Research Repos:** `/design/units/agents-study/research/{opencode,oh-my-openagent,goose,hermes-agent,openclaw,autoresearch}`

---

## 1. System-by-System Delegation Architectures

### 1.1 Devin (Cognition Labs) — Manager/Worker Hierarchy

**Architecture Type:** Hierarchical orchestrator with VM isolation

Devin implements the most production-grade manager/worker model found in this study. A primary "coordinator" Devin scopes work, assigns pieces to managed "worker" Devins, monitors progress, resolves conflicts, and compiles results.

**Key Characteristics:**
- **Manager/Worker Ratio:** 1 manager + up to 10 workers (per enterprise announcement)
- **Isolation Boundary:** Each worker runs in its own isolated VM with its own terminal, browser, and development environment
- **Concurrency Model:** Workers run in parallel; manager synthesizes results
- **Conflict Resolution:** Manager reads full trajectories from workers, identifies conflicts, merges changes into one branch or PR
- **Task Decomposition:** Automatic — Devin analyzes work, identifies parallelizable components, spins up managed instances
- **Scheduling:** Composes with recurring schedules (Scheduled Devins) — e.g., weekly QA pass with parallel managed Devins per page
- **Lifecycle Control:** Manager can message children mid-task, put them to sleep, terminate them, monitor ACU consumption
- **Improvement Loop:** Manager reads worker trajectories to improve future task decomposition

**Source:** [Cognition — Devin can now Manage Devins](https://www.cognition-labs.com/blog/devin-can-now-manage-devins), [Devin September 2024 Product Update](http://www.cognition-labs.com/blog/sept-24-product-update)

**ACE Relevance:** Devin's VM-level isolation is the gold standard for fault containment. ACE should consider pod-level isolation with namespace separation for workers.

---

### 1.2 OpenCode (anomalyco/opencode) — Native + File-Based Agent Registry

**Architecture Type:** Single-registry multi-agent with permission-filtered subagents

OpenCode has a unified agent registry that merges native agents (built into the binary) with file-based agents (defined in Markdown under `~/.config/opencode/agents/` or `.opencode/agents/`). Subagents are invoked via `@mention` syntax or automatically by primary agents.

**Key Characteristics:**
- **Built-in Agents:**
  - Primary: `Build` (all tools), `Plan` (read-only, permission-restricted)
  - Subagent: `General` (full tools except todo), `Explore` (read-only)
  - Hidden: `Compaction`, `Title`, `Summary` (system agents)
- **Mode Switching:** Agents operate in `primary` or `subagent` mode. Plan/Build toggle via Tab key. Plan mode filters tools (edit, bash → `ask`)
- **Subagent Invocation:** `@general help me search for this function` triggers subagent with own child session
- **Session Hierarchy:** Parent creates child sessions; navigation via `session_child_first`, `session_child_cycle`, `session_parent`
- **Depth/Child Limits:** `maxSpawnDepth` and `maxChildrenPerAgent` configurable; when reached, agent gets system prompt to summarize and recommend remaining tasks
- **Tool Restrictions:** Permission system per agent — `edit`, `bash`, `webfetch` can be `allow`, `ask`, or `deny`
- **Configuration:** JSON (`opencode.json`) or Markdown (`.opencode/agents/review.md`)

**Files Analyzed:**
- `packages/web/src/content/docs/agents.mdx` — agent configuration docs (752 lines)
- `packages/opencode/src/cli/cmd/tui/routes/session/subagent-footer.tsx`

**ACE Relevance:** OpenCode's permission-filtered agent model is the right approach for ACE. The `@mention` invocation pattern is elegant and should be adopted.

---

### 1.3 Oh My OpenAgent (code-yeongyu/oh-my-openagent) — Intent Gate → Planner → Executor Chain

**Architecture Type:** Specialized agent ecosystem with explicit orchestration chain and parallel background execution

Oh My OpenAgent implements the most sophisticated multi-agent ecosystem in this study: 11 specialized agents, a three-layer Intent Gate → Prometheus (planner) → Atlas (orchestrator) → Workers chain, and a full-featured BackgroundManager for async parallel execution.

**11 Built-in Agents:**

| Agent | Model | Mode | Purpose |
|-------|-------|------|---------|
| **Sisyphus** | claude-opus-4-7 max | primary | Main orchestrator, plans + delegates |
| **Hephaestus** | gpt-5.4 medium | all | Autonomous deep worker |
| **Oracle** | gpt-5.4 high | subagent | Read-only consultation |
| **Librarian** | gpt-5.4-mini-fast | subagent | External docs/code search |
| **Explore** | gpt-5.4-mini-fast | subagent | Contextual grep |
| **Multimodal-Looker** | gpt-5.3-codex medium | subagent | PDF/image analysis |
| **Metis** | claude-opus-4-7 max | subagent | Pre-planning consultant |
| **Momus** | gpt-5.4 xhigh | subagent | Plan reviewer |
| **Atlas** | claude-sonnet-4-6 | primary | Todo-list orchestrator |
| **Prometheus** | claude-opus-4-7 max | internal | Strategic planner |
| **Sisyphus-Junior** | claude-sonnet-4-6 | all | Category-spawned executor |

**Orchestration Chain:**
1. **IntentGate** — classifies user intent (research/implementation/investigation/evaluation/fix) before any action
2. **Prometheus** — strategic planner, markdown-only output, forbidden paths enforcement
3. **Atlas** — todo-list orchestrator, spawns workers via `task` tool
4. **Workers** — execute via 8 delegation categories (visual-engineering, ultrabrain, deep, artistry, quick, unspecified-low, unspecified-high, writing)

**BackgroundManager (~10k LOC, 47 files):**
- Concurrency: 5 concurrent per model/provider (configurable via `background_task` config)
- Polling: 3s interval, completion via idle events + stability detection (10s unchanged)
- Circuit breaker support
- Spawner subdirectory (8 files) for session creation and prompt injection

**Tool Restrictions by Agent:**

| Agent | Denied Tools |
|-------|-------------|
| Oracle | write, edit, task, call_omo_agent |
| Librarian | write, edit, task, call_omo_agent |
| Explore | write, edit, task, call_omo_agent |
| Multimodal-Looker | ALL except read |
| Atlas | task, call_omo_agent |
| Momus | write, edit, task |

**Files Analyzed:**
- `src/agents/sisyphus.ts` — main orchestrator with Intent Gate (559 LOC)
- `src/agents/atlas/index.ts` — todo-list orchestrator
- `src/agents/prometheus/index.ts` — strategic planner
- `src/features/background-agent/manager.ts` — BackgroundManager (2208 lines)
- `src/tools/delegate-task/executor.ts` — delegation executor
- `src/tools/delegate-task/builtin-categories.ts` — 8 delegation categories

**ACE Relevance:** The IntentGate → Planner → Executor chain is a proven pattern. ACE should implement explicit intent classification before task routing. The BackgroundManager concurrency model (5 per model) is a good default ceiling.

---

### 1.4 Hermes Agent (nousresearch/hermes-agent) — Isolated Subagent Context with Depth Limits

**Architecture Type:** Thread-based isolated subagent spawning with tool restriction

Hermes Agent implements delegation via `delegate_tool.py`, which spawns child AIAgent instances in threads with isolated contexts and restricted toolsets.

**Key Characteristics:**
- **Isolation:** Each child gets a fresh conversation (no parent history), own task_id, own terminal session, restricted toolsets
- **Blocked Tools (never given to children):** `delegate_task` (no recursive delegation), `clarify`, `memory` (no shared MEMORY.md), `send_message`, `execute_code`
- **Default Toolsets:** terminal, file, web (parent's MCP toolsets optionally inherited)
- **Max Depth:** Default 1 (flat: parent → child), configurable to 2 or 3 via `max_spawn_depth`
- **Roles:** `leaf` (cannot delegate) vs `orchestrator` (can delegate further, only at depth < max_spawn_depth)
- **Concurrency:** ThreadPoolExecutor with configurable max concurrent children (default 3, config: `delegation.max_concurrent_children`)
- **Timeout:** Default 300s per child, configurable via `delegation.child_timeout_seconds`
- **Heartbeat:** Parent activity heartbeat during delegation to prevent gateway inactivity timeout
- **Child Toolset Intersection:** Child toolsets intersected with parent's — subagent never gains tools parent lacks
- **Orchestrator Kill Switch:** Global `orchestrator_enabled` config flag disables role="orchestrator" entirely
- **Progress Callback:** Relays child tool calls to parent display (tree-view in CLI, batched to gateway)
- **Result Summary:** Parent sees only the delegation call + summary result, never child's intermediate tool calls

**Files Analyzed:**
- `tools/delegate_tool.py` (2164 lines) — the full delegation implementation

**ACE Relevance:** Hermes's tool blocking list is essential reading for any delegation system. The heartbeat mechanism for preventing inactivity timeouts is a critical implementation detail. ACE should adopt similar isolation boundaries.

---

### 1.5 OpenClaw (openclaw) — Gateway Single-Agent-per-Session Architecture

**Architecture Type:** Gateway-mediated session spawning with lifecycle hooks

OpenClaw implements subagent support through the `sessions_spawn` tool. The gateway is fundamentally single-agent-per-session, but subagents are tracked with `spawnedBy` lineage and lifecycle hooks.

**Key Characteristics:**
- **Single-Agent-per-Session:** Each session runs one agent; subagents are separate sessions linked by `spawnedBy`
- **Session Key Format:** `agent:{agentId}:subagent:{uuid}` with nested subagent keys: `agent:main:subagent:parent:subagent:child`
- **SpawnedBy Tracking:** Every subagent session stores which session spawned it
- **Depth Limits:** `maxSpawnDepth` (default configurable), checked against stored `spawnDepth`
- **Child Limits:** `maxChildrenPerAgent` — limits active children per session
- **Lifecycle Hooks:**
  - `subagent_spawning` — thread binding preparation
  - `subagent_spawned` — post-spawn notification
  - `subagent_ended` — cleanup, farewell message
- **Thread Binding:** Subagents can be bound to messaging threads (Discord, Slack, etc.)
- **Sandbox Inheritance:** Subagent sandbox mode inherits from parent or requires specific runtime
- **ACL:** `allowAgents` list restricts which agentIds a session can spawn
- **Cleanup Modes:** `delete` (remove all traces) vs `keep` (retain transcript)
- **Mode Options:** `run` (ephemeral) vs `session` (persistent, thread-bound)
- **Workspace Inheritance:** Subagent inherits or overrides parent's workspace directory

**Files Analyzed:**
- `src/agents/subagent-spawn.ts` (942 lines) — main spawn logic
- `src/agents/subagent-spawn.depth-limits.test.ts` (180 lines) — depth limit tests
- `src/agents/tools/sessions-spawn-tool.ts` — spawn tool implementation
- `src/auto-reply/reply/commands-subagents/action-spawn.ts` — slash command spawn

**ACE Relevance:** OpenClaw's lifecycle hook system (`subagent_spawning`, `subagent_spawned`, `subagent_ended`) provides a clean extensibility model. The `spawnedBy` tracking enables parent-child session graphs.

---

### 1.6 Goose (aaif-goose/goose) — Subagent Handler with Extensions

**Architecture Type:** Rust-based subagent handler with extension loading

Goose implements subagent delegation via `subagent_handler.rs`, which constructs an Agent with task-specific configuration, applies recipe components, and streams messages back to the parent.

**Key Characteristics:**
- **SubagentPromptContext:** Template variables include `max_turns`, `subagent_id`, `task_instructions`, `tool_count`, `available_tools`
- **Extension Loading:** Subagents can have extensions added from task config
- **Tool Visibility:** `is_tool_visible_to_model` filters tools per subagent
- **Provider Override:** Subagent can use different provider than parent
- **Message Notifications:** Tool calls from subagents are forwarded via MCP `LoggingMessageNotification` with `subagent_id`
- **Session Config:** `max_turns`, `schedule_id`, `retry_config` per subagent
- **Conversation Streaming:** Subagent streams messages back via `AgentEvent::Message`
- **Response Extraction:** `return_last_only` flag controls whether parent sees all messages or just final output

**Files Analyzed:**
- `crates/goose/src/agents/subagent_handler.rs` (346 lines)
- `crates/goose/src/prompt_template.rs` — references `subagent_system.md` template

**ACE Relevance:** Goose's extension loading pattern for subagents is worth studying. The tool visibility filter is a clean mechanism for per-subagent tool restriction.

---

### 1.7 Autoresearch (karpathy/autoresearch) — Minimal Single-Agent Loop

**Architecture Type:** No delegation — single agent experiment loop

Autoresearch has no multi-agent delegation. The 630-line experiment loop (`program.md`, `train.py`, `prepare.py`) is a single-agent system with no spawning, no subagents, and no worker pools.

**ACE Relevance:** Autoresearch is the null case — not every agent system needs delegation. ACE should consider whether simple single-agent loops are sufficient for certain task types.

---

## 2. Cross-Cutting Comparison

### 2.1 Hierarchy Models

| System | Model | Depth | Worker Count |
|--------|-------|-------|-------------|
| Devin | Manager/Worker (VM-isolated) | 2 (manager + 10 workers) | Up to 10 |
| OpenCode | Single-registry + session tree | Configurable | Per-session limit |
| Oh My OpenAgent | Intent Gate → Planner → Executor → Workers | 3-4 | 5 per model (background) |
| Hermes Agent | Parent → Child (thread) | Max 3 (configurable) | 3 concurrent (configurable) |
| OpenClaw | Gateway session tree | Configurable maxSpawnDepth | Per-session limit |
| Goose | Handler → Agent stream | 1 (no nesting) | Not specified |
| Autoresearch | None | 0 | 0 |

### 2.2 Context Isolation Approaches

| System | Isolation Model | Parent Context Visible to Child |
|--------|----------------|-------------------------------|
| Devin | Full VM isolation | No (clean slate per worker) |
| OpenCode | Child session, parent spawns | No intermediate visibility |
| Oh My OpenAgent | Background session isolation | Session-based, polling for results |
| Hermes Agent | Fresh conversation, no parent history | Never intermediate calls |
| OpenClaw | Separate session with spawnedBy link | Via lifecycle hooks only |
| Goose | Fresh conversation, tool filtering | Tool notifications only |
| Autoresearch | N/A | N/A |

### 2.3 Parallel Execution Patterns

| System | Parallelism | Concurrency Control |
|--------|-------------|-------------------|
| Devin | Up to 10 workers in parallel | Manager coordinates |
| Oh My OpenAgent | BackgroundManager (5/model) + category parallelization | ConcurrencyManager FIFO |
| Hermes Agent | ThreadPoolExecutor (max 3 concurrent) | Configurable max_concurrent_children |
| OpenCode | Session spawning | maxChildrenPerAgent |
| Goose | Streaming messages | Not specified |
| OpenClaw | Multiple active subagent sessions | Per-session child limit |
| Autoresearch | Sequential experiment loop | Fixed 5-min budget |

### 2.4 Conflict Resolution

| System | Resolution Strategy |
|--------|-------------------|
| Devin | Manager reads full worker trajectories, identifies conflicts, merges into single PR |
| OpenCode | No automatic resolution — user navigates sessions |
| Oh My OpenAgent | Sisyphus (orchestrator) synthesizes results; Atlas verifies completion |
| Hermes Agent | No automatic resolution — parent gets summary |
| OpenClaw | No automatic resolution — lifecycle hooks for manual intervention |
| Goose | Tool notifications forwarded; parent decides |

### 2.5 Depth Limits

| System | Default Max Depth | Configurable | Hard Cap |
|--------|-------------------|--------------|----------|
| Devin | 2 (manager + workers) | Not stated | 10 workers |
| OpenCode | Configurable | Yes | Not stated |
| Oh My OpenAgent | Inherent chain (3-4 layers) | Via config | Concurrency cap |
| Hermes Agent | 1 (flat) | Yes (max 3) | 3 |
| OpenClaw | Configurable | Yes | Not stated |
| Goose | 1 (no nesting) | N/A | N/A |
| Autoresearch | 0 | N/A | N/A |

### 2.6 Role Specialization

| System | Specialized Roles | Verification |
|--------|------------------|--------------|
| Devin | Manager (coordinator), Worker (executor) | Per-worker test verification |
| OpenCode | Build (full), Plan (read-only), General (orchestrator), Explore (read-only) | Permission system |
| Oh My OpenAgent | Sisyphus (orchestrator), Hephaestus (worker), Oracle (consultant), etc. | Tool restrictions per agent |
| Hermes Agent | Leaf (cannot delegate), Orchestrator (can delegate) | Toolset stripping |
| OpenClaw | Role determined by spawn mode + capabilities | Spawn depth/capability checks |
| Goose | Task-specific via recipe configuration | Tool visibility filtering |

### 2.7 Verification Patterns

| System | Verification | Completion Signal |
|--------|--------------|-------------------|
| Devin | Each worker verifies own changes by running tests | Manager merges verified changes |
| Oh My OpenAgent | Atlas verifies completion; Sisyphus synthesizes | Session idle + 10s stability |
| Hermes Agent | Child runs to completion or timeout | Final response summary |
| OpenClaw | Lifecycle hooks; user steers | Session status |
| OpenCode | User reviews child session | Task completion |

---

## 3. Key Implementation Patterns

### 3.1 Delegation Tool Pattern (Hermes Agent)

```
Parent calls delegate_task(goal, context?, toolsets?, role?)
  → _build_child_agent() constructs child AIAgent
  → ThreadPoolExecutor runs child.run_conversation()
  → Parent blocks until all children complete
  → Child summary returned to parent
```

Blocked tools are stripped from child: `delegate_task`, `clarify`, `memory`, `send_message`, `execute_code`.

### 3.2 Background Task Pattern (Oh My OpenAgent)

```
task() tool → BackgroundManager.launch()
  → ConcurrencyManager.acquire_slot()
  → Spawner creates session + injects prompt
  → Polling at 3s intervals
  → Completion: idle event + 10s stability detection
  → Result injected into parent session
```

### 3.3 Session Tree Pattern (OpenClaw)

```
sessions_spawn tool
  → Gateway creates child session key: agent:{target}:subagent:{uuid}
  → spawnDepth, spawnedBy stored in session store
  → Lifecycle hooks: subagent_spawning → subagent_spawned → subagent_ended
  → Parent tracks via spawnedBy lineage
```

### 3.4 Intent Gate Pattern (Oh My OpenAgent)

```
User message → IntentGate classifies:
  research → explore/librarian → synthesize → answer
  implementation → plan → delegate or execute
  investigation → explore → report findings
  evaluation → evaluate → propose → wait for confirmation
  fix → diagnose → fix minimally
```

---

## 4. ACE Recommendation Table

| Pattern | System | Recommendation | Rationale |
|---------|--------|---------------|----------|
| Manager/Worker hierarchy with VM isolation | Devin | **ADOPT** | Fault containment, parallel execution, clean result synthesis. ACE pods should map to this model. |
| Intent classification before routing | Oh My OpenAgent | **ADOPT** | Prevents misclassification errors. ACE should implement intent detection as first-class operation. |
| `@mention` subagent invocation | OpenCode | **ADOPT** | Clean, user-visible invocation pattern. Natural language routing. |
| Permission-filtered agent modes | OpenCode | **ADOPT** | Essential for安全. Build (full) vs Plan (restricted) is the right model. |
| Max depth limit with leaf/orchestrator roles | Hermes Agent | **ADOPT** | Prevents runaway delegation. ACE should enforce depth limits with `role=leaf` for constrained tasks. |
| Tool blocking list for children | Hermes Agent | **ADOPT** | `delegate_task`, `clarify`, `memory`, `send_message` blocked from children is correct. ACE should maintain this list. |
| BackgroundManager concurrency per model | Oh My OpenAgent | **ADOPT** | 5 concurrent per model/provider is a well-tested ceiling. ACE should implement per-provider concurrency limits. |
| Lifecycle hooks (spawn, spawned, ended) | OpenClaw | **ADOPT** | Clean extensibility for observability and control. ACE should implement `pod_spawning`, `pod_spawned`, `pod_ended`. |
| spawnedBy tracking for session lineage | OpenClaw | **ADOPT** | Essential for parent-child session graphs, cleanup, and human navigation. ACE should track `parentPodId`. |
| Polling-based result fetching | Oh My OpenAgent | **ADOPT** | Robust completion detection: idle event + stability period. ACE should not rely on single signals. |
| Heartbeat during delegation | Hermes Agent | **ADOPT** | Critical for preventing gateway/manager timeouts during long-running subagent tasks. |
| Progress callback relay to parent | Hermes Agent | **ADOPT** | Tree-view display of child activity is essential for observability. ACE should implement pod progress streaming. |
| Sibling write detection (file_state) | Hermes Agent | **ADAPT** | Useful warning pattern — parent re-reads files modified by children. ACE should implement similar cross-pod file change detection. |
| Automatic task decomposition | Devin | **ADAPT** | Not all systems can reliably decompose tasks. ACE should implement this only for specific task types (migration, lint, refactor). |
| Orchestrator kill switch | Hermes Agent | **ADOPT** | Global `orchestrator_enabled` flag allows disabling nested delegation without code changes. ACE needs this. |
| Thread binding for subagent sessions | OpenClaw | **ADAPT** | Useful for messaging integration but complex. ACE should consider for future messaging surfaces. |
| Single-agent-per-session gateway | OpenClaw | **AVOID** | Too restrictive. ACE pods should support multiple agents per session where appropriate. |
| No delegation (autoresearch) | Autoresearch | **ADOPT** as option | Not every task needs delegation. Simple single-agent loops are valid for experimental/research tasks. ACE should support both modes. |
| VM-level isolation per worker | Devin | **ADAPT** | Full VM isolation is heavyweight for most tasks. ACE pods should use namespace/cgroup isolation as the common case, full VMs only for high-risk tasks. |

---

## 5. ACE Pod Tree Topology — Recommended Design

Based on the cross-system analysis, ACE should implement:

### 5.1 Hierarchy Model
```
Manager Pod (root)
  ├── Worker Pod (depth 1, leaf by default)
  ├── Worker Pod (depth 1, leaf by default)
  └── Worker Pod (depth 1, leaf by default)
       └── Sub-Worker Pod (depth 2, orchestrator if depth >= 2)
```

### 5.2 Core Properties
- **Depth Limit:** Default 2, configurable to 3
- **Concurrency:** 5 workers per pod tree (matching Oh My OpenAgent's proven default)
- **Isolation:** Namespace-level by default, full VM isolation as opt-in for high-risk tasks
- **Role Model:** `leaf` (cannot spawn) vs `orchestrator` (can spawn if depth < max)
- **Parent Context:** Fresh conversation per pod, no intermediate visibility

### 5.3 Required ACE Implementations
1. **Tool Blocking List:** `delegate_task`, `clarify`, `memory`, `send_message`, `execute_code` blocked from leaf pods
2. **Lifecycle Events:** `pod_spawning`, `pod_spawned`, `pod_ended` hooks
3. **Progress Streaming:** Pod activity relayed to parent in real-time
4. **Heartbeat:** Parent heartbeat during long-running pod tasks
5. **Completion Detection:** Dual-signal (idle + stability period)
6. **spawnedBy Tracking:** Full parent-child lineage graph
7. **Concurrency Manager:** FIFO queue per provider/model with configurable limits
8. **Intent Classification:** First-pass intent detection before task routing

---

## 6. Research Gaps & Future Investigation

1. **Conflict Resolution:** No system has a fully automated conflict resolution strategy. Devin's "manager reads trajectories and merges" is the most sophisticated approach but still requires manager judgment.

2. **Cross-Pod File State:** Hermes Agent's `file_state.writes_since()` pattern for sibling write detection is promising but not well-tested across VM boundaries.

3. **Nested Orchestrator Stability:** Hermes allows depth 3 with nested orchestrators; production stability of depth-3 chains is unknown.

4. **Devin VM Allocation:** The mechanism for VM provisioning and cleanup in Devin's managed Devins is not publicly documented.

---

**Files Reviewed:**
- `hermes-agent/tools/delegate_tool.py` — full delegation implementation
- `openclaw/src/agents/subagent-spawn.ts` — session-based spawning
- `oh-my-openagent/src/agents/sisyphus.ts` — orchestrator with Intent Gate
- `oh-my-openagent/src/features/background-agent/manager.ts` — BackgroundManager
- `goose/crates/goose/src/agents/subagent_handler.rs` — Rust subagent handler
- `opencode/packages/web/src/content/docs/agents.mdx` — agent configuration

**Web Sources:**
- [Devin can now Manage Devins](https://www.cognition-labs.com/blog/devin-can-now-manage-devins)
- [Devin September 2024 Update — MultiDevin](http://www.cognition-labs.com/blog/sept-24-product-update)
- [Orchestrating 10 AI Agents: Patterns That Actually Work](https://dev.to/toji_openclaw_fd3ff67586a/orchestrating-10-ai-agents-patterns-that-actually-work-23bm)
