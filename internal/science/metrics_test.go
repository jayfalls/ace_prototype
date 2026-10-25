package science

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func openMetricsDB(t *testing.T) *sql.DB {
	t.Helper()
	db, _ := sql.Open("sqlite", ":memory:?cache=shared")
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	for _, ddl := range []string{
		`CREATE TABLE thought_traces (layer_code TEXT, loop_kind TEXT, trigger_summary TEXT, latency_ms INTEGER, prompt_tokens INTEGER, cached_tokens INTEGER, completion_tokens INTEGER, confidence REAL, action_result TEXT, created_at DATETIME)`,
		`CREATE TABLE mission_dag_nodes (id TEXT, state TEXT, updated_at DATETIME)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}
	migrateMetrics(db)
	return db
}

func insertTrace(t *testing.T, db *sql.DB, layer, loop, summary, ts string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO thought_traces (layer_code,loop_kind,trigger_summary,latency_ms,prompt_tokens,cached_tokens,completion_tokens,confidence,action_result,created_at) VALUES (?,?,?,100,100,10,50,0.9,'success',?)`,
		layer, loop, summary, ts)
	if err != nil {
		t.Fatal(err)
	}
}

func seedMetricFixture(t *testing.T, db *sql.DB, now string) {
	insertTrace(t, db, "L1", "gate", "L1 gate sandbox_sh", now) // baseline-reset boundary
	for i := 0; i < 4; i++ {
		insertTrace(t, db, "L6", "task", "[PROVENANCE LOCKED L1(M)->L2(S)->L3(C)->L4(T_A)] spec", now)
	}
	for i := 0; i < 2; i++ {
		insertTrace(t, db, "L6", "task", "[PROVENANCE LOCKED L1(M)->L2(S)->L3(C)->L4(T_C)] spec", now)
	}
	insertTrace(t, db, "L6", "task", "[PROVENANCE LOCKED L1(M)->L2(S)->L3(C)->L4(T_B)] spec", now)
	insertTrace(t, db, "L4", "wave", "L4 wave step", now)
	_, _ = db.Exec(`INSERT INTO mission_dag_nodes (id,state,updated_at) VALUES ('T_A','COMPLETED',?),('T_B','STALE',?),('T_C','COMPLETED',?)`, now, now, now)
}

func standingMetrics(t *testing.T, db *sql.DB) map[string]float64 {
	t.Helper()
	got := map[string]float64{}
	rows, err := db.Query(`SELECT metric, value FROM standing_metrics`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var k string
		var v float64
		_ = rows.Scan(&k, &v)
		got[k] = v
	}
	rows.Close()
	return got
}

func TestRecordStandingMetrics(t *testing.T) {
	db := openMetricsDB(t)
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	seedMetricFixture(t, db, now)

	RecordStandingMetrics(context.Background(), db, 3600)
	got := standingMetrics(t, db)

	// L6 turns per completed task: T_A=4, T_C=2 -> {2,4}; T_B stale -> out.
	if got["l6_turns_per_completed_task_median"] != 3.0 {
		t.Fatalf("median=%v want 3.0", got["l6_turns_per_completed_task_median"])
	}
	if got["l6_turns_per_completed_task_p90"] != 4.0 {
		t.Fatalf("p90=%v want 4.0", got["l6_turns_per_completed_task_p90"])
	}
	if got["l6_turns_per_completed_task_n"] != 2 {
		t.Fatalf("n=%v want 2", got["l6_turns_per_completed_task_n"])
	}
	if _, ok := got["l6_turns_per_completed_task_ci95_lo"]; ok {
		t.Fatal("n=2 cannot reach 95% coverage — CI keys must be absent")
	}
	if got["turns_per_success"] != 4.5 { // 9 traces / 2 completed
		t.Fatalf("turns_per_success=%v want 4.5", got["turns_per_success"])
	}
	if _, ok := got["turns_per_successful_task"]; ok {
		t.Fatal("turns_per_successful_task removed — it mixed every layer's traces")
	}
	if _, ok := got["turns_per_task"]; ok {
		t.Fatal("turns_per_task must not exist")
	}
	if got["task_success"] != 2.0/3.0 {
		t.Fatalf("task_success=%v want 0.667", got["task_success"])
	}
	if got["cost_per_success"] != 675 { // (9*(100+50))/2
		t.Fatalf("cost_per_success=%v want 675", got["cost_per_success"])
	}
	if got["layer_share_L6"] <= 0 || got["layer_share_L4"] <= 0 || got["layer_share_L1"] <= 0 {
		t.Fatalf("layer shares missing: %v", got)
	}
	if got["cache_hit_rate"] <= 0 {
		t.Fatalf("cache_hit_rate missing: %v", got)
	}
	if _, ok := got["l3_estimate_error"]; !ok {
		t.Fatal("l3_estimate_error missing")
	}
}

// A task whose L6 traces straddle the window start is partially observed and
// must be dropped from the per-task distribution, not counted truncated.
func TestL6TurnsPerTask_StraddlingTaskExcluded(t *testing.T) {
	db := openMetricsDB(t)
	now := time.Now().UTC()
	seedMetricFixture(t, db, now.Format("2006-01-02 15:04:05"))
	insertTrace(t, db, "L6", "task", "[PROVENANCE LOCKED L1(M)->L2(S)->L3(C)->L4(T_A)] spec",
		now.Add(-2*time.Hour).Format("2006-01-02 15:04:05"))

	RecordStandingMetrics(context.Background(), db, 3600)
	got := standingMetrics(t, db)

	if got["l6_turns_per_completed_task_n"] != 1 {
		t.Fatalf("n=%v want 1 — T_A straddles the boundary", got["l6_turns_per_completed_task_n"])
	}
	if got["l6_turns_per_completed_task_median"] != 2.0 {
		t.Fatalf("median=%v want 2.0 (T_C only)", got["l6_turns_per_completed_task_median"])
	}
}

func TestMedianOrderStatCI(t *testing.T) {
	xs := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	lo, hi, ok := medianOrderStatCI(xs, 0.05)
	if !ok || lo != 2 || hi != 9 {
		t.Fatalf("n=10 CI=[%v,%v] ok=%v want [2,9]", lo, hi, ok)
	}
	if _, _, ok := medianOrderStatCI([]float64{1, 2, 3, 4, 5}, 0.05); ok {
		t.Fatal("n=5 cannot reach 95% coverage — must return ok=false")
	}
}

func TestConceptRegistryAndStaleCount(t *testing.T) {
	db, _ := sql.Open("sqlite", ":memory:?cache=shared")
	defer db.Close()
	db.SetMaxOpenConns(1)
	migrateMetrics(db)
	ctx := context.Background()
	if err := RegisterConcept(ctx, db, Concept{Name: "old_flow.py", Kind: "module", Owner: "ace", Purpose: "old"}); err != nil {
		t.Fatal(err)
	}
	if err := MarkSuperseded(ctx, db, "old_flow.py", "new_flow.py"); err != nil {
		t.Fatal(err)
	}
	// Superseded concept whose path still exists live -> counts.
	if n := StaleReferenceCount(db, func(s string) bool { return s == "old_flow.py" }); n != 1 {
		t.Fatalf("stale=%d want 1", n)
	}
	// Path gone -> clean.
	if n := StaleReferenceCount(db, func(s string) bool { return false }); n != 0 {
		t.Fatalf("stale=%d want 0", n)
	}
	SeedRegistry(ctx, db)
	cs, err := Concepts(ctx, db)
	if err != nil || len(cs) < 5 {
		t.Fatalf("seed concepts=%d err=%v", len(cs), err)
	}
}
