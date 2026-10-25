package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"ace/internal/core"
	"github.com/google/uuid"
)

type Store struct {
	db       *sql.DB
	wikiRoot *os.Root
	wikiDir  string
	telem    core.TelemetrySink
}

func NewStore(db *sql.DB, wikiRoot *os.Root, wikiDir string, telem core.TelemetrySink) (*Store, error) {
	s := &Store{
		db:       db,
		wikiRoot: wikiRoot,
		wikiDir:  wikiDir,
		telem:    telem,
	}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	if err := s.seedWiki(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS memories (
		id TEXT PRIMARY KEY,
		agent_id TEXT NOT NULL,
		layer_code TEXT NOT NULL,
		tier TEXT NOT NULL DEFAULT 'medium',
		category TEXT NOT NULL,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		tags TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		type TEXT,
		source TEXT,
		verified_at DATETIME,
		importance REAL
	);
	CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
		id UNINDEXED,
		title,
		content,
		tags,
		category
	);
	CREATE TABLE IF NOT EXISTS chat_messages (
		id TEXT PRIMARY KEY,
		agent_id TEXT NOT NULL,
		role TEXT NOT NULL,
		source TEXT NOT NULL,
		content TEXT NOT NULL,
		reasoning TEXT NOT NULL,
		model TEXT NOT NULL,
		latency_ms INTEGER NOT NULL,
		cycle_id TEXT NOT NULL,
		created_at DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_chat_messages_created ON chat_messages(created_at ASC);
	CREATE TABLE IF NOT EXISTS layer_states (
		layer_id INTEGER PRIMARY KEY,
		code TEXT NOT NULL,
		name TEXT NOT NULL,
		role TEXT NOT NULL,
		state TEXT NOT NULL,
		summary TEXT NOT NULL,
		active_directive TEXT NOT NULL,
		northbound_signal TEXT NOT NULL,
		last_action TEXT NOT NULL,
		confidence REAL NOT NULL,
		model TEXT NOT NULL,
		last_latency_ms INTEGER NOT NULL,
		total_invocations INTEGER NOT NULL,
		inner_loop_step INTEGER NOT NULL,
		inner_loop_max INTEGER NOT NULL,
		updated_at DATETIME NOT NULL
	);
	CREATE TABLE IF NOT EXISTS engine_meta (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME NOT NULL
	);
	CREATE TABLE IF NOT EXISTS connectome_nodes (
		id TEXT PRIMARY KEY,
		kind TEXT NOT NULL,
		path TEXT NOT NULL DEFAULT '',
		symbol TEXT NOT NULL DEFAULT '',
		summary TEXT NOT NULL DEFAULT '',
		hash TEXT NOT NULL DEFAULT '',
		size_bytes INTEGER NOT NULL DEFAULT 0,
		state TEXT NOT NULL DEFAULT 'UNEXPLORED',
		owner_layer INTEGER NOT NULL DEFAULT 0,
		blocking_weight REAL NOT NULL DEFAULT 1.0,
		info_value REAL NOT NULL DEFAULT 1.0,
		expected_cost REAL NOT NULL DEFAULT 1.0,
		evidence_ref TEXT NOT NULL DEFAULT '',
		cycle_epoch INTEGER NOT NULL DEFAULT 0,
		updated_at DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_connectome_nodes_kind ON connectome_nodes(kind, state);
	CREATE TABLE IF NOT EXISTS connectome_edges (
		src TEXT NOT NULL,
		dst TEXT NOT NULL,
		kind TEXT NOT NULL,
		weight REAL NOT NULL DEFAULT 1.0,
		updated_at DATETIME NOT NULL,
		PRIMARY KEY (src, dst, kind)
	);
	CREATE INDEX IF NOT EXISTS idx_connectome_edges_dst ON connectome_edges(dst);
	CREATE TABLE IF NOT EXISTS spec_registry (
		milestone_id TEXT PRIMARY KEY,
		spec_path TEXT NOT NULL,
		frozen_hash TEXT NOT NULL,
		size_bytes INTEGER NOT NULL,
		status TEXT NOT NULL DEFAULT 'LIVING',
		updated_at DATETIME NOT NULL
	);
	CREATE TABLE IF NOT EXISTS file_leases (
		path TEXT PRIMARY KEY,
		task_id TEXT NOT NULL,
		leased_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL
	);
	CREATE TABLE IF NOT EXISTS veto_ledger (
		veto_hash TEXT PRIMARY KEY,
		milestone_id TEXT NOT NULL,
		veto_kind TEXT NOT NULL,
		target_path TEXT NOT NULL,
		count INTEGER NOT NULL DEFAULT 0,
		last_reason TEXT NOT NULL DEFAULT '',
		last_cycle_id TEXT NOT NULL DEFAULT '',
		disputed INTEGER NOT NULL DEFAULT 0,
		updated_at DATETIME NOT NULL
	);
	CREATE TABLE IF NOT EXISTS human_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source TEXT NOT NULL,
		event_ts DATETIME NOT NULL,
		hour_utc INTEGER NOT NULL,
		reply_latency_sec REAL NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_human_events_hour ON human_events(hour_utc);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	// Ensure backward-compatible migration for existing DBs without the 'tier' column
	_, _ = s.db.Exec(`ALTER TABLE memories ADD COLUMN tier TEXT NOT NULL DEFAULT 'medium'`)
	// v0.9 memory metadata (spec 9.6); tolerated ALTERs for existing DBs.
	_, _ = s.db.Exec(`ALTER TABLE memories ADD COLUMN type TEXT`)
	_, _ = s.db.Exec(`ALTER TABLE memories ADD COLUMN source TEXT`)
	_, _ = s.db.Exec(`ALTER TABLE memories ADD COLUMN verified_at DATETIME`)
	_, _ = s.db.Exec(`ALTER TABLE memories ADD COLUMN importance REAL`)
	// Drop the retired approval column from pre-autonomy schemas (error
	// tolerated: absent column or old sqlite — the table is authoritative either way).
	_, _ = s.db.Exec(`ALTER TABLE spec_registry DROP COLUMN human_approval_ref`)
	return nil
}

func (s *Store) seedWiki() error {
	defaults := map[string]string{
		"index.md": `# ACE Knowledge Wiki Index

## Canonical Architecture, Strategy & Research Pages
- The constitution (Heuristic Imperatives, UDHR, autonomy triad, epistemic law) is compiled into the binary and pinned by SHA-256 — it is not a wiki page and cannot be edited by any layer.
- The owner's mission lives in the owner-set config (ace init / ace mission reload), never in the wiki.
- [goals.md](goals.md) — Active User Mission, Strategic Roadmap & Falsifiable Milestones
- [log.md](log.md) — Chronological 6-layer cognitive & experimental log
- [notes/architecture.md](notes/architecture.md) — ACE 6-Layer Bidirectional Bus & Cognitive Architecture Map
`,
		"goals.md": `# Active Mission & Strategic Roadmap

## Overall North-Star Goal
Give ACE any goal in this file (or via Chat) — even a single sentence with zero prior context. Layer 2 (Global Strategy) will automatically research the domain (via ` + "`web_search`" + ` and ` + "`web_fetch`" + `), decompose the goal into concrete ` + "`- [ ]`" + ` milestones, construct real datasets in ` + "`/workspace/datasets/`" + `, engineer modular code in ` + "`/workspace/`" + `, and empirically verify every milestone via Mode-3 falsification gates.
`,
		"log.md": fmt.Sprintf("# ACE 6-Layer Cognitive Log\n\n## [%s] ACE Initialized\n- 6-Layer Cognitive Architecture (L1 Aspirational, L2 Global Strategy, L3 Agent Model, L4 Executive, L5 Cognitive Control, L6 Task Prosecution) online with Northbound & Southbound buses.\n", time.Now().UTC().Format(time.RFC3339)),
		"notes/architecture.md": `# ACE 6-Layer Cognitive Architecture (Shapiro et al., arXiv:2310.06775)

- **L1 Aspirational**: Heuristic Imperatives, ethical constitution (soul.md), and epistemic integrity (introspective; Southbound output).
- **L2 Global Strategy**: Environmental grounding, live web/arXiv literature discovery, and strategic roadmap generation (goals.md).
- **L3 Agent Model**: Proprioceptive self-telemetry, capability/limitation tracking, episodic/semantic memory, and dataset architecture.
- **L4 Executive Function**: Project roadmap, resource/risk management, contingency planning, and H0/H1 falsifiable contracts.
- **L5 Cognitive Control**: Task selection, dynamic task switching, frustration tolerance/damping, and System-1 GPU gating.
- **L6 Task Prosecution**: Environmental actuators, dataset builder, modular code engineer, and Mode-3 self-test verifier.
`,
		"skills/ponytail.md": `# Skill: Ponytail (Pragmatic Senior Engineering Mode)
- Ship the smallest, cleanest code change that fully solves the task and passes self-tests.
- Prefer Python 3 stdlib in /workspace with deterministic --selftest modes and zero external bloat.
- Always verify every script or file edit by running it inside the sandbox before setting done=true.
`,
		"skills/tickets.md": `# Skill: Tickets (Task & Evidence Tracker in /workspace/bin/ticket)
- Track multi-step work using the pre-installed CLI: ticket add "<title>" --goal "<goal>"
- Log progress & checklist items: ticket log <id> --did "..." --doing "..." --learned "..." --new-item "<step>" --pad
- Record empirical verification proof: ticket evidence <id> <idx> "<test stdout>"
`,
		"skills/knowledge_base.md": `# Skill: Knowledge Base CLI (/workspace/bin/kb)
- Search prior knowledge & Wiki before deriving from scratch: kb search "<query>"
- Record non-obvious findings & architectural discoveries: kb note "<title>" --body "..." --tags "..."
`,
		"skills/dataset_construction.md": `# Skill: Dataset Construction & Registry
- Datasets live in /workspace/datasets/<dataset_id>.json with fields: dataset_id, version, provenance, train_split, held_out_split.
- Split deterministically (e.g. SHA-256 of a stable record ID) and register each dataset in /workspace/registry.json.
- Record per-item predictions in /workspace/results/ so the gate can recompute statistics; never self-declare verdicts.
`,
		"skills/harness2_propose_probe_compose.md": `# Skill: Harness^2 Recursive Agent Harnessing (arXiv:2610.14224)
- Separate Task-Level Execution Harness (propose-probe-compose candidate harness units on held-out tasks) from the Harnessing-Level Continual Improvement Library (retaining validated components and edit-contrast lessons across tasks).
- Prefer on-demand C6 Skills (/wiki/skills/*.md) and C7 Scripts/Tools (/workspace/) over always-on system prompt bloat (C1-C4).
`,
		"skills/hierarchical_memory.md": `# Skill: Hierarchical Memory & Architecture Synthesis
- Compare and synthesize memory, skill, and tool architectures from research gathered via web_search/web_fetch and the workspace corpus.
- Store reusable procedural workflows as C6 Skills (/wiki/skills/*.md) and counter-evidence-aware semantic facts via memory_remember.
`,
	}

	for path, content := range defaults {
		if _, err := s.ReadWiki(path); err != nil {
			if err := s.WriteWiki(path, content); err != nil {
				return err
			}
		}
	}
	return nil
}

var pulsePrefixRe = regexp.MustCompile(`(?i)\b(pulse|epoch|cyc|cycle)[_-]?[a-z0-9]+\b`)

func normalizeMemoryKey(s string) string {
	cleaned := pulsePrefixRe.ReplaceAllString(strings.ToLower(s), "")
	cleaned = strings.NewReplacer("_", " ", "-", " ", ":", " ", ".", " ").Replace(cleaned)
	return strings.Join(strings.Fields(cleaned), " ")
}

func wordJaccardSimilarity(a, b string) float64 {
	wordsA := strings.Fields(normalizeMemoryKey(a))
	wordsB := strings.Fields(normalizeMemoryKey(b))
	if len(wordsA) == 0 || len(wordsB) == 0 {
		return 0
	}
	setA := make(map[string]bool, len(wordsA))
	for _, w := range wordsA {
		if len(w) > 2 {
			setA[w] = true
		}
	}
	setB := make(map[string]bool, len(wordsB))
	for _, w := range wordsB {
		if len(w) > 2 {
			setB[w] = true
		}
	}
	if len(setA) == 0 || len(setB) == 0 {
		return 0
	}
	inter := 0
	for w := range setA {
		if setB[w] {
			inter++
		}
	}
	union := len(setA) + len(setB) - inter
	if union <= 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// Remember admits an entry under the writer's authority. Storage-level
// rule (spec 9.6): a layer may only write to its own layer_code, and L1
// may write anywhere — cross-layer writes are rejected here, not in
// prompts.
func (s *Store) Remember(ctx context.Context, callerLayer string, entry core.MemoryEntry) (core.MemoryEntry, error) {
	if err := checkWriteRight(callerLayer, entry.LayerCode); err != nil {
		return entry, err
	}
	if err := validateType(entry); err != nil {
		return entry, err
	}
	now := time.Now().UTC()
	if entry.ID == "" {
		entry.ID = "mem_" + uuid.NewString()[:12]
	}
	if entry.AgentID == "" {
		entry.AgentID = "ace-primary"
	}
	if entry.Tier == "" {
		entry.Tier = "medium"
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	if entry.Importance <= 0 {
		entry.Importance = 1
	}
	// A caller-supplied updated_at drives the stale-write check (spec:
	// older updated_at writes are rejected); absent one, now is fresh.
	callerStamped := !entry.UpdatedAt.IsZero()
	if !callerStamped {
		entry.UpdatedAt = now
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return entry, err
	}
	defer func() { _ = tx.Rollback() }()

	// 1. Exact title match or normalized-key / Jaccard semantic deduplication gate within the same layer
	var existingID string
	_ = tx.QueryRowContext(ctx, `SELECT id FROM memories WHERE layer_code = ? AND title = ? LIMIT 1`, entry.LayerCode, entry.Title).Scan(&existingID)
	if existingID == "" {
		normNewTitle := normalizeMemoryKey(entry.Title)
		newCombined := entry.Title + " " + entry.Content
		rows, qErr := tx.QueryContext(ctx, `SELECT id, title, content FROM memories WHERE layer_code = ? ORDER BY updated_at DESC LIMIT 20`, entry.LayerCode)
		if qErr == nil {
			for rows.Next() {
				var candID, candTitle, candContent string
				if rows.Scan(&candID, &candTitle, &candContent) == nil {
					if normNewTitle != "" && (normalizeMemoryKey(candTitle) == normNewTitle || wordJaccardSimilarity(entry.Title, candTitle) >= 0.50) {
						existingID = candID
						break
					}
					if wordJaccardSimilarity(newCombined, candTitle+" "+candContent) >= 0.42 {
						existingID = candID
						break
					}
				}
			}
			rows.Close()
		}
	}
	if existingID != "" {
		if callerStamped {
			// Monotonic-write rule: a caller-stamped update older than the
			// stored row is stale and rejected inside the same tx.
			var prevUpdated string
			if err := tx.QueryRowContext(ctx, `SELECT updated_at FROM memories WHERE id = ?`, existingID).Scan(&prevUpdated); err == nil {
				if pt, perr := time.Parse(tsLayout, prevUpdated); perr == nil && entry.UpdatedAt.Before(pt) {
					return entry, fmt.Errorf("stale write rejected: updated_at %s precedes stored %s", entry.UpdatedAt, pt)
				}
			}
		}
		entry.ID = existingID
		_, _ = tx.ExecContext(ctx, `
			UPDATE memories SET tier = ?, category = ?, title = ?, content = ?, tags = ?, updated_at = ?,
				type = ?, source = ?, verified_at = ?, importance = ?
			WHERE id = ?
		`, entry.Tier, entry.Category, entry.Title, entry.Content, entry.Tags, entry.UpdatedAt,
			entry.Type, entry.Source, nullTime(entry.VerifiedAt), entry.Importance, existingID)
		_, _ = tx.ExecContext(ctx, `DELETE FROM memories_fts WHERE id = ?`, existingID)
	} else {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO memories (id, agent_id, layer_code, tier, category, title, content, tags, created_at, updated_at, type, source, verified_at, importance)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, entry.ID, entry.AgentID, entry.LayerCode, entry.Tier, entry.Category, entry.Title, entry.Content, entry.Tags, entry.CreatedAt, entry.UpdatedAt,
			entry.Type, entry.Source, nullTime(entry.VerifiedAt), entry.Importance)
		if err != nil {
			return entry, err
		}
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO memories_fts (id, title, content, tags, category)
		VALUES (?, ?, ?, ?, ?)
	`, entry.ID, entry.Title, entry.Content, entry.Tags, entry.Category)
	if err != nil {
		return entry, err
	}

	// Episodic cap: over the limit, evict the lowest retention score
	// inside the same transaction so the cap is atomic with the write.
	if err := enforceEpisodicCap(ctx, tx); err != nil {
		return entry, err
	}

	if err := tx.Commit(); err != nil {
		return entry, err
	}

	if s.telem != nil {
		s.telem.EmitLive("memory.recorded", entry)
	}
	return entry, nil
}

func (s *Store) Search(ctx context.Context, query string, limit int) ([]core.MemoryEntry, error) {
	if limit <= 0 || limit > 50 {
		limit = 15
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return s.RecentMemories(ctx, limit)
	}

	// Sanitize FTS5 query tokens into prefix OR matches
	fields := strings.FieldsFunc(query, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	})
	if len(fields) == 0 {
		return s.RecentMemories(ctx, limit)
	}
	ftsTerms := make([]string, 0, len(fields))
	for _, f := range fields {
		if len(f) > 1 {
			ftsTerms = append(ftsTerms, fmt.Sprintf(`"%s"*`, f))
		}
	}
	if len(ftsTerms) == 0 {
		return s.RecentMemories(ctx, limit)
	}
	ftsExpr := strings.Join(ftsTerms, " OR ")

	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.agent_id, m.layer_code, m.tier, m.category, m.title, m.content, m.tags, m.created_at, m.updated_at
		FROM memories_fts f
		JOIN memories m ON m.id = f.id
		WHERE memories_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, ftsExpr, limit)
	if err != nil {
		return s.RecentMemories(ctx, limit)
	}
	defer rows.Close()

	var out []core.MemoryEntry
	for rows.Next() {
		var m core.MemoryEntry
		if err := rows.Scan(&m.ID, &m.AgentID, &m.LayerCode, &m.Tier, &m.Category, &m.Title, &m.Content, &m.Tags, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SearchForLayer enforces the original ACE design's Layer Memory Isolation:
// Layer N only sees its own layer memories (short/medium/long) + shared GLOBAL/USER memories.
func (s *Store) SearchForLayer(ctx context.Context, layerCode string, query string, limit int) ([]core.MemoryEntry, error) {
	if limit <= 0 || limit > 30 {
		limit = 8
	}
	all, err := s.Search(ctx, query, limit*3)
	if err != nil {
		return nil, err
	}
	filtered := make([]core.MemoryEntry, 0, limit)
	seen := make(map[string]bool)
	for _, m := range all {
		if m.LayerCode == layerCode || m.LayerCode == "GLOBAL" || m.LayerCode == "USER" {
			filtered = append(filtered, m)
			seen[m.ID] = true
			if len(filtered) >= limit {
				return filtered, nil
			}
		}
	}
	// Also always inject recent short/medium-term memories for this layer + GLOBAL
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, agent_id, layer_code, tier, category, title, content, tags, created_at, updated_at
		FROM memories
		WHERE layer_code IN (?, 'GLOBAL', 'USER')
		ORDER BY updated_at DESC
		LIMIT ?
	`, layerCode, limit)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var m core.MemoryEntry
			if err := rows.Scan(&m.ID, &m.AgentID, &m.LayerCode, &m.Tier, &m.Category, &m.Title, &m.Content, &m.Tags, &m.CreatedAt, &m.UpdatedAt); err == nil {
				if !seen[m.ID] && len(filtered) < limit {
					filtered = append(filtered, m)
					seen[m.ID] = true
				}
			}
		}
	}
	return filtered, nil
}

func (s *Store) RecentMemories(ctx context.Context, limit int) ([]core.MemoryEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, agent_id, layer_code, tier, category, title, content, tags, created_at, updated_at
		FROM memories
		ORDER BY updated_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.MemoryEntry
	for rows.Next() {
		var m core.MemoryEntry
		if err := rows.Scan(&m.ID, &m.AgentID, &m.LayerCode, &m.Tier, &m.Category, &m.Title, &m.Content, &m.Tags, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func cleanWikiPath(relPath string) (string, error) {
	clean := strings.TrimSpace(strings.TrimPrefix(relPath, "/"))
	clean = strings.TrimPrefix(clean, "wiki/")
	if clean == "" || clean == "." {
		return "", errors.New("empty wiki path")
	}
	if strings.Contains(clean, "..") {
		return "", errors.New("wiki path traversal '..' rejected")
	}
	if !strings.HasSuffix(clean, ".md") {
		clean += ".md"
	}
	return clean, nil
}

func (s *Store) ReadWiki(relPath string) (string, error) {
	clean, err := cleanWikiPath(relPath)
	if err != nil {
		return "", err
	}
	f, err := s.wikiRoot.Open(clean)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 128*1024))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Store) WriteWiki(relPath string, content string) error {
	clean, err := cleanWikiPath(relPath)
	if err != nil {
		return err
	}
	dir := filepath.Dir(clean)
	if dir != "." && dir != "" {
		parts := strings.Split(filepath.ToSlash(dir), "/")
		curr := ""
		for _, p := range parts {
			if p == "" || p == "." {
				continue
			}
			if curr == "" {
				curr = p
			}
			if curr != p {
				curr = curr + "/" + p
			}
			if err := s.wikiRoot.Mkdir(curr, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
				return err
			}
		}
	}
	// Smart merge guard for goals.md: if a layer passes only a new "## Phase N" section in wiki_write("goals.md"),
	// preserve existing phases and append/replace the section instead of wiping prior phases!
	if clean == "goals.md" && !strings.Contains(content, "# Active Mission") {
		if existing, rErr := s.ReadWiki("goals.md"); rErr == nil && strings.TrimSpace(existing) != "" {
			trimmedNew := strings.TrimSpace(content)
			if strings.HasPrefix(trimmedNew, "## Phase") {
				headerLine := strings.Split(trimmedNew, "\n")[0]
				if !strings.Contains(existing, headerLine) {
					content = strings.TrimRight(existing, "\n") + "\n\n" + trimmedNew + "\n"
				}
			}
		}
	}
	f, err := s.wikiRoot.OpenFile(clean, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		return err
	}
	// Only auto-link top-level canonical wiki pages (never ephemeral notes/* or archive/*,
	// which previously bloated index.md to 74KB and invalidated the system prompt KV prefix cache!).
	if clean != "index.md" && clean != "log.md" && !strings.Contains(clean, "/") {
		if idx, err := s.ReadWiki("index.md"); err == nil && len(idx) < 2200 && !strings.Contains(idx, "("+clean+")") {
			entryLine := fmt.Sprintf("\n- [%s](%s) — Canonical wiki page", clean, clean)
			_ = s.writeRawWikiFile("index.md", strings.TrimRight(idx, "\n")+entryLine+"\n")
		}
	}
	if s.telem != nil {
		s.telem.EmitLive("wiki.updated", map[string]string{"path": clean})
	}
	return nil
}

func (s *Store) writeRawWikiFile(clean string, content string) error {
	f, err := s.wikiRoot.OpenFile(clean, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

func (s *Store) ListWiki() ([]core.WikiPageMeta, error) {
	var topPages []core.WikiPageMeta
	var notePages []core.WikiPageMeta
	err := filepath.WalkDir(s.wikiDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "archive" || strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, err := filepath.Rel(s.wikiDir, path)
		if err != nil {
			return nil
		}
		slashRel := filepath.ToSlash(rel)
		info, err := d.Info()
		if err != nil {
			return nil
		}
		meta := core.WikiPageMeta{
			Path:      slashRel,
			Title:     strings.TrimSuffix(filepath.Base(rel), ".md"),
			SizeBytes: info.Size(),
			UpdatedAt: info.ModTime().UTC(),
		}
		if strings.HasPrefix(slashRel, "notes/") {
			notePages = append(notePages, meta)
		} else {
			topPages = append(topPages, meta)
		}
		return nil
	})
	// Cap notes/ to the 10 most recent notes so prompt size never grows unboundedly
	sort.Slice(notePages, func(i, j int) bool {
		return notePages[i].UpdatedAt.After(notePages[j].UpdatedAt)
	})
	if len(notePages) > 10 {
		notePages = notePages[:10]
	}
	pages := append(topPages, notePages...)
	sort.Slice(pages, func(i, j int) bool {
		return pages[i].Path < pages[j].Path
	})
	return pages, err
}

func (s *Store) SaveChatMessage(ctx context.Context, msg core.ChatMessage) error {
	if msg.ID == "" {
		msg.ID = "cht_" + uuid.NewString()[:12]
	}
	if msg.AgentID == "" {
		msg.AgentID = "ace-primary"
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now().UTC()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO chat_messages (
			id, agent_id, role, source, content, reasoning, model, latency_ms, cycle_id, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, msg.ID, msg.AgentID, msg.Role, msg.Source, msg.Content, msg.Reasoning, msg.Model, msg.LatencyMs, msg.CycleID, msg.CreatedAt)
	if err != nil {
		return err
	}
	if s.telem != nil {
		s.telem.EmitLive("chat.message", msg)
	}
	return nil
}

func (s *Store) RecentChatMessages(ctx context.Context, limit int) ([]core.ChatMessage, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, agent_id, role, source, content, reasoning, model, latency_ms, cycle_id, created_at
		FROM (
			SELECT id, agent_id, role, source, content, reasoning, model, latency_ms, cycle_id, created_at
			FROM chat_messages
			ORDER BY created_at DESC
			LIMIT ?
		) sub
		ORDER BY created_at ASC
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.ChatMessage
	for rows.Next() {
		var m core.ChatMessage
		if err := rows.Scan(&m.ID, &m.AgentID, &m.Role, &m.Source, &m.Content, &m.Reasoning, &m.Model, &m.LatencyMs, &m.CycleID, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) SaveLayerStatus(ctx context.Context, st core.LayerStatus) error {
	if st.UpdatedAt.IsZero() {
		st.UpdatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO layer_states (
			layer_id, code, name, role, state, summary, active_directive, northbound_signal,
			last_action, confidence, model, last_latency_ms, total_invocations,
			inner_loop_step, inner_loop_max, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(layer_id) DO UPDATE SET
			code = excluded.code,
			name = excluded.name,
			role = excluded.role,
			state = excluded.state,
			summary = excluded.summary,
			active_directive = excluded.active_directive,
			northbound_signal = excluded.northbound_signal,
			last_action = excluded.last_action,
			confidence = excluded.confidence,
			model = excluded.model,
			last_latency_ms = excluded.last_latency_ms,
			total_invocations = excluded.total_invocations,
			inner_loop_step = excluded.inner_loop_step,
			inner_loop_max = excluded.inner_loop_max,
			updated_at = excluded.updated_at
	`, int(st.LayerID), st.Code, st.Name, st.Role, st.State, st.Summary, st.ActiveDirective,
		st.NorthboundSignal, st.LastAction, st.Confidence, st.Model, st.LastLatencyMs,
		st.TotalInvocations, st.InnerLoopStep, st.InnerLoopMax, st.UpdatedAt)
	return err
}

func (s *Store) LoadLayerStatuses(ctx context.Context) ([]core.LayerStatus, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT layer_id, code, name, role, state, summary, active_directive, northbound_signal,
		       last_action, confidence, model, last_latency_ms, total_invocations,
		       inner_loop_step, inner_loop_max, updated_at
		FROM layer_states
		ORDER BY layer_id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.LayerStatus
	for rows.Next() {
		var st core.LayerStatus
		var id int
		if err := rows.Scan(
			&id, &st.Code, &st.Name, &st.Role, &st.State, &st.Summary,
			&st.ActiveDirective, &st.NorthboundSignal, &st.LastAction,
			&st.Confidence, &st.Model, &st.LastLatencyMs, &st.TotalInvocations,
			&st.InnerLoopStep, &st.InnerLoopMax, &st.UpdatedAt,
		); err != nil {
			return nil, err
		}
		st.LayerID = core.LayerID(id)
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *Store) SaveEngineMeta(ctx context.Context, key string, value string) error {
	t0 := time.Now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO engine_meta (key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = excluded.updated_at
	`, key, value, time.Now().UTC())
	if waitMs := time.Since(t0).Milliseconds(); waitMs >= 5 && s.telem != nil {
		_ = s.telem.RecordSpan(ctx, core.TelemetrySpan{
			Operation:  "db_write_wait_ms",
			LayerCode:  "SYS",
			Status:     "ok",
			DurationMs: waitMs,
			Detail:     fmt.Sprintf("key=%s wait_ms=%d", key, waitMs),
		})
	}
	return err
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) LoadEngineMeta(ctx context.Context, key string) (string, error) {
	var val string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM engine_meta WHERE key = ?`, key).Scan(&val)
	return val, err
}
