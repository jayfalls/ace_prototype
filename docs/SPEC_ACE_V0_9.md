# SPEC ACE v0.9 — Composable Core, Context and Time

Status: build spec for v0.9. Source of truth: `docs/PLAN_ACE_V0_8_TO_V0_12.md` §9.1–9.6, v0.9
acceptance, §2, §4 and §5 (the plan wins wherever they disagree).

Audience: the build agent. Each slice says what exists today (file:line on `ace-do` at
`e2d958ae`), what to build, where it lives, its config keys, the tests that must fail on broken
code, and the check in the live run. New v0.9 code goes in dedicated `internal/<component>/`
packages, never into `internal/cognitive` (prosecutor.go is 1,946 lines, engine.go 867,
mission_dag.go 877). Code moved out of `internal/cognitive` is deleted there in the same commit.

Build order: 9.0 → 9.2 → 9.1 → 9.3 → 9.4 → 9.5 → 9.6 → acceptance. 9.2 goes before 9.1
because the Provider port is the first port and the plugin registry wraps it.
Commit to `ace-do` with no PRs (`main` only accepts PRs). Build every slice back to back
without waiting for a review or a go. Each pillar ships behind a flag (`features.<name>`,
default on) so it can be ablated.
ACE runs live on the current mission (`~/.config/ace/mission.md`) throughout. The builder fixes
what breaks (regression test, hot-restart, confirm live). A stall (ACE up, no model calls) is a
bug. Acceptance run: 1–2 h, or 4–6 h when the owner sleeps, travels or doesn't check in.

**Primary metric (pre-registered, §4):** L6 turns per completed task, non-inferior on task
success rate (margin 2 pt absolute). Computed by `internal/science/metrics.go:77-115`
(`l6_turns_per_completed_task_{median,p90,n,ci95_lo,ci95_hi}`, era since first gate trace,
partially observed tasks dropped). Baseline = the v0.8 acceptance run, frozen in 9.0. MDE 10 pt
on the success proportion; α 0.05, power 0.8.

**Honest-measurement rule (§2, binding on the builder's reports and on ACE's own gates,
metrics and experiments):**
- Fair: both arms share mission, window length, logging and metric definition. Any change to
  logging or a metric definition resets the baseline, and the reset is recorded.
- Comprehensive: every arm, attempt and failure is counted. Results are distributions with n
  and a 95% CI, never single ratios.
- Honest: a metric is named for exactly what it computes. No window or subset is chosen after
  seeing the data.
- Enforced in code: `standing_metrics` rows carry `definition_sha` (sha256 of the SQL plus the
  Go formula source string) and `baseline_id`. A comparison across different `definition_sha`
  values errors instead of returning a number. Test: two rows with differing `definition_sha`
  → `science.Compare` returns an error.

---

## 9.0 v0.8-run leftovers — packages `internal/config`, `internal/plan`, `internal/science`

**Credentials from ACE's own config (plan 9.2 "Providers", §6):**
- Today: `internal/llm/opencode_free.go:36-69` `resolveOpenCodeCredential` reads env vars, then
  falls back to another app's credential (`~/.local/share/opencode/opencode.db`, `credential`
  table). No `.env` loader exists. The owner has written `~/.config/ace/.env` (mode 0600,
  key `ACE_LLM_API_KEY`).
- Build `internal/config/env.go`: `LoadEnv() error`. It reads `filepath.Join(Dir(), ".env")`
  with `KEY=VALUE` lines, `#` comments and optional quotes. It is called first in `main()`
  (`cmd/ace/main.go:38`, before the `ACE_WORKER` branch at :48), so the supervisor and the worker
  both get it and the child inherits it via `os.Environ()` (:340).
  - A missing file is not an error.
  - If `mode & 0o077 != 0`, startup fails with `"<path> must be mode 0600"`.
  - Variables already set in the process env win. Values are never logged; only the key names
    are logged, at startup.
- Delete the `opencode.db` branch (`opencode_free.go:44-68`) and the `database/sql` import. With
  no key, the provider uses `"public"` (free tier only), as it does today when the db is absent.
- Tests (`internal/config/env_test.go`, `internal/llm/opencode_free_test.go`):
  - a 0644 `.env` → `LoadEnv` errors;
  - a 0600 file sets an unset key and does not override a set key;
  - with `HOME` pointed at a temp dir containing a populated `opencode.db` and no env key,
    `resolveOpenCodeCredential()` returns `"public"`.
- Live check: the worker's `/proc/<pid>/environ` holds `ACE_LLM_API_KEY` (check the name only).
  Model calls succeed on `deepseek-v4.1-flash`. `opencode.db` is never opened
  (`strace -e openat` or `lsof` on the worker shows no path containing `opencode.db`).

**Duplicate L2 strategies:**
- Today: `mission_dag.go:546-559` rejects a new strategy at word-shingle Jaccard ≥ 0.8. In the
  live DAG, S15, S17 and S19 are the same strategy ("widen the C_S13 corpus") in different words,
  and all three passed.
- Build `internal/plan/dedupe.go`: `DuplicateOf(dir string, existing []Node) (id string, ok bool)`.
  A strategy is a duplicate when either holds:
  - shingle Jaccard ≥ `plan.duplicate_jaccard` (0.8, §5); or
  - its set of workspace artefact names (identifiers matching `[a-z0-9_]+\.(py|json)` or
    `c_s\d+_[a-z]+`; the regex lives in profile config, not Go) overlaps an existing open or
    finished strategy's set at Jaccard ≥ `plan.duplicate_artifact_jaccard` (0.5).
- `mission_dag.go:546-559` calls it. The veto row stays as written (`veto_kind='duplicate_strategy'`).
- Before setting the 0.5 default, the builder computes the full pairwise matrix on the 20 live
  L2 rows. Report the true positives (S15/S17/S19) and the false positives against all other
  pairs. Commit the matrix as `docs/evidence/v0_9_dedupe_matrix.md`. The default ships only if
  the matrix has zero false positives.
- Test `internal/plan/dedupe_test.go`: fixtures copied from the S15/S17/S19 titles and
  directives → flagged. S1 vs S2 → not flagged. Reverting the `||` artefact branch makes the
  test fail.

**30 STALE DAG nodes:**
- Today: one mission re-anchor at 2026-10-25T11:01Z (`markStaleOnMissionReload`,
  `mission_dag.go:84-96`) left 30 STALE nodes: 2 L2 (S13, S14) and 28 L3. 24 of the L3s sit
  under COMPLETED strategies. Nothing ever leaves STALE.
- Build `internal/plan/stale.go`: `SweepStale(ctx, db, rootSoulHash)`. It runs on each
  `StepMissionDAG` wave:
  - a STALE node whose L2 branch is COMPLETED, or whose branch has a newer strategy that
    `DuplicateOf` matches, → `state='RETIRED'` with `retired_reason` (new column) and a
    concept-registry row (8.8);
  - any other STALE node is re-anchored once: its `soul_hash` is set to the root's and its
    state reset to the stage's entry state, with an L1 Tier-1 verdict row;
  - a second STALE on the same node retires it.
- Test: a seeded DAG holds one STALE L3 under a COMPLETED L2 and one under an open L2 →
  RETIRED and re-anchored respectively. Running the sweep twice leaves the result unchanged.
- Live check: `SELECT COUNT(*) FROM mission_dag_nodes WHERE state='STALE'` is 0 after one wave.
  The RETIRED count is 30 minus the re-anchored count, and both numbers go in the 9.0 commit
  message.

**Baseline freeze:**
- `ace science baseline freeze --name v0.8` (new `runControl` sub in `cmd/ace/main.go:413`)
  copies the v0.8 acceptance window's `standing_metrics` rows into `baselines(id, name,
  window_start, window_end, definition_sha, metric, value, n, ci_lo, ci_hi)`. The window is the
  acceptance run's start and end as recorded in its tag message, never chosen afterwards.
- Pyramid shares are reported two ways: by calls and by provider-reported tokens. Gate calls
  (Tier 1/2, logged as `layer_code='L1'` since 2026-10-25 12:30) are reported as `L1_gate`
  alongside `L1` deliberation. The bands stay as §5 states; see open question Q1.
- Test: freezing twice under the same name errors. A baseline with a different
  `definition_sha` than the live metric errors when compared.

## 9.2 Provider registry — package `internal/provider`

Today: one concrete provider, `llm.OpenCodeFreeProvider` (`internal/llm/opencode_free.go`,
1,160 lines), built at `cmd/ace/main.go:107`. Its pieces:
- tier isolation in `FallbackChain` (:485-524);
- a per-model cooldown in `recordModelOutcome` (:526);
- hard-coded timeouts: client 190 s (:293); per call 75/150/60 s (:968-975);
- the "opencode hook": impersonating the opencode CLI with `openCodeUserAgent`,
  `openCodeGatePreamble`, `stubToolNames`/`buildStubTools` (:30-31, :71-120), and session IDs
  persisted through `sessionPersistable` (`engine.go:207-250`);
- the endpoint const `https://opencode.ai/inference/go/openai/v1/chat/completions` (:30).

The port `core.LLMProvider` (`internal/core/ports.go:52-58`) stays the contract.

**Bifrost adoption (gated measurement, do this first):**
- Candidate: `github.com/maximhq/bifrost/core` (Apache-2.0), used as a library only. No gateway,
  no UI, no Bifrost plugins.
- On a scratch branch, add it and build `cmd/ace`. Record in `docs/evidence/v0_9_bifrost.md`:
  - binary size vs 19,600,772 B;
  - `go list -m all | wc -l` vs the current count (the plan says 33; re-measure on `e2d958ae`
    first and use that number);
  - the `go` directive and toolchain bump it forces (Bifrost requires go ≥ 1.27.3; the host has
    1.27.1 and `go.mod` says `go 1.26`);
  - a clean `go build ./... && go test ./...`;
  - one live streaming call to OpenCode Go through Bifrost's native provider using
    `ACE_LLM_API_KEY`, with provider-reported prompt, cached and completion tokens.
- Adopt if the build is clean and the live call returns provider-reported usage. Size and
  module growth are reported, not gated. If it fails, `internal/provider/openai_compat.go` (the
  existing streaming client moved out of `internal/llm`) stays the only adapter, and the reason
  is recorded.

**Build** (`internal/provider/`, each file ≤ 500 lines):
- `registry.go`: `Registry` implements `core.LLMProvider` and routes by role.
  - `Register(a Adapter)`, where `Adapter` is
    `{ Name() string; Stream(ctx, Request) (Response, error) }`, implemented by `bifrost.go`
    or `openai_compat.go`.
  - `Route(role Role) []ModelRef` returns the ordered fallback list for a role.
- `tiers.go`: roles `main`, `fast`, plus per-layer roles from config:
  - L1–L2 `reasoning`, L3–L4 `general`, L5 `efficient`, L6 `tool`;
  - temperature 0.2 for the logic roles, 0.7 for creative.
  - Rules moved from `FallbackChain`: main never falls back to a free model (the list for main
    is `[main]`, used until its quota is spent); free falls back only to free; `fast` is ordered
    by measured p50 latency.
  - `isFreeTierModelID` (:480) becomes a catalogue field `price.input == 0 && price.output == 0`,
    not a suffix match.
- `breaker.go`: per provider group and per model. Cooldown 60 s, doubling, max 300 s (§5).
  Open → skip to the next group in declared priority.
- `errors.go`: `Classify(err, status) Class` with classes `transient` (5xx, 408, 429 without a
  quota body, network), `quota` (429 with a quota/daily body), `auth` (401/403), `invalid`
  (400/404/422), `timeout`.
  - Only `transient` and `timeout` retry: full-jitter backoff, base 1 s, cap 8 s, 2 retries,
    then the next group.
  - On exhaustion the error carries class, HTTP code and the provider's reason string.
- Timeouts: `provider.timeout.cloud` 60 s, `provider.timeout.local` 300 s. These replace
  :293 and :968-975.
- `catalogue.go`: `models_catalogue` table with `(id, provider, source_url, price_in,
  price_out, price_cached, context, cutoff, aa_index, removed_at)`. A change in the catalogue
  (add, remove, price change) emits `provider.churn` on the bus. A subscriber re-runs Tier-1
  calibration (`internal/gate`) and the standing benchmark before the model takes a role. A
  removed model is re-routed by tier.
- `usage.go`: writes `usage_events(id, call_id, layer, role, model, provider, billing_path,
  prompt_tokens, cached_tokens, completion_tokens, retries, duration_ms, cost, created_at)`.
  - Tokens come only from provider-reported usage. A response without usage stores NULL, never
    an estimate.
  - Cost = in × price_in + out × price_out + cached × price_cached.
  - `billing_path` ∈ `subscription|metered|free`.
- `quota.go`: subscription windows (`5h`, `weekly`) as % used, from `usage_events` against
  `provider.quota.<id>.<window>` limits. Metered providers report cost against the spend cap
  (§5, 0 until set).
- Thinking is on only for role `main`. Gate, router and side calls send thinking off.
- Side-model sizes: `small|medium|big` per role in config; the task's declared size picks one.
- `internal/crypto/field.go`: AES-256-GCM per field, ported from
  `backend/internal/crypto/field_encryption.go` at `ac7d68c9`. Stored keys are encrypted, and
  the API and UI show only the last 4 characters. The master key comes from `.env`
  (`ACE_FIELD_KEY`), never from the db.
- Delete the opencode hook (user agent, preamble, stub tools, session-ID impersonation,
  `sessionPersistable` in `engine.go:207-250`). It leaves the repo for the owner's personal
  extension (plan 9.1). Keep `EvaluateBinaryGate` and `ReadoutD1Logprobs` (:742-870) by moving
  them to `internal/gate`. Delete `internal/llm` when it is empty.

**Config keys:** `models.main`, `models.fast[]`, `models.roles.{l1..l6}`,
`models.temperature.{logic,creative}`, `provider.groups[]` (name, adapter, priority, base_url,
key_env), `provider.timeout.{cloud,local}`, `provider.breaker.{cooldown,max}`,
`provider.retry.{base,cap,max}`, `provider.quota.*`, `features.provider_registry`.

**Tests** (`internal/provider/*_test.go`, with an `httptest` server as the fake provider):
- main model returns 429 quota → error surfaced; a free model is never called (assert the fake's
  request log);
- free model fails → the next free model is called, never a paid one;
- 3 × 503 → breaker opens, the next group in priority is used, and the cooldown doubles to
  120 s on the second trip;
- 400 → no retry;
- a response without usage → NULL tokens in `usage_events`;
- removing a catalogue row → `provider.churn` emitted and the role re-routed;
- field crypto round-trips, and a tampered ciphertext fails to decrypt.

**Live check:** ACE runs through `Registry`. `usage_events` has rows for every layer with
non-NULL tokens and `billing_path='subscription'` for main. A fault injected with
`provider.groups[0].base_url` pointed at a dead port trips the breaker and fails over within one
call, visible in `usage_events.retries` and the bus.

## 9.1 Plugin core — package `internal/plugin`

Today: no registry. Senses are hard-wired in `internal/cognitive/sensorium.go` (685 lines).
Tools are a `switch` in `prosecutor.go:1340` `dispatchLayerAction` (e.g. `web_search` :1640),
with HTTP clients built inline in `web_tools.go:76,480`, `sensorium.go:680` and
`launch_tools.go:24-45`. Telegram is constructed directly in `cmd/ace/main.go:113`. Ports in
`internal/core/ports.go` cover Provider, Bus, Memory, Sandbox, Telemetry and Sense. The name
`cordis.go` in `internal/cognitive` is a file-snapshot fiber, unrelated to the Cordis plugin
model.

**Build:**
- `internal/core/ports.go` adds `Tool`, `Channel`, `Gate` and `ContextProvider` ports.
  - `Tool{ Name(); Schema() json.RawMessage; ReadOnly() bool; Call(ctx, args) (Result, error) }`
  - `Channel{ Name(); Start(ctx, Bus) error; Send(ctx, Msg) error }`
  - Gate wraps `internal/gate`.
  - `ContextProvider{ Name(); Inputs(layer) []Block }`
- `manifest.go`: `plugin.toml` holds `name`, `version`, `kind` (port), `provides[]`,
  `depends[]` (dependency keys), `egress[]` (hosts), `secrets[]` (env names) and `entry`
  (built-in id or `mcp:<command>`). It is parsed without running plugin code.
- `registry.go`: `Load(dirs...)` builds a dependency graph and rejects cycles at load, naming
  the cycle. A plugin can call only its declared `depends`. An undeclared lookup returns
  `ErrUndeclared` and logs it.
- `isolate.go`: every port call is wrapped in `defer recover()`. A failure to load, a tool-name
  clash or a panic at runtime → the plugin is disabled, reported on the bus
  (`kind=plugin_disabled`, with a `summary`) and the core loop continues.
- `mcp.go`: an MCP stdio client (`initialize`, `tools/list`, `tools/call`). Each MCP tool is
  registered as a `Tool` from its manifest. Adopt a maintained Go MCP SDK first (§2 "adopt
  before build"); record the choice and its size in the commit.
- `hooks.go`: at most 8 hooks: `pre_tool`, `post_tool`, `pre_model`, `post_model`,
  `on_bus`, `on_compact`, `on_task_done`, `on_start`.
- `egress.go`: one `http.Client` factory with a per-mission host allowlist
  (`egress.allow[]`). An off-list host fails with `ErrEgressDenied` before dialling.
  - Secrets are brokered: a plugin declares `secrets[]` and the broker injects the header. The
    tool never receives the raw value.
  - Every inline client listed above, and the provider adapters, go through it. Bifrost takes an
    injected `http.Client` or transport; if it can't, record that as a gap.
- The action list given to models is generated from the registry, replacing the hand-written
  list in `prompts.go`.
- Moves (deleting the originals in the same commit):
  - Telegram → `internal/channels` implementing `Channel`;
  - web, search and launch tools → `internal/tools/` as built-in plugins;
  - senses → `internal/senses/` as built-in Sense plugins, internal (CPU, mem, disk, net, LLM
    usage, quota) and external (web, channels, files). Each layer gets a configured sense mix
    (`senses.layer.<n>[]`).
- Skills: `SKILL.md` loader with precedence workspace > user > global. Third-party skills are
  scanned at install (deny-list commands, egress hosts) and quarantined until they pass the
  gate. The lifecycle is in 9.6.
- Profiles: domain content (prompts' domain phrases, artefact regexes, dataset names) lives in
  `profiles/<name>/`. `domain_strings_test.go` keeps passing.
- Invariant: every model request is reconstructable from the event log. `pre_model` writes
  `model_requests(call_id, layer, blocks_json_sha, injected_block_ids[])`, and a test rebuilds
  the request from the log byte for byte.

**Config keys:** `plugins.dirs[]`, `plugins.disabled[]`, `egress.allow[]`, `senses.layer.<n>[]`,
`skills.dirs.{workspace,user,global}`, `features.plugins`.

**Tests:**
- a manifest with a dependency cycle → load error naming the cycle;
- an undeclared dependency lookup → `ErrUndeclared`;
- a tool that panics → disabled, and the next tick runs;
- a duplicate tool name → the second is disabled;
- an MCP fixture server (a Go test binary) exposes `echo` → callable with no Go change to ACE;
- a request to a host off the allowlist → `ErrEgressDenied`, and the fake server sees no
  connection;
- a tool requesting a secret receives a header, and its args/env never contain the value;
- an event-log rebuild of a model request equals the sent bytes.

**Live check:** `GET /api/plugins` lists every plugin with its state. One MCP tool added by
manifest only is called by L6 in the live run. `prosecutor.go` has lost the tool switch, and
`wc -l` shows the reduction.

## 9.3 Layers, bus and cadence — packages `internal/bus`, `internal/layers`

Today:
- `MemoryBus.Publish` (`internal/bus/memory_bus.go:55-90`) accepts any envelope.
- `BusEnvelope` (`internal/core/types.go:114-126`) has `Summary` but it is not required, and
  has no `TaskID`.
- Topics are free-form.
- The cascade polls every 2 s (`cascade.go:17-18` `autonomousTickerLoop`) and runs layers in a
  fixed sequence (`runLegacySequentialCascade` :219, `executeParallelL1L2L3` :450).

**Build:**
- `core.BusEnvelope` adds `TaskID string`.
- `internal/bus/validate.go`: `Validate(env) error`, called first in `Publish`. It rejects:
  - empty `Summary`;
  - a topic not matching `^ace\.engine\.[a-z0-9_-]+\.layer\.[1-6]\.(input|output)$`;
  - `|FromLayer-ToLayer| != 1` (L1 reads everything via `Subscribe`, but publishes only to L2);
  - empty `AgentID` or `CycleID`.
  A rejected publish returns an error and writes `bus_rejects`.
- `internal/layers/contracts.go`: one JSON Schema per layer (embedded), validated on each layer
  output:
  - L2: beliefs `{claim, p, source, credibility}`, reconciliations, strategies as hypotheses,
    a belief summary;
  - L3: a self-model `{capabilities, limits, config, state, measured_capacity}`;
  - L4: plan steps `{id, deps[], checkpoint, success_criteria}`, risks `{risk, contingency}`,
    inventory;
  - L5: `done_criteria` per task, `dwell_s`, `thrash`, status `{task, pct}`;
  - L6: `expected_feedback` and thresholds declared before acting.
  An invalid output is retried up to 3 times with a prefilled response opening, then escalates.
- `internal/layers/scheduler.go` replaces `autonomousTickerLoop`. Each layer has a mailbox. A
  layer runs when it holds both guidance from above and data from below:
  - L1 runs on events, plus a slow review (`layers.l1.review_interval`, 15 m);
  - the L2/L3/L4 cluster runs per `mission_dag_nodes` node whose parents are done;
  - L5 runs per tick of its own clock;
  - L6 runs continuously.
  Every layer can escalate. Delete `cascade.go:17-95` and the sequential path once the
  scheduler passes the live check.
- Redirects: L6 checks the redirect channel between tool calls (existing 8.2 mechanism),
  unchanged.
- `internal/layers/tools_exec.go`: tools with `ReadOnly()` run in parallel (cap
  `layers.parallel_reads`, 4). Write tools run serially, and two writes whose paths overlap are
  never in flight together.
- Hot reload: the sectioned config (`config.d/*.toml`) is watched, and a change emits an
  in-band notice (9.5 injection log).
- Each layer's sensor allowlist (`senses.layer.<n>`) is enforced by the registry.
- L5 status gains a minimum dwell time (`layers.l5.min_dwell`, 60 s) and a thrash metric
  (switches per hour).

**Tests:**
- L2→L5 publish → rejected;
- empty summary → rejected;
- a malformed topic → rejected;
- an L4 output missing `success_criteria` → 3 retries, then escalation;
- with guidance but no data, the layer does not run; when data arrives, it runs;
- two writes to overlapping paths are serialised (assert the start/end order);
- 3 read-only tools overlap in time.

**Live check:**
- `bus_rejects` count by reason is reported;
- 0 calls from the 2 s ticker (the ticker is deleted);
- idle CPU with no work is under 1%;
- L6 turns per completed task (the primary metric) is tracked against the frozen baseline.

## 9.4 Time management — package `internal/tasks`

Today: no durable tasks, schedules or checkpoints. Exec has a hard 25 s timeout
(`internal/sandbox/jail.go:22,188`). Budget is not tracked.

**Build:**
- Tables: `runs`, `tasks(id, kind, parent_id, layer, state, deadline, idempotency_key UNIQUE,
  progress, heartbeat_at, created_at)`, `schedules(id, name UNIQUE, rrule, next_due, last_run_id,
  enabled)`, `checkpoints(task_id, seq, blob, created_at)`. `usage_events` comes from 9.2.
- `manager.go`: one shared `Manager` for every layer: `Submit`, `Cancel`, `Poll`, `Resume`.
- The primitives L5 executes:
  - `schedule.go`: `next_due` advances only after a run row reaches `done`. Deduped by
    `(name, due)`.
  - `heartbeat.go`: skips and logs the skip reason when there is nothing to do.
  - `monitor.go`: a Sense plugin with a condition that fires an event.
  - `deadline.go`: L4 is alerted when projected finish > deadline.
  - `job.go`: long jobs with a progress heartbeat, cancel and checkpoint/resume. A side effect
    runs only if its `idempotency_key` has no `done` row.
  - `child.go`: depth 1. The tool blocklist (`delegate`, `memory_write`, `send_message`) is
    enforced at the registry, not by the prompt. Each child has a per-provider concurrency cap,
    a timeout and a parent kill switch.
- Tool timeout 120 s (§5, `tasks.tool_timeout`). It replaces the 25 s `maxExecTimeout`. Past
  the timeout, the call detaches to a background handle the layer can poll, read or cancel.
- Time in prompts: every layer prompt gets the current time, the time since the last owner
  contact, and derived availability (measured from owner message timestamps, never a fixed
  window).
- `budget.go`, the budget governor, works from the 9.2 quota windows and burn rate. It
  allocates per layer and per branch, with L6 getting the bulk. When projected burn exceeds the
  remaining window, non-L6 cadence is scaled by remaining ÷ projected (§5). It measures quota
  utilisation and cost per successful task.
- Ownership:
  - L2 writes commitments (`schedules` rows);
  - L3 learns duration and cost distributions per task kind from `usage_events` and reports
    estimate error;
  - L4 owns the time plan (deadlines, schedules, conflicts by value × imperative score);
  - L5 owns dispatch, preemption, retries and resume.
- `internal/plan/assumptions.go`: each plan node stores `assumptions[]` (claim ids). A new fact
  touching a claim marks only that node's subtree STALE, and 9.0's `SweepStale` replans only
  that subtree. Conflicts route as follows: facts → the uncertainty ledger; priorities → L4;
  values → L1 Tier 2; unresolvable → an owner question.
- Measured: replan rate, estimate error, deadline hit rate, and stale-wait time.

**Config keys:** `tasks.tool_timeout`, `tasks.child.{max_depth,blocklist[],concurrency,timeout}`,
`tasks.heartbeat_interval`, `budget.slowdown`, `features.tasks`.

**Tests:**
- kill the process mid-job (a test helper `os.Exit`s between side effects) → on resume the job
  continues from the last checkpoint, and the side-effect counter equals the planned count, with
  no duplicates;
- a weekly schedule with a fake clock fires once per week across a restart, and `next_due` does
  not advance when the run fails;
- a monitor fires after a restart;
- a child calling `send_message` → refused;
- a planted fact contradicting an assumption of node N marks only N's subtree STALE, and
  siblings keep their state;
- a tool running past 120 s returns a handle, not an error.

**Live check:** one real schedule (e.g. an hourly self-stats digest) fires on time across the
live run's restarts. `usage_events` drives a visible quota % in `/api/status`. A forced worker
kill mid-job resumes with no duplicate side effects in the event log.

## 9.5 Context system — package `internal/contextsys`

Today, context assembly lives in `internal/cognitive/context_build.go` (408 lines):
- hot files (:305);
- inventory (:121);
- bus context (:256);
- skills index (:395);
- the RRF slice in `connectome.go` (`SliceBudgetMaxBytes` :72, k=60).

The turn-0.5 router is in `connectome.go:618-660`. No compaction, recap or injection log exists.

**Build:**
- `stages.go`:
  - Turn 0 is deterministic injection: map regions of interest, memory, focused spec, task,
    files, self-stats.
  - Turn 0.5 is the router. It is off by default (`context.turn05`, false) and moves from
    `connectome.go:618`.
  - Turn 1 is the worker's own tools.
  Each stage is a composable with byte-budgeted sub-buckets.
- `map.go`: the problem-space map (files, docs, people, sources, open questions, specs,
  datasets) as nodes and edges in SQLite.
  - A deterministic tree search from the region of interest covers every downstream branch,
    and its frontier feeds turn 0.
  - The uncertainty ledger and the living specs (9.6) are nodes.
  - Each layer has its own view; the cluster shares one.
  - Move `connectome.go` here and delete the original.
- `closure.go`: open items (unviewed files, unanswered questions, untested assumptions, …)
  ranked by value of information and worked within the exploration budget (10%, §5). They never
  block work.
- Budgets: buckets and specs in bytes (turn-0 slice 8 KB, spec 2,800 B); prompt totals in
  provider-reported tokens. L1–L5 prompts target under 4k tokens. L6 prompts may use up to 256k
  with a stable, cache-friendly prefix ordered stable-first.
- `contract.go`: each layer declares its turn-0 inputs. `ContractTest` asserts that every
  declared input is present in the built prompt.
- `overflow.go`: input over its cap → written to a file. The model gets the head, the size, the
  path and a read-by-range instruction. This replaces `SpilloverTruncate`
  (`internal/core/ports.go:98`, sandbox) and `smartFormatHotFile` (`context_build.go:16`).
- `compact.go`: tiers snip → micro → collapse → auto at 50/65/80/90% of the model's context
  (from the 9.2 catalogue).
  - snip, micro and collapse are deterministic: a raw tail of recent turns plus capped tool
    output, with no model call;
  - auto is a model summary with the fixed template Goal / Constraints / Progress / Blocked /
    Next, updated in place, and it stops after 3 consecutive failures.
- `recap.go`: a one-line recap per turn. After compaction the last N (`context.recap_n`, 5)
  turns return with time and stats, and older turns are bucketed.
- `injection_log.go`: every injected block (constitution, memory, config or model change,
  compaction) is an event with source, size and time. Control-plane changes reach the model as
  in-band notices.
- Sessions: each layer keeps a persistent session. Cache hit rate =
  Σ cached_tokens ÷ Σ prompt_tokens (provider-reported, from `usage_events`), a standing metric.
- Measured: turns to first useful action, retrieval miss rate (later reads of files that turn 0
  could have injected), tokens per success.

**Config keys:** `context.turn05`, `context.budget.{slice,spec,bucket.*}`,
`context.compaction.{snip,micro,collapse,auto}`, `context.recap_n`, `features.context`.

**Tests:**
- per layer, `ContractTest` fails when a declared input is removed from the builder;
- a 1 MB tool result → a file is written and the prompt holds head + size + path, under the cap;
- a synthetic session crossing 50/65/80/90% fires each tier exactly once, and the hot path
  makes no model calls (counted via a fake adapter);
- 3 auto-summary failures → stop;
- with the same inputs, the prompt prefix is byte-identical across two builds.

**Live check:** the median L1–L5 prompt is under 4k tokens (provider-reported). The cache hit
rate is reported per layer. Each compaction tier fires at least once in the run, or the run
report says why not.

## 9.6 Memory, living specs and skills — packages `internal/memory`, `internal/specs`, `internal/skills`

Today:
- `internal/memory/store.go` (782 lines) holds FTS5 episodic memory and the wiki, with no
  types, caps or retention score.
- `internal/cognitive/spec_gate.go` (365 lines) holds spec checks.
- `internal/cognitive/wiki_skills.go` (573 lines) holds skills with a posterior mean (:38),
  routing (:232) and distillation (:297).

**Build:**
- Memory records get `type ∈ {episodic, semantic, procedural}`. Semantic records carry
  `source` and `verified_at`.
- Admission:
  - caps (semantic 2,200 B per profile summary, episodic 10,000 entries, §5);
  - a monotonic `updated_at` stale-write check (an older write is rejected);
  - atomic writes (a single transaction).
- Retention: score = importance × 0.5^(age ÷ 30 d). The lowest score is evicted when over cap.
  A consolidation pass merges near-duplicates.
- Write rights: L1's memory accepts writes only from `LayerID==1`, enforced in the store and
  not in prompts. Retrieval feeds L2–L4 via turn 0.
- Health to the watchdog: retrieval hit rate, admission reject rate, and age since last
  verification.
- `internal/specs`: move `spec_gate.go`. Each task has a living spec (in scope, out of scope,
  nuances) of at most 2,800 B, enforced by the gate. When output needs the spec to change, the
  spec is consolidated, not appended. The spec stays live until the done criteria pass the gate
  and L1 closes the task. The output-vs-spec check rejects divergence.
- `internal/skills`: move `wiki_skills.go`.
  - Skills are auto-distilled from successful trajectories.
  - Each carries a Beta(α, β) posterior and is retired after ≥ 5 uses when
    P(rate < 0.5) > 0.9 (§5). Retirement writes a concept-registry row (8.8).
  - Progressive disclosure: about 100 characters per skill in the prompt.
  - Uses the 9.1 `SKILL.md` format.

**Config keys:** `memory.caps.{semantic_bytes,episodic_entries}`, `memory.half_life`,
`specs.max_bytes`, `skills.retire.{min_uses,threshold,confidence}`, `features.memory_v09`.

**Tests:**
- an L3 write to L1 memory → rejected;
- a write with an older `updated_at` → rejected;
- over cap → the lowest score is evicted (a fixed clock gives deterministic order);
- a 2,801 B spec → rejected;
- output touching an out-of-scope path → rejected;
- a skill with 6 uses and 1 success → retired and registered;
- with 6 uses and 5 successes → kept.

**Live check:** the watchdog shows the three memory health metrics. At least one spec is
consolidated, not appended, during the run. The skills index renders at about 100 chars per
skill.

## §5 Starting values (config keys; see the plan table)

Circuit breaker 60 s→300 s doubling, retry full-jitter base 1 s / cap 8 s / 2 retries, timeouts
cloud 60 s / local 300 s, tool timeout 120 s then detach, compaction 50/65/80/90%, turn-0 slice
8 KB, spec 2,800 B, memory caps (semantic 2,200 B per profile summary, episodic 10,000),
retention `0.5^(age/30d)`, skill retirement ≥5 uses and P(rate<0.5)>0.9, budget slow-down by
remaining ÷ projected, exploration 10%, duplicate strategy Jaccard 0.8 (+ artefact overlap 0.5,
9.0), spend cap 0, non-inferiority 2 pt, α 0.05, power 0.8, MDE 10 pt. Every change to these is
a logged L1 verdict.

## Acceptance (v0.9)

- **No-code extensibility:** an MCP tool added by manifest only is called live. The provider is
  swapped by editing `provider.groups` alone.
- **Fault isolation:** an injected panicking plugin is disabled, and the loop keeps running
  (ticks continue).
- **Providers:** a fault-injected provider trips the breaker and fails over in declared order.
  The main model is never replaced by a free model (zero `usage_events` with role `main` and
  `billing_path='free'`).
- **Child agents:** a blocked tool call from a child is refused.
- **Egress:** an off-allowlist host is refused before dialling.
- **Durability:** a crash injected mid-job resumes from its checkpoint with no duplicated side
  effects.
- **Schedules:** a weekly schedule and a monitor both fire correctly across restarts.
- **Replanning:** a planted contradicting fact replans only its subtree.
- **Bus:** a non-adjacent publish and a summary-less message are both rejected.
- **Context:** every layer's turn-0 inputs arrive, bucket budgets hold, each compaction
  threshold fires, and the cache hit rate is reported.
- **Specs:** an over-budget spec or divergent output is rejected.
- **Skills:** a skill with a falling posterior is retired and registered.
- **Model churn:** removing a catalogue model re-routes by tier and re-runs calibration.
- **Credentials:** `opencode.db` is never opened, and a `.env` that isn't 0600 is refused.
- **Primary metric:** L6 turns per completed task (median, p90, n, 95% CI) vs the frozen v0.8
  baseline, plus success rate within 2 pt (non-inferiority, one-sided α 0.05). Reported under
  the honest-measurement rule with the same `definition_sha` as the baseline, or the baseline is
  reset and re-measured first.
- **Acceptance run (1–2 h, or 4–6 h when the owner sleeps, travels or doesn't check in) on the
  current mission:** zero stalls, 0 STALE nodes left without a sweep decision, and no duplicate
  L2 strategies by the 9.0 rule. CI is green (`go test ./...`, domain strings, dead code).
