# Owner asks → plan traceability

**Sources:**
- Pi session `2026-10-17T21-56-45` (154 owner messages, 2026-10-17 to 10-24).
- Pi harness sessions `2026-09-06T09-26` and `2026-10-18T09-31` (Lytehouse, factory and Godot sessions excluded).
- Claude session `c24b841b` (2026-10-22 to 10-24).
- Ticket #1708 activity log and KB pages.
- Shapiro repo `ACE_Framework.md`; ACE's own `SPEC_ACE_V0_7_REWRITE.md`.

**Status:**
- **ok**: covered.
- **fixed**: was partial in an earlier draft, now complete.
- **added**: was missing from an earlier draft, now added.

## A. Architecture and Shapiro fidelity

| # | Ask (date) | Plan | Status |
|---|---|---|---|
| A1 | Dave's blueprint is the whole point; heuristic imperatives (10-23) | 8.1, 8.3 | ok |
| A2 | Mission: single steerable directive, unshifting once set at L1 (10-23) | 8.1 | ok |
| A3 | Nothing may run before L1 (10-23) | 8.2 invariant + acceptance | added |
| A4 | Pyramid: L1 least, L2–L4 thin and uneven, L6 the workhorse (10-23) | 8.2 measured invariant | fixed |
| A5 | Keep the L2–L4 parallel cluster as a dependency tree with shared map and wiki; L6 per-task workpad; L5 tracks and consolidates up to the cluster; L1.5 gate (10-22, 10-24) | 9.3 | added |
| A6 | The layers are the whole point; never short-circuit them (10-21). Later (10-24): a cheap gate may pass simple decisions, full L1 when uncertain | 8.2 tiers; 10 study validates and optimises | ok (10-24: test, never remove) |
| A7 | Imperatives in every layer's core ethos, aware or not (10-24) | 8.3 | ok |
| A8 | Ingrained in code, not a replaceable soul.md (10-24) | 8.1 | ok |
| A9 | Every layer gets the access and data its role needs, wired per the paper (10-21) | 9.3 Shapiro fixes, section B below | fixed |
| A10 | Maybe not a simple up/down stream: zones, internal channels, different pacings (10-18) | 9.3 cadence | added |
| A11 | Root-to-tip: high-level thought steers 24/7 action in real time (10-24) | 8.2 redirect reaches L6; 9.3 preemption | fixed |
| A12 | L1 cheap gate plus full L1 (10-24) | 8.2 | ok |

## B. Shapiro repo requirements (subagent audit)

| # | Requirement | Plan | Status |
|---|---|---|---|
| B1 | Tier 0 exempted workpad writes, contradicting "L1 verdict for every side effect" | 8.2: Tier 0 writes a rule-ID verdict row | fixed |
| B2 | Redirect skipped L6 | 8.2: redirect reaches L2–L6 | fixed |
| B3 | Two buses (north/south); adjacent hops enforced; messages human-readable | 9.3 envelope | added |
| B4 | L2 probabilistic beliefs, source credibility, contradiction reconciliation | 9.3 layer contracts | added |
| B5 | L3 episodic memory and a config/state self-model | 9.6, 9.3 | added |
| B6 | L4 risks, contingencies, resource inventory, success criteria | 9.3, 9.4 | added |
| B7 | L5 definition of done, switch hysteresis, northbound status | 9.3 | added |
| B8 | L6 success/failure contract (expected feedback, thresholds) | 9.3 | added |
| B9 | Integrity: out-of-band reads, config inventory, model drift inspection | 8.5 | added |
| B10 | L1 issues mission objectives (repo) vs human-only mission (plan) | §0, 8.1 | ok (10-24: mission, personality, capabilities at creation; ACE proposes goals) |
| B11 | Cognition-first; physical embodiment | §2, 10 capability set | added |
| B12 | L1 is fully introspective: input only from lower layers, read-only, via L2; output only to L2 | 8.2, 9.3 | added (paper §3.2.6–3.2.7: tiered L1 observes all layer I/O, writes only verdicts) |
| B13 | L5 frustration = running success/failure ratio that signals task-switch; cognitive damping = deliberation over options before acting | 9.3, 8.3 | added (paper §3.6.2–3.6.4: goes beyond switch hysteresis — explicit attempt ratio + options weighing) |

## C. Scientist, measurement, honesty

| # | Ask (date) | Plan | Status |
|---|---|---|---|
| C1 | Scientific method; every statement needs proof (10-18) | 8.6 | ok |
| C2 | MUST NOT CHEAT; no shortcuts (10-20) | 8.6 lock, red-team fixtures | ok |
| C3 | Science is ACE's core loop, not only an end gate (10-24) | 8.7 | added |
| C4 | Builds its own datasets and benchmarks and optimises against them (10-20, 10-21) | 8.7 | fixed |
| C5 | Public SOTA benchmarks: Terminal-Bench, ARC, HLE, math, finance, law (10-22) | 10 external validation | added |
| C6 | Research must be current, not April 2026 (10-23) | 8.6 freshness rule | added |
| C7 | Model the traces regularly: turns, cost, cache, performance (repeated) | 8.7 standing metrics | fixed |
| C8 | Compare with your own daily output and quality (10-20) | 10 mission suite | added |
| C9 | Per-version measurement plan | section 5 | added |

## D. Autonomy, owner relationship, time

| # | Ask (date) | Plan | Status |
|---|---|---|---|
| D1 | No human approval anywhere; ACE self-approves what it can and asks the owner only for access it can't grant itself (10-24) | §0, 8.4 | ok |
| D2 | "The only human gate *for now*": owner merges PRs, requested on Telegram (10-24) | §0, 8.4 | ok (revisable) |
| D3 | Contradictions go to the owner on Telegram: plain English, short (10-22) | 8.4 owner questions | fixed |
| D4 | Time-aware: owner sleeps, replies later; availability derived, never hard-coded (10-22) | §2, 8.4 | ok |
| D5 | Telegram: no spam, no tool-exec noise (10-18) | 8.4 | added |
| D6 | Chat more agentic, with personality (10-18) | 10 profile | added |
| D7 | Time management: schedules, monitors, long jobs, conflicts, plans morph with information (10-24) | 9.4 | ok |
| D8 | Async/continuous/resumable/polling tasks with one shared manager (10-22) | 9.4 | ok |
| D9 | Runs 24/7, soaks up subscription usage efficiently (10-18) | 9.4 budget governor | added |
| D10 | Manual start only; no systemd, no autostart (10-21) | 10 install | added |
| D11 | Uncertainty closure is a prioritised, budgeted effort, never a hard gate (ticket) | 9.5 | fixed |

## E. Context, memory, specs

| # | Ask (date) | Plan | Status |
|---|---|---|---|
| E1 | Context is everything: right context at turn 0 (10-22) | 9.5 | added |
| E2 | Turn 0 deterministic / 0.5 routing / 1 search, as composables with sub-buckets (10-22) | 9.5 | fixed |
| E3 | Problem-space map: connectomes, deterministic tree search over regions of interest, cross-cutting; the spec plugs into the map (10-22) | 9.5 | added |
| E4 | Context buckets bounded by fixed SLA-style budgets (10-20) | 9.5 | added |
| E5 | Each layer fed its context without asking, plus dedicated per-layer tools (10-22) | 9.5 | added |
| E6 | 256k context with caching; cache hits; per-layer persistent sessions (10-18) | 9.5 | added |
| E7 | Catastrophic forgetting → distillation into a clean living spec (in scope, out of scope, nuances) (10-22) | 9.6 | added |
| E8 | 2800 B spec budget, enforced and uncheatable; output must match spec; living until the human calls it done (10-22) | 9.6 | added |
| E9 | Skills plus automatic skill management (wiki-skill) (10-22) | 9.6 | fixed |
| E10 | Proprioception (cpu/mem/disk/net/LLM usage, capacity) plus exteroception (10-22) | 9.1 Sense port, 9.3 L3 | fixed |
| E11 | Any sensor: temp, UV, IR, lidar, radio, robot tactile/gyro/GPS; reality replicas (10-23) | 9.1, v0.12 candidates | added |

## F. Engineering standards

| # | Ask (date) | Plan | Status |
|---|---|---|---|
| F1 | Single self-contained portable Go binary, domain-agnostic (10-17, 10-22) | §2 | ok |
| F2 | Modular, composable, plugin-based like the DeepSeek harness (10-17, 10-24) | 9.1 | ok |
| F3 | Composition like Godot: shared systems, standards, connectors, no duplicates (10-22) | §2 principles | added |
| F4 | Filesystem 2 nesting levels (3 rarely); no god scripts (10-18, 10-22) | §2 principles | added |
| F5 | Deterministic first, model only where needed; derive, don't hard-code (10-22) | §2 principles | added |
| F6 | Least privilege, with access paved deliberately and shown to help (10-22) | 10 capability set | added |
| F7 | Clean up after itself: dead code and dead concepts retired at a structural level, with prompt and gate fixes (10-20) | 8.8 | added |
| F8 | Isolated sandbox top priority (10-18) | §0 | ok |
| F9 | Pi tools: hashline edit, LSP, blackhole (10-18) | 10 capability set | added |
| F10 | Monitor the first hour after every launch and patch live (10-21 onward) | §4 launch | added |
| F11 | Commit, tag every version, push (10-22) | §4 build | ok |
| F16 | Every measurement fair, comprehensive and honest (10-25) | §2 Honest measurement | added |
| F17 | OpenCode Go has an API key; ACE uses it from its own config (10-25) | 9.2 | added |
| F18 | Bifrost core as the provider library, for broad provider support (10-25) | 9.2 | added |
| F19 | Honest measurement binds the builder and ACE itself (10-25) | §2 Honest measurement | added |
| F20 | Keep the current mission until a v0.12/v0.13 reset; first-time setup is an interview that recommends a context-calibrated mission, owner-editable (10-25) | v0.12 reset, Going | added |
| F21 | Voice and camera as the first embodiment: talk to ACE in the web UI via mic, speaker and webcam; Bark 3.0 speech-to-text; motion-gated person detection with confidence and time tuning; re-ID later (10-25) | v0.11 Voice and camera | added |
| F22 | Isolation, security, observability and compliance versions (10-25) | v0.13–v0.16 | added |
| F15 | Compressed timelines: acceptance runs are 1–2 h, or 4–6 h when the owner is away, never 24 h (10-25) | §4 build, v0.8 acceptance | added |
| F14 | One command runs ACE: `./ace` is the watchdog supervisor and runs the agent as its child, no separate watchdog command (10-25) | §3 watchdog, spec 8.5 | added |
| F13 | Builder works on the `ace-do` branch with no PRs (`main` only accepts PRs); only ACE uses the PR flow, and may keep one long-lived branch across versions (10-25) | §0 PR gate, §4 build | added |
| F12 | Build every version while ACE runs live and fix it as it runs, so each version is proven to work (10-25) | §4 live build | added |

## G. Models and cost

| # | Ask (date) | Plan | Status |
|---|---|---|---|
| G1 | Thinker vs fast verify/gate tiers; optimise for speed, then fall back (10-22) | 9.2 | added |
| G2 | Free fallback only from a free model; never fall back the main model to free (10-22) | 9.2 | added |
| G3 | Handle model churn; AA index as yardstick (10-22) | 9.2 | added |
| G4 | Optimise cost and turns against performance (10-23) | 8.7, 9.4 | fixed |
| G5 | Local 4-bit GPU gate (10-18) | 8.2 Tier 1 | ok |

## H. Generality and capability

| # | Ask (date) | Plan | Status |
|---|---|---|---|
| H1 | Any user installs it, gives only a goal, zero context (10-21, ticket). "I'm not convinced it's generalisable yet" (10-24) | 10, 8.0 CI grep | partial: proven only by the v0.10 fresh-VM test |
| H2 | No domain flows in core (10-21) | 8.0 CI grep | ok |
| H3 | L6 is the outside world, not just code (10-19) | 10 capability set | added |
| H4 | Serious, comprehensive web search; proactive research (10-22) | 10 capability set | added |
| H5 | Real goals: make money, social media, create accounts, launch products (10-18) | 10 capability set | ok (10-24: access, under spend cap and full-L1 guards) |
| H6 | Will become personal and research assistant (10-23) | 10 capability set | added |
| H7 | Builds its successors (10-23) | 10 self-improvement | ok |
| H8 | Running ACE must not update itself; separate improvement harness (10-20) | 8.4, 10 | ok |
| H9 | Competitor comparison against current versions, run by ACE (10-23, 10-24) | 10 | ok |
| H10 | Use the v0.7 research and its rewrite spec; don't lose it (10-24) | §3, 8.0 | fixed (rewrite spec copied to `docs/SPEC_ACE_V0_7_REWRITE.md` and assessed) |
| H11 | Use all eras' research and the original code (10-24) | §1, 9.2, 9.3 | ok |

## I. UI

| # | Ask (date) | Plan | Status |
|---|---|---|---|
| I1 | Visual, reactive dashboard: see thoughts, chat (10-17) | 11 | ok |
| I2 | Explainer of what each layer sees and decides, from our data (10-18) | 11 inspector | ok |
| I3 | Readable (10-18, 10-23) | 11 | ok |
| I4 | 3D interactive architecture visualisation (10-23) vs minimal readability-first (10-24) | 11 | ok: later wins, minimal; 3D dropped |
| I5 | Anatomy for Telegram explainers and the UI (10-22) vs "unreadable" (10-23) | §0, 8.0 | ok (10-24: anatomy removed; visual idea to v0.11) |

## J. Asks from the other Pi sessions

From the Pi harness sessions `2026-09-06T09-26` (S1) and `2026-10-18T09-31` (S5), plus ACE mentions in this session. Lytehouse, factory, LyteKnowledge, telemetry and Godot sessions are excluded: per the owner, they have nothing to do with ACE.

| # | Ask (date, session) | Plan | Status |
|---|---|---|---|
| J1 | Every owner directive tracked verbatim with status, kept across compaction, unmet ones reported (S5 10-18) | 8.4 directive ledger | added |
| J2 | A status or steering probe is answered at once; the loop keeps going (S1 09-08) | 8.4 | added |
| J3 | End each cycle with actions, measured results and uncertainties to the owner channel (S1 09-06) | 8.4 | added |
| J4 | Questions as fixed templates with options, trade-offs and a recommendation (S1 09-06) | 8.4 | added |
| J5 | Owner-set models and parameters are authoritative; ACE proposes changes with evidence (S1 09-06) | §2 | added |
| J6 | Halt acts in seconds, cancels in-flight calls and children, and reports what it cancelled (S1 09-06) | 8.5 | added |
| J7 | Detect "said it would act, then stopped" as a stall and resume (S1 09-06) | 8.5 | added |
| J8 | A plugin fault is caught, reported and the plugin disabled; it never kills the core; tool names are unique (S1 09-06, S5 10-18) | 9.1 | added |
| J9 | Adopt maintained components before building; build only for a recorded gap (S1 09-06) | §2 | added |
| J10 | Thinking only on the main model; side and gate calls run with it off (S5 10-18) | 9.2 | added |
| J11 | Subscriptions budgeted by quota windows (5h, weekly) as %; metered providers by cost (S1 09-06) | 9.2, 9.4 | added |
| J12 | Retry only classified transient errors, bounded, then surface the code (S1 09-06) | 9.2 | added |
| J13 | Size-tiered side models: small, medium, big (S1 09-06) | 9.2 | added |
| J14 | More providers: Google/Vertex and subscription OAuth, through ACE's own config (S1 09-06, S5 10-18) | 9.2 | added |
| J15 | Every call has a default timeout; long actions detach to a retrievable handle (S1 09-06, 09-07) | 9.4 | added |
| J16 | Tool output or pasted input over a cap goes to a file; the model gets head, size, path and "read by range"; never a silent clip (S1 09-06) | 9.5 | added |
| J17 | Deterministic compaction: keeps a raw tail, caps tool output, no LLM in the hot path (S1 09-06, 09-08) | 9.5 | added |
| J18 | Post-compaction recap: last N turns with time and stats, older turns bucketed (S5 10-18) | 9.5 | added |
| J19 | Every injected block (identity, memory, config or model change, compaction) is a visible event; control-plane changes injected in-band (S1 09-06) | 9.5, 11 | added |
| J20 | Deterministic deny list for destructive commands, no LLM in the path (S1 09-06; a Claude Code session deleted the owner's home dir) | 8.2 Tier 0; overrides by access request (8.4) | added |
| J21 | Liveness: every in-flight action shows state, elapsed time, sub-step and heartbeat age; thinking shows duration and tokens, not text (S5 10-18) | 11 | added |
| J22 | Pi features the owner valued: hashline edits (32-line cap), LSP, vetted DAP only, blackhole, background manager, monitors and schedules, quota % in the status bar, collapsed tool groups | 10, 11 | added as candidates |
| J23 | Explore uncertain areas, not only rank them (10-22) | 8.7 | added |
| J24 | Specs and plans never live in tmp; kept in the repo with their research (10-24) | 8.8 | added |
| J25 | Clean up in the same step that changes, adds, updates or rewrites (10-24) | 8.8 | fixed |
| J26 | Pi agent's 9 v0.7.1 root-cause bugs (forwarded 10-24) | §3, 8.0 | added |
| J27 | Where ACE runs (which machine) is undecided (10-24) | §0 | ok (10-24: owner's machine) |

## K. Contradictions between owner messages

The later message wins.

| # | Earlier | Later | Reading |
|---|---|---|---|
| K1 | "never short-circuit the layers" (10-21) | cheap gate passes simple decisions (10-24) | Tiered L1 (8.2); every side effect still gets a verdict |
| K2 | "close uncertainties and measure" (10-22) | "raise uncertainties with me" (10-24) | Measure first, raise what remains |
| K3 | 3D view (10-23) | minimal, readability first (10-24) | Minimal |
| K4 | v0.7 archived and paused (plan) | use its research, don't lose the spec (10-24) | Use it; spec kept in the repo |

## Not carried

- **Lytehouse, factory and Godot rules:** not ACE (owner, 10-24). This includes mining the owner's turns: ACE has no human turns to minimise.

- **TimesFM:** dropped by the owner (10-22).
- **SQLite concurrency:** the owner raised it (10-22); SQLite with no migration was decided later.
- **Mercury Decide for System 1:** folded into Tier-1 candidates (8.2).
