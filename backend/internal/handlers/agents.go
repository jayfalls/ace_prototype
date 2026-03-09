package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ace/framework/backend/internal/middleware"
	"github.com/ace/framework/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AgentHandler struct {
	agentService *services.AgentService
}

func NewAgentHandler(agentService *services.AgentService) *AgentHandler {
	return &AgentHandler{agentService: agentService}
}

type CreateAgentRequest struct {
	Name        string          `json:"name" binding:"required,max=100"`
	Description *string         `json:"description"`
	Config      json.RawMessage `json:"config"`
}

type UpdateAgentRequest struct {
	Name        *string         `json:"name" binding:"omitempty,max=100"`
	Description *string         `json:"description"`
	Config      json.RawMessage `json:"config"`
	Status      *string         `json:"status"`
}

func (h *AgentHandler) Create(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Not authenticated"}})
		return
	}

	var req CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	ownerID, _ := uuid.Parse(userID)
	agent, err := h.agentService.CreateAgent(c.Request.Context(), services.CreateAgentInput{
		OwnerID:     ownerID,
		Name:        req.Name,
		Description: req.Description,
		Config:      req.Config,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "Failed to create agent"}})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": agent})
}

func (h *AgentHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Not authenticated"}})
		return
	}

	ownerID, _ := uuid.Parse(userID)
	status := c.Query("status")
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "20"), 10, 32)
	offset, _ := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 32)

	agents, err := h.agentService.ListAgents(c.Request.Context(), ownerID, status, int32(limit), int32(offset))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "Failed to list agents"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": agents})
}

func (h *AgentHandler) Get(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Not authenticated"}})
		return
	}

	agentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "Invalid agent ID"}})
		return
	}

	ownerID, _ := uuid.Parse(userID)
	agent, err := h.agentService.GetAgent(c.Request.Context(), agentID, ownerID)
	if err == services.ErrAgentNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
		return
	}
	if err == services.ErrAgentAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "FORBIDDEN", "message": "Access denied"}})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "Failed to get agent"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": agent})
}

func (h *AgentHandler) Update(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Not authenticated"}})
		return
	}

	agentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "Invalid agent ID"}})
		return
	}

	var req UpdateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	ownerID, _ := uuid.Parse(userID)
	agent, err := h.agentService.UpdateAgent(c.Request.Context(), agentID, ownerID, req.Name, req.Description, req.Config, req.Status)
	if err == services.ErrAgentNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
		return
	}
	if err == services.ErrAgentAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "FORBIDDEN", "message": "Access denied"}})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "Failed to update agent"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": agent})
}

func (h *AgentHandler) Delete(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Not authenticated"}})
		return
	}

	agentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "Invalid agent ID"}})
		return
	}

	ownerID, _ := uuid.Parse(userID)
	err = h.agentService.DeleteAgent(c.Request.Context(), agentID, ownerID)
	if err == services.ErrAgentNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Agent not found"}})
		return
	}
	if err == services.ErrAgentAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "FORBIDDEN", "message": "Access denied"}})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "Failed to delete agent"}})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *AgentHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/agents", h.Create)
	r.GET("/agents", h.List)
	r.GET("/agents/:id", h.Get)
	r.PUT("/agents/:id", h.Update)
	r.DELETE("/agents/:id", h.Delete)
}
