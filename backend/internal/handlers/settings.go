package handlers

import (
	"net/http"

	"github.com/ace/framework/backend/internal/services"
	"github.com/gin-gonic/gin"
)

type SettingsHandler struct {
	settingsService *services.SettingsService
}

func NewSettingsHandler(settingsService *services.SettingsService) *SettingsHandler {
	return &SettingsHandler{settingsService: settingsService}
}

func (h *SettingsHandler) ListAgentSettings(c *gin.Context) {
	// TODO: Implement with real DB queries
	c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
}

func (h *SettingsHandler) UpsertAgentSetting(c *gin.Context) {
	// TODO: Implement with real DB queries
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"success": true}})
}

func (h *SettingsHandler) DeleteAgentSetting(c *gin.Context) {
	// TODO: Implement with real DB queries
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"success": true}})
}

func (h *SettingsHandler) ListSystemSettings(c *gin.Context) {
	// TODO: Implement with real DB queries
	c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
}

func (h *SettingsHandler) UpsertSystemSetting(c *gin.Context) {
	// TODO: Implement with real DB queries
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"success": true}})
}
