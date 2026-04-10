package model

import (
	"time"

	"github.com/google/uuid"
)

// PermissionLevel represents the level of permission for a resource.
type PermissionLevel string

const (
	PermissionView  PermissionLevel = "view"
	PermissionUse   PermissionLevel = "use"
	PermissionAdmin PermissionLevel = "admin"
)

// ResourcePermission represents a permission granted to a user for a specific resource.
type ResourcePermission struct {
	ID              uuid.UUID       `json:"id"`
	UserID          uuid.UUID       `json:"user_id"`
	ResourceType    string          `json:"resource_type"`
	ResourceID      uuid.UUID       `json:"resource_id"`
	PermissionLevel PermissionLevel `json:"permission_level"`
	GrantedBy       *uuid.UUID      `json:"granted_by,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}

// HasPermission checks if the permission satisfies the required level.
func (p *ResourcePermission) HasPermission(required PermissionLevel) bool {
	levelValues := map[PermissionLevel]int{
		PermissionView:  1,
		PermissionUse:   2,
		PermissionAdmin: 3,
	}

	granted := levelValues[p.PermissionLevel]
	req := levelValues[required]

	return granted >= req
}

// ResourceTypes defines the available resource types in the system.
type ResourceType string

const (
	ResourceTypeAgent  ResourceType = "agent"
	ResourceTypeTool   ResourceType = "tool"
	ResourceTypeSkill  ResourceType = "skill"
	ResourceTypeConfig ResourceType = "config"
)

// IsValidResourceType checks if the resource type is valid.
func IsValidResourceType(rt string) bool {
	switch rt {
	case string(ResourceTypeAgent), string(ResourceTypeTool),
		string(ResourceTypeSkill), string(ResourceTypeConfig):
		return true
	}
	return false
}
