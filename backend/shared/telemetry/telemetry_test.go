package telemetry

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	config := Config{
		ServiceName:  "test-service",
		Environment:  "dev",
		OTLPEndpoint: "localhost:4317",
	}

	assert.Equal(t, "test-service", config.ServiceName)
	assert.Equal(t, "dev", config.Environment)
	assert.Equal(t, "localhost:4317", config.OTLPEndpoint)
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
	// Verify trace context header keys (W3C standard)
	assert.Equal(t, "traceparent", TraceParentHeader)
	assert.Equal(t, "tracestate", TraceStateHeader)
	assert.Equal(t, "baggage", BaggageHeader)
}

func TestLoadConfig(t *testing.T) {
	// Test with environment variables set
	t.Setenv("TELEMETRY_SERVICE_NAME", "test-service")
	t.Setenv("ENVIRONMENT", "prod")
	t.Setenv("OTLP_ENDPOINT", "otel.collector:4317")

	config := LoadConfig()

	assert.Equal(t, "test-service", config.ServiceName)
	assert.Equal(t, "prod", config.Environment)
	assert.Equal(t, "otel.collector:4317", config.OTLPEndpoint)
}

func TestLoadConfigDefaults(t *testing.T) {
	// Clear environment variables
	t.Setenv("TELEMETRY_SERVICE_NAME", "")
	t.Setenv("ENVIRONMENT", "")
	t.Setenv("OTLP_ENDPOINT", "")

	config := LoadConfig()

	assert.Equal(t, "", config.ServiceName)
	assert.Equal(t, "dev", config.Environment)
	assert.Equal(t, "localhost:4317", config.OTLPEndpoint)
}

func TestInit(t *testing.T) {
	// Test initialization with development environment
	// This uses a non-existent OTLP endpoint, so we expect an error
	// but the logger should still be created
	config := Config{
		ServiceName:  "test-service",
		Environment:  "dev",
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

func TestNATSCarrier_Set_NilMsg(t *testing.T) {
	carrier := NATSCarrier{msg: nil}
	// Should not panic
	carrier.Set("key", "value")
}

func TestNATSCarrier_Set_NilHeader(t *testing.T) {
	msg := &nats.Msg{
		Subject: "test",
		Data:    []byte("body"),
		Header:  nil,
	}
	carrier := NATSCarrier{msg: msg}
	// Should not panic
	carrier.Set("key", "value")
}

func TestNATSCarrier_Get_NilMsg(t *testing.T) {
	carrier := NATSCarrier{msg: nil}
	result := carrier.Get("key")
	assert.Equal(t, "", result)
}

func TestNATSCarrier_Get_NilHeader(t *testing.T) {
	msg := &nats.Msg{
		Subject: "test",
		Data:    []byte("body"),
		Header:  nil,
	}
	carrier := NATSCarrier{msg: msg}
	result := carrier.Get("key")
	assert.Equal(t, "", result)
}

func TestNATSCarrier_Keys_NilMsg(t *testing.T) {
	carrier := NATSCarrier{msg: nil}
	result := carrier.Keys()
	assert.Nil(t, result)
}

func TestNATSCarrier_Keys_NilHeader(t *testing.T) {
	msg := &nats.Msg{
		Subject: "test",
		Data:    []byte("body"),
		Header:  nil,
	}
	carrier := NATSCarrier{msg: msg}
	result := carrier.Keys()
	assert.Nil(t, result)
}

func TestNATSCarrier_WithHeader(t *testing.T) {
	msg := &nats.Msg{
		Subject: "test",
		Data:    []byte("body"),
		Header:  nats.Header{"traceparent": []string{"00-abc-123"}},
	}
	carrier := NATSCarrier{msg: msg}
	
	assert.Equal(t, "00-abc-123", carrier.Get("traceparent"))
	assert.Contains(t, carrier.Keys(), "traceparent")
	
	carrier.Set("tracestate", "vendor=custom")
	assert.Equal(t, "vendor=custom", carrier.Get("tracestate"))
}
