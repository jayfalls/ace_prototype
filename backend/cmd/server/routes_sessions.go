package main

import (
	"net/http"
	"time"

	"github.com/ace/framework/backend/internal/db"
	"github.com/ace/framework/backend/internal/llm"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// setupSessionRoutes configures session endpoints
func setupSessionRoutes(router *gin.Engine, v1 *gin.RouterGroup, database db.Database) {
	protected := v1.Group("")
	protected.Use(func(c *gin.Context) {
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

	// List sessions
	protected.GET("/sessions", func(c *gin.Context) {
		userID := c.GetString("userID")
		agentID := c.Query("agent_id")

		sessions, err := database.ListSessions(c.Request.Context(), userID, agentID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to list sessions"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": sessions})
	})

	// Get session
	protected.GET("/sessions/:id", func(c *gin.Context) {
		id := c.Param("id")
		userID := c.GetString("userID")

		session, err := database.GetSession(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Session not found"}})
			return
		}
		if session.UserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "FORBIDDEN", "message": "Access denied"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": session})
	})

	// Create session
	protected.POST("/sessions", func(c *gin.Context) {
		userID := c.GetString("userID")
		var req struct {
			AgentID string `json:"agent_id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}

		// Verify agent exists
		agent, err := database.GetAgent(c.Request.Context(), req.AgentID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
			return
		}

		session := &db.Session{
			ID:        uuid.New().String(),
			AgentID:   req.AgentID,
			UserID:    userID,
			Status:    "running",
			StartedAt: time.Now().UTC(),
		}
		if err := database.CreateSession(c.Request.Context(), session); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to create session"}})
			return
		}

		// Get engine for this agent
		engine := getAgentEngine(req.AgentID)
		if engine != nil {
			// Start engine if not already running
			ctx := context.Background()
			engine.Start(ctx)
		}

		c.JSON(http.StatusCreated, gin.H{"data": session})
	})

	// Delete session (end session)
	protected.DELETE("/sessions/:id", func(c *gin.Context) {
		id := c.Param("id")
		userID := c.GetString("userID")

		session, err := database.GetSession(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Session not found"}})
			return
		}
		if session.UserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "FORBIDDEN", "message": "Access denied"}})
			return
		}

		session.Status = "ended"
		session.EndedAt = timePtr(time.Now().UTC())
		if err := database.UpdateSession(c.Request.Context(), session); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to end session"}})
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": session})
	})

	// ============ CHAT ============
	// List chats
	protected.GET("/chats", func(c *gin.Context) {
		sessionID := c.Query("session_id")
		if sessionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "session_id required"}})
			return
		}

		chats, err := database.ListChats(c.Request.Context(), sessionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to list chats"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": chats})
	})

	// Send chat message
	protected.POST("/chats", func(c *gin.Context) {
		var req struct {
			SessionID string `json:"session_id" binding:"required"`
			Message   string `json:"message" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}

		// Get session
		session, err := database.GetSession(c.Request.Context(), req.SessionID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Session not found"}})
			return
		}

		// Save user message
		userMsg := &db.Chat{
			ID:        uuid.New().String(),
			SessionID: req.SessionID,
			Role:      "user",
			Content:   req.Message,
			CreatedAt: time.Now().UTC(),
		}
		if err := database.CreateChat(c.Request.Context(), userMsg); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to save message"}})
			return
		}

		// Get agent config
		agent, _ := database.GetAgent(c.Request.Context(), session.AgentID)
		var apiKey string
		var model string
		var providerType llm.ProviderType
		if agent != nil && agent.Config != nil {
			if providerID, ok := agent.Config["provider_id"].(string); ok {
				provider, _ := database.GetProvider(c.Request.Context(), providerID)
				if provider != nil {
					apiKey = provider.APIKey
					model = provider.Model
					providerType = llm.ProviderType(provider.Type)
				}
			}
		}

		// Process through engine if available
		engine := getAgentEngine(session.AgentID)
		if engine != nil && apiKey != "" {
			result, err := engine.ProcessCycle(c.Request.Context(), req.Message)
			if err == nil && len(result.Thoughts) > 0 {
				// Save thought records
				for _, thought := range result.Thoughts {
					dbThought := &db.Thought{
						ID:        thought.ID.String(),
						SessionID: req.SessionID,
						Layer:     thought.Layer.String(),
						Content:   thought.Content,
						CreatedAt: time.Now().UTC(),
					}
					database.CreateThought(c.Request.Context(), dbThought)
				}
			}

			// Get last layer output as response
			var response string
			if result != nil {
				if taskOutput, ok := result.LayerOutputs[layers.LayerTaskProsecution]; ok {
					if data, ok := taskOutput.Data.(map[string]interface{}); ok {
						if resultStr, ok := data["result"].(string); ok {
							response = resultStr
						}
					}
				}
			}
			if response == "" {
				response = "I processed your message through the cognitive engine."
			}

			// Save assistant message
			assistantMsg := &db.Chat{
				ID:        uuid.New().String(),
				SessionID: req.SessionID,
				Role:      "assistant",
				Content:   response,
				CreatedAt: time.Now().UTC(),
			}
			if err := database.CreateChat(c.Request.Context(), assistantMsg); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to save response"}})
				return
			}

			chats, _ := database.ListChats(c.Request.Context(), req.SessionID)
			c.JSON(http.StatusOK, gin.H{"data": chats})
		} else {
			// No engine - return simple response
			response := "Agent is not running. Please start the agent first."

			assistantMsg := &db.Chat{
				ID:        uuid.New().String(),
				SessionID: req.SessionID,
				Role:      "assistant",
				Content:   response,
				CreatedAt: time.Now().UTC(),
			}
			database.CreateChat(c.Request.Context(), assistantMsg)

			chats, _ := database.ListChats(c.Request.Context(), req.SessionID)
			c.JSON(http.StatusOK, gin.H{"data": chats})
		}
	})
}

func timePtr(t time.Time) *time.Time {
	return &t
}
