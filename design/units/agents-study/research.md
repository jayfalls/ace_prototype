# Research: Existing Agents Study

## Methodology

This document synthesizes findings from primary source analysis of 13 systems (7 agent frameworks, 6 specialized systems) and 5 research papers. All listed systems have been analyzed. Research was conducted via direct repository inspection, official documentation, architecture deep-dives, and community discourse analysis. Every claim is traceable to a specific source.

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
| **Oh My OpenAgent** | TypeScript | OpenCode plugin | Open source | Moderate | 11 specialized agents, 46 hooks, 3-layer orchestration |
| **Devin** | Unknown | Cloud (VM-per-instance) | Proprietary | N/A | Hierarchical orchestration, managed sub-agents |

### Specialized Systems

| System | Purpose | Key Relevance to ACE |
|--------|---------|---------------------|
| **pi-mono** | Agent toolkit + vLLM pods | Clean runtime/surface separation |
| **andrej-karpathy-skills** | Prompt-pattern skill library | Skill-as-instruction-pattern (agentskills.io compatible) |
| **karpathy/autoresearch** | Autonomous ML experiment loop | Self-improving code via fixed-time experiments + git rollback |
| **playwright-skill** | Browser automation skill | Model-invoked dynamic code generation skill execution |
| **honcho** | User-context memory library | Entity-centric memory, continual learning |
| **OpenViking** | Context database for agents | Filesystem-paradigm context unification |
| **MSA** | Memory Sparse Attention | End-to-end differentiable long-term memory |

### Research Papers

| Paper | Institution | Core Contribution |
|-------|-------------|-------------------|
| **TurboQuant** | Google Research | KV cache compression (6x, ~3 bits, near-zero loss) |
| **RLM** | Independent | Recursive Language Models for document analysis |
| **arxiv 2603.28052v1** | Stanford / MIT / KRAFTON | Meta-Harness: automated harness engineering via filesystem-access agent |
| **arxiv 2506.13131** | Google DeepMind | AlphaEvolve: evolutionary coding agent for algorithmic discovery |
| **rotorquant** | scrya-com | Clifford algebra KV cache compression (beats TurboQuant on PPL, speed, params) |

---

## Dimension 1: Core Agent Loop Architecture

### Findings

**OpenClaw** uses a dual-loop design: an outer loop selects the next task; an inner loop executes it via ReAct (Reason + Act). Runs are serialized per session key to prevent race conditions. The loop emits lifecycle and stream events throughout. Auto-compaction triggers retry with reset buffers.

**Claude Code** drives the entire system through an `AsyncGenerator` in `query.ts`. The loop streams model output, executes tools, recovers from errors, and compresses context. A key innovation is **speculative execution**: read-only tools start during model streaming before the response completes. Tools are partitioned by safety classification — reads run in parallel, writes serialize.

**Hermes Agent** explicitly rejects ReAct. Its `AIAgent` class (~10,700 lines) uses **delegation** as the primary multi-step mechanism: `tools/delegate_tool.py` spawns child agents with isolated context, restricted toolsets, and max depth of 2. For reasoning it relies on Claude 4.6+ adaptive thinking with configurable `reasoning_effort`. Multiple tool calls execute concurrently via `ThreadPoolExecutor`.

**Goose** follows a standard conversation loop: load extensions → assemble system prompt → stream LLM → dispatch tools → feed results back. Subagents are first-class: Goose autonomously decides to spawn them in autonomous permission mode. Recipes define reusable subagent configurations.

**Devin** has evolved from single-agent to **distributed orchestrator**. A manager Devin decomposes tasks and delegates to up to 10 worker Devins, each in an isolated VM. The manager scopes work, monitors progress, resolves conflicts, and compiles results.

**pi-mono** makes the loop architecture explicit: `pi-agent-core` holds the turn loop (context shaping → LLM call → tool execution → continuation decision). The same core powers CLI, TUI, web UI, and Slack bot surfaces.

**Open Code** uses a client/server architecture where the agent runs on the host and the TUI (or any client) connects remotely. It has two built-in agent modes — `plan` (read-only, denies file edits, asks permission before bash) and `code` (default) — switchable via `Tab`. Permissions filter tools **before** the model sees them, then check again at execution time. A general subagent (`@general`) handles complex multistep searches. The orchestration model merges native agents (defined in code) and file-based agents into the same registry, executed through the same prompt, permission, and session pipeline.

**Oh My OpenAgent** (an OpenCode plugin, 160k LOC) uses a **three-layer architecture**: Planning Layer (Prometheus planner + Metis consultant + Momus reviewer), Execution Layer (Atlas orchestrator), and Worker Layer (specialized agents: Sisyphus-Junior, Oracle, Explore, Librarian, Frontend, etc.). An **Intent Gate** classifies user requests (research, implementation, investigation, fix) before routing. Prometheus generates plans written to `.sisyphus/plans/*.md`; Momus reviews and returns OKAY/REJECT. Atlas reads the plan and delegates tasks to workers by **category** (`visual-engineering`, `ultrabrain`, `quick`, `deep`), which automatically maps to the right model. Background agents run in parallel. The system uses 46 lifecycle hooks across 5 tiers (Session → Tool-Guard → Transform → Continuation → Skill).

**karpathy/autoresearch** is a 630-line Python autonomous experiment loop for single-GPU ML research. It has **three fixed roles**: human edits `program.md` (research strategy / "research org code"), AI agent edits `train.py` (the only file it touches), and fixed infrastructure lives in `prepare.py`. The loop: agent reads `program.md` + current `train.py` + `results.tsv` history → proposes a code change → commits to git → runs training for exactly 5 minutes (fixed wall-clock budget) → measures `val_bpb` → keeps commit if improved, `git reset` if not. Error handling feeds crash logs back to the agent for revision. The **"NEVER STOP"** principle requires the agent to continue autonomously until manually interrupted. The fixed time budget makes experiments directly comparable regardless of what the agent changes (model size, batch size, architecture). Shopify adapted it internally and reported a 19% validation improvement. Karpathy frames the outer loop as optimizing `program.md` (human) while the inner loop optimizes `train.py` (agent).

### Trade-offs

| Approach | Pros | Cons |
|----------|------|------|
| **ReAct (OpenClaw, Claude Code)** | Simple, well-understood, easy to debug | Reasoning contaminated by execution details, context bloat |
| **Delegation (Hermes, Devin, Goose)** | Clean separation, parallel execution, failure isolation | Higher overhead, complex context merging, depth limits |
| **AsyncGenerator (Claude Code)** | Natural backpressure, cancellation, streaming | Tight coupling of streaming and execution logic |
| **Dual-loop (OpenClaw)** | Planning separated from execution | Outer loop can become bottleneck |
| **Client/server (Open Code)** | Remote driving, surface independence | Adds network complexity, latency |
| **Three-layer + Intent Gate (OmO)** | Clean separation, model-agnostic routing | 160k LOC plugin, high complexity |
| **Fixed-time experiment loop (autoresearch)** | Comparable experiments, git-based rollback, autonomous | Single-threaded, no parallelism, domain-specific |

### ACE Recommendation: **Adapt**

ACE's six cognitive layers are already a form of structural delegation — each layer handles a different abstraction level. The evidence strongly supports:

1. **Adopt** Claude Code's speculative execution for L6 (Task Prosecution) — starting read-only tools while the model streams is a zero-cost latency win.
2. **Adopt** Hermes' concurrent tool execution via worker pools within a layer loop.
3. **Adapt** Devin's hierarchical pod structure for swarm coordination — the manager/worker pattern maps directly to ACE's pod tree.
4. **Adopt** Oh My OpenAgent's **Intent Gate** concept for ACE's Senses layer — classify incoming requests by type before routing to the appropriate cognitive layer.
5. **Adopt** Oh My OpenAgent's **category-based model routing** for the Providers unit — map task categories to model capabilities rather than hardcoding model assignments.
6. **Adopt** Karpathy's autoresearch **fixed-time experiment budget** for ACE's Learning Loop evaluations — when testing prompt/harness variants, run each for a fixed wall-clock time to ensure comparability across different configurations.
7. **Adopt** Karpathy's autoresearch **git-based commit/rollback** pattern for ACE's Learning Loop — treat every proposed configuration change as a commit; keep if evaluation improves, reset if it degrades or crashes.
8. **Avoid** OpenClaw's single serialized session lane for ACE's core engine; NATS message routing already provides the concurrency primitives we need.
9. **Avoid** Oh My OpenAgent's monolithic plugin approach (160k LOC); ACE's layer separation achieves similar specialization with cleaner boundaries.

---

## Dimension 2: Memory Systems

### Findings

**OpenClaw** uses local Markdown files for memory. Files are compacted when context runs low. The system is "persistent and inspectable, but also fragile." Community members have built elaborate three-layer memory systems on top because retention is unreliable. Memory works through semantic search rather than keyword matching.

**Claude Code** uses file-based memory with an LLM-powered recall system (Sonnet side-query selects relevant memories, not embedding search). Four memory types exist with staleness warnings. The MEM.md file is read at session start and written at session end.

**Hermes Agent** uses three memory layers: Session memory (SQLite + FTS5), Persistent memory (Markdown files with validated workflows), and Honcho user modeling. It uses **on-demand retrieval**, not full-context loading. After tool-heavy turns, it opportunistically captures trajectories as skills.

**Honcho** (used by Hermes) is an entity-centric memory library. It uses peer modeling where both users and agents are "peers." Messages trigger asynchronous reasoning about peer psychology. A `context` endpoint returns combined messages, conclusions, and summaries up to a token limit. Collections organize global user data; Metamessages store flexible session-local context.

**OpenViking** unifies memory, resources, and skills into a **virtual filesystem** (`viking://` protocol). It uses L0/L1/L2 three-tier progressive loading, directory recursive retrieval, and two-stage retrieval (vector search + rerank). The filesystem paradigm replaces fragmented vector storage.

**MSA** (Memory Sparse Attention) is the most radical approach: it integrates retrieval and generation into a single differentiable loop. Document latent states (K/V) are chunk-mean pooled. A router projector selects Top-k documents, and their compressed K/V is concatenated with local K/V for autoregressive decoding. Achieves 100M token contexts on 2xA800 GPUs with <9% degradation.

**Open Code** has no built-in persistent memory, but the plugin ecosystem has developed sophisticated memory systems. `opencode-working-memory` implements a **four-tier memory architecture**: Core Memory (persistent goal/progress/context blocks that survive compaction), Working Memory (smart slot-based system with guaranteed-visibility slots for errors, decisions, todos, dependencies, plus a ranked pool with exponential decay), Memory Pressure Monitoring (real-time token tracking with automatic interventions at 75%/90%), and Smart Pruning (pressure-aware tool output compression). `open-mem` provides persistent cross-session memory with AI compression into typed observations (decision, bugfix, feature, refactor, discovery, change) with importance scores, progressive disclosure (token-budgeted index injected into system prompt), and 9 memory tools (`memory.find`, `memory.create`, etc.). Typical compression ratio: ~96%.

### Trade-offs

| Approach | Pros | Cons |
|----------|------|------|
| **File-based (OpenClaw, Claude Code)** | Human-inspectable, simple, zero schema | Fragile, no structured querying, race conditions |
| **SQLite + FTS5 (Hermes session)** | Fast, structured, ACID | Not semantic, requires hybrid search |
| **Entity-centric async reasoning (Honcho)** | Rich user models, continual learning | Costly, complex, opaque reasoning process |
| **Virtual filesystem (OpenViking)** | Unified context types, hierarchical retrieval | New paradigm, ecosystem immaturity |
| **Native sparse attention (MSA)** | End-to-end differentiable, massive scale | Requires model architecture changes, hardware-specific |
| **Plugin memory (Open Code)** | Rich ecosystem, specialized solutions | No default memory, fragmentation across plugins |

### ACE Recommendation: **Adapt**

ACE's existing L1-L4 memory architecture is well-positioned. The research validates and refines it:

1. **Adopt** OpenViking's filesystem-paradigm for L4 organization — the URI-based hierarchical structure maps naturally to ACE's tree nodes with parent references.
2. **Adopt** Honcho's entity-centric peer model for cross-agent memory attribution — every memory node should track which agent (peer) created it.
3. **Adopt** MSA's progressive loading concept (L0/L1/L2) for ACE's existing tier structure — rename/combine to align terminology.
4. **Adapt** Claude Code's LLM-powered recall as a secondary retrieval path for L4 — use embedding search for candidate generation, then a lightweight LLM call for relevance scoring.
5. **Adopt** Open Code's `opencode-working-memory` slot-based working memory for ACE's L1 — guaranteed-visibility slots (errors, decisions, todos, dependencies) with exponential decay prevent critical context from being pruned.
6. **Adopt** Open Code's `open-mem` typed observation compression for L2/L3 summaries — structured types with importance scores enable richer retrieval than plain text.
7. **Avoid** pure file-based memory; ACE's SQLite + PostgreSQL hybrid is the correct choice for production reliability.

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

**andrej-karpathy-skills** is not a framework but a **skill-as-prompt-pattern** library. It consists of a single `CLAUDE.md` / `AGENTS.md` file encoding four principles (Think Before Coding, Simplicity First, Surgical Changes, Goal-Driven Execution) and a `SKILL.md` packaged for the agentskills.io standard. The repository demonstrates multi-platform skill packaging: Claude Code plugin, Cursor rule, Codex plugin, and repo-scoped skill discovery. The shared `SKILL.md` is the single source of truth; platform wrappers point at it rather than maintaining copies.

**playwright-skill** is a Claude Code Skill for browser automation that demonstrates **model-invoked dynamic skill execution**. The agent does not call pre-built tools; it autonomously writes custom Playwright code for each request, then executes it via a universal executor (`run.js`). The skill uses progressive disclosure (concise `SKILL.md` with full `API_REFERENCE.md` loaded only when needed), safe cleanup, and auto-detection of running dev servers. It implements the open Agent Skills specification. This pattern shows that skills can be "generative" — the skill provides the capability to generate code, not just static scripts.

**Oh My OpenAgent** has a three-tier MCP system: built-in remote MCPs (websearch via Exa/Tavily, context7, grep_app), Claude Code `.mcp.json` integrations, and skill-embedded MCPs managed by `SkillMcpManager` (stdio + HTTP). It also has 26 custom tools across 15 directories and a skill/command system. Tools are created via factory pattern and registered in a `ToolRegistry`.

### Trade-offs

| Approach | Pros | Cons |
|----------|------|------|
| **MCP standard (Goose)** | Interoperability, large ecosystem | Dependency on external standard evolution |
| **Self-registering tool files (Hermes, OpenClaw)** | Simple to add, discoverable | No sandboxing by default, security risk |
| **Auto-generated skills (Hermes)** | Captures emergent workflows | Quality varies, requires validation |
| **Recipe templates (Goose)** | Reusable, shareable | Static, not adaptive |
| **Skill-as-prompt-pattern (Karpathy)** | Simple, portable, human-readable | No execution logic, purely instructional |
| **Model-invoked dynamic skill (Playwright)** | Handles arbitrary tasks within domain | Requires runtime code generation, security review needed |
| **Three-tier MCP (OmO)** | Flexible integration at multiple layers | Complex management, potential conflicts across tiers |

### ACE Recommendation: **Adapt**

1. **Adopt** MCP as the primary tool interface standard — ACE's tool system should speak MCP natively for ecosystem compatibility.
2. **Adopt** Hermes' YAML-frontmatter skill format for ACE's internal skill definitions.
3. **Adapt** Hermes' auto-skill-generation for the Learning Loop — but require human review or automated validation before activation.
4. **Adopt** andrej-karpathy-skills' **skill-as-prompt-pattern** approach for ACE's layer prompts — encode cognitive principles (Think Before Acting, Simplicity First, etc.) as versioned prompt skills rather than hardcoded system text.
5. **Adapt** playwright-skill's **model-invoked dynamic execution** for ACE's L6 tool system — when a static tool is insufficient, the agent can generate and execute code within a sandboxed environment.
6. **Adopt** Oh My OpenAgent's **three-tier MCP integration** (built-in + config + skill-embedded) for ACE's tool discovery pipeline.
7. **Avoid** OpenClaw's unverified skill marketplace model; ACE should require cryptographically signed skills or sandboxed execution.

---

## Dimension 5: Multi-Agent Delegation & Orchestration

### Findings

**Devin** uses explicit manager/worker hierarchy: 1 manager + up to 10 workers, each in isolated VMs. The manager distributes tasks, monitors progress, resolves conflicts, merges changes. Workers are state-contained; failures don't cascade.

**Goose** supports subagents via natural language delegation. Internal subagents inherit the parent's context and extensions (optionally restricted). External subagents integrate via MCP. Goosetown (community project) implements "flocks" — researcher, writer, worker, reviewer roles with a "Town Wall" broadcast channel.

**Hermes Agent** uses `delegate_tool.py` to spawn child agents with isolated context, restricted toolsets, and max depth of 2. Parent blocks until children return summaries. No recursive delegation allowed in children.

**OpenClaw** has subagent support but the gateway is fundamentally single-agent-per-session.

**Oh My OpenAgent** implements the most explicit **specialized-agent delegation** pattern studied. It has 11 built-in agents with rigid role separation: Prometheus (strategic planning), Atlas (todo orchestration and execution), Oracle (architecture consultation), Librarian (documentation/code search), Explore (fast codebase grep), Sisyphus-Junior (task executor), and others. The execution flow is: Intent Gate classifies request → Prometheus plans (with Metis consultation and Momus review) → Atlas executes by distributing tasks to specialized subagents → results accumulate back to Atlas. Atlas independently verifies completion. The system supports **parallel background execution** — research, implementation, and verification happen simultaneously.

**Open Code** merges native agents (defined in code) and file-based agents into the same registry. Its built-in `plan` and `code` agents represent a lightweight specialization pattern, and `@general` provides on-demand subagent invocation for complex searches.

### Trade-offs

| Approach | Pros | Cons |
|----------|------|------|
| **VM isolation (Devin)** | True failure containment, resource boundaries | High overhead, slow spawn, cloud-only |
| **Process isolation (Goose subagents)** | Medium overhead, local execution | Context duplication, sync complexity |
| **In-process delegation (Hermes)** | Fast, low overhead | No true isolation, crash propagation risk |
| **Specialized-agent hierarchy (OmO)** | Clean roles, parallel execution, verification | Rigid, 160k LOC, hard to customize roles |
| **Lightweight mode agents (Open Code)** | Simple, switchable, low overhead | Only two built-in modes, limited specialization |

### ACE Recommendation: **Adapt**

ACE's pod tree architecture is validated by Devin's manager/worker pattern and Goose's flock pattern. Specific recommendations:

1. **Adopt** Devin's explicit manager/worker topology for pod internals — the root pod is the manager, leaf pods are workers.
2. **Adopt** Goose's "Town Wall" concept for inter-pod broadcast — maps to NATS subject fan-out.
3. **Adapt** Hermes' depth-limiting (max 2) for intra-pod delegation to prevent runaway spawning.
4. **Adopt** Oh My OpenAgent's **specialized agent roles** for ACE's cognitive layers — the existing L1-L6 structure maps directly to role specialization (Prometheus → L2/L3, Atlas → L4, Sisyphus-Junior → L6, etc.).
5. **Adopt** Oh My OpenAgent's **independent verification step** (Momus reviewer) for ACE's Safety Monitor — have a separate layer review plans before execution.
6. **Avoid** VM-per-agent overhead; ACE's single-binary model uses process-level isolation via goroutines + bounded worker pools.
7. **Avoid** Oh My OpenAgent's rigid 11-agent hierarchy; ACE's 6 layers plus configurable pod topology achieve the same with fewer fixed roles.

---

## Dimension 6: Self-Improvement & Learning Loops

### Findings

**Hermes Agent** has the most explicit learning loop: after tool-heavy turns, it opportunistically captures trajectories as skills. Skills live in `tools/skill_manager_tool.py` and `tools/skills_hub.py`. The agent edits its own skill files. Persistent memory accumulates validated workflows.

**Devin** creates and improves **playbooks** — reusable session templates. It analyzes session outcomes, identifies patterns, extracts learnings, and refines playbooks based on feedback. Also manages knowledge (deduplicate, consolidate, create entries).

**Claude Code** has no explicit self-improvement loop in the open-source SDK; improvement comes from model updates and CLAUDE.md project-specific instructions.

**OpenClaw** has heartbeat rules (`HEARTBEAT.md`) and scheduled tasks, but no systematic learning mechanism.

**Research: RISE** (Recursive IntroSpEction) fine-tunes LLMs to self-improve over multiple turns. It formulates fine-tuning as a multi-turn MDP and uses online imitation learning + reward-weighted supervised learning. Demonstrates that even 7B models can improve themselves sequentially on reasoning tasks.

**AlphaEvolve** (Google DeepMind, arxiv 2506.13131) is an **evolutionary coding agent** for scientific and algorithmic discovery. It orchestrates an autonomous pipeline of LLMs whose task is to improve an algorithm by making direct changes to code. It uses an evolutionary approach with continuous feedback from one or more evaluators, iteratively improving the algorithm. Demonstrated results: optimizing data center scheduling algorithms, finding functionally equivalent simplifications in hardware accelerator circuit design, accelerating LLM training, and discovering a 4×4 complex matrix multiplication algorithm using 48 scalar multiplications — the first improvement over Strassen's algorithm in 56 years.

**Meta-Harness** (Stanford/MIT/KRAFTON, arxiv 2603.28052v1) is an **outer-loop system that searches over harness code** for LLM applications. It uses a coding-agent proposer (Claude Code Opus-4.6) with access to a growing filesystem containing all prior candidates' source code, scores, and execution traces. The proposer queries the filesystem via grep/cat rather than ingesting everything as a single prompt. Key insight: access to **raw execution traces** (not compressed summaries) is the key ingredient — scores-only reaches 34.6 median accuracy, scores-plus-summary reaches 34.9, while full Meta-Harness reaches 50.0. On TerminalBench-2, discovered harnesses surpass hand-engineered agents. A search run completes in a few hours of wall-clock time.

**karpathy/autoresearch** is the most minimal self-improvement system studied: 630 lines of Python where an AI agent autonomously edits `train.py`, runs a 5-minute training experiment, and keeps or discards the change based on `val_bpb`. The human never touches Python; they only edit `program.md` — a Markdown file that serves as the "research org code" giving the agent its strategy. The loop includes error recovery: if training crashes, the agent reads `tail -n 5 run.log`, revises the code, and retries. The **"NEVER STOP"** instruction explicitly prohibits the agent from asking the human whether to continue — it runs indefinitely until interrupted. The outer loop is the human improving `program.md`; the inner loop is the agent improving `train.py`. This creates a **meta-learning stack** where both the research process and its outputs evolve.

### Trade-offs

| Approach | Pros | Cons |
|----------|------|------|
| **Skill capture (Hermes)** | Concrete, inspectable, immediately useful | Requires tool-heavy turns, quality varies |
| **Playbook evolution (Devin)** | Structured, human-reviewable | Cloud-only, proprietary |
| **RL fine-tuning (RISE)** | Fundamental capability improvement | Expensive, requires training infra, model-specific |
| **Evolutionary code search (AlphaEvolve)** | Discovers novel algorithms, provably correct | Requires evaluators, domain-specific, high compute |
| **Harness auto-optimization (Meta-Harness)** | Improves harness code, generalizes across models | Requires strong coding-agent proposer, hours per run |
| **Fixed-time experiment loop (autoresearch)** | Minimal code, autonomous overnight, comparable results | Single-threaded, no population diversity, crashes waste time |

### ACE Recommendation: **Adapt**

1. **Adopt** Hermes' trajectory-to-skill capture for ACE's Learning Loop — when L6 completes a multi-tool task successfully, extract a skill template.
2. **Adapt** Devin's playbook concept into ACE's **immutable configuration records** — each learned configuration is a new versioned row, not an in-place update.
3. **Adapt** RISE's multi-turn MDP formulation for ACE's layer loop tuning — treat layer configuration as a policy to be improved via feedback signals.
4. **Adopt** Meta-Harness's **filesystem-access harness optimization** for ACE's Learning Loop — store all layer execution traces, scores, and configurations in a queryable filesystem so the Learning Loop can inspect raw history rather than compressed summaries.
5. **Adapt** AlphaEvolve's **evolutionary evaluator pattern** for ACE's self-improvement — maintain a population of prompt/harness variants, evaluate them against task streams, and evolve the Pareto frontier.
6. **Adopt** Karpathy's autoresearch **program.md as "research org code"** for ACE's Learning Loop — the human should edit high-level strategy documents (what to explore, what to avoid, evaluation metrics) while the agent edits low-level configurations autonomously.
7. **Adopt** Karpathy's autoresearch **error-recovery retry loop** for ACE's Learning Loop — when a configuration change causes a crash or degradation, feed the error trace back to the agent and ask for a revision rather than terminating.
8. **Adopt** Karpathy's autoresearch **"NEVER STOP" autonomy principle** for ACE's background global loops (Memory Manager, Learning Loop, Swarm Coordinator) — they should run continuously without polling for human permission, only pausing on explicit interrupt.
9. **Avoid** in-place prompt mutation; ACE's immutability constraint prevents this.

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

**Open Code** provides both a TUI and a **client/server architecture** — the agent runs on the host machine while the TUI (or any other client, including a mobile app) connects remotely. This means the frontend is just one possible client. It has out-of-the-box LSP support and is built by neovim users with a terminal-first philosophy. Configuration is via project-local `CLAUDE.md` / `AGENTS.md` files that merge with global settings.

### Trade-offs

| Approach | Pros | Cons |
|----------|------|------|
| **File-based config (OpenClaw)** | Version-controllable, inspectable | No validation, cryptic errors, high friction |
| **Interactive wizard (Hermes)** | Guided, harder to mess up | Harder to automate, drift from defaults |
| **SDK + events (Claude Code)** | Programmable, testable | Requires code to use |
| **Client/server (Open Code)** | Remote access, multiple clients | Adds network layer, needs auth |
| **Project-local config (Open Code)** | Version-controllable, portable | Requires per-project setup |

### ACE Recommendation: **Adapt**

1. **Adopt** Claude Code's typed event streaming for ACE's Layer Inspector and real-time UI.
2. **Adopt** Goose's profile-based configuration for agent/pod configs.
3. **Adapt** Hermes' multi-surface gateway pattern for ACE's Chat Interface.
4. **Adopt** Open Code's **client/server architecture** for ACE's frontend/backend separation — the cognitive engine runs in the Go backend, the SvelteKit frontend connects via WebSocket/API.
5. **Adopt** Open Code's **project-local config files** (`CLAUDE.md` / `AGENTS.md`) for ACE's per-agent prompt layering — version-controlled, mergeable, inspectable.
6. **Avoid** OpenClaw's raw JSON config approach; ACE should use structured, validated configurations with immutable versioning.

---

## Dimension 9: Safety & Security

### Findings

**OpenClaw** has severe security issues: CVE-2026-25253 (CVSS 8.8) exposed unauthenticated `/api/export-auth` endpoint allowing API key extraction. 135,000+ publicly exposed instances. 12% malware rate in skills. Cisco called it "a security nightmare."

**Claude Code** implements a 14-step permission pipeline: permission resolution, speculative execution, concurrent batching by safety classification. Tools have `isConcurrencySafe()` checks. Read/write partitioning prevents race conditions.

**Hermes Agent** has `tools/approval.py` for dangerous command detection. Subagents get restricted toolsets (no recursive delegation, no `clarify`, no `memory`, no `send_message`).

**Goose** has permission modes: manual approval, smart approval, autonomous, chat-only. Subagents can have restricted extensions.

**Open Code** implements **two-phase permission filtering**: tools are filtered **before** the model sees them (preventing the model from even considering dangerous tools), then checked again at execution time. The `plan` agent mode denies all file edits by default and asks permission before running bash commands.

### ACE Recommendation: **Adapt**

1. **Adopt** Claude Code's safety-classification-based tool partitioning for L6.
2. **Adopt** Goose's graded permission modes (manual → smart → autonomous) as configurable per-agent settings.
3. **Adopt** Hermes' tool restriction for delegated sub-agents.
4. **Adopt** Open Code's **two-phase permission filtering** — filter tool availability before the model sees them, then validate again at execution time. This prevents the model from reasoning about tools it cannot use.
5. **Avoid** OpenClaw's security posture entirely — unauthenticated endpoints and unverified skills are unacceptable for ACE.

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

### Meta-Harness (Stanford / MIT / KRAFTON, arxiv 2603.28052v1)

Meta-Harness is an outer-loop system that searches over harness code for LLM applications. It uses a coding-agent proposer with access to a growing filesystem containing all prior candidates' source code, scores, and execution traces. The proposer queries the filesystem via grep/cat rather than ingesting everything as a single prompt. Key results: +7.7 points on online text classification with 4× fewer tokens; +4.7 points on IMO-level math reasoning across five held-out models; #1 among Haiku 4.5 agents on TerminalBench-2. The critical insight is that access to **raw execution traces** (not compressed summaries) is the key ingredient — scores-only reaches 34.6 median accuracy, scores-plus-summary reaches 34.9, while full Meta-Harness reaches 50.0.

**Implication for ACE:** The Learning Loop should store all layer execution traces, prompts, tool calls, model outputs, and scores in a queryable filesystem (or database) so that future harness optimization can inspect raw history rather than relying on compressed summaries or scalar rewards. ACE's SQLite-backed observability stack is well-positioned for this.

### AlphaEvolve (Google DeepMind, arxiv 2506.13131)

AlphaEvolve is an evolutionary coding agent for scientific and algorithmic discovery. It orchestrates an autonomous pipeline of LLMs to improve algorithms by making direct changes to code, with continuous feedback from evaluators. It discovered novel provably correct algorithms, including a 4×4 complex matrix multiplication using 48 scalar multiplications — the first improvement over Strassen's algorithm in 56 years. It also optimized data center scheduling, simplified hardware accelerator circuit design, and accelerated its own LLM training.

**Implication for ACE:** The Learning Loop can adopt an evolutionary population-based approach: maintain a population of layer prompt/harness variants, evaluate them against task streams, and evolve the Pareto frontier. This requires robust evaluators and immutable variant tracking — both align with ACE's existing constraints.

### RotorQuant (scrya-com)

RotorQuant is a drop-in KV cache quantization method that beats Google's TurboQuant on every axis: better perplexity (6.91 vs 7.07), 28% faster decode, 5.3× faster prefill, and 44× fewer parameters. It uses Clifford algebra (geometric algebra) rotors instead of Walsh-Hadamard transforms, achieving 10–19× speedup over TurboQuant's BLAS matmul. It has drop-in llama.cpp integration and works on both CUDA and Apple Silicon (Metal).

**Implication for ACE:** If ACE deploys local inference via vLLM or llama.cpp, RotorQuant is the preferred quantization method over TurboQuant. It enables larger context windows and higher throughput on the same hardware. For cloud-provider inference, this is irrelevant unless the provider adopts it.

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
8. **Client/server architecture** (Open Code) — remote access, surface independence.
9. **Intent Gate classification** (Oh My OpenAgent) — routing by intent reduces misinterpretation.
10. **Model-invoked dynamic skill execution** (playwright-skill) — generative skills handle arbitrary tasks.
11. **Harness auto-optimization** (Meta-Harness) — raw execution traces enable genuine harness improvement.
12. **Evolutionary algorithm discovery** (AlphaEvolve) — population-based search discovers novel solutions.
13. **Slot-based working memory** (Open Code plugins) — guaranteed-visibility slots prevent critical context loss.
14. **Fixed-time experiment budget** (Karpathy autoresearch) — makes experiments comparable regardless of what changes.
15. **Git-based commit/rollback** (Karpathy autoresearch) — immutable experiment history with automatic reversion.
16. **"Research org code" separation** (Karpathy autoresearch) — human edits strategy (program.md), agent edits execution (train.py).

### What Doesn't Work

1. **File-based memory at scale** (OpenClaw) — fragmentation, race conditions, poor retrieval.
2. **Unverified skill marketplaces** (OpenClaw) — security catastrophe.
3. **Raw JSON config without validation** (OpenClaw) — fragile, poor DX.
4. **Monolithic agent loops** (Hermes' 10,700-line `run_agent.py`) — unmaintainable.
5. **No explicit learning loop** (Claude Code SDK, OpenClaw) — agents don't improve with use.
6. **Proprietary cloud-only orchestration** (Devin) — not reproducible, vendor-locked.
7. **Monolithic plugin architecture** (Oh My OpenAgent, 160k LOC) — excessive complexity for the value delivered.
8. **No built-in persistent memory** (Open Code) — forces users to rely on third-party plugins for basic functionality.
9. **Compressed feedback for optimization** (prior text optimizers) — scalar scores and LLM summaries strip diagnostically useful details.
10. **Synchronous single-threaded loops** (Karpathy autoresearch) — no parallelism means experiments are serialized; one crash blocks the queue.
11. **Domain-specific experiment loops** (Karpathy autoresearch) — the 5-minute training loop is tightly coupled to ML training; generalizing to other domains requires redesign.

---

## Gaps Requiring Follow-Up Research

1. **User feedback deep-dive** — Community sentiment analysis from Reddit, Discord, GitHub issues requires systematic collection.
2. **Benchmark comparisons** — Quantitative performance comparisons across frameworks on shared tasks (SWE-bench, TerminalBench, etc.) not yet synthesized.
3. **Cost analysis** — Token usage and infrastructure cost comparisons across delegation patterns, memory systems, and loop architectures not yet performed.

---

## Conclusion

The existing agent landscape validates ACE's foundational architecture decisions (NATS messaging, tiered memory, hierarchical pods) while providing specific, actionable improvements for nearly every subsystem:

- **Loops:** Add speculative execution and concurrent tool dispatch. Adopt Intent Gate classification for Senses.
- **Memory:** Adopt OpenViking's filesystem paradigm for L4, Honcho's peer model for attribution, slot-based working memory for L1.
- **Tools:** Standardize on MCP, auto-generate skills with validation, adopt model-invoked dynamic execution for complex tasks.
- **Delegation:** Enforce depth limits, adopt manager/worker pod topology, add independent verification step.
- **Learning:** Capture trajectories as skills, version all learned configurations, store raw execution traces for harness optimization. Adopt fixed-time experiment budgets, git-based commit/rollback, and "research org code" separation for the Learning Loop.
- **Safety:** Implement graded permission modes, safety-classified tool partitioning, and two-phase permission filtering.
- **UX/DX:** Use client/server architecture, project-local config files, and typed event streaming.

The next documents in this unit will cross-cut all systems across each dimension in depth, producing the detailed comparison files listed in the BRD.
