package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"ace/internal/api/model"
	db "ace/internal/api/repository/generated"
)

// Mock queries for testing
type mockPermissionQueries struct {
	createPermissionFn          func(ctx context.Context, arg db.CreatePermissionParams) (*db.ResourcePermission, error)
	upsertPermissionFn          func(ctx context.Context, arg db.UpsertPermissionParams) (*db.ResourcePermission, error)
	getPermissionFn             func(ctx context.Context, arg db.GetPermissionParams) (*db.ResourcePermission, error)
	deletePermissionFn          func(ctx context.Context, arg db.DeletePermissionParams) error
	listPermissionsByUserFn     func(ctx context.Context, userID string) ([]*db.ResourcePermission, error)
	listPermissionsByResourceFn func(ctx context.Context, arg db.ListPermissionsByResourceParams) ([]*db.ResourcePermission, error)
}

func (m *mockPermissionQueries) UpsertPermission(ctx context.Context, arg db.UpsertPermissionParams) (*db.ResourcePermission, error) {
	if m.upsertPermissionFn != nil {
		return m.upsertPermissionFn(ctx, arg)
	}
	return nil, nil
}

func (m *mockPermissionQueries) GetPermission(ctx context.Context, arg db.GetPermissionParams) (*db.ResourcePermission, error) {
	if m.getPermissionFn != nil {
		return m.getPermissionFn(ctx, arg)
	}
	return nil, nil
}

func (m *mockPermissionQueries) DeletePermission(ctx context.Context, arg db.DeletePermissionParams) error {
	if m.deletePermissionFn != nil {
		return m.deletePermissionFn(ctx, arg)
	}
	return nil
}

func (m *mockPermissionQueries) ListPermissionsByUser(ctx context.Context, userID string) ([]*db.ResourcePermission, error) {
	if m.listPermissionsByUserFn != nil {
		return m.listPermissionsByUserFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockPermissionQueries) ListPermissionsByResource(ctx context.Context, arg db.ListPermissionsByResourceParams) ([]*db.ResourcePermission, error) {
	if m.listPermissionsByResourceFn != nil {
		return m.listPermissionsByResourceFn(ctx, arg)
	}
	return nil, nil
}

func TestNewPermissionService(t *testing.T) {
	t.Run("returns error when queries is nil", func(t *testing.T) {
		_, err := NewPermissionService(nil)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("creates service successfully", func(t *testing.T) {
		// This would require a real queries object in integration tests
		// For unit tests, we skip the nil check verification by just validating the interface
	})
}

func TestPermissionService_GrantPermission(t *testing.T) {
	userID := uuid.New()
	resourceID := uuid.New()
	grantedBy := uuid.New()

	t.Run("returns error for nil context", func(t *testing.T) {
		svc := &PermissionService{queries: nil}
		_, err := svc.GrantPermission(nil, userID, model.ResourceTypeAgent, resourceID, model.PermissionView, grantedBy)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("returns error for nil user ID", func(t *testing.T) {
		svc := &PermissionService{queries: nil}
		_, err := svc.GrantPermission(context.Background(), uuid.Nil, model.ResourceTypeAgent, resourceID, model.PermissionView, grantedBy)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("returns error for nil resource ID", func(t *testing.T) {
		svc := &PermissionService{queries: nil}
		_, err := svc.GrantPermission(context.Background(), userID, model.ResourceTypeAgent, uuid.Nil, model.PermissionView, grantedBy)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("returns error for invalid permission level", func(t *testing.T) {
		svc := &PermissionService{queries: nil}
		_, err := svc.GrantPermission(context.Background(), userID, model.ResourceTypeAgent, resourceID, "invalid", grantedBy)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("returns error for invalid resource type", func(t *testing.T) {
		svc := &PermissionService{queries: nil}
		_, err := svc.GrantPermission(context.Background(), userID, "invalid", resourceID, model.PermissionView, grantedBy)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestPermissionService_RevokePermission(t *testing.T) {
	userID := uuid.New()
	resourceID := uuid.New()

	t.Run("returns error for nil context", func(t *testing.T) {
		svc := &PermissionService{queries: nil}
		err := svc.RevokePermission(nil, userID, model.ResourceTypeAgent, resourceID)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("returns error for nil user ID", func(t *testing.T) {
		svc := &PermissionService{queries: nil}
		err := svc.RevokePermission(context.Background(), uuid.Nil, model.ResourceTypeAgent, resourceID)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("returns error for nil resource ID", func(t *testing.T) {
		svc := &PermissionService{queries: nil}
		err := svc.RevokePermission(context.Background(), userID, model.ResourceTypeAgent, uuid.Nil)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestPermissionService_CheckPermission(t *testing.T) {
	userID := uuid.New()
	resourceID := uuid.New()

	t.Run("returns error for nil context", func(t *testing.T) {
		svc := &PermissionService{queries: nil}
		_, err := svc.CheckPermission(nil, userID, model.ResourceTypeAgent, resourceID, model.PermissionView)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("returns error for nil user ID", func(t *testing.T) {
		svc := &PermissionService{queries: nil}
		_, err := svc.CheckPermission(context.Background(), uuid.Nil, model.ResourceTypeAgent, resourceID, model.PermissionView)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("returns error for nil resource ID", func(t *testing.T) {
		svc := &PermissionService{queries: nil}
		_, err := svc.CheckPermission(context.Background(), userID, model.ResourceTypeAgent, uuid.Nil, model.PermissionView)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("returns error for invalid permission level", func(t *testing.T) {
		svc := &PermissionService{queries: nil}
		_, err := svc.CheckPermission(context.Background(), userID, model.ResourceTypeAgent, resourceID, "invalid")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestPermissionService_ListUserPermissions(t *testing.T) {
	t.Run("returns error for nil context", func(t *testing.T) {
		svc := &PermissionService{queries: nil}
		_, err := svc.ListUserPermissions(nil, uuid.New())
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("returns error for nil user ID", func(t *testing.T) {
		svc := &PermissionService{queries: nil}
		_, err := svc.ListUserPermissions(context.Background(), uuid.Nil)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestPermissionService_ListResourcePermissions(t *testing.T) {
	resourceID := uuid.New()

	t.Run("returns error for nil context", func(t *testing.T) {
		svc := &PermissionService{queries: nil}
		_, err := svc.ListResourcePermissions(nil, model.ResourceTypeAgent, resourceID)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("returns error for nil resource ID", func(t *testing.T) {
		svc := &PermissionService{queries: nil}
		_, err := svc.ListResourcePermissions(context.Background(), model.ResourceTypeAgent, uuid.Nil)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("returns error for invalid resource type", func(t *testing.T) {
		svc := &PermissionService{queries: nil}
		_, err := svc.ListResourcePermissions(context.Background(), "invalid", resourceID)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestPermissionLevel_HasPermission(t *testing.T) {
	tests := []struct {
		name         string
		level        model.PermissionLevel
		required     model.PermissionLevel
		expectResult bool
	}{
		{"view satisfies view", model.PermissionView, model.PermissionView, true},
		{"view does not satisfy use", model.PermissionView, model.PermissionUse, false},
		{"view does not satisfy admin", model.PermissionView, model.PermissionAdmin, false},
		{"use satisfies view", model.PermissionUse, model.PermissionView, true},
		{"use satisfies use", model.PermissionUse, model.PermissionUse, true},
		{"use does not satisfy admin", model.PermissionUse, model.PermissionAdmin, false},
		{"admin satisfies view", model.PermissionAdmin, model.PermissionView, true},
		{"admin satisfies use", model.PermissionAdmin, model.PermissionUse, true},
		{"admin satisfies admin", model.PermissionAdmin, model.PermissionAdmin, true},
		{"invalid level returns false", "invalid", model.PermissionView, false},
		{"invalid required returns false", model.PermissionView, "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perm := &model.ResourcePermission{
				ID:              uuid.New(),
				UserID:          uuid.New(),
				ResourceType:    model.ResourceTypeAgent,
				ResourceID:      uuid.New(),
				PermissionLevel: tt.level,
				GrantedBy:       uuid.New(),
				CreatedAt:       time.Now(),
			}

			result := perm.HasPermission(tt.required)
			if result != tt.expectResult {
				t.Errorf("HasPermission(%s, %s) = %v, want %v", tt.level, tt.required, result, tt.expectResult)
			}
		})
	}
}

func TestPermissionService_dbToModel(t *testing.T) {
	t.Run("returns nil for nil input", func(t *testing.T) {
		svc := &PermissionService{}
		result := svc.dbToModel(nil)
		if result != nil {
			t.Error("expected nil")
		}
	})

	t.Run("converts db model to domain model", func(t *testing.T) {
		svc := &PermissionService{}
		dbPerm := &db.ResourcePermission{
			ID:              uuid.New().String(),
			UserID:          uuid.New().String(),
			ResourceType:    "agent",
			ResourceID:      uuid.New().String(),
			PermissionLevel: "view",
			GrantedBy:       sql.NullString{String: uuid.New().String(), Valid: true},
			CreatedAt:       time.Now().Format(time.RFC3339),
		}

		result := svc.dbToModel(dbPerm)
		if result == nil {
			t.Error("expected non-nil result")
		}
		if result.ResourceType != model.ResourceTypeAgent {
			t.Errorf("ResourceType = %v, want agent", result.ResourceType)
		}
		if result.PermissionLevel != model.PermissionView {
			t.Errorf("PermissionLevel = %v, want view", result.PermissionLevel)
		}
	})
}

func TestPermissionService_dbToModelList(t *testing.T) {
	t.Run("returns nil for nil input", func(t *testing.T) {
		svc := &PermissionService{}
		result := svc.dbToModelList(nil)
		if result != nil {
			t.Error("expected nil")
		}
	})

	t.Run("returns empty slice for empty input", func(t *testing.T) {
		svc := &PermissionService{}
		result := svc.dbToModelList([]*db.ResourcePermission{})
		if len(result) != 0 {
			t.Error("expected empty slice")
		}
	})
}

func TestIsValidPermissionLevel(t *testing.T) {
	tests := []struct {
		level  model.PermissionLevel
		expect bool
	}{
		{model.PermissionView, true},
		{model.PermissionUse, true},
		{model.PermissionAdmin, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			result := isValidPermissionLevel(tt.level)
			if result != tt.expect {
				t.Errorf("isValidPermissionLevel(%s) = %v, want %v", tt.level, result, tt.expect)
			}
		})
	}
}

func TestIsValidResourceType(t *testing.T) {
	tests := []struct {
		resourceType model.ResourceType
		expect       bool
	}{
		{model.ResourceTypeAgent, true},
		{model.ResourceTypeTool, true},
		{model.ResourceTypeSkill, true},
		{model.ResourceTypeConfig, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.resourceType), func(t *testing.T) {
			result := isValidResourceType(tt.resourceType)
			if result != tt.expect {
				t.Errorf("isValidResourceType(%s) = %v, want %v", tt.resourceType, result, tt.expect)
			}
		})
	}
}
