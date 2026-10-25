package skills

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Spec test: a skill with 6 uses and 1 success — P(rate<0.5)>0.9 — is
// retired and produces the concept-registry registration payload.
func TestRetireSixUseOneSuccess(t *testing.T) {
	s := WikiSkillRecord{
		Key: "flaky-skill", Alpha: 2, Beta: 6, // 1 success + prior1, 5 fails + prior1
		Observations: 6, Verification: "tested",
	}
	require.True(t, ShouldRetire(s))
	got, payload, ok := Retire(context.Background(), s)
	require.True(t, ok)
	require.True(t, got.Retired)
	require.Contains(t, payload, "flaky-skill")
	require.Contains(t, payload, "retired")
}

// Spec test: 6 uses / 5 successes — posterior well above the floor — is
// retained.
func TestRetainSixUseFiveSuccess(t *testing.T) {
	s := WikiSkillRecord{
		Key: "good-skill", Alpha: 6, Beta: 2, // 5 successes + prior1, 1 fail + prior1
		Observations: 6,
	}
	require.False(t, ShouldRetire(s))
	_, _, ok := Retire(context.Background(), s)
	require.False(t, ok)
}

// Under 5 uses the skill is never retired regardless of record.
func TestRetireNeedsMinUses(t *testing.T) {
	s := WikiSkillRecord{Key: "young", Alpha: 1, Beta: 4, Observations: 3}
	require.False(t, ShouldRetire(s))
}

// RenderIndex honours the ~100-char progressive-disclosure view.
func TestRenderIndexCompact(t *testing.T) {
	idx := WikiSkillLifecycleIndex{Skills: []WikiSkillRecord{
		{Key: "a", Alpha: 6, Beta: 2, Observations: 6, Verification: "v"},
		{Key: "retired", Alpha: 2, Beta: 6, Observations: 6, Retired: true},
	}}
	out := RenderIndex(idx.Skills)
	require.Contains(t, out, "a")
	require.NotContains(t, out, "retired")
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		require.LessOrEqual(t, len(line), 120)
	}
}
