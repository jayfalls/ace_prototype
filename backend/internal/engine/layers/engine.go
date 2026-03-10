package layers

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Engine coordinates all ACE layers
type Engine struct {
	agentID    uuid.UUID
	layers     map[LayerType]Layer
	bus        Bus
	mu         sync.RWMutex
	running    bool
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

// Start begins processing cycles
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return nil
	}

	e.running = true
	return nil
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
