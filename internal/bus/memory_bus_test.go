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
		Topic:     "ace.bus.south.L1->L2",
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
