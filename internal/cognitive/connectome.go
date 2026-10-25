package cognitive

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ace/internal/config"
	"ace/internal/contextsys"
	"ace/internal/core"
	"ace/internal/provider"
)

// Spec 9.5: the connectome model lives in internal/contextsys (the
// problem-space map). These aliases keep the engine's orchestration
// methods readable without a package-qualified rewrite.
type ConnectomeNode = contextsys.ConnectomeNode
type ConnectomeEdge = contextsys.ConnectomeEdge

const DefaultSliceMaxBytes = contextsys.DefaultSliceMaxBytes

var (
	connectomePathTokenRe      = contextsys.ConnectomePathTokenRe
	connectomeWordTokenRe      = contextsys.ConnectomeWordTokenRe
	pyDefClassLineRe           = contextsys.PyDefClassLineRe
	goFuncTypeLineRe           = contextsys.GoFuncTypeLineRe
	summarizeFileForConnectome = contextsys.SummarizeFileForConnectome
	classifyConnectomeFileKind = contextsys.ClassifyConnectomeFileKind
	resolveConnectomePathToken = contextsys.ResolveConnectomePathToken
	sanitizeTaskID             = contextsys.SanitizeTaskID
	SliceBudgetMaxBytes        = contextsys.SliceBudgetMaxBytes
	RankConnectomeCandidates   = contextsys.RankConnectomeCandidates
	IsHarnessMintedEvidenceRef = contextsys.IsHarnessMintedEvidenceRef
)

func (e *Engine) getDB() *sql.DB {
	if s, ok := e.memory.(interface{ DB() *sql.DB }); ok {
		return s.DB()
	}
	return nil
}

// extracts mention/import edges, and updates SQLite connectome_nodes and connectome_edges.
func (e *Engine) RefreshConnectomeGraph(ctx context.Context, cycleEpoch int64) (map[string]ConnectomeNode, map[string][]string) {
	wsDir, wikiDir := e.resolveSandboxDirs()
	files := make(map[string]string) // virtual path (/workspace/... or /wiki/...) -> host path
	for _, rootSpec := range []struct {
		dir  string
		pref string
	}{
		{wsDir, "/workspace/"},
		{wikiDir, "/wiki/"},
	} {
		if rootSpec.dir == "" {
			continue
		}
		_ = filepath.WalkDir(rootSpec.dir, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				base := d.Name()
				if strings.HasPrefix(base, ".") || base == "node_modules" || base == "__pycache__" || base == "workpads" {
					if p != rootSpec.dir {
						return filepath.SkipDir
					}
				}
				return nil
			}
			rel, rerr := filepath.Rel(rootSpec.dir, p)
			if rerr != nil {
				return nil
			}
			vpath := rootSpec.pref + filepath.ToSlash(rel)
			files[vpath] = p
			return nil
		})
	}

	byBase := make(map[string][]string)
	for vp := range files {
		b := filepath.Base(vp)
		byBase[b] = append(byBase[b], vp)
	}

	nodes := make(map[string]ConnectomeNode, len(files))
	undirected := make(map[string]map[string]bool)
	addUndirected := func(a, b string) {
		if a == b || a == "" || b == "" {
			return
		}
		if undirected[a] == nil {
			undirected[a] = make(map[string]bool)
		}
		if undirected[b] == nil {
			undirected[b] = make(map[string]bool)
		}
		undirected[a][b] = true
		undirected[b][a] = true
	}

	now := time.Now().UTC()
	var edges []ConnectomeEdge

	for vp, hp := range files {
		info, err := os.Stat(hp)
		if err != nil {
			continue
		}
		sz := int(info.Size())
		kind := classifyConnectomeFileKind(vp)
		summary, rawSample, fileHash := summarizeFileForConnectome(vp, hp, sz)
		id := "file:" + vp
		nodes[id] = ConnectomeNode{
			ID:         id,
			Kind:       kind,
			Path:       vp,
			Summary:    summary,
			Hash:       fileHash,
			SizeBytes:  sz,
			State:      "UNEXPLORED",
			CycleEpoch: cycleEpoch,
			UpdatedAt:  now,
		}

		// Mention / import edges from text sample
		if rawSample != "" {
			matches := connectomePathTokenRe.FindAllString(rawSample, -1)
			seen := make(map[string]bool)
			for _, tok := range matches {
				for _, targetVP := range resolveConnectomePathToken(tok, files, byBase) {
					if targetVP != vp && !seen[targetVP] {
						seen[targetVP] = true
						dstID := "file:" + targetVP
						edges = append(edges, ConnectomeEdge{
							Src:    id,
							Dst:    dstID,
							Kind:   "MENTIONS",
							Weight: 1.0,
						})
						addUndirected(id, dstID)
					}
				}
			}
		}
	}

	db := e.getDB()
	if db != nil {
		// Preserve existing node states (INSPECTED, IMPLEMENTED, VERIFIED) and load non-file nodes (UNCERTAINTY, CONTRADICTION, etc.)
		rows, err := db.QueryContext(ctx, `
			SELECT id, kind, path, symbol, summary, hash, size_bytes, state, owner_layer,
			       blocking_weight, info_value, expected_cost, evidence_ref, cycle_epoch, updated_at
			FROM connectome_nodes
		`)
		if err == nil {
			for rows.Next() {
				var n ConnectomeNode
				if scanErr := rows.Scan(
					&n.ID, &n.Kind, &n.Path, &n.Symbol, &n.Summary, &n.Hash, &n.SizeBytes, &n.State, &n.OwnerLayer,
					&n.BlockingWeight, &n.InfoValue, &n.ExpectedCost, &n.EvidenceRef, &n.CycleEpoch, &n.UpdatedAt,
				); scanErr == nil {
					if cur, exists := nodes[n.ID]; exists {
						if cur.Hash == n.Hash && n.State != "" && n.State != "UNEXPLORED" {
							cur.State = n.State
							cur.EvidenceRef = n.EvidenceRef
						}
						nodes[n.ID] = cur
					} else if !strings.HasPrefix(n.ID, "file:") {
						nodes[n.ID] = n
					}
				}
			}
			rows.Close()
		}

		// Persist updated file nodes & edges in a fast transaction
		t0 := time.Now()
		tx, txErr := db.BeginTx(ctx, nil)
		if txErr == nil {
			stmtNode, _ := tx.PrepareContext(ctx, `
				INSERT INTO connectome_nodes (
					id, kind, path, symbol, summary, hash, size_bytes, state, owner_layer,
					blocking_weight, info_value, expected_cost, evidence_ref, cycle_epoch, updated_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT(id) DO UPDATE SET
					summary = excluded.summary,
					hash = excluded.hash,
					size_bytes = excluded.size_bytes,
					state = excluded.state,
					cycle_epoch = excluded.cycle_epoch,
					updated_at = excluded.updated_at
			`)
			if stmtNode != nil {
				for _, n := range nodes {
					if strings.HasPrefix(n.ID, "file:") {
						_, _ = stmtNode.ExecContext(ctx,
							n.ID, n.Kind, n.Path, n.Symbol, n.Summary, n.Hash, n.SizeBytes, n.State, n.OwnerLayer,
							n.BlockingWeight, n.InfoValue, n.ExpectedCost, n.EvidenceRef, n.CycleEpoch, now,
						)
					}
				}
				stmtNode.Close()
			}
			stmtEdge, _ := tx.PrepareContext(ctx, `
				INSERT INTO connectome_edges (src, dst, kind, weight, updated_at)
				VALUES (?, ?, ?, ?, ?)
				ON CONFLICT(src, dst, kind) DO UPDATE SET
					weight = excluded.weight,
					updated_at = excluded.updated_at
			`)
			if stmtEdge != nil {
				for _, ed := range edges {
					_, _ = stmtEdge.ExecContext(ctx, ed.Src, ed.Dst, ed.Kind, ed.Weight, now)
				}
				stmtEdge.Close()
			}
			_ = tx.Commit()
			if waitMs := time.Since(t0).Milliseconds(); waitMs >= 5 && e.telem != nil {
				_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
					Operation:  "db_write_wait_ms",
					LayerCode:  "SYS",
					Status:     "ok",
					DurationMs: waitMs,
					Detail:     fmt.Sprintf("op=refresh_connectome nodes=%d edges=%d", len(nodes), len(edges)),
				})
			}
		}

		// Also include persisted CO_CHANGED / DEPENDS_ON edges in undirected adjacency
		erows, eerr := db.QueryContext(ctx, `SELECT src, dst FROM connectome_edges`)
		if eerr == nil {
			for erows.Next() {
				var s, d string
				if erows.Scan(&s, &d) == nil {
					addUndirected(s, d)
				}
			}
			erows.Close()
		}
	}

	adj := make(map[string][]string, len(undirected))
	for u, nb := range undirected {
		list := make([]string, 0, len(nb))
		for v := range nb {
			list = append(list, v)
		}
		sort.Strings(list)
		adj[u] = list
	}
	return nodes, adj
}

// BuildConnectomeSlice packs ranked candidate summaries + open VOI-sorted uncertainties into the separate Slice Budget (default 8KB).
// Also supports Turn 0.5 router behind ACE_TURN05=on|off|ab.
func (e *Engine) BuildConnectomeSlice(ctx context.Context, cycleID string, milestoneID string, directive string, goalText string, specText string) (string, map[string]bool) {
	nodes, adj := e.RefreshConnectomeGraph(ctx, time.Now().Unix())
	ranked := RankConnectomeCandidates(nodes, adj, directive, goalText, specText)

	// Turn-0.5 router is off by default — context.turn05 in config.d
	// (spec 9.5; supersedes the ACE_TURN05 env experiment flag).
	useTurn05 := e.layerKnobs.Load().(config.LayerKnobs).Turn05

	ordered := ranked
	if useTurn05 && len(ranked) > 5 {
		top40 := ranked
		if len(top40) > 40 {
			top40 = top40[:40]
		}
		picked := e.runTurn05RetrievalRouter(ctx, cycleID, directive, goalText, top40)
		if len(picked) > 0 {
			seen := make(map[string]bool, len(picked))
			reordered := make([]ConnectomeNode, 0, len(ranked))
			for _, n := range picked {
				seen[n.ID] = true
				reordered = append(reordered, n)
			}
			for _, n := range ranked {
				if !seen[n.ID] {
					reordered = append(reordered, n)
				}
			}
			ordered = reordered
		}
	}

	budget := SliceBudgetMaxBytes()
	var b strings.Builder
	b.WriteString(fmt.Sprintf("### Deterministic Connectome Slice (RRF PPR+BM25, turn0.5=%v, budget=%dB)\n", useTurn05, budget))

	// First pack top VOI Open Uncertainties / Contradictions (up to 1,400B)
	var uncertainties []ConnectomeNode
	for _, n := range nodes {
		if (n.Kind == "UNCERTAINTY" || n.Kind == "CONTRADICTION") && (n.State == "OPEN" || n.State == "PROBING") {
			uncertainties = append(uncertainties, n)
		}
	}
	sort.Slice(uncertainties, func(i, j int) bool {
		return uncertainties[i].VOI() > uncertainties[j].VOI()
	})
	if len(uncertainties) > 0 {
		b.WriteString("#### Open Uncertainties & Contradictions (Ranked by VOI = w*V/C)\n")
		for idx, u := range uncertainties {
			if idx >= 8 || b.Len() > 1400 {
				break
			}
			b.WriteString(fmt.Sprintf("- [%s|%s|VOI=%.2f] `%s`: %s\n", u.Kind, u.State, u.VOI(), u.ID, u.Summary))
		}
	}

	b.WriteString("#### Ranked Workspace & Wiki Connectome Nodes\n")
	preloadedPaths := make(map[string]bool)
	for idx, n := range ordered {
		line := fmt.Sprintf("%d. [%s|%s] %s\n", idx+1, n.Kind, n.State, n.Summary)
		if b.Len()+len(line) > budget {
			break
		}
		b.WriteString(line)
		if n.Path != "" {
			preloadedPaths[n.Path] = true
		}
	}
	return b.String(), preloadedPaths
}

// runTurn05RetrievalRouter uses the fast free router slot (opencode-go/beam-free -> cat-jam-free)
// to pick up to 10 critical files from the deterministic top-40 candidates.
func (e *Engine) runTurn05RetrievalRouter(ctx context.Context, cycleID string, directive string, goalText string, top40 []ConnectomeNode) []ConnectomeNode {
	if e.llm == nil || len(top40) == 0 {
		return nil
	}
	e.mu.RLock()
	if cycleID != "" && e.lastTurn05CycleID == cycleID {
		cached := e.lastTurn05Picked
		e.mu.RUnlock()
		return cached
	}
	e.mu.RUnlock()
	var candLines strings.Builder
	for i, n := range top40 {
		candLines.WriteString(fmt.Sprintf("%d. %s\n", i+1, n.Summary))
	}
	dShort := directive
	if len(dShort) > 1200 {
		dShort = dShort[:1200]
	}
	gShort := goalText
	if len(gShort) > 1200 {
		gShort = gShort[:1200]
	}
	prompt := fmt.Sprintf(
		"You choose which files a coding agent must have in context before it starts.\nTask directive:\n%s\n\nProject goal excerpt:\n%s\n\nCandidate files (path, size, summary):\n%s\nReturn ONLY a JSON array of up to 10 candidate numbers, most necessary first.",
		dShort, gShort, candLines.String(),
	)

	rctx, cancel := context.WithTimeout(provider.WithLayer(ctx, "L3"), 12*time.Second)
	defer cancel()

	t0 := time.Now()
	resp, err := e.llm.StreamComplete(rctx, core.CompletionRequest{
		Model:        "beam-free",
		SystemPrompt: "You are a fast Turn-0.5 retrieval router. Output ONLY a JSON array of integers like [1, 3, 5].",
		Messages: []core.ChatTurn{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.0,
		MaxTokens:   256,
	}, nil)
	if err != nil {
		return nil
	}

	raw := strings.TrimSpace(resp.Content)
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start < 0 || end <= start {
		return nil
	}
	var indices []int
	if err := json.Unmarshal([]byte(raw[start:end+1]), &indices); err != nil {
		return nil
	}
	var picked []ConnectomeNode
	seen := make(map[int]bool)
	for _, idx := range indices {
		if idx >= 1 && idx <= len(top40) && !seen[idx] {
			seen[idx] = true
			picked = append(picked, top40[idx-1])
			if len(picked) >= 10 {
				break
			}
		}
	}
	if e.telem != nil {
		_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
			TraceID:    cycleID,
			AgentID:    e.agentID,
			LayerCode:  "L5",
			Operation:  "turn05_router",
			Model:      resp.Model,
			Status:     "ok",
			DurationMs: time.Since(t0).Milliseconds(),
			Detail:     fmt.Sprintf("model=%s picked=%d/%d", resp.Model, len(picked), len(top40)),
		})
	}
	if cycleID != "" {
		e.mu.Lock()
		e.lastTurn05CycleID = cycleID
		e.lastTurn05Picked = picked
		e.mu.Unlock()
	}
	return picked
}

// RecordConnectomeInspection marks a file node as INSPECTED or IMPLEMENTED and emits a `slice_miss` span
// if L6 had to read a file that was not preloaded in its Turn 0 / Turn 0.5 Connectome slice.
func (e *Engine) RecordConnectomeInspection(ctx context.Context, cycleID string, relPath string, newState string, evidenceRef string, preloaded map[string]bool) {
	clean := strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(relPath)), "/")
	vpath := "/workspace/" + clean
	if strings.HasPrefix(clean, "wiki/") {
		vpath = "/" + clean
	}
	if newState == "INSPECTED" && preloaded != nil && len(preloaded) > 0 && !preloaded[vpath] && e.telem != nil {
		_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
			TraceID:   cycleID,
			AgentID:   e.agentID,
			LayerCode: "L6",
			Operation: "slice_miss",
			Status:    "miss",
			Detail:    fmt.Sprintf("path=%s", vpath),
		})
	}
	db := e.getDB()
	if db == nil {
		return
	}
	_, _ = db.ExecContext(ctx, `
		UPDATE connectome_nodes
		SET state = ?, evidence_ref = CASE WHEN ? != '' THEN ? ELSE evidence_ref END, updated_at = ?
		WHERE id = ?
	`, newState, evidenceRef, evidenceRef, time.Now().UTC(), "file:"+vpath)
}

// RecordCoChangedEdges links files modified in the same successful cycle with CO_CHANGED edges.
func (e *Engine) RecordCoChangedEdges(ctx context.Context, relPaths []string) {
	db := e.getDB()
	if db == nil || len(relPaths) < 2 {
		return
	}
	now := time.Now().UTC()
	for i := 0; i < len(relPaths); i++ {
		for j := i + 1; j < len(relPaths); j++ {
			a := "file:/workspace/" + strings.TrimPrefix(filepath.ToSlash(relPaths[i]), "/")
			b := "file:/workspace/" + strings.TrimPrefix(filepath.ToSlash(relPaths[j]), "/")
			if a == b {
				continue
			}
			_, _ = db.ExecContext(ctx, `
				INSERT INTO connectome_edges (src, dst, kind, weight, updated_at)
				VALUES (?, ?, 'CO_CHANGED', 1.0, ?)
				ON CONFLICT(src, dst, kind) DO UPDATE SET
					weight = connectome_edges.weight + 0.5,
					updated_at = excluded.updated_at
			`, a, b, now)
		}
	}
}

// UpsertUncertaintyNode creates or updates a typed UNCERTAINTY or CONTRADICTION node in the shared Connectome.
// Layer namespace rules (§1.2): L2=2, L3=3, L4=4, L5=5, L6=6.
// Closing a node (`state == "CLOSED"`) requires a harness-minted evidenceRef (`tht_*`, `spn_*`, `test_*`, or `human_*`).
func (e *Engine) UpsertUncertaintyNode(ctx context.Context, node ConnectomeNode) error {
	db := e.getDB()
	if db == nil {
		return nil
	}
	if node.State == "CLOSED" {
		ref := strings.TrimSpace(node.EvidenceRef)
		if !IsHarnessMintedEvidenceRef(ref) {
			return fmt.Errorf("cannot close uncertainty %s without a harness-minted evidence_ref (got %q)", node.ID, ref)
		}
	}
	if node.Kind == "" {
		node.Kind = "UNCERTAINTY"
	}
	if node.State == "" {
		node.State = "OPEN"
	}
	if node.BlockingWeight <= 0 {
		node.BlockingWeight = 1.0
	}
	if node.InfoValue <= 0 {
		node.InfoValue = 1.0
	}
	if node.ExpectedCost <= 0 {
		node.ExpectedCost = 1.0
	}
	now := time.Now().UTC()
	_, err := db.ExecContext(ctx, `
		INSERT INTO connectome_nodes (
			id, kind, path, symbol, summary, hash, size_bytes, state, owner_layer,
			blocking_weight, info_value, expected_cost, evidence_ref, cycle_epoch, updated_at
		) VALUES (?, ?, ?, ?, ?, '', 0, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			summary = CASE WHEN excluded.summary != '' THEN excluded.summary ELSE connectome_nodes.summary END,
			state = excluded.state,
			blocking_weight = excluded.blocking_weight,
			info_value = excluded.info_value,
			expected_cost = excluded.expected_cost,
			evidence_ref = CASE WHEN excluded.evidence_ref != '' THEN excluded.evidence_ref ELSE connectome_nodes.evidence_ref END,
			updated_at = excluded.updated_at
	`, node.ID, node.Kind, node.Path, node.Symbol, node.Summary, node.State, node.OwnerLayer,
		node.BlockingWeight, node.InfoValue, node.ExpectedCost, node.EvidenceRef, node.CycleEpoch, now)
	return err
}

// AcquireFileLease acquires or renews an exclusive file lease for a task workpad (§6.2).
func (e *Engine) AcquireFileLease(ctx context.Context, path string, taskID string, ttl time.Duration) (bool, string) {
	db := e.getDB()
	if db == nil {
		return true, ""
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	now := time.Now().UTC()
	clean := filepath.ToSlash(strings.TrimSpace(path))

	var holder string
	var exp time.Time
	err := db.QueryRowContext(ctx, `SELECT task_id, expires_at FROM file_leases WHERE path = ?`, clean).Scan(&holder, &exp)
	if err == nil && holder != taskID && exp.After(now) {
		return false, holder
	}
	_, _ = db.ExecContext(ctx, `
		INSERT INTO file_leases (path, task_id, leased_at, expires_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			task_id = excluded.task_id,
			leased_at = excluded.leased_at,
			expires_at = excluded.expires_at
	`, clean, taskID, now, now.Add(ttl))
	return true, ""
}

func (e *Engine) ReleaseTaskLeases(ctx context.Context, taskID string) {
	db := e.getDB()
	if db == nil {
		return
	}
	_, _ = db.ExecContext(ctx, `DELETE FROM file_leases WHERE task_id = ?`, taskID)
}

// StageWorkpadFile stages a written file in `.ace-sandbox/workpads/<task_id>/` and atomically merges it into `/workspace/`.
func (e *Engine) StageWorkpadFile(taskID string, relPath string, content []byte) error {
	wsDir, _ := e.resolveSandboxDirs()
	if wsDir == "" || taskID == "" {
		return nil
	}
	sandboxBase := filepath.Dir(wsDir)
	cleanRel := strings.TrimPrefix(filepath.ToSlash(relPath), "/")
	wpFile := filepath.Join(sandboxBase, "workpads", sanitizeTaskID(taskID), cleanRel)
	if err := os.MkdirAll(filepath.Dir(wpFile), 0o755); err != nil {
		return err
	}
	return os.WriteFile(wpFile, content, 0o644)
}

func (e *Engine) MergeWorkpadToWorkspace(ctx context.Context, taskID string) ([]string, error) {
	wsDir, _ := e.resolveSandboxDirs()
	if wsDir == "" || taskID == "" {
		return nil, nil
	}
	sandboxBase := filepath.Dir(wsDir)
	wpRoot := filepath.Join(sandboxBase, "workpads", sanitizeTaskID(taskID))
	if _, err := os.Stat(wpRoot); err != nil {
		return nil, nil
	}
	var merged []string
	err := filepath.WalkDir(wpRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, rerr := filepath.Rel(wpRoot, p)
		if rerr != nil {
			return rerr
		}
		dst := filepath.Join(wsDir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		srcF, err := os.Open(p)
		if err != nil {
			return err
		}
		defer srcF.Close()
		dstF, err := os.Create(dst)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(dstF, srcF)
		dstF.Close()
		if copyErr != nil {
			return copyErr
		}
		merged = append(merged, filepath.ToSlash(rel))
		return nil
	})
	e.ReleaseTaskLeases(ctx, taskID)
	return merged, err
}
