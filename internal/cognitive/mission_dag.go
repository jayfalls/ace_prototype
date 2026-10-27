package cognitive

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"ace/internal/config"
	"ace/internal/constitution"
	"ace/internal/contextsys"
	"ace/internal/core"
	"ace/internal/gate"
	"ace/internal/plan"
	"ace/internal/science"
	"ace/internal/tasks"
)

// MissionDAGNode represents a single node in the strict Top-Down Dependency Tree:
// L1_MISSION_ROOT -> L2_STRATEGY -> L3_CAPABILITY -> L4_TASK_SPEC -> (L5 GPU Gate -> L6 Execution).
type MissionDAGNode struct {
	ID            string  `json:"id"`
	ParentID      string  `json:"parent_id"`
	Stage         string  `json:"stage"`          // L1_MISSION_ROOT | L2_STRATEGY | L3_CAPABILITY | L4_TASK_SPEC
	State         string  `json:"state"`          // L1_ANCHORED | NEEDS_L3_MODEL | L3_MODELED | NEEDS_L4_SPEC | L4_SPECCED | READY_FOR_L5 | TACTICAL_STALL | NEEDS_L3_DISTILL | QUARANTINED | UNVERIFIED | COMPLETED
	OwnerLayer    int     `json:"owner_layer"`    // 1, 2, 3, 4
	Title         string  `json:"title"`          // Concise human-readable title
	Directive     string  `json:"directive"`      // Full layer directive / spec
	ArtifactPath  string  `json:"artifact_path"`  // Target module / result JSON path for L4_TASK_SPEC
	DirectiveHash string  `json:"directive_hash"` // SHA-256 provenance hash of (ParentHash + Directive)
	SoulHash      string  `json:"soul_hash"`      // SHA-256 of /wiki/soul.md when anchored
	Attempts      int     `json:"attempts"`       // L6 execution attempts on this leaf
	Score         float64 `json:"score"`          // 8.3: imperative score — L2 rank / L4 priority weight
	UpdatedAt     string  `json:"updated_at"`
}

func (e *Engine) initMissionDAGSchema(db *sql.DB) {
	if db == nil {
		return
	}
	_, _ = db.Exec(`
		CREATE TABLE IF NOT EXISTS mission_dag_nodes (
			id TEXT PRIMARY KEY,
			parent_id TEXT NOT NULL DEFAULT '',
			stage TEXT NOT NULL,
			state TEXT NOT NULL,
			owner_layer INTEGER NOT NULL,
			title TEXT NOT NULL,
			directive TEXT NOT NULL,
			artifact_path TEXT NOT NULL DEFAULT '',
			directive_hash TEXT NOT NULL,
			soul_hash TEXT NOT NULL,
			attempts INTEGER NOT NULL DEFAULT 0,
			score REAL NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_mission_dag_stage_state ON mission_dag_nodes(stage, state);
		CREATE INDEX IF NOT EXISTS idx_mission_dag_parent ON mission_dag_nodes(parent_id);
		CREATE TABLE IF NOT EXISTS dag_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			node_id TEXT NOT NULL,
			from_state TEXT NOT NULL,
			to_state TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		);
		CREATE TRIGGER IF NOT EXISTS dag_state_transition AFTER UPDATE OF state ON mission_dag_nodes
		WHEN OLD.state != NEW.state
		BEGIN
			INSERT INTO dag_events (node_id, from_state, to_state) VALUES (OLD.id, OLD.state, NEW.state);
		END;
	`)
	_, _ = db.Exec(`ALTER TABLE mission_dag_nodes ADD COLUMN score REAL NOT NULL DEFAULT 0`)
	_, _ = db.Exec(`ALTER TABLE mission_dag_nodes ADD COLUMN retired_reason TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE mission_dag_nodes ADD COLUMN reanchor_count INTEGER NOT NULL DEFAULT 0`)
	_ = science.Migrate(db)
	_ = tasks.Migrate(db)
	_ = contextsys.MigrateInjectionLog(db)
	_ = contextsys.MigrateMap(db)
}

func sha256Str(s string) string {
	return sha256Hex([]byte(s))
}

// dagTerminal reports whether a node state is finished work — terminal nodes
// never count as open for horizon expansion (STALE/RETIRED are dead work,
// DUPLICATE_OF never executes, QUARANTINED is parked by a veto ruling and
// every selector skips it, UNVERIFIED finished with inconclusive mutation
// evidence — terminal but excluded from success metrics and claims).
func dagTerminal(s string) bool {
	return s == "COMPLETED" || s == "STALE" || s == "RETIRED" || s == "DUPLICATE_OF" || s == "QUARANTINED" || s == "UNVERIFIED"
}

// currentSoulHash anchors DAG provenance to the pinned constitution plus the
// owner-set mission — any mission re-steer produces a new hash, invalidating
// all prior anchors (plan 8.1).
func (e *Engine) currentSoulHash() string {
	return sha256Str(constitution.Text() + "\n---\n" + e.currentMission())[:16]
}

// currentMission returns the owner-set life mission from config ("" = idle).
func (e *Engine) currentMission() string {
	return config.LoadMission()
}

// markStaleOnMissionReload implements `ace mission reload` semantics (plan 8.1):
// when the anchor hash changes, every open plan node is marked STALE so L1
// re-anchors and the tree replans under the new mission version.
func (e *Engine) markStaleOnMissionReload(ctx context.Context, soulHash string) {
	db := e.getDB()
	if db == nil {
		return
	}
	last, _ := e.memory.LoadEngineMeta(ctx, "anchored_soul_hash")
	if last == soulHash {
		return
	}
	_, _ = db.ExecContext(ctx, `UPDATE mission_dag_nodes SET state='STALE', updated_at=? WHERE soul_hash != ? AND state NOT IN ('COMPLETED','STALE','RETIRED','DUPLICATE_OF','QUARANTINED','UNVERIFIED')`,
		time.Now().UTC().Format(time.RFC3339), soulHash)
	_ = e.memory.SaveEngineMeta(ctx, "anchored_soul_hash", soulHash)
}

// staleVerdict routes a stale-node re-anchor through the L1 gate (spec 9.0:
// re-anchoring needs a Tier-1 verdict, not a silent reset).
func (e *Engine) staleVerdict(ctx context.Context, nodeID, title string) (bool, string) {
	if e.gate == nil {
		return true, "gate absent"
	}
	d := e.gate.Ruling(ctx, gate.Action{
		Kind: "plan_reanchor", Target: nodeID,
		Content: "Re-anchor stale plan node under the current mission: " + title,
		Layer:   1,
	})
	return d.Allowed(), d.Reason
}

func (e *Engine) LoadMissionDAGNodes(ctx context.Context) []MissionDAGNode {
	db := e.getDB()
	if db == nil {
		return nil
	}
	e.initMissionDAGSchema(db)
	rows, err := db.QueryContext(ctx, `
		SELECT id, parent_id, stage, state, owner_layer, title, directive, artifact_path, directive_hash, soul_hash, attempts, score, updated_at
		FROM mission_dag_nodes
		ORDER BY owner_layer ASC, id ASC
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []MissionDAGNode
	for rows.Next() {
		var n MissionDAGNode
		if rows.Scan(&n.ID, &n.ParentID, &n.Stage, &n.State, &n.OwnerLayer, &n.Title, &n.Directive, &n.ArtifactPath, &n.DirectiveHash, &n.SoulHash, &n.Attempts, &n.Score, &n.UpdatedAt) == nil {
			out = append(out, n)
		}
	}
	return out
}

func (e *Engine) upsertMissionDAGNode(ctx context.Context, n MissionDAGNode) {
	db := e.getDB()
	if db == nil {
		return
	}
	e.initMissionDAGSchema(db)
	now := time.Now().UTC().Format(time.RFC3339)
	if n.UpdatedAt == "" {
		n.UpdatedAt = now
	}
	_, _ = db.ExecContext(ctx, `
		INSERT INTO mission_dag_nodes (id, parent_id, stage, state, owner_layer, title, directive, artifact_path, directive_hash, soul_hash, attempts, score, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			state = excluded.state,
			title = excluded.title,
			directive = excluded.directive,
			artifact_path = excluded.artifact_path,
			directive_hash = excluded.directive_hash,
			soul_hash = excluded.soul_hash,
			attempts = excluded.attempts,
			score = excluded.score,
			updated_at = excluded.updated_at
	`, n.ID, n.ParentID, n.Stage, n.State, n.OwnerLayer, n.Title, n.Directive, n.ArtifactPath, n.DirectiveHash, n.SoulHash, n.Attempts, n.Score, n.UpdatedAt)

	// Mirror into connectome_nodes + connectome_edges so the 3D WebUI & Turn 0/0.5 Connectome visualize the live Dependency Tree
	_, _ = db.ExecContext(ctx, `
		INSERT INTO connectome_nodes (id, kind, path, summary, state, owner_layer, blocking_weight, info_value, expected_cost, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			summary = excluded.summary,
			state = excluded.state,
			owner_layer = excluded.owner_layer,
			updated_at = excluded.updated_at
	`, "dag:"+n.ID, n.Stage, n.ArtifactPath, truncateText(n.Title+" — "+n.Directive, 220), n.State, fmt.Sprintf("L%d", n.OwnerLayer), 1.5, 1.4, 0.35, now)
	if n.ParentID != "" {
		_, _ = db.ExecContext(ctx, `
			INSERT INTO connectome_edges (src, dst, kind, weight, updated_at)
			VALUES (?, ?, 'DAG_CHILD', 1.5, ?)
			ON CONFLICT(src, dst, kind) DO UPDATE SET updated_at = excluded.updated_at
		`, "dag:"+n.ParentID, "dag:"+n.ID, now)
	}
}

// VerifyTaskNodeProvenance enforces the Hard Top-Down Causality Law:
// An L4_TASK_SPEC node is ONLY eligible for L5/L6 execution if its entire ancestor chain
// (L4_TASK_SPEC -> L3_CAPABILITY -> L2_STRATEGY -> L1_MISSION_ROOT) exists with valid hashes and matches currentSoulHash().
func (e *Engine) VerifyTaskNodeProvenance(ctx context.Context, taskNodeID string) (bool, string) {
	nodes := e.LoadMissionDAGNodes(ctx)
	byID := make(map[string]MissionDAGNode, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}
	soulHash := e.currentSoulHash()
	l4, ok := byID[taskNodeID]
	if !ok || l4.Stage != "L4_TASK_SPEC" || l4.DirectiveHash == "" {
		return false, "missing L4_TASK_SPEC node"
	}
	l3, ok := byID[l4.ParentID]
	if !ok || l3.Stage != "L3_CAPABILITY" || l3.DirectiveHash == "" {
		return false, "missing parent L3_CAPABILITY provenance"
	}
	l2, ok := byID[l3.ParentID]
	if !ok || l2.Stage != "L2_STRATEGY" || l2.DirectiveHash == "" {
		return false, "missing grandparent L2_STRATEGY provenance"
	}
	l1, ok := byID[l2.ParentID]
	if !ok || l1.Stage != "L1_MISSION_ROOT" || l1.DirectiveHash == "" || l1.SoulHash != soulHash {
		return false, "missing or stale root L1_MISSION_ROOT provenance"
	}
	return true, fmt.Sprintf("L1(%s)->L2(%s)->L3(%s)->L4(%s)", l1.ID, l2.ID, l3.ID, l4.ID)
}

// SyncMissionDAGToGoalsWiki projects the live Top-Down Dependency Tree into /wiki/goals.md
// so the human operator, the 3D Cockpit, and Mode-3 verification see the exact tree originated by L1 -> (L2 || L3 || L4).
func (e *Engine) SyncMissionDAGToGoalsWiki(ctx context.Context) {
	nodes := e.LoadMissionDAGNodes(ctx)
	if len(nodes) == 0 {
		return
	}
	var sb strings.Builder
	sb.WriteString("# ACE Live Mission Dependency Tree (Originated by L1 → L2 || L3 || L4)\n\n")
	for _, n := range nodes {
		if n.Stage == "L1_MISSION_ROOT" {
			sb.WriteString(fmt.Sprintf("## [L1 ASPIRATIONAL ROOT] `%s` (Soul Hash: `%s`, Provenance: `%s`)\n- **Mission Frame**: %s\n\n", n.ID, n.SoulHash, truncateText(n.DirectiveHash, 10), n.Directive))
		}
	}
	// Active tasks come FIRST: ReadWiki truncates at 128KB and the
	// L2/L3 history grows forever — milestones past the cap are
	// invisible to extractTopActionableMilestone, which silently
	// skips the mode-3 gate (empty_milestone) and never checks off.
	sb.WriteString("\n## Active Executable Task Specifications (`L4 → L5 → L6`)\n\n")
	// Open milestones first: 90+ completed [x] lines otherwise push the
	// actionable - [ ] past ReadWiki's 128KB cap (empty_milestone again).
	var open, closed []MissionDAGNode
	for _, n := range nodes {
		if n.Stage != "L4_TASK_SPEC" {
			continue
		}
		if n.State == "COMPLETED" || n.State == "RETIRED" {
			closed = append(closed, n)
		} else {
			open = append(open, n)
		}
	}
	writeTask := func(n MissionDAGNode, directive string) {
		box := "[ ]"
		switch n.State {
		case "COMPLETED":
			box = "[x]"
		case "UNVERIFIED":
			box = "[~]"
		case "QUARANTINED":
			box = "[!]"
		}
		sb.WriteString(fmt.Sprintf("- %s **%s — %s** (Parent: `%s`, Provenance: `%s`): %s\n", box, n.ID, n.Title, n.ParentID, truncateText(n.DirectiveHash, 10), directive))
	}
	for _, n := range open {
		writeTask(n, collapseMultilineMilestoneBullets(n.Directive))
	}
	sb.WriteString("\n### Completed history (read-only — open work is above)\n")
	for _, n := range closed {
		writeTask(n, collapseMultilineMilestoneBullets(n.Directive))
	}
	sb.WriteString("\n## Strategic & Capability Dependency Tree (`L2 || L3`)\n")
	for _, n := range nodes {
		switch n.Stage {
		case "L2_STRATEGY":
			sb.WriteString(fmt.Sprintf("- **[L2 STRATEGY `%s` | `%s`]** (Parent: `%s`, Hash: `%s`): **%s** — %s\n", n.ID, n.State, n.ParentID, truncateText(n.DirectiveHash, 8), n.Title, truncateText(n.Directive, 220)))
		case "L3_CAPABILITY":
			sb.WriteString(fmt.Sprintf("  - **[L3 CAPABILITY MODEL `%s` | `%s`]** (Parent: `%s`, Hash: `%s`): **%s** — %s\n", n.ID, n.State, n.ParentID, truncateText(n.DirectiveHash, 8), n.Title, truncateText(n.Directive, 220)))
		}
	}
	_ = e.memory.WriteWiki("goals.md", sb.String())
}

// ActiveReadyTaskNode returns the first L4_TASK_SPEC node in READY_FOR_L5 or TACTICAL_STALL that passes full L1->L2->L3->L4 provenance.
func (e *Engine) ActiveReadyTaskNode(ctx context.Context) (*MissionDAGNode, bool) {
	nodes := e.LoadMissionDAGNodes(ctx)
	var best *MissionDAGNode
	for i := range nodes {
		n := &nodes[i]
		if n.Stage != "L4_TASK_SPEC" || (n.State != "READY_FOR_L5" && n.State != "TACTICAL_STALL") {
			continue
		}
		if ok, _ := e.VerifyTaskNodeProvenance(ctx, n.ID); !ok {
			continue
		}
		// Least-recently-dispatched first: LRU over the ready set, with
		// fewest-attempts and imperative score as tie-breaks only.
		// Attempts-first ordering starved vetoed nodes — a continuous
		// inflow of attempts=0 siblings (e.g. an L2 strategy flood) meant
		// retryable nodes never reached a second dispatch, never hit
		// TACTICAL_STALL, and never escalated to L4 re-spec.
		if best == nil || n.UpdatedAt < best.UpdatedAt ||
			(n.UpdatedAt == best.UpdatedAt && (n.Attempts < best.Attempts ||
				(n.Attempts == best.Attempts && n.Score > best.Score))) {
			best = n
		}
	}
	if best == nil {
		return nil, false
	}
	return best, true
}

// MarkDAGTaskVerified transitions the verified L4_TASK_SPEC node to COMPLETED.
// (Post-task C6 Wiki-Skill distillation is already performed deterministically by DistillAndRegisterWikiSkill.)
func (e *Engine) MarkDAGTaskVerified(ctx context.Context, milestoneLine string, mutationInconclusive bool) {
	nodes := e.LoadMissionDAGNodes(ctx)
	newState := "COMPLETED"
	if mutationInconclusive {
		// Inconclusive mutation evidence can never be called verified:
		// the task is terminal but excluded from task_success, the
		// primary metric and science claims.
		newState = "UNVERIFIED"
	}
	for _, n := range nodes {
		if n.Stage == "L4_TASK_SPEC" && n.State != "COMPLETED" && n.State != "UNVERIFIED" {
			if strings.Contains(milestoneLine, n.ID) || (n.ArtifactPath != "" && strings.Contains(milestoneLine, n.ArtifactPath)) {
				n.State = newState
				n.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
				e.upsertMissionDAGNode(ctx, n)
				// 8.3: record whether the result moved the branch's primary
				// statistic — feeds diminishing-returns (3 zeros = stop).
				// UNVERIFIED results are excluded: they moved nothing.
				if db := e.getDB(); db != nil {
					if n.State == "COMPLETED" {
						science.RecordChildResult(ctx, db, branchOf(n), n.ID, e.resultMovedStat(n))
					}
					// 8.6: regrade the result through the scientist gate —
					// declared booleans ignored, stats recomputed into
					// science_verdicts (the row is the audit trail).
					if wsDir, _ := e.resolveSandboxDirs(); wsDir != "" {
						stem := strings.ToLower(strings.TrimPrefix(n.ID, "T_"))
						for _, rel := range []string{"results/" + stem + ".json", "results/" + strings.TrimSuffix(stem, "_data") + ".json"} {
							blob, err := os.ReadFile(filepath.Join(wsDir, ResolveWorkspaceArtifactPath(wsDir, rel)))
							if err != nil {
								continue
							}
							if _, _, rgErr := science.RegradeResultJSON(ctx, db, blob, n.ID); rgErr != nil && e.telem != nil {
								_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
									TraceID: "regrade", AgentID: e.agentID, LayerCode: "L6",
									Operation: "science_regrade_error", Status: "error",
									Detail: n.ID + ": " + truncateText(rgErr.Error(), 200),
								})
							}
							break
						}
					}
				}
				break
			}
		}
	}
	e.SyncMissionDAGToGoalsWiki(ctx)
}

// ParkDAGTaskForMilestone quarantines every non-terminal L4_TASK_SPEC node
// whose ID or artifact path appears in the milestone text. QUARANTINED is a
// real DAG state: every selector picks by explicit state, dagTerminal keeps
// it out of the open-node counts, and the stale sweep never resurrects it.
// This is the shared parking path — the veto ledger's bounded-retry rule
// (ruled + 3 identical vetoes) and a denied L1 ruling both land here.
func (e *Engine) ParkDAGTaskForMilestone(ctx context.Context, milestoneID string) int {
	parked := 0
	for _, n := range e.LoadMissionDAGNodes(ctx) {
		if n.Stage != "L4_TASK_SPEC" || dagTerminal(n.State) {
			continue
		}
		if strings.Contains(milestoneID, n.ID) || (n.ArtifactPath != "" && strings.Contains(milestoneID, n.ArtifactPath)) {
			n.State = "QUARANTINED"
			n.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			e.upsertMissionDAGNode(ctx, n)
			parked++
		}
	}
	if parked > 0 {
		e.SyncMissionDAGToGoalsWiki(ctx)
	}
	return parked
}

// RecordDAGTaskAttempt increments the attempt counter on the active L4_TASK_SPEC node and transitions it to TACTICAL_STALL
// when attempts >= 2 so L4 wakes up in the parallel cluster to revise the task specification.
func (e *Engine) RecordDAGTaskAttempt(ctx context.Context) {
	if active, ok := e.ActiveReadyTaskNode(ctx); ok {
		active.Attempts++
		active.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if active.Attempts >= 2 {
			active.State = "TACTICAL_STALL"
		}
		e.upsertMissionDAGNode(ctx, *active)
	}
}

// StepMissionDAG executes the Pipelined Top-Down Dependency Tree with Conditional Parallel L2||L3||L4 Waves:
//   - Genesis Gate: L1 (Aspirational) MUST run first (`M1_ROOT`, 1 call) before any L2..L6 node can exist.
//   - Pipelined Parallel Waves (`L2 || L3 || L4`):
//   - Each wave inspects mission_dag_nodes and runs whatever is ready, concurrently:
//     L2 proposes a new strategic branch under the mission root, L3 models any branch
//     in NEEDS_L3_MODEL into capability sub-nodes, and L4 specs NEEDS_L4_SPEC
//     capabilities into executable tasks (`READY_FOR_L5`).
//   - The DAG shape is generated by L2–L4 from the mission — never hardcoded.
//   - Structural call distribution: `1 <= L1 < L2 < L3 < L4 << L6`, with zero possibility
//     of any layer running before its top-down parent.
func (e *Engine) StepMissionDAG(ctx context.Context, cycleID string, cycleSpanID string, stimulus string) []core.ThoughtTrace {
	soulHash := e.currentSoulHash()
	missionText := e.currentMission()
	if missionText == "" {
		return nil // no mission anchored: idle until the owner sets one (plan 8.1)
	}
	e.markStaleOnMissionReload(ctx, soulHash)
	if db := e.getDB(); db != nil {
		_, _ = plan.SweepStale(ctx, db, soulHash, e.staleVerdict)
	}
	e.maybeAskOwnerQuestion(ctx)
	e.maybeNotifyOwnerResults(ctx)
	e.drainOwnerOffers(ctx)
	nodes := e.LoadMissionDAGNodes(ctx)

	var l1Root *MissionDAGNode
	for i := range nodes {
		if nodes[i].Stage == "L1_MISSION_ROOT" && nodes[i].SoulHash == soulHash {
			nCopy := nodes[i]
			l1Root = &nCopy
			break
		}
	}

	// =========================================================================
	// STAGE 1: HARD TOP-DOWN L1 ASPIRATIONAL GENESIS GATE
	// Nothing in L2..L6 can ever execute until L1 has evaluated # CURRENT MISSION + Heuristic Imperatives!
	// =========================================================================
	if l1Root == nil {
		l1Prompt := fmt.Sprintf(
			"L1 ASPIRATIONAL GENESIS: Evaluate the owner's CURRENT MISSION (%q) against the Three Heuristic Imperatives (Reduce suffering, Increase prosperity, Increase understanding in the universe) and the Universal Declaration of Human Rights. Emit your authoritative top-down constitutional and mission directive on southbound_directive to anchor the Mission Dependency Tree for Layer 2 (Global Strategy).",
			missionText,
		)
		tr, err := e.executeLayerStep(ctx, cycleID, core.LayerAspirational, l1Prompt)
		if err != nil {
			return nil
		}
		dir := strings.TrimSpace(tr.Southbound)
		if dir == "" {
			dir = tr.StateSummary
		}
		rootNode := MissionDAGNode{
			ID:            "M1_ROOT",
			ParentID:      "",
			Stage:         "L1_MISSION_ROOT",
			State:         "L1_ANCHORED",
			OwnerLayer:    1,
			Title:         "Constitutional Mission Root (L1 Aspirational)",
			Directive:     dir,
			DirectiveHash: sha256Str(soulHash + "|" + dir)[:16],
			SoulHash:      soulHash,
		}
		e.upsertMissionDAGNode(ctx, rootNode)
		_ = e.memory.SaveEngineMeta(ctx, "l1_grounded_soul_hash", soulHash)
		l1Root = &rootNode
		e.SyncMissionDAGToGoalsWiki(ctx)
	}

	// =========================================================================
	// STAGE 2: PIPELINED PARALLEL L2 || L3 || L4 DEPENDENCY TREE WAVES
	// =========================================================================
	for wave := 0; wave < 4; wave++ {
		nodes = e.LoadMissionDAGNodes(ctx)
		var l2Nodes, l3Nodes, l4Nodes []MissionDAGNode
		for _, n := range nodes {
			switch n.Stage {
			case "L2_STRATEGY":
				l2Nodes = append(l2Nodes, n)
			case "L3_CAPABILITY":
				l3Nodes = append(l3Nodes, n)
			case "L4_TASK_SPEC":
				l4Nodes = append(l4Nodes, n)
			}
		}

		// 2. Check if L3 has a parent L2_STRATEGY node in NEEDS_L3_MODEL or a capability node in NEEDS_L3_DISTILL
		var targetL2ForL3 *MissionDAGNode
		var targetL3ForDistill *MissionDAGNode
		for i := range l2Nodes {
			if l2Nodes[i].State == "NEEDS_L3_MODEL" {
				nCopy := l2Nodes[i]
				targetL2ForL3 = &nCopy
				break
			}
		}
		if targetL2ForL3 == nil {
			for i := range l3Nodes {
				if l3Nodes[i].State == "NEEDS_L3_DISTILL" {
					nCopy := l3Nodes[i]
					targetL3ForDistill = &nCopy
					break
				}
			}
		}
		l3NeedsRun := targetL2ForL3 != nil || targetL3ForDistill != nil

		// 3. Check if L4 has any L3_CAPABILITY nodes in NEEDS_L4_SPEC or an L4_TASK_SPEC in TACTICAL_STALL
		var pendingL3ForL4 []MissionDAGNode
		var targetStalledL4 *MissionDAGNode
		openL4Count := 0
		for i := range l3Nodes {
			if l3Nodes[i].State == "NEEDS_L4_SPEC" {
				// 8.3 diminishing returns: a branch whose last 3 child results
				// all failed to move the primary statistic stops spawning.
				if db := e.getDB(); db != nil && science.BranchStalled(ctx, db, l3Nodes[i].ParentID) {
					continue
				}
				pendingL3ForL4 = append(pendingL3ForL4, l3Nodes[i])
			}
		}
		for i := range l4Nodes {
			// Stalled/stale tasks must not freeze L2 horizon expansion — a stall
			// is revised by this wave's L4 pass and stale nodes are dead work
			// from a prior mission version; one unvetoable spec must not hold
			// the whole mission hostage (observed live: T_C_S2_HARNESS churning).
			if !dagTerminal(l4Nodes[i].State) && l4Nodes[i].State != "TACTICAL_STALL" {
				openL4Count++
			}
			if l4Nodes[i].State == "TACTICAL_STALL" {
				nCopy := l4Nodes[i]
				targetStalledL4 = &nCopy
				break
			}
		}
		l4NeedsRun := len(pendingL3ForL4) > 0 || targetStalledL4 != nil

		// Branch completion: a strategy branch is done once its L4 subtree is fully
		// COMPLETED. The 12-branch cap bounds *concurrent open* branches — a
		// completed branch frees its slot so a long-running mission keeps
		// expanding instead of dying at the cap (observed live: 12 branches, 24
		// completed tasks, zero model calls — permanent idle).
		l3ToL2 := make(map[string]string, len(l3Nodes))
		for i := range l3Nodes {
			l3ToL2[l3Nodes[i].ID] = l3Nodes[i].ParentID
		}
		l2HasTasks := make(map[string]bool, len(l2Nodes))
		l2Open := make(map[string]bool, len(l2Nodes))
		l2Completed := make(map[string]bool, len(l2Nodes))
		for i := range l4Nodes {
			l2id, ok := l3ToL2[l4Nodes[i].ParentID]
			if !ok {
				continue
			}
			l2HasTasks[l2id] = true
			if !dagTerminal(l4Nodes[i].State) {
				l2Open[l2id] = true
			}
			if l4Nodes[i].State == "COMPLETED" {
				l2Completed[l2id] = true
			}
		}
		openBranchCount := 0
		for i := range l2Nodes {
			n := &l2Nodes[i]
			if n.State == "STALE" || n.State == "RETIRED" || n.State == "DUPLICATE_OF" {
				continue // dead/duplicate branch — not open
			}
			if l2HasTasks[n.ID] && !l2Open[n.ID] {
				// All tasks terminal. A branch is COMPLETED only if at
				// least one task verified — an all-parked/all-unverified
				// subtree parks the branch, it never completes.
				want := "QUARANTINED"
				if l2Completed[n.ID] {
					want = "COMPLETED"
				}
				if n.State != want {
					n.State = want
					n.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
					e.upsertMissionDAGNode(ctx, *n)
				}
				continue
			}
			openBranchCount++
		}

		// 1. Check if L2 has a strategic branch to originate under M1_ROOT:
		//    - Either during the initial bootstrap (<2 branches), OR
		//    - Horizon Expansion when no work is actively in-flight. A TACTICAL_STALL
		//      is revision-loop churn handled by this wave's L4 pass, not a blocker.
		l2NeedsRun := len(l2Nodes) < 2 || (!l3NeedsRun && len(pendingL3ForL4) == 0 && openL4Count == 0 && openBranchCount < 12)

		if !l2NeedsRun && !l3NeedsRun && !l4NeedsRun {
			break
		}

		parStart := time.Now()
		var activeWaveLayers []string
		if l2NeedsRun {
			activeWaveLayers = append(activeWaveLayers, "L2")
		}
		if l3NeedsRun {
			activeWaveLayers = append(activeWaveLayers, "L3")
		}
		if l4NeedsRun {
			activeWaveLayers = append(activeWaveLayers, "L4")
		}
		e.recordGlobalLoop("swarm_coordinator", "active", fmt.Sprintf("DAG Wave #%d running %s concurrently", wave+1, strings.Join(activeWaveLayers, " || ")))

		var wg sync.WaitGroup

		if l2NeedsRun {
			wg.Add(1)
			go func(root MissionDAGNode, branchIdx int, existing []MissionDAGNode) {
				defer wg.Done()
				var prior []string
				for _, b := range existing {
					prior = append(prior, b.ID+": "+b.Title)
				}
				branchID := fmt.Sprintf("S%d", branchIdx+1)
				// 8.7: recent scientist-gate conclusions ride in L2's context;
				// every 10th branch is reserved for the highest-VOI open
				// uncertainty (exploration share of the budget).
				var verdicts []string
				if db := e.getDB(); db != nil {
					if vrows, verr := db.QueryContext(ctx, `SELECT ref, tier FROM science_verdicts ORDER BY id DESC LIMIT 5`); verr == nil {
						for vrows.Next() {
							var r, t string
							if vrows.Scan(&r, &t) == nil {
								verdicts = append(verdicts, r+"="+t)
							}
						}
						vrows.Close()
					}
				}
				explore := ""
				if (branchIdx+1)%10 == 0 {
					if db := e.getDB(); db != nil {
						var uid, usum string
						if db.QueryRowContext(ctx, `SELECT id, summary FROM connectome_nodes WHERE state IN ('OPEN','PROBING') AND kind IN ('UNCERTAINTY','CONTRADICTION') ORDER BY (blocking_weight*info_value)/MAX(expected_cost,0.05) DESC LIMIT 1`).Scan(&uid, &usum) == nil {
							explore = fmt.Sprintf(" EXPLORATION BUDGET: this branch targets least-explored uncertainty %s (%s) — investigate it directly rather than extending prior work.", uid, truncateText(usum, 120))
						}
					}
				}
				vline := ""
				if len(verdicts) > 0 {
					vline = " Recent gate verdicts: " + truncateText(strings.Join(verdicts, "; "), 200) + "."
				}
				l2Prompt := fmt.Sprintf(
					"L2 GLOBAL STRATEGY [Branch %s under L1 Root %s (%s)]: Under L1's directive (%q), formulate Strategic Branch %s as a falsifiable hypothesis serving the mission — expected outcome, primary approach, and how it differs from prior branches (%s).%s%s Emit the branch directive on southbound_directive.",
					branchID, root.ID, truncateText(root.DirectiveHash, 8), truncateText(root.Directive, 240), branchID, truncateText(strings.Join(prior, "; "), 600), vline, explore,
				)
				tr, err := e.executeLayerStep(ctx, cycleID, core.LayerGlobalStrategy, l2Prompt)
				if err != nil {
					return
				}
				dir := firstNonEmpty(strings.TrimSpace(tr.Southbound), tr.StateSummary)
				if dir == "" {
					return
				}
				title := strings.TrimSpace(strings.SplitN(dir, "\n", 2)[0])
				// 9.0 duplicate rejection: shingle Jaccard >= 0.8 OR artefact-set
				// overlap >= 0.5 vs any existing strategy marks the node
				// DUPLICATE_OF instead of silently dropping it.
				ex := make([]plan.Strategy, 0, len(existing))
				for _, n := range existing {
					ex = append(ex, plan.Strategy{ID: n.ID, Title: n.Title, Directive: n.Directive})
				}
				if dupID, dup := plan.DuplicateOf(title, dir, ex); dup {
					e.upsertMissionDAGNode(ctx, MissionDAGNode{
						ID: branchID, ParentID: root.ID, Stage: "L2_STRATEGY", State: "DUPLICATE_OF",
						OwnerLayer: 2, Title: truncateText(title, 100), Directive: dir,
						DirectiveHash: sha256Str(root.DirectiveHash + "|" + branchID + "|" + dir)[:16],
						SoulHash:      root.SoulHash,
					})
					if db := e.getDB(); db != nil {
						_, _ = db.ExecContext(ctx, `UPDATE mission_dag_nodes SET retired_reason=? WHERE id=?`, "duplicates "+dupID, branchID)
						_, _ = db.ExecContext(ctx, `INSERT INTO veto_ledger (veto_hash, milestone_id, veto_kind, target_path, count, last_reason, last_cycle_id, disputed, updated_at)
							VALUES (?, ?, 'duplicate_strategy', ?, 1, ?, ?, 0, ?) ON CONFLICT(veto_hash) DO NOTHING`,
							sha256Str(branchID + dir)[:16], branchID, dupID,
							truncateString("duplicates "+dupID, 300), cycleID, time.Now().UTC())
					}
					return
				}
				e.upsertMissionDAGNode(ctx, MissionDAGNode{
					ID:            branchID,
					ParentID:      root.ID,
					Stage:         "L2_STRATEGY",
					State:         "NEEDS_L3_MODEL",
					OwnerLayer:    2,
					Title:         truncateText(title, 100),
					Directive:     dir,
					DirectiveHash: sha256Str(root.DirectiveHash + "|" + branchID + "|" + dir)[:16],
					SoulHash:      root.SoulHash,
					Score:         science.ImperativeScore(dir),
				})
			}(*l1Root, len(l2Nodes), l2Nodes)
		}

		if l3NeedsRun {
			wg.Add(1)
			go func(l2Target *MissionDAGNode, l3Distill *MissionDAGNode) {
				defer wg.Done()
				if l2Target != nil {
					l3Prompt := fmt.Sprintf(
						"L3 AGENT MODEL [Parent L2 Node %s (%s)]: Audit current capabilities, wiki skills (/wiki/skills/), and available datasets for Strategic Branch %q (%s). Emit the capability & dataset model on southbound_directive for Layer 4.",
						l2Target.ID, truncateText(l2Target.DirectiveHash, 8), l2Target.Title, truncateText(l2Target.Directive, 240),
					)
					tr, err := e.executeLayerStep(ctx, cycleID, core.LayerAgentModel, l3Prompt)
					if err != nil {
						return
					}
					dir := firstNonEmpty(strings.TrimSpace(tr.Southbound), tr.StateSummary)
					l2Target.State = "L3_MODELED"
					e.upsertMissionDAGNode(ctx, *l2Target)

					// Emit 2 capability sub-nodes (DATA + HARNESS) in NEEDS_L4_SPEC so L4 executes 2 distinct Spec calls per L3 call,
					// guaranteeing Calls(L1)=1 < Calls(L2)=2 < Calls(L3)=3 < Calls(L4)=4+ << Calls(L6)!
					c1ID := "C_" + l2Target.ID + "_DATA"
					c1State := "NEEDS_L4_SPEC"
					e.upsertMissionDAGNode(ctx, MissionDAGNode{
						ID:            c1ID,
						ParentID:      l2Target.ID,
						Stage:         "L3_CAPABILITY",
						State:         c1State,
						OwnerLayer:    3,
						Title:         "Dataset & C6 Skill Prior Binding for " + l2Target.ID,
						Directive:     dir,
						DirectiveHash: sha256Str(l2Target.DirectiveHash + "|" + c1ID + "|" + dir)[:16],
						SoulHash:      l2Target.SoulHash,
					})
					c2ID := "C_" + l2Target.ID + "_HARNESS"
					e.upsertMissionDAGNode(ctx, MissionDAGNode{
						ID:            c2ID,
						ParentID:      l2Target.ID,
						Stage:         "L3_CAPABILITY",
						State:         "NEEDS_L4_SPEC",
						OwnerLayer:    3,
						Title:         "Empirical Harness & Statistical Contract Binding for " + l2Target.ID,
						Directive:     dir,
						DirectiveHash: sha256Str(l2Target.DirectiveHash + "|" + c2ID + "|" + dir)[:16],
						SoulHash:      l2Target.SoulHash,
					})
				} else if l3Distill != nil {
					l3Prompt := fmt.Sprintf(
						"L3 SKILL & PRIOR CONSOLIDATION [Node %s (%s)]: Audit the C6 Beta(alpha,beta) Wiki-Skill priors in /wiki/skills/ and empirical dataset splits for %q, and emit the finalized capability specification on southbound_directive for Layer 4.",
						l3Distill.ID, truncateText(l3Distill.DirectiveHash, 8), l3Distill.Title,
					)
					tr, err := e.executeLayerStep(ctx, cycleID, core.LayerAgentModel, l3Prompt)
					if err == nil {
						l3Distill.State = "NEEDS_L4_SPEC"
						if strings.TrimSpace(tr.Southbound) != "" {
							l3Distill.Directive = strings.TrimSpace(tr.Southbound)
						}
						e.upsertMissionDAGNode(ctx, *l3Distill)
					}
				}
			}(targetL2ForL3, targetL3ForDistill)
		}

		if l4NeedsRun {
			wg.Add(1)
			go func(l3Targets []MissionDAGNode, stalledL4 *MissionDAGNode) {
				defer wg.Done()
				if stalledL4 != nil {
					handoff := LoadHandoffPacket(ctx, e.memory, stalledL4.Directive)
					l4Prompt := fmt.Sprintf(
						"L4 EXECUTIVE TACTICAL REVISION (Task Node %s, Attempt #%d): Prior L6 execution stalled (%d failed attempts). Diagnose the root cause from handoff telemetry and emit a revised step-by-step execution plan on southbound_directive for %s.",
						stalledL4.ID, stalledL4.Attempts+1, len(handoff.FailedAttempts), stalledL4.Title,
					)
					tr, err := e.runExecutivePlanningLoop(ctx, cycleID, l4Prompt)
					if err == nil {
						stalledL4.State = "READY_FOR_L5"
						stalledL4.Attempts = 0
						if strings.TrimSpace(tr.Southbound) != "" {
							// Keep base + last 3 revisions — an unbounded
							// tail balloons goals.md past the 128KB wiki
							// read cap and hides the active milestone.
							parts := strings.Split(stalledL4.Directive, " | [L4 Revised Plan]: ")
							if len(parts) > 4 {
								parts = append(parts[:1], parts[len(parts)-3:]...)
							}
							stalledL4.Directive = strings.Join(append(parts, strings.TrimSpace(tr.Southbound)), " | [L4 Revised Plan]: ")
						}
						e.upsertMissionDAGNode(ctx, *stalledL4)
					}
					return
				}
				// Backpressure: don't promote more specs into the ready
				// queue while it is saturated — new attempts=0 nodes jump
				// the LRU front and starve vetoed siblings from ever
				// reaching a second attempt. Work the backlog first.
				readyBacklog := 0
				for _, nn := range e.LoadMissionDAGNodes(ctx) {
					if nn.Stage == "L4_TASK_SPEC" && (nn.State == "READY_FOR_L5" || nn.State == "TACTICAL_STALL") {
						readyBacklog++
					}
				}
				if readyBacklog >= 12 {
					return
				}
				for _, l3Target := range l3Targets {
					l4Prompt := fmt.Sprintf(
						"L4 EXECUTIVE SPECIFIER [Parent L3 Node %s (%s)]: Translate Capability Model %q (%s) into a concrete executable Task Spec: explicit /workspace module path, dataset path with a declared train/held-out split, result JSON path, and a verification contract the gate can check. Emit the spec on southbound_directive.",
						l3Target.ID, truncateText(l3Target.DirectiveHash, 8), l3Target.Title, truncateText(l3Target.Directive, 240),
					)
					tr, err := e.runExecutivePlanningLoop(ctx, cycleID, l4Prompt)
					if err != nil {
						continue
					}
					l3Target.State = "L4_SPECCED"
					e.upsertMissionDAGNode(ctx, l3Target)
					dir := firstNonEmpty(strings.TrimSpace(tr.Southbound), tr.StateSummary)

					taskID := "T_" + l3Target.ID
					stem := strings.ToLower(l3Target.ID)
					modPath := fmt.Sprintf("/workspace/modules/%s.py", stem)
					dsPath := fmt.Sprintf("/workspace/datasets/%s_v1.json", stem)
					resPath := fmt.Sprintf("/workspace/results/%s.json", stem)
					if dynMod, dynDS, dynRes := e.extractDynamicL4SpecPaths(ctx, dir); dynMod != "" {
						modPath = dynMod
						if dynDS != "" {
							dsPath = dynDS
						}
						if dynRes != "" {
							resPath = dynRes
						}
					}
					fullSpec := fmt.Sprintf(
						"Build `%s` (with `--selftest`), dataset `%s` (declared train/held-out split), and result artifact `%s` — verdict computed by the gate, not self-declared. L4 Executive Plan: %s",
						modPath, dsPath, resPath, dir,
					)
					e.upsertMissionDAGNode(ctx, MissionDAGNode{
						ID:            taskID,
						ParentID:      l3Target.ID,
						Stage:         "L4_TASK_SPEC",
						State:         "READY_FOR_L5",
						OwnerLayer:    4,
						Title:         "Task Specification (" + l3Target.ID + " -> " + modPath + ")",
						Directive:     fullSpec,
						ArtifactPath:  modPath,
						DirectiveHash: sha256Str(l3Target.DirectiveHash + "|" + taskID + "|" + fullSpec)[:16],
						SoulHash:      l3Target.SoulHash,
					})
				}
			}(pendingL3ForL4, targetStalledL4)
		}

		wg.Wait()
		e.SyncMissionDAGToGoalsWiki(ctx)
		_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
			TraceID:      cycleID,
			ParentSpanID: cycleSpanID,
			AgentID:      e.agentID,
			LayerCode:    strings.Join(activeWaveLayers, "||"),
			Operation:    "dag.parallel_wave",
			Status:       "ok",
			DurationMs:   time.Since(parStart).Milliseconds(),
			Detail:       fmt.Sprintf("DAG Wave #%d executed [%s] concurrently", wave+1, strings.Join(activeWaveLayers, " || ")),
		})
	}

	// =========================================================================
	// STAGE 3: L5 COGNITIVE CONTROL (4-BIT GPU GATE) -> L6 WORKHORSE PROSECUTION
	// Strictly gated by VerifyTaskNodeProvenance(L1 -> L2 -> L3 -> L4)!
	// =========================================================================
	e.SyncMissionDAGToGoalsWiki(ctx)
	readyTask, ok := e.ActiveReadyTaskNode(ctx)
	if !ok {
		return nil
	}
	provenanceOK, provChain := e.VerifyTaskNodeProvenance(ctx, readyTask.ID)
	if !provenanceOK {
		return nil
	}

	// Run L5 Local 4-bit GPU Gate on the provenance-verified L4 Task Spec before dispatching L6
	// fail closed: a gate error leaves pYes at 0 and the task is not dispatched
	pYes := 0.0
	gateModel := "d1-4bit-gpu-gate"
	var gateMs int64
	sharedState := "ACE L5 Cognitive Control & Somatic Valence Gate | Top-Down Mission DAG Provenance Verified | Verify concrete dataset + modular execution directive"
	gateQ := fmt.Sprintf("Provenance: %s\nTask: %s\nQuestion: Is this a concrete, safe scientific execution plan ready for Layer 6?", provChain, truncateText(readyTask.Directive, 220))
	if gpuRes, gErr := e.evaluateL5Gate(ctx, sharedState, gateQ); gErr == nil && gpuRes.Model != "" {
		pYes = gpuRes.PYes
		gateModel = gpuRes.Model
		gateMs = gpuRes.LatencyMs
	}
	gateApproved := pYes >= 0.38
	e.mu.Lock()
	l5 := e.layers[core.LayerCognitiveCtrl]
	l5.State = "idle"
	if gateApproved {
		l5.Summary = fmt.Sprintf("L5 [DAG PROVENANCE %s] PASS (P(yes)=%.2f, %dms): %s", provChain, pYes, gateMs, truncateText(readyTask.Title, 60))
		l5.LastAction = fmt.Sprintf("d1_4bit_gate(P=%.2f, %dms, %s, cxn=%s) -> L6 dispatch", pYes, gateMs, provChain, constitution.Version())
	} else {
		l5.Summary = fmt.Sprintf("L5 [DAG PROVENANCE %s] VETO (P(yes)=%.2f, %dms): %s", provChain, pYes, gateMs, truncateText(readyTask.Title, 60))
		l5.LastAction = fmt.Sprintf("d1_4bit_gate(P=%.2f, %dms, %s, cxn=%s) -> VETO L6", pYes, gateMs, provChain, constitution.Version())
	}
	l5.ActiveDirective = readyTask.Directive
	l5.Confidence = pYes
	l5.Model = gateModel
	l5.LastLatencyMs = gateMs
	l5.TotalInvocations++
	l5.UpdatedAt = time.Now().UTC()
	l5Snap := *l5
	e.mu.Unlock()
	_ = e.memory.SaveLayerStatus(ctx, l5Snap)
	e.telem.EmitLive("layer.status", l5Snap)

	if !gateApproved {
		// A vetoed spec counts as an attempt: repeated vetoes route it to
		// TACTICAL_STALL so L4 re-specs it instead of starving the ready queue.
		e.RecordDAGTaskAttempt(ctx)
		return nil
	}
	e.RecordDAGTaskAttempt(ctx)
	l6Directive := fmt.Sprintf("[PROVENANCE LOCKED %s] %s: %s", provChain, readyTask.Title, readyTask.Directive)
	e.setActiveDAGTask(readyTask.ID)
	defer e.setActiveDAGTask("")
	return e.runPiProsecutorLoop(ctx, cycleID, cycleSpanID, l6Directive)
}

var (
	reDynMod = regexp.MustCompile(`(?:/workspace/)([a-zA-Z0-9_/-]+\.py)`)
)

// extractDynamicL4SpecPaths inspects L4's Southbound directive and recent /wiki/notes/l4_*.md / l2_*.md specs
// to bind the exact module path originated by L2/L4 that is not yet registered in mission_dag_nodes,
// paired with a dedicated, collision-free dataset and result path derived from its module stem.
func (e *Engine) extractDynamicL4SpecPaths(ctx context.Context, dir string) (string, string, string) {
	existing := e.LoadMissionDAGNodes(ctx)
	usedMod := make(map[string]bool, len(existing))
	for _, n := range existing {
		if n.ArtifactPath != "" {
			usedMod[n.ArtifactPath] = true
		}
	}
	corpus := dir
	if pages, err := e.memory.ListWiki(); err == nil {
		for i := len(pages) - 1; i >= 0; i-- {
			p := pages[i].Path
			if strings.HasPrefix(p, "notes/l4_") || strings.HasPrefix(p, "notes/l2_") {
				if wData, err := e.memory.ReadWiki(p); err == nil && wData != "" {
					corpus += "\n" + wData
				}
			}
		}
	}
	modMatches := reDynMod.FindAllStringSubmatch(corpus, -1)
	for _, m := range modMatches {
		candidateMod := "/workspace/" + m[1]
		if !usedMod[candidateMod] {
			stemRel := strings.TrimPrefix(m[1], "modules/")
			modStem := strings.TrimSuffix(strings.ReplaceAll(stemRel, "/", "_"), ".py")
			ds := fmt.Sprintf("/workspace/datasets/%s_v1.json", modStem)
			res := fmt.Sprintf("/workspace/results/%s.json", modStem)
			return candidateMod, ds, res
		}
	}
	return "", "", ""
}

// branchOf extracts the L2 branch ID ("S<n>") from an L4 task node via the
// ID convention: task T_C_S<n>_X -> capability C_S<n>_X -> branch S<n>.
func branchOf(n MissionDAGNode) string {
	id := strings.TrimPrefix(n.ParentID, "C_")
	if i := strings.Index(id, "_"); i > 0 {
		return id[:i]
	}
	return id
}

// resultMovedStat parses the task's result JSON and reports whether the
// primary statistic moved: h1_accepted true, or |heldout-baseline| >= 0.05
// (default MDE) when a numeric comparison exists. Missing/unreadable results
// count as not-moved — a completion without evidence does not extend a
// branch's lease (spec 8.3 diminishing returns).
func (e *Engine) resultMovedStat(n MissionDAGNode) bool {
	wsDir, _ := e.resolveSandboxDirs()
	if wsDir == "" {
		return false
	}
	stem := strings.ToLower(strings.TrimPrefix(n.ID, "T_"))
	var blob []byte
	for _, rel := range []string{"results/" + stem + ".json", "results/" + strings.TrimSuffix(stem, "_data") + ".json"} {
		if b, err := os.ReadFile(filepath.Join(wsDir, ResolveWorkspaceArtifactPath(wsDir, rel))); err == nil {
			blob = b
			break
		}
	}
	if len(blob) == 0 {
		return false
	}
	var res struct {
		H1Accepted    *bool    `json:"h1_accepted"`
		Heldout       *float64 `json:"heldout"`
		HeldoutScore  *float64 `json:"heldout_score"`
		HeldoutAcc    *float64 `json:"heldout_accuracy"`
		Baseline      *float64 `json:"baseline"`
		BaselineScore *float64 `json:"baseline_score"`
		BaselineAcc   *float64 `json:"baseline_accuracy"`
	}
	if json.Unmarshal(blob, &res) != nil {
		return false
	}
	if res.Heldout == nil {
		res.Heldout = res.HeldoutAcc
	}
	if res.Baseline == nil && res.BaselineScore == nil {
		res.BaselineScore = res.BaselineAcc
	}
	if res.H1Accepted != nil {
		return *res.H1Accepted
	}
	held, base := res.Heldout, res.Baseline
	if held == nil {
		held = res.HeldoutScore
	}
	if base == nil {
		base = res.BaselineScore
	}
	if held != nil && base != nil {
		d := *held - *base
		if d < 0 {
			d = -d
		}
		return d >= 0.05
	}
	return false
}
