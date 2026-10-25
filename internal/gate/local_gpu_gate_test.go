package gate

import (
	"math"
	"testing"
)

func TestAnthonyD1ReadoutLogprobs(t *testing.T) {
	// Exact test vectors from Anthony's LyteFEX commit a756d6a (tests/extractors/ml_models/decision_test.py)
	openDoor := map[string]float64{
		"yes":  -0.05156010761857033,
		"no":   -3.301560163497925,
		"Yes":  -5.239059925079346,
		" yes": -7.489059925079346,
	}
	if p, ok := ReadoutD1Logprobs(openDoor); !ok || math.Abs(p-0.963) > 1e-3 {
		t.Fatalf("open door: got (%v, %.4f), want (true, ~0.963)", ok, p)
	}

	closedDoor := map[string]float64{
		"no":  -0.15982569754123688,
		"yes": -2.0348258018493652,
		"No":  -5.659825801849365,
		"Yes": -7.347325801849365,
	}
	if p, ok := ReadoutD1Logprobs(closedDoor); !ok || math.Abs(p-0.133) > 1e-3 {
		t.Fatalf("closed door: got (%v, %.4f), want (true, ~0.133)", ok, p)
	}

	bestCase := map[string]float64{
		"Yes": -1.0,
		"yes": -2.0,
		"NO":  -1.0,
	}
	if p, ok := ReadoutD1Logprobs(bestCase); !ok || math.Abs(p-0.5) > 1e-3 {
		t.Fatalf("best case: got (%v, %.4f), want (true, 0.5)", ok, p)
	}

	ignoresLeadingSpace := map[string]float64{
		" yes": -0.1,
		"yes":  -3.0,
		"no":   -3.0,
	}
	if p, ok := ReadoutD1Logprobs(ignoresLeadingSpace); !ok || math.Abs(p-0.5) > 1e-3 {
		t.Fatalf("ignores leading space: got (%v, %.4f), want (true, 0.5)", ok, p)
	}

	missingSide := map[string]float64{
		"yes":  -0.1,
		"true": -5.0,
	}
	if _, ok := ReadoutD1Logprobs(missingSide); ok {
		t.Fatalf("expected missing 'no' side to return ok=false")
	}
}
