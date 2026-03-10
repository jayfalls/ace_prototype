package engine

import (
	"context"
	"sync"
	"time"

	"github.com/ace/framework/backend/internal/engine/layers"
	"github.com/ace/framework/backend/internal/messaging"
	"github.com/google/uuid"
)

// AgentStatus represents the current status of an agent
type AgentStatus string

const (
	AgentStatusStopped AgentStatus = "stopped"
	AgentStatusStarting AgentStatus = "starting"
	AgentStatusRunning AgentStatus = "running"
	AgentStatusPaused AgentStatus = "paused"
	AgentStatusError AgentStatus = "error"
)

// Agent represents a running agent instance
type Agent struct {
	ID          uuid.UUID
	Name        string
	Description string
	UserID      string
	Status      AgentStatus
	Engine      *layers.Engine
	Bus         messaging.Publisher
	Config      AgentConfig
	CreatedAt   time.Time
	UpdatedAt   time.Time
	mu          sync.RWMutex
}

// AgentConfig holds agent configuration
type AgentConfig struct {
	MaxCycles    int           // 0 = infinite
	MaxTime      time.Duration // 0 = no limit
	StopOnError  bool
	Model        string
	Provider     string
	LLMEnabled   bool
}

// AgentService manages agent lifecycle
type AgentService struct {
	agents map[uuid.UUID]*Agent
	bus    messaging.Publisher
	mu     sync.RWMutex
}

// NewAgentService creates a new agent service
func NewAgentService(bus messaging.Publisher) *AgentService {
	return &AgentService{
		agents: make(map[uuid.UUID]*Agent),
		bus:    bus,
	}
}

// CreateAgent creates a new agent
func (s *AgentService) CreateAgent(ctx context.Context, name, description, userID string, config AgentConfig) (*Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	agentID := uuid.New()
	
	// Create engine with bus
	var bus layers.Bus
	if s.bus != nil {
		bus = layers.NewInMemoryBus()
	} else {
		bus = layers.NewInMemoryBus()
	}
	
	engine := layers.NewEngine(agentID, bus)

	agent := &Agent{
		ID:          agentID,
		Name:        name,
		Description: description,
		UserID:      userID,
		Status:      AgentStatusStopped,
		Engine:      engine,
		Bus:         s.bus,
		Config:      config,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	s.agents[agentID] = agent
	return agent, nil
}

// StartAgent starts an agent
func (s *AgentService) StartAgent(ctx context.Context, agentID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, ok := s.agents[agentID]
	if !ok {
		return ErrAgentNotFound
	}

	if agent.Status == AgentStatusRunning {
		return ErrAgentAlreadyRunning
	}

	agent.mu.Lock()
	defer agent.mu.Unlock()

	agent.Status = AgentStatusStarting
	agent.UpdatedAt = time.Now()

	// Start the engine
	if err := agent.Engine.Start(ctx); err != nil {
		agent.Status = AgentStatusError
		return err
	}

	agent.Status = AgentStatusRunning
	agent.UpdatedAt = time.Now()

	return nil
}

// StopAgent stops an agent
func (s *AgentService) StopAgent(ctx context.Context, agentID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, ok := s.agents[agentID]
	if !ok {
		return ErrAgentNotFound
	}

	agent.mu.Lock()
	defer agent.mu.Unlock()

	if agent.Status != AgentStatusRunning && agent.Status != AgentStatusPaused {
		return ErrAgentNotRunning
	}

	// Stop the engine
	if err := agent.Engine.Stop(); err != nil {
		return err
	}

	agent.Status = AgentStatusStopped
	agent.UpdatedAt = time.Now()

	return nil
}

// GetAgent returns an agent by ID
func (s *AgentService) GetAgent(agentID uuid.UUID) (*Agent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agent, ok := s.agents[agentID]
	return agent, ok
}

// ListAgents returns all agents
func (s *AgentService) ListAgents() []*Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agents := make([]*Agent, 0, len(s.agents))
	for _, agent := range s.agents {
		agents = append(agents, agent)
	}
	return agents
}

// ProcessInput sends input to an agent and returns the result
func (s *AgentService) ProcessInput(ctx context.Context, agentID uuid.UUID, input interface{}) (*layers.CycleResult, error) {
	s.mu.RLock()
	agent, ok := s.agents[agentID]
	s.mu.RUnlock()

	if !ok {
		return nil, ErrAgentNotFound
	}

	if agent.Status != AgentStatusRunning {
		return nil, ErrAgentNotRunning
	}

	return agent.Engine.ProcessCycle(ctx, input)
}

// DeleteAgent removes an agent
func (s *AgentService) DeleteAgent(agentID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, ok := s.agents[agentID]
	if !ok {
		return ErrAgentNotFound
	}

	if agent.Status == AgentStatusRunning {
		agent.Engine.Stop()
	}

	delete(s.agents, agentID)
	return nil
}

// Errors
var (
	ErrAgentNotFound      = &AgentError{"agent not found"}
	ErrAgentAlreadyRunning = &AgentError{"agent already running"}
	ErrAgentNotRunning    = &AgentError{"agent not running"}
)

type AgentError struct {
	msg string
}

func (e *AgentError) Error() string {
	return e.msg
}
