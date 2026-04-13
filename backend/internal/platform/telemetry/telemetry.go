// Package telemetry provides custom OpenTelemetry initialization with SQLite exporters.
package telemetry

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Config holds telemetry configuration for the platform layer.
type Config struct {
	Mode          string // "embedded" or "external"
	OTLPEndpoint  string // Used when mode is "external"
	ServiceName   string
	Environment   string
	LogDir        string
	PruneInterval time.Duration
}

// Telemetry holds all observability components for the platform.
type Telemetry struct {
	Logger    *zap.Logger
	Tracer    trace.Tracer
	Meter     metric.Meter
	DB        *sql.DB
	Shutdown  func(context.Context) error
	PruneStop func()
}

// Init initializes the telemetry subsystem with SQLite exporters for embedded mode.
func Init(ctx context.Context, cfg *Config, db *sql.DB) (*Telemetry, error) {
	// Create dual-output logger
	logger, err := NewLogger(cfg.ServiceName, cfg.Environment, cfg.LogDir)
	if err != nil {
		return nil, fmt.Errorf("init logger: %w", err)
	}

	var shutdownTrace func(context.Context) error
	var shutdownMetric func(context.Context) error

	// Initialize based on mode
	if cfg.Mode == "embedded" {
		shutdownTrace, shutdownMetric, err = initEmbedded(ctx, cfg, db, logger)
	} else {
		shutdownTrace, shutdownMetric, err = initExternal(ctx, cfg, logger)
	}
	if err != nil {
		logger.Error("failed to initialize telemetry", zap.Error(err))
		return nil, fmt.Errorf("init telemetry: %w", err)
	}

	// Combined shutdown function
	shutdown := func(ctx context.Context) error {
		var errs []error
		if shutdownTrace != nil {
			if err := shutdownTrace(ctx); err != nil {
				errs = append(errs, fmt.Errorf("shutdown tracer: %w", err))
			}
		}
		if shutdownMetric != nil {
			if err := shutdownMetric(ctx); err != nil {
				errs = append(errs, fmt.Errorf("shutdown meter: %w", err))
			}
		}
		if err := logger.Sync(); err != nil {
			errs = append(errs, fmt.Errorf("sync logger: %w", err))
		}
		if len(errs) > 0 {
			return fmt.Errorf("telemetry shutdown errors: %v", errs)
		}
		return nil
	}

	tel := &Telemetry{
		Logger:   logger,
		Tracer:   otel.GetTracerProvider().Tracer(cfg.ServiceName),
		Meter:    otel.GetMeterProvider().Meter(cfg.ServiceName),
		DB:       db,
		Shutdown: shutdown,
	}

	// Start pruning goroutine
	stopPrune := startPruning(ctx, db, cfg.PruneInterval, logger)
	tel.PruneStop = stopPrune

	logger.Info("telemetry initialized",
		zap.String("mode", cfg.Mode),
		zap.String("service_name", cfg.ServiceName),
	)

	return tel, nil
}

// initEmbedded initializes OTel SDK with SQLite exporters.
func initEmbedded(ctx context.Context, cfg *Config, db *sql.DB, logger *zap.Logger) (func(context.Context) error, func(context.Context) error, error) {
	// Create SQLite span exporter
	spanExporter, err := NewSQLiteSpanExporter(db)
	if err != nil {
		return nil, nil, fmt.Errorf("create span exporter: %w", err)
	}

	// Create SQLite metric exporter
	metricExporter, err := NewSQLiteMetricExporter(db)
	if err != nil {
		return nil, nil, fmt.Errorf("create metric exporter: %w", err)
	}

	// Create periodic reader for metrics (exports every 10 seconds)
	metricReader := sdkmetric.NewPeriodicReader(metricExporter,
		sdkmetric.WithInterval(10*time.Second),
	)

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create resource: %w", err)
	}

	// Create trace provider with SQLite exporter
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(spanExporter),
		sdktrace.WithResource(res),
	)

	// Create meter provider with periodic reader
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(metricReader),
		sdkmetric.WithResource(res),
	)

	// Set global providers
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, mp.Shutdown, nil
}

// initExternal initializes OTel SDK with OTLP exporters.
func initExternal(ctx context.Context, cfg *Config, logger *zap.Logger) (func(context.Context) error, func(context.Context) error, error) {
	// Create OTLP trace exporter
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create resource: %w", err)
	}

	// Create trace provider with OTLP exporter
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)

	// Create meter provider (external mode uses default Prometheus)
	mp := sdkmetric.NewMeterProvider()

	// Set global providers
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	logger.Info("external telemetry configured",
		zap.String("otlp_endpoint", cfg.OTLPEndpoint),
	)

	return tp.Shutdown, mp.Shutdown, nil
}

// startPruning starts a goroutine that periodically prunes old telemetry data.
func startPruning(ctx context.Context, db *sql.DB, interval time.Duration, logger *zap.Logger) func() {
	stopCh := make(chan struct{})

	go func() {
		// Run immediately on startup
		pruneAll(ctx, db, logger)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				pruneAll(ctx, db, logger)
			case <-stopCh:
				logger.Info("pruning goroutine stopped")
				return
			case <-ctx.Done():
				logger.Info("pruning goroutine context cancelled")
				return
			}
		}
	}()

	return func() {
		close(stopCh)
	}
}

// pruneAll prunes all telemetry tables.
func pruneAll(ctx context.Context, db *sql.DB, logger *zap.Logger) {
	// Prune spans older than 7 days
	if err := pruneSpans(ctx, db); err != nil {
		logger.Warn("failed to prune spans", zap.Error(err))
	}

	// Prune metrics older than 24 hours
	if err := pruneMetrics(ctx, db); err != nil {
		logger.Warn("failed to prune metrics", zap.Error(err))
	}

	// Prune usage events older than 90 days
	if err := pruneUsageEvents(ctx, db); err != nil {
		logger.Warn("failed to prune usage events", zap.Error(err))
	}
}

// pruneSpans deletes spans older than 7 days.
func pruneSpans(ctx context.Context, db *sql.DB) error {
	result, err := db.ExecContext(ctx, `
		DELETE FROM ott_spans 
		WHERE created_at < datetime('now', '-7 days')
	`)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows > 0 {
		return nil
	}
	return nil
}

// pruneMetrics deletes metrics older than 24 hours.
func pruneMetrics(ctx context.Context, db *sql.DB) error {
	result, err := db.ExecContext(ctx, `
		DELETE FROM ott_metrics 
		WHERE created_at < datetime('now', '-1 day')
	`)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows > 0 {
		return nil
	}
	return nil
}

// pruneUsageEvents deletes usage events older than 90 days.
func pruneUsageEvents(ctx context.Context, db *sql.DB) error {
	result, err := db.ExecContext(ctx, `
		DELETE FROM usage_events 
		WHERE created_at < datetime('now', '-90 days')
	`)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows > 0 {
		return nil
	}
	return nil
}
