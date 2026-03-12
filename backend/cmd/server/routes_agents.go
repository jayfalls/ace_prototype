package main

import (
	"net/http"

	"github.com/ace/framework/backend/internal/db"
	"github.com/ace/framework/backend/internal/engine/layers"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// RegisterAgentRoutes registers all agent-related routes
func RegisterAgentRoutes(router *gin.RouterGroup, protected *gin.RouterGroup, cfg *ServerConfig) {
	// List agents
	protected.GET("/agents", func(c *gin.Context) {
		userID := c.GetString("userID")
		agents, err := db.GetAgents(c.Request.Context(), userID)
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
		agent, err := db.GetAgent(c.Request.Context(), id)
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

		now := nowFunc()
		agent := &db.Agent{
			ID:          uuid.New().String(),
			UserID:      userID,
			Name:        req.Name,
			Description: req.Description,
			Status:      "inactive",
			Config:      map[string]interface{}{"provider_id": req.ProviderID},
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := db.CreateAgent(c.Request.Context(), agent); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to create agent"}})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"data": agent})
	})

	// Update agent
	protected.PUT("/agents/:id", func(c *gin.Context) {
		id := c.Param("id")
		agent, err := db.GetAgent(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
			return
		}
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Config      map[string]interface{} `json:"config"`
			Enabled     *bool  `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		if req.Name != "" {
			agent.Name = req.Name
		}
		if req.Description != "" {
			agent.Description = req.Description
		}
		if req.Config != nil {
			agent.Config = req.Config
		}
		if req.Enabled != nil {
			if *req.Enabled {
				agent.Status = "active"
			} else {
				agent.Status = "inactive"
			}
		}
		agent.UpdatedAt = nowFunc()
		if err := db.UpdateAgent(c.Request.Context(), agent); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to update agent"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": agent})
	})

	// Delete agent
	protected.DELETE("/agents/:id", func(c *gin.Context) {
		id := c.Param("id")
		agent, err := db.GetAgent(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
			return
		}
		if err := db.DeleteAgent(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to delete agent"}})
			return
		}
		// Remove agent engine if exists
		removeAgentEngine(id)
		c.JSON(http.StatusOK, gin.H{"data": agent})
	})

	// Get agent status
	protected.GET("/agents/:id/status", func(c *gin.Context) {
		id := c.Param("id")
		agent, err := db.GetAgent(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
			return
		}
		
		// Check if engine is running
		engine := getAgentEngine(id)
		engineRunning := engine != nil && engine.IsRunning()
		
		c.JSON(http.StatusOK, gin.H{"data": gin.H{
			"status":       agent.Status,
			"engine_active": engineRunning,
		}})
	})

	// Get agent tools
	protected.GET("/agents/:id/tools", func(c *gin.Context) {
		id := c.Param("id")
		_, err := db.GetAgent(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
			return
		}
		// Return available tools
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
	})

	// Add tool to agent
	protected.POST("/agents/:id/tools", func(c *gin.Context) {
		id := c.Param("id")
		_, err := db.GetAgent(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
			return
		}
		var req struct {
			ToolID string `json:"tool_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		log.Info().Str("agent", id).Str("tool", req.ToolID).Msg("Tool added to agent")
		c.JSON(http.StatusCreated, gin.H{"data": gin.H{"message": "Tool added"}})
	})
}
