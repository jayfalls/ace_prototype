package llm

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// SchemaVersion constant
// =============================================================================

func TestSchemaVersion(t *testing.T) {
	assert.Equal(t, "2024-05-04/v1", SchemaVersion)
}

// =============================================================================
// Role validation
// =============================================================================

func TestIsValidRole_DefinedRoles(t *testing.T) {
	tests := []Role{RoleSystem, RoleUser, RoleAssistant, RoleTool}
	for _, r := range tests {
		t.Run(string(r), func(t *testing.T) {
			assert.True(t, IsValidRole(r), "expected %q to be valid", r)
		})
	}
}

func TestIsValidRole_InvalidRoles(t *testing.T) {
	invalid := []Role{"", "admin", "guest", "moderator", "unknown"}
	for _, r := range invalid {
		t.Run(string(r), func(t *testing.T) {
			assert.False(t, IsValidRole(r), "expected %q to be invalid", r)
		})
	}
}

// =============================================================================
// LLMParameters bounds
// =============================================================================

func TestLLMParameters_IsValid_Empty(t *testing.T) {
	p := LLMParameters{}
	assert.True(t, p.IsValid())
}

func TestLLMParameters_IsValid_TemperatureBoundary(t *testing.T) {
	var (
		below  float32 = -0.01
		zero   float32 = 0.0
		mid    float32 = 1.0
		top    float32 = 2.0
		above  float32 = 2.01
	)
	assert.False(t, LLMParameters{Temperature: &below}.IsValid())
	assert.True(t, LLMParameters{Temperature: &zero}.IsValid())
	assert.True(t, LLMParameters{Temperature: &mid}.IsValid())
	assert.True(t, LLMParameters{Temperature: &top}.IsValid())
	assert.False(t, LLMParameters{Temperature: &above}.IsValid())
}

func TestLLMParameters_IsValid_TopPBoundary(t *testing.T) {
	var (
		below float32 = -0.01
		zero  float32 = 0.0
		mid   float32 = 0.5
		top   float32 = 1.0
		above float32 = 1.01
	)
	assert.False(t, LLMParameters{TopP: &below}.IsValid())
	assert.True(t, LLMParameters{TopP: &zero}.IsValid())
	assert.True(t, LLMParameters{TopP: &mid}.IsValid())
	assert.True(t, LLMParameters{TopP: &top}.IsValid())
	assert.False(t, LLMParameters{TopP: &above}.IsValid())
}

func TestLLMParameters_IsValid_MaxTokensBoundary(t *testing.T) {
	var (
		zero     int32 = 0
		negative int32 = -1
		positive int32 = 1
		large    int32 = 128000
	)
	assert.False(t, LLMParameters{MaxTokens: &zero}.IsValid())
	assert.False(t, LLMParameters{MaxTokens: &negative}.IsValid())
	assert.True(t, LLMParameters{MaxTokens: &positive}.IsValid())
	assert.True(t, LLMParameters{MaxTokens: &large}.IsValid())
}

func TestLLMParameters_IsValid_MultipleInvalidFields(t *testing.T) {
	var (
		badTemp float32 = -1.0
		badTopP float32 = 1.5
		badTok  int32   = 0
	)
	p := LLMParameters{
		Temperature: &badTemp,
		TopP:        &badTopP,
		MaxTokens:   &badTok,
	}
	assert.False(t, p.IsValid())
}

// =============================================================================
// JSON round-trip: LLMRequest
// =============================================================================

func TestLLMRequest_JSONRoundTrip_Populated(t *testing.T) {
	var (
		temp  float32 = 0.7
		topP  float32 = 0.9
		maxT  int32   = 4096
		model         = "gpt-4o"
	)
	original := LLMRequest{
		SchemaVersion:   SchemaVersion,
		ProviderGroupID: "550e8400-e29b-41d4-a716-446655440000",
		ModelOverride:   &model,
		SystemPrompt:    "You are a helpful assistant.",
		Messages: []ChatMessage{
			{Role: RoleSystem, Content: "System message"},
			{Role: RoleUser, Content: "Hello"},
		},
		Parameters: LLMParameters{
			Temperature: &temp,
			TopP:        &topP,
			MaxTokens:   &maxT,
			StopSequences: []string{"stop1", "stop2"},
		},
		Stream: false,
		Metadata: map[string]string{
			"agent_id":       "agent-1",
			"correlation_id": "corr-abc",
		},
	}

	raw, err := json.Marshal(original)
	require.NoError(t, err)

	var restored LLMRequest
	err = json.Unmarshal(raw, &restored)
	require.NoError(t, err)

	assert.Equal(t, original.SchemaVersion, restored.SchemaVersion)
	assert.Equal(t, original.ProviderGroupID, restored.ProviderGroupID)
	require.NotNil(t, restored.ModelOverride)
	assert.Equal(t, model, *restored.ModelOverride)
	assert.Equal(t, original.SystemPrompt, restored.SystemPrompt)
	require.Len(t, restored.Messages, 2)
	assert.Equal(t, original.Messages[0].Role, restored.Messages[0].Role)
	assert.Equal(t, original.Messages[0].Content, restored.Messages[0].Content)
	assert.Equal(t, original.Messages[1].Role, restored.Messages[1].Role)
	assert.Equal(t, original.Messages[1].Content, restored.Messages[1].Content)
	require.NotNil(t, restored.Parameters.Temperature)
	assert.InDelta(t, temp, *restored.Parameters.Temperature, 0.001)
	require.NotNil(t, restored.Parameters.TopP)
	assert.InDelta(t, topP, *restored.Parameters.TopP, 0.001)
	require.NotNil(t, restored.Parameters.MaxTokens)
	assert.Equal(t, maxT, *restored.Parameters.MaxTokens)
	require.Len(t, restored.Parameters.StopSequences, 2)
	assert.Equal(t, "stop1", restored.Parameters.StopSequences[0])
	assert.Equal(t, "stop2", restored.Parameters.StopSequences[1])
	assert.False(t, restored.Stream)
	assert.Equal(t, "agent-1", restored.Metadata["agent_id"])
	assert.Equal(t, "corr-abc", restored.Metadata["correlation_id"])
}

func TestLLMRequest_JSONRoundTrip_Minimal(t *testing.T) {
	original := LLMRequest{
		SchemaVersion:   SchemaVersion,
		ProviderGroupID: "550e8400-e29b-41d4-a716-446655440000",
		Messages:        []ChatMessage{},
		Parameters:      LLMParameters{},
	}

	raw, err := json.Marshal(original)
	require.NoError(t, err)

	var restored LLMRequest
	err = json.Unmarshal(raw, &restored)
	require.NoError(t, err)

	assert.Equal(t, original.SchemaVersion, restored.SchemaVersion)
	assert.Equal(t, original.ProviderGroupID, restored.ProviderGroupID)
	assert.Nil(t, restored.ModelOverride)
	assert.Empty(t, restored.SystemPrompt)
	assert.Empty(t, restored.Messages)
	assert.False(t, restored.Stream)
	assert.Nil(t, restored.Metadata)
}

// =============================================================================
// JSON round-trip: LLMResponse
// =============================================================================

func TestLLMResponse_JSONRoundTrip_Success(t *testing.T) {
	original := LLMResponse{
		SchemaVersion:   SchemaVersion,
		Success:         true,
		Text:            "Hello, world!",
		Model:           "gpt-4o",
		ProviderID:      "prov-1",
		ProviderGroupID: "grp-1",
		Usage: TokenUsage{
			InputTokens:  150,
			OutputTokens: 42,
			CachedTokens: 0,
			CostUSD:      0.00126,
		},
		DurationMs: 842,
		RetryCount: 0,
		Error:      nil,
	}

	raw, err := json.Marshal(original)
	require.NoError(t, err)

	var restored LLMResponse
	err = json.Unmarshal(raw, &restored)
	require.NoError(t, err)

	assert.Equal(t, original.SchemaVersion, restored.SchemaVersion)
	assert.True(t, restored.Success)
	assert.Equal(t, original.Text, restored.Text)
	assert.Equal(t, original.Model, restored.Model)
	assert.Equal(t, original.ProviderID, restored.ProviderID)
	assert.Equal(t, original.ProviderGroupID, restored.ProviderGroupID)
	assert.Equal(t, original.Usage.InputTokens, restored.Usage.InputTokens)
	assert.Equal(t, original.Usage.OutputTokens, restored.Usage.OutputTokens)
	assert.Equal(t, original.Usage.CachedTokens, restored.Usage.CachedTokens)
	assert.InDelta(t, original.Usage.CostUSD, restored.Usage.CostUSD, 0.00001)
	assert.Equal(t, original.DurationMs, restored.DurationMs)
	assert.Equal(t, original.RetryCount, restored.RetryCount)
	assert.Nil(t, restored.Error)
}

func TestLLMResponse_JSONRoundTrip_Error(t *testing.T) {
	original := LLMResponse{
		SchemaVersion:   SchemaVersion,
		Success:         false,
		Text:            "",
		Model:           "",
		ProviderID:      "",
		ProviderGroupID: "grp-1",
		Usage:           TokenUsage{},
		DurationMs:      0,
		RetryCount:      2,
		Error: &LLMError{
			Code:      "rate_limited",
			Message:   "RPM limit exceeded",
			Retriable: true,
		},
	}

	raw, err := json.Marshal(original)
	require.NoError(t, err)

	var restored LLMResponse
	err = json.Unmarshal(raw, &restored)
	require.NoError(t, err)

	assert.False(t, restored.Success)
	assert.Empty(t, restored.Text)
	assert.Empty(t, restored.Model)
	assert.Equal(t, original.RetryCount, restored.RetryCount)
	require.NotNil(t, restored.Error)
	assert.Equal(t, "rate_limited", restored.Error.Code)
	assert.Equal(t, "RPM limit exceeded", restored.Error.Message)
	assert.True(t, restored.Error.Retriable)
}

// =============================================================================
// Zero values: marshal without error
// =============================================================================

func TestLLMRequest_Marshal_ZeroValue(t *testing.T) {
	req := LLMRequest{}
	raw, err := json.Marshal(req)
	require.NoError(t, err)

	// Zero-value SchemaVersion is empty string, so the JSON output will contain "".
	// The key is that marshaling itself should not error.
	var restored LLMRequest
	err = json.Unmarshal(raw, &restored)
	require.NoError(t, err)

	assert.Empty(t, restored.SchemaVersion)
	assert.Empty(t, restored.ProviderGroupID)
	assert.Nil(t, restored.ModelOverride)
	assert.Empty(t, restored.SystemPrompt)
	assert.Empty(t, restored.Messages)
	assert.False(t, restored.Stream)
	assert.Nil(t, restored.Metadata)
}

func TestLLMResponse_Marshal_ZeroValue(t *testing.T) {
	resp := LLMResponse{}
	raw, err := json.Marshal(resp)
	require.NoError(t, err)

	var restored LLMResponse
	err = json.Unmarshal(raw, &restored)
	require.NoError(t, err)

	assert.Empty(t, restored.SchemaVersion)
	assert.False(t, restored.Success)
	assert.Empty(t, restored.Text)
	assert.Empty(t, restored.Model)
	assert.Empty(t, restored.ProviderID)
	assert.Empty(t, restored.ProviderGroupID)
	assert.Equal(t, int64(0), restored.DurationMs)
	assert.Equal(t, 0, restored.RetryCount)
	assert.Nil(t, restored.Error)
}

func TestTokenUsage_Marshal_ZeroValue(t *testing.T) {
	usage := TokenUsage{}
	raw, err := json.Marshal(usage)
	require.NoError(t, err)

	var restored TokenUsage
	err = json.Unmarshal(raw, &restored)
	require.NoError(t, err)

	assert.Equal(t, int32(0), restored.InputTokens)
	assert.Equal(t, int32(0), restored.OutputTokens)
	assert.Equal(t, int32(0), restored.CachedTokens)
	assert.InDelta(t, 0.0, restored.CostUSD, 0.00001)
}

// =============================================================================
// omitempty behavior
// =============================================================================

func TestLLMRequest_JSON_OmitEmptyFields(t *testing.T) {
	req := LLMRequest{
		SchemaVersion:   SchemaVersion,
		ProviderGroupID: "grp-1",
		Messages:        []ChatMessage{},
		Parameters:      LLMParameters{},
	}

	raw, err := json.Marshal(req)
	require.NoError(t, err)

	var rawMap map[string]json.RawMessage
	err = json.Unmarshal(raw, &rawMap)
	require.NoError(t, err)

	// model_override, metadata should be absent
	_, hasModelOverride := rawMap["model_override"]
	assert.False(t, hasModelOverride, "model_override should be omitted when nil")

	_, hasMetadata := rawMap["metadata"]
	assert.False(t, hasMetadata, "metadata should be omitted when nil")

	// parameters fields without omitempty would be present even if zero
	// but since our fields are omitempty, check that specific zero pointers are absent
	paramsRaw, hasParams := rawMap["parameters"]
	assert.True(t, hasParams)
	var paramsMap map[string]json.RawMessage
	err = json.Unmarshal(paramsRaw, &paramsMap)
	require.NoError(t, err)
	_, hasTemp := paramsMap["temperature"]
	assert.False(t, hasTemp, "temperature should be omitted when nil")
	_, hasTopP := paramsMap["top_p"]
	assert.False(t, hasTopP, "top_p should be omitted when nil")
}

func TestLLMResponse_JSON_OmitEmptyFields(t *testing.T) {
	resp := LLMResponse{
		SchemaVersion:   SchemaVersion,
		Success:         true,
		Model:           "gpt-4o",
		ProviderID:      "prov-1",
		ProviderGroupID: "grp-1",
	}

	raw, err := json.Marshal(resp)
	require.NoError(t, err)

	var rawMap map[string]json.RawMessage
	err = json.Unmarshal(raw, &rawMap)
	require.NoError(t, err)

	// text and error should be absent when empty/nil
	_, hasText := rawMap["text"]
	assert.False(t, hasText, "text should be omitted when empty")

	// usage is a struct (not pointer), so omitempty does not suppress zero-valued structs.
	// This is expected Go behavior: structs are never considered "empty" for omitempty.
	_, hasUsage := rawMap["usage"]
	assert.True(t, hasUsage, "usage struct is always present even when zero-valued")

	_, hasError := rawMap["error"]
	assert.False(t, hasError, "error should be omitted when nil")
}
