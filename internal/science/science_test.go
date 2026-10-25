package science

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestShingleJaccard(t *testing.T) {
	a := "build a dataset joining harness outcomes by script path"
	b := "build a dataset joining harness outcomes by script path today"
	if j := ShingleJaccard(a, a); j != 1.0 {
		t.Fatalf("identical directives Jaccard=%v want 1.0", j)
	}
	if j := ShingleJaccard(a, b); j < 0.8 {
		t.Fatalf("near-duplicate Jaccard=%v want >=0.8", j)
	}
	if j := ShingleJaccard(a, "probe the oracle with mutated artifacts"); j >= 0.8 {
		t.Fatalf("distinct directives Jaccard=%v want <0.8", j)
	}
}

func TestImperativeScore(t *testing.T) {
	if s := ImperativeScore("research and measure the honest verification integrity audit"); s <= 0 {
		t.Fatalf("research directive scored %v", s)
	}
	full := ImperativeScore("research, measure, and build a safe verifiable tool that improves capability")
	if ImperativeScore("zzzzz") >= full {
		t.Fatal("gibberish should score below a full-spectrum directive")
	}
}

func TestBranchStalled(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if BranchStalled(ctx, db, "S1") {
		t.Fatal("branch with no results must not be stalled")
	}
	RecordChildResult(ctx, db, "S1", "T1", false)
	RecordChildResult(ctx, db, "S1", "T2", false)
	RecordChildResult(ctx, db, "S1", "T3", false)
	if !BranchStalled(ctx, db, "S1") {
		t.Fatal("3 consecutive unmoved results must stall the branch")
	}
	RecordChildResult(ctx, db, "S1", "T4", true)
	if BranchStalled(ctx, db, "S1") {
		t.Fatal("a moved result resets the stall streak")
	}
}
