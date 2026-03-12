package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ace/framework/backend/internal/config"
	"github.com/ace/framework/backend/internal/db"
	"github.com/ace/framework/backend/internal/llm"
	"github.com/ace/framework/backend/internal/engine/layers"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RegisterSessionRoutes registers session-related routes
func RegisterSessionRoutes(protected *gin.RouterGroup, cfg *config.Config) {
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
		agent, err := db.GetAgent(c.Request.Context(), req.AgentID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
			return
		}
		
		// Try to get provider config from agent's provider_id
		var providerType llm.ProviderType = llm.ProviderOpenAI
		apiKey, baseURL, model := "", "", "gpt-4"
		
		// Check agent config for provider_id
		if providerID, ok := agent.Config["provider_id"].(string); ok && providerID != "" {
			provider, err := db.GetProvider(c.Request.Context(), providerID)
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

		// Start the engine with full startup sequence
		ctx := context.Background()
		startupErr := engine.Start(ctx)
		
		// Get startup status
		startupStatus := engine.GetStartupStatus()
		
		// Check if startup failed
		if startupErr != nil || !engine.IsStartupComplete() {
			// Find the failed step
			var failedStep string
			for _, s := range startupStatus {
				if s.Status == "failed" {
					failedStep = s.Step
					break
				}
			}
			
			// Remove the engine since startup failed
			removeAgentEngine(req.AgentID)
			
			// Update agent status to failed
			agent.Status = "failed"
			agent.UpdatedAt = nowFunc()
			db.UpdateAgent(c.Request.Context(), agent)
			
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{
				"code": "STARTUP_FAILED",
				"message": "Agent startup failed",
				"failed_step": failedStep,
				"startup_status": startupStatus,
			}})
			return
		}

		now := nowFunc()
		session := &db.Session{
			ID:        uuid.New().String(),
			AgentID:   req.AgentID,
			UserID:    userID,
			Status:    "running",
			CreatedAt: now,
			UpdatedAt: now,
		}
		
		if err := db.CreateSession(c.Request.Context(), session); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to create session"}})
			return
		}
		
		// Update agent status in DB
		agent.Status = "running"
		agent.UpdatedAt = now
		db.UpdateAgent(c.Request.Context(), agent)
		
		// Return session with startup info
		c.JSON(http.StatusCreated, gin.H{"data": gin.H{
			"session": session,
			"startup_status": startupStatus,
		}})
	})

	// End session (stop agent)
	protected.DELETE("/sessions/:id", func(c *gin.Context) {
		id := c.Param("id")
		session, err := db.GetSession(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Session not found"}})
			return
		}
		
		// Stop agent engine
		removeAgentEngine(session.AgentID)
		
		// Update session status
		session.Status = "ended"
		session.UpdatedAt = nowFunc()
		db.UpdateSession(c.Request.Context(), session)
		
		// Update agent status
		agent, err := db.GetAgent(c.Request.Context(), session.AgentID)
		if err == nil {
			agent.Status = "inactive"
			agent.UpdatedAt = nowFunc()
			db.UpdateAgent(c.Request.Context(), agent)
		}
		
		c.JSON(http.StatusOK, gin.H{"data": session})
	})

	// Get session
	protected.GET("/sessions/:id", func(c *gin.Context) {
		id := c.Param("id")
		session, err := db.GetSession(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Session not found"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": session})
	})

	// Get sessions for agent
	protected.GET("/agents/:id/sessions", func(c *gin.Context) {
		agentID := c.Param("id")
		sessions, err := db.GetSessions(c.Request.Context(), agentID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to get sessions"}})
			return
		}
		if sessions == nil {
			sessions = []db.Session{}
		}
		c.JSON(http.StatusOK, gin.H{"data": sessions})
	})
}
