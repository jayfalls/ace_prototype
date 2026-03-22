// internal/handler/health.go
package handler

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"ace/api/internal/response"
	"ace/shared/messaging"
	"ace/shared/telemetry"
)

type HealthHandler struct {
	pool      *pgxpool.Pool
	nats      messaging.Client
	telemetry *telemetry.Telemetry
}

func NewHealthHandler(pool *pgxpool.Pool, nats messaging.Client, telemetry *telemetry.Telemetry) *HealthHandler {
	return &HealthHandler{pool: pool, nats: nats, telemetry: telemetry}
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
// @Tags health
// @Produce json
// @Success 200 {object} map[string]any
// @Failure 503 {object} map[string]any
// @Router /health/ready [get]
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
