// Package router provides HTTP routing configuration for the API service.
package router

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"ace/internal/api/handler"
	mw "ace/internal/api/middleware"
	db "ace/internal/api/repository/generated"
	"ace/internal/api/service"
	"ace/internal/caching"
)

// OpenAPI spec document (cached at package level for performance)
var openAPISpec = buildOpenAPISpec()

// Error definitions for router validation.
var (
	ErrConfigRequired           = errors.New("config is required")
	ErrQueriesRequired          = errors.New("queries is required")
	ErrAuthServiceRequired      = errors.New("auth service is required")
	ErrTokenServiceRequired     = errors.New("token service is required")
	ErrMagicLinkServiceRequired = errors.New("magic link service is required")
)

// SubsystemHealth holds health status for a subsystem.
type SubsystemHealth struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// HealthStatus holds the overall health status and individual checks.
type HealthStatus struct {
	Status string                     `json:"status"`
	Checks map[string]SubsystemHealth `json:"checks"`
}

// Config holds all dependencies needed to create the router.
type Config struct {
	App              *AppConfig
	Queries          *db.Queries
	AuthService      *service.AuthService
	TokenService     *service.TokenService
	MagicLinkService *service.MagicLinkService
	DB               *sql.DB
	NATSConn         *nats.Conn
	Cache            caching.CacheBackend
}

// AppConfig holds the basic app configuration needed for the router.
type AppConfig struct {
	Host               string
	Port               int
	CORSAllowedOrigins []string
}

// New creates a new chi router with all routes and middleware configured.
// Middleware order: Recovery → Logger → CORS → RateLimit → Auth → Handler
func New(cfg *Config) (*chi.Mux, error) {
	// Validate required dependencies
	if cfg == nil {
		return nil, ErrConfigRequired
	}
	if cfg.App == nil {
		return nil, ErrConfigRequired
	}

	// Create handlers only if dependencies are provided
	var authHandler *handler.AuthHandler
	var sessionHandler *handler.SessionHandler
	var adminHandler *handler.AdminHandler

	if cfg.Queries != nil && cfg.AuthService != nil && cfg.TokenService != nil && cfg.MagicLinkService != nil {
		var err error
		authHandler, err = handler.NewAuthHandler(
			cfg.Queries,
			cfg.AuthService,
			cfg.TokenService,
			cfg.MagicLinkService,
		)
		if err != nil {
			return nil, err
		}

		sessionHandler, err = handler.NewSessionHandler(cfg.Queries)
		if err != nil {
			return nil, err
		}

		adminHandler, err = handler.NewAdminHandler(cfg.Queries)
		if err != nil {
			return nil, err
		}
	}

	// Create auth middleware if token service is available
	var authMw *mw.AuthMiddleware
	if cfg.TokenService != nil {
		authMw = mw.NewAuthMiddleware(cfg.TokenService)
	}

	// Create RBAC middleware
	rbacMw := mw.NewRBACMiddleware()

	// Create router
	r := chi.NewRouter()

	// Global middleware (applied in order: Recovery → Logger → CORS → RateLimit)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Set CORS if origins are provided
	if len(cfg.App.CORSAllowedOrigins) > 0 {
		r.Use(mw.CORS(cfg.App.CORSAllowedOrigins))
	}

	// OpenAPI spec endpoint
	r.Get("/openapi.json", openAPIHandler())

	// Health check routes (no auth required)
	r.Group(func(r chi.Router) {
		r.Get("/health/live", healthLiveHandler())
		r.Get("/health/ready", healthReadyHandler(cfg))
	})

	// Mount API routes only if handlers are available
	if authHandler != nil && sessionHandler != nil && adminHandler != nil && authMw != nil {
		// Auth routes (no auth required)
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authHandler.Register)
			r.Post("/login", authHandler.Login)
			r.Post("/logout", authHandler.Logout)
			r.Post("/refresh", authHandler.Refresh)
			r.Post("/password/reset/request", authHandler.ResetPasswordRequest)
			r.Post("/password/reset/confirm", authHandler.ResetPasswordConfirm)
			r.Post("/magic-link/request", authHandler.MagicLinkRequest)
			r.Post("/magic-link/verify", authHandler.MagicLinkVerify)
		})

		// Protected routes (auth required)
		r.Group(func(r chi.Router) {
			r.Use(authMw.RequireAuth())

			r.Get("/auth/me", sessionHandler.Me)
			r.Get("/auth/me/sessions", sessionHandler.ListSessions)
			r.Delete("/auth/me/sessions/{id}", sessionHandler.RevokeSession)
		})

		// Admin routes (auth + admin role required)
		r.Group(func(r chi.Router) {
			r.Use(authMw.RequireAuth())
			r.Use(rbacMw.RequireAdmin())

			r.Get("/admin/users", adminHandler.ListUsers)
			r.Get("/admin/users/{id}", adminHandler.GetUser)
			r.Put("/admin/users/{id}/role", adminHandler.UpdateUserRole)
			r.Post("/admin/users/{id}/suspend", adminHandler.SuspendUser)
			r.Post("/admin/users/{id}/restore", adminHandler.RestoreUser)
		})
	}

	// Telemetry routes (stubs for now)
	r.Route("/telemetry", func(r chi.Router) {
		// TODO: Wire up telemetry inspector endpoints in Slice 10
		r.Get("/health", func(w http.ResponseWriter, t *http.Request) {
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		})
	})

	return r, nil
}

// healthLiveHandler returns a simple liveness check handler.
func healthLiveHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// openAPIHandler returns the OpenAPI specification as JSON.
func openAPIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(openAPISpec)
	}
}

// healthReadyHandler returns a readiness check handler with configurable pool.
func healthReadyHandler(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		status := HealthStatus{
			Status: "ok",
			Checks: make(map[string]SubsystemHealth),
		}

		// Check database
		if cfg.DB != nil {
			if err := cfg.DB.PingContext(r.Context()); err != nil {
				status.Checks["database"] = SubsystemHealth{Status: "fail", Reason: err.Error()}
				status.Status = "degraded"
			} else {
				status.Checks["database"] = SubsystemHealth{Status: "ok"}
			}
		} else {
			status.Checks["database"] = SubsystemHealth{Status: "not_initialized"}
		}

		// Check NATS
		if cfg.NATSConn != nil && cfg.NATSConn.IsConnected() {
			status.Checks["nats"] = SubsystemHealth{Status: "ok"}
		} else {
			status.Checks["nats"] = SubsystemHealth{Status: "not_connected"}
			status.Status = "degraded"
		}

		// Check cache
		if cfg.Cache != nil {
			status.Checks["cache"] = SubsystemHealth{Status: "ok"}
		} else {
			status.Checks["cache"] = SubsystemHealth{Status: "not_initialized"}
		}

		httpStatus := http.StatusOK
		if status.Status == "degraded" {
			httpStatus = http.StatusServiceUnavailable
		}

		w.WriteHeader(httpStatus)
		json.NewEncoder(w).Encode(status)
	}
}

// WithUserID adds user ID to context.
func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, mw.UserIDKey, userID)
}

// WithUserRole adds user role to context.
func WithUserRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, mw.UserRoleKey, role)
}

// GetUserRoleFromContext retrieves user role from context.
func GetUserRoleFromContext(ctx context.Context) string {
	if role, ok := ctx.Value(mw.UserRoleKey).(string); ok {
		return role
	}
	return ""
}

// buildOpenAPISpec constructs the OpenAPI 3.0 specification for the API.
func buildOpenAPISpec() map[string]interface{} {
	return map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":       "ACE API",
			"description": "Agent Configuration Engine API",
			"version":     "1.0.0",
		},
		"servers": []map[string]interface{}{
			{"url": "http://localhost:8080", "description": "Local development server"},
		},
		"paths": map[string]interface{}{
			"/health/live": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Liveness check",
					"operationId": "getHealthLive",
					"tags":        []string{"health"},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Service is alive",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"status": map[string]interface{}{"type": "string"},
										},
									},
								},
							},
						},
					},
				},
			},
			"/health/ready": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Readiness check",
					"operationId": "getHealthReady",
					"tags":        []string{"health"},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Service is ready",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"status": map[string]interface{}{"type": "string"},
											"checks": map[string]interface{}{
												"type": "object",
												"additionalProperties": map[string]interface{}{
													"type": "object",
													"properties": map[string]interface{}{
														"status": map[string]interface{}{"type": "string"},
														"reason": map[string]interface{}{"type": "string"},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"/auth/register": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Register a new user",
					"operationId": "register",
					"tags":        []string{"auth"},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"email", "password"},
									"properties": map[string]interface{}{
										"email":    map[string]interface{}{"type": "string", "format": "email"},
										"password": map[string]interface{}{"type": "string", "minLength": 8},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"201": map[string]interface{}{
							"description": "User registered successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{"$ref": "#/components/schemas/TokenResponse"},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "Invalid request",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{"$ref": "#/components/schemas/APIError"},
								},
							},
						},
						"409": map[string]interface{}{
							"description": "User already exists",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{"$ref": "#/components/schemas/APIError"},
								},
							},
						},
					},
				},
			},
			"/auth/login": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Login with email and password",
					"operationId": "login",
					"tags":        []string{"auth"},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"email", "password"},
									"properties": map[string]interface{}{
										"email":    map[string]interface{}{"type": "string", "format": "email"},
										"password": map[string]interface{}{"type": "string"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Login successful",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{"$ref": "#/components/schemas/TokenResponse"},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Invalid credentials",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{"$ref": "#/components/schemas/APIError"},
								},
							},
						},
					},
				},
			},
			"/auth/logout": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Logout current session",
					"operationId": "logout",
					"tags":        []string{"auth"},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"session_id"},
									"properties": map[string]interface{}{
										"session_id": map[string]interface{}{"type": "string", "format": "uuid"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Logged out successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"message": map[string]interface{}{"type": "string"},
										},
									},
								},
							},
						},
					},
				},
			},
			"/auth/refresh": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Refresh access token",
					"operationId": "refresh",
					"tags":        []string{"auth"},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"refresh_token"},
									"properties": map[string]interface{}{
										"refresh_token": map[string]interface{}{"type": "string"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Token refreshed",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{"$ref": "#/components/schemas/TokenResponse"},
								},
							},
						},
					},
				},
			},
			"/auth/password/reset/request": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Request password reset",
					"operationId": "passwordResetRequest",
					"tags":        []string{"auth"},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"email"},
									"properties": map[string]interface{}{
										"email": map[string]interface{}{"type": "string", "format": "email"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Reset email sent if account exists",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"message": map[string]interface{}{"type": "string"},
										},
									},
								},
							},
						},
					},
				},
			},
			"/auth/password/reset/confirm": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Confirm password reset",
					"operationId": "passwordResetConfirm",
					"tags":        []string{"auth"},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"token", "new_password"},
									"properties": map[string]interface{}{
										"token":        map[string]interface{}{"type": "string"},
										"new_password": map[string]interface{}{"type": "string", "minLength": 8},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Password reset successful",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{"$ref": "#/components/schemas/TokenResponse"},
								},
							},
						},
					},
				},
			},
			"/auth/magic-link/request": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Request magic link",
					"operationId": "magicLinkRequest",
					"tags":        []string{"auth"},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"email"},
									"properties": map[string]interface{}{
										"email": map[string]interface{}{"type": "string", "format": "email"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Magic link sent if account exists",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"message": map[string]interface{}{"type": "string"},
										},
									},
								},
							},
						},
					},
				},
			},
			"/auth/magic-link/verify": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Verify magic link",
					"operationId": "magicLinkVerify",
					"tags":        []string{"auth"},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"token"},
									"properties": map[string]interface{}{
										"token": map[string]interface{}{"type": "string"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Magic link verified",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{"$ref": "#/components/schemas/TokenResponse"},
								},
							},
						},
					},
				},
			},
			"/auth/me": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Get current user profile",
					"operationId": "getMe",
					"tags":        []string{"session"},
					"security":    []map[string]interface{}{{"bearerAuth": []interface{}{}}},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Current user profile",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{"$ref": "#/components/schemas/UserResponse"},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{"$ref": "#/components/schemas/APIError"},
								},
							},
						},
					},
				},
			},
			"/auth/me/sessions": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "List user's active sessions",
					"operationId": "listSessions",
					"tags":        []string{"session"},
					"security":    []map[string]interface{}{{"bearerAuth": []interface{}{}}},
					"parameters": []map[string]interface{}{
						{"name": "page", "in": "query", "schema": map[string]interface{}{"type": "integer", "default": 1}},
						{"name": "limit", "in": "query", "schema": map[string]interface{}{"type": "integer", "default": 20}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "List of sessions",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{"$ref": "#/components/schemas/SessionsListResponse"},
								},
							},
						},
					},
				},
			},
			"/auth/me/sessions/{id}": map[string]interface{}{
				"delete": map[string]interface{}{
					"summary":     "Revoke a specific session",
					"operationId": "revokeSession",
					"tags":        []string{"session"},
					"security":    []map[string]interface{}{{"bearerAuth": []interface{}{}}},
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string", "format": "uuid"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Session revoked",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"message": map[string]interface{}{"type": "string"},
										},
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "Session not found",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{"$ref": "#/components/schemas/APIError"},
								},
							},
						},
					},
				},
			},
			"/admin/users": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "List all users (paginated)",
					"operationId": "listUsers",
					"tags":        []string{"admin"},
					"security":    []map[string]interface{}{{"bearerAuth": []interface{}{}}},
					"parameters": []map[string]interface{}{
						{"name": "page", "in": "query", "schema": map[string]interface{}{"type": "integer", "default": 1}},
						{"name": "limit", "in": "query", "schema": map[string]interface{}{"type": "integer", "default": 20}},
						{"name": "status", "in": "query", "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "List of users",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{"$ref": "#/components/schemas/UsersListResponse"},
								},
							},
						},
						"403": map[string]interface{}{
							"description": "Admin access required",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{"$ref": "#/components/schemas/APIError"},
								},
							},
						},
					},
				},
			},
			"/admin/users/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Get user details",
					"operationId": "getUser",
					"tags":        []string{"admin"},
					"security":    []map[string]interface{}{{"bearerAuth": []interface{}{}}},
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string", "format": "uuid"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "User details",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{"$ref": "#/components/schemas/AdminUserResponse"},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "User not found",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{"$ref": "#/components/schemas/APIError"},
								},
							},
						},
					},
				},
			},
			"/admin/users/{id}/role": map[string]interface{}{
				"put": map[string]interface{}{
					"summary":     "Update user role",
					"operationId": "updateUserRole",
					"tags":        []string{"admin"},
					"security":    []map[string]interface{}{{"bearerAuth": []interface{}{}}},
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string", "format": "uuid"}},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"role"},
									"properties": map[string]interface{}{
										"role": map[string]interface{}{"type": "string", "enum": []string{"user", "admin", "viewer"}},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Role updated",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{"$ref": "#/components/schemas/AdminUserResponse"},
								},
							},
						},
					},
				},
			},
			"/admin/users/{id}/suspend": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Suspend a user",
					"operationId": "suspendUser",
					"tags":        []string{"admin"},
					"security":    []map[string]interface{}{{"bearerAuth": []interface{}{}}},
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string", "format": "uuid"}},
					},
					"requestBody": map[string]interface{}{
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"reason": map[string]interface{}{"type": "string"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "User suspended",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{"$ref": "#/components/schemas/AdminUserResponse"},
								},
							},
						},
					},
				},
			},
			"/admin/users/{id}/restore": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Restore a suspended user",
					"operationId": "restoreUser",
					"tags":        []string{"admin"},
					"security":    []map[string]interface{}{{"bearerAuth": []interface{}{}}},
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string", "format": "uuid"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "User restored",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{"$ref": "#/components/schemas/AdminUserResponse"},
								},
							},
						},
					},
				},
			},
			"/telemetry/health": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Telemetry health check",
					"operationId": "getTelemetryHealth",
					"tags":        []string{"telemetry"},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Telemetry health status",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"status": map[string]interface{}{"type": "string"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"bearerAuth": map[string]interface{}{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
			},
			"schemas": map[string]interface{}{
				"TokenResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"user":          map[string]interface{}{"$ref": "#/components/schemas/User"},
						"access_token":  map[string]interface{}{"type": "string"},
						"refresh_token": map[string]interface{}{"type": "string"},
						"expires_in":    map[string]interface{}{"type": "integer"},
						"token_type":    map[string]interface{}{"type": "string"},
					},
				},
				"User": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id":         map[string]interface{}{"type": "string", "format": "uuid"},
						"email":      map[string]interface{}{"type": "string", "format": "email"},
						"role":       map[string]interface{}{"type": "string"},
						"status":     map[string]interface{}{"type": "string"},
						"created_at": map[string]interface{}{"type": "string"},
						"updated_at": map[string]interface{}{"type": "string"},
					},
				},
				"UserResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id":         map[string]interface{}{"type": "string", "format": "uuid"},
						"email":      map[string]interface{}{"type": "string", "format": "email"},
						"role":       map[string]interface{}{"type": "string"},
						"status":     map[string]interface{}{"type": "string"},
						"created_at": map[string]interface{}{"type": "string"},
						"updated_at": map[string]interface{}{"type": "string"},
					},
				},
				"AdminUserResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id":               map[string]interface{}{"type": "string", "format": "uuid"},
						"email":            map[string]interface{}{"type": "string", "format": "email"},
						"role":             map[string]interface{}{"type": "string"},
						"status":           map[string]interface{}{"type": "string"},
						"suspended_at":     map[string]interface{}{"type": "string"},
						"suspended_reason": map[string]interface{}{"type": "string"},
						"created_at":       map[string]interface{}{"type": "string"},
						"updated_at":       map[string]interface{}{"type": "string"},
					},
				},
				"SessionsListResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"sessions": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"$ref": "#/components/schemas/Session",
							},
						},
						"total": map[string]interface{}{"type": "integer"},
						"page":  map[string]interface{}{"type": "integer"},
						"limit": map[string]interface{}{"type": "integer"},
					},
				},
				"Session": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id":           map[string]interface{}{"type": "string", "format": "uuid"},
						"user_id":      map[string]interface{}{"type": "string", "format": "uuid"},
						"user_agent":   map[string]interface{}{"type": "string"},
						"ip_address":   map[string]interface{}{"type": "string"},
						"last_used_at": map[string]interface{}{"type": "string"},
						"expires_at":   map[string]interface{}{"type": "string"},
						"created_at":   map[string]interface{}{"type": "string"},
					},
				},
				"UsersListResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"users": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"$ref": "#/components/schemas/UserListItem",
							},
						},
						"total": map[string]interface{}{"type": "integer"},
						"page":  map[string]interface{}{"type": "integer"},
						"limit": map[string]interface{}{"type": "integer"},
					},
				},
				"UserListItem": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id":         map[string]interface{}{"type": "string", "format": "uuid"},
						"email":      map[string]interface{}{"type": "string", "format": "email"},
						"role":       map[string]interface{}{"type": "string"},
						"status":     map[string]interface{}{"type": "string"},
						"created_at": map[string]interface{}{"type": "string"},
						"updated_at": map[string]interface{}{"type": "string"},
					},
				},
				"APIError": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"success": map[string]interface{}{"type": "boolean"},
						"error": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"code":    map[string]interface{}{"type": "string"},
								"message": map[string]interface{}{"type": "string"},
								"details": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"field":   map[string]interface{}{"type": "string"},
											"message": map[string]interface{}{"type": "string"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
