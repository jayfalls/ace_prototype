package science

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestRecordStandingMetrics(t *testing.T) {
	db, _ := sql.Open("sqlite", ":memory:?cache=shared")
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, ddl := range []string{
		`CREATE TABLE thought_traces (layer_code TEXT, latency_ms INTEGER, prompt_tokens INTEGER, cached_tokens INTEGER, completion_tokens INTEGER, confidence REAL, action_result TEXT, created_at DATETIME)`,
		`CREATE TABLE mission_dag_nodes (id TEXT, state TEXT, updated_at DATETIME)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}
	migrateMetrics(db)
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	for _, r := range [][7]any{
		{"L6", 100, 1000, 500, 200, 0.9, "success"},
		{"L6", 200, 500, 0, 100, 0.1, "failed"},
		{"L4", 50, 200, 0, 50, 0.5, "success"},
	} {
		_, err := db.Exec(`INSERT INTO thought_traces (layer_code,latency_ms,prompt_tokens,cached_tokens,completion_tokens,confidence,action_result,created_at) VALUES (?,?,?,?,?,?,?,?)`,
			r[0], r[1], r[2], r[3], r[4], r[5], r[6], now)
		if err != nil {
			t.Fatal(err)
		}
	}
	_, _ = db.Exec(`INSERT INTO mission_dag_nodes (id,state,updated_at) VALUES ('T_A','COMPLETED',?),('T_B','STALE',?)`, now, now)

	RecordStandingMetrics(context.Background(), db, 3600)

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

	if got["turns_per_successful_task"] != 3.0 { // 3 traces / 1 completed
		t.Fatalf("turns_per_successful_task=%v want 3.0", got["turns_per_successful_task"])
	}
	if _, ok := got["turns_per_task"]; ok {
		t.Fatal("turns_per_task must not exist — it is not the v0.9 primary metric")
	}
	if got["task_success"] != 0.5 {
		t.Fatalf("task_success=%v want 0.5", got["task_success"])
	}
	if got["cost_per_success"] != 2050 { // (1000+200+500+100+200+50)/1
		t.Fatalf("cost_per_success=%v want 2050", got["cost_per_success"])
	}
	if got["layer_share_L6"] <= 0 || got["layer_share_L4"] <= 0 {
		t.Fatalf("layer shares missing: %v", got)
	}
	if got["cache_hit_rate"] <= 0 {
		t.Fatalf("cache_hit_rate missing: %v", got)
	}
	if _, ok := got["l3_estimate_error"]; !ok {
		t.Fatal("l3_estimate_error missing")
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
