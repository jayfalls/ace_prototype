package telemetry

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// NewLogger creates a structured JSON logger with dual output (stdout + file).
// The file output uses lumberjack for log rotation.
func NewLogger(serviceName, environment, logDir string) (*zap.Logger, error) {
	// Determine log level based on environment
	level := zapcore.InfoLevel
	if environment == "development" || environment == "dev" {
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

	// Build JSON encoder
	encoder := zapcore.NewJSONEncoder(encoderConfig)

	// Create file output with lumberjack rotation
	var fileSyncer zapcore.WriteSyncer
	if logDir != "" {
		// Ensure log directory exists
		if err := os.MkdirAll(logDir, 0700); err != nil {
			// Fall back to stdout only if we can't create log dir
			fileSyncer = zapcore.AddSync(os.Stdout)
		} else {
			lumberjackLogger := &lumberjack.Logger{
				Filename:   filepath.Join(logDir, "ace.log"),
				MaxSize:    100, // MB
				MaxBackups: 3,
				MaxAge:     28, // days
				Compress:   true,
			}
			fileSyncer = zapcore.AddSync(lumberjackLogger)
		}
	} else {
		fileSyncer = zapcore.AddSync(os.Stdout)
	}

	// Createtee to write to both stdout and file
	var stdoutSyncer zapcore.WriteSyncer
	if logDir != "" {
		stdoutSyncer = zapcore.NewMultiWriteSyncer(
			zapcore.AddSync(os.Stdout),
			fileSyncer,
		)
	} else {
		stdoutSyncer = zapcore.AddSync(os.Stdout)
	}

	// Build logger with level and dual output
	logger := zap.New(
		zapcore.NewTee(
			zapcore.NewCore(encoder, stdoutSyncer, level),
		),
	)

	// Add service name as global field (mandatory)
	logger = logger.With(
		zap.String("service_name", serviceName),
	)

	return logger, nil
}

// NewLoggerWithStdout creates a logger that only writes to stdout.
func NewLoggerWithStdout(serviceName, environment string) (*zap.Logger, error) {
	// Determine log level based on environment
	level := zapcore.InfoLevel
	if environment == "development" || environment == "dev" {
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

	// Build JSON encoder
	encoder := zapcore.NewJSONEncoder(encoderConfig)

	// Create stdout syncer
	stdoutSyncer := zapcore.AddSync(os.Stdout)

	// Build logger with level
	logger := zap.New(
		zapcore.NewTee(
			zapcore.NewCore(encoder, stdoutSyncer, level),
		),
	)

	// Add service name as global field (mandatory)
	logger = logger.With(
		zap.String("service_name", serviceName),
	)

	return logger, nil
}
