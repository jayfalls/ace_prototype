package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ace/framework/backend/internal/db"
	"github.com/ace/framework/backend/internal/engine/layers"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
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

	// Send chat message - processes through cognitive layers
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
		
		// Save user message
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

		agentID := session.AgentID
		var responseContent string

		// Process through cognitive engine
		if agentID != "" {
			engine := getAgentEngine(agentID)
			log.Printf("Chat handler: engine for agent %s = %v", agentID, engine)
			
			if engine != nil {
				// Process through layers synchronously to get response
				ctx := context.Background()
				result, err := engine.ProcessCycle(ctx, req.Message)
				log.Printf("Chat handler: ProcessCycle completed, result=%v, err=%v", result != nil, err)
				
				if err != nil {
					responseContent = fmt.Sprintf("Error processing message: %v", err)
				} else if result != nil && len(result.Thoughts) > 0 {
					// Extract response from layer outputs - use L1 (Aspirational) layer output as the response
					if output, ok := result.LayerOutputs[layers.LayerAspirational]; ok {
						if data, ok := output.Data.(map[string]interface{}); ok {
							if guidance, ok := data["ethical_guidance"].(string); ok {
								responseContent = fmt.Sprintf("Thought: %s", guidance)
							}
						}
					}
					// If no specific response, summarize the thoughts
					if responseContent == "" {
						var thoughtSummary string
						for i, thought := range result.Thoughts {
							if i > 0 {
								thoughtSummary += "; "
							}
							thoughtSummary += thought.Content
						}
						responseContent = thoughtSummary
					}
					
					// Store layer thoughts in DB for visualization
					for _, thought := range result.Thoughts {
						thoughtRecord := &db.Thought{
							ID:        thought.ID.String(),
							SessionID: req.SessionID,
							Layer:     thought.Layer.String(),
							Content:   thought.Content,
							CreatedAt: nowFunc(),
						}
						db.CreateThought(c.Request.Context(), thoughtRecord)
					}
				} else {
					responseContent = "No cognitive response generated. Ensure provider is configured."
				}
			} else {
				responseContent = "No active engine for this agent"
			}
		} else {
			responseContent = "No agent associated with this session"
		}

		// Save assistant response
		assistantMsg := &db.ChatMessage{
			ID:        uuid.New().String(),
			AgentID:   session.AgentID,
			Role:      "assistant",
			Content:   responseContent,
			CreatedAt: nowFunc(),
		}
		db.CreateChatMessage(c.Request.Context(), assistantMsg)

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
