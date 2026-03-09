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

// SQLC Query Parameters
type CreateUserParams struct {
	Email        string `json:"email"`
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
	Role         string `json:"role"`
}

type UpdateUserParams struct {
	ID       uuid.UUID `json:"id"`
	Email    string    `json:"email"`
	Username string    `json:"username"`
	Role     string    `json:"role"`
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

// SQLC Query Parameters
type CreateAgentParams struct {
	OwnerID     uuid.UUID       `json:"owner_id"`
	Name        string          `json:"name"`
	Description *string         `json:"description"`
	Config      json.RawMessage `json:"config"`
	Status      string          `json:"status"`
}

type UpdateAgentParams struct {
	ID          uuid.UUID       `json:"id"`
	Name        string          `json:"name"`
	Description *string         `json:"description"`
	Config      json.RawMessage `json:"config"`
	Status      string          `json:"status"`
}

type Session struct {
	ID        uuid.UUID       `json:"id"`
	AgentID   uuid.UUID       `json:"agent_id"`
	OwnerID   uuid.UUID       `json:"owner_id"`
	Status    string          `json:"status"`
	StartedAt time.Time      `json:"started_at"`
	EndedAt   *time.Time     `json:"ended_at"`
	Metadata  json.RawMessage `json:"metadata"`
}

// SQLC Query Parameters
type CreateSessionParams struct {
	AgentID  uuid.UUID       `json:"agent_id"`
	OwnerID  uuid.UUID       `json:"owner_id"`
	Status   string          `json:"status"`
	Metadata json.RawMessage `json:"metadata"`
}

type EndSessionParams struct {
	ID     uuid.UUID `json:"id"`
	Status string   `json:"status"`
}

type Thought struct {
	ID        uuid.UUID       `json:"id"`
	SessionID uuid.UUID       `json:"session_id"`
	Layer     string          `json:"layer"`
	Content   string          `json:"content"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt time.Time       `json:"created_at"`
}

// SQLC Query Parameters
type CreateThoughtParams struct {
	SessionID uuid.UUID       `json:"session_id"`
	Layer     string          `json:"layer"`
	Content   string          `json:"content"`
	Metadata  json.RawMessage `json:"metadata"`
}

type Memory struct {
	ID          uuid.UUID       `json:"id"`
	OwnerID     uuid.UUID       `json:"owner_id"`
	AgentID     *uuid.UUID     `json:"agent_id"`
	Content     string          `json:"content"`
	MemoryType  string          `json:"memory_type"`
	ParentID    *uuid.UUID     `json:"parent_id"`
	Tags        []string       `json:"tags"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// SQLC Query Parameters
type CreateMemoryParams struct {
	OwnerID    uuid.UUID       `json:"owner_id"`
	AgentID    *uuid.UUID     `json:"agent_id"`
	Content    string          `json:"content"`
	MemoryType string          `json:"memory_type"`
	ParentID   *uuid.UUID     `json:"parent_id"`
	Tags       []string       `json:"tags"`
	Metadata   json.RawMessage `json:"metadata"`
}

type UpdateMemoryParams struct {
	ID         uuid.UUID       `json:"id"`
	Content    string          `json:"content"`
	MemoryType string          `json:"memory_type"`
	ParentID   *uuid.UUID     `json:"parent_id"`
	Tags       []string       `json:"tags"`
	Metadata   json.RawMessage `json:"metadata"`
}

type LLMProvider struct {
	ID              uuid.UUID       `json:"id"`
	OwnerID         uuid.UUID       `json:"owner_id"`
	Name            string          `json:"name"`
	ProviderType    string          `json:"provider_type"`
	APIKeyEncrypted *string        `json:"api_key_encrypted"`
	BaseURL         *string        `json:"base_url"`
	Model           *string        `json:"model"`
	Config          json.RawMessage `json:"config"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// SQLC Query Parameters
type CreateLLMProviderParams struct {
	OwnerID         uuid.UUID       `json:"owner_id"`
	Name            string          `json:"name"`
	ProviderType    string          `json:"provider_type"`
	APIKeyEncrypted *string        `json:"api_key_encrypted"`
	BaseURL         *string        `json:"base_url"`
	Model           *string        `json:"model"`
	Config          json.RawMessage `json:"config"`
}

type UpdateLLMProviderParams struct {
	ID              uuid.UUID       `json:"id"`
	Name            string          `json:"name"`
	ProviderType    string          `json:"provider_type"`
	APIKeyEncrypted *string        `json:"api_key_encrypted"`
	BaseURL         *string        `json:"base_url"`
	Model           *string        `json:"model"`
	Config          json.RawMessage `json:"config"`
}

type CreateLLMAttachmentParams struct {
	AgentID    uuid.UUID       `json:"agent_id"`
	ProviderID uuid.UUID       `json:"provider_id"`
	Layer      string          `json:"layer"`
	Priority   int32           `json:"priority"`
	Config     json.RawMessage `json:"config"`
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

// SQLC Query Parameters
type GetAgentSettingParams struct {
	AgentID uuid.UUID `json:"agent_id"`
	Key     string   `json:"key"`
}

type UpsertAgentSettingParams struct {
	AgentID uuid.UUID `json:"agent_id"`
	Key     string   `json:"key"`
	Value   string   `json:"value"`
}

type DeleteAgentSettingParams struct {
	AgentID uuid.UUID `json:"agent_id"`
	Key     string   `json:"key"`
}

type SystemSetting struct {
	ID        uuid.UUID `json:"id"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SQLC Query Parameters
type UpsertSystemSettingParams struct {
	Key   string `json:"key"`
	Value string `json:"value"`
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

// SQLC Query Parameters
type GetToolWhitelistParams struct {
	AgentID  uuid.UUID `json:"agent_id"`
	ToolName string   `json:"tool_name"`
}

type UpsertToolWhitelistParams struct {
	AgentID  uuid.UUID       `json:"agent_id"`
	ToolName string          `json:"tool_name"`
	Enabled  bool            `json:"enabled"`
	Config   json.RawMessage `json:"config"`
}

type DeleteToolWhitelistParams struct {
	AgentID  uuid.UUID `json:"agent_id"`
	ToolName string   `json:"tool_name"`
}
