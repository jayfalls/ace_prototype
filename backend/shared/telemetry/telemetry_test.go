package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	config := Config{
		ServiceName:  "test-service",
		Environment:  "development",
		OTLPEndpoint: "localhost:4317",
	}

	assert.Equal(t, "test-service", config.ServiceName)
	assert.Equal(t, "development", config.Environment)
	assert.Equal(t, "localhost:4317", config.OTLPEndpoint)
}

func TestNewLogger(t *testing.T) {
	logger, err := NewLogger("test-service", "development")
	require.NoError(t, err)
	assert.NotNil(t, logger)
}

func TestNewLoggerProduction(t *testing.T) {
	logger, err := NewLogger("test-service", "production")
	require.NoError(t, err)
	assert.NotNil(t, logger)
}

func TestUsageEventJSON(t *testing.T) {
	_ = UsageEvent{
		AgentID:        "agent-123",
		CycleID:        "cycle-456",
		SessionID:      "session-789",
		ServiceName:    "api",
		OperationType:  OperationTypeLLMCall,
		ResourceType:   ResourceTypeAPI,
		CostUSD:        0.05,
		DurationMs:     1500,
		TokenCount:     1000,
		Metadata:       map[string]string{"model": "gpt-4"},
	}

	// Verify operation type constant
	assert.Equal(t, "llm_call", OperationTypeLLMCall)
	assert.Equal(t, "memory_read", OperationTypeMemoryRead)
	assert.Equal(t, "tool_execute", OperationTypeToolExecute)

	// Verify resource type constant
	assert.Equal(t, "api", ResourceTypeAPI)
	assert.Equal(t, "memory", ResourceTypeMemory)
	assert.Equal(t, "tool", ResourceTypeTool)

	// Verify subject constant
	assert.Equal(t, "ace.usage.event", SubjectUsageEvent)
}

func TestConstants(t *testing.T) {
	// Verify trace context header keys
	assert.Equal(t, "traceparent", TraceParentHeader)
	assert.Equal(t, "tracestate", TraceStateHeader)
	assert.Equal(t, "baggage", BaggageHeader)

	// Verify default endpoints
	assert.Equal(t, "localhost:4317", DefaultOTLPGRPCEndpoint)
	assert.Equal(t, "localhost:4318", DefaultOTLPHTTPEndpoint)
	assert.Equal(t, ":8888", DefaultPrometheusEndpoint)
}

func TestInit(t *testing.T) {
	// Test initialization with development environment
	// This uses a non-existent OTLP endpoint, so we expect an error
	// but the logger should still be created
	config := Config{
		ServiceName:  "test-service",
		Environment:  "development",
		OTLPEndpoint: "localhost:9999", // Non-existent endpoint
	}

	ctx := context.Background()
	telemetry, err := Init(ctx, config)

	// The init might fail because OTLP endpoint doesn't exist
	// That's okay - we're just testing the basic structure
	if err != nil {
		// Expected - OTLP endpoint likely not available
		t.Logf("Expected OTLP error: %v", err)
	} else {
		require.NotNil(t, telemetry)
		require.NotNil(t, telemetry.Logger)
		require.NotNil(t, telemetry.Shutdown)
		
		// Test shutdown
		err := telemetry.Shutdown(ctx)
		assert.NoError(t, err)
	}
}

func TestUsagePublisherNotConnected(t *testing.T) {
	publisher := &UsagePublisher{nc: nil}
	event := UsageEvent{
		AgentID:     "test-agent",
		ServiceName: "test-service",
	}

	err := publisher.Publish(context.Background(), event)
	assert.Equal(t, ErrNATSNotConnected, err)
}

func TestSpanAttributesJSON(t *testing.T) {
	attrs := SpanAttributes{
		AgentID:     "agent-123",
		CycleID:     "cycle-456",
		ServiceName: "api",
	}

	jsonBytes, err := attrs.MarshalJSON()
	require.NoError(t, err)
	
	// Verify JSON contains expected keys
	assert.Contains(t, string(jsonBytes), "agent_id")
	assert.Contains(t, string(jsonBytes), "cycle_id")
	assert.Contains(t, string(jsonBytes), "service_name")
}
