package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ace/framework/backend/internal/config"
	"github.com/ace/framework/backend/internal/db"
	"github.com/ace/framework/backend/internal/engine/layers"
	"github.com/ace/framework/backend/internal/llm"
	"github.com/ace/framework/backend/internal/mcp"
	"github.com/ace/framework/backend/internal/messaging"
	"github.com/ace/framework/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"
)

type User struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	PasswordHash string `json:"-"`
	CreatedAt    string `json:"created_at"`
}

type JWTClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// In-memory stores
var (
	users           = make(map[string]*User)
	userByEmail     = make(map[string]*User)
	refreshTokens   = make(map[string]string) // userID -> refreshToken
	jwtSecret       = "ace-mvp-secret-key-change-in-production"
	tokenExpiry     = time.Hour
	agentEngines    = make(map[string]*layers.Engine) // Per-agent engines
	agentLLMConfigs = make(map[string]map[string]string) // agentID -> LLM config
	enginesMu       sync.RWMutex
	memoryStore     = make(map[string]interface{}) // session memory store (global memory)
	database        db.Database // PostgreSQL database
)

// Helper function to create and start an agent engine
func createAgentEngine(agentID string, providerType llm.ProviderType, apiKey, baseURL, model string) *layers.Engine {
	id, _ := uuid.Parse(agentID)
	engine := layers.NewEngine(id, nil)
	
	log.Printf("createAgentEngine: agentID=%s, providerType=%s, apiKey present=%v, baseURL=%s, model=%s", 
		agentID, providerType, apiKey != "", baseURL, model)
	
	// If LLM config provided, wire the layers
	if apiKey != "" {
		llmConfig := llm.Config{
			APIKey:     apiKey,
			BaseURL:    baseURL,
			Model:      model,
			MaxRetries: 3,
			Timeout:    30,
		}
		provider, err := llm.NewProvider(providerType, llmConfig)
		if err == nil {
			if p, ok := provider.(llm.Provider); ok {
				layers.WireAllLayers(engine, p, model)
				log.Printf("Wired LLM provider to agent %s", agentID)
			} else {
				log.Printf("Failed to cast provider for agent %s", agentID)
			}
		} else {
			log.Printf("Failed to create LLM provider for agent %s: %v", agentID, err)
		}
	} else {
		log.Printf("No API key provided for agent %s, cannot start engine", agentID)
	}
	
	// Start the engine in background
	go func() {
		ctx := context.Background()
		engine.Start(ctx)
	}()
	
	return engine
}

// GetAgentEngine returns the engine for an agent
func getAgentEngine(agentID string) *layers.Engine {
	enginesMu.RLock()
	defer enginesMu.RUnlock()
	return agentEngines[agentID]
}

// SetAgentEngine sets the engine for an agent
func setAgentEngine(agentID string, engine *layers.Engine) {
	enginesMu.Lock()
	defer enginesMu.Unlock()
	agentEngines[agentID] = engine
}

// RemoveAgentEngine removes the engine for an agent
func removeAgentEngine(agentID string) {
	enginesMu.Lock()
	defer enginesMu.Unlock()
	if engine, ok := agentEngines[agentID]; ok {
		engine.Stop()
		delete(agentEngines, agentID)
	}
}

func main() {
	// Load configuration
	cfg := config.Load()

	// Setup logger
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	if cfg.Log.Format == "console" {
		logger = logger.Output(zerolog.ConsoleWriter{})
	}
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// Initialize database (PostgreSQL if configured, otherwise in-memory)
	ctx := context.Background()
	var err error
	database, err = db.NewDatabase(ctx, &cfg.Database)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initialize database")
		os.Exit(1)
	}
	defer database.Close()

	logger.Info().Msg("Database initialized")

	// Initialize NATS (required)
	var msgBus messaging.Publisher
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}
	msgBus, err = messaging.NewNATSClient(natsURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to NATS - NATS is required")
		return
	}
	defer msgBus.Close()
	logger.Info().Msg("Messaging initialized")

	// Initialize LLM provider based on configuration
	var llmProvider llm.Provider
	providerType := llm.ProviderType(cfg.LLM.Provider)
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		apiKey = cfg.LLM.APIKey
	}
	
	if apiKey != "" {
		llmConfig := llm.Config{
			APIKey:     apiKey,
			BaseURL:    cfg.LLM.BaseURL,
			Model:      cfg.LLM.DefaultModel,
			MaxRetries: cfg.LLM.MaxRetries,
			Timeout:    cfg.LLM.Timeout,
		}
		provider, err := llm.NewProvider(providerType, llmConfig)
		if err != nil {
			logger.Warn().Err(err).Msgf("Failed to create %s provider", providerType)
		} else {
			llmProvider = provider.(llm.Provider)
			logger.Info().Str("provider", string(providerType)).Str("model", cfg.LLM.DefaultModel).Msg("LLM provider configured")
		}
	} else {
		logger.Warn().Msg("No LLM API key configured")
	}

	// Initialize cognitive engine with LLM
	agentID := uuid.New()
	engine := layers.NewEngine(agentID, nil)
	
	// Wire LLM to all layers if available
	if llmProvider != nil {
		layers.WireAllLayers(engine, llmProvider, cfg.LLM.DefaultModel)
		logger.Info().Msg("Cognitive engine ready with LLM provider")
	} else {
		logger.Warn().Msg("Running without LLM - cognitive engine will not function")
	}

	// Initialize MCP server
	mcpServer := mcp.NewServer()
	mcp.DefaultTools(mcpServer)
	logger.Info().Msg("MCP server initialized")

	// Initialize services
	authService := services.NewAuthService(cfg.JWT.Secret, cfg.JWT.AccessExpiry, cfg.JWT.RefreshExpiry)
	_ = authService // suppress unused warning

	// Create Gin router
	router := gin.Default()
	
	// Rate limiting middleware
	rateLimiter := rate.NewLimiter(rate.Limit(100), 100) // 100 requests per second
	router.Use(func(c *gin.Context) {
		if !rateLimiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			c.Abort()
			return
		}
		c.Next()
	})
	
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

	// MCP endpoints
	router.GET("/mcp/tools", func(c *gin.Context) {
		tools := mcpServer.ListTools()
		c.JSON(http.StatusOK, gin.H{"tools": tools})
	})

	router.POST("/mcp/tools/:name", func(c *gin.Context) {
		toolName := c.Param("name")
		var args map[string]interface{}
		if err := c.ShouldBindJSON(&args); err != nil {
			args = make(map[string]interface{})
		}
		
		result, err := mcpServer.CallTool(c.Request.Context(), toolName, args)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)
	})

	router.GET("/mcp/resources", func(c *gin.Context) {
		resources := mcpServer.ListResources()
		c.JSON(http.StatusOK, gin.H{"resources": resources})
	})

	router.GET("/mcp/prompts", func(c *gin.Context) {
		prompts := mcpServer.ListPrompts()
		c.JSON(http.StatusOK, gin.H{"prompts": prompts})
	})

	// ACE Engine processing endpoint
	router.POST("/engine/process", func(c *gin.Context) {
		var req struct {
			Input string `json:"input"`
			Layer string `json:"layer"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		
		// Process through engine
		result, err := engine.ProcessCycle(c.Request.Context(), req.Input)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		
		c.JSON(http.StatusOK, gin.H{
			"cycle_id": result.CycleID,
			"layers":   len(result.LayerOutputs),
			"thoughts": result.Thoughts,
		})
	})

	// WebSocket upgrader
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for MVP
		},
	}

	// WebSocket endpoint for real-time thought streaming
	router.GET("/ws/agents/:id", func(c *gin.Context) {
		agentID := c.Param("id")
		
		// Validate token from query param or header
		token := c.Query("token")
		if token == "" {
			authHeader := c.GetHeader("Authorization")
			if len(authHeader) > 7 {
				token = authHeader[7:]
			}
		}
		
		// Validate JWT token
		if token != "" {
			claims := &JWTClaims{}
			_, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
				return []byte(jwtSecret), nil
			})
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
				return
			}
			// Set user ID from claims
			c.Set("userID", claims.UserID)
		}

		// Upgrade to WebSocket
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}
		defer conn.Close()

		// Send welcome message
		conn.WriteJSON(gin.H{
			"type": "connected",
			"data": gin.H{
				"agent_id": agentID,
				"message":  "Connected to agent thought stream",
			},
		})

		// Simulate thought updates
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		cycle := 0
		for {
			select {
			case <-ticker.C:
				cycle++
				layers := []string{"perception", "reasoning", "action", "reflection"}
				contents := []string{
					"Processing input from user",
					"Analyzing available options",
					"Executing selected action",
					"Reviewing and reflecting on results",
				}
				
				for i, layer := range layers {
					thought := gin.H{
						"id":         uuid.New().String(),
						"agent_id":   agentID,
						"layer":      layer,
						"content":    contents[i],
						"cycle":      cycle,
						"created_at": time.Now().UTC().Format(time.RFC3339),
					}
					
					err := conn.WriteJSON(gin.H{
						"type": "thought",
						"data": thought,
					})
					if err != nil {
						return
					}
				}
			}
		}
	})

	// API v1 routes
	v1 := router.Group("/api/v1")

	// Protected routes with JWT auth middleware
	protected := v1.Group("")
	protected.Use(func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Missing authorization header"}})
			c.Abort()
			return
		}
		
		// Extract token from "Bearer <token>"
		tokenString := authHeader
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenString = authHeader[7:]
		}
		
		// Validate JWT
		claims := &JWTClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})
		
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Invalid token"}})
			c.Abort()
			return
		}
		c.Set("userID", claims.UserID)
		c.Set("email", claims.Email)
		c.Next()
	})

	// ============ AUTH ============
	// Register
	v1.POST("/auth/register", func(c *gin.Context) {
		var req struct {
			Email    string `json:"email" binding:"required,email"`
			Password string `json:"password" binding:"required,min=8"`
			Name     string `json:"name" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		
		// Check if user exists in database
		existingUser, err := database.GetUserByEmail(c.Request.Context(), req.Email)
		if err == nil && existingUser != nil {
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "EMAIL_EXISTS", "message": "Email already registered"}})
			return
		}
		
		// Hash password
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "HASH_ERROR", "message": "Failed to hash password"}})
			return
		}
		
		now := time.Now().UTC()
		user := &db.User{
			ID:        uuid.New().String(),
			Email:     req.Email,
			Name:      req.Name,
			Password:  string(hash),
			CreatedAt: now,
			UpdatedAt: now,
		}
		
		if err := database.CreateUser(c.Request.Context(), user); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to create user: " + err.Error()}})
			return
		}
		
		// Generate token
		token, err := generateTokenFromDB(user)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "TOKEN_ERROR", "message": "Failed to generate token"}})
			return
		}

		// Store refresh token
		refreshToken := uuid.New().String()
		refreshTokens[user.ID] = refreshToken

		c.JSON(http.StatusCreated, gin.H{"data": gin.H{
			"user": gin.H{
				"id":         user.ID,
				"email":      user.Email,
				"name":       user.Name,
				"created_at": user.CreatedAt.Format(time.RFC3339),
			},
			"access_token":  token,
			"refresh_token": refreshToken,
			"expires_in":    int(tokenExpiry.Seconds()),
		}})
	})

	// Login
	v1.POST("/auth/login", func(c *gin.Context) {
		var req struct {
			Email    string `json:"email" binding:"required,email"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		
		// Get user from database
		user, err := database.GetUserByEmail(c.Request.Context(), req.Email)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "INVALID_CREDENTIALS", "message": "Invalid email or password"}})
			return
		}
		
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "INVALID_CREDENTIALS", "message": "Invalid email or password"}})
			return
		}
		
		token, err := generateTokenFromDB(user)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "TOKEN_ERROR", "message": "Failed to generate token"}})
			return
		}

		// Store refresh token
		refreshToken := uuid.New().String()
		refreshTokens[user.ID] = refreshToken

		c.JSON(http.StatusOK, gin.H{"data": gin.H{
			"access_token":  token,
			"refresh_token": refreshToken,
			"expires_in":    int(tokenExpiry.Seconds()),
		}})
	})

	// Get current user
	protected.GET("/auth/me", func(c *gin.Context) {
		userID := c.GetString("userID")
		user, err := database.GetUserByID(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "User not found"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": gin.H{
			"id":         user.ID,
			"email":      user.Email,
			"name":       user.Name,
			"created_at": user.CreatedAt.Format(time.RFC3339),
		}})
	})

	// Refresh token
	protected.POST("/auth/refresh", func(c *gin.Context) {
		userID := c.GetString("userID")
		
		// Verify refresh token was provided
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			// If no body, try header
			req.RefreshToken = c.GetHeader("X-Refresh-Token")
		}
		
		// Validate refresh token
		if req.RefreshToken != "" {
			storedToken, exists := refreshTokens[userID]
			if !exists || storedToken != req.RefreshToken {
				c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "INVALID_TOKEN", "message": "Invalid refresh token"}})
				return
			}
		}

		user, err := database.GetUserByID(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "User not found"}})
			return
		}

		token, err := generateTokenFromDB(user)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "TOKEN_ERROR", "message": "Failed to generate token"}})
			return
		}

		// Optionally rotate refresh token
		newRefreshToken := uuid.New().String()
		refreshTokens[userID] = newRefreshToken

		c.JSON(http.StatusOK, gin.H{"data": gin.H{
			"access_token":  token,
			"refresh_token": newRefreshToken,
			"expires_in":   int(tokenExpiry.Seconds()),
		}})
	})

	// ============ TOOLS ============
	// Available tools (static list for MVP)
	availableTools := []map[string]interface{}{
		{"id": "web_search", "name": "Web Search", "description": "Search the web for information", "category": "research"},
		{"id": "file_read", "name": "File Read", "description": "Read files from disk", "category": "filesystem"},
		{"id": "file_write", "name": "File Write", "description": "Write files to disk", "category": "filesystem"},
		{"id": "execute_code", "name": "Execute Code", "description": "Run code snippets", "category": "execution"},
		{"id": "curl", "name": "HTTP Request", "description": "Make HTTP requests", "category": "network"},
		{"id": "database_query", "name": "Database Query", "description": "Execute database queries", "category": "data"},
	}

	// Agent tool whitelist store
	agentTools := make(map[string][]map[string]interface{})

	// List available tools
	protected.GET("/tools", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": availableTools})
	})

	// List agent tools
	protected.GET("/agents/:id/tools", func(c *gin.Context) {
		agentID := c.Param("id")
		tools, exists := agentTools[agentID]
		if !exists {
			tools = []map[string]interface{}{}
		}
		c.JSON(http.StatusOK, gin.H{"data": tools})
	})

	// Add tool to whitelist
	protected.POST("/agents/:id/tools", func(c *gin.Context) {
		agentID := c.Param("id")
		var req struct {
			ToolID string `json:"tool_id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		
		// Find tool
		var tool map[string]interface{}
		for _, t := range availableTools {
			if t["id"] == req.ToolID {
				tool = t
				break
			}
		}
		if tool == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Tool not found"}})
			return
		}
		
		// Add to whitelist
		whitelistTool := map[string]interface{}{
			"id":         uuid.New().String(),
			"agent_id":   agentID,
			"tool_id":    req.ToolID,
			"name":       tool["name"],
			"enabled":    true,
			"created_at": time.Now().UTC().Format(time.RFC3339),
		}
		agentTools[agentID] = append(agentTools[agentID], whitelistTool)
		c.JSON(http.StatusCreated, gin.H{"data": whitelistTool})
	})

	// Remove tool from whitelist
	protected.DELETE("/agents/:id/tools/:toolId", func(c *gin.Context) {
		agentID := c.Param("id")
		toolID := c.Param("toolId")
		
		tools, exists := agentTools[agentID]
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "No tools found for agent"}})
			return
		}
		
		for i, t := range tools {
			if t["id"] == toolID {
				agentTools[agentID] = append(tools[:i], tools[i+1:]...)
				c.JSON(http.StatusOK, gin.H{"message": "Tool removed"})
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Tool not found"}})
	})

	// ============ AGENTS ============
	// List agents
	protected.GET("/agents", func(c *gin.Context) {
		userID := c.GetString("userID")
		agents, err := database.GetAgents(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to get agents: " + err.Error()}})
			return
		}
		if agents == nil {
			agents = []db.Agent{}
		}
		c.JSON(http.StatusOK, gin.H{"data": agents})
	})

	// Get agent
	protected.GET("/agents/:id", func(c *gin.Context) {
		id := c.Param("id")
		agent, err := database.GetAgent(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": agent})
	})

	// Create agent
	protected.POST("/agents", func(c *gin.Context) {
		userID := c.GetString("userID")
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			ProviderID  string `json:"provider_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		// Validate: provider_id is required
		if req.ProviderID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "provider_id is required"}})
			return
		}
		// Verify provider exists
		provider, err := database.GetProvider(c.Request.Context(), req.ProviderID)
		if err != nil || provider == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "Invalid provider_id: provider not found"}})
			return
		}
		// Build config with provider_id
		config := map[string]interface{}{
			"provider_id": req.ProviderID,
		}
		now := time.Now().UTC()
		agent := &db.Agent{
			ID:          uuid.New().String(),
			Name:        req.Name,
			Description: req.Description,
			UserID:      userID,
			Config:      config,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := database.CreateAgent(c.Request.Context(), agent); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to create agent: " + err.Error()}})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"data": agent})
	})

	// Update agent
	protected.PUT("/agents/:id", func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			ProviderID  string `json:"provider_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		agent, err := database.GetAgent(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
			return
		}
		if req.Name != "" {
			agent.Name = req.Name
		}
		if req.Description != "" {
			agent.Description = req.Description
		}
		if req.ProviderID != "" {
			agent.Config["provider_id"] = req.ProviderID
		}
		agent.UpdatedAt = time.Now().UTC()
		if err := database.UpdateAgent(c.Request.Context(), agent); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to update agent: " + err.Error()}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": agent})
	})

	// Delete agent
	protected.DELETE("/agents/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := database.DeleteAgent(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Agent deleted"})
	})

	// ============ SESSIONS (Running) ============
	// List sessions
	protected.GET("/sessions", func(c *gin.Context) {
		userID := c.GetString("userID")
		agentID := c.Query("agent_id")
		
		var sessions []db.Session
		var err error
		
		if agentID != "" {
			sessions, err = database.GetSessions(c.Request.Context(), agentID)
		} else {
			sessions, err = database.GetSessionsByUser(c.Request.Context(), userID)
		}
		
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to get sessions"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": sessions})
	})

	// Get session
	protected.GET("/sessions/:id", func(c *gin.Context) {
		id := c.Param("id")
		session, err := database.GetSession(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Session not found"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": session})
	})

	// Create session (start agent)
	protected.POST("/sessions", func(c *gin.Context) {
		userID := c.GetString("userID")
		var req struct {
			AgentID string `json:"agent_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		
		// Get agent from DB
		agent, err := database.GetAgent(c.Request.Context(), req.AgentID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
			return
		}
		
		// Try to get provider config from agent's provider_id
		var providerType llm.ProviderType = llm.ProviderOpenAI
		apiKey, baseURL, model := "", "", "gpt-4"
		
		// Check agent config for provider_id
		if providerID, ok := agent.Config["provider_id"].(string); ok && providerID != "" {
			provider, err := database.GetProvider(c.Request.Context(), providerID)
			if err == nil && provider != nil {
				apiKey = provider.APIKey
				baseURL = provider.BaseURL
				model = provider.Model
				providerType = llm.ProviderType(provider.Type)
			}
		}
		
		// Fall back to global config if no provider found
		if apiKey == "" && cfg.LLM.APIKey != "" {
			apiKey = cfg.LLM.APIKey
			baseURL = cfg.LLM.BaseURL
			model = cfg.LLM.DefaultModel
			providerType = llm.ProviderType(cfg.LLM.Provider)
		}
		
		// Create and start the agent engine
		engine := createAgentEngine(req.AgentID, providerType, apiKey, baseURL, model)
		setAgentEngine(req.AgentID, engine)

		now := time.Now().UTC()
		session := &db.Session{
			ID:        uuid.New().String(),
			AgentID:   req.AgentID,
			UserID:    userID,
			Status:    "running",
			CreatedAt: now,
			UpdatedAt: now,
		}
		
		if err := database.CreateSession(c.Request.Context(), session); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to create session"}})
			return
		}
		
		// Update agent status in DB
		agent.Status = "running"
		agent.UpdatedAt = now
		database.UpdateAgent(c.Request.Context(), agent)
		
		c.JSON(http.StatusCreated, gin.H{"data": session})
	})

	// End session (stop agent)
	protected.DELETE("/sessions/:id", func(c *gin.Context) {
		id := c.Param("id")
		
		session, err := database.GetSession(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Session not found"}})
			return
		}
		
		now := time.Now().UTC()
		session.Status = "ended"
		session.UpdatedAt = now
		database.UpdateSession(c.Request.Context(), session)
		
		// Update agent status and stop engine
		if agent, err := database.GetAgent(c.Request.Context(), session.AgentID); err == nil {
			agent.Status = "inactive"
			agent.UpdatedAt = now
			database.UpdateAgent(c.Request.Context(), agent)
		}
		
		// Stop the agent engine
		removeAgentEngine(session.AgentID)
		
		c.JSON(http.StatusOK, gin.H{"data": session})
	})

	// ============ CHAT ============
	// List chats for session (get chat messages)
	protected.GET("/chats", func(c *gin.Context) {
		sessionID := c.Query("session_id")
		if sessionID != "" {
			// Get session to find agent
			session, err := database.GetSession(c.Request.Context(), sessionID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Session not found"}})
				return
			}
			messages, err := database.GetChatMessages(c.Request.Context(), session.AgentID, 50)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to get messages"}})
				return
			}
			c.JSON(http.StatusOK, gin.H{"data": messages})
		} else {
			c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		}
	})

	// Send chat message - goes to global memory, Chat Loop generates response
	protected.POST("/chats", func(c *gin.Context) {
		var req struct {
			SessionID string `json:"session_id"`
			Message   string `json:"message"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		
		// Get session to find agent
		session, err := database.GetSession(c.Request.Context(), req.SessionID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Session not found"}})
			return
		}
		
		// Save user message
		msg := &db.ChatMessage{
			ID:        uuid.New().String(),
			AgentID:   session.AgentID,
			Role:      "user",
			Content:   req.Message,
			CreatedAt: time.Now().UTC(),
		}
		if err := database.CreateChatMessage(c.Request.Context(), msg); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to save message"}})
			return
		}

		// Store in global memory (simulated memory store)
		sessionMemKey := fmt.Sprintf("memory:%s", req.SessionID)
		if memoryStore[sessionMemKey] == nil {
			memoryStore[sessionMemKey] = []map[string]interface{}{}
		}
		memoryStore[sessionMemKey] = append(memoryStore[sessionMemKey].([]map[string]interface{}), map[string]interface{}{
			"type":    "chat_message",
			"role":    "user",
			"content": req.Message,
			"time":    time.Now().UTC(),
		})

		agentID := session.AgentID
		var responseContent string

		// Chat Loop generates fast response from global memory (NOT from layers)
		if agentID != "" {
			// Read chat history from memory
			mem := memoryStore[sessionMemKey]
			var chatHistory string
			if mem != nil {
				for _, m := range mem.([]map[string]interface{}) {
					if m["type"] == "chat_message" {
						chatHistory += fmt.Sprintf("%s: %s\n", m["role"], m["content"])
					}
				}
			}

			// Chat Loop response - fast path from global memory
			if chatHistory != "" {
				responseContent = fmt.Sprintf("Chat Loop: I received your message \"%s\". Reading from global memory shows our conversation context.", req.Message)
			} else {
				responseContent = fmt.Sprintf("Chat Loop: I received your message: %s", req.Message)
			}

			// Store assistant response in memory
			memoryStore[sessionMemKey] = append(memoryStore[sessionMemKey].([]map[string]interface{}), map[string]interface{}{
				"type":    "chat_message",
				"role":    "assistant",
				"content": responseContent,
				"time":    time.Now().UTC(),
			})

			// TRIGGER LAYER PROCESSING (async) - layers read from memory and process
			// This is separate from the fast chat response
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("Chat handler: panic in goroutine: %v", r)
					}
				}()
				engine := getAgentEngine(agentID)
				log.Printf("Chat handler: engine for agent %s = %v", agentID, engine)
				if engine != nil {
					ctx := context.Background()
					result, err := engine.ProcessCycle(ctx, req.Message)
					log.Printf("Chat handler: ProcessCycle completed, result=%v, err=%v", result != nil, err)
					if result != nil {
						log.Printf("Chat handler: thoughts count = %d", len(result.Thoughts))
					}
					if err == nil && result != nil && len(result.Thoughts) > 0 {
						// Store layer thoughts in DB for visualization
						for _, thought := range result.Thoughts {
							thoughtRecord := &db.Thought{
								ID:        thought.ID.String(),
								SessionID: req.SessionID,
								Layer:     thought.Layer.String(),
								Content:   thought.Content,
								CreatedAt: time.Now().UTC(),
							}
							database.CreateThought(c.Request.Context(), thoughtRecord)
						}
					}
				}
			}()
		} else {
			responseContent = fmt.Sprintf("I received your message: %s. (No active session)", req.Message)
		}

		// Save assistant response
		assistantMsg := &db.ChatMessage{
			ID:        uuid.New().String(),
			AgentID:   session.AgentID,
			Role:      "assistant",
			Content:   responseContent,
			CreatedAt: time.Now().UTC(),
		}
		database.CreateChatMessage(c.Request.Context(), assistantMsg)

		c.JSON(http.StatusOK, gin.H{"data": []interface{}{msg, assistantMsg}})
	})

	// ============ THOUGHTS (Visualizations) ============
	// List thoughts for session
	protected.GET("/thoughts", func(c *gin.Context) {
		sessionID := c.Query("session_id")
		if sessionID != "" {
			thoughts, err := database.GetThoughtsBySession(c.Request.Context(), sessionID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to get thoughts: " + err.Error()}})
				return
			}
			c.JSON(http.StatusOK, gin.H{"data": thoughts})
		} else {
			c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		}
	})

	// ============ MEMORIES ============
	// List memories by agent
	protected.GET("/agents/:id/memories", func(c *gin.Context) {
		agentID := c.Param("id")
		memories, err := database.GetMemoriesByAgent(c.Request.Context(), agentID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to get memories"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": memories})
	})

	// Create memory for agent
	protected.POST("/agents/:id/memories", func(c *gin.Context) {
		agentID := c.Param("id")
		var req struct {
			Content    string   `json:"content" binding:"required"`
			Tags       []string `json:"tags"`
			MemoryType string   `json:"memory_type"`
			ParentID   string   `json:"parent_id"`
			Importance int      `json:"importance"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		if req.MemoryType == "" {
			req.MemoryType = "short_term"
		}
		if req.Importance == 0 {
			req.Importance = 5
		}
		
		now := time.Now().UTC()
		metadata := map[string]interface{}{
			"tags":       req.Tags,
			"importance": req.Importance,
			"memory_type": req.MemoryType,
			"parent_id":  req.ParentID,
		}
		memory := &db.Memory{
			ID:        uuid.New().String(),
			AgentID:   agentID,
			Type:      req.MemoryType,
			Content:   req.Content,
			Metadata:  metadata,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := database.CreateMemory(c.Request.Context(), memory); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to create memory"}})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"data": memory})
	})

	// Get memory
	protected.GET("/agents/:id/memories/:memoryId", func(c *gin.Context) {
		memoryID := c.Param("memoryId")
		memory, err := database.GetMemory(c.Request.Context(), memoryID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Memory not found"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": memory})
	})

	// Update memory
	protected.PUT("/agents/:id/memories/:memoryId", func(c *gin.Context) {
		memoryID := c.Param("memoryId")
		var req struct {
			Content    string   `json:"content"`
			Tags       []string `json:"tags"`
			Importance int      `json:"importance"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		memory, err := database.GetMemory(c.Request.Context(), memoryID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Memory not found"}})
			return
		}
		if req.Content != "" {
			memory.Content = req.Content
		}
		if req.Tags != nil {
			memory.Metadata["tags"] = req.Tags
		}
		if req.Importance > 0 {
			memory.Metadata["importance"] = req.Importance
		}
		memory.UpdatedAt = time.Now().UTC()
		if err := database.UpdateMemory(c.Request.Context(), memory); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to update memory"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": memory})
	})

	// Delete memory
	protected.DELETE("/agents/:id/memories/:memoryId", func(c *gin.Context) {
		memoryID := c.Param("memoryId")
		if err := database.DeleteMemory(c.Request.Context(), memoryID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Memory not found"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Memory deleted"})
	})

	// Search memories
	protected.GET("/agents/:id/memories/search", func(c *gin.Context) {
		agentID := c.Param("id")
		query := c.Query("q")
		
		var memories []db.Memory
		var err error
		
		if query != "" {
			memories, err = database.SearchMemories(c.Request.Context(), agentID, query)
		} else {
			memories, err = database.GetMemoriesByAgent(c.Request.Context(), agentID)
		}
		
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to search memories"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": memories})
	})

	// ============ LLM PROVIDERS (Settings) ============
	// List providers
	protected.GET("/providers", func(c *gin.Context) {
		providers, err := database.GetProviders(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to get providers"}})
			return
		}
		if providers == nil {
			providers = []db.Provider{}
		}
		// Mask API keys in response
		for i := range providers {
			providers[i].APIKey = "***"
		}
		c.JSON(http.StatusOK, gin.H{"data": providers})
	})

	// Get provider
	protected.GET("/providers/:id", func(c *gin.Context) {
		id := c.Param("id")
		provider, err := database.GetProvider(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Provider not found"}})
			return
		}
		provider.APIKey = "***"
		c.JSON(http.StatusOK, gin.H{"data": provider})
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
		
		now := time.Now().UTC()
		provider := &db.Provider{
			ID:       uuid.New().String(),
			Name:     req.Name,
			Type:     req.ProviderType,
			APIKey:   req.APIKey,
			BaseURL:  req.BaseURL,
			Model:    req.Model,
			Enabled:  true,
			CreatedAt: now,
			UpdatedAt: now,
		}
		
		if err := database.CreateProvider(c.Request.Context(), provider); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to create provider"}})
			return
		}
		
		// Mask API key in response
		provider.APIKey = "***"
		c.JSON(http.StatusCreated, gin.H{"data": provider})
	})

	// Test provider connection
	protected.POST("/providers/test", func(c *gin.Context) {
		var req struct {
			ProviderType string `json:"provider_type"`
			APIKey       string `json:"api_key"`
			BaseURL      string `json:"base_url"`
			Model        string `json:"model"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}

		// Get the base URL for the provider type
		baseURL := req.BaseURL
		if baseURL == "" {
			switch req.ProviderType {
			case "openrouter":
				baseURL = "https://openrouter.ai/api/v1"
			case "openai":
				baseURL = "https://api.openai.com/v1"
			case "anthropic":
				baseURL = "https://api.anthropic.com"
			case "xai":
				baseURL = "https://api.x.ai/v1"
			case "ollama":
				baseURL = "http://localhost:11434"
			case "llama.cpp":
				baseURL = "http://localhost:8080"
			case "deepseek":
				baseURL = "https://api.deepseek.com/v1"
			case "mistral":
				baseURL = "https://api.mistral.ai/v1"
			case "cohere":
				baseURL = "https://api.cohere.ai/v1"
			default:
				baseURL = "https://api.openai.com/v1"
			}
		}

		llmConfig := llm.Config{
			APIKey:     req.APIKey,
			BaseURL:    baseURL,
			Model:      req.Model,
			MaxRetries: 3,
			Timeout:    30,
		}

		providerType := llm.ProviderType(req.ProviderType)
		provider, err := llm.NewProvider(providerType, llmConfig)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "PROVIDER_ERROR", "message": err.Error()}})
			return
		}

		err = provider.(llm.Provider).TestConnection()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "CONNECTION_FAILED", "message": err.Error()}})
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "ok", "message": "Connection successful"}})
	})

	// Update provider
	protected.PUT("/providers/:id", func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Name         string `json:"name"`
			ProviderType string `json:"provider_type"`
			APIKey       string `json:"api_key"`
			BaseURL      string `json:"base_url"`
			Model        string `json:"model"`
			Enabled      *bool  `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		
		provider, err := database.GetProvider(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Provider not found"}})
			return
		}
		
		if req.Name != "" {
			provider.Name = req.Name
		}
		if req.ProviderType != "" {
			provider.Type = req.ProviderType
		}
		if req.APIKey != "" && req.APIKey != "***" {
			provider.APIKey = req.APIKey
		}
		if req.BaseURL != "" {
			provider.BaseURL = req.BaseURL
		}
		if req.Model != "" {
			provider.Model = req.Model
		}
		if req.Enabled != nil {
			provider.Enabled = *req.Enabled
		}
		provider.UpdatedAt = time.Now().UTC()
		
		if err := database.UpdateProvider(c.Request.Context(), provider); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to update provider"}})
			return
		}
		
		provider.APIKey = "***"
		c.JSON(http.StatusOK, gin.H{"data": provider})
	})

	// Delete provider
	protected.DELETE("/providers/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := database.DeleteProvider(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Provider not found"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Provider deleted"})
	})

	// ============ AGENT SETTINGS ============
	// Get agent settings
	protected.GET("/agents/:id/settings", func(c *gin.Context) {
		agentID := c.Param("id")
		
		// Verify agent exists
		agent, err := database.GetAgent(c.Request.Context(), agentID)
		if err != nil || agent == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
			return
		}
		
		// Get provider config if available
		enginesMu.RLock()
		llmConfig, hasConfig := agentLLMConfigs[agentID]
		enginesMu.RUnlock()
		
		settings := []map[string]interface{}{
			{"key": "max_tokens", "value": "2048"},
			{"key": "temperature", "value": "0.7"},
			{"key": "top_p", "value": "0.9"},
			{"key": "provider_id", "value": agent.Config["provider_id"]},
		}
		
		if hasConfig {
			settings = append(settings, map[string]interface{}{
				"key": "llm_provider", "value": llmConfig["provider_type"],
			})
			settings = append(settings, map[string]interface{}{
				"key": "llm_model", "value": llmConfig["model"],
			})
		}
		
		c.JSON(http.StatusOK, gin.H{"data": settings})
	})

	// Update agent settings
	protected.PUT("/agents/:id/settings", func(c *gin.Context) {
		agentID := c.Param("id")
		var req []map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		
		// Check for LLM settings and store them
		for _, setting := range req {
			key, _ := setting["key"].(string)
			value, _ := setting["value"].(string)
			
			if key == "llm_provider" || key == "llm_model" || key == "llm_api_key" || key == "llm_base_url" {
				enginesMu.Lock()
				if agentLLMConfigs[agentID] == nil {
					agentLLMConfigs[agentID] = make(map[string]string)
				}
				agentLLMConfigs[agentID][strings.Replace(key, "llm_", "", 1)] = value
				enginesMu.Unlock()
			}
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

func generateToken(user *User) (string, error) {
	claims := JWTClaims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}

// generateTokenFromDB generates a JWT token from a db.User
func generateTokenFromDB(user *db.User) (string, error) {
	claims := JWTClaims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}

func splitAndTrim(s string) []string {
	var result []string
	if s == "" {
		return []string{}
	}
	for _, part := range strings.Split(s, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
