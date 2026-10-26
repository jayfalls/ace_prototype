// metrics.go — §8.7 standing metrics: computed every cycle from
// thought_traces + mission_dag_nodes and appended to standing_metrics so
// each run carries turns/task, cost/success, latency, cache-hit rate,
// per-layer call share, task success and L3 estimate error.
package science

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"math"
	"sort"
	"strings"
	"time"
)

// metricDefinitionLiteral names every metric and the window rule exactly as
// computed below. Any change to a definition MUST change this literal — the
// hash stamps every standing_metrics row and freezes into baselines, so a
// definition change resets the baseline instead of silently comparing
// unlike quantities (spec 9.0 honest-measurement).
const metricDefinitionLiteral = "v0.9.3|era:latest-baseline-end-else-first-gate-trace|" +
	"l6_turns_per_completed_task{median,p90,n,ci95_orderstat,attr:l4-close-paren}|" +
	"turns_per_success|cost_per_success|task_success{excl:unverified}|tasks_unverified|" +
	"latency_ms|cache_hit_rate|l3_estimate_error|" +
	"layer_share_{calls,tokens,l1_gate_calls}"

// MetricDefinitionSHA is the identity of the metric definitions above.
var MetricDefinitionSHA = func() string {
	sum := sha256.Sum256([]byte(metricDefinitionLiteral))
	return hex.EncodeToString(sum[:])[:16]
}()

func migrateMetrics(db *sql.DB) {
	_, _ = db.Exec(`
		CREATE TABLE IF NOT EXISTS standing_metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			window_start TEXT NOT NULL,
			window_end TEXT NOT NULL,
			metric TEXT NOT NULL,
			value REAL NOT NULL,
			definition_sha TEXT NOT NULL DEFAULT '',
			baseline_id INTEGER NOT NULL DEFAULT 0,
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
		CREATE TABLE IF NOT EXISTS baselines (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			definition_sha TEXT NOT NULL,
			window_start TEXT NOT NULL,
			window_end TEXT NOT NULL,
			metrics TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`)
	_, _ = db.Exec(`ALTER TABLE standing_metrics ADD COLUMN definition_sha TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE standing_metrics ADD COLUMN baseline_id INTEGER NOT NULL DEFAULT 0`)
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
	// v0.9.1 era anchor: the current window begins at the newest frozen
	// baseline's window_end — i.e. the boundary between the old release's
	// measured era and the new binary's deploy. Until a baseline exists,
	// fall back to the first gate trace (the original honest anchor). This
	// makes "current" rows count only post-deploy work; comparing a new-era
	// row to an old-era baseline errors on definition_sha mismatch instead
	// of reporting a false delta (owner rule: no data, not "unchanged").
	var eraStart sql.NullString
	_ = db.QueryRowContext(ctx, `SELECT window_end FROM baselines ORDER BY id DESC LIMIT 1`).Scan(&eraStart)
	if eraStart.Valid && eraStart.String != "" {
		startStr = eraStart.String
	} else {
		var gateStart sql.NullString
		_ = db.QueryRowContext(ctx, `SELECT MIN(replace(substr(created_at,1,19),'T',' ')) FROM thought_traces WHERE loop_kind='gate'`).Scan(&gateStart)
		if gateStart.Valid {
			startStr = gateStart.String
		}
	}

	var traces, tasksDone, tasksAll int
	var tokIn, tokOut, tokCached, latSum int64
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0), COALESCE(SUM(cached_tokens),0), COALESCE(SUM(latency_ms),0) FROM thought_traces WHERE replace(substr(created_at,1,19),'T',' ') >= ?`, startStr).Scan(&traces, &tokIn, &tokOut, &tokCached, &latSum)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mission_dag_nodes WHERE id LIKE 'T_%' AND state != 'UNVERIFIED' AND replace(substr(updated_at,1,19),'T',' ') >= ?`, startStr).Scan(&tasksAll)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mission_dag_nodes WHERE id LIKE 'T_%' AND state='COMPLETED' AND replace(substr(updated_at,1,19),'T',' ') >= ?`, startStr).Scan(&tasksDone)
	var tasksUnverified int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mission_dag_nodes WHERE id LIKE 'T_%' AND state='UNVERIFIED' AND replace(substr(updated_at,1,19),'T',' ') >= ?`, startStr).Scan(&tasksUnverified)
	m["tasks_unverified"] = float64(tasksUnverified)

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
	// Tag shape: "[PROVENANCE LOCKED L1(..)->L2(..)->L3(..)->L4(T_ID)]" —
	// extract the id between 'L4(' and its closing ')' in Go; the first
	// ')' in the row belongs to the L1 tag, so SQL-only parsing is wrong.
	prow, err := db.QueryContext(ctx, `SELECT trigger_summary FROM thought_traces WHERE layer_code='L6' AND replace(substr(created_at,1,19),'T',' ') >= ? AND trigger_summary LIKE '%L4(T_%'`, startStr)
	counts := map[string]int{}
	if err == nil {
		for prow.Next() {
			var ts string
			_ = prow.Scan(&ts)
			i := strings.Index(ts, "L4(")
			if i < 0 {
				continue
			}
			rest := ts[i+3:]
			if j := strings.IndexByte(rest, ')'); j > 0 {
				counts[rest[:j]]++
			}
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
	rows, err := db.QueryContext(ctx, `SELECT layer_code, COUNT(*), COALESCE(SUM(prompt_tokens+completion_tokens),0) FROM thought_traces WHERE replace(substr(created_at,1,19),'T',' ') >= ? GROUP BY layer_code`, startStr)
	if err == nil {
		var total, tokTotal int64
		share := map[string]int64{}
		tokShare := map[string]int64{}
		for rows.Next() {
			var l string
			var c, t int64
			_ = rows.Scan(&l, &c, &t)
			share[l] = c
			tokShare[l] = t
			total += c
			tokTotal += t
		}
		rows.Close()
		if total > 0 {
			for l, c := range share {
				m["layer_share_"+l] = float64(c) / float64(total)
			}
		}
		if tokTotal > 0 {
			for l, t := range tokShare {
				m["layer_token_share_"+l] = float64(t) / float64(tokTotal)
			}
		}
	}
	// L1 gate calls shown separately from L1 layer calls (spec 9.0 pyramid).
	var gateCalls int64
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM thought_traces WHERE loop_kind='gate' AND replace(substr(created_at,1,19),'T',' ') >= ?`, startStr).Scan(&gateCalls)
	if traces > 0 {
		m["layer_share_l1_gate"] = float64(gateCalls) / float64(traces)
	}
	for k, v := range m {
		_, _ = db.ExecContext(ctx, `INSERT INTO standing_metrics (window_start, window_end, metric, value, definition_sha) VALUES (?,?,?,?,?)`,
			startStr, endStr, k, v, MetricDefinitionSHA)
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
