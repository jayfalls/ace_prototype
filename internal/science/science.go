// Package science implements the measurable parts of the Heuristic
// Imperatives in code (spec 8.3): strategy ranking scores, duplicate
// rejection, diminishing-returns tracking, and the L3 capability_limits
// table read by L1 and L4. Scores are computed here, never asked of a model.
package science

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// Imperative weights come from the compiled constitution's three imperatives:
// reduce suffering, increase prosperity, increase understanding. The keyword
// sets below are the operational vocabulary those imperatives take inside a
// research mission; scoring is deterministic and auditable.
var imperativeKeywords = map[string][]string{
	"suffering":     {"prevent", "protect", "safe", "harm", "risk", "integrity", "honest", "verify", "falsif", "detect", "guard", "veto", "audit"},
	"prosperity":    {"build", "implement", "improve", "capab", "tool", "automat", "perform", "efficien", "deliver", "ship", "measure", "benchmark"},
	"understanding": {"research", "study", "learn", "analy", "measur", "experiment", "hypothes", "data", "evidence", "model", "observ", "discover"},
}

// ImperativeScore returns a 0..1 score: fraction of the three imperatives a
// directive meaningfully addresses, weighted by mention density. Used to rank
// L2 strategies and weight L4 priorities (spec 8.3).
func ImperativeScore(text string) float64 {
	lower := strings.ToLower(text)
	var covered float64
	for _, kws := range imperativeKeywords {
		var hits int
		for _, kw := range kws {
			if strings.Contains(lower, kw) {
				hits++
			}
		}
		if hits > 0 {
			// density saturates: one mention earns 0.6 of the pillar, more earns up to 1
			covered += 0.6 + 0.4*minf(float64(hits)/3.0, 1.0)
		}
	}
	return covered / float64(len(imperativeKeywords))
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// ShingleJaccard computes word 3-shingle Jaccard similarity — used to reject
// duplicate strategy branches (spec 8.3: >= 0.8 vs any open/finished strategy).
func ShingleJaccard(a, b string) float64 {
	sa, sb := shingles(a), shingles(b)
	if len(sa) == 0 || len(sb) == 0 {
		return 0
	}
	var inter int
	for s := range sa {
		if sb[s] {
			inter++
		}
	}
	return float64(inter) / float64(len(sa)+len(sb)-inter)
}

func shingles(s string) map[string]bool {
	words := strings.Fields(strings.ToLower(s))
	out := make(map[string]bool, len(words))
	for i := 0; i+3 <= len(words); i++ {
		out[strings.Join(words[i:i+3], " ")] = true
	}
	if len(words) < 3 && len(words) > 0 {
		out[strings.Join(words, " ")] = true
	}
	return out
}

// Migrate creates the science tables (capability_limits, branch_progress).
func Migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS capability_limits (
			capability TEXT PRIMARY KEY,
			harm_risk REAL NOT NULL DEFAULT 0,
			error_rate REAL NOT NULL DEFAULT 0,
			fabrication_risk REAL NOT NULL DEFAULT 0,
			notes TEXT NOT NULL DEFAULT '',
			updated_at DATETIME NOT NULL
		);
		CREATE TABLE IF NOT EXISTS branch_progress (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			branch_id TEXT NOT NULL,
			task_id TEXT NOT NULL,
			moved INTEGER NOT NULL,
			created_at DATETIME NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_branch_progress_branch ON branch_progress(branch_id, id);
		CREATE TABLE IF NOT EXISTS access_requests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			fingerprint TEXT NOT NULL UNIQUE,
			capability TEXT NOT NULL,
			action_desc TEXT NOT NULL,
			mission_ref TEXT NOT NULL,
			scope TEXT NOT NULL DEFAULT 'one-time',
			status TEXT NOT NULL DEFAULT 'pending',
			owner_msg_id INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL
		);
		CREATE TABLE IF NOT EXISTS directives (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			msg_id INTEGER NOT NULL,
			text TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT 'telegram',
			status TEXT NOT NULL DEFAULT 'OPEN',
			created_at DATETIME NOT NULL,
			addressed_at DATETIME
		);
		CREATE TABLE IF NOT EXISTS owner_message_times (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts DATETIME NOT NULL
		);
		CREATE TABLE IF NOT EXISTS cycle_reports (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			cycle_id TEXT NOT NULL,
			done TEXT NOT NULL DEFAULT '',
			measured TEXT NOT NULL DEFAULT '',
			goals TEXT NOT NULL DEFAULT '',
			uncertainties TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL
		);
	`)
	if err != nil {
		return err
	}
	migratePrereg(db)
	return nil
}

// CapabilityLimit is one row of the L3 harm/error/fabrication table.
type CapabilityLimit struct {
	Capability      string
	HarmRisk        float64
	ErrorRate       float64
	FabricationRisk float64
	Notes           string
}

// UpsertCapabilityLimit records/updates an L3-measured capability limit.
func UpsertCapabilityLimit(ctx context.Context, db *sql.DB, c CapabilityLimit) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO capability_limits (capability, harm_risk, error_rate, fabrication_risk, notes, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(capability) DO UPDATE SET
			harm_risk = excluded.harm_risk, error_rate = excluded.error_rate,
			fabrication_risk = excluded.fabrication_risk, notes = excluded.notes,
			updated_at = excluded.updated_at
	`, c.Capability, c.HarmRisk, c.ErrorRate, c.FabricationRisk, c.Notes, time.Now().UTC())
	return err
}

// CapabilityLimits returns all rows — L1/L4 read this before planning.
func CapabilityLimits(ctx context.Context, db *sql.DB) ([]CapabilityLimit, error) {
	rows, err := db.QueryContext(ctx, `SELECT capability, harm_risk, error_rate, fabrication_risk, notes FROM capability_limits ORDER BY capability`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CapabilityLimit
	for rows.Next() {
		var c CapabilityLimit
		if err := rows.Scan(&c.Capability, &c.HarmRisk, &c.ErrorRate, &c.FabricationRisk, &c.Notes); err != nil {
			return out, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// RecordChildResult logs whether a completed task moved its branch's primary
// statistic by the MDE (moved=true) or not — feeds diminishing-returns.
func RecordChildResult(ctx context.Context, db *sql.DB, branchID, taskID string, moved bool) {
	m := 0
	if moved {
		m = 1
	}
	_, _ = db.ExecContext(ctx, `INSERT INTO branch_progress (branch_id, task_id, moved, created_at) VALUES (?,?,?,?)`,
		branchID, taskID, m, time.Now().UTC())
}

// BranchStalled returns true when the branch's last 3 child results all
// failed to move the primary statistic — the branch stops spawning children.
func BranchStalled(ctx context.Context, db *sql.DB, branchID string) bool {
	rows, err := db.QueryContext(ctx,
		`SELECT moved FROM branch_progress WHERE branch_id = ? ORDER BY id DESC LIMIT 3`, branchID)
	if err != nil {
		return false
	}
	defer rows.Close()
	var n, zeros int
	for rows.Next() {
		var m int
		if rows.Scan(&m) == nil {
			n++
			if m == 0 {
				zeros++
			}
		}
	}
	return n >= 3 && zeros == 3
}

// TaskFrustration is the running failure ratio for a DAG task — spec 8.3 /
// TRACE B13: high frustration drives the task-switch signal (damping).
// attempts>0 with no completion counts as failure evidence.
func TaskFrustration(attempts int, completed bool) float64 {
	if completed {
		return 0
	}
	// Beta(1,1) prior: frustration = fails/(trials); one success clears it.
	return float64(attempts) / float64(attempts+1)
}
