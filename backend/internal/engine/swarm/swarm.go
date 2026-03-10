package swarm

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Agent represents an agent in the swarm
type Agent struct {
	ID        uuid.UUID
	Name      string
	Status    AgentStatus
	Position  Position
	JoinedAt  time.Time
	LastSeen  time.Time
}

// Position represents agent location in swarm
type Position struct {
	X float64
	Y float64
}

// AgentStatus defines agent state
type AgentStatus int

const (
	AgentIdle AgentStatus = iota
	AgentWorking
	AgentBlocked
	AgentFailed
)

// Coordinator manages multi-agent coordination
type Coordinator struct {
	mu           sync.RWMutex
	agents       map[uuid.UUID]*Agent
	tasks        map[uuid.UUID]*Task
	strategies   map[StrategyType]Strategy
}

// Task represents a work item
type Task struct {
	ID          uuid.UUID
	Name        string
	Status      TaskStatus
	AssignedTo  *uuid.UUID
	CreatedAt   time.Time
	CompletedAt *time.Time
	Result      interface{}
}

// TaskStatus defines task state
type TaskStatus int

const (
	TaskPending TaskStatus = iota
	TaskAssigned
	TaskInProgress
	TaskCompleted
	TaskFailed
)

// StrategyType defines coordination strategy
type StrategyType int

const (
	StrategyRoundRobin StrategyType = iota
	StrategyLeastLoaded
	StrategyRandom
)

// Strategy defines task assignment logic
type Strategy interface {
	Name() string
	SelectAgent(agents []*Agent, task *Task) *uuid.UUID
}

// NewCoordinator creates a swarm coordinator
func NewCoordinator() *Coordinator {
	c := &Coordinator{
		agents:   make(map[uuid.UUID]*Agent),
		tasks:    make(map[uuid.UUID]*Task),
		strategies: make(map[StrategyType]Strategy),
	}

	// Register default strategies
	c.strategies[StrategyRoundRobin] = &RoundRobinStrategy{}
	c.strategies[StrategyLeastLoaded] = &LeastLoadedStrategy{}
	c.strategies[StrategyRandom] = &RandomStrategy{}

	return c
}

// RegisterAgent adds an agent to the swarm
func (c *Coordinator) RegisterAgent(ctx context.Context, name string) (*Agent, error) {
	agent := &Agent{
		ID:       uuid.New(),
		Name:     name,
		Status:   AgentIdle,
		Position: Position{X: 0, Y: 0},
		JoinedAt: time.Now(),
		LastSeen: time.Now(),
	}

	c.mu.Lock()
	c.agents[agent.ID] = agent
	c.mu.Unlock()

	return agent, nil
}

// UnregisterAgent removes an agent
func (c *Coordinator) UnregisterAgent(agentID uuid.UUID) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.agents[agentID]; !ok {
		return fmt.Errorf("agent not found")
	}

	delete(c.agents, agentID)
	return nil
}

// GetAgent returns an agent
func (c *Coordinator) GetAgent(agentID uuid.UUID) (*Agent, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	agent, ok := c.agents[agentID]
	return agent, ok
}

// ListAgents returns all agents
func (c *Coordinator) ListAgents() []*Agent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*Agent, 0, len(c.agents))
	for _, a := range c.agents {
		result = append(result, a)
	}
	return result
}

// CreateTask adds a task
func (c *Coordinator) CreateTask(ctx context.Context, name string) (*Task, error) {
	task := &Task{
		ID:        uuid.New(),
		Name:      name,
		Status:    TaskPending,
		CreatedAt: time.Now(),
	}

	c.mu.Lock()
	c.tasks[task.ID] = task
	c.mu.Unlock()

	return task, nil
}

// AssignTask assigns a task to an agent using strategy
func (c *Coordinator) AssignTask(ctx context.Context, taskID, agentID uuid.UUID) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	task, ok := c.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found")
	}

	agent, ok := c.agents[agentID]
	if !ok {
		return fmt.Errorf("agent not found")
	}

	task.AssignedTo = &agentID
	task.Status = TaskAssigned
	agent.Status = AgentWorking

	return nil
}

// AutoAssign uses strategy to assign task
func (c *Coordinator) AutoAssign(ctx context.Context, taskID uuid.UUID, strategy StrategyType) (*uuid.UUID, error) {
	c.mu.Lock()
	task, ok := c.tasks[taskID]
	c.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("task not found")
	}

	// Get available agents
	c.mu.RLock()
	var available []*Agent
	for _, a := range c.agents {
		if a.Status == AgentIdle {
			available = append(available, a)
		}
	}
	c.mu.RUnlock()

	if len(available) == 0 {
		return nil, fmt.Errorf("no available agents")
	}

	// Select agent using strategy
	strategyImpl, ok := c.strategies[strategy]
	if !ok {
		strategyImpl = c.strategies[StrategyLeastLoaded]
	}

	agentID := strategyImpl.SelectAgent(available, task)
	if agentID == nil {
		return nil, fmt.Errorf("strategy could not select agent")
	}

	// Assign
	if err := c.AssignTask(ctx, taskID, *agentID); err != nil {
		return nil, err
	}

	return agentID, nil
}

// CompleteTask marks task as done
func (c *Coordinator) CompleteTask(taskID uuid.UUID, result interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	task, ok := c.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found")
	}

	now := time.Now()
	task.CompletedAt = &now
	task.Status = TaskCompleted
	task.Result = result

	// Free agent
	if task.AssignedTo != nil {
		if agent, ok := c.agents[*task.AssignedTo]; ok {
			agent.Status = AgentIdle
		}
	}

	return nil
}

// UpdateHeartbeat updates agent last seen
func (c *Coordinator) UpdateHeartbeat(agentID uuid.UUID) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	agent, ok := c.agents[agentID]
	if !ok {
		return fmt.Errorf("agent not found")
	}

	agent.LastSeen = time.Now()
	return nil
}

// RoundRobinStrategy assigns to agents in rotation
type RoundRobinStrategy struct {
	current int
	mu      sync.Mutex
}

func (s *RoundRobinStrategy) Name() string { return "round_robin" }

func (s *RoundRobinStrategy) SelectAgent(agents []*Agent, task *Task) *uuid.UUID {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(agents) == 0 {
		return nil
	}

	agent := agents[s.current%len(agents)]
	s.current++

	return &agent.ID
}

// LeastLoadedStrategy assigns to agent with fewest tasks
type LeastLoadedStrategy struct{}

func (s *LeastLoadedStrategy) Name() string { return "least_loaded" }

func (s *LeastLoadedStrategy) SelectAgent(agents []*Agent, task *Task) *uuid.UUID {
	if len(agents) == 0 {
		return nil
	}

	// Simple: return first available
	// Real implementation would count tasks per agent
	return &agents[0].ID
}

// RandomStrategy assigns randomly
type RandomStrategy struct{}

func (s *RandomStrategy) Name() string { return "random" }

func (s *RandomStrategy) SelectAgent(agents []*Agent, task *Task) *uuid.UUID {
	if len(agents) == 0 {
		return nil
	}

	// Simple: return first
	// Would use rand in real implementation
	return &agents[0].ID
}
