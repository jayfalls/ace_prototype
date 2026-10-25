package cognitive

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"ace/internal/core"
)

const DefaultSliceMaxBytes = 8192

type ConnectomeNode struct {
	ID             string    `json:"id"`
	Kind           string    `json:"kind"` // FILE_MODULE, SYMBOL, DATASET, TEST, SPEC_CLAUSE, METRIC_BELIEF, CITATION, UNCERTAINTY, CONTRADICTION
	Path           string    `json:"path"`
	Symbol         string    `json:"symbol"`
	Summary        string    `json:"summary"`
	Hash           string    `json:"hash"`
	SizeBytes      int       `json:"size_bytes"`
	State          string    `json:"state"` // UNEXPLORED, INSPECTED, IMPLEMENTED, VERIFIED, OPEN, PROBING, CLOSED, QUARANTINED, DEFAULTED
	OwnerLayer     int       `json:"owner_layer"`
	BlockingWeight float64   `json:"blocking_weight"`
	InfoValue      float64   `json:"info_value"`
	ExpectedCost   float64   `json:"expected_cost"`
	EvidenceRef    string    `json:"evidence_ref"`
	CycleEpoch     int64     `json:"cycle_epoch"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (n ConnectomeNode) VOI() float64 {
	cost := n.ExpectedCost
	if cost <= 0.05 {
		cost = 0.05
	}
	return (n.BlockingWeight * n.InfoValue) / cost
}

type ConnectomeEdge struct {
	Src    string  `json:"src"`
	Dst    string  `json:"dst"`
	Kind   string  `json:"kind"` // IMPORTS, CALLS, MENTIONS, CO_CHANGED, READS_DATA, PRODUCES, TESTED_BY, SPECIFIES, DEPENDS_ON, CONTRADICTS
	Weight float64 `json:"weight"`
}

var (
	connectomePathTokenRe = regexp.MustCompile(`(?:/workspace/|/wiki/)?[\w./-]*[\w-]+\.(?:jsonl|json|parquet|py|go|ts|js|sh|sql|db|csv|md|txt)\b`)
	connectomeWordTokenRe = regexp.MustCompile(`[a-z][a-z0-9]{3,}`)
	pyDefClassLineRe      = regexp.MustCompile(`^\s*(def|class)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(\([^)]*\))?`)
	goFuncTypeLineRe      = regexp.MustCompile(`^\s*func\s+(?:\([^)]+\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\([^)]*\)|^\s*type\s+([A-Za-z_][A-Za-z0-9_]*)\s+(?:struct|interface)`)
)

func (e *Engine) getDB() *sql.DB {
	if s, ok := e.memory.(interface{ DB() *sql.DB }); ok {
		return s.DB()
	}
	return nil
}

func SliceBudgetMaxBytes() int {
	if raw := strings.TrimSpace(os.Getenv("ACE_SLICE_MAX_BYTES")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 512 && v <= 65536 {
			return v
		}
	}
	return DefaultSliceMaxBytes
}

// RefreshConnectomeGraph deterministically indexes /workspace and /wiki files, builds compact summaries,
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

func classifyConnectomeFileKind(vpath string) string {
	lower := strings.ToLower(vpath)
	switch {
	case strings.HasPrefix(lower, "/wiki/specs/"):
		return "SPEC_CLAUSE"
	case strings.Contains(lower, "test"):
		return "TEST"
	case strings.HasSuffix(lower, ".db") || strings.HasSuffix(lower, ".json") || strings.HasSuffix(lower, ".jsonl") || strings.HasSuffix(lower, ".csv") || strings.HasSuffix(lower, ".parquet"):
		return "DATASET"
	default:
		return "FILE_MODULE"
	}
}

func summarizeFileForConnectome(vpath string, hpath string, sz int) (summary string, rawSample string, hashHex string) {
	f, err := os.Open(hpath)
	if err != nil {
		return fmt.Sprintf("%s (%dB)", vpath, sz), "", ""
	}
	defer f.Close()

	buf := make([]byte, 32768)
	n, _ := f.Read(buf)
	sample := buf[:n]
	sum := sha256.Sum256(sample)
	hashHex = hex.EncodeToString(sum[:8])

	ext := strings.ToLower(filepath.Ext(vpath))
	if ext == ".db" || ext == ".sqlite" || ext == ".parquet" {
		return fmt.Sprintf("%s (%dB) binary dataset/sqlite", vpath, sz), "", hashHex
	}

	rawSample = string(sample)
	lines := strings.Split(rawSample, "\n")

	switch ext {
	case ".py":
		var defs []string
		for _, line := range lines {
			if pyDefClassLineRe.MatchString(line) {
				clean := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), ":"))
				if len(clean) > 72 {
					clean = clean[:72]
				}
				defs = append(defs, clean)
				if len(defs) >= 5 {
					break
				}
			}
		}
		if len(defs) > 0 {
			return fmt.Sprintf("%s (%dB) defs: %s", vpath, sz, strings.Join(defs, "; ")), rawSample, hashHex
		}
	case ".go":
		var defs []string
		for _, line := range lines {
			if goFuncTypeLineRe.MatchString(line) {
				clean := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), "{"))
				if len(clean) > 72 {
					clean = clean[:72]
				}
				defs = append(defs, clean)
				if len(defs) >= 5 {
					break
				}
			}
		}
		if len(defs) > 0 {
			return fmt.Sprintf("%s (%dB) symbols: %s", vpath, sz, strings.Join(defs, "; ")), rawSample, hashHex
		}
	case ".json", ".jsonl":
		var obj map[string]any
		if json.Unmarshal(sample, &obj) == nil && len(obj) > 0 {
			keys := make([]string, 0, len(obj))
			for k := range obj {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			if len(keys) > 8 {
				keys = keys[:8]
			}
			return fmt.Sprintf("%s (%dB) keys=[%s]", vpath, sz, strings.Join(keys, ", ")), rawSample, hashHex
		}
	case ".md", ".txt":
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				if len(trimmed) > 85 {
					trimmed = trimmed[:85]
				}
				return fmt.Sprintf("%s (%dB) %s", vpath, sz, trimmed), rawSample, hashHex
			}
		}
	}
	return fmt.Sprintf("%s (%dB)", vpath, sz), rawSample, hashHex
}

func resolveConnectomePathToken(tok string, files map[string]string, byBase map[string][]string) []string {
	clean := strings.TrimSpace(tok)
	if _, ok := files[clean]; ok {
		return []string{clean}
	}
	trimmed := strings.TrimLeft(clean, "/")
	for _, pref := range []string{"/workspace/", "/wiki/"} {
		if _, ok := files[pref+trimmed]; ok {
			return []string{pref + trimmed}
		}
	}
	return byBase[filepath.Base(clean)]
}

// RankConnectomeCandidates executes Reciprocal Rank Fusion (RRF, k=60) over:
// 1. Personalized PageRank (alpha=0.6, 25 iterations) seeded from directive + spec paths, and
// 2. Lexical IDF overlap between (directive + goal + spec) tokens and node path/summary tokens.
func RankConnectomeCandidates(nodes map[string]ConnectomeNode, adj map[string][]string, directive string, goalText string, specText string) []ConnectomeNode {
	files := make(map[string]string, len(nodes))
	byBase := make(map[string][]string)
	for _, n := range nodes {
		if n.Path != "" {
			files[n.Path] = n.ID
			byBase[filepath.Base(n.Path)] = append(byBase[filepath.Base(n.Path)], n.Path)
		}
	}

	// Extract explicit path seeds from directive + spec
	seedSet := make(map[string]bool)
	for _, match := range connectomePathTokenRe.FindAllString(directive+"\n"+specText, -1) {
		for _, vp := range resolveConnectomePathToken(match, files, byBase) {
			if id, ok := files[vp]; ok {
				seedSet[id] = true
			}
		}
	}

	// 1. Personalized PageRank over undirected graph
	ppr := make(map[string]float64)
	if len(seedSet) > 0 {
		seedWeight := 1.0 / float64(len(seedSet))
		for s := range seedSet {
			ppr[s] = seedWeight
		}
		const alpha = 0.6
		for iter := 0; iter < 25; iter++ {
			next := make(map[string]float64, len(nodes))
			for u, val := range ppr {
				nbs := adj[u]
				if len(nbs) == 0 {
					next[u] += alpha * val
					continue
				}
				share := (alpha * val) / float64(len(nbs))
				for _, v := range nbs {
					next[v] += share
				}
			}
			for s := range seedSet {
				next[s] += (1.0 - alpha) * seedWeight
			}
			ppr = next
		}
	}

	// 2. Lexical IDF scoring over path + summary tokens
	nodeToks := make(map[string]map[string]bool, len(nodes))
	df := make(map[string]int)
	for id, n := range nodes {
		raw := strings.ToLower(strings.NewReplacer("/", " ", "_", " ", ".", " ", "-", " ").Replace(n.Path + " " + n.Summary))
		tset := make(map[string]bool)
		for _, w := range connectomeWordTokenRe.FindAllString(raw, -1) {
			tset[w] = true
		}
		nodeToks[id] = tset
		for w := range tset {
			df[w]++
		}
	}

	queryToks := make(map[string]bool)
	qRaw := strings.ToLower(strings.NewReplacer("/", " ", "_", " ", ".", " ", "-", " ").Replace(directive + " " + goalText + " " + specText))
	for _, w := range connectomeWordTokenRe.FindAllString(qRaw, -1) {
		queryToks[w] = true
	}

	nTotal := float64(len(nodes) + 1)
	lex := make(map[string]float64, len(nodes))
	for id, tset := range nodeToks {
		var score float64
		for w := range queryToks {
			if tset[w] && df[w] > 0 {
				score += math.Log(nTotal / float64(df[w]))
			}
		}
		if score > 0 {
			lex[id] = score
		}
	}

	// 3. Reciprocal Rank Fusion (k=60)
	rrf := make(map[string]float64, len(nodes))
	addRankList := func(scores map[string]float64) {
		ids := make([]string, 0, len(scores))
		for id, sc := range scores {
			if sc > 0 {
				ids = append(ids, id)
			}
		}
		sort.Slice(ids, func(i, j int) bool {
			if scores[ids[i]] == scores[ids[j]] {
				return ids[i] < ids[j]
			}
			return scores[ids[i]] > scores[ids[j]]
		})
		if len(ids) > 100 {
			ids = ids[:100]
		}
		for rank, id := range ids {
			rrf[id] += 1.0 / float64(60+rank)
		}
	}
	addRankList(ppr)
	addRankList(lex)

	// Boost explicit seed paths so direct anchors are always top-ranked
	for s := range seedSet {
		rrf[s] += 0.05
	}

	all := make([]ConnectomeNode, 0, len(nodes))
	for _, n := range nodes {
		if strings.HasPrefix(n.ID, "file:") {
			all = append(all, n)
		}
	}
	sort.Slice(all, func(i, j int) bool {
		si, sj := rrf[all[i].ID], rrf[all[j].ID]
		if si == sj {
			return all[i].ID < all[j].ID
		}
		return si > sj
	})
	return all
}

// BuildConnectomeSlice packs ranked candidate summaries + open VOI-sorted uncertainties into the separate Slice Budget (default 8KB).
// Also supports Turn 0.5 router behind ACE_TURN05=on|off|ab.
func (e *Engine) BuildConnectomeSlice(ctx context.Context, cycleID string, milestoneID string, directive string, goalText string, specText string) (string, map[string]bool) {
	nodes, adj := e.RefreshConnectomeGraph(ctx, time.Now().Unix())
	ranked := RankConnectomeCandidates(nodes, adj, directive, goalText, specText)

	mode := strings.ToLower(strings.TrimSpace(os.Getenv("ACE_TURN05")))
	if mode == "" {
		mode = "ab"
	}
	useTurn05 := false
	switch mode {
	case "on", "true", "1":
		useTurn05 = true
	case "ab":
		sum := sha256.Sum256([]byte(milestoneID + "|" + directive))
		useTurn05 = (sum[0]%2 == 0)
	}

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

	rctx, cancel := context.WithTimeout(ctx, 12*time.Second)
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

func IsHarnessMintedEvidenceRef(ref string) bool {
	for _, pref := range []string{"tht_", "spn_", "span_", "cyc_", "test_", "human_", "selftest_"} {
		if strings.HasPrefix(ref, pref) && len(ref) > len(pref)+3 {
			return true
		}
	}
	return false
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

func sanitizeTaskID(raw string) string {
	clean := regexp.MustCompile(`[^A-Za-z0-9_.-]+`).ReplaceAllString(strings.TrimSpace(raw), "_")
	if len(clean) > 48 {
		clean = clean[:48]
	}
	if clean == "" {
		return "default_task"
	}
	return clean
}
