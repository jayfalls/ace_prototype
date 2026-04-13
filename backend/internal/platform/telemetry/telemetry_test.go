package telemetry

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ace/internal/platform/database"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	_ "modernc.org/sqlite"
)

func TestNewSQLiteSpanExporter(t *testing.T) {
	// Create temporary directory for test database
	tmpDir, err := os.MkdirTemp("", "telemetry-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Open test database
	db, err := sql.Open("sqlite", filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ott_spans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trace_id TEXT NOT NULL,
			span_id TEXT NOT NULL,
			parent_span_id TEXT,
			operation_name TEXT NOT NULL,
			service_name TEXT NOT NULL,
			start_time TEXT NOT NULL,
			end_time TEXT NOT NULL,
			duration_ms INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'ok',
			attributes TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Test creating exporter
	exporter, err := NewSQLiteSpanExporter(db)
	if err != nil {
		t.Fatalf("failed to create exporter: %v", err)
	}

	if exporter == nil {
		t.Fatal("expected non-nil exporter")
	}

	// Test shutdown
	err = exporter.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
}

func TestNewSQLiteMetricExporter(t *testing.T) {
	// Create temporary directory for test database
	tmpDir, err := os.MkdirTemp("", "telemetry-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Open test database
	db, err := sql.Open("sqlite", filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ott_metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'counter',
			labels TEXT,
			value REAL NOT NULL,
			timestamp TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Test creating exporter
	exporter, err := NewSQLiteMetricExporter(db)
	if err != nil {
		t.Fatalf("failed to create exporter: %v", err)
	}

	if exporter == nil {
		t.Fatal("expected non-nil exporter")
	}

	// Test temporality
	temporality := exporter.Temporality(metric.InstrumentKindCounter)
	if temporality != metricdata.CumulativeTemporality {
		t.Errorf("expected cumulative temporality, got %v", temporality)
	}

	// Test aggregation
	agg := exporter.Aggregation(metric.InstrumentKindCounter)
	if agg == nil {
		t.Error("expected non-nil aggregation")
	}

	// Test shutdown
	err = exporter.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
}

func TestPruneSpans(t *testing.T) {
	// Create temporary directory for test database
	tmpDir, err := os.MkdirTemp("", "telemetry-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Open test database
	db, err := sql.Open("sqlite", filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ott_spans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trace_id TEXT NOT NULL,
			span_id TEXT NOT NULL,
			parent_span_id TEXT,
			operation_name TEXT NOT NULL,
			service_name TEXT NOT NULL,
			start_time TEXT NOT NULL,
			end_time TEXT NOT NULL,
			duration_ms INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'ok',
			attributes TEXT,
			created_at TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert test spans - one recent, one old
	now := time.Now().Format(time.RFC3339)
	oldTime := time.Now().Add(-8 * 24 * time.Hour).Format(time.RFC3339) // 8 days ago

	_, err = db.Exec(`INSERT INTO ott_spans (trace_id, span_id, operation_name, service_name, start_time, end_time, duration_ms, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"trace1", "span1", "op1", "svc1", now, now, 100, "ok", now)
	if err != nil {
		t.Fatalf("failed to insert recent span: %v", err)
	}

	_, err = db.Exec(`INSERT INTO ott_spans (trace_id, span_id, operation_name, service_name, start_time, end_time, duration_ms, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"trace2", "span2", "op2", "svc2", oldTime, oldTime, 100, "ok", oldTime)
	if err != nil {
		t.Fatalf("failed to insert old span: %v", err)
	}

	// Verify we have 2 spans
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM ott_spans").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count spans: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 spans, got %d", count)
	}

	// Prune old spans
	err = pruneSpans(context.Background(), db)
	if err != nil {
		t.Fatalf("pruneSpans failed: %v", err)
	}

	// Verify we have 1 span (the recent one)
	err = db.QueryRow("SELECT COUNT(*) FROM ott_spans").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count spans after prune: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 span after prune, got %d", count)
	}
}

func TestPruneMetrics(t *testing.T) {
	// Create temporary directory for test database
	tmpDir, err := os.MkdirTemp("", "telemetry-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Open test database
	db, err := sql.Open("sqlite", filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ott_metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'counter',
			labels TEXT,
			value REAL NOT NULL,
			timestamp TEXT NOT NULL,
			created_at TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert test metrics - one recent, one old (using SQLite datetime format for proper comparison)
	now := time.Now().Format("2006-01-02 15:04:05")
	oldTime := time.Now().Add(-48 * time.Hour).Format("2006-01-02 15:04:05") // 48 hours ago (clearly older than 24h threshold)

	_, err = db.Exec(`INSERT INTO ott_metrics (name, type, value, timestamp, created_at) VALUES (?, ?, ?, ?, ?)`,
		"metric1", "counter", 1.0, now, now)
	if err != nil {
		t.Fatalf("failed to insert recent metric: %v", err)
	}

	_, err = db.Exec(`INSERT INTO ott_metrics (name, type, value, timestamp, created_at) VALUES (?, ?, ?, ?, ?)`,
		"metric2", "counter", 1.0, oldTime, oldTime)
	if err != nil {
		t.Fatalf("failed to insert old metric: %v", err)
	}

	// Verify we have 2 metrics
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM ott_metrics").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count metrics: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 metrics, got %d", count)
	}

	// Prune old metrics
	err = pruneMetrics(context.Background(), db)
	if err != nil {
		t.Fatalf("pruneMetrics failed: %v", err)
	}

	// Verify we have 1 metric (the recent one)
	err = db.QueryRow("SELECT COUNT(*) FROM ott_metrics").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count metrics after prune: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 metric after prune, got %d", count)
	}
}

func TestPruneUsageEvents(t *testing.T) {
	// Create temporary directory for test database
	tmpDir, err := os.MkdirTemp("", "telemetry-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Open test database
	db, err := sql.Open("sqlite", filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS usage_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			agent_id TEXT NOT NULL,
			session_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			model TEXT,
			input_tokens INTEGER,
			output_tokens INTEGER,
			cost_usd REAL,
			duration_ms INTEGER,
			metadata TEXT,
			created_at TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert test events - one recent, one old
	now := time.Now().Format(time.RFC3339)
	oldTime := time.Now().Add(-91 * 24 * time.Hour).Format(time.RFC3339) // 91 days ago

	_, err = db.Exec(`INSERT INTO usage_events (agent_id, session_id, event_type, created_at) VALUES (?, ?, ?, ?)`,
		"agent1", "session1", "llm_call", now)
	if err != nil {
		t.Fatalf("failed to insert recent event: %v", err)
	}

	_, err = db.Exec(`INSERT INTO usage_events (agent_id, session_id, event_type, created_at) VALUES (?, ?, ?, ?)`,
		"agent2", "session2", "llm_call", oldTime)
	if err != nil {
		t.Fatalf("failed to insert old event: %v", err)
	}

	// Verify we have 2 events
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM usage_events").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count events: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 events, got %d", count)
	}

	// Prune old events
	err = pruneUsageEvents(context.Background(), db)
	if err != nil {
		t.Fatalf("pruneUsageEvents failed: %v", err)
	}

	// Verify we have 1 event (the recent one)
	err = db.QueryRow("SELECT COUNT(*) FROM usage_events").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count events after prune: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 event after prune, got %d", count)
	}
}

func TestNewLogger(t *testing.T) {
	// Create temporary directory for log file
	tmpDir, err := os.MkdirTemp("", "telemetry-log-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logDir := filepath.Join(tmpDir, "logs")

	// Create logger with file output
	logger, err := NewLogger("test-service", "development", logDir)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	if logger == nil {
		t.Fatal("expected non-nil logger")
	}

	// Log a test message
	logger.Info("test message")

	// Sync the logger (ignore sync errors in test environment)
	_ = logger.Sync()

	// Check that log file was created
	logFile := filepath.Join(logDir, "ace.log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("expected log file to be created")
	}
}

func TestNewLoggerWithStdout(t *testing.T) {
	// Create logger with stdout only
	logger, err := NewLoggerWithStdout("test-service", "development")
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	if logger == nil {
		t.Fatal("expected non-nil logger")
	}

	// Log a test message
	logger.Info("test message")

	// Sync the logger (ignore sync errors in test environment)
	_ = logger.Sync()
}

func TestInspector(t *testing.T) {
	// Create temporary directory for test database
	tmpDir, err := os.MkdirTemp("", "telemetry-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Open test database
	db, err := sql.Open("sqlite", filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ott_spans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trace_id TEXT NOT NULL,
			span_id TEXT NOT NULL,
			parent_span_id TEXT,
			operation_name TEXT NOT NULL,
			service_name TEXT NOT NULL,
			start_time TEXT NOT NULL,
			end_time TEXT NOT NULL,
			duration_ms INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'ok',
			attributes TEXT,
			created_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS ott_metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'counter',
			labels TEXT,
			value REAL NOT NULL,
			timestamp TEXT NOT NULL,
			created_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS usage_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			agent_id TEXT NOT NULL,
			session_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			model TEXT,
			input_tokens INTEGER,
			output_tokens INTEGER,
			cost_usd REAL,
			duration_ms INTEGER,
			metadata TEXT,
			created_at TEXT NOT NULL
		);
	`)
	if err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	// Create inspector
	inspector := NewInspector(db, nil)
	if inspector == nil {
		t.Fatal("expected non-nil inspector")
	}

	// Test health endpoint
	// Note: we can't easily test HTTP handlers without a full server setup
	// but we can verify the inspector is properly initialized
	if inspector.db != db {
		t.Error("inspector database not set correctly")
	}
}

// Helper function used in tests - must be in the same package
func openTestDB(t *testing.T) (*sql.DB, func()) {
	tmpDir, err := os.MkdirTemp("", "ace-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	db, err := database.Open(&database.Config{
		Mode:    "embedded",
		DataDir: tmpDir,
	})
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to open database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return db, cleanup
}
