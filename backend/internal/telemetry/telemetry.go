package telemetry

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Config holds the telemetry configuration
type Config struct {
	ServiceName  string // Service name (e.g., "api", "agent")
	Environment  string // Environment: "dev" or "prod"
	OTLPEndpoint string // OTel Collector endpoint for traces/metrics
}

// LoadConfig loads telemetry configuration from environment variables
func LoadConfig() Config {
	return Config{
		ServiceName:  getEnvString("TELEMETRY_SERVICE_NAME", ""),
		Environment:  getEnvString("ENVIRONMENT", "dev"),
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

	// Store global reference for health checking
	SetGlobalTraceProvider(tp)

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
