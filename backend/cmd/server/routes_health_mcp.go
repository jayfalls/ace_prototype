package main

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// setupHealthRoutes configures health check endpoints
func setupHealthRoutes(router *gin.Engine) {
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "healthy",
			"version":   "1.0.0",
		})
	})

	router.GET("/health/ready", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"ready": true,
			"checks": gin.H{
				"database": true,
			},
		})
	})

	// Prometheus metrics
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

// setupMCPRoutes configures MCP endpoints
func setupMCPRoutes(router *gin.Engine, mcpServer *mcp.Server) {
	router.GET("/mcp/tools", func(c *gin.Context) {
		tools := mcpServer.ListTools()
		c.JSON(200, gin.H{"tools": tools})
	})

	router.POST("/mcp/tools/:name", func(c *gin.Context) {
		toolName := c.Param("name")
		var args map[string]interface{}
		if err := c.ShouldBindJSON(&args); err != nil {
			args = make(map[string]interface{})
		}

		result, err := mcpServer.CallTool(c.Request.Context(), toolName, args)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, result)
	})

	router.GET("/mcp/resources", func(c *gin.Context) {
		resources := mcpServer.ListResources()
		c.JSON(200, gin.H{"resources": resources})
	})

	router.GET("/mcp/prompts", func(c *gin.Context) {
		prompts := mcpServer.ListPrompts()
		c.JSON(200, gin.H{"prompts": prompts})
	})
}
