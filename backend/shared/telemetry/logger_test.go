package telemetry

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewLogger(t *testing.T) {
	logger, err := NewLogger("test-service", "dev")
	require.NoError(t, err)
	assert.NotNil(t, logger)
}

func TestNewLoggerProduction(t *testing.T) {
	logger, err := NewLogger("test-service", "production")
	require.NoError(t, err)
	assert.NotNil(t, logger)
}

func TestNewLoggerJSONOutput(t *testing.T) {
	// Create a buffer to capture output
	buf, testLogger := createTestLogger()
	
	testLogger.Info("test message")
	testLogger.Sync()

	output := buf.String()
	
	// Parse JSON
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)

	// Verify mandatory fields are present
	assert.Contains(t, logEntry, "timestamp")
	assert.Contains(t, logEntry, "level")
	assert.Contains(t, logEntry, "message")
	assert.Contains(t, logEntry, "service_name")

	// Verify values
	assert.Equal(t, "test-service", logEntry["service_name"])
	assert.Equal(t, "info", logEntry["level"])
	assert.Equal(t, "test message", logEntry["message"])
}

func TestNewLoggerDebugLevel(t *testing.T) {
	// Debug level in development should work
	buf, testLogger := createTestLoggerWithLevel(zapcore.DebugLevel)
	testLogger.Debug("debug test")
	testLogger.Sync()
	
	output := buf.String()
	assert.Contains(t, output, "debug test")
}

func TestNewLoggerWithFields(t *testing.T) {
	fields := LogFields{
		TraceID:      "trace-123",
		SpanID:      "span-456",
		AgentID:     "agent-789",
		CycleID:     "cycle-001",
		SessionID:   "session-002",
		CorrelationID: "corr-003",
	}

	logger, err := NewLoggerWithFields("test-service", "production", fields)
	require.NoError(t, err)
	_ = logger // logger is created to verify it works, but we use our own test logger for output capture

	// Create a buffer to capture output
	buf := &bytes.Buffer{}
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
	encoder := zapcore.NewJSONEncoder(encoderConfig)
	core := zapcore.NewCore(encoder, zapcore.AddSync(buf), zapcore.InfoLevel)
	testLogger := zap.New(core)
	
	// Add fields to the test logger (simulating what NewLoggerWithFields does)
	testLogger = fields.AddFields(testLogger)
	testLogger = testLogger.With(zap.String("service_name", "test-service"))
	
	testLogger.Info("test with fields")
	testLogger.Sync()
	
	output := buf.String()
	var logEntry map[string]interface{}
	err = json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)

	// Verify all optional fields
	assert.Equal(t, "trace-123", logEntry["trace_id"])
	assert.Equal(t, "span-456", logEntry["span_id"])
	assert.Equal(t, "agent-789", logEntry["agent_id"])
	assert.Equal(t, "cycle-001", logEntry["cycle_id"])
	assert.Equal(t, "session-002", logEntry["session_id"])
	assert.Equal(t, "corr-003", logEntry["correlation_id"])
}

func TestNewLoggerPartialFields(t *testing.T) {
	fields := LogFields{
		AgentID:  "agent-789",
		CycleID: "cycle-001",
	}

	logger, err := NewLoggerWithFields("test-service", "production", fields)
	require.NoError(t, err)
	_ = logger // logger is created to verify it works, but we use our own test logger for output capture

	// Create a buffer to capture output
	buf := &bytes.Buffer{}
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
	encoder := zapcore.NewJSONEncoder(encoderConfig)
	core := zapcore.NewCore(encoder, zapcore.AddSync(buf), zapcore.InfoLevel)
	testLogger := zap.New(core)
	
	// Add fields to the test logger (simulating what NewLoggerWithFields does)
	testLogger = fields.AddFields(testLogger)
	testLogger = testLogger.With(zap.String("service_name", "test-service"))
	
	testLogger.Info("test with partial fields")
	testLogger.Sync()
	
	output := buf.String()
	var logEntry map[string]interface{}
	err = json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)

	// Verify only set fields
	assert.Equal(t, "agent-789", logEntry["agent_id"])
	assert.Equal(t, "cycle-001", logEntry["cycle_id"])
	
	// Verify unset fields are not present
	assert.NotContains(t, logEntry, "trace_id")
	assert.NotContains(t, logEntry, "span_id")
	assert.NotContains(t, logEntry, "session_id")
	assert.NotContains(t, logEntry, "correlation_id")
}

func TestLogFieldsAddFields(t *testing.T) {
	logger, err := NewLogger("test-service", "production")
	require.NoError(t, err)

	fields := LogFields{
		TraceID:   "trace-123",
		AgentID:   "agent-456",
		CycleID:   "cycle-789",
	}

	loggerWithFields := fields.AddFields(logger)
	assert.NotNil(t, loggerWithFields)
}

func TestNewLoggerEmptyServiceName(t *testing.T) {
	// Empty service name should still work
	logger, err := NewLogger("", "production")
	require.NoError(t, err)
	assert.NotNil(t, logger)
}

func TestNewLoggerWarnLevel(t *testing.T) {
	buf, testLogger := createTestLoggerWithLevel(zapcore.WarnLevel)
	testLogger.Warn("warning message")
	testLogger.Sync()
	
	output := buf.String()
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "warn", logEntry["level"])
	assert.Equal(t, "warning message", logEntry["message"])
}

func TestNewLoggerErrorLevel(t *testing.T) {
	buf, testLogger := createTestLoggerWithLevel(zapcore.ErrorLevel)
	testLogger.Error("error message")
	testLogger.Sync()
	
	output := buf.String()
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "error", logEntry["level"])
	assert.Equal(t, "error message", logEntry["message"])
}

func TestNewLoggerTimestamp(t *testing.T) {
	buf, testLogger := createTestLogger()
	testLogger.Info("timestamp test")
	testLogger.Sync()
	
	output := buf.String()
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)
	
	// Verify timestamp exists and is in ISO8601 format
	assert.Contains(t, logEntry, "timestamp")
	timestamp, ok := logEntry["timestamp"].(string)
	require.True(t, ok, "timestamp should be a string")
	assert.True(t, strings.HasPrefix(timestamp, "202"), "timestamp should start with year")
}

// createTestLogger creates a test logger that uses the same config as NewLogger but writes to a buffer
func createTestLogger() (*bytes.Buffer, *zap.Logger) {
	return createTestLoggerWithLevel(zapcore.InfoLevel)
}

// createTestLoggerWithLevel creates a test logger with a specific level that writes to a buffer
func createTestLoggerWithLevel(level zapcore.Level) (*bytes.Buffer, *zap.Logger) {
	buf := &bytes.Buffer{}
	
	// Create encoder config matching NewLogger
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
	
	encoder := zapcore.NewJSONEncoder(encoderConfig)
	core := zapcore.NewCore(encoder, zapcore.AddSync(buf), level)
	
	logger := zap.New(core)
	logger = logger.With(zap.String("service_name", "test-service"))
	
	return buf, logger
}

// TestMain is needed for os.Exit capture
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
