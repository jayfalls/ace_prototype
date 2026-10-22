# Opus Review v2: Empirical Re-test of the v0.6 Handoff

This replaces §0 of review v1. I re-ran the analysis on the archived v0.1–v0.3 trace DBs and on the v0.3 workspace. Each number below comes from a script listed at the end.

## TL;DR

1. **The 8.4× "Turn-0 cliff" does not replicate.** It comes from how the two groups were built. With the bugs fixed, first step was not a reach-out: 65.8%, first step was a reach-out: 60.9%. That's a gap of about 5 points, not 8×.
2. **Stuck loops wasted most of the gated work.** The real Mode-3 gate scored 71 passes out of 513 gated cycles (v0.2 + v0.3). Three milestones stuck in loops used up **357 of those 513 cycles (70%) and produced 1 pass**. Two of the three loops were gate bugs. The worker diagnosed each one correctly in its own thoughts, then retried anyway, up to 266 times, because it had no way to escalate.
3. **Missing context was not the main failure.** In 387 of 387 "missing artifact" vetoes, the artifact's path was already in L6's prompt. 98% of the files L6 opened to inspect (150 of 153) were already named in its prompt. Reach-outs fetch *content* (schemas, keys, lines), not *location*.
4. **The connectome slice as written in the spec (2 hops downstream of the directive's anchors) recalls 44%** of the files L6 actually used. Reverse edges raise that to 62%. Anchoring on words from the goal text raises it to 88%. **How the anchors are picked matters far more than how many hops you take.** With the slice ranked and capped at 2,800B, recall is 37%.
5. **The uncertainty map is only partly in the plan** (§3 below). It's missing: one shared map, a rule for which uncertainty to close next, a way to open uncertainties automatically from repeated failures, and closure proofs the harness can check. The Completeness Law has no bound.

## 1. E1: Replicating the cliff

Your exact script, re-run, and then each bug removed one at a time (v0.1–v0.3, L6):

| Step | No reach-out first | Reach-out first |
|---|---|---|
| 0. Original (`ORDER BY id`, group A must contain a mutation) | n=413, 1.92 steps, **77.5%** | n=304, 2.35 steps, **9.2%** |
| 1. Order steps by `created_at` | n=370, 83.5% | n=411, 14.4% |
| 2. Drop the "must mutate" filter on A | n=2008, 15.4% | n=411, 14.4% |
| 3. Same outcome for both (last step `exit=0`) | n=2008, 73.8% | n=411, 62.3% |
| 4. Multi-step cycles only (n≥2) | n=605, 2.47 steps, **65.8%** | n=345, 2.65 steps, **60.9%** |

The bugs:
- **Random step order.** Trace IDs are `tht_<uuid>`, so `ORDER BY id` gives a random step order and "first step" is a random step.
- **Outcome leaks into the grouping.** Group A had to contain a `workspace_write` and group B didn't. The pass test *also* required a mutation. So group A was partly defined by its outcome.
- **"first_cycle_pass" isn't the milestone gate.** It's "the last step printed exit=0 or PASS". v0.1 has no Mode-3 gate at all.
- **Rows silently dropped.** The 717 cycles are A ∪ B after filtering; 1,702 L6 cycles were left out without notice.

**Real Mode-3 outcome, split by attempt number on the same milestone** (v0.2 + v0.3, 74 milestones):

| Attempt | First step was a reach-out | First step was a write |
|---|---|---|
| 1 | 1/14 = 7% (95% CI 1–31%) | 33/50 = 66% (CI 52–78%) |
| 2–3 | 2/8 = 25% | 13/28 = 46% |
| 4+ | 3/49 = 6% | 4/209 = 2% |

On first attempts there is a real gap, but it rests on n=14. It's also confounded: a worker reaches out first more often when the task is new or unclear. That's a reason to run the A/B, not evidence for the claim. From attempt 4 on, everything fails whatever the first step is. Those are the stuck loops.

## 2. E1c: Why gated cycles actually failed

**Mode-3 vetoes (444):** missing or empty artifact 87.6%, selftest/substance 7.2%, hygiene 2.9%, other 2.3%.

**Stuck loops** (more than 10 attempts): 3 of 74 milestones used 357 of 513 gated cycles and produced 1 pass.
- **Axis 8.1, v0.2: 266 attempts over 7.5h**, every veto `Missing ... /workspace/v03_*.py`. The milestone text contained a glob literal. L6's own thought: *"only goals.md glob token `/workspace/v03_*.py` and unchecked box block conformance."*
- **Axis 47, v0.2: 77 attempts**, every veto `Missing ... /workspace/benchmarks/`, which is a directory path. L6's own thought: *"artifacts already exist and verified; prior failure was a mis-issued directive, not code."*
- v0.4 now skips wildcard paths (`internal/cognitive/mode3_gate.go:30`). That fixes this one case. There is still no general mechanism, so the next gate bug will loop the same way.

**L6 step errors** (1,646 of 3,878 steps had an error signal): type/value 42%, assertion/test failure 21%, path not found 12.6%, schema (table/column/key) 5%, symbol/import 3.1%, unclassified 16%. Errors a connectome could prevent (path, schema, symbol) make up about **21% of errors**.

## 3. Does v0.6 include the uncertainty map and exploration the user asked for?

Partly. Here are the user's uncertainty types against the spec:

| User's uncertainty type | v0.6 spec | Status |
|---|---|---|
| Unviewed files | node state `UNEXPLORED` | ✔ |
| Unvalidated assumptions | `ASSUMPTION_UNVALIDATED`, `DEPENDS_ON_ASSUMPTION` | ✔ |
| Contradictions | `CONTRADICTED`, L1.5, Telegram | ✔ |
| Unrun simulations | one line in spec §5 ledger | text only; no node type or state |
| Untested claims | `TESTED_BY` edge | no state for "claim with no test" |
| Unanswered questions / missing feedback | `WAITING_HUMAN` on a blocked branch | questions aren't map nodes; no default or expiry when the human doesn't reply |
| Unfinished research | citations in the L2 map view | no state |

**What's missing:**
1. **One shared map.** The ledger lives inside each task's 2,800B spec. Uncertainties that span tasks or the cluster have nowhere to live, and the ledger competes with §3 (interfaces) for bytes. Put the uncertainty map in the shared connectome as typed nodes, and let each spec's §5 hold references to them only.
2. **An exploration policy.** Nothing ranks which uncertainty to close next. Rank by blocking-ness × value of information ÷ cost to close (cost estimated from past closures of that type). The Completeness Law says to close *everything reachable*, which has no bound. On an 11-module workspace, 3 hops already reaches 40 of 139 nodes (median). Bound it: every node that spec §3 binds, plus that node's 1-hop neighbours, must be `INSPECTED`.
3. **Opening uncertainties automatically from evidence.** The biggest measured waste, repeated identical vetoes, never turns into a ledger entry. **Add a stuck detector:** when the same veto-reason hash appears 3 times in a row, open a `CONTRADICTED` node (gate vs worker), stop retrying, and send L1.5 → Telegram the worker's own diagnosis in at most 4 lines. Let L6 file a *gate dispute* with evidence. On the archived data this would have saved up to about 345 of 513 gated cycles (67%). It is the cheapest v0.6 change with the strongest evidence behind it, and it should land before the connectome.
4. **Typed closure proofs.** `[CLOSED: empirical_proof_ref]` can point at anything, including the agent's own prose. The ref must be a span, test-run or trace ID that the harness wrote and the agent cannot write.

## 4. E2: Connectome replay (v0.3 workspace)

I built the graph deterministically: 139 nodes (11 `.py`, 128 data/docs), 71 directed edges (imports, plus data files named in string literals). It has one hub already: `benchmark_runner.py` (degree 15). I replayed 81 L6 cycles. "Used" means a pre-existing file that appears in that cycle's action targets; files created inside the cycle are excluded.

| Anchors → | Directive paths only | + directive word ↔ filename | + goal-text words |
|---|---|---|---|
| anchors only | 26% | 32% | 86% |
| 2-hop downstream (spec) | 44% | 52% | 88% |
| 2-hop both directions | 62% | 68% | 88% |
| 3-hop both directions | 64% | 70% | 88% |

Each cell is recall of the files L6 used. With goal anchors and 2 hops in both directions, the slice is about 63 nodes and 37KB at signature/schema level (median). With full file contents it's about 280MB because of the DBs. **So the slice must carry summaries (signatures, schemas, sample keys), never file contents.**

**Recall vs budget** (nodes ranked by anchor and graph score, packed greedily):

| Budget | 1.2KB | 2.2KB | 2.8KB | 4.8KB | 8KB | 16KB |
|---|---|---|---|---|---|---|
| Recall | 21% | 34% | 37% | 44% | 56% | 68% |
| Precision | 11% | 16% | 16% | 15% | 12% | 10% |

What this means:
- The 2,800B budget belongs to the **spec**. Don't also make it the slice budget. Give the slice its own budget, start at 8KB, and publish the recall-vs-budget curve from live data.
- The Turn 0.5 router's job is to **rank anchors**, not to walk the graph. Graph walking is cheap and deterministic. Choosing anchors is where recall is won or lost.
- The deterministic precision ceiling is low (10–16%). That is the router's measurable target: the same recall at half the bytes.
- Keep the v1 rule that Turn 0.5 can only add nodes. Log every Turn 1 read of a node that wasn't preloaded as a miss. That gives recall-at-budget per cycle for free.
- LTL G4 (`ltl_monitor.go:90`) blocks reads after N read-only turns. Keep it, but log each time it fires. Every read it blocks is also a Turn 0.5 miss you won't see.

## 5. What v0.6 should do, ordered by evidence

1. **Stuck detector, gate disputes and escalation** (§3.3). Primary metric: attempts-to-pass per milestone.
2. **Uncertainty nodes in the shared connectome**, a bounded Completeness Law, and typed closure proofs (§3).
3. **Turn 0 anchoring:** goal-text anchors, reverse edges, ranked packing into a separate slice budget. Log misses.
4. **Turn 0.5 router behind a flag**, with an on/off A/B on held-out milestones. Metrics: Mode-3 pass per gated cycle, and recall at equal bytes.
5. Everything else (storage, sensorium, anatomy) after these, one flag at a time.

## 6. Vendor and model claims (web check, 2026-10-23)

| Claim | Verdict |
|---|---|
| Mercury Decide `:free`, `/v1/systemone` | Partly verified. Free variant has **32,768 context**, not 66K. **No end date found** for the free tier. OpenRouter `:free` limits: 20 req/min; **50 req/day** if fewer than 10 credits have ever been bought, 1,000/day otherwise. A router doing several calls per cycle will hit 50/day in minutes. Buy 10 credits or have a fallback. |
| Mercury 3 at $0.10/$0.35 | That's half-price launch pricing (list $0.20/$0.70), with no end date. |
| OpenCode Go | $10/mo verified. $16/5h, $40/week, $80/month are **derived** (DeepSeek V4.1 Flash $80 cap × 20% / 50%), not printed. Beam Free and LongCat 2.5 are "free for a limited time". `jev-1.13-free` not found. |
| Beam Max | **Doesn't exist.** |
| Beam 331–485 tok/s | Not found. Third-party measurements are 228–263 tok/s, which matches your measured 250. |
| Step 5 Preview 43.7 / 217 tok/s | **Refuted.** AA lists index 44 and 72–85 tok/s. Drop it from the speed-ordered routing list. |
| GLM-5.3-Flash 41.8 / 132 tok/s | Unverifiable. |
| DeepSeek V4.1 Flash 1017: AA 45, 209 tok/s | Verified. |
| arXiv 2610.14224 (Harness²), WikiSkill 2608.27454 | Both papers exist. Google affiliation isn't stated on the arXiv pages. |
| anatomy repo | MIT. Rasterizing needs headless Chrome. This confirms the plan to port the primitives to Go. |

## 7. Security

`github.com/jayfalls/ace_prototype` is **public**. `SPEC_ACE_V0_6.md` (commits b9882ce8 and 10fde013) contains the Telegram chat_id and a token prefix. A search of git history didn't find the full token. Move both out of the spec into config. If the full token was ever pasted anywhere else that is public, rotate it.

## 8. Not measured, and limits

- **SQLite contention.** `.ace-sandbox/ace.db` has no v0.4 traces or spans yet, so lock-wait time is unmeasured. The v1 conclusion (`SetMaxOpenConns(1)` is the bottleneck) is still a hypothesis.
- **Router recall.** No LLM router was run. All anchor strategies above are deterministic.
- **Generalization.** One small workspace (v0.3), 81 cycles. "Used" is a lower bound (paths parsed from action targets). Re-run E2 on v0.6's own workspace once it has more than 50 modules.

Scripts (read-only against the archive DBs), in `/tmp/claude-1000/-var-home-jay/c24b841b-76cb-4585-a5f4-a4f29076ddc0/scratchpad/exp/`:
`e1_cliff.py`, `e1b_rerun.py`, `e1c_causes.py`, `e2_connectome.py`, `e2b.py` (anchor modes: base / lex / lex+goal), `e2c.py` (budgeted ranking).
