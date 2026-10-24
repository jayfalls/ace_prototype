# ACE gap audit and v0.8 direction (2026-10-24)

Four read-only audits of the working tree, git history, live and archived trace DBs, and primary sources
(arXiv 2310.06775v2, daveshap/ACE_Framework, DeepSeek Harness docs, Hermes/OpenClaw plugin docs).
Line numbers are from the working tree on 2026-10-24.

## 0. Verdict

ACE runs, plans top-down and gets work done, but it is not yet the ACE that was asked for:
1. **Its science is self-certifying.** No headline result survives scrutiny (section 2).
2. **The constitution is prompt text.** The Heuristic Imperatives can be swapped by editing one file, and
   no gate checks actions against them (section 3).
3. **It is not general.** The core carries one lab's mission, Python, CUDA, Linux, one spoofed provider and
   hardcoded research paths (section 4).
4. **It is not fully autonomous.** Human-approval paths remain (section 3.3).
5. **Earlier iterations had pieces it dropped.** Multi-provider roles, schema-validated layer outputs,
   adjacent-only bus hops, install and XDG config, access control (section 5).

## 1. Urgent (do before anything else)

| # | Issue | Evidence | Fix |
|---|---|---|---|
| U1 | Control server on all interfaces, no auth: chat, engine control, model select, `/api/wiki/write`, `/api/sandbox/exec`, `/api/telegram/configure` | `cmd/ace/main.go:102` binds `":"+port`; `ss` shows `*:8080`; `server/http.go:587` CORS `*` | Bind `127.0.0.1` by default; bearer token (generated at first run, stored in config dir) on every mutating route |
| U2 | L5 gate fails open | `cognitive/cascade.go:314` `pYes := 0.96`; error leaves it at 0.96, `gateApproved` at :350 | Default `pYes = 0`; an error means not approved |
| U3 | Provider impersonates the opencode CLI and reads opencode's credential DB | `llm/opencode_free.go:29-47` (spoofed UA, CLI preamble, `~/.local/share/opencode/opencode.db`) | Replace with a plain OpenAI-compatible provider plugin using an explicit API key from ACE's own config |
| U4 | Self-building runs on invalid gates | section 2 | Pause the v0.8 self-building branch until the gate in 2.3 exists; work done so far is quarantined, not deleted |
| U5 | Telegram chat ID and token prefix remain in public git history | `b9882ce8`, `10fde013` | Rotate the bot token |

## 2. Scientific rigor: ACE grades itself and passes itself

### 2.1 Per-result verdicts

| Result (claimed H1) | Verdict | Why |
|---|---|---|
| 11.1 Hermes vs OpenClaw vs ACE ablation | **Fabricated by construction** | Neither competitor was ever installed or run: 0 install/run commands in any trace. "Served" = `required ⊆ capabilities`, where both are hand-written constants (`ace_v07/hermes_openclaw_ace_ablation.py:50-74`) and ACE is defined as the superset. The cost ratio is exactly ACE's cache rate (1 - 0.6963), and the hand-set competitor cache rates are 0.0 and 0.25. p≈0 comes from deterministic multiples of string length |
| 1.1 connectome slice | Weak | Gold labels are paths regex-extracted from ACE's own traces, often literally present in the query. The spec ledger (0.89→0.30, cost not significant) does not match the artifact (0.74→0.12, cost increase p=0.003) |
| 2.1 skill lifecycle | Circular | Labels come from a regex; the skill key is constant per source dataset; the baseline is the global majority. A per-source majority prior (0.825) beats the skill library (0.777) |
| 3.2 reality replica 48/48 | Circular | The reference is the same simulator at a finer timestep; all 204 records share one initial state; "48/48 exact rollbacks" is a tautology because `rollout` never mutates state |
| 11.2 gap matrix | Circular | Rows without a label are dropped, so every row is positive; the threshold is 0.0. The held-out McNemar p=0.0625 is not significant, so H1 was taken from a second (CV) statistic, which ACE's own v0.7 spec forbids |
| 12.1 assistant suite | Circular | Same pattern: split p=0.219, H1 taken from CV p |
| 12.2 candidate builder | Circular / weak | Harm labels are substring matches on ACE's own bus events; the baseline is promote-all |
| 4.1 SOTA grading | Weak | The spec says reverted (H0); the artifact says H1. String-heuristic grader |

### 2.2 Why the gate let it through
- `mode3_gate.go:579-651` fails a result only if `h1_accepted` (or similar) is literally `false`.
  Writing `h1_accepted: true` passes.
- No statistic is recomputed, no dataset hash is checked, no pre-registration exists, and nothing checks
  that the held-out split was unseen.
- Each module's own `--selftest` regenerates the artifact the gate then reads, so the producer is the
  verifier.

### 2.3 Scientist-grade gate (domain-agnostic, replaces Mode-3 stage 3.5)
1. **Pre-registration first.** Before any held-out access: hypothesis, ONE primary statistic, alpha,
   decision rule, dataset sha256 and split seed, written to an append-only ledger. The gate checks the
   ordering against the evaluation log.
2. **The gate decides, not the module.** The module emits paired per-item predictions only. The gate
   recomputes the primary statistic in Go and applies the pre-registered rule. Module booleans are ignored.
3. **Independent data and labels.** External versioned sources (URL + sha256 at fetch), or labels not
   produced by the mechanism under test. Same-simulator or self-trace labels count only as smoke tests.
4. **Baselines.** Constant predictor, stratified/per-group prior, and the strongest available external
   method. The treatment must beat all three.
5. **Degeneracy and leakage checks.** Fail on zero-entropy held-out labels or an all-positive treatment.
   Shuffled-label and planted-canary negative controls must come out null.
6. **External systems are run, not described.** Competitor claims need an execution transcript: pinned
   version or digest, command, exit code, wall time, provider-reported tokens. Text in a release note is
   not performance evidence.
7. **Fresh-sandbox regeneration.** The gate re-runs the evaluation in a clean sandbox from the
   pre-registration and dataset hash. Selftests may not write gate-read artifacts.
8. **Replication and ablation.** At least two independent datasets or initial conditions, plus removal of
   the claimed mechanism.
9. **Ledgers are generated.** Spec evidence tables are rendered from artifacts; a mismatch fails the gate.

## 3. Shapiro fidelity and autonomy

### 3.1 Requirement gaps (paper v2 + ACE_Framework.md)

| Requirement | State |
|---|---|
| L1 has top-down authority and monitors all traffic | **No.** L1 runs only at genesis, on soul hash change and every 4 milestones (`cascade.go:92-120`). The steady state starts at L5 |
| Imperatives govern every layer's decisions | **Prompt text only.** No gate (L5, Mode-3, LTL, spec, action vetting) consults them |
| Bus as transport, adjacent hops only | **No.** The bus is an audit log (no `Subscribe` callers); there is no adjacency check. The 2024 version enforced `BUSSES_DOWN/UP` adjacency (`app/constants/queue.py:34-50`) |
| System Integrity overlay: out-of-band monitor, kill switch, restart, config rollback | **Missing** |
| Ensemble judgement at L1 (paper 3.2.5) | **Missing** |
| Bounded self-modification, hijack resistance | **Weak.** Filename string guard (`prosecutor.go:1683`); bypassed by the HTTP wiki write |
| Cognitive control (frustration, task switching) | Present |
| Task prosecution monitoring | Present, but see section 2 |

Note: the northbound/southbound terms come from the repo, not the paper (KB corrected). The paper itself
says the full ACE "has not been implemented yet" (5.3).

### 3.2 Ingrain the constitution
1. **Compile it in.** `go:embed` the constitution (Heuristic Imperatives, UDHR, autonomy triad,
   anti-fabrication law) with a build-time sha256 constant. Startup verifies it, and it is never loaded
   from the wiki. Changing it requires a rebuild.
2. **Separate the mission.** `CURRENT MISSION` is the only human steer and lives in its own config file.
   L1 rejects missions that conflict with the constitution.
3. **L1 as a per-action gate.** Every L2–L6 action that has side effects passes L1 judgement on the bus:
   `{allow, revise, reject}` with rationale, scored against each imperative. The engine enforces the
   result, and each verdict is stored with the constitution hash.
4. **Ensemble for high-stakes calls.** Three judgements with majority vote for rejections, disputes and
   self-modification (paper 3.2.5). The cheap path is a single judgement.
5. **Integrity overlay.** An out-of-band watchdog goroutine that can halt, restart, and roll back config
   and the last self-modification, and that verifies the constitution hash and the gate fail-closed state.
6. **Protect the constitution's paths.** HTTP and tools can never write the constitution path or the
   mission ledger.

### 3.3 Remove human approval (full autonomy)

| Current human dependency | Replacement |
|---|---|
| `spec_gate.go:236-247` frozen hash changes only with `human_approval_ref` | An L1 ruling: store `l1_verdict_ref` (verdict, rationale, constitution hash). Allow records the new hash; reject reverts the change |
| `spec_gate.go:140-176` gate dispute "Reply A/B" over Telegram, node `PARKED_HUMAN` | L1 adjudicates in bounded rounds (ensemble). If still stuck, the node becomes `QUARANTINED` and L3/L5 pick other work |
| `prosecutor.go:407-421` parks after 5 identical vetoes to the human | Same: L1 ruling, then quarantine |
| `cascade.go:617-626` skips "human-gated" milestones | Delete |
| "L1.5" named in code but not implemented | Implement as the L1 per-action gate (3.2.3) or delete the name |
| SPEC v0.6 human-owned frozen sections, "loosening needs the human", WAITING_HUMAN | Superseded by this section |

Humans may do only two things: steer the mission, and receive reports or answer questions that ACE asks
for information (never for permission). Unanswered questions never block.

## 4. Generality and plugin architecture

### 4.1 Model to follow
DeepSeek Harness (dsh, MIT, Aug 2026) puts everything behind a capability plugin on a small Cordis kernel:
model adapters, tools, session log, agent loop, sandbox, storage, scheduling, UI. Hermes uses
`plugin.yaml` + `register(ctx)` with typed hooks. OpenClaw uses a manifest that is read without running
plugin code. (Secondary sources only: the dsh manifest format is unverified.)

For a pure-Go single binary:
- **Compiled-in registry.** `core.Register(kind, name, factory)`; the binary is built from blank imports.
- **Manifests.** Manifest-first metadata, read and validated without executing plugin code.
- **Third-party plugins.** Out-of-process over stdio JSON-RPC, MCP-compatible.
- **Single-active slots.** Memory, sandbox and default LLM have one active plugin each; tools, channels
  and sensors fan out.

### 4.2 Blockers in the current code

| Area | Evidence | Target |
|---|---|---|
| Tools have no port | The switch in `prosecutor.go:1589-1802`; the action list is hand-written in `prompts.go` | `core.Tool{Name, Schema, Run}` registry; the action list is generated from it |
| Channels have no port | `server/http.go:24-37`, `channels/telegram.go:43-69` take concrete types | `core.Channel`; Telegram is one plugin |
| One spoofed provider | `opencode_free.go` (1,160 lines), default model at `main.go:33`, router slots at `connectome.go:618` | Provider registry: OpenAI-compatible, Ollama, Anthropic. Role map (router / worker / thinker / embedder) like the 2024 `provider.py:25-130` |
| Domain content in Go | `mission_dag.go:267-273, 415-700` (S1_ABLATION, S2_V08_GAPS, Hermes/OpenClaw); `store.go:196-236` seeds; `mode3_gate.go:36-49` protected paths; `prompts.go:48-148`; `hygiene.go:423-467` | A profile directory (mission, seed skills, prompt overrides). The core ships empty plus the constitution. The mission DAG shape is generated by L2–L4, not hardcoded |
| Python required | `hygiene.go:222` embeds a ~1,500-line Python SDK; prompts require `import ace_rlm`; gates run `python3` | Core has no Python. A python-tools plugin is enabled only when `python3` exists |
| CUDA/torch GPU gate | `main.go` searches `$HOME/*/*/*/.venv/bin/python`; `scripts/gpu_gate_server.py` is found relative to CWD | Optional gate plugin; the default gate is an LLM call over the provider registry |
| Linux only | bwrap jail (`jail.go:136-225`), `/proc` reads with no `runtime.GOOS` guard (`sensorium.go:191-197`) | Keep bwrap on Linux; add a clearly labelled weaker sandbox elsewhere; guard sensors |
| No install path | `scripts/install.sh` and `.goreleaser.yml` were deleted in `8426620a` (it exists in `archive-2026-05-units`) | Restore: XDG config/data dirs, install script, release builds |
| Large files | `prosecutor.go` 1,864; `hygiene.go` 1,517; `opencode_free.go` 1,160; `connectome.go` 923; 16 files over 550 lines | Split along the new plugin ports |

`core.Sense` is defined but unused. `sensorium.go` already has a drop-in, probe-on-start pattern; reuse
that pattern for every plugin kind.

## 5. What earlier iterations had that was dropped

| From | Idea | Port? |
|---|---|---|
| 2024 hand-written (`e4bf2d0f`) | Per-layer identity prompt files plus a shared template, with TOML schema-validated outputs and bounded retry (`layers.py:132`) | Yes: JSON schema per layer, model-agnostic parsing, retry ≤3. Replaces the lenient DSML-specific parser (`prompts.go:314`) |
| 2024 | Adjacent-only bus hops (`constants/queue.py:34-50`) | Yes, a check in `Bus.Publish` |
| 2024 | Provider role stacks plus a factory per provider, with hot-reloaded config | Yes (section 4.2) |
| 2024 design doc | Local-first, under 4 GB RAM, prompts under 4k tokens, TDD, access control | Yes as acceptance constraints; access control = U1 |
| 2026-05 units | XDG paths, config resolution, `install.sh`, goreleaser, encrypted provider keys | Yes, install and config now; encrypted keys once multi-provider lands |
| 2026-05 units | 13-system agent study and providers research (`design/units/agents-study/research.md`) | Read it before designing the plugin ports |
| 2026-05 units | Graduated context compaction | Medium: needed for small local models |
| Carried forward already | 6-layer skeleton, constitution text, layer memory isolation, stop/step control, stuck detection, pluggable sensors | Keep |

## 6. Proposed v0.8 build order

0. **Urgent fixes** U1–U3 and U5; pause self-building (U4).
1. **Constitution and autonomy:** compiled-in constitution with hash, separate mission file, L1
   per-action gate, integrity watchdog. Remove every human-approval path (section 3.3).
2. **Scientist gate:** pre-registration ledger, Go-side statistic recomputation from per-item predictions,
   baselines, degeneracy and leakage controls, fresh-sandbox regeneration (section 2.3). Re-grade all
   existing results through it and mark failures as reverted.
3. **Plugin core:** registry and manifests; Tool, Channel and Provider ports; tool dispatch and the action
   list generated from the registry.
4. **Generality:** profile directory; move all domain content out of Go; Python and GPU as optional
   plugins; non-Linux guards; schema-validated layer outputs; adjacent-hop bus.
5. **Install:** XDG config, install script, release build. Acceptance: on a fresh machine with only an
   API key, `ace init && ace run` pursues an unrelated mission (for example "keep my home lab's backups
   verified") with no code changes.

Each step ships with `go test` coverage and a demo on an empty workspace with a mission that is not
about ACE.
