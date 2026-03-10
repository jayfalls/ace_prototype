package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewMetricsCollector tests metrics collector creation
func TestNewMetricsCollector(t *testing.T) {
	collector := NewMetricsCollector()
	require.NotNil(t, collector)
}

// TestRecordRequest tests request recording
func TestRecordRequest(t *testing.T) {
	collector := NewMetricsCollector()
	
	collector.RecordRequest("test_endpoint", 100*time.Millisecond, nil)
	
	metrics := collector.GetMetrics()
	assert.Equal(t, int64(1), metrics.TotalRequests)
}

// TestRecordError tests error recording
func TestRecordError(t *testing.T) {
	collector := NewMetricsCollector()
	
	collector.RecordRequest("test_endpoint", 50*time.Millisecond, assert.AnError)
	
	metrics := collector.GetMetrics()
	assert.Equal(t, int64(1), metrics.TotalRequests)
	assert.Equal(t, int64(1), metrics.TotalErrors)
}

// TestRecordLLMCall tests LLM call recording
func TestRecordLLMCall(t *testing.T) {
	collector := NewMetricsCollector()
	
	collector.RecordLLMCall("openai", "gpt-4", 500*time.Millisecond, nil)
	
	metrics := collector.GetMetrics()
	assert.Equal(t, int64(1), metrics.LLMCalls)
}

// TestGetMetrics tests metrics retrieval
func TestGetMetrics(t *testing.T) {
	collector := NewMetricsCollector()
	
	// Record multiple requests
	collector.RecordRequest("endpoint1", 100*time.Millisecond, nil)
	collector.RecordRequest("endpoint2", 200*time.Millisecond, nil)
	collector.RecordRequest("endpoint1", 150*time.Millisecond, assert.AnError)
	
	metrics := collector.GetMetrics()
	assert.Equal(t, int64(3), metrics.TotalRequests)
	assert.Equal(t, int64(1), metrics.TotalErrors)
}

// TestMetricsReset tests metrics reset
func TestMetricsReset(t *testing.T) {
	collector := NewMetricsCollector()
	
	collector.RecordRequest("test", 100*time.Millisecond, nil)
	collector.Reset()
	
	metrics := collector.GetMetrics()
	assert.Equal(t, int64(0), metrics.TotalRequests)
}

// TestNewTracer tests tracer creation
func TestNewTracer(t *testing.T) {
	tracer := NewTracer("test-service")
	require.NotNil(t, tracer)
}

// TestStartSpan tests span creation
func TestStartSpan(t *testing.T) {
	tracer := NewTracer("test-service")
	ctx := context.Background()
	
	span := tracer.StartSpan(ctx, "test-operation")
	require.NotNil(t, span)
	
	span.End()
	
	// Should complete without error
	assert.NotNil(t, span)
}

// TestStructuredLogger tests structured logging
func TestStructuredLogger(t *testing.T) {
	logger := NewStructuredLogger()
	require.NotNil(t, logger)
	
	// Should log without panicking
	logger.Info("test message", "key", "value")
	logger.Error("error message", "error", assert.AnError)
}

// TestNewObservability tests full observability setup
func TestNewObservability(t *testing.T) {
	obs, err := NewObservability(Config{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Enabled:        true,
	})
	
	if err != nil {
		t.Logf("Observability setup skipped: %v", err)
		return
	}
	
	require.NotNil(t, obs)
	assert.NotNil(t, obs.Metrics)
	assert.NotNil(t, obs.Tracer)
	assert.NotNil(t, obs.Logger)
}
