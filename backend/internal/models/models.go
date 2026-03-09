package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Agent struct {
	ID          uuid.UUID       `json:"id"`
	OwnerID     uuid.UUID       `json:"owner_id"`
	Name        string          `json:"name"`
	Description *string         `json:"description"`
	Config      json.RawMessage `json:"config"`
	Status      string          `json:"status"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type Session struct {
	ID        uuid.UUID       `json:"id"`
	AgentID   uuid.UUID       `json:"agent_id"`
	OwnerID   uuid.UUID       `json:"owner_id"`
	Status    string          `json:"status"`
	StartedAt time.Time       `json:"started_at"`
	EndedAt   *time.Time      `json:"ended_at"`
	Metadata  json.RawMessage `json:"metadata"`
}

type Thought struct {
	ID        uuid.UUID       `json:"id"`
	SessionID uuid.UUID       `json:"session_id"`
	Layer     string          `json:"layer"`
	Content   string          `json:"content"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt time.Time       `json:"created_at"`
}

type Memory struct {
	ID          uuid.UUID       `json:"id"`
	OwnerID     uuid.UUID       `json:"owner_id"`
	AgentID     *uuid.UUID      `json:"agent_id"`
	Content     string          `json:"content"`
	MemoryType  string          `json:"memory_type"`
	ParentID    *uuid.UUID      `json:"parent_id"`
	Tags        []string        `json:"tags"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type LLMProvider struct {
	ID              uuid.UUID       `json:"id"`
	OwnerID         uuid.UUID       `json:"owner_id"`
	Name            string          `json:"name"`
	ProviderType    string          `json:"provider_type"`
	APIKeyEncrypted *string        `json:"api_key_encrypted"`
	BaseURL         *string         `json:"base_url"`
	Model           *string         `json:"model"`
	Config          json.RawMessage `json:"config"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type LLMAttachment struct {
	ID         uuid.UUID       `json:"id"`
	AgentID    uuid.UUID       `json:"agent_id"`
	ProviderID uuid.UUID       `json:"provider_id"`
	Layer      string          `json:"layer"`
	Priority   int             `json:"priority"`
	Config     json.RawMessage `json:"config"`
	CreatedAt  time.Time       `json:"created_at"`
}

type AgentSetting struct {
	ID        uuid.UUID `json:"id"`
	AgentID   uuid.UUID `json:"agent_id"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SystemSetting struct {
	ID        uuid.UUID `json:"id"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ToolWhitelist struct {
	ID        uuid.UUID       `json:"id"`
	AgentID   uuid.UUID       `json:"agent_id"`
	ToolName  string          `json:"tool_name"`
	Enabled   bool            `json:"enabled"`
	Config    json.RawMessage `json:"config"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}
