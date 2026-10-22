package cognitive

import (
	"context"
	"database/sql"
	"testing"

	"ace/internal/bus"
	"ace/internal/core"
	"ace/internal/memory"
	"ace/internal/sandbox"
	"ace/internal/telemetry"
	_ "modernc.org/sqlite"
	"github.com/stretchr/testify/require"
)

type fakeProvider struct {
	active string
}

func (f *fakeProvider) Name() string { return "fake" }
func (f *fakeProvider) Models() []core.ModelInfo {
	return []core.ModelInfo{{ID: "test-model", Name: "Test Model"}}
}
func (f *fakeProvider) ActiveModel() string { return f.active }
func (f *fakeProvider) SetActiveModel(m string) error {
	f.active = m
	return nil
}
func (f *fakeProvider) StreamComplete(ctx context.Context, req core.CompletionRequest, onDelta func(core.StreamDelta)) (core.CompletionResponse, error) {
	out := `{
		"thought": "Analyzing directive and executing sandbox verification.",
		"state_summary": "Executed cognitive step cleanly.",
		"confidence": 0.94,
		"southbound_directive": "Proceed with task execution in /workspace.",
		"northbound_signal": "Step nominal.",
		"action": {
			"type": "sandbox_sh",
			"target": "echo ACE_L6_EXEC_OK",
			"content": ""
		}
	}`
	if onDelta != nil {
		onDelta(core.StreamDelta{ReasoningDelta: "reasoning...", ContentDelta: out})
	}
	return core.CompletionResponse{
		Model:            f.active,
		Content:          out,
		Reasoning:        "reasoning...",
		PromptTokens:     50,
		CompletionTokens: 30,
		LatencyMs:        12,
	}, nil
}

func TestEngineStepAndChatAndL6Sandbox(t *testing.T) {
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

	fp := &fakeProvider{active: "test-model"}
	eng := NewEngine(fp, memBus, memStore, jail, telem)

	// 1. Step L1 -> should emit southbound directive to L2
	t1, err := eng.StepSingleLayer(context.Background(), core.LayerAspirational, "Initialize mission alignment")
	require.NoError(t, err)
	require.Equal(t, "L1", t1.LayerCode)
	require.Equal(t, "Executed cognitive step cleanly.", t1.StateSummary)

	// 2. Step L6 -> should execute sandbox_sh inside bwrap jail
	t6, err := eng.StepSingleLayer(context.Background(), core.LayerTaskProsecution, "Run echo check in sandbox")
	require.NoError(t, err)
	require.Equal(t, "L6", t6.LayerCode)
	require.Contains(t, t6.ActionResult, "ACE_L6_EXEC_OK")

	// 3. Fast Chat loop -> saves user & assistant messages and emits northbound sense event
	eng.SetMode(core.ModeManualStep, 45)
	reply, err := eng.Chat(context.Background(), "Hello ACE, how are your layers?")
	require.NoError(t, err)
	require.NotEmpty(t, reply.Content)

	busEvents, err := memBus.Recent(10)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(busEvents), 3)
}
