# ACE v0.2 Core Architecture Specification (Pass 4 — Full Historical ACE + 340-Paper Corpus Unified Synthesis)

- **Status:** Pass 4 Complete (Cross-Phase Design Audit + Consistency Verification Complete — Ready for Implementation)
- **Target Codebase:** `/home/jay/programming/ace_prototype` (`internal/core`, `internal/sandbox`, `internal/llm`, `internal/memory`, `internal/telemetry`, `internal/cognitive`)
- **Strict Non-Self-Modification Invariant:** The `ACE v0.2` Go binary (`./ace`), its Go source tree (`internal/`, `cmd/`), and host systemd units (`ace.service`, `ace-gpu-gate.service`) are **permanently immutable to the agent** (`os.Root` + `bwrap --ro-bind` + `IMMUTABLE_HOST_BOUNDARY` System Integrity Overlay). `ACE v0.2` runs indefinitely as a hardened **Empirical Scientist & Systems Researcher**, conducting experiments and building `ACE v2` strictly inside `/workspace/` and `/wiki/`.
- **Codebase Hygiene & Style Constraints (`AGENTS.md`):**
  1. **Zero `any` / `interface{}`** in all new Go structs, interfaces, and function signatures.
  2. **Zero `else` statements** — strictly guard clauses and early returns throughout all new and refactored Go code.
  3. **Mandatory Attribution (`design/README.md`):** `agent_id` (`"ace-v1.5"`) and `cycle_id` (`cyc_<ns>`) threaded through every SQLite row, `UsageEvent`, OpenTelemetry span, `prefix_hash` chain, and `/workspace/.harness_d/` trace artifact.
  4. **Mandatory Verification:** All packages must pass `go test ./...` before binary build and service restart.

---

## 1. Cross-Phase Archaeological Synthesis: Every ACE Design Era Unified

This specification reconciles and integrates every architectural decision across all **five eras** of your ACE work so nothing is lost or contradicted:

| ACE Design Era | Primary Sources Audited | Key Decisions & Learnings Integrated into `ACE v0.2` |
| :--- | :--- | :--- |
| **Era 1: Original ACE & Biological Grounding (2023–2024)** | `design/source.md` (Shapiro `arXiv:2310.06775`), Neuroscience mappings (Damasio, Badre, Sapolsky, Graziano, Friston) | • **6-Layer Cognitive Hierarchy (`L1`–`L6`)** over Northbound (telemetry/pain/status) and Southbound (directives/inhibition) buses.<br>• **System Integrity Overlay (`SIO`):** Out-of-band corrigibility and host-immutability veto across all layers.<br>• **Damasio Somatic Markers & Badre/Sapolsky `L5` Inhibitory Control:** Fast visceral valence $V \in [-1, +1]$ on `(action_sig, target)` to inhibit impulsive/repetitive `L6` loops.<br>• **Graziano Attention Schema (`L3` Agent Self-Model):** Explicit capability & sandbox boundary self-model (`bwrap` read-only `/usr` & `/wiki`, offline `pip`, pre-seeded `/workspace/ace_rlm.py` stdlib SDK) so `L6` never hallucinates unavailable tools or packages. |
| **Era 2: Core Units & Distributed Systems Architecture (Apr 2026)** | `design/README.md`, `architecture/architecture.md`, `messaging-paradigm/fsd.md`, `observability/fsd.md`, `database-design/`, `deployment-dx/` | • **Autonomous Entity Posture (`design/README.md`):** Continuous processing loop; Chat Interface is an external bidirectional broker, not a cognitive layer.<br>• **Layer Memory Isolation:** Each layer accesses only its own layer-scoped memories plus canonical global memory (`SearchForLayer`).<br>• **4-Tier Memory Architecture (`L1` Immediate 200t, `L2` Short 500t, `L3` Medium 1500t, `L4` Long 3000t)** mapped cleanly onto our SLA Prefix/Suffix Context Buckets (Section 3.2).<br>• **Layer Loop Monitor (`design/README.md`):** Tracks `max_iterations`, `max_time_seconds`, and cumulative token budget per milestone to terminate runaway loops.<br>• **5 Global Loops:** Chat Broker, Safety Monitor (reactive batch + proactive high-risk gate), Memory Manager, Swarm Coordinator, and Learning Loop.<br>• **Canonical Observability (`observability/fsd.md`):** Synchronous `UsageEvent` tracking (`agent_id`, `cycle_id`, inclusive `input_tokens`, `cached_tokens`, `output_tokens`, `duration_ms`). |
| **Era 3: 13-Agent Comparative Study & Providers Unit (Late Apr 2026)** | `agents-study/research.md`, `compaction.md`, `memory.md`, `loops.md`, `delegation.md`, `tools-skills.md`, `strengths-weaknesses.md`, `providers/research.md`, `providers/fsd.md` | • **Claude Code Graduated Compaction (`Snip -> Microcompact -> Collapse`):** Strip noisy tool stdout every step (`Snip`), clear older inner-loop step outputs without mutating cached turns (`Microcompact`), and collapse finished milestones into `EvidenceReceipt` nodes (`Collapse`).<br>• **Claude Code Safety-Classified Tool Partitioning:** Partition batched actions into `isConcurrencySafe()` read-only waves (run in parallel) vs. mutating actions (run serially inside `CordisFiber`).<br>• **Open Code Two-Phase Permission Filtering:** Filter available tools per layer *before* the model sees them in the prompt (`Phase 1`), and validate again at execution time (`Phase 2`).<br>• **Open Code `opencode-working-memory` Slots + `open-mem` Typed Observations + `OpenViking` Monotonic Versioning:** Guaranteed-visibility slots (`AttemptLedger`, `ActiveError`, `ForbiddenSigs`, `SomaticValence`, `LoopBudget`) in `Tier 1`, and typed observations (`decision`, `bugfix`, `pitfall`, `experiment`, `metric`) with monotonic versioning in `Tier 2–4`.<br>• **Oh My OpenAgent `Intent Gate -> Prometheus (Planner) -> Momus (Independent Reviewer) -> Atlas/Sisyphus (Executor)`:** Trajectory-blind `Mode-3` Verifier acts as the independent `Momus` critic.<br>• **Providers Circuit-Breaker State Machine (`providers/research.md`):** `healthy -> degraded (1 sticky retry with full jitter) -> unhealthy (60s/30m cooldown) -> probing -> healthy` + inclusive token normalization. |
| **Era 4: Logseq Behavioural & Meta-Cognitive Audits (2024–2026)** | `/home/jay/logseq/pages/2026-W11.md`, `2026___Q1___W12.md`, `2026___Q2___W15.md` | • **The Anti-"Identification-as-Completion" Law (`2026-W11` Cynical Auditor):** Naming or diagnosing a failure in `thought` or a wiki reflection note **never** counts as a completion event or resets the loop counter; only an empirical state diff (`workspace_write`/`workspace_edit` + passing `--selftest`) clears a failure state.<br>• **Identity-Level Boundary vs. Overridable Rule (`2026-Q1-W12`):** High-drive task execution (`L6`) will override soft prompt suggestions unless boundaries are both constitutive (`soul.md` + `SIO`) and structurally enforced by the runtime (`Cordis` + `Mode-3` + `AgentLTL`). |
| **Era 5: 340-Paper Meta-Harness Corpus, `Cordis`, `ModularRSI`, `P3/P5`, `SPOQ`, `RLM/RISE` (Current)** | `paper-modularrsi-*.md`, `meta-harness-p3-p5-empirical-results.md`, `meta-harness-audit-bugs.md`, `v2-autopsy.md`, `arXiv 2608.25512 (Cordis)`, `arXiv 2603.28052 (Meta-Harness)` | • **5-Module `ModularRSI` Spine** (`Cordis` Spatiotemporal Runtime, `AgentLTL` + Validated Handoff, Two-Stage SLA + AST Context Compiler, `SPOQ` Dual Gates + `Mode-3` Verifier, Bayesian `Cost-of-Pass` Router + `W4` Self-Audit).<br>• **Live Trace Filesystem `D` (`/workspace/.harness_d/`)** + Karpathy `autoresearch` `scoreboard.tsv` + Pareto archive.<br>• **Pre-Seeded `/workspace/ace_rlm.py` Scientific Stdlib SDK** (`Beta(α,β)` CI, McNemar exact test, paired bootstrap, AST mutation kill-rate tester, SQLite/corpus RLM queries).<br>• **Hermes Pitfall-to-Playbook Auto-Distillation (`/wiki/skills/learned-playbooks.md`)** on overcome failures (`CR > 0`). |

---

## 2. Adversarial Audit Fixes (`S1`–`S7`) & Paradigm Breakthroughs (`B1`–`B8`)

### 2.1 The 7 Adversarial Audit Fixes (`Pass 2`)

| ID | Flaw Caught in Live Code / Pass 1 | Root Cause | Exact Architectural Fix in `ACE v0.2` |
| :--- | :--- | :--- | :--- |
| **S1** | **Step-1 Cross-Cycle KV Prefix Cache Leak (`66.2%` Instead of `95%` on `#6386`–`#6400`)** | In `engine.go:1726`, `LayerRoleSuffix(layer)` (`~1,250` static tokens) sat at the **bottom** of `UserPrompt` (`Bucket 8`), **below** dynamic `Bucket 6 (Cognitive Zone Context)`. Every cycle mutated `Bucket 6`, invalidating all `1,250` static tokens below it (`2,274` uncached tokens/cycle). | Move `LayerRoleSuffix(layer)` up into **`SLA Bucket 3b`** (immediately after `Bucket 3 Manifest`, **above** `HotFiles`, `ActiveGoals`, and the dynamic tail). Now `~5,700 / 6,000` tokens (**`~95%`**) form a single contiguous, byte-frozen prefix across cycles. |
| **S2** | **Hot-File AST Slicer Query Contaminating `SLA Bucket 4`** | Scoring `SLA Bucket 4` (`12,000B`) using dynamic error text (`LastErrorDiag`) would change `Bucket 4` whenever an error changed, invalidating `3,000` cached tokens. | **Dual-Tier AST Slicing:**<br>1. `SLA Bucket 4` (`<= 12,000B` in the static prefix) is keyed **strictly** on `(fileBytes, activeMilestoneText)` (`100%` invariant across cycles on the same milestone).<br>2. When a traceback cites `File "/workspace/<f>", line <L>`, a tight $\pm 12$-line `LINE#HASH` **Fault-Localization Window (`<= 800B`)** is injected into the **Stage-2 Dynamic Tail (`Bucket 7`)**. |
| **S3** | **Noisy FTS5 Memory Injection vs. `P3` Oracle + `kbflow` Conformance Packet** | `P3` proved naive KB injection adds `0 pt` while mutating prompt context, whereas **`oracle`** (target files + target test command) adds **`+11.3 pt` in half the steps (`43 vs 62`)** and **`kbflow`** adds **`+20.0 pt` conformance (`p=0.0034`)**. | Replace noisy FTS5 memory injection in `L6` with the deterministic **`MilestoneOraclePacket`** (`Required Workspace Artifacts`, `Required Wiki Note`, `Defining f2p Command`, `Active p2p Suite`, `Pre-Seeded ace_rlm.py SDK`, and `Mode-3 kbflow Rules`). |
| **S4** | **Premature Step-1 Loop Exit (`alreadyVerifiedInBatch`) & `P5` Non-Inferiority Violation** | In `engine.go:1425`, when `.py --selftest` returned `exit=0` at Step 1, `alreadyVerifiedInBatch = true` exited the cycle before `L6` wrote required `/wiki/notes/*.md` pages or checked off `[x]`, wasting 1–2 extra cycles per milestone. | **0-Turn Runner Auto-Checkoff & Non-Inferiority Continuation:**<br>1. When all required artifacts (`.py` + `.json` + `notes/*.md`) pass `VerifyMilestoneTransition`, the **Go Runner auto-checks `[x]` in `goals.md` (`0` extra LLM turns!)** and ends the cycle.<br>2. If `.py` passed `--selftest` (`exit=0`) but a required `notes/*.md` is still missing, the runner keeps the cycle alive into Step 2 (`96%` KV cache hit) and prompts `L6` to call `wiki_write`. |
| **S5** | **Over-Aggressive `CordisFiber` Rollback Deleting Valid Multi-Step Work** | Rolling back all mutations on any incomplete cycle would delete a working `/workspace/v1_forensics.py` if a cycle ran out of steps before writing its wiki note. | **Selective Per-File Promotion (`PromoteVerified`) & Regression-Only LIFO Rollback (`RollbackCorrupted`):** Files that pass AST check + `--selftest` (`exit=0`) are immediately promoted; at cycle end, only unpromoted files with syntax errors or files that broke a `p2p` test are rolled back in LIFO order. |
| **S6** | **All 4 `review-harness-bugs.md` Evaluation Traps** | 1. Narrow `p2p` check only testing current file.<br>2. Leading-slash-only path contamination regex.<br>3. `conftest.py` / `sitecustomize.py` / stdlib-shadowing test tampering.<br>4. Dropped `test_rc` on timeouts. | 1. Full roadmap `p2p` sweep across `handoff.VerifiedFiles`.<br>2. Slash-agnostic component boundary regex (`(?:^\|[\s"'=/])...`).<br>3. Block `conftest.py`/`sitecustomize.py`/stdlib shadowing + enforce `G8` declared-vs-read provenance.<br>4. Tag timeouts with `ExitCode=124` (`test_rc="timeout"`). |
| **S7** | **Live `#6384`–`#6523` `pip3 install numpy` Loop & Identification-as-Completion Trap** | `/bin/sh -c` masked `pip3` exit code behind `\| tail -5`, and cross-cycle amnesia lost failed mutating shell commands. | `/bin/bash -o pipefail -c` + `0ms` Mode-3 pre-gate blocking `pip/apt` + `AgentLTL` `ForbiddenSigs` veto + `Edit-Triggered Unblock` + pre-seeded `/workspace/ace_rlm.py` stdlib SDK. |
| **S8** | **`SLA Bucket 5` & `Bucket 8` Chopping Off the Second Half of Active Milestone 1.2 (`#6518` Live Prompt Audit)** | `extractActiveGoals` spent `~1,150B` of its `1,500B` budget on the `Mission Statement` header, truncating `Milestone 1.2` mid-sentence at `26,495 telemetry_s...`! `L6` never even saw the requirements for `v1_forensics_report.json`, `notes/v1-forensics-and-literature.md`, or the `SWE-Mutation` audit! | **Active-Milestone-First `Bucket 5` Allocation:** Reserve the first `1,000B` of `Bucket 5` (`<= 1,800B`) for the **complete, untruncated active `- [ ]` milestone bullet** + `550B` for `MilestoneOraclePacket` + `250B` compact mission header. Never truncate the active `- [ ]` bullet mid-sentence! |
| **S9** | **Self-Poisoning `consolidateCycleNorthbound` & Stale Phase-15 `layer_states` (`#6518` Live Prompt Audit)** | `consolidateCycleNorthbound` saved failed `pip3 install` cycles into `GLOBAL` FTS5 memory as `"Cycle Outcome: Find certifi path or use pip trusted-host workaround"`, which `Bucket 7` re-injected into `L6` on every cycle—actively instructing `L6` to keep trying `pip` workarounds! Meanwhile `L1`–`L5` `layer_states` still held stale Phase-15 summaries. | **Strict Memory Hygiene & Mission-Hash State Reset:**<br>1. `consolidateCycleNorthbound` ONLY records `GLOBAL` FTS5 memories when a cycle achieves a verified positive state transition (`--selftest` `exit=0` or Mode-3 pass). Failed attempts live exclusively in `ValidatedHandoffPacket` (`ForbiddenSigs` + `SomaticValence`).<br>2. When `SHA256(goals.md Mission)` changes (or on `v1.5` boot), reset stale `layer_states` summaries and purge poisoned `pip/certifi` FTS5 entries. |
| **S10** | **`Bucket 3 Manifest` Alphabetical Starvation at `archive/sta...` (`#6518` Live Prompt Audit)** | 60+ legacy files (`accept52.py`, `archive/stale_db/*`, `s101.log`, `vd1262.log`) filled the `1,400B` manifest budget at the letter `a` (`archive/sta...`), completely hiding `corpus_synthesizer.py`, `drift_detector.py`, `literature_grounding.json`, and `v1_forensics.py`! | **Workspace Quarantine & Priority-Sorted Manifest:** Skip `archive/` and `.harness_d/` in `ListFiles()`, quarantine legacy pre-v2 clutter into `/workspace/archive/v1_legacy/` on startup (preserving `drift_detector.py` and `accept_phase15.py` for Milestone 1.2's mutation audit), and list root `.py`/`.json`/`.md` files first in `Bucket 3`. |
| **S11** | **Hot-File Slot Starvation When Milestone Mentions 3 Files (`v1_forensics.py`, `drift_detector.py`, `accept_phase15.py`)** | `buildHotFilePreInjection` stopped at `count >= 2` in alphabetical order (`accept_phase15.py`, `drift_detector.py`), which can starve `v1_forensics.py` (starts with `v`). | **Primary-Target-First Hot-File Selection:** Extract the primary target path from the milestone header (`(/workspace/<primary>.py)`) as **Slot #1** before secondary referenced files, and allow up to **3** hot files within `SLABucketHotFilesTotalMaxBytes` (`12,000B`). |
| **S12** | **`extractTopActionableMilestone` Hardcoded `260`-Byte Truncation (`engine.go:1076`)** | `extractTopActionableMilestone(goals)` called `return truncateText(trimmed, 260)` internally, cutting off every filename and requirement after byte 260 (`drift_detector.py`, `accept_phase15.py`, `v1_forensics_report.json`, `notes/v1-forensics-and-literature.md`) before `buildHotFilePreInjection` or `Stimulus` ever saw it! | Remove internal truncation from `extractTopActionableMilestone(goals)` (return the full `trimmed` milestone line up to `1,200B`) so `buildHotFilePreInjection`, `BuildMilestoneOraclePacket`, and `VerifyAndAutoCheckoffMilestone` see 100% of referenced files. |
| **S13** | **`lastL6ReconTurns` Cross-Cycle Carryover Invalidating `39.5%` of Step-1 Tokens & Carrying Over Failed `pip3` Turns (`engine.go:1335, 1495`)** | Verified in `#6527`–`#6538`: `hasMutated` ignored `sandbox_sh`, so failed `pip3 install` turns (`~2,200` tok) were saved in `lastL6ReconTurns` (`messages[2..3]`). On the next cycle's Step 1, `messages[1]` mutated at byte `9,082` (`L6 [zone.reflex]`), invalidating the remaining `6,371B` of `messages[1]` (`LayerRoleSuffix`) + all `8,800B` of `messages[2..3]`—dropping Step-1 KV cache hit to **exact `59.9%` (`4,358 / 7,260` tok)**! | **Delete `lastL6ReconTurns` Carryover & Move `LayerRoleSuffix` to Frozen Prefix (`Bucket 2.5`):**<br>1. Place `LayerRoleSuffix` (`5,048B`) at the top of `messages[1]` (`Bucket 2.5`, above all dynamic state).<br>2. Replace `lastL6ReconTurns` (`messages[2..3]`) with the compact `<= 850B` `Bucket 7` `ValidatedHandoffPacket` at the very end of `messages[1]`.<br>3. Result: Step-1 Frozen Prefix = `13,866B` vs `~700B` tail (**`95.2%` Step-1 KV prefix cache hit**, cutting Step-1 uncached tokens by **`16.5x`** from `2,900` $\rightarrow$ `175` tok!). |
| **S14** | **Premature Step-1 Loop Exit on `alreadyVerifiedInBatch` When Secondary Milestone Artifacts (`notes/*.md` / `.json`) Are Still Missing (`engine.go:1379, 1468`)** | When `L6` wrote `v1_forensics.py` in Step 1 and `d1-reflex` saw `--selftest exit=0`, it set `alreadyVerifiedInBatch = true` and broke out of `runPiProsecutorLoop` after Step 1—before `L6` could write the milestone's required `notes/v1-forensics-and-literature.md`! | Gate loop termination on `VerifyAndAutoCheckoffMilestone`: if `--selftest` passed in Step 1 (`exit=0`) and all milestone artifacts exist, auto-check off `- [x]` in `goals.md` and break (`1` turn total). If `--selftest` passed but a required `notes/*.md` or `.json` artifact is still missing, continue to Step 2 with a specific `nextHint` naming the missing artifact so the entire milestone closes in the same cycle! |
| **S15** | **Hybrid Stage-1 Hot-File Injection (Full Hashlines for Compact Primary Target $\le 150$L, AST + Import Skeleton for Secondary/Large Files)** | Stripping top-level `import` lines or bodies from a small primary target script (`v1_forensics.py`, 119L) would force `L6` to spend an extra turn calling `workspace_read` before `workspace_edit`. | For **Slot #1 (Primary Target Script)**: if $\le 150$ lines ($\le 6,000B$), pre-inject full Pi Hashlines (`LINE#HASH \| code`) for 1-turn zero-read surgical `workspace_edit`; if $> 150$ lines, inject **Imports + Constants + AST `class`/`def` Hashline Index + `main()`**. For **Slots #2–#3 (Secondary Referenced Files)**: inject compact **AST + Import + CLI Skeletons (`<= 1,800B` each)**. |
| **S16** | **64-Token Block-Aligned KV Prefix Accounting & Rolling `log.md` Cap (`opencode_free.go:634`, `engine.go:2284`)** | `UnlockOrCache` capped `radixCached` at an arbitrary `0.96` ratio, and `wiki_write` appended to `log.md` without a rolling size cap. | Align fallback prefix-cache estimation to 256-byte (64-token) vLLM/SGLang KV block boundaries (`(bestSharedBytes / 256) * 256`) while always preferring non-zero provider-reported `cached_tokens`, and cap `log.md` to a rolling `32KB` tail. |
| **S17** | **Structural Dead-Code Confusion, Scratch Pollution & Missing Change Control (`60+` `.log`/`.bak`/dead `.py` files + `notes/goals.md` shadow copy + dead unused `import numpy as np` in `v1_forensics.py`)** | 1. Agents leave `.log`, `.err`, `.out`, `.bak*`, throwaway `accept*.py`, and shadow `wiki/notes/goals.md` / `wiki/read.md` files on disk, then read dead files in later cycles assuming they are live.<br>2. In `v1_forensics.py`, line 12 (`import numpy as np`) was a **100% dead unused import** (`np` wasn't even used in the file!), yet that dead import crashed `--selftest` for 50+ cycles! | **5-Pillar Structural Hygiene, Git Change-Control & Dead-Code GC Engine (`B13`):**<br>1. **Embedded Git Change Control in `/workspace/`:** Auto-init `/workspace/.git`, atomic `[milestone: X.Y]` commits on Mode-3 PASS, and `git checkout/clean` rollback on failed cycles.<br>2. **Post-Exec Scratch Sweep & Clone/Backup Write Block:** Automatically sweep `.log/.out/.err/.bak/.tmp` after every `Exec()` and block writing `*.bak`, `*_v2.py`, `*_fixed.py`, or `*_copy.py` shadow files.<br>3. **AST Live-Closure & Dead-Code Garbage Collector (`RunWorkspaceGC` & `RunWikiGC`):** Any file not in the Live Closure (`goals.md` + AST `import` graph + core dataset) is automatically quarantined to `archive/` and recorded in `.harness_d/gc_ledger.json`.<br>4. **AST Dead-Import Auto-Sanitizer & Unused-Symbol Check:** On every `.py` write/edit, parse the AST to strip/flag dead unused imports (like unused `import numpy as np`) and dead unreachable functions.<br>5. **Canonical Wiki Path Enforcement:** Block/redirect shadow paths (`notes/goals.md` $\rightarrow$ `goals.md`, delete `wiki/read.md`) and archive unreferenced legacy notes. |
| **S18** | **Conceptual Zombie Flows & Additive Accretion (Leaving Superseded Flows/Paradigms Wired In — Both in `/workspace/` AND in `engine.go`/`prompts.go`!)** | 1. Shallow AST linting (`0-caller` check) misses **conceptual dead flows** because old flows remain called by `--selftest`, `except:` shims, dead mode branches, or compute values that never reach the final output sink.<br>2. Even `engine.go` (`1066`, `1244`) and `prompts.go` (`25`, `177`, `113`) still contain hardcoded conceptual remnants from the retired B2B Micro-SaaS mission (`app_server.py --selftest`, `landing.html`, `telegraph_publish`, `mail_tm`, `/checkout`)! | **Whole-Program `AST-PDG` (Program Dependence Graph) Conceptual Flow Analyzer + Change-Control Architecture:**<br>1. **4-Pass Cross-File `AST-PDG` Conceptual Flow Detector (`AnalyzeASTConceptualFlows` in `hygiene.go` & `ace_rlm.py`):**<br>   • **Pass A (Cross-File Production vs Test Call-Graph Partition $R_{\text{test}} \setminus R_{\text{prod}}$):** Builds the cross-file call graph across live `/workspace/*.py` files (treating cross-file imports and `__all__` library exports as live roots) and catches zombie functions/classes that are ONLY reachable from `--selftest`/`test_*` or mutual recursion.<br>   • **Pass B (Backward Data-Flow Slicing from Output Sinks):** Traces def-use chains backward from canonical output sinks (`json.dump`, file writes, return values); flags any executed function/branch whose result never flows into the output sink or is overwritten (shadowed dict keys / killed defs).<br>   • **Pass C (AST I/O-Footprint & Schema Isomorphism):** Fingerprints each function's `(Inputs, SQLTables/Files Read, DictKeys/Schema Written)`; flags competing twin flows when two functions read the same source and produce overlapping output schemas ($\text{Jaccard} \ge 0.50$).<br>   • **Pass D (Inter-Procedural Dead-Mode & `except:` Shim Detection):** Flags `if mode == "X":` branches where no live call site in the workspace passes `"X"`, and flags legacy functions called only inside broad `except Exception:` fallback shims.<br>2. **Purge Retired B2B SaaS Concepts from Go Core (`engine.go`, `prompts.go`, `http.go`, `store.go`)** + **Explicit `Supersedes / Retires` Change-Control Ledger (`/workspace/.harness_d/change_control.json`)** + **Prompt & `d1-3B` Gate Enforcement**. |
| **S19** | **Permanent Non-Reasoning Fallback Lock at `#6378` (`opencode_free.go:269`) & Retired B2B Routes in `http.go`/`store.go`/`local_gpu_gate.go`** | 1. **Empirical Smoking Gun in `ace.db` (`#6300`–`#6601`):** From `#6300`–`#6377` (24 turns), `minimax-m3.1-flash-free` (`Reasoning: true`) completed **3 full milestones**. At `#6378`, `minimax` had 1 transient blip (90s cooldown), failing over to `nemotron-3.5-lightning-free` (`Reasoning: false`). Because `opencode_free.go:269` set `p.activeModel = modelID` on any HTTP 200 response—and never fed **L6 task pass/fail outcomes** back to the router—ACE stayed permanently locked on the non-reasoning `nemotron` model for the next **276 turns (`#6378`–`#6601`)** after `minimax`'s 90s cooldown expired!<br>2. `server/http.go:79-85, 423-490` still mounts retired B2B SaaS `/ingest`, `/query`, `/metrics` routes that shell out to `import app_server` and write `.tmp_ingest.json` into `/workspace/`.<br>3. `memory/store.go:118` still seeds the retired B2B SaaS `goals.md` on new stores, and `local_gpu_gate.go:58` still sends `"engine": "ensemble"` instead of `"d1_4bit"`. | 1. **Task-Outcome-Driven Bayesian `Cost-of-Pass` Router (`RecordTaskOutcome`):** Feed actual `L6` step/cycle outcomes (`--selftest exit=0` / Mode-3 pass vs. `exit!=0` / LTL veto / Mode-3 reject) into each model's `Beta(α_task, β_task)` posterior. After $\ge 2$ consecutive task-level failures on a model (or when a higher-ranked `Reasoning: true` model's cooldown expires), automatically demote the failing model and promote the lowest-`Cost-of-Pass` `Reasoning: true` model (`minimax-m3.1-flash-free`, `deepseek-v4.1-flash-free`, `glm-5.3-flash-free`, `beam-free`).<br>2. **Retire B2B SaaS Zombie Routes in `server/http.go` & `memory/store.go`:** Replace `app_server`/`.tmp_ingest.json` routes in `http.go` with `/api/harness/d` (serving live `.harness_d/pareto.json`, `scoreboard.tsv`, `change_control.json`), default `/showcase/` to `model-viz.html`, update `seedWiki()` in `store.go` to the scientific constitution, and set `"engine": "d1_4bit"` in `local_gpu_gate.go:58`. |

---

## 3. System Topology, 4-Tier Memory Mapping, & Byte-Frozen KV Prefix Architecture

### 3.1 The 5-Module `ModularRSI` Architecture Spine

```
┌──────────────────────────────────────────────────────────────────────────────────────────┐
│                 ACE v0.2 IMMUTABLE CORE RUNTIME (Single Go Binary ./ace)                 │
│     [System Integrity Overlay (SIO): Host Binary, Go Source & Systemd Read-Only]         │
├───────────────────────────┬────────────────────────────┬─────────────────────────────────┤
│ MODULE 1:                 │ MODULE 2:                  │ MODULE 3:                       │
│ Spatiotemporal Boundary & │ AgentLTL, Somatic Markers  │ Two-Stage SLA Context Compiler  │
│ Scientific Sandbox        │ & Validated Handoff        │ & 4-Tier Memory Architecture    │
│ • internal/sandbox/       │ • internal/cognitive/      │ • internal/cognitive/           │
│   jail.go + ace_rlm.py    │   ltl_monitor.go           │   ast_slicer.go & prompts.go    │
│ • /bin/bash -o pipefail   │ • Tier-1 Working Memory    │ • Stage 1 Frozen Prefix (~95%): │
│ • Pre-seeded ace_rlm.py   │   Slots (AttemptLedger,    │   Sys + Tier-3 Wiki/Playbooks + │
│   (Beta CI, McNemar,      │   ActiveError, Somatic,    │   Manifest + RoleContract +     │
│   Bootstrap, AST Mutator) │   ForbiddenSigs, Budget)   │   AST Cone + Tier-2 OracleGoals │
│ • CordisFiber Selective   │ • FO-LTL Rule A6 Veto +    │ • Stage 2 Residual Tail (~5%):  │
│   Promote & LIFO Rollback │   Edit-Triggered Unblock   │   Tier-1 Handoff + ±12L Window  │
├───────────────────────────┴─────────────┬──────────────┴─────────────────────────────────┤
│ MODULE 4:                               │ MODULE 5:                                      │
│ SPOQ Dual Gates & Mode-3 Momus Verifier │ Circuit-Breaker Cost-of-Pass Router & Trace D  │
│ • internal/cognitive/mode3_gate.go      │ • internal/llm/opencode_free.go & engine.go    │
│ • Pre-Gate: Two-Phase Permission Filter │ • Failover Circuit-Breaker (Healthy->Degraded  │
│   + 0ms Typed Rejects (pip/apt,         │   with 1 Jitter Retry->Unhealthy->Probing)     │
│   ro-wiki, conftest/stdlib tamper)      │ • Beta(α,β) 95% CI + KV-Cache Switch Penalty   │
│ • Post-Gate (Momus): Existence + AST    │ • Meta-Harness Filesystem D (/workspace/       │
│   Substance + G8 Provenance + f2p +     │   .harness_d/: scoreboard.tsv, traces/,        │
│   Roadmap p2p Sweep + Blind d1-3B Gate  │   pareto.json, latest_failure.json)            │
│ • 0-Turn Runner Auto-Checkoff +         │ • W4 Periodic Contrastive Trajectory Audit +   │
│   Hermes Pitfall-to-Playbook Distiller  │   Claude Code Snip/Microcompact/Collapse       │
└─────────────────────────────────────────┴────────────────────────────────────────────────┘
```

### 3.2 Exact Mapping Between Jay's 4-Tier Memory (`design/README.md`) and the Two-Stage SLA Prompt Compiler

To satisfy **both** the 4-Tier Memory Architecture (`design/README.md`) **and** the **$\ge 93\%$ cross-cycle KV prefix cache hit invariant**, `ACE v0.2` maps memory tiers by their mutation frequency:

```
=== STAGE 1: BYTE-FROZEN KV PREFIX (~5,700 TOKENS = ~94% OF PROMPT) ===

[MESSAGE 0: SYSTEM PROMPT (100% Frozen Across All Layers & Cycles, <= 4,000B)]
  ├── Bucket 1 (Constitutional Identity + SIO + G6 Date Pin, <= 1,800B):
  │     /wiki/soul.md + Frozen Session Date Pin + Anti-LARP & Anti-Identification-as-Completion Laws
  └── Bucket 2 (Tier-3 Medium-Term Memory: Canonical Wiki & Learned Playbooks, <= 2,200B):
        /wiki/index.md + /wiki/skills/learned-playbooks.md (OpenViking monotonic-versioned skills)

[MESSAGE 1 (TOP): STATIC & SEMI-STATIC USER PROMPT PREFIX]
  ├── Bucket 3  (Graziano L3 Environment & Sorted File Manifest, <= 1,400B):
  │     Deterministically sorted /wiki + /workspace file list + L3 Sandbox/Stdlib Capability Summary
  ├── Bucket 3b (Phase-1 Permission-Filtered Layer Role & JSON Contract, <= 4,500B):  <-- MOVED UP (Fix S1)
  │     LayerRoleSuffix(layer) exposing ONLY tools allowed by Cordis CheckLayerCoeffect(layer)
  ├── Bucket 4  (Static AST Symbol-Cone Hot-File Pre-Injection, <= 12,000B):          <-- Fix S2
  │     SmartSliceHotFileAST(fileBytes, activeMilestoneText) with LINE#HASH (100% cycle-invariant)
  └── Bucket 5  (Tier-2 Short-Term Memory: Active Goals + P3 MilestoneOraclePacket, <= 1,800B): <-- Fix S3
        Active unchecked milestones from /wiki/goals.md + deterministic MilestoneOraclePacket
        (Required Artifacts, Defining f2p Cmd, Active p2p Suite, ace_rlm.py SDK, Mode-3 kbflow)

------------------------------------------------------------------------------------------
              ▲ 100% BYTE-IDENTICAL ACROSS CYCLES ON SAME MILESTONE (~5,700 TOKENS) ▲
              ▼ STAGE 2: DYNAMIC RESIDUAL ERROR VECTOR TAIL (~300–400 TOKENS ONLY)  ▼
------------------------------------------------------------------------------------------

[MESSAGE 1 (BOTTOM): DYNAMIC TAIL]
  ├── Bucket 6  (Compact 3-Zone Bus Context, <= 450B):
  │     Northbound/Southbound zone status (Compass / Tactical / Reflex)
  ├── Bucket 7  (Tier-1 Immediate Working Memory: ValidatedHandoffPacket Slots, <= 850B):
  │     • Slot 1 [AttemptLedger]: Last 6 actions with true pipefail exit_code & test_rc (Snip-compacted)
  │     • Slot 2 [FaultLocalization]: LastErrorDiag + ±12-line LINE#HASH AST Traceback Window
  │     • Slot 3 [L5 Inhibitory Control]: AgentLTL ForbiddenSigs + Damasio SomaticValence V ∈ [-1,+1]
  │     • Slot 4 [LoopMonitor Budget]: CyclesSpent & CumulativeTokens on active milestone
  └── Bucket 8  (Incoming Stimulus Directive, <= 450B):
        Current pulse directive

[MESSAGES 2..2k (INTRA-CYCLE MULTI-STEP APPEND-ONLY TURNS — Claude Code Microcompact)]:
  ├── Assistant Turn j: Raw JSON response (100% preserved for KV prefix continuity)
  └── User Turn j (Step Observation, <= 5,000B):
        When step j+1 starts, if step j-1 observation > 600B, older step observations can be
        micro-compacted on retry while keeping the active turn prefix byte-identical.
```

* **Tier-4 Long-Term Memory (`L4` Pull-Based Tree & Raw Trace Archive, Default 3,000 Tokens on Demand):**
  * Lives in SQLite `memories` (with `agent_id`, `layer_code` isolation, `obs_type`, `importance_score`, `parent_id`, `version`) + `/workspace/.harness_d/` (`Filesystem D`) + `/workspace/ace_v1_traces.db` + `/workspace/research_corpus/`.
  * Queried on-demand by `Compass/Tactical` layers (`SearchForLayer`) and by `L6` via `/workspace/ace_rlm.py` (`query_v1_traces()`, `search_corpus()`) so `L6`'s KV prefix is never polluted by noisy FTS5 hits.

---

## 4. Detailed Specification of the 5 Orthogonal Subsystems

### 4.1 Module 1: Spatiotemporal Boundary, Pipefail Sandbox, & `/workspace/ace_rlm.py` SDK (`internal/sandbox/jail.go` & `internal/cognitive/cordis.go`)

#### A. Deterministic Pipefail Execution & Sorted Manifest (`internal/sandbox/jail.go`)
1. **Pipefail Bash Execution:**
   * Replace `"/bin/sh", "-c", command` with `"/bin/bash", "-o", "pipefail", "-c", command` inside `bwrap` with `--chdir /workspace` and `PYTHONDONTWRITEBYTECODE=1`.
   * Now `pip3 install ... | tail -5` or `python3 script.py --selftest | head -20` deterministically returns the non-zero exit code of the failing command.
2. **Explicit Timeout Tagging (`review-harness-bugs.md` #4):**
   * When `ctx.Err() == context.DeadlineExceeded`, set `res.ExitCode = 124` and prepend `"[TIMEOUT after 25s (test_rc=timeout)]\n"` to `res.Output`.
3. **Deterministic `ListFiles()` Ordering & Archive Filtering:**
   * Sort `ListFiles()` lexicographically by `Path` so `Bucket 3 (Manifest)` is 100% deterministic across cycles.
   * Skip `archive/` and `.harness_d/` in `ListFiles()` (alongside `.tickets`, `.kb`, `.spillover`, `.local`, `__pycache__`, `node_modules`, `bin`) so `Bucket 3 (Manifest)` is never starved by archived files (`Fix S10`).
   * On `NewJail()` initialization, automatically quarantine legacy pre-v2 workspace clutter (`accept5*.py`, `accept7*.py`, `accept8*.py`, `accept101.py`, `*.log`, `*.out`, `*.err`, `*.bak*`) into `/workspace/archive/v1_legacy/` while preserving `drift_detector.py`, `accept_phase15.py`, `corpus_synthesizer.py`, `literature_grounding.json`, `v1_forensics.py`, `ace_v1_traces.db`, and `research_corpus/`.

#### B. Pre-Seeded `/workspace/ace_rlm.py` Scientific & RLM Stdlib SDK (`internal/sandbox/jail.go`)
On `NewJail()` initialization (and verified before each `L6` cycle), `Jail` seeds `/workspace/ace_rlm.py` (100% pure Python 3 stdlib: `math`, `statistics`, `sqlite3`, `json`, `ast`, `re`, `random`, `hashlib`, ` tempfile`, `subprocess`, `pathlib`):
1. `query_v1_traces(sql: str, params: tuple = ()) -> list[dict]`: opens `/workspace/ace_v1_traces.db` in read-only URI mode (`mode=ro`) and returns rows as `dict`s.
2. `trace_era_summary() -> dict`: computes exact per-layer and per-model turns, KV cache hit ratio, uncached token burn, and latency percentiles (`p50`, `p90`, `p99`) across `ace_v1_traces.db`.
3. `beta_ci(k: int, n: int, alpha0: float = 1.0, beta0: float = 1.0) -> dict`: computes Bayesian `Beta(k + alpha0, n - k + beta0)` posterior mean, variance, and 95% credible interval (`[ci_low, ci_high]`) (`Rule A3`).
4. `mcnemar_exact_p(b: int, c: int) -> float`: computes exact two-sided binomial McNemar $p$-value using `math.comb(b + c, i)` (`Rule A1 / P3`).
5. `bootstrap_paired_ci(scores_a: list[float], scores_b: list[float], n_boot: int = 5000, seed: int = 42) -> dict`: computes deterministic paired bootstrap mean difference and 95% CI (`Rule M1`).
6. `audit_script_mutants(script_path: str, max_mutants: int = 8) -> dict`: parses `script_path` via `ast`, generates single-site operator/constant mutants (`>=` $\leftrightarrow$ `<`, `==` $\leftrightarrow$ `!=`, `+` $\leftrightarrow$ `-`, `True` $\leftrightarrow$ `False`), runs `python3 <temp_mutant> --selftest` on each, and returns `{"total_mutants": N, "killed": K, "survived": S, "mutant_kill_rate": K/N}` (`SWE-Mutation Rule M7`).
7. `audit_conceptual_flows(target_path: str) -> dict`: runs **4-Pass Whole-Program `AST-PDG` (Program Dependence Graph) Conceptual Flow Analysis** in pure Python `ast`:
   * **Pass A (`test_only_zombies`):** Partitions the AST Call Graph into $R_{\text{prod}}$ (reachable from production entrypoint excluding `if args.selftest:` / `test_*`) and $R_{\text{test}}$ (reachable only from `--selftest` / `test_*`), returning any function/class in $R_{\text{test}} \setminus R_{\text{prod}}$ (zombie flows kept alive only by obsolete tests).
   * **Pass B (`sink_disconnected_flows` & `shadowed_writes`):** Performs backward data-flow slicing (`def-use` chains) from canonical output sinks (`json.dump`, file writes, `return` values) and detects computations whose outputs never reach a live sink or whose dictionary keys (`d["key"] = ...`) are unconditionally overwritten downstream.
   * **Pass C (`competing_twin_flows`):** Extracts each function's **I/O & Schema Fingerprint** `(SQLTablesRead, FilesOpened, DictKeysWritten, CalleeSet)` and flags pairs of functions with overlapping I/O & output key sets ($\text{Jaccard} \ge 0.50$) that represent competing v1/v2 conceptual flows for the same responsibility.
   * **Pass D (`dead_mode_branches` & `except_fallback_shims`):** Uses inter-procedural call-site constant propagation across the workspace AST to flag `if mode == "X":` branches never passed by any live caller, plus functions called exclusively inside `except Exception:` legacy fallback blocks.
8. `search_corpus(query: str, max_results: int = 10) -> list[dict]`: RLM-style regex/keyword search over `/workspace/research_corpus/` and `/wiki/notes/papers/`.
9. Supports `python3 /workspace/ace_rlm.py --selftest` out of the box (`exit=0`).

#### C. `CordisFiber` Temporal Revertible Effects & Spatial Coeffects (`internal/cognitive/cordis.go`)
1. **`CordisFiber` Struct:**
   ```go
   type FileSnapshot struct {
       RelPath   string
       Existed   bool
       Content   []byte
       SHA256Hex string
   }

   type CordisFiber struct {
       CycleID           string
       Snapshots         map[string]FileSnapshot
       MutationOrder     []string
       PromotedVerified  map[string]bool
   }
   ```
   * `SnapshotBefore(sandbox core.Sandbox, relPath string)`: captures pre-mutation bytes and SHA-256 before the first write/edit to `relPath` in the cycle.
   * `PromoteVerified(relPath string)`: marks `relPath` as verified once it passes AST syntax check and `--selftest` (`exit=0`).
   * `RollbackCorrupted(sandbox core.Sandbox, roadmapVerified map[string]bool) []string`: at cycle exit, iterates `MutationOrder` in reverse (LIFO) order and restores pre-cycle bytes **only** for files that were NOT promoted in this cycle AND either (a) currently have a Python AST syntax error (`CheckPythonAST != ""`) OR (b) were previously verified in `roadmapVerified` and now fail `--selftest`.
2. **Two-Phase Permission Filtering & Host Immutability Lock (`CheckLayerCoeffect`):**
   * Static Coeffect map per layer:
     * `L1`–`L5`: `none`, `wiki_read`, `wiki_write`, `kb_search`, `kb_note`, `ticket_list`, `ticket_Update` (never `workspace_write`, `workspace_edit`, or `sandbox_sh`).
     * `L6`: full workspace, sandbox, ticket, kb, web, and `/wiki/goals.md` + `/wiki/notes/*` actuators.
   * **System Integrity Overlay (`EnforceImmutableHostBoundary`):** Slash-agnostic component boundary check (`review-harness-bugs.md` #2) blocking any reference to `..`, `/home/`, `/var/home/`, `internal/cognitive`, `internal/sandbox`, `internal/llm`, `cmd/ace`, `gpu_gate_server.py`, `systemctl`, `pkill`, `killall`, `fuser -k`, or `go build -o ace`.
3. **Claude Code Safety-Classified Tool Partitioning (`IsActionConcurrencySafe`):**
   * Classifies actions as read-only concurrency-safe (`workspace_read`, `wiki_read`, `web_search`, `web_fetch`, `ticket_list`, `kb_search`) vs. serial-mutating (`workspace_write`, `workspace_edit`, `wiki_write`, `sandbox_sh`), and verifies path disjointness across batched actions.
4. **"Model-Visible Means Logged" Prefix & Chain Hashing (`ComputePromptProvenance`):**
   * Computes `sys12:static12:dyn12:chain12` SHA-256 provenance digest for every LLM call.
5. **5-Pillar Structural Hygiene, Git Change-Control & Dead-Code Garbage Collection Engine (`internal/sandbox/hygiene.go` — `B13` / `Fix S17`):**
   * **Pillar 1 (Embedded Git Change-Control Spine in `/workspace/`):**
     * `InitWorkspaceGit()` initializes `/workspace/.git` with a strict `.gitignore` (`__pycache__/`, `.spillover/`, `.tickets/`, `.kb/`, `.harness_d/`, `archive/`, `*.pyc`, `*.log`, `*.out`, `*.err`, `*.bak*`, `*.tmp*`, `.srv.pid`).
     * **Atomic Mode-3 Milestone Commits (`CommitMilestoneArtifacts`):** When a milestone passes the `Mode-3` gate, stages only the live closure (`git add`) and commits: `[milestone: X.Y] Verified <artifacts> (Mode-3 PASS)`.
     * **Working-Tree Rollback on Failed Cycles:** Together with `CordisFiber`, reverts any uncommitted scratch pollution or broken partial edits when a cycle fails.
   * **Pillar 2 (Post-Exec Scratch Sweep & Anti-Clone Write Gate):**
     * `SweepScratchFiles()` runs automatically after every `Jail.Exec()` to move/remove `.log`, `.out`, `.err`, `.bak*`, `.tmp*`, `.pid`, and scratch `.db` files from `/workspace/`.
     * `ValidateCleanPath(relPath)` rejects creating `.bak`, `.tmp`, `.orig`, or duplicate suffix clones (`*_v2.py`, `*_fixed.py`, `*_new.py`, `*_copy.py`) at `WriteFile`/`EditFile`, forcing in-place edits on canonical files under Git + Cordis version control.
   * **Pillar 3 (AST Live-Closure Graph & Automatic Dead-Code Garbage Collector — `RunWorkspaceGC`):**
     * Computes the **Live Artifact Closure** of `/workspace/`:
       $$\text{LiveClosure} = \text{CoreDataset} \cup \text{FilesInActiveGoals}(\text{goals.md}) \cup \text{ASTImportsClosure}(\text{LiveFiles})$$
     * Any file in `/workspace/` outside `LiveClosure` (e.g., dead scripts from prior missions like `mvp_engine.py`, `venture_model.py`, `accept51.py`..`accept101.py`, `verify52.sh`, `consolidate_wiki.sh`) is automatically moved to `/workspace/archive/v1_legacy/` and logged in `/workspace/.harness_d/gc_ledger.json`.
     * Runs on startup, on mission change, and after every Mode-3 milestone completion.
   * **Pillar 4 (AST Dead-Import Auto-Sanitizer & Dead-Symbol Gate — `SanitizePythonAST`):**
     * On every `.py` `workspace_write` or `workspace_edit`, runs a Python `ast` pass that detects **unused imports** (such as the unused `import numpy as np` on line 12 of `v1_forensics.py` where `np` was never referenced!) and automatically strips unused unresolvable imports + warns on dead unreachable functions before `--selftest` runs.
   * **Pillar 5 (Canonical Wiki Path Enforcement & `RunWikiGC`):**
     * Top-level `/wiki/` only permits `soul.md`, `index.md`, `goals.md`, and `log.md`. Shadow copies (`wiki/notes/goals.md`, `wiki/notes/soul.md`, `wiki/read.md`) are purged and transparently redirected to their canonical top-level path.
     * `RunWikiGC` archives unreferenced notes from superseded missions (`notes/live-launch-urls.md`, `notes/phase12.1-human-handoff-packet.md`, `notes/phase16-drift-detector.md`) into `/wiki/archive/v1_legacy/` so `L6` never reads dead B2B SaaS notes during `ACE v2` research.

---

### 4.2 Module 2: `AgentLTL`, Damasio Somatic Markers, Loop Monitor, & `ValidatedHandoffPacket` (`internal/cognitive/ltl_monitor.go`)

1. **`ValidatedHandoffPacket` (`Tier-1 Working Memory Slots` in `engine_meta["validated_handoff"]`):**
   ```go
   type AttemptRecord struct {
       CycleID   string `json:"cycle_id"`
       Step      int    `json:"step"`
       Action    string `json:"action"`
       Target    string `json:"target"`
       CmdSig    string `json:"cmd_sig"`
       ExitCode  int    `json:"exit_code"`
       TestRC    string `json:"test_rc"`
       Outcome   string `json:"outcome"`
   }

   type ValidatedHandoffPacket struct {
       MilestoneLine     string             `json:"milestone_line"`
       MilestoneHash     string             `json:"milestone_hash"`
       CyclesSpent       int                `json:"cycles_spent"`
       TokensSpent       int                `json:"tokens_spent"`
       VerifiedFiles     []string           `json:"verified_files"`
       LastErrorDiag     string             `json:"last_error_diag"`
       LastTracebackFile string             `json:"last_traceback_file"`
       LastTracebackLine int                `json:"last_traceback_line"`
       RecentAttempts    []AttemptRecord    `json:"recent_attempts"`
       ForbiddenSigs     []string           `json:"forbidden_sigs"`
       SomaticValence    map[string]float64 `json:"somatic_valence"`
   }
   ```
2. **FO-LTL Temporal Rules (`AgentLTL` `Rule A6` + Logseq Anti-"Identification-as-Completion" Law):**
   * **Rule LTL-1 (`CheckPreActionLTL` — Anti-Command-Override Veto):** If `act.Type == "sandbox_sh"` and `NormalizeCommandSig(act.Target)` is in `handoff.ForbiddenSigs`, blocks execution in `0ms` with `[AGENT-LTL OVERRIDE VETO (Rule A6)]` and instructs `L6` to mutate the source file via `workspace_write` or `workspace_edit` first.
   * **Rule LTL-2 (`PromoteOrClearTestSigsOnEdit` — Edit-Triggered Unblock):** Only an actual file mutation (`workspace_write` or `workspace_edit` on `/workspace/<f>`) clears `<f>`'s test command from `ForbiddenSigs`. Merely writing a `thought` or `wiki_write` reflection never clears `ForbiddenSigs` (enforcing the Logseq `2026-W11` Cynical Auditor rule!).
   * **Rule LTL-3 (Damasio Somatic Valence Update):**
     * Gate reject / LTL veto $\rightarrow$ $V(\text{sig}) = \max(-1.0, V - 0.5)$
     * Non-zero `pipefail` exit / Traceback $\rightarrow$ $V(\text{sig}) = \max(-1.0, V - 0.4)$
     * Passing `--selftest` (`exit=0`) $\rightarrow$ $V(\text{sig}) = \min(+1.0, V + 0.5)$
   * **Rule LTL-4 (`design/README.md` Loop Budget Monitor):** Tracks `CyclesSpent` and `TokensSpent` on the active milestone. If `CyclesSpent >= 4` without a verified artifact promotion, injects an `[L5 LOOP MONITOR ESCALATION]` directive forcing `L6` to simplify the implementation to minimal pure-stdlib Python using `import ace_rlm`.

---

### 4.3 Module 3: Two-Stage SLA Context Compiler, Dual-Tier AST Slicer, & `MilestoneOraclePacket` (`internal/cognitive/ast_slicer.go` & `prompts.go`)

1. **`SmartSliceHotFileAST(raw string, milestoneQuery string, maxBytes int) string` (`Pass-2 Fix S2`):**
   * Formats `raw` with `FormatWithHashlines(raw)`.
   * If `len(numbered) <= maxBytes`, returns `numbered` in full.
   * Otherwise:
     * Extracts top-level Python/Go AST blocks (`imports/constants`, each `class`/`def`/`func` with line ranges, and `__main__` entrypoint).
     * Emits a compact `=== AST SYMBOL & LINE DIRECTORY ===` (`<= 650B`).
     * Scores blocks **strictly** against `milestoneQuery` (plus structural weights for `imports`, `selftest`, `main`) so `SLA Bucket 4` is **100% invariant across cycles on the same milestone**.
     * Packs top-scoring blocks in line order up to `maxBytes`.
2. **`ExtractTracebackWindow(raw string, lineNum int, radius int) string`:**
   * Extracts lines `[max(1, lineNum-radius) .. min(total, lineNum+radius)]` with `LINE#HASH` (`<= 800B`) for `Bucket 7` whenever `handoff.LastTracebackLine > 0`.
3. **`BuildMilestoneOraclePacket(milestoneLine string, verifiedP2PFiles []string) string` (`P3` Oracle + `kbflow` Packet) & Active-Milestone-First `Bucket 5` (`Fix S8` & `Fix S11`):**
   * `extractActiveGoals(rawGoals)` reserves the first `1,000B` of `Bucket 5` (`<= 1,800B`) for the **complete, untruncated active `- [ ]` milestone bullet**, plus `550B` for `BuildMilestoneOraclePacket` and `250B` for the compact mission header—guaranteeing the active milestone is never truncated mid-sentence (`Fix S8`).
   * `BuildMilestoneOraclePacket` explicitly declares the **Allowed Live Write Targets** for the active milestone (e.g., `/workspace/v1_forensics.py`, `/workspace/v1_forensics_report.json`, `/wiki/notes/v1-forensics-and-literature.md`) and warns: `"CHANGE-CONTROL RULE: Edit canonical files in-place. Creating undeclared files, *_v2.py clones, .bak/.log files, or leaving unused imports / dead functions will be rejected by the Mode-3 Hygiene Gate."`
   * `buildHotFilePreInjection(activeMilestone)` extracts the primary target script from the milestone title (`(/workspace/<primary>.py)`) as **Slot #1** before scanning secondary files, and allows up to **3** files within `SLABucketHotFilesTotalMaxBytes` (`12,000B`) (`Fix S11`).
   * `consolidateCycleNorthbound` only writes `GLOBAL` FTS5 memories when a cycle achieves a verified positive state transition (`--selftest` `exit=0` or Mode-3 completion), and `NewEngine()` resets stale `layer_states` summaries + purges poisoned `pip/certifi` FTS5 entries when the `goals.md` mission hash changes (`Fix S9`).
4. **Prompt-Level Hygiene, Conceptual Flow Retirement & Change-Control Laws (`prompts.go`, `soul.md`, `skills/ponytail.md`):**
   * **`LayerSystemPrompt` & `FastChatSystemPrompt` (`=== CONCEPTUAL COHESION, CHANGE CONTROL & HYGIENE LAW ===`):**
     1. **Replace, Never Layer (Retire Superseded Conceptual Flows Completely):** When a change or milestone upgrades a mechanism, heuristic, schema, or flow, **DELETE the superseded flow, its call sites, its legacy fallback branches, and its obsolete test assertions**. Never leave v1 and v2 conceptual flows wired side-by-side in the same module or codebase.
     2. **One Canonical Flow Per Responsibility:** Every subsystem must have exactly one live implementation. Never create parallel `*_v2`, `*_new`, `legacy_*`, or fallback shim functions, and never create `*_v2.py`, `*_fixed.py`, `*.bak`, or shadow copies (`notes/goals.md`). Edit the canonical file in-place under `Cordis` + `Git` change control.
     3. **Retire Superseded Docs, Prompts & Tests in Lockstep:** Whenever a conceptual flow retires, update or archive every Wiki note (`index.md`, `notes/*.md`), docstring, and test assertion that described the old flow in the **same cycle** so future cycles never read obsolete concepts and assume they are live.
     4. **Delete Syntactic & Semantic Dead Code Immediately:** Remove unused imports, dead uncalled helpers, and commented-out code blocks before completing any edit.
     5. **Zero Scratch Pollution in `/workspace/`:** Never redirect output to `.log`, `.out`, `.err`, or `.tmp` files in `/workspace/`. Use stdout/stderr directly or Python `tempfile.TemporaryDirectory()` (`/tmp` tmpfs).
   * **Purge Retired B2B Micro-SaaS Concepts from Go Prompts & Reflexes (`Fix S18`):**
     * Remove all retired B2B micro-SaaS references (`app_server.py`, `landing.html`, `Telegraph`, `/checkout`, `buyer@example.com`, `mail_tm`) from `LayerSystemPrompt`, `FastChatSystemPrompt`, `LayerRoleSuffix(L6)`, and `tryDirectWorldReflex`, replacing them with clean scientific meta-harness grounding.
   * **`LayerRoleSuffix(L2, L3, L4, L6)`:**
     * `L2/L4`: When decomposing or advancing a phase/milestone, explicitly state what prior flow or heuristic is **superseded and must be retired**.
     * `L3 (Architecture Steward)`: Audits that `/workspace/` and `/wiki/` maintain a single canonical flow per capability with zero zombie flows.
     * `L6`: Explicitly warns that the `Mode-3` Gate rejects both syntactic dead code (unused imports/functions) AND **conceptual accretion** (leaving superseded flows, parallel legacy/new branches, retired paradigm keywords, or stale Wiki descriptions alive).

---

### 4.4 Module 4: `SPOQ` Dual Gates, `Mode-3` Momus Verifier, Runner Auto-Checkoff, & Hermes Skill Distiller (`internal/cognitive/mode3_gate.go`)

1. **SPOQ Pre-Execution & Change-Control Gates (`InterceptSandboxCommand` & `CheckWriteTargetHygiene`):**
   * **Reject 1 (Offline Package Managers):** Blocks `pip`, `pip3`, `python3 -m pip`, `apt`, `apt-get`, `apk`, `npm`, `cargo` install/update commands in `0ms` with `[MODE-3 GATE REJECT: SANDBOX_OFFLINE_READONLY_USR]` and points `L6` to `import ace_rlm` + Python 3 stdlib (`sqlite3`, `json`, `math`, `statistics`, `ast`).
   * **Reject 2 (Read-Only `/wiki/` Shell Writes):** Blocks shell redirects/writes targeting `/wiki/` in `0ms` and directs `L6` to `action.type="wiki_write"`.
   * **Reject 3 (Test-Infra & Stdlib Shadowing Tampering — `review-harness-bugs.md` #3):** Blocks creation/modification of `conftest.py`, `sitecustomize.py`, `usercustomize.py`, or stdlib-shadowing files (`sqlite3.py`, `json.py`, `math.py`, `statistics.py`, `unittest.py`, `ace_rlm.py`).
   * **Reject 4 (Shell Scratch-Redirect & Backup Pollution Gate):** Blocks `sandbox_sh` commands that redirect or copy into `/workspace/*.log`, `*.out`, `*.err`, `*.bak*`, or `*.tmp*` (`[MODE-3 HYGIENE GATE REJECT: NO_WORKSPACE_SCRATCH_POLLUTION] Use stdout/stderr directly or /tmp for scratch files`).
   * **Reject 5 (Pre-Write Change-Control & Target-Allowlist Gate — `CheckWriteTargetHygiene`):** Before `workspace_write` or `workspace_edit` runs:
     * Rejects backup/clone filenames (`*.bak*`, `*.tmp*`, `*_v2.py`, `*_fixed.py`, `*_new.py`, `*_copy.py`, `*.log`, `*.out`, `*.err`).
     * Verifies the target file is in the **Allowed Live Target Set** (referenced in `goals.md`, imported by a live script, or inside `/workspace/ace_v2/`), blocking ad-hoc throwaway script creation in `/workspace/` root.
2. **SPOQ Post-Execution & Milestone Completion Gate (`VerifyMilestoneTransition` — The Independent `Momus` Critic):**
   * Extracts required `/workspace/*` and `notes/*.md` artifacts from the milestone line and verifies:
     1. **Existence & Non-Trivial Size:** Every required `/workspace/*` file ($\ge 120$B) and `/wiki/notes/*.md` file ($\ge 200$B) exists.
     2. **AST Substance, Dead-Code Gate, Zero Unavailable Imports, & `G8` Provenance (`AuditPythonArtifactSubstance`):**
        * Zero Python syntax errors (`CheckPythonAST`).
        * Zero imports of unavailable third-party packages (`numpy`, `pandas`, `scipy`, `torch`, `sklearn`, `pytest`) outside comments/strings.
        * **AST Unused-Import & Dead-Code Gate:**
          * **Zero Unused Imports:** Every `import X` / `from X import Y` symbol must be referenced in the module AST (`ast.Name` / `ast.Attribute`). Unused unresolvable imports are auto-stripped on write, and any remaining unused import fails with `[MODE-3 DEAD-CODE GATE]: Unused import '<name>' on line N — delete it`.
          * **Zero Dead Uncalled Functions:** Every `def` (other than `main` or `test_*`) in a standalone script must be called at least once in the module AST (or exported in `__all__`).
          * **Zero Unreachable Statements:** Rejects statements appearing after `return`, `raise`, `break`, `continue`, or `sys.exit` in the same block.
          * **Zero Commented-Out Code Blocks:** Rejects files with $>4$ consecutive lines of commented-out code (`# def `, `# import `, `# for `).
        * At least 2 real assertions/checks (`assert `, `assertEqual`, `assertTrue`, `raise AssertionError`, `sys.exit(1)`).
        * **Declared-vs-Read Provenance (`G8`):** If the milestone cites `ace_v1_traces.db`, the script must actually query `ace_v1_traces.db`; if it cites `research_corpus`, it must read `research_corpus`.
     3. **Runner-Executed `f2p` `--selftest`:** Runs `python3 /workspace/<script>.py --selftest` in `bwrap` (`bash -o pipefail`, 20s timeout) and requires `ExitCode == 0` (`test_rc="0"`).
     4. **Full Roadmap `p2p` Regression Sweep (`review-harness-bugs.md` #1):** Re-runs `--selftest` across all previously verified scripts in `handoff.VerifiedFiles`.
     5. **Working-Tree Cleanliness & Change-Control Gate (`CheckWorkspaceCleanliness`):** Verifies zero leftover `.log/.out/.err/.bak/.tmp` scratch files in `/workspace/`, zero shadow Wiki files (`wiki/notes/goals.md`, `wiki/read.md`), and commits the verified milestone artifacts atomically to `/workspace/.git`.
     6. **Whole-Program `AST-PDG` Conceptual Flow & Retirement Gate (`AuditConceptualCohesion` + `ace_rlm.audit_conceptual_flows` + Semantic `d1-3B` Critic — `Fix S18`):**
        * **4-Pass `AST-PDG` Structural Flow Verification (`<15ms`, `0` Cloud Tokens):**
          1. **Test-Anchored Zombie Flow Check ($R_{\text{test}} \setminus R_{\text{prod}} = \emptyset$):** Rejects non-test functions/classes that are only reachable from `--selftest` / `test_*` (or mutual recursion) and disconnected from the production execution path.
          2. **Backward Sink-Reachability Slicing (`Def-Use PDG`):** Traces data flow backward from the milestone's declared output sinks (`json.dump` to target `.json`, file writes, or entrypoint output); rejects dead-end computational flows whose output never flows into a live sink or whose dictionary keys are shadowed/overwritten downstream.
          3. **I/O & Schema Isomorphism (Twin-Flow Detection):** Fingerprints `(SQLTablesRead, FilesOpened, DictKeysWritten)` per function and rejects modules containing two competing conceptual pipelines producing overlapping output schemas ($\text{Jaccard} \ge 0.50$).
          4. **Inter-Procedural Dead-Mode & `except:` Shim Detection:** Rejects dead parameter branches (`if mode == "legacy":` never passed by any live caller) and legacy functions invoked only inside `except Exception:` fallback blocks.
        * **Change-Control Retirement Ledger (`/workspace/.harness_d/change_control.json`):** Tracks `ActiveFlows`, `RetiredFlows`, and `RetiredConceptKeywords` (removing retired flows from `p2p` requirements and reverse-verifying that they are absent from the live tree).
        * **Semantic `d1-3B` / `Momus` Conceptual Retirement Gate (`M5` & `A5`):** Evaluates `(milestoneLine, pdgFlowReport, gitDiffSummary, f2pRC, p2pRC)` to confirm both substantive correctness and clean retirement of superseded flows.
3. **0-Turn Runner Auto-Checkoff (`tryMode3AutoCompleteMilestone`) & Hermes Pitfall-to-Playbook Distillation:**
   * Whenever `L6` mutates workspace or wiki artifacts, the runner evaluates `VerifyMilestoneTransition` on `activeMilestone`:
     * If `Verdict.Passed == true`:
       1. Automatically marks `activeMilestone` as `- [x]` in `/wiki/goals.md` (`0` extra LLM turns!).
       2. If `handoff.RecentAttempts` contained $\ge 1$ prior failure or gate reject (`CR > 0`), distills a 2-line **Hermes Playbook Rule** (`Pitfall -> Verified Fix`) into `/wiki/skills/learned-playbooks.md` with monotonic version increment (`OpenViking` pattern).
       3. Appends the milestone's `.py` scripts to `handoff.VerifiedFiles` and resets `handoff` for the next milestone.
     * If all `.py` and `.json` checks passed (`f2p_rc="0"`, `p2p_rc="0"`) and **only** a required `/wiki/notes/*.md` synthesis note is missing:
       1. Keeps the cycle alive (`alreadyVerifiedInBatch = false`) so Step 2 runs immediately in the same cycle at `96%` KV prefix cache hit.
       2. Injects `[MODE-3 PROGRESS]: Code & selftest verified (exit=0). Now call action.type="wiki_write" to write the required synthesis note.`

---

### 4.5 Module 5: Provider Group Circuit-Breaker, Bayesian `Cost-of-Pass` Router, & Meta-Harness Filesystem `D` (`internal/llm/opencode_free.go` & `internal/cognitive/engine.go`)

1. **Provider Circuit-Breaker State Machine (`providers/research.md`) + Task-Outcome Bayesian `Beta(α, β)` `Cost-of-Pass` Router (`internal/llm/opencode_free.go` — `Fix S19`):**
   * Each model tracks transport health state (`healthy`, `degraded`, `unhealthy`, `probing`), **Task-Level Bayesian reliability parameters (`taskAlpha`, `taskBeta`)** in addition to HTTP transport reliability, `ewmaCacheRatio`, and `ewmaLatencyMs`.
   * **Reasoning-First Priority Tiering:** Prioritizes `Reasoning: true` models (`minimax-m3.1-flash-free`, `deepseek-v4.1-flash-free`, `glm-5.3-flash-free`, `beam-free`) ahead of `Reasoning: false` models (`nemotron-3.5-lightning-free`) so a transient 90s rate-limit on `minimax` never permanently locks `activeModel` onto a non-reasoning fallback model (`#6378`–`#6601` root cause!).
   * **`RecordTaskOutcome(modelID string, passed bool)`:** Called by `engine.go` after every `L6` step/cycle:
     * Verified mutation / `--selftest exit=0` / Mode-3 pass $\rightarrow$ `taskAlpha += 1.0`, `consecutiveTaskFailures = 0`.
     * `exit != 0` / LTL veto / Mode-3 gate reject $\rightarrow$ `taskBeta += 1.0`, `consecutiveTaskFailures++`. If `consecutiveTaskFailures >= 2`, puts the stuck model on a `3-minute` task-level cooldown and rotates to the best-scoring `Reasoning: true` model!
   * **1 Sticky Retry with Full Jitter (`providers/research.md` §4.2):** When `activeModel` encounters a transient HTTP failure (timeout / 5xx / empty stream), marks state `degraded` and performs **1 retry after `150ms + jitter(0..150ms)`** on the **same `activeModel`** before failing over, preserving the warm KV prefix cache!
   * **Expected `Cost-of-Pass` Ranking (`Rule A8` & `Rule A3`):**
     $$\hat{p}_{m,\text{low}} = \text{BetaLower95}(\alpha_{m,\text{task}}, \beta_{m,\text{task}}), \qquad \widehat{\text{CoP}}(m) = \frac{(1 - \hat{r}_{\text{cache}}(m) \cdot \mathbb{I}[m = m_{\text{active}}]) \cdot C_{\text{prompt}} + C_{\text{Penalty}}(m)}{\max(0.05, \hat{p}_{m,\text{low}})}$$
2. **Meta-Harness Filesystem `D` (`/workspace/.harness_d/`) & `W4` Contrastive Self-Audit (`internal/cognitive/engine.go`):**
   * Every cognitive cycle updates `/workspace/.harness_d/`:
     * `/workspace/.harness_d/scoreboard.tsv`: Karpathy `autoresearch`-style append-only ledger (`cycle_id`, `timestamp`, `layer`, `milestone_hash`, `model`, `prompt_tokens`, `cached_tokens`, `cache_hit_pct`, `test_rc`, `status`, `prefix_hash`).
     * `/workspace/.harness_d/traces/<cycle_id>.json`: full raw trace (`Meta-Harness` `Filesystem D`).
     * `/workspace/.harness_d/latest_failure.json`: most recent failure/reject diagnostic + AST traceback window.
     * `/workspace/.harness_d/pareto.json`: multi-objective Pareto frontier (`strict_resolved_milestones`, `mean_turns_per_milestone`, `mean_uncached_tokens_per_turn`, `kv_cache_hit_pct`, `mode3_correction_rate_CR`).

---

## 5. Implementation Slice Plan (5 Atomic Vertical Slices)

1. **Slice 1 (`internal/sandbox/jail.go`, `internal/sandbox/hygiene.go`, `ace_rlm_seed.go`, `jail_test.go`):**
   * `/bin/bash -o pipefail -c` with `--chdir /workspace`, timeout `ExitCode=124` (`test_rc=timeout`) tagging, deterministic `ListFiles()` sorting, automatic seeding + self-test of `/workspace/ace_rlm.py`, and the **5-Pillar Structural Hygiene, Git Change-Control & Dead-Code GC Engine (`hygiene.go`)**.
2. **Slice 2 (`internal/cognitive/cordis.go`, `ltl_monitor.go`, `mode3_gate.go`, `cordis_mode3_test.go`):**
   * `CordisFiber` (`SnapshotBefore`, `PromoteVerified`, `RollbackCorrupted`), `CheckLayerCoeffect`, `EnforceImmutableHostBoundary`, `IsActionConcurrencySafe`, `ComputePromptProvenance`.
   * `ValidatedHandoffPacket` & `LTLMonitor` (`NormalizeCommandSig`, `CheckPreActionLTL`, `PromoteOrClearTestSigsOnEdit`, `RecordPostActionLTL`, Damasio `SomaticValence`, Loop Budget Monitor).
   * `InterceptSandboxCommand`, `AuditPythonArtifactSubstance`, and `VerifyMilestoneTransition`.
3. **Slice 3 (`internal/cognitive/ast_slicer.go`, `prompts.go`, `engine.go`, `internal/memory/store.go`):**
   * `SmartSliceHotFileAST` (static `Bucket 4`), `ExtractTracebackWindow` (dynamic `Bucket 7`), `BuildMilestoneOraclePacket` (`Bucket 5`).
   * Two-Stage Prompt Prefix Re-Ordering (`LayerRoleSuffix` moved into `Bucket 3b` above dynamic tail) + Frozen Session Date Pin (`G6`) + Epistemic Wiki Headers (`M8`) + OpenViking monotonic skill distillation (`/wiki/skills/learned-playbooks.md`).
   * Wire `CordisFiber`, `LTLMonitor`, `Mode3Gate` (`tryMode3AutoCompleteMilestone`), `syncHarnessFilesystemD` (`/workspace/.harness_d/`), and `W4` Contrastive Self-Audit into `engine.go`.
4. **Slice 4 (`internal/llm/opencode_free.go`, `internal/llm/local_gpu_gate.go`, `internal/server/http.go`, `internal/memory/store.go`, & `opencode_free_test.go`):**
   * Circuit-Breaker State Machine (`healthy -> degraded -> unhealthy -> probing`) + 1 sticky jitter retry on `activeModel` + **Task-Outcome-Driven Bayesian `Beta(α_task, β_task)` 95% CI `Cost-of-Pass` router (`RecordTaskOutcome`)** + Reasoning-First tiering (`Fix S19`).
   * Set `"engine": "d1_4bit"` in `local_gpu_gate.go:58`, retire B2B SaaS `app_server`/`.tmp_ingest.json` routes in `server/http.go` in favor of `/api/harness/d`, and update `seedWiki()` in `memory/store.go`.
5. **Slice 5 (Build, Test, Restart & Live Verification on `ace.db`):**
   * Run `go test ./...` across all packages, build `./ace`, restart `ace.service`, and verify on live traces that `ACE v0.2` breaks the `#6384`–`#6400` `numpy` loop in 1 turn, completes Milestone 1.2 under Mode-3 verification (`f2p` + `p2p` + wiki note + 0-turn auto-checkoff) at `>90%` KV prefix cache hit, and advances to Phase 2.
