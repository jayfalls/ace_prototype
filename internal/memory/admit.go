package memory

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"ace/internal/core"
)

// Spec 9.6 admission rules, enforced at the storage boundary.
const (
	// SemanticCapBytes caps a semantic fact's content (spec: 2,200 B).
	SemanticCapBytes = 2200
	// tsLayout matches how updated_at is stored by the writer.
	tsLayout = time.RFC3339Nano
	// retentionHalfLife is the retention decay horizon (spec: 30 d).
	retentionHalfLife = 30 * 24 * time.Hour
)

// EpisodicCapEntries is the total episodic entry limit (spec: 10,000).
// A var so tests can shrink the cap without inserting 10k rows.
var EpisodicCapEntries = 10000

// checkWriteRight enforces L1-only authority in storage: a layer writes
// only to its own layer_code; L1 (and empty = legacy/operator path)
// writes anywhere.
func checkWriteRight(callerLayer, targetLayer string) error {
	if callerLayer == "" || callerLayer == "L1" {
		return nil
	}
	if callerLayer == targetLayer || targetLayer == "GLOBAL" {
		return nil
	}
	return fmt.Errorf("write denied: %s may not write %s memory (L1-only authority)", callerLayer, targetLayer)
}

// validateType applies the semantic content cap; the entry type defaults
// to episodic for callers that predate the type field.
func validateType(entry core.MemoryEntry) error {
	if entry.Type == "semantic" && len(entry.Content) > SemanticCapBytes {
		return fmt.Errorf("semantic fact is %d bytes, exceeds the %d-byte cap — consolidate, don't append", len(entry.Content), SemanticCapBytes)
	}
	return nil
}

// RetentionScore = importance × 0.5^(age / 30d) — the spec's decay curve.
func RetentionScore(importance float64, age time.Duration) float64 {
	if importance <= 0 {
		importance = 1
	}
	return importance * math.Pow(0.5, float64(age)/float64(retentionHalfLife))
}

// enforceEpisodicCap evicts lowest-retention episodic rows over the cap.
// It runs inside the writer's transaction so cap enforcement is atomic.
func enforceEpisodicCap(ctx context.Context, tx *sql.Tx) error {
	var n int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM memories WHERE COALESCE(type,'episodic') = 'episodic'`).Scan(&n); err != nil {
		return err
	}
	if n <= EpisodicCapEntries {
		return nil
	}
	rows, err := tx.QueryContext(ctx,
		`SELECT id, importance, created_at FROM memories WHERE COALESCE(type,'episodic') = 'episodic'`)
	if err != nil {
		return err
	}
	type cand struct {
		id    string
		score float64
	}
	var cands []cand
	now := time.Now().UTC()
	for rows.Next() {
		var id, created string
		var imp float64
		if rows.Scan(&id, &imp, &created) == nil {
			age := now.Sub(parseTS(created))
			cands = append(cands, cand{id, RetentionScore(imp, age)})
		}
	}
	rows.Close()
	for i := 0; i < len(cands)-1; i++ {
		for j := i + 1; j < len(cands); j++ {
			if cands[j].score < cands[i].score {
				cands[i], cands[j] = cands[j], cands[i]
			}
		}
	}
	for i := 0; i < n-EpisodicCapEntries && i < len(cands); i++ {
		if _, err := tx.ExecContext(ctx, `DELETE FROM memories WHERE id = ?`, cands[i].id); err != nil {
			return err
		}
		_, _ = tx.ExecContext(ctx, `DELETE FROM memories_fts WHERE id = ?`, cands[i].id)
	}
	return nil
}

func parseTS(s string) time.Time {
	if t, err := time.Parse(tsLayout, s); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02 15:04:05.999999999-07:00", s); err == nil {
		return t
	}
	return time.Now().UTC()
}

func nullTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t.Format(tsLayout)
}

// Consolidate merges the content of src into dst in one transaction —
// the spec's "consolidate, don't append" for memory consolidation.
// The survivor keeps the newest content and the max importance.
func (s *Store) Consolidate(ctx context.Context, srcID, dstID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var srcContent string
	var srcImp float64
	if err := tx.QueryRowContext(ctx, `SELECT content, importance FROM memories WHERE id = ?`, srcID).
		Scan(&srcContent, &srcImp); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE memories SET content = content || char(10) || ?, importance = MAX(importance, ?), updated_at = ? WHERE id = ?`,
		srcContent, srcImp, time.Now().UTC().Format(tsLayout), dstID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM memories WHERE id = ?`, srcID); err != nil {
		return err
	}
	_, _ = tx.ExecContext(ctx, `DELETE FROM memories_fts WHERE id = ?`, srcID)
	return tx.Commit()
}

// MemoryHealth is the watchdog-visible view of the memory subsystem:
// counts by type, whether caps are pressured, and retention stats.
type MemoryHealth struct {
	Episodic           int  `json:"episodic"`
	Semantic           int  `json:"semantic"`
	Procedural         int  `json:"procedural"`
	EpisodicOverCap    bool `json:"episodic_over_cap"`
	UnverifiedSemantic int  `json:"unverified_semantic"`
}

// Health snapshots the store for the watchdog sense.
func (s *Store) Health(ctx context.Context) MemoryHealth {
	var h MemoryHealth
	_ = s.db.QueryRowContext(ctx,
		`SELECT
			COALESCE(SUM(CASE WHEN COALESCE(type,'episodic')='episodic' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN type='semantic' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN type='procedural' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN type='semantic' AND verified_at IS NULL THEN 1 ELSE 0 END),0)
		 FROM memories`).Scan(&h.Episodic, &h.Semantic, &h.Procedural, &h.UnverifiedSemantic)
	h.EpisodicOverCap = h.Episodic > EpisodicCapEntries
	return h
}
