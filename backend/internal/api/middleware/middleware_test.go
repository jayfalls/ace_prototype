package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"ace/internal/api/model"
)

func TestGetUserIDFromContext(t *testing.T) {
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), UserIDKey, userID)

	result := GetUserIDFromContext(ctx)
	assert.Equal(t, userID, result)
}

func TestGetUserIDFromContext_Empty(t *testing.T) {
	ctx := context.Background()

	result := GetUserIDFromContext(ctx)
	assert.Nil(t, result)
}

func TestGetUserRoleFromContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), UserRoleKey, model.RoleAdmin)

	result := GetUserRoleFromContext(ctx)
	assert.Equal(t, model.RoleAdmin, result)
}

func TestGetUserRoleFromContext_Empty(t *testing.T) {
	ctx := context.Background()

	result := GetUserRoleFromContext(ctx)
	assert.Equal(t, model.UserRole(""), result)
}

func TestGetTokenClaimsFromContext(t *testing.T) {
	claims := &model.TokenClaims{
		Sub:  uuid.New(),
		Role: string(model.RoleUser),
	}
	ctx := context.WithValue(context.Background(), TokenClaimsKey, claims)

	result := GetTokenClaimsFromContext(ctx)
	assert.Equal(t, claims, result)
}

func TestGetTokenClaimsFromContext_Empty(t *testing.T) {
	ctx := context.Background()

	result := GetTokenClaimsFromContext(ctx)
	assert.Nil(t, result)
}

// TestRequireAuth_MissingHeader tests that RequireAuth returns 401 when Authorization header is missing.
func TestRequireAuth_MissingHeader(t *testing.T) {
	authMW := NewAuthMiddleware(nil)

	handler := authMW.RequireAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestRequireAuth_InvalidFormat tests that RequireAuth returns 401 for invalid Authorization format.
func TestRequireAuth_InvalidFormat(t *testing.T) {
	authMW := NewAuthMiddleware(nil)

	handler := authMW.RequireAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "InvalidFormat")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestRequireAuth_MissingToken tests that RequireAuth returns 401 when token is empty.
func TestRequireAuth_MissingToken(t *testing.T) {
	authMW := NewAuthMiddleware(nil)

	handler := authMW.RequireAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestRequireRole_NoRolesInContext tests that RequireRole returns 403 when no role in context.
func TestRequireRole_NoRolesInContext(t *testing.T) {
	rbacMW := NewRBACMiddleware()

	handler := rbacMW.RequireRole(model.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestRequireRole_Success tests that RequireRole allows access when role matches.
func TestRequireRole_Success(t *testing.T) {
	rbacMW := NewRBACMiddleware()

	handler := rbacMW.RequireRole(model.RoleAdmin, model.RoleUser)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserRoleKey, model.RoleUser))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRequireRole_InsufficientRole tests that RequireRole returns 403 when role doesn't match.
func TestRequireRole_InsufficientRole(t *testing.T) {
	rbacMW := NewRBACMiddleware()

	handler := rbacMW.RequireRole(model.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserRoleKey, model.RoleUser))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestHasRole tests the HasRole helper function.
func TestHasRole(t *testing.T) {
	ctx := context.WithValue(context.Background(), UserRoleKey, model.RoleUser)

	assert.True(t, HasRole(ctx, model.RoleUser))
	assert.True(t, HasRole(ctx, model.RoleAdmin, model.RoleUser))
	assert.False(t, HasRole(ctx, model.RoleAdmin))
	assert.False(t, HasRole(ctx, model.RoleViewer))
}

// TestIsAdmin tests the IsAdmin helper function.
func TestIsAdmin(t *testing.T) {
	ctxAdmin := context.WithValue(context.Background(), UserRoleKey, model.RoleAdmin)
	ctxUser := context.WithValue(context.Background(), UserRoleKey, model.RoleUser)
	ctxEmpty := context.Background()

	assert.True(t, IsAdmin(ctxAdmin))
	assert.False(t, IsAdmin(ctxUser))
	assert.False(t, IsAdmin(ctxEmpty))
}
