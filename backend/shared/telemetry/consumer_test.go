package telemetry

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewUsageConsumer(t *testing.T) {
	// Test with nil connection and pool
	consumer := NewUsageConsumer(nil, nil)
	assert.NotNil(t, consumer)
	assert.Nil(t, consumer.sub)
	assert.Nil(t, consumer.pool)
	assert.Nil(t, consumer.logger)
}

func TestNewUsageConsumerWithLogger(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	consumer := NewUsageConsumerWithLogger(nil, nil, logger)
	assert.NotNil(t, consumer)
	assert.NotNil(t, consumer.logger)
}

func TestUsageConsumerStartWithNilNATS(t *testing.T) {
	consumer := NewUsageConsumer(nil, nil)
	err := consumer.Start(context.Background())
	assert.Equal(t, ErrNATSNotConnected, err)
}

func TestUsageConsumerStop(t *testing.T) {
	consumer := NewUsageConsumer(nil, nil)
	// Should not panic when subscription is nil
	consumer.Stop()
	assert.Nil(t, consumer.sub)
}

func TestInsertUsageEventWithNilPool(t *testing.T) {
	consumer := NewUsageConsumer(nil, nil)
	err := consumer.insertUsageEvent(context.Background(), UsageEvent{
		AgentID:       "agent-123",
		CycleID:       "cycle-456",
		SessionID:     "session-789",
		ServiceName:   "api",
		OperationType: OperationTypeLLMCall,
		ResourceType:  ResourceTypeAPI,
	})
	assert.Error(t, err) // Should fail because pool is nil
	assert.Equal(t, ErrDatabaseNotConnected, err)
}

func TestUsageEventJSONParsing(t *testing.T) {
	// Test that the JSON parsing in handleMessage would work correctly
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
		"metadata": {"model": "gpt-4", "region": "us-east-1"}
	}`

	var event UsageEvent
	err := json.Unmarshal([]byte(jsonStr), &event)
	require.NoError(t, err)

	assert.Equal(t, "agent-123", event.AgentID)
	assert.Equal(t, "cycle-456", event.CycleID)
	assert.Equal(t, "session-789", event.SessionID)
	assert.Equal(t, "api", event.ServiceName)
	assert.Equal(t, "llm_call", event.OperationType)
	assert.Equal(t, "api", event.ResourceType)
	assert.Equal(t, 0.05, event.CostUSD)
	assert.Equal(t, int64(1500), event.DurationMs)
	assert.Equal(t, int64(1000), event.TokenCount)
	assert.Equal(t, "gpt-4", event.Metadata["model"])
	assert.Equal(t, "us-east-1", event.Metadata["region"])
}

func TestUsageEventMetadataConversion(t *testing.T) {
	// Test that metadata is correctly converted to JSONB string
	event := UsageEvent{
		AgentID:     "agent-123",
		ServiceName: "api",
		Metadata:    map[string]string{"key1": "value1", "key2": "value2"},
	}

	metadataJSON, err := json.Marshal(event.Metadata)
	require.NoError(t, err)
	assert.Equal(t, `{"key1":"value1","key2":"value2"}`, string(metadataJSON))
}

func TestUsageEventMetadataEmpty(t *testing.T) {
	// Test that nil metadata doesn't cause issues
	event := UsageEvent{
		AgentID:     "agent-123",
		ServiceName: "api",
		Metadata:    nil,
	}

	var metadataJSON []byte
	var err error
	if event.Metadata != nil && len(event.Metadata) > 0 {
		metadataJSON, err = json.Marshal(event.Metadata)
	} else {
		metadataJSON = []byte("{}")
	}

	require.NoError(t, err)
	assert.Equal(t, "{}", string(metadataJSON))
}

func TestErrDatabaseNotConnected(t *testing.T) {
	assert.Equal(t, "database: connection pool not available", ErrDatabaseNotConnected.Error())
}
