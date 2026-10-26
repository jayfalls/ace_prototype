package cognitive

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"ace/internal/core"
	"ace/internal/sandbox"
	"github.com/stretchr/testify/require"
)

func TestCordisLTLAndMode3Gate(t *testing.T) {
	tmp := t.TempDir()
	jail, err := sandbox.NewJail(tmp)
	require.NoError(t, err)
	defer jail.Close()

	// 1. Test CordisFiber LIFO Snapshot & RevertAll
	fiber := NewCordisFiber(jail.WorkspaceDir())
	require.NoError(t, jail.WriteFile("exp.py", "def good():\n    return 1\n"))
	fiber.SnapshotFile("exp.py")
	require.NoError(t, jail.WriteFile("exp.py", "def broken(:\n"))
	require.True(t, fiber.HasEmpiricalDiff())
	reverted := fiber.RevertAll()
	require.Contains(t, reverted, "exp.py")
	restored, err := jail.ReadFile("exp.py")
	require.NoError(t, err)
	require.Equal(t, "def good():\n    return 1\n", restored)

	// 2. Test LTL Monitor G1 (offline pip install ban) and G2 (cross-cycle failed command ban)
	pkt := CrossCycleHandoffPacket{ActiveMilestone: "Milestone 1.2"}
	ok, reason := EvaluateLTLAction(core.LayerAction{
		Type:    "sandbox_sh",
		Content: "pip3 install numpy scipy",
	}, pkt, nil)
	require.False(t, ok)
	require.Contains(t, reason, "LTL VETO G1")

	RecordHandoffAttempt(&pkt, "cyc_1", core.LayerAction{
		Type:    "sandbox_sh",
		Content: "python3 /workspace/v1_forensics.py --selftest",
	}, 1, "ModuleNotFoundError: No module named 'numpy'", false, nil, nil)

	ok2, reason2 := EvaluateLTLAction(core.LayerAction{
		Type:    "sandbox_sh",
		Content: "python3 /workspace/v1_forensics.py --selftest",
	}, pkt, nil)
	require.False(t, ok2)
	require.Contains(t, reason2, "LTL VETO G2")

	// 3. Test Mode-3 Gate rejects missing artifacts, vacuous tests, and unused imports, then passes valid code
	// Also verify wildcard globs like `/workspace/v03_*.py` and SDK `workspace_audit.py` are ignored as literal required outputs
	msLine := "- [ ] **Milestone 1.2**: Build `/workspace/demo_forensics.py` (with `--selftest` using `/workspace/workspace_audit.py` over `/workspace/v03_*.py`) and `/workspace/demo_report.json` and `notes/demo-findings.md`."
	wsArt, wikiArt := ExtractMilestoneArtifacts(msLine)
	require.Equal(t, []string{"demo_forensics.py", "demo_report.json"}, wsArt)
	require.Equal(t, []string{"notes/demo-findings.md"}, wikiArt)
	require.Equal(t, "demo_forensics.py", ExtractPrimaryMilestoneScript(msLine))
	require.Equal(t, "milestone 1.2", extractMilestoneIdentifier(msLine))

	res1 := EvaluateMode3MilestoneGate(context.Background(), msLine, jail, jail.WorkspaceDir(), jail.WikiDir(), nil, 3)
	require.False(t, res1.Passed)
	require.NotEmpty(t, res1.MissingArtifacts)

	// 3a. Verify Mode-3 Gate rejects unused imports or tautological asserts
	dirtyPy := `import sys, json
def compute():
    return {"count": 42}
if __name__ == "__main__":
    r = compute()
    if "--selftest" in sys.argv:
        assert r["count"] == 42
        print("SELFTEST OK")
`
	require.NoError(t, jail.WriteFile("demo_forensics.py", dirtyPy))
	require.NoError(t, jail.WriteFile("demo_report.json", `{"count": 42}`))
	require.NoError(t, os.MkdirAll(filepath.Join(jail.WikiDir(), "notes"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(jail.WikiDir(), "notes", "demo-findings.md"), []byte("# Demo Findings\n\nEmpirical evaluation with p < 0.001 and Beta(10,1) 95% CI [0.75, 0.99] confirming hypothesis H1 over H0 baseline.\n"), 0o644))

	resDirty := EvaluateMode3MilestoneGate(context.Background(), msLine, jail, jail.WorkspaceDir(), jail.WikiDir(), nil, 3)
	require.False(t, resDirty.Passed, "Mode-3 gate must reject unused import json")
	require.Contains(t, resDirty.Reason, "unused_imports")

	// 3b. Write clean, non-vacuous artifact that uses its imports and kills mutants, then verify Mode-3 Gate passes
	validPy := `import sys, json
def compute(x):
    return {"count": x + 2}
if __name__ == "__main__":
    r = compute(40)
    if "--selftest" in sys.argv:
        assert r["count"] == 42
        print(json.dumps(r))
`
	require.NoError(t, jail.WriteFile("demo_forensics.py", validPy))
	res2 := EvaluateMode3MilestoneGate(context.Background(), msLine, jail, jail.WorkspaceDir(), jail.WikiDir(), nil, 3)
	require.True(t, res2.Passed, "Mode-3 gate should pass valid artifacts, got reason: %s", res2.Reason)

	// 4. Test Mode-3 Dataset Provenance/Split Gate and Scientific Falsification Gate (rejects h1_accepted=false)
	require.NoError(t, jail.WriteFile("demo_report.json", `{"count": 42, "h1_accepted": false}`))
	resFalsified := EvaluateMode3MilestoneGate(context.Background(), msLine, jail, jail.WorkspaceDir(), jail.WikiDir(), nil, 3)
	require.False(t, resFalsified.Passed, "Mode-3 gate must reject checkoff when h1_accepted is false")
	require.Contains(t, resFalsified.Reason, "SCIENTIFIC FALSIFICATION")

	require.NoError(t, jail.WriteFile("demo_report.json", `{"count": 42, "h1_accepted": true}`))
	resAccepted := EvaluateMode3MilestoneGate(context.Background(), msLine, jail, jail.WorkspaceDir(), jail.WikiDir(), nil, 3)
	require.True(t, resAccepted.Passed)

	// 5. Verify domain-agnostic plain-English user milestone (without explicit /workspace/ paths in text)
	// validates via verifiedFiles created during the milestone!
	plainGoalLine := "- [ ] Build and benchmark an anomaly detector on real data"
	resPlainEmpty := EvaluateMode3MilestoneGate(context.Background(), plainGoalLine, jail, jail.WorkspaceDir(), jail.WikiDir(), nil, 3)
	require.False(t, resPlainEmpty.Passed)
	resPlainVerified := EvaluateMode3MilestoneGate(context.Background(), plainGoalLine, jail, jail.WorkspaceDir(), jail.WikiDir(), []string{"demo_forensics.py", "demo_report.json"}, 3)
	require.True(t, resPlainVerified.Passed, "Plain-English user goal should pass Mode-3 when verifiedFiles pass --selftest and JSON gates: %s", resPlainVerified.Reason)
}
