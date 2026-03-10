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

	"github.com/ace/framework/backend/internal/config"
	"github.com/ace/framework/backend/internal/db"
	"github.com/ace/framework/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
	_ = services.NewAuthService(cfg.JWT.Secret, cfg.JWT.AccessExpiry, cfg.JWT.RefreshExpiry)

	// Create Gin router
	router := gin.Default()
	
	// CORS middleware
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

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

	// Demo login endpoint (no auth required)
	v1.POST("/demo/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"access_token":  "demo-token",
				"refresh_token": "demo-refresh",
				"expires_in":    900,
			},
		})
	})

	// Demo register (no auth required)
	v1.POST("/demo/register", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"access_token":  "demo-token",
				"refresh_token": "demo-refresh",
				"expires_in":    900,
			},
		})
	})

	// Demo agents endpoint (in-memory for demo)
	demoAgents := []map[string]interface{}{}

	// Protected routes with demo auth middleware
	protected := v1.Group("")
	protected.Use(func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Missing authorization header"}})
			c.Abort()
			return
		}
		// Accept demo-token for testing
		if authHeader != "Bearer demo-token" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Invalid token"}})
			c.Abort()
			return
		}
		c.Set("userID", "demo-user-id")
		c.Next()
	})

	// List agents
	protected.GET("/agents", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": demoAgents})
	})

	// Get agent
	protected.GET("/agents/:id", func(c *gin.Context) {
		id := c.Param("id")
		for _, agent := range demoAgents {
			if agent["id"] == id {
				c.JSON(http.StatusOK, gin.H{"data": agent})
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
	})

	// Create agent
	protected.POST("/agents", func(c *gin.Context) {
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		agent := map[string]interface{}{
			"id":          uuid.New().String(),
			"name":        req.Name,
			"description": req.Description,
			"status":      "inactive",
			"owner_id":    "demo-user-id",
			"config":      json.RawMessage("{}"),
			"created_at":  time.Now().UTC(),
			"updated_at":  time.Now().UTC(),
		}
		demoAgents = append(demoAgents, agent)
		c.JSON(http.StatusCreated, gin.H{"data": agent})
	})

	// Update agent
	protected.PUT("/agents/:id", func(c *gin.Context) {
		id := c.Param("id")
		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		for i, agent := range demoAgents {
			if agent["id"] == id {
				req["id"] = id
				req["updated_at"] = time.Now().UTC()
				demoAgents[i] = req
				c.JSON(http.StatusOK, gin.H{"data": agent})
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
	})

	// Delete agent
	protected.DELETE("/agents/:id", func(c *gin.Context) {
		id := c.Param("id")
		for i, agent := range demoAgents {
			if agent["id"] == id {
				demoAgents = append(demoAgents[:i], demoAgents[i+1:]...)
				c.JSON(http.StatusOK, gin.H{"message": "Agent deleted"})
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
	})

	// Sessions endpoint
	protected.GET("/sessions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
	})

	protected.POST("/sessions", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"data": map[string]interface{}{"id": uuid.New().String()}})
	})

	// Memories endpoint
	protected.GET("/memories", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
	})

	protected.POST("/memories", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"data": map[string]interface{}{"id": uuid.New().String()}})
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
