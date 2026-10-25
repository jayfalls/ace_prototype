# v0.9 Slice 9.6 — Memory, Living Specs, Skills

## What moved

| From | To |
|---|---|
| `internal/cognitive/wiki_skills.go` (model, posterior, routing helpers) | `internal/skills/` |
| `internal/cognitive/spec_gate.go` (budget, veto hash, sections, summaries) | `internal/specs/` |
| memory write gate (was nowhere — prompts only) | `internal/memory/admit.go` |

Compatibility aliases remain in `internal/cognitive` (`wiki_skills.go`,
`spec_gate.go`, `connectome.go`) so engine methods read unchanged.

## Storage-boundary rules now enforced

- **L1-only writes** (`checkWriteRight`): a layer writes only to its own
  `layer_code`; L1 and operator (empty) write anywhere; `GLOBAL` is the
  shared episodic target any layer may write. Rejection happens inside
  `Remember` before the transaction begins.
- **Stale-write rejection**: a caller-stamped `updated_at` earlier than
  the stored row aborts the write inside the same tx (`stale write
  rejected`). Uncalled (zero) stamps default to `now` — always fresh.
- **Semantic cap**: `type=semantic` content > 2,200 B rejected
  (`SemanticCapBytes`).
- **Episodic cap**: > 10,000 rows evicts the lowest retention score
  `importance × 0.5^(age/30d)` inside the writer's transaction.
- **New columns** (`type`, `source`, `verified_at`, `importance`) —
  tolerated ALTERs; migrated live on `.ace-sandbox/ace.db`.
- **Consolidate**: merges src→dst (content append, max importance) in one
  tx — the memory-side "consolidate, don't append".

## Living-spec rules now enforced

- **2,800 B cap**: `EnsureLivingSpecForMilestone` rejects over-cap specs
  (previously truncated silently); `specs.ValidateSpec`.
- **Out-of-scope output**: `specs.CheckOutOfScope` parses the spec's
  `## Out of scope` section and rejects touched paths under declared
  entries; wired into `EvaluateMode3MilestoneGate` before Stage 1 —
  divergence fails the gate with `living spec divergence: ...`.
- **Consolidate**: `specs.Consolidate` merges sections by header and
  rejects over-cap results instead of appending indefinitely.

## Skill retirement

- Spec rule: `Observations >= 5` **and** `P(rate < 0.5) > 0.9` using the
  exact Beta CDF (`science.IncBeta`, the validated incomplete-beta port).
  Replaces the ad-hoc `obs>=4 && mean<0.55` rule.
- On retire: record marked, `SupersededBy=retired_low_posterior`,
  `RetirementLog` entry, and `science.RegisterConcept` writes the
  `concept_registry` row (`kind=skill`, owner L6).
- `skills.RenderIndex` — progressive-disclosure index ≤120 chars/skill.

## Tests (all in-suite)

- `TestL3WriteToL1MemoryRejected` — storage-boundary denial + same-layer,
  L1-anywhere, L6→GLOBAL allowed.
- `TestStaleWriteRejected` — older `updated_at` rejected.
- `TestLowestRetentionEvictedOverCap` — cap shrunk to 3; 90-day
  0.01-importance row evicted over fresh high-importance.
- `TestSemanticCapRejected` — 2,201 B rejected.
- `TestSpecOverCapRejected` — 2,801 B rejected, 2,800 accepted.
- `TestOutOfScopeOutputRejected` — declared paths rejected.
- `TestConsolidateMergesNotAppends` — section merge, cap holds.
- `TestRetireSixUseOneSuccess` / `TestRetainSixUseFiveSuccess` /
  `TestRetireNeedsMinUses` / `TestRenderIndexCompact`.

## Live evidence

- Worker `2907617` (restart 8) running the new binary; supervisor 2792795
  unchanged.
- `PRAGMA table_info(memories)` on live `.ace-sandbox/ace.db`: all four
  v0.9 columns present — migration applied on open.
- 191 memory rows live, all episodic (`type` NULL→episodic default).
- Watchdog `memory_store` check reports `episodic=N/10000
  unverified_semantic=N` every cycle.

## Honest notes

- `MemoryHealth` (Store method `Health`) exists for callers holding the store;
  the watchdog reads the same facts straight from SQL (its established
  pattern — it opens the DB independently of the worker).
- `RenderIndex` is not yet injected into any prompt — the index exists
  and is unit-tested; wiring it into the L6 turn-1 bucket is a v0.9.1
  candidate once the context bucket for skills lands.
