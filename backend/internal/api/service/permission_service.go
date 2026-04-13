// Package service provides permission management services.
package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"ace/internal/api/model"
	db "ace/internal/api/repository/generated"
)

// PermissionService handles resource-level permission operations.
type PermissionService struct {
	queries *db.Queries
}

// NewPermissionService creates a new permission service with the given dependencies.
func NewPermissionService(queries *db.Queries) (*PermissionService, error) {
	if queries == nil {
		return nil, errors.New("queries is required")
	}

	return &PermissionService{
		queries: queries,
	}, nil
}

// GrantPermission creates or updates a permission for a user on a resource.
// It uses upsert to handle existing permissions.
func (s *PermissionService) GrantPermission(
	ctx context.Context,
	userID uuid.UUID,
	resourceType model.ResourceType,
	resourceID uuid.UUID,
	permissionLevel model.PermissionLevel,
	grantedBy uuid.UUID,
) (*model.ResourcePermission, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if userID == uuid.Nil {
		return nil, errors.New("user ID is required")
	}
	if resourceID == uuid.Nil {
		return nil, errors.New("resource ID is required")
	}
	if !isValidPermissionLevel(permissionLevel) {
		return nil, errors.New("invalid permission level")
	}
	if grantedBy == uuid.Nil {
		return nil, errors.New("granted by is required")
	}

	// Validate resource type
	if !isValidResourceType(resourceType) {
		return nil, errors.New("invalid resource type")
	}

	// Upsert permission (creates or updates)
	dbPerm, err := s.queries.UpsertPermission(ctx, db.UpsertPermissionParams{
		ID:              uuid.New().String(),
		UserID:          userID.String(),
		ResourceType:    string(resourceType),
		ResourceID:      resourceID.String(),
		PermissionLevel: string(permissionLevel),
		GrantedBy:       sql.NullString{String: grantedBy.String(), Valid: true},
		CreatedAt:       "",
	})
	if err != nil {
		return nil, fmt.Errorf("upsert permission: %w", err)
	}

	return s.dbToModel(dbPerm), nil
}

// RevokePermission removes a permission for a user on a resource.
func (s *PermissionService) RevokePermission(
	ctx context.Context,
	userID uuid.UUID,
	resourceType model.ResourceType,
	resourceID uuid.UUID,
) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	if userID == uuid.Nil {
		return errors.New("user ID is required")
	}
	if resourceID == uuid.Nil {
		return errors.New("resource ID is required")
	}

	err := s.queries.DeletePermission(ctx, db.DeletePermissionParams{
		UserID:       userID.String(),
		ResourceType: string(resourceType),
		ResourceID:   resourceID.String(),
	})
	if err != nil {
		return fmt.Errorf("delete permission: %w", err)
	}

	return nil
}

// CheckPermission verifies if a user has the required permission level for a resource.
// Permission hierarchy: view (1) < use (2) < admin (3)
// Returns true if the user's permission level is >= required level.
func (s *PermissionService) CheckPermission(
	ctx context.Context,
	userID uuid.UUID,
	resourceType model.ResourceType,
	resourceID uuid.UUID,
	requiredLevel model.PermissionLevel,
) (bool, error) {
	if ctx == nil {
		return false, errors.New("context is required")
	}
	if userID == uuid.Nil {
		return false, errors.New("user ID is required")
	}
	if resourceID == uuid.Nil {
		return false, errors.New("resource ID is required")
	}
	if !isValidPermissionLevel(requiredLevel) {
		return false, errors.New("invalid required permission level")
	}

	dbPerm, err := s.queries.GetPermission(ctx, db.GetPermissionParams{
		UserID:       userID.String(),
		ResourceType: string(resourceType),
		ResourceID:   resourceID.String(),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No permission exists
			return false, nil
		}
		return false, fmt.Errorf("get permission: %w", err)
	}

	// Convert and check using model's HasPermission method
	perm := s.dbToModel(dbPerm)
	return perm.HasPermission(requiredLevel), nil
}

// ListUserPermissions retrieves all permissions for a specific user.
func (s *PermissionService) ListUserPermissions(ctx context.Context, userID uuid.UUID) ([]model.ResourcePermission, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if userID == uuid.Nil {
		return nil, errors.New("user ID is required")
	}

	dbPerms, err := s.queries.ListPermissionsByUser(ctx, userID.String())
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}

	return s.dbToModelList(dbPerms), nil
}

// ListResourcePermissions retrieves all users with permissions for a specific resource.
func (s *PermissionService) ListResourcePermissions(
	ctx context.Context,
	resourceType model.ResourceType,
	resourceID uuid.UUID,
) ([]model.ResourcePermission, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if resourceID == uuid.Nil {
		return nil, errors.New("resource ID is required")
	}

	// Validate resource type
	if !isValidResourceType(resourceType) {
		return nil, errors.New("invalid resource type")
	}

	dbPerms, err := s.queries.ListPermissionsByResource(ctx, db.ListPermissionsByResourceParams{
		ResourceType: string(resourceType),
		ResourceID:   resourceID.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}

	return s.dbToModelList(dbPerms), nil
}

// dbToModel converts a database ResourcePermission to a model ResourcePermission.
func (s *PermissionService) dbToModel(dbPerm *db.ResourcePermission) *model.ResourcePermission {
	if dbPerm == nil {
		return nil
	}

	id, _ := uuid.Parse(dbPerm.ID)
	userID, _ := uuid.Parse(dbPerm.UserID)
	resourceID, _ := uuid.Parse(dbPerm.ResourceID)
	grantedBy, _ := uuid.Parse(dbPerm.GrantedBy.String)
	createdAt, _ := time.Parse(time.RFC3339, dbPerm.CreatedAt)

	return &model.ResourcePermission{
		ID:              id,
		UserID:          userID,
		ResourceType:    model.ResourceType(dbPerm.ResourceType),
		ResourceID:      resourceID,
		PermissionLevel: model.PermissionLevel(dbPerm.PermissionLevel),
		GrantedBy:       grantedBy,
		CreatedAt:       createdAt,
	}
}

// dbToModelList converts a slice of database permissions to model permissions.
func (s *PermissionService) dbToModelList(dbPerms []*db.ResourcePermission) []model.ResourcePermission {
	if dbPerms == nil {
		return nil
	}

	result := make([]model.ResourcePermission, 0, len(dbPerms))
	for _, p := range dbPerms {
		if m := s.dbToModel(p); m != nil {
			result = append(result, *m)
		}
	}

	return result
}

// isValidPermissionLevel checks if the permission level is valid.
func isValidPermissionLevel(level model.PermissionLevel) bool {
	switch level {
	case model.PermissionView, model.PermissionUse, model.PermissionAdmin:
		return true
	default:
		return false
	}
}

// isValidResourceType checks if the resource type is valid.
func isValidResourceType(rt model.ResourceType) bool {
	switch rt {
	case model.ResourceTypeAgent, model.ResourceTypeTool,
		model.ResourceTypeSkill, model.ResourceTypeConfig:
		return true
	default:
		return false
	}
}
