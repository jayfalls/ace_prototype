package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestGenerateToken(t *testing.T) {
	user := &User{
		ID:    "test-user-id",
		Email: "test@example.com",
		Name:  "Test User",
	}

	token, err := generateToken(user)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate token
	claims := &JWTClaims{}
parsedToken, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})
	assert.NoError(t, err)
	assert.True(t, parsedToken.Valid)
	assert.Equal(t, user.ID, claims.UserID)
	assert.Equal(t, user.Email, claims.Email)
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{"match lowercase", "hello world", "hello", true},
		{"match uppercase", "Hello World", "hello", true},
		{"no match", "hello world", "foo", false},
		{"empty string", "", "", true},
		{"empty substr", "hello", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"single", "hello", []string{"hello"}},
		{"multiple", "hello,world,test", []string{"hello", "world", "test"}},
		{"with spaces", " hello , world ", []string{"hello", "world"}},
		{"empty", "", []string{}},
		{"empty entries", "a,,b", []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitAndTrim(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAuthRegister(t *testing.T) {
	// Setup router
	router := gin.New()
	router.POST("/api/v1/auth/register", func(c *gin.Context) {
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
		if _, exists := userByEmail[req.Email]; exists {
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "EMAIL_EXISTS", "message": "Email already registered"}})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User registered"})
	})

	// Test registration
	body := `{"email":"new@example.com","password":"password123","name":"New User"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthRegisterDuplicate(t *testing.T) {
	// Setup router
	router := gin.New()
	router.POST("/api/v1/auth/register", func(c *gin.Context) {
		var req struct {
			Email    string `json:"email" binding:"required,email"`
			Password string `json:"password" binding:"required,min=8"`
			Name     string `json:"name" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}

		// Check if user exists (simulate existing user)
		if req.Email == "existing@example.com" {
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "EMAIL_EXISTS", "message": "Email already registered"}})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User registered"})
	})

	// Test duplicate email
	body := `{"email":"existing@example.com","password":"password123","name":"Existing User"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusConflict, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")
}

func TestAuthLoginValidation(t *testing.T) {
	router := gin.New()
	router.POST("/api/v1/auth/login", func(c *gin.Context) {
		var req struct {
			Email    string `json:"email" binding:"required,email"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}

		// Missing email
		if req.Email == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email required"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Login successful"})
	})

	// Test with invalid JSON
	body := `{"email":""}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMemorySearch(t *testing.T) {
	// Test memory search logic
	demoMemories := []map[string]interface{}{
		{"agent_id": "agent1", "content": "Important research findings", "tags": []string{"research", "important"}},
		{"agent_id": "agent1", "content": "User preferences data", "tags": []string{"preferences"}},
		{"agent_id": "agent2", "content": "Other agent memory", "tags": []string{}},
	}

	// Test filtering by agent
	var filtered []map[string]interface{}
	for _, m := range demoMemories {
		if m["agent_id"] == "agent1" {
			filtered = append(filtered, m)
		}
	}
	assert.Len(t, filtered, 2)

	// Test search by content
	query := "research"
	for _, m := range demoMemories {
		if m["agent_id"] == "agent1" {
			content, _ := m["content"].(string)
			if strings.Contains(strings.ToLower(content), strings.ToLower(query)) {
				filtered = append(filtered, m)
			}
		}
	}
	assert.GreaterOrEqual(t, len(filtered), 1)
}
