package main

import (
	"fmt"
	"net/http"

	"github.com/ace/framework/backend/internal/db"
	"github.com/ace/framework/backend/internal/engine/layers"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RegisterChatRoutes registers chat and thought routes
func RegisterChatRoutes(protected *gin.RouterGroup) {
	// List chat messages
	protected.GET("/chats", func(c *gin.Context) {
		sessionID := c.Query("session_id")
		if sessionID != "" {
			messages, err := db.GetChatMessages(c.Request.Context(), sessionID, 50)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to get messages"}})
				return
			}
			c.JSON(http.StatusOK, gin.H{"data": messages})
		} else {
			c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		}
	})

	// Send chat message - via global chat loop, memories
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
		session, err := db.GetSession(c.Request.Context(), req.SessionID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Session not found"}})
			return
		}
		
		// Check if agent is ready (startup complete)
		engine := getAgentEngine(session.AgentID)
		if engine == nil || !engine.IsStartupComplete() {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "NOT_READY", "message": "Agent not ready. Start agent first."}})
			return
		}
		
		// Save user message to memories
		msg := &db.ChatMessage{
			ID:        uuid.New().String(),
			AgentID:   session.AgentID,
			Role:      "user",
			Content:   req.Message,
			CreatedAt: nowFunc(),
		}
		if err := db.CreateChatMessage(c.Request.Context(), msg); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to save message"}})
			return
		}

		// Global chat loop processes the message through memories
		// It interfaces with the layers for telemetry but chat goes to memories
		responseContent := fmt.Sprintf("Chat Loop: Received your message. (Memories interface active)")
		
		// Optionally: Summarize for telemetry to layers
		// This is a brief version sent to layers, not the full chat
		telemetrySummary := fmt.Sprintf("User: %s", req.Message)
		_ = telemetrySummary // Reserved for telemetry to layers

		// Save assistant response
		assistantMsg := &db.ChatMessage{
			ID:        uuid.New().String(),
			AgentID:   session.AgentID,
			Role:      "assistant",
			Content:   responseContent,
			CreatedAt: nowFunc(),
		}
		if err := db.CreateChatMessage(c.Request.Context(), assistantMsg); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to save response"}})
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": []interface{}{msg, assistantMsg}})
	})

	// List thoughts for session
	protected.GET("/thoughts", func(c *gin.Context) {
		sessionID := c.Query("session_id")
		if sessionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "session_id required"}})
			return
		}
		thoughts, err := db.GetThoughtsBySession(c.Request.Context(), sessionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to get thoughts"}})
			return
		}
		if thoughts == nil {
			thoughts = []db.Thought{}
		}
		c.JSON(http.StatusOK, gin.H{"data": thoughts})
	})
}
