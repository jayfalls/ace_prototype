# Slice 10: Self-Improvement & Learning Loops

**Unit:** agents-study  
**Output:** `design/units/agents-study/study/self-improvement.md`  
**Research done by:** Dev Loop execution against live source files and papers  
**Date:** 2026-04-23

---

## 1. Overview

Self-improvement in agentic systems spans a spectrum from simple trajectory capture to full outer-loop optimization of model harnesses. This slice examines seven distinct self-improvement mechanisms across production systems (Hermes Agent, OpenClaw, Devin) and research systems (autoresearch, RISE, AlphaEvolve, Meta-Harness). The cross-cutting dimensions are: feedback loop architecture, what is being optimized, the search/budget mechanism, git-based rollback, population-based search, RL fine-tuning, and meta-learning stacks.

---

## 2. Systems with Self-Improvement Mechanisms

### 2.1 Hermes Agent — Trajectory-to-Skill Capture

**Source:** `hermes-agent/tools/skill_manager_tool.py`, `hermes-agent/agent/trajectory.py`

Hermes Agent has two complementary self-improvement mechanisms:

**Trajectory saving** (`agent/trajectory.py`): After each conversation, the full ShareGPT-format trajectory (messages + tool calls + model outputs) is appended to a JSONL file. Completed trajectories go to `trajectory_samples.jsonl`; failed ones to `failed_trajectories.jsonl`. This is passive capture — the agent does not yet synthesize trajectories into reusable artifacts automatically.

**Skill creation** (`tools/skill_manager_tool.py`): The agent has a dedicated `skill_manage()` tool that can `create`, `edit`, `patch`, `delete` skills. Skills are YAML-frontmatter + markdown body files stored in `~/.hermes/skills/`. The tool schema explicitly instructs the agent when to create skills:

- Complex task succeeded (5+ tool calls)
- Errors were overcome
- User-corrected approach worked
- Non-trivial workflow discovered
- User asks to remember a procedure

Skill updates trigger when instructions are stale, OS-specific failures occur, or missing steps/pitfalls are found during use.

**Key design details:**
- Security scan on write: if `skills.guard_agent_created=true`, new/edited skills are scanned. On block, the write is rolled back atomically (`_atomic_write_text` + rollback on scan failure).
- Fuzzy matching for patches: uses the same engine as the file patch tool so indentation/whitespace mismatches don't cause failures.
- Skill directory structure: `SKILL.md` + `references/`, `templates/`, `scripts/`, `assets/` subdirectories.
- Max content: 100K chars for SKILL.md, 1 MiB per supporting file.
- External-dir skills (from hub installs) are read-only; only `~/.hermes/skills/` skills can be modified.

**Evaluation:** The trajectory → skill synthesis is not yet automated. The agent must manually review failed trajectories and author SKILL.md content. Quality of captured skills depends entirely on the agent's ability to distill a successful approach into trigger conditions, numbered steps, pitfalls, and verification steps. The `skills_hub.py` system (Hub) manages external skill installation from GitHub repos, with trust levels (builtin, trusted, community), quarantine, and audit logging — but this is skill *acquisition*, not skill *creation*.

### 2.2 Karpathy/autoresearch — 5-Minute Fixed Budget Git-Driven Search

**Source:** `autoresearch/program.md`, `autoresearch/train.py`, `autoresearch/prepare.py`

The autoresearch system is the most disciplined self-improvement loop in the study. It is a pure research automation loop where the LLM edits `train.py` (the only mutable file) to minimize `val_bpb` (bits per byte) on a fixed evaluation harness.

**The loop (from `program.md`):**

```
LOOP FOREVER:
  1. Look at git state (current branch/commit)
  2. Tune train.py with an experimental idea
  3. git commit
  4. Run: uv run train.py > run.log 2>&1
  5. grep "^val_bpb:" run.log
  6. If grep empty → crash → read tail -n 50 run.log → fix or skip
  7. Record in results.tsv (untracked): commit, val_bpb, memory_gb, status, description
  8. If val_bpb improved → KEEP (advance branch)
  9. If val_bpb worse → git reset back to start
```

**Key design decisions:**
- **Fixed time budget:** 5 minutes wall-clock training time. This eliminates the need to reason about training efficiency — always the same budget.
- **Git as rollback mechanism:** `git reset --hard` to the last known-good commit on regression. Git commit hash is the experiment identifier.
- **`prepare.py` is immutable:** Fixed constants (MAX_SEQ_LEN=2048, TIME_BUDGET=300s, EVAL_TOKENS), fixed tokenizer, fixed dataloader, fixed `evaluate_bpb()` metric. The LLM cannot touch the evaluation harness.
- **Simplicity criterion:** Explicitly weighted. "Removing something and getting equal or better results is a great outcome — that's a simplification win."
- **Never stop:** "Do NOT pause to ask the human if you should continue." The loop runs indefinitely until manually interrupted.
- **Crash protocol:** Dumb/easy fix (typo, missing import) → fix and retry. Fundamentally broken idea → log "crash" and move on.
- **results.tsv is untracked:** Not committed to git. Stays as a local log.

**What the LLM modifies in `train.py`:** model architecture (depth, aspect ratio, window pattern), optimizer hyperparameters (LR, betas, weight decay), training schedule (warmup/warmdown ratios, final LR fraction), batch size, Muon vs AdamW configuration. All 630 lines of `train.py` are fair game.

**What the LLM cannot modify:** `prepare.py`, the evaluation harness, installed packages, the time budget.

**Evaluation:** This is the gold standard for disciplined self-improvement loops. The fixed budget eliminates iteration-time reasoning overhead. Git-based rollback is trivial and reliable. The simplicity criterion prevents feature accumulation. The untracked results.tsv provides a permanent record outside the git history. The "NEVER STOP" directive ensures continuous experimentation.

### 2.2.1 Karpathy's "AI Psychosis" and the Loopy Era

**Source:** No Priors podcast (March 2026), Karpathy's loopy era talks

Karpathy describes a fundamental shift in his workflow since December 2025:

> "I don't think I've typed a line of code probably since December... this is an extremely large change... I kind of feel like I was in this perpetual, I still am often in this state of AI psychosis."

**The transition:**
- **Before:** 80% code written by hand, 20% delegation to agents
- **After:** Effectively 0% hand-coding — entirely agent-delegated
- **Reason:** The unlock in coding agents around December 2025 was dramatic and sudden

**"Claws" — Persistent Agent Entities:**

Karpathy uses the term "claw" for persistent, semi-autonomous agents that run continuously:

> "The LLM sort of part is now taken for granted. The agent part is now taken for granted. Now the claw-like entities are taken for granted and now you can have multiple of them."

Example: "Dobby the Elf" — a home automation agent controlled via WhatsApp:
- Scans LAN for smart devices
- Reverses APIs for Sonos/lights
- Builds WhatsApp control portal
- Integrates Quinn vision model alerts
- Controls everything in natural language

**The leverage equation:**
- **Human input:** Small number of tokens, arranged once
- **Agent output:** Massive amounts of work done on behalf of human
- **The metric:** Token throughput per human involvement ratio

**Program.md as organizational spec:**

The `program.md` concept extends beyond research to organizational description:

> "Focus on designing metrics, automation loops, and `program.md`-style specifications for orgs/agents."

A `program.md` is an executable specification of how an organization or process should run — written in markdown, interpreted by agents, optimized by the loop.

**Implications for ACE's self-improvement:**
1. **Maximum leverage = minimum human in loop** — design for autonomous operation
2. **Claws as persistent agents** — ACE should support long-running persistent agents, not just one-shot invocations
3. **The human as metric designer** — not code writer; humans own objectives, agents own execution
4. **Program.md as loop protocol** — ACE should implement executable organizational specs that agents interpret

### 2.3 OpenClaw — Heartbeat Rules and Scheduled Tasks

**Source:** `openclaw/docs/start/openclaw.md`, `openclaw/docs/gateway/heartbeat`, `openclaw/docs/reference/templates/HEARTBEAT.md`, `openclaw/docs/reference/templates/AGENTS.md`

OpenClaw's self-improvement is based on scheduled wake-ups and systematic task tracking rather than automatic code modification.

**Heartbeat mechanism:**
- Default interval: 30 minutes (`agents.defaults.heartbeat.every: "30m"`)
- Configurable per-agent: `agents.list[].heartbeat`
- Trigger file: `HEARTBEAT.md` in the workspace root. If the file exists but contains only blank lines and markdown headers, the heartbeat is skipped (saves API calls). If the file is missing, the heartbeat still fires.
- Default heartbeat prompt: "Read HEARTBEAT.md if it exists. Follow it strictly. Do not infer or repeat old tasks from prior chats. If nothing needs attention, reply HEARTBEAT_OK."
- On `HEARTBEAT_OK` response: outbound delivery is suppressed.
- Skipped heartbeat reasons logged: `quiet-hours`, `empty-heartbeat-file`, `no-tasks-due`, `alerts-disabled`, `requests-in-flight`.

**HEARTBEAT.md task mode:** The heartbeat file can contain a checklist with intervals. Due timestamps are only advanced after a real heartbeat run. Task mode is active when the heartbeat file has structured tasks with schedules.

**Cron scheduling:** OpenClaw has a `cron` tool for precise scheduling. Cron jobs are named, deduplicated, and tracked. Each cron job contributes a directory with run logs, delivery status, and marker tracking. Deduplication ensures natural timer fires do not produce duplicate deliveries.

**State tracking:** `memory/heartbeat-state.json` tracks which checks have been performed and their intervals. The agent is instructed to batch similar periodic checks into `HEARTBEAT.md` rather than creating multiple cron jobs.

**Evaluation:** OpenClaw's self-improvement is procedural and human-in-the-loop rather than automatic. The agent maintains checklists in `HEARTBEAT.md` and schedules cron jobs, but the improvement comes from systematic execution of known tasks rather than discovery of new approaches. No code is modified; the system improves task completion reliability, not capability ceilings.

### 2.4 Devin — Playbook System and Knowledge Management

**Source:** Devin product docs (docs.devin.ai), Cognition blog posts

Devin's self-improvement operates at two layers: session-level learning and organizational-level knowledge management.

**Playbooks:**
A playbook is a reusable document that functions as a custom system prompt for a repeated task. Unlike a static prompt, a playbook is iteratively refined based on real session outcomes. A well-written playbook includes:
- The outcome to achieve
- The steps required to get there
- Postconditions (specifications)
- Advice to correct Devin's priors
- Forbidden actions
- Required input or context from the initiator

Playbooks can be assigned a macro (e.g., `!data-tutorial`) for quick invocation. Enterprise playbooks are shared across all organizations in an enterprise.

**Knowledge base:**
Knowledge is a collection of tips, documentation, and instructions that Devin "knows" across all future sessions. Devin automatically recalls relevant Knowledge and can also auto-suggest new Knowledge entries from sessions. Knowledge deduplication and conflict resolution are supported.

**Session outcome analysis:**
Devin can analyze why a session succeeded or failed, identify patterns, and extract learnings. This analysis feeds back into playbook refinement and knowledge base updates.

**Automation with playbooks:**
- Playbooks can be attached to trigger-based workflows (e.g., when a `Bug` label is added to a Jira ticket, a `!triage-bug` playbook fires automatically)
- Multiple Devins can run in parallel on the same problem, each using a different playbook or skill
- MCP server exposes programmatic playbook and knowledge management

**Snapshot system:**
Machine state snapshots save which repos are cloned and environment is set up, allowing future sessions to resume from a pre-configured state rather than starting from scratch each time.

**Evaluation:** Devin's self-improvement is human-assisted rather than fully automated. The system excels at capturing and replicating successful session patterns via playbooks and maintaining organizational knowledge. The key gap is automatic code/harness improvement — playbooks encode procedures but do not modify the underlying system. The automation macros enable trigger-based workflows but require upfront playbook authoring.

### 2.5 RISE / Recursive Language Models — Self-Improving via Recursive MDP

**Source:** `research/papers/rlm-rise.html` (full blog post + paper)

RISE (Recursive IntroSpEction) formulates self-improvement as a multi-turn Markov Decision Process (MDP) where a root LM recursively calls sub-LMs to decompose and process long context, with online imitation learning and reward-weighted supervised learning.

**Core mechanism:**
- The root LM receives only the query (not the full context) and access to a Python REPL environment that stores the context as a variable.
- The root LM can spawn recursive sub-LM calls (depth=1 in the paper) inside the REPL to peek, grep, partition+map, or summarize chunks of the context.
- Final answer is returned via `FINAL(answer)` or `FINAL_VAR(var_name)`.
- The trajectory of recursive calls — how the LM chose to decompose the context — is itself a learning signal.

**Self-improvement formulation:**
- RLMs trained explicitly to recursively reason represent the next milestone in inference-time scaling (after CoT and ReAct).
- The trajectory of recursive interactions is interpretable and optimizable as a scalar reward.
- Recursive depth of 1 is sufficient for most current long-context benchmarks; deeper recursion is a natural extension.
- The framework is hardware/optimizer-agnostic — any LM can be wrapped as an RLM.

**Key results:**
- RLM(GPT-5-mini) outperforms GPT-5 on OOLONG 132k-token split: >33% raw score improvement, >double the correct answers.
- RLM(GPT-5-mini) is cheaper per query than GPT-5 (median query cost).
- On BrowseComp-Plus (1000 documents, 10M+ tokens), RLM(GPT-5) maintains perfect accuracy while all other methods degrade.
- No context rot observed at 10M+ tokens.

**Relationship to self-improvement:**
RISE is not a self-improving agent system per se — it is an inference strategy for LMs. However, the recursive decomposition trajectory is itself a learnable artifact. Future work: train models to produce better decomposition strategies, analogous to how reasoning models are trained on CoT traces.

**Evaluation:** RISE demonstrates that self-improvement can happen at the inference-strategy layer, not just the weight or prompt layer. The ability to recursively self-query and accumulate sub-answers is a form of meta-learning that avoids context-length limitations entirely.

### 2.6 AlphaEvolve — Population-Based Evolutionary Coding Agent

**Source:** `research/papers/alphaevolve.pdf`, web search (arxiv 2506.13131)

AlphaEvolve is Google's evolutionary coding agent that uses a population of LLM-generated algorithm variants, evaluated by automated evaluators, evolving toward better solutions over time.

**Architecture:**
- **Population database:** MAP-elites-inspired algorithm + island-based population model. Stores algorithm variants with their scores.
- **Prompt sampler:** Assembles prompts for LLM generation from the population database.
- **LLM pipeline:** Multiple LLMs (Gemini) generate, critique, and evolve algorithm candidates.
- **Evaluator:** Automated evaluation metrics that provide objective, quantifiable assessment of accuracy and quality. Ground truth via code execution.
- **Evolutionary selection:** Which programs survive to future prompts is determined by the evolutionary algorithm.

**Key results:**
- Improved Strassen's 4×4 complex matrix multiplication algorithm: 48 scalar multiplications (first improvement in 56 years).
- Data center scheduling algorithm optimization.
- Hardware accelerator circuit simplification.
- Training acceleration for AlphaEvolve's own LLM.
- 14 different algorithmic targets improved.
- 75% of 50+ math/combinatorics problems rediscovered state-of-the-art.

**What is being optimized:** Pure algorithm code (Python functions with specific signatures). The evaluator is a fixed oracle (ground truth + execution). No harness modification.

**Evaluation:** AlphaEvolve demonstrates that population-based evolutionary search over LLM-generated candidates, grounded by automated evaluators, can produce genuine novel discoveries. The key enabler is the evaluator — without a reliable automatic scoring function, evolution cannot proceed. For ACE, this suggests that trajectory-to-skill capture needs an automatic quality evaluator, not just human-assisted skill authoring.

### 2.7 Meta-Harness — Outer-Loop Harness Optimization via Filesystem Traces

**Source:** `research/papers/meta-harness.pdf`, web search (arxiv 2603.28052v1)

Meta-Harness is the most directly relevant system for ACE's learning loop because it optimizes the *harness* (the code around a fixed model that determines what to store, retrieve, and present), not the model weights.

**Key insight:** Existing text optimizers (OPRO, TextGrad, AlphaEvolve) compress feedback too aggressively into short summaries or scalar scores. Harness optimization requires selective inspection of prior code and execution traces.

**Architecture:**
- **Proposer:** A coding agent (Claude Code with Opus 4.6). Not a next-token model — it decides which prior artifacts to inspect, diagnoses failure modes from execution traces, and makes targeted code edits.
- **Filesystem D:** A growing directory containing the full history of all prior candidate harnesses. Each candidate contributes: source code, evaluation scores, full execution traces (prompts, tool calls, model outputs, state updates).
- **Search loop:** Propose → Evaluate → Log → Repeat. ~60 harness evaluations over 20 iterations (2 candidates/iteration).

**The critical ablation:** Access to raw execution traces is the key ingredient.

| Feedback Type | TerminalBench-2 Accuracy |
|---|---|
| Scores only | 41.3 |
| Scores + summary | 38.7 |
| Full filesystem (scores + traces) | **56.7** |

This 15-point gap demonstrates that trace-level diagnosis enables qualitatively different repair strategies compared to scalar feedback.

**Filesystem access pattern:**
- Proposer uses `grep` and `cat` to query the filesystem, not a single ingested prompt.
- The filesystem is typically far larger than the proposer's context window.
- Median 82 files read per iteration.
- Harness format: single-file Python program (prompting + retrieval + memory + orchestration).

**Results:**
- Online text classification: +7.7 points over state-of-the-art context management, 4x fewer context tokens.
- Math reasoning (200 IMO-level problems): +4.7 points average across 5 held-out models.
- TerminalBench-2: 34.6 → 50.0 (discovered harness surpasses best hand-engineered baseline).

**What is being optimized:** The harness code itself — the scaffolding that wraps the LM. This includes prompt templates, retrieval logic, memory management, and orchestration. It is the most direct analog to ACE's "learning loop" concern.

**Evaluation:** Meta-Harness is the proof-of-concept that outer-loop harness optimization via filesystem-trace access is both feasible and high-impact. For ACE, the key takeaway is: raw execution traces + filesystem interface + coding-agent proposer > scalar score feedback. This directly informs ACE's learning loop design.

---

## 3. Cross-Cutting Comparison

### 3.1 Feedback Loop Patterns

| System | Loop Type | What is Optimized | Feedback Signal |
|---|---|---|---|
| Hermes Agent | Trajectory capture → manual skill authoring | Agent procedures (SKILL.md) | Human-assisted; agent reviews failed trajectories |
| autoresearch | Outer-loop experiment (fixed 5-min budget) | Model architecture + hyperparameters (train.py) | val_bpb (bits per byte) |
| OpenClaw | Scheduled heartbeat + cron | Task execution reliability | HEARTBEAT_OK / task completion |
| Devin | Playbook authoring + session analysis | Organizational procedures (playbooks) | Human-assisted; session outcome analysis |
| RISE | Recursive multi-turn MDP | Inference strategy (how to decompose context) | Final answer correctness + sub-call traces |
| AlphaEvolve | Population evolution (MAP-elites + island model) | Algorithm code | Automated evaluator (ground truth + execution) |
| Meta-Harness | Outer-loop harness search (coding agent) | Harness scaffolding code | Full execution traces + filesystem access |

**Convergent pattern:** All systems separate the evaluator (fixed) from the proposer (flexible). The evaluator provides the ground truth; the proposer generates candidates. None of the systems try to learn from a single scalar signal in isolation — all preserve some form of trace or trajectory.

**Divergent patterns:**
- Hermes Agent and Devin rely on human-assisted synthesis (trajectories → skills, sessions → playbooks).
- autoresearch, AlphaEvolve, and Meta-Harness are fully automated.
- OpenClaw is procedural (scheduled checklist execution), not discovery-oriented.

### 3.2 Configuration Evolution

| System | What Changes | How | Rollback |
|---|---|---|---|
| Hermes Agent | SKILL.md files (procedural knowledge) | `skill_manage()` tool with create/edit/patch/delete | Atomic write + security scan rollback |
| autoresearch | train.py (model code) | Direct file edit by LLM | `git reset --hard` to last good commit |
| OpenClaw | HEARTBEAT.md (task checklists) | Agent edits workspace file | Manual (file-based) |
| Devin | Playbooks + Knowledge entries | Human-assisted authoring + auto-suggest | Playbook version history |
| RISE | Inference strategy (RLM wrapper) | Training-time RL on recursive call patterns | N/A (inference-time) |
| AlphaEvolve | Algorithm code (Python functions) | LLM generate + evolutionary selection | Population keeps prior candidates |
| Meta-Harness | Harness code (single-file Python) | Coding agent proposer edits | Pareto frontier selection over candidates |

### 3.3 Harness Optimization

Harness optimization — modifying the scaffolding around a fixed model — is the central theme of Meta-Harness and the most relevant pattern for ACE.

**Key findings from Meta-Harness:**
1. Raw execution traces are critical: 15-point accuracy gap between scores-only and full-trace feedback.
2. The coding-agent proposer (not raw next-token model) can selectively diagnose failure modes and propose targeted edits.
3. Filesystem interface enables inspection at scale (median 82 files/iteration, far exceeding context window).
4. ~60 evaluations over 20 iterations is sufficient for meaningful harness discovery.

**Implication for ACE:** The harness is the code that decides what to store, retrieve, and present to the model. In ACE terms, this includes: prompt templates, tool routing logic, memory tier decisions, compaction triggers, and delegation policies. Meta-Harness demonstrates that automated harness discovery is feasible and high-impact.

### 3.4 Experiment Budgets

| System | Budget | Discipline |
|---|---|---|
| autoresearch | 5 minutes wall-clock (fixed) | LLM cannot modify budget or harness |
| AlphaEvolve | Not fixed; governed by evaluator convergence + population diversity | Population-based termination |
| Meta-Harness | ~60 evaluations / 20 iterations | Pareto frontier over accuracy + context cost |

**autoresearch insight:** Fixed time budget eliminates iteration-time reasoning. The LLM does not need to estimate "is this change worth testing?" — everything gets exactly 5 minutes. This is the most copyable pattern for ACE's learning loop.

### 3.5 Git-Based Rollback

**autoresearch** is the canonical reference: `git commit` on each experiment, `git reset --hard` on regression. The git commit hash is the experiment identifier. This is the simplest possible rollback mechanism and requires zero infrastructure beyond git.

**Hermes Agent** uses atomic file writes with rollback on security scan failure — not git-based, but still reliable. The skill edit operation backs up original content before writing and restores on scan block.

**Devin** uses machine snapshots (saved workspace state) for rollback at the environment level.

**Recommendation for ACE:** Git-based rollback for code/harness experiments (learned artifacts versioned as files). Atomic writes with rollback for runtime state modifications (skill files, config changes).

### 3.6 Population-Based Search

**AlphaEvolve:** MAP-elites + island-based population. Multiple LLM instances generate candidates; evolutionary algorithm selects which survive to future rounds. Preserves diversity — the population is not a single best solution but a distribution of approaches.

**Meta-Harness:** Pareto frontier selection over accuracy × context cost. Multiple candidates evaluated per iteration; Pareto-optimal set is preserved.

**Implication for ACE:** Single-candidate optimization (try one change, keep or discard) is fragile. Population-based approaches preserve multiple viable approaches and enable crossover (combining features from different candidates). This is especially valuable when the evaluation metric is noisy or multi-dimensional.

### 3.7 RL Fine-Tuning

**RISE:** Formulates the recursive decomposition as a learnable RL problem. The trajectory of recursive interactions (how the root LM chose to partition context) can be treated as a policy to be optimized. This is inference-time RL, not training-time RL — the model learns to decompose better at inference time via recursive self-query.

**AlphaEvolve:** Training acceleration via evolved algorithms. The discovered scheduling algorithm improved AlphaEvolve's own training infrastructure — a genuine self-improvement loop at the system level.

**Devin (implicit):** Session outcome analysis can inform fine-tuning signals, though Devin does not publicly disclose a fine-tuning pipeline.

**Implication for ACE:** RL fine-tuning is viable at two layers: (1) inference-time strategy (how the agent decomposes problems, allocates context, routes between tools), and (2) system-level optimization (the agent improving the infrastructure it runs on). RISE demonstrates layer (1) is achievable now. Layer (2) requires AlphaEvolve-scale infrastructure.

### 3.8 Meta-Learning Stacks

A meta-learning stack is a system that learns how to improve its own learning process. The systems studied here vary in stack depth:

| System | Meta-Learning Depth |
|---|---|
| autoresearch | Shallow: LLM proposes → evaluates → git rollback. No meta-level learning about *how* to propose. |
| Hermes Agent | Medium: Trajectory capture → skill authoring. But synthesis is manual, not automated. |
| OpenClaw | Shallow: Scheduled tasks execute reliably, but no discovery of new procedures. |
| Devin | Medium-deep: Playbooks + Knowledge + session analysis + MCP automation. Human-assisted meta-learning. |
| RISE | Deep: Recursive self-query at the inference level. The model learns to decompose problems recursively. |
| AlphaEvolve | Deep: Population evolution of algorithm code. The system discovers novel algorithms, including improving its own training. |
| Meta-Harness | Deep: Outer-loop harness optimization via coding agent + filesystem traces. The harness learns to be a better scaffold. |

---

## 4. ACE Recommendation Table

| Pattern | System | Adopt / Avoid / Adapt | Rationale |
|---|---|---|---|
| **Fixed time budget for experiments** | autoresearch | **Adopt** | Eliminates iteration-time reasoning overhead. LLM knows exactly how long each experiment runs. Trivially implementable. |
| **Git-based rollback for experiments** | autoresearch | **Adopt** | Zero infrastructure. Commit hash = experiment ID. `git reset` = rollback. Already part of any code-oriented workflow. |
| **Trajectory capture → skill authoring** | Hermes Agent | **Adopt** | Passive capture is low-cost. Human-assisted synthesis is the current best practice for procedural knowledge. Automatic synthesis (trajectory → SKILL.md) is future work. |
| **Atomic writes with rollback on failure** | Hermes Agent | **Adopt** | Essential for any system that modifies its own code/state. Security scan → rollback chain prevents corrupted skills from persisting. |
| **Raw execution trace storage** | Meta-Harness | **Adopt** | 15-point accuracy gap vs scalar feedback. For ACE, storing raw traces of agent sessions enables harness-level diagnosis. Do not compress traces to summaries during the learning loop. |
| **Filesystem interface for feedback** | Meta-Harness | **Adopt** | Allows the proposer to selectively inspect relevant history without context window overflow. Coding agent uses `grep`/`cat` — same tools it already knows. |
| **Coding-agent proposer (not raw LM)** | Meta-Harness | **Adopt** | A coding agent decides *what* to inspect, diagnoses failure modes, and proposes targeted edits. Raw next-token models cannot selectively navigate large histories. |
| **Pareto frontier selection** | Meta-Harness, AlphaEvolve | **Adopt** | Multi-objective optimization (accuracy + cost + latency) is essential for production systems. Single-scalar optimization ignores tradeoffs. |
| **Population-based search (MAP-elites)** | AlphaEvolve | **Adapt** | Preserves diversity of approaches. ACE should maintain a population of harness variants, not single-candidate optimization. Lower overhead than AlphaEvolve's full LLM pipeline. |
| **HEARTBEAT.md task checklists** | OpenClaw | **Adapt** | Systematic task tracking via heartbeat is valuable for long-running agents. ACE should implement heartbeat-driven checklist execution for deferred tasks. |
| **Playbook system for recurring tasks** | Devin | **Adapt** | Captures procedures as versioned documents. ACE should implement playbook-style skill documentation with version history and trigger conditions. |
| **Knowledge base with auto-suggest** | Devin | **Adapt** | Organizational knowledge that persists across sessions. ACE should implement a session-transcending knowledge layer with auto-suggest from successful sessions. |
| **Never-stop experiment loop** | autoresearch | **Adopt** | Autonomous experimentation without human check-ins. ACE's learning loop should run continuously in the background, surfacing findings rather than asking for permission. |
| **RLM recursive self-query (inference-time)** | RISE | **Monitor** | Promising for long-context decomposition. Not ready for immediate adoption — requires training-time RL on recursive call patterns. Monitor for model support. |
| **Simplicity criterion in experiments** | autoresearch | **Adopt** | "Removing something and getting equal or better results is a simplification win." Prevents feature accumulation in the harness. ACE should track simplification experiments equally. |
| **Skill quality gate (security scan)** | Hermes Agent | **Adopt** | Agent-edited code poses malware risk. All agent-created skills must pass security scan before activation. ACE should implement a skill quality gate. |
| **HEARTBEAT.md skip on empty** | OpenClaw | **Adopt** | Empty heartbeat files (only headers) should skip API calls. Reduces wasted inference. ACE should implement heartbeat skip logic. |
| **results.tsv untracked by git** | autoresearch | **Adopt** | Local log outside experiment history. ACE should maintain a permanent results log separate from versioned artifacts. |
| **Human-assisted playbook synthesis** | Devin | **Avoid** | Time-intensive and not scalable. ACE should aim for automated playbook generation from trajectory analysis, not human authoring. |
| **Single-candidate optimization (keep/discard)** | Most systems | **Avoid** | Fragile when evaluation is noisy. Population-based approaches are more robust. ACE should maintain multiple candidates. |
| **Compressed scalar feedback** | OPRO, TextGrad | **Avoid** | Meta-Harness demonstrates 15-point accuracy loss from compression. Raw traces + filesystem interface is strictly superior for harness optimization. |

---

## 5. Key Findings for ACE

### 5.1 The Harness is the Frontier

Meta-Harness proves that the performance gap between hand-engineered and automatically discovered harnesses is enormous (TerminalBench-2: 34.6 → 50.0). The harness — the code that decides what to store, retrieve, and present to the model — is a higher-leverage optimization target than model weights for a fixed base model. ACE should invest in outer-loop harness optimization as a first-class capability.

### 5.2 Raw Traces > Scalar Scores

The single most important technical finding from this slice: **raw execution traces are the critical feedback medium for harness optimization.** Scores alone achieve 41.3 on TerminalBench-2; full traces with filesystem access achieves 56.7. ACE's learning loop must store raw traces, not summaries, and expose them via a filesystem interface.

### 5.3 Fixed Budget Eliminates Overhead

autoresearch's 5-minute fixed budget is the most copyable pattern for ACE. The LLM does not reason about whether an experiment is worth running — it knows exactly the budget and iterates within it. This is the right abstraction for ACE's learning loop: fixed time, maximize quality of discovered improvements.

### 5.4 Population > Single Candidate

Single-candidate optimization (try one change, keep or discard) is the dominant pattern in simpler systems (Hermes Agent skills, early Devin playbooks). Population-based approaches (AlphaEvolve, Meta-Harness) are strictly superior for harness optimization. ACE should maintain a Pareto frontier of harness variants.

### 5.5 Git for Artifact Versioning

autoresearch's git-based experiment tracking is zero-overhead and battle-tested. Commit hash = experiment ID. `git tag` or branch naming = experiment family. `git reset` = rollback. ACE should version all learned artifacts (harness variants, skill files, playbook versions) in git.

### 5.6 Security Gates for Self-Modification

Hermes Agent's security scan → atomic rollback chain is essential infrastructure for any system that modifies its own code. Without it, a corrupted or malicious skill can persist and be reloaded. ACE must implement a security gate before any agent-edited artifact is activated.

### 5.7 Trajectory Capture is Necessary but Not Sufficient

Hermes Agent captures trajectories but does not synthesize them automatically. The gap between "captured trajectory" and "learned skill" requires a synthesis step that is currently human-assisted. ACE should aim to close this loop: trajectory capture → automated quality evaluation → skill synthesis.

### 5.8 Scheduled Wake-Up Enables Long-Horizon Task

OpenClaw's heartbeat mechanism allows agents to make progress on deferred tasks without continuous human engagement. For ACE, this is essential for background learning loops — the agent wakes up, executes a heartbeat checklist, and returns to sleep. This pattern should be adapted, not the OpenClaw-specific heartbeat prompt.

---

## 6. Research Gaps

1. **Automated trajectory → skill synthesis:** No system in the study automatically synthesizes a ShareGPT trajectory into a SKILL.md. This is the critical missing piece for a fully automated learning loop.

2. **Multi-objective harness optimization in production:** Meta-Harness demonstrates Pareto frontier selection in research. It is not yet clear whether this is feasible in a production system with latency constraints.

3. **Self-modifying safety at scale:** Hermes Agent's security scan is for skill files. A system that modifies its own core harness code (not just skills) requires a deeper safety architecture.

4. **RLM training for recursive decomposition:** RISE demonstrates the inference-time strategy but does not yet publish trained models. The training-time RL for recursive decomposition is future work.

5. **Population diversity maintenance cost:** AlphaEvolve's MAP-elites population requires significant compute. The minimum viable population size for ACE's learning loop is an open question.

---

## 7. Conclusion

Self-improvement in agentic systems ranges from passive trajectory capture to full outer-loop evolutionary optimization. The most impactful mechanisms for ACE's learning loop are:

1. **Fixed time budget** (autoresearch) — eliminates iteration-time reasoning overhead
2. **Git-based experiment tracking** (autoresearch) — zero-overhead versioning and rollback
3. **Raw trace storage + filesystem interface** (Meta-Harness) — the critical feedback medium, 15 points better than scalar scores
4. **Coding-agent proposer** (Meta-Harness) — selective diagnosis and targeted editing
5. **Population-based search** (AlphaEvolve, Meta-Harness) — Pareto frontier over accuracy × cost
6. **Security-gated atomic writes** (Hermes Agent) — safe self-modification infrastructure

The pattern to avoid is compressed scalar feedback: any learning loop that reduces execution history to a single score will underperform one that exposes full traces via a filesystem interface.

**Slice Completed:** 10 / 14  
**Files Affected:** `design/units/agents-study/study/self-improvement.md`
