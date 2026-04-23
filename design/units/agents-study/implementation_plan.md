# Implementation Plan: Existing Agents Study

**Unit:** agents-study  
**Type:** Pure research unit — zero code dependencies  
**Goal:** Produce 14 cross-cutting comparison documents that analyze 13 systems and 5 research papers across dimensional axes, feeding actionable adopt/avoid/adapt recommendations into ACE's upcoming cognitive units.

**Critical Correction:** `research.md` is a high-level MAP, not the deliverable. The Dev Loop performs the actual deep research for each slice by cloning codebases, reading source files, and fetching full papers. Each slice produces exactly one file in `study/`.

**Slice Ordering Principle:** Architecture first (foundational topology), then core runtime dimensions (memory, compaction, loops), then coordination dimensions (delegation, tools-skills, browser-automation, computer-use, communication), then meta dimensions (self-improvement, ux-dx), then synthesis dimensions (strengths-weaknesses, user-feedback), then research-synthesis last (depends on all prior slices).

---

## Systems Under Study (13)

| # | System | Repo / Source |
|---|--------|---------------|
| 1 | **OpenClaw** | `https://github.com/openclaw/openclaw` |
| 2 | **Claude Code** | `https://github.com/chauncygu/collection-claude-code-source-code` |
| 3 | **Open Code** | `https://github.com/anomalyco/opencode` |
| 4 | **Oh My OpenAgent** | `https://github.com/code-yeongyu/oh-my-openagent` |
| 5 | **Devin** | Research required — Cognition's autonomous coding agent |
| 6 | **Goose** | `https://github.com/aaif-goose/goose` |
| 7 | **Hermes Agent** | `https://github.com/nousresearch/hermes-agent` |
| 8 | **pi-mono** | `https://github.com/badlogic/pi-mono` |
| 9 | **andrej-karpathy-skills** | `https://github.com/forrestchang/andrej-karpathy-skills` |
| 10 | **playwright-skill** | `https://github.com/lackeyjb/playwright-skill` |
| 11 | **honcho** | `https://github.com/plastic-labs/honcho` |
| 12 | **OpenViking** | `https://github.com/volcengine/OpenViking` |
| 13 | **MSA** | `https://github.com/EverMind-AI/MSA` |

**Additional Repo for Study:**
| **karpathy/autoresearch** | `https://github.com/karpathy/autoresearch` |

## Research Papers (5)

| # | Paper | Source |
|---|-------|--------|
| 1 | **TurboQuant** | `https://research.google/blog/turboquant-redefining-ai-efficiency-with-extreme-compression/` |
| 2 | **RLM / RISE** | `https://alexzhang13.github.io/blog/2025/rlm/` |
| 3 | **Meta-Harness** (arxiv 2603.28052v1) | `https://arxiv.org/html/2603.28052v1` |
| 4 | **AlphaEvolve** (arxiv 2506.13131) | `https://arxiv.org/abs/2506.13131` |
| 5 | **rotorquant** | `https://github.com/scrya-com/rotorquant` |

---

## Vertical Slices

### Slice 0: Research Environment Setup (NO PR)

**Output File:** None — this is a one-time setup step.

**Backend:** Clone all codebases and fetch all papers into `design/units/agents-study/research/` so subsequent slices do not re-clone.

**Codebases to Clone:**
```bash
mkdir -p design/units/agents-study/research/
cd design/units/agents-study/research/
git clone https://github.com/openclaw/openclaw
git clone https://github.com/chauncygu/collection-claude-code-source-code
git clone https://github.com/anomalyco/opencode
git clone https://github.com/code-yeongyu/oh-my-openagent
git clone https://github.com/aaif-goose/goose
git clone https://github.com/nousresearch/hermes-agent
git clone https://github.com/badlogic/pi-mono
git clone https://github.com/forrestchang/andrej-karpathy-skills
git clone https://github.com/lackeyjb/playwright-skill
git clone https://github.com/plastic-labs/honcho
git clone https://github.com/volcengine/OpenViking
git clone https://github.com/EverMind-AI/MSA
git clone https://github.com/karpathy/autoresearch
git clone https://github.com/scrya-com/rotorquant
```

**Papers to Fetch:**
```bash
mkdir -p design/units/agents-study/research/papers/
cd design/units/agents-study/research/papers/
# TurboQuant blog post (fetch as markdown)
# RLM / RISE blog + paper (fetch as markdown)
# Meta-Harness (arxiv 2603.28052v1)
# AlphaEvolve (arxiv 2506.13131)
# Devin architecture / UX blog posts (fetch as markdown)
# Google Stitch blog posts / docs (fetch as markdown)
# Claude Computer Use, Claude Cowork, Claude Design docs (fetch as markdown)
```

**Frontend:** N/A

**Test:** All repos cloned successfully (no shallow clones — full history needed for blame/annotation). All papers downloaded and readable. Directory structure verified.

---

### Slice 1: `study/architecture.md` — Core Architecture Patterns

**Output File:** `design/units/agents-study/study/architecture.md`

**Codebases to Clone (from Slice 0):**
- `openclaw/openclaw` — inspect gateway process, session routing, adapter pattern, deployment model
- `aaif-goose/goose` — inspect desktop/CLI/API surface separation, extension loading, MCP integration topology
- `nousresearch/hermes-agent` — inspect gateway daemon structure, 15+ surface routing, messaging layer
- `badlogic/pi-mono` — inspect `pi-agent-core` vs surfaces (CLI, TUI, Web, Slack), runtime separation
- `chauncygu/collection-claude-code-source-code` — inspect SDK architecture, AsyncGenerator loop, client/server boundary
- `anomalyco/opencode` — inspect client/server architecture, agent process topology, deployment model
- `code-yeongyu/oh-my-openagent` — inspect three-layer architecture (Planning / Execution / Worker), Intent Gate, 160k LOC structure

**Research to Fetch:**
- Devin architecture blog posts / technical reports (Cognition Labs)

**Investigation Instructions:**
- **OpenClaw:** How is the Gateway structured? What are channel adapters? How are sessions routed and serialized? Single-process or multi-process? Draw the topology.
- **Claude Code:** How does the SDK layer relate to the CLI? What is the boundary between the AsyncGenerator loop and the UI? Is there a server/client split?
- **Open Code:** What is the client/server architecture? How does the agent process relate to the TUI? Can multiple clients connect? What is the deployment topology?
- **Oh My OpenAgent:** What is the three-layer architecture (Planning / Execution / Worker)? How does the Intent Gate fit? Where does the 160k LOC live?
- **Devin:** What is the manager/worker topology? How are VMs allocated? What is the deployment model (cloud-only, on-premise)?
- **Goose:** How are desktop, CLI, and API surfaces layered over the core? What is the extension loading lifecycle? How does MCP fit into the topology?
- **Hermes Agent:** How does the gateway daemon route to 15+ messaging surfaces? What is the process model?
- **pi-mono:** How does `pi-agent-core` remain surface-agnostic? What is the runtime/surface boundary?
- **autoresearch (karpathy):** What is the minimal loop structure? How do `program.md`, `train.py`, and `prepare.py` interact?

**Frontend (Document):** Write cross-cutting comparison of system topology, deployment models, runtime boundaries, surface separation, and design philosophies. Each system gets a dedicated subsection with specific architectural details. Conclude with ACE Recommendation table.

**Test:** Every listed system appears with specific architectural details (not surface summaries); ACE recommendations explicitly map to design choices.

---

### Slice 2: `study/memory.md` — Memory Systems

**Output File:** `design/units/agents-study/study/memory.md`

**Codebases to Clone (from Slice 0):**
- `openclaw/openclaw` — inspect memory file structure, compaction logic, semantic search implementation
- `nousresearch/hermes-agent` — inspect session memory (SQLite + FTS5), persistent memory (Markdown files), Honcho integration
- `plastic-labs/honcho` — inspect entity-centric memory, peer modeling, async reasoning, `context` endpoint, collections, metamessages
- `volcengine/OpenViking` — inspect virtual filesystem (`viking://`), L0/L1/L2 tiers, directory retrieval, two-stage retrieval
- `EverMind-AI/MSA` — inspect differentiable attention integration, chunk-mean pooling, router projector, Memory Parallel
- `chauncygu/collection-claude-code-source-code` — inspect MEM.md, LLM-powered recall, four memory types, staleness warnings
- `anomalyco/opencode` — inspect plugin memory systems (`opencode-working-memory`, `open-mem`) if available in repo or plugin docs

**Research to Fetch:**
- Open Code plugin memory systems: `opencode-working-memory` and `open-mem` repositories / docs
- Claude Code memory SDK docs (official)
- **RLM paper:** `https://alexzhang13.github.io/blog/2025/rlm/` — read full paper on Recursive Language Models. How does recursive processing of long documents relate to memory architecture? What is the recursion depth? How does it manage context windows?

**Investigation Instructions:**
- **OpenClaw:** How are memory files structured? What is the compaction trigger? How does semantic search work vs keyword? What are community workarounds for fragility?
- **Claude Code:** What are the four memory types? How does the LLM-powered recall system work (Sonnet side-query)? What is the MEM.md lifecycle?
- **Hermes Agent:** How is session memory structured in SQLite? What FTS5 queries are used? How does persistent memory (Markdown) differ? How is Honcho integrated?
- **Honcho:** How does entity-centric peer modeling work? What triggers async reasoning? What does the `context` endpoint return? How are collections and metamessages structured?
- **OpenViking:** How does the `viking://` virtual filesystem unify memory/resources/skills? What is L0/L1/L2 progressive loading? How does two-stage retrieval (vector + rerank) work?
- **MSA:** How is retrieval embedded inside Transformer attention? What is chunk-mean pooling? How does the router projector select Top-k? What hardware does it require?
- **Open Code (plugins):** How does `opencode-working-memory` implement four-tier memory? What are smart slots and exponential decay? How does `open-mem` compress into typed observations? What is the progressive disclosure model?
- **RLM:** How does the recursive language model handle document memory? What is the recursive decomposition strategy? How does it compare to embedding-based retrieval (OpenViking) and native sparse attention (MSA)? What implications does it have for ACE's L2/L3 summarization tiers?

**Frontend (Document):** Write cross-cutting comparison of storage backends, retrieval strategies, attribution models, tier structures, consolidation approaches, compression ratios, and recursive memory models.

**Test:** All 13 systems analyzed across memory dimensions; ACE L1–L4 alignment recommendations are explicit and traceable to specific system patterns.

---

### Slice 3: `study/compaction.md` — Context Compaction & Token Budgets

**Output File:** `design/units/agents-study/study/compaction.md`

**Codebases to Clone (from Slice 0):**
- `openclaw/openclaw` — inspect auto-compaction logic, stream events, retry reset behavior
- `nousresearch/hermes-agent` — inspect preflight compression check, ephemeral prompt layers, prompt caching markers
- `chauncygu/collection-claude-code-source-code` — inspect 4-layer compression (snip, microcompact, collapse, autocompact), slot reservation, `compact_boundary` events
- `EverMind-AI/MSA` — inspect how MSA handles 100M-token contexts without compaction (native sparse attention as alternative)

**Research to Fetch:**
- Open Code `opencode-working-memory` plugin source/docs — token pressure monitoring, automatic interventions at 75%/90%, smart pruning

**Investigation Instructions:**
- **Claude Code:** What exactly happens in each of the 4 compression layers? What are the thresholds? How does slot reservation work (8K → 64K escalation)? What events does the SDK emit?
- **OpenClaw:** What triggers auto-compaction? How are in-memory buffers and tool summaries reset on retry? What configuration options exist?
- **Hermes Agent:** What is the preflight compression threshold (>50% context)? What ephemeral prompt layers are injected? How are Anthropic prompt caching markers applied?
- **Open Code (plugins):** How does `opencode-working-memory` monitor token pressure in real time? What automatic interventions happen at 75% and 90%? What is pressure-aware tool output compression?
- **MSA:** How does MSA avoid compaction entirely? What are the tradeoffs of native sparse attention vs graduated compaction?

**Frontend (Document):** Compare compression strategies, token budget management, pressure monitoring, slot reservation, context loss mitigation, and the alternative of avoiding compaction via sparse attention.

**Test:** All compaction strategies documented with technical specifics; ACE 3-tier compaction strategy derived with clear L1→L2→L3→L4 mappings.

---

### Slice 4: `study/loops.md` — Agent Loops & Execution Cycles

**Output File:** `design/units/agents-study/study/loops.md`

**Codebases to Clone (from Slice 0):**
- `openclaw/openclaw` — inspect dual-loop design (outer task loop + inner ReAct loop), lifecycle events, stream events, serialization per session key
- `nousresearch/hermes-agent` — inspect `AIAgent` class loop, delegation-driven multi-step mechanism, concurrent tool execution via `ThreadPoolExecutor`, rejection of ReAct
- `aaif-goose/goose` — inspect standard conversation loop, extension loading → system prompt → stream LLM → dispatch tools → feed results
- `badlogic/pi-mono` — inspect `pi-agent-core` turn loop: context shaping → LLM call → tool execution → continuation decision
- `chauncygu/collection-claude-code-source-code` — inspect `AsyncGenerator` in `query.ts`, speculative execution, error recovery, tool partitioning by safety classification
- `anomalyco/opencode` — inspect `plan` vs `code` agent modes, `@general` subagent, permission filtering before model sees tools
- `code-yeongyu/oh-my-openagent` — inspect Intent Gate → Prometheus plan → Atlas execute → workers loop chain

**Research to Fetch:**
- Karpathy autoresearch loop: `https://github.com/karpathy/autoresearch` — clone and inspect the 630-line experiment loop
- Devin loop architecture (blog posts, technical reports on distributed orchestrator evolution)

**Investigation Instructions:**
- **OpenClaw:** How does the outer loop select tasks vs the inner ReAct loop execute them? What are lifecycle/stream events? How does auto-compaction trigger retry?
- **Claude Code:** How does the `AsyncGenerator` stream model output while executing tools? What is speculative execution (read-only tools during streaming)? How are tools partitioned by safety classification?
- **Hermes Agent:** Why does it reject ReAct? How does the delegation-driven loop work? How does `ThreadPoolExecutor` enable concurrent tool calls? What is the max delegation depth?
- **Goose:** What is the standard conversation loop? How does it decide to spawn subagents in autonomous mode?
- **Devin:** How did the loop evolve from single-agent to distributed orchestrator? How does the manager monitor worker progress?
- **pi-mono:** What is the explicit turn loop structure? How does the same core power multiple surfaces?
- **Oh My OpenAgent:** How does the Intent Gate classify requests before the loop starts? How does Atlas distribute tasks to workers by category? How do background agents run in parallel?
- **Open Code:** What is the loop structure for `plan` vs `code` modes? How does `@general` subagent invocation work?
- **autoresearch:** What is the exact loop: read → propose → commit → run 5 min → measure → keep/reset? How does error recovery work? What is the "NEVER STOP" principle?

**Frontend (Document):** Compare execution cycles, concurrency models, speculative execution, error recovery, iteration patterns, and loop architectures (ReAct, AsyncGenerator, delegation, dual-loop, fixed-time experiment).

**Test:** All loop architectures are mapped; speculative execution and concurrent dispatch patterns are documented with source citations; ACE layer loop design implications are explicit.

---

### Slice 5: `study/delegation.md` — Multi-Agent Delegation & Orchestration

**Output File:** `design/units/agents-study/study/delegation.md`

**Codebases to Clone (from Slice 0):**
- `nousresearch/hermes-agent` — inspect `tools/delegate_tool.py`, child agent spawning, isolated context, restricted toolsets, max depth of 2
- `aaif-goose/goose` — inspect subagent delegation, natural language spawning, internal/external subagents, Goosetown "flocks" and "Town Wall"
- `openclaw/openclaw` — inspect subagent support, gateway single-agent-per-session limitation
- `anomalyco/opencode` — inspect native agents vs file-based agents, `@general` subagent, plan/code mode switching
- `code-yeongyu/oh-my-openagent` — inspect 11 built-in agents, role separation, parallel background execution

**Research to Fetch:**
- Devin manager/worker hierarchy (technical reports, blog posts on VM isolation, up to 10 workers)

**Investigation Instructions:**
- **Devin:** How does the manager distribute tasks? How are up to 10 workers monitored? How are conflicts resolved? What is the VM isolation boundary?
- **Goose:** How do subagents inherit parent context? What is the difference between internal and external subagents? How does the "Town Wall" broadcast work?
- **Hermes Agent:** How does `delegate_tool.py` spawn children? What context is isolated vs shared? What tool restrictions apply? Why max depth of 2?
- **OpenClaw:** What subagent support exists? Why is the gateway fundamentally single-agent-per-session?
- **Oh My OpenAgent:** What are the 11 specialized agents and their roles? How does the Intent Gate → Prometheus → Atlas → Workers chain work? How does Atlas verify completion? How do background agents run in parallel?
- **Open Code:** How do native agents and file-based agents merge into the same registry? What is the `@general` subagent invocation pattern?

**Frontend (Document):** Compare hierarchy models, context isolation, parallel execution, conflict resolution, depth limits, role specialization, and verification patterns.

**Test:** All delegation models covered; ACE pod tree topology mappings (manager/worker, Town Wall, depth limits) are explicit.

---

### Slice 6: `study/tools-skills.md` — Tool & Skill Systems

**Output File:** `design/units/agents-study/study/tools-skills.md`

**Codebases to Clone (from Slice 0):**
- `aaif-goose/goose` — inspect MCP extension loading, recipe system, skill sharing
- `nousresearch/hermes-agent` — inspect 47 registered tools across 19 toolsets, self-registration, YAML-frontmatter skills, auto-generation from trajectories, `tools/skill_manager_tool.py`, `tools/skills_hub.py`
- `openclaw/openclaw` — inspect YAML + Markdown skills folder, skill injection at runtime, marketplace model
- `badlogic/pi-mono` — inspect `ToolDefinition` registry, custom rendering functions, built-in tool lifecycle
- `forrestchang/andrej-karpathy-skills` — inspect `SKILL.md` packaging, agentskills.io compatibility, multi-platform wrappers
- `lackeyjb/playwright-skill` — inspect model-invoked dynamic execution, `run.js` universal executor, progressive disclosure, `SKILL.md` + `API_REFERENCE.md`
- `code-yeongyu/oh-my-openagent` — inspect three-tier MCP system, built-in remote MCPs, `.mcp.json` integrations, `SkillMcpManager`, `ToolRegistry` factory pattern
- `anomalyco/opencode` — inspect two-phase permission filtering, `plan` mode restrictions

**Research to Fetch:**
- Claude Code tool classification and permission pipeline

**Investigation Instructions:**
- **Goose:** How does MCP standard integration work? How many extensions are available? What is a recipe? How are skills shared?
- **Hermes Agent:** How do tools self-register at import time? What does the YAML frontmatter contain? How does auto-generation from trajectories work? What is the agentskills.io standard?
- **OpenClaw:** How are skills structured? How are they injected at runtime? What are community marketplace patterns? What are the security findings (Cisco report)?
- **pi-mono:** How does `ToolDefinition` registry work? What are custom rendering functions?
- **Karpathy skills:** How is a skill packaged as a single `SKILL.md`? What are the four principles encoded? How do platform wrappers point to the same source?
- **playwright-skill:** How does the agent generate custom Playwright code per request? How does the universal executor work? What is progressive disclosure?
- **Oh My OpenAgent:** How does the three-tier MCP system work? What is the `ToolRegistry` factory pattern? How are skill-embedded MCPs managed?
- **Claude Code:** How are tools classified by safety? What is the 14-step permission pipeline?
- **Open Code:** How does two-phase permission filtering work? What tools are hidden from the model in `plan` mode?

**Frontend (Document):** Compare tool discovery, execution models, sandboxing, interoperability (MCP), auto-generation, security models, and skill packaging standards.

**Test:** All tool/skill models covered; MCP adoption recommendation is explicit; security warnings (OpenClaw malware rate) are documented with evidence.

---

### Slice 7: `study/browser-automation.md` — Browser Automation & Frontend Testing

**Output File:** `design/units/agents-study/study/browser-automation.md`

**Codebases to Clone (from Slice 0):**
- `lackeyjb/playwright-skill` — inspect model-invoked dynamic browser automation, `run.js` executor, progressive disclosure
- `badlogic/pi-mono` — inspect web surface capabilities, how the agent interacts with browser contexts
- `anomalyco/opencode` — inspect browser tools, any built-in web automation or testing capabilities
- `aaif-goose/goose` — inspect browser extensions, MCP-based web tools
- `chauncygu/collection-claude-code-source-code` — inspect browser capabilities, any web-based tool execution

**Research to Fetch:**
- **Google Stitch** — search and fetch docs/blog posts on Google's browser/UI generation tool. How does it generate UIs from prompts? What is its automation model?
- **Devin browser use** — how does Devin interact with browsers in its VM environment?
- **Claude Code browser tools** — any official documentation on browser-based tool usage

**Investigation Instructions:**
- **playwright-skill:** How does the agent generate custom Playwright code per request? What is the `run.js` universal executor? How does it handle safe cleanup and auto-detection of dev servers?
- **pi-mono:** Does the web surface enable browser automation? How does the agent core interact with web contexts?
- **Open Code:** Are there built-in browser tools? How does the agent handle web-based tasks?
- **Goose:** What browser extensions exist? How does MCP enable web tool interoperability?
- **Claude Code:** Are there browser-based tools? How does the SDK handle web automation?
- **Google Stitch:** What is the architecture? How does it generate or manipulate browser UIs? What primitives does it expose?
- **Devin:** How does it use browsers within its VM? Is there a dedicated browser automation skill?
- **Cross-system:** Compare approaches: dynamic code generation (playwright-skill) vs MCP extensions (Goose) vs UI generation (Google Stitch) vs VM-based browsing (Devin).

**Frontend (Document):** Compare browser automation strategies, frontend testing capabilities, web tool interoperability, UI generation vs automation, and security models for web execution.

**Test:** All systems with browser capabilities are analyzed; Google Stitch is documented; ACE recommendations for web tool integration are explicit.

---

### Slice 8: `study/computer-use.md` — Computer Use & Desktop Automation

**Output File:** `design/units/agents-study/study/computer-use.md`

**Codebases to Clone (from Slice 0):**
- `aaif-goose/goose` — inspect desktop app, OS-level automation capabilities
- `badlogic/pi-mono` — inspect desktop surface capabilities, any OS interaction primitives
- `chauncygu/collection-claude-code-source-code` — inspect any computer-use or desktop automation features

**Research to Fetch:**
- **Claude Computer Use** — Anthropic's computer-use API/docs. How does it control GUIs, screen coordinates, mouse/keyboard?
- **Claude Cowork** — search for this product. How does it differ from Computer Use? What is its collaboration model?
- **Claude Design** — search for this product. How does it handle design/UI tasks?
- **Devin VM desktop** — how does Devin interact with the desktop environment in its VM? Screen recording, GUI automation?

**Investigation Instructions:**
- **Claude Computer Use:** What is the API? How are screen coordinates mapped? What OS-level primitives are exposed (screenshot, click, type, scroll)? How is safety handled?
- **Claude Cowork:** What is the product? How does it enable multi-agent or human-agent collaboration on desktop tasks?
- **Claude Design:** What is the scope? How does it handle design tools (Figma, etc.)?
- **Goose desktop:** What desktop automation does the Goose desktop app provide? Is it limited to terminal or does it interact with native OS APIs?
- **pi-mono desktop:** What does the desktop surface enable? Any OS-level tool integration?
- **Devin:** How does the VM desktop work? Can it see the screen? Does it use VNC, RDP, or native APIs?
- **Cross-system:** Compare screen-coordinate-based control (Claude) vs API-based desktop integration (Goose/pi-mono) vs VM sandboxing (Devin). Which approach aligns with ACE's single-binary model?

**Frontend (Document):** Compare desktop automation strategies, GUI interaction models, OS-level primitives, sandboxing approaches, and security implications.

**Test:** Claude Computer Use, Cowork, and Design are all documented; Devin VM desktop is analyzed; ACE recommendations for desktop tool integration are explicit.

---

### Slice 9: `study/communication.md` — Internal Communication Patterns

**Output File:** `design/units/agents-study/study/communication.md`

**Codebases to Clone (from Slice 0):**
- `openclaw/openclaw` — inspect Gateway message routing, channel adapters, normalization, session routing
- `aaif-goose/goose` — inspect MCP tool communication, "Town Wall" broadcast channel in Goosetown
- `badlogic/pi-mono` — inspect runtime/surface separation, how `pi-agent-core` communicates with CLI/TUI/Web/Slack
- `nousresearch/hermes-agent` — inspect gateway daemon routing to 15+ surfaces
- `anomalyco/opencode` — inspect client/server protocol, TUI-to-agent communication
- `code-yeongyu/oh-my-openagent` — inspect inter-layer communication, 46 lifecycle hooks propagation

**Research to Fetch:**
- Devin inter-VM communication — how does the manager communicate with worker VMs?

**Investigation Instructions:**
- **OpenClaw:** How do messages arrive via channel adapters? What is normalized? How are they routed to sessions? Is the Gateway a bottleneck?
- **Goose:** How does MCP handle tool communication? What is the "Town Wall" broadcast? How do flocks communicate?
- **pi-mono:** How does the runtime communicate with surfaces? What protocol is used? Is it bidirectional?
- **Hermes Agent:** How does the gateway route to 15+ surfaces simultaneously? What is the messaging protocol?
- **Open Code:** What protocol connects the TUI to the agent server? Is it WebSocket, HTTP, or something else?
- **Oh My OpenAgent:** How do the three layers communicate? What are the 46 lifecycle hooks and how do they propagate?
- **Devin:** How does the manager communicate with workers across VM boundaries?
- **ACE (existing):** Document how NATS with typed subjects and JetStream compares to all of the above.

**Frontend (Document):** Compare message passing, context propagation, broadcast patterns, surface independence, protocol choices, and validate ACE's NATS decision with comparative evidence.

**Test:** All communication architectures covered; validates existing ACE NATS decision with comparative evidence.

---

### Slice 10: `study/self-improvement.md` — Self-Improvement & Learning Loops

**Output File:** `design/units/agents-study/study/self-improvement.md`

**Codebases to Clone (from Slice 0):**
- `nousresearch/hermes-agent` — inspect trajectory-to-skill capture, `tools/skill_manager_tool.py`, `tools/skills_hub.py`, agent editing its own skill files
- `karpathy/autoresearch` — inspect `program.md`, `train.py`, `prepare.py`, full 630-line experiment loop

**Research to Fetch:**
- **RLM / RISE paper:** `https://alexzhang13.github.io/blog/2025/rlm/` — read full paper on Recursive IntroSpEction, multi-turn MDP, online imitation learning
- **AlphaEvolve paper:** `https://arxiv.org/abs/2506.13131` — read full paper on evolutionary coding agent, evaluators, algorithmic discovery
- **Meta-Harness paper:** `https://arxiv.org/html/2603.28052v1` — read full paper on harness auto-optimization, filesystem-access agent, raw execution traces
- **Devin playbook system** — blog posts / technical reports on playbooks, knowledge management, deduplication

**Investigation Instructions:**
- **Hermes Agent:** How does it capture trajectories as skills after tool-heavy turns? What is the skill quality like? How does persistent memory accumulate validated workflows?
- **Devin:** How are playbooks created and improved? How does it analyze session outcomes? What is the knowledge management pipeline?
- **RISE:** How is fine-tuning formulated as a multi-turn MDP? What is online imitation learning + reward-weighted supervised learning? What model sizes were tested?
- **AlphaEvolve:** How does the evolutionary pipeline work? What evaluators are used? What were the concrete results (matrix multiplication, data center scheduling)? How is the population maintained?
- **Meta-Harness:** How does the coding-agent proposer access the filesystem? Why are raw execution traces critical (34.6 → 50.0 accuracy)? How long does a search run take?
- **autoresearch:** What is the exact 630-line loop? How does `program.md` serve as "research org code"? How does the 5-minute fixed budget work? How does git commit/rollback work? What is the error recovery pattern?
- **OpenClaw:** What are heartbeat rules and scheduled tasks? Is there any systematic learning?
- **Claude Code SDK:** Is there any explicit self-improvement loop?

**Frontend (Document):** Compare feedback loops, configuration evolution, harness optimization, experiment budgets, git-based rollback, population-based search, RL fine-tuning, and meta-learning stacks.

**Test:** All learning mechanisms covered; ACE Learning Loop design recommendations (trajectory-to-skill, fixed-time budgets, raw trace storage, evolutionary evaluator) are explicit.

---

### Slice 11: `study/ux-dx.md` — Developer Experience & User Interfaces

**Output File:** `design/units/agents-study/study/ux-dx.md`

**Codebases to Clone (from Slice 0):**
- `openclaw/openclaw` — inspect `openclaw.json`, `SOUL.md`, `HEARTBEAT.md` configuration files, setup friction
- `nousresearch/hermes-agent` — inspect interactive wizard (~3,100 lines), multi-surface gateway setup
- `chauncygu/collection-claude-code-source-code` — inspect SDK typed events, CLI vs headless modes
- `aaif-goose/goose` — inspect desktop app, CLI/TUI, API surfaces, `~/.config/goose/` profiles, multi-provider support
- `anomalyco/opencode` — inspect TUI, client/server remote access, LSP support, `CLAUDE.md` / `AGENTS.md` project-local config
- `code-yeongyu/oh-my-openagent` — inspect OpenCode plugin experience, 46 hooks configuration

**Research to Fetch:**
- Devin UX/DX: cloud VM interface, manager dashboard, worker monitoring UI, onboarding flow (blog posts, demo videos, reviews)
- Community sentiment on setup friction (Reddit, Discord, GitHub issues for OpenClaw, Hermes, Goose)

**Investigation Instructions:**
- **OpenClaw:** What configuration files exist? What is the setup process? What are common error reports? How long does setup take?
- **Claude Code:** What typed events does the SDK emit? How does the CLI differ from headless SDK mode? What UI construction is possible?
- **Goose:** How many providers are supported? What is the profile system? How does configuration work across desktop/CLI/API?
- **Hermes Agent:** How does the interactive wizard work? What surfaces are supported? What is the gateway daemon setup process?
- **Open Code:** What is the TUI experience? How does remote driving work? What is the LSP integration? How do `CLAUDE.md` / `AGENTS.md` files merge with global settings?
- **Oh My OpenAgent:** What is the plugin installation process? How are the 46 hooks configured?
- **pi-mono:** How do CLI, TUI, Web, and Slack surfaces differ in UX?
- **Devin:** What does the cloud VM interface look like? How does the manager dashboard display worker progress? What is the onboarding experience? How do users interact with the VM desktop? What are the reported UX pain points from early access users?

**Frontend (Document):** Compare setup friction, configuration models, debugging surfaces, multi-client support, developer onboarding, cloud VM interfaces, and project-local vs global configuration.

**Test:** All UX patterns covered, including Devin; ACE frontend/backend separation and project-local config recommendations are explicit.

---

### Slice 12: `study/strengths-weaknesses.md` — Comparative Strengths & Weaknesses

**Output File:** `design/units/agents-study/study/strengths-weaknesses.md`

**Dependencies:** Requires completion of Slices 1–11 (all dimensional slices before synthesis).

**Codebases:** No new clones required — synthesize from prior slice research.

**Research to Fetch:**
- Systematic collection of specific claims from prior slices.
- Cross-reference with `research.md` map for any gaps.

**Investigation Instructions:**
- Synthesize findings from Slices 1–11 into two structured lists: **What Works** and **What Doesn't Work**.
- For each item, identify the specific system(s), the dimensional source (architecture, memory, etc.), and the evidence (source file, issue number, quote).
- Map each finding to an ACE decision: **adopt**, **avoid**, or **adapt**.
- Identify contradictions between systems (e.g., one system's strength is another's weakness).
- Highlight patterns that appear across multiple systems (convergent evolution) vs unique innovations.

**Frontend (Document):** Structured list of strengths and weaknesses with per-system evidence, dimensional tags, and ACE adopt/avoid/adapt mapping for every item. Include convergence/unique innovation analysis.

**Test:** Every strength/weakness traces to a specific system and dimension; no vague claims; ACE mapping present for every item.

---

### Slice 13: `study/user-feedback.md` — Community User Feedback

**Output File:** `design/units/agents-study/study/user-feedback.md`

**Dependencies:** Best done after Slice 12 (strengths-weaknesses) to guide targeted sentiment search, but can run in parallel if resources allow.

**Codebases:** No new clones required.

**Research to Fetch (Systematic Web Research):**
- **GitHub Issues:** Search `repo:openclaw/openclaw`, `repo:aaif-goose/goose`, `repo:nousresearch/hermes-agent`, `repo:badlogic/pi-mono`, `repo:plastic-labs/honcho`, `repo:volcengine/OpenViking`, `repo:EverMind-AI/MSA`, `repo:anomalyco/opencode`, `repo:code-yeongyu/oh-my-openagent` for labels like `bug`, `feature request`, `help wanted`, `security`
- **Reddit:** Search r/LocalLLaMA, r/artificial, r/MachineLearning for threads on Claude Code, Devin, OpenClaw, Goose, Open Code, Oh My OpenAgent
- **Discord:** Search official servers (Goose, Hermes, OpenClaw) for common complaints and praise
- **Hacker News:** Search for Show HN and discussion threads on each system
- **Security Reports:** CVE-2026-25253 details for OpenClaw, Cisco security assessment

**Investigation Instructions:**
- For each open-source system, collect at least 5 specific pieces of feedback (complaints or praise) with direct links/quotes.
- Categorize feedback by dimension: setup, memory, tools, delegation, safety, performance, cost, UX.
- Identify recurring themes across systems (e.g., "file-based memory is fragile" appears for multiple systems).
- Document specific numbers where available: setup time reports, token cost complaints, performance benchmarks from users.
- For proprietary systems (Claude Code, Devin), collect feedback from public reviews, Twitter, Hacker News, blog posts.

**Frontend (Document):** Documented feedback with evidence (links, quotes, issue numbers). Synthesize patterns across systems. Align with strengths/weaknesses findings.

**Test:** Every claim has a citation or traceable source; patterns are synthesized (not just a list of quotes); aligns with strengths/weaknesses findings.

---

### Slice 14: `study/research-synthesis.md` — Research Papers Synthesis

**Output File:** `design/units/agents-study/study/research-synthesis.md`

**Dependencies:** Requires completion of all prior dimensional slices (1–11) and Slice 10 (self-improvement, which covers AlphaEvolve, Meta-Harness, RISE).

**Research to Fetch (Full Papers):**
- **TurboQuant:** `https://research.google/blog/turboquant-redefining-ai-efficiency-with-extreme-compression/` — read full technical details, numbers (6x memory, 8x speedup, ~3 bits, near-zero loss)
- **RLM / RISE:** `https://alexzhang13.github.io/blog/2025/rlm/` — read full paper on recursive language models and self-improvement
- **Meta-Harness:** `https://arxiv.org/html/2603.28052v1` — read full paper, extract harness engineering details, accuracy numbers (34.6 vs 34.9 vs 50.0), TerminalBench-2 results
- **AlphaEvolve:** `https://arxiv.org/abs/2506.13131` — read full paper, extract evolutionary pipeline, evaluator design, concrete results (matrix multiplication, data center scheduling, circuit design, LLM training acceleration)
- **rotorquant:** `https://github.com/scrya-com/rotorquant` — read README, benchmark numbers (PPL 6.91 vs 7.07, 28% faster decode, 5.3x faster prefill), Clifford algebra approach, llama.cpp integration

**Investigation Instructions:**
- **TurboQuant:** What is the two-stage approach (PolarQuant + QJL)? What does data-oblivious mean? What hardware was tested? What are the exact compression ratios and accuracy tradeoffs?
- **MSA:** How does Memory Sparse Attention achieve 100M tokens on 2xA800 GPUs? What is chunk-mean pooling? What is the router projector? What is Memory Parallel?
- **RLM / RISE:** How does the recursive language model work? What is the multi-turn MDP formulation? What were the model sizes and tasks? What is the self-improvement trajectory?
- **Meta-Harness:** How does the outer-loop search work? What is the filesystem-access pattern? Why are raw execution traces the key ingredient? What are the exact accuracy numbers? How long does a search run take?
- **AlphaEvolve:** How does the evolutionary pipeline orchestrate LLMs? What evaluators are used? What are the concrete scientific results? How is the population maintained? What is the mutation/crossover strategy?
- **rotorquant:** How does Clifford algebra rotor quantization work? How does it compare to TurboQuant on every axis? What is the llama.cpp integration status? What hardware is supported?
- **Cross-paper synthesis:** How do TurboQuant and rotorquant relate to ACE's memory/Providers units? How do AlphaEvolve and Meta-Harness relate to ACE's Learning Loop? How does MSA relate to ACE's long-term memory architecture?

**Frontend (Document):** Detailed per-paper technical summary followed by explicit ACE implication. Include TurboQuant vs RotorQuant comparison table. Conclude with cross-paper synthesis and prioritized recommendations for ACE units.

**Test:** Every paper has a technical summary and explicit ACE implication; no vague recommendations; quantitative claims are sourced.

---

## Unit-Level Success Criteria

1. All 14 deliverable files exist in `design/units/agents-study/study/`.
2. Every listed system is analyzed across every relevant output dimension.
3. Each file contains specific, comparable design decisions (not surface summaries).
4. Findings explicitly map to ACE design choices: **adopt**, **avoid**, or **adapt**.
5. User feedback is documented with evidence (Slice 13).
6. Self-improvement mechanisms are thoroughly compared where present (Slice 10).
7. All cross-cutting documents use consistent structure: per-system detail → trade-off matrix → ACE Recommendation.
8. Every slice specifies exact codebases to clone and exact files/functions to investigate.

---

**Deliverable:** `implementation_plan.md`  
**Vertical Status:** 14/14 Slices Planned  
**Files Affected:**
- `/home/jay/programming/ace_prototype/design/units/agents-study/implementation_plan.md`
- `/home/jay/programming/ace_prototype/design/units/agents-study/study/architecture.md` (planned)
- `/home/jay/programming/ace_prototype/design/units/agents-study/study/memory.md` (planned)
- `/home/jay/programming/ace_prototype/design/units/agents-study/study/compaction.md` (planned)
- `/home/jay/programming/ace_prototype/design/units/agents-study/study/loops.md` (planned)
- `/home/jay/programming/ace_prototype/design/units/agents-study/study/delegation.md` (planned)
- `/home/jay/programming/ace_prototype/design/units/agents-study/study/tools-skills.md` (planned)
- `/home/jay/programming/ace_prototype/design/units/agents-study/study/browser-automation.md` (planned)
- `/home/jay/programming/ace_prototype/design/units/agents-study/study/computer-use.md` (planned)
- `/home/jay/programming/ace_prototype/design/units/agents-study/study/communication.md` (planned)
- `/home/jay/programming/ace_prototype/design/units/agents-study/study/self-improvement.md` (planned)
- `/home/jay/programming/ace_prototype/design/units/agents-study/study/ux-dx.md` (planned)
- `/home/jay/programming/ace_prototype/design/units/agents-study/study/strengths-weaknesses.md` (planned)
- `/home/jay/programming/ace_prototype/design/units/agents-study/study/user-feedback.md` (planned)
- `/home/jay/programming/ace_prototype/design/units/agents-study/study/research-synthesis.md` (planned)
