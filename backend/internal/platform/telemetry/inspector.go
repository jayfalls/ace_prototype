package telemetry

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"
)

// Inspector handles telemetry inspection HTTP endpoints.
type Inspector struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewInspector creates a new telemetry inspector.
func NewInspector(db *sql.DB, logger *zap.Logger) *Inspector {
	return &Inspector{
		db:     db,
		logger: logger,
	}
}

// RegisterRoutes registers the inspector HTTP routes.
func (i *Inspector) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/telemetry/spans", i.handleSpans)
	mux.HandleFunc("/telemetry/metrics", i.handleMetrics)
	mux.HandleFunc("/telemetry/usage", i.handleUsage)
	mux.HandleFunc("/telemetry/health", i.handleHealth)
}

// Span represents a trace span from the database.
type Span struct {
	TraceID      string                 `json:"trace_id"`
	SpanID       string                 `json:"span_id"`
	ParentSpanID string                 `json:"parent_span_id,omitempty"`
	Operation    string                 `json:"operation"`
	Service      string                 `json:"service"`
	StartTime    string                 `json:"start_time"`
	EndTime      string                 `json:"end_time"`
	DurationMs   int64                  `json:"duration_ms"`
	Status       string                 `json:"status"`
	Attributes   map[string]interface{} `json:"attributes,omitempty"`
}

// handleSpans handles GET /telemetry/spans
func (i *Inspector) handleSpans(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Parse query parameters
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	service := r.URL.Query().Get("service")
	operation := r.URL.Query().Get("operation")

	// Build query
	query := `
		SELECT trace_id, span_id, parent_span_id, operation_name, service_name,
		       start_time, end_time, duration_ms, status, attributes
		FROM ott_spans
		WHERE 1=1
	`
	args := []interface{}{}
	argIdx := 1

	if service != "" {
		query += fmt.Sprintf(" AND service_name = $%d", argIdx)
		args = append(args, service)
		argIdx++
	}
	if operation != "" {
		query += fmt.Sprintf(" AND operation_name = $%d", argIdx)
		args = append(args, operation)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY start_time DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := i.db.QueryContext(ctx, query, args...)
	if err != nil {
		i.logger.Error("failed to query spans", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	spans := make([]Span, 0)
	for rows.Next() {
		var s Span
		var attrsJSON sql.NullString
		var parentSpanID sql.NullString

		err := rows.Scan(
			&s.TraceID, &s.SpanID, &parentSpanID, &s.Operation, &s.Service,
			&s.StartTime, &s.EndTime, &s.DurationMs, &s.Status, &attrsJSON,
		)
		if err != nil {
			i.logger.Error("failed to scan span", zap.Error(err))
			continue
		}

		if parentSpanID.Valid {
			s.ParentSpanID = parentSpanID.String
		}
		if attrsJSON.Valid && attrsJSON.String != "" {
			if err := json.Unmarshal([]byte(attrsJSON.String), &s.Attributes); err != nil {
				s.Attributes = make(map[string]interface{})
			}
		}

		spans = append(spans, s)
	}

	// Get total count
	var total int
	countQuery := "SELECT COUNT(*) FROM ott_spans WHERE 1=1"
	if service != "" {
		countQuery += " AND service_name = ?"
	}
	if operation != "" {
		countQuery += " AND operation_name = ?"
	}

	countArgs := []interface{}{}
	if service != "" {
		countArgs = append(countArgs, service)
	}
	if operation != "" {
		countArgs = append(countArgs, operation)
	}

	i.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"spans":  spans,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// Metric represents a metric from the database.
type Metric struct {
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Labels    map[string]string `json:"labels"`
	Value     float64           `json:"value"`
	Timestamp string            `json:"timestamp"`
}

// handleMetrics handles GET /telemetry/metrics
func (i *Inspector) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Parse query parameters
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	name := r.URL.Query().Get("name")

	// Build query
	query := `
		SELECT name, type, labels, value, timestamp
		FROM ott_metrics
		WHERE 1=1
	`
	args := []interface{}{}
	if name != "" {
		query += " AND name = ?"
		args = append(args, name)
	}

	query += " ORDER BY timestamp DESC LIMIT ? OFFSET 0"
	args = append(args, limit)

	rows, err := i.db.QueryContext(ctx, query, args...)
	if err != nil {
		i.logger.Error("failed to query metrics", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	metrics := make([]Metric, 0)
	for rows.Next() {
		var m Metric
		var labelsJSON string

		err := rows.Scan(&m.Name, &m.Type, &labelsJSON, &m.Value, &m.Timestamp)
		if err != nil {
			i.logger.Error("failed to scan metric", zap.Error(err))
			continue
		}

		if labelsJSON != "" {
			if err := json.Unmarshal([]byte(labelsJSON), &m.Labels); err != nil {
				m.Labels = make(map[string]string)
			}
		} else {
			m.Labels = make(map[string]string)
		}

		metrics = append(metrics, m)
	}

	// Get total count
	var total int
	i.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ott_metrics").Scan(&total)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"metrics": metrics,
		"total":   total,
		"limit":   limit,
	})
}

// UsageEvent represents a usage event from the database.
type UsageEvent struct {
	ID           int64   `json:"id"`
	AgentID      string  `json:"agent_id"`
	SessionID    string  `json:"session_id"`
	EventType    string  `json:"event_type"`
	Model        string  `json:"model,omitempty"`
	InputTokens  int64   `json:"input_tokens,omitempty"`
	OutputTokens int64   `json:"output_tokens,omitempty"`
	CostUSD      float64 `json:"cost_usd,omitempty"`
	DurationMs   int64   `json:"duration_ms,omitempty"`
	Timestamp    string  `json:"timestamp"`
}

// handleUsage handles GET /telemetry/usage
func (i *Inspector) handleUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Parse query parameters
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	agentID := r.URL.Query().Get("agent_id")
	eventType := r.URL.Query().Get("event_type")

	// Build query
	query := `
		SELECT id, agent_id, session_id, event_type, model,
		       input_tokens, output_tokens, cost_usd, duration_ms, created_at
		FROM usage_events
		WHERE 1=1
	`
	args := []interface{}{}

	if agentID != "" {
		query += " AND agent_id = ?"
		args = append(args, agentID)
	}
	if eventType != "" {
		query += " AND event_type = ?"
		args = append(args, eventType)
	}

	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := i.db.QueryContext(ctx, query, args...)
	if err != nil {
		i.logger.Error("failed to query usage events", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	events := make([]UsageEvent, 0)
	for rows.Next() {
		var e UsageEvent
		var model sql.NullString
		var inputTokens, outputTokens sql.NullInt64
		var costUSD sql.NullFloat64
		var durationMs sql.NullInt64

		err := rows.Scan(
			&e.ID, &e.AgentID, &e.SessionID, &e.EventType, &model,
			&inputTokens, &outputTokens, &costUSD, &durationMs, &e.Timestamp,
		)
		if err != nil {
			i.logger.Error("failed to scan usage event", zap.Error(err))
			continue
		}

		if model.Valid {
			e.Model = model.String
		}
		if inputTokens.Valid {
			e.InputTokens = inputTokens.Int64
		}
		if outputTokens.Valid {
			e.OutputTokens = outputTokens.Int64
		}
		if costUSD.Valid {
			e.CostUSD = costUSD.Float64
		}
		if durationMs.Valid {
			e.DurationMs = durationMs.Int64
		}

		events = append(events, e)
	}

	// Get total count
	var total int
	countQuery := "SELECT COUNT(*) FROM usage_events WHERE 1=1"
	countArgs := []interface{}{}
	if agentID != "" {
		countQuery += " AND agent_id = ?"
		countArgs = append(countArgs, agentID)
	}
	if eventType != "" {
		countQuery += " AND event_type = ?"
		countArgs = append(countArgs, eventType)
	}
	i.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"events": events,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// handleHealth handles GET /telemetry/health
func (i *Inspector) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	response := map[string]interface{}{
		"status": "healthy",
		"checks": map[string]interface{}{
			"database": map[string]interface{}{
				"status": "ok",
			},
		},
	}

	// Check database connectivity
	if err := i.db.PingContext(ctx); err != nil {
		response["status"] = "degraded"
		response["checks"].(map[string]interface{})["database"] = map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	// Get spans count from last hour
	var spansLastHour int64
	i.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM ott_spans 
		WHERE created_at > datetime('now', '-1 hour')
	`).Scan(&spansLastHour)

	// Get metrics count from last hour
	var metricsLastHour int64
	i.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM ott_metrics 
		WHERE created_at > datetime('now', '-1 hour')
	`).Scan(&metricsLastHour)

	response["checks"].(map[string]interface{})["telemetry"] = map[string]interface{}{
		"status":            "ok",
		"spans_last_hour":   spansLastHour,
		"metrics_last_hour": metricsLastHour,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
