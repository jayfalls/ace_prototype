package handlers

import (
	"net/http"

	"github.com/ace/framework/backend/internal/middleware"
	"github.com/ace/framework/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UserHandler struct {
	userService *services.UserService
	authService *services.AuthService
}

func NewUserHandler(userService *services.UserService, authService *services.AuthService) *UserHandler {
	return &UserHandler{
		userService: userService,
		authService: authService,
	}
}

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Username string `json:"username" binding:"required,alphanum,min=3,max=30"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type UpdateUserRequest struct {
	Email    string `json:"email" binding:"omitempty,email"`
	Username string `json:"username" binding:"omitempty,alphanum,min=3,max=30"`
}

func (h *UserHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	// Hash password
	hash, err := h.authService.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "Failed to process request"}})
		return
	}

	// Create user
	user, err := h.userService.CreateUser(c.Request.Context(), services.CreateUserInput{
		Email:        req.Email,
		Username:     req.Username,
		PasswordHash: hash,
		Role:         "user",
	})

	if err == services.ErrUserExists {
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "CONFLICT", "message": "User already exists"}})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "Failed to create user"}})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": user})
}

func (h *UserHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	// Get user by email
	user, err := h.userService.GetUserByEmail(c.Request.Context(), req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Invalid credentials"}})
		return
	}

	// Check password
	if !h.authService.CheckPassword(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Invalid credentials"}})
		return
	}

	// Generate tokens
	tokens, err := h.authService.GenerateTokenPair(user.ID, user.Username, user.Email, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "Failed to generate token"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": tokens})
}

func (h *UserHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	tokens, err := h.authService.RefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Invalid refresh token"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": tokens})
}

func (h *UserHandler) GetMe(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Not authenticated"}})
		return
	}

	id, _ := uuid.Parse(userID)
	user, err := h.userService.GetUserByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "User not found"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": user})
}

func (h *UserHandler) UpdateMe(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Not authenticated"}})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	id, _ := uuid.Parse(userID)
	user, err := h.userService.UpdateUser(c.Request.Context(), id, req.Email, req.Username, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "Failed to update user"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": user})
}

func (h *UserHandler) DeleteMe(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "Not authenticated"}})
		return
	}

	id, _ := uuid.Parse(userID)
	if err := h.userService.DeleteUser(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "Failed to delete user"}})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *UserHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/register", h.Register)
	r.POST("/login", h.Login)
	r.POST("/refresh", h.Refresh)

	auth := r.Group("")
	auth.Use(middleware.AuthMiddleware(h.authService))
	{
		auth.GET("/me", h.GetMe)
		auth.PUT("/me", h.UpdateMe)
		auth.DELETE("/me", h.DeleteMe)
	}
}
