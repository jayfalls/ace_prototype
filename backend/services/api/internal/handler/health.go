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
	DB      string `json:"db"`
	Err     string `json:"err,omitempty"`
	Created string `json:"created,omitempty"`
}

// Health handles GET /health requests - checks and persists health status.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check database connectivity
	dbStatus := "healthy"
	var errMsg *string
	if err := h.healthService.DBHealthCheck(ctx); err != nil {
		dbStatus = "unhealthy"
		errStr := err.Error()
		errMsg = &errStr
	}

	// Persist health check record
	health, err := h.healthService.CreateHealthCheck(ctx, dbStatus, errMsg)
	if err != nil {
		log.Printf("Failed to persist health check: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(HealthResponse{
			DB: dbStatus,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(HealthResponse{
		DB:      health.DB,
		Err:     health.Err,
		Created: health.Created.Format(time.RFC3339),
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

	response := make([]HealthResponse, len(healthChecks))
	for i, hc := range healthChecks {
		response[i] = HealthResponse{
			DB:      hc.DB,
			Err:     hc.Err,
			Created: hc.Created.Format(time.RFC3339),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
