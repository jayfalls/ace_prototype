package services

import (
	"context"
	"errors"

	"github.com/ace/framework/backend/internal/models"
	"github.com/google/uuid"
)

var (
	ErrAgentNotFound = errors.New("agent not found")
	ErrAgentAccess   = errors.New("access denied to agent")
)

type AgentService struct {
	queries interface {
		CreateAgent(ctx context.Context, arg models.CreateAgentParams) (models.Agent, error)
		GetAgentByID(ctx context.Context, id uuid.UUID) (models.Agent, error)
		ListAgentsByOwner(ctx context.Context, ownerID uuid.UUID, limit, offset int32) ([]models.Agent, error)
		ListAgentsByStatus(ctx context.Context, ownerID uuid.UUID, status string, limit, offset int32) ([]models.Agent, error)
		UpdateAgent(ctx context.Context, arg models.UpdateAgentParams) (models.Agent, error)
		DeleteAgent(ctx context.Context, id uuid.UUID) error
	}
}

func NewAgentService(q interface {
	CreateAgent(ctx context.Context, arg models.CreateAgentParams) (models.Agent, error)
	GetAgentByID(ctx context.Context, id uuid.UUID) (models.Agent, error)
	ListAgentsByOwner(ctx context.Context, ownerID uuid.UUID, limit, offset int32) ([]models.Agent, error)
	ListAgentsByStatus(ctx context.Context, ownerID uuid.UUID, status string, limit, offset int32) ([]models.Agent, error)
	UpdateAgent(ctx context.Context, arg models.UpdateAgentParams) (models.Agent, error)
	DeleteAgent(ctx context.Context, id uuid.UUID) error
}) *AgentService {
	return &AgentService{queries: q}
}

type CreateAgentInput struct {
	OwnerID     uuid.UUID
	Name        string
	Description *string
	Config      []byte
}

func (s *AgentService) CreateAgent(ctx context.Context, input CreateAgentInput) (models.Agent, error) {
	if input.Name == "" {
		return models.Agent{}, ErrInvalidInput
	}

	config := input.Config
	if config == nil {
		config = []byte("{}")
	}

	return s.queries.CreateAgent(ctx, models.CreateAgentParams{
		OwnerID:     input.OwnerID,
		Name:        input.Name,
		Description: input.Description,
		Config:      config,
		Status:      "inactive",
	})
}

func (s *AgentService) GetAgent(ctx context.Context, id, ownerID uuid.UUID) (models.Agent, error) {
	agent, err := s.queries.GetAgentByID(ctx, id)
	if err != nil {
		return models.Agent{}, ErrAgentNotFound
	}

	if agent.OwnerID != ownerID {
		return models.Agent{}, ErrAgentAccess
	}

	return agent, nil
}

func (s *AgentService) ListAgents(ctx context.Context, ownerID uuid.UUID, status string, limit, offset int32) ([]models.Agent, error) {
	if limit <= 0 {
		limit = 20
	}

	if status != "" {
		return s.queries.ListAgentsByStatus(ctx, ownerID, status, limit, offset)
	}

	return s.queries.ListAgentsByOwner(ctx, ownerID, limit, offset)
}

func (s *AgentService) UpdateAgent(ctx context.Context, id, ownerID uuid.UUID, name, description *string, config []byte, status *string) (models.Agent, error) {
	agent, err := s.queries.GetAgentByID(ctx, id)
	if err != nil {
		return models.Agent{}, ErrAgentNotFound
	}

	if agent.OwnerID != ownerID {
		return models.Agent{}, ErrAgentAccess
	}

	if name == nil {
		name = &agent.Name
	}
	if description == nil {
		description = agent.Description
	}
	if config == nil {
		config = agent.Config
	}
	if status == nil {
		status = &agent.Status
	}

	return s.queries.UpdateAgent(ctx, models.UpdateAgentParams{
		ID:          id,
		Name:        *name,
		Description: description,
		Config:      config,
		Status:      *status,
	})
}

func (s *AgentService) DeleteAgent(ctx context.Context, id, ownerID uuid.UUID) error {
	agent, err := s.queries.GetAgentByID(ctx, id)
	if err != nil {
		return ErrAgentNotFound
	}

	if agent.OwnerID != ownerID {
		return ErrAgentAccess
	}

	return s.queries.DeleteAgent(ctx, id)
}
