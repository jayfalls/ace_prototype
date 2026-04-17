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
	httpSwagger "github.com/swaggo/http-swagger"

	"ace/docs"
	"ace/internal/api/handler"
	mw "ace/internal/api/middleware"
	"ace/internal/api/realtime"
	db "ace/internal/api/repository/generated"
	"ace/internal/api/service"
	"ace/internal/caching"
)

// GetOpenAPISpec returns the OpenAPI spec as bytes using swaggo.
func GetOpenAPISpec() ([]byte, error) {
	return []byte(docs.SwaggerInfo.ReadDoc()), nil
}

// Error definitions for router validation.
var (
	ErrConfigRequired       = errors.New("config is required")
	ErrQueriesRequired      = errors.New("queries is required")
	ErrAuthServiceRequired  = errors.New("auth service is required")
	ErrTokenServiceRequired = errors.New("token service is required")
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
	App          *AppConfig
	Queries      *db.Queries
	AuthService  *service.AuthService
	TokenService *service.TokenService
	DB           *sql.DB
	NATSConn     *nats.Conn
	Cache        caching.CacheBackend
	Hub          *realtime.Hub
	SPAHandler   http.Handler // Serves the SPA (embedded assets or Vite proxy)
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
	if cfg.Queries == nil {
		return nil, ErrQueriesRequired
	}
	if cfg.AuthService == nil {
		return nil, ErrAuthServiceRequired
	}
	if cfg.TokenService == nil {
		return nil, ErrTokenServiceRequired
	}

	// Create handlers - these must be available
	authHandler, err := handler.NewAuthHandler(
		cfg.Queries,
		cfg.AuthService,
		cfg.TokenService,
	)
	if err != nil {
		return nil, err
	}

	sessionHandler, err := handler.NewSessionHandler(cfg.Queries)
	if err != nil {
		return nil, err
	}

	adminHandler, err := handler.NewAdminHandler(cfg.Queries)
	if err != nil {
		return nil, err
	}

	// Create telemetry handler
	telemetryHandler, err := handler.NewTelemetryHandler(
		cfg.Queries,
		cfg.DB,
		cfg.NATSConn,
		cfg.Cache,
	)
	if err != nil {
		return nil, err
	}

	// Create auth middleware
	authMw := mw.NewAuthMiddleware(cfg.TokenService)

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

	// Swagger UI
	r.Get("/swagger/*", httpSwagger.WrapHandler)

	// Health check routes (no auth required, no /api prefix)
	r.Group(func(r chi.Router) {
		r.Get("/health/live", healthLiveHandler())
		r.Get("/health/ready", healthReadyHandler(cfg))
	})

	// API routes group (all under /api prefix)
	r.Route("/api", func(r chi.Router) {
		// Public user listing for login screen (no auth required)
		r.Get("/users", authHandler.ListUsers)

		// Auth routes (no auth required)
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authHandler.RegisterWithPIN)
			r.Post("/login", authHandler.LoginWithPIN)
			r.Post("/logout", authHandler.Logout)
			r.Post("/refresh", authHandler.Refresh)
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

		// Telemetry routes (auth required)
		r.Route("/telemetry", func(r chi.Router) {
			r.Use(authMw.RequireAuth())
			r.Get("/spans", telemetryHandler.Spans)
			r.Get("/metrics", telemetryHandler.Metrics)
			r.Get("/usage", telemetryHandler.Usage)
			r.Get("/health", telemetryHandler.Health)
		})
	})

	// Realtime routes
	if cfg.Hub != nil {
		// WebSocket — no auth middleware, auth via first message
		r.Get("/api/ws", realtime.HandleWebSocket(cfg.Hub, cfg.TokenService))

		// Polling — protected by auth middleware
		r.Group(func(r chi.Router) {
			r.Use(authMw.RequireAuth())
			r.Get("/api/realtime/updates", realtime.HandlePolling(cfg.Hub))
		})
	}

	// SPA catch-all route - must be last to not intercept API routes
	if cfg.SPAHandler != nil {
		r.NotFound(cfg.SPAHandler.ServeHTTP)
	}

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
		spec, err := GetOpenAPISpec()
		if err != nil {
			http.Error(w, "failed to read OpenAPI spec", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(spec)
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
