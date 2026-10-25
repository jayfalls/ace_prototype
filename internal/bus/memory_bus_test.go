package bus

import (
	"context"
	"database/sql"
	"testing"

	"ace/internal/core"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestMemoryBusPublishSubscribeRecent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	b, err := NewMemoryBus(db, nil)
	require.NoError(t, err)

	var received []core.BusEnvelope
	unsub := b.Subscribe(func(env core.BusEnvelope) {
		received = append(received, env)
	})
	defer unsub()

	err = b.Publish(context.Background(), core.BusEnvelope{
		CycleID:   "cyc_1",
		AgentID:   "ace-primary",
		Topic:     InputTopic("ace-primary", 2),
		Direction: core.DirectionSouthbound,
		FromLayer: 1,
		ToLayer:   2,
		Kind:      "directive",
		Summary:   "Align strategy with user intent",
		Payload:   "Assist user with autonomous cognitive research",
	})
	require.NoError(t, err)
	require.Len(t, received, 1)

	recent, err := b.Recent(10)
	require.NoError(t, err)
	require.Len(t, recent, 1)
	require.Equal(t, "Align strategy with user intent", recent[0].Summary)
}

// Spec 9.3 validation contract: L2->L5 publish, empty summary and malformed
// topics are all rejected and recorded in bus_rejects.
func TestPublishValidationRejects(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	b, err := NewMemoryBus(db, nil)
	require.NoError(t, err)

	base := core.BusEnvelope{
		CycleID: "cyc_1", AgentID: "ace-primary",
		Direction: core.DirectionSouthbound,
		Kind:      "directive", Summary: "s", Payload: "p",
	}

	// L2 -> L5: non-adjacent hop.
	bad := base
	bad.Topic, bad.FromLayer, bad.ToLayer = InputTopic("ace-primary", 5), 2, 5
	require.Error(t, b.Publish(context.Background(), bad))

	// Empty summary.
	bad = base
	bad.Topic, bad.FromLayer, bad.ToLayer = InputTopic("ace-primary", 2), 1, 2
	bad.Summary = ""
	require.Error(t, b.Publish(context.Background(), bad))

	// Malformed topic.
	bad = base
	bad.Topic, bad.FromLayer, bad.ToLayer = "ace.bus.south.L1->L2", 1, 2
	require.Error(t, b.Publish(context.Background(), bad))

	var rejects int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM bus_rejects`).Scan(&rejects))
	require.Equal(t, 3, rejects)
}
