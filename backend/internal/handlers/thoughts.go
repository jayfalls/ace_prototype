package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ace/framework/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ThoughtHandler struct {
	thoughtService *services.ThoughtService
	sessionService *services.SessionService
}

func NewThoughtHandler(thoughtService *services.ThoughtService, sessionService *services.SessionService) *ThoughtHandler {
	return &ThoughtHandler{
		thoughtService: thoughtService,
		sessionService: sessionService,
	}
}

type CreateThoughtRequest struct {
	Layer    string          `json:"layer" binding:"required"`
	Content  string          `json:"content" binding:"required"`
	Metadata json.RawMessage `json:"metadata"`
}

func (h *ThoughtHandler) Create(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("session_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "Invalid session ID"}})
		return
	}

	// Verify session exists
	_, err = h.sessionService.GetSession(c.Request.Context(), sessionID, uuid.Nil)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Session not found"}})
		return
	}

	var req CreateThoughtRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	thought, err := h.thoughtService.CreateThought(c.Request.Context(), services.CreateThoughtInput{
		SessionID: sessionID,
		Layer:     req.Layer,
		Content:   req.Content,
		Metadata:  req.Metadata,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "Failed to create thought"}})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": thought})
}

func (h *ThoughtHandler) List(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("session_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "Invalid session ID"}})
		return
	}

	layer := c.Query("layer")
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "100"), 10, 32)
	offset, _ := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 32)

	thoughts, err := h.thoughtService.ListThoughts(c.Request.Context(), sessionID, layer, int32(limit), int32(offset))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "Failed to list thoughts"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": thoughts})
}

func (h *ThoughtHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/sessions/:session_id/thoughts", h.Create)
	r.GET("/sessions/:session_id/thoughts", h.List)
}
