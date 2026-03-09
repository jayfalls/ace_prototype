package services

import (
	"context"
	"errors"

	"github.com/ace/framework/backend/internal/models"
	"github.com/google/uuid"
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionAccess  = errors.New("access denied to session")
)

type SessionService struct {
	queries interface {
		CreateSession(ctx context.Context, arg models.CreateSessionParams) (models.Session, error)
		GetSessionByID(ctx context.Context, id uuid.UUID) (models.Session, error)
		ListSessionsByAgent(ctx context.Context, agentID uuid.UUID, limit, offset int32) ([]models.Session, error)
		ListSessionsByOwner(ctx context.Context, ownerID uuid.UUID, limit, offset int32) ([]models.Session, error)
		EndSession(ctx context.Context, arg models.EndSessionParams) (models.Session, error)
		DeleteSession(ctx context.Context, id uuid.UUID) error
	}
}

func NewSessionService(q interface {
	CreateSession(ctx context.Context, arg models.CreateSessionParams) (models.Session, error)
	GetSessionByID(ctx context.Context, id uuid.UUID) (models.Session, error)
	ListSessionsByAgent(ctx context.Context, agentID uuid.UUID, limit, offset int32) ([]models.Session, error)
	ListSessionsByOwner(ctx context.Context, ownerID uuid.UUID, limit, offset int32) ([]models.Session, error)
	EndSession(ctx context.Context, arg models.EndSessionParams) (models.Session, error)
	DeleteSession(ctx context.Context, id uuid.UUID) error
}) *SessionService {
	return &SessionService{queries: q}
}

type CreateSessionInput struct {
	AgentID   uuid.UUID
	OwnerID   uuid.UUID
	Metadata  []byte
}

func (s *SessionService) CreateSession(ctx context.Context, input CreateSessionInput) (models.Session, error) {
	metadata := input.Metadata
	if metadata == nil {
		metadata = []byte("{}")
	}

	return s.queries.CreateSession(ctx, models.CreateSessionParams{
		AgentID:  input.AgentID,
		OwnerID:  input.OwnerID,
		Status:   "active",
		Metadata: metadata,
	})
}

func (s *SessionService) GetSession(ctx context.Context, id, ownerID uuid.UUID) (models.Session, error) {
	session, err := s.queries.GetSessionByID(ctx, id)
	if err != nil {
		return models.Session{}, ErrSessionNotFound
	}

	if session.OwnerID != ownerID {
		return models.Session{}, ErrSessionAccess
	}

	return session, nil
}

func (s *SessionService) ListSessionsByAgent(ctx context.Context, agentID, ownerID uuid.UUID, limit, offset int32) ([]models.Session, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.queries.ListSessionsByAgent(ctx, agentID, limit, offset)
}

func (s *SessionService) ListSessionsByOwner(ctx context.Context, ownerID uuid.UUID, limit, offset int32) ([]models.Session, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.queries.ListSessionsByOwner(ctx, ownerID, limit, offset)
}

func (s *SessionService) EndSession(ctx context.Context, id, ownerID uuid.UUID, status string) (models.Session, error) {
	session, err := s.queries.GetSessionByID(ctx, id)
	if err != nil {
		return models.Session{}, ErrSessionNotFound
	}

	if session.OwnerID != ownerID {
		return models.Session{}, ErrSessionAccess
	}

	if status == "" {
		status = "completed"
	}

	return s.queries.EndSession(ctx, models.EndSessionParams{
		ID:     id,
		Status: status,
	})
}

func (s *SessionService) DeleteSession(ctx context.Context, id, ownerID uuid.UUID) error {
	session, err := s.queries.GetSessionByID(ctx, id)
	if err != nil {
		return ErrSessionNotFound
	}

	if session.OwnerID != ownerID {
		return ErrSessionAccess
	}

	return s.queries.DeleteSession(ctx, id)
}
