// prereg.go — pre-registration ledger (spec 8.6): hypothesis, primary
// statistic, alpha, MDE, dataset sha are frozen before the first run.
// Changing any frozen field registers a NEW attempt on the same question —
// goalposts are never moved, only re-attempted, and alpha is Holm-corrected
// across attempts. Every attempt is reported.
package science

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"
)

func sha256Bytes(b []byte) [32]byte {
	return sha256.Sum256(b)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// Question is a frozen pre-registered hypothesis.
type Question struct {
	ID          string // stable question key
	Hypothesis  string
	PrimaryStat string  // "accuracy" | "delta_score" | custom name
	Alpha       float64 // base alpha (default 0.05)
	MDE         float64 // minimum detectable effect (default 0.10)
	DatasetSHA  string
	CreatedAt   string
}

// Attempt is one run against a question. SpecHash captures the frozen fields;
// a run whose frozen fields differ from the registered question records a new
// attempt, never an in-place edit.
type Attempt struct {
	QuestionID string
	Attempt    int
	SpecHash   string
	ResultRef  string
	Verdict    string // EXTERNAL | PROVISIONAL | NO_LARGE_EFFECT | REJECTED
	PValue     float64
	CreatedAt  string
}

// RegisterQuestion inserts a question or returns the existing registration.
func RegisterQuestion(ctx context.Context, db *sql.DB, q Question) error {
	if q.Alpha == 0 {
		q.Alpha = 0.05
	}
	if q.MDE == 0 {
		q.MDE = 0.10
	}
	_, err := db.ExecContext(ctx, `INSERT INTO prereg_questions
		(id, hypothesis, primary_stat, alpha, mde, dataset_sha, created_at)
		VALUES (?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`,
		q.ID, q.Hypothesis, q.PrimaryStat, q.Alpha, q.MDE, q.DatasetSHA,
		time.Now().UTC().Format(time.RFC3339))
	return err
}

// FrozenSpecHash identifies the frozen fields — changed fields => new attempt.
func (q Question) FrozenSpecHash() string {
	sum := fmt.Sprintf("%s|%s|%.4f|%.4f|%s", q.Hypothesis, q.PrimaryStat, q.Alpha, q.MDE, q.DatasetSHA)
	h := sha256Bytes([]byte(sum))
	return fmt.Sprintf("%x", h[:8])
}

// RecordAttempt appends an attempt row. attempts are 1-indexed per question.
func RecordAttempt(ctx context.Context, db *sql.DB, a Attempt) (int, error) {
	var n int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM prereg_attempts WHERE question_id=?`, a.QuestionID).Scan(&n)
	n++
	_, err := db.ExecContext(ctx, `INSERT INTO prereg_attempts
		(question_id, attempt, spec_hash, result_ref, verdict, p_value, created_at)
		VALUES (?,?,?,?,?,?,?)`,
		a.QuestionID, n, a.SpecHash, a.ResultRef, a.Verdict, a.PValue,
		time.Now().UTC().Format(time.RFC3339))
	return n, err
}

// HolmAlpha returns the Holm-corrected alpha for the k-th smallest p-value
// (1-indexed rank) out of m attempts: alpha/(m+1-rank).
func HolmAlpha(baseAlpha float64, m, rank int) float64 {
	if m < 1 {
		m = 1
	}
	if rank < 1 {
		rank = 1
	}
	return baseAlpha / float64(m+1-rank)
}

// HolmVerdict: given per-attempt p-values (unordered), apply Holm–Bonferroni —
// sort ascending, find first i where p_i >= alpha/(m+1-i); hypotheses below
// that index are rejected at the family level. Returns rejected attempt idx.
func HolmVerdict(pvals []float64, alpha float64) []bool {
	type pv struct {
		p   float64
		idx int
	}
	var s []pv
	for i, p := range pvals {
		s = append(s, pv{p, i})
	}
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j].p < s[i].p {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
	out := make([]bool, len(pvals))
	for rank, e := range s {
		if e.p < HolmAlpha(alpha, len(s), rank+1) {
			out[e.idx] = true
		} else {
			break // first failure stops the step-down
		}
	}
	return out
}

func migratePrereg(db *sql.DB) {
	_, _ = db.Exec(`
		CREATE TABLE IF NOT EXISTS prereg_questions (
			id TEXT PRIMARY KEY,
			hypothesis TEXT NOT NULL,
			primary_stat TEXT NOT NULL,
			alpha REAL NOT NULL DEFAULT 0.05,
			mde REAL NOT NULL DEFAULT 0.10,
			dataset_sha TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL
		);
		CREATE TABLE IF NOT EXISTS prereg_attempts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			question_id TEXT NOT NULL,
			attempt INTEGER NOT NULL,
			spec_hash TEXT NOT NULL,
			result_ref TEXT NOT NULL DEFAULT '',
			verdict TEXT NOT NULL,
			p_value REAL,
			created_at DATETIME NOT NULL
		);
		CREATE TABLE IF NOT EXISTS science_verdicts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			question_id TEXT NOT NULL,
			result_ref TEXT NOT NULL,
			tier TEXT NOT NULL,
			stat REAL,
			p_value REAL,
			detail TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL
		);
	`)
}
