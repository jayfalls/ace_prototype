// metrics.go — §8.7 standing metrics: computed every cycle from
// thought_traces + mission_dag_nodes and appended to standing_metrics so
// each run carries turns/task, cost/success, latency, cache-hit rate,
// per-layer call share, task success and L3 estimate error.
package science

import (
	"context"
	"database/sql"
	"time"
)

func migrateMetrics(db *sql.DB) {
	_, _ = db.Exec(`
		CREATE TABLE IF NOT EXISTS standing_metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			window_start TEXT NOT NULL,
			window_end TEXT NOT NULL,
			metric TEXT NOT NULL,
			value REAL NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_standing_metrics_metric ON standing_metrics(metric, created_at);
		CREATE TABLE IF NOT EXISTS concept_registry (
			name TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			owner TEXT NOT NULL,
			purpose TEXT NOT NULL,
			supersedes TEXT NOT NULL DEFAULT '',
			superseded_by TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`)
}

// RecordStandingMetrics computes the §8.7 standing metrics over the last
// windowSecs seconds and appends them to standing_metrics. Metrics the data
// can't support yet are skipped, never faked.
func RecordStandingMetrics(ctx context.Context, db *sql.DB, windowSecs int) {
	if db == nil || windowSecs <= 0 {
		return
	}
	end := time.Now().UTC()
	start := end.Add(-time.Duration(windowSecs) * time.Second)
	m := map[string]float64{}

	var traces, tasksDone, tasksAll int
	var tokIn, tokOut, tokCached, latSum int64
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0), COALESCE(SUM(cached_tokens),0), COALESCE(SUM(latency_ms),0) FROM thought_traces WHERE created_at >= ?`, start).Scan(&traces, &tokIn, &tokOut, &tokCached, &latSum)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mission_dag_nodes WHERE id LIKE 'T_%' AND updated_at >= ?`, start).Scan(&tasksAll)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mission_dag_nodes WHERE id LIKE 'T_%' AND state='COMPLETED' AND updated_at >= ?`, start).Scan(&tasksDone)

	if tasksAll > 0 {
		m["task_success"] = float64(tasksDone) / float64(tasksAll)
	}
	if tasksDone > 0 {
		// v0.9 primary metric: turns per SUCCESSFUL task — completed
		// tasks only, never pending/stale ones in the denominator.
		m["turns_per_successful_task"] = float64(traces) / float64(tasksDone)
		m["cost_per_success"] = float64(tokIn+tokOut) / float64(tasksDone)
	}
	if traces > 0 {
		m["latency_ms"] = float64(latSum) / float64(traces)
	}
	if tokIn > 0 {
		m["cache_hit_rate"] = float64(tokCached) / float64(tokIn)
	}
	var confErr sql.NullFloat64
	_ = db.QueryRowContext(ctx, `SELECT AVG(ABS(confidence - CASE WHEN action_result LIKE 'success%' OR action_result='ok' OR action_result='completed' THEN 1.0 ELSE 0.0 END)) FROM thought_traces WHERE created_at >= ? AND confidence IS NOT NULL AND confidence != ''`, start).Scan(&confErr)
	if confErr.Valid {
		m["l3_estimate_error"] = confErr.Float64
	}
	rows, err := db.QueryContext(ctx, `SELECT layer_code, COUNT(*) FROM thought_traces WHERE created_at >= ? GROUP BY layer_code`, start)
	if err == nil {
		var total int64
		share := map[string]int64{}
		for rows.Next() {
			var l string
			var c int64
			_ = rows.Scan(&l, &c)
			share[l] = c
			total += c
		}
		rows.Close()
		if total > 0 {
			for l, c := range share {
				m["layer_share_"+l] = float64(c) / float64(total)
			}
		}
	}
	for k, v := range m {
		_, _ = db.ExecContext(ctx, `INSERT INTO standing_metrics (window_start, window_end, metric, value) VALUES (?,?,?,?)`,
			start.Format(time.RFC3339), end.Format(time.RFC3339), k, v)
	}
}
