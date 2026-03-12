package main

import (
	"net/http"
	"time"

	"github.com/ace/framework/backend/internal/db"
	"github.com/ace/framework/backend/internal/llm"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RegisterProviderRoutes registers provider-related routes
func RegisterProviderRoutes(protected *gin.RouterGroup) {
	// List providers
	protected.GET("/providers", func(c *gin.Context) {
		providers, err := db.GetProviders(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to get providers"}})
			return
		}
		if providers == nil {
			providers = []db.Provider{}
		}
		// Mask API keys in response
		for i := range providers {
			providers[i].APIKey = "***"
		}
		c.JSON(http.StatusOK, gin.H{"data": providers})
	})

	// Get provider
	protected.GET("/providers/:id", func(c *gin.Context) {
		id := c.Param("id")
		provider, err := db.GetProvider(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Provider not found"}})
			return
		}
		provider.APIKey = "***"
		c.JSON(http.StatusOK, gin.H{"data": provider})
	})

	// Create provider
	protected.POST("/providers", func(c *gin.Context) {
		var req struct {
			Name         string `json:"name"`
			ProviderType string `json:"provider_type"`
			APIKey       string `json:"api_key"`
			BaseURL      string `json:"base_url"`
			Model        string `json:"model"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		
		now := nowFunc()
		provider := &db.Provider{
			ID:       uuid.New().String(),
			Name:     req.Name,
			Type:     req.ProviderType,
			APIKey:  req.APIKey,
			BaseURL: req.BaseURL,
			Model:   req.Model,
			Enabled: true,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := db.CreateProvider(c.Request.Context(), provider); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to create provider"}})
			return
		}
		provider.APIKey = "***"
		c.JSON(http.StatusCreated, gin.H{"data": provider})
	})

	// Update provider
	protected.PUT("/providers/:id", func(c *gin.Context) {
		id := c.Param("id")
		provider, err := db.GetProvider(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Provider not found"}})
			return
		}
		var req struct {
			Name         string `json:"name"`
			ProviderType string `json:"provider_type"`
			APIKey       string `json:"api_key"`
			BaseURL      string `json:"base_url"`
			Model        string `json:"model"`
			Enabled      *bool  `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		if req.Name != "" {
			provider.Name = req.Name
		}
		if req.ProviderType != "" {
			provider.Type = req.ProviderType
		}
		if req.APIKey != "" {
			provider.APIKey = req.APIKey
		}
		if req.BaseURL != "" {
			provider.BaseURL = req.BaseURL
		}
		if req.Model != "" {
			provider.Model = req.Model
		}
		if req.Enabled != nil {
			provider.Enabled = *req.Enabled
		}
		provider.UpdatedAt = nowFunc()
		if err := db.UpdateProvider(c.Request.Context(), provider); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to update provider"}})
			return
		}
		provider.APIKey = "***"
		c.JSON(http.StatusOK, gin.H{"data": provider})
	})

	// Delete provider
	protected.DELETE("/providers/:id", func(c *gin.Context) {
		id := c.Param("id")
		provider, err := db.GetProvider(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Provider not found"}})
			return
		}
		if err := db.DeleteProvider(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to delete provider"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": provider})
	})

	// Test provider connection
	protected.POST("/providers/test", func(c *gin.Context) {
		var req struct {
			ProviderType string `json:"provider_type"`
			APIKey      string `json:"api_key"`
			BaseURL     string `json:"base_url"`
			Model       string `json:"model"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		
		// Try to create provider and test connection
		llmConfig := llm.Config{
			APIKey:     req.APIKey,
			BaseURL:    req.BaseURL,
			Model:      req.Model,
			MaxRetries: 3,
			Timeout:    30,
		}
		providerType := llm.ProviderType(req.ProviderType)
		provider, err := llm.NewProvider(providerType, llmConfig)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "PROVIDER_ERROR", "message": "Failed to create provider: " + err.Error()}})
			return
		}
		
		if p, ok := provider.(llm.Provider); ok {
			err = p.TestConnection()
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "CONNECTION_ERROR", "message": "Connection failed: " + err.Error()}})
				return
			}
			c.JSON(http.StatusOK, gin.H{"data": gin.H{"message": "Connection successful"}})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "PROVIDER_ERROR", "message": "Failed to cast provider"}})
		}
	})
}
