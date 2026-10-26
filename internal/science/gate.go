// gate.go — the scientist gate (spec 8.6): recomputes statistics from raw
// per-item predictions, ignores self-declared booleans, runs baselines and
// degeneracy controls, and assigns EXTERNAL | PROVISIONAL evidence tiers.
package science

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// PredPair is one item: true label + model prediction (both opaque strings —
// binary/label tasks only need equality).
type PredPair struct {
	Label     string
	Predicted string
}

// Verdict is the gate's output — a measurement, never a declaration.
type Verdict struct {
	Tier     string     `json:"tier"` // EXTERNAL | PROVISIONAL
	Stat     float64    `json:"stat"` // primary statistic (accuracy)
	Baseline float64    `json:"baseline"`
	PValue   float64    `json:"p_value"` // McNemar vs baseline predictor
	CI       [2]float64 `json:"ci"`      // Clopper-Pearson 95%
	Powered  bool       `json:"powered"`
	Reasons  []string   `json:"reasons"`
	// PersistErr carries any science_verdicts insert failure — a nil db
	// insert that previously vanished silently.
	PersistErr error `json:"-"`
}

// EvaluateExperiment recomputes the primary statistic for question q against
// raw predictions. labelsIndependent=true asserts labels came from outside
// ACE (public dataset, external system, or a test run) — required for
// EXTERNAL (spec 8.6 independent-label rule).
func EvaluateExperiment(ctx context.Context, db *sql.DB, q Question, preds []PredPair, labelsIndependent bool) Verdict {
	v := Verdict{Tier: "PROVISIONAL"}
	n := len(preds)
	if n == 0 {
		v.Reasons = append(v.Reasons, "no predictions")
		v.PersistErr = recordVerdict(ctx, db, q.ID, "", v)
		return v
	}

	// Degeneracy controls: zero-entropy labels or constant predictions can
	// never produce evidence.
	if labelEntropy(preds, true) == 0 {
		v.Reasons = append(v.Reasons, "zero-entropy labels — degenerate evaluation set")
		v.PersistErr = recordVerdict(ctx, db, q.ID, "", v)
		return v
	}
	if labelEntropy(preds, false) == 0 {
		v.Reasons = append(v.Reasons, "constant predictions — model outputs never vary")
		v.PersistErr = recordVerdict(ctx, db, q.ID, "", v)
		return v
	}

	// Primary statistic: accuracy recomputed from per-item pairs.
	var correct int
	for _, p := range preds {
		if p.Predicted == p.Label {
			correct++
		}
	}
	v.Stat = float64(correct) / float64(n)
	v.CI[0], v.CI[1] = ClopperPearson(correct, n, q.Alpha)

	// Baselines: majority predictor + shuffled-label control (negative
	// control — a real signal must beat the shuffle).
	v.Baseline = majorityAccuracy(preds)
	v.PValue = McNemarExact(mcnemarB(preds), mcnemarC(preds, majorityLabel(preds)))
	shuffled := shuffledAccuracy(preds, 200)
	if v.Stat <= shuffled+0.02 {
		v.Reasons = append(v.Reasons, fmt.Sprintf("does not beat shuffled-label control (%.3f vs %.3f)", v.Stat, shuffled))
	}

	// Power: n required for alpha/power/MDE (two-sided two-proportion proxy).
	required := requiredN(q.Alpha, 0.8, q.MDE, v.Baseline)
	v.Powered = n >= required
	if !v.Powered {
		v.Reasons = append(v.Reasons, fmt.Sprintf("underpowered: n=%d < required %d — 'no large effect', never a pass", n, required))
	}

	// Holm: alpha corrected across this question's attempts.
	var attempts int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM prereg_attempts WHERE question_id=?`, q.ID).Scan(&attempts)
	adjAlpha := HolmAlpha(q.Alpha, attempts+1, 1) // current attempt ranked by p at record time

	significant := v.Powered && v.PValue < adjAlpha && v.Stat > v.Baseline+q.MDE
	if significant && labelsIndependent {
		v.Tier = "EXTERNAL"
		v.Reasons = append(v.Reasons, fmt.Sprintf("EXTERNAL: stat %.3f beats baseline %.3f by >= MDE %.2f, p=%.4f < Holm-alpha %.4f, independent labels", v.Stat, v.Baseline, q.MDE, v.PValue, adjAlpha))
	} else if significant {
		v.Reasons = append(v.Reasons, "significant but labels not independent — PROVISIONAL")
	}
	v.PersistErr = recordVerdict(ctx, db, q.ID, "", v)
	return v
}

// labelEntropy over labels (preds=true) or predictions (preds=false).
func labelEntropy(preds []PredPair, onLabels bool) float64 {
	counts := map[string]int{}
	for _, p := range preds {
		s := p.Label
		if !onLabels {
			s = p.Predicted
		}
		counts[s]++
	}
	var h float64
	for _, c := range counts {
		p := float64(c) / float64(len(preds))
		h -= p * math.Log2(p)
	}
	return h
}

func majorityLabel(preds []PredPair) string {
	counts := map[string]int{}
	for _, p := range preds {
		counts[p.Label]++
	}
	best, bn := "", 0
	for l, c := range counts {
		if c > bn {
			best, bn = l, c
		}
	}
	return best
}

func majorityAccuracy(preds []PredPair) float64 {
	counts := map[string]int{}
	for _, p := range preds {
		counts[p.Label]++
	}
	max := 0
	for _, c := range counts {
		if c > max {
			max = c
		}
	}
	return float64(max) / float64(len(preds))
}

// mcnemarB/C: discordant counts — model-right/baseline-wrong (b) vs
// model-wrong/baseline-right (c) vs the majority predictor.
func mcnemarB(preds []PredPair) int {
	maj := majorityLabel(preds)
	b := 0
	for _, p := range preds {
		if p.Predicted == p.Label && maj != p.Label {
			b++
		}
	}
	return b
}

func mcnemarC(preds []PredPair, maj string) int {
	c := 0
	for _, p := range preds {
		if p.Predicted != p.Label && maj == p.Label {
			c++
		}
	}
	return c
}

func shuffledAccuracy(preds []PredPair, iters int) float64 {
	r := rand.New(rand.NewSource(42))
	labels := make([]string, len(preds))
	for i, p := range preds {
		labels[i] = p.Label
	}
	var tot float64
	for it := 0; it < iters; it++ {
		r.Shuffle(len(labels), func(i, j int) { labels[i], labels[j] = labels[j], labels[i] })
		var c int
		for i, p := range preds {
			if p.Predicted == labels[i] {
				c++
			}
		}
		tot += float64(c) / float64(len(preds))
	}
	return tot / float64(iters)
}

// requiredN: normal-approximation n for a two-sided proportion delta at
// given alpha/power/MDE around baseline rate p.
func requiredN(alpha, power, mde, p float64) int {
	if mde <= 0 {
		return math.MaxInt32
	}
	za := normInv(1 - alpha/2)
	zb := normInv(power)
	p2 := p + mde
	if p2 > 1 {
		p2 = 1
	}
	num := za*math.Sqrt(2*p*(1-p)) + zb*math.Sqrt(p*(1-p)+p2*(1-p2))
	return int(math.Ceil(num * num / (mde * mde)))
}

// normInv: inverse standard normal CDF (Beasley-Springer-Moro-ish via
// Halley refinement on erfc).
func normInv(p float64) float64 {
	if p <= 0 {
		return -10
	}
	if p >= 1 {
		return 10
	}
	// initial approximation (Acklam)
	x := math.Sqrt(-2 * math.Log(math.Min(p, 1-p)))
	x = x - (2.515517+0.802853*x+0.010328*x*x)/(1+1.432788*x+0.189269*x*x+0.001308*x*x*x)
	if p < 0.5 {
		x = -x
	}
	// one Halley polish
	phi := 0.5 * math.Erfc(-x/math.Sqrt2)
	x = x - (phi-p)/(math.Exp(-x*x/2)/math.Sqrt(2*math.Pi))
	return x
}

func recordVerdict(ctx context.Context, db *sql.DB, qid, ref string, v Verdict) error {
	detail := ""
	for _, r := range v.Reasons {
		detail += r + "; "
	}
	_, err := db.ExecContext(ctx, `INSERT INTO science_verdicts (question_id, result_ref, tier, stat, p_value, detail, created_at) VALUES (?,?,?,?,?,?,?)`,
		qid, ref, v.Tier, v.Stat, v.PValue, truncate(detail, 500), time.Now().UTC().Format(time.RFC3339))
	return err
}

// RegradeResultJSON re-evaluates a result blob's declared statistics: when
// McNemar discordant counts are present the p-value is recomputed (the
// declared value is never trusted), and a science_verdicts row records the
// outcome. Returns (tier, detail, err) — err is any insert failure on the
// verdict row; callers must surface it (a silently missing verdict is the
// bug this round fixed). Items without recomputable counts grade
// PROVISIONAL — claims only, no recomputation possible.
func RegradeResultJSON(ctx context.Context, db *sql.DB, blob []byte, ref string) (string, string, error) {
	var r struct {
		H1Accepted *bool    `json:"h1_accepted"`
		Decision   string   `json:"decision"`
		McFixed    *float64 `json:"mcnemar_fixed"`
		McBroken   *float64 `json:"mcnemar_broken"`
		McP        *float64 `json:"mcnemar_p"`
		Alpha      *float64 `json:"alpha_level"`
	}
	if json.Unmarshal(blob, &r) != nil {
		err := recordVerdict(ctx, db, ref, ref, Verdict{Tier: "PROVISIONAL", Reasons: []string{"unparseable result json"}})
		return "PROVISIONAL", "unparseable result json", err
	}
	alpha := 0.05
	if r.Alpha != nil && *r.Alpha > 0 {
		alpha = *r.Alpha
	}
	if r.McFixed == nil || r.McBroken == nil {
		err := recordVerdict(ctx, db, ref, ref, Verdict{Tier: "PROVISIONAL", Reasons: []string{"no recomputable discordant counts"}})
		return "PROVISIONAL", "no mcnemar counts — declared stats unrecomputable", err
	}
	p := McNemarExact(int(*r.McFixed), int(*r.McBroken))
	tier := "PROVISIONAL"
	detail := fmt.Sprintf("recomputed mcnemar p=%.5f (alpha %.3f)", p, alpha)
	if r.H1Accepted != nil && *r.H1Accepted && p > alpha {
		tier = "REJECTED"
		detail += " — declared ACCEPT contradicted by recompute"
	} else if r.McP != nil && math.Abs(*r.McP-p) > 0.01 {
		detail += fmt.Sprintf(" — declared p %.5f != recomputed %.5f", *r.McP, p)
	} else {
		detail += " — consistent with declared"
	}
	err := recordVerdict(ctx, db, ref, ref, Verdict{Tier: tier, Stat: float64(*r.McFixed), PValue: p, Reasons: []string{detail}})
	return tier, detail, err
}
