package visualizations

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ThoughtTraceVisualizer shows layer-by-layer thinking
type ThoughtTraceVisualizer struct {
	mu       sync.RWMutex
	traces   map[uuid.UUID][]ThoughtEntry
	maxSize  int
}

// ThoughtEntry represents one thought in the trace
type ThoughtEntry struct {
	Layer     string    `json:"layer"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// NewThoughtTraceVisualizer creates a thought trace visualizer
func NewThoughtTraceVisualizer() *ThoughtTraceVisualizer {
	return &ThoughtTraceVisualizer{
		traces:  make(map[uuid.UUID][]ThoughtEntry),
		maxSize: 1000,
	}
}

// AddEntry adds a thought to a trace
func (v *ThoughtTraceVisualizer) AddEntry(sessionID uuid.UUID, layer, content string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	entry := ThoughtEntry{
		Layer:     layer,
		Content:   content,
		Timestamp: time.Now(),
	}

	v.traces[sessionID] = append(v.traces[sessionID], entry)

	if len(v.traces[sessionID]) > v.maxSize {
		v.traces[sessionID] = v.traces[sessionID][len(v.traces[sessionID])-v.maxSize:]
	}
}

// GetTrace returns the full trace
func (v *ThoughtTraceVisualizer) GetTrace(sessionID uuid.UUID) []ThoughtEntry {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.traces[sessionID]
}

// GetTraceJSON returns trace as JSON
func (v *ThoughtTraceVisualizer) GetTraceJSON(sessionID uuid.UUID) (string, error) {
	trace := v.GetTrace(sessionID)
	data, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// MemoryTreeVisualizer shows memory hierarchy
type MemoryTreeVisualizer struct {
	mu    sync.RWMutex
	trees map[uuid.UUID]MemoryNode
}

// MemoryNode represents a node in the memory tree
type MemoryNode struct {
	ID        uuid.UUID     `json:"id"`
	Content   string        `json:"content"`
	Tags      []string      `json:"tags"`
	Children  []MemoryNode  `json:"children"`
	Expanded  bool          `json:"expanded"`
}

// NewMemoryTreeVisualizer creates a memory tree visualizer
func NewMemoryTreeVisualizer() *MemoryTreeVisualizer {
	return &MemoryTreeVisualizer{
		trees: make(map[uuid.UUID]MemoryNode),
	}
}

// Update updates the tree structure
func (v *MemoryTreeVisualizer) Update(agentID uuid.UUID, rootID uuid.UUID, nodes map[uuid.UUID]MemoryNode) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.trees[agentID] = nodes[rootID]
}

// GetTree returns the tree for an agent
func (v *MemoryTreeVisualizer) GetTree(agentID uuid.UUID) (MemoryNode, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	node, ok := v.trees[agentID]
	return node, ok
}

// AgentStateVisualizer shows current agent status
type AgentStateVisualizer struct {
	mu    sync.RWMutex
	state map[uuid.UUID]StateData
}

// StateData represents agent state
type StateData struct {
	AgentID    uuid.UUID   `json:"agent_id"`
	Status     string      `json:"status"`
	CurrentTask string      `json:"current_task"`
	ActiveLoops []string   `json:"active_loops"`
	MemoryUsed int         `json:"memory_used"`
	LastCycle  time.Time   `json:"last_cycle"`
	Layers     []LayerState `json:"layers"`
}

// LayerState represents individual layer state
type LayerState struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Output string `json:"output,omitempty"`
}

// NewAgentStateVisualizer creates an agent state visualizer
func NewAgentStateVisualizer() *AgentStateVisualizer {
	return &AgentStateVisualizer{
		state: make(map[uuid.UUID]StateData),
	}
}

// Update updates agent state
func (v *AgentStateVisualizer) Update(agentID uuid.UUID, state StateData) {
	v.mu.Lock()
	defer v.mu.Unlock()
	state.AgentID = agentID
	state.LastCycle = time.Now()
	v.state[agentID] = state
}

// Get returns current state
func (v *AgentStateVisualizer) Get(agentID uuid.UUID) (StateData, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	state, ok := v.state[agentID]
	return state, ok
}

// Manager coordinates all visualizers
type Manager struct {
	mu              sync.RWMutex
	thoughtTrace    *ThoughtTraceVisualizer
	memoryTree      *MemoryTreeVisualizer
	agentState      *AgentStateVisualizer
}

// NewManager creates a visualization manager
func NewManager() *Manager {
	return &Manager{
		thoughtTrace: NewThoughtTraceVisualizer(),
		memoryTree:   NewMemoryTreeVisualizer(),
		agentState:   NewAgentStateVisualizer(),
	}
}

// RenderDashboard generates dashboard data
func (m *Manager) RenderDashboard(agentID, sessionID uuid.UUID) (string, error) {
	state, _ := m.agentState.Get(agentID)
	trace := m.thoughtTrace.GetTrace(sessionID)

	dashboard := map[string]interface{}{
		"agent_state":   state,
		"thought_trace": trace,
		"timestamp":     time.Now(),
	}

	data, err := json.MarshalIndent(dashboard, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to render dashboard: %w", err)
	}

	return string(data), nil
}
