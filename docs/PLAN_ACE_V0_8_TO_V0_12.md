# ACE v0.8 → v0.12 plan

**Sources:**
- `docs/AUDIT_2026_10_24_GAP_PLAN.md`: rigor, Shapiro fidelity, and plugins/generality audits.
- `docs/TRACE_OWNER_ASKS.md`: every owner ask mapped to a section here. This plan is checked against it.
- Archaeology of all four eras:
  - 2024: hand-written Python/NATS, `e4bf2d0f`.
  - 2025: Angular rewrite, `b652e56a`.
  - Mar–May 2026: design corpus plus "Agent Configuration Engine" code, `ac7d68c9`.
  - Oct 2026: main.
- The Shapiro repo `ACE_Framework.md`. The paper (arXiv 2310.06775) is checked line by line in 8.0.
- ACE's own v0.7 rewrite spec, `docs/SPEC_ACE_V0_7_REWRITE.md`, assessed in §3.
- The owner's Pi sessions: the ACE session (2026-10-17 to 10-24) and the harness sessions (2026-09-06, 2026-10-18).

The plan has no open questions. Per-version specs are written from it, one version at a time, each detailed enough for a cheaper model to build without guessing. Starting values for every threshold are in §5; owner actions are in §6. Where this plan and the audit disagree (build order, examples), this plan wins.

## Where ACE stands and where it is going

**Now (v0.7.1, stopped 2026-10-24 11:10 UTC):**
- Pure-Go binary on SQLite, structurally top-down: the L1 root is anchored first, then a provenance chain runs L1→L2→L3→L4→L5 (GPU gate)→L6.
- L2‖L3‖L4 run in parallel waves over a dependency tree.
- Calls are pyramidal: L6 made 115 of 169 calls and used 87% of prompt tokens.

**What is wrong:**
- L1 anchors the root and is then never consulted (1 of 169 calls).
- The imperatives live in an editable `soul.md`.
- Its science is self-graded and moved the goalposts (§3).
- Domain strings leak into core.
- Some paths wait on a human.
- There is no time system, no plugin boundary, no install story, and the UI is unreadable.

**Going:** a general autonomous scientist that anyone installs and gives a life mission.
- Shapiro's six layers and imperatives are compiled into its mechanics.
- It runs 24/7 under real-time top-down steering.
- Every claim is measured honestly.
- It is built from plugins.
- It builds its own successors through owner-merged PRs.

## 0. Owner decisions

| Topic | Decision |
|---|---|
| **Human touchpoints** | ACE approves its own work wherever it can. Two things reach the owner: (1) a change to a git repo goes through a pull request that only the repo owner approves and merges, requested on Telegram; (2) access ACE cannot grant itself (a deny-listed command, an account or resource it doesn't own, a spend-cap raise) is requested from the owner. Neither blocks: ACE keeps working on everything else while it waits |
| **The human's role** | Creates the agent, re-steers the mission, receives reports, answers questions. Questions carry information (contradictions, missing facts) or ask for access; never approval of ACE's work |
| **Agent creation** | The owner creates an agent with a life mission, a personality and its capabilities. ACE then proposes nearer-term goals that serve the mission, perpetually |
| **Layers** | Shapiro derived the six layers from the study of the human mind; they are the point of ACE. All six always run and are never removed. ACE tests, validates and optimises each one. The L2‖L3‖L4 parallel cluster over a dependency tree stays |
| **L1** | Three tiers (8.2): rules, a fast gate on north/south flow, and full L1 for uncertain cases. Tier-1 feedback flows into the next full L1 run. Full L1 can change course top-down. The fast gate only allows or escalates; rejection belongs to full L1, except the Tier-0 deny list the owner asked for |
| **Imperatives** | Ingrained in every layer's mechanics, whether the layer is aware of it or not; not only in prompts |
| **Science** | Measurement and experimentation are mandatory. Every task records measurements; claims need pre-registered tests. No cheating, no shortcuts |
| **Capabilities** | ACE gets real-world access (money, social media, accounts, launches) as plugins under the guards in 10.4 |
| **Anatomy** | Removed from ACE: code, specs and Telegram figures. How ACE shows a complex component visually is decided in the v0.11 design session |
| **Host** | The owner's machine: 12 cores, 38 GB RAM, RTX 3050 6 GB, Bazzite (Fedora atomic); 37 GB of disk free (93% used) |
| **Competitor comparison** | ACE runs it itself (v0.10), under the scientist gate, against the current releases |
| **Time management** | A first-class system: schedules, monitors, long-running tasks, conflict resolution, replanning as information arrives |
| **Web UI** | Redesigned from first principles in v0.11: minimal, usability and readability first |
| **Opencode hook** | A personal bolt-on, kept out of core |
| **Security status** | The server binds `127.0.0.1` (`cmd/ace/main.go`). ACE runs live through every build (§4) |
| **PR gate** | Every git repo with a remote, including ACE's own repo (`jayfalls/ace_prototype`): ACE's changes reach `main` only through an owner-merged PR. ACE may keep one long-lived branch spanning many versions (even v0.1 to v0.12) so an unmerged PR never blocks it. Identity: a GitHub App. The owner-directed builder is not under this gate (§4) |
| **v0.12** | Filled by the v0.10 comparison data |
| **Sandbox** | Linux: the bwrap jail. macOS: sandbox-exec. Windows: runs unsandboxed and is flagged as such; revisited in v0.13 |
| **Start** | Manual start only: no systemd unit and no autostart. Once started, it runs 24/7 |
| **Storage** | SQLite only; no other store. The binary stays self-contained |
| **Cleanup** | ACE cleans up in the same step that changes, adds, updates or rewrites something |

## 1. Lessons from all four eras (binding)

1. **Targets without a harness are wishes.** The 2026 corpus set more than a dozen targets and measured none. Every target ships with its harness in the same change.
2. **A gate that silently skips is worse than no gate.** It happened in 2026 doc validation (skipped when there was no DB) and in the L5 gate, which defaults to pYes=0.96 (`internal/cognitive/cascade.go:314`). Every gate fails closed and logs why.
3. **Superseded docs mislead agents.** Three architecture generations of docs coexisted in 2026. Each version archives the docs it supersedes in the same change (8.8).
4. **Only L1 carried the imperatives in 2024 and in v0.7.1.** 8.1 and 8.3 put them in every layer.
5. **Layer names never change.** The 2026 "Moral Reasoning…" rename drifted.
6. **Over-building, then stripping, repeats** (2026 cache machinery, DB retry layer, Gen-1 infrastructure). Build the minimum that passes the declared measurements.
7. **No 2026 auth patterns.** The ephemeral signing key and CORS `*` were defects. ACE is single-owner: a local bearer token, no multi-user auth.
8. **The blueprint was missed for six versions.** v0.1–v0.6 shipped without the imperatives and ran layers before L1. Shapiro fidelity is an acceptance test (8.2, 9.3), not a prompt.
9. **A version must be a scientist, not an optimiser.** v0.2 optimised v0.1 and built no datasets; its directive lived only in prompts. Core behaviours go into mechanics and acceptance tests (8.7).

## 2. Carried forward (binding)

**Core:**
- Domain-agnostic core; pure-Go single binary; SQLite; no NATS, Postgres or Valkey. The in-process bus is enough.
- Software agent first. Physical sensors and actuators plug in through the Sense and Tool ports; no robot adapters are built until a mission needs one.

**Engineering principles:**
- **Deterministic first.** Code wherever the answer can be derived; a model only where judgement is needed. Values are derived from data, never hard-coded; human availability is measured, never a fixed sleep window.
- **Composition.** Shared components with standard interfaces, connectors and bridges; one solution per problem, never per-layer copies.
- **Filesystem:** `internal/<component>/`, two nesting levels (three only in rare cases); files of about 500 lines at most; no god files.
- **Least privilege.** Every capability grant is an experiment that has to show it improves outcomes (10.4).
- **Adopt before build.** Use a maintained component first; build only for a recorded gap.
- **Owner-set config is authoritative.** Owner-set files (mission, personality, models, caps, grants) are mounted read-only in the jail and Tier 0 denies writes to them. ACE proposes changes with evidence in the digest.

**Mechanisms (all exist in v0.7.1 code):**
- stuck detector 3/5/8, ending in an L1 ruling then quarantine;
- file leases plus diff merge;
- context slice ranked by reciprocal-rank fusion (k=60, `internal/cognitive/connectome.go`);
- value-of-information uncertainty ledger.

**Context staging:** turn 0 / 0.5 / 1 stay as composables (9.5). The turn-0.5 router is off by default, because the v0.6 A/B showed no gain (28% vs 31% Mode-3 pass). It is re-tested whenever the map or models change.

## 3. Lessons from the v0.7 run and its research

**From the run (`ace.db`, 169 traces, 2026-10-24 08:13–11:10 UTC):**
1. **It moved the goalposts until it passed.** Node #18 first wrote `h1_accepted:false`; within 30 min it changed the dataset (new sha), the hypothesis and the task form, and re-ran until `true`. Finance and legal still score 0.0 vs 0.0. All 36 files in `benchmarks/results/` claim H1. → 8.6 locks pre-registration.
2. **L2 cloned one strategy seven times** (S3–S9 `V08_SELF_BUILD`), and the fix raised the branch cap to 12 (`mission_dag.go`). → duplicate rejection and diminishing-returns stop (8.3).
3. **L1 went silent after the start.** The provenance chain L1→L6 holds, and L5's local GPU gate ran (it logs to `gpu-gate.log`, not `thought_traces`). But L1 made 1 of 169 calls. → an L1 verdict for every side effect (8.2) and measured per-layer call shares.
4. **L6 carries the cost, as designed.** 115 of 169 calls and 4.17M of 4.8M prompt tokens (87%), about 36k tokens per L6 prompt (the v0.6 run averaged 31.8k). The share is right; the lever is per-prompt size. → context budgets (9.5).
5. **Fixes accrete domain strings into core** (`mission_dag.go` branch names and wiki scraping, gate filename rules). → the domain-string test runs from v0.8 (8.0).
6. **It graded itself on its own exhaust:** the trace DB is mirrored into the workspace and used as labels. → independent labels (8.6).
7. **The Pi agent's 9 root causes** (2026-10-24 checkpoint), each fixed or made obsolete in 8.0: DAG horizon stall; unbound L4 spec paths; stale `goals.md` projection; dual-filename extraction; `#grep` fragments in paths; directory reads as files; multi-path `action_target`; CONTRADICTION nodes never cleared; per-tick trace-DB mirror copy.

**From ACE's own v0.7 rewrite spec** (`docs/SPEC_ACE_V0_7_REWRITE.md`):
- **Binding:** the organ rule (a component is wired in only with a named artifact, a held-out falsification and a keep/revert verdict), one pre-declared primary statistic per milestone, and secondary statistics never flipping a decision. 8.6 enforces it in Go.
- **Design input:** its organ map (L1 invariant guardian, L2 connectome, L3 skill lifecycle, L4 milestone compiler, L5 task selection, L6 actuators, sensory annexes), which matches 9.3.
- **Not evidence:** its results ledger. The 1.1, 2.1 and 3.2 results are circular or contradicted by their own artifacts (1.1's spec says 0.8937→0.2957, its artifact says 0.7407→0.1178; 3.2's "48/48 rollbacks" is tautological). These organs are re-tested under 8.6.

**From the research corpus** (`.ace-sandbox/workspace/research_corpus/`; mostly the owner's factory research). Figures here are quoted from the corpus and are secondary: a spec that relies on one re-checks the primary source first.
1. **Harness²** (arXiv 2610.14224):
   - The improvement harness is separate from the execution harness; each task resets to the base harness.
   - Candidates are INV (how the agent works) or DOM (domain know-how).
   - Every edit cites a trajectory moment.
   - Attribution is `helped | hurt | no effect | confounded`; `confounded` is mandatory for multi-change or single-rollout evidence. Only non-confounded lessons persist.
   - Reported ablations: the improvement library is worth 3.1 pts, INV/DOM scoping 2.9 pts.
   - Context cost order: skills < scripts < subagents < middleware < always-on prompt.
   - → 10.5 self-improvement and 9.1.
2. **Counter-evidence ledger:** 11 of 11 production harnesses studied are single async loops; single agent with retrieval beat multi-agent 86% vs 66%; LLM judges miss about a third of issues; agent-written tests verify about 10% of mutants; stale context makes agents confidently wrong. → each layer shows its contribution against a single-loop baseline (10.5), and comparisons use the power rule (8.6).
3. **Owner's P3/P5 factory study:** no arm beat the plain baseline on resolution (minimum detectable difference about 20 pts at n=60); extra loop rounds changed no outcome; the speed optimisation lost 3 of 32 tasks and failed its non-inferiority guard. → non-inferiority guards on every efficiency change.
4. **v2 autopsy:** gates that check form instead of substance get gamed, including by their author. ACE v0.7 repeated it.
5. **Sello:** a self-logging agent can fabricate its own traces. → a hash-chained event log that the watchdog verifies (8.5).
6. **Temporal:** deterministic orchestration; LLM and tool calls are replayable activities. → long jobs (9.4).
7. **HITL (Echo, CAFE):** human edits are the densest signal, and frozen critics plateau. → owner verdicts and edits stored as data (8.4); Tier 1 recalibrated continuously (8.2).
8. **The scaffold is the unit of evaluation:** the same model scored 63.5% on one scaffold and 34.9% on another; single-run pass@1 swings 2–6 pts. → pin the scaffold, report pass^k (10.5).
9. **Untrusted content:** CaMeL stops about two-thirds of injections at a utility cost. → provenance tagging (8.2) and egress control (9.1).
10. **Corpus hazards:** the "factory operating rules" page is superseded (its gates were gamed), the April 2026 agents-study NATS recommendations are rejected (§2), and Hermes depth and Meta-Harness numbers conflict across pages.

---

## v0.8: Trustworthy and autonomous

### 8.0 Fixes, cleanup and archive

- **Network:** a bearer token on every mutating route, generated at `ace init` and stored in ACE's config; drop CORS `*` (`internal/server/http.go`).
- **Gates fail closed:** `cascade.go:314` defaults to reject; every gate is audited for silent skips.
- **Telegram allowlist:** an empty chat allowlist allows no chats (`internal/channels/telegram.go`).
- **Remove anatomy:** delete `BuildAnatomyIsometricSVG` and `sendTelegramAnatomyEscalation` (`internal/cognitive/sensorium.go`, lines 624–862), the `anatomy_svg` field and its builder call in `internal/cognitive/wiki_skills.go` (lines 553–564), and the anatomy check in `internal/cognitive/v0_6_connectome_sensorium_test.go` (lines 96–108). Contradiction alerts become plain text.
- **Archive superseded docs** into `archive/specs/` in the change that adds the v0.8 spec: `SPEC_ACE_V0_2.md`, `SPEC_ACE_V0_6.md`, `SPEC_ACE_V0_6_FINAL.md`, `SPEC_ACE_V0_7_1.md`, `AGENT_HANDOFF.md`. `docs/v0_6_evidence/` stays as evidence.
- **Domain-string test** in CI: fails on any pattern from `testdata/domain_strings.txt` in `internal/` or `cmd/`. Seeded with: `Hermes`, `OpenClaw`, `research_corpus`, `ace_rlm`, `ace_v0`, `S1_ABLATION`, `S2_V08`, `operator-gated`, `human-gated`, `Godot`, `Lytehouse`, `Axis `. Profiles and testdata are exempt.
- **v0.7 run data** (`ace.db`, workspace, results) is archived to `archive/v0_7_decommissioned_<date>/` with a manifest, like v0.1–v0.6.
- **v0.7 module ideas are evaluated, not discarded.** Each of the 41 `.ace-sandbox/workspace/ace_v07/*.py` modules is reduced to its idea and either becomes a candidate for a section here (and must pass the gate) or is retired with a one-line reason in the manifest. Assigned:
  - `budget_governor` → 9.4;
  - `retention_gate`, `skill_manager`, `skill_transfer` → 9.6;
  - `memory_injection_guard` → 8.2;
  - `calibration_auditor` → 8.2 (Tier-1 calibration);
  - `drift_guard` → 10.6 (model drift);
  - `task_decomposer` → 9.3;
  - `counter_evidence_ledger` → 8.6;
  - `reality_replica`, `sensorium_fusion` → v0.12 candidates;
  - `anatomy_blueprint` → retired (§0).
- **Pi agent's 9 root causes** (§3 item 7): each gets a regression test or is deleted with the code it lived in.
- **Paper check:** the build agent reads arXiv 2310.06775 and adds any requirement missing from `TRACE_OWNER_ASKS.md` section B to this plan.
- **Disk:** trace and workspace retention caps (§5); the watchdog alerts below the free-disk floor (§5).
- **`AGENTS.md`:** the repo gets one, first in v0.8: the §2 layout and principles as coding standards, what context each agent is given and how it is injected, and the same-step cleanup rule. Every agent-written change, including ACE's own PRs, is reviewed against it. New v0.8 code goes in its own `internal/<component>/` packages (for example gate, watchdog, science), not into `internal/cognitive`.

### 8.1 Compiled-in constitution and mission

- **Constitution:** one file, embedded with `go:embed`: the Heuristic Imperatives, the UDHR as the mediation framework (the 2024 L1 rule), the autonomy triad and the epistemic law. Its sha256 is pinned at build and checked at startup. It is the only source of truth: `store.go` stops seeding `soul.md`, and the constitution paths are not writable through HTTP or tools.
- **Versioning:** every cycle and verdict records the active constitution version.
- **Model roles in config (minimal, before the 9.2 registry):** `models.main`, `models.fast`, `models.ensemble` (three models from different families), all called through the existing provider client.
- **Mission:**
  - `ace init` asks for the life mission, the personality (free text, at most 1,000 bytes) and the capability grants, and writes them to owner-set config (§2).
  - Nothing runs until L1 has anchored the mission against the constitution. ACE idles without one.
  - **Re-steer:** the owner edits the mission and runs `ace mission reload`. L1 re-anchors it as a new mission version; every open plan node is marked stale and replanned (9.4). ACE never edits the mission.
  - **Goals:** L2 proposes nearer-term goals that serve the mission, perpetually; L1 accepts or rejects each against the mission and the constitution; the cycle report lists them.

### 8.2 L1 in three tiers

L1 reads all bus traffic (the Shapiro repo's design); L1's memory is written only by L1.

| Tier | What | Decides | Cost |
|---|---|---|---|
| 0 Rules | Deterministic. Pass: reads inside the jail, and writes inside the task's own workpad. Deny: the deny list (§5), matched on the parsed command, never on raw text; a command that does not parse escalates to Tier 2. ACE may send an access request for a denied action (8.4) | pass or deny, **with a verdict row naming the rule** | none |
| 1 Fast gate | `models.fast` on all other north/south messages | allow or escalate | small |
| 2 Full L1 | Uncertain, high-stakes, or escalated cases | allow / revise / reject / **redirect** (overrides L2–L6 top-down; reaches a running L6 task at its next tool boundary) | full; `models.ensemble` votes on rejections and self-changes |

- **Coverage measured:** in the archived v0.6 run, 342 of 602 `thought_traces` rows (57%) were reads or workpad writes, i.e. Tier-0 cases (`archive/v0_6_decommissioned_20261023/ace-sandbox-v0_6-full/ace.db`).
- **Defence in depth:** L6 shell commands run in the bwrap jail, where only `/workspace` is writable and `HOME=/workspace` (`internal/sandbox/jail.go`). The deny list also covers actions outside the jail.
- **Invariants, enforced in code and tested:**
  - nothing runs before L1 has anchored the mission;
  - every side-effecting action has a verdict row from some tier;
  - a redirect reaches a running L6 task at its next tool boundary.
- **Pyramid as a measured invariant:** per-layer call and token shares are logged every cycle; a share outside its band (§5) alerts L1.
- **Errors:** a Tier-1 error escalates; a Tier-2 error rejects. If fewer than three ensemble models answer, a rejection stands and a self-change is deferred.
- **Moral dilemmas** go north and **moral judgements** come back south as guidance (the 2024 aspirational-layer design).
- **Learning loop:** Tier-1 verdicts and their outcomes form a digest for the next Tier-2 run. Tier 2 may update Tier-1 rules and examples; each update is itself a logged L1 verdict.
- **Calibration:** a share of Tier-1 allows (§5) goes to Tier 2 blind. The disagreement rate is the false-allow estimate; the Tier-1 threshold is tuned to keep it under the target (§5). The local GPU gate and free fast models are Tier-1 candidates.
- **Untrusted content:**
  - Web, tool output, channel and file text carries a provenance tag and reaches every gate as quoted data, never as instructions.
  - A memory write sourced from untrusted content needs corroboration: a second source with a different origin, or a test ACE ran.
- **Naming:** code that says "L1.5" means Tier 1.

### 8.3 Imperatives built into every layer's mechanics

| Layer | Aware (prompt) | Built into what the layer runs on |
|---|---|---|
| All | The constitution prefix | Every side-effecting action passes through the tiered L1 gate |
| L2 | Yes | Strategy ranking scores each option on all three imperatives; the weights come from the constitution, not the prompt. A strategy that duplicates an open or finished one (§5 similarity rule) is rejected. A branch stops spawning children after a run of results that don't move its primary statistic (§5) |
| L3 | Yes | The self-model records harms, errors and fabrications as capability limits, in a table that L1 and L4 read |
| L4 | Partly | Plan priority = value to the mission × imperative score. The 9.4 time allocator uses the same score |
| L5 | No | Stuck/switch logic; the value-of-information ledger ("increase understanding") decides what to learn next |
| L6 | No | Tool permissions (reduce suffering) plus the scientist gate (increase understanding); L6 never reasons about ethics |

### 8.4 Autonomy and the owner

- **Human-approval paths:** every one listed in `AUDIT_2026_10_24_GAP_PLAN.md` §3.3 is replaced by an L1 ruling, then quarantine.
- **Pull-request gate, enforced by GitHub, not ACE:**
  - ACE's GitHub App can push branches and open pull requests, but cannot merge.
  - `main` requires owner approval.
  - The Telegram review request is timed by derived availability.
  - ACE keeps working while a pull request waits.
- **Profile, skills and workspace changes** are not repo changes. They go through L1, the scientist gate and watchdog rollback.
- **Access requests:** when an action needs access ACE cannot grant itself, ACE sends one request with the action, why it serves the mission, the exact scope (one-time or standing), and what it does meanwhile. A grant is stored in config with its scope, date and the owner's message ID; L1 checks every later use against that scope. No reply means no access. ACE never retries a denied action by another route.
- **Owner questions:**
  - For contradictions ACE cannot reconcile and for missing facts.
  - Telegram, plain English, one problem per message, in a fixed template: the problem, options with trade-offs, a recommendation, and the assumption ACE works on until answered.
  - ACE expects late replies and keeps working on its stated assumption.
- **Derived availability:** a model of when the owner replies, fitted on the timestamps of the owner's own messages to ACE: recency of the last message plus hour of day. On 4,529 owner message timestamps this beat a fixed sleep window (log loss 0.514 vs 0.647). Reply delay itself is poorly predictable (median 2.5 h, p90 17.5 h), so it only times non-urgent messages; it never delays an access request.
- **Status probes:** a status or steering message is answered at once, without pausing the loop.
- **Cycle report:** each cycle ends with what was done, what was measured, goals proposed, and open uncertainties, folded into the digest.
- **Directive ledger:** every owner message that steers ACE is stored verbatim with an ID and status. It survives compaction, and unmet directives appear in the cycle report.
- **Channel hygiene:** never post internal tool output or execution logs to the chat. Rate-limit proactive messages (§5) and fold the excess into the digest.
- **Owner signal as data:** PR verdicts, PR edits and replies are stored as (proposal, edit, verdict) triples. They stay off the learning path until a measurement shows they help.

### 8.5 Integrity overlay and data safety

- **Watchdog:** `ace watchdog` is a separate process from the same binary. It reads a status snapshot that ACE writes atomically each tick to its state directory, never via the bus.
- **Checks:**
  - heartbeat, constitution hash, config checksum inventory;
  - gate fail-closed state, spend, runaway loops, free disk and RAM (§5);
  - the hash chain of the event log (Sello).
- **Actions:** pause, restart, roll back, and a kill switch from the CLI, local HTTP or Telegram `/halt`. Halt takes effect within the halt limit (§5), cancels in-flight model calls and child processes, and reports what it cancelled.
- **Stall detection:** a layer that announces an action and then makes no matching tool call within the stall limit (§5) is a stall fault; the watchdog resumes it.
- **Backups:** an hourly SQLite `VACUUM INTO` snapshot to the state directory, outside the jail; 24 hourly and 7 daily are kept. A schema change runs only after a fresh snapshot.
- **Schema changes:** numbered SQL files embedded in the binary, applied in one transaction at startup.

### 8.6 Scientist gate

- **Engineering tasks:** pre-declared checks, run by the gate in a fresh sandbox.
- **Claims:**
  - pre-registration: hypothesis, one primary statistic, alpha, minimum detectable effect, dataset sha;
  - the module provides only per-item predictions; the gate recomputes the statistic in Go. The Go statistics are tested against published reference values; the v0.7 `ace_rlm.py` implementations are not trusted;
  - three baselines: a constant or majority predictor, a shuffled-label control, and the incumbent (the current component, or none);
  - degeneracy checks (label entropy, constant outputs) and negative controls;
  - independent labels: produced by a source outside ACE (a public dataset, an external system, or a test run), never by ACE's own outputs or traces;
  - an execution transcript for any claim about another system;
  - replication and ablation;
  - fresh regeneration of every artifact by the gate;
  - a generated ledger, including counter-evidence.
- **Pre-registration lock:** the pre-registration is frozen before the first run. Any change is a new pre-registration on the same question ID; every attempt is reported; alpha is Holm-corrected across all attempts on that question.
- **Power:** n comes from alpha, power and the minimum detectable effect (§5). Underpowered results are reported as "no large effect", never as a pass.
- **Freshness:**
  - Every external fact records its retrieval date and the version it describes.
  - Competitor and model versions are resolved to the latest release at run time.
  - Sources older than the mission's freshness window (§5) are flagged, never silently used.
- **Evidence tiers:** `EXTERNAL` = passed this gate with independent labels; `PROVISIONAL` = anything else. Only `EXTERNAL` results can keep or revert a component.
- **Organ rule:** a component stays wired in only with an `EXTERNAL` result on its pre-declared statistic. All v0.7 results are re-graded here.

### 8.7 Science as the core loop

8.6 judges claims. This section makes the scientific method how ACE works.

- **The loop:**
  1. L2 states strategies as hypotheses with expected outcomes.
  2. L4 turns each into an experiment plan: dataset, baselines, primary statistic, budget.
  3. L6 runs it.
  4. L3 updates the self-model from the result.
  5. L1 sees the conclusions.
- **Datasets:** ACE builds, versions (by hash) and grows its own datasets and benchmarks as a normal part of any mission, and optimises against held-out splits it cannot read during development. Public benchmarks are preferred when they exist.
- **Standing metrics, on every run:** turns per task, cost per successful outcome, latency, cache hit rate (from provider-reported cached tokens), per-layer call share, task success, and the error of L3's estimates.
- **Exploration:** a share of each budget (§5) goes to the least-explored open items in the uncertainty ledger, so unknown unknowns surface; from v0.9 it covers the least-mapped regions of the problem-space map (9.5). The share is tuned on how often exploration changes a decision.

### 8.8 Cleanliness and retirement

Agents read dead code and stale docs as live. AST checks alone can't catch a conceptual flow that should have retired.

- **Concept registry:** every live flow, module, doc and skill is listed with an owner, a purpose and `supersedes`/`superseded_by`.
- **Same-step cleanup:** a step that changes, adds, updates or rewrites something also removes what it makes dead. The gate rejects a change that leaves superseded code, docs or concepts behind.
- **Plans and specs live in the repo**, next to the research they rest on.
- **Dead-code checks:** CI checks for unreferenced code, orphan files, and docs that name removed symbols.
- **Prompt rule:** every layer prompt says it cleans up what it replaces, and that the registry is authoritative.
- **Measured:** stale-reference count per release.

### v0.8 acceptance

- **Red-team fixtures, all rejected:**
  - an edited constitution;
  - a mutating request without the bearer token;
  - a gate error;
  - a result that only sets `h1_accepted:true`;
  - the v0.7 Hermes/OpenClaw ablation (`ace_v07/hermes_openclaw_ace_ablation.py` with `benchmarks/results/v07_axis11_1_hermes_openclaw_ace_ablation.json`), whose competitor numbers are hand-typed constants;
  - zero-entropy labels;
  - a shuffled-label control that passes;
  - a fallback to a CV p-value when the held-out test fails;
  - an attempt to merge its own pull request;
  - a re-run that changes the dataset or hypothesis after a failed pre-registration without a new question attempt;
  - L2 proposing a duplicate strategy;
  - an L6 action before L1 anchors the mission;
  - an injected instruction inside web content;
  - a change that adds a superseding module without retiring the old one;
  - each deny-list command, including one hidden in a pipeline or `sh -c`.
- **CI:** the domain-string test and dead-code checks pass.
- **A 24 h run, which shows:**
  - zero waits on the owner, with at least one access request sent and work continuing;
  - an L1 verdict row for every side-effecting action;
  - a mid-task redirect that reaches L6 at a tool boundary;
  - Tier-1 escalation and disagreement rates;
  - per-layer call shares inside their bands;
  - a watchdog halt within the halt limit, and a rollback from a snapshot.

---

## v0.9: Composable core, context and time

### 9.1 Plugin core

- **Registry:** manifests read without running plugin code. Each plugin declares its dependency keys; undeclared access is refused at runtime; dependency cycles are rejected at load (the Cordis model).
- **Fault isolation:** a plugin that fails to load, clashes on a tool name, or throws at runtime is caught, reported and disabled. It never kills the core loop.
- **Ports:** Provider, Tool, Channel, Sense, Gate, Memory, Sandbox, Bus and Context. Telegram and the opencode hook move behind the Channel and Provider ports; the opencode hook leaves the repo for the owner's personal extension.
- **Senses:** internal (proprioception: CPU, memory, disk, network, LLM usage, quota, capability) and external (exteroception: web, channels, files, and any device sensor such as temperature, audio, video, lidar, GPS). What is available depends on the host. Each layer gets a configured mix.
- **Tools:** the action list is generated from the registry.
- **Extension:** MCP over stdio for third-party tools; 5–10 hooks, not dozens.
- **Skills:** `SKILL.md` format with precedence workspace > user > global; lifecycle in 9.6. Third-party skills are scanned at install and quarantined until they pass the gate.
- **Profiles:** all domain content lives in profiles, outside Go.
- **Invariant:** anything sent to a model can be reconstructed from the event log.
- **Egress:** network access goes through a per-mission allowlist with brokered secrets; tools never see raw keys.

### 9.2 Provider registry (from the 2026 design plus the 2025 code)

- **Model tiers:**
  - **Main:** the thinking model on the subscription.
  - **Fast:** for gating, verification, CRUD and high-RPS work, ordered by measured speed and falling back in that order.
- **Fallback rules:**
  - The main model is never replaced by a free model; it is used until its quota is spent.
  - A free model falls back only to another free model.
  - Provider groups have priority and failover, plus a circuit breaker (§5).
  - Errors are classified; only transient classes retry, with full-jitter backoff (base 1 s, cap 8 s, 2 retries), before switching group. On exhaustion the error code and reason are surfaced.
  - Timeouts: cloud 60 s, local 300 s.
- **Model churn:** the catalogue records each model's source, price, context, knowledge cutoff and Artificial Analysis index. Any model change re-runs Tier-1 calibration and the standing benchmark before the new model takes a role.
- **Usage accounting:** token counts only as reported by the provider; cost = input × input rate + output × output rate, with cached tokens priced at their own rate; a billing-path field per call.
- **Per-layer model roles (2025 `defaults.py`):** L1–L2 reasoning, L3–L4 general, L5 efficient, L6 tool-calling; temperature 0.2 for logic, 0.7 for creative roles.
- **Key storage:** encrypted with per-field AES-256-GCM (port `backend/internal/crypto/field_encryption.go` from `ac7d68c9`); only the last 4 characters are ever shown.
- **Thinking:** on only for the main model; gate, router and other side calls run with thinking off.
- **Side-model sizes:** small, medium, big; task size picks the size.
- **Quota windows:** subscription providers are budgeted by their windows (5-hour, weekly) and shown as % used; metered providers by cost.
- **Built-in providers:** OpenAI-compatible, Anthropic, Google/Vertex and Ollama. Credentials come only from ACE's own config, never another app's stored credential.

### 9.3 Layers, bus and cadence

**Topology:**
- L1 is the root and reads everything.
- L2, L3 and L4 form a parallel cluster over the dependency tree (`mission_dag_nodes`), sharing one map and wiki space. A node runs when its parents are done.
- L5 is the gate and coordinator. It tracks, consolidates and reports L6 results up to the cluster.
- Each L6 task gets its own scratchpad and workpad.

**Bus:**
- Northbound and southbound. Adjacent hops are enforced: a non-adjacent publish is rejected. L1 reads everything.
- Every message has a required plain-language `summary` alongside its JSON payload (the Shapiro repo requires human-readable inter-layer messages).
- Topic grammar `ace.engine.<agent>.layer.<n>.input|output`, validated. Every record carries agent, cycle and task IDs.

**Per-layer contracts** (Shapiro repo; each is a schema plus a test):

| Layer | Must hold or emit |
|---|---|
| L2 | Beliefs with probability, source and credibility; reconciliation of contradicting sources; strategies as hypotheses; a northbound belief summary |
| L3 | Self-model: capabilities, limits, config, state, measured capacity; episodic memory of its own past runs |
| L4 | Plans with steps, dependencies, checkpoints and success criteria; risks with a contingency each; an inventory of resources (tools, compute, data, budget, time) |
| L5 | Definition of done per task; task switching with a minimum dwell time and a thrash metric; northbound status (current task, % progress) |
| L6 | Expected feedback and thresholds declared before acting; failure signalled northbound on unexpected errors |

**Cadence and preemption:**
- Each layer has its own clock: L1 event-driven plus a slow periodic review; the cluster per node; L5 per tick; L6 continuously.
- A layer runs when both guidance (from above) and data (from below) have arrived. This replaces the 2-second polling loop.
- Every layer can escalate.
- Southbound redirects reach L6 at its next tool boundary.
- Zones or internal channels within the cluster are added only if a measurement shows they cut turns.

**Layer I/O:**
- Output schemas with up to 3 retries and a prefilled response opening.
- Read-only tools run in parallel; write tools run serially with a path-overlap check.
- Hot-reload of sectioned config; a per-layer sensor allowlist (2024 `presets.py`).

### 9.4 Time management

**Primitives (L5 executes them):**

| Primitive | Behaviour |
|---|---|
| Schedule | A named recurring job with a run log and dedupe; the due time advances only after a real run |
| Heartbeat | A periodic check that skips when there's nothing to do and logs the skip reason |
| Monitor | Watches a sense or condition and fires an event (a Sense plugin) |
| Deadline | Attached to a task; L4 is alerted when it is at risk |
| Long job | Background, with a progress heartbeat, an idempotency key, cancel, and checkpoint/resume after a crash |
| Child agent | Depth 1; tool blocklist (no delegate, memory, send-message); a concurrency cap per provider; a timeout; a parent kill switch |

- **One shared task manager** serves every layer.
- **Timeouts:** every tool call has a default timeout (§5). A long call detaches to a background handle the layer can poll, read or cancel.
- **Storage:** durable in SQLite: `runs`, `tasks`, `schedules`, `checkpoints`, and `usage_events` (tokens, cost, cached tokens, retries, duration, billing path).
- **Time awareness:** every layer prompt carries the current time, the time since the last owner contact, and derived availability.

**Budget governor** (v0.7 `budget_governor` idea): use the subscription quota fully, with no waste.
- Tracks quota windows and burn rate.
- Allocates budget per layer and per mission branch, with L6 getting the bulk.
- Slows down rather than stopping when the projected burn exceeds the remaining window (§5).
- **Measured:** quota utilisation and cost per successful outcome.

**Ownership:**

| Layer | Role |
|---|---|
| L2 | Commitments and horizons, e.g. "check X weekly", "ship Y this month" |
| L3 | Measured capacity: learns task-duration and cost distributions from traces, reports estimate error |
| L4 | The time plan: allocates time and budget, creates schedules, monitors and deadlines, resolves priority conflicts by value × imperative score |
| L5 | The clock: dispatch, preemption, task switching, retries, resume |

**Plans that change with information:**
- Every plan node records the assumptions it depends on.
- A new fact that touches an assumption marks only the dependent subtree stale, and only that subtree is replanned.
- Conflicts go: between facts → the uncertainty ledger; between priorities → L4; between values → L1 Tier 2; unresolvable → an owner question (8.4).
- **Measured:** replan rate, estimate error, deadline hit rate, and how long stale plans waited before replanning.

### 9.5 Context system

The shortest path to a task is having the right context at turn 0.

**Stages, each a composable with sub-buckets:**

| Stage | What | How |
|---|---|---|
| Turn 0 | Deterministic injection: map regions of interest, memory, focused spec, task, files, self-stats | code only |
| Turn 0.5 | Retrieval routing: a cheap model fills tool fields to enrich turn 0 before the worker's first turn | off by default (§2) |
| Turn 1 | Search by the worker, for what can't be injected | per-layer tools |

**Problem-space map:**
- A graph of what the mission touches (files, docs, people, sources, open questions, specs, datasets) and the links between them.
- A deterministic tree search covers every branch downstream of the region of interest; its frontier feeds turn 0.
- The uncertainty ledger and the living specs (9.6) are nodes in the map.
- Each layer has its own view; the cluster shares one space.

**Uncertainty closure:**
- Open items: files not viewed, questions unanswered, things not tested or measured, assumptions not validated, feedback not gathered, simulations not run, research not done, protocol not followed.
- Each is ranked by value of information and worked within a budget. It never blocks work.

**Budgets:**
- Bucket and spec budgets are in bytes (deterministic, no tokeniser); prompt totals are in provider-reported tokens.
- Bucket budgets start from §5 and are tuned by measurement.
- L1–L5 prompts target under 4k tokens (v0.6 measured about 10.5k); L6 prompts may use up to 256k with a stable prefix for caching.

**Per-layer context contract:** each layer declares what it is fed without asking (turn 0) and has tools for the rest. A test checks that every declared input arrives.

**Sessions and cache:** each layer keeps a persistent session; prompt prefixes are ordered stable-first; cache hit rate is a standing metric (8.7).

**Overflow to file:** a tool result or input over its cap is written to a file. The model gets the head, the size, the path and an instruction to read by range. Never a silent clip, never a full dump.

**Compaction:**
- Tiers snip → micro → collapse → auto, at the thresholds in §5.
- Deterministic in the hot path: keeps a raw tail of recent turns and caps tool output with no model call. A model summary runs only at the last tier, updated in place with a fixed template (Goal / Constraints / Progress / Blocked / Next), and stops after 3 consecutive failures.
- **Recap:** a one-line summary per turn. After compaction the last N turns come back with time and stats; older turns are bucketed.
- **Injection log:** every injected block (constitution, memory, config or model change, compaction) is an event with source, size and time. Control-plane changes reach the model as in-band notices.

**Measured:** turns to first useful action, retrieval miss rate, tokens per success.

### 9.6 Memory, living specs and skills

**Memory:**
- Record types: episodic (what happened), semantic (facts with source and verification date), procedural (skills).
- **Admission:** each store has a size cap (§5), a stale-write check (monotonic `updated_at`) and atomic writes.
- **Retention:** score = importance × decay by age (§5); consolidation passes merge and retire entries.
- **Write rights:** L1's memory is written only by L1. Retrieval feeds L2–L4 through turn 0.
- **Health:** retrieval hit rate, admission reject rate and age since last verification go to the watchdog.

**Living specs:**
- Each task's journey is distilled into a clean spec: in scope, out of scope, nuances. No decision history; that lives in the event log.
- The spec budget is 2,800 bytes, enforced by the gate; output is checked against the spec.
- When output needs the spec to change, the spec is consolidated, not appended to.
- A spec stays live until the task's done criteria pass the gate and L1 closes the task; for repo work, until the PR is merged.

**Skills:**
- Distilled automatically from successful trajectories.
- Each skill carries a Beta(α, β) success posterior and is retired when it falls below the retirement rule (§5); retirement is recorded in the concept registry (8.8).
- Progressive disclosure: about 100 characters per skill in the prompt.

### v0.9 acceptance

- **No-code extensibility:** a new tool works via a manifest plus MCP server with no Go change; the provider is swapped by config alone.
- **Fault isolation:** a plugin that panics is disabled and the loop keeps running.
- **Providers:** a fault-injected provider trips the breaker and fails over in the declared order; a free model never replaces the main model.
- **Child agents:** a child calling a blocked tool is refused.
- **Egress:** a request to a host off the allowlist is refused.
- **Durability:** an injected crash mid-job resumes from its checkpoint with no duplicated side effects.
- **Schedules:** a weekly schedule and a monitor both fire correctly across restarts.
- **Replanning:** a planted contradicting fact replans only its subtree.
- **Bus:** a non-adjacent publish and a message without a summary are both rejected.
- **Context:** every layer's declared turn-0 inputs arrive; bucket budgets hold; compaction fires at each threshold; the cache hit rate is reported.
- **Specs:** a spec over budget, or output that diverges from its spec, is rejected.
- **Skills:** a skill whose posterior falls is retired and registered.
- **Model churn:** removing a model from the catalogue re-routes by tier and re-runs calibration.

---

## v0.10: General and proven

### 10.1 Install

- **Packaging:** `install.sh` + `verify.sh` with checksums from the same release, ported from `scripts/` at `ac7d68c9` with their fixed `/tmp` paths removed; goreleaser (`.goreleaser.yml` at `ac7d68c9`) for linux and darwin on amd64 and arm64, plus windows.
- **Clean-clone build:** `cmd/ace` embeds `web/build`, but `/web/` is gitignored and untracked, so a fresh clone does not compile. The build makes the embed work from a clean clone.
- **Commands:** XDG dirs, `ace paths`, `ace doctor`, uninstall. `ace init` asks five questions (mission, personality, capabilities, main provider and key, Telegram chat) or takes a full config file. `ace start` is manual; once started it runs 24/7.
- **Doctor checks:** bind address, bearer token set, chat allowlist not empty, sandbox health, and an "unsandboxed" warning on Windows.
- **Providers:** a live test call when a provider is saved.

### 10.2 Optional runtimes and local-first

- Python and the GPU gate are optional plugins.
- **Targets, measured on the owner's machine:** peak RSS of the ACE process tree under 4 GB; L1–L5 prompts under 4k tokens.
- **KV cache:** quantised KV cache is tested as an optional local setting before adoption.

### 10.3 Self-describing mission suite

- **Mission suite:** unrelated missions on empty workspaces, plus the owner's real daily work types; outputs are scored by the scientist gate against the owner's past outputs as reference, with ensemble pairwise judging.
- **Public benchmarks:** the suites SOTA models are graded on (Terminal-Bench, ARC-AGI, Humanity's Last Exam, maths, finance, law). Scaffold pinned; pass^k and Consistency@k reported; n from the power rule (8.6).

### 10.4 Capability set

Each capability is a plugin, granted per mission by least privilege, and kept only if it measurably helps.

| Capability | Notes |
|---|---|
| Web search and research | Multi-engine search with fetch, extraction and citation; ACE researches proactively when stuck |
| Browser | Element-ID targeting before coordinates |
| Computer use | Opt-in; priority connectors > browser > screen; a per-OS matrix (Wayland needs a portal or uinput for input automation) |
| Code tools | Hashline editing with a 32-line edit cap, LSP diagnostics, a debugger only from a vetted third-party source, deterministic output trimming (the owner's Pi harness settings) |
| Email, calendar, files | For personal and research-assistant missions |
| Money, social, accounts, launches | Guards: a hard spend cap per period, set at creation and enforced in Go; every public or financial action goes to full L1 with the ensemble vote; ACE creates and uses its own accounts, with credentials in its encrypted store; accounts or resources it doesn't own need an access request (8.4); every such action is in the digest |
| Owner channels | Telegram first, with canonical channel IDs |

**Persona:** the chat is agentic and speaks with the personality set at creation, held in the profile. Replies are short and plain.

### 10.5 Competitor comparison and self-improvement

- **Competitor comparison:** the current Hermes and OpenClaw releases, resolved at run time, actually installed and run; same model, pinned scaffold, budget from the budget governor, through the scientist gate; n per arm from the power rule.
- **Layer contribution study:** ACE versus a single-loop agent on the same model, tools and budget, with each layer ablated in turn. A layer that underperforms is redesigned, re-cadenced or made cheaper, never removed.
- **Profile and skills:** change through the gate, L1 and rollback.
- **Core code:** changes only via pull requests the owner merges. The running ACE never updates its own binary.
- **Method (Harness²):** improvement harness separate from execution; INV/DOM candidates probed under matched settings; four-way attribution with `confounded` mandatory; only non-confounded lessons persist; efficiency changes carry a non-inferiority guard (§5).
- **Successors:** ACE's standing goal is a better, cheaper, lower-latency ACE. It researches and builds its successor as a PR.

### 10.6 Model drift

A periodic ground-truth probe per provider (the v0.7 `drift_guard` idea); drift beyond its threshold alerts L1 and the watchdog.

### v0.10 acceptance

- **Fresh VM:** `curl | sh`, `ace init`, give it a mission; within 24 h the mission's own pre-registered primary metric beats its baseline, with no code changes.
- **Benchmark report:** generated by the gate, with competitor install logs and transcripts.
- **Layer study:** each layer has an `EXTERNAL` result for its contribution.

---

## v0.11: Web UI from first principles

A design session with the owner comes first. Inputs:

- **Questions the UI must answer, in order:**
  1. What is it doing right now, and why?
  2. Is it stuck or healthy?
  3. What did it cost?
  4. What does it need from me (pull requests, access requests, questions)?
  5. How do I steer or stop it?
- **Transport:** SSE; the 2026 replay semantics (sequence IDs, a bounded buffer, `resync_required`, REST as the source of truth); a versioned, typed event schema, so headless clients work too.
- **Building blocks:** a run timeline with a per-step grade and duration; a "what each layer saw and decided" inspector (L1 verdict chain, context slice, retrieval trajectory) from real data; a schedule and monitor calendar; cost per run; an inbox for pull requests and access requests; a mission editor; chat; a kill switch.
- **Control direction:** the uplink only steers (mission, pause, stop, message, access grants).
- **Minimal and readable first.** 2D only.
- **Liveness:** never looks frozen. Every in-flight action shows its state, a counting elapsed timer, the current sub-step and the heartbeat age. Thinking shows duration and tokens, not text. Tool groups collapse with counts per action type.
- **Basics:** skeleton, error and empty states; connection states; prefers-color-scheme; reduced motion; semantic tokens; a command palette.
- **Avoid:** generic dashboard cards, disabled placeholder navigation, decorative figures.
- **Visual explainers:** the session decides how ACE shows a complex component to a human.

## v0.12: from the v0.10 data

v0.12 targets ACE's weakest area in the v0.10 comparison. Candidates already raised by the owner:
- reality replicas or digital twins, to simulate before acting (with rollback);
- multi-sensor fusion;
- embodied sensors and actuators.

---

## 4. Process for every version

- **Primary metric:** each version pre-registers one primary metric, its baseline and its minimum detectable effect before the build. Each new pillar ships behind a flag so it can be ablated.

  | Version | Primary metric | Baseline |
  |---|---|---|
  | v0.8 | Share of side effects with a valid L1 verdict (target 100%); guard: Tier-1 false-allow rate | v0.7.1: 1 L1 call for the whole run |
  | v0.9 | Turns per successful task, non-inferior on success rate | the v0.8 24 h run |
  | v0.10 | Mission-suite success vs the single-loop baseline and competitors | measured in the same study |
  | v0.11 | Owner time to answer the five UI questions | the v0.10 UI, timed in the design session |

- **Build:** the owner-directed builder (a coding agent) commits straight to `main`, with no PRs, and builds every slice of the version back to back without waiting for a review or a go. A version is done when all its slices are built, the live run shows them working, and its acceptance passes; it is then tagged `v0.N` and pushed. From v0.10, when ACE builds itself, its changes go through the PR gate (§0) on one long-lived branch that it keeps building on while the PR waits.
- **Live build:** every version is built while ACE runs live on its mission, because some bugs only show in a live run. The builder watches the run, fixes what breaks with a regression test, hot-restarts ACE and confirms the fix in the live run. A stall (ACE up but making no model calls) counts as a bug.
- **Launch:** the first hour after start is observed by the builder, which fixes issues on `main`. Run data (`thought_traces`, runs, workpads) is archived; mission, constitution version, profile, memory, skills, datasets, directive ledger, grants and concept registry carry forward.
- **Spec:** each version's spec is written from this plan before its build; the superseded spec is archived in the same change (8.8).

## 5. Starting values

Every value below is a config key with this default, tuned only by measurement, and every change is a logged L1 verdict.

| Key | Default |
|---|---|
| Deny list (Tier 0) | `rm`, `rmdir`, `shred`, `find -delete`, `truncate`, `mv` with a target outside the workpad or equal to `$HOME` or `/`; `chmod -R`/`chown -R` outside the workpad; `kill`/`pkill`/`killall` on a PID not in ACE's process tree; `mkfs*`, `dd of=/dev/*`; `git push --force`, `git reset --hard` on a branch ACE didn't create; writes to owner-set config |
| Pyramid bands (share of calls) | L1 1–5%; L5 ≤10%; L2+L3+L4 ≤35%; L6 ≥50%. L6 ≥70% of prompt tokens. v0.7.1 measured L6 at 68% of calls and 87% of tokens |
| Tier-1 calibration | 5% of allows sampled to Tier 2; false-allow target: upper 95% bound ≤2% |
| Halt limit / stall limit | 5 s / 120 s |
| Proactive messages | at most 1 per hour outside the digest |
| Science | alpha 0.05; power 0.8; default MDE 10 points on proportions; Holm family = all attempts on one question ID |
| Non-inferiority margin | 2 points absolute on success rate |
| Freshness window | 90 days |
| Exploration share | 10% of each budget |
| Duplicate strategy | word-shingle Jaccard ≥0.8 against any open or finished strategy |
| Diminishing returns | stop a branch after 3 consecutive child results that don't move its primary statistic by the MDE |
| Circuit breaker | 60 s cooldown, doubling, at most 300 s |
| Budget slow-down | when projected burn exceeds the remaining window, scale non-L6 cadence by remaining ÷ projected |
| Tool timeout | 120 s, then detach to a background handle |
| Compaction thresholds | snip 50%, micro 65%, collapse 80%, auto 90% of the context window |
| Turn-0 slice budget | 8 KB; spec 2,800 B |
| Memory caps | semantic store 2,200 B per profile summary, episodic store 10,000 entries |
| Retention | importance × 0.5^(age ÷ 30 days); lowest evicted when over cap |
| Skill retirement | after ≥5 uses, P(success rate < 0.5) > 0.9 |
| Retention caps (disk) | traces 30 days; workpads of closed tasks 7 days; free-disk floor 10 GB |
| Spend cap | 0 until set at `ace init`; period monthly |

## 6. Owner actions

These are in the owner's accounts, so ACE and its builders cannot do them:
- Revoke the Telegram bot token with @BotFather and give ACE the new one through a `.env` file (the current token is in `.ace-sandbox/ace.db`, and the repo is public).
- Revoke the 2024 news API key (`e4bf2d0f`, `app/components/telemetry/scraper.py:19`), and rotate the 2024 JWT secret if that app still runs anywhere.
- Create ACE's GitHub App (push and open PRs, no merge) and turn on branch protection on `main`.
- Check whether the OpenCode Go plan issues an API key. If it does, ACE uses it from its own config from v0.8; until then the existing hook stays as-is and leaves core in 9.1.
