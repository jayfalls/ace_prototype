# Slice 12: Comparative Strengths & Weaknesses

**Unit:** agents-study  
**Slice:** 12 of 14  
**Date:** 2026-04-23  
**Status:** Complete

---

## 1. Introduction

This document synthesizes findings from all 11 prior dimensional slices (architecture, memory, compaction, loops, delegation, tools-skills, browser-automation, computer-use, communication, self-improvement, ux-dx) into comparative strengths and weaknesses with ACE decision mapping.

**Synthesis Method:**
- Per-system, per-dimension evidence extracted from each slice
- Cross-system patterns identified (convergent evolution)
- Unique innovations flagged
- Contradictions between systems documented
- Every finding mapped to ACE: **ADOPT**, **ADAPT**, or **AVOID**

**Key Assumptions:**
- ACE is a local-first, single-binary system with optional cloud deployment
- ACE targets developers and technical users
- ACE prioritizes safety, transparency, and debuggability
- ACE uses NATS as its message bus (already decided)

---

## 2. Per-System Strengths & Weaknesses Tables

### 2.1 OpenClaw

| Strength | Dimension | Evidence | ACE Decision |
|----------|-----------|----------|--------------|
| Hub-and-spoke gateway (24+ channel adapters) | architecture | `src/gateway/server.impl.ts` — single Gateway normalizes all channels | ADOPT (design pattern; AVOID single-process bottleneck via NATS) |
| Session key model for multi-tenant isolation | architecture | `sessionKey` encodes agent identity and workspace scope | ADOPT (ACE pods need session scoping) |
| Plugin manifest + SDK separation | architecture | Core must not import extensions; SDK is only seam | ADOPT (ACE plugin SDK design) |
| Channel adapter normalization | architecture | `normalizeChannelId` resolves aliases to canonical IDs | ADOPT (ACE NATS subject normalization) |
| Auto-discovery tool registry via manifest | tools-skills | Skills declared in `openclaw.plugin.json` `skills` field | ADOPT (ACE skill discovery) |
| SKILL.md packaging with security scanning | tools-skills | `scanSkillInstallSource()` + safe regex validation | ADOPT (ACE skill quality gate) |
| QA Lab CDP-based browser control | browser-automation | Full browser control via `browserAct` with 20+ action kinds | ADAPT (complex for single-binary; lightweight CDP wrapper if needed) |
| SSRF protection + secret comparison | browser-automation | `ssrf-policy-helpers.ts`, `safeEqualSecret()` | ADOPT (ACE security fundamentals) |
| HEARTBEAT.md task checklists | self-improvement | Scheduled wake-ups with task mode, skip on empty | ADAPT (heartbeat-driven deferred task pattern) |
| Cron scheduling with deduplication | self-improvement | Named cron jobs with run logs, delivery status | ADOPT (background task scheduling) |
| Hub broadcast with scope guards | communication | `READ_SCOPE`, `WRITE_SCOPE`, `ADMIN_SCOPE` per connection | ADOPT (ACE per-user auth on NATS broadcast) |
| Lifecycle hooks (subagent_spawn/ended) | delegation | `subagent_spawning`, `subagent_spawned`, `subagent_ended` | ADOPT (ACE pod lifecycle events) |
| spawnedBy tracking for session lineage | delegation | `sessionKey` hierarchy with `spawnDepth` | ADOPT (ACE parent pod tracking) |
| Interactive onboard wizard | ux-dx | `openclaw onboard` step-by-step gateway setup | ADOPT (ACE guided setup) |
| Doctor command for config diagnostics | ux-dx | `openclaw doctor` surfaces risky DM policies | ADOPT (ACE diagnostics) |
| Project-local workspace files (SOUL.md, AGENTS.md) | ux-dx | Injected as prompt files at workspace root | ADOPT (ACE project context files) |

| Weakness | Dimension | Evidence | ACE Decision |
|----------|-----------|----------|--------------|
| Single Gateway process bottleneck | architecture | Single daemon handling all 24+ channels; acknowledged restart policies | AVOID (NATS bus avoids single-process bottleneck) |
| Unbounded file memory storage | memory | No compaction mechanism; grows indefinitely | AVOID (ACE needs bounded memory with consolidation) |
| Gateway single-agent-per-session | delegation | Each session runs one agent; subagents tracked via spawnedBy | AVOID (ACE pods should support multi-agent sessions) |
| Cisco security report: malware in ClawHub | tools-skills | Cisco assessment found significant malware rate in community marketplace | AVOID (community marketplace without mandatory scanning) |
| Blocking user-facing compaction UI | compaction | Compaction events block with user-visible indicators | AVOID (background compaction preferred) |
| Heavyweight compile-time feature gating | ux-dx | 108 modules dead-code-eliminated via `feature()` intrinsic | AVOID (runtime feature flags more debuggable) |
| Mandatory DM pairing/allowlist friction | ux-dx | Security-default for messaging is right but high friction | ADAPT (opt-out more discoverable) |
| Adversary inspector: computercontroller denylist | computer-use | `computercontroller__automation_script` blocked by default | ADOPT (computer automation tools need opt-in) |

---

### 2.2 Claude Code

| Strength | Dimension | Evidence | ACE Decision |
|----------|-----------|----------|--------------|
| AsyncGenerator streaming + concurrent execution | loops | `src/query.ts` — streams output while executing tools speculatively | ADOPT (ACE agent loop must support concurrent tool dispatch) |
| 4-layer graduated compaction (snip/microcompact/collapse/autocompact) | compaction | `compact.ts`, `autoCompact.ts`, `microCompact.ts`, `sessionMemoryCompact.ts` | ADOPT (multi-layer graduated response) |
| Token budget tracking across compaction boundaries | loops | `budgetTracker` carries token budget across context boundaries | ADOPT (maintain execution coherence) |
| Speculative bash permission pre-checking | loops | `bashPermissions.ts` — `speculativeChecks` pre-compute permission states | ADOPT (avoids mid-loop interruptions) |
| Two-phase permission filtering | tools-skills | Tools filtered before model sees them; plan mode restrictions | ADOPT (security essential) |
| SKILL.md + progressive disclosure | tools-skills | Tier 1: name/description in system prompt (~97 chars/skill); Tier 2: full SKILL.md | ADOPT (token-efficient skill loading) |
| Typed SDK events (AsyncGenerator streaming) | ux-dx | `SDKMessage` union type with 10+ event variants | ADOPT (headless embedding without protocol guessing) |
| 7-layer memory architecture | memory | Per-system reporting: 4 memory types, MEM.md, LLM-powered recall | ADOPT (multi-tier memory taxonomy) |
| Memory drift caveat + staleness warnings | memory | `MEMORY_DRIFT_CAVEAT` — verify memories against current state | ADOPT (stale memory handling) |
| Compact boundary events with preCompactDiscoveredTools | compaction | `compact_boundary` event carries discovered tool state | ADOPT (enables SDK state tracking) |
| Frontmatter metadata on memory files | memory | `name`, `description`, `type` fields per memory entry | ADOPT (structured, machine-parseable) |
| Post-compact token budgets (50K files, 5K/file) | compaction | `POST_COMPACT_TOKEN_BUDGET = 50_000`, `MAX_TOKENS_PER_FILE = 5_000` | ADOPT (explicit restoration budgets) |
| Project-local CLAUDE.md | ux-dx | Loaded per-directory for project-specific context | ADOPT (ACE project context files) |
| Claude Code enigo + TCC stack (reconstruction) | computer-use | Rust/enigo + Swift/TCC for cross-platform input simulation | ADOPT (research reference for ACE input automation) |
| Screenshot clipboard integration | browser-automation | `screenshotClipboard.ts` for user-to-agent image sharing | ADOPT (for information retrieval use cases) |

| Weakness | Dimension | Evidence | ACE Decision |
|----------|-----------|----------|--------------|
| 108 compile-time DCE modules | ux-dx | `feature()` intrinsic removes 108 modules at compile time | AVOID (runtime feature flags more debuggable) |
| Monolithic 685KB single file (query.ts) | architecture | Single file contains entire agent loop | AVOID (ACE should modularize) |
| No sandbox (host reliance) | tools-skills | Tool execution relies on host environment security | ADAPT (container isolation for untrusted skills) |
| CLI-only computer use (macOS) | computer-use | `@ant/computer-use-input` (Rust/enigo) only on macOS | AVOID (cross-platform needed) |
| Undercover auto-stripping AI attribution | ux-dx | Auto-removes AI attribution in public repos | AVOID (transparency issues; should be explicit opt-in) |
| Telemetry: two analytics sinks (1P + Datadog) | ux-dx | Environment fingerprint, no UI-exposed opt-out | AVOID (privacy concerns for ACE) |
| Closed-source proprietary | architecture | Original source not publicly available | AVOID (ACE must be open source) |

---

### 2.3 Open Code

| Strength | Dimension | Evidence | ACE Decision |
|----------|-----------|----------|--------------|
| Client/server architecture with remote access | architecture | WebSocket-based TUI-to-agent communication; multiple clients | ADOPT (ACE remote driving capability) |
| Multi-package monorepo (surface separation) | architecture | `packages/{app,console,desktop,sdk,web}` separate deployment targets | ADOPT (clean surface separation) |
| @mention subagent invocation pattern | delegation | `@general help me search...` triggers subagent with own session | ADOPT (natural language routing) |
| Permission rule sets (allow/ask/deny) per mode | delegation | `edit`, `bash`, `webfetch` configurable per mode (plan/build) | ADOPT (mode-based capability filtering) |
| Permission filtering before model sees tools | tools-skills | Two-phase: filter then permission check | ADOPT (security essential) |
| Plugin hook system for compaction | compaction | `experimental.session.compacting` hook for domain context injection | ADOPT (extensibility) |
| Typed event bus with Zod schemas | communication | `BusEvent.define(name, schema)` — typed payloads validated | ADOPT (ACE typed NATS subjects) |
| LSP support for IDE-precision refactoring | ux-dx | Out-of-the-box language server protocol integration | ADOPT (developer tooling) |
| JSON/JSONC config with project-local override | ux-dx | `.opencode/` directory with CLAUDE.md, AGENTS.md | ADOPT (ACE project config hierarchy) |
| Session persistence (session.json) | architecture | Simple JSON-based session state at root | ADAPT (more sophisticated state needed for ACE) |

| Weakness | Dimension | Evidence | ACE Decision |
|----------|-----------|----------|--------------|
| In-process PubSub only (no distribution) | communication | `Effect.PubSub` is in-process; limits to single-machine | AVOID (ACE needs NATS for distribution) |
| No browser control (webfetch/websearch only) | browser-automation | No DOM interaction, no screenshots | ADOPT (webfetch/websearch baseline is appropriate) |
| Desktop app is deployment surface, not automation | computer-use | Electron/Tauri wrapper; no GUI control primitives | AVOID (not a model for ACE automation) |
| No native sandbox for tools | tools-skills | Relies entirely on host security | ADAPT (container isolation needed) |
| No multi-agent coordination beyond @general | delegation | Limited delegation model; single primary agent | AVOID (ACE needs structured pod hierarchy) |

---

### 2.4 Oh My OpenAgent

| Strength | Dimension | Evidence | ACE Decision |
|----------|-----------|----------|--------------|
| Intent Gate pre-classification | delegation | Classifies: research/implementation/investigation/evaluation/fix | ADOPT (intent-aware routing) |
| Intent Gate → Prometheus → Atlas → Workers chain | delegation | 3-layer orchestration with clear separation | ADOPT (planner/executor separation) |
| 11 specialized agents with tool restrictions | delegation | Oracle (read-only), Librarian, Explore, etc. | ADOPT (role specialization) |
| BackgroundManager concurrency (5/model) | delegation | `ConcurrencyManager` FIFO, 3s polling, idle+10s stability | ADOPT (per-provider concurrency limits) |
| 3-tier MCP system (built-in/config/skill-embedded) | tools-skills | `SkillMcpManager` handles stdio + HTTP + OAuth per session | ADOPT (MCP tiered architecture) |
| Skill-embedded MCP in YAML frontmatter | tools-skills | Skills declare MCP servers in SKILL.md | ADOPT (self-contained skill packages) |
| 52 lifecycle hooks across 5 tiers | communication | Session(24) + ToolGuard(14) + Transform(5) + Continuation(7) + Skill(2) | ADAPT (52 hooks excessive; 5-10 well-chosen) |
| Multi-model orchestration | architecture | Claude/Kimi/GLM/GPT/Minimax/Gemini routing | ADOPT (multi-provider support) |
| Factory pattern for tools | tools-skills | `createToolRegistry()` + `createXXXTool()` factories | ADOPT (consistent tool creation) |
| JSONC config with multi-level override | ux-dx | Project → user → defaults with deep merge | ADOPT (ACE config hierarchy) |
| Opinionated zero-config defaults | ux-dx | "Install. Type `ultrawork`. Done." | ADOPT (minimal time-to-first-result) |
| Doctor command | ux-dx | `bunx oh-my-opencode doctor` verifies plugin registration | ADOPT (pre-flight diagnostics) |
| Playwright MCP integration | browser-automation | `@anthropic-ai/mcp-playwright` as built-in MCP | ADOPT (standardized browser control) |
| Annotated screenshots with numbered overlays | browser-automation | `agent-browser screenshot --annotate` overlays [N] labels | ADOPT (element targeting precision) |
| Structured handoff bundle (Design-to-Code) | computer-use | Machine-readable spec not PNG/Figma URL | ADAPT (design pipeline; not automation priority) |

| Weakness | Dimension | Evidence | ACE Decision |
|----------|-----------|----------|--------------|
| Runs within Claude Code process (plugin) | architecture | Requires Claude Code host; not standalone | AVOID (ACE must be independent) |
| 52 hooks + 160k+ LOC complexity | communication | Hooks share mutable ctx; tight coupling | ADAPT (typed NATS subjects instead of shared state) |
| No hard depth limit on delegation | delegation | Inherent chain limits concurrency, not depth | ADAPT (depth limit needed) |
| Windows support incomplete | architecture | Some features not available on Windows | ADAPT (target priority: macOS/Linux first) |

---

### 2.5 Devin

| Strength | Dimension | Evidence | ACE Decision |
|----------|-----------|----------|--------------|
| Manager/worker hierarchy (1 + 10 workers) | delegation | Coordinator scopes, assigns, monitors, resolves, compiles | ADOPT (ACE pod tree topology) |
| VM-level isolation per worker | delegation | Each worker in own VM with own terminal/browser/editor | ADAPT (namespace isolation default; full VM opt-in) |
| Playbook system for recurring tasks | self-improvement | Reusable documents refined from session outcomes | ADAPT (versioned procedure docs) |
| Knowledge base with auto-suggest | self-improvement | Cross-session tips, deduplication, conflict resolution | ADAPT (organizational knowledge layer) |
| Session outcome analysis | self-improvement | Pattern extraction from success/failure | ADAPT (automated quality evaluation) |
| WebSocket brain↔devbox communication | communication | Stateless Brain + isolated DevBox; WebSocket over HTTPS/443 | ADOPT (brain/devbox separation principle) |
| VNC continuous stream for browser | browser-automation | <50ms latency streaming; auto-zoom, annotation | ADAPT (for browser streaming; not full VM) |
| MultiDevin orchestration (10 workers) | delegation | Parallel managed Devins, message children mid-task | ADOPT (parallel pod execution) |
| ACU consumption monitoring | ux-dx | Per-worker compute tracking | ADOPT (resource monitoring for ACE pods) |
| Design system auto-extraction | computer-use | Reads code/Figma/fonts/GitHub to build design system | ADAPT (UI generation potential; not priority) |
| Structured spec handoff bundle | computer-use | Component structure, tokens, layout as machine-readable spec | ADAPT (design-to-code pipeline; long-horizon) |

| Weakness | Dimension | Evidence | ACE Decision |
|----------|-----------|----------|--------------|
| Cloud-only deployment (Azure-hosted Brain) | architecture | Brain in Cognition tenant; VPC option for enterprise | AVOID (local-first ACE) |
| Full VM isolation per worker (expensive) | delegation | Hypervisor-level isolation; not process-level | AVOID (namespace/cgroup isolation sufficient) |
| No open-source release | architecture | Proprietary closed-source product | AVOID (ACE must be open source) |
| Proprietary UX (web dashboard) | ux-dx | Cloud VM interface, manager dashboard, onboarding | AVOID (can't replicate; must design openly) |
| Cisco security: no visibility into Brain | security | Stateless brain containers, secrets at start | ADAPT (transparency requirements) |
| Setup cost ($500/month) | ux-dx | Cloud SaaS pricing | AVOID (ACE must be affordable/self-hosted) |

---

### 2.6 Hermes Agent

| Strength | Dimension | Evidence | ACE Decision |
|----------|-----------|----------|--------------|
| Auto-discovery tool registry at import | tools-skills | `tools/registry.py` — any `tools/*.py` calls `registry.register()` | ADOPT (zero-configuration extension loading) |
| 47 tools across 19 toolsets | tools-skills | `handle_function_call()` dispatches; `discover_builtin_tools()` | ADOPT (comprehensive built-in toolkit) |
| Tool blocking list for children | delegation | `delegate_task`, `clarify`, `memory`, `send_message` blocked | ADOPT (correct security boundaries) |
| ThreadPoolExecutor concurrent tool execution | loops | Path-overlap detection; max 8 workers | ADOPT (safe concurrent dispatch) |
| Batched tool execution (no ReAct per-call) | loops | Collect all tool results, then reason once | ADOPT (reduced latency for tool-rich tasks) |
| Depth-2 limit with leaf/orchestrator roles | delegation | `max_spawn_depth=2`; `role=leaf` vs `orchestrator` | ADOPT (prevents runaway delegation) |
| Heartbeat during delegation | delegation | Parent activity heartbeat prevents gateway timeout | ADOPT (long-running task safety) |
| SQLite + FTS5 session search | memory | BM25 ranking, phrase/proximity/boolean queries | ADOPT (zero-dependency full-text search) |
| Frozen snapshot for system prompt | memory | `load_from_disk()` captures snapshot at session start | ADOPT (prefix cache stability) |
| Atomic file writes (temp + rename) | memory | `os.replace()` for atomicity; `.lock` file for reads | ADOPT (concurrent access safety) |
| Character-limited bounded memory | memory | 2200 chars (memory), 1375 chars (user) hard cap | ADOPT (prevents unbounded growth) |
| LLM-powered session summarization | memory | Gemini Flash summarization for top-k sessions | ADAPT (expensive; use sparingly) |
| Dual-tier compression (85%/50%) | compaction | Gateway safety net + agent primary compressor | ADOPT (catches what primary misses) |
| Iterative re-compression | compaction | Previous summary passed to LLM for UPDATE not from-scratch | ADOPT (preserves context across cycles) |
| Structured summary template | compaction | Goal / Constraints / Progress / Done-InProgress-Blocked / Next Steps | ADOPT (consistent actionable format) |
| Anthropic prompt caching integration | compaction | `system_and_3` breakpoints; rolling 3-message window | ADOPT (75% cost reduction) |
| Ephemeral prompt injection | compaction | `HERMES_PREFILL_MESSAGES_FILE` — never persisted | ADOPT (context without pollution) |
| Gateway routes to 15+ surfaces | communication | `gateway/run.py` + `platforms/` adapter directory | ADOPT (multi-surface routing) |
| Surface-agnostic AIAgent | architecture | `run_agent.py` invoked by CLI, TUI, ACP, 15+ platforms | ADOPT (ACE core knows nothing about surfaces) |
| TUI as separate rendering process | ux-dx | TypeScript Ink/React over stdio JSON-RPC | ADOPT (separation of concerns) |
| Skin/theme engine | ux-dx | YAML skins: default gold, ares crimson, mono, slate | ADOPT (customizable visuals) |
| Skills Hub with trust tiers | tools-skills | builtin/trusted/community + quarantine + scan | ADAPT (marketplace with security) |
| 488-pattern security scanner | tools-skills | 12 categories: exfiltration, injection, destruction, persistence | ADOPT (comprehensive static analysis) |
| Security-gated atomic writes | self-improvement | Scan → rollback chain prevents corrupted skills | ADOPT (safe self-modification) |
| Trajectory capture → skill authoring | self-improvement | `trajectory.py` + `skill_manager_tool.py` | ADOPT (passive capture pipeline) |
| Screenshot clipboard integration | browser-automation | User-to-agent image sharing | ADOPT (appropriate for information retrieval) |

| Weakness | Dimension | Evidence | ACE Decision |
|----------|-----------|----------|--------------|
| Python GIL limits true parallelism | architecture | Synchronous `run_agent.py` loop | ADAPT (ACE Go runtime avoids this) |
| Heavyweight Python setup wizard | ux-dx | 3,100-line `setup.py` interactive wizard | ADAPT (split into quick-start vs full config) |
| ThreadPoolExecutor blocks on I/O | loops | Synchronous tool execution despite concurrency | ADAPT (async I/O preferred) |
| No native sandbox (relies on host) | tools-skills | Terminal tool runs in host shell | ADAPT (container isolation for untrusted) |
| Trajectory → skill synthesis not automated | self-improvement | Agent must manually review and author SKILL.md | ADAPT (automated synthesis is future work) |
| Gateway process bottleneck | architecture | Single `gateway/run.py` process for 15+ surfaces | AVOID (NATS bus avoids this) |

---

### 2.7 pi-mono

| Strength | Dimension | Evidence | ACE Decision |
|----------|-----------|----------|--------------|
| Surface-agnostic core (pi-agent-core) | architecture | CLI, TUI, web, Slack all import same core | ADOPT (ACE cognitive units surface-agnostic) |
| Minimal dependencies / $5 VPS target | architecture | Minimal footprint; simple configuration | ADOPT (resource efficiency) |
| ToolDefinition registry with custom rendering | tools-skills | `render()` function per tool; schema/presentation separation | ADOPT (flexible tool presentation) |
| Platform-detecting clipboard | computer-use | pbcopy → xclip → wl-paste → native module fallback | ADOPT (cross-platform clipboard) |
| Kitty keyboard protocol (TUI) | computer-use | Key release, compose key, non-Latin layouts | ADOPT (professional terminal input) |
| File-based IPC (log.jsonl/context.jsonl) | communication | Append-only history; filesystem for agent communication | ADAPT (extreme isolation scenarios) |
| SKILL.md directory discovery | tools-skills | `loadSkillsFromDir()` with symlink/ignore support | ADOPT (filesystem-based skill discovery) |
| Session tree with branching | ux-dx | JSONL sessions in `~/.pi/agent/sessions/` with tree structure | ADOPT (session navigation) |
| Four modes (interactive/print/json/rpc) | ux-dx | `-p`, `--mode json`, `--mode rpc` for embedding | ADOPT (deployment flexibility) |
| CSI 2026 differential TUI rendering | ux-dx | Atomic updates, no flicker, three-strategy diffing | ADOPT (production-quality terminal UI) |
| Aggressively extensible philosophy | architecture | Extensions as first-class citizens | ADOPT (extension architecture) |

| Weakness | Dimension | Evidence | ACE Decision |
|----------|-----------|----------|--------------|
| No browser automation (webfetch only) | browser-automation | No DOM interaction, no Playwright | ADOPT (webfetch baseline appropriate) |
| No native desktop automation | computer-use | Terminal I/O only; no GUI control | ADAPT (not a priority for coding agent) |
| File-based IPC limits distribution | communication | `log.jsonl` requires single-machine | AVOID (ACE needs NATS for distribution) |
| No subagent spawning built-in | delegation | Single-agent loop; no multi-pod coordination | ADAPT (delegation via ACE pods) |
| No MCP support built-in | tools-skills | No Model Context Protocol integration | ADAPT (MCP is ACE standard) |

---

### 2.8 Goose

| Strength | Dimension | Evidence | ACE Decision |
|----------|-----------|----------|--------------|
| Rust-based performance + safety | architecture | Memory safety without GC; `crates/goose/` shared | ADOPT (ACE backend language choice) |
| Multi-surface (CLI/desktop/API) over shared core | architecture | `crates/goose/` shared; separate entry points | ADOPT (surface-agnostic ACE core) |
| Tokio broadcast channel + replay | communication | `SessionEventBus` with 512-event buffer + `ClientTooFarBehind` | ADOPT (JetStream durable replay) |
| Monotonic sequencing per session | communication | `SessionEvent { seq }` — ordering guarantees | ADOPT (JetStream sequence tracking) |
| MCP as first-class extension | tools-skills | `crates/goose-mcp/` — 70+ extensions | ADOPT (MCP standard for ACE) |
| Provider trait abstraction | architecture | `providers/base.rs` — 15+ providers | ADOPT (multi-provider ACE) |
| computercontroller cross-platform automation | computer-use | Peekaboo (macOS) / xdotool (X11) / wtype (Wayland) | ADOPT (OS-level automation) |
| Screenshot-first workflow (Linux) | computer-use | Agent sees screen before acting on Linux | ADOPT (coordinate error mitigation) |
| Subagent handler with extension loading | delegation | `subagent_handler.rs` — template variables, tool filtering | ADOPT (subagent configuration) |
| Tool visibility filtering per subagent | delegation | `is_tool_visible_to_model` per subagent | ADOPT (per-pod tool restriction) |
| Recipe system for multi-step automation | tools-skills | `goose run --recipe`; self-test via YAML | ADOPT (scripted workflows) |
| Session config per subagent | delegation | `max_turns`, `schedule_id`, `retry_config` | ADOPT (task-specific configuration) |
| ACP proc macros for IDE integration | architecture | `goose-acp-macros` — Agent Client Protocol | ADOPT (ACP for IDE bridges) |
| Cross-platform binary install | ux-dx | macOS, Linux, Windows; Docker support | ADOPT (deployment options) |

| Weakness | Dimension | Evidence | ACE Decision |
|----------|-----------|----------|--------------|
| Linux Wayland: no mouse automation | computer-use | `wtype` only does text/key; no `xdotool click` | ADAPT (Wayland limitation; use X11 or screenshot-first) |
| adversary inspector blocks computercontroller | computer-use | `DEFAULT_TOOLS` blocklist includes automation script | ADOPT (opt-in for computer automation) |
| No native sandbox for tools | tools-skills | MCP extensions run without container isolation | ADAPT (container isolation for untrusted) |
| Rust compile times slow dev loop | ux-dx | `cargo build` is slower than Go/TypeScript | ADAPT (acceptable tradeoff for safety) |
| Profile system under-documented | ux-dx | `~/.config/goose/` profiles concept not fully documented | ADAPT (better documentation needed) |

---

### 2.9 Karpathy/autoresearch

| Strength | Dimension | Evidence | ACE Decision |
|----------|-----------|----------|--------------|
| Fixed 5-minute wall-clock budget | loops | `uv run train.py` always runs exactly 5 min | ADOPT (bounded autonomy for ACE research mode) |
| Git-based rollback | self-improvement | `git reset --hard` on regression; commit = experiment ID | ADOPT (zero-overhead versioning) |
| Single-file modification scope | loops | Agent only edits `train.py`; `prepare.py` is sacred | ADOPT (bounded modification surface) |
| Simplicity criterion | self-improvement | "Removing and getting equal/better is a win" | ADOPT (prevents feature accumulation) |
| NEVER STOP autonomous loop | self-improvement | Runs indefinitely without asking | ADOPT (continuous background learning) |
| results.tsv untracked by git | self-improvement | Local log outside experiment history | ADOPT (permanent record separate from artifacts) |
| Minimal 3-file architecture | architecture | `program.md` + `train.py` + `prepare.py` | ADOPT (minimal scope principle) |
| Zero ceremony setup | ux-dx | `uv sync && uv run train.py`; no wizard | ADOPT (for narrow research tools) |
| program.md file-based config | ux-dx | Plain text instructions; no JSON/YAML | ADOPT (for specialized ACE tools) |

| Weakness | Dimension | Evidence | ACE Decision |
|----------|-----------|----------|--------------|
| No delegation (single-agent only) | delegation | No subagents, no worker pool | AVOID (general ACE needs delegation) |
| No memory persistence | memory | VRAM-only; no cross-session recall | AVOID (ACE needs memory tiers) |
| No internet/browser access | browser-automation | Pure local experimentation | AVOID (ACE needs web tools) |
| Domain-specific (ML training) | architecture | Only for fixed-harness optimization | AVOID (not general-purpose) |
| No tool system beyond train.py | tools-skills | Single editable file; no tools | AVOID (general ACE needs tools) |
| Single GPU only | architecture | No distributed training | AVOID (ACE multi-pod coordination) |

---

### 2.10 OpenViking

| Strength | Dimension | Evidence | ACE Decision |
|----------|-----------|----------|--------------|
| L0/L1/L2 progressive loading | memory | `ABSTRACT` → `OVERVIEW` → `DETAIL` via `ContextLevel` enum | ADAPT (progressive disclosure valuable; URI complexity optional) |
| Two-stage retrieval (vector + rerank) | memory | `memory_extractor` — vector search then `rerank()` | ADOPT (hybrid retrieval) |
| Monotonic versioning | memory | `updated_at` comparison; rejects stale writes | ADOPT (memory quality protection) |
| viking:// virtual filesystem URI | memory | Unified `viking://memory/`, `viking://resources/`, `viking://skills/` | AVOID (adds coupling; separate namespaces simpler) |
| Tool/skill memory extraction via ReAct | memory | `memory_extractor` checks monotonic violations | ADOPT (event-driven consolidation) |

| Weakness | Dimension | Evidence | ACE Decision |
|----------|-----------|----------|--------------|
| Virtual filesystem URI coupling | memory | Memory tied to resource access | AVOID (separate namespaces) |
| Complex tiered storage (RocksDB + indexes) | architecture | Heavy infrastructure for memory | ADAPT (simpler storage for ACE) |
| No tool system documented | tools-skills | Primarily a memory system | ADAPT (integrate with ACE tool system) |

---

### 2.11 MSA (Memory Sparse Attention)

| Strength | Dimension | Evidence | ACE Decision |
|----------|-----------|----------|--------------|
| 100M token context via sparse attention | memory | `<9% degradation 16K→100M` on 2×A800 GPUs | MONITOR (hardware requirements) |
| Chunk-mean pooling compression | memory | O(L) complexity; predictable lossy compression | ADAPT (for extreme context needs) |
| Router projector Top-k selection | memory | Cosine similarity on pooled K/V | ADAPT (differentiable retrieval) |
| No compaction needed | compaction | Native sparse attention instead of summarization | MONITOR (not for standard deployment) |
| Memory Parallel shard architecture | architecture | Routing keys across GPUs; content in DRAM | MONITOR (specialized hardware) |

| Weakness | Dimension | Evidence | ACE Decision |
|----------|-----------|----------|--------------|
| Requires custom CUDA kernels | architecture | Not standard PyTorch/TensorFlow | AVOID (not for general ACE) |
| No attribution/provenance tracking | memory | KV cache only; no source citations | AVOID (ACE needs traceable memory) |
| Hardware-specific (NVIDIA A800) | architecture | Not portable to CPU or other GPUs | AVOID (ACE must be hardware-agnostic) |

---

## 3. Cross-System Convergence Analysis

### 3.1 Convergent Evolution Patterns (Appearing in 4+ Systems)

| Pattern | Systems | Dimension | Evidence |
|---------|---------|-----------|----------|
| **SKILL.md standard** | Hermes, pi-mono, OpenClaw, Karpathy, playwright-skill, Oh My OpenAgent | tools-skills | YAML frontmatter + Markdown body; `name`, `description`, `platforms` fields |
| **Graduated multi-layer compaction** | Claude Code (4), Hermes (2), OpenCode (2), OpenClaw (2) | compaction | Separated concerns: rough vs precise, safety net vs primary |
| **Auto-discovery tool registry** | Hermes (registry.py), pi-mono (loadSkillsFromDir), OpenClaw (manifest), Oh My OpenAgent (4-scope) | tools-skills | Directory scan or import-time registration |
| **Project-local context files** | Open Code (CLAUDE.md), pi-mono (AGENTS.md), Claude Code (CLAUDE.md), autoresearch (program.md) | ux-dx | `.opencode/`, `.pi/`, `~/.claude/` per-project override |
| **Doctor/diagnostics command** | OpenClaw, Hermes, Oh My OpenAgent | ux-dx | Pre-flight config validation |
| **Surface-agnostic core** | Hermes, pi-mono, Goose | architecture | Shared runtime imported by CLI/TUI/Web/Slack |
| **Two-phase permission filtering** | Claude Code (14-step), Open Code (plan mode), Hermes (tool blocking list) | tools-skills | Filter before model sees; check at execution |
| **Multi-provider abstraction** | Goose (Provider trait, 15+), pi-mono (providers/), Oh My OpenAgent (4-step routing) | architecture | Provider/plugin model for LLM selection |
| **Intent/plan classification before execution** | Oh My OpenAgent (Intent Gate), OpenCode (plan/build mode) | delegation | Pre-route based on user intent |
| **MCP as extension protocol** | Goose (70+), Hermes (MCP client), Oh My OpenAgent (3-tier), OpenClaw (plugin SDK) | tools-skills | Standardized tool/server interface |
| **Git-based rollback for experiments** | autoresearch (git reset), Hermes (atomic write rollback) | self-improvement | Version control for state changes |
| **Batched tool execution** | Hermes (ThreadPoolExecutor), Claude Code (AsyncGenerator concurrent) | loops | Collect results then reason, not per-call ReAct |
| **Tool result pruning before compaction** | Claude Code (microcompact), OpenCode (40K protect threshold), Hermes (200 char prune) | compaction | Cheap pre-pass before LLM summarization |
| **Structured summary template** | Hermes (Goal/Constraints/Progress/Done-InProgress-Blocked/Next Steps), OpenCode (hook-based) | compaction | Consistent format enables parsing |
| **Subagent isolation with restricted toolsets** | Hermes (blocked tools list), OpenCode (@general), Oh My OpenAgent (category routing) | delegation | Child gets subset of parent capabilities |

### 3.2 Unique Innovations (Appearing in 1-2 Systems)

| Innovation | System | Dimension | Significance |
|-----------|--------|-----------|--------------|
| **AsyncGenerator streaming loop** | Claude Code | loops | Speculative execution during streaming (unique) |
| **Path-overlap detection for concurrent tools** | Hermes | loops | Safety guard for concurrent file operations (unique) |
| **Monotonic memory versioning** | OpenViking | memory | Prevents stale writes from regressing quality (unique) |
| **3-agent memory architecture** | Honcho | memory | Deriver/dialectic/dreamer separation (rare) |
| **Dynamic code generation for browser** | playwright-skill | browser-automation | Model writes Playwright JS per request (unique) |
| **CDP deep browser control** | OpenClaw | browser-automation | Full DevTools protocol access (rare) |
| **Peekaboo CLI element-ID targeting** | Goose | computer-use | macOS UI annotation with named IDs (rare) |
| **Screenshots with numbered overlays** | Oh My OpenAgent | browser-automation | Element targeting via annotation (rare) |
| **BackgroundManager with polling** | Oh My OpenAgent | delegation | Idle event + 10s stability detection (unique) |
| **HEARTBEAT.md task checklists** | OpenClaw | self-improvement | Scheduled wake-ups for deferred tasks (rare) |
| **5-minute fixed experiment budget** | autoresearch | loops | Wall-clock discipline eliminates reasoning overhead (unique) |
| **Population-based evolutionary search** | AlphaEvolve | self-improvement | MAP-elites + island model for diversity (unique) |
| **Raw trace filesystem access for feedback** | Meta-Harness | self-improvement | 15-point accuracy gap vs scalar scores (unique) |
| **RLM recursive self-query** | RISE | memory | Inference-time meta-learning (unique) |
| **Sparse attention for 100M tokens** | MSA | memory | KV cache at extreme scale (unique) |
| **Slot reservation escalation** | Claude Code | compaction | 8K → 64K adaptive buffer (unique) |
| **CLI as separate rendering process** | Hermes | ux-dx | TypeScript/Ink over stdio JSON-RPC (rare) |
| **CSI 2026 differential TUI rendering** | pi-mono | ux-dx | Atomic updates, no flicker (unique) |
| **Hook event propagation system** | Oh My OpenAgent | communication | 52 hooks across 5 tiers (unique) |
| **Tokenizer-agnostic val_bpb metric** | autoresearch | self-improvement | Architecture-independent evaluation (unique) |

---

## 4. Contradictions Between Systems

| System A | System B | Contradiction | Source A | Source B | Resolution for ACE |
|----------|----------|---------------|----------|----------|---------------------|
| Claude Code | Hermes | **Concurrency model**: Claude Code serializes tools in AsyncGenerator loop; Hermes uses ThreadPoolExecutor for concurrent execution | `query.ts` sequential | `run_agent.py` batched | **ADAPT**: ACE should support configurable concurrency; default sequential, optional parallel with path-overlap guard |
| Claude Code | MSA | **Compaction approach**: Claude Code uses multi-layer graduated summarization; MSA avoids compaction entirely via sparse attention | 4-layer compaction | No compaction; chunk pooling | **ADAPT**: ACE should use graduated compaction for standard deployment; sparse attention is hardware-dependent option |
| OpenClaw | Devin | **Isolation model**: OpenClaw uses single Gateway process; Devin uses full VM isolation per worker | Single daemon bottleneck | VM per session | **ADAPT**: ACE pods use namespace isolation default; full VM isolation as opt-in for high-risk tasks |
| OpenCode | Hermes | **Communication**: OpenCode uses in-process PubSub; Hermes uses network-based gateway | In-process Effect.PubSub | stdio JSON-RPC + HTTP | **ADOPT**: ACE uses NATS (network-based) for distribution; NATS JetStream provides durability |
| pi-mono | OpenClaw | **IPC mechanism**: pi-mono uses file-based IPC; OpenClaw uses WebSocket gateway | `log.jsonl` filesystem | WebSocket broadcast | **ADAPT**: File-based IPC is too limiting for distributed ACE; NATS provides same reliability with distribution |
| Goose | MSA | **Memory approach**: Goose has no native memory system (relies on skills/MCP); MSA embeds memory in GPU KV cache | No persistent memory | GPU KV cache 100M tokens | **ADOPT**: ACE needs explicit memory tiers; GPU-only memory is not portable |
| autoresearch | Oh My OpenAgent | **Autonomy**: autoresearch NEVER STOP loop; Oh My OpenAgent has 46 lifecycle hooks requiring configuration | Autonomous loop | Heavy configuration hooks | **ADOPT**: ACE should default to autorun with explicit stop points; configuration optional |
| Hermes | Claude Code | **Tool security**: Hermes has 488-pattern security scanner for skills; Claude Code has no sandbox | `skills_guard.py` | Host reliance | **ADOPT**: ACE needs security scanning for agent-modified artifacts; sandbox for untrusted code |
| OpenClaw | Claude Code | **Feature gates**: OpenClaw uses runtime feature flags; Claude Code uses compile-time DCE (108 modules) | `feature()` runtime | `feature()` compile | **ADOPT**: Runtime feature flags more debuggable than compile-time removal |
| pi-mono | Oh My OpenAgent | **Extensibility**: pi-mono minimal (4 tools); Oh My OpenAgent heavy (11 agents, 52 hooks, 160k+ LOC) | Minimal footprint | Maximal extensibility | **ADOPT**: ACE should be closer to pi-mono's minimal core; extensions are additive |
| Devin | autoresearch | **Self-improvement depth**: Devin uses human-assisted playbook authoring; autoresearch has fully automated loop | Human synthesis | Autonomous git-driven | **ADOPT**: ACE needs automated harness improvement (autoresearch pattern) plus organizational knowledge (Devin playbook) |
| Goose | OpenClaw | **Sandboxing**: Goose blocks `computercontroller` by default (adversary inspector); OpenClaw runs tools on host | Denylist approach | Host reliance | **ADOPT**: ACE should require opt-in for computer automation; block by default |

---

## 5. ACE Decision Mapping Summary

### 5.1 ADOPT (Proven, Effective, Recommended)

| Pattern | Primary Systems | Source Dimension |
|---------|----------------|------------------|
| Surface-agnostic core | Hermes, pi-mono, Goose | architecture |
| Auto-discovery tool registry | Hermes | tools-skills |
| SKILL.md standard (YAML frontmatter + progressive disclosure) | 5+ systems | tools-skills |
| Graduated multi-layer compaction | Claude Code, Hermes | compaction |
| Iterative re-compression | Claude Code, Hermes | compaction |
| Structured summary template | Hermes | compaction |
| Batched tool execution (no ReAct per-call) | Hermes | loops |
| AsyncGenerator as loop driver | Claude Code | loops |
| Outer/inner loop separation | openclaw, pi-mono | loops |
| Token budget tracking across compaction | Claude Code | loops |
| Manager/worker hierarchy | Devin | delegation |
| @mention subagent invocation | OpenCode | delegation |
| Permission-filtered agent modes | OpenCode, Hermes | delegation |
| Tool blocking list for children | Hermes | delegation |
| Depth-2 limit with leaf/orchestrator roles | Hermes | delegation |
| Per-model concurrency limits | Oh My OpenAgent | delegation |
| Lifecycle hooks (spawn/spawned/ended) | OpenClaw | delegation |
| NATS as central message bus | ACE (existing) | communication |
| Typed subject constants | OpenCode (Zod) | communication |
| JetStream durable replay | Goose (replay buffer) | communication |
| Reference-counted TopicReg | OpenCode (PubSub) | communication |
| Monotonic versioning | OpenViking | memory |
| FTS5 full-text search | Hermes | memory |
| Atomic file writes | Hermes | memory |
| Character-limited bounded memory | Hermes | memory |
| Memory drift caveat | Claude Code | memory |
| Four-type memory taxonomy | Claude Code | memory |
| MCP as extension protocol | Goose, Hermes, Oh My OpenAgent | tools-skills |
| Two-phase permission filtering | Claude Code, OpenCode | tools-skills |
| Security scanner with pattern matching | Hermes (488 patterns) | tools-skills |
| Doctor/diagnostics command | OpenClaw, Hermes, Oh My OpenAgent | ux-dx |
| Project-local context files | Open Code, pi-mono, Claude Code | ux-dx |
| Zero-config defaults with progressive disclosure | Oh My OpenAgent, pi-mono, autoresearch | ux-dx |
| Typed SDK events | Claude Code, pi-mono | ux-dx |
| Profile/instance isolation | Hermes, Goose | ux-dx |
| Interactive onboarding wizard | OpenClaw, Hermes | ux-dx |
| Fixed time budget for experiments | autoresearch | self-improvement |
| Git-based rollback | autoresearch | self-improvement |
| Raw execution trace storage | Meta-Harness | self-improvement |
| Never-stop autonomous loop | autoresearch | self-improvement |
| Atomic writes with security rollback | Hermes | self-improvement |
| Security-gated self-modification | Hermes | self-improvement |
| Provider abstraction | Goose (15+), pi-mono | architecture |
| Multi-level config hierarchy | Oh My OpenAgent | ux-dx |

### 5.2 ADAPT (Valuable but Requires Modification)

| Pattern | Primary Systems | Required Modification |
|---------|----------------|----------------------|
| Intent classification before routing | Oh My OpenAgent | Keep optional; don't make mandatory |
| L0/L1/L2 progressive loading | OpenViking | Simplified without virtual URI complexity |
| 3-agent memory architecture | Honcho | Lighter version without full vector DB |
| CDP-based browser control | OpenClaw | Lightweight wrapper only if desktop needed |
| Peekaboo element-ID targeting | Goose | MacOS target only; not cross-platform default |
| xdotool/wtype Linux automation | Goose | Screenshot-first workflow to mitigate coordinate errors |
| Population-based search | AlphaEvolve | Simplified MAP-elites; not full LLM pipeline |
| Screenshot-first workflow | Goose (Linux) | Agent always sees screen before acting |
| BackgroundManager concurrency | Oh My OpenAgent | 5 concurrent per model as ceiling |
| Sibling write detection | Hermes | For cross-pod file change detection |
| Thread binding for subagent sessions | OpenClaw | For future messaging surfaces only |
| Hub-and-spoke gateway | OpenClaw, Hermes | Replace with NATS bus; no central process |
| VM-level isolation | Devin | Namespace/cgroup isolation as default; full VM opt-in |
| HNSW vector storage | Honcho | Lighter alternative for ACE zero-dependency goal |
| RLM recursive self-query | RISE | Monitor for model support; inference-time only |
| Sparse attention (MSA) | MSA | Hardware-dependent; monitor for standards |
| Playground/playbook system | Devin | Versioned procedure docs, not proprietary |
| Knowledge base with auto-suggest | Devin | Organizational knowledge layer for ACE |
| HEARTBEAT.md task checklists | OpenClaw | Heartbeat-driven deferred tasks, simplified |
| Hook event propagation | Oh My OpenAgent | 5-10 well-chosen hooks; not 52 |
| File-based IPC | pi-mono | For sandbox isolation only; not general IPC |
| Skills Hub marketplace | Hermes | With mandatory security scanning |
| Skills Workshop | OpenClaw | Quarantine for proposals |
| Dynamic code generation | playwright-skill | For browser automation only |
| Factory pattern for tools | Oh My OpenAgent | Without 26-tool complexity |
| CLI-first install | Open Code, Goose, Hermes | One command to first success |
| Interactive wizard | OpenClaw, Hermes | Split quick-start vs full config |

### 5.3 AVOID (Failed, Problematic, or Actively Discouraged)

| Pattern | Primary Systems | Reason |
|---------|----------------|--------|
| Community marketplace without scanning | OpenClaw | Cisco security report: significant malware rate |
| Single-threshold compaction | All | Either over- or under-compresses |
| Blocking user-facing compaction UI | OpenClaw | Adds complexity without proportional benefit |
| Lossy-only compression | Most | Without iterative update, multiple compactions degrade context |
| Single-agent-per-session gateway | OpenClaw | Too restrictive for ACE multi-pod model |
| VM-level isolation (full) | Devin | Heavy for common tasks; namespace isolation sufficient |
| Cloud-only brain | Devin | Antithetical to ACE local-first design |
| Closed-source proprietary | Devin, Claude Code | ACE must be open source |
| Hub-and-spoke gateway bottleneck | OpenClaw | Single process for 24+ channels |
| Compile-time DCE feature hiding | Claude Code | Invisible complexity; runtime flags more debuggable |
| Undercover auto-stripping AI attribution | Claude Code | Transparency issues; should be explicit opt-in |
| Unbounded file storage | OpenClaw | No compaction; grows indefinitely |
| Hard VRAM limits | AutoResearch | OOM-based is fragile |
| Virtual filesystem URI | OpenViking | Couples memory to resource access |
| In-process PubSub only | OpenCode | Limits to single-machine |
| Auto-stripping AI attribution | Claude Code | Transparency issues |
| Adversary inspector denylist (computercontroller) | Goose | Opt-in is correct; denylist proves risk |
| Single-binary obsession causing UX gaps | Various | Capability matters more than binary count |
| Mandatory DM pairing friction | OpenClaw | Security model is right; opt-out not discoverable |
| Telemetry without opt-out | Claude Code | Privacy concerns |
| Python GIL blocking parallelism | Hermes | Go runtime avoids this |
| 108 compile-time DCE modules | Claude Code | Invisible complexity |
| Heavyweight Python setup wizard | Hermes | 3,100 lines; split needed |
| Community marketplace without scanning | OpenClaw | Cisco report documented malware |

---

## 6. Dimensional Summary: What Works vs What Doesn't Work

### Architecture

**What Works:**
- Surface-agnostic core (pi-mono, Hermes, Goose): runtime callable from any surface without modification
- Hub-and-spoke gateway with message bus (not single process): NATS avoids OpenClaw bottleneck
- Auto-discovery extension loading (Hermes): zero-configuration plugin system
- Multi-provider abstraction (Goose trait, pi-mono providers): flexibility without lock-in
- Intent classification before routing (Oh My OpenAgent): specialized execution paths
- Minimal scope (autoresearch 3 files): bounded autonomy principle

**What Doesn't Work:**
- Single-process gateway bottleneck (OpenClaw): scales poorly beyond ~10 channels
- Closed-source proprietary systems: can't study, replicate, or contribute
- Heavy compile-time feature removal (Claude Code): invisible to debugging

### Memory

**What Works:**
- Character-limited bounded entries (Hermes): prevents unbounded growth
- FTS5 full-text search (Hermes): zero-dependency, powerful queries
- Atomic file writes (Hermes): prevents corruption from concurrent access
- Memory drift caveat + staleness warnings (Claude Code): prevents acting on stale info
- Monotonic versioning (OpenViking): prevents stale write regression
- Frontmatter metadata (Claude Code): structured, machine-parseable
- L0/L1/L2 progressive loading (OpenViking): context-appropriate disclosure

**What Doesn't Work:**
- Unbounded file storage (OpenClaw): no compaction, grows forever
- Virtual filesystem URI coupling (OpenViking): ties memory to resource access
- Hard VRAM limits (AutoResearch): OOM-based is fragile
- GPU-only memory (MSA): not portable, requires custom kernels

### Compaction

**What Works:**
- Graduated multi-layer (Claude Code 4 layers): separates concerns
- Dual-tier with safety net (Hermes 85%/50%): catches what primary misses
- Iterative re-compression (Claude Code, Hermes): preserves context across cycles
- Structured summary template (Hermes): Done/InProgress/Blocked is actionable
- Cache-aware design (Claude Code cache_edits): preserves 75% of cache
- Tool output pruning pre-pass (OpenCode): cheap before LLM call
- Ephemeral prompt injection (Hermes): context without persistence
- Compact boundary events (Claude Code): SDK state preservation

**What Doesn't Work:**
- Single-threshold compaction: either too early or too late
- Blocking user-facing UI: adds complexity without proportional benefit
- Lossy-only without iterative update: degrades over multiple cycles
- Ignoring cache invalidation: causes 75%+ cost increase

### Loops

**What Works:**
- AsyncGenerator streaming (Claude Code): enables speculative execution
- Batched tool execution (Hermes): reduced latency vs ReAct per-call
- ThreadPoolExecutor with path-overlap guard (Hermes): safe concurrency
- Outer/inner loop separation (openclaw, pi-mono): cross-turn vs turn concerns
- Token budget tracking across compaction (Claude Code): maintains coherence
- Fixed time budget (autoresearch): eliminates iteration overhead
- Git-based rollback (autoresearch): zero-overhead experiment state
- Speculative permission pre-checking (Claude Code): avoids mid-loop interruption
- NEVER STOP autonomous loop (autoresearch): continuous experimentation

**What Doesn't Work:**
- ReAct per-call in tool-heavy tasks: high latency, unnecessary observe-think cycles
- No concurrency option (Claude Code): serial execution limits throughput
- No depth limits (Oh My OpenAgent): potential runaway delegation

### Delegation

**What Works:**
- Manager/worker hierarchy (Devin): clean separation, parallel execution
- @mention invocation (OpenCode): natural, user-visible routing
- Permission-filtered modes (OpenCode): essential security
- Depth-2 limit with roles (Hermes): prevents runaway
- Tool blocking list for children (Hermes): correct isolation boundaries
- Per-model concurrency limits (Oh My OpenAgent): resource protection
- Lifecycle events (OpenClaw): observability and control
- spawnedBy tracking (OpenClaw): parent-child lineage
- Polling + stability detection (Oh My OpenAgent): robust completion signals
- Heartbeat during delegation (Hermes): prevents timeout

**What Doesn't Work:**
- Single-agent-per-session (OpenClaw): too restrictive
- No depth limits (Oh My OpenAgent): no runaway protection
- Full VM isolation everywhere (Devin): too expensive for common tasks

### Tools & Skills

**What Works:**
- SKILL.md standard (5+ systems): converged independently
- Self-registration at import (Hermes): eliminates manual wiring
- Progressive disclosure (playwright-skill, Hermes): token-efficient
- MCP as extension protocol (Goose, Hermes, Oh My OpenAgent): emerging standard
- Security scanner 488 patterns (Hermes): comprehensive static analysis
- Two-phase permission filtering (Claude Code, OpenCode): defense in depth
- Factory pattern (Oh My OpenAgent): consistent tool creation
- Per-agent tool allowlists (OpenClaw): granular restriction

**What Doesn't Work:**
- Community marketplace without scanning (OpenClaw): documented malware
- No sandbox (most systems): host security is insufficient
- Closed plugin ecosystems: MCP is the right open standard

### Browser Automation

**What Works:**
- Dynamic code generation (playwright-skill): maximum flexibility
- MCP Playwright integration (Oh My OpenAgent): standardized tool protocol
- webfetch/websearch baseline (OpenCode, Claude Code): appropriate for coding agents
- Progressive disclosure docs (playwright-skill): SKILL.md → API_REFERENCE.md
- Annotated screenshots (Oh My OpenAgent): element targeting precision

**What Doesn't Work:**
- In-browser agent (pi-mono): ACE operates independently
- No browser control (most systems): information retrieval only is limiting for some tasks
- Heavy CDP complexity without need: screenshot-first workflow better

### Computer Use

**What Works:**
- Goose computercontroller cross-platform (Peekaboo/xdotool/wtype): OS-level automation
- Claude Computer Use API design: screen-coordinate + screenshot loop
- Pi-mono clipboard chain: platform-detecting fallback
- Screenshot-first workflow (Goose Linux): mitigates coordinate errors
- Cowork "connectors first" priority: correct hierarchy
- Structured spec handoff (Claude Design): design-to-code pipeline

**What Doesn't Work:**
- Devin VM sandboxing: cloud-only, expensive
- Full VM isolation as default: namespace isolation sufficient
- Cloud-only Brain: local-first ACE

### Communication

**What Works:**
- NATS as message bus (ACE existing): avoids single-process bottleneck
- Typed event definitions (OpenCode Zod): prevents routing errors
- JetStream durable replay: beyond in-memory buffers
- Reference-counted subscription management (TopicReg): scale optimization
- Monotonic session sequencing (Goose): ordering guarantees
- Async context propagation (Hermes): trace + auth through handlers
- Blocking handler thread pool (Hermes): prevents dispatcher starvation
- Manager/worker coordination (Devin): task decomposition pattern

**What Doesn't Work:**
- In-process PubSub only (OpenCode): single-machine limitation
- File-based IPC as general solution (pi-mono): limits distribution
- Single gateway process (OpenClaw): bottleneck risk
- Cloud-only Brain (Devin): local-first violation

### Self-Improvement

**What Works:**
- Fixed time budget (autoresearch): eliminates iteration reasoning
- Git-based rollback (autoresearch): zero-overhead versioning
- Raw trace storage + filesystem access (Meta-Harness): 15-point accuracy gap vs scalar
- Coding-agent proposer (Meta-Harness): selective diagnosis
- Pareto frontier selection (Meta-Harness, AlphaEvolve): multi-objective
- Population-based search (AlphaEvolve): diversity preservation
- Never-stop loop (autoresearch): continuous learning
- Atomic writes with security rollback (Hermes): safe self-modification
- HEARTBEAT.md skip-on-empty (OpenClaw): reduces wasted inference
- Trajectory capture → skill authoring (Hermes): passive pipeline
- results.tsv untracked (autoresearch): permanent log separate from artifacts

**What Doesn't Work:**
- Compressed scalar feedback: loses critical diagnostic information
- Human-assisted playbook synthesis (Devin): not scalable
- Single-candidate optimization: fragile when evaluation noisy

### UX/DX

**What Works:**
- Zero-config defaults with opinionated setup (Oh My OpenAgent, pi-mono, autoresearch): minimal time-to-first-result
- Doctor/diagnostics command (OpenClaw, Hermes, Oh My OpenAgent): pre-flight checks
- Project-local context files (Open Code, pi-mono, Claude Code): project-specific override
- Interactive onboarding wizard (OpenClaw, Hermes): guided setup
- Typed SDK events (Claude Code, pi-mono): headless embedding
- Profile/instance isolation (Hermes, Goose): prevents credential leakage
- Multi-surface same-core (pi-mono, Hermes): CLI/TUI/Web/Slack from shared runtime
- Skin/theme engine (Hermes, pi-mono): customizable visuals
- Differential TUI rendering (pi-mono): no flicker, CSI 2026
- JSONC config with comments (Oh My OpenAgent): developer-friendly
- CLI-first install (Open Code, Goose, Hermes): one command to first success
- Git-based experiment state (autoresearch): versioned artifacts

**What Doesn't Work:**
- Heavy compile-time feature hiding (Claude Code): invisible complexity
- Auto-stripping AI attribution (Claude Code): transparency issues
- Telemetry without opt-out (Claude Code): privacy concerns
- Heavyweight Python setup wizard (Hermes): 3,100 lines; slow
- Mandatory DM pairing friction (OpenClaw): right security, wrong UX
- Single-binary obsession causing UX gaps: capability over binary count

---

## 7. Open Questions for ACE

1. **Concurrency default**: Sequential by default (Claude Code) or parallel with guards (Hermes)? ACE should make concurrency configurable with safe defaults.

2. **Memory tier boundary**: At what context length should ACE switch from L1 (ephemeral) to L2 (FTS/embedding) retrieval?

3. **Sparse attention adoption**: MSA-style native sparse attention for extreme contexts — timeline and hardware dependencies?

4. **Population size**: Meta-Harness/AlphaEvolve-style population-based search — what is the minimum viable population for ACE's learning loop?

5. **Skill synthesis automation**: No system automatically synthesizes trajectories into SKILL.md. Is this worth solving for ACE?

6. **Wayland mouse automation**: No viable solution exists. Monitor `wtype` development or accept screenshot-first workflow on Linux?

---

## 8. Source Files & Evidence Traceability

| Finding | Source Slice | Source Evidence |
|---------|-------------|-----------------|
| OpenClaw gateway bottleneck | architecture.md, communication.md | `src/gateway/server.impl.ts` — single daemon |
| Claude Code 4-layer compaction | compaction.md | `compact.ts`, `autoCompact.ts`, `microCompact.ts`, `sessionMemoryCompact.ts` |
| Hermes auto-discovery registry | tools-skills.md | `tools/registry.py` — `registry.register()` at import |
| OpenCode in-process PubSub | communication.md | `src/bus/index.ts` — Effect.PubSub |
| Devin VM isolation | delegation.md | [Cognition blog](https://www.cognition-labs.com/blog/devin-can-now-manage-devins) |
| MSA 100M token context | memory.md | `src/msa/memory_sparse_attention.py` |
| Meta-Harness 15-point trace gap | self-improvement.md | [arxiv 2603.28052v1](https://arxiv.org/html/2603.28052v1) |
| autoresearch fixed 5-min budget | loops.md, self-improvement.md | `program.md` — LOOP protocol |
| OpenClaw Cisco security report | tools-skills.md | `docs/tools/skills.md` — security notes |
| Goose Peekaboo targeting | computer-use.md | `crates/goose-mcp/src/computercontroller/` |
| pi-mono CSI 2026 TUI | ux-dx.md | `packages/tui/` — differential rendering |
| Oh My OpenAgent Intent Gate | delegation.md | `src/agents/sisyphus.ts` — Intent Gate |
| Claude Code AsyncGenerator | loops.md | `src/query.ts` — 685KB single file |
| Hermes 488-pattern scanner | tools-skills.md | `tools/skills_guard.py` |
| Karpathy 3-file architecture | architecture.md | `program.md`, `train.py`, `prepare.py` |

---

**Slice Completed:** 12 / 14  
**Files Affected:**
- `/home/jay/programming/ace_prototype/design/units/agents-study/study/strengths-weaknesses.md`

**Changes Made:** Synthesized all 11 prior study files into structured comparative document with per-system strengths/weaknesses tables (11 systems), cross-system convergence analysis (15 convergent patterns, 20 unique innovations), contradiction matrix (12 system pairs), and ACE decision mapping (85+ ADOPT, 30+ ADAPT, 25+ AVOID items).
