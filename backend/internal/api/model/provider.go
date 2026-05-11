package model

// ProviderType represents a supported LLM provider type.
type ProviderType string

const (
	ProviderOpenAI     ProviderType = "openai"
	ProviderAnthropic  ProviderType = "anthropic"
	ProviderGoogle     ProviderType = "google"
	ProviderAzure      ProviderType = "azure"
	ProviderBedrock    ProviderType = "bedrock"
	ProviderGroq       ProviderType = "groq"
	ProviderTogether   ProviderType = "together"
	ProviderMistral    ProviderType = "mistral"
	ProviderCohere     ProviderType = "cohere"
	ProviderXAI        ProviderType = "xai"
	ProviderDeepSeek   ProviderType = "deepseek"
	ProviderAlibaba    ProviderType = "alibaba"
	ProviderBaidu      ProviderType = "baidu"
	ProviderByteDance  ProviderType = "bytedance"
	ProviderZhipu      ProviderType = "zhipu"
	Provider01AI       ProviderType = "01ai"
	ProviderNVIDIA     ProviderType = "nvidia"
	ProviderOpenRouter ProviderType = "openrouter"
	ProviderOllama     ProviderType = "ollama"
	ProviderLLamacpp   ProviderType = "llamacpp"
	ProviderCustom     ProviderType = "custom"
)

// ValidProviderTypes is the set of all valid provider types.
var ValidProviderTypes = map[ProviderType]bool{
	ProviderOpenAI:     true,
	ProviderAnthropic:  true,
	ProviderGoogle:     true,
	ProviderAzure:      true,
	ProviderBedrock:    true,
	ProviderGroq:       true,
	ProviderTogether:   true,
	ProviderMistral:    true,
	ProviderCohere:     true,
	ProviderXAI:        true,
	ProviderDeepSeek:   true,
	ProviderAlibaba:    true,
	ProviderBaidu:      true,
	ProviderByteDance:  true,
	ProviderZhipu:      true,
	Provider01AI:       true,
	ProviderNVIDIA:     true,
	ProviderOpenRouter: true,
	ProviderOllama:     true,
	ProviderLLamacpp:   true,
	ProviderCustom:     true,
}

// ProviderTypesWithoutAPIKey is the set of provider types that do not require an API key.
var ProviderTypesWithoutAPIKey = map[ProviderType]bool{
	ProviderOllama:   true,
	ProviderLLamacpp: true,
}

// IsValidProviderType checks whether a provider type string is a valid enum value.
func IsValidProviderType(t string) bool {
	return ValidProviderTypes[ProviderType(t)]
}

// IsAPIKeyRequired returns true if the given provider type requires an API key.
func IsAPIKeyRequired(t ProviderType) bool {
	return !ProviderTypesWithoutAPIKey[t]
}

// ProviderResponse is returned by all provider API endpoints.
type ProviderResponse struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	ProviderType ProviderType           `json:"provider_type"`
	BaseURL      string                 `json:"base_url"`
	APIKeyMasked string                 `json:"api_key_masked"`
	ConfigJSON   map[string]interface{} `json:"config_json"`
	IsEnabled    bool                   `json:"is_enabled"`
	CreatedAt    string                 `json:"created_at"`
	UpdatedAt    string                 `json:"updated_at"`
}

// ProviderCreateRequest is the request body for creating a provider.
type ProviderCreateRequest struct {
	Name         string                 `json:"name"`
	ProviderType ProviderType           `json:"provider_type"`
	BaseURL      string                 `json:"base_url"`
	APIKey       string                 `json:"api_key"`
	ConfigJSON   map[string]interface{} `json:"config_json,omitempty"`
}

// ProviderUpdateRequest is the request body for updating a provider (partial update).
// Pointer fields indicate presence: nil means "don't update".
type ProviderUpdateRequest struct {
	Name       *string                 `json:"name,omitempty"`
	BaseURL    *string                 `json:"base_url,omitempty"`
	APIKey     *string                 `json:"api_key,omitempty"`
	ConfigJSON *map[string]interface{} `json:"config_json,omitempty"`
	IsEnabled  *bool                   `json:"is_enabled,omitempty"`
}
