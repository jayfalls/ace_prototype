# ACE Prototype (`v0.7.1`) — Complete Agent-to-Agent Handoff

**Last Updated**: 2026-10-24
**Handoff File Path**: `AGENT_HANDOFF.md`

---

## 1. Repository, Workspaces & Key File Paths

### Primary Repository & Binary
- **Repository Root**: `<repo root>`
- **Git Remote**: `git@github.com:jayfalls/ace_prototype.git` (branch `main` tracking `origin/ace-v0.4`)
- **Compiled Binary**: `ace` (built via `go build -o ace ./cmd/ace`)
- **Daemon Log**: `/tmp/ace-v0_7_1.log`

### Isolated Runtime Sandbox (`.ace-sandbox/`)
> **CRITICAL USER PREFERENCE**: Host isolation is top priority. All ACE agent file and shell operations run inside `.ace-sandbox` via `os.Root` (`openat2` kernel path confinement) + `bubblewrap` (`bwrap --unshare-all --share-net`, `/home` unmounted inside jail).
- **Sandbox Root**: `.ace-sandbox`
- **SQLite State Store (WAL mode)**: `.ace-sandbox/ace.db`
- **Agent Wiki (`L1`–`L6` shared knowledge & constitutional core)**:
  - `.ace-sandbox/wiki/soul.md` — **Immutable L1 Constitutional Core** (Three Heuristic Imperatives + Universal Declaration of Human Rights + Full Autonomy Triad + singular steerable Mission). All layers `L1`–`L6` are blocked from `wiki_write` on `soul.md` / `mission.md` in `internal/cognitive/prosecutor.go`.
  - `.ace-sandbox/wiki/goals.md` — Read-only Markdown projection of the live SQLite `mission_dag_nodes` Dependency Tree (`SyncMissionDAGToGoalsWiki`).
  - `.ace-sandbox/wiki/skills/` — **29 C6 Wiki-Skills** (including 22+ autonomously distilled from Mode-3 verified trajectories via `internal/cognitive/wiki_skills.go`).
  - `.ace-sandbox/wiki/notes/` — `L2` strategic branches (`l2_strategy_branch_*.md`), `L3` capability audits, and `L4` frozen task specs (`l4_*.md`).
- **Agent Workspace (`/workspace/` inside `bwrap`)**:
  - `.ace-sandbox/workspace/ace_rlm.py` — Pre-injected Scientific SDK (`query_traces`, `load_dataset`, `build_and_register_dataset` with deterministic 75/25 SHA-256 splits, `beta_ci`, `mcnemar_exact_p`, `wilcoxon_signed_rank`, `audit_conceptual_flows`, `audit_script_mutants`).
  - `.ace-sandbox/workspace/ace_v07/` — Autonomously built & Mode-3 verified Python modules (`hermes_openclaw_ace_ablation.py`, `v08_gap_illuminator.py`, `assistant_suite.py`, `candidate_builder.py`, `budget_governor.py`, `retention_gate.py`, `self_build_verifier.py`, `proactive_assistant_loop.py`, `open_world_continual.py`, `self_improvement_ledger.py`, `self_build_loop.py`, `proactive_assistant.py`, etc.).
  - `.ace-sandbox/workspace/benchmarks/` — `registry.json`, `datasets/*.json` (13 SOTA public benchmarks + `v07`/`v08` empirical split datasets), `results/*.json` (Mode-3 verified `H1_ACCEPTED` experiment artifacts), and `live_traces.db` (mirror of `ace.db`).
  - `.ace-sandbox/workspace/research_corpus/` — 128-file research corpus (`CORPUS_INDEX.md`, domain syntheses, paper extracts, competitor teardowns including October 2026 `Hermes Agent v0.22.4` and `OpenClaw v2026.10.6`).

### Historical Decommissioned Archives (Full Postmortem Sandboxes + Trace DBs)
- `archive/v0_1_decommissioned_20261020/`
- `archive/v0_2_decommissioned_20261021/`
- `archive/v0_3_decommissioned_20261022/`
- `archive/v0_4_decommissioned_20261023/`
- `archive/v0_6_decommissioned_20261023/`
- **Original Logseq & Prior Prototypes**:
  - `~/Documents/notes/pages/ACE.md` (and `Agentic Context Engineering.md`, `AI - Agents.md`)
  - `~/programming/ace_2/` and `~/programming/ace_framework/`

---

## 2. Architecture & Specification Docs

1. **`SPEC_ACE_V0_7_1.md`** — Current active specification:
   - **Pipelined Top-Down Mission Dependency Tree (`MissionDAG`)** stored in SQLite (`mission_dag_nodes`), enforcing cryptographic provenance `L1_MISSION_ROOT -> L2_STRATEGY -> L3_CAPABILITY -> L4_TASK_SPEC -> L5 GPU Gate -> L6 Execution`.
   - **Conditional Parallel `L2 || L3 || L4` Scheduler**: Uses `sync.WaitGroup` waves (`StepMissionDAG` in `internal/cognitive/mission_dag.go`) to run only layers whose input queue has ready nodes, structurally guaranteeing the pyramidal call distribution `1 <= L1 < L2 < L3 < L4 << L6`.
2. **`.ace-sandbox/workspace/SPEC_ACE_V0_7_REWRITE.md`** — `v0.7` self-building candidate architecture, automated Wiki-Skill distillation, and 3D Isometric Cognitive Tower WebUI (`GET /api/v07/cockpit`).
3. **`.ace-sandbox/workspace/SPEC_ACE_V0_8_SELF_BUILDING.md`** — Autonomously authored `v0.8` specification produced by `Axis_11_2` (`v08_gap_illuminator.py`).
4. **`SPEC_ACE_V0_6_FINAL.md`** — Connectome Turn 0/0.5/1 context pipeline, Pluggable Proprioceptive/Exteroceptive Sensorium (24-bin Beta-Binomial circadian model from empirical `human_events`), Living Spec Gate, and Strict Two-Way Tier Isolation Law.

---

## 3. Core Go Codebase Map (`internal/`)

| File | Responsibility |
| :--- | :--- |
| `cmd/ace/main.go` | CLI flags, sandbox verification, managed child `gpu_gate_server.py` launcher, HTTP server startup (`:8080`). |
| `internal/cognitive/mission_dag.go` | **`v0.7.1` Core**: `MissionDAGNode` schema, `seedMissionDAG`, `VerifyTaskNodeProvenance`, `SyncMissionDAGToGoalsWiki`, `StepMissionDAG` (3-stage conditional parallel `L2 || L3 || L4` wave scheduler + `extractDynamicL4SpecPaths` + `L5 -> L6` dispatch). |
| `internal/cognitive/cascade.go` | Autonomous ticker loop (`autonomousTickerLoop`), `runCascade` entry point (delegates autonomous cycles to `StepMissionDAG`), and `mirrorLiveTracesDB` (gated on non-empty trace execution). |
| `internal/cognitive/prosecutor.go` | `L6` Pi-style inner loop (`runPiProsecutorLoop`), multi-action batch execution (mutations ordered before `sandbox_sh`), `#grep=<regex>` & directory `workspace_read` support, `;`-separated multi-file read splitting, constitutional immutability guard (`soul.md`/`mission.md`), and `verifyAndAutoCheckoffMilestone`. |
| `internal/cognitive/mode3_gate.go` | Deterministic **Mode-3 Falsification Gate**: artifact existence, fast in-memory Python AST non-vacuity check, `bwrap` `--selftest` execution, AST-PDG conceptual flow hygiene, SWE-Mutation falsification, JSON dataset split/hypothesis verification (`h1_accepted: true`), and `p2p` regression suite. |
| `internal/cognitive/context_build.go` | Turn 0 / Turn 0.5 / Turn 1 context compiler, Connectome ROI slice pre-injection, and `RouteTopWikiSkill` (`C6` skill injection). |
| `internal/cognitive/connectome.go` | SQLite-backed Connectome graph (`connectome_nodes`, `connectome_edges`), 2-hop downstream dependency search, RRF retrieval. |
| `internal/cognitive/wiki_skills.go` | Automated `C6` Wiki-Skill distillation from Mode-3 verified trajectories, `Beta(α, β)` posterior tracking, and retirement lifecycle. |
| `internal/cognitive/sensorium.go` | Proprioceptive host vitals + Exteroceptive sensors + 24-bin Beta-Binomial human circadian model + parametric isometric `anatomy` SVG generator for Telegram escalations. |
| `internal/llm/opencode_free.go` | OpenCode Go subscription (`opencode-go/deepseek-v4.1-flash`) & free-tier client, per-layer KV cache session IDs, 65s prefill idle timeout, `Retry-After` global cooldown, and **Strict Two-Way Tier Isolation Law** (subscription models NEVER fall back to free models). |
| `internal/server/http.go` & `web/src/routes/+page.svelte` | Glass-Box Cognitive Cockpit REST/SSE API + 3D Isometric Cognitive Tower WebUI (`http://localhost:8080`). |

---

## 4. Live Endpoints, Credentials & External Integrations

1. **WebUI & REST API (`http://localhost:8080`)**:
   - `GET http://localhost:8080/` — 3D Isometric Cognitive Cockpit WebUI
   - `GET http://localhost:8080/api/engine/status` — Live layer statuses, sandbox health, active model, and metrics
   - `GET http://localhost:8080/api/v07/cockpit` — 3D tower telemetry, Connectome nodes/edges, Wiki-Skills posteriors, and benchmark results
   - `GET http://localhost:8080/api/telemetry/thoughts?limit=50` — Recent `thought_traces`
   - `GET http://localhost:8080/api/telemetry/spans?limit=100` — Recent `telemetry_spans`
2. **Local 4-Bit GPU Decision Gate (`http://127.0.0.1:8091/gate`)**:
   - Script: `scripts/gpu_gate_server.py`
   - Python venv: `~/.claude/skills/LyteKnowledge/.venv/bin/python` (or `~/lytehouse/repos/LyteFEX/.venv/bin/python`)
   - Model: `Qwen/Qwen3.5-0.8B` 4-bit NF4 on CUDA (`~738 MiB` VRAM), with KV trunk caching (`>95%` hit rate, `<200ms` latency) for `L5` and `L6` world/decision gating.
3. **OpenCode Go Subscription Authentication**:
   - Auth file: `~/.local/share/opencode/auth.json` (auto-loaded by `internal/llm/opencode_free.go`)
   - Endpoint: `https://opencode.ai/zen/go/v1/chat/completions`
   - Primary Model: `deepseek-v4.1-flash` (`opencode-go/deepseek-v4.1-flash`)
   - Privacy setting on OpenCode Go dashboard is set to **Global**.
4. **Telegram Bot Integration**:
   - Credentials (`telegram_bot_token`, `telegram_allowed_chat_id`) are stored in `.ace-sandbox/ace.db` (`engine_meta` table) — **never hardcode secrets or chat IDs in source files or committed docs**.
   - Escalation policy: Only escalates to Telegram after `>= 5` identical `VetoReasonHash` failures (or `>= 8` for debugging/scientific falsification) and renders a parametric isometric `anatomy` SVG (`internal/cognitive/sensorium.go`).

---

## 5. Standard Operating Procedures & Commands

### A. Important Operational Rules
- **NO `systemd` Services**: Never create or enable `systemd` user/system services (`ace.service`, etc.). Always manage `./ace` and `gpu_gate_server.py` as manual background processes.
- **Process Launch (`setsid`)**: When starting `./ace` from an agent subshell, always use `setsid ./ace > /tmp/ace-v0_7_1.log 2>&1 < /dev/null &` so the daemon is detached from the tool subshell's process group.
- **Killing `./ace` Safely**: Use `pkill -9 -x ace` (exact binary name match) — **never** use `pkill -f "./ace"`, which will match and kill the invoking bash subshell itself (`exit code 137`).

### B. Build, Test & Hot-Swap Procedure
```bash
cd <repo root>

# 1. Run unit tests
go test ./internal/cognitive/... ./internal/sandbox/...

# 2. Build into a staging binary and atomic-swap
go build -o ace.new ./cmd/ace && mv -f ace.new ace

# 3. Restart the daemon cleanly (gpu_gate_server.py on :8091 stays warm)
pkill -9 -x ace 2>/dev/null || true
sleep 1
setsid ./ace > /tmp/ace-v0_7_1.log 2>&1 < /dev/null &
sleep 2
ss -tlnp | grep -E "8080|8091"
```

### C. One-Shot Live Telemetry & Mission DAG Diagnostic Command
```bash
python3 - << 'PY'
import sqlite3
conn = sqlite3.connect(".ace-sandbox/ace.db")
cur = conn.cursor()

print("=== 1. PYRAMID CALL DISTRIBUTION ===")
for r in cur.execute("""
    SELECT layer_code, count(*), round(avg(latency_ms)/1000.0,1),
           sum(prompt_tokens), sum(cached_tokens),
           round(100.0*sum(cached_tokens)/max(1,sum(prompt_tokens)),1)
    FROM thought_traces GROUP BY layer_code ORDER BY layer_code
"""):
    print(" ", r)

print("\n=== 2. MISSION DAG NODES ===")
for r in cur.execute("SELECT id, parent_id, stage, state, attempts, artifact_path FROM mission_dag_nodes ORDER BY owner_layer, id"):
    print(" ", r)

print("\n=== 3. LATEST 10 THOUGHT TRACES ===")
for r in cur.execute("SELECT rowid, layer_code, latency_ms, action_type, substr(action_target,1,50), substr(state_summary,1,90) FROM thought_traces ORDER BY rowid DESC LIMIT 10"):
    print(" ", r)
PY
```

### D. Decommissioning & Archiving a Version (When Transitioning e.g. `v0.7.1 -> v0.8`)
1. Stop `./ace` (`pkill -9 -x ace`).
2. Copy the entire `.ace-sandbox/` directory to `archive/v0_7_decommissioned_<YYYYMMDD>/ace-sandbox-v0_7-full/`.
3. Export `thought_traces` and `telemetry_spans` into `.ace-sandbox/workspace/benchmarks/ace_v0_7_traces.db` so the next generation can run cross-generation empirical ablations against historical runs.

---

## 6. Tracking: Tickets & LyteKnowledge (`kb`)

- **Active Ticket**: **`#1708`** (*"Build & Launch ACE v0.3 Scientific Research Harness + 1-Hour Live Monitoring"* — tracks the full `v0.3 -> v0.4 -> v0.6 -> v0.7.1` evolution).
  - Inspect via: `ticket list` or `ticket check 1708`
- **Key LyteKnowledge (`kb`) Notes**:
  - `kb search "ACE v0.7"` / `kb search "ACE v0.6"`
  - `ace-v0-7-1-1-hour-monitoring-9-mission-dag-mode-3-gate-bug-fixes`
  - `ace-v0-7-rewrite-architecture-self-building-wiki-skills-3d-cockpit`
  - `ace-v0-5-v0-6-composite-architecture-deterministic-connectome-spec`
  - `ace-v0-4-sota-benchmarks-multi-vertical-web-research-and-system-agnostic-architecture`

---

## 7. Current State Snapshot at Handoff (`2026-10-24`)
- **Running Processes**:
  - **Shut down at rest** (`./ace` on `:8080` and `scripts/gpu_gate_server.py` on `:8091` are both stopped).
- **Mission DAG Progress**:
  - **`18 / 18` `L4_TASK_SPEC` nodes `COMPLETED` & Mode-3 Verified** across Strategic Branches `S1`–`S9` (`169` thought traces on `opencode-go/deepseek-v4.1-flash`, **`78.9%` KV prefix cache hit rate** on `L6`, strict `1 < 9 < 10 < 32 << 117` pyramidal call hierarchy).
  - Ready to resume cleanly from `.ace-sandbox/ace.db` whenever started next.
