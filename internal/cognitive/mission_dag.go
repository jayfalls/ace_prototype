package cognitive

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"ace/internal/core"
)

// MissionDAGNode represents a single node in the strict Top-Down Dependency Tree:
// L1_MISSION_ROOT -> L2_STRATEGY -> L3_CAPABILITY -> L4_TASK_SPEC -> (L5 GPU Gate -> L6 Execution).
type MissionDAGNode struct {
	ID            string `json:"id"`
	ParentID      string `json:"parent_id"`
	Stage         string `json:"stage"`          // L1_MISSION_ROOT | L2_STRATEGY | L3_CAPABILITY | L4_TASK_SPEC
	State         string `json:"state"`          // L1_ANCHORED | NEEDS_L3_MODEL | L3_MODELED | NEEDS_L4_SPEC | L4_SPECCED | READY_FOR_L5 | TACTICAL_STALL | NEEDS_L3_DISTILL | COMPLETED
	OwnerLayer    int    `json:"owner_layer"`    // 1, 2, 3, 4
	Title         string `json:"title"`          // Concise human-readable title
	Directive     string `json:"directive"`      // Full layer directive / spec
	ArtifactPath  string `json:"artifact_path"`  // Target module / result JSON path for L4_TASK_SPEC
	DirectiveHash string `json:"directive_hash"` // SHA-256 provenance hash of (ParentHash + Directive)
	SoulHash      string `json:"soul_hash"`      // SHA-256 of /wiki/soul.md when anchored
	Attempts      int    `json:"attempts"`       // L6 execution attempts on this leaf
	UpdatedAt     string `json:"updated_at"`
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
			updated_at DATETIME NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_mission_dag_stage_state ON mission_dag_nodes(stage, state);
		CREATE INDEX IF NOT EXISTS idx_mission_dag_parent ON mission_dag_nodes(parent_id);
	`)
}

func sha256Str(s string) string {
	return sha256Hex([]byte(s))
}

func (e *Engine) currentSoulHash() string {
	soul, _ := e.memory.ReadWiki("soul.md")
	return sha256Str(strings.TrimSpace(soul))[:16]
}

// extractCurrentMissionFromSoul extracts the human's single steerable # CURRENT MISSION from soul.md.
// The heading must start a line — soul.md also mentions `# CURRENT MISSION` inline in prose.
func (e *Engine) extractCurrentMissionFromSoul() string {
	soul, _ := e.memory.ReadWiki("soul.md")
	lines := strings.Split(soul, "\n")
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "# CURRENT MISSION") {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ""
	}
	var kept []string
	for _, l := range lines[start:] {
		tl := strings.TrimSpace(l)
		if strings.HasPrefix(tl, "# ") {
			break
		}
		if tl != "" && !strings.HasPrefix(tl, "*(") {
			kept = append(kept, tl)
		}
	}
	return strings.Join(kept, " ")
}

func (e *Engine) LoadMissionDAGNodes(ctx context.Context) []MissionDAGNode {
	db := e.getDB()
	if db == nil {
		return nil
	}
	e.initMissionDAGSchema(db)
	rows, err := db.QueryContext(ctx, `
		SELECT id, parent_id, stage, state, owner_layer, title, directive, artifact_path, directive_hash, soul_hash, attempts, updated_at
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
		if rows.Scan(&n.ID, &n.ParentID, &n.Stage, &n.State, &n.OwnerLayer, &n.Title, &n.Directive, &n.ArtifactPath, &n.DirectiveHash, &n.SoulHash, &n.Attempts, &n.UpdatedAt) == nil {
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
		INSERT INTO mission_dag_nodes (id, parent_id, stage, state, owner_layer, title, directive, artifact_path, directive_hash, soul_hash, attempts, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			state = excluded.state,
			title = excluded.title,
			directive = excluded.directive,
			artifact_path = excluded.artifact_path,
			directive_hash = excluded.directive_hash,
			soul_hash = excluded.soul_hash,
			attempts = excluded.attempts,
			updated_at = excluded.updated_at
	`, n.ID, n.ParentID, n.Stage, n.State, n.OwnerLayer, n.Title, n.Directive, n.ArtifactPath, n.DirectiveHash, n.SoulHash, n.Attempts, n.UpdatedAt)

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
	sb.WriteString("## Strategic & Capability Dependency Tree (`L2 || L3`)\n")
	for _, n := range nodes {
		switch n.Stage {
		case "L2_STRATEGY":
			sb.WriteString(fmt.Sprintf("- **[L2 STRATEGY `%s` | `%s`]** (Parent: `%s`, Hash: `%s`): **%s** — %s\n", n.ID, n.State, n.ParentID, truncateText(n.DirectiveHash, 8), n.Title, truncateText(n.Directive, 220)))
		case "L3_CAPABILITY":
			sb.WriteString(fmt.Sprintf("  - **[L3 CAPABILITY MODEL `%s` | `%s`]** (Parent: `%s`, Hash: `%s`): **%s** — %s\n", n.ID, n.State, n.ParentID, truncateText(n.DirectiveHash, 8), n.Title, truncateText(n.Directive, 220)))
		}
	}
	sb.WriteString("\n## Active Executable Task Specifications (`L4 → L5 → L6`)\n\n")
	for _, n := range nodes {
		if n.Stage == "L4_TASK_SPEC" {
			box := "[ ]"
			if n.State == "COMPLETED" {
				box = "[x]"
			}
			sb.WriteString(fmt.Sprintf("- %s **%s — %s** (Parent: `%s`, Provenance: `%s`): %s\n", box, n.ID, n.Title, n.ParentID, truncateText(n.DirectiveHash, 10), collapseMultilineMilestoneBullets(n.Directive)))
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
		// fewest attempts first, then least-recently-touched: a vetoed/stalled
		// task rotates behind its siblings instead of starving the queue.
		if best == nil || n.Attempts < best.Attempts ||
			(n.Attempts == best.Attempts && n.UpdatedAt < best.UpdatedAt) {
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
func (e *Engine) MarkDAGTaskVerified(ctx context.Context, milestoneLine string) {
	nodes := e.LoadMissionDAGNodes(ctx)
	for _, n := range nodes {
		if n.Stage == "L4_TASK_SPEC" && n.State != "COMPLETED" {
			if strings.Contains(milestoneLine, n.ID) || (n.ArtifactPath != "" && strings.Contains(milestoneLine, n.ArtifactPath)) {
				n.State = "COMPLETED"
				e.upsertMissionDAGNode(ctx, n)
				break
			}
		}
	}
	e.SyncMissionDAGToGoalsWiki(ctx)
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
	missionText := e.extractCurrentMissionFromSoul()
	if missionText == "" {
		return nil // no mission anchored: idle until the owner sets one (plan 8.1)
	}
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
			"L1 ASPIRATIONAL GENESIS: Evaluate the human's # CURRENT MISSION in /wiki/soul.md (%q) against the Three Heuristic Imperatives (Reduce suffering, Increase prosperity, Increase understanding in the universe) and the Universal Declaration of Human Rights. Emit your authoritative top-down constitutional and mission directive on southbound_directive to anchor the Mission Dependency Tree for Layer 2 (Global Strategy).",
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
				pendingL3ForL4 = append(pendingL3ForL4, l3Nodes[i])
			}
		}
		for i := range l4Nodes {
			// A stalled task is being revised by this wave's L4 pass — it must not
			// freeze L2 horizon expansion, or one unvetoable spec holds the whole
			// mission hostage (observed live: T_C_S2_HARNESS churning 15+ cycles).
			if l4Nodes[i].State != "COMPLETED" && l4Nodes[i].State != "TACTICAL_STALL" {
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
		for i := range l4Nodes {
			l2id, ok := l3ToL2[l4Nodes[i].ParentID]
			if !ok {
				continue
			}
			l2HasTasks[l2id] = true
			if l4Nodes[i].State != "COMPLETED" {
				l2Open[l2id] = true
			}
		}
		openBranchCount := 0
		for i := range l2Nodes {
			n := &l2Nodes[i]
			if l2HasTasks[n.ID] && !l2Open[n.ID] {
				if n.State != "COMPLETED" {
					n.State = "COMPLETED"
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
				l2Prompt := fmt.Sprintf(
					"L2 GLOBAL STRATEGY [Branch %s under L1 Root %s (%s)]: Under L1's directive (%q), formulate Strategic Branch %s as a falsifiable hypothesis serving the mission — expected outcome, primary approach, and how it differs from prior branches (%s). Emit the branch directive on southbound_directive.",
					branchID, root.ID, truncateText(root.DirectiveHash, 8), truncateText(root.Directive, 240), branchID, truncateText(strings.Join(prior, "; "), 600),
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
							stalledL4.Directive = stalledL4.Directive + " | [L4 Revised Plan]: " + strings.TrimSpace(tr.Southbound)
						}
						e.upsertMissionDAGNode(ctx, *stalledL4)
					}
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
		l5.LastAction = fmt.Sprintf("d1_4bit_gate(P=%.2f, %dms, %s) -> L6 dispatch", pYes, gateMs, provChain)
	} else {
		l5.Summary = fmt.Sprintf("L5 [DAG PROVENANCE %s] VETO (P(yes)=%.2f, %dms): %s", provChain, pYes, gateMs, truncateText(readyTask.Title, 60))
		l5.LastAction = fmt.Sprintf("d1_4bit_gate(P=%.2f, %dms, %s) -> VETO L6", pYes, gateMs, provChain)
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
