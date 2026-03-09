package services

import (
	"context"
	"errors"

	"github.com/ace/framework/backend/internal/models"
	"github.com/google/uuid"
)

var (
	ErrMemoryNotFound = errors.New("memory not found")
	ErrMemoryAccess  = errors.New("access denied to memory")
)

type MemoryService struct {
	queries interface {
		CreateMemory(ctx context.Context, arg models.CreateMemoryParams) (models.Memory, error)
		GetMemoryByID(ctx context.Context, id uuid.UUID) (models.Memory, error)
		ListMemoriesByOwner(ctx context.Context, ownerID uuid.UUID, limit, offset int32) ([]models.Memory, error)
		ListMemoriesByType(ctx context.Context, ownerID uuid.UUID, memoryType string, limit, offset int32) ([]models.Memory, error)
		ListMemoriesByAgent(ctx context.Context, ownerID, agentID uuid.UUID, limit, offset int32) ([]models.Memory, error)
		SearchMemories(ctx context.Context, ownerID uuid.UUID, query string, limit, offset int32) ([]models.Memory, error)
		UpdateMemory(ctx context.Context, arg models.UpdateMemoryParams) (models.Memory, error)
		DeleteMemory(ctx context.Context, id uuid.UUID) error
	}
}

func NewMemoryService(q interface {
	CreateMemory(ctx context.Context, arg models.CreateMemoryParams) (models.Memory, error)
	GetMemoryByID(ctx context.Context, id uuid.UUID) (models.Memory, error)
	ListMemoriesByOwner(ctx context.Context, ownerID uuid.UUID, limit, offset int32) ([]models.Memory, error)
	ListMemoriesByType(ctx context.Context, ownerID uuid.UUID, memoryType string, limit, offset int32) ([]models.Memory, error)
	ListMemoriesByAgent(ctx context.Context, ownerID, agentID uuid.UUID, limit, offset int32) ([]models.Memory, error)
	SearchMemories(ctx context.Context, ownerID uuid.UUID, query string, limit, offset int32) ([]models.Memory, error)
	UpdateMemory(ctx context.Context, arg models.UpdateMemoryParams) (models.Memory, error)
	DeleteMemory(ctx context.Context, id uuid.UUID) error
}) *MemoryService {
	return &MemoryService{queries: q}
}

type CreateMemoryInput struct {
	OwnerID    uuid.UUID
	AgentID    *uuid.UUID
	Content    string
	MemoryType string
	ParentID   *uuid.UUID
	Tags       []string
	Metadata   []byte
}

func (s *MemoryService) CreateMemory(ctx context.Context, input CreateMemoryInput) (models.Memory, error) {
	if input.Content == "" {
		return models.Memory{}, ErrInvalidInput
	}

	if input.MemoryType == "" {
		input.MemoryType = "general"
	}

	metadata := input.Metadata
	if metadata == nil {
		metadata = []byte("{}")
	}

	return s.queries.CreateMemory(ctx, models.CreateMemoryParams{
		OwnerID:    input.OwnerID,
		AgentID:    input.AgentID,
		Content:    input.Content,
		MemoryType: input.MemoryType,
		ParentID:   input.ParentID,
		Tags:       input.Tags,
		Metadata:   metadata,
	})
}

func (s *MemoryService) GetMemory(ctx context.Context, id, ownerID uuid.UUID) (models.Memory, error) {
	memory, err := s.queries.GetMemoryByID(ctx, id)
	if err != nil {
		return models.Memory{}, ErrMemoryNotFound
	}

	if memory.OwnerID != ownerID {
		return models.Memory{}, ErrMemoryAccess
	}

	return memory, nil
}

func (s *MemoryService) ListMemories(ctx context.Context, ownerID uuid.UUID, memoryType, search string, agentID *uuid.UUID, limit, offset int32) ([]models.Memory, error) {
	if limit <= 0 {
		limit = 20
	}

	if search != "" {
		return s.queries.SearchMemories(ctx, ownerID, search, limit, offset)
	}

	if memoryType != "" {
		return s.queries.ListMemoriesByType(ctx, ownerID, memoryType, limit, offset)
	}

	if agentID != nil {
		return s.queries.ListMemoriesByAgent(ctx, ownerID, *agentID, limit, offset)
	}

	return s.queries.ListMemoriesByOwner(ctx, ownerID, limit, offset)
}

func (s *MemoryService) UpdateMemory(ctx context.Context, id, ownerID uuid.UUID, content, memoryType *string, parentID *uuid.UUID, tags *[]string, metadata []byte) (models.Memory, error) {
	memory, err := s.queries.GetMemoryByID(ctx, id)
	if err != nil {
		return models.Memory{}, ErrMemoryNotFound
	}

	if memory.OwnerID != ownerID {
		return models.Memory{}, ErrMemoryAccess
	}

	if content == nil {
		content = &memory.Content
	}
	if memoryType == nil {
		memoryType = &memory.MemoryType
	}
	if parentID == nil {
		parentID = memory.ParentID
	}
	if tags == nil {
		tags = &memory.Tags
	}
	if metadata == nil {
		metadata = memory.Metadata
	}

	return s.queries.UpdateMemory(ctx, models.UpdateMemoryParams{
		ID:         id,
		Content:    *content,
		MemoryType: *memoryType,
		ParentID:   parentID,
		Tags:       *tags,
		Metadata:   metadata,
	})
}

func (s *MemoryService) DeleteMemory(ctx context.Context, id, ownerID uuid.UUID) error {
	memory, err := s.queries.GetMemoryByID(ctx, id)
	if err != nil {
		return ErrMemoryNotFound
	}

	if memory.OwnerID != ownerID {
		return ErrMemoryAccess
	}

	return s.queries.DeleteMemory(ctx, id)
}
