package model

import (
	"time"

	"github.com/google/uuid"
)

// PermissionLevel represents the level of access granted.
type PermissionLevel string

const (
	PermissionView  PermissionLevel = "view"
	PermissionUse   PermissionLevel = "use"
	PermissionAdmin PermissionLevel = "admin"
)

// ResourceType represents the type of resource being protected.
type ResourceType string

const (
	ResourceTypeAgent  ResourceType = "agent"
	ResourceTypeTool   ResourceType = "tool"
	ResourceTypeSkill  ResourceType = "skill"
	ResourceTypeConfig ResourceType = "config"
)

// ResourcePermission represents a user's permission to access a specific resource.
type ResourcePermission struct {
	ID              uuid.UUID       `json:"id"`
	UserID          uuid.UUID       `json:"user_id"`
	ResourceType    ResourceType    `json:"resource_type"`
	ResourceID      uuid.UUID       `json:"resource_id"`
	PermissionLevel PermissionLevel `json:"permission_level"`
	GrantedBy       uuid.UUID       `json:"granted_by"`
	CreatedAt       time.Time       `json:"created_at"`
}

// HasPermission returns true if the permission level is sufficient.
func (p *ResourcePermission) HasPermission(requiredLevel PermissionLevel) bool {
	levelOrder := map[PermissionLevel]int{
		PermissionView:  1,
		PermissionUse:   2,
		PermissionAdmin: 3,
	}

	current, ok := levelOrder[p.PermissionLevel]
	if !ok {
		return false
	}

	required, ok := levelOrder[requiredLevel]
	if !ok {
		return false
	}

	return current >= required
}
