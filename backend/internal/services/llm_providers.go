package services

import (
	"context"
	"errors"

	"github.com/ace/framework/backend/internal/llm"
	"github.com/ace/framework/backend/internal/models"
	"github.com/google/uuid"
)

var (
	ErrProviderNotFound = errors.New("provider not found")
	ErrProviderAccess  = errors.New("access denied to provider")
)

type TestProviderInput struct {
	ProviderType string
	APIKey       *string
	BaseURL      *string
	Model        *string
}

type LLMProviderService struct {
	queries interface {
		CreateLLMProvider(ctx context.Context, arg models.CreateLLMProviderParams) (models.LLMProvider, error)
		GetLLMProviderByID(ctx context.Context, id uuid.UUID) (models.LLMProvider, error)
		ListLLMProvidersByOwner(ctx context.Context, ownerID uuid.UUID) ([]models.LLMProvider, error)
		UpdateLLMProvider(ctx context.Context, arg models.UpdateLLMProviderParams) (models.LLMProvider, error)
		DeleteLLMProvider(ctx context.Context, id uuid.UUID) error
		CreateLLMAttachment(ctx context.Context, arg models.CreateLLMAttachmentParams) (models.LLMAttachment, error)
		ListLLMAttachmentsByAgent(ctx context.Context, agentID uuid.UUID) ([]models.LLMAttachment, error)
		DeleteLLMAttachment(ctx context.Context, id uuid.UUID) error
	}
}

func NewLLMProviderService(q interface {
	CreateLLMProvider(ctx context.Context, arg models.CreateLLMProviderParams) (models.LLMProvider, error)
	GetLLMProviderByID(ctx context.Context, id uuid.UUID) (models.LLMProvider, error)
	ListLLMProvidersByOwner(ctx context.Context, ownerID uuid.UUID) ([]models.LLMProvider, error)
	UpdateLLMProvider(ctx context.Context, arg models.UpdateLLMProviderParams) (models.LLMProvider, error)
	DeleteLLMProvider(ctx context.Context, id uuid.UUID) error
	CreateLLMAttachment(ctx context.Context, arg models.CreateLLMAttachmentParams) (models.LLMAttachment, error)
	ListLLMAttachmentsByAgent(ctx context.Context, agentID uuid.UUID) ([]models.LLMAttachment, error)
	DeleteLLMAttachment(ctx context.Context, id uuid.UUID) error
}) *LLMProviderService {
	return &LLMProviderService{queries: q}
}

type CreateProviderInput struct {
	OwnerID       uuid.UUID
	Name          string
	ProviderType  string
	APIKey        *string
	BaseURL       *string
	Model         *string
	Config        []byte
}

func (s *LLMProviderService) CreateProvider(ctx context.Context, input CreateProviderInput) (models.LLMProvider, error) {
	if input.Name == "" || input.ProviderType == "" {
		return models.LLMProvider{}, ErrInvalidInput
	}

	config := input.Config
	if config == nil {
		config = []byte("{}")
	}

	// Create the provider
	provider, err := s.queries.CreateLLMProvider(ctx, models.CreateLLMProviderParams{
		OwnerID:         input.OwnerID,
		Name:            input.Name,
		ProviderType:    input.ProviderType,
		APIKeyEncrypted: input.APIKey,
		BaseURL:         input.BaseURL,
		Model:           input.Model,
		Config:          config,
	})
	if err != nil {
		return provider, err
	}

	// Auto-wire to all layers for this provider (default behavior)
	// This creates LLMAttachments for all 6 layers + global loops
	layers := []string{
		"aspirational",
		"global_strategy",
		"agent_model",
		"executive_function",
		"cognitive_control",
		"task_prosecution",
		"global_loop",      // Global loop layer
		"layer_loop",       // Layer loop for each layer
	}

	for i, layer := range layers {
		_, err := s.queries.CreateLLMAttachment(ctx, models.CreateLLMAttachmentParams{
			AgentID:   input.OwnerID, // Use ownerID as default agent context
			ProviderID: provider.ID,
			Layer:     layer,
			Priority:  int32(i),
			Config:    config,
		})
		if err != nil {
			// Log but don't fail - attachments are best-effort
			continue
		}
	}

	return provider, nil
}

func (s *LLMProviderService) GetProvider(ctx context.Context, id, ownerID uuid.UUID) (models.LLMProvider, error) {
	provider, err := s.queries.GetLLMProviderByID(ctx, id)
	if err != nil {
		return models.LLMProvider{}, ErrProviderNotFound
	}

	if provider.OwnerID != ownerID {
		return models.LLMProvider{}, ErrProviderAccess
	}

	return provider, nil
}

func (s *LLMProviderService) ListProviders(ctx context.Context, ownerID uuid.UUID) ([]models.LLMProvider, error) {
	return s.queries.ListLLMProvidersByOwner(ctx, ownerID)
}

func (s *LLMProviderService) UpdateProvider(ctx context.Context, id, ownerID uuid.UUID, name, providerType *string, apiKey, baseURL, model *string, config []byte) (models.LLMProvider, error) {
	provider, err := s.queries.GetLLMProviderByID(ctx, id)
	if err != nil {
		return models.LLMProvider{}, ErrProviderNotFound
	}

	if provider.OwnerID != ownerID {
		return models.LLMProvider{}, ErrProviderAccess
	}

	if name == nil {
		name = &provider.Name
	}
	if providerType == nil {
		providerType = &provider.ProviderType
	}
	if apiKey == nil {
		apiKey = provider.APIKeyEncrypted
	}
	if baseURL == nil {
		baseURL = provider.BaseURL
	}
	if model == nil {
		model = provider.Model
	}
	if config == nil {
		config = provider.Config
	}

	return s.queries.UpdateLLMProvider(ctx, models.UpdateLLMProviderParams{
		ID:              id,
		Name:            *name,
		ProviderType:    *providerType,
		APIKeyEncrypted: apiKey,
		BaseURL:         baseURL,
		Model:           model,
		Config:          config,
	})
}

func (s *LLMProviderService) DeleteProvider(ctx context.Context, id, ownerID uuid.UUID) error {
	provider, err := s.queries.GetLLMProviderByID(ctx, id)
	if err != nil {
		return ErrProviderNotFound
	}

	if provider.OwnerID != ownerID {
		return ErrProviderAccess
	}

	return s.queries.DeleteLLMProvider(ctx, id)
}

func (s *LLMProviderService) CreateAttachment(ctx context.Context, agentID, providerID uuid.UUID, layer string, priority int, config []byte) (models.LLMAttachment, error) {
	if config == nil {
		config = []byte("{}")
	}

	return s.queries.CreateLLMAttachment(ctx, models.CreateLLMAttachmentParams{
		AgentID:  agentID,
		ProviderID: providerID,
		Layer:    layer,
		Priority: int32(priority),
		Config:   config,
	})
}

func (s *LLMProviderService) ListAttachments(ctx context.Context, agentID uuid.UUID) ([]models.LLMAttachment, error) {
	return s.queries.ListLLMAttachmentsByAgent(ctx, agentID)
}

func (s *LLMProviderService) DeleteAttachment(ctx context.Context, id uuid.UUID) error {
	return s.queries.DeleteLLMAttachment(ctx, id)
}

func (s *LLMProviderService) TestProvider(ctx context.Context, input TestProviderInput) error {
	_ = ctx
	apiKey := ""
	if input.APIKey != nil {
		apiKey = *input.APIKey
	}
	baseURL := ""
	if input.BaseURL != nil {
		baseURL = *input.BaseURL
	}
	model := ""
	if input.Model != nil {
		model = *input.Model
	}
	return llm.TestProvider(input.ProviderType, apiKey, baseURL, model)
}
