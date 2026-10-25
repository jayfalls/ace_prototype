package contextsys

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// Spec: ContractTest fails when a declared input is removed from the
// built prompt — checked per layer.
func TestContractTestPerLayer(t *testing.T) {
	full := strings.Join([]string{
		"CONSTITUTION", "CURRENT MISSION", "ACTIVE GOALS", "TEMPORAL CONTEXT",
	}, " | ")
	for _, c := range LayerContracts {
		if err := ContractTest(c.Layer, full); err != nil {
			t.Fatalf("layer %d contract failed on complete prompt: %v", c.Layer, err)
		}
	}
	// Removing a declared input must fail for the layer that declares it.
	broken := strings.Replace(full, "TEMPORAL CONTEXT", "SPATIAL CONTEXT", 1)
	for _, c := range LayerContracts {
		if err := ContractTest(c.Layer, broken); err == nil {
			t.Fatalf("layer %d contract passed with a declared input removed", c.Layer)
		}
	}
}

// Spec: a 1 MB tool result writes a file; the prompt holds head + size +
// path and stays under the cap.
func TestOverflowWritesFile(t *testing.T) {
	dir := t.TempDir()
	o := NewOverflow(dir)
	big := strings.Repeat("x", 1<<20)
	out := o.Handle("tool-result.json", big, 8192)
	if len(out) > 8192 {
		t.Fatalf("overflow output %d bytes exceeds cap", len(out))
	}
	for _, want := range []string{"OVERFLOW", "bytes total", "spill-tool-result.json", "read by range"} {
		if !strings.Contains(out, want) {
			t.Fatalf("overflow marker %q missing from output", want)
		}
	}
	b, err := os.ReadFile(dir + "/spill-tool-result.json")
	if err != nil || len(b) != len(big) {
		t.Fatalf("spill file: err=%v len=%d want %d", err, len(b), len(big))
	}
	// Under-cap input passes through untouched.
	if got := o.Handle("small", "tiny", 100); got != "tiny" {
		t.Fatalf("under-cap passthrough changed content: %q", got)
	}
}

// Spec: a synthetic session crossing 50/65/80/90% fires each tier exactly
// once and the hot path makes no model calls (counted via fake adapter).
func TestCompactionTiersFireOnce(t *testing.T) {
	modelCalls := 0
	var events []string
	sum := func(ctx context.Context, prompt string) (string, error) {
		modelCalls++
		return "Goal\nx\nConstraints\ny\nProgress\nz\nBlocked\nnone\nNext\nn", nil
	}
	c := NewCompactor(sum, func(ev InjectionEvent) { events = append(events, ev.Source) })
	var turns []Turn
	for i := 0; i < 40; i++ {
		turns = append(turns, Turn{Role: "tool", Content: strings.Repeat("d", 2048), ToolBytes: 2048, At: time.Now()})
	}
	fired := map[string]int{}
	for _, fill := range []float64{0.51, 0.51, 0.66, 0.66, 0.81, 0.81, 0.91, 0.91} {
		if out := c.MaybeCompact(context.Background(), turns, fill, 100000); out != nil {
			fired[c.TierFor(fill).String()]++
		}
	}
	for _, tier := range []string{"snip", "micro", "collapse", "auto"} {
		if fired[tier] != 1 {
			t.Fatalf("tier %s fired %d times, want exactly 1", tier, fired[tier])
		}
	}
	if modelCalls != 1 {
		t.Fatalf("model calls = %d, want 1 (only the auto tier calls the model)", modelCalls)
	}
}

// Spec: 3 consecutive auto-summary failures stop the auto tier.
func TestAutoStopsAfterThreeFailures(t *testing.T) {
	calls := 0
	c := NewCompactor(func(ctx context.Context, p string) (string, error) {
		calls++
		return "", errors.New("summarizer down")
	}, nil)
	turns := []Turn{{Role: "user", Content: "work"}}
	for i := 0; i < 5; i++ {
		// Each MaybeCompact must be a fresh auto-tier crossing.
		c.lastTier = TierCollapse
		_ = c.MaybeCompact(context.Background(), turns, 0.95, 100000)
	}
	if calls != 3 {
		t.Fatalf("summarizer calls = %d, want exactly 3 before stop", calls)
	}
	if !c.AutoDisabled() {
		t.Fatal("auto tier not disabled after 3 failures")
	}
}

// Spec: identical inputs produce a byte-identical prompt prefix across
// two builds (stable-first ordering for the prefix cache).
func TestByteIdenticalPrefix(t *testing.T) {
	build := func() string {
		s := NewStage(StageTurn0, "t0",
			BucketSpec{"soul", 4096}, BucketSpec{"goals", 2048}, BucketSpec{"files", 8192})
		for _, name := range StableOrder([]string{"files", "soul", "goals"}) {
			s.Bucket(name).Fill(name + "-body")
		}
		return s.Render()
	}
	if build() != build() {
		t.Fatal("identical inputs produced different renders")
	}
	// Render order is the declared bucket order — soul is the stable prefix.
	if got := build(); !strings.HasPrefix(got, "soul-body") {
		t.Fatalf("declared order broken: prefix = %q", got[:20])
	}
}

// Spec: closure ranking by VOI/cost within the 10% exploration budget.
func TestClosureWithinBudget(t *testing.T) {
	items := []ClosureItem{
		{ID: "a", Kind: "open_question", VOI: 0.9, Cost: 4},
		{ID: "b", Kind: "unviewed_file", VOI: 0.6, Cost: 1},
		{ID: "c", Kind: "untested_assumption", VOI: 0.5, Cost: 8},
	}
	ranked := RankClosure(items)
	if ranked[0].ID != "b" {
		t.Fatalf("rank[0] = %s, want b (VOI/cost 0.6)", ranked[0].ID)
	}
	chosen, rem := WithinBudget(ranked, 4)
	if len(chosen) != 1 || chosen[0].ID != "b" {
		t.Fatalf("budget chose %v, want [b] (a costs 4 > remaining 3)", chosen)
	}
	if rem != 3 {
		t.Fatalf("remaining = %v, want 3", rem)
	}
}

// Spec: map frontier — deterministic tree search from an ROI covering
// every downstream branch, honoring per-layer visibility.
func TestMapFrontier(t *testing.T) {
	db := openTestDB(t)
	if err := MigrateMap(db); err != nil {
		t.Fatal(err)
	}
	m := NewMap(db)
	ctx := context.Background()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(m.AddNode(ctx, MapNode{ID: "roi", Kind: "spec", Ref: "s"}))
	must(m.AddNode(ctx, MapNode{ID: "f1", Kind: "file", Ref: "/workspace/a.py"}))
	must(m.AddNode(ctx, MapNode{ID: "f2", Kind: "file", Ref: "/workspace/b.py"}))
	must(m.AddNode(ctx, MapNode{ID: "q1", Kind: "question", Ref: "open?"}))
	must(m.AddNode(ctx, MapNode{ID: "hidden", Kind: "file", Ref: "/workspace/l6only", Layers: LayerMask(6)}))
	must(m.AddEdge(ctx, MapEdge{FromID: "roi", ToID: "f1", Kind: "mentions"}))
	must(m.AddEdge(ctx, MapEdge{FromID: "roi", ToID: "q1", Kind: "mentions"}))
	must(m.AddEdge(ctx, MapEdge{FromID: "f1", ToID: "f2", Kind: "depends_on"}))
	must(m.AddEdge(ctx, MapEdge{FromID: "q1", ToID: "hidden", Kind: "depends_on"}))

	fr, err := m.Frontier(ctx, "roi", 6, 0)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, n := range fr {
		ids = append(ids, n.ID)
	}
	want := []string{"f1", "q1", "f2", "hidden"} // BFS: level 1, then level 2
	if fmt.Sprint(ids) != fmt.Sprint(want) {
		t.Fatalf("frontier = %v, want %v", ids, want)
	}
	// Layer 5 cannot see the L6-only node.
	fr5, _ := m.Frontier(ctx, "roi", 5, 0)
	for _, n := range fr5 {
		if n.ID == "hidden" {
			t.Fatal("layer 5 saw an L6-only node")
		}
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
