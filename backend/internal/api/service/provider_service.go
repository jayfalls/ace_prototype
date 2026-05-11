package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"ace/internal/api/model"
	db "ace/internal/api/repository/generated"
	"ace/internal/crypto"
)

// ErrProviderNotFound is returned when a provider is not found.
var ErrProviderNotFound = errors.New("provider not found")

// ErrDuplicateName is returned when a provider name already exists.
var ErrDuplicateName = errors.New("provider name already exists")

// ProviderService handles provider business logic and encryption.
type ProviderService struct {
	queries   *db.Queries
	masterKey []byte
}

// NewProviderService creates a new ProviderService.
func NewProviderService(queries *db.Queries, masterKey []byte) (*ProviderService, error) {
	if queries == nil {
		return nil, errors.New("queries is required")
	}
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes, got %d", len(masterKey))
	}

	return &ProviderService{
		queries:   queries,
		masterKey: masterKey,
	}, nil
}

// CreateProvider validates, encrypts the API key, and stores a new provider.
func (s *ProviderService) CreateProvider(ctx context.Context, req model.ProviderCreateRequest) (*model.ProviderResponse, error) {
	if err := s.validateCreateRequest(req); err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	id := uuid.New().String()
	isEnabled := int64(1)

	// Encrypt the API key if required
	var encField crypto.EncryptedField
	if model.IsAPIKeyRequired(req.ProviderType) {
		var err error
		encField, err = crypto.EncryptField(req.APIKey, s.masterKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt API key: %w", err)
		}
	}
	// For types without API key, encrypted fields stay zero/nil (provider can't actually call APIs).

	configJSON := "{}"
	if req.ConfigJSON != nil {
		data, err := json.Marshal(req.ConfigJSON)
		if err != nil {
			return nil, fmt.Errorf("marshal config_json: %w", err)
		}
		configJSON = string(data)
	}

	dbProvider, err := s.queries.CreateProvider(ctx, db.CreateProviderParams{
		ID:                id,
		Name:              req.Name,
		ProviderType:      string(req.ProviderType),
		BaseUrl:           req.BaseURL,
		EncryptedApiKey:   encField.Ciphertext,
		ApiKeyNonce:       encField.Nonce,
		EncryptedDek:      encField.EncryptedDEK,
		DekNonce:          encField.DEKNonce,
		EncryptionVersion: int64(encField.EncryptionVersion),
		ConfigJson:        configJSON,
		IsEnabled:         isEnabled,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		// SQLite UNIQUE constraint violation
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrDuplicateName
		}
		return nil, fmt.Errorf("create provider: %w", err)
	}

	return s.convertDBProviderToResponse(dbProvider), nil
}

// GetProvider returns a single provider by ID.
func (s *ProviderService) GetProvider(ctx context.Context, id string) (*model.ProviderResponse, error) {
	dbProvider, err := s.queries.GetProvider(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, ErrProviderNotFound
		}
		return nil, fmt.Errorf("get provider: %w", err)
	}
	return s.convertDBProviderToResponse(dbProvider), nil
}

// ListProviders returns all providers, with masked API keys.
func (s *ProviderService) ListProviders(ctx context.Context) ([]model.ProviderResponse, error) {
	dbProviders, err := s.queries.ListProviders(ctx)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}

	responses := make([]model.ProviderResponse, len(dbProviders))
	for i, p := range dbProviders {
		responses[i] = *s.convertDBProviderToResponse(p)
	}
	return responses, nil
}

// UpdateProvider performs a partial update on a provider.
func (s *ProviderService) UpdateProvider(ctx context.Context, id string, req model.ProviderUpdateRequest) (*model.ProviderResponse, error) {
	existing, err := s.queries.GetProvider(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, ErrProviderNotFound
		}
		return nil, fmt.Errorf("get provider for update: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	params := db.UpdateProviderParams{
		Name:              existing.Name,
		BaseUrl:           existing.BaseUrl,
		EncryptedApiKey:   existing.EncryptedApiKey,
		ApiKeyNonce:       existing.ApiKeyNonce,
		EncryptedDek:      existing.EncryptedDek,
		DekNonce:          existing.DekNonce,
		EncryptionVersion: existing.EncryptionVersion,
		ConfigJson:        existing.ConfigJson,
		IsEnabled:         existing.IsEnabled,
		UpdatedAt:         now,
		ID:                id,
	}

	if req.Name != nil {
		params.Name = *req.Name
	}
	if req.BaseURL != nil {
		params.BaseUrl = *req.BaseURL
	}
	if req.APIKey != nil {
		encField, err := crypto.EncryptField(*req.APIKey, s.masterKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt API key: %w", err)
		}
		params.EncryptedApiKey = encField.Ciphertext
		params.ApiKeyNonce = encField.Nonce
		params.EncryptedDek = encField.EncryptedDEK
		params.DekNonce = encField.DEKNonce
		params.EncryptionVersion = int64(encField.EncryptionVersion)
	}
	if req.ConfigJSON != nil {
		data, err := json.Marshal(*req.ConfigJSON)
		if err != nil {
			return nil, fmt.Errorf("marshal config_json: %w", err)
		}
		params.ConfigJson = string(data)
	}
	if req.IsEnabled != nil {
		if *req.IsEnabled {
			params.IsEnabled = 1
		} else {
			params.IsEnabled = 0
		}
	}

	dbProvider, err := s.queries.UpdateProvider(ctx, params)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrDuplicateName
		}
		return nil, fmt.Errorf("update provider: %w", err)
	}

	return s.convertDBProviderToResponse(dbProvider), nil
}

// DeleteProvider removes a provider by ID.
func (s *ProviderService) DeleteProvider(ctx context.Context, id string) error {
	_, err := s.queries.GetProvider(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return ErrProviderNotFound
		}
		return fmt.Errorf("get provider for delete: %w", err)
	}

	if err := s.queries.DeleteProvider(ctx, id); err != nil {
		return fmt.Errorf("delete provider: %w", err)
	}
	return nil
}

// validateCreateRequest validates the required fields for creating a provider.
func (s *ProviderService) validateCreateRequest(req model.ProviderCreateRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if len(req.Name) > 255 {
		return fmt.Errorf("name must be at most 255 characters")
	}
	if !model.IsValidProviderType(string(req.ProviderType)) {
		return fmt.Errorf("invalid provider_type: %s", req.ProviderType)
	}
	if strings.TrimSpace(req.BaseURL) == "" {
		return fmt.Errorf("base_url is required")
	}
	if _, err := url.ParseRequestURI(req.BaseURL); err != nil {
		return fmt.Errorf("invalid base_url: %w", err)
	}
	if model.IsAPIKeyRequired(req.ProviderType) && strings.TrimSpace(req.APIKey) == "" {
		return fmt.Errorf("api_key is required for provider type %s", req.ProviderType)
	}
	return nil
}

// convertDBProviderToResponse converts a database Provider model to a response DTO.
// The API key is never decrypted; only a masked version is returned.
func (s *ProviderService) convertDBProviderToResponse(dbProvider *db.Provider) *model.ProviderResponse {
	var configJSON map[string]interface{}
	if err := json.Unmarshal([]byte(dbProvider.ConfigJson), &configJSON); err != nil {
		configJSON = make(map[string]interface{})
	}

	return &model.ProviderResponse{
		ID:           dbProvider.ID,
		Name:         dbProvider.Name,
		ProviderType: model.ProviderType(dbProvider.ProviderType),
		BaseURL:      dbProvider.BaseUrl,
		APIKeyMasked: maskAPIKey(dbProvider.EncryptedApiKey),
		ConfigJSON:   configJSON,
		IsEnabled:    dbProvider.IsEnabled == 1,
		CreatedAt:    dbProvider.CreatedAt,
		UpdatedAt:    dbProvider.UpdatedAt,
	}
}

// maskAPIKey returns a masked version of the API key showing only the last 4 characters.
// Since the stored value is encrypted binary, we can't derive the real key length.
// Instead, we return "****" for the masked key since we never decrypt it here.
//
// For the actual mask, when we have the plaintext (which we don't in convertDBProviderToResponse),
// the format is "...XXXX" with last 4 chars. Since we intentionally never decrypt in this path,
// we return a consistent "****" mask. The service could decrypt to produce the proper mask,
// but by design we avoid decryption for read-only listing.
func maskAPIKey(encryptedBytes []byte) string {
	if len(encryptedBytes) == 0 {
		return ""
	}
	return "****"
}

// maskPlaintextAPIKey masks a plaintext API key showing "..." prefix + last 4 characters.
// Used when the key has been decrypted (e.g., during create/update response).
func maskPlaintextAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 4 {
		return "****"
	}
	last4 := key[len(key)-4:]
	return "..." + last4
}

// DecryptAndMaskAPIKey decrypts the encrypted API key and returns the properly masked version.
// This is useful when we want the actual last-4-char mask instead of "****".
func (s *ProviderService) DecryptAndMaskAPIKey(dbProvider *db.Provider) string {
	if len(dbProvider.EncryptedApiKey) == 0 {
		return ""
	}

	field := crypto.EncryptedField{
		Ciphertext:        dbProvider.EncryptedApiKey,
		Nonce:             dbProvider.ApiKeyNonce,
		EncryptedDEK:      dbProvider.EncryptedDek,
		DEKNonce:          dbProvider.DekNonce,
		EncryptionVersion: int(dbProvider.EncryptionVersion),
	}

	plaintext, err := crypto.DecryptField(field, s.masterKey)
	if err != nil {
		return "****"
	}

	return maskPlaintextAPIKey(plaintext)
}
