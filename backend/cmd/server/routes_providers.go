package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/ace/framework/backend/internal/config"
	"github.com/ace/framework/backend/internal/db"
	"github.com/ace/framework/backend/internal/llm"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// setupProviderRoutes configures LLM provider endpoints
func setupProviderRoutes(router *gin.Engine, v1 *gin.RouterGroup, database db.Database, cfg *config.Config) {
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

	// List providers
	protected.GET("/providers", func(c *gin.Context) {
		userID := c.GetString("userID")
		providers, err := database.ListProviders(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to list providers"}})
			return
		}
		// Mask API keys
		for i := range providers {
			providers[i].APIKey = "***"
		}
		c.JSON(http.StatusOK, gin.H{"data": providers})
	})

	// Get provider
	protected.GET("/providers/:id", func(c *gin.Context) {
		id := c.Param("id")
		provider, err := database.GetProvider(c.Request.Context(), id)
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
			Name         string `json:"name" binding:"required"`
			ProviderType string `json:"provider_type" binding:"required"`
			APIKey       string `json:"api_key" binding:"required"`
			BaseURL      string `json:"base_url"`
			Model        string `json:"model"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}

		// Get userID from context
		userID := c.GetString("userID")

		// Get the base URL for the provider type
		baseURL := req.BaseURL
		if baseURL == "" {
			switch req.ProviderType {
			case "openrouter":
				baseURL = "https://openrouter.ai/api/v1"
			case "openai":
				baseURL = "https://api.openai.com/v1"
			case "anthropic":
				baseURL = "https://api.anthropic.com"
			case "xai":
				baseURL = "https://api.x.ai/v1"
			case "ollama":
				baseURL = "http://localhost:11434"
			case "llama.cpp":
				baseURL = "http://localhost:8080"
			case "deepseek":
				baseURL = "https://api.deepseek.com/v1"
			case "mistral":
				baseURL = "https://api.mistral.ai/v1"
			case "cohere":
				baseURL = "https://api.cohere.ai/v1"
			default:
				baseURL = "https://api.openai.com/v1"
			}
		}

		provider := &db.Provider{
			ID:           uuid.New().String(),
			OwnerID:      userID,
			Name:         req.Name,
			Type:         req.ProviderType,
			APIKey:       req.APIKey,
			BaseURL:      baseURL,
			Model:        req.Model,
			Enabled:      true,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		if err := database.CreateProvider(c.Request.Context(), provider); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to create provider"}})
			return
		}

		// Mask API key in response
		provider.APIKey = "***"
		c.JSON(http.StatusCreated, gin.H{"data": provider})
	})

	// Test provider connection
	protected.POST("/providers/test", func(c *gin.Context) {
		var req struct {
			ProviderType string `json:"provider_type"`
			APIKey       string `json:"api_key"`
			BaseURL      string `json:"base_url"`
			Model        string `json:"model"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}

		// Get the base URL for the provider type
		baseURL := req.BaseURL
		if baseURL == "" {
			switch req.ProviderType {
			case "openrouter":
				baseURL = "https://openrouter.ai/api/v1"
			case "openai":
				baseURL = "https://api.openai.com/v1"
			case "anthropic":
				baseURL = "https://api.anthropic.com"
			case "xai":
				baseURL = "https://api.x.ai/v1"
			case "ollama":
				baseURL = "http://localhost:11434"
			case "llama.cpp":
				baseURL = "http://localhost:8080"
			case "deepseek":
				baseURL = "https://api.deepseek.com/v1"
			case "mistral":
				baseURL = "https://api.mistral.ai/v1"
			case "cohere":
				baseURL = "https://api.cohere.ai/v1"
			default:
				baseURL = "https://api.openai.com/v1"
			}
		}

		llmConfig := llm.Config{
			APIKey:     req.APIKey,
			BaseURL:    baseURL,
			Model:      req.Model,
			MaxRetries: 3,
			Timeout:    30,
		}

		providerType := llm.ProviderType(req.ProviderType)
		provider, err := llm.NewProvider(providerType, llmConfig)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "PROVIDER_ERROR", "message": err.Error()}})
			return
		}

		err = provider.(llm.Provider).TestConnection()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "CONNECTION_FAILED", "message": err.Error()}})
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "ok", "message": "Connection successful"}})
	})

	// Update provider
	protected.PUT("/providers/:id", func(c *gin.Context) {
		id := c.Param("id")
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

		provider, err := database.GetProvider(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Provider not found"}})
			return
		}

		if req.Name != "" {
			provider.Name = req.Name
		}
		if req.ProviderType != "" {
			provider.Type = req.ProviderType
		}
		if req.APIKey != "" && req.APIKey != "***" {
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
		provider.UpdatedAt = time.Now().UTC()

		if err := database.UpdateProvider(c.Request.Context(), provider); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to update provider"}})
			return
		}

		provider.APIKey = "***"
		c.JSON(http.StatusOK, gin.H{"data": provider})
	})

	// Delete provider
	protected.DELETE("/providers/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := database.DeleteProvider(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Provider not found"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Provider deleted"})
	})
}
