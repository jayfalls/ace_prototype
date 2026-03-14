package telemetry

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogFields holds optional logging fields for correlation
type LogFields struct {
	TraceID      string
	SpanID       string
	AgentID      string
	CycleID      string
	SessionID    string
	CorrelationID string
}

// AddFields adds optional fields to a zap.Logger
func (f LogFields) AddFields(logger *zap.Logger) *zap.Logger {
	if f.TraceID != "" {
		logger = logger.With(zap.String("trace_id", f.TraceID))
	}
	if f.SpanID != "" {
		logger = logger.With(zap.String("span_id", f.SpanID))
	}
	if f.AgentID != "" {
		logger = logger.With(zap.String("agent_id", f.AgentID))
	}
	if f.CycleID != "" {
		logger = logger.With(zap.String("cycle_id", f.CycleID))
	}
	if f.SessionID != "" {
		logger = logger.With(zap.String("session_id", f.SessionID))
	}
	if f.CorrelationID != "" {
		logger = logger.With(zap.String("correlation_id", f.CorrelationID))
	}
	return logger
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

	// Add service name as global field (mandatory)
	logger = logger.With(
		zap.String("service_name", serviceName),
	)

	return logger, nil
}

// NewLoggerWithFields creates a structured JSON logger with optional correlation fields
func NewLoggerWithFields(serviceName, environment string, fields LogFields) (*zap.Logger, error) {
	logger, err := NewLogger(serviceName, environment)
	if err != nil {
		return nil, err
	}
	return fields.AddFields(logger), nil
}
