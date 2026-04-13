package telemetry

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace"
)

// SQLiteSpanExporter implements trace.SpanExporter for SQLite storage.
type SQLiteSpanExporter struct {
	db *sql.DB
}

// NewSQLiteSpanExporter creates a new SQLite span exporter.
func NewSQLiteSpanExporter(db *sql.DB) (*SQLiteSpanExporter, error) {
	return &SQLiteSpanExporter{db: db}, nil
}

// ExportSpans exports spans to the SQLite database.
func (e *SQLiteSpanExporter) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	if len(spans) == 0 {
		return nil
	}

	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO ott_spans (
			trace_id, span_id, parent_span_id, operation_name, service_name,
			start_time, end_time, duration_ms, status, attributes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, span := range spans {
		// Marshal span attributes to JSON
		attrs := make(map[string]interface{})

		// Get service name from resource
		serviceName := ""
		for _, attr := range span.Resource().Attributes() {
			if attr.Key == "service.name" {
				serviceName = fmt.Sprintf("%v", attr.Value)
				break
			}
		}

		for _, attr := range span.Resource().Attributes() {
			attrs[string(attr.Key)] = formatSpanAttrValue(attr.Value)
		}
		for _, attr := range span.Attributes() {
			attrs[string(attr.Key)] = formatSpanAttrValue(attr.Value)
		}

		attrsJSON, err := json.Marshal(attrs)
		if err != nil {
			attrsJSON = []byte("{}")
		}

		// Determine status
		status := "ok"
		if span.Status().Code == codes.Error {
			status = "error"
		}

		// Get parent span ID
		parentSpanID := ""
		if span.Parent().IsValid() {
			parentSpanID = span.Parent().SpanID().String()
		}

		// Get operation name
		operationName := span.Name()

		_, err = stmt.ExecContext(ctx,
			span.SpanContext().TraceID().String(),
			span.SpanContext().SpanID().String(),
			parentSpanID,
			operationName,
			serviceName,
			span.StartTime().Format(time.RFC3339Nano),
			span.EndTime().Format(time.RFC3339Nano),
			span.EndTime().Sub(span.StartTime()).Milliseconds(),
			status,
			string(attrsJSON),
		)
		if err != nil {
			return fmt.Errorf("insert span: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// formatSpanAttrValue converts an attribute value to a string representation.
func formatSpanAttrValue(v attribute.Value) string {
	switch v.Type() {
	case attribute.STRING:
		return v.AsString()
	case attribute.BOOL:
		if v.AsBool() {
			return "true"
		}
		return "false"
	case attribute.INT64:
		return fmt.Sprintf("%d", v.AsInt64())
	case attribute.FLOAT64:
		return fmt.Sprintf("%f", v.AsFloat64())
	default:
		return fmt.Sprintf("%v", v)
	}
}

// Shutdown shuts down the exporter.
func (e *SQLiteSpanExporter) Shutdown(ctx context.Context) error {
	// No-op for SQLite exporter - connection is managed externally
	return nil
}

// ConsoleSpanExporter is a simple span exporter for debugging.
type ConsoleSpanExporter struct{}

// NewConsoleSpanExporter creates a new console span exporter.
func NewConsoleSpanExporter() *ConsoleSpanExporter {
	return &ConsoleSpanExporter{}
}

// ExportSpans prints spans to stdout.
func (e *ConsoleSpanExporter) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	for _, span := range spans {
		fmt.Printf("Span: %s/%s (%s) %s %dms\n",
			span.SpanContext().TraceID().String(),
			span.SpanContext().SpanID().String(),
			span.Name(),
			span.Status().Code,
			span.EndTime().Sub(span.StartTime()).Milliseconds(),
		)
	}
	return nil
}

// Shutdown shuts down the exporter.
func (e *ConsoleSpanExporter) Shutdown(ctx context.Context) error {
	return nil
}
