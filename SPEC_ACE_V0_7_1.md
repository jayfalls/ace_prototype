# `SPEC_ACE_V0_7_1.md` — Pipelined Top-Down Mission Dependency Tree & Conditional Parallel `L2 || L3 || L4`

## 1. Core Architectural Invariants
1. **Unshifting Root at `L1` (`Aspirational Layer`)**:
   - `/wiki/soul.md` holds Dave Shapiro's **Three Heuristic Imperatives** (*Reduce suffering in the universe*, *Increase prosperity in the universe*, *Increase understanding in the universe*), the **Universal Declaration of Human Rights**, the **Full Autonomy Triad** (*Self-Directing, Self-Correcting, Self-Improving*), and the **single human-steerable `# CURRENT MISSION`**.
   - `/wiki/soul.md` is strictly read-only to all agent layers (`L1`–`L6`).
2. **Hard Top-Down Provenance Gate (`L1 → L2 → L3 → L4 → L5 → L6`)**:
   - No node at layer $L_k$ can exist or execute without a valid cryptographic `parent_id` and `directive_hash` from layer $L_{k-1}$:
     $$\text{M1\_ROOT } (L_1) \longrightarrow \text{S\_BRANCH } (L_2) \longrightarrow \text{C\_CAPABILITY } (L_3) \longrightarrow \text{TASK\_SPEC } (L_4) \longrightarrow \text{GPU\_GATE } (L_5) \longrightarrow \text{PROSECUTOR } (L_6)$$
   - `VerifyTaskNodeProvenance()` verifies the entire ancestor chain (`L4 -> L3 -> L2 -> L1`) and `soul_hash` before `L5` or `L6` can execute.
3. **Conditional Parallel `L2 || L3 || L4` Waves Over the Dependency Tree**:
   - On each DAG wave, the scheduler inspects `mission_dag_nodes` for ready work in `{L2, L3, L4}` and executes all ready stages **concurrently in parallel (`sync.WaitGroup`)**:
     - **Wave 0**: `L1` anchors `M1_ROOT` (`1` call) $\rightarrow$ `L2` originates `S1_ABLATION`.
     - **Wave 1 (`L2 || L3` Parallel)**: `L2` originates `S2_V08_GAPS` **in parallel with** `L3` modeling `S1_ABLATION` (`C_S1_ABLATION_DATA` + `C_S1_ABLATION_HARNESS`).
     - **Wave 2 (`L3 || L4` Parallel)**: `L3` models `S2_V08_GAPS` and consolidates `C6 Beta(α, β)` Wiki-Skill priors **in parallel with** `L4` speccing `S1_ABLATION` into executable task nodes (`READY_FOR_L5`).
     - **Wave 3**: `L4` specs `S2_V08_GAPS` into executable task nodes (`READY_FOR_L5`).
4. **Structural Pyramidal Call Distribution (`1 <= L1 < L2 < L3 < L4 << L6`)**:
   - Because `1` `L1` Mission Root fans out into `2` `L2` Strategic Branches $\rightarrow$ `3` `L3` Capability & Skill-Prior Models $\rightarrow$ `4+` `L4` Task Specifications & Tactical Revisions $\rightarrow$ `10–50+` `L6` Workhorse Sandbox Execution turns, the call counts obey:
     $$1 \le \text{Calls}(L_1) < \text{Calls}(L_2) < \text{Calls}(L_3) < \text{Calls}(L_4) \ll \text{Calls}(L_6)$$
     while `goals.md` is automatically projected from `mission_dag_nodes`.
