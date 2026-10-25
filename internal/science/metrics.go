// metrics.go — §8.7 standing metrics: computed every cycle from
// thought_traces + mission_dag_nodes and appended to standing_metrics so
// each run carries turns/task, cost/success, latency, cache-hit rate,
// per-layer call share, task success and L3 estimate error.
package science

import (
	"context"
	"database/sql"
	"math"
	"sort"
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
	startStr := start.Format("2006-01-02 15:04:05")
	endStr := end.Format("2006-01-02 15:04:05")
	m := map[string]float64{}

	// L1-gate logging (first loop_kind='gate' trace) changed the trace
	// population, which resets the baseline: the honest window is the whole
	// era since that change, so every metric in a row shares it. A task
	// takes minutes — bounding the per-task distribution by the rolling
	// tick would leave almost no fully-observed task. substr+replace
	// normalises the two stored timestamp formats — datetime() returns
	// NULL on the '+0000 UTC' suffix form.
	var gateStart sql.NullString
	_ = db.QueryRowContext(ctx, `SELECT MIN(replace(substr(created_at,1,19),'T',' ')) FROM thought_traces WHERE loop_kind='gate'`).Scan(&gateStart)
	if gateStart.Valid {
		startStr = gateStart.String
	}

	var traces, tasksDone, tasksAll int
	var tokIn, tokOut, tokCached, latSum int64
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0), COALESCE(SUM(cached_tokens),0), COALESCE(SUM(latency_ms),0) FROM thought_traces WHERE replace(substr(created_at,1,19),'T',' ') >= ?`, startStr).Scan(&traces, &tokIn, &tokOut, &tokCached, &latSum)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mission_dag_nodes WHERE id LIKE 'T_%' AND replace(substr(updated_at,1,19),'T',' ') >= ?`, startStr).Scan(&tasksAll)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mission_dag_nodes WHERE id LIKE 'T_%' AND state='COMPLETED' AND replace(substr(updated_at,1,19),'T',' ') >= ?`, startStr).Scan(&tasksDone)

	if tasksAll > 0 {
		m["task_success"] = float64(tasksDone) / float64(tasksAll)
	}
	if tasksDone > 0 {
		m["turns_per_success"] = float64(traces) / float64(tasksDone)
		m["cost_per_success"] = float64(tokIn+tokOut) / float64(tasksDone)
	}
	// v0.9 primary metric: L6 turns per completed task, a distribution.
	// Traces carry no task_id; the L4(T_*) provenance tag in trigger_summary
	// is the attribution. A task whose L6 traces begin before the window is
	// partially observed and dropped — never counted with truncated turns.
	tidExpr := `substr(trigger_summary, instr(trigger_summary,'L4(')+3, instr(trigger_summary,')]')-instr(trigger_summary,'L4(')-3)`
	prow, err := db.QueryContext(ctx, `SELECT `+tidExpr+`, COUNT(*) FROM thought_traces WHERE layer_code='L6' AND replace(substr(created_at,1,19),'T',' ') >= ? AND trigger_summary LIKE '%L4(T_%' GROUP BY 1`, startStr)
	var counts map[string]int
	if err == nil {
		counts = map[string]int{}
		for prow.Next() {
			var tid string
			var cnt int
			_ = prow.Scan(&tid, &cnt)
			counts[tid] = cnt
		}
		prow.Close()
	}
	var l6 []float64
	for tid, cnt := range counts {
		var state string
		if db.QueryRowContext(ctx, `SELECT state FROM mission_dag_nodes WHERE id=?`, tid).Scan(&state) != nil || state != "COMPLETED" {
			continue
		}
		var pre int
		_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM thought_traces WHERE layer_code='L6' AND replace(substr(created_at,1,19),'T',' ') < ? AND trigger_summary LIKE '%L4('||?||')%'`, startStr, tid).Scan(&pre)
		if pre == 0 {
			l6 = append(l6, float64(cnt))
		}
	}
	if len(l6) > 0 {
		sort.Float64s(l6)
		n := float64(len(l6))
		m["l6_turns_per_completed_task_median"] = median(l6)
		m["l6_turns_per_completed_task_p90"] = l6[int(math.Ceil(0.9*n))-1]
		m["l6_turns_per_completed_task_n"] = n
		if lo, hi, ok := medianOrderStatCI(l6, 0.05); ok {
			m["l6_turns_per_completed_task_ci95_lo"] = lo
			m["l6_turns_per_completed_task_ci95_hi"] = hi
		}
	}
	if traces > 0 {
		m["latency_ms"] = float64(latSum) / float64(traces)
	}
	if tokIn > 0 {
		m["cache_hit_rate"] = float64(tokCached) / float64(tokIn)
	}
	var confErr sql.NullFloat64
	_ = db.QueryRowContext(ctx, `SELECT AVG(ABS(confidence - CASE WHEN action_result LIKE 'success%' OR action_result='ok' OR action_result='completed' THEN 1.0 ELSE 0.0 END)) FROM thought_traces WHERE replace(substr(created_at,1,19),'T',' ') >= ? AND confidence IS NOT NULL AND confidence != ''`, startStr).Scan(&confErr)
	if confErr.Valid {
		m["l3_estimate_error"] = confErr.Float64
	}
	rows, err := db.QueryContext(ctx, `SELECT layer_code, COUNT(*) FROM thought_traces WHERE replace(substr(created_at,1,19),'T',' ') >= ? GROUP BY layer_code`, startStr)
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
			startStr, endStr, k, v)
	}
}

func median(xs []float64) float64 {
	n := len(xs)
	if n%2 == 1 {
		return xs[n/2]
	}
	return (xs[n/2-1] + xs[n/2]) / 2
}

// medianOrderStatCI returns the distribution-free CI for the median: the
// widest symmetric order-statistic pair whose binomial coverage still
// reaches 1-alpha. ok=false when n is too small to reach it — a looser
// interval must not ship under a "95%" name.
func medianOrderStatCI(xs []float64, alpha float64) (lo, hi float64, ok bool) {
	n := len(xs)
	for r := 1; r <= (n+1)/2; r++ {
		if 2*binomCDF(r-1, n, 0.5) > alpha {
			break
		}
		lo, hi, ok = xs[r-1], xs[n-r], true
	}
	return
}
