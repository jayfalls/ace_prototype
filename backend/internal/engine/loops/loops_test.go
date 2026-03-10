package loops

import (
	"context"
	"testing"
	"time"

	"github.com/ace/framework/backend/internal/engine/layers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLayerLoop tests the layer loop
func TestLayerLoop(t *testing.T) {
	agentID := uuid.New()
	engine := layers.NewEngine(agentID, nil)
	
	ctx := context.Background()
	
	loop := NewLayerLoop(engine, LayerLoopConfig{
		MaxCycles:   3,
		MaxTime:     10 * time.Second,
		StopOnError: true,
	})
	
	result, err := loop.Run(ctx, "test input")
	require.NoError(t, err)
	require.NotNil(t, result)
	
	// Should complete 3 cycles
	assert.Equal(t, 3, result.CyclesCompleted)
	assert.Less(t, result.TotalTime, 10*time.Second)
}

// TestLayerLoopInfinite tests infinite layer loop
func TestLayerLoopInfinite(t *testing.T) {
	agentID := uuid.New()
	engine := layers.NewEngine(agentID, nil)
	
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	loop := NewLayerLoop(engine, LayerLoopConfig{
		MaxCycles:   0, // infinite
		MaxTime:     0,
		StopOnError: true,
	})
	
	result, err := loop.Run(ctx, "test input")
	// Should timeout
	assert.Error(t, err)
	assert.NotNil(t, result)
}

// TestLayerLoopConfig tests loop configuration
func TestLayerLoopConfig(t *testing.T) {
	cfg := LayerLoopConfig{
		MaxCycles:   5,
		MaxTime:     30 * time.Second,
		StopOnError: false,
	}
	
	assert.Equal(t, 5, cfg.MaxCycles)
	assert.Equal(t, 30*time.Second, cfg.MaxTime)
	assert.False(t, cfg.StopOnError)
}

// TestGlobalLoops tests global loops initialization
func TestGlobalLoops(t *testing.T) {
	bus := layers.NewInMemoryBus()
	gl := NewGlobalLoops(bus)
	
	require.NotNil(t, gl)
	assert.NotNil(t, gl.ChatLoop)
	assert.NotNil(t, gl.SafetyMonitor)
}

// TestChatLoop tests chat loop
func TestChatLoop(t *testing.T) {
	bus := layers.NewInMemoryBus()
	chat := NewChatLoop(bus)
	
	require.NotNil(t, chat)
	
	ctx := context.Background()
	result := chat.Process(ctx, "hello")
	
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Response)
}

// TestSafetyMonitor tests safety monitoring
func TestSafetyMonitor(t *testing.T) {
	bus := layers.NewInMemoryBus()
	safety := NewSafetyMonitor(bus)
	
	require.NotNil(t, safety)
	
	ctx := context.Background()
	
	// Test safe input
	result := safety.Check(ctx, "hello world")
	assert.True(t, result.Safe)
	assert.Empty(t, result.Violations)
	
	// Test unsafe input (if threat detection is implemented)
	// result = safety.Check(ctx, "malicious command")
	// assert.False(t, result.Safe)
}

// TestLoopState tests loop state transitions
func TestLoopState(t *testing.T) {
	state := LoopStateIdle
	
	// Transition to running
	state = state.Start()
	assert.Equal(t, LoopStateRunning, state)
	
	// Transition to paused
	state = state.Pause()
	assert.Equal(t, LoopStatePaused, state)
	
	// Transition back to running
	state = state.Resume()
	assert.Equal(t, LoopStateRunning, state)
	
	// Transition to stopped
	state = state.Stop()
	assert.Equal(t, LoopStateStopped, state)
}
