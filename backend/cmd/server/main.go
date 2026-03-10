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
	demoSessions := []map[string]interface{}{}
	demoMemories := []map[string]interface{}{}
	demoProviders := []map[string]interface{}{}
	demoThoughts := []map[string]interface{}{}
	demoChats := []map[string]interface{}{}

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

	// ============ AGENTS ============
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

	// ============ SESSIONS (Running) ============
	// List sessions
	protected.GET("/sessions", func(c *gin.Context) {
		agentID := c.Query("agent_id")
		if agentID != "" {
			var filtered []map[string]interface{}
			for _, s := range demoSessions {
				if s["agent_id"] == agentID {
					filtered = append(filtered, s)
				}
			}
			c.JSON(http.StatusOK, gin.H{"data": filtered})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": demoSessions})
	})

	// Get session
	protected.GET("/sessions/:id", func(c *gin.Context) {
		id := c.Param("id")
		for _, session := range demoSessions {
			if session["id"] == id {
				c.JSON(http.StatusOK, gin.H{"data": session})
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Session not found"}})
	})

	// Create session (start agent)
	protected.POST("/sessions", func(c *gin.Context) {
		var req struct {
			AgentID string `json:"agent_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		// Find agent
		var agent map[string]interface{}
		for _, a := range demoAgents {
			if a["id"] == req.AgentID {
				agent = a
				break
			}
		}
		if agent == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
			return
		}
		session := map[string]interface{}{
			"id":         uuid.New().String(),
			"agent_id":   req.AgentID,
			"owner_id":   "demo-user-id",
			"status":     "running",
			"started_at": time.Now().UTC(),
			"metadata":   json.RawMessage("{}"),
		}
		// Update agent status
		for i, a := range demoAgents {
			if a["id"] == req.AgentID {
				a["status"] = "running"
				demoAgents[i] = a
				break
			}
		}
		demoSessions = append(demoSessions, session)
		c.JSON(http.StatusCreated, gin.H{"data": session})
	})

	// End session (stop agent)
	protected.DELETE("/sessions/:id", func(c *gin.Context) {
		id := c.Param("id")
		for i, session := range demoSessions {
			if session["id"] == id {
				now := time.Now().UTC()
				session["status"] = "ended"
				session["ended_at"] = now
				demoSessions[i] = session
				// Update agent status
				if agentID, ok := session["agent_id"].(string); ok {
					for j, a := range demoAgents {
						if a["id"] == agentID {
							a["status"] = "inactive"
							demoAgents[j] = a
							break
						}
					}
				}
				c.JSON(http.StatusOK, gin.H{"data": session})
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Session not found"}})
	})

	// ============ CHAT ============
	// List chats for session
	protected.GET("/chats", func(c *gin.Context) {
		sessionID := c.Query("session_id")
		if sessionID != "" {
			var filtered []map[string]interface{}
			for _, chat := range demoChats {
				if chat["session_id"] == sessionID {
					filtered = append(filtered, chat)
				}
			}
			c.JSON(http.StatusOK, gin.H{"data": filtered})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": demoChats})
	})

	// Send chat message
	protected.POST("/chats", func(c *gin.Context) {
		var req struct {
			SessionID string `json:"session_id"`
			Message   string `json:"message"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		userMsg := map[string]interface{}{
			"id":         uuid.New().String(),
			"session_id": req.SessionID,
			"role":       "user",
			"content":    req.Message,
			"created_at": time.Now().UTC(),
		}
		demoChats = append(demoChats, userMsg)
		
		// Simulate agent response
		agentMsg := map[string]interface{}{
			"id":         uuid.New().String(),
			"session_id": req.SessionID,
			"role":       "assistant",
			"content":    "I received your message: " + req.Message + ". This is a demo response from the ACE agent.",
			"created_at": time.Now().UTC(),
		}
		demoChats = append(demoChats, agentMsg)
		
		c.JSON(http.StatusOK, gin.H{"data": []map[string]interface{}{userMsg, agentMsg}})
	})

	// ============ THOUGHTS (Visualizations) ============
	// List thoughts for session
	protected.GET("/thoughts", func(c *gin.Context) {
		sessionID := c.Query("session_id")
		if sessionID != "" {
			var filtered []map[string]interface{}
			for _, thought := range demoThoughts {
				if thought["session_id"] == sessionID {
					filtered = append(filtered, thought)
				}
			}
			c.JSON(http.StatusOK, gin.H{"data": filtered})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": demoThoughts})
	})

	// Simulate thought generation
	protected.POST("/thoughts/simulate", func(c *gin.Context) {
		var req struct {
			SessionID string `json:"session_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		// Generate demo thoughts
		layers := []string{"perception", "reasoning", "action", "reflection"}
		contents := []string{
			"Processing user input: analyzing request",
			"Evaluating available actions and tools",
			"Determining optimal response strategy",
			"Generating final output for user",
		}
		for i, layer := range layers {
			thought := map[string]interface{}{
				"id":         uuid.New().String(),
				"session_id": req.SessionID,
				"layer":      layer,
				"content":    contents[i],
				"metadata":   json.RawMessage("{}"),
				"created_at": time.Now().UTC(),
			}
			demoThoughts = append(demoThoughts, thought)
		}
		c.JSON(http.StatusOK, gin.H{"data": demoThoughts})
	})

	// ============ MEMORIES ============
	// List memories
	protected.GET("/memories", func(c *gin.Context) {
		agentID := c.Query("agent_id")
		if agentID != "" {
			var filtered []map[string]interface{}
			for _, m := range demoMemories {
				if m["agent_id"] == agentID {
					filtered = append(filtered, m)
				}
			}
			c.JSON(http.StatusOK, gin.H{"data": filtered})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": demoMemories})
	})

	// Create memory
	protected.POST("/memories", func(c *gin.Context) {
		var req struct {
			AgentID    string `json:"agent_id"`
			Content    string `json:"content"`
			MemoryType string `json:"memory_type"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		memory := map[string]interface{}{
			"id":          uuid.New().String(),
			"agent_id":    req.AgentID,
			"owner_id":    "demo-user-id",
			"content":     req.Content,
			"memory_type": req.MemoryType,
			"tags":        []string{},
			"created_at":  time.Now().UTC(),
			"updated_at":  time.Now().UTC(),
		}
		demoMemories = append(demoMemories, memory)
		c.JSON(http.StatusCreated, gin.H{"data": memory})
	})

	// Delete memory
	protected.DELETE("/memories/:id", func(c *gin.Context) {
		id := c.Param("id")
		for i, m := range demoMemories {
			if m["id"] == id {
				demoMemories = append(demoMemories[:i], demoMemories[i+1:]...)
				c.JSON(http.StatusOK, gin.H{"message": "Memory deleted"})
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Memory not found"}})
	})

	// ============ LLM PROVIDERS (Settings) ============
	// List providers
	protected.GET("/providers", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": demoProviders})
	})

	// Get provider
	protected.GET("/providers/:id", func(c *gin.Context) {
		id := c.Param("id")
		for _, p := range demoProviders {
			if p["id"] == id {
				c.JSON(http.StatusOK, gin.H{"data": p})
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Provider not found"}})
	})

	// Create provider
	protected.POST("/providers", func(c *gin.Context) {
		var req struct {
			Name         string `json:"name"`
			ProviderType string `json:"provider_type"`
			APIKey       string `json:"api_key"`
			BaseURL      string `json:"base_url"`
			Model        string `json:"model"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		provider := map[string]interface{}{
			"id":              uuid.New().String(),
			"owner_id":        "demo-user-id",
			"name":            req.Name,
			"provider_type":   req.ProviderType,
			"api_key_encrypted": "***",
			"base_url":        req.BaseURL,
			"model":           req.Model,
			"config":          json.RawMessage("{}"),
			"created_at":      time.Now().UTC(),
			"updated_at":      time.Now().UTC(),
		}
		demoProviders = append(demoProviders, provider)
		c.JSON(http.StatusCreated, gin.H{"data": provider})
	})

	// Update provider
	protected.PUT("/providers/:id", func(c *gin.Context) {
		id := c.Param("id")
		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		for i, p := range demoProviders {
			if p["id"] == id {
				req["id"] = id
				req["updated_at"] = time.Now().UTC()
				demoProviders[i] = req
				c.JSON(http.StatusOK, gin.H{"data": p})
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Provider not found"}})
	})

	// Delete provider
	protected.DELETE("/providers/:id", func(c *gin.Context) {
		id := c.Param("id")
		for i, p := range demoProviders {
			if p["id"] == id {
				demoProviders = append(demoProviders[:i], demoProviders[i+1:]...)
				c.JSON(http.StatusOK, gin.H{"message": "Provider deleted"})
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Provider not found"}})
	})

	// ============ AGENT SETTINGS ============
	// Get agent settings
	protected.GET("/agents/:id/settings", func(c *gin.Context) {
		_ = c.Param("id")
		settings := []map[string]interface{}{
			{"key": "max_tokens", "value": "2048"},
			{"key": "temperature", "value": "0.7"},
			{"key": "top_p", "value": "0.9"},
			{"key": "provider_id", "value": ""},
		}
		c.JSON(http.StatusOK, gin.H{"data": settings})
	})

	// Update agent settings
	protected.PUT("/agents/:id/settings", func(c *gin.Context) {
		_ = c.Param("id")
		var req []map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": req, "message": "Settings updated"})
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
