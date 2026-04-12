// Package middleware provides HTTP middleware for the API service.
package middleware

import (
	"context"
	"errors"
	"net/http"

	"ace/api/internal/model"
)

// ErrForbidden is returned when the user lacks required permissions.
var ErrForbidden = errors.New("forbidden")

// PermissionChecker defines an interface for checking resource-level permissions.
// Implement this to add more granular permission checks.
type PermissionChecker interface {
	// HasPermission checks if the user has the specified permission for a resource.
	// resourceType: e.g., "document", "project", "team"
	// resourceID: the specific resource ID
	// action: e.g., "read", "write", "delete"
	HasPermission(ctx context.Context, userID interface{}, resourceType, resourceID, action string) (bool, error)
}

// RBACMiddleware handles role-based access control.
type RBACMiddleware struct {
	permissionChecker PermissionChecker
}

// NewRBACMiddleware creates a new RBAC middleware.
func NewRBACMiddleware() *RBACMiddleware {
	return &RBACMiddleware{}
}

// SetPermissionChecker sets the permission checker for resource-level permissions.
func (m *RBACMiddleware) SetPermissionChecker(checker PermissionChecker) {
	m.permissionChecker = checker
}

// RequireRole returns a middleware that checks if the authenticated user has the required role.
// The user must have been authenticated via RequireAuth middleware before this middleware runs.
//
// Roles (in order of hierarchy): admin > user > viewer
//   - admin: full access to all resources
//   - user: standard access for regular users
//   - viewer: read-only access
//
// Returns 403 if:
//   - User is not authenticated (context lacks user info)
//   - User's role is not in the allowed roles list
func (m *RBACMiddleware) RequireRole(allowedRoles ...model.UserRole) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user role from context (set by RequireAuth middleware)
			userRole := GetUserRoleFromContext(r.Context())

			if userRole == "" {
				http.Error(w, "user role not found in context", http.StatusForbidden)
				return
			}

			// Check if user's role is in the allowed roles
			allowed := false
			for _, role := range allowedRoles {
				if userRole == role {
					allowed = true
					break
				}
			}

			if !allowed {
				http.Error(w, "insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermission returns a middleware that checks resource-level permissions.
// This is an additional check beyond role-based access.
//
// The user must have been authenticated via RequireAuth middleware before this middleware runs.
//
// Returns 403 if:
//   - User is not authenticated
//   - User lacks the required permission for the resource
func (m *RBACMiddleware) RequirePermission(resourceType, action string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If no permission checker is configured, skip the check
			if m.permissionChecker == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Get user ID from context
			userID := GetUserIDFromContext(r.Context())
			if userID == nil {
				http.Error(w, "user not authenticated", http.StatusForbidden)
				return
			}

			// Get resource ID from URL path (assuming chi router pattern)
			// The resource ID is expected to be in the URL path
			// You may need to adjust this based on your routing structure
			resourceID := r.URL.Path // Default: use full path, override in specific handlers

			// Check permission
			hasPermission, err := m.permissionChecker.HasPermission(
				r.Context(),
				userID,
				resourceType,
				resourceID,
				action,
			)
			if err != nil {
				http.Error(w, "permission check failed", http.StatusInternalServerError)
				return
			}

			if !hasPermission {
				http.Error(w, "insufficient permissions for this resource", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// IsAdmin returns true if the user has admin role.
func IsAdmin(ctx context.Context) bool {
	return GetUserRoleFromContext(ctx) == model.RoleAdmin
}

// HasRole checks if the user has any of the specified roles.
func HasRole(ctx context.Context, roles ...model.UserRole) bool {
	userRole := GetUserRoleFromContext(ctx)
	for _, role := range roles {
		if userRole == role {
			return true
		}
	}
	return false
}

// RequireAdmin returns a convenience middleware that requires admin role.
func (m *RBACMiddleware) RequireAdmin() func(next http.Handler) http.Handler {
	return m.RequireRole(model.RoleAdmin)
}

// RequireUserOrAdmin returns a convenience middleware that requires user or admin role.
func (m *RBACMiddleware) RequireUserOrAdmin() func(next http.Handler) http.Handler {
	return m.RequireRole(model.RoleAdmin, model.RoleUser)
}
