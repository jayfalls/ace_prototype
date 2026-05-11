// Package llm provides normalized request/response types for LLM provider interaction.
// These types are used by the LLM Gateway, provider adapters, and NATS message contracts.
package llm

// SchemaVersion is the current LLM message contract schema version.
// Format: YYYY-MM-DD/vN. Bump when making backward-incompatible changes.
const SchemaVersion = "2024-05-04/v1"

// Role represents the role of a chat participant.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ValidRoles is the set of recognized chat roles.
var ValidRoles = map[Role]bool{
	RoleSystem:    true,
	RoleUser:      true,
	RoleAssistant: true,
	RoleTool:      true,
}

// IsValidRole returns true if the given role is one of the defined constants.
func IsValidRole(r Role) bool {
	return ValidRoles[r]
}

// ChatMessage represents a single message in an LLM conversation.
type ChatMessage struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// LLMParameters holds optional tuning knobs for LLM requests.
// All pointer fields use omitempty to avoid sending zero-valued parameters.
type LLMParameters struct {
	Temperature      *float32 `json:"temperature,omitempty"`
	TopP             *float32 `json:"top_p,omitempty"`
	TopK             *int32   `json:"top_k,omitempty"`
	MaxTokens        *int32   `json:"max_tokens,omitempty"`
	StopSequences    []string `json:"stop_sequences,omitempty"`
	PresencePenalty  *float32 `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`
}

// IsValid checks that parameter values are within acceptable bounds.
// Temperature: 0.0 to 2.0. TopP: 0.0 to 1.0. MaxTokens: positive.
func (p LLMParameters) IsValid() bool {
	if p.Temperature != nil && (*p.Temperature < 0.0 || *p.Temperature > 2.0) {
		return false
	}
	if p.TopP != nil && (*p.TopP < 0.0 || *p.TopP > 1.0) {
		return false
	}
	if p.MaxTokens != nil && *p.MaxTokens <= 0 {
		return false
	}
	return true
}

// LLMRequest is the normalized request payload sent to the LLM Gateway.
type LLMRequest struct {
	SchemaVersion   string            `json:"schema_version"`
	ProviderGroupID string            `json:"provider_group_id"`
	ModelOverride   *string           `json:"model_override,omitempty"`
	SystemPrompt    string            `json:"system_prompt"`
	Messages        []ChatMessage     `json:"messages"`
	Parameters      LLMParameters     `json:"parameters"`
	Stream          bool              `json:"stream"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// TokenUsage captures input/output/cached token counts and computed cost.
type TokenUsage struct {
	InputTokens  int32   `json:"input_tokens"`
	OutputTokens int32   `json:"output_tokens"`
	CachedTokens int32   `json:"cached_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

// LLMError is a normalized error returned from an LLM provider or the gateway.
type LLMError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retriable bool   `json:"retriable"`
}

// LLMResponse is the normalized response payload returned from the LLM Gateway.
type LLMResponse struct {
	SchemaVersion   string     `json:"schema_version"`
	Success         bool       `json:"success"`
	Text            string     `json:"text,omitempty"`
	Model           string     `json:"model"`
	ProviderID      string     `json:"provider_id"`
	ProviderGroupID string     `json:"provider_group_id"`
	Usage           TokenUsage `json:"usage,omitempty"`
	DurationMs      int64      `json:"duration_ms"`
	RetryCount      int        `json:"retry_count"`
	Error           *LLMError  `json:"error,omitempty"`
}
