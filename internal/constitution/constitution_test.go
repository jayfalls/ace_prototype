package constitution

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The pin is the build-time guard: any constitution.md edit without a
// PinnedSHA256 bump in the same commit fails here and at startup Check().
func TestPinnedSHA256MatchesEmbed(t *testing.T) {
	require.NoError(t, Check())
}

func TestConstitutionContents(t *testing.T) {
	for _, section := range []string{
		"Heuristic Imperatives", "Reduce suffering", "Increase prosperity",
		"Increase understanding", "Universal Declaration of Human Rights",
		"Autonomy Triad", "Epistemic Law",
	} {
		require.Contains(t, Text(), section)
	}
	require.True(t, strings.HasPrefix(strings.TrimSpace(Text()), "# ACE Constitution"))
}
