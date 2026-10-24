# ACE v0.6 Master Architecture Specification & Opus Review Handoff

## 0. Purpose & Context of This Document
This specification consolidates all empirical findings from **ACE Generations `v0.1` $\rightarrow$ `v0.4`** (`7,909` recorded cognitive thought traces across `83+` hours of autonomous operation) and defines the complete architecture for **`ACE v0.6`** (plus the forward interface for **`ACE v0.7` Automatic Procedural Wiki-Skill Compilation**).

It is designed as a **zero-gap architectural blueprint** for adversarial design review with **Claude Opus** before implementation begins.

---

## 1. Empirical Ground Truth Across Generations (`v0.1` $\rightarrow$ `v0.4`)

### 1.1 Historical Generation Ledger (`github.com:jayfalls/ace_prototype.git`, Tags `v0.1`–`v0.4`)

| Generation | Runtime & Scope | Traces / Cycles | Prompt Tokens (KV Cache Hit %) | Output Tokens | Mean Cost on DeepSeek V4.1 Flash (24/7 Blended) | Key Architectural Outcome & Bottleneck Discovered |
| :--- | :--- | :---: | :---: | :---: | :---: | :--- |
| **`v0.1`** | `51.5h` soak (B2B Micro-SaaS + initial 6-layer loop) | `6,639` / `2,523` | `75.29M` (`50.4%`) | `4.47M` | `$9.64` (`$0.187/hr`, `$0.803/milestone`) | Proved `bwrap` + `os.Root` host isolation and local GPU gate; suffered from **Cross-Cycle Amnesia** and masked shell pipeline exit codes. |
| **`v0.2`** | `23.9h` soak (Cordis Runtime + Mode-3 Gate + 2-Stage SLA Compiler) | `741` / `456` | `15.99M` (`86.7%`) | `2.72M` | `$2.29` (`$0.096/hr`, `$0.051/milestone`) | Solved KV cache stability (`86.7%`) and substance verification (`45` axes verified), but stagnated on compliance-checklist audit scripts instead of architectural evolution. |
| **`v0.3`** | `7.9h` run (6-Layer Scientific State Machine + Harness^2 Improver) | `529` / `118` | `7.46M` (`67.0%` overall, `96%+` post-session-split) | `0.95M` | `$1.09` (`$0.138/hr`, `$0.044/milestone`) | Verified `25/26` milestones (`59,553` empirical dataset records, `2.6×` faster step latency at `11.4s`), but relied heavily on threshold calibration over deep problem-space mapping and hit SQLite/rate-limit contention. |
| **`v0.4`** | Multi-Vertical Web/GitHub/arXiv Discovery + `13` SOTA Public Benchmarks | Live baseline | Pre-cached `13` SOTA benchmarks (`80` rows each, 75/25 SHA-256 splits) | — | — | Added multi-vertical discovery (`DuckDuckGo`, `GitHub`, `OpenAlex/arXiv`, `HuggingFace`, `Wikipedia`, `HN`) and `13` public SOTA benchmarks (`SWE-bench Verified`, `Terminal-Bench 2.1`, `GPQA-Diamond`, `HLE`, `MATH-500`, `MMLU-Pro`, etc.). |

### 1.2 The Turn-0 Context Completeness Cliff (`n = 717` Multi-Step Execution Cycles)
Forensic analysis of all `717` multi-step `L6` execution cycles across `v0.1`–`v0.3` revealed the single largest predictor of first-cycle task completion:

| Worker Starting Context State | Cycles (`n`) | Mean Worker Steps | Mean Prompt Tokens | Mean Output Tokens | First-Cycle Milestone Pass Rate |
| :--- | :---: | :---: | :---: | :---: | :---: |
| **Turn-0 Context Complete (`0` reach-out reads on Turn 1)** | `413` | **`1.92`** | `39,838` | `6,737` | **`77.5%`** |
| **Missing Turn-0 Context (`1+` reach-out `workspace_read`/`wiki_read`/`ls` on Turn 1)** | `304` | `2.35` | `42,658` | `1,968` | **`9.2%`** *(8.4× collapse, $p < 10^{-70}$)* |

**Architectural Law #1 (Context Saturation Law)**:
When the primary worker LLM has to spend its first turn figuring out *what files exist*, *how symbols connect*, or *what the spec requires*, single-cycle success collapses by **8.4× (`77.5%` $\rightarrow$ `9.2%`)**. Therefore, context saturation must happen **before** the worker LLM takes its first turn, via a structured **`Turn 0 (Deterministic)` $\rightarrow$ `Turn 0.5 (Automatic Retrieval Routing)` $\rightarrow$ `Turn 1 (Active Search & Execution)`** pipeline.

**Architectural Law #2 (Empirical Derivation Law — Never Hardcode What Can Be Measured)**:
No behavioral parameter, circadian window, model capability score, or context threshold may be hardcoded if it can be empirically derived from live telemetry, historical traces, OS/environment signals, or provider catalogs.

---

## 2. Core Topology of ACE v0.6: Dual-Gated Parallel Cluster & Isolated Task Workpads

ACE v0.6 replaces the linear 6-layer waterfall with a **Dual-Gated Parallel Cluster Topology** modeled on Godot-style composition (`Node` + `Component` + `SignalBus` + `Shared/Local Resource Spaces`):

```
                      ┌──────────────────────────────────────────────────┐
                      │           LAYER 1: ASPIRATIONAL SOVEREIGN        │
                      │   (Constitution, Epistemic Integrity, Mission,   │
                      │    Human Alignment & Contradiction Resolution)   │
                      └────────────────────────▲─────────────────────────┘
                                               │ Northbound Escalations
                                               ▼ Southbound Imperatives
                      ┌──────────────────────────────────────────────────┐
                      │     GATE 1.5: CONSTITUTIONAL & CONTRADICTION     │
                      │            DECISION GATE (/v1/systemone)         │
                      │  • Filters L2-L4 northbound signals for genuine  │
                      │    contradictions, scope drift, or human blockers│
                      │  • Escalates irreconcilable contradictions to    │
                      │    L1 -> Human (Telegram) in plain English       │
                      └────────────────────────▲─────────────────────────┘
                                               │
        ┌──────────────────────────────────────┼──────────────────────────────────────┐
        │                  SHARED L2–L4 CLUSTER MAP & LIVING SPEC SPACE               │
        │        (Shared Connectome Graph, Living Specs /wiki/specs/*, Wiki, KB)      │
        │  ┌─────────────────────┐   ┌─────────────────────┐   ┌───────────────────┐  │
        │  │  LAYER 2: STRATEGY  │◄─►│ LAYER 3: AGENT MODEL│◄─►│ LAYER 4: EXECUTIVE│  │
        │  │  • External World & │   │ • Proprioceptive    │   │ • Project Manager │  │
        │  │    Literature Map   │   │   Vitals & Capacity │   │ • Dependency &    │  │
        │  │  • Competitor/Web   │   │ • Skill & Dataset   │   │   Risk Connectome │  │
        │  │    Sensing          │   │   Connectome Map    │   │ • Living Spec     │  │
        │  │  • Layer-2 Sub-Map  │   │ • Layer-3 Sub-Map   │   │   Owner (2,800B)  │  │
        │  └─────────────────────┘   └─────────────────────┘   └───────────────────┘  │
        └──────────────────────────────────────▲──────────────────────────────────────┘
                                               │ Northbound Consolidated Task Deltas
                                               ▼ Southbound Task Specs + ROI Connectome
                      ┌──────────────────────────────────────────────────┐
                      │      LAYER 5: TACTICAL GATE, MULTI-TASK          │
                      │     SCHEDULER & NORTHBOUND CONSOLIDATOR          │
                      │  • Enforces Spec Completeness & Anti-Cheating    │
                      │  • Runs Turn 0.5 Retrieval Router to saturate    │
                      │    each L6 worker's context before Turn 1        │
                      │  • Coordinates Async / Polling / Human-Waiting   │
                      │    tasks & consolidates L6 outputs back to Spec  │
                      └───────┬────────────────────────┬─────────────────┘
                              │                        │
              ┌───────────────▼───────┐        ┌───────▼───────────────┐
              │   L6 WORKER #1        │        │   L6 WORKER #2        │
              │ • Isolated Workpad:   │  ...   │ • Isolated Workpad:   │
              │   .workpads/task_A/   │        │   .workpads/task_B/   │
              │ • Private Scratchpad, │        │ • Private Scratchpad, │
              │   Local Map & History │        │   Local Map & History │
              └───────────────────────┘        └───────────────────────┘
```

### 2.1 Why This Topology Works
1. **Parallel Interconnected `L2 || L3 || L4` Cluster**:
   * `L2` (External Strategy & Web/Literature Grounding), `L3` (Self-Referential Agent Model, Hardware/LLM Capacity & Dataset/Skill Inventory), and `L4` (Executive Project Manager, Risk/Dependency Planner & Living Spec Owner) operate **in parallel** over a **Shared Cluster Connectome & Living Spec Space**, while each maintains its own specialized **Layer Sub-Map**.
2. **Gate `L1.5` (Constitutional & Contradiction Gate)**:
   * Sits between `L1` and the `L2–L4` cluster using a fast `/v1/systemone` decision model (`jev-1.13-free` / `mercury-decide` / local 4-bit GPU gate).
   * Prevents routine tactical chatter from triggering expensive `L1` invocations, while immediately catching **contradictions** (conflicting empirical data, conflicting spec clauses, or ambiguous user goals) and routing them to `L1` $\rightarrow$ **Human via Telegram**.
3. **Gate `L5` (Tactical Gate, Multi-Task Coordinator & Consolidator)**:
   * Sits between the `L2–L4` cluster and `L6` Task Prosecutors.
   * **Southbound**: Refuses to launch an `L6` task unless its **Living Spec (`2,800B`)** and **Turn 0 + Turn 0.5 Connectome Context** are saturated.
   * **Northbound**: Tracks multiple concurrent/async/polling `L6` tasks, verifies that `L6` outputs conform to the Living Spec (or triggers a **Living Spec Consolidation** if empirical findings require updating the Spec), and reports clean deltas up to `L2–L4`.
4. **Isolated `L6` Task Workpads (`.workpads/<task_id>/`)**:
   * Every individual task at `L6` gets its own isolated workpad directory, episodic scratchpad, local Connectome subgraph, and Cordis revertible fiber so tasks never cross-contaminate each other's context window or file state.

---

## 3. The 3-Stage Context Saturation Pipeline: `Turn 0` $\rightarrow$ `Turn 0.5` $\rightarrow$ `Turn 1`

Every layer invocation is structured into three distinct context-assembly stages, divided into **hierarchical bounded sub-buckets**:

### 3.1 `Turn 0`: Deterministic Zero-LLM Sub-Buckets
Populated in `<5ms` purely by deterministic Go/AST/MVCC lookups (preserving the `95%+` KV prefix cache):
* `SubBucket 0.1 — Constitutional & Role Prefix`: Canonical `soul.md` + `index.md` + `goals.md` active milestone.
* `SubBucket 0.2 — Living Spec (`2,800B` enforced)`: The active task's `/wiki/specs/<task_id>.md`.
* `SubBucket 0.3 — Deterministic Connectome ROI Anchor (`~2,200B`)`: 1-hop AST/symbol/dataset/test neighborhood around the files/entities named in the Living Spec.
* `SubBucket 0.4 — Focused Episodic/Semantic Memory & Task Workpad`: The layer's or `L6` task's private `.workpads/<task_id>/state.json` + attempt ledger.
* `SubBucket 0.5 — Sensorium Digest (Proprioception + Exteroception)`: Bounded hardware/OS/GPU/LLM vitals + derived temporal/human-availability posterior.

### 3.2 `Turn 0.5`: Automatic Retrieval Routing (Pre-Worker Context Saturation)
* **What Problem It Solves**: Deterministic `Turn 0` knows explicit links (imports, named files), but cannot always know *which semantic memory query, which corpus paper section, which specific line range of a 600-line dependency, or which public dataset schema* the worker LLM will need on Turn 1.
* **How `Turn 0.5` Works**:
  1. Before the primary worker LLM (`DeepSeek V4.1 Flash`) is called, a **Hybrid Cheap/Decision Router** runs (`<300ms` via local 4-bit GPU gate, `jev-1.13-free`, `mercury-decide`, or `beam-free`/`longcat-2.5-preview-free`).
  2. It inspects `(Active Task + Living Spec + Turn 0 Connectome Map)` and emits **structured tool-input parameters** for read-only context-gathering actuators:
     ```json
     {
       "connectome_expand_nodes": ["ace_core/router.py::score_candidate", "dataset:public_swe_bench_verified"],
       "file_line_slices": [{"path": "ace_core/router.py", "start": 45, "end": 120}],
       "memory_fts_queries": ["pareto guard calibration failure"],
       "corpus_search_queries": ["Harness^2 propose probe compose"],
       "spec_clauses_to_pin": ["Section 3: Interfaces", "Section 5: Open Uncertainties"]
     }
     ```
  3. The harness executes all of those read-only retrievals in parallel in Go and **injects the results directly into the worker LLM's initial prompt (`Step 0.5 Saturated Context Sub-Bucket`)**.
  4. **Result**: When the worker LLM (`DeepSeek V4.1 Flash`) runs its very first inference call, it sees a **100% pre-saturated context** as if it were Turn 0 — eliminating the `8.4×` reach-out penalty without burning a worker turn!

### 3.3 `Turn 1`: Active Search, Uncertainty Closure & Execution
* With `Turn 0` (deterministic) + `Turn 0.5` (routed retrieval) already in its context window, the worker LLM on `Turn 1` either:
  1. **Executes directly** (writing/editing code + running verification in a single batched turn), OR
  2. **Performs targeted external search** (`web_search`, `web_fetch`, simulation probe) *only* for genuine external unknowns marked `[UNINSPECTED_DOWNSTREAM]` or `[ASSUMPTION_UNVALIDATED]` on the Connectome Map.

---

## 4. Pillar 1: Composite Problem-Space Connectome Map (Shared + Per-Layer Sub-Maps)

Instead of asking an LLM to "list files it hasn't read", ACE v0.6 maintains a deterministic **Multi-Layer Connectome Graph**:

### 4.1 Connectoid Node & Edge Schema
* **Node Types**:
  - `FILE_MODULE` (`/workspace/...`), `AST_SYMBOL` (`fn`/`class`/`struct` with signature & line hash), `DATASET_SCHEMA` (`registry.json` + DB tables), `SPEC_CLAUSE` (`/wiki/specs/<id>.md#section`), `TEST_INVARIANT` (`--selftest` / Mode-3 check), `ASSUMPTION_NODE`, `CONTRADICTION_NODE`, `ASYNC_TASK_NODE`.
* **Directed Edges**:
  - `IMPORTS`, `CALLS`, `READS_DATA`, `PRODUCES_ARTIFACT`, `GOVERNED_BY_SPEC`, `TESTED_BY`, `DEPENDS_ON_ASSUMPTION`, `HAS_CONTRADICTION`.
* **Deterministic Epistemic Coverage States on Every Node**:
  - `UNEXPLORED` $\rightarrow$ `INSPECTED` $\rightarrow$ `IMPLEMENTED` $\rightarrow$ `EMPIRICALLY_VERIFIED` (or `FALSIFIED` / `CONTRADICTED`).

### 4.2 Shared Cluster Map vs. Per-Layer Composite Views
* **Shared Cluster Connectome**: Stored in the concurrent MVCC graph store; links all specs, modules, datasets, assumptions, and tests.
* **Per-Layer Composite Map Projections**:
  - **L2 Map View**: External dependencies, research corpus citations, competitor benchmarks, market/web nodes.
  - **L3 Map View**: Hardware/compute nodes, LLM model-tier nodes, dataset splits, and procedural skill nodes.
  - **L4 Map View**: Critical-path dependency tree, assumption validation status, risk/contingency branches, and Living Spec links.
  - **L6 Task Workpad Map View**: The 2-hop downstream call/data/test subgraph for that specific task's Region of Interest (ROI).
* **Tree-Search Completeness Law**: A task cannot transition to `DONE` while any node reachable downstream of its ROI anchor in the Connectome Graph remains in state `UNEXPLORED`, `ASSUMPTION_UNVALIDATED`, or `CONTRADICTED`.

---

## 5. Pillar 2: `2,800B` Enforced Living Spec & Anti-Cheating Consolidation Flow

### 5.1 The `2,800-Byte` Living Spec Contract (`/wiki/specs/<task_id>.md`)
Empirically calibrated across our milestone corpus (`2,800B` $\approx 700$ tokens = `8.8%` of a 32KB prompt window, tunable via `ACE_SPEC_DISTILL_MAX_BYTES`), every active task must have a **Living Spec** containing 5 mandatory sections:
1. **`## 1. Scope Boundaries`**: Explicit `IN_SCOPE` and `OUT_OF_SCOPE` bullets (zero narrative history of how decisions were made).
2. **`## 2. Verified Facts & Empirical Baselines`**: Measured numbers, dataset split sizes, and hardware/latency constraints with provenance.
3. **`## 3. Target Connectome & Interfaces`**: Exact file paths, function signatures, input/output schemas, and Connectome node IDs.
4. **`## 4. Nuances, Edge Cases & Failure Modes`**: Specific invariants, boundary conditions, and concurrency/security constraints.
5. **`## 5. Uncertainty & Contradiction Ledger`**: Explicit list of assumptions to validate, simulations to run, and open contradictions (must all reach `[CLOSED: empirical_proof_ref]` before task completion).

### 5.2 Anti-Cheating & Bidirectional Spec Consolidation Gate (Enforced by `L5` & Mode-3)
LLMs frequently try to "cheat" specs either by writing vague 3-line specs or by changing code behavior/interfaces during debugging without updating the spec (or watering down the spec to match broken code). ACE v0.6 blocks both:
1. **Pre-Execution Spec Gate (`L5` Southbound)**:
   * Rejects any Spec missing any of the 5 sections, lacking concrete function/file/dataset interfaces in Section 3, or containing unlinked prose instead of Connectome node bindings.
2. **Post-Execution AST-to-Spec Conformance Audit (`L5` Northbound & Mode-3)**:
   * Deterministically parses the AST of modified `/workspace` files and JSON outputs and verifies that every file, function signature, dataset split, and statistical test declared in `Section 3` and `Section 4` of the Living Spec actually exists and passes.
3. **Living Spec Consolidation Loop**:
   * If empirical execution in `L6` proves that an interface or parameter in the Spec *must* change to satisfy $H_1$ on held-out data, `L5` triggers `SpecDistillerComponent` to consolidate the empirical delta back into `/wiki/specs/<task_id>.md` (recording a cryptographic diff in change control), **unless** the change violates `Section 1 (Scope Boundaries)` or lowers the acceptance bar (which is flagged as a `CONTRADICTION_NODE` and escalated to `L1.5` $\rightarrow$ Human).
4. **Human Final Sign-Off Law**:
   * Specs remain in `STATUS: LIVING` (and stay eligible for iterative refinement) until **the Human explicitly marks the overarching goal/spec `DONE`** (or approves completion via Telegram/Cockpit).

---

## 6. Pillar 3 & 4: Godot-Style Composite Systems, Derived Circadian Sensorium & Concurrent Storage

### 6.1 Moving Beyond Single-Writer SQLite: Composite MVCC + JetStream Storage Engine
With `L2 || L3 || L4` running in parallel, `Turn 0.5` firing concurrent retrieval probes, multiple `L6` task workpads executing simultaneously, and `AsyncTaskScheduler` + `TelegramBridge` polling continuously, **standard SQLite becomes a concurrency bottleneck (`SQLITE_BUSY` single-writer lock contention)**.

ACE v0.6 introduces a clean **`StorageEngine` Composite Interface** backed by pure-Go concurrent engines:
1. **Hot Concurrent State & Graph Store — `BadgerDB v4` / CockroachDB `Pebble` (Embedded Pure-Go MVCC LSM Store)**:
   * Zero CGO, full multi-reader / concurrent atomic write batching (`WriteBatch`) with **zero `SQLITE_BUSY` lock contention**.
   * Stores: `ConnectomeGraph` nodes/edges, `L6` isolated workpad state (`workpad:<task_id>:*`), `AsyncTaskScheduler` state machine, KV prompt/response radix cache, and high-frequency telemetry spans.
2. **Typed Inter-Layer Bus & Reactive Streams — Embedded `NATS JetStream` (or in-process ring-buffer bus with Pebble WAL)**:
   * Powers the `L1 <-> L1.5 <-> L2||L3||L4 <-> L5 <-> L6_k` pub/sub signal channels with mandatory attribution headers (`AgentID`, `CycleID`, `LayerID`, `TaskID`, `CorrelationID`).
3. **Analytical & Full-Text Search Store — Read-Optimized SQLite WAL (with single serialized writer actor) / DuckDB**:
   * Used strictly for append-only archival `thought_traces`, FTS5 semantic memory search, and offline benchmark analytics where a single background writer channel drains batches every `500ms` without ever blocking cognitive goroutines.

### 6.2 `SensoriumComponent`: Proprioception (Self) + Exteroception (World & Derived Human Circadian Model)
1. **Proprioception (Self-Referential Vitals)**:
   * **OS & Compute (`htop` Sensor)**: Live CPU load average & core utilization (`/proc/loadavg`, `/proc/stat`), RAM used/available (`/proc/meminfo`), RTX 3050 GPU VRAM free/used & gate latency (`127.0.0.1:8091/health`), `/workspace` disk free/iops (`syscall.Statfs`), and network RTT.
   * **LLM Capacity & Economics Sensor**: Live OpenCode Go rolling-window burn (`5h: $X / $16`, `7d: $Y / $40`, `30d: $Z / $80`), active model tier health posteriors `Beta(α, β)`, KV prefix cache hit %, and token velocity (`tok/s`).
2. **Exteroception (Externally-Referential Senses & Empirical Human Circadian Derivation)**:
   * **Zero Hardcoded Sleep Windows**: Instead of hardcoding `23:00–07:30`, the `CircadianDerivationSensor` computes a real-time **24-hour Bayesian Activity Posterior** $P(\text{human\_active} \mid h_{\text{local}}, d_{\text{week}}, \Delta t_{\text{idle}})$ derived from all observable human timestamps:
     1. Historical timestamps & inter-reply latencies of your **Telegram messages** (`channel_settings` / message log),
     2. **Web Cockpit chat & API interactions**,
     3. **Local Git commit timestamps** (`git log --format=%aI`), and
     4. **Local Workstation Session Idle Signals** (`loginctl` / systemd-logind `IdleHint` & `IdleSinceHintMonotonic` / active PTY modification times when running on your machine).
   * From this derived posterior, every layer sees:
     * `LocalTime: Fri 2026-10-23 00:42 SAST`,
     * `DerivedHumanAvailability: P(active_now)=0.12 | LastHumanSignalAgo=8m | ExpectedReplyLatency_p50=6.4h (low-activity window derived from 30d history)`.
   * **Telegram Plain-English Contradiction Protocol + `anatomy` Isometric Machine Figures (`https://github.com/wheresryan22/anatomy`)**:
     * When `L1.5` or `L4` escalates a contradiction or unresolvable uncertainty via `ask_human_telegram`:
       - Sends **1 short, plain-English message (`<= 4` lines)** explaining the contradiction and options A vs B.
       - **Mandatory `anatomy` Visual Conceptualization for Complex Components**: Whenever the bot needs to convey a complex component, architectural trade-off, or multi-part interaction on Telegram, it **ALWAYS** uses the `anatomy` framework (`/home/jay/.pi/agent/skills/anatomy/`, `https://github.com/wheresryan22/anatomy`) to conceptualize the idea as a crafted, four-tone shaded isometric physical bench instrument (a test rig, dial gauge, pin-tumbler lock, gantry router, or valve manifold verified with `kit/audit.mjs`), rendering and sending the figure (`sendPhoto` / interactive HTML/PDF via `sendDocument`) alongside the concise 3–4 line plain-English message rather than dumping walls of text or flat box diagrams.
       - Sets the task's `AsyncTaskScheduler` expectation horizon to the derived $\mathbb{E}[\Delta t_{\text{reply}}]$ (so it never nags or double-texts while you sleep), parks the blocked branch as `WAITING_HUMAN`, and immediately routes `L6` to unblocked workpads.

---

## 7. Model Economics, OpenCode Go, `Mercury Decide` & `v0.7` Skill Roadmap

### 7.1 Answer on `inception/mercury-decide:free` & OpenCode Go Model Tiering
1. **How long will `inception/mercury-decide:free` stay free & what is the catch?**
   * On OpenRouter, `inception/mercury-decide:free` has **no fixed expiration date**, **but** it has two strict catches:
     1. **Rate limit**: Capped at **`20 req/min` and `50 req/day`** on free OpenRouter accounts (rises to **`1,000 req/day`** once your OpenRouter account has purchased $\ge \$10$ in lifetime credits).
     2. **Data policy**: OpenRouter marks `inception/mercury-decide:free` as `Privacy: Trains` (prompts may be used for training), whereas the paid `inception/mercury-decide` (`$0.02 / 1M` input tokens, **`$0 / 1M` output tokens**, **`$0 / 1M` cache read**, zero training retention) costs virtually nothing (`$0.02` per million input tokens!).
   * Furthermore, OpenCode itself now hosts **`jev-1.13-free` (`OpenCode · Jev 1.13`)**, which uses the **exact same `/v1/systemone` structured decision schema** (`choice`, `score`, `noul` yes/no probabilities) as `Mercury Decide`!
2. **Now That You Have OpenCode Go (`$10/mo` = `$80/mo` DeepSeek V4.1 Flash + `∞` Beam/LongCat)**:
   * **Capability Slot 1 — `System1_Gate` (`L1.5`, `L5`, `Turn 0.5` fast scoring)**:
     * Primary: Local 4-bit GPU Gate on RTX 3050 (`127.0.0.1:8091`, `<2ms` cached, `$0`).
     * Cloud co-processor: OpenCode `jev-1.13-free` / OpenRouter `inception/mercury-decide` (`/v1/systemone`).
   * **Capability Slot 2 — `Turn0_5_Router_And_SpecDistiller` (`Turn 0.5` retrieval routing, `2,800B` Spec distillation, AST/spec audits)**:
     * **Speed-First Primary**: OpenCode Go **`Beam` (`beam-free`, `1M` context, `∞` allowance)** — empirically **5.6× to 10.8× faster in `tok/s` (`250.4–485.7 tok/s` vs `30–45 tok/s`)** and **16.5× faster in median step latency (`2.76s` vs `45.46s`, `420–600ms` TTFT)** than `LongCat 2.5 Preview` (`1.6T/48B` MoE, `6.4s` TTFT).
     * **Speed-Ordered Fallbacks**: `Step 5 Preview` (`217.4 tok/s`, `898ms` TTFT, AA `43.7`) $\rightarrow$ `GLM-5.3-Flash` (`132.6 tok/s`, AA `41.8`) $\rightarrow$ `LongCat 2.5 Preview Free` (`44.6 tok/s`, `∞` allowance, AA est. `~36.2`).
   * **Capability Slot 3 — `Frontier_Thinker` (`L1`, `L2||L3||L4` synthesis, complex `L6` code/experiment synthesis)**:
     * Primary: OpenCode Go **`DeepSeek V4.1 Flash 1017`** (**AA Intelligence Index `45` [#2/121 globally]**, `209 tok/s`, `1M` context, `$80/mo` included allowance on Go = `~38,700` traces/month).

---

## 8. Penned-In Roadmap for `ACE v0.7` (Post-`v0.6` Run & Learnings)

Once `ACE v0.6` has run and accumulated empirical traces under the new Composite + Connectome + Turn 0.5 architecture, **`ACE v0.7`** will introduce two flagship upgrades grounded in `v0.6` operational data:

### 8.1 Automatic Skill Management & Procedural Wiki-Skill Compilation System
Inspired by Google's Wiki-Skill / Voyager / Harness^2 C6 continual library:
1. **Automatic Trajectory-to-Skill Distillation**:
   * Whenever an `L6` workpad resolves a non-trivial uncertainty branch or passes a Mode-3 gate after overcoming a failure, a background compiler extracts the verified sequence `(Problem Signature, Preconditions, Parameterized Code/Tool Template, Verification Test, Anti-Patterns)` into a versioned `/wiki/skills/<domain>/<skill_id>.md` + executable `/workspace/skills/<skill_id>.py`.
2. **Connectome-Indexed Skill Graph**:
   * Skills become first-class `SKILL_NODE`s in the Connectome Graph with `SOLVES_PROBLEM_CLASS` and `COMPOSES_WITH_SKILL` edges, automatically retrieved at **`Turn 0.5`** whenever a matching problem signature appears.
3. **Empirical Skill Evolution, ELO/Beta Scoring & Garbage Collection**:
   * Every skill tracks its own `Beta(α, β)` pass-rate posterior and token-savings delta across cycles. Skills that regress or become superseded are automatically refactored or retired via Replace-Never-Layer change control.

### 8.2 Full Isometric `anatomy` Physical-Machine WebUI (`https://github.com/wheresryan22/anatomy`)
In `ACE v0.7`, the Glass-Box WebUI (`http://localhost:8080`) will be rebuilt from the ground up as a live, interactive **`anatomy` Isometric Physical Machine** (`https://github.com/wheresryan22/anatomy`):
* **The Object Is the Explanation**: Instead of flat dashboard cards and tables, the entire ACE cognitive engine is rendered as a dense, crafted isometric bench machine with four-tone shading, true painter's depth order (`kit/audit.mjs`), and optional world-space WebGL shaders:
  - **`Turn 0 / Turn 0.5` Context Lock**: A physical pin-tumbler lock & gantry carriage that visibly aligns pins as `Turn 0` (deterministic) and `Turn 0.5` (`Beam` retrieval router) saturate context slots before theplug turns to fire `Turn 1`.
  - **`L1` Sovereign & `L1.5` Gate**: An upper balance beam and escapement gate with live tension springs.
  - **`L2 || L3 || L4` Parallel Cluster Manifold**: Three coupled rotary drums/drawers sharing a physical pantograph map table (the Shared Connectome Graph).
  - **`L5` Tactical Gate & `L6` Workpad Lathes**: Individual removable workpad carriages on rails, with dial gauges (`Sensorium` CPU/RAM/VRAM/KV-cache/Quota indicators) whose needles move in real time from the exact same telemetry model that drives the engine.

---

## 9. Specific Review Questions for Claude Opus

When reviewing this blueprint, please stress-test and critique the following 5 architectural questions:
1. **`Turn 0` $\rightarrow$ `Turn 0.5` $\rightarrow$ `Turn 1` Retrieval Router Contract**:
   * What failure modes exist when a cheap/decision model (`Turn 0.5`) selects the retrieval tool inputs (`connectome_expand_nodes`, `file_line_slices`, `memory_fts_queries`) for a frontier thinker (`Turn 1`)? How should we bound false-negative omissions at `Turn 0.5` so the thinker isn't blinded by an overly narrow `Turn 0.5` filter?
2. **Parallel `L2 || L3 || L4` Cluster Synchronization & Shared Connectome Consistency**:
   * When `L2` (External Strategy), `L3` (Self-Model/Capabilities), and `L4` (Executive Spec Owner) run concurrently and mutate/annotate the Shared Cluster Map, what conflict-resolution invariant should `L5` (Southbound) and `L1.5` (Northbound) enforce when `L2` and `L3` emit competing constraints?
3. **Anti-Cheating Enforcement on the `2,800B` Living Spec**:
   * What adversarial loopholes could an LLM exploit to satisfy a syntactic AST-to-Spec conformance check while still subtly drifting from the user's true intent, and what deterministic check closes those loopholes without requiring a human in the loop on every micro-step?
4. **Storage Architecture (`BadgerDB`/`Pebble` MVCC + Embedded NATS JetStream vs. Actor-Serialized SQLite WAL)**:
   * For a single-binary Go autonomous agent running parallel `L2–L4` + `N` concurrent `L6` workpads + high-frequency telemetry spans, is pure-Go **`Pebble`/`BadgerDB` MVCC + in-process typed pub/sub** the cleanest zero-CGO foundation, or is there a sharper hybrid pattern?
5. **Derived Bayesian Circadian & Availability Model**:
   * Are there edge cases in deriving $P(\text{human\_active\_now})$ and $\mathbb{E}[\Delta t_{\text{reply}}]$ from `(Telegram timestamps + Cockpit activity + Git commits + OS loginctl IdleHint)` (e.g., travel/timezone shifts or long deep-work blocks) that we should guard against in the escalation scheduler?
