// internal/handler/health.go
package handler

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"ace/internal/api/realtime"
	"ace/internal/api/response"
	"ace/internal/messaging"
	"ace/internal/telemetry"
)

type HealthHandler struct {
	pool      *pgxpool.Pool
	nats      messaging.Client
	telemetry *telemetry.Telemetry
	hub       *realtime.Hub
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(pool *pgxpool.Pool, nats messaging.Client, telemetry *telemetry.Telemetry, hub *realtime.Hub) *HealthHandler {
	return &HealthHandler{pool: pool, nats: nats, telemetry: telemetry, hub: hub}
}

// @Summary Liveness probe
// @Tags health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health/live [get]
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// @Summary Readiness probe (includes DB and NATS checks)
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	type checkResult struct {
		Status string `json:"status"`
		Reason string `json:"reason,omitempty"`
	}

	checks := map[string]checkResult{}
	overallStatus := "ok"
	httpStatus := http.StatusOK

	// Database check - required for serving traffic
	checks["database"] = checkResult{Status: "ok"}
	if err := h.pool.Ping(r.Context()); err != nil {
		overallStatus = "degraded"
		checks["database"] = checkResult{Status: "fail", Reason: "ping failed"}
		httpStatus = http.StatusServiceUnavailable
	}

	// NATS check - required for serving traffic
	checks["nats"] = checkResult{Status: "ok"}
	if h.nats != nil {
		if err := h.nats.HealthCheck(); err != nil {
			overallStatus = "degraded"
			checks["nats"] = checkResult{Status: "fail", Reason: err.Error()}
			httpStatus = http.StatusServiceUnavailable
		}
	}

	// Realtime hub check - verifies hub is running and NATS connection is alive
	if h.hub != nil {
		checks["realtime"] = checkResult{Status: "ok"}
		if !h.hub.IsRunning() {
			overallStatus = "degraded"
			checks["realtime"] = checkResult{Status: "fail", Reason: "hub not running"}
			httpStatus = http.StatusServiceUnavailable
		} else if !h.hub.NATSConnected() {
			overallStatus = "degraded"
			checks["realtime"] = checkResult{Status: "fail", Reason: "NATS not connected"}
			httpStatus = http.StatusServiceUnavailable
		}
	}

	response.JSON(w, httpStatus, map[string]any{
		"status": overallStatus,
		"checks": checks,
	})
}

// @Summary Exporter health status
// @Tags health
// @Produce json
// @Success 200 {object} map[string]any
// @Failure 503 {object} map[string]any
// @Router /health/exporters [get]
func (h *HealthHandler) Exporters(w http.ResponseWriter, r *http.Request) {
	type checkResult struct {
		Status string `json:"status"`
		Reason string `json:"reason,omitempty"`
	}

	checks := map[string]checkResult{}
	overallStatus := "ok"
	httpStatus := http.StatusOK

	// Telemetry/OTLP exporter check
	checks["telemetry"] = checkResult{Status: "ok"}
	if h.telemetry != nil {
		if err := telemetry.HealthCheck(); err != nil {
			overallStatus = "degraded"
			checks["telemetry"] = checkResult{Status: "fail", Reason: err.Error()}
			httpStatus = http.StatusServiceUnavailable
		}
	}

	response.JSON(w, httpStatus, map[string]any{
		"status": overallStatus,
		"checks": checks,
	})
}
