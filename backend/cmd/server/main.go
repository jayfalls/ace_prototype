package main

import (
	"context"
	"encoding/json"
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
		log.Printf("No API key provided for agent %s, using mock responses", agentID)
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
	database, err := db.NewDatabase(ctx, &cfg.Database)
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
			logger.Warn().Err(err).Msgf("Failed to create %s provider, using mock", providerType)
		} else {
			llmProvider = provider.(llm.Provider)
			logger.Info().Str("provider", string(providerType)).Str("model", cfg.LLM.DefaultModel).Msg("LLM provider configured")
		}
	} else {
		logger.Warn().Msg("No LLM API key configured, using mock responses")
	}

	// Initialize cognitive engine with LLM
	agentID := uuid.New()
	engine := layers.NewEngine(agentID, nil)
	
	// Wire LLM to all layers if available
	if llmProvider != nil {
		layers.WireAllLayers(engine, llmProvider, cfg.LLM.DefaultModel)
		logger.Info().Msg("Cognitive engine ready with LLM provider")
	} else {
		logger.Warn().Msg("Running without LLM - using mock layer responses")
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
		
		// Validate token (accept demo-token for now)
		if token != "demo-token" && token != "" {
			claims := &JWTClaims{}
			_, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
				return []byte(jwtSecret), nil
			})
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
				return
			}
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
		
		// Extract token from "Bearer <token>"
		tokenString := authHeader
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenString = authHeader[7:]
		}
		
		// First try JWT validation
		claims := &JWTClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})
		
		if err == nil && token.Valid {
			c.Set("userID", claims.UserID)
			c.Set("email", claims.Email)
			c.Next()
			return
		}
		
		// Fallback: accept demo-token for testing
		if authHeader != "Bearer demo-token" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Invalid token"}})
			c.Abort()
			return
		}
		c.Set("userID", "demo-user-id")
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
		
		// Check if user exists
		if _, exists := userByEmail[req.Email]; exists {
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "EMAIL_EXISTS", "message": "Email already registered"}})
			return
		}
		
		// Hash password
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "HASH_ERROR", "message": "Failed to hash password"}})
			return
		}
		
		user := &User{
			ID:           uuid.New().String(),
			Email:        req.Email,
			Name:         req.Name,
			PasswordHash: string(hash),
			CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		}
		users[user.ID] = user
		userByEmail[user.Email] = user
		
		// Generate token
		token, err := generateToken(user)
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
				"created_at": user.CreatedAt,
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
		
		user, exists := userByEmail[req.Email]
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "INVALID_CREDENTIALS", "message": "Invalid email or password"}})
			return
		}
		
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "INVALID_CREDENTIALS", "message": "Invalid email or password"}})
			return
		}
		
		token, err := generateToken(user)
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
		user, exists := users[userID]
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "User not found"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": gin.H{
			"id":         user.ID,
			"email":      user.Email,
			"name":       user.Name,
			"created_at": user.CreatedAt,
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

		user, exists := users[userID]
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "User not found"}})
			return
		}

		token, err := generateToken(user)
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
			ProviderID  string `json:"provider_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		// Build config with provider_id if provided
		config := json.RawMessage("{}")
		if req.ProviderID != "" {
			cfg := map[string]string{"provider_id": req.ProviderID}
			configBytes, _ := json.Marshal(cfg)
			config = json.RawMessage(configBytes)
		}
		agent := map[string]interface{}{
			"id":          uuid.New().String(),
			"name":        req.Name,
			"description": req.Description,
			"status":      "inactive",
			"owner_id":    "demo-user-id",
			"config":      config,
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
		
		// Try to get provider config from agent's provider_id
		var providerType llm.ProviderType = llm.ProviderOpenAI
		apiKey, baseURL, model := "", "", "gpt-4"
		
		// Check agent config for provider_id
		log.Printf("Session creation: agentID=%s", req.AgentID)
		if cfg, ok := agent["config"].(json.RawMessage); ok {
			var cfgMap map[string]interface{}
			if err := json.Unmarshal(cfg, &cfgMap); err == nil {
				log.Printf("Session creation: agent config=%v", cfgMap)
				if providerID, ok := cfgMap["provider_id"].(string); ok && providerID != "" {
					log.Printf("Session creation: providerID=%s", providerID)
					// Look up the provider
					for _, p := range demoProviders {
						log.Printf("Session creation: checking provider %v", p["id"])
						if p["id"] == providerID {
							// Get API key from config - handle both map and json.RawMessage
							if pcfg, ok := p["config"].(map[string]interface{}); ok {
								if key, ok := pcfg["api_key"].(string); ok && key != "" {
									apiKey = key
								}
							} else if pcfg, ok := p["config"].(json.RawMessage); ok {
								var pcfgMap map[string]interface{}
								if err := json.Unmarshal(pcfg, &pcfgMap); err == nil {
									if key, ok := pcfgMap["api_key"].(string); ok && key != "" {
										apiKey = key
									}
								}
							}
							if bu, ok := p["base_url"].(string); ok {
								baseURL = bu
							}
							if m, ok := p["default_model"].(string); ok && m != "" {
								model = m
							}
							if pt, ok := p["provider_type"].(string); ok {
								providerType = llm.ProviderType(pt)
							}
							break
						}
					}
				}
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
				// Update agent status and stop engine
				if agentID, ok := session["agent_id"].(string); ok {
					for j, a := range demoAgents {
						if a["id"] == agentID {
							a["status"] = "inactive"
							demoAgents[j] = a
							break
						}
					}
					// Stop the agent engine
					removeAgentEngine(agentID)
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
		userMsg := map[string]interface{}{
			"id":         uuid.New().String(),
			"session_id": req.SessionID,
			"role":       "user",
			"content":    req.Message,
			"created_at": time.Now().UTC(),
		}
		demoChats = append(demoChats, userMsg)

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

		// Find session and get agent
		var agentID string
		for _, s := range demoSessions {
			if s["id"] == req.SessionID {
				agentID, _ = s["agent_id"].(string)
				break
			}
		}

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
						// Store layer thoughts in memory for visualization
						thoughtsKey := fmt.Sprintf("thoughts:%s", req.SessionID)
						if memoryStore[thoughtsKey] == nil {
							memoryStore[thoughtsKey] = []map[string]interface{}{}
						}
						for _, thought := range result.Thoughts {
							memoryStore[thoughtsKey] = append(memoryStore[thoughtsKey].([]map[string]interface{}), map[string]interface{}{
								"layer":   thought.Layer.String(),
								"content": thought.Content,
								"time":    time.Now().UTC(),
							})
							// Also add to demoThoughts for API
							demoThought := map[string]interface{}{
								"id":         thought.ID,
								"session_id": req.SessionID,
								"layer":      thought.Layer.String(),
								"content":    thought.Content,
								"metadata":   json.RawMessage("{}"),
								"created_at": time.Now().UTC(),
							}
							demoThoughts = append(demoThoughts, demoThought)
						}
					}
				}
			}()
		} else {
			responseContent = fmt.Sprintf("I received your message: %s. (No active session)", req.Message)
		}

		agentMsg := map[string]interface{}{
			"id":         uuid.New().String(),
			"session_id": req.SessionID,
			"role":       "assistant",
			"content":    responseContent,
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
		// Generate demo thoughts with proper layer names
		layers := []string{"aspirational", "global_strategy", "agent_model", "executive_function", "cognitive_control", "task_prosecution"}
		contents := []string{
			"Evaluating ethical implications: Ensure actions align with core values",
			"Formulating strategy: High-level plan created",
			"Updating self-model: Agent state updated",
			"Managing tasks: Task list managed",
			"Making decision: Decision made",
			"Executing: Executed",
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
	// List memories by agent
	protected.GET("/agents/:id/memories", func(c *gin.Context) {
		agentID := c.Param("id")
		var filtered []map[string]interface{}
		for _, m := range demoMemories {
			if m["agent_id"] == agentID {
				filtered = append(filtered, m)
			}
		}
		c.JSON(http.StatusOK, gin.H{"data": filtered})
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
		memory := map[string]interface{}{
			"id":          uuid.New().String(),
			"agent_id":    agentID,
			"parent_id":   req.ParentID,
			"owner_id":    "demo-user-id",
			"content":     req.Content,
			"tags":        req.Tags,
			"memory_type": req.MemoryType,
			"importance":  req.Importance,
			"created_at":  time.Now().UTC().Format(time.RFC3339),
			"updated_at":  time.Now().UTC().Format(time.RFC3339),
		}
		demoMemories = append(demoMemories, memory)
		c.JSON(http.StatusCreated, gin.H{"data": memory})
	})

	// Get memory
	protected.GET("/agents/:id/memories/:memoryId", func(c *gin.Context) {
		memoryID := c.Param("memoryId")
		for _, m := range demoMemories {
			if m["id"] == memoryID {
				c.JSON(http.StatusOK, gin.H{"data": m})
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Memory not found"}})
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
		for i, m := range demoMemories {
			if m["id"] == memoryID {
				if req.Content != "" {
					m["content"] = req.Content
				}
				if req.Tags != nil {
					m["tags"] = req.Tags
				}
				if req.Importance > 0 {
					m["importance"] = req.Importance
				}
				m["updated_at"] = time.Now().UTC().Format(time.RFC3339)
				demoMemories[i] = m
				c.JSON(http.StatusOK, gin.H{"data": m})
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Memory not found"}})
	})

	// Delete memory
	protected.DELETE("/agents/:id/memories/:memoryId", func(c *gin.Context) {
		memoryID := c.Param("memoryId")
		for i, m := range demoMemories {
			if m["id"] == memoryID {
				demoMemories = append(demoMemories[:i], demoMemories[i+1:]...)
				c.JSON(http.StatusOK, gin.H{"message": "Memory deleted"})
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Memory not found"}})
	})

	// Search memories
	protected.GET("/agents/:id/memories/search", func(c *gin.Context) {
		agentID := c.Param("id")
		query := c.Query("q")
		tags := c.Query("tags")
		
		var filtered []map[string]interface{}
		tagMap := make(map[string]bool)
		if tags != "" {
			for _, t := range splitAndTrim(tags) {
				tagMap[t] = true
			}
		}
		
		for _, m := range demoMemories {
			if m["agent_id"] != agentID {
				continue
			}
			// Match query in content
			if query != "" {
				content, ok := m["content"].(string)
				if !ok || !contains(content, query) {
					continue
				}
			}
			// Match tags
			if len(tagMap) > 0 {
				mTags, ok := m["tags"].([]string)
				if !ok {
					continue
				}
				match := false
				for _, t := range mTags {
					if tagMap[t] {
						match = true
						break
					}
				}
				if !match {
					continue
				}
			}
			filtered = append(filtered, m)
		}
		c.JSON(http.StatusOK, gin.H{"data": filtered})
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
		// Store API key in config (not encrypted in demo mode)
		cfg := map[string]string{
			"api_key": req.APIKey,
		}
		cfgBytes, _ := json.Marshal(cfg)
		
		provider := map[string]interface{}{
			"id":                uuid.New().String(),
			"owner_id":          "demo-user-id",
			"name":              req.Name,
			"provider_type":     req.ProviderType,
			"api_key_encrypted": "***",
			"base_url":          req.BaseURL,
			"default_model":     req.Model,
			"config":            json.RawMessage(cfgBytes),
			"created_at":        time.Now().UTC(),
			"updated_at":        time.Now().UTC(),
		}
		demoProviders = append(demoProviders, provider)
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
		agentID := c.Param("id")
		
		// Get provider config if available
		enginesMu.RLock()
		llmConfig, hasConfig := agentLLMConfigs[agentID]
		enginesMu.RUnlock()
		
		settings := []map[string]interface{}{
			{"key": "max_tokens", "value": "2048"},
			{"key": "temperature", "value": "0.7"},
			{"key": "top_p", "value": "0.9"},
			{"key": "provider_id", "value": ""},
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
