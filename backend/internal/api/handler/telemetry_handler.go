// Package handler contains HTTP request handlers for the telemetry inspector.
package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nats-io/nats.go"

	db "ace/internal/api/repository/generated"
	"ace/internal/api/response"
	"ace/internal/caching"
)

// Valid time windows for metrics aggregation.
var validWindows = map[string]bool{
	"5m":  true,
	"15m": true,
	"1h":  true,
	"6h":  true,
	"24h": true,
}

// TelemetryHandler handles telemetry inspector HTTP requests.
type TelemetryHandler struct {
	queries  *db.Queries
	db       *sql.DB
	natsConn *nats.Conn
	cache    caching.CacheBackend
}

// NewTelemetryHandler creates a new TelemetryHandler.
func NewTelemetryHandler(
	queries *db.Queries,
	db *sql.DB,
	natsConn *nats.Conn,
	cache caching.CacheBackend,
) (*TelemetryHandler, error) {
	if queries == nil {
		return nil, errors.New("queries is required")
	}
	if db == nil {
		return nil, errors.New("database is required")
	}

	h := &TelemetryHandler{
		queries:  queries,
		db:       db,
		natsConn: natsConn,
		cache:    cache,
	}

	return h, nil
}

// SpanResponse represents a single span in API responses.
type SpanResponse struct {
	TraceID    string         `json:"trace_id"`
	SpanID     string         `json:"span_id"`
	Operation  string         `json:"operation"`
	Service    string         `json:"service"`
	StartTime  string         `json:"start_time"`
	EndTime    string         `json:"end_time"`
	DurationMs int64          `json:"duration_ms"`
	Status     string         `json:"status"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// SpansResponse represents the response for GET /telemetry/spans.
type SpansResponse struct {
	Spans  []SpanResponse `json:"spans"`
	Total  int64          `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

// Spans handles GET /telemetry/spans - Query recent trace spans.
func (h *TelemetryHandler) Spans(w http.ResponseWriter, r *http.Request) {
	var ctx context.Context = r.Context()

	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	service := r.URL.Query().Get("service")
	operation := r.URL.Query().Get("operation")
	startTime := r.URL.Query().Get("start_time")
	endTime := r.URL.Query().Get("end_time")

	// Default values
	limit := 50
	offset := 0

	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit < 1 || parsedLimit > 1000 {
			response.BadRequest(w, "invalid_limit", "limit must be between 1 and 1000")
			return
		}
		limit = parsedLimit
	}

	if offsetStr != "" {
		parsedOffset, err := strconv.Atoi(offsetStr)
		if err != nil || parsedOffset < 0 {
			response.BadRequest(w, "invalid_offset", "offset must be non-negative")
			return
		}
		offset = parsedOffset
	}

	// Validate time formats if provided
	if startTime != "" {
		if _, err := time.Parse(time.RFC3339, startTime); err != nil {
			response.BadRequest(w, "invalid_start_time", "start_time must be RFC3339 format")
			return
		}
	}
	if endTime != "" {
		if _, err := time.Parse(time.RFC3339, endTime); err != nil {
			response.BadRequest(w, "invalid_end_time", "end_time must be RFC3339 format")
			return
		}
	}

	// Set default time range if not provided (24h ago to now)
	if startTime == "" {
		startTime = time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	}
	if endTime == "" {
		endTime = time.Now().Format(time.RFC3339)
	}

	// Query spans from database
	emptyStr := ""
	params := db.ListSpansParams{
		Column1:       &emptyStr,
		ServiceName:   service,
		Column3:       &emptyStr,
		OperationName: operation,
		Column5:       &emptyStr,
		StartTime:     startTime,
		Column7:       &emptyStr,
		EndTime:       endTime,
		Limit:         int64(limit),
		Offset:        int64(offset),
	}

	// If service is empty, use NULL pattern match
	if service == "" {
		params.Column1 = nil
		params.ServiceName = ""
	}
	if operation == "" {
		params.Column3 = nil
		params.OperationName = ""
	}
	if startTime == "" {
		params.Column5 = nil
		params.StartTime = ""
	}
	if endTime == "" {
		params.Column7 = nil
		params.EndTime = ""
	}

	spans, err := h.queries.ListSpans(ctx, params)
	if err != nil {
		response.InternalError(w, "Failed to query spans")
		return
	}

	// Get total count
	countParams := db.ListSpansCountParams{
		Column1:       params.Column1,
		ServiceName:   params.ServiceName,
		Column3:       params.Column3,
		OperationName: params.OperationName,
		Column5:       params.Column5,
		StartTime:     params.StartTime,
		Column7:       params.Column7,
		EndTime:       params.EndTime,
	}
	total, err := h.queries.ListSpansCount(ctx, countParams)
	if err != nil {
		response.InternalError(w, "Failed to count spans")
		return
	}

	// Convert to response format
	spanResponses := make([]SpanResponse, 0, len(spans))
	for _, span := range spans {
		spanResp := SpanResponse{
			TraceID:    span.TraceID,
			SpanID:     span.SpanID,
			Operation:  span.OperationName,
			Service:    span.ServiceName,
			StartTime:  span.StartTime,
			EndTime:    span.EndTime,
			DurationMs: span.DurationMs,
			Status:     span.Status,
		}

		// Parse attributes JSON if present
		if span.Attributes.Valid && span.Attributes.String != "" {
			var attrs map[string]any
			if err := json.Unmarshal([]byte(span.Attributes.String), &attrs); err == nil {
				spanResp.Attributes = attrs
			}
		}

		spanResponses = append(spanResponses, spanResp)
	}

	response.JSON(w, http.StatusOK, SpansResponse{
		Spans:  spanResponses,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// MetricResponse represents a single metric in API responses.
type MetricResponse struct {
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Labels    map[string]string `json:"labels,omitempty"`
	Value     float64           `json:"value"`
	Timestamp string            `json:"timestamp"`
	Window    string            `json:"window,omitempty"`
}

// MetricsResponse represents the response for GET /telemetry/metrics.
type MetricsResponse struct {
	Metrics []MetricResponse `json:"metrics"`
	Total   int64            `json:"total"`
	Limit   int              `json:"limit"`
}

// Metrics handles GET /telemetry/metrics - Query metric summaries.
func (h *TelemetryHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	var ctx context.Context = r.Context()

	// Parse query parameters
	name := r.URL.Query().Get("name")
	window := r.URL.Query().Get("window")
	limitStr := r.URL.Query().Get("limit")

	// Default values
	limit := 50

	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit < 1 || parsedLimit > 200 {
			response.BadRequest(w, "invalid_limit", "limit must be between 1 and 200")
			return
		}
		limit = parsedLimit
	}

	// Validate window
	if window == "" {
		window = "1h"
	}
	if !validWindows[window] {
		response.BadRequest(w, "invalid_window", "window must be one of: 5m, 15m, 1h, 6h, 24h")
		return
	}

	// Calculate time range based on window
	now := time.Now()
	var startTime time.Time
	switch window {
	case "5m":
		startTime = now.Add(-5 * time.Minute)
	case "15m":
		startTime = now.Add(-15 * time.Minute)
	case "1h":
		startTime = now.Add(-1 * time.Hour)
	case "6h":
		startTime = now.Add(-6 * time.Hour)
	case "24h":
		startTime = now.Add(-24 * time.Hour)
	}

	emptyStr := ""
	params := db.ListMetricsParams{
		Column1:     &emptyStr,
		Name:        name,
		Column3:     &emptyStr,
		Timestamp:   startTime.Format(time.RFC3339),
		Column5:     &emptyStr,
		Timestamp_2: now.Format(time.RFC3339),
		Limit:       int64(limit),
		Offset:      0,
	}

	if name == "" {
		params.Column1 = nil
		params.Name = ""
	}

	metrics, err := h.queries.ListMetrics(ctx, params)
	if err != nil {
		response.InternalError(w, "Failed to query metrics")
		return
	}

	// Get total count
	countParams := db.ListMetricsCountParams{
		Column1:     params.Column1,
		Name:        params.Name,
		Column3:     params.Column3,
		Timestamp:   params.Timestamp,
		Column5:     params.Column5,
		Timestamp_2: params.Timestamp_2,
	}
	total, err := h.queries.ListMetricsCount(ctx, countParams)
	if err != nil {
		response.InternalError(w, "Failed to count metrics")
		return
	}

	// Convert to response format
	metricResponses := make([]MetricResponse, 0, len(metrics))
	for _, metric := range metrics {
		metricResp := MetricResponse{
			Name:      metric.Name,
			Type:      metric.Type,
			Value:     metric.Value,
			Timestamp: metric.Timestamp,
			Window:    window,
		}

		// Parse labels JSON if present
		if metric.Labels.Valid && metric.Labels.String != "" {
			var labels map[string]string
			if err := json.Unmarshal([]byte(metric.Labels.String), &labels); err == nil {
				metricResp.Labels = labels
			}
		}

		metricResponses = append(metricResponses, metricResp)
	}

	response.JSON(w, http.StatusOK, MetricsResponse{
		Metrics: metricResponses,
		Total:   total,
		Limit:   limit,
	})
}

// UsageEventResponse represents a single usage event in API responses.
type UsageEventResponse struct {
	ID           string   `json:"id"`
	AgentID      string   `json:"agent_id"`
	SessionID    string   `json:"session_id"`
	EventType    string   `json:"event_type"`
	Model        *string  `json:"model,omitempty"`
	InputTokens  *int64   `json:"input_tokens,omitempty"`
	OutputTokens *int64   `json:"output_tokens,omitempty"`
	CostUsd      *float64 `json:"cost_usd,omitempty"`
	DurationMs   *int64   `json:"duration_ms,omitempty"`
	Timestamp    string   `json:"timestamp"`
}

// UsageResponse represents the response for GET /telemetry/usage.
type UsageResponse struct {
	Events []UsageEventResponse `json:"events"`
	Total  int64                `json:"total"`
	Limit  int                  `json:"limit"`
	Offset int                  `json:"offset"`
}

// Usage handles GET /telemetry/usage - Query cost attribution data.
func (h *TelemetryHandler) Usage(w http.ResponseWriter, r *http.Request) {
	var ctx context.Context = r.Context()

	// Parse query parameters
	agentID := r.URL.Query().Get("agent_id")
	eventType := r.URL.Query().Get("event_type")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	// Default values
	limit := 100
	offset := 0

	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit < 1 || parsedLimit > 500 {
			response.BadRequest(w, "invalid_limit", "limit must be between 1 and 500")
			return
		}
		limit = parsedLimit
	}

	if offsetStr != "" {
		parsedOffset, err := strconv.Atoi(offsetStr)
		if err != nil || parsedOffset < 0 {
			response.BadRequest(w, "invalid_offset", "offset must be non-negative")
			return
		}
		offset = parsedOffset
	}

	// Validate agent_id format (UUID) if provided
	if agentID != "" {
		// Basic UUID format validation (8-4-4-4-12 hex characters)
		if len(agentID) != 36 {
			response.BadRequest(w, "invalid_agent_id", "agent_id must be a valid UUID")
			return
		}
	}

	// Validate time formats if provided
	if from != "" {
		if _, err := time.Parse(time.RFC3339, from); err != nil {
			response.BadRequest(w, "invalid_from", "from must be RFC3339 format")
			return
		}
	}
	if to != "" {
		if _, err := time.Parse(time.RFC3339, to); err != nil {
			response.BadRequest(w, "invalid_to", "to must be RFC3339 format")
			return
		}
	}

	// Set default time range if not provided (7d ago to now)
	if from == "" {
		from = time.Now().Add(-7 * 24 * time.Hour).Format(time.RFC3339)
	}
	if to == "" {
		to = time.Now().Format(time.RFC3339)
	}

	emptyStr := ""
	params := db.ListUsageEventsParams{
		Column1:     &emptyStr,
		AgentID:     agentID,
		Column3:     &emptyStr,
		EventType:   eventType,
		Column5:     &emptyStr,
		CreatedAt:   from,
		Column7:     &emptyStr,
		CreatedAt_2: to,
		Limit:       int64(limit),
		Offset:      int64(offset),
	}

	if agentID == "" {
		params.Column1 = nil
		params.AgentID = ""
	}
	if eventType == "" {
		params.Column3 = nil
		params.EventType = ""
	}

	events, err := h.queries.ListUsageEvents(ctx, params)
	if err != nil {
		response.InternalError(w, "Failed to query usage events")
		return
	}

	// Get total count
	countParams := db.ListUsageEventsCountParams{
		Column1:     params.Column1,
		AgentID:     params.AgentID,
		Column3:     params.Column3,
		EventType:   params.EventType,
		Column5:     params.Column5,
		CreatedAt:   params.CreatedAt,
		Column7:     params.Column7,
		CreatedAt_2: params.CreatedAt_2,
	}
	total, err := h.queries.ListUsageEventsCount(ctx, countParams)
	if err != nil {
		response.InternalError(w, "Failed to count usage events")
		return
	}

	// Convert to response format
	eventResponses := make([]UsageEventResponse, 0, len(events))
	for _, event := range events {
		eventResp := UsageEventResponse{
			ID:        fmt.Sprintf("%d", event.ID),
			AgentID:   event.AgentID,
			SessionID: event.SessionID,
			EventType: event.EventType,
			Timestamp: event.CreatedAt,
		}

		if event.Model.Valid {
			eventResp.Model = &event.Model.String
		}
		if event.InputTokens.Valid {
			eventResp.InputTokens = &event.InputTokens.Int64
		}
		if event.OutputTokens.Valid {
			eventResp.OutputTokens = &event.OutputTokens.Int64
		}
		if event.CostUsd.Valid {
			eventResp.CostUsd = &event.CostUsd.Float64
		}
		if event.DurationMs.Valid {
			eventResp.DurationMs = &event.DurationMs.Int64
		}

		eventResponses = append(eventResponses, eventResp)
	}

	response.JSON(w, http.StatusOK, UsageResponse{
		Events: eventResponses,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// SubsystemCheck represents the health status of a single subsystem.
type SubsystemCheck struct {
	Status           string  `json:"status"`
	Mode             string  `json:"mode,omitempty"`
	Path             string  `json:"path,omitempty"`
	SizeBytes        int64   `json:"size_bytes,omitempty"`
	Connections      int     `json:"connections,omitempty"`
	MaxCostBytes     int64   `json:"max_cost_bytes,omitempty"`
	CurrentCostBytes int64   `json:"current_cost_bytes,omitempty"`
	HitRate          float64 `json:"hit_rate,omitempty"`
	SpansLastHour    int64   `json:"spans_last_hour,omitempty"`
	MetricsLastHour  int64   `json:"metrics_last_hour,omitempty"`
	Error            string  `json:"error,omitempty"`
}

// TelemetryHealthResponse represents the response for GET /telemetry/health.
type TelemetryHealthResponse struct {
	Status string                    `json:"status"`
	Checks map[string]SubsystemCheck `json:"checks"`
}

// Health handles GET /telemetry/health - Returns subsystem health status.
func (h *TelemetryHandler) Health(w http.ResponseWriter, r *http.Request) {
	var ctx context.Context = r.Context()

	response_data := TelemetryHealthResponse{
		Status: "healthy",
		Checks: make(map[string]SubsystemCheck),
	}

	// Check database
	dbCheck := SubsystemCheck{Status: "ok"}
	if h.db != nil {
		if err := h.db.PingContext(ctx); err != nil {
			dbCheck.Status = "error"
			dbCheck.Error = err.Error()
			response_data.Status = "degraded"
		}
		// Get database size
		if response_data.Status == "ok" {
			var size int64
			err := h.db.QueryRowContext(ctx, "SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()").Scan(&size)
			if err == nil {
				dbCheck.SizeBytes = size
			}
		}
	} else {
		dbCheck.Status = "not_initialized"
		response_data.Status = "degraded"
	}
	dbCheck.Mode = "embedded"
	response_data.Checks["database"] = dbCheck

	// Check messaging (NATS)
	natsCheck := SubsystemCheck{Status: "ok", Mode: "embedded"}
	if h.natsConn != nil {
		natsCheck.Connections = 0 // Embedded mode has no external connections
		if !h.natsConn.IsConnected() {
			natsCheck.Status = "error"
			natsCheck.Error = "NATS not connected"
			response_data.Status = "degraded"
		}
	} else {
		natsCheck.Status = "not_initialized"
		response_data.Status = "degraded"
	}
	response_data.Checks["messaging"] = natsCheck

	// Check cache
	cacheCheck := SubsystemCheck{Status: "ok", Mode: "embedded"}
	if h.cache != nil {
		// CacheBackend doesn't provide stats, so we just indicate it's available
		// The hit rate and size would require the higher-level Cache interface
	} else {
		cacheCheck.Status = "not_initialized"
		response_data.Status = "degraded"
	}
	response_data.Checks["cache"] = cacheCheck

	// Check telemetry subsystem
	telemetryCheck := SubsystemCheck{Status: "ok", Mode: "embedded"}
	spansCount, err := h.queries.CountSpansLastHour(ctx)
	if err != nil {
		telemetryCheck.Status = "error"
		telemetryCheck.Error = fmt.Sprintf("failed to count spans: %v", err)
		response_data.Status = "degraded"
	} else {
		telemetryCheck.SpansLastHour = spansCount
	}

	metricsCount, err := h.queries.CountMetricsLastHour(ctx)
	if err != nil {
		telemetryCheck.Status = "error"
		telemetryCheck.Error = fmt.Sprintf("failed to count metrics: %v", err)
		response_data.Status = "degraded"
	} else {
		telemetryCheck.MetricsLastHour = metricsCount
	}
	response_data.Checks["telemetry"] = telemetryCheck

	// Determine HTTP status code
	httpStatus := http.StatusOK
	if response_data.Status == "degraded" {
		httpStatus = http.StatusServiceUnavailable
	}

	response.JSON(w, httpStatus, response_data)
}

// Mount registers the telemetry routes on the given router with JWT auth.
func (h *TelemetryHandler) Mount(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/spans", h.Spans)
		r.Get("/metrics", h.Metrics)
		r.Get("/usage", h.Usage)
		r.Get("/health", h.Health)
	})
}
