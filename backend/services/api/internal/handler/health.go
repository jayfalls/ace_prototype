// internal/handler/health.go
package handler

import (
    "net/http"

    "github.com/jackc/pgx/v5/pgxpool"

    "ace/api/internal/response"
)


type HealthHandler struct {
    pool *pgxpool.Pool
}


func NewHealthHandler(pool *pgxpool.Pool) *HealthHandler {
    return &HealthHandler{pool: pool}
}

func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
    response.JSON(w, http.StatusOK, map[string]string{
        "status": "ok",
    })
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
    type checkResult struct {
        Status string `json:"status"`
        Reason string `json:"reason,omitempty"`
    }

    checks := map[string]checkResult{}
    overallStatus := "ok"
    httpStatus := http.StatusOK
	checks["database"] = checkResult{Status: "ok"}

    if err := h.pool.Ping(r.Context()); err != nil {
		overallStatus = "degraded"
        checks["database"] = checkResult{Status: "fail", Reason: "ping failed"}
        httpStatus = http.StatusServiceUnavailable
    }

    response.JSON(w, httpStatus, map[string]any{
        "status": overallStatus,
        "checks": checks,
    })
}
