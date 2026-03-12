package layers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// StartupStatus represents the status of agent startup
type StartupStatus struct {
	Step          string    `json:"step"`
	Status        string    `json:"status"` // pending, running, completed, failed
	Error         string    `json:"error,omitempty"`
	StartedAt     time.Time `json:"started_at"`
	CompletedAt   time.Time `json:"completed_at,omitempty"`
	DurationMs   int64     `json:"duration_ms"`
}

// Engine coordinates all ACE layers
type Engine struct {
	agentID       uuid.UUID
	layers        map[LayerType]Layer
	bus           Bus
	mu            sync.RWMutex
	running       bool
	startupStatus []*StartupStatus
	cycleCount    int
	busMessages   int
}

// Bus handles inter-layer communication
type Bus interface {
	Publish(ctx context.Context, msg Message) error
	Subscribe(ctx context.Context, layerType LayerType, handler func(Message) error) error
	Unsubscribe(layerType LayerType) error
}

// NewEngine creates a new cognitive engine
func NewEngine(agentID uuid.UUID, bus Bus) *Engine {
	e := &Engine{
		agentID: agentID,
		layers:  make(map[LayerType]Layer),
		bus:     bus,
	}

	// Register all 6 layers
	e.layers[LayerAspirational] = NewAspirationalLayer()
	e.layers[LayerGlobalStrategy] = NewGlobalStrategyLayer()
	e.layers[LayerAgentModel] = NewAgentModelLayer()
	e.layers[LayerExecutiveFunction] = NewExecutiveFunctionLayer()
	e.layers[LayerCognitiveControl] = NewCognitiveControlLayer()
	e.layers[LayerTaskProsecution] = NewTaskProsecutionLayer()

	return e
}

// Start begins processing cycles with full startup sequence
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return nil
	}

	// Initialize startup status tracking
	e.startupStatus = []*StartupStatus{
		{Step: "provider", Status: "pending", StartedAt: time.Now()},
		{Step: "layers", Status: "pending", StartedAt: time.Now()},
		{Step: "bus", Status: "pending", StartedAt: time.Now()},
		{Step: "tools", Status: "pending", StartedAt: time.Now()},
		{Step: "ready", Status: "pending", StartedAt: time.Now()},
	}

	// Step 1: Check provider (already wired via WireAllLayers)
	e.startupStatus[0].Status = "completed"
	e.startupStatus[0].CompletedAt = time.Now()
	e.startupStatus[0].DurationMs = time.Since(e.startupStatus[0].StartedAt).Milliseconds()

	// Step 2: Initialize layers
	e.startupStatus[1].Status = "running"
	hasLLM := false
	for _, layer := range e.layers {
		if layer.GetLLMProvider() != nil {
			hasLLM = true
			break
		}
	}
	if !hasLLM {
		e.startupStatus[1].Status = "failed"
		e.startupStatus[1].Error = "no LLM provider wired to layers"
		e.startupStatus[2].Status = "pending"
		e.startupStatus[3].Status = "pending"
		e.startupStatus[4].Status = "pending"
		return fmt.Errorf("startup failed: no LLM provider")
	}
	e.startupStatus[1].Status = "completed"
	e.startupStatus[1].CompletedAt = time.Now()
	e.startupStatus[1].DurationMs = time.Since(e.startupStatus[1].StartedAt).Milliseconds()

	// Step 3: Initialize bus
	e.startupStatus[2].Status = "running"
	if e.bus == nil {
		e.startupStatus[2].Status = "failed"
		e.startupStatus[2].Error = "message bus not initialized"
		e.startupStatus[3].Status = "pending"
		e.startupStatus[4].Status = "pending"
		return fmt.Errorf("startup failed: no message bus")
	}
	e.startupStatus[2].Status = "completed"
	e.startupStatus[2].CompletedAt = time.Now()
	e.startupStatus[2].DurationMs = time.Since(e.startupStatus[2].StartedAt).Milliseconds()

	// Step 4: Load tools (placeholder - MCP tools would be loaded here)
	e.startupStatus[3].Status = "completed"
	e.startupStatus[3].CompletedAt = time.Now()
	e.startupStatus[3].DurationMs = time.Since(e.startupStatus[3].StartedAt).Milliseconds()

	// Step 5: Mark ready
	e.startupStatus[4].Status = "completed"
	e.startupStatus[4].CompletedAt = time.Now()
	e.startupStatus[4].DurationMs = time.Since(e.startupStatus[4].StartedAt).Milliseconds()

	e.running = true
	e.cycleCount = 0
	return nil
}

// GetStartupStatus returns the startup sequence status
func (e *Engine) GetStartupStatus() []*StartupStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.startupStatus
}

// GetCycleCount returns the number of cycles processed
func (e *Engine) GetCycleCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cycleCount
}

// GetBusMessageCount returns the number of messages sent via bus
func (e *Engine) GetBusMessageCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.busMessages
}

// IsStartupComplete returns true if startup completed successfully
func (e *Engine) IsStartupComplete() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if len(e.startupStatus) == 0 {
		return false
	}
	// Check if the final step (ready) is completed
	return e.startupStatus[len(e.startupStatus)-1].Status == "completed"
}

// Stop halts processing
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.running = false
	return nil
}

// ProcessCycle runs one complete cycle through all layers
func (e *Engine) ProcessCycle(ctx context.Context, input interface{}) (*CycleResult, error) {
	cycleID := uuid.New()
	result := &CycleResult{
		CycleID:   cycleID,
		StartTime: time.Now(),
		LayerOutputs: make(map[LayerType]*LayerOutput),
	}

	// Create memory context (empty for now, will be populated from memory service)
	memCtx := &MemoryContext{
		ShortTerm:  []MemoryItem{},
		MediumTerm: []MemoryItem{},
		LongTerm:   []MemoryItem{},
		Global:     []MemoryItem{},
	}

	// Process from L6 (bottom) to L1 (top) - northbound
	// Then from L1 to L6 - southbound
	layerOrder := []LayerType{
		LayerTaskProsecution,      // L6
		LayerCognitiveControl,    // L5
		LayerExecutiveFunction,   // L4
		LayerAgentModel,          // L3
		LayerGlobalStrategy,      // L2
		LayerAspirational,        // L1
	}

	var northboundMsgs []Message
	var southboundMsgs []Message

	for _, lt := range layerOrder {
		layer, ok := e.layers[lt]
		if !ok {
			continue
		}

		layerInput := &LayerInput{
			AgentID:    e.agentID,
			SessionID:  uuid.New(), // TODO: Get from context
			CycleID:    cycleID,
			LayerID:    uuid.New(),
			Data:       input,
			Memory:     memCtx,
			Northbound: northboundMsgs,
			Southbound: southboundMsgs,
		}

		output, err := layer.Process(ctx, layerInput)
		if err != nil {
			result.Error = err
			return result, err
		}

		result.LayerOutputs[lt] = output
		result.Thoughts = append(result.Thoughts, output.Thoughts...)

		// Aggregate messages for next layer
		northboundMsgs = append(northboundMsgs, output.Northbound...)
		southboundMsgs = append(southboundMsgs, output.Southbound...)

		// Publish layer output to bus
		if e.bus != nil {
			msg := Message{
				ID:          uuid.New(),
				Type:        MessageObservation,
				SourceLayer: lt,
				Payload:     output.Data,
				Timestamp:   time.Now(),
			}
			_ = e.bus.Publish(ctx, msg)
		}
	}

	result.EndTime = time.Now()
	return result, nil
}

// GetLayer returns a specific layer
func (e *Engine) GetLayer(layerType LayerType) (Layer, bool) {
	layer, ok := e.layers[layerType]
	return layer, ok
}

// IsRunning returns engine state
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// CycleResult contains the results of one processing cycle
type CycleResult struct {
	CycleID      uuid.UUID
	StartTime    time.Time
	EndTime      time.Time
	LayerOutputs map[LayerType]*LayerOutput
	Thoughts     []Thought
	Error       error
}

// AllLayers returns all registered layers
func (e *Engine) AllLayers() []Layer {
	layers := make([]Layer, 0, len(e.layers))
	for _, l := range e.layers {
		layers = append(layers, l)
	}
	return layers
}
