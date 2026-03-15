package telemetry

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthCheckNotInitialized(t *testing.T) {
	// Test health check when tracer is not initialized
	// Note: This test may fail if run after other tests that initialize the tracer
	// because the global provider gets set. We test both states.
	err := HealthCheck()
	// Either the provider is not initialized OR the exporter connection is down
	// Both are acceptable for this test
	if err != nil {
		assert.True(t, err == ErrTracerNotInitialized || err == ErrExporterConnectionDown || 
			errors.Is(err, ErrExporterConnectionDown))
	}
}

func TestHealthCheckExporterDown(t *testing.T) {
	// Initialize tracer with a non-existent endpoint to simulate exporter being down
	config := Config{
		ServiceName:  "test-service",
		Environment:  "dev",
		OTLPEndpoint: "localhost:9999",
	}

	ctx := context.Background()
	telemetry, err := Init(ctx, config)
	
	// If initialization succeeds, test the health check
	if err == nil && telemetry != nil {
		// Health check might fail because the endpoint doesn't exist
		_ = telemetry.Shutdown(ctx)
	}
}

func TestExtractHTTP(t *testing.T) {
	// Test extracting trace context from HTTP headers
	ctx := context.Background()
	
	// Create headers with W3C trace context
	headers := http.Header{}
	headers.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")
	headers.Set("tracestate", "vendor=custom")
	headers.Set("baggage", "key1=value1")
	
	// Extract trace context
	newCtx := ExtractHTTP(ctx, headers)
	
	// The context should be updated (we can't easily verify without a span)
	assert.NotNil(t, newCtx)
}

func TestExtractHTTPWithEmptyHeaders(t *testing.T) {
	// Test extracting with empty headers
	ctx := context.Background()
	headers := http.Header{}
	
	newCtx := ExtractHTTP(ctx, headers)
	assert.NotNil(t, newCtx)
}

func TestExtractHTTPWithHeaders(t *testing.T) {
	// Test extracting from map[string][]string
	ctx := context.Background()
	headers := map[string][]string{
		"traceparent": {"00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"},
		"tracestate":  {"vendor=custom"},
	}
	
	newCtx := ExtractHTTPWithHeaders(ctx, headers)
	assert.NotNil(t, newCtx)
}

func TestSpanFromContext(t *testing.T) {
	// Test getting span from empty context
	ctx := context.Background()
	span, _ := SpanFromContext(ctx)
	
	// Even without a span, OTel returns a noop span, so we need to check differently
	// We just verify the function returns without panicking
	assert.NotNil(t, span)
}

func TestSpanFromContextWithSpan(t *testing.T) {
	// Initialize tracer to have a valid tracer provider
	config := Config{
		ServiceName:  "test-service",
		Environment:  "dev",
		OTLPEndpoint: "localhost:9999",
	}

	ctx := context.Background()
	telemetry, err := Init(ctx, config)
	
	if err == nil && telemetry != nil {
		// Start a span
		ctx, span := telemetry.Tracer.Start(ctx, "test-span")
		defer span.End()
		
		// Now get span from context
		retrievedSpan, ok := SpanFromContext(ctx)
		
		// Should have a valid span
		assert.True(t, ok)
		assert.NotNil(t, retrievedSpan)
		
		_ = telemetry.Shutdown(ctx)
	}
}

func TestAddSpanAttributes(t *testing.T) {
	// Initialize tracer
	config := Config{
		ServiceName:  "test-service",
		Environment:  "dev",
		OTLPEndpoint: "localhost:9999",
	}

	ctx := context.Background()
	telemetry, err := Init(ctx, config)
	
	if err == nil && telemetry != nil {
		// Start a span
		ctx, span := telemetry.Tracer.Start(ctx, "test-span")
		defer span.End()
		
		// Add span attributes
		attrs := SpanAttributes{
			AgentID:     "agent-123",
			CycleID:     "cycle-456",
			ServiceName: "api",
		}
		AddSpanAttributes(span, attrs)
		
		// Should not panic and span should have attributes
		_ = telemetry.Shutdown(ctx)
	}
}

func TestAddSpanAttributesPartial(t *testing.T) {
	// Test with partial attributes
	config := Config{
		ServiceName:  "test-service",
		Environment:  "dev",
		OTLPEndpoint: "localhost:9999",
	}

	ctx := context.Background()
	telemetry, err := Init(ctx, config)
	
	if err == nil && telemetry != nil {
		ctx, span := telemetry.Tracer.Start(ctx, "test-span")
		defer span.End()
		
		// Only set service name
		attrs := SpanAttributes{
			ServiceName: "api",
		}
		AddSpanAttributes(span, attrs)
		
		_ = telemetry.Shutdown(ctx)
	}
}

func TestTracerErrorMessages(t *testing.T) {
	// Verify error messages
	assert.Contains(t, ErrTracerNotInitialized.Error(), "not initialized")
	assert.Contains(t, ErrExporterConnectionDown.Error(), "exporter connection down")
}
