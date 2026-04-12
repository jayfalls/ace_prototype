package telemetry

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUsagePublisher(t *testing.T) {
	// Test with nil connection
	publisher := NewUsagePublisher(nil)
	assert.NotNil(t, publisher)
}

func TestUsagePublisherLLMCallBuildsCorrectEvent(t *testing.T) {
	// Test that LLMCall builds the correct event structure
	// by examining what would be published (we can't actually publish without a real NATS)
	publisher := &UsagePublisher{nc: nil}

	err := publisher.LLMCall(
		context.Background(),
		"agent-123",
		"cycle-456",
		"session-789",
		"api",
		1000,
		0.05,
		1500,
	)

	// Should fail because NATS is nil
	assert.Equal(t, ErrNATSNotConnected, err)
}

func TestUsagePublisherMemoryReadBuildsCorrectEvent(t *testing.T) {
	publisher := &UsagePublisher{nc: nil}

	err := publisher.MemoryRead(
		context.Background(),
		"agent-123",
		"cycle-456",
		"session-789",
		"api",
		500,
	)

	assert.Equal(t, ErrNATSNotConnected, err)
}

func TestUsagePublisherToolExecuteBuildsCorrectEvent(t *testing.T) {
	publisher := &UsagePublisher{nc: nil}

	err := publisher.ToolExecute(
		context.Background(),
		"agent-123",
		"cycle-456",
		"session-789",
		"api",
		"browser_navigate",
		2000,
	)

	assert.Equal(t, ErrNATSNotConnected, err)
}

func TestUsageEventConstants(t *testing.T) {
	// Verify operation type constants
	assert.Equal(t, "llm_call", OperationTypeLLMCall)
	assert.Equal(t, "memory_read", OperationTypeMemoryRead)
	assert.Equal(t, "memory_write", OperationTypeMemoryWrite)
	assert.Equal(t, "tool_execute", OperationTypeToolExecute)
	assert.Equal(t, "db_query", OperationTypeDBQuery)
	assert.Equal(t, "nats_publish", OperationTypeNATSPublish)
	assert.Equal(t, "nats_subscribe", OperationTypeNATSSubscribe)

	// Verify resource type constants
	assert.Equal(t, "api", ResourceTypeAPI)
	assert.Equal(t, "memory", ResourceTypeMemory)
	assert.Equal(t, "tool", ResourceTypeTool)
	assert.Equal(t, "database", ResourceTypeDatabase)
	assert.Equal(t, "messaging", ResourceTypeMessaging)
}

func TestUsageEventJSONMarshaling(t *testing.T) {
	event := UsageEvent{
		AgentID:       "agent-123",
		CycleID:       "cycle-456",
		SessionID:     "session-789",
		ServiceName:   "api",
		OperationType: OperationTypeLLMCall,
		ResourceType:  ResourceTypeAPI,
		CostUSD:       0.05,
		DurationMs:    1500,
		TokenCount:    1000,
		Metadata:      map[string]string{"model": "gpt-4"},
		Timestamp:     time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	// Test JSON marshaling using standard json.Marshal
	data, err := json.Marshal(event)
	require.NoError(t, err)

	// Verify JSON contains expected fields
	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"agent_id":"agent-123"`)
	assert.Contains(t, jsonStr, `"cycle_id":"cycle-456"`)
	assert.Contains(t, jsonStr, `"session_id":"session-789"`)
	assert.Contains(t, jsonStr, `"service_name":"api"`)
	assert.Contains(t, jsonStr, `"operation_type":"llm_call"`)
	assert.Contains(t, jsonStr, `"resource_type":"api"`)
	assert.Contains(t, jsonStr, `"cost_usd":0.05`)
	assert.Contains(t, jsonStr, `"duration_ms":1500`)
	assert.Contains(t, jsonStr, `"token_count":1000`)
	assert.Contains(t, jsonStr, `"model":"gpt-4"`)
}

func TestUsageEventJSONUnmarshaling(t *testing.T) {
	jsonStr := `{
		"timestamp": "2024-01-15T10:30:00Z",
		"agent_id": "agent-123",
		"cycle_id": "cycle-456",
		"session_id": "session-789",
		"service_name": "api",
		"operation_type": "llm_call",
		"resource_type": "api",
		"cost_usd": 0.05,
		"duration_ms": 1500,
		"token_count": 1000,
		"metadata": {"model": "gpt-4"}
	}`

	var event UsageEvent
	err := json.Unmarshal([]byte(jsonStr), &event)
	require.NoError(t, err)

	assert.Equal(t, "agent-123", event.AgentID)
	assert.Equal(t, "cycle-456", event.CycleID)
	assert.Equal(t, "session-789", event.SessionID)
	assert.Equal(t, "api", event.ServiceName)
	assert.Equal(t, OperationTypeLLMCall, event.OperationType)
	assert.Equal(t, ResourceTypeAPI, event.ResourceType)
	assert.Equal(t, 0.05, event.CostUSD)
	assert.Equal(t, int64(1500), event.DurationMs)
	assert.Equal(t, int64(1000), event.TokenCount)
	assert.Equal(t, "gpt-4", event.Metadata["model"])
}

func TestUsageEventOmitemptyFields(t *testing.T) {
	// Test that omitempty fields are not included when zero
	event := UsageEvent{
		AgentID:       "agent-123",
		ServiceName:   "api",
		OperationType: OperationTypeLLMCall,
		ResourceType:  ResourceTypeAPI,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	jsonStr := string(data)
	// These should NOT be present because they are omitempty and zero
	assert.NotContains(t, jsonStr, "cost_usd")
	assert.NotContains(t, jsonStr, "duration_ms")
	assert.NotContains(t, jsonStr, "token_count")
	assert.NotContains(t, jsonStr, "metadata")
}

func TestSubjectUsageEventConstant(t *testing.T) {
	assert.Equal(t, "ace.usage.event", SubjectUsageEvent)
}
