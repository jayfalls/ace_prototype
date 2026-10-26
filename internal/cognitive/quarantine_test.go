package cognitive

import (
	"context"
	"database/sql"
	"testing"

	"ace/internal/bus"
	"ace/internal/memory"
	"ace/internal/sandbox"
	"ace/internal/telemetry"
	"github.com/stretchr/testify/require"
)

// Owner rule (v0.9 acceptance): quarantine is a real DAG state — ruled
// disputes get at most 3 more identical vetoes, then the node parks and
// every selector skips it.
func TestVetoRulingParksDAGNode(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	jail, err := sandbox.NewJail(t.TempDir())
	require.NoError(t, err)
	defer jail.Close()

	telem, err := telemetry.NewSink(db)
	require.NoError(t, err)
	memBus, err := bus.NewMemoryBus(db, telem)
	require.NoError(t, err)
	memStore, err := memory.NewStore(db, jail.WikiRoot(), jail.WikiDir(), telem)
	require.NoError(t, err)

	eng := NewEngine(&fakeProvider{active: "test-model"}, memBus, memStore, jail, telem)
	ctx := context.Background()

	milestone := "- [ ] **T_Q_TEST — Task Specification**: Build `/workspace/modules/q_test.py` with --selftest."
	eng.upsertMissionDAGNode(ctx, MissionDAGNode{
		ID: "T_Q_TEST", ParentID: "C_Q_TEST", Stage: "L4_TASK_SPEC",
		State: "READY_FOR_L5", OwnerLayer: 4, Title: "q test",
		ArtifactPath: "/workspace/modules/q_test.py", SoulHash: "s",
	})
	// A sibling that must keep running.
	eng.upsertMissionDAGNode(ctx, MissionDAGNode{
		ID: "T_Q_OTHER", ParentID: "C_Q_OTHER", Stage: "L4_TASK_SPEC",
		State: "READY_FOR_L5", OwnerLayer: 4, Title: "other",
		ArtifactPath: "/workspace/modules/q_other.py", SoulHash: "s",
	})

	// Seed a ruled dispute: count=2 at ruling time.
	vhash, vkind, tpath := ComputeVetoReasonHash(milestone, "missing artifact /workspace/x.py")
	_, err = db.Exec(`INSERT INTO veto_ledger (veto_hash, milestone_id, veto_kind, target_path, count, last_reason, last_cycle_id, disputed, ruled_count, updated_at)
		VALUES (?,?,?,?,2,'x','cyc_0',1,2,'2026-01-01')`, vhash, milestone, vkind, tpath)
	require.NoError(t, err)

	// Vetoes +1, +2 below the bound: still open.
	for i := 0; i < 2; i++ {
		eng.RecordGateVetoAndCheckStuck(ctx, "cyc_q", milestone, "missing artifact /workspace/x.py", "retrying")
	}
	n := dagNodeByID(t, db, "T_Q_TEST")
	require.Equal(t, "READY_FOR_L5", n)

	// The 3rd identical veto after the ruling parks the node.
	eng.RecordGateVetoAndCheckStuck(ctx, "cyc_q", milestone, "missing artifact /workspace/x.py", "still failing")
	require.Equal(t, "QUARANTINED", dagNodeByID(t, db, "T_Q_TEST"))
	require.Equal(t, "READY_FOR_L5", dagNodeByID(t, db, "T_Q_OTHER"))

	// Legacy overflow: disputed long before ruled_count existed parks on sight.
	milestone2 := "- [ ] **T_Q_LEGACY — Task Specification**: Build `/workspace/modules/q_legacy.py`."
	eng.upsertMissionDAGNode(ctx, MissionDAGNode{
		ID: "T_Q_LEGACY", ParentID: "C_Q_LEGACY", Stage: "L4_TASK_SPEC",
		State: "READY_FOR_L5", OwnerLayer: 4, Title: "legacy",
		ArtifactPath: "/workspace/modules/q_legacy.py", SoulHash: "s",
	})
	vhash2, vkind2, tpath2 := ComputeVetoReasonHash(milestone2, "hygiene violation")
	_, err = db.Exec(`INSERT INTO veto_ledger (veto_hash, milestone_id, veto_kind, target_path, count, last_reason, last_cycle_id, disputed, ruled_count, updated_at)
		VALUES (?,?,?,?,817,'x','cyc_0',1,0,'2026-01-01')`, vhash2, milestone2, vkind2, tpath2)
	require.NoError(t, err)
	eng.RecordGateVetoAndCheckStuck(ctx, "cyc_q", milestone2, "hygiene violation", "")
	require.Equal(t, "QUARANTINED", dagNodeByID(t, db, "T_Q_LEGACY"))

	// Shared selection never returns a quarantined node — the orphaned
	// test chain fails provenance anyway, but QUARANTINED must not be it.
	for i := 0; i < 4; i++ {
		active, ok := eng.ActiveReadyTaskNode(ctx)
		if ok {
			require.NotEqual(t, "T_Q_TEST", active.ID)
			require.NotEqual(t, "T_Q_LEGACY", active.ID)
			require.Equal(t, "READY_FOR_L5", active.State)
		}
	}
}

func dagNodeByID(t *testing.T, db *sql.DB, id string) string {
	var state string
	require.NoError(t, db.QueryRow(`SELECT state FROM mission_dag_nodes WHERE id=?`, id).Scan(&state))
	return state
}


