# Concept Registry

Generated from `internal/science` concept_registry seeds — do not hand-edit.

| name | kind | owner | purpose | supersedes | superseded_by |
|---|---|---|---|---|---|
| docs/AUDIT_2026_10_24_GAP_PLAN.md | doc | owner | v0.7.1 audit findings |  |  |
| docs/PLAN_ACE_V0_8_TO_V0_12.md | doc | owner | version roadmap |  |  |
| docs/SPEC_ACE_V0_8.md | doc | owner | v0.8 build spec | PLAN_ACE_V0_8_TO_V0_12.md §8 |  |
| archive/v0_7_decommissioned_20261024 | module | owner | retired v0.7 experiment harness | v0.7 live code |  |
| internal/channels/telegram.go | module | ace | owner channel: directives, grants, reports |  |  |
| internal/cognitive | module | ace | six-layer engine, mission DAG, sensorium, connectome |  |  |
| internal/config | module | ace | owner-set config: mission, grants, model roles |  |  |
| internal/constitution | module | ace | code-owned constitution, sha-pinned at boot |  |  |
| internal/gate | module | ace | tiered L1 verdict gate (T0/T1/T2) | internal/cognitive L1 inline veto |  |
| internal/science | module | ace | scientist gate, prereg ledger, stats, metrics, registry |  |  |
| internal/watchdog | module | ace | out-of-process liveness/integrity watchdog |  |  |
