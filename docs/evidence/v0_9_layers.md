# v0.9 slice 9.3 — layers, bus and cadence (evidence)

Branch `ace-do`. Spec: `docs/SPEC_ACE_V0_9.md` §9.3.

## Bus contract (internal/bus/validate.go)

- Topic regex `^ace\.engine\.[a-z0-9_-]+\.layer\.[1-6]\.(input|output)$`;
  `.input` names the destination layer.
- Requires non-empty `summary`, `agent_id`, `cycle_id`; adjacent hops only
  (`|from-to| == 1`); `TaskID` added to `core.BusEnvelope`.
- `Publish` validates first; violations return an error and land in the
  `bus_rejects` table (migration added in `memory_bus.go`).
- All 8 publish sites migrated to adjacent-hop `InputTopic` envelopes
  (engine, prosecutor ×4, cascade ×2, cmd/ace churn).

## Layer output contracts (internal/layers/contracts.go)

- `ValidateOutput(layer, contract, hasAction)` checks per-layer required
  keys: L2 beliefs `{claim,p,source,credibility}` + `belief_summary`;
  L3 `self_model{capabilities,limits,config,state,measured_capacity}`;
  L4 `plan[]{id,deps,checkpoint,success_criteria}` + `risks[]{risk,contingency}`;
  L5 `done_criteria`, `dwell_s`, `status{task,pct}`; L6 `expected_feedback`
  when an action is taken.
- `ContractPolicy` retries deficient outputs up to 3 times with a prefilled
  opening (`ContractHint`), then escalates — wired into
  `executeLayerStepWithFrozenPrompt`; escalation publishes northbound to
  layer-1 input (`kind=contract_escalation`) and emits a telemetry event.
- Prompts emit the per-layer contract shape via `ContractPromptLine`.

## Mailbox scheduler (internal/layers/scheduler.go)

- Per-layer mailboxes fed by bus subscription; `Ready` rules: L1 events +
  `layers.l1.review_interval` (15m); L2–L4 guidance from above AND data
  from below (northbound mail or a live DAG node via `DataBelow`);
  L5 own clock + `layers.l5.min_dwell` (60s); L6 continuous.
- `schedulerLoop` (cascade.go) replaces `autonomousTickerLoop` — blocks on
  `sched.Wake()` + `NextWake` timers, no 2s polling. L5 dispatches feed a
  `l5.thrash_per_hour` telemetry span.
- Tool cadence (`internal/layers/tools_exec.go`): consecutive read-only
  actions overlap capped at `layers.parallel_reads` (4); mutating calls
  serialize globally; per-path R/W locks prevent same-path overlap. Wired
  into the prosecutor batch loop.
- `internal/config/sectioned.go`: `config.d/*.toml` → `LayerKnobs`
  (l1 review, parallel_reads, l5 period/min_dwell, senses.layer.<n>);
  `WatchLayerKnobs` mtime-polls every 5s → `applyLayerKnobs` hot-swaps the
  executor, scheduler cadence, senses allowlists, and publishes the
  in-band `config_reload` notice (L6→L5 hop).

## Tests

- `internal/layers/layers_test.go`: L4 missing `success_criteria` → 3
  retries + escalation; valid contract passes first try; guidance-without-
  data blocks, data unlocks; L1 event/review + L5 dwell cadence; 3
  read-only calls overlap; overlapping writes serialized.
- `internal/bus/memory_bus_test.go`: L2→L5, empty summary, malformed topic
  rejected; 3 rows in `bus_rejects`.
- `go test ./...`: 16/16 packages ok; `go vet` clean.

## Live verification (worker 2865901, supervisor 2792795, 2026-10-25)

- Post-restart genesis: scheduler fired an L1 review cascade ~70s after
  boot (trace `T_C_S24_DATA` at 18:35:04Z), then L5-clock workhorse
  pulses (`engine_meta.global_loops` Pulse #3 18:35:13Z) — no ticker.
- `bus_rejects` table exists, 0 rows: all live traffic contract-conforming.
- Deleted ticker: `autonomousTickerLoop` removed; `Start` launches
  `schedulerLoop` only.
