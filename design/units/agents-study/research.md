# Research: Existing Agents Study

## Methodology

This document synthesizes findings from primary source analysis of 13 systems (7 agent frameworks, 6 specialized systems) and 5 research papers. Research was conducted via direct repository inspection, official documentation, architecture deep-dives, and community discourse analysis. Every claim is traceable to a specific source.

The analysis is organized around **cross-cutting dimensions** rather than per-system summaries, because ACE's design decisions are dimensional ("how should memory work?") not system-specific ("what does OpenClaw do?"). Each dimension concludes with an **ACE Recommendation**: **Adopt**, **Avoid**, or **Adapt**.

---

## Systems Inventory

### Agent Frameworks

| System | Language | Deployment | License | Stars (approx) | Core Paradigm |
|--------|----------|------------|---------|----------------|---------------|
| **OpenClaw** | TypeScript | Single process (Gateway) | MIT | High | ReAct loop, file-based memory, skills |
| **Claude Code** | TypeScript | CLI / SDK | Proprietary (npm) | N/A | AsyncGenerator loop, 4-layer compression, file-based memory |
| **Goose** | Rust | Desktop, CLI, API | MIT (Linux Foundation) | High | MCP-based extensions, subagent delegation, recipes |
| **Hermes Agent** | Python | Gateway daemon | Open source | 47k+ | Delegation (not ReAct), skill auto-generation, Honcho memory |
| **pi-mono** | TypeScript | Packages (CLI, TUI, Web, Slack) | Open source | Moderate | Layered runtime (pi-ai → pi-agent → surfaces) |
| **Oh My OpenAgent** | TBD | TBD | TBD | TBD | TBD — research required |
| **Devin** | Unknown | Cloud (VM-per-instance) | Proprietary | N/A | Hierarchical orchestration, managed sub-agents |

### Specialized Systems

| System | Purpose | Key Relevance to ACE |
|--------|---------|---------------------|
| **pi-mono** | Agent toolkit + vLLM pods | Clean runtime/surface separation |
| **andrej-karpathy-skills** | Skill patterns | Skill specification patterns |
| **playwright-skill** | Browser automation skill | Skill execution model |
| **honcho** | User-context memory library | Entity-centric memory, continual learning |
| **OpenViking** | Context database for agents | Filesystem-paradigm context unification |
| **MSA** | Memory Sparse Attention | End-to-end differentiable long-term memory |

### Research Papers

| Paper | Institution | Core Contribution |
|-------|-------------|-------------------|
| **TurboQuant** | Google Research | KV cache compression (6x, ~3 bits, near-zero loss) |
| **RLM** | Independent | Recursive Language Models for document analysis |
| **arxiv 2603.28052v1** | TBD | TBD — requires follow-up |
| **arxiv 2506.13131** | TBD | TBD — requires follow-up |
| **rotorquant** | scrya-com | Quantization tooling |

---

## Dimension 1: Core Agent Loop Architecture

### Findings

**OpenClaw** uses a dual-loop design: an outer loop selects the next task; an inner loop executes it via ReAct (Reason + Act). Runs are serialized per session key to prevent race conditions. The loop emits lifecycle and stream events throughout. Auto-compaction triggers retry with reset buffers.

**Claude Code** drives the entire system through an `AsyncGenerator` in `query.ts`. The loop streams model output, executes tools, recovers from errors, and compresses context. A key innovation is **speculative execution**: read-only tools start during model streaming before the response completes. Tools are partitioned by safety classification — reads run in parallel, writes serialize.

**Hermes Agent** explicitly rejects ReAct. Its `AIAgent` class (~10,700 lines) uses **delegation** as the primary multi-step mechanism: `tools/delegate_tool.py` spawns child agents with isolated context, restricted toolsets, and max depth of 2. For reasoning it relies on Claude 4.6+ adaptive thinking with configurable `reasoning_effort`. Multiple tool calls execute concurrently via `ThreadPoolExecutor`.

**Goose** follows a standard conversation loop: load extensions → assemble system prompt → stream LLM → dispatch tools → feed results back. Subagents are first-class: Goose autonomously decides to spawn them in autonomous permission mode. Recipes define reusable subagent configurations.

**Devin** has evolved from single-agent to **distributed orchestrator**. A manager Devin decomposes tasks and delegates to up to 10 worker Devins, each in an isolated VM. The manager scopes work, monitors progress, resolves conflicts, and compiles results.

**pi-mono** makes the loop architecture explicit: `pi-agent-core` holds the turn loop (context shaping → LLM call → tool execution → continuation decision). The same core powers CLI, TUI, web UI, and Slack bot surfaces.

### Trade-offs

| Approach | Pros | Cons |
|----------|------|------|
| **ReAct (OpenClaw, Claude Code)** | Simple, well-understood, easy to debug | Reasoning contaminated by execution details, context bloat |
| **Delegation (Hermes, Devin, Goose)** | Clean separation, parallel execution, failure isolation | Higher overhead, complex context merging, depth limits |
| **AsyncGenerator (Claude Code)** | Natural backpressure, cancellation, streaming | Tight coupling of streaming and execution logic |
| **Dual-loop (OpenClaw)** | Planning separated from execution | Outer loop can become bottleneck |

### ACE Recommendation: **Adapt**

ACE's six cognitive layers are already a form of structural delegation — each layer handles a different abstraction level. The evidence strongly supports:

1. **Adopt** Claude Code's speculative execution for L6 (Task Prosecution) — starting read-only tools while the model streams is a zero-cost latency win.
2. **Adopt** Hermes' concurrent tool execution via worker pools within a layer loop.
3. **Adapt** Devin's hierarchical pod structure for swarm coordination — the manager/worker pattern maps directly to ACE's pod tree.
4. **Avoid** OpenClaw's single serialized session lane for ACE's core engine; NATS message routing already provides the concurrency primitives we need.

---

## Dimension 2: Memory Systems

### Findings

**OpenClaw** uses local Markdown files for memory. Files are compacted when context runs low. The system is "persistent and inspectable, but also fragile." Community members have built elaborate three-layer memory systems on top because retention is unreliable. Memory works through semantic search rather than keyword matching.

**Claude Code** uses file-based memory with an LLM-powered recall system (Sonnet side-query selects relevant memories, not embedding search). Four memory types exist with staleness warnings. The MEM.md file is read at session start and written at session end.

**Hermes Agent** uses three memory layers: Session memory (SQLite + FTS5), Persistent memory (Markdown files with validated workflows), and Honcho user modeling. It uses **on-demand retrieval**, not full-context loading. After tool-heavy turns, it opportunistically captures trajectories as skills.

**Honcho** (used by Hermes) is an entity-centric memory library. It uses peer modeling where both users and agents are "peers." Messages trigger asynchronous reasoning about peer psychology. A `context` endpoint returns combined messages, conclusions, and summaries up to a token limit. Collections organize global user data; Metamessages store flexible session-local context.

**OpenViking** unifies memory, resources, and skills into a **virtual filesystem** (`viking://` protocol). It uses L0/L1/L2 three-tier progressive loading, directory recursive retrieval, and two-stage retrieval (vector search + rerank). The filesystem paradigm replaces fragmented vector storage.

**MSA** (Memory Sparse Attention) is the most radical approach: it integrates retrieval and generation into a single differentiable loop. Document latent states (K/V) are chunk-mean pooled. A router projector selects Top-k documents, and their compressed K/V is concatenated with local K/V for autoregressive decoding. Achieves 100M token contexts on 2xA800 GPUs with <9% degradation.

### Trade-offs

| Approach | Pros | Cons |
|----------|------|------|
| **File-based (OpenClaw, Claude Code)** | Human-inspectable, simple, zero schema | Fragile, no structured querying, race conditions |
| **SQLite + FTS5 (Hermes session)** | Fast, structured, ACID | Not semantic, requires hybrid search |
| **Entity-centric async reasoning (Honcho)** | Rich user models, continual learning | Costly, complex, opaque reasoning process |
| **Virtual filesystem (OpenViking)** | Unified context types, hierarchical retrieval | New paradigm, ecosystem immaturity |
| **Native sparse attention (MSA)** | End-to-end differentiable, massive scale | Requires model architecture changes, hardware-specific |

### ACE Recommendation: **Adapt**

ACE's existing L1-L4 memory architecture is well-positioned. The research validates and refines it:

1. **Adopt** OpenViking's filesystem-paradigm for L4 organization — the URI-based hierarchical structure maps naturally to ACE's tree nodes with parent references.
2. **Adopt** Honcho's entity-centric peer model for cross-agent memory attribution — every memory node should track which agent (peer) created it.
3. **Adopt** MSA's progressive loading concept (L0/L1/L2) for ACE's existing tier structure — rename/combine to align terminology.
4. **Adapt** Claude Code's LLM-powered recall as a secondary retrieval path for L4 — use embedding search for candidate generation, then a lightweight LLM call for relevance scoring.
5. **Avoid** pure file-based memory; ACE's SQLite + PostgreSQL hybrid is the correct choice for production reliability.

---

## Dimension 3: Context Compaction & Token Budgets

### Findings

**Claude Code** implements the most sophisticated compaction: **4-layer compression** — snip, microcompact, collapse, autocompact — each lighter than the next. It also uses slot reservation (8K default output cap, escalate to 64K on hit, saving context in 99% of requests). The SDK emits `compact_boundary` events in the stream.

**OpenClaw** has auto-compaction that emits `compaction` stream events and can trigger retry. On retry, in-memory buffers and tool summaries are reset to avoid duplicate output. Uses a combination of summarization, sliding window, and truncation depending on configuration.

**Hermes Agent** checks if preflight compression is needed (>50% context) before each API call. It injects ephemeral prompt layers (budget warnings, context pressure) and applies prompt caching markers on Anthropic. Persistent memory is flushed before context is lost.

### Trade-offs

| Strategy | Pros | Cons |
|----------|------|------|
| **4-layer graduated (Claude Code)** | Graceful degradation, predictable behavior | Complex implementation, many code paths |
| **Summarization + sliding window (OpenClaw)** | Simple, configurable | Abrupt context loss, "forgetting" |
| **Preflight check + ephemeral warnings (Hermes)** | Proactive, model-aware | Adds tokens for warnings themselves |

### ACE Recommendation: **Adapt**

ACE should implement a **3-tier compaction strategy**:

1. **L1 compaction** (snip): Remove verbatim tool output noise, keep results — applied every iteration.
2. **L2 compaction** (summarize): Summarize older conversation turns when crossing 75% of L1 budget — maps to L1→L2 promotion.
3. **L3 compaction** (archive): Flush to persistent storage and replace with node references — maps to L2→L3→L4 pipeline.

**Adopt** Claude Code's slot reservation pattern for LLM output budgets within layer loops.

---

## Dimension 4: Tool & Skill Systems

### Findings

**Goose** is built on the **Model Context Protocol (MCP)** — an open standard for tool/extension interoperability. 70+ extensions available. Recipes are reusable task templates with specific instructions, extensions, and behavior. Skills can be shared and referenced by name.

**Hermes Agent** has 47 registered tools across 19 toolsets. Each tool file self-registers at import time. Skills are Markdown files with YAML frontmatter, platform conditions, inline shell snippets, and parameter slots. The agent can **auto-generate skills** from tool-heavy trajectories. Compatible with agentskills.io standard.

**OpenClaw** uses YAML + Markdown files in a skills folder. Skills are modular and injected into the prompt at runtime. The community has built a large skill marketplace, but Cisco found 93% of skill developers have no verified identity and 12% malware rate in skills.

**pi-mono** uses a `ToolDefinition` registry with custom rendering functions. The `AgentSession` coordinates tool lifecycle with extension hooks. Built-in tools are optimized for coding tasks.

### Trade-offs

| Approach | Pros | Cons |
|----------|------|------|
| **MCP standard (Goose)** | Interoperability, large ecosystem | Dependency on external standard evolution |
| **Self-registering tool files (Hermes, OpenClaw)** | Simple to add, discoverable | No sandboxing by default, security risk |
| **Auto-generated skills (Hermes)** | Captures emergent workflows | Quality varies, requires validation |
| **Recipe templates (Goose)** | Reusable, shareable | Static, not adaptive |

### ACE Recommendation: **Adapt**

1. **Adopt** MCP as the primary tool interface standard — ACE's tool system should speak MCP natively for ecosystem compatibility.
2. **Adopt** Hermes' YAML-frontmatter skill format for ACE's internal skill definitions.
3. **Adapt** Hermes' auto-skill-generation for the Learning Loop — but require human review or automated validation before activation.
4. **Avoid** OpenClaw's unverified skill marketplace model; ACE should require cryptographically signed skills or sandboxed execution.

---

## Dimension 5: Multi-Agent Delegation & Orchestration

### Findings

**Devin** uses explicit manager/worker hierarchy: 1 manager + up to 10 workers, each in isolated VMs. The manager distributes tasks, monitors progress, resolves conflicts, merges changes. Workers are state-contained; failures don't cascade.

**Goose** supports subagents via natural language delegation. Internal subagents inherit the parent's context and extensions (optionally restricted). External subagents integrate via MCP. Goosetown (community project) implements "flocks" — researcher, writer, worker, reviewer roles with a "Town Wall" broadcast channel.

**Hermes Agent** uses `delegate_tool.py` to spawn child agents with isolated context, restricted toolsets, and max depth of 2. Parent blocks until children return summaries. No recursive delegation allowed in children.

**OpenClaw** has subagent support but the gateway is fundamentally single-agent-per-session.

### Trade-offs

| Approach | Pros | Cons |
|----------|------|------|
| **VM isolation (Devin)** | True failure containment, resource boundaries | High overhead, slow spawn, cloud-only |
| **Process isolation (Goose subagents)** | Medium overhead, local execution | Context duplication, sync complexity |
| **In-process delegation (Hermes)** | Fast, low overhead | No true isolation, crash propagation risk |

### ACE Recommendation: **Adapt**

ACE's pod tree architecture is validated by Devin's manager/worker pattern and Goose's flock pattern. Specific recommendations:

1. **Adopt** Devin's explicit manager/worker topology for pod internals — the root pod is the manager, leaf pods are workers.
2. **Adopt** Goose's "Town Wall" concept for inter-pod broadcast — maps to NATS subject fan-out.
3. **Adapt** Hermes' depth-limiting (max 2) for intra-pod delegation to prevent runaway spawning.
4. **Avoid** VM-per-agent overhead; ACE's single-binary model uses process-level isolation via goroutines + bounded worker pools.

---

## Dimension 6: Self-Improvement & Learning Loops

### Findings

**Hermes Agent** has the most explicit learning loop: after tool-heavy turns, it opportunistically captures trajectories as skills. Skills live in `tools/skill_manager_tool.py` and `tools/skills_hub.py`. The agent edits its own skill files. Persistent memory accumulates validated workflows.

**Devin** creates and improves **playbooks** — reusable session templates. It analyzes session outcomes, identifies patterns, extracts learnings, and refines playbooks based on feedback. Also manages knowledge (deduplicate, consolidate, create entries).

**Claude Code** has no explicit self-improvement loop in the open-source SDK; improvement comes from model updates and CLAUDE.md project-specific instructions.

**OpenClaw** has heartbeat rules (`HEARTBEAT.md`) and scheduled tasks, but no systematic learning mechanism.

**Research: RISE** (Recursive IntroSpEction) fine-tunes LLMs to self-improve over multiple turns. It formulates fine-tuning as a multi-turn MDP and uses online imitation learning + reward-weighted supervised learning. Demonstrates that even 7B models can improve themselves sequentially on reasoning tasks.

### Trade-offs

| Approach | Pros | Cons |
|----------|------|------|
| **Skill capture (Hermes)** | Concrete, inspectable, immediately useful | Requires tool-heavy turns, quality varies |
| **Playbook evolution (Devin)** | Structured, human-reviewable | Cloud-only, proprietary |
| **RL fine-tuning (RISE)** | Fundamental capability improvement | Expensive, requires training infra, model-specific |

### ACE Recommendation: **Adapt**

1. **Adopt** Hermes' trajectory-to-skill capture for ACE's Learning Loop — when L6 completes a multi-tool task successfully, extract a skill template.
2. **Adapt** Devin's playbook concept into ACE's **immutable configuration records** — each learned configuration is a new versioned row, not an in-place update.
3. **Adapt** RISE's multi-turn MDP formulation for ACE's layer loop tuning — treat layer configuration as a policy to be improved via feedback signals.
4. **Avoid** in-place prompt mutation; ACE's immutability constraint prevents this.

---

## Dimension 7: Communication Patterns

### Findings

**OpenClaw** routes everything through a single Gateway process. Messages arrive via channel adapters, are normalized, routed to sessions, and returned via the router. The gateway is both a bottleneck and a serialization point.

**Goose** uses MCP for tool communication and has a "Town Wall" broadcast channel in Goosetown for inter-agent messaging.

**pi-mono** cleanly separates the runtime (`pi-agent-core`) from surfaces (TUI, web, Slack). The runtime knows nothing about how it's being consumed.

**ACE** (existing design) uses embedded NATS with typed subjects and JetStream streams. This is already more sophisticated than most frameworks studied.

### ACE Recommendation: **Adopt** (existing design validated)

ACE's NATS-based messaging is architecturally superior to the Gateway or direct-call patterns found in most frameworks. No changes recommended. The study validates the existing decision.

---

## Dimension 8: UX/DX Patterns

### Findings

**OpenClaw** configuration is entirely file-based (`openclaw.json`, `SOUL.md`, `HEARTBEAT.md`). This is transparent but fragile — malformed JSON crashes the gateway with cryptic errors. Users report spending 12+ hours on basic setup.

**Claude Code** provides both interactive CLI and headless SDK modes. The SDK yields typed events (`AssistantMessage`, `SystemMessage`, `SDKCompactBoundaryMessage`) enabling rich UI construction.

**Goose** offers desktop app, CLI/TUI, and API. Multi-provider support (15+) is built-in. Configuration stored in `~/.config/goose/` with profiles.

**Hermes Agent** supports 15+ messaging surfaces simultaneously (Telegram, Discord, Slack, etc.). All route through a single gateway daemon. Setup is via interactive wizard (~3,100 lines).

### Trade-offs

| Approach | Pros | Cons |
|----------|------|------|
| **File-based config (OpenClaw)** | Version-controllable, inspectable | No validation, cryptic errors, high friction |
| **Interactive wizard (Hermes)** | Guided, harder to mess up | Harder to automate, drift from defaults |
| **SDK + events (Claude Code)** | Programmable, testable | Requires code to use |

### ACE Recommendation: **Adapt**

1. **Adopt** Claude Code's typed event streaming for ACE's Layer Inspector and real-time UI.
2. **Adopt** Goose's profile-based configuration for agent/pod configs.
3. **Adapt** Hermes' multi-surface gateway pattern for ACE's Chat Interface.
4. **Avoid** OpenClaw's raw JSON config approach; ACE should use structured, validated configurations with immutable versioning.

---

## Dimension 9: Safety & Security

### Findings

**OpenClaw** has severe security issues: CVE-2026-25253 (CVSS 8.8) exposed unauthenticated `/api/export-auth` endpoint allowing API key extraction. 135,000+ publicly exposed instances. 12% malware rate in skills. Cisco called it "a security nightmare."

**Claude Code** implements a 14-step permission pipeline: permission resolution, speculative execution, concurrent batching by safety classification. Tools have `isConcurrencySafe()` checks. Read/write partitioning prevents race conditions.

**Hermes Agent** has `tools/approval.py` for dangerous command detection. Subagents get restricted toolsets (no recursive delegation, no `clarify`, no `memory`, no `send_message`).

**Goose** has permission modes: manual approval, smart approval, autonomous, chat-only. Subagents can have restricted extensions.

### ACE Recommendation: **Adapt**

1. **Adopt** Claude Code's safety-classification-based tool partitioning for L6.
2. **Adopt** Goose's graded permission modes (manual → smart → autonomous) as configurable per-agent settings.
3. **Adopt** Hermes' tool restriction for delegated sub-agents.
4. **Avoid** OpenClaw's security posture entirely — unauthenticated endpoints and unverified skills are unacceptable for ACE.

---

## Research Papers Synthesis

### TurboQuant (Google Research, ICLR 2026)

TurboQuant compresses KV caches to ~3 bits with near-zero accuracy loss, achieving 6x memory reduction and up to 8x attention speedup on H100. Uses two-stage approach: PolarQuant (polar-coordinate rotation + scalar quantization) + QJL (1-bit residual correction). Data-oblivious — no k-means training required.

**Implication for ACE:** If ACE deploys local inference (via pi-mono's vLLM pods or similar), TurboQuant enables dramatically larger context windows on the same hardware. For the cloud-provider path, this is irrelevant today but may reduce costs within 12 months as providers adopt it.

### MSA (EverMind)

Memory Sparse Attention achieves 100M-token effective memory on 2xA800 GPUs by embedding retrieval inside Transformer attention layers. End-to-end differentiable, chunk-mean pooled K/V, router projector for Top-k selection, Memory Parallel for distributed inference.

**Implication for ACE:** MSA is the long-term direction for memory architecture, but it requires model re-architecture. ACE should track this for the Providers unit and consider a research integration path. Not actionable for the current SQLite/PostgreSQL memory stack.

### RLM / RISE

Recursive Language Models and Recursive IntroSpEction both demonstrate that models can be trained to improve their own outputs over multiple turns. RISE specifically uses iterative fine-tuning as a multi-turn MDP.

**Implication for ACE:** The Learning Loop should collect multi-turn interaction data now, even if fine-tuning infrastructure is not yet built. This data will be essential for future self-improvement capabilities.

---

## Cross-Cutting Strengths & Weaknesses

### What Works

1. **Speculative tool execution** (Claude Code) — zero-cost latency reduction.
2. **MCP for tool interoperability** (Goose) — ecosystem leverage.
3. **Graduated context compaction** (Claude Code) — predictable degradation.
4. **Auto-skill generation** (Hermes) — concrete self-improvement.
5. **VM-level isolation for multi-agent** (Devin) — true fault containment.
6. **Virtual filesystem for context** (OpenViking) — unified mental model.
7. **Typed event streaming** (Claude Code SDK) — programmable observability.

### What Doesn't Work

1. **File-based memory at scale** (OpenClaw) — fragmentation, race conditions, poor retrieval.
2. **Unverified skill marketplaces** (OpenClaw) — security catastrophe.
3. **Raw JSON config without validation** (OpenClaw) — fragile, poor DX.
4. **Monolithic agent loops** (Hermes' 10,700-line `run_agent.py`) — unmaintainable.
5. **No explicit learning loop** (Claude Code SDK, OpenClaw) — agents don't improve with use.
6. **Proprietary cloud-only orchestration** (Devin) — not reproducible, vendor-locked.

---

## Gaps Requiring Follow-Up Research

1. **Oh My OpenAgent** — No research conducted yet; architecture unknown.
2. **Open Code** — No research conducted yet; likely coding-focused agent.
3. **arxiv 2603.28052v1 & 2506.13131** — Papers not yet fetched and synthesized.
4. **rotorquant** — Repository inspected but not deeply analyzed; relation to TurboQuant unclear.
5. **andrej-karpathy-skills & playwright-skill** — Skill pattern repositories; need deeper inspection for skill specification patterns.
6. **User feedback deep-dive** — Community sentiment analysis from Reddit, Discord, GitHub issues requires systematic collection.

---

## Conclusion

The existing agent landscape validates ACE's foundational architecture decisions (NATS messaging, tiered memory, hierarchical pods) while providing specific, actionable improvements for nearly every subsystem:

- **Loops:** Add speculative execution and concurrent tool dispatch.
- **Memory:** Adopt OpenViking's filesystem paradigm for L4, Honcho's peer model for attribution.
- **Tools:** Standardize on MCP, auto-generate skills with validation.
- **Delegation:** Enforce depth limits, adopt manager/worker pod topology.
- **Learning:** Capture trajectories as skills, version all learned configurations.
- **Safety:** Implement graded permission modes and safety-classified tool partitioning.

The next documents in this unit will cross-cut all systems across each dimension in depth, producing the detailed comparison files listed in the BRD.
