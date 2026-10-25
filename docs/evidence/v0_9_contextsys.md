# Slice 9.5 — Context system (internal/contextsys)

## What landed

- `stages.go` — composable Stage/byte-budgeted Bucket; turn-0/0.5/1 ids;
  slice 8 KB / spec 2,800 B / L1–L5 4k-token / L6 256k constants.
- `map.go` — SQLite problem-space map (`map_nodes`, `map_edges`); BFS
  `Frontier(roi, layer)` with per-layer visibility bitmask; `OpenItems`.
- `connectome.go` — the connectome model MOVED here from
  `internal/cognitive/connectome.go` (spec: delete the original). The
  cognitive file now holds only engine orchestration methods behind
  type/var aliases.
- `closure.go` — VOI/cost-ranked open items, `WithinBudget` selector
  (10% exploration budget, never blocks).
- `contract.go` — per-layer declared turn-0 inputs + `ContractTest`.
- `overflow.go` — over-cap input → `spill-<name>` file + head + size +
  path + read-by-range instruction. Wired into `sandbox_sh` output
  (replaces `SpilloverTruncate`); `HeadOrMeta` for hot files.
- `compact.go` — `Compactor`: snip 50% / micro 65% / collapse 80%
  (deterministic tail+cap, zero model calls) / auto 90% (fixed-template
  summary, stops after 3 consecutive failures). Each tier fires once per
  crossing.
- `recap.go` — per-turn one-line recap; post-compaction last-N turns with
  time+stats, older turns bucketed (`context.recap_n`).
- `injection_log.go` — `injection_log` table + in-memory tail; every
  injected block recorded (source, bytes, time); control events drain to
  the model as in-band `=== CONTROL NOTICES ===` prompt blocks.
- `metrics.go` — `CacheHitRate` (Σcached/Σprompt from usage_events,
  provider-reported), `TokensPerSuccess`, `RetrievalMiss`,
  `TurnsToFirstAction`.

## Wiring

- `context.turn05` (default **false**) replaces `ACE_TURN05=ab` — the
  router is now owner-opt-in. `context.recap_n`, `features.context` knobs.
- `maybeCompact` runs on priorTurns before message assembly; fill =
  last provider-reported prompt_tokens ÷ catalogue context window
  (`Registry.ModelContext`, new export).
- Soul/constitution injection recorded per layer call; config.d reload
  publishes a control notice.
- `injection_log`/`map_*` migrate at the engine schema chokepoint.

## Spec tests (all green)

ContractTest fails per layer on removed input · 1 MB tool result → file +
head+size+path under cap · 50/65/80/90 crossings fire each tier exactly
once, 1 model call total (auto only) · 3 auto failures → disabled ·
byte-identical prefix · VOI-budget selection · BFS frontier with layer
visibility.

## Live check (worker 2895872, provider-reported)

| layer | calls | cache hit rate | avg prompt tok | under 4k? |
|---|---|---|---|---|
| L1 | 318 | 0.598 | 625 | yes |
| L2 | 2 | 0.268 | 10,967 | **no** |
| L3 | 19 | 0.130 | 3,611 | yes |
| L4 | 44 | 0.405 | 10,819 | **no** |
| L6 | 178 | 0.802 | 27,126 | n/a (≤256k) |

- `injection_log` rows land per layer call (`constitution+mission`
  block, 6,764 B).
- Compaction tiers did **not** fire this window: measured fill stayed
  under 50% (L6 avg 27k vs the catalogue window). That is the honest
  "why not" per spec.
- L2/L4 prompts exceed the 4k target — stimulus-heavy payloads; flagged
  as a finding, not hidden.
