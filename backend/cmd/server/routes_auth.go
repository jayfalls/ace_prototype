package main

import (
	"net/http"
	"time"

	"github.com/ace/framework/backend/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// setupAuthRoutes configures authentication endpoints
func setupAuthRoutes(router *gin.Engine, v1 *gin.RouterGroup, database db.Database) {
	// Register
	v1.POST("/auth/register", func(c *gin.Context) {
		var req struct {
			Email    string `json:"email" binding:"required,email"`
			Password string `json:"password" binding:"required,min=8"`
			Name     string `json:"name" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}

		// Check if user exists
		existing, _ := database.GetUserByEmail(c.Request.Context(), req.Email)
		if existing != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "USER_EXISTS", "message": "Email already registered"}})
			return
		}

		// Hash password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "HASH_ERROR", "message": "Failed to hash password"}})
			return
		}

		// Create user
		user := &db.User{
			ID:        uuid.New().String(),
			Email:     req.Email,
			Name:      req.Name,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := database.CreateUser(c.Request.Context(), user); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DB_ERROR", "message": "Failed to create user"}})
			return
		}

		// Generate token
		token, err := generateTokenFromDB(user)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "TOKEN_ERROR", "message": "Failed to generate token"}})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"data": gin.H{"user": user, "token": token}})
	})

	// Login
	v1.POST("/auth/login", func(c *gin.Context) {
		var req struct {
			Email    string `json:"email" binding:"required,email"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}

		// Find user
		user, err := database.GetUserByEmail(c.Request.Context(), req.Email)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Invalid credentials"}})
			return
		}

		// Verify password
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Invalid credentials"}})
			return
		}

		// Generate token
		token, err := generateTokenFromDB(user)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "TOKEN_ERROR", "message": "Failed to generate token"}})
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": gin.H{"user": user, "token": token}})
	})

	// Get current user
	v1.GET("/auth/me", func(c *gin.Context) {
		userID := c.GetString("userID")
		user, err := database.GetUser(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "User not found"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": user})
	})
}
