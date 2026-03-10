package db

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ace/framework/backend/internal/config"
	"github.com/rs/zerolog/log"
)

var logger = log.Logger

// Database interface for data access
type Database interface {
	// Users
	CreateUser(ctx context.Context, user *User) error
	GetUserByID(ctx context.Context, id string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	UpdateUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, id string) error

	// Agents
	CreateAgent(ctx context.Context, agent *Agent) error
	GetAgent(ctx context.Context, id string) (*Agent, error)
	GetAgents(ctx context.Context, userID string) ([]Agent, error)
	UpdateAgent(ctx context.Context, agent *Agent) error
	DeleteAgent(ctx context.Context, id string) error

	// Memories
	CreateMemory(ctx context.Context, memory *Memory) error
	GetMemory(ctx context.Context, id string) (*Memory, error)
	GetMemoriesByAgent(ctx context.Context, agentID string) ([]Memory, error)
	SearchMemories(ctx context.Context, agentID, query string) ([]Memory, error)
	UpdateMemory(ctx context.Context, memory *Memory) error
	DeleteMemory(ctx context.Context, id string) error

	// Tools
	CreateTool(ctx context.Context, tool *Tool) error
	GetTool(ctx context.Context, id string) (*Tool, error)
	GetTools(ctx context.Context) ([]Tool, error)
	UpdateTool(ctx context.Context, tool *Tool) error
	DeleteTool(ctx context.Context, id string) error

	// Providers
	CreateProvider(ctx context.Context, provider *Provider) error
	GetProvider(ctx context.Context, id string) (*Provider, error)
	GetProviders(ctx context.Context) ([]Provider, error)
	UpdateProvider(ctx context.Context, provider *Provider) error
	DeleteProvider(ctx context.Context, id string) error

	// Chat Messages
	CreateChatMessage(ctx context.Context, msg *ChatMessage) error
	GetChatMessages(ctx context.Context, agentID string, limit int) ([]ChatMessage, error)

	Close() error
}

// User represents a user in the system
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Password  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Agent represents an agent in the system
type Agent struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	UserID      string                 `json:"user_id"`
	Config      map[string]interface{} `json:"config"`
	Model       string                 `json:"model"`
	Provider    string                 `json:"provider"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// Memory represents a memory in the system
type Memory struct {
	ID        string                 `json:"id"`
	AgentID   string                 `json:"agent_id"`
	Type      string                 `json:"type"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// Tool represents a tool in the system
type Tool struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Provider represents an LLM provider
type Provider struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	APIKey    string `json:"-"`
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	Enabled   bool   `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ChatMessage represents a chat message
type ChatMessage struct {
	ID        string `json:"id"`
	AgentID   string `json:"agent_id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// ============ PostgreSQL Database ============

type PostgresDB struct {
	pool *pgxpool.Pool
}

func NewPostgresDB(ctx context.Context, connStr string) (*PostgresDB, error) {
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &PostgresDB{pool: pool}
	if err := db.migrate(ctx); err != nil {
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}

	return db, nil
}

func (db *PostgresDB) migrate(ctx context.Context) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			name VARCHAR(255) NOT NULL,
			password VARCHAR(255) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS agents (
			id UUID PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			user_id UUID REFERENCES users(id),
			config JSONB,
			model VARCHAR(255),
			provider VARCHAR(255),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS memories (
			id UUID PRIMARY KEY,
			agent_id UUID REFERENCES agents(id) ON DELETE CASCADE,
			type VARCHAR(255) NOT NULL,
			content TEXT NOT NULL,
			metadata JSONB,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS tools (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			category VARCHAR(255),
			enabled BOOLEAN DEFAULT true,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS providers (
			id UUID PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			type VARCHAR(255) NOT NULL,
			api_key VARCHAR(255),
			base_url VARCHAR(500),
			model VARCHAR(255),
			enabled BOOLEAN DEFAULT true,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS chat_messages (
			id UUID PRIMARY KEY,
			agent_id UUID REFERENCES agents(id) ON DELETE CASCADE,
			role VARCHAR(50) NOT NULL,
			content TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
	}

	for _, m := range migrations {
		if _, err := db.pool.Exec(ctx, m); err != nil {
			return err
		}
	}

	return nil
}

func (db *PostgresDB) Close() error {
	db.pool.Close()
	return nil
}

// User methods
func (db *PostgresDB) CreateUser(ctx context.Context, user *User) error {
	_, err := db.pool.Exec(ctx,
		"INSERT INTO users (id, email, name, password, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)",
		user.ID, user.Email, user.Name, user.Password, user.CreatedAt, user.UpdatedAt)
	return err
}

func (db *PostgresDB) GetUserByID(ctx context.Context, id string) (*User, error) {
	var user User
	err := db.pool.QueryRow(ctx, "SELECT id, email, name, password, created_at, updated_at FROM users WHERE id = $1", id).
		Scan(&user.ID, &user.Email, &user.Name, &user.Password, &user.CreatedAt, &user.UpdatedAt)
	return &user, err
}

func (db *PostgresDB) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := db.pool.QueryRow(ctx, "SELECT id, email, name, password, created_at, updated_at FROM users WHERE email = $1", email).
		Scan(&user.ID, &user.Email, &user.Name, &user.Password, &user.CreatedAt, &user.UpdatedAt)
	return &user, err
}

func (db *PostgresDB) UpdateUser(ctx context.Context, user *User) error {
	_, err := db.pool.Exec(ctx, "UPDATE users SET email = $1, name = $2, updated_at = $3 WHERE id = $4",
		user.Email, user.Name, user.UpdatedAt, user.ID)
	return err
}

func (db *PostgresDB) DeleteUser(ctx context.Context, id string) error {
	_, err := db.pool.Exec(ctx, "DELETE FROM users WHERE id = $1", id)
	return err
}

// Agent methods
func (db *PostgresDB) CreateAgent(ctx context.Context, agent *Agent) error {
	config, _ := json.Marshal(agent.Config)
	_, err := db.pool.Exec(ctx,
		"INSERT INTO agents (id, name, description, user_id, config, model, provider, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
		agent.ID, agent.Name, agent.Description, agent.UserID, config, agent.Model, agent.Provider, agent.CreatedAt, agent.UpdatedAt)
	return err
}

func (db *PostgresDB) GetAgent(ctx context.Context, id string) (*Agent, error) {
	var agent Agent
	var config []byte
	err := db.pool.QueryRow(ctx, "SELECT id, name, description, user_id, config, model, provider, created_at, updated_at FROM agents WHERE id = $1", id).
		Scan(&agent.ID, &agent.Name, &agent.Description, &agent.UserID, &config, &agent.Model, &agent.Provider, &agent.CreatedAt, &agent.UpdatedAt)
	if config != nil {
		json.Unmarshal(config, &agent.Config)
	}
	return &agent, err
}

func (db *PostgresDB) GetAgents(ctx context.Context, userID string) ([]Agent, error) {
	rows, err := db.pool.Query(ctx, "SELECT id, name, description, user_id, config, model, provider, created_at, updated_at FROM agents WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []Agent
	for rows.Next() {
		var agent Agent
		var config []byte
		rows.Scan(&agent.ID, &agent.Name, &agent.Description, &agent.UserID, &config, &agent.Model, &agent.Provider, &agent.CreatedAt, &agent.UpdatedAt)
		if config != nil {
			json.Unmarshal(config, &agent.Config)
		}
		agents = append(agents, agent)
	}
	return agents, nil
}

func (db *PostgresDB) UpdateAgent(ctx context.Context, agent *Agent) error {
	config, _ := json.Marshal(agent.Config)
	_, err := db.pool.Exec(ctx, "UPDATE agents SET name = $1, description = $2, config = $3, model = $4, provider = $5, updated_at = $6 WHERE id = $7",
		agent.Name, agent.Description, config, agent.Model, agent.Provider, agent.UpdatedAt, agent.ID)
	return err
}

func (db *PostgresDB) DeleteAgent(ctx context.Context, id string) error {
	_, err := db.pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", id)
	return err
}

// Memory methods
func (db *PostgresDB) CreateMemory(ctx context.Context, memory *Memory) error {
	metadata, _ := json.Marshal(memory.Metadata)
	_, err := db.pool.Exec(ctx,
		"INSERT INTO memories (id, agent_id, type, content, metadata, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		memory.ID, memory.AgentID, memory.Type, memory.Content, metadata, memory.CreatedAt, memory.UpdatedAt)
	return err
}

func (db *PostgresDB) GetMemory(ctx context.Context, id string) (*Memory, error) {
	var memory Memory
	var metadata []byte
	err := db.pool.QueryRow(ctx, "SELECT id, agent_id, type, content, metadata, created_at, updated_at FROM memories WHERE id = $1", id).
		Scan(&memory.ID, &memory.AgentID, &memory.Type, &memory.Content, &metadata, &memory.CreatedAt, &memory.UpdatedAt)
	if metadata != nil {
		json.Unmarshal(metadata, &memory.Metadata)
	}
	return &memory, err
}

func (db *PostgresDB) GetMemoriesByAgent(ctx context.Context, agentID string) ([]Memory, error) {
	rows, err := db.pool.Query(ctx, "SELECT id, agent_id, type, content, metadata, created_at, updated_at FROM memories WHERE agent_id = $1 ORDER BY created_at DESC", agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []Memory
	for rows.Next() {
		var memory Memory
		var metadata []byte
		rows.Scan(&memory.ID, &memory.AgentID, &memory.Type, &memory.Content, &metadata, &memory.CreatedAt, &memory.UpdatedAt)
		if metadata != nil {
			json.Unmarshal(metadata, &memory.Metadata)
		}
		memories = append(memories, memory)
	}
	return memories, nil
}

func (db *PostgresDB) SearchMemories(ctx context.Context, agentID, query string) ([]Memory, error) {
	rows, err := db.pool.Query(ctx, "SELECT id, agent_id, type, content, metadata, created_at, updated_at FROM memories WHERE agent_id = $1 AND content ILIKE $2 ORDER BY created_at DESC", agentID, "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []Memory
	for rows.Next() {
		var memory Memory
		var metadata []byte
		rows.Scan(&memory.ID, &memory.AgentID, &memory.Type, &memory.Content, &metadata, &memory.CreatedAt, &memory.UpdatedAt)
		if metadata != nil {
			json.Unmarshal(metadata, &memory.Metadata)
		}
		memories = append(memories, memory)
	}
	return memories, nil
}

func (db *PostgresDB) UpdateMemory(ctx context.Context, memory *Memory) error {
	metadata, _ := json.Marshal(memory.Metadata)
	_, err := db.pool.Exec(ctx, "UPDATE memories SET type = $1, content = $2, metadata = $3, updated_at = $4 WHERE id = $5",
		memory.Type, memory.Content, metadata, memory.UpdatedAt, memory.ID)
	return err
}

func (db *PostgresDB) DeleteMemory(ctx context.Context, id string) error {
	_, err := db.pool.Exec(ctx, "DELETE FROM memories WHERE id = $1", id)
	return err
}

// Tool methods
func (db *PostgresDB) CreateTool(ctx context.Context, tool *Tool) error {
	_, err := db.pool.Exec(ctx,
		"INSERT INTO tools (id, name, description, category, enabled, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		tool.ID, tool.Name, tool.Description, tool.Category, tool.Enabled, tool.CreatedAt, tool.UpdatedAt)
	return err
}

func (db *PostgresDB) GetTool(ctx context.Context, id string) (*Tool, error) {
	var tool Tool
	err := db.pool.QueryRow(ctx, "SELECT id, name, description, category, enabled, created_at, updated_at FROM tools WHERE id = $1", id).
		Scan(&tool.ID, &tool.Name, &tool.Description, &tool.Category, &tool.Enabled, &tool.CreatedAt, &tool.UpdatedAt)
	return &tool, err
}

func (db *PostgresDB) GetTools(ctx context.Context) ([]Tool, error) {
	rows, err := db.pool.Query(ctx, "SELECT id, name, description, category, enabled, created_at, updated_at FROM tools")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tools []Tool
	for rows.Next() {
		var tool Tool
		rows.Scan(&tool.ID, &tool.Name, &tool.Description, &tool.Category, &tool.Enabled, &tool.CreatedAt, &tool.UpdatedAt)
		tools = append(tools, tool)
	}
	return tools, nil
}

func (db *PostgresDB) UpdateTool(ctx context.Context, tool *Tool) error {
	_, err := db.pool.Exec(ctx, "UPDATE tools SET name = $1, description = $2, category = $3, enabled = $4, updated_at = $5 WHERE id = $6",
		tool.Name, tool.Description, tool.Category, tool.Enabled, tool.UpdatedAt, tool.ID)
	return err
}

func (db *PostgresDB) DeleteTool(ctx context.Context, id string) error {
	_, err := db.pool.Exec(ctx, "DELETE FROM tools WHERE id = $1", id)
	return err
}

// Provider methods
func (db *PostgresDB) CreateProvider(ctx context.Context, provider *Provider) error {
	_, err := db.pool.Exec(ctx,
		"INSERT INTO providers (id, name, type, api_key, base_url, model, enabled, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
		provider.ID, provider.Name, provider.Type, provider.APIKey, provider.BaseURL, provider.Model, provider.Enabled, provider.CreatedAt, provider.UpdatedAt)
	return err
}

func (db *PostgresDB) GetProvider(ctx context.Context, id string) (*Provider, error) {
	var provider Provider
	err := db.pool.QueryRow(ctx, "SELECT id, name, type, api_key, base_url, model, enabled, created_at, updated_at FROM providers WHERE id = $1", id).
		Scan(&provider.ID, &provider.Name, &provider.Type, &provider.APIKey, &provider.BaseURL, &provider.Model, &provider.Enabled, &provider.CreatedAt, &provider.UpdatedAt)
	return &provider, err
}

func (db *PostgresDB) GetProviders(ctx context.Context) ([]Provider, error) {
	rows, err := db.pool.Query(ctx, "SELECT id, name, type, api_key, base_url, model, enabled, created_at, updated_at FROM providers")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []Provider
	for rows.Next() {
		var provider Provider
		rows.Scan(&provider.ID, &provider.Name, &provider.Type, &provider.APIKey, &provider.BaseURL, &provider.Model, &provider.Enabled, &provider.CreatedAt, &provider.UpdatedAt)
		providers = append(providers, provider)
	}
	return providers, nil
}

func (db *PostgresDB) UpdateProvider(ctx context.Context, provider *Provider) error {
	_, err := db.pool.Exec(ctx, "UPDATE providers SET name = $1, type = $2, api_key = $3, base_url = $4, model = $5, enabled = $6, updated_at = $7 WHERE id = $8",
		provider.Name, provider.Type, provider.APIKey, provider.BaseURL, provider.Model, provider.Enabled, provider.UpdatedAt, provider.ID)
	return err
}

func (db *PostgresDB) DeleteProvider(ctx context.Context, id string) error {
	_, err := db.pool.Exec(ctx, "DELETE FROM providers WHERE id = $1", id)
	return err
}

// Chat messages
func (db *PostgresDB) CreateChatMessage(ctx context.Context, msg *ChatMessage) error {
	_, err := db.pool.Exec(ctx,
		"INSERT INTO chat_messages (id, agent_id, role, content, created_at) VALUES ($1, $2, $3, $4, $5)",
		msg.ID, msg.AgentID, msg.Role, msg.Content, msg.CreatedAt)
	return err
}

func (db *PostgresDB) GetChatMessages(ctx context.Context, agentID string, limit int) ([]ChatMessage, error) {
	rows, err := db.pool.Query(ctx, "SELECT id, agent_id, role, content, created_at FROM chat_messages WHERE agent_id = $1 ORDER BY created_at DESC LIMIT $2", agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []ChatMessage
	for rows.Next() {
		var msg ChatMessage
		rows.Scan(&msg.ID, &msg.AgentID, &msg.Role, &msg.Content, &msg.CreatedAt)
		messages = append(messages, msg)
	}
	return messages, nil
}

// ============ In-Memory Database (MVP) ============

type InMemoryDB struct {
	mu          sync.RWMutex
	users       map[string]*User
	usersByEmail map[string]*User
	agents      map[string]*Agent
	memories    map[string]*Memory
	tools       map[string]*Tool
	providers   map[string]*Provider
	messages    map[string][]ChatMessage
}

func NewInMemoryDB() *InMemoryDB {
	db := &InMemoryDB{
		users:     make(map[string]*User),
		agents:    make(map[string]*Agent),
		memories:  make(map[string]*Memory),
		tools:     make(map[string]*Tool),
		providers: make(map[string]*Provider),
		messages:  make(map[string][]ChatMessage),
	}
	db.usersByEmail = make(map[string]*User)
	
	// Add default tools
	defaultTools := []Tool{
		{ID: "web_search", Name: "Web Search", Description: "Search the web for information", Category: "research", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "file_read", Name: "File Read", Description: "Read files from disk", Category: "filesystem", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "file_write", Name: "File Write", Description: "Write files to disk", Category: "filesystem", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "execute_code", Name: "Execute Code", Description: "Run code snippets", Category: "execution", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "curl", Name: "HTTP Request", Description: "Make HTTP requests", Category: "network", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "database_query", Name: "Database Query", Description: "Execute database queries", Category: "data", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	for _, tool := range defaultTools {
		db.tools[tool.ID] = &tool
	}
	
	return db
}

func (db *InMemoryDB) Close() error { return nil }

// User methods
func (db *InMemoryDB) CreateUser(ctx context.Context, user *User) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.users[user.ID] = user
	db.usersByEmail[user.Email] = user
	return nil
}

func (db *InMemoryDB) GetUserByID(ctx context.Context, id string) (*User, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	if user, ok := db.users[id]; ok {
		return user, nil
	}
	return nil, fmt.Errorf("user not found")
}

func (db *InMemoryDB) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	if user, ok := db.usersByEmail[email]; ok {
		return user, nil
	}
	return nil, fmt.Errorf("user not found")
}

func (db *InMemoryDB) UpdateUser(ctx context.Context, user *User) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.users[user.ID] = user
	db.usersByEmail[user.Email] = user
	return nil
}

func (db *InMemoryDB) DeleteUser(ctx context.Context, id string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if user, ok := db.users[id]; ok {
		delete(db.usersByEmail, user.Email)
		delete(db.users, id)
	}
	return nil
}

// Agent methods
func (db *InMemoryDB) CreateAgent(ctx context.Context, agent *Agent) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.agents[agent.ID] = agent
	return nil
}

func (db *InMemoryDB) GetAgent(ctx context.Context, id string) (*Agent, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	if agent, ok := db.agents[id]; ok {
		return agent, nil
	}
	return nil, fmt.Errorf("agent not found")
}

func (db *InMemoryDB) GetAgents(ctx context.Context, userID string) ([]Agent, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	var agents []Agent
	for _, agent := range db.agents {
		if agent.UserID == userID {
			agents = append(agents, *agent)
		}
	}
	return agents, nil
}

func (db *InMemoryDB) UpdateAgent(ctx context.Context, agent *Agent) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.agents[agent.ID] = agent
	return nil
}

func (db *InMemoryDB) DeleteAgent(ctx context.Context, id string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	delete(db.agents, id)
	return nil
}

// Memory methods
func (db *InMemoryDB) CreateMemory(ctx context.Context, memory *Memory) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.memories[memory.ID] = memory
	return nil
}

func (db *InMemoryDB) GetMemory(ctx context.Context, id string) (*Memory, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	if memory, ok := db.memories[id]; ok {
		return memory, nil
	}
	return nil, fmt.Errorf("memory not found")
}

func (db *InMemoryDB) GetMemoriesByAgent(ctx context.Context, agentID string) ([]Memory, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	var memories []Memory
	for _, memory := range db.memories {
		if memory.AgentID == agentID {
			memories = append(memories, *memory)
		}
	}
	return memories, nil
}

func (db *InMemoryDB) SearchMemories(ctx context.Context, agentID, query string) ([]Memory, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	var memories []Memory
	for _, memory := range db.memories {
		if memory.AgentID == agentID && contains(memory.Content, query) {
			memories = append(memories, *memory)
		}
	}
	return memories, nil
}

func (db *InMemoryDB) UpdateMemory(ctx context.Context, memory *Memory) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.memories[memory.ID] = memory
	return nil
}

func (db *InMemoryDB) DeleteMemory(ctx context.Context, id string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	delete(db.memories, id)
	return nil
}

// Tool methods
func (db *InMemoryDB) CreateTool(ctx context.Context, tool *Tool) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.tools[tool.ID] = tool
	return nil
}

func (db *InMemoryDB) GetTool(ctx context.Context, id string) (*Tool, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	if tool, ok := db.tools[id]; ok {
		return tool, nil
	}
	return nil, fmt.Errorf("tool not found")
}

func (db *InMemoryDB) GetTools(ctx context.Context) ([]Tool, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	var tools []Tool
	for _, tool := range db.tools {
		tools = append(tools, *tool)
	}
	return tools, nil
}

func (db *InMemoryDB) UpdateTool(ctx context.Context, tool *Tool) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.tools[tool.ID] = tool
	return nil
}

func (db *InMemoryDB) DeleteTool(ctx context.Context, id string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	delete(db.tools, id)
	return nil
}

// Provider methods
func (db *InMemoryDB) CreateProvider(ctx context.Context, provider *Provider) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.providers[provider.ID] = provider
	return nil
}

func (db *InMemoryDB) GetProvider(ctx context.Context, id string) (*Provider, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	if provider, ok := db.providers[id]; ok {
		return provider, nil
	}
	return nil, fmt.Errorf("provider not found")
}

func (db *InMemoryDB) GetProviders(ctx context.Context) ([]Provider, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	var providers []Provider
	for _, provider := range db.providers {
		providers = append(providers, *provider)
	}
	return providers, nil
}

func (db *InMemoryDB) UpdateProvider(ctx context.Context, provider *Provider) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.providers[provider.ID] = provider
	return nil
}

func (db *InMemoryDB) DeleteProvider(ctx context.Context, id string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	delete(db.providers, id)
	return nil
}

// Chat messages
func (db *InMemoryDB) CreateChatMessage(ctx context.Context, msg *ChatMessage) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.messages[msg.AgentID] = append([]ChatMessage{*msg}, db.messages[msg.AgentID]...)
	return nil
}

func (db *InMemoryDB) GetChatMessages(ctx context.Context, agentID string, limit int) ([]ChatMessage, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	messages := db.messages[agentID]
	if limit > 0 && len(messages) > limit {
		return messages[:limit], nil
	}
	return messages, nil
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// NewDatabase creates a new database based on environment
func NewDatabase(ctx context.Context, cfg *config.DatabaseConfig) (Database, error) {
	// Try PostgreSQL first if config is provided
	if cfg != nil && cfg.Host != "" {
		connStr := fmt.Sprintf(
			"postgres://%s:%s@%s:%d/%s?sslmode=%s",
			cfg.User,
			cfg.Password,
			cfg.Host,
			cfg.Port,
			cfg.DBName,
			cfg.SSLMode,
		)
		// Try PostgreSQL connection
		if pgdb, err := NewPostgresDB(ctx, connStr); err == nil {
			logger.Info().Str("host", cfg.Host).Msg("Connected to PostgreSQL")
			return pgdb, nil
		} else {
			logger.Warn().Err(err).Msgf("Failed to connect to PostgreSQL at %s, using in-memory", cfg.Host)
		}
	}
	
	// Fall back to DATABASE_URL env var
	connStr := os.Getenv("DATABASE_URL")
	if connStr != "" {
		return NewPostgresDB(ctx, connStr)
	}
	
	// Default to in-memory
	logger.Info().Msg("Using in-memory database")
	return NewInMemoryDB(), nil
}

// CreateDemoData creates demo data for testing
func (db *InMemoryDB) CreateDemoData() {
	db.mu.Lock()
	defer db.mu.Unlock()

	now := time.Now()
	userID := uuid.New().String()
	
	// Create demo user
	db.users[userID] = &User{
		ID:        userID,
		Email:     "demo@example.com",
		Name:      "Demo User",
		Password:  "demo123",
		CreatedAt: now,
		UpdatedAt: now,
	}
	db.usersByEmail["demo@example.com"] = db.users[userID]

	// Create demo agents
	agents := []Agent{
		{ID: uuid.New().String(), Name: "Research Agent", Description: "Helps with research tasks", UserID: userID, Model: "gpt-4", Provider: "openai", CreatedAt: now, UpdatedAt: now},
		{ID: uuid.New().String(), Name: "Coding Agent", Description: "Helps with coding tasks", UserID: userID, Model: "claude-3-5-sonnet-20241022", Provider: "anthropic", CreatedAt: now, UpdatedAt: now},
		{ID: uuid.New().String(), Name: "Writing Agent", Description: "Helps with writing tasks", UserID: userID, Model: "gpt-4o", Provider: "openai", CreatedAt: now, UpdatedAt: now},
	}
	for _, agent := range agents {
		agentCopy := agent
		db.agents[agentCopy.ID] = &agentCopy
	}

	// Create demo providers
	providers := []Provider{
		{ID: uuid.New().String(), Name: "OpenAI", Type: "openai", Model: "gpt-4", Enabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: uuid.New().String(), Name: "Anthropic", Type: "anthropic", Model: "claude-3-5-sonnet-20241022", Enabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: uuid.New().String(), Name: "Ollama", Type: "ollama", BaseURL: "http://localhost:11434", Model: "llama2", Enabled: true, CreatedAt: now, UpdatedAt: now},
	}
	for _, provider := range providers {
		providerCopy := provider
		db.providers[providerCopy.ID] = &providerCopy
	}
}
