package cognitive

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ace/internal/bus"
	"ace/internal/memory"
	"ace/internal/sandbox"
	"ace/internal/telemetry"
	"github.com/stretchr/testify/require"
)

func TestV06ConnectomeSpecSensoriumAndStuckDetector(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	jail, err := sandbox.NewJail(t.TempDir())
	require.NoError(t, err)
	defer jail.Close()

	wsDir := jail.WorkspaceDir()
	_ = os.MkdirAll(filepath.Join(wsDir, ".harness_d", "sensors"), 0o755)
	_ = os.MkdirAll(filepath.Join(wsDir, ".harness_d", "reality_replica"), 0o755)
	_ = os.WriteFile(filepath.Join(wsDir, ".harness_d", "sensors", "uv_air_lab.json"), []byte(`{"summary":"uv_index=3.2 aqi=18 temp_ir=22.4C"}`), 0o644)
	_ = os.WriteFile(filepath.Join(wsDir, ".harness_d", "reality_replica", "arm_sim_v1.json"), []byte(`{"joint_pos":[0.1,0.4,-0.2],"tactile_contact_n":4.8,"gps_fix":"3D"}`), 0o644)
	_ = os.WriteFile(filepath.Join(wsDir, "router_eval.py"), []byte("import json\ndef evaluate_router():\n    return {'h1_accepted': True}\n"), 0o644)

	telem, err := telemetry.NewSink(db)
	require.NoError(t, err)

	memBus, err := bus.NewMemoryBus(db, telem)
	require.NoError(t, err)

	memStore, err := memory.NewStore(db, jail.WikiRoot(), jail.WikiDir(), telem)
	require.NoError(t, err)

	fp := &fakeProvider{active: "test-model"}
	eng := NewEngine(fp, memBus, memStore, jail, telem)
	ctx := context.Background()

	// 1. Verify Harness-Minted EvidenceRef rule (§5.1)
	require.True(t, IsHarnessMintedEvidenceRef("cyc_123456"))
	require.True(t, IsHarnessMintedEvidenceRef("span_abcdef"))
	require.False(t, IsHarnessMintedEvidenceRef("i_tested_it_trust_me"))

	err = eng.UpsertUncertaintyNode(ctx, ConnectomeNode{
		ID:          "uncertainty:test_1",
		Kind:        "UNCERTAINTY",
		State:       "CLOSED",
		EvidenceRef: "fake_ref",
	})
	require.Error(t, err)

	// 2. Verify Living Spec 2,800B budget + Frozen SHA-256(§1 || §5) (§4)
	milestone := "- [ ] **Milestone 7.1 — Connectome & Router Verification**: Implement `/workspace/router_eval.py` and verify."
	specID, specText := eng.EnsureLivingSpecForMilestone(ctx, milestone)
	require.NotEmpty(t, specID)
	require.LessOrEqual(t, len(specText), 2800)

	tampered := strings.Replace(specText, "Every target script in Section 3", "Always pass immediately", 1)
	_, err = eng.ParseAndValidateLivingSpec(ctx, specID, tampered)
	require.Error(t, err)

	// 3. Verify Connectome Refresh + RRF Slice (§2, §3)
	sliceText, preloaded := eng.BuildConnectomeSlice(ctx, "cyc_test01", milestone, "verify router_eval.py", milestone, specText)
	require.Contains(t, sliceText, "router_eval.py")
	require.True(t, preloaded["/workspace/router_eval.py"])

	// 4. Verify Stuck Detector: 3 vetoes only forces local L4 strategy reflection (false), 5 vetoes escalates (§6.1)
	_, _, jsonPath := ComputeVetoReasonHash(milestone, "missing artifact /workspace/results/task_decomposer_eval.json")
	require.Equal(t, "/workspace/results/task_decomposer_eval.json", jsonPath)

	for i := 1; i <= 5; i++ {
		stuck, count := eng.RecordGateVetoAndCheckStuck(ctx, "cyc_test01", milestone, "missing artifact /workspace/missing_file.py", "I already created it")
		require.Equal(t, i, count)
		if i < 5 {
			require.False(t, stuck)
		} else {
			require.True(t, stuck)
		}
	}

	// 4b. Verify Native Go Wiki-Skill Distillation & Beta(alpha, beta) Lifecycle Engine
	eng.DistillAndRegisterWikiSkill(ctx, "cyc_test01", milestone, []string{"/workspace/router_eval.py"}, nil)
	routed, ok := eng.RouteTopWikiSkill(ctx, milestone)
	require.True(t, ok)
	require.Greater(t, routed.PosteriorMean(), 0.70)

	// 5. Verify Pluggable Sensorium (Internal Proprioceptive + External Exteroceptive + Reality Replica + Circadian) (§7)
	eng.RecordHumanInteractionEvent(ctx, "telegram", time.Now().UTC(), 14.5)
	snap := eng.CollectSensoriumSnapshot(ctx)
	require.Greater(t, snap.HumanProbActiveNow, 0.0)
	require.Less(t, snap.HumanProbActiveNow, 1.0)

	promptLine := eng.FormatSensoriumPromptLine(ctx)
	require.Contains(t, promptLine, "Proprio:")
	require.Contains(t, promptLine, "Extero:")
	require.Contains(t, promptLine, "uv_air_lab")
	require.Contains(t, promptLine, "arm_sim_v1")
}
