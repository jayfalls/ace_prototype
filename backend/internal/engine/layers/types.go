package layers

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// LayerType represents the six ACE layers
type LayerType int

const (
	LayerAspirational LayerType = iota + 1 // L1: Moral compass
	LayerGlobalStrategy                     // L2: High-level planning
	LayerAgentModel                        // L3: Self-modeling
	LayerExecutiveFunction                 // L4: Task management
	LayerCognitiveControl                  // L5: Decision-making
	LayerTaskProsecution                  // L6: Execution
)

// String returns the layer name
func (l LayerType) String() string {
	switch l {
	case LayerAspirational:
		return "aspirational"
	case LayerGlobalStrategy:
		return "global_strategy"
	case LayerAgentModel:
		return "agent_model"
	case LayerExecutiveFunction:
		return "executive_function"
	case LayerCognitiveControl:
		return "cognitive_control"
	case LayerTaskProsecution:
		return "task_prosecution"
	default:
		return "unknown"
	}
}

// LayerConfig holds configuration for a layer
type LayerConfig struct {
	MaxCycles    int           // Maximum cycles per execution (0 = infinite)
	MaxTime      time.Duration // Maximum time per execution
	LoopType     LoopType      // finite or infinite
	Enabled      bool          // Whether layer is active
}

// LoopType defines the processing loop behavior
type LoopType int

const (
	LoopInfinite LoopType = iota // Runs until goal reached
	LoopFinite                   // Runs for fixed iterations
)

// Layer represents a cognitive layer in the ACE architecture
type Layer interface {
	Type() LayerType
	Name() string
	Config() LayerConfig
	Process(ctx context.Context, input *LayerInput) (*LayerOutput, error)
	SetConfig(config LayerConfig)
}

// BaseLayer provides common functionality for all layers
type BaseLayer struct {
	layerType    LayerType
	name         string
	config       LayerConfig
}

// NewBaseLayer creates a new base layer
func NewBaseLayer(layerType LayerType, name string) *BaseLayer {
	return &BaseLayer{
		layerType: layerType,
		name:      name,
		config: LayerConfig{
			MaxCycles: 10,
			MaxTime:   30 * time.Second,
			LoopType:  LoopFinite,
			Enabled:   true,
		},
	}
}

func (b *BaseLayer) Type() LayerType     { return b.layerType }
func (b *BaseLayer) Name() string        { return b.name }
func (b *BaseLayer) Config() LayerConfig { return b.config }
func (b *BaseLayer) SetConfig(config LayerConfig) { b.config = config }

// LayerInput represents input to a layer
type LayerInput struct {
	AgentID      uuid.UUID
	SessionID    uuid.UUID
	CycleID      uuid.UUID
	LayerID      uuid.UUID
	Data         interface{}    // Input data (depends on layer)
	Memory       *MemoryContext // Access to memory modules
	Northbound   []Message     // Messages from lower layers
	Southbound   []Message     // Messages from higher layers
}

// LayerOutput represents output from a layer
type LayerOutput struct {
	LayerID      uuid.UUID
	CycleID      uuid.UUID
	Data         interface{} // Output data (depends on layer)
	Northbound   []Message  // Messages to send northbound
	Southbound   []Message  // Messages to send southbound
	Thoughts     []Thought  // Thought records for debugging
	Actions      []Action   // Actions to execute
}

// MemoryContext provides access to layer memory modules
type MemoryContext struct {
	ShortTerm  []MemoryItem // Always injected
	MediumTerm []MemoryItem // Always injected
	LongTerm   []MemoryItem // Retrieved from tree
	Global     []MemoryItem // Global memory access
}

// MemoryItem represents a memory entry
type MemoryItem struct {
	ID        uuid.UUID
	Content   string
	Tags      []string
	Importance float64
	Timestamp time.Time
}

// Message represents inter-layer communication
type Message struct {
	ID         uuid.UUID
	Type      MessageType
	SourceLayer LayerType
	TargetLayer LayerType
	Payload    interface{}
	Timestamp  time.Time
}

// MessageType defines the type of message
type MessageType int

const (
	MessageDirective MessageType = iota // Top-down command
	MessageFeedback                    // Bottom-up feedback
	MessageObservation                // Sensory input
	MessageLearning                   // Learning signal
)

// Thought represents a thought record for debugging
type Thought struct {
	ID        uuid.UUID
	Layer     LayerType
	Content   string
	Timestamp time.Time
}

// Action represents an action to be executed
type Action struct {
	ID       uuid.UUID
	Type     ActionType
	Target   string
	Payload  interface{}
}

// ActionType defines the type of action
type ActionType int

const (
	ActionExecuteTool ActionType = iota
	ActionRespond
	ActionStoreMemory
	ActionUpdateState
)

// AspirationalLayer (L1) - Moral compass
type AspirationalLayer struct {
	*BaseLayer
}

// NewAspirationalLayer creates L1
func NewAspirationalLayer() *AspirationalLayer {
	return &AspirationalLayer{
		BaseLayer: NewBaseLayer(LayerAspirational, "L1_Aspirational"),
	}
}

func (l *AspirationalLayer) Process(ctx context.Context, input *LayerInput) (*LayerOutput, error) {
	// Mock implementation - returns ethical guidance
	return &LayerOutput{
		LayerID:    input.LayerID,
		CycleID:    input.CycleID,
		Data:       map[string]interface{}{"ethical_guidance": "Ensure actions align with core values"},
		Northbound: []Message{},
		Southbound: []Message{},
		Thoughts: []Thought{{
			ID:      uuid.New(),
			Layer:   LayerAspirational,
			Content: "Evaluating ethical implications",
		}},
		Actions: []Action{},
	}, nil
}

// GlobalStrategyLayer (L2) - High-level planning
type GlobalStrategyLayer struct {
	*BaseLayer
}

func NewGlobalStrategyLayer() *GlobalStrategyLayer {
	return &GlobalStrategyLayer{
		BaseLayer: NewBaseLayer(LayerGlobalStrategy, "L2_GlobalStrategy"),
	}
}

func (l *GlobalStrategyLayer) Process(ctx context.Context, input *LayerInput) (*LayerOutput, error) {
	return &LayerOutput{
		LayerID:  input.LayerID,
		CycleID:  input.CycleID,
		Data:     map[string]interface{}{"strategy": "High-level plan created"},
		Thoughts: []Thought{{ID: uuid.New(), Layer: LayerGlobalStrategy, Content: "Formulating strategy"}},
	}, nil
}

// AgentModelLayer (L3) - Self-modeling
type AgentModelLayer struct {
	*BaseLayer
}

func NewAgentModelLayer() *AgentModelLayer {
	return &AgentModelLayer{
		BaseLayer: NewBaseLayer(LayerAgentModel, "L3_AgentModel"),
	}
}

func (l *AgentModelLayer) Process(ctx context.Context, input *LayerInput) (*LayerOutput, error) {
	return &LayerOutput{
		LayerID:  input.LayerID,
		CycleID:  input.CycleID,
		Data:     map[string]interface{}{"self_model": "Agent state updated"},
		Thoughts: []Thought{{ID: uuid.New(), Layer: LayerAgentModel, Content: "Updating self-model"}},
	}, nil
}

// ExecutiveFunctionLayer (L4) - Task management
type ExecutiveFunctionLayer struct {
	*BaseLayer
}

func NewExecutiveFunctionLayer() *ExecutiveFunctionLayer {
	return &ExecutiveFunctionLayer{
		BaseLayer: NewBaseLayer(LayerExecutiveFunction, "L4_ExecutiveFunction"),
	}
}

func (l *ExecutiveFunctionLayer) Process(ctx context.Context, input *LayerInput) (*LayerOutput, error) {
	return &LayerOutput{
		LayerID:  input.LayerID,
		CycleID:  input.CycleID,
		Data:     map[string]interface{}{"tasks": "Task list managed"},
		Thoughts: []Thought{{ID: uuid.New(), Layer: LayerExecutiveFunction, Content: "Managing tasks"}},
	}, nil
}

// CognitiveControlLayer (L5) - Decision-making
type CognitiveControlLayer struct {
	*BaseLayer
}

func NewCognitiveControlLayer() *CognitiveControlLayer {
	return &CognitiveControlLayer{
		BaseLayer: NewBaseLayer(LayerCognitiveControl, "L5_CognitiveControl"),
	}
}

func (l *CognitiveControlLayer) Process(ctx context.Context, input *LayerInput) (*LayerOutput, error) {
	return &LayerOutput{
		LayerID:  input.LayerID,
		CycleID:  input.CycleID,
		Data:     map[string]interface{}{"decision": "Decision made"},
		Thoughts: []Thought{{ID: uuid.New(), Layer: LayerCognitiveControl, Content: "Making decision"}},
	}, nil
}

// TaskProsecutionLayer (L6) - Execution
type TaskProsecutionLayer struct {
	*BaseLayer
}

func NewTaskProsecutionLayer() *TaskProsecutionLayer {
	return &TaskProsecutionLayer{
		BaseLayer: NewBaseLayer(LayerTaskProsecution, "L6_TaskProsecution"),
	}
}

func (l *TaskProsecutionLayer) Process(ctx context.Context, input *LayerInput) (*LayerOutput, error) {
	return &LayerOutput{
		LayerID:  input.LayerID,
		CycleID:  input.CycleID,
		Data:     map[string]interface{}{"executed": true},
		Thoughts: []Thought{{ID: uuid.New(), Layer: LayerTaskProsecution, Content: "Executing action"}},
	}, nil
}
