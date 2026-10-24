package cognitive

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

// Retention caps (plan §5): traces/telemetry 30 days, closed-task workpads 7 days.
const (
	retentionTraceDays   = 30
	retentionWorkpadDays = 7
	retentionInterval    = time.Hour
)

var retentionTables = []string{
	"thought_traces",
	"telemetry_spans",
	"telemetry_timeseries",
	"bus_events",
}

// maybeRunRetentionSweep enforces the disk retention caps at most once per hour.
// Runs on every tick head (before the idle early-return) so an idle agent still reclaims space.
func (e *Engine) maybeRunRetentionSweep(ctx context.Context) {
	db := e.getDB()
	if db == nil {
		return
	}
	if last, err := e.memory.LoadEngineMeta(ctx, "last_retention_sweep"); err == nil && last != "" {
		if t, perr := time.Parse(time.RFC3339, last); perr == nil && time.Since(t) < retentionInterval {
			return
		}
	}

	cutoff := time.Now().UTC().Add(-retentionTraceDays * 24 * time.Hour).Format("2006-01-02 15:04:05")
	for _, table := range retentionTables {
		_, _ = db.ExecContext(ctx, `DELETE FROM `+table+` WHERE created_at < ?`, cutoff)
	}

	e.sweepStaleWorkpads()

	_ = e.memory.SaveEngineMeta(ctx, "last_retention_sweep", time.Now().UTC().Format(time.RFC3339))
}

// sweepStaleWorkpads removes .ace-sandbox/workpads/<task_id> staging dirs untouched
// for 7 days — a workpad is only live while its task runs; merged content survives in /workspace.
func (e *Engine) sweepStaleWorkpads() {
	wsDir, _ := e.resolveSandboxDirs()
	if wsDir == "" {
		return
	}
	wpRoot := filepath.Join(filepath.Dir(wsDir), "workpads")
	entries, err := os.ReadDir(wpRoot)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-retentionWorkpadDays * 24 * time.Hour)
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		info, err := ent.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.RemoveAll(filepath.Join(wpRoot, ent.Name()))
	}
}
