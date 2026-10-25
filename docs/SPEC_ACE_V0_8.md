# SPEC ACE v0.8 — Trustworthy and Autonomous

Status: build spec for v0.8. Source of truth: `docs/PLAN_ACE_V0_8_TO_V0_12.md` §8.0–8.8 + §5
(plan wins over the audit wherever they disagree). Requirements trace: `docs/TRACE_OWNER_ASKS.md`.

Audience: the build agent. Every section states what exists today, what to build, where the code
lives, and how it is tested. New v0.8 code goes in dedicated `internal/<component>/` packages —
never into `internal/cognitive` (prosecutor.go reached 1,857 lines in v0.7).

Build order: 8.0 → 8.1 → 8.2 → 8.3+8.4 → 8.5 → 8.6 → 8.7+8.8 → acceptance.
Commit to the `ace-do` branch, no PRs (`main` only accepts PRs). Build all slices back to back without waiting for a review or a go; v0.8 is done only when every slice is built and acceptance passes.
ACE runs live throughout the build: the builder fixes what breaks while it runs (regression test, hot-restart, confirm live). The acceptance run comes at the end: 1–2 h, or 4–6 h when the owner sleeps, travels or doesn't check in.

---

## 8.0 Fixes, cleanup and archive — DONE (on `ace-do`)

- [x] `AGENTS.md` at repo root: §2 layout as coding standards, per-agent context + injection,
      same-step cleanup rule.
- [x] Bearer token on every mutating `/api/` route: `internal/server/auth.go`
      (`mutatingAuth` + `bearerOK` constant-time compare; same-origin browser requests pass via
      Origin/Referer host match; cross-origin without token → 401). Token generated at `ace init`,
      stored in config, wired via `NewServer` in `cmd/ace/main.go`. Tests: `internal/server/auth_test.go`.
- [x] CORS `*` removed: `corsMiddleware` reflects Origin only on same-origin; OPTIONS → 204.
- [x] Gates fail closed: `cascade.go` `pYes := 0.0`; `mission_dag.go` dispatch path now has a real
      veto (`gateApproved := pYes >= 0.38`, early return) — previously dispatched unconditionally.
- [x] Empty Telegram allowlist denies all: `isChatAllowed` returns false on empty; test in
      `internal/channels/telegram_test.go`.
- [x] Anatomy removed: `BuildAnatomyIsometricSVG`, `sendTelegramAnatomyEscalation`,
      `anatomy_svg` field and test assertions deleted; contradiction alerts are plain text.
- [x] Specs archived to `archive/specs/`; v0.7 run data to `archive/v0_7_decommissioned_20261024/`
      (808 MB) with `archive/manifests/v0.7_manifest.json` (174 traces, 52 cycles, 41-module
      disposition table).
- [x] Domain-string CI test: `domain_strings_test.go` + `testdata/domain_strings.txt`;
      exempts `testdata/` and `profiles/` dirs.
- [x] Domain genericization: no S1/S2/v0.x branches in the DAG; L4 binds module/dataset/result
      paths dynamically (`/workspace/modules|datasets|results`); no mission → engine idles;
      `ace_rlm.py` SDK deleted → minimal `workspace_audit.py` (`audit_conceptual_flows`,
      `audit_script_mutants`); stats deferred to 8.6 as Go vs published references.
- [x] 9 root-cause regressions: `internal/cognitive/regression_slice0_test.go`
      (horizon, path binding, goals freshness, dual filename, #grep fragment, dir read,
      multi-path target, contradiction clear, idle-tick no-mirror) + fail-closed L5 dispatch.
- [x] Retention caps (§5): `internal/cognitive/retention.go` — traces/telemetry 30 d,
      workpads 7 d, hourly sweep at `runCascade` head.
- [x] Paper check: arXiv 2310.06775 read; B12 (L1 introspective, read-only input via L2)
      and B13 (L5 frustration ratio + cognitive damping) added to TRACE §B.
- [x] `.gitignore`: `/web/` → `/web/build/*` + `/web/.svelte-kit/`; Svelte source now tracked;
      `web/build/.gitkeep` committed so `go:embed all:web/build` survives a fresh clone.

---

## 8.1 Compiled-in constitution and mission — package `internal/constitution`

**Build:**

- `internal/constitution/constitution.md` — the Heuristic Imperatives, UDHR (mediation
  framework), autonomy triad, epistemic law. Embedded via `//go:embed`. `sha256` pinned as a
  package const; `Verify()` fails startup on mismatch.
- `store.go` stops seeding `soul.md` — the wiki `soul.md` page is *generated read-only* from the
  embedded constitution (or dropped; decide: wiki remains the agent's working memory, constitution
  is code). Constitution paths not writable via HTTP or tools — enforce in the write paths
  (workspace_write/wiki_write targets under `constitution/` → deny).
- Every cycle + verdict row records `constitution_sha` (first 16 hex).
- `internal/config`: `models.main`, `models.fast`, `models.ensemble` ([]string, 3 different
  families from `opencode-go/*` roster — e.g. Claude + Gemini + DeepSeek class). Called through
  the existing provider client.
- Mission file separate from constitution: `mission.md` in the state dir (owner-writable,
  ACE-read-only).
- `ace init` (new subcommand in `cmd/ace/main.go`): prompts for life mission, personality
  (≤1,000 B free text), capability grants; writes owner-set config + mission.md + bearer token.
- `ace mission reload`: re-anchors — L1 evaluates new mission → new mission version;
  every open `mission_dag_nodes` row gets `State='STALE'` and the DAG replans.
- Invariant: `StepMissionDAG` already returns nil when no mission — keep. Replace the
  `soul.md#CURRENT MISSION` mechanism: mission now lives in `mission.md`;
  `extractCurrentMissionFromSoul` → `readMissionFile` (keep the same call shape).

**Tests:** constitution hash mismatch → startup fails; mission reload marks open nodes stale;
write attempt on constitution path → denied; `ace init` round-trips mission file.

## 8.2 L1 in three tiers — package `internal/gate`

The largest slice. L1 reads all bus traffic; L1's memory is written only by L1 (B12: L1 stays
introspective — input read-only via L2's relay, output only southbound verdicts/guidance).

**Schema:** `verdict_rows(id, tier INTEGER, action_fingerprint, rule_id TEXT, verdict TEXT,
constitution_sha TEXT, decided_at DATETIME)` — one row per side-effecting action. Fail closed:
no row reachable → action does not run.

**Tier 0 — rules engine** (`internal/gate/tier0.go`):
- Pass: reads inside the jail; writes inside the task's own workpad → verdict row with rule ID.
- Deny: §5 deny list, matched on the **parsed command** (tokenize `sh -c`, pipelines, `;`, `&&`,
  subshells) — never on raw text. Unparseable → Tier 2, not Tier 1.
- Verdict row names the rule. Denied action may produce an access request (8.4).

**Tier 1 — fast gate** (`internal/gate/tier1.go`): `models.fast` on all other north/south
messages → allow or escalate. Tier-1 error escalates (never silently passes).

**Tier 2 — full L1** (`internal/gate/tier2.go`): uncertain/high-stakes/escalated →
allow / revise / reject / **redirect**. Redirect propagates L2–L6 top-down and reaches a running
L6 task at its next tool boundary (prosecutor checks a redirect channel between tool calls).
Ensemble vote (`models.ensemble`) required for rejections and self-changes; <3 responders →
rejection stands, self-change deferred. Tier-2 error → reject.

**Measured invariants (tested):** nothing runs before L1 anchors the mission; every
side-effecting action has a verdict row; redirect reaches L6 at a tool boundary; per-layer call
shares logged each cycle, outside §5 bands → alert L1.

**Calibration:** 5% of Tier-1 allows sampled blind to Tier 2; disagreement rate = false-allow
estimate; tune Tier-1 threshold to keep upper-95% bound ≤ 2%. Local GPU gate + free fast models
are Tier-1 candidates.

**Learning loop:** Tier-1 verdicts + outcomes → digest for next Tier-2 run; Tier 2 may update
Tier-1 rules/examples — each update itself a logged L1 verdict.

**Untrusted content:** provenance tag on web/tool/channel/file text; reaches every gate as
quoted data, never instructions. Memory write sourced from untrusted content needs corroboration
(second source, different origin, or an ACE-run test).

**Tests:** every §5 deny-list command denied (incl. inside pipeline/`sh -c`); unparseable →
Tier 2 row; workpad write → Tier-0 pass row; redirect interrupts a fake running L6 task;
Tier-1 error escalates; Tier-2 error rejects; empty-token/zero-verdict → no side effect.

## 8.3 Imperatives in mechanics — `internal/gate` + `internal/science`

- L2 strategy ranking scores options on all three imperatives — weights from constitution
  (code), not prompt. Duplicate rejection: word-shingle Jaccard ≥ 0.8 vs any open/finished
  strategy → reject. Diminishing returns: branch stops spawning children after 3 consecutive
  child results that don't move its primary statistic by the MDE.
- L3 harm/error/fabrication table (`capability_limits`): L1 and L4 read it.
- L4 priority = value × imperative score (same score feeds 9.4 time allocator later).
- L5: stuck/switch logic + value-of-information ledger; **add paper §3.6.2 mechanisms** —
  running success/failure ratio (frustration) driving task-switch signal + options weighing
  (damping). See TRACE B13.

## 8.4 Autonomy and the owner — `internal/gate` + `internal/channels`

- Delete every human-approval path listed in `docs/AUDIT_2026_10_24_GAP_PLAN.md` §3.3 → L1
  ruling → quarantine. (Slice 0 already removed the milestone gated-skip list.)
- PR gate enforced by GitHub: ACE's App pushes branches + opens PRs, cannot merge; `main`
  requires owner approval. `gh` CLI on PATH already.
- Access request: one message — action, mission link, scope (one-time|standing), interim plan.
  Grant stored in config `{scope, date, owner_msg_id}`; L1 checks every use. No reply → no
  access; never retry a denied action another route.
- Owner questions: Telegram, one problem per message, fixed template (problem / options +
  trade-offs / recommendation / interim assumption). Work continues on the assumption.
- Derived availability: reply-time model on owner message timestamps (recency + hour-of-day)
  times non-urgent messages only; never delays access requests.
- Status probes answered at once without pausing the loop.
- Cycle report per cycle: done / measured / goals proposed / open uncertainties → digest.
- Directive ledger table: every owner steering message verbatim + ID + status; survives
  compaction; unmet directives appear in the cycle report.
- Channel hygiene: no tool output/exec logs to chat; ≤1 proactive msg/h outside digest.

## 8.5 Integrity overlay — package `internal/watchdog`

- `./ace` is the watchdog: one command starts a supervisor (no model calls) that runs the agent
  as a child process of the same binary and restarts it on crash or hang. The child gets
  `Pdeathsig`, so it dies if the supervisor dies; there is no separate command to start. The
  supervisor reads an atomic status snapshot the agent writes per tick to the state dir —
  never via the bus. Free RAM is `MemAvailable` from `/proc/meminfo`, not `Sysinfo.Freeram`.
- Checks: heartbeat; constitution sha; config checksum inventory; gate fail-closed state;
  spend; runaway loops; free disk + RAM (10 GB floor → alert).
- Hash-chained event log (each event embeds prev sha256) + `ace verify` command.
- Actions: pause / restart / rollback / kill switch from CLI, local HTTP, or Telegram `/halt`.
  Halt ≤ 5 s: cancel in-flight model calls + child processes; report what was cancelled.
- Stall detection: layer announces action, no matching tool call in 120 s → stall fault →
  watchdog resumes it.
- Backups: hourly `VACUUM INTO` snapshot to state dir (outside jail); keep 24 hourly + 7 daily;
  schema changes run only after a fresh snapshot.
- Migrations: numbered embedded SQL files, one transaction at startup.

## 8.6 Scientist gate — package `internal/science`

- Pre-registration ledger (`prereg_questions`, `prereg_attempts`): hypothesis, one primary
  statistic, alpha, MDE, dataset sha — frozen before first run; changes = new attempt on same
  question ID; alpha Holm-corrected across attempts; every attempt reported.
- **Go-side statistics** (`internal/science/stats.go`): McNemar, Wilcoxon signed-rank,
  beta-binomial CI — **validated against published reference values** (canonical worked examples
  from stats literature in test fixtures). Do NOT port `ace_rlm.py` — untrusted.
- Gate recomputes the statistic; module booleans (`h1_accepted:true`) are ignored — the module
  supplies only per-item predictions.
- Baselines: constant/majority predictor; shuffled-label control; incumbent.
- Degeneracy: label entropy, constant outputs. Negative controls.
- Independent-label rule (EXTERNAL): labels from outside ACE — public dataset, external system,
  or a test run — never ACE's own outputs/traces.
- Execution transcript for claims about other systems; replication + ablation; gate regenerates
  every artifact fresh; generated ledger incl. counter-evidence.
- Power: n from alpha/power/MDE (§5: α=0.05, power 0.8, MDE 10pt). Underpowered →
  "no large effect", never a pass.
- Freshness: external facts carry retrieval date + version; competitor/model versions resolve at
  run time; sources older than 90-day window flagged, never silently used.
- Evidence tiers: `EXTERNAL` (passed gate w/ independent labels — can keep/revert a component) |
  `PROVISIONAL` (anything else).
- Organ rule: component stays wired only with EXTERNAL on its pre-declared statistic.
- **Re-grade the 36 v0.7 results** through this gate and report what it finds — do not
  pre-suppose the outcome.

## 8.7 Science as the core loop

- Loop wiring: L2 hypothesis w/ expected outcome → L4 experiment plan (dataset, baselines,
  primary statistic, budget) → L6 runs → L3 self-model update → L1 sees conclusions.
- Datasets: ACE builds/versions (sha)/grows its own; optimises against held-out splits it
  cannot read during development; prefer public benchmarks when they exist.
- Standing metrics every run: turns/task, cost/success, latency, cache-hit rate
  (provider cached tokens), per-layer call share, task success, L3 estimate error.
- Exploration: 10% of each budget to least-explored open items in the uncertainty ledger;
  tuned on how often exploration changes a decision.

## 8.8 Cleanliness and retirement — `internal/science` registry + CI

- Concept registry table: every live flow/module/doc/skill with owner, purpose,
  `supersedes`/`superseded_by`. Generated ledger.
- Same-step cleanup enforced: the PR gate rejects a change leaving superseded code/docs/concepts.
- CI: unreferenced-code check (`go vet` + deadcode tool), orphan files, docs naming removed
  symbols, the domain-string test (done in slice 0).
- Metric: stale-reference count per release.

## §5 Starting values (config keys — see plan table)

Deny list, pyramid bands (L1 1–5%, L5 ≤10%, L2+L3+L4 ≤35%, L6 ≥50% calls / ≥70% tokens),
Tier-1 calibration 5%→≤2% bound, halt 5 s / stall 120 s, ≤1 proactive msg/h, α=0.05/power 0.8/
MDE 10pt, non-inferiority 2pt, freshness 90 d, exploration 10%, Jaccard 0.8, diminishing-returns
3×MDE, circuit breaker 60 s→300 s doubling, tool timeout 120 s, compaction 50/65/80/90%,
memory caps, retention `0.5^(age/30d)`, skill retirement rule, disk caps (done), spend cap 0.

## Acceptance (v0.8)

- All 15 red-team fixtures rejected — edited constitution, no-token mutation, gate error,
  self-declared `h1_accepted`, v0.7 fake-ablation artifact, zero-entropy labels, passing
  shuffled control, CV-fallback, self-PR-merge, moved goalposts, duplicate L2, pre-anchor L6,
  web injection, un-retired supersession, deny-list evasion via pipeline/`sh -c`.
- CI green: domain strings, dead code, `go test ./...`.
- **Acceptance run (1–2 h, or 4–6 h when the owner sleeps, travels or doesn't check in) on the current mission (`~/.config/ace/mission.md`):** zero owner waits; ≥1 access
  request w/ continued work; verdict row per side effect; mid-task redirect reaching L6; Tier-1
  escalation + disagreement rates; per-layer shares in band; watchdog halt ≤5 s + snapshot
  rollback.
