package services

import (
	"context"

	"github.com/ace/framework/backend/internal/models"
	"github.com/google/uuid"
)

type ToolWhitelistService struct {
	queries interface {
		GetToolWhitelist(ctx context.Context, arg models.GetToolWhitelistParams) (models.ToolWhitelist, error)
		ListToolWhitelists(ctx context.Context, agentID uuid.UUID) ([]models.ToolWhitelist, error)
		UpsertToolWhitelist(ctx context.Context, arg models.UpsertToolWhitelistParams) (models.ToolWhitelist, error)
		DeleteToolWhitelist(ctx context.Context, arg models.DeleteToolWhitelistParams) error
	}
}

func NewToolWhitelistService(q interface {
	GetToolWhitelist(ctx context.Context, arg models.GetToolWhitelistParams) (models.ToolWhitelist, error)
	ListToolWhitelists(ctx context.Context, agentID uuid.UUID) ([]models.ToolWhitelist, error)
	UpsertToolWhitelist(ctx context.Context, arg models.UpsertToolWhitelistParams) (models.ToolWhitelist, error)
	DeleteToolWhitelist(ctx context.Context, arg models.DeleteToolWhitelistParams) error
}) *ToolWhitelistService {
	return &ToolWhitelistService{queries: q}
}

func (s *ToolWhitelistService) GetTool(ctx context.Context, agentID uuid.UUID, toolName string) (models.ToolWhitelist, error) {
	return s.queries.GetToolWhitelist(ctx, models.GetToolWhitelistParams{
		AgentID:  agentID,
		ToolName: toolName,
	})
}

func (s *ToolWhitelistService) ListTools(ctx context.Context, agentID uuid.UUID) ([]models.ToolWhitelist, error) {
	return s.queries.ListToolWhitelists(ctx, agentID)
}

func (s *ToolWhitelistService) SetTool(ctx context.Context, agentID uuid.UUID, toolName string, enabled bool, config []byte) (models.ToolWhitelist, error) {
	if config == nil {
		config = []byte("{}")
	}
	return s.queries.UpsertToolWhitelist(ctx, models.UpsertToolWhitelistParams{
		AgentID:  agentID,
		ToolName: toolName,
		Enabled:  enabled,
		Config:   config,
	})
}

func (s *ToolWhitelistService) DeleteTool(ctx context.Context, agentID uuid.UUID, toolName string) error {
	return s.queries.DeleteToolWhitelist(ctx, models.DeleteToolWhitelistParams{
		AgentID:  agentID,
		ToolName: toolName,
	})
}

func (s *ToolWhitelistService) IsToolEnabled(ctx context.Context, agentID uuid.UUID, toolName string) (bool, error) {
	tool, err := s.queries.GetToolWhitelist(ctx, models.GetToolWhitelistParams{
		AgentID:  agentID,
		ToolName: toolName,
	})
	if err != nil {
		// If not found, default to disabled
		return false, nil
	}
	return tool.Enabled, nil
}
