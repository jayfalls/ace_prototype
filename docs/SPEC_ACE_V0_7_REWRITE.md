# SPEC — ACE v0.7 Full Rewrite Architecture (Axis 5.1 Synthesis)

Status: pre-registered architecture specification, derived only from executed and verified axis artifacts in `/workspace/benchmarks/results/`. No claim below is admitted without a machine-readable artifact.

## 1. Epistemic posture

Every architectural decision in v0.7 must be an *organ* with (a) a named artifact, (b) a held-out falsification record, (c) a keep/revert verdict. Architecture without an artifact is treated as a hypothesis, not a component. The blueprint module `ace_v07/anatomy_blueprint.py` is the executable register of that rule: it reads the axis JSONs at runtime, extracts each organ's evidence, computes a traceability coverage with an exact Beta credible interval, and renders the isometric `anatomy` figure.

## 2. Verified evidence ledger

| Axis | Organ | Held-out result | Statistic | Decision |
|---|---|---|---|---|
| 1.1 | Connectome slice retriever | slice miss 0.8937 -> 0.2957 (reduction 0.5980) on 301 held-out turns of 1182 labelled turns from 7677 harvested rows; corpus 262 nodes, top-k 3 | exact McNemar broken=11 fixed=191, p<1e-8; treatment Beta mean 0.7030 | H1_ACCEPTED |
| 2.1 | Skill lifecycle engine | baseline multiple_choice_reasoning accuracy 0.2961 (61 correct) vs mined skill library arm; routing coverage and retirement events recorded | McNemar + Beta CI in artifact | H1_ACCEPTED |
| 3.1 | Sensorium fusion | multi-plane fusion and circadian falsification arms recorded separately | fusion_h1_accepted / circadian_h1_accepted flags in artifact | recorded per arm |
| 3.2 | Reality replica / digital twin | static t0 invariant check 0.6042 (29/48) vs multi-step replica rollout 1.0000 (48/48), 48/48 exact rollbacks, 4 invariants (joint_limit, ...) | McNemar replica-right-baseline-wrong=19, wrong-right=0, p=3.81e-6; replica Beta CI low 0.9275 | H1_ACCEPTED |
| 4.1 | SOTA multi-domain grading | global majority answer 0.0566 (12/212) vs context-compiled exemplar retrieval 0.1274 (27/212) over 5 domains (finance, frontier_exam, law, math, swe) | exact McNemar p=0.0107, but pre-registered decision H0_NOT_REJECTED | H0_NOT_REJECTED -> REVERTED |

Datasets are versioned in `/workspace/benchmarks/registry.json` with deterministic SHA-256 75/25 splits (replica dataset `v07_reality_replica_actuation_v1`, sha256 `c664041a...`; SOTA cache sha256 `a849e8f4...`).

## 3. Honest negative result (Axis 4.1) and the resulting methodology fix

Axis 4.1 is the single non-replicated mechanism. The exact McNemar test is nominally significant (p=0.0107), yet the recorded pre-registered decision is `H0_NOT_REJECTED` because the Beta credible intervals overlap (baseline [0.0297, 0.0970] style interval around 0.0566 vs candidate around 0.1274 with n=212). This discordance is a *governance* defect, not a data defect: two competing statistics were allowed to answer one question.

v0.7 fix (binding): each milestone must declare exactly one primary statistic before execution. Secondary statistics may be logged, never used to flip the decision. Consequently `context_compiled_domain_exemplar_retrieval` is **reverted as a primary answering path** and retained only as a diagnostic exemplar channel, with its measured lift (12 -> 27 held-out correct over 212 items) reported as an unresolved effect to be re-tested under a single-statistic protocol.

## 4. v0.7 organ map (anatomy)

- **L1 Aspirational** — invariant guardian: heuristic imperatives, anti-fabrication law, and the keep/revert gate itself.
- **L2 Strategy** — corpus connectome index (262 nodes, RRF over PPR + BM25 + co-change) feeding turn-0.5 context locking.
- **L3 Self-model** — skill lifecycle engine (posterior-ranked skills, retirement on decay) plus dataset registry introspection.
- **L4 Executive** — milestone compiler: turns a directive into falsifiable H0/H1 with one primary statistic, writes the living spec.
- **L5 Control** — task selection and System-1 gate; frust tolerance and switching.
- **L6 Prosecution** — actuators: dataset construction, module authoring, `--selftest`, mutation audit, Northbound telemetry.
- **Sensory/effector annexes** — sensorium fusion (compute/thermal/power/circadian planes) and the embodied reality replica with exact rollback (48/48).

## 5. Organ contracts

Each organ exposes `evaluate(held_out)` -> `{accuracy, beta_ci, mcnemar, decision}` and must declare `dataset_id` + `sha256`. Organs are wired by the connectome index; an organ with no verified artifact may not be wired into the prosecution path. The anatomy figure (`ace_v07/anatomy_isometric.svg`) is the canonical visual contract for the WebUI and Telegram surfaces: one isometric block per organ, coloured by layer, labelled with its held-out metric, and marked REVERTED where the gate rejected it.

## 6. Telemetry and audit contract

`/workspace/benchmarks/results/v07_axis5_1_synthesis_audit.json` carries: organ list with per-organ metrics, traceability coverage with Beta CI, `h0_hypothesis`, `h1_hypothesis`, `h1_accepted`, `decision`, `alpha_level`, and an explicit `reverted_mechanisms` array. The audit is regenerated by the blueprint, never hand-edited.

## 7. Reproduction

```
python3 /workspace/ace_v07/anatomy_blueprint.py --selftest
python3 /workspace/ace_v07/anatomy_blueprint.py --render
python3 /workspace/ace_rlm.py --audit-flows /workspace/ace_v07
```

## 8. Open risks

1. Axis 1.1 token cost is slightly higher under treatment (420.05 -> 431.59, Wilcoxon p=0.112, not significant) — cost must stay monitored as corpus grows past 262 nodes.
2. Axis 4.1 remains unresolved pending a single-statistic re-test; no v0.7 component may depend on it.
3. Axis 3.2 evidence is single-replica (`embodied_rig_twin_v1`); transfer to a second twin is unverified.
4. Axis 3.1 fusion and circadian arms carry independent flags; a joint falsification is still outstanding.
