# ACE v0.6 Implementation Spec (final, 2026-10-23)

Supersedes `SPEC_ACE_V0_6.md` for implementation. That file stays as the design history.
Every number below was measured on the archived v0.1–v0.3 trace DBs, the v0.3 workspace, commit-history
replays of 4 repos (88–1,342 files), or a local benchmark. Evidence: `OPUS_REVIEW_V2_EMPIRICAL.md` and the
experiment scripts (copied to `docs/v0_6_evidence/` if the implementer wants to re-run them).

## 0. Hard constraints

1. **Domain-agnostic core.** No bespoke domain flows, file names, axis/milestone names, languages or
   human schedules in Go. Anything domain-specific is learned from the workspace or data at runtime.
2. **Self-contained portable Go binary.** Pure Go, no CGO, no Node, no Python, no headless browser, no
   external services required to run. Reads no files outside its own sandbox/config (for example no
   `~/.pi/...` skill paths).
3. **SQLite stays** (modernc, current single connection). No Pebble/Badger/NATS/JetStream.
   Measured load: 0.1–0.4 rows/s mean, 25/s peak. Current config sustains 2,274 writes/s at p99 < 2ms.
   Zero lock errors across all archives. Revisit only if `db_write_wait_ms` exceeds 5% of cycle time.
4. **Never block on the human.** Ask, park the branch, keep working on something else.
5. **No secrets in docs or code.** Telegram chat ID and bot token come from env/config only. Remove the
   chat_id and token prefix from `SPEC_ACE_V0_6.md` (repo is public) and rotate the bot token if the full
   token was ever committed.

## 1. Topology

```
L1   Aspirational (mission, constitution, human alignment)
L1.5 Gate: filters northbound signals; contradictions, gate disputes, scope changes -> L1 -> human
L2 || L3 || L4   run in parallel over one shared map + wiki/spec space, each with its own view
     L2 strategy / external world   L3 self-model (compute, quota, models, skills, human availability)
     L4 executive (plan, dependencies, risk, owns living specs)
L5   Gate + scheduler: launches L6 tasks, grants file leases, runs Turn 0/0.5, merges diffs,
     consolidates results north, owns shared state files
L6   Task workers, each with its own workpad (.workpads/<task_id>/: scratchpad, attempt ledger, copy of
     leased files) and private history
```

- L5 owns shared state files (`goals.md`-style). L6 never writes them directly; it proposes a change and
  L5 applies it. (`goals.md` took 310 writes in v0.2.)
- The bus carries `AgentID, CycleID, LayerID, TaskID, CorrelationID` on every message (keep the existing
  in-process bus).

### 1.1 Code layout (flat, composable, no god files)

Max two levels: `internal/<component>/<file>.go`. No nested packages. One package per component;
components talk through `internal/core` interfaces and the bus, never by reaching into each other.

```
cmd/ace/            main: wiring only
internal/core/      shared types + ports (interfaces)
internal/bus/       in-process typed bus
internal/layers/    L1–L4 layer loops (one file per layer)
internal/gates/     L1.5, L5 gate logic, Mode-3 checks, gate disputes, stuck detector
internal/workpad/   L6 worker loop, workpads, leases, diff/merge
internal/mapgraph/  map tables, mention/co-change edges, fusion ranking, summaries
internal/turns/     Turn 0 sub-buckets, Turn 0.5 router, slice-miss logging
internal/spec/      living spec gate, frozen-hash ledger, uncertainty ledger
internal/sensorium/ self vitals, availability model, quota meter
internal/llm/       model clients, capability-slot routing
internal/figure/    pure-Go isometric SVG kit
internal/channels/  Telegram
internal/memory/    SQLite store
internal/sandbox/   jail + tools
internal/server/    HTTP/cockpit
internal/telemetry/ spans, time series
```

- **File size:** about 500 lines per file at most. Split by responsibility, not by line count.
- **Component wiring:** each component is a struct implementing a `core` port, so it can be composed,
  swapped or stubbed in tests. Nothing domain-specific is wired in.
- **Existing god files:** `internal/cognitive/engine.go` (3,376 lines) is split into `layers/`, `gates/`,
  `workpad/` and `turns/` as build step 0, with behaviour unchanged and the existing tests passing.
  `sandbox/hygiene.go` (1,517 lines) and `llm/opencode_free.go` (1,068 lines) are split the same way when
  they are touched.

## 2. Context pipeline: Turn 0 -> Turn 0.5 -> Turn 1 (composable stages)

Each stage builds on the one before. Later stages may only add or reorder, never remove.
The Turn 0 slice is the guaranteed floor if anything after it fails.

Rule of thumb, from the measurements: **deterministic code does everything that is a lookup**: graph,
ranking, summaries, budgets, gates, leases, stuck detection. **A model is used only for judgement over
the shortlist**: what among the top 40 the task really needs, how to word a spec, what to ask the human.
The model never replaces the deterministic floor, and it stays only where live A/B data shows it beats
the deterministic result.

### 2.1 Turn 0: deterministic, no LLM

Sub-buckets, in this order (stable prefix first, so the KV prefix cache keeps hitting):
1. Role and constitution prefix.
2. Living spec (section 4), at most 2,800B.
3. Context slice (section 3): ranked node summaries under its own budget (start 8KB, `ACE_SLICE_MAX_BYTES`).
   It is separate from the spec budget, and lists the IDs of omitted nodes.
4. Workpad state, attempt ledger and the last veto reasons for this milestone.
5. Sensorium digest (section 7).

### 2.2 Turn 0.5: routed retrieval

A cheap model (capability slot `router`, section 8) sees the directive, spec and the top 40 ranked
candidates with summaries. It returns structured read-only requests:
`{nodes_to_add, file_line_ranges, json_key_samples, db_schemas, memory_queries, web_queries}`.
Go runs them in parallel and appends the results.
- Must time out (2s default) and degrade to Turn 0 alone. A router failure is logged, never fatal.
- Ships behind `ACE_TURN05=on|off|ab`. Default `ab` alternates per task, so its value gets measured
  live (section 9). The offline pilot produced no result: the public free endpoint returned 403, and
  the run via the Go subscription timed out at 25 min. The decision is made from v0.6 live data.

### 2.3 Turn 1: worker acts or searches

- The worker executes, or searches for what is still missing.
- Every read of a node that was not preloaded is logged as a **slice miss** (`slice_miss` span with
  node ID and the cause: not in graph / ranked below budget / router dropped).
- Keep LTL G4 (`ltl_monitor.go`), but log each time it fires. Each block is a miss that would otherwise
  go unseen.

What workers actually fetched in v0.3 recon: line ranges 48, JSON key samples 47, whole files 22,
directory listings 10, DB schemas 5. 98% of fetched files were already named in the prompt, so they
needed **contents**, not locations. Slice summaries must therefore carry signatures, JSON keys with a
sample value, DB table schemas and the first heading of docs.

## 3. Problem-space map (connectome)

### 3.1 Graph
- **Nodes:** every file in the workspace and wiki (any type), plus `SPEC_CLAUSE`, `TEST`, `ASSUMPTION`,
  `CONTRADICTION`, `ASYNC_TASK`, `SKILL` (reserved for v0.7).
- **Edges, all language-agnostic:**
  - `MENTIONS`: A's text contains B's path, its basename, or its unique stem of 6+ characters.
  - `CO_CHANGED`: counts of files changed together in git history, or in ACE's own workpad merges when
    there is no git.
  - `SPEC_GOVERNS`, `TESTED_BY`, `PRODUCES` are recorded from specs, gates and runs, not parsed from code.
- An optional per-language parser may add edges, but nothing may depend on it. On v0.3 the mention graph
  matched or beat a Python AST graph: 1-hop 68% recall against 62% for AST 2-hop.
- Edges are traversed in both directions. Downstream-only traversal missed callers and tests
  (44% recall against 62% with both directions).
- Node state: `UNEXPLORED -> INSPECTED -> IMPLEMENTED -> VERIFIED` (or `FALSIFIED`/`CONTRADICTED`).
- Incremental rebuild on file change (mtime/hash). Store it in SQLite tables `map_nodes` and `map_edges`.

### 3.2 Ranking (no hop walks)
2-hop BFS covers 24–53% of a large repo because of hub files (degree 250–597), so do not walk by hops.
Rank candidates with reciprocal-rank fusion (k=60) of:
1. Personalized PageRank (alpha 0.6, 25 iterations) seeded from anchors: paths named in the directive or
   spec.
2. Lexical match: IDF-weighted overlap of directive, spec and goal words with path tokens.
3. Co-change with the anchors.

Measured recall of files the task really touched (commit replay, 4 repos):

| Signal | R@10 | R@20 |
|---|---|---|
| PageRank only | 6–42% | 17–69% |
| lexical only | 11–30% | 13–49% |
| co-change only | 38–58% | 50–73% |
| **fusion** | **55–78%** | **66–87%** |

No single signal wins on every repo, so all three are required.
Pack summaries greedily in rank order into the slice budget.

### 3.3 Layer views
L2, L3, L4 and the L6 workpads read the same tables with different seed sets and node-type filters.
There is no separate store per layer.

## 4. Living spec (2,800B, enforced)

`/wiki/specs/<task_id>.md`, rejected (never truncated) if over `ACE_SPEC_DISTILL_MAX_BYTES` (2,800).
Sections:
1. **Scope** (in / out). Human-owned and frozen once approved.
2. **Acceptance bar** (measurable checks). Human-owned and frozen once approved.
3. **Interfaces:** map node IDs plus the one-line contract for each. Not inlined signatures, which take a
   median of 1,692B and exceed 2,800B in 15% of tasks; the slice carries the signatures.
4. **Nuances and failure modes.**
5. **Uncertainty ledger** (section 5).

Anti-cheating:
- Frozen sections are stored as SHA-256 hashes in an approval ledger outside agent write access. Any
  change to them is a `CONTRADICTION` and goes to L1.5, then the human.
- Agent-owned sections may change, but every change is logged as a diff. Thresholds can tighten without
  approval; loosening needs the human.
- Acceptance tests are written before implementation and are read-only to L6. A held-out check is not
  readable from L6.
- L5 runs a deterministic post-run audit: every node ID in section 3 exists and the acceptance checks pass.
- A spec stays `LIVING` until the human marks it done.

## 5. Uncertainty handling: a balancing act, not a gate

The goal is to close as many knowledge gaps as are worth closing to make good decisions, without getting
stuck in the weeds. The old rule "cannot finish while anything reachable is unexplored" is removed: on
v0.3, 3 hops already reached 40 of 139 files.

- **Ledger entries** have a kind (`unviewed_node`, `unanswered_question`, `untested_claim`,
  `unvalidated_assumption`, `missing_feedback`, `unrun_simulation`, `unfinished_research`), a cost
  estimate and a value-of-information score.
- **Value of information:** high if closing the entry could change a decision, touches the acceptance bar
  or the write set, or blocks a dependency. Low otherwise.
- **Work order:** close entries in VOI-per-cost order within a per-task budget (`ACE_UNCERTAINTY_BUDGET`,
  default 20% of task steps).
- **Completion:** only entries tied to the acceptance bar or the write set block completion. Everything
  else stays logged and visible, and L2–L4 can pick it up later.
- **Human-dependent entries** are asked, parked as `WAITING_HUMAN`, and never block other work.

## 6. Execution control

### 6.1 Stuck-loop detection and gate disputes (highest-evidence change)
In v0.2–v0.3, 3 of 74 milestones used 357 of 513 gated cycles (70%) and produced 1 pass. Two of them
were gate bugs (a literal `*` glob path, and a directory treated as a file) that the worker had diagnosed
in its own notes but had no way to escalate.

Normalize the veto reason (strip numbers), then:
- **Same reason 3 times in a row:** forced strategy change. The worker re-reads its own last attempts and
  must list at least 2 hypotheses, including "the gate or spec is wrong". Replaying the real veto history,
  escalating at 3 would fire 11 times, 8 of them on milestones that later passed on their own, so 3 only
  forces reflection and never escalates.
- **Same reason 5 times in a row:** park the milestone and file a **gate dispute**: the worker's evidence,
  the gate's check and the artifact path. L1.5 first runs deterministic checks (glob in path, path is a
  directory, file exists but is empty), then escalates to the human if those don't settle it. L5 moves on
  to other work. Replay: k=5 parks 4 milestones (2 of which would have passed later) and cuts gated cycles
  to 34% of the original.
- **Any reason 8 times in a row:** the same park and dispute.
- **Self-check before claiming done:** L6 runs the gate's deterministic artifact check itself before
  claiming done. 87.6% of vetoes were `missing_required_artifact`, and in all 387 the path was already in
  the prompt.

### 6.2 Parallel L6 with file leases and change management
62–68% of consecutive task pairs wrote the same file, so parallelism needs ownership.
- **Write sets:** each task declares its write set in spec section 3.
- **Leases:** L5 grants exclusive leases per file. Tasks with overlapping write sets never run at the same
  time, and a task that needs an unleased file asks L5.
- **Workpads:** a workpad works on copies of the leased files plus the base hash of each.
- **Merge:** L5 checks that each base hash still matches, applies the diff, records it (task, base hash,
  new hash, diff) and releases the lease. On a hash mismatch the task is re-queued with the new base and
  the diff in its context. Nothing is ever blindly overwritten.
- **Read-only:** reads of non-leased files are always allowed.
- **Concurrency:** start with `ACE_L6_MAX_PARALLEL=2`.

## 7. Sensorium

### 7.1 Self (proprioception)
CPU/RAM (`/proc`), GPU gate health and latency, disk free, network RTT, model health per model
(success, latency and measured tok/s), and the quota meter (section 8).

### 7.2 Human availability (derived, never hardcoded)
- **Remove** `SLEEP_WINDOW` and any fixed hours.
- **Sources** (whatever exists): Telegram message times, Cockpit/API interactions, git commit times in
  the workspace, and the desktop idle hint (logind `IdleHint`) when running on the user's machine.
- **Model:** P(active now) from recency of the last human signal combined with an hour-of-week base rate
  learned from history. Recency is the strongest signal (log loss 0.60 to 0.51); a fixed window is no
  better than the base rate; Telegram-only data is worse than a constant, so use every source available.
- **Reply latency** is not predictable (mean absolute error about 5h). Do not schedule around a predicted
  reply time. Ask, park, continue, and batch non-urgent questions into one digest message.
- **Digest line** shown to every layer: local time, P(active), time since the last human signal.

### 7.3 Telegram protocol
- **Message shape:** one short plain-English message (4 lines or fewer): the contradiction or question
  plus options A/B. No walls of text.
- **Anatomy figures:** when conveying a complex component, attach an anatomy-style isometric figure.
  Implement it as a small **pure-Go port** of the anatomy primitives: isometric projection, four-tone
  shading and painter's-order sort, emitting SVG. The anatomy repo is MIT; port the ideas and do not vendor
  its LICENSE into this repo.
- **Delivery:** send the SVG via `sendDocument`. PNG rasterization in pure Go is optional (for example
  `oksvg`/`rasterx`) if `sendPhoto` is wanted.
- **No re-asking:** never double-text a parked question.

## 8. Models and economics

- **Capability slots** (`router`, `worker`, `thinker`) map to an ordered model list in config, not in
  code. Routing uses **live measured** tok/s, success rate and latency from 7.1, not published benchmark
  numbers. (Several handoff figures were wrong: "Beam Max" does not exist, and Step 5 Preview's speed was
  refuted.)
- **Strict Tier Isolation Law (Free -> Free ONLY; Never Escalate Cost on Fallback):**
  - **Free Tier (`router` / `distiller` / high-RPS slots, `$0/$0` models only):** A free-tier slot (`opencode-go/beam-free` -> `opencode-go/cat-jam-free` -> local 4-bit GPU gate / `inception/mercury-decide:free`) MUST **only** fall back to other `$0` free models (`cost.input == 0 && cost.output == 0`). A free model fallback is strictly forbidden from crossing into any paid/metered model.
  - **Subscription Allowance Tier (`worker` / `thinker` slots):** Uses `opencode-go/deepseek-v4.1-flash` (`$0.15/$0.60`, `$0.003/1M` cached input; fallback to `opencode-go/glm-5.3-flash` / `opencode-go/mimo-v2.6-flash` at equal/lower `$0.14–$0.15/1M` tier), and when approaching window budget caps (`80%` of `$16/5h`, `$40/week`, or `$80/month`), **downshifts into the Free Tier** (`beam-free` / `cat-jam-free`), never upward to higher-cost models (`deepseek-v4-pro`, `kimi-k3`, etc.).
- **Live OpenCode Go OAuth Credential (`opencode.db`) & Endpoint Probing:**
  - Read the active OAuth token directly from `~/.local/share/opencode/opencode.db` (`credential` table, `integration_id='opencode'`) with mandatory `x-opencode-session` and `x-opencode-request` headers against `https://opencode.ai/inference/go/openai/v1/chat/completions`.
  - Automatically quarantine deprecated/unavailable models (`410 ModelDeprecated` on `minimax-m3.1-flash-free`, `400/402` on `step-5-preview-free` / `longcat-2.5-preview-free`) and detect workspace privacy-region errors (`400: This Go model requires Global regions` on `deepseek-v4.1-flash`) on startup.
- **Mercury Decide:** `inception/mercury-decide:free` (OpenRouter) is an optional `router` backend: no
  stated end date, 50 req/day under 10 lifetime credits, 1,000/day at 10+.

## 9. Telemetry and acceptance for v0.6

Each item is a metric emitted as a span or time series; together they define success for the v0.6 run.

| Metric | Target after first 48h |
|---|---|
| Share of gated cycles spent on milestones with more than 10 attempts | < 25% (was 70%) |
| Gate disputes filed / resolved, with outcome | all resolved or with the human |
| Slice recall (1 - misses / files used), per budget | reported; >= 60% |
| Turn 0.5 on vs off: first-attempt pass rate, steps to done, recall | reported with n per arm |
| Lease conflicts and re-queues | reported; no lost writes |
| Spec rejects (over budget, missing section, frozen hash changed) | reported |
| Availability model log loss vs a constant | lower than constant |
| `db_write_wait_ms` share of cycle time | < 5% |
| Quota burn per window, and fallback reasons | reported |

## 10. Build order

0. Split `engine.go` into the §1.1 layout (pure refactor, tests green, no behaviour change).
1. Remove secrets from docs; add the `slice_miss`, `veto_reason`, `db_write_wait_ms` spans.
2. Stuck detector, gate disputes, L6 self-check (section 6.1).
3. Map tables, mention and co-change edges, fusion ranking, summaries, slice budget (section 3, 2.1).
4. Spec gate with frozen-hash ledger and node-ID interfaces (section 4).
5. Uncertainty ledger with VOI ordering (section 5).
6. Leases, workpad diffs, merge (section 6.2); then raise L6 parallelism.
7. Turn 0.5 router behind `ACE_TURN05=ab` (section 2.2).
8. Sensorium availability model; Telegram digest and pure-Go figures (section 7).
9. Capability-slot routing on measured stats; quota meter (section 8).
Each step ships with one runnable check (`go test`) and is demoed on a fresh empty workspace with a
goal that is not about ACE itself.

## 11. Out of scope for v0.6 (v0.7)

- Automatic skill management (Wiki-Skill style, arXiv 2608.27454): `SKILL` nodes, distillation from
  passed gates, pass-rate tracking, retirement.
- Anatomy-style WebUI built from the same pure-Go figure kit.
