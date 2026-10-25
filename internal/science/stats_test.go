package science

import (
	"math"
	"testing"
)

// Reference values below are published/canonical worked examples, not derived
// from any ACE code path (spec 8.6: not ported from the v0.7 reference).

func TestMcNemarExact_Published(t *testing.T) {
	// R exact mcnemar.test on discordants b=6, c=16 -> p = 0.0524811
	// (2 * P(X<=6 | Bin(22, 0.5)) = 0.05248).
	if p := McNemarExact(6, 16); math.Abs(p-0.05248) > 1e-4 {
		t.Fatalf("McNemar(6,16)=%.5f want ~0.05248", p)
	}
	// b=c -> p = 1 (no discordance difference).
	if p := McNemarExact(10, 10); p != 1.0 {
		t.Fatalf("McNemar(10,10)=%v want 1", p)
	}
	// Strong asymmetry: b=0,c=12 -> 2*(1/2)^12 = 0.000488.
	if p := McNemarExact(0, 12); math.Abs(p-0.000488) > 1e-6 {
		t.Fatalf("McNemar(0,12)=%.6f want ~0.000488", p)
	}
}

func TestWilcoxonSignedRank_Published(t *testing.T) {
	// All-positive n=8: W- = 0, exact two-sided p = 2/2^8 = 0.0078125.
	d := []float64{1.83, 0.50, 1.62, 2.48, 1.68, 1.88, 1.55, 3.06}
	if p := WilcoxonSignedRank(d); math.Abs(p-0.0078125) > 1e-6 {
		t.Fatalf("Wilcoxon all-positive n=8 p=%.6f want 0.0078125", p)
	}
	// Symmetric diffs -> large p.
	sym := []float64{1, -1, 2, -2, 3, -3, 4, -4}
	if p := WilcoxonSignedRank(sym); p < 0.5 {
		t.Fatalf("Wilcoxon symmetric p=%v want >0.5", p)
	}
	// n=5 all positive: 2/32 = 0.0625.
	if p := WilcoxonSignedRank([]float64{1, 2, 3, 4, 5}); math.Abs(p-0.0625) > 1e-6 {
		t.Fatalf("Wilcoxon n=5 p=%.4f want 0.0625", p)
	}
}

func TestClopperPearson_Published(t *testing.T) {
	// R binom.test(1, 10, conf=0.95): (0.002528579, 0.4450161)
	lo, hi := ClopperPearson(1, 10, 0.05)
	if math.Abs(lo-0.00253) > 1e-4 || math.Abs(hi-0.44502) > 1e-4 {
		t.Fatalf("CP(1,10)=(%.5f,%.5f) want (0.00253,0.44502)", lo, hi)
	}
	// R binom.test(5, 20): (0.08657147, 0.4910459)
	lo, hi = ClopperPearson(5, 20, 0.05)
	if math.Abs(lo-0.08657) > 1e-4 || math.Abs(hi-0.49105) > 1e-4 {
		t.Fatalf("CP(5,20)=(%.5f,%.5f) want (0.08657,0.49105)", lo, hi)
	}
	// Edges: x=0 lower bound 0; x=n upper bound 1.
	lo, hi = ClopperPearson(0, 10, 0.05)
	if lo != 0 || hi <= 0.3 {
		t.Fatalf("CP(0,10)=(%.4f,%.4f)", lo, hi)
	}
}
