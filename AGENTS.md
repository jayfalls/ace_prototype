# AGENTS.md — ACE repository rules

Binding for every agent that writes code here, including ACE's own PRs. Derived from
`docs/PLAN_ACE_V0_8_TO_V0_12.md` §2 and §8.8; the plan wins any disagreement.

## What ACE is

A domain-agnostic, fully autonomous, six-layer cognitive scientist. Pure-Go single
binary, SQLite only, no NATS/Postgres/Valkey. The Shapiro six layers (L1–L6) always
run and are never removed; the L2‖L3‖L4 parallel cluster over a dependency tree stays.

## Required reading before touching a subsystem

- `docs/PLAN_ACE_V0_8_TO_V0_12.md` — the binding plan (§5 holds every threshold's default).
- `docs/SPEC_ACE_V0_8.md` — the current version's build contract.
- `docs/TRACE_OWNER_ASKS.md` — which owner ask each design choice serves.
- `docs/AUDIT_2026_10_24_GAP_PLAN.md` — what was wrong with v0.7 and must not regress.

Context is injected per task: the task's spec section, the files it touches, and their
tests. Do not load the whole corpus; `archive/` is history, never a build reference.

## Coding standards

- **Layout:** `internal/<component>/`, at most two nesting levels. New v0.8 code goes in
  its own package (e.g. `internal/gate`, `internal/watchdog`, `internal/science`,
  `internal/config`) — never bulk up `internal/cognitive`. Files ≈500 lines max; no god
  files. `profile/` holds all domain content; `internal/` and `cmd/` contain none.
- **Deterministic first:** code wherever the answer is derivable; a model only where
  judgement is needed. Values are derived from data, never hard-coded. Thresholds live
  in config with a §5 default, tuned by measurement only.
- **Composition:** shared components with standard interfaces; one solution per problem,
  never per-layer copies. Adopt a maintained component before building.
- **Domain-agnostic core:** no mission-, benchmark-, competitor- or lab-specific strings
  in `internal/`/`cmd/` — enforced by the domain-string test against
  `testdata/domain_strings.txt` (profiles and testdata are exempt).
- **Owner-set config is authoritative:** mission, personality, models, caps and grants
  are read-only to the agent. ACE proposes changes with evidence; it never edits them.

## Non-negotiable invariants

- **Fail closed:** every gate rejects on error and logs why. A gate that silently skips
  is worse than no gate. Default answers are `deny`/`reject`, never `allow`.
- **L1 first:** nothing runs before L1 anchors the mission; every side-effecting action
  carries a verdict row from the tiered L1 gate.
- **Honest science:** pre-registration before held-out access; the gate recomputes
  statistics — module-reported booleans are ignored; only `EXTERNAL` evidence
  (independent labels) keeps or reverts a component. No goalpost moves: a changed
  hypothesis or dataset is a new pre-registered attempt, Holm-corrected.
- **PR gate (ACE only):** ACE's own repo changes reach `main` only through an owner-merged
  pull request; ACE may keep one long-lived branch across versions so it is never blocked.
  An owner-directed builder commits to the `ace-do` branch (`main` only accepts PRs).
  The running ACE never updates its own binary.
- **No human-approval paths:** disputes and stuck work get an L1 ruling, then quarantine.
  Humans steer the mission, merge PRs, grant access, and answer questions — nothing else.

## Same-step cleanup

A change that adds, updates or rewrites something also removes what it makes dead —
code, docs, skills, concepts — in the same change. The concept registry (plan §8.8) is
authoritative. A gate rejects a change that leaves superseded artifacts behind.

## Verification

- `go test ./...` must pass; new non-trivial logic ships with a test in the same change.
- `go build ./...` clean; keep the single self-contained binary self-contained.
- Evidence for scientific claims goes through the scientist gate, not PR text.
