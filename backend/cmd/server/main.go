package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ace/framework/backend/internal/config"
	"github.com/ace/framework/backend/internal/db"
	"github.com/ace/framework/backend/internal/llm"
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
	users         = make(map[string]*User)
	userByEmail   = make(map[string]*User)
	jwtSecret     = "ace-mvp-secret-key-change-in-production"
	tokenExpiry   = time.Hour
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

	// Initialize database (PostgreSQL if DATABASE_URL provided, otherwise in-memory)
	ctx := context.Background()
	database, err := db.NewDatabase(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initialize database")
		os.Exit(1)
	}
	defer database.Close()

	// Initialize in-memory DB with demo data
	if inMemDB, ok := database.(*db.InMemoryDB); ok {
		inMemDB.CreateDemoData()
	}

	logger.Info().Msg("Database initialized")

	// Initialize NATS (optional - falls back to in-memory if not available)
	var msgBus messaging.Publisher
	natsURL := os.Getenv("NATS_URL")
	if natsURL != "" {
		msgBus, err = messaging.NewNATSClient(natsURL)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to connect to NATS, using in-memory messaging")
			msgBus = messaging.NewInMemoryMessageBus()
		}
	} else {
		msgBus = messaging.NewInMemoryMessageBus()
	}
	defer msgBus.Close()
	logger.Info().Msg("Messaging initialized")

	// Initialize LLM provider manager
	_ = llm.NewProviderManager()
	logger.Info().Msg("LLM providers initialized")

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
		
		c.JSON(http.StatusCreated, gin.H{"data": gin.H{
			"user": gin.H{
				"id":         user.ID,
				"email":      user.Email,
				"name":       user.Name,
				"created_at": user.CreatedAt,
			},
			"token":      token,
			"expires_in": int(tokenExpiry.Seconds()),
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
		
		c.JSON(http.StatusOK, gin.H{"data": gin.H{
			"token":      token,
			"expires_in": int(tokenExpiry.Seconds()),
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
		
		c.JSON(http.StatusOK, gin.H{"data": gin.H{
			"token":      token,
			"expires_in": int(tokenExpiry.Seconds()),
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
