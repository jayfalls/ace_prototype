package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ace/framework/backend/internal/engine/telemetry"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

// TelemetryService handles observability data collection
type TelemetryService struct {
	metrics *telemetry.MetricsCollector
	tracer  *telemetry.Tracer
	logger  *zerolog.Logger
	server  *http.Server
}

func main() {
	// Setup logger
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// Initialize observability
	obs, err := telemetry.NewObservability(telemetry.Config{
		ServiceName:    "ace-telemetry",
		ServiceVersion: "1.0.0",
		Enabled:       true,
	})
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to initialize full observability, using basic")
	}

	// Create Gin router
	router := gin.Default()

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "telemetry",
			"timestamp": time.Now().UTC(),
		})
	})

	router.GET("/health/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"ready": true,
			"service": "telemetry",
		})
	})

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// ============ Telemetry APIs ============

	// Record a metric
	router.POST("/api/v1/metrics", func(c *gin.Context) {
		var req struct {
			Name      string                 `json:"name" binding:"required"`
			Value     float64                `json:"value"`
			Labels    map[string]string      `json:"labels"`
			Timestamp time.Time              `json:"timestamp"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.Timestamp.IsZero() {
			req.Timestamp = time.Now()
		}

		// Record metric
		obs.Metrics.RecordRequest(req.Name, time.Millisecond*100, nil)

		c.JSON(http.StatusOK, gin.H{
			"message": "metric recorded",
			"name":    req.Name,
		})
	})

	// Get all metrics
	router.GET("/api/v1/metrics", func(c *gin.Context) {
		metrics := obs.Metrics.GetMetrics()
		c.JSON(http.StatusOK, gin.H{
			"total_requests":  metrics.TotalRequests,
			"total_errors":    metrics.TotalErrors,
			"llm_calls":       metrics.LLMCalls,
			"avg_duration_ms": metrics.AvgDurationMs,
		})
	})

	// Record LLM call
	router.POST("/api/v1/telemetry/llm", func(c *gin.Context) {
		var req struct {
			Provider  string        `json:"provider" binding:"required"`
			Model     string        `json:"model" binding:"required"`
			Duration  time.Duration `json:"duration"`
			Tokens    int           `json:"tokens"`
			Success   bool          `json:"success"`
			Error     string        `json:"error"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var callErr error
		if !req.Success && req.Error != "" {
			callErr = &telemetryError{req.Error}
		}

		obs.Metrics.RecordLLMCall(req.Provider, req.Model, req.Duration, callErr)

		c.JSON(http.StatusOK, gin.H{
			"message": "LLM call recorded",
			"provider": req.Provider,
			"model":    req.Model,
		})
	})

	// Record agent event
	router.POST("/api/v1/telemetry/agent", func(c *gin.Context) {
		var req struct {
			AgentID   string                 `json:"agent_id" binding:"required"`
			Event     string                 `json:"event" binding:"required"`
			Layer     string                 `json:"layer"`
			Metadata  map[string]interface{} `json:"metadata"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Log the event
		logger.Info().
			Str("agent_id", req.AgentID).
			Str("event", req.Event).
			Str("layer", req.Layer).
			Interface("metadata", req.Metadata).
			Msg("agent event")

		c.JSON(http.StatusOK, gin.H{
			"message": "event recorded",
			"event":   req.Event,
		})
	})

	// Query logs
	router.GET("/api/v1/logs", func(c *gin.Context) {
		level := c.Query("level")
		limit := c.DefaultQuery("limit", "100")

		// Return sample log entries
		logs := []map[string]interface{}{
			{
				"timestamp": time.Now().Add(-time.Minute).Format(time.RFC3339),
				"level":     "info",
				"message":   "Telemetry service started",
			},
			{
				"timestamp": time.Now().Add(-30*time.Second).Format(time.RFC3339),
				"level":     "info",
				"message":   "Metrics endpoint accessed",
			},
		}

		if level != "" {
			var filtered []map[string]interface{}
			for _, l := range logs {
				if l["level"] == level {
					filtered = append(filtered, l)
				}
			}
			logs = filtered
		}

		c.JSON(http.StatusOK, gin.H{"logs": logs})
	})

	// Create server
	port := os.Getenv("TELEMETRY_PORT")
	if port == "" {
		port = "8081"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Start server
	go func() {
		logger.Info().Str("port", port).Msg("Starting telemetry service")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start telemetry server: %v", err)
		}
	}()

	// Wait for interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down telemetry service...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Server forced to shutdown")
	}

	logger.Info().Msg("Telemetry service exited")
}

type telemetryError struct {
	msg string
}

func (e *telemetryError) Error() string {
	return e.msg
}

// MarshalJSON for telemetryError
func (e *telemetryError) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.msg)
}
