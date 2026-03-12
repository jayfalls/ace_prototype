package main

import (
	"net/http"
	"time"

	"github.com/ace/framework/backend/internal/db"
	"github.com/ace/framework/backend/internal/engine/layers"
	"github.com/ace/framework/backend/internal/llm"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// setupAgentRoutes configures agent endpoints
func setupAgentRoutes(router *gin.Engine, v1 *gin.RouterGroup, database db.Database, cfg *config.Config, llmProvider llm.Provider) {
	protected := v1.Group("")
	protected.Use(func(c *gin.Context) {
		// Auth middleware logic
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Authorization header required"}})
			c.Abort()
			return
		}

		tokenString := authHeader
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenString = authHeader[7:]
		}

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

	// List agents
	protected.GET("/agents", func(c *gin.Context) {
		userID := c.GetString("userID")
		limit := c.DefaultQuery("limit", "20")
		offset := c.DefaultQuery("offset", "0")

		agents, err := database.ListAgents(c.Request.Context(), userID, limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to list agents"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": agents})
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

	// Get agent
	protected.GET("/agents/:id", func(c *gin.Context) {
		id := c.Param("id")
		userID := c.GetString("userID")

		agent, err := database.GetAgent(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
			return
		}
		if agent.UserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "FORBIDDEN", "message": "Access denied"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": agent})
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
		removeAgentEngine(id)
		c.JSON(http.StatusOK, gin.H{"message": "Agent deleted"})
	})

	// Start agent
	protected.POST("/agents/:id/start", func(c *gin.Context) {
		id := c.Param("id")
		agent, err := database.GetAgent(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
			return
		}

		// Get provider config
		var providerType llm.ProviderType = llm.ProviderOpenAI
		apiKey := ""
		baseURL := ""
		model := ""

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

		// Create and start engine
		engine := createAgentEngine(id, providerType, apiKey, baseURL, model)
		setAgentEngine(id, engine)

		agent.Status = "running"
		agent.UpdatedAt = time.Now().UTC()
		database.UpdateAgent(c.Request.Context(), agent)

		c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "running", "agent_id": id}})
	})

	// Stop agent
	protected.POST("/agents/:id/stop", func(c *gin.Context) {
		id := c.Param("id")
		agent, err := database.GetAgent(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
			return
		}

		removeAgentEngine(id)

		agent.Status = "stopped"
		agent.UpdatedAt = time.Now().UTC()
		database.UpdateAgent(c.Request.Context(), agent)

		c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "stopped", "agent_id": id}})
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
}

// createAgentEngine creates an engine for an agent
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
