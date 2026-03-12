package layers

import (
	"context"
	"fmt"
	"time"

	"github.com/ace/framework/backend/internal/llm"
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
	LLMProvider  interface{}  // LLM provider for this layer
	Model        string        // Model to use
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
	SetLLMProvider(provider llm.Provider)
	GetLLMProvider() llm.Provider
}

// BaseLayer provides common functionality for all layers
type BaseLayer struct {
	layerType    LayerType
	name         string
	config       LayerConfig
	llmProvider  llm.Provider
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

// SetLLMProvider sets the LLM provider for this layer
func (b *BaseLayer) SetLLMProvider(provider llm.Provider) {
	b.llmProvider = provider
}

// GetLLMProvider gets the LLM provider for this layer
func (b *BaseLayer) GetLLMProvider() llm.Provider {
	return b.llmProvider
}

// ProcessWithLLM processes input using the LLM
func (b *BaseLayer) ProcessWithLLM(ctx context.Context, layerName, systemPrompt, userInput string) (string, error) {
	if b.llmProvider == nil {
		return "", fmt.Errorf("no LLM provider configured for layer %s", layerName)
	}

	req := llm.ChatRequest{
		Model: b.config.Model,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userInput},
		},
		Temperature: 0.7,
		MaxTokens:  2048,
	}

	resp, err := b.llmProvider.Chat(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}

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
	// Check if LLM is available
	if l.llmProvider == nil {
		return &LayerOutput{
			LayerID:    input.LayerID,
			CycleID:    input.CycleID,
			Data:       map[string]interface{}{"error": "no LLM provider configured"},
			Northbound: []Message{},
			Southbound: []Message{},
			Thoughts: []Thought{{
				ID:      uuid.New(),
				Layer:   LayerAspirational,
				Content: "Cannot process: no LLM provider configured",
			}},
			Actions: []Action{},
		}, nil
	}

	ethicalGuidance, err := l.ProcessWithLLM(ctx, "L1_Aspirational", 
		"You are the moral compass layer. Provide ethical guidance for the following input.",
		fmt.Sprintf("Input: %v", input.Data))
	if err != nil {
		return nil, fmt.Errorf("LLM processing failed: %w", err)
	}
	
	return &LayerOutput{
		LayerID:    input.LayerID,
		CycleID:    input.CycleID,
		Data:       map[string]interface{}{"ethical_guidance": ethicalGuidance},
		Northbound: []Message{},
		Southbound: []Message{},
		Thoughts: []Thought{{
			ID:      uuid.New(),
			Layer:   LayerAspirational,
			Content: "Evaluating ethical implications: " + ethicalGuidance,
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
	// Check if LLM is available
	if l.llmProvider == nil {
		return &LayerOutput{
			LayerID:  input.LayerID,
			CycleID:  input.CycleID,
			Data:     map[string]interface{}{"error": "no LLM provider configured"},
			Thoughts: []Thought{{ID: uuid.New(), Layer: LayerGlobalStrategy, Content: "Cannot process: no LLM provider configured"}},
		}, nil
	}

	inputStr := fmt.Sprintf("%v", input.Data)
	
	strategy, err := l.ProcessWithLLM(ctx, "L2_GlobalStrategy",
		"You are the strategic planning layer. Create high-level plans and strategies.",
		fmt.Sprintf("Current task: %s", inputStr))
	if err != nil {
		return nil, fmt.Errorf("LLM processing failed: %w", err)
	}
	
	return &LayerOutput{
		LayerID:  input.LayerID,
		CycleID:  input.CycleID,
		Data:     map[string]interface{}{"strategy": strategy},
		Thoughts: []Thought{{ID: uuid.New(), Layer: LayerGlobalStrategy, Content: "Formulating strategy: " + strategy}},
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
	// Check if LLM is available
	if l.llmProvider == nil {
		return &LayerOutput{
			LayerID:  input.LayerID,
			CycleID:  input.CycleID,
			Data:     map[string]interface{}{"error": "no LLM provider configured"},
			Thoughts: []Thought{{ID: uuid.New(), Layer: LayerAgentModel, Content: "Cannot process: no LLM provider configured"}},
		}, nil
	}

	inputStr := fmt.Sprintf("%v", input.Data)
	
	selfModel, err := l.ProcessWithLLM(ctx, "L3_AgentModel",
		"You are the self-modeling layer. Analyze the agent's capabilities and limitations.",
		fmt.Sprintf("Current context: %s", inputStr))
	if err != nil {
		return nil, fmt.Errorf("LLM processing failed: %w", err)
	}
	
	return &LayerOutput{
		LayerID:  input.LayerID,
		CycleID:  input.CycleID,
		Data:     map[string]interface{}{"self_model": selfModel},
		Thoughts: []Thought{{ID: uuid.New(), Layer: LayerAgentModel, Content: "Updating self-model: " + selfModel}},
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
	// Check if LLM is available
	if l.llmProvider == nil {
		return &LayerOutput{
			LayerID:  input.LayerID,
			CycleID:  input.CycleID,
			Data:     map[string]interface{}{"error": "no LLM provider configured"},
			Thoughts: []Thought{{ID: uuid.New(), Layer: LayerExecutiveFunction, Content: "Cannot process: no LLM provider configured"}},
		}, nil
	}

	inputStr := fmt.Sprintf("%v", input.Data)
	
	taskMgmt, err := l.ProcessWithLLM(ctx, "L4_ExecutiveFunction",
		"You are the executive function layer. Manage tasks, switch contexts, and allocate cognitive resources.",
		fmt.Sprintf("Current context: %s", inputStr))
	if err != nil {
		return nil, fmt.Errorf("LLM processing failed: %w", err)
	}
	
	return &LayerOutput{
		LayerID:  input.LayerID,
		CycleID:  input.CycleID,
		Data:     map[string]interface{}{"tasks": taskMgmt},
		Thoughts: []Thought{{ID: uuid.New(), Layer: LayerExecutiveFunction, Content: "Managing tasks: " + taskMgmt}},
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
	// Check if LLM is available
	if l.llmProvider == nil {
		return &LayerOutput{
			LayerID:  input.LayerID,
			CycleID:  input.CycleID,
			Data:     map[string]interface{}{"error": "no LLM provider configured"},
			Thoughts: []Thought{{ID: uuid.New(), Layer: LayerCognitiveControl, Content: "Cannot process: no LLM provider configured"}},
		}, nil
	}

	inputStr := fmt.Sprintf("%v", input.Data)
	
	decision, err := l.ProcessWithLLM(ctx, "L5_CognitiveControl",
		"You are the cognitive control layer. Make decisions, manage attention, and resolve conflicts.",
		fmt.Sprintf("Current context: %s", inputStr))
	if err != nil {
		return nil, fmt.Errorf("LLM processing failed: %w", err)
	}
	
	return &LayerOutput{
		LayerID:  input.LayerID,
		CycleID:  input.CycleID,
		Data:     map[string]interface{}{"decision": decision},
		Thoughts: []Thought{{ID: uuid.New(), Layer: LayerCognitiveControl, Content: "Making decision: " + decision}},
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
	// Check if LLM is available
	if l.llmProvider == nil {
		return &LayerOutput{
			LayerID:  input.LayerID,
			CycleID:  input.CycleID,
			Data:     map[string]interface{}{"error": "no LLM provider configured"},
			Thoughts: []Thought{{ID: uuid.New(), Layer: LayerTaskProsecution, Content: "Cannot process: no LLM provider configured"}},
		}, nil
	}

	inputStr := fmt.Sprintf("%v", input.Data)
	
	execution, err := l.ProcessWithLLM(ctx, "L6_TaskProsecution",
		"You are the task prosecution layer. Execute actions and interact with the environment.",
		fmt.Sprintf("Current task: %s", inputStr))
	if err != nil {
		return nil, fmt.Errorf("LLM processing failed: %w", err)
	}
	
	return &LayerOutput{
		LayerID:  input.LayerID,
		CycleID:  input.CycleID,
		Data:     map[string]interface{}{"executed": true, "result": execution},
		Thoughts: []Thought{{ID: uuid.New(), Layer: LayerTaskProsecution, Content: "Executing: " + execution}},
	}, nil
}

// WireAllLayers wires all layers to the same provider
func WireAllLayers(engine *Engine, provider llm.Provider, model string) {
	for lt := LayerAspirational; lt <= LayerTaskProsecution; lt++ {
		layer, ok := engine.GetLayer(lt)
		if !ok {
			continue
		}
		
		layer.SetConfig(LayerConfig{
			Model:   model,
			Enabled: true,
		})
		
		if provider != nil {
			layer.SetLLMProvider(provider)
		}
	}
}
