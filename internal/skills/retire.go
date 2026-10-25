package skills

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ace/internal/science"
)

// Retirement policy (spec 9.6 / §5): retire after >= RetireMinUses
// observations when P(rate < 0.5) > RetireConfidence — i.e. the Beta(α,β)
// posterior says the skill is almost surely worse than a coin flip.
const (
	RetireMinUses    = 5
	RetireRateFloor  = 0.5
	RetireConfidence = 0.9
)

// ShouldRetire reports whether the record's posterior has fallen below
// the retirement rule. Uses = Observations; P(rate<0.5) is the exact
// Beta CDF via the incomplete beta (science.IncBeta).
func ShouldRetire(s WikiSkillRecord) bool {
	if s.Retired || s.Observations < RetireMinUses {
		return false
	}
	return science.IncBeta(RetireRateFloor, s.Alpha, s.Beta) > RetireConfidence
}

// Retire marks the record retired and reports the registration text the
// caller persists as a concept-registry row (spec 8.8). It returns the
// updated record, the registry payload, and whether retirement happened.
func Retire(ctx context.Context, s WikiSkillRecord) (WikiSkillRecord, string, bool) {
	if !ShouldRetire(s) {
		return s, "", false
	}
	s.Retired = true
	s.UpdatedAt = time.Now().UTC()
	payload := fmt.Sprintf(
		"skill %s retired: %d observations, Beta(α=%.1f, β=%.1f), P(rate<%.1f)=%.2f > %.1f",
		s.Key, s.Observations, s.Alpha, s.Beta, RetireRateFloor,
		science.IncBeta(RetireRateFloor, s.Alpha, s.Beta), RetireConfidence)
	return s, payload, true
}

// RenderIndex is the spec's progressive-disclosure view: ~100 chars per
// skill — title, posterior mean and verification, nothing else.
func RenderIndex(recs []WikiSkillRecord) string {
	var b strings.Builder
	for _, s := range recs {
		if s.Retired {
			continue
		}
		line := fmt.Sprintf("- %s (mean %.2f, n=%d): %s", s.Key, s.PosteriorMean(), s.Observations, s.Verification)
		if len(line) > 120 {
			line = line[:117] + "..."
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}
