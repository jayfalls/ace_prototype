package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/ace/framework/backend/internal/middleware"
	"github.com/ace/framework/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type LLMProviderHandler struct {
	providerService *services.LLMProviderService
	agentService   *services.AgentService
}

func NewLLMProviderHandler(providerService *services.LLMProviderService, agentService *services.AgentService) *LLMProviderHandler {
	return &LLMProviderHandler{
		providerService: providerService,
		agentService:   agentService,
	}
}

type CreateProviderRequest struct {
	Name         string  `json:"name" binding:"required"`
	ProviderType string  `json:"provider_type" binding:"required"`
	APIKey       *string `json:"api_key"`
	BaseURL      *string `json:"base_url"`
	Model        *string `json:"model"`
}

type UpdateProviderRequest struct {
	Name         *string `json:"name"`
	ProviderType *string `json:"provider_type"`
	APIKey       *string `json:"api_key"`
	BaseURL      *string `json:"base_url"`
	Model        *string `json:"model"`
}

type CreateAttachmentRequest struct {
	ProviderID uuid.UUID `json:"provider_id" binding:"required"`
	Layer      string    `json:"layer" binding:"required"`
	Priority   int       `json:"priority"`
}

func (h *LLMProviderHandler) CreateProvider(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Not authenticated"}})
		return
	}

	var req CreateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	ownerID, _ := uuid.Parse(userID)
	provider, err := h.providerService.CreateProvider(c.Request.Context(), services.CreateProviderInput{
		OwnerID:       ownerID,
		Name:          req.Name,
		ProviderType:  req.ProviderType,
		APIKey:        req.APIKey,
		BaseURL:       req.BaseURL,
		Model:         req.Model,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "Failed to create provider"}})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": provider})
}

func (h *LLMProviderHandler) ListProviders(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Not authenticated"}})
		return
	}

	ownerID, _ := uuid.Parse(userID)
	providers, err := h.providerService.ListProviders(c.Request.Context(), ownerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "Failed to list providers"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": providers})
}

func (h *LLMProviderHandler) GetProvider(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Not authenticated"}})
		return
	}

	providerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "Invalid provider ID"}})
		return
	}

	ownerID, _ := uuid.Parse(userID)
	provider, err := h.providerService.GetProvider(c.Request.Context(), providerID, ownerID)
	if err == services.ErrProviderNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Provider not found"}})
		return
	}
	if err == services.ErrProviderAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "FORBIDDEN", "message": "Access denied"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": provider})
}

func (h *LLMProviderHandler) UpdateProvider(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Not authenticated"}})
		return
	}

	providerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "Invalid provider ID"}})
		return
	}

	var req UpdateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	ownerID, _ := uuid.Parse(userID)
	provider, err := h.providerService.UpdateProvider(c.Request.Context(), providerID, ownerID, req.Name, req.ProviderType, req.APIKey, req.BaseURL, req.Model, nil)
	if err == services.ErrProviderNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Provider not found"}})
		return
	}
	if err == services.ErrProviderAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "FORBIDDEN", "message": "Access denied"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": provider})
}

func (h *LLMProviderHandler) DeleteProvider(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Not authenticated"}})
		return
	}

	providerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "Invalid provider ID"}})
		return
	}

	ownerID, _ := uuid.Parse(userID)
	err = h.providerService.DeleteProvider(c.Request.Context(), providerID, ownerID)
	if err == services.ErrProviderNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Provider not found"}})
		return
	}
	if err == services.ErrProviderAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "FORBIDDEN", "message": "Access denied"}})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *LLMProviderHandler) CreateAttachment(c *gin.Context) {
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

	// Verify agent belongs to user
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

	var req CreateAttachmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	attachment, err := h.providerService.CreateAttachment(c.Request.Context(), agentID, req.ProviderID, req.Layer, req.Priority, json.RawMessage("{}"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "Failed to create attachment"}})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": attachment})
}

func (h *LLMProviderHandler) ListAttachments(c *gin.Context) {
	agentID, err := uuid.Parse(c.Param("agent_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "Invalid agent ID"}})
		return
	}

	attachments, err := h.providerService.ListAttachments(c.Request.Context(), agentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "Failed to list attachments"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": attachments})
}

func (h *LLMProviderHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/llm-providers", h.CreateProvider)
	r.GET("/llm-providers", h.ListProviders)
	r.GET("/llm-providers/:id", h.GetProvider)
	r.PUT("/llm-providers/:id", h.UpdateProvider)
	r.DELETE("/llm-providers/:id", h.DeleteProvider)

	r.POST("/agents/:agent_id/llm-attachments", h.CreateAttachment)
	r.GET("/agents/:agent_id/llm-attachments", h.ListAttachments)
}
