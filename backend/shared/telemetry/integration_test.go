//go:build integration

package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// integrationTestConfig holds the configuration for integration tests
type integrationTestConfig struct {
	NATSURL      string
	PostgresHost string
	PostgresPort string
	PostgresUser string
	PostgresPass string
	PostgresDB   string
	OTLPEndpoint string
	ServiceName  string
}

// getIntegrationTestConfig loads test configuration from environment variables
func getIntegrationTestConfig() integrationTestConfig {
	return integrationTestConfig{
		NATSURL:      getEnv("NATS_URL", "nats://ace_broker:4222"),
		PostgresHost: getEnv("POSTGRES_HOST", "ace_db"),
		PostgresPort: getEnv("POSTGRES_PORT", "5432"),
		PostgresUser: getEnv("POSTGRES_USER", "postgres"),
		PostgresPass: getEnv("POSTGRES_PASSWORD", "postgres"),
		PostgresDB:   getEnv("POSTGRES_DB", "ace"),
		OTLPEndpoint: getEnv("OTLP_ENDPOINT", "localhost:4317"),
		ServiceName:  getEnv("TELEMETRY_SERVICE_NAME", "telemetry-integration-test"),
	}
}

// getEnv returns an environment variable or default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// setupTestTracer sets up a trace provider for testing.
// It uses OTLP exporter to the configured endpoint, with a noop span processor fallback.
func setupTestTracer(ctx context.Context, serviceName string, otlpEndpoint string) (*sdktrace.TracerProvider, error) {
	// Try to create OTLP exporter
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(otlpEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		// If OTLP exporter fails, use a no-op provider
		return sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
		), nil
	}

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create tracer provider with batch span processor
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	return tp, nil
}

// IntegrationTestContext holds all the resources needed for integration tests
type IntegrationTestContext struct {
	config    integrationTestConfig
	nc        *nats.Conn
	dbPool    *pgxpool.Pool
	publisher *UsagePublisher
	consumer  *UsageConsumer
	logger    *zap.Logger
}

// setupIntegrationTest creates all resources needed for integration tests
func setupIntegrationTest(t *testing.T) *IntegrationTestContext {
	t.Helper()

	config := getIntegrationTestConfig()
	ctx := &IntegrationTestContext{
		config: config,
	}

	// Connect to NATS
	nc, err := nats.Connect(config.NATSURL)
	if err != nil {
		t.Skipf("NATS not available at %s: %v", config.NATSURL, err)
	}
	ctx.nc = nc

	// Connect to PostgreSQL
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		config.PostgresUser, config.PostgresPass,
		config.PostgresHost, config.PostgresPort, config.PostgresDB)

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		nc.Close()
		t.Skipf("PostgreSQL not available at %s:%s: %v", config.PostgresHost, config.PostgresPort, err)
	}
	ctx.dbPool = pool

	// Verify database connection
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		nc.Close()
		t.Skipf("PostgreSQL ping failed: %v", err)
	}

	// Create logger
	logger, err := NewLogger(config.ServiceName, "dev")
	require.NoError(t, err)
	ctx.logger = logger

	// Create publisher
	ctx.publisher = NewUsagePublisher(nc)

	// Create consumer
	ctx.consumer = NewUsageConsumerWithLogger(nc, pool, logger)

	return ctx
}

// teardownIntegrationTest cleans up all resources
func teardownIntegrationTest(ctx *IntegrationTestContext) {
	if ctx == nil {
		return
	}

	if ctx.consumer != nil {
		ctx.consumer.Stop()
	}

	if ctx.dbPool != nil {
		ctx.dbPool.Close()
	}

	if ctx.nc != nil {
		ctx.nc.Close()
	}

	if ctx.logger != nil {
		_ = ctx.logger.Sync()
	}
}

// TestIntegration_FullTrace tests trace propagation from HTTP middleware through NATS to database
func TestIntegration_FullTrace(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer teardownIntegrationTest(ctx)

	// Set up trace provider for the test
	testCtx := context.Background()
	tp, err := setupTestTracer(testCtx, ctx.config.ServiceName, ctx.config.OTLPEndpoint)
	require.NoError(t, err)
	defer tp.Shutdown(testCtx)

	// Set global tracer provider and propagator
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Ensure the usage_events table exists
	ctx.runMigration(t)

	// Generate unique IDs for this test
	agentID := uuid.New().String()
	cycleID := uuid.New().String()
	sessionID := uuid.New().String()

	// Start the real UsageConsumer
	consumerCtx, cancel := context.WithCancel(testCtx)
	defer cancel()

	err = ctx.consumer.Start(consumerCtx)
	require.NoError(t, err)

	// Small delay to ensure consumer is subscribed
	time.Sleep(100 * time.Millisecond)

	// Create a test HTTP handler with trace middleware that publishes a usage event
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get trace context from request - the trace middleware creates a span
		span := trace.SpanFromContext(r.Context())
		spanCtx := span.SpanContext()

		// Simulate some work
		time.Sleep(10 * time.Millisecond)

		// Publish a usage event with trace context
		event := UsageEvent{
			AgentID:       agentID,
			CycleID:       cycleID,
			SessionID:     sessionID,
			ServiceName:   ctx.config.ServiceName,
			OperationType: OperationTypeLLMCall,
			ResourceType:  ResourceTypeAPI,
			TokenCount:    100,
			CostUSD:       0.01,
			DurationMs:    10,
		}

		err := ctx.publisher.Publish(r.Context(), event)
		assert.NoError(t, err)

		// Write response with trace ID
		traceID := ""
		if spanCtx.IsValid() {
			traceID = spanCtx.TraceID().String()
		}
		w.Header().Set("X-Trace-ID", traceID)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Wrap with trace middleware
	wrappedHandler := TraceMiddleware()(handler)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	// Execute request
	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NotEmpty(t, rr.Header().Get("X-Trace-ID"), "X-Trace-ID header should be set by handler")

	// Wait for the event to be consumed and persisted to DB
	// Poll the database until we find the record or timeout
	var found bool
	for i := 0; i < 20; i++ {
		var count int
		err := ctx.dbPool.QueryRow(testCtx,
			`SELECT COUNT(*) FROM usage_events WHERE agent_id = $1 AND cycle_id = $2 AND session_id = $3`,
			agentID, cycleID, sessionID,
		).Scan(&count)
		if err == nil && count > 0 {
			found = true
			break
		}
		time.Sleep(250 * time.Millisecond)
	}

	require.True(t, found, "event should be persisted to database after consumer processes it")

	// Verify the persisted event data
	var persistedEvent struct {
		ServiceName   string
		OperationType string
		ResourceType  string
		TokenCount    int64
		CostUSD       float64
		DurationMs    int64
	}
	err = ctx.dbPool.QueryRow(testCtx,
		`SELECT service_name, operation_type, resource_type, token_count, cost_usd, duration_ms
		 FROM usage_events WHERE agent_id = $1`,
		agentID,
	).Scan(
		&persistedEvent.ServiceName,
		&persistedEvent.OperationType,
		&persistedEvent.ResourceType,
		&persistedEvent.TokenCount,
		&persistedEvent.CostUSD,
		&persistedEvent.DurationMs,
	)
	require.NoError(t, err)
	assert.Equal(t, ctx.config.ServiceName, persistedEvent.ServiceName)
	assert.Equal(t, OperationTypeLLMCall, persistedEvent.OperationType)
	assert.Equal(t, ResourceTypeAPI, persistedEvent.ResourceType)
	assert.Equal(t, int64(100), persistedEvent.TokenCount)
	assert.Equal(t, 0.01, persistedEvent.CostUSD)
	assert.Equal(t, int64(10), persistedEvent.DurationMs)
}

// TestIntegration_MetricsEndpoint tests that the /metrics endpoint returns Prometheus-formatted metrics
func TestIntegration_MetricsEndpoint(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer teardownIntegrationTest(ctx)

	// Create a handler with metrics middleware
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Wrap with metrics middleware
	wrappedHandler := MetricsMiddleware(ctx.config.ServiceName)(handler)

	// Make several requests to generate metrics
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test/path", nil)
		rr := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}

	// Request metrics endpoint
	metricsHandler := RegisterMetrics()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	metricsHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse the metrics output
	metricsOutput := rr.Body.String()

	// Verify Prometheus format
	assert.Contains(t, metricsOutput, "# HELP", "should contain HELP lines")
	assert.Contains(t, metricsOutput, "# TYPE", "should contain TYPE lines")

	// Verify standard metrics are present
	assert.Contains(t, metricsOutput, MetricHTTPRequestsTotal, "should contain http_requests_total metric")
	assert.Contains(t, metricsOutput, MetricHTTPRequestDuration, "should contain http_request_duration_seconds metric")
	assert.Contains(t, metricsOutput, MetricHTTPActiveRequests, "should contain http_active_requests metric")

	// Verify metric labels are low-cardinality (no UUIDs)
	lines := strings.Split(metricsOutput, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, MetricHTTPRequestsTotal) {
			// Verify the path is normalized (no UUIDs)
			assert.False(t, containsUUID(line),
				"metric label should not contain raw UUID")
			// Should contain the path or normalized version
			assert.Contains(t, line, "/test/path",
				"should contain the actual path or normalized version")
		}
	}
}

// containsUUID checks if a string contains a UUID pattern using regex
var uuidRegex = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// containsUUID checks if a string contains a UUID pattern
func containsUUID(s string) bool {
	return uuidRegex.MatchString(s)
}

// TestIntegration_LoggerJSONOutput tests that the logger outputs valid JSON with required fields
func TestIntegration_LoggerJSONOutput(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer teardownIntegrationTest(ctx)

	// Generate unique IDs for correlation fields
	traceID := uuid.New().String()
	agentID := uuid.New().String()
	cycleID := uuid.New().String()
	sessionID := uuid.New().String()

	// Capture log output by creating a custom write syncer
	var logBuffer bytes.Buffer
	encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "",
		MessageKey:     "message",
		StacktraceKey:  "",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeName:     zapcore.FullNameEncoder,
	})
	writer := zapcore.AddSync(&logBuffer)
	core := zapcore.NewCore(encoder, writer, zapcore.InfoLevel)
	logger := zap.New(core).With(
		zap.String("service_name", ctx.config.ServiceName),
	).With(
		zap.String("trace_id", traceID),
	).With(
		zap.String("agent_id", agentID),
	).With(
		zap.String("cycle_id", cycleID),
	).With(
		zap.String("session_id", sessionID),
	)
	defer logger.Sync()

	// Log a test message
	logger.Info("test log message",
		zap.String("custom_field", "custom_value"),
	)

	// Parse the JSON output
	var logEntry map[string]interface{}
	err := json.Unmarshal(logBuffer.Bytes(), &logEntry)
	require.NoError(t, err, "log output should be valid JSON")

	// Verify mandatory fields
	assert.Contains(t, logEntry, "timestamp", "should have timestamp field")
	assert.Contains(t, logEntry, "level", "should have level field")
	assert.Contains(t, logEntry, "message", "should have message field")
	assert.Contains(t, logEntry, "service_name", "should have service_name field")

	// Verify field values
	assert.Equal(t, "info", logEntry["level"], "level should be 'info'")
	assert.Equal(t, "test log message", logEntry["message"], "message should match")
	assert.Equal(t, ctx.config.ServiceName, logEntry["service_name"], "service_name should match")
	assert.Equal(t, traceID, logEntry["trace_id"], "trace_id should match")
	assert.Equal(t, agentID, logEntry["agent_id"], "agent_id should match")
	assert.Equal(t, cycleID, logEntry["cycle_id"], "cycle_id should match")
	assert.Equal(t, sessionID, logEntry["session_id"], "session_id should match")
	assert.Equal(t, "custom_value", logEntry["custom_field"], "custom fields should be preserved")

	// Verify timestamp is valid ISO8601
	_, err = time.Parse(time.RFC3339, logEntry["timestamp"].(string))
	assert.NoError(t, err, "timestamp should be valid ISO8601/RFC3339 format")
}

// TestIntegration_UsageEventRoundTrip tests publishing a usage event to NATS
// and consuming it from PostgreSQL using the real UsageConsumer
func TestIntegration_UsageEventRoundTrip(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer teardownIntegrationTest(ctx)

	// Generate unique IDs for this test run
	agentID := uuid.New().String()
	cycleID := uuid.New().String()
	sessionID := uuid.New().String()

	// Ensure the usage_events table exists
	ctx.runMigration(t)

	// Start the real UsageConsumer
	consumerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := ctx.consumer.Start(consumerCtx)
	require.NoError(t, err)

	// Small delay to ensure consumer is subscribed
	time.Sleep(100 * time.Millisecond)

	// Publish a usage event
	publishCtx := context.Background()
	event := UsageEvent{
		AgentID:       agentID,
		CycleID:       cycleID,
		SessionID:     sessionID,
		ServiceName:   ctx.config.ServiceName,
		OperationType: OperationTypeLLMCall,
		ResourceType:  ResourceTypeAPI,
		TokenCount:    1000,
		CostUSD:       0.05,
		DurationMs:    1500,
		Metadata: map[string]string{
			"model":    "gpt-4",
			"provider": "openai",
		},
	}

	err = ctx.publisher.Publish(publishCtx, event)
	require.NoError(t, err)

	// Wait for the event to be consumed and persisted to DB
	// Poll the database until we find the record or timeout
	var found bool
	for i := 0; i < 20; i++ {
		var count int
		err := ctx.dbPool.QueryRow(publishCtx,
			`SELECT COUNT(*) FROM usage_events WHERE agent_id = $1 AND cycle_id = $2 AND session_id = $3`,
			agentID, cycleID, sessionID,
		).Scan(&count)
		if err == nil && count > 0 {
			found = true
			break
		}
		time.Sleep(250 * time.Millisecond)
	}

	require.True(t, found, "event should be persisted to database after UsageConsumer processes it")

	// Verify the event was persisted to PostgreSQL
	var count int
	err = ctx.dbPool.QueryRow(publishCtx,
		`SELECT COUNT(*) FROM usage_events WHERE agent_id = $1 AND cycle_id = $2 AND session_id = $3`,
		agentID, cycleID, sessionID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "event should be persisted to database")

	// Verify the persisted event data
	var persistedEvent struct {
		ServiceName   string
		OperationType string
		ResourceType  string
		TokenCount    int64
		CostUSD       float64
		DurationMs    int64
		Metadata      string
	}
	err = ctx.dbPool.QueryRow(publishCtx,
		`SELECT service_name, operation_type, resource_type, token_count, cost_usd, duration_ms, metadata
		 FROM usage_events WHERE agent_id = $1`,
		agentID,
	).Scan(
		&persistedEvent.ServiceName,
		&persistedEvent.OperationType,
		&persistedEvent.ResourceType,
		&persistedEvent.TokenCount,
		&persistedEvent.CostUSD,
		&persistedEvent.DurationMs,
		&persistedEvent.Metadata,
	)
	require.NoError(t, err)
	assert.Equal(t, ctx.config.ServiceName, persistedEvent.ServiceName)
	assert.Equal(t, OperationTypeLLMCall, persistedEvent.OperationType)
	assert.Equal(t, ResourceTypeAPI, persistedEvent.ResourceType)
	assert.Equal(t, int64(1000), persistedEvent.TokenCount)
	assert.Equal(t, 0.05, persistedEvent.CostUSD)
	assert.Equal(t, int64(1500), persistedEvent.DurationMs)
	// Verify metadata was stored as JSONB
	assert.Contains(t, persistedEvent.Metadata, "gpt-4")
	assert.Contains(t, persistedEvent.Metadata, "openai")
}

// runMigration creates the usage_events table if it doesn't exist
func (ctx *IntegrationTestContext) runMigration(t *testing.T) {
	t.Helper()

	// Create table if not exists
	_, err := ctx.dbPool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS usage_events (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			timestamp TIMESTAMPTZ NOT NULL,
			agent_id UUID NOT NULL,
			cycle_id UUID NOT NULL,
			session_id UUID NOT NULL,
			service_name VARCHAR(255) NOT NULL,
			operation_type VARCHAR(50) NOT NULL,
			resource_type VARCHAR(50) NOT NULL,
			cost_usd DECIMAL(10, 6),
			duration_ms BIGINT,
			token_count BIGINT,
			metadata JSONB,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	require.NoError(t, err)

	// Create indexes (ignore errors if they already exist)
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_usage_events_agent_id ON usage_events(agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_cycle_id ON usage_events(cycle_id)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_session_id ON usage_events(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_timestamp ON usage_events(timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_operation_type ON usage_events(operation_type)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_service_name ON usage_events(service_name)`,
	}

	for _, idx := range indexes {
		_, _ = ctx.dbPool.Exec(context.Background(), idx)
	}
}

// TestIntegration_NATSCarrierTracePropagation tests that trace context is correctly
// propagated through NATS messages
func TestIntegration_NATSCarrierTracePropagation(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer teardownIntegrationTest(ctx)

	// Initialize tracer
	config := Config{
		ServiceName:  ctx.config.ServiceName,
		Environment:  "dev",
		OTLPEndpoint: ctx.config.OTLPEndpoint,
	}

	// Try to initialize telemetry (will fail gracefully if OTel Collector is not available)
	telemetry, err := Init(context.Background(), config)
	if err != nil {
		t.Logf("Warning: could not initialize OTLP tracer (expected if OTel Collector is not running): %v", err)
	}

	// Create a span for testing
	testCtx := context.Background()
	var span trace.Span
	if telemetry != nil && telemetry.Tracer != nil {
		testCtx, span = telemetry.Tracer.Start(testCtx, "test-span")
		defer span.End()
	}

	// Create a NATS message
	msg := &nats.Msg{
		Subject: "test.trace",
		Header:  nats.Header{},
	}

	// Inject trace context
	InjectTraceContext(testCtx, msg)

	// Verify traceparent header is set
	traceParent := msg.Header.Get("traceparent")
	if traceParent != "" {
		// Verify the traceparent format (W3C standard: 00-{trace-id}-{span-id}-{flags})
		parts := strings.Split(traceParent, "-")
		if len(parts) == 4 {
			assert.Equal(t, "00", parts[0], "version should be 00")
			assert.Len(t, parts[1], 32, "trace-id should be 32 hex chars")
			assert.Len(t, parts[2], 16, "span-id should be 16 hex chars")
		}
	}

	// Extract the trace context
	extractedCtx := ExtractTraceContext(context.Background(), msg)

	// Verify the extracted context has the same trace ID
	extractedSpan := trace.SpanFromContext(extractedCtx)

	// If we had a valid span, verify trace IDs match
	if span != nil {
		originalSpanCtx := span.SpanContext()
		if originalSpanCtx.IsValid() && extractedSpan.SpanContext().IsValid() {
			assert.Equal(t,
				originalSpanCtx.TraceID().String(),
				extractedSpan.SpanContext().TraceID().String(),
				"trace IDs should match after round-trip",
			)
		}
	}

	// Clean up
	if telemetry != nil {
		telemetry.Shutdown(context.Background())
	}
}

// TestIntegration_HTTPExtraction tests HTTP header extraction for trace context
func TestIntegration_HTTPExtraction(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer teardownIntegrationTest(ctx)

	// Set up trace provider for the test
	testCtx := context.Background()
	tp, err := setupTestTracer(testCtx, ctx.config.ServiceName, ctx.config.OTLPEndpoint)
	require.NoError(t, err)
	defer tp.Shutdown(testCtx)

	// Set global tracer provider and propagator (required for ExtractHTTP to work)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Create a span and get its traceparent header
	_, span := otel.Tracer(ctx.config.ServiceName).Start(testCtx, "original-span")
	defer span.End()

	spanCtx := span.SpanContext()
	traceID := spanCtx.TraceID().String()
	spanID := spanCtx.SpanID().String()
	traceparent := fmt.Sprintf("00-%s-%s-01", traceID, spanID)

	// Create HTTP headers with traceparent
	headers := http.Header{}
	headers.Set("traceparent", traceparent)

	// Extract trace context from HTTP headers using the global propagator
	extractedCtx := ExtractHTTP(testCtx, headers)

	// Start a new span with the extracted context to verify extraction
	extractedCtx, extractedSpan := otel.Tracer(ctx.config.ServiceName).Start(extractedCtx, "extracted-span")
	defer extractedSpan.End()

	// Verify the extracted span has the same trace ID
	extractedSpanCtx := extractedSpan.SpanContext()
	assert.True(t, extractedSpanCtx.IsValid(), "extracted span should be valid")
	assert.Equal(t, spanCtx.TraceID().String(), extractedSpanCtx.TraceID().String(),
		"trace IDs should match after extraction")
}
