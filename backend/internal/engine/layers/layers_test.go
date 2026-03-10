package layers

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var agentID = uuid.New()

// TestLayerTypes tests layer type constants
func TestLayerTypes(t *testing.T) {
	tests := []struct {
		layer     LayerType
		expected  string
	}{
		{LayerAspirational, "aspirational"},
		{LayerGlobalStrategy, "global_strategy"},
		{LayerAgentModel, "agent_model"},
		{LayerExecutiveFunction, "executive_function"},
		{LayerCognitiveControl, "cognitive_control"},
		{LayerTaskProsecution, "task_prosecution"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.layer.String())
		})
	}
}

// TestEngineCreation tests engine initialization
func TestEngineCreation(t *testing.T) {
	engine := NewEngine(agentID, nil)
	require.NotNil(t, engine)

	// Check all 6 layers registered
	assert.True(t, engine.IsRunning() == false)

	// Get all layers
	layers := engine.AllLayers()
	assert.Len(t, layers, 6)
}

// TestEngineGetLayer tests layer retrieval
func TestEngineGetLayer(t *testing.T) {
	engine := NewEngine(agentID, nil)

	layer, ok := engine.GetLayer(LayerAspirational)
	assert.True(t, ok)
	assert.NotNil(t, layer)
	assert.Equal(t, "L1_Aspirational", layer.Name())

	// Test invalid layer
	_, ok = engine.GetLayer(LayerType(999))
	assert.False(t, ok)
}

// TestProcessCycle tests one complete cycle
func TestProcessCycle(t *testing.T) {
	engine := NewEngine(agentID, nil)
	ctx := context.Background()

	result, err := engine.ProcessCycle(ctx, "test input")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.NotEqual(t, uuid.Nil, result.CycleID)
	assert.NotZero(t, result.StartTime)
	assert.NotZero(t, result.EndTime)
	assert.Greater(t, result.EndTime, result.StartTime)

	// Check all layers processed
	assert.Len(t, result.LayerOutputs, 6)
	assert.Len(t, result.Thoughts, 6)
}

// TestLayerProcess tests individual layer processing
func TestLayerProcess(t *testing.T) {
	layer := NewAspirationalLayer()
	ctx := context.Background()

	input := &LayerInput{
		AgentID:   agentID,
		SessionID: uuid.New(),
		CycleID:   uuid.New(),
		LayerID:   uuid.New(),
		Data:      "test data",
		Memory: &MemoryContext{
			ShortTerm:  []MemoryItem{},
			MediumTerm: []MemoryItem{},
			LongTerm:   []MemoryItem{},
			Global:     []MemoryItem{},
		},
	}

	output, err := layer.Process(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, output)

	assert.NotNil(t, output.Data)
	assert.Len(t, output.Thoughts, 1)
	assert.Equal(t, LayerAspirational, output.LayerID)
}

// TestMemoryContext tests memory context
func TestMemoryContext(t *testing.T) {
	mem := &MemoryContext{
		ShortTerm: []MemoryItem{
			{Content: "short term 1"},
			{Content: "short term 2"},
		},
		MediumTerm: []MemoryItem{
			{Content: "medium term 1"},
		},
		LongTerm: []MemoryItem{
			{Content: "long term 1"},
			{Content: "long term 2"},
			{Content: "long term 3"},
		},
		Global: []MemoryItem{},
	}

	assert.Len(t, mem.ShortTerm, 2)
	assert.Len(t, mem.MediumTerm, 1)
	assert.Len(t, mem.LongTerm, 3)
	assert.Len(t, mem.Global, 0)
}

// TestMessageTypes tests message type constants
func TestMessageTypes(t *testing.T) {
	assert.Equal(t, MessageType(1), MessageDirective)
	assert.Equal(t, MessageType(2), MessageFeedback)
	assert.Equal(t, MessageType(3), MessageObservation)
	assert.Equal(t, MessageType(4), MessageLearning)
}

// TestLoopTypes tests loop type constants
func TestLoopTypes(t *testing.T) {
	assert.Equal(t, LoopType(0), LoopInfinite)
	assert.Equal(t, LoopType(1), LoopFinite)
}

// TestInMemoryBus tests in-memory bus
func TestInMemoryBus(t *testing.T) {
	bus := NewInMemoryBus()
	require.NotNil(t, bus)

	ctx := context.Background()
	var receivedMsg Message

	// Subscribe
	err := bus.Subscribe(ctx, LayerAspirational, func(msg Message) error {
		receivedMsg = msg
		return nil
	})
	require.NoError(t, err)

	// Publish
	msg := Message{
		ID:          uuid.New(),
		Type:        MessageObservation,
		SourceLayer: LayerAspirational,
		Payload:     "test payload",
		Timestamp:   time.Now(),
	}
	err = bus.Publish(ctx, msg)
	require.NoError(t, err)

	// Give time for message to propagate
	time.Sleep(10 * time.Millisecond)

	// Message should be received (in async handler)
	assert.Equal(t, "test payload", receivedMsg.Payload)

	// Cleanup
	bus.Close()
}
