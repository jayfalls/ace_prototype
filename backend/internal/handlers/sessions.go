package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ace/framework/backend/internal/middleware"
	"github.com/ace/framework/backend/internal/models"
	"github.com/ace/framework/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type SessionHandler struct {
	sessionService *services.SessionService
	agentService   *services.AgentService
}

func NewSessionHandler(sessionService *services.SessionService, agentService *services.AgentService) *SessionHandler {
	return &SessionHandler{
		sessionService: sessionService,
		agentService:   agentService,
	}
}

type CreateSessionRequest struct {
	Metadata json.RawMessage `json:"metadata"`
}

type EndSessionRequest struct {
	Status string `json:"status"`
}

func (h *SessionHandler) Create(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Not authenticated"}})
		return
	}

	agentID, err := uuid.Parse(c.Param("agent_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "Invalid agent ID"}})
		return
	}

	// Verify agent exists and user owns it
	ownerID, _ := uuid.Parse(userID)
	_, err = h.agentService.GetAgent(c.Request.Context(), agentID, ownerID)
	if err == services.ErrAgentNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
		return
	}
	if err == services.ErrAgentAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "FORBIDDEN", "message": "Access denied"}})
		return
	}

	var req CreateSessionRequest
	c.ShouldBindJSON(&req)

	session, err := h.sessionService.CreateSession(c.Request.Context(), services.CreateSessionInput{
		AgentID:  agentID,
		OwnerID:  ownerID,
		Metadata: req.Metadata,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "Failed to create session"}})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": session})
}

func (h *SessionHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Not authenticated"}})
		return
	}

	ownerID, _ := uuid.Parse(userID)
	agentID := c.Param("agent_id")
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "20"), 10, 32)
	offset, _ := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 32)

	var sessions []models.Session
	var err error

	if agentID != "" {
		agentUUID, _ := uuid.Parse(agentID)
		sessions, err = h.sessionService.ListSessionsByAgent(c.Request.Context(), agentUUID, ownerID, int32(limit), int32(offset))
	} else {
		sessions, err = h.sessionService.ListSessionsByOwner(c.Request.Context(), ownerID, int32(limit), int32(offset))
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "Failed to list sessions"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": sessions})
}

func (h *SessionHandler) Get(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Not authenticated"}})
		return
	}

	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "Invalid session ID"}})
		return
	}

	ownerID, _ := uuid.Parse(userID)
	session, err := h.sessionService.GetSession(c.Request.Context(), sessionID, ownerID)
	if err == services.ErrSessionNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Session not found"}})
		return
	}
	if err == services.ErrSessionAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "FORBIDDEN", "message": "Access denied"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": session})
}

func (h *SessionHandler) End(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Not authenticated"}})
		return
	}

	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "Invalid session ID"}})
		return
	}

	var req EndSessionRequest
	c.ShouldBindJSON(&req)

	ownerID, _ := uuid.Parse(userID)
	session, err := h.sessionService.EndSession(c.Request.Context(), sessionID, ownerID, req.Status)
	if err == services.ErrSessionNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Session not found"}})
		return
	}
	if err == services.ErrSessionAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "FORBIDDEN", "message": "Access denied"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": session})
}

func (h *SessionHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/agents/:agent_id/sessions", h.Create)
	r.GET("/agents/:agent_id/sessions", h.List)
	r.GET("/sessions/:id", h.Get)
	r.DELETE("/sessions/:id", h.End)
}
