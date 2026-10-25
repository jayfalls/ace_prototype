// baseline.go — §9.0 baseline freeze: a named snapshot of standing_metrics
// stamped with the metric-definition hash. Compare refuses cross-definition
// comparisons so a metric change can never masquerade as a regression or an
// improvement (honest-measurement rule).
package science

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// Baseline is a frozen, named standing_metrics snapshot.
type Baseline struct {
	ID            int64
	Name          string
	DefinitionSHA string
	WindowStart   string
	WindowEnd     string
	Metrics       map[string]float64
}

// FreezeBaseline snapshots the latest value of every metric computed under
// the current definition and stamps the contributing rows with the new
// baseline's id. Re-freezing the same name replaces it — the frozen window
// is explicit in the row either way.
func FreezeBaseline(ctx context.Context, db *sql.DB, name string) (*Baseline, error) {
	if db == nil {
		return nil, fmt.Errorf("no db")
	}
	migrateMetrics(db)
	rows, err := db.QueryContext(ctx, `SELECT metric, value, window_start, window_end FROM standing_metrics
		WHERE definition_sha=? AND id IN (SELECT MAX(id) FROM standing_metrics WHERE definition_sha=? GROUP BY metric)`,
		MetricDefinitionSHA, MetricDefinitionSHA)
	if err != nil {
		return nil, err
	}
	m := map[string]float64{}
	var wStart, wEnd string
	for rows.Next() {
		var k, ws, we string
		var v float64
		if err := rows.Scan(&k, &v, &ws, &we); err != nil {
			rows.Close()
			return nil, err
		}
		m[k] = v
		if wStart == "" || ws < wStart {
			wStart = ws
		}
		if we > wEnd {
			wEnd = we
		}
	}
	rows.Close()
	if len(m) == 0 {
		return nil, fmt.Errorf("no standing metrics under definition %s to freeze", MetricDefinitionSHA)
	}
	mj, _ := json.Marshal(m)
	if _, err := db.ExecContext(ctx, `INSERT INTO baselines (name, definition_sha, window_start, window_end, metrics)
		VALUES (?,?,?,?,?) ON CONFLICT(name) DO UPDATE SET definition_sha=excluded.definition_sha,
		window_start=excluded.window_start, window_end=excluded.window_end, metrics=excluded.metrics`,
		name, MetricDefinitionSHA, wStart, wEnd, string(mj)); err != nil {
		return nil, err
	}
	var id int64
	if err := db.QueryRowContext(ctx, `SELECT id FROM baselines WHERE name=?`, name).Scan(&id); err != nil {
		return nil, err
	}
	// Stamp this era's rows: they belong to the baseline just frozen.
	_, _ = db.ExecContext(ctx, `UPDATE standing_metrics SET baseline_id=? WHERE baseline_id=0 AND definition_sha=?`,
		id, MetricDefinitionSHA)
	return &Baseline{ID: id, Name: name, DefinitionSHA: MetricDefinitionSHA, WindowStart: wStart, WindowEnd: wEnd, Metrics: m}, nil
}

// Delta is one metric's baseline-vs-current pair; absent on either side is a
// nil pointer-free zone — missing keys simply don't appear.
type Delta struct {
	Baseline float64 `json:"baseline"`
	Current  float64 `json:"current"`
	Delta    float64 `json:"delta"`
}

// Compare returns per-metric deltas between a frozen baseline and the latest
// standing metrics. It refuses — hard error — when the current rows carry a
// different definition hash: comparing across definitions is exactly the
// goalpost-moving the honest-measurement rule exists to prevent.
func Compare(ctx context.Context, db *sql.DB, baselineName string) (map[string]Delta, error) {
	var b Baseline
	var mj string
	if err := db.QueryRowContext(ctx, `SELECT id, name, definition_sha, window_start, window_end, metrics FROM baselines WHERE name=?`,
		baselineName).Scan(&b.ID, &b.Name, &b.DefinitionSHA, &b.WindowStart, &b.WindowEnd, &mj); err != nil {
		return nil, fmt.Errorf("baseline %q: %w", baselineName, err)
	}
	if err := json.Unmarshal([]byte(mj), &b.Metrics); err != nil {
		return nil, err
	}
	var liveSHA string
	if err := db.QueryRowContext(ctx, `SELECT definition_sha FROM standing_metrics WHERE definition_sha != '' ORDER BY id DESC LIMIT 1`).Scan(&liveSHA); err != nil {
		return nil, fmt.Errorf("no current metrics to compare: %w", err)
	}
	if liveSHA != b.DefinitionSHA {
		return nil, fmt.Errorf("metric definition changed: baseline=%s current=%s — freeze a new baseline instead of comparing",
			b.DefinitionSHA, liveSHA)
	}
	rows, err := db.QueryContext(ctx, `SELECT metric, value FROM standing_metrics
		WHERE definition_sha=? AND id IN (SELECT MAX(id) FROM standing_metrics WHERE definition_sha=? GROUP BY metric)`,
		liveSHA, liveSHA)
	if err != nil {
		return nil, err
	}
	out := map[string]Delta{}
	for rows.Next() {
		var k string
		var v float64
		if err := rows.Scan(&k, &v); err != nil {
			rows.Close()
			return nil, err
		}
		if bv, ok := b.Metrics[k]; ok {
			out[k] = Delta{Baseline: bv, Current: v, Delta: v - bv}
		}
	}
	rows.Close()
	return out, nil
}
