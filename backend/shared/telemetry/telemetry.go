package telemetry

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config holds the telemetry configuration
type Config struct {
	ServiceName   string // Service name (e.g., "api", "agent")
	Environment   string // Environment: "development", "staging", "production"
	OTLPEndpoint  string // OTel Collector endpoint for traces/metrics
}

// LoadConfig loads telemetry configuration from environment variables
func LoadConfig() Config {
	return Config{
		ServiceName:  getEnvString("TELEMETRY_SERVICE_NAME", ""),
		Environment:  getEnvString("TELEMETRY_ENVIRONMENT", "development"),
		OTLPEndpoint: getEnvString("OTLP_ENDPOINT", "localhost:4317"),
	}
}

// getEnvString returns the environment variable or default value.
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Telemetry holds all observability components
type Telemetry struct {
	Tracer   trace.Tracer
	Meter    metric.Meter
	Logger   *zap.Logger
	Usage    *UsagePublisher
	Shutdown func(context.Context) error
}

// Init initializes all telemetry components (traces, metrics, logging)
func Init(ctx context.Context, config Config) (*Telemetry, error) {
	// Initialize logger
	logger, err := NewLogger(config.ServiceName, config.Environment)
	if err != nil {
		return nil, err
	}

	// Initialize tracer
	tracer, shutdownTracer, err := initTracer(ctx, config)
	if err != nil {
		logger.Error("failed to initialize tracer", zap.Error(err))
		return nil, err
	}

	// Initialize meter
	meter, shutdownMeter, err := initMeter(config)
	if err != nil {
		logger.Error("failed to initialize meter", zap.Error(err))
		shutdownTracer(ctx)
		return nil, err
	}

	// Combined shutdown function
	shutdown := func(ctx context.Context) error {
		if err := shutdownTracer(ctx); err != nil {
			logger.Error("failed to shutdown tracer", zap.Error(err))
		}
		if err := shutdownMeter(ctx); err != nil {
			logger.Error("failed to shutdown meter", zap.Error(err))
		}
		logger.Sync()
		return nil
	}

	return &Telemetry{
		Tracer:   tracer,
		Meter:    meter,
		Logger:   logger,
		Usage:    nil, // Will be set when NATS is available
		Shutdown: shutdown,
	}, nil
}

// initTracer initializes the OpenTelemetry tracer
func initTracer(ctx context.Context, config Config) (trace.Tracer, func(context.Context) error, error) {
	// Create OTLP exporter
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(config.OTLPEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, nil, err
	}

	// Create resource with service attributes
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(config.ServiceName),
			semconv.DeploymentEnvironment(config.Environment),
		),
	)
	if err != nil {
		return nil, nil, err
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set global trace provider with W3C propagator
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Tracer(config.ServiceName), tp.Shutdown, nil
}

// initMeter initializes the OpenTelemetry meter with Prometheus exporter
func initMeter(config Config) (metric.Meter, func(context.Context) error, error) {
	// Create Prometheus exporter
	exporter, err := prometheus.New()
	if err != nil {
		return nil, nil, err
	}

	// Create meter provider
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
	)

	// Set global meter provider
	otel.SetMeterProvider(provider)

	return provider.Meter(config.ServiceName), provider.Shutdown, nil
}

// NewLogger creates a structured JSON logger
func NewLogger(serviceName, environment string) (*zap.Logger, error) {
	// Determine log level based on environment
	level := zapcore.InfoLevel
	if environment == "development" {
		level = zapcore.DebugLevel
	}

	// Create encoder config for JSON output
	encoderConfig := zapcore.EncoderConfig{
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
	}

	// Build logger with JSON encoder
	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Encoding:        "json",
		EncoderConfig:   encoderConfig,
		OutputPaths:     []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := config.Build()
	if err != nil {
		return nil, err
	}

	// Add service name and environment as global fields
	logger = logger.With(
		zap.String("service_name", serviceName),
		zap.String("environment", environment),
	)

	return logger, nil
}

// UsagePublisher publishes usage events to NATS
type UsagePublisher struct {
	nc *nats.Conn
}

// NewUsagePublisher creates a new usage event publisher
func NewUsagePublisher(nc *nats.Conn) *UsagePublisher {
	return &UsagePublisher{nc: nc}
}

// Publish emits a usage event to NATS
func (p *UsagePublisher) Publish(ctx context.Context, event UsageEvent) error {
	if p.nc == nil {
		return ErrNATSNotConnected
	}

	event.Timestamp = time.Now().UTC()

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := &nats.Msg{
		Subject: SubjectUsageEvent,
		Data:    data,
	}

	// Inject trace context if available
	InjectTraceContext(ctx, msg)

	return p.nc.PublishMsg(msg)
}

// LLMCall publishes an LLM call usage event
func (p *UsagePublisher) LLMCall(ctx context.Context, agentID, cycleID, sessionID, service string, tokens int64, costUSD float64, durationMs int64) error {
	return p.Publish(ctx, UsageEvent{
		AgentID:       agentID,
		CycleID:       cycleID,
		SessionID:     sessionID,
		ServiceName:   service,
		OperationType: OperationTypeLLMCall,
		ResourceType:  ResourceTypeAPI,
		TokenCount:    tokens,
		CostUSD:       costUSD,
		DurationMs:    durationMs,
	})
}

// MemoryRead publishes a memory read usage event
func (p *UsagePublisher) MemoryRead(ctx context.Context, agentID, cycleID, sessionID, service string, durationMs int64) error {
	return p.Publish(ctx, UsageEvent{
		AgentID:       agentID,
		CycleID:       cycleID,
		SessionID:     sessionID,
		ServiceName:   service,
		OperationType: OperationTypeMemoryRead,
		ResourceType:  ResourceTypeMemory,
		DurationMs:    durationMs,
	})
}

// ToolExecute publishes a tool execution usage event
func (p *UsagePublisher) ToolExecute(ctx context.Context, agentID, cycleID, sessionID, service, toolName string, durationMs int64) error {
	return p.Publish(ctx, UsageEvent{
		AgentID:       agentID,
		CycleID:       cycleID,
		SessionID:     sessionID,
		ServiceName:   service,
		OperationType: OperationTypeToolExecute,
		ResourceType:  ResourceTypeTool,
		DurationMs:    durationMs,
		Metadata:      map[string]string{"tool_name": toolName},
	})
}
