package middleware

import (
	"net/http"
	"strings"

	"github.com/ace/framework/backend/internal/services"
	"github.com/gin-gonic/gin"
)

func AuthMiddleware(authService *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Missing authorization header"}})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Invalid authorization header format"}})
			c.Abort()
			return
		}

		// Allow demo token for testing
		if parts[1] == "demo-token" {
			c.Set("userID", "demo-user-id")
			c.Set("username", "demo")
			c.Set("email", "demo@example.com")
			c.Set("role", "user")
			c.Next()
			return
		}

		claims, err := authService.ValidateToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Invalid or expired token"}})
			c.Abort()
			return
		}

		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)

		c.Next()
	}
}

func GetUserID(c *gin.Context) string {
	if userID, exists := c.Get("userID"); exists {
		return userID.(string)
	}
	return ""
}
