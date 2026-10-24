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
	"ace/internal/core"
	"ace/internal/memory"
	"ace/internal/sandbox"
	"ace/internal/telemetry"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func newRegressionEngine(t *testing.T) (*Engine, *sandbox.Jail, *memory.Store, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	jail, err := sandbox.NewJail(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { jail.Close() })

	telem, err := telemetry.NewSink(db)
	require.NoError(t, err)
	memBus, err := bus.NewMemoryBus(db, telem)
	require.NoError(t, err)
	memStore, err := memory.NewStore(db, jail.WikiRoot(), jail.WikiDir(), telem)
	require.NoError(t, err)

	eng := NewEngine(&fakeProvider{active: "test-model"}, memBus, memStore, jail, telem)
	return eng, jail, memStore, db
}

// seedMission replaces the `*(none ...)*` placeholder under # CURRENT MISSION,
// mirroring what `ace init` does — there is exactly one heading in soul.md.
func seedMission(t *testing.T, memStore *memory.Store, mission string) {
	t.Helper()
	soul, err := memStore.ReadWiki("soul.md")
	require.NoError(t, err)
	lines := strings.Split(soul, "\n")
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "# CURRENT MISSION") && i+1 < len(lines) {
			lines[i+1] = mission
			break
		}
	}
	require.NoError(t, memStore.WriteWiki("soul.md", strings.Join(lines, "\n")))
}

// Bug: v0.7 hardcoded a fallback mission, so the DAG could never be idle.
// Fix: no mission in soul.md means no L1 genesis and zero traces.
func TestDAG_IdlesWithoutMission(t *testing.T) {
	eng, _, _, _ := newRegressionEngine(t)
	ctx := context.Background()

	require.Empty(t, eng.extractCurrentMissionFromSoul(), "seeded placeholder must not parse as a mission")
	traces := eng.StepMissionDAG(ctx, "cyc-1", "span-1", "tick")
	require.Empty(t, traces)
}

// Bug: DAG horizon stalled — completed waves never triggered a new L2 branch.
// Fix: with all spawned branches complete and no open L4 work, L2 must expand the horizon.
func TestDAG_HorizonExpandsAfterWaveCompletes(t *testing.T) {
	eng, _, memStore, _ := newRegressionEngine(t)
	ctx := context.Background()
	seedMission(t, memStore, "Verify the office plant soil stays moist.")
	soulHash := eng.currentSoulHash()

	eng.upsertMissionDAGNode(ctx, MissionDAGNode{ID: "M1_ROOT", Stage: "L1_MISSION_ROOT", State: "L1_ANCHORED", OwnerLayer: 1, Title: "root", Directive: "d", DirectiveHash: "h1", SoulHash: soulHash})
	eng.upsertMissionDAGNode(ctx, MissionDAGNode{ID: "L2_a", ParentID: "M1_ROOT", Stage: "L2_STRATEGY", State: "COMPLETED", OwnerLayer: 2, Title: "a", Directive: "d", DirectiveHash: "h2"})
	eng.upsertMissionDAGNode(ctx, MissionDAGNode{ID: "L2_b", ParentID: "M1_ROOT", Stage: "L2_STRATEGY", State: "COMPLETED", OwnerLayer: 2, Title: "b", Directive: "d", DirectiveHash: "h3"})

	before := 0
	for _, n := range eng.LoadMissionDAGNodes(ctx) {
		if n.Stage == "L2_STRATEGY" {
			before++
		}
	}
	eng.StepMissionDAG(ctx, "cyc-2", "span-2", "tick")
	after := 0
	for _, n := range eng.LoadMissionDAGNodes(ctx) {
		if n.Stage == "L2_STRATEGY" {
			after++
		}
	}
	require.Greater(t, after, before, "exhausted L2 horizon must trigger a new L2 strategy wave")
}

// Bug: L4 task specs pointed at hardcoded paths that did not exist in the workspace.
// Fix: extractDynamicL4SpecPaths binds the module path the directive actually names.
func TestL4SpecPaths_BindFromDirective(t *testing.T) {
	eng, _, memStore, _ := newRegressionEngine(t)
	ctx := context.Background()
	seedMission(t, memStore, "Keep the garage freezer below -18C.")

	dir := "Build and run /workspace/modules/freezer_monitor.py against a dataset."
	mod, ds, res := eng.extractDynamicL4SpecPaths(ctx, dir)
	require.Equal(t, "/workspace/modules/freezer_monitor.py", mod)
	require.Equal(t, "/workspace/datasets/freezer_monitor_v1.json", ds)
	require.Equal(t, "/workspace/results/freezer_monitor.json", res)

	// A module already claimed by the DAG is never rebound to a second task.
	eng.upsertMissionDAGNode(ctx, MissionDAGNode{ID: "t1", ParentID: "x", Stage: "L4_TASK_SPEC", State: "READY_FOR_L5", OwnerLayer: 4, Title: "t", Directive: "d", ArtifactPath: "/workspace/modules/freezer_monitor.py", DirectiveHash: "h"})
	mod2, _, _ := eng.extractDynamicL4SpecPaths(ctx, dir)
	require.NotEqual(t, "/workspace/modules/freezer_monitor.py", mod2)
}

// Bug: goals.md was a stale append-projection of the DAG.
// Fix: SyncMissionDAGToGoalsWiki rewrites goals.md from the live node table.
func TestGoalsProjection_ReflectsCurrentTree(t *testing.T) {
	eng, _, memStore, _ := newRegressionEngine(t)
	ctx := context.Background()

	eng.upsertMissionDAGNode(ctx, MissionDAGNode{ID: "M1_ROOT", Stage: "L1_MISSION_ROOT", State: "L1_ANCHORED", OwnerLayer: 1, Title: "t", Directive: "original mission frame", DirectiveHash: "h1"})
	eng.SyncMissionDAGToGoalsWiki(ctx)
	g1, err := memStore.ReadWiki("goals.md")
	require.NoError(t, err)
	require.Contains(t, g1, "original mission frame")

	eng.upsertMissionDAGNode(ctx, MissionDAGNode{ID: "M1_ROOT", Stage: "L1_MISSION_ROOT", State: "L1_ANCHORED", OwnerLayer: 1, Title: "t", Directive: "revised mission frame", DirectiveHash: "h1"})
	eng.SyncMissionDAGToGoalsWiki(ctx)
	g2, err := memStore.ReadWiki("goals.md")
	require.NoError(t, err)
	require.Contains(t, g2, "revised mission frame")
	require.NotContains(t, g2, "original mission frame", "stale projection must be overwritten, not appended")
}

// Bug: milestone artifact lines naming both "x.py" and "pkg_x.py" resolved to nothing.
// Fix: ResolveWorkspaceArtifactPath falls back to canonical subdirs by basename.
func TestArtifactPath_DualFilenameResolution(t *testing.T) {
	eng, jail, _, _ := newRegressionEngine(t)
	_ = eng
	require.NoError(t, jail.WriteFile("modules/metric_probe.py", "x = 1\n"))

	resolved := ResolveWorkspaceArtifactPath(jail.WorkspaceDir(), "metric_probe.py")
	require.Equal(t, "modules/metric_probe.py", resolved)

	resolvedBare := ResolveWorkspaceArtifactPath(jail.WorkspaceDir(), "modules/metric_probe.py")
	require.Equal(t, "modules/metric_probe.py", resolvedBare)
}

// Bug: "file.md#grep=pattern" fragments were passed to openat and failed.
// Fix: workspace_read parses #grep= fragments and runs the grep instead.
func TestWorkspaceRead_GrepFragment(t *testing.T) {
	eng, jail, _, _ := newRegressionEngine(t)
	require.NoError(t, jail.WriteFile("modules/scan.py", "def alpha():\n    return 1\n\ndef beta():\n    return 2\n"))

	res := eng.executeLayerAction(context.Background(), "cyc-3", core.LayerTaskProsecution, core.LayerAction{
		Type:   "workspace_read",
		Target: "modules/scan.py#grep=^def",
	}, "test-model")
	require.Contains(t, res, "def alpha")
	require.Contains(t, res, "def beta")
	require.NotContains(t, res, "workspace_read error")
}

// Bug: reading a directory path crashed as a file read.
// Fix: workspace_read returns a directory listing instead.
func TestWorkspaceRead_DirectoryListing(t *testing.T) {
	eng, jail, _, _ := newRegressionEngine(t)
	require.NoError(t, jail.WriteFile("modules/probe_a.py", "x = 1\n"))

	res := eng.executeLayerAction(context.Background(), "cyc-4", core.LayerTaskProsecution, core.LayerAction{
		Type:   "workspace_read",
		Target: "modules",
	}, "test-model")
	require.Contains(t, res, "Directory")
	require.Contains(t, res, "probe_a.py")
	require.NotContains(t, res, "is a directory")
}

// Bug: a multi-path action_target ("a.py;b.sh") was treated as one filename.
// Fix: splitScriptTargets normalizes, dedupes and verifies each script independently.
func TestSplitScriptTargets_MultiPath(t *testing.T) {
	got := splitScriptTargets("/workspace/modules/a.py; modules/b.sh ; notes/c.md; modules/a.py")
	require.Equal(t, []string{"modules/a.py", "modules/b.sh"}, got)
	require.Empty(t, splitScriptTargets("notes/only.md"))
}

// Bug: CONTRADICTION connectome nodes were never cleared, blocking the prosecutor forever.
// Fix: milestone verification resolves every open CONTRADICTION node.
func TestContradictionNodes_ResolvedOnVerify(t *testing.T) {
	eng, _, _, db := newRegressionEngine(t)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `INSERT INTO connectome_nodes (id, kind, path, summary, state, owner_layer, blocking_weight, info_value, expected_cost, updated_at)
		VALUES ('c1', 'CONTRADICTION', 'x', 'conflict', 'OPEN', 'L3', 1.5, 1.0, 0.3, '2026-01-01')`)
	require.NoError(t, err)

	eng.resolveContradictionNodes(ctx)

	var state string
	var weight float64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT state, blocking_weight FROM connectome_nodes WHERE id='c1'`).Scan(&state, &weight))
	require.Equal(t, "RESOLVED", state)
	require.Equal(t, 0.0, weight)
}

// Bug: the live trace DB was mirrored into the workspace on every tick.
// Fix: an idle cascade (nothing ran, no L6 traces) returns before the mirror call.
func TestCascade_IdleTickDoesNotMirrorTraceDB(t *testing.T) {
	eng, jail, _, _ := newRegressionEngine(t)

	eng.runCascade(context.Background(), cycleJob{CycleID: "cyc-5", Trigger: "idle", StartLayer: core.LayerExecutive, MaxHops: 4})

	mirror := filepath.Join(jail.WorkspaceDir(), "datasets", "live_traces.db")
	_, err := os.Stat(mirror)
	require.True(t, os.IsNotExist(err), "idle tick must not copy the trace DB into the workspace")
}

// Bug: L5 gate failure still dispatched L6 (fail-open, pYes defaulted high, no veto path).
// Fix: gate error leaves pYes at 0 and the L4 task is NOT dispatched.
func TestL5Gate_DownBlocksL6Dispatch(t *testing.T) {
	eng, _, memStore, _ := newRegressionEngine(t)
	ctx := context.Background()
	seedMission(t, memStore, "Verify the attic humidity stays under 60%.")
	soulHash := eng.currentSoulHash()

	eng.upsertMissionDAGNode(ctx, MissionDAGNode{ID: "M1_ROOT", Stage: "L1_MISSION_ROOT", State: "L1_ANCHORED", OwnerLayer: 1, Title: "root", Directive: "d", DirectiveHash: "h1", SoulHash: soulHash})
	eng.upsertMissionDAGNode(ctx, MissionDAGNode{ID: "L2_a", ParentID: "M1_ROOT", Stage: "L2_STRATEGY", State: "COMPLETED", OwnerLayer: 2, Title: "a", Directive: "d", DirectiveHash: "h2"})
	eng.upsertMissionDAGNode(ctx, MissionDAGNode{ID: "L3_a", ParentID: "L2_a", Stage: "L3_CAPABILITY", State: "COMPLETED", OwnerLayer: 3, Title: "a", Directive: "d", DirectiveHash: "h3"})
	eng.upsertMissionDAGNode(ctx, MissionDAGNode{ID: "L4_a", ParentID: "L3_a", Stage: "L4_TASK_SPEC", State: "READY_FOR_L5", OwnerLayer: 4, Title: "run probe", Directive: "run /workspace/modules/probe.py", ArtifactPath: "/workspace/modules/probe.py", DirectiveHash: "h4"})

	// Nothing is listening on the L5 gate port: evaluateL5Gate must error and dispatch must not happen.
	traces := eng.StepMissionDAG(ctx, "cyc-6", "span-6", "tick")
	require.Empty(t, traces, "L5 gate unreachable must veto L6 dispatch")

	task, ok := eng.ActiveReadyTaskNode(ctx)
	require.True(t, ok, "vetoed task must remain pending, not consumed")
	require.Equal(t, "L4_a", task.ID)
}

// Plan §5 retention caps: traces/telemetry older than 30 days are deleted,
// and closed-task workpad dirs untouched for 7 days are removed.
func TestRetentionSweep_DeletesOldTracesAndWorkpads(t *testing.T) {
	eng, jail, _, db := newRegressionEngine(t)
	ctx := context.Background()

	old := time.Now().UTC().Add(-31 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	fresh := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := db.ExecContext(ctx, `INSERT INTO thought_traces
		(id, cycle_id, agent_id, layer_id, layer_code, loop_kind, model, trigger_summary, system_prompt, user_prompt, raw_reasoning, raw_response, thought, state_summary, southbound, northbound, action_type, action_target, action_result, confidence, latency_ms, prompt_tokens, completion_tokens, created_at)
		VALUES ('old-trace','c','a',6,'L6','x','m','s','p','u','r','r','t','s','sb','nb','none','','ok',0.5,1,1,1,?)`, old)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO thought_traces
		(id, cycle_id, agent_id, layer_id, layer_code, loop_kind, model, trigger_summary, system_prompt, user_prompt, raw_reasoning, raw_response, thought, state_summary, southbound, northbound, action_type, action_target, action_result, confidence, latency_ms, prompt_tokens, completion_tokens, created_at)
		VALUES ('fresh-trace','c','a',6,'L6','x','m','s','p','u','r','r','t','s','sb','nb','none','','ok',0.5,1,1,1,?)`, fresh)
	require.NoError(t, err)

	wpRoot := filepath.Join(filepath.Dir(jail.WorkspaceDir()), "workpads")
	oldWP := filepath.Join(wpRoot, "task_old")
	newWP := filepath.Join(wpRoot, "task_new")
	require.NoError(t, os.MkdirAll(oldWP, 0o755))
	require.NoError(t, os.MkdirAll(newWP, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(oldWP, "x.py"), []byte("x=1"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(newWP, "y.py"), []byte("y=1"), 0o644))
	oldTime := time.Now().Add(-8 * 24 * time.Hour)
	require.NoError(t, os.Chtimes(oldWP, oldTime, oldTime))

	eng.maybeRunRetentionSweep(ctx)

	var cnt int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM thought_traces WHERE id='old-trace'`).Scan(&cnt))
	require.Equal(t, 0, cnt, "trace older than 30 days must be deleted")
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM thought_traces WHERE id='fresh-trace'`).Scan(&cnt))
	require.Equal(t, 1, cnt, "fresh trace must be retained")

	_, err = os.Stat(oldWP)
	require.True(t, os.IsNotExist(err), "workpad older than 7 days must be removed")
	_, err = os.Stat(newWP)
	require.NoError(t, err, "recent workpad must be retained")

	last, err := eng.memory.LoadEngineMeta(ctx, "last_retention_sweep")
	require.NoError(t, err)
	require.NotEmpty(t, last)
}
