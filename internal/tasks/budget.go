package tasks

import (
	"context"
	"database/sql"
	"strconv"
	"time"
)

// CadenceScale is the spec-9.4 budget governor's output: the factor by
// which NON-L6 cadence slows when projected burn exceeds the remaining
// quota window. It reads usage_events (9.2) inside the trailing window and
// returns remaining ÷ projected, clamped to [0.1, 1.0] — 1.0 means no
// slowdown.
//
// windowSeconds: the quota window (e.g. a 5h rolling window).
// quotaUnits:    the window's budget in metered units (tokens).
func CadenceScale(ctx context.Context, db *sql.DB, windowSeconds int, quotaUnits float64) float64 {
	if db == nil || quotaUnits <= 0 || windowSeconds <= 0 {
		return 1.0
	}
	bound := "-" + strconv.Itoa(windowSeconds) + " seconds"
	var used float64
	_ = db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(prompt_tokens + completion_tokens), 0) FROM usage_events
		 WHERE created_at >= datetime('now', ?)`, bound).Scan(&used)
	if used <= 0 {
		return 1.0
	}
	remaining := quotaUnits - used
	if remaining <= 0 {
		return 0.1
	}
	// Project the burn rate observed so far over the rest of the window.
	var oldest string
	_ = db.QueryRowContext(ctx,
		`SELECT MIN(created_at) FROM usage_events WHERE created_at >= datetime('now', ?)`, bound).Scan(&oldest)
	elapsed := 1.0
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, oldest); err == nil {
			if d := time.Since(t).Seconds(); d > 1 {
				elapsed = d
			}
			break
		}
	}
	projected := (used / elapsed) * float64(windowSeconds)
	scale := remaining / projected
	if scale > 1 {
		return 1.0
	}
	if scale < 0.1 {
		return 0.1
	}
	return scale
}
