package science

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func gateDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func mkPreds(n int, acc float64, labels ...string) []PredPair {
	out := make([]PredPair, 0, n)
	for i := 0; i < n; i++ {
		l := labels[i%len(labels)]
		pred := l
		if float64(i)/float64(n) >= acc {
			pred = "wrong-" + l
		}
		out = append(out, PredPair{Label: l, Predicted: pred})
	}
	return out
}

func TestGateRejectsZeroEntropyLabels(t *testing.T) {
	db := gateDB(t)
	q := Question{ID: "q1", PrimaryStat: "accuracy"}
	preds := mkPreds(200, 0.9, "pos")
	v := EvaluateExperiment(context.Background(), db, q, preds, true)
	if v.Tier == "EXTERNAL" {
		t.Fatal("zero-entropy labels must never reach EXTERNAL")
	}
	found := false
	for _, r := range v.Reasons {
		if r == "zero-entropy labels — degenerate evaluation set" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected zero-entropy reason")
	}
}

func TestGateRejectsConstantPredictions(t *testing.T) {
	db := gateDB(t)
	q := Question{ID: "q2", PrimaryStat: "accuracy"}
	var preds []PredPair
	for i := 0; i < 200; i++ {
		l := "pos"
		if i%2 == 0 {
			l = "neg"
		}
		preds = append(preds, PredPair{Label: l, Predicted: "pos"})
	}
	v := EvaluateExperiment(context.Background(), db, q, preds, true)
	if v.Tier == "EXTERNAL" {
		t.Fatal("constant predictions must never pass")
	}
}

func TestGateUnderpoweredNeverPasses(t *testing.T) {
	db := gateDB(t)
	q := Question{ID: "q3", PrimaryStat: "accuracy", MDE: 0.10}
	preds := mkPreds(30, 0.75, "pos", "neg")
	v := EvaluateExperiment(context.Background(), db, q, preds, true)
	if v.Powered {
		t.Fatalf("n=30 must be underpowered for MDE=0.10")
	}
	if v.Tier == "EXTERNAL" {
		t.Fatal("underpowered must not reach EXTERNAL")
	}
}

func TestGateIndependentLabelRule(t *testing.T) {
	db := gateDB(t)
	RegisterQuestion(context.Background(), db, Question{ID: "q4", Hypothesis: "h", PrimaryStat: "accuracy", MDE: 0.5})
	preds := mkPreds(400, 0.95, "pos", "neg")
	v := EvaluateExperiment(context.Background(), db, Question{ID: "q4", PrimaryStat: "accuracy", MDE: 0.5, Alpha: 0.05}, preds, true)
	if v.Stat < 0.9 {
		t.Fatalf("recomputed stat %.3f", v.Stat)
	}
	v2 := EvaluateExperiment(context.Background(), db, Question{ID: "q4", PrimaryStat: "accuracy", MDE: 0.5, Alpha: 0.05}, preds, false)
	if v2.Tier == "EXTERNAL" {
		t.Fatal("non-independent labels must stay PROVISIONAL")
	}
}
