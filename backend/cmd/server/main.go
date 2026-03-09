package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ace/framework/backend/internal/config"
	"github.com/ace/framework/backend/internal/db"
	"github.com/ace/framework/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Setup logger
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	if cfg.Log.Format == "console" {
		logger = logger.Output(zerolog.ConsoleWriter{})
	}
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// Connect to database
	if err := db.Connect(&cfg.Database); err != nil {
		logger.Error().Err(err).Msg("Failed to connect to database")
		os.Exit(1)
	}
	defer db.Disconnect()
	logger.Info().Msg("Connected to database")

	// Initialize services
	authService := services.NewAuthService(cfg.JWT.Secret, cfg.JWT.AccessExpiry, cfg.JWT.RefreshExpiry)
	_ = authService

	// Create Gin router
	router := gin.Default()

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"version":   "1.0.0",
			"timestamp": time.Now().UTC(),
		})
	})

	router.GET("/health/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"ready": true,
			"checks": gin.H{
				"database": true,
			},
		})
	})

	// Prometheus metrics
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API v1 routes
	v1 := router.Group("/api/v1")

	// Demo login endpoint
	v1.POST("/demo/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"access_token":  "demo-token",
				"refresh_token": "demo-refresh",
				"expires_in":    900,
			},
		})
	})

	logger.Info().Str("port", cfg.Server.Port).Msg("Starting server")

	// Create server
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info().Msg("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Server forced to shutdown")
	}

	logger.Info().Msg("Server exited")
}
