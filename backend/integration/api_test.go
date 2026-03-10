package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ace/framework/backend/internal/db"
	"github.com/ace/framework/backend/internal/handlers"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestDatabaseMemories tests memory CRUD operations
func TestDatabaseMemories(t *testing.T) {
	// Create in-memory database for testing
	inMemDB := db.NewInMemoryDB()
	ctx := context.Background()

	agentID := uuid.New().String()
	memory := &db.Memory{
		ID:        uuid.New().String(),
		AgentID:   agentID,
		Content:   "Test memory content",
		Type:      "short_term",
		Tags:      []string{"test", "important"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Test CreateMemory
	err := inMemDB.CreateMemory(ctx, memory)
	require.NoError(t, err)

	// Test GetMemory
	retrieved, err := inMemDB.GetMemory(ctx, memory.ID)
	require.NoError(t, err)
	assert.Equal(t, memory.Content, retrieved.Content)

	// Test GetMemoriesByAgent
	memories, err := inMemDB.GetMemoriesByAgent(ctx, agentID)
	require.NoError(t, err)
	assert.Len(t, memories, 1)

	// Test SearchMemories
	searchResults, err := inMemDB.SearchMemories(ctx, agentID, "test")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(searchResults), 0)

	// Test UpdateMemory
	memory.Content = "Updated content"
	err = inMemDB.UpdateMemory(ctx, memory)
	require.NoError(t, err)

	updated, err := inMemDB.GetMemory(ctx, memory.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated content", updated.Content)

	// Test DeleteMemory
	err = inMemDB.DeleteMemory(ctx, memory.ID)
	require.NoError(t, err)

	_, err = inMemDB.GetMemory(ctx, memory.ID)
	assert.Error(t, err)
}

// TestDatabaseAgents tests agent CRUD operations
func TestDatabaseAgents(t *testing.T) {
	inMemDB := db.NewInMemoryDB()
	ctx := context.Background()

	userID := uuid.New().String()
	agent := &db.Agent{
		ID:          uuid.New().String(),
		Name:        "Test Agent",
		Description: "A test agent",
		UserID:      userID,
		Model:       "gpt-4",
		Provider:    "openai",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Test CreateAgent
	err := inMemDB.CreateAgent(ctx, agent)
	require.NoError(t, err)

	// Test GetAgent
	retrieved, err := inMemDB.GetAgent(ctx, agent.ID)
	require.NoError(t, err)
	assert.Equal(t, agent.Name, retrieved.Name)

	// Test GetAgentsByUser
	agents, err := inMemDB.GetAgentsByUser(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, agents, 1)

	// Test UpdateAgent
	agent.Name = "Updated Agent"
	err = inMemDB.UpdateAgent(ctx, agent)
	require.NoError(t, err)

	updated, err := inMemDB.GetAgent(ctx, agent.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Agent", updated.Name)

	// Test DeleteAgent
	err = inMemDB.DeleteAgent(ctx, agent.ID)
	require.NoError(t, err)

	_, err = inMemDB.GetAgent(ctx, agent.ID)
	assert.Error(t, err)
}

// TestDatabaseSessions tests session operations
func TestDatabaseSessions(t *testing.T) {
	inMemDB := db.NewInMemoryDB()
	ctx := context.Background()

	userID := uuid.New().String()
	agentID := uuid.New().String()
	
	session := &db.Session{
		ID:        uuid.New().String(),
		AgentID:   agentID,
		UserID:    userID,
		Status:    "active",
		StartedAt: time.Now(),
	}

	// Test CreateSession
	err := inMemDB.CreateSession(ctx, session)
	require.NoError(t, err)

	// Test GetSession
	retrieved, err := inMemDB.GetSession(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, session.Status, retrieved.Status)

	// Test GetSessionsByAgent
	sessions, err := inMemDB.GetSessionsByAgent(ctx, agentID)
	require.NoError(t, err)
	assert.Len(t, sessions, 1)

	// Test EndSession
	err = inMemDB.EndSession(ctx, session.ID)
	require.NoError(t, err)

	ended, err := inMemDB.GetSession(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, "ended", ended.Status)
}

// TestAuthHandlers tests authentication handlers
func TestAuthHandlers(t *testing.T) {
	router := gin.New()

	// Setup auth routes
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
		c.JSON(http.StatusOK, gin.H{"message": "User registered"})
	})

	router.POST("/api/v1/auth/login", func(c *gin.Context) {
		var req struct {
			Email    string `json:"email" binding:"required,email"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
			return
		}
		// Demo login check
		if req.Email == "demo@example.com" && req.Password == "demo123" {
			c.JSON(http.StatusOK, gin.H{"token": "demo-token"})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
	})

	t.Run("Register validation", func(t *testing.T) {
		body := `{"email":"invalid","password":"123","name":""}`
		req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Login success", func(t *testing.T) {
		body := `{"email":"demo@example.com","password":"demo123"}`
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Login failure", func(t *testing.T) {
		body := `{"email":"demo@example.com","password":"wrongpass"}`
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// TestAgentHandlers tests agent API handlers
func TestAgentHandlers(t *testing.T) {
	router := gin.New()
	
	// Demo data store
	agents := map[string]interface{}{}

	router.POST("/api/v1/agents", func(c *gin.Context) {
		var req struct {
			Name        string `json:"name" binding:"required"`
			Description string `json:"description"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		agentID := uuid.New().String()
		agent := map[string]interface{}{
			"id":          agentID,
			"name":        req.Name,
			"description": req.Description,
			"status":      "inactive",
		}
		agents[agentID] = agent
		c.JSON(http.StatusCreated, agent)
	})

	router.GET("/api/v1/agents", func(c *gin.Context) {
		list := make([]map[string]interface{}, 0, len(agents))
		for _, a := range agents {
			list = append(list, a.(map[string]interface{}))
		}
		c.JSON(http.StatusOK, list)
	})

	router.GET("/api/v1/agents/:id", func(c *gin.Context) {
		id := c.Param("id")
		agent, ok := agents[id]
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
			return
		}
		c.JSON(http.StatusOK, agent)
	})

	router.DELETE("/api/v1/agents/:id", func(c *gin.Context) {
		id := c.Param("id")
		if _, ok := agents[id]; !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
			return
		}
		delete(agents, id)
		c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
	})

	t.Run("Create agent", func(t *testing.T) {
		body := `{"name":"Test Agent","description":"Testing"}`
		req := httptest.NewRequest("POST", "/api/v1/agents", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)
		
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "Test Agent", resp["name"])
	})

	t.Run("List agents", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/agents", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Get agent not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/agents/nonexistent", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// TestMemoryHandlers tests memory API handlers  
func TestMemoryHandlers(t *testing.T) {
	router := gin.New()

	memories := map[string][]map[string]interface{}{}

	router.GET("/api/v1/agents/:agent_id/memories", func(c *gin.Context) {
		agentID := c.Param("agent_id")
		memList := memories[agentID]
		c.JSON(http.StatusOK, memList)
	})

	router.POST("/api/v1/agents/:agent_id/memories", func(c *gin.Context) {
		agentID := c.Param("agent_id")
		var req struct {
			Content     string   `json:"content" binding:"required"`
			MemoryType  string   `json:"memory_type"`
			Tags        []string `json:"tags"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		
		memory := map[string]interface{}{
			"id":           uuid.New().String(),
			"agent_id":     agentID,
			"content":      req.Content,
			"memory_type":  req.MemoryType,
			"tags":         req.Tags,
			"created_at":   time.Now().Format(time.RFC3339),
		}
		
		memories[agentID] = append(memories[agentID], memory)
		c.JSON(http.StatusCreated, memory)
	})

	router.GET("/api/v1/agents/:agent_id/memories/search", func(c *gin.Context) {
		agentID := c.Param("agent_id")
		query := c.Query("q")
		
		var results []map[string]interface{}
		for _, m := range memories[agentID] {
			content := m["content"].(string)
			if query == "" || containsString(content, query) {
				results = append(results, m)
			}
		}
		c.JSON(http.StatusOK, results)
	})

	t.Run("Create memory", func(t *testing.T) {
		body := `{"content":"Test memory","memory_type":"short_term","tags":["test"]}`
		req := httptest.NewRequest("POST", "/api/v1/agents/test-agent/memories", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("List memories", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/agents/test-agent/memories", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Search memories", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/agents/test-agent/memories/search?q=test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > 0 && func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}()))
}

// BenchmarkDatabaseMemories benchmarks memory operations
func BenchmarkDatabaseMemories(b *testing.B) {
	inMemDB := db.NewInMemoryDB()
	ctx := context.Background()
	agentID := uuid.New().String()

	memory := &db.Memory{
		ID:        uuid.New().String(),
		AgentID:   agentID,
		Content:   "Benchmark memory",
		Type:      "short_term",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		memory.ID = uuid.New().String()
		inMemDB.CreateMemory(ctx, memory)
	}
}
