package services

import (
	"context"

	"github.com/ace/framework/backend/internal/models"
	"github.com/google/uuid"
)

type SettingsService struct {
	queries interface {
		GetAgentSetting(ctx context.Context, arg models.GetAgentSettingParams) (models.AgentSetting, error)
		ListAgentSettings(ctx context.Context, agentID uuid.UUID) ([]models.AgentSetting, error)
		UpsertAgentSetting(ctx context.Context, arg models.UpsertAgentSettingParams) (models.AgentSetting, error)
		DeleteAgentSetting(ctx context.Context, arg models.DeleteAgentSettingParams) error
		GetSystemSetting(ctx context.Context, key string) (models.SystemSetting, error)
		ListSystemSettings(ctx context.Context) ([]models.SystemSetting, error)
		UpsertSystemSetting(ctx context.Context, arg models.UpsertSystemSettingParams) (models.SystemSetting, error)
		DeleteSystemSetting(ctx context.Context, key string) error
	}
}

func NewSettingsService(q interface {
	GetAgentSetting(ctx context.Context, arg models.GetAgentSettingParams) (models.AgentSetting, error)
	ListAgentSettings(ctx context.Context, agentID uuid.UUID) ([]models.AgentSetting, error)
	UpsertAgentSetting(ctx context.Context, arg models.UpsertAgentSettingParams) (models.AgentSetting, error)
	DeleteAgentSetting(ctx context.Context, arg models.DeleteAgentSettingParams) error
	GetSystemSetting(ctx context.Context, key string) (models.SystemSetting, error)
	ListSystemSettings(ctx context.Context) ([]models.SystemSetting, error)
	UpsertSystemSetting(ctx context.Context, arg models.UpsertSystemSettingParams) (models.SystemSetting, error)
	DeleteSystemSetting(ctx context.Context, key string) error
}) *SettingsService {
	return &SettingsService{queries: q}
}

func (s *SettingsService) GetAgentSetting(ctx context.Context, agentID uuid.UUID, key string) (string, error) {
	setting, err := s.queries.GetAgentSetting(ctx, models.GetAgentSettingParams{
		AgentID: agentID,
		Key:     key,
	})
	if err != nil {
		return "", nil // Return empty string if not found
	}
	return setting.Value, nil
}

func (s *SettingsService) ListAgentSettings(ctx context.Context, agentID uuid.UUID) ([]models.AgentSetting, error) {
	return s.queries.ListAgentSettings(ctx, agentID)
}

func (s *SettingsService) SetAgentSetting(ctx context.Context, agentID uuid.UUID, key, value string) (models.AgentSetting, error) {
	return s.queries.UpsertAgentSetting(ctx, models.UpsertAgentSettingParams{
		AgentID: agentID,
		Key:     key,
		Value:   value,
	})
}

func (s *SettingsService) DeleteAgentSetting(ctx context.Context, agentID uuid.UUID, key string) error {
	return s.queries.DeleteAgentSetting(ctx, models.DeleteAgentSettingParams{
		AgentID: agentID,
		Key:     key,
	})
}

func (s *SettingsService) GetSystemSetting(ctx context.Context, key string) (string, error) {
	setting, err := s.queries.GetSystemSetting(ctx, key)
	if err != nil {
		return "", nil
	}
	return setting.Value, nil
}

func (s *SettingsService) ListSystemSettings(ctx context.Context) ([]models.SystemSetting, error) {
	return s.queries.ListSystemSettings(ctx)
}

func (s *SettingsService) SetSystemSetting(ctx context.Context, key, value string) (models.SystemSetting, error) {
	return s.queries.UpsertSystemSetting(ctx, models.UpsertSystemSettingParams{
		Key:   key,
		Value: value,
	})
}

func (s *SettingsService) DeleteSystemSetting(ctx context.Context, key string) error {
	return s.queries.DeleteSystemSetting(ctx, key)
}
