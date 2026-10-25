// stats.go — Go-side statistics for the scientist gate (spec 8.6).
// Implemented from first principles (regularized incomplete beta via
// Lentz continued fraction; exact enumeration for small n); validated in
// stats_test.go against published reference values. Deliberately NOT a port
// of the untrusted v0.7 reference implementation.
package science

import (
	"math"
	"sort"
)

// --- core special functions -------------------------------------------------

// incBeta is the regularized incomplete beta I_x(a,b) — the Beta(a,b) CDF.
// Continued-fraction evaluation, standard Numerical Recipes formulation.
func incBeta(x, a, b float64) float64 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	lab, _ := math.Lgamma(a + b)
	la, _ := math.Lgamma(a)
	lb, _ := math.Lgamma(b)
	lnbeta := lab - la - lb
	front := math.Exp(a*math.Log(x) + b*math.Log(1-x) + lnbeta)
	if x < (a+1)/(a+b+2) {
		return front * betacf(a, b, x) / a
	}
	return 1 - front*betacf(b, a, 1-x)/b
}

func betacf(a, b, x float64) float64 {
	const maxIter = 200
	const eps = 3e-14
	qab, qap, qam := a+b, a+1.0, a-1.0
	c := 1.0
	d := 1.0 - qab*x/qap
	if math.Abs(d) < 1e-300 {
		d = 1e-300
	}
	d = 1 / d
	h := d
	for m := 1; m <= maxIter; m++ {
		m2 := 2 * m
		aa := float64(m) * (b - float64(m)) * x / ((qam + float64(m2)) * (a + float64(m2)))
		d = 1 + aa*d
		if math.Abs(d) < 1e-300 {
			d = 1e-300
		}
		c = 1 + aa/c
		if math.Abs(c) < 1e-300 {
			c = 1e-300
		}
		d = 1 / d
		h *= d * c
		aa = -(a + float64(m)) * (qab + float64(m)) * x / ((a + float64(m2)) * (qap + float64(m2)))
		d = 1 + aa*d
		if math.Abs(d) < 1e-300 {
			d = 1e-300
		}
		c = 1 + aa/c
		if math.Abs(c) < 1e-300 {
			c = 1e-300
		}
		d = 1 / d
		del := d * c
		h *= del
		if math.Abs(del-1.0) < eps {
			break
		}
	}
	return h
}

// betaInv inverts the Beta(a,b) CDF by bisection (monotone, robust).
func betaInv(p, a, b float64) float64 {
	lo, hi := 0.0, 1.0
	for i := 0; i < 80; i++ {
		mid := (lo + hi) / 2
		if incBeta(mid, a, b) < p {
			lo = mid
		} else {
			hi = mid
		}
	}
	return (lo + hi) / 2
}

// binomCDF: P(X <= k) for X ~ Binomial(n, p), via the regularized incomplete
// beta identity I_{1-p}(n-k, k+1).
func binomCDF(k, n int, p float64) float64 {
	if k < 0 {
		return 0
	}
	if k >= n {
		return 1
	}
	return incBeta(1-p, float64(n-k), float64(k+1))
}

// --- tests ------------------------------------------------------------------

// McNemarExact: discordant counts b (type A/B errors) and c; two-sided exact
// binomial p against H0 of equal discordance.
func McNemarExact(b, c int) (p float64) {
	n := b + c
	if n == 0 {
		return 1
	}
	m := min(b, c)
	return math.Min(1.0, 2*binomCDF(m, n, 0.5))
}

// McNemarChi2: continuity-corrected chi-square approximation (large n).
func McNemarChi2(b, c int) (chi2 float64) {
	n := float64(b + c)
	if n == 0 {
		return 0
	}
	d := math.Abs(float64(b-c)) - 1
	if d < 0 {
		d = 0
	}
	return d * d / n
}

// WilcoxonSignedRank: two-sided p for paired differences. Exact enumeration
// of sign assignments for n <= 20 (zeros dropped); normal approximation with
// continuity correction for larger n.
func WilcoxonSignedRank(diffs []float64) (p float64) {
	type obs struct {
		abs float64
		pos bool
	}
	var v []obs
	for _, d := range diffs {
		if d != 0 {
			v = append(v, obs{math.Abs(d), d > 0})
		}
	}
	n := len(v)
	if n == 0 {
		return 1
	}
	// average ranks with tie handling
	ord := make([]int, n)
	for i := range ord {
		ord[i] = i
	}
	sort.Slice(ord, func(i, j int) bool { return v[ord[i]].abs < v[ord[j]].abs })
	ranks := make([]float64, n)
	for i := 0; i < n; {
		j := i
		for j < n && v[ord[j]].abs == v[ord[i]].abs {
			j++
		}
		avg := float64(i+j+1) / 2.0
		for k := i; k < j; k++ {
			ranks[ord[k]] = avg
		}
		i = j
	}
	var wplus float64
	for i, o := range v {
		if o.pos {
			wplus += ranks[i]
		}
	}
	total := float64(n) * float64(n+1) / 2.0
	w := math.Min(wplus, total-wplus)

	if n <= 20 {
		// exact: doubled ranks are integral; enumerate sign assignments.
		r2 := make([]int, n)
		for i := range r2 {
			r2[i] = int(math.Round(ranks[i] * 2))
		}
		target := int(math.Round(w * 2))
		var count, all int
		var walk func(i, sum int)
		walk = func(i, sum int) {
			if i == n {
				all++
				if sum <= target {
					count++
				}
				return
			}
			walk(i+1, sum)
			walk(i+1, sum+r2[i])
		}
		walk(0, 0)
		return math.Min(1.0, 2*float64(count)/float64(all))
	}
	mu := total / 2
	sigma := math.Sqrt(float64(n) * float64(n+1) * float64(2*n+1) / 24.0)
	z := (w - mu + 0.5) / sigma
	return math.Min(1.0, math.Erfc(math.Abs(z)/math.Sqrt2))
}

// ClopperPearson: exact beta-binomial two-sided CI for x successes in n
// trials at confidence (1-alpha). Lower = BetaInv(a/2; x, n-x+1),
// upper = BetaInv(1-a/2; x+1, n-x).
func ClopperPearson(x, n int, alpha float64) (lo, hi float64) {
	if n <= 0 {
		return 0, 1
	}
	if x <= 0 {
		lo = 0
	} else {
		lo = betaInv(alpha/2, float64(x), float64(n-x+1))
	}
	if x >= n {
		hi = 1
	} else {
		hi = betaInv(1-alpha/2, float64(x+1), float64(n-x))
	}
	return lo, hi
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// IncBeta exports the regularized incomplete beta for cross-package use
// (skills posterior retirement, spec 9.6).
func IncBeta(x, a, b float64) float64 { return incBeta(x, a, b) }
