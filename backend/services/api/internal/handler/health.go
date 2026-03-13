// Package handler provides HTTP handlers for the API service.
package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"ace/api/internal/service"
)

// HealthHandler handles health check related requests.
type HealthHandler struct {
	healthService *service.HealthService
}

// NewHealthHandler creates a new health handler with the given service.
func NewHealthHandler(healthService *service.HealthService) *HealthHandler {
	return &HealthHandler{
		healthService: healthService,
	}
}

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status    string `json:"status"`
	DB        string `json:"db"`
	Message   string `json:"message,omitempty"`
	CheckedAt string `json:"checked_at,omitempty"`
}

// Health handles GET /health requests - checks and persists health status.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check database connectivity
	dbStatus := "healthy"
	if err := h.healthService.DBHealthCheck(ctx); err != nil {
		log.Printf("Database health check failed: %v", err)
		dbStatus = "unhealthy"
	}

	// Persist health check record
	health, err := h.healthService.CreateHealthCheck(ctx)
	if err != nil {
		log.Printf("Failed to persist health check: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(HealthResponse{
			Status:  "OK",
			DB:      dbStatus,
			Message: "Failed to persist health check",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(HealthResponse{
		Status:    "OK",
		DB:        dbStatus,
		Message:   health.Message,
		CheckedAt: health.CheckedAt.Format(time.RFC3339),
	})
}

// ListHealthChecks handles GET /health/history requests.
func (h *HealthHandler) ListHealthChecks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	healthChecks, err := h.healthService.ListHealthChecks(ctx, 10)
	if err != nil {
		log.Printf("Failed to list health checks: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to retrieve health history",
		})
		return
	}

	type healthCheckResponse struct {
		Status    string `json:"status"`
		Message   string `json:"message"`
		CheckedAt string `json:"checked_at"`
	}

	response := make([]healthCheckResponse, len(healthChecks))
	for i, hc := range healthChecks {
		response[i] = healthCheckResponse{
			Status:    hc.Status,
			Message:   hc.Message,
			CheckedAt: hc.CheckedAt.Format(time.RFC3339),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
