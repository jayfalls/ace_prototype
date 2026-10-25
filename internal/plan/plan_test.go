package plan

import (
	"context"
	"database/sql"
	"testing"

	"ace/internal/science"
	_ "modernc.org/sqlite"
)

func TestDuplicateOfShingle(t *testing.T) {
	existing := []Strategy{{ID: "S1", Title: "widen corpus", Directive: "widen c_s13_data corpus to all real modules"}}
	dup, ok := DuplicateOf("widen corpus", "widen c_s13_data corpus to all real modules", existing)
	if !ok || dup != "S1" {
		t.Fatalf("verbatim restatement should duplicate S1, got %q %v", dup, ok)
	}
}

func TestDuplicateOfArtefacts(t *testing.T) {
	// Different wording, same target module — the case word-shingles missed live.
	existing := []Strategy{{ID: "S16", Title: "hold spec", Directive: "hold until L2 reads S15 note, c_s13_data results"}}
	dup, ok := DuplicateOf("widen corpus differently", "c_s13_data enumerates ALL real modules and helpers", existing)
	if !ok || dup != "S16" {
		t.Fatalf("same artefact set should duplicate S16, got %q %v", dup, ok)
	}
	if _, ok := DuplicateOf("new direction", "build evaluation dataset eval_v2.json from traces", existing); ok {
		t.Fatal("distinct artefact set must not flag as duplicate")
	}
}

func setupStaleDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE mission_dag_nodes (id TEXT PRIMARY KEY, parent_id TEXT, stage TEXT, state TEXT, title TEXT,
		directive TEXT, soul_hash TEXT, reanchor_count INTEGER NOT NULL DEFAULT 0, retired_reason TEXT NOT NULL DEFAULT '', updated_at TEXT)`); err != nil {
		t.Fatal(err)
	}
	if err := science.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestSweepStaleRetireUnderCompletedParent(t *testing.T) {
	db := setupStaleDB(t)
	defer db.Close()
	ctx := context.Background()
	_, _ = db.Exec(`INSERT INTO mission_dag_nodes (id,parent_id,stage,state,title,directive,soul_hash,updated_at) VALUES
		('root','', 'L1_MISSION_ROOT','COMPLETED','r','','s0',''),
		('S1','root','L2_STRATEGY','STALE','stale branch','','s0','')`)
	n, err := SweepStale(ctx, db, "s1", nil)
	if err != nil || n != 1 {
		t.Fatalf("sweep: n=%d err=%v", n, err)
	}
	var state, reason string
	_ = db.QueryRow(`SELECT state, retired_reason FROM mission_dag_nodes WHERE id='S1'`).Scan(&state, &reason)
	if state != "RETIRED" || reason == "" {
		t.Fatalf("expected RETIRED with reason, got %s %q", state, reason)
	}
	var reg string
	if db.QueryRow(`SELECT superseded_by FROM concept_registry WHERE name='S1'`).Scan(&reg); reg != "retired" {
		t.Fatalf("registry row missing: %q", reg)
	}
}

func TestSweepStaleReanchorThenRetire(t *testing.T) {
	db := setupStaleDB(t)
	defer db.Close()
	ctx := context.Background()
	_, _ = db.Exec(`INSERT INTO mission_dag_nodes (id,parent_id,stage,state,title,directive,soul_hash,updated_at) VALUES
		('root','','L1_MISSION_ROOT','L1_ANCHORED','r','','s1',''),
		('T1','c1','L4_TASK_SPEC','STALE','stale task','','s0',''),
		('c1','s1','L3_CAPABILITY','L4_SPECCED','c','','s1','')`)
	calls := 0
	verdict := func(_ context.Context, id, title string) (bool, string) { calls++; return true, "still relevant" }
	if _, err := SweepStale(ctx, db, "s1", verdict); err != nil {
		t.Fatal(err)
	}
	var state, soul string
	var ra int
	_ = db.QueryRow(`SELECT state, soul_hash, reanchor_count FROM mission_dag_nodes WHERE id='T1'`).Scan(&state, &soul, &ra)
	if state != "READY_FOR_L5" || soul != "s1" || ra != 1 || calls != 1 {
		t.Fatalf("re-anchor: state=%s soul=%s reanchors=%d calls=%d", state, soul, ra, calls)
	}
	// Second STALE -> retire, no further verdict calls.
	_, _ = db.Exec(`UPDATE mission_dag_nodes SET state='STALE' WHERE id='T1'`)
	if _, err := SweepStale(ctx, db, "s2", verdict); err != nil {
		t.Fatal(err)
	}
	_ = db.QueryRow(`SELECT state FROM mission_dag_nodes WHERE id='T1'`).Scan(&state)
	if state != "RETIRED" || calls != 1 {
		t.Fatalf("second stale must retire without another verdict: %s calls=%d", state, calls)
	}
}

func TestSweepStaleDeniedReanchorRetires(t *testing.T) {
	db := setupStaleDB(t)
	defer db.Close()
	ctx := context.Background()
	_, _ = db.Exec(`INSERT INTO mission_dag_nodes (id,parent_id,stage,state,title,directive,soul_hash,updated_at) VALUES
		('root','','L1_MISSION_ROOT','L1_ANCHORED','r','','s1',''),
		('S9','root','L2_STRATEGY','STALE','off-mission branch','','s0','')`)
	verdict := func(_ context.Context, id, title string) (bool, string) { return false, "no longer serves mission" }
	if _, err := SweepStale(ctx, db, "s1", verdict); err != nil {
		t.Fatal(err)
	}
	var state, reason string
	_ = db.QueryRow(`SELECT state, retired_reason FROM mission_dag_nodes WHERE id='S9'`).Scan(&state, &reason)
	if state != "RETIRED" || reason != "re-anchor denied: no longer serves mission" {
		t.Fatalf("denied re-anchor must retire: %s %q", state, reason)
	}
}
