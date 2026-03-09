package services

import (
	"context"

	"github.com/ace/framework/backend/internal/models"
	"github.com/google/uuid"
)

type ThoughtService struct {
	queries interface {
		CreateThought(ctx context.Context, arg models.CreateThoughtParams) (models.Thought, error)
		GetThoughtByID(ctx context.Context, id uuid.UUID) (models.Thought, error)
		ListThoughtsBySession(ctx context.Context, sessionID uuid.UUID, limit, offset int32) ([]models.Thought, error)
		ListThoughtsByLayer(ctx context.Context, sessionID uuid.UUID, layer string, limit, offset int32) ([]models.Thought, error)
		DeleteThought(ctx context.Context, id uuid.UUID) error
	}
}

func NewThoughtService(q interface {
	CreateThought(ctx context.Context, arg models.CreateThoughtParams) (models.Thought, error)
	GetThoughtByID(ctx context.Context, id uuid.UUID) (models.Thought, error)
	ListThoughtsBySession(ctx context.Context, sessionID uuid.UUID, limit, offset int32) ([]models.Thought, error)
	ListThoughtsByLayer(ctx context.Context, sessionID uuid.UUID, layer string, limit, offset int32) ([]models.Thought, error)
	DeleteThought(ctx context.Context, id uuid.UUID) error
}) *ThoughtService {
	return &ThoughtService{queries: q}
}

type CreateThoughtInput struct {
	SessionID uuid.UUID
	Layer     string
	Content   string
	Metadata  []byte
}

func (s *ThoughtService) CreateThought(ctx context.Context, input CreateThoughtInput) (models.Thought, error) {
	metadata := input.Metadata
	if metadata == nil {
		metadata = []byte("{}")
	}

	return s.queries.CreateThought(ctx, models.CreateThoughtParams{
		SessionID: input.SessionID,
		Layer:     input.Layer,
		Content:   input.Content,
		Metadata:  metadata,
	})
}

func (s *ThoughtService) ListThoughts(ctx context.Context, sessionID uuid.UUID, layer string, limit, offset int32) ([]models.Thought, error) {
	if limit <= 0 {
		limit = 100
	}

	if layer != "" {
		return s.queries.ListThoughtsByLayer(ctx, sessionID, layer, limit, offset)
	}

	return s.queries.ListThoughtsBySession(ctx, sessionID, limit, offset)
}
