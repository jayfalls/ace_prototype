# Slice 9.4 — Durable Time Management (internal/tasks)

## What landed

- `internal/tasks` — shared manager over durable tables `runs`, `tasks`
  (idempotency_key UNIQUE, depth, heartbeat_at), `schedules`, `checkpoints`.
- `schedule.go` — RRULE-lite (`FREQ=MINUTELY|HOURLY|DAILY|WEEKLY`, INTERVAL,
  BYDAY); `AdvanceSchedule` moves `next_due` only after a run row reaches
  DONE; dedupe by `(name, due)` via the run row's idempotency key.
- `job.go` — `RunJob` (idempotent side effects via `SideEffectOnce`),
  `DeadlineCheck` (projected-finish alert).
- `monitor.go` — edge-triggered durable monitors, survive restart.
- `child.go` — depth-1 children, per-provider concurrency cap, child
  timeout, parent kill switch cascades via `Cancel`.
- `exec.go` — `ExecOrHandle`: past `tasks.tool_timeout` (120s) the call
  detaches to a `kind=exec_handle` task — poll/read/cancel, not an error.
- `heartbeat.go` — `HeartbeatTick`: monitors + due schedules each pass.
- `budget.go` — `CadenceScale`: burn-rate projection over usage_events;
  non-L6 cadences stretch, L6 stays runnable.
- `plan/assumptions.go` — `MarkStaleByClaim`: a contradicting fact marks
  only the claiming node's subtree STALE.

## Engine wiring

- `jail.go`: `maxExecTimeout` 25s → 120s (`tasks.tool_timeout`), new
  `ExecWithTimeout` hard-capped at 30min for detached runs.
- `actionSandboxSh` runs through `taskMgr.ExecOrHandle` — a >120s call
  returns a handle task id the layer can poll.
- Child tool blocklist enforced at `executeLayerAction` dispatch when the
  active task is a child row (depth ≥ 1) — not just prompt text.
- `=== TEMPORAL CONTEXT ===` prompt block: current UTC, time since last
  owner message, measured circadian availability.
- `schedulerLoop` seeds `self_metrics_rollup` (FREQ=HOURLY), ticks
  monitors/schedules each pass, delivers schedule fires as L5 envelopes,
  reconciles `next_due`, and on startup re-arms crash-surviving tasks into
  L5 via `Resume`.
- Budget governor: smallest `provider.quota.*.5h` limit →
  `engine.SetBudgetQuota`; slowdown stretches L1/L5 cadence only.
- `tasks.Migrate` runs at the engine schema chokepoint
  (`mission_dag.go`) — tables exist on every boot.

## Spec tests (all green)

- Kill mid-job → resume from checkpoint: side-effect count == planned,
  zero duplicates (`SideEffectOnce` skips completed keys).
- Weekly schedule on fake clock: fires once per week across restart;
  `next_due` does NOT advance after a failed run.
- Monitor fires after restart.
- Child attempting `send_message` refused (blocklist).
- Contradicting fact marks only the claim subtree STALE; siblings keep
  state.
- Exec past timeout returns `*Handle`, not an error.

## Live evidence (worker 2883624)

- `tasks`, `runs`, `schedules`, `checkpoints` migrated on boot.
- Fixture schedule `live_check_minutely` (INTERVAL=2, past next_due):
  HeartbeatTick fired → `schedule_run` task RUNNING→DONE,
  `last_run_id` stamped, `ReconcileSchedules` advanced `next_due`
  +interval — observed across two consecutive fires, dedupe key carried
  the real due unix ts. Fixture rows removed after verification.
- Startup `Resume` delivered recovered live tasks into L5's mailbox as
  `task_recovery` envelopes.
