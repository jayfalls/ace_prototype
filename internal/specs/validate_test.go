package specs

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Spec test: a living spec over the 2,800-byte cap is rejected.
func TestSpecOverCapRejected(t *testing.T) {
	over := strings.Repeat("x", 2801)
	require.Error(t, ValidateSpec(over, 0))
	require.NoError(t, ValidateSpec(strings.Repeat("x", 2800), 0))
}

// Spec test: output touching an out-of-scope path is rejected.
func TestOutOfScopeOutputRejected(t *testing.T) {
	spec := `# Living spec
## In scope
- internal/skills/

## Out of scope
- internal/provider/registry.go
- cmd/ace/main.go

## Nuances
- none`
	err := CheckOutOfScope(spec, []string{"internal/skills/skills.go"})
	require.NoError(t, err)
	err = CheckOutOfScope(spec, []string{"internal/provider/registry.go"})
	require.Error(t, err, "declared out-of-scope path must be rejected")
	err = CheckOutOfScope(spec, []string{"cmd/ace/main.go"})
	require.Error(t, err)
}

// Consolidate merges sections by header and stays under the cap.
func TestConsolidateMergesNotAppends(t *testing.T) {
	spec := "## In scope\n- old item\n\n## Nuances\n- keep me"
	update := "## In scope\n- new item"
	out, err := Consolidate(spec, update, 0)
	require.NoError(t, err)
	require.Contains(t, out, "new item")
	require.NotContains(t, out, "old item")
	require.Contains(t, out, "keep me")
	require.NoError(t, ValidateSpec(out, 0))
}
