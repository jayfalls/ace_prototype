# ACE v0.6 Core Architecture Specification: Godot-Style Composite Engine, Deterministic Connectome Map & Sensorium

## 1. Executive Summary & Empirical Grounding

Across `7,909` historical thought traces (`v0.1`: `6,639` traces; `v0.2`: `741` traces; `v0.3`: `529` traces), empirical trace forensics revealed the single strongest determinant of autonomous agent efficiency:

| L6 Starting Context State (`n = 717` execution cycles) | Mean L6 Steps | Mean Prompt Tokens | Mean Output Tokens | First-Cycle Milestone Pass Rate |
| :--- | :---: | :---: | :---: | :---: |
| **Turn-0 Context Complete (`0` reach-out turns needed, `n=413`)** | **`1.92`** | `39,838` | `6,737` | **`77.5%`** |
| **Missing Turn-0 Context (`1+` reach-out/read turns needed, `n=304`)** | `2.35` | `42,658` | `1,968` | **`9.2%`** *(8.4× collapse)* |

**Core Law of ACE v0.6**: *An agent's effectiveness and path to minimum steps is governed by Turn-0 / Step-0.5 context completeness.* Rather than relying on the LLM to guess which files it hasn't read or perform blind sequential reach-outs, ACE v0.6 introduces a **Godot-Style Composite Architecture** where every cognitive layer is composed of reusable, bounded **Components** (`Sensorium`, `ConnectomeMap`, `SpecDistiller`, `AsyncTaskScheduler`, `AAIndexModelRouter`) that deterministically populate **Step 0**, **Step 0.5**, and **Step 1** context slots before the thinker emits a single token.

---

## 2. Godot-Style Composite Architecture (`internal/composite/`)

Instead of monolithic per-layer switch statements, ACE v0.6 structures the cognitive runtime around 4 composable primitives (inspired by Godot's `Node` + `Component` + `Signal` + `Resource` pattern):

1. **`Component` Interface (Bounded Context & Capability Plugins)**:
   ```go
   type ContextPhase int
   const (
       PhaseStep0_StaticPrefix   ContextPhase = iota // Immutable across turns/layers (KV prefix cached)
       PhaseStep0_5_DeterministicROI                 // Pre-LLM deterministic Connectome 2-hop slice
       PhaseStep1_ActiveDelta                        // Dynamic turn feedback, uncertainty delta & async events
   )

   type LayerComponent interface {
       ID() string
       ByteBudget(layer core.LayerID, phase ContextPhase) int
       RenderContext(ctx context.Context, state *CycleState, layer core.LayerID, phase ContextPhase) (string, error)
       Tools(layer core.LayerID) []ToolDescriptor
       ExecuteTool(ctx context.Context, state *CycleState, call core.LayerAction) (string, bool, error)
   }
   ```
   - Every layer (`L1`..`L6`) is simply an ordered composition of `LayerComponent` attachments with strict, tunable byte budgets (`ByteBudget`), guaranteeing least privilege and zero unbound prompt growth.

2. **`SignalBus` (Typed Northbound / Southbound / Lateral Signals)**:
   - Standardized pub/sub connector bus where components emit typed signals (`SignalUncertaintyOpened`, `SignalContradictionDetected`, `SignalSpecDistilled`, `SignalAsyncTaskCompleted`, `SignalHumanReplyReceived`, `SignalQuotaWarning`) without tight coupling.

---

## 3. Pillar 1: Deterministic Problem-Space Connectome & Uncertainty Map (`ConnectomeComponent`)

### 3.1 Why LLM File-Lists Fail & How the Connectome Solves It
Asking an LLM to "list files it hasn't explored" fails because the LLM cannot know the topology of what it hasn't seen. Instead, ACE v0.6 builds a **Deterministic Cross-Cutting Connectome Graph** (`/workspace/.harness_d/connectome_map.json`) in Go/Python before LLM invocation:

* **Connectoid Node Types**:
  1. `code_module`: `/workspace/**/*.py` or `.go` with AST functions, classes, and line hashes.
  2. `symbol_fn`: Individual callable functions/methods and their signatures.
  3. `dataset`: Registered datasets in `/workspace/benchmarks/registry.json` + SQLite tables.
  4. `spec_section`: Sections of the active Living Spec (`/wiki/specs/<id>.md`).
  5. `test_gate`: `--selftest` assertions, Mode-3 invariants, and held-out benchmark evaluators.
  6. `uncertainty_node`: Explicitly tracked assumptions, unmeasured metrics, or contradictions.
* **Directed Connectome Edges**:
  - `IMPORTS`, `CALLS`, `READS_DATASET`, `WRITES_ARTIFACT`, `SPECIFIED_BY`, `VERIFIED_BY`, `BLOCKED_BY_UNCERTAINTY`.

### 3.2 Step 0 $\rightarrow$ Step 0.5 $\rightarrow$ Step 1 Deterministic ROI Tree Search
1. **Step 0 (Anchor ROI Seed)**:
   - Deterministic extraction of the Region of Interest (ROI) anchor nodes from the active milestone in `goals.md` and `/wiki/specs/<id>.md`.
2. **Step 0.5 (Pre-LLM Deterministic Downstream Tree Search)**:
   - Before calling the LLM, the engine runs a deterministic BFS/DFS traversal downstream from the ROI anchors across all edges (`IMPORTS`, `CALLS`, `READS_DATASET`, `SPECIFIED_BY`, `VERIFIED_BY`).
   - **Empirical Size**: Across the entire `v0.3` 11-module, 193-edge codebase, the full workspace Connectome is `10,815B` (`~2,703` tokens), and a **2-Hop ROI Connectome Slice** for a target module (`harness2_improver.py` + `ace_rlm.py` + dataset schemas + open uncertainties) is only **`2,217 bytes` (`~554` tokens)** while covering **100% of downstream dependencies**.
   - Every node in the ROI slice is tagged with its live epistemic state:
     - `[VERIFIED]`, `[INSPECTED]`, `[UNINSPECTED_DOWNSTREAM]`, `[ASSUMPTION_UNVALIDATED]`, or `[CONTRADICTION_OPEN]`.
3. **Step 1 (Guided Uncertainty Closure)**:
   - If any node in the downstream ROI tree is `[UNINSPECTED_DOWNSTREAM]`, `[ASSUMPTION_UNVALIDATED]`, or `[CONTRADICTION_OPEN]`, the Connectome component highlights the exact missing branch at Step 1 and blocks premature checkoff until the branch is empirically tested or resolved.

---

## 4. Pillar 2: Measured & Tunable Spec Distillation Flow (`SpecDistillerComponent`)

### 4.1 Empirical Byte-Budget Optimization
Across `40` real milestone notes and domain syntheses (`min=32B`, `median=2,054B`, `p90=4,257B`, `max=6,965B`):
* A `1,200B` limit truncates `82.5%` of real specs.
* **`2,800 bytes` (`~700` tokens, `8.8%` of a 32KB prompt window)** is the Pareto knee: it fits `70.0%` of raw notes untruncated and **100% of distilled 5-section specs** without wasting KV cache space.
* **Tunable Configuration**: Exposed via `ACE_SPEC_DISTILL_MAX_BYTES` (default `2800`, tunable per layer/task between `1600` and `4800` bytes).

### 4.2 Two-Stage Gather $\rightarrow$ Distill Pipeline
* **Raw Scratchpad (`/workspace/.harness_d/raw/<task_id>.md`)**: Web searches, paper extracts, and exploratory probes land here first and never pollute the permanent prompt prefix.
* **Distilled Spec (`/wiki/specs/<task_id>.md`)**: Compressed via Tier-1 high-RPS models (`Beam Free` / `LongCat 2.5`) into a strict 5-section contract that plugs directly into the Connectome Map:
  1. `In-Scope & Out-of-Scope` (bulleted, zero historical narrative of how decisions were reached),
  2. `Verified Facts & Measured Baselines` (exact numbers + provenance),
  3. `Connectome ROI & Target Interfaces` (exact module/function/dataset nodes),
  4. `Nuances, Invariants & Edge Cases`,
  5. `Open Uncertainties & Contradictions` (linked by ID to `connectome_map.json`).

---

## 5. Pillar 3: Shared `AsyncTaskScheduler` & Time-Aware Human Escalation (`TelegramComponent`)

### 5.1 Unified Per-Layer & Cross-Layer `AsyncTaskScheduler`
A single shared composite service (`internal/composite/scheduler.go`, backed by SQLite `async_tasks`) manages all non-instantaneous work across all 6 layers:
* **Task Kinds**:
  - `CONTINUOUS_DAEMON` (e.g., background benchmark/market monitor),
  - `POLLING_PROBE` (e.g., waiting on external API/rate-limit window or long simulation),
  - `RESUMABLE_PIPELINE` (multi-step research/experiment checkpointed across cycles),
  - `HUMAN_TELEGRAM_WAIT` (awaiting user reply on a contradiction or scope question).
* **Non-Blocking State Machine**: When a task enters `WAITING_POLL` or `WAITING_HUMAN`, `L5` (Cognitive Control) automatically parks that branch and switches `L6` to the next unblocked ROI branch or milestone, resuming seamlessly the moment the poll condition or Telegram reply arrives.

### 5.2 Time-Aware, Plain-English Telegram Contradiction Escalation
* **Restored Credentials**: Bot token (`89853393...`) and Chat ID (`7223316734`) are restored in `.ace-sandbox/ace.db` (`channel_settings`).
* **When Used**: Triggered via `ask_human_telegram` when `L1`/`L2`/`L4` encounters an irreconcilable contradiction, ambiguous scope boundary, or blocked external uncertainty.
* **Strict Plain-English Format (`<= 4` short lines)**:
  - No internal layer jargon (`L4`, `LTL G4`, `Mode-3`).
  - Plain English: *(1) What I'm working on, (2) The contradiction/question, (3) Option A vs Option B, and (4) What I'm working on in the background while waiting for your reply.*
* **Real-World Time & Human Circadian Awareness**:
  - The `Sensorium` injects local wall-clock time (`SAST / UTC+2`), elapsed time since last cycle/user message, and human circadian state (`SLEEP_WINDOW: 23:00–07:30 local` vs `AWAKE_WINDOW`).
  - During `SLEEP_WINDOW` (or when `last_user_reply_ago > 30m`), the agent sends at most 1 concise message per blocker, sets expected reply horizon to `6h–10h`, and continues autonomous background work without nagging.

---

## 6. Pillar 4: Composite `Sensorium` (Proprioception + Exteroception) & Least-Privilege Layer Matrix

### 6.1 Dual-Mode `SensoriumComponent`
1. **Proprioception (Self-Referential Vitals)**:
   - **OS/Hardware (`htop` digest)**: Host/sandbox CPU load (`/proc/loadavg`, core count), RAM used/available (`/proc/meminfo`), RTX 3050 GPU VRAM & health (`127.0.0.1:8091/health`), `/workspace` disk free (`syscall.Statfs`), and network egress latency.
   - **Cognitive & LLM Vitals**: Active model tier, rolling `5h` / `7d` / `30d` token & dollar burn (`$16/5h`, `$40/wk`, `$80/mo` OpenCode Go gauge), KV prefix cache hit %, and per-layer step latency.
2. **Exteroception (Externally-Referential Senses)**:
   - **Temporal & Circadian Sense**: Real-world timestamp, day-of-week, human sleep/awake window, delta since last cycle and last user message.
   - **Environmental & Benchmark Sense**: Reachable external APIs, public SOTA dataset cache status, and incoming Telegram/Web chat signals.

### 6.2 Least-Privilege Layer Plugin & Tool Matrix

| Layer | Step 0 / 0.5 / 1 Auto-Injected Composite Slots | Dedicated Least-Privilege Tools (Reach-Out Only When Needed) |
| :--- | :--- | :--- |
| **L1 Aspirational** | `Soul` + `Goals` + `Exteroception:Temporal/Human` + `Connectome:Contradictions` | `audit_contradictions`, `ask_human_telegram` |
| **L2 Global Strategy** | `DistilledSpec` + `CorpusIndex` + `Connectome:MacroGraph` | `web_search`, `web_fetch`, `distill_spec` |
| **L3 Agent Model** | `Proprioception:OS_GPU_LLM_Vitals` + `Connectome:SkillAndDataInventory` | `probe_vitals`, `curate_skill`, `memory_search` |
| **L4 Executive** | `DistilledSpec` + `Connectome:2Hop_ROI_Slice` + `AsyncTaskLedger` | `update_connectome_uncertainty`, `schedule_async_task`, `ask_human_telegram` |
| **L5 Cognitive Control** | `Proprioception:Valence_Quota` + `AsyncTaskLedger` + `Exteroception:Circadian` | `switch_active_branch`, `select_model_tier` |
| **L6 Task Prosecution** | `DistilledSpec` + `Connectome:2Hop_ROI_Slice` + `HotFilePreInjection` + `OracleGate` | `workspace_read`, `workspace_edit`, `workspace_write`, `sandbox_sh`, `web_search`, `web_fetch` |

---

## 7. Dynamic Model Catalog & Artificial Analysis (AA) Index Router (`AAIndexRouterComponent`)

To ensure ACE never breaks when promotional models (`Beam Free`, `LongCat 2.5 Preview Free`) expire or new models launch:
* **Capability Slots (Decoupled from Hardcoded Model IDs)**:
  1. `Slot:System1_DecisionGate`: Local 4-bit GPU Gate (`Qwen3.5-0.8B` / `d1` on `:8091`, `<2ms` cached) with optional cloud fallback to OpenRouter `inception/mercury-decide:free` (`66K` context, `$0`, `14 decisions/sec` calibrated probabilities via `/v1/systemone`).
  2. `Slot:HighRPS_Distiller_Verifier`: Routes to zero-marginal-cost / high-throughput models on OpenCode Go (`beam-free`, `longcat-2.5-preview-free`, `glm-5.3-flash`) for Spec Distillation, Connectome summarization, and CRUD checks.
  3. `Slot:Frontier_Thinker`: Routes to highest **Artificial Analysis (AA) Intelligence Index / Cost-per-Task** model (`deepseek-v4.1-flash-1017`: AA Intelligence Index **`45` [#2/121]**, `209.4 tok/s`, `1M` context, `98%` cache discount at `$0.003/1M` cached).
* **Live Catalog & Drift Resilience**: Queries `/zen/v1/models` + OpenRouter catalog on startup and every 6 hours, scoring models by `AA_Score / Effective_Cached_Cost` combined with our online Bayesian `Beta(alpha, beta)` success tracker so deprecated or rate-limited models are swapped out automatically with zero code changes.
