package handlers

import (
	"net/http"
	"strconv"

	"github.com/ace/framework/backend/internal/middleware"
	"github.com/ace/framework/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type MemoryHandler struct {
	memoryService *services.MemoryService
}

func NewMemoryHandler(memoryService *services.MemoryService) *MemoryHandler {
	return &MemoryHandler{memoryService: memoryService}
}

type CreateMemoryRequest struct {
	AgentID    *uuid.UUID `json:"agent_id"`
	Content    string     `json:"content" binding:"required"`
	MemoryType string     `json:"memory_type"`
	ParentID   *uuid.UUID `json:"parent_id"`
	Tags       []string   `json:"tags"`
}

type UpdateMemoryRequest struct {
	Content    *string    `json:"content"`
	MemoryType *string    `json:"memory_type"`
	ParentID   *uuid.UUID `json:"parent_id"`
	Tags       *[]string  `json:"tags"`
}

func (h *MemoryHandler) Create(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Not authenticated"}})
		return
	}

	var req CreateMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	ownerID, _ := uuid.Parse(userID)
	memory, err := h.memoryService.CreateMemory(c.Request.Context(), services.CreateMemoryInput{
		OwnerID:    ownerID,
		AgentID:    req.AgentID,
		Content:    req.Content,
		MemoryType: req.MemoryType,
		ParentID:   req.ParentID,
		Tags:       req.Tags,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "Failed to create memory"}})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": memory})
}

func (h *MemoryHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Not authenticated"}})
		return
	}

	ownerID, _ := uuid.Parse(userID)
	memoryType := c.Query("type")
	search := c.Query("search")
	agentIDStr := c.Query("agent_id")
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "20"), 10, 32)
	offset, _ := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 32)

	var agentID *uuid.UUID
	if agentIDStr != "" {
		id, _ := uuid.Parse(agentIDStr)
		agentID = &id
	}

	memories, err := h.memoryService.ListMemories(c.Request.Context(), ownerID, memoryType, search, agentID, int32(limit), int32(offset))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "Failed to list memories"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": memories})
}

func (h *MemoryHandler) Get(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Not authenticated"}})
		return
	}

	memoryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "Invalid memory ID"}})
		return
	}

	ownerID, _ := uuid.Parse(userID)
	memory, err := h.memoryService.GetMemory(c.Request.Context(), memoryID, ownerID)
	if err == services.ErrMemoryNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Memory not found"}})
		return
	}
	if err == services.ErrMemoryAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "FORBIDDEN", "message": "Access denied"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": memory})
}

func (h *MemoryHandler) Update(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Not authenticated"}})
		return
	}

	memoryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "Invalid memory ID"}})
		return
	}

	var req UpdateMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	ownerID, _ := uuid.Parse(userID)
	memory, err := h.memoryService.UpdateMemory(c.Request.Context(), memoryID, ownerID, req.Content, req.MemoryType, req.ParentID, req.Tags, nil)
	if err == services.ErrMemoryNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Memory not found"}})
		return
	}
	if err == services.ErrMemoryAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "FORBIDDEN", "message": "Access denied"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": memory})
}

func (h *MemoryHandler) Delete(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Not authenticated"}})
		return
	}

	memoryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "Invalid memory ID"}})
		return
	}

	ownerID, _ := uuid.Parse(userID)
	err = h.memoryService.DeleteMemory(c.Request.Context(), memoryID, ownerID)
	if err == services.ErrMemoryNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Memory not found"}})
		return
	}
	if err == services.ErrMemoryAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "FORBIDDEN", "message": "Access denied"}})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *MemoryHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/memories", h.Create)
	r.GET("/memories", h.List)
	r.GET("/memories/:id", h.Get)
	r.PUT("/memories/:id", h.Update)
	r.DELETE("/memories/:id", h.Delete)
}
