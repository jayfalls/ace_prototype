// Package handler provides HTTP handlers for the API service.
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"ace/api/internal/repository/generated"
)

// HealthHandler handles health check related requests.
type HealthHandler struct {
	queries *generated.Queries
}

// NewHealthHandler creates a new health handler with the given queries.
func NewHealthHandler(queries *generated.Queries) *HealthHandler {
	return &HealthHandler{
		queries: queries,
	}
}

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status    string `json:"status"`
	DB        string `json:"db"`
	Health    string `json:"health,omitempty"`
	Message   string `json:"message,omitempty"`
	CheckedAt string `json:"checked_at,omitempty"`
}

// Health handles GET /health requests using SQLC queries.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// First, try to get the latest health check from the database
	var healthResponse HealthResponse

	latestHealth, err := h.queries.GetLatestHealthCheck(ctx)
	if err != nil {
		log.Printf("Failed to get latest health check: %v", err)
		// If no records, create one
		newHealth, createErr := h.queries.CreateHealthCheck(ctx, generated.CreateHealthCheckParams{
			Status:  "healthy",
			Message: "System is operational",
		})
		if createErr != nil {
			log.Printf("Failed to create health check: %v", createErr)
			healthResponse = HealthResponse{
				Status:  "OK",
				DB:      "healthy",
				Health:  "degraded",
				Message: fmt.Sprintf("Database query failed: %v", err),
			}
		} else {
			healthResponse = HealthResponse{
				Status:    "OK",
				DB:        "healthy",
				Health:    newHealth.Status,
				Message:   newHealth.Message,
				CheckedAt: newHealth.CheckedAt.Format(time.RFC3339),
			}
		}
	} else {
		healthResponse = HealthResponse{
			Status:    "OK",
			DB:        "healthy",
			Health:    latestHealth.Status,
			Message:   latestHealth.Message,
			CheckedAt: latestHealth.CheckedAt.Format(time.RFC3339),
		}
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(healthResponse); err != nil {
		log.Printf("Failed to encode health response: %v", err)
	}
}

// ListHealthChecks handles GET /health/history requests.
func (h *HealthHandler) ListHealthChecks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	healthChecks, err := h.queries.ListHealthChecks(ctx, 10)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Failed to list health checks: %v", err),
		})
		return
	}

	type healthCheckResponse struct {
		ID        int32  `json:"id"`
		Status    string `json:"status"`
		Message   string `json:"message"`
		CheckedAt string `json:"checked_at"`
	}

	response := make([]healthCheckResponse, len(healthChecks))
	for i, hc := range healthChecks {
		response[i] = healthCheckResponse{
			ID:        hc.ID,
			Status:    hc.Status,
			Message:   hc.Message,
			CheckedAt: hc.CheckedAt.Format(time.RFC3339),
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
