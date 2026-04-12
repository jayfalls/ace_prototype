// Package main is the entry point for the API service.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/AxelTahmid/annot8"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"ace/api/internal/config"
	"ace/api/internal/handler"
	"ace/api/internal/middleware"
	db "ace/api/internal/repository/generated"
	"ace/api/internal/router"
	"ace/api/internal/service"
	_ "ace/api/migrations"
	"ace/shared/messaging"
	"ace/shared/telemetry"
	_ "ace/shared/telemetry/migrations"
)

// migrate runs all pending database migrations using Goose.
// It is called during server startup before any HTTP traffic is served.
func migrate(databaseURL string) error {
	goose.SetTableName("schema_migrations")
	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open database for migrations: %w", err)
	}
	defer sqlDB.Close()

	if err := goose.Up(sqlDB, "migrations"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	log.Println("Migrations completed successfully")
	return nil
}

// newRouter creates the main HTTP router with all services and handlers.
func newRouter(cfg *config.Config, pool *pgxpool.Pool, natsClient messaging.Client, tel *telemetry.Telemetry) *chi.Mux {
	// Create SQLC queries instance
	queries := db.New(pool)

	// Initialize TokenService
	tokenSvc, err := service.NewTokenService(&service.TokenConfig{
		Issuer:          "ace-auth",
		Audience:        "ace-api",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	})
	if err != nil {
		log.Fatalf("Failed to create token service: %v", err)
	}

	// Initialize AuthService (uses password functions from password_service.go internally)
	authSvc, err := service.NewAuthService(queries, tokenSvc)
	if err != nil {
		log.Fatalf("Failed to create auth service: %v", err)
	}

	// Initialize MagicLinkService (uses password functions from password_service.go internally)
	magicLinkSvc, err := service.NewMagicLinkService(queries, nil)
	if err != nil {
		log.Fatalf("Failed to create magic link service: %v", err)
	}

	// Create router config
	routerCfg := &router.Config{
		App:              cfg,
		Queries:          queries,
		AuthService:      authSvc,
		TokenService:     tokenSvc,
		MagicLinkService: magicLinkSvc,
	}

	// Create the router from internal/router/router.go
	apiRouter, err := router.New(routerCfg)
	if err != nil {
		log.Fatalf("Failed to create router: %v", err)
	}

	// Create health handler
	healthHandler := handler.NewHealthHandler(pool, natsClient, tel)

	// Create example handler
	exampleHandler := handler.NewExampleHandler()

	// Create main router with global middleware
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recovery)
	r.Use(middleware.CORS(cfg.CORSAllowedOrigins))

	// Add telemetry middleware
	r.Use(telemetry.TraceMiddleware())
	r.Use(telemetry.MetricsMiddleware("api"))
	r.Use(telemetry.LoggerMiddleware("api"))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"message": "ACE API Server"}`)
	})

	r.Get("/health/live", healthHandler.Live)
	r.Get("/health/ready", healthHandler.Ready)
	r.Handle("/metrics", telemetry.RegisterMetrics())

	// OpenAPI spec endpoint
	r.Get("/openapi.json", func(w http.ResponseWriter, req *http.Request) {
		gen := annot8.NewGenerator()
		spec := gen.GenerateSpec(apiRouter, annot8.Config{
			Title:       "ACE API",
			Description: "ACE Framework API",
			Version:     "0.1.0",
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = spec
		data, _ := json.Marshal(spec)
		w.Write(data)
	})

	// Swagger UI
	r.Get("/docs", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>ACE API Docs</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist/swagger-ui.css">
</head><body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist/swagger-ui-bundle.js"></script>
<script>SwaggerUIBundle({url: "/openapi.json", dom_id: "#swagger-ui"})</script>
</body></html>`)
	})

	// Mount API routes at /api
	r.Mount("/api", apiRouter)

	// Example routes
	r.Route("/examples", func(r chi.Router) {
		r.Post("/", exampleHandler.Create)
		r.Get("/{id}", exampleHandler.Get)
	})

	return r
}

func serve(host, port string, handler http.Handler) {
	addr := host + ":" + port
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		log.Printf("Starting ACE API server on %s...", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	dbPool, err := setupDatabase(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to setup database: %v", err)
	}
	defer dbPool.Close()

	// Run database migrations
	if err := migrate(cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Create NATS client
	natsClient, err := messaging.NewClient(messaging.Config{
		URLs:          cfg.NATSURL,
		Name:          "ace-api",
		Timeout:       10 * time.Second,
		MaxReconnect:  5,
		ReconnectWait: 2 * time.Second,
	})
	if err != nil {
		log.Printf("Warning: Failed to create NATS client: %v (running without messaging)", err)
		natsClient = nil
	}
	if natsClient != nil {
		defer natsClient.Close()
	}

	// Initialize telemetry
	tel, err := telemetry.Init(ctx, telemetry.Config{
		ServiceName:  "api",
		Environment:  cfg.Environment,
		OTLPEndpoint: cfg.OTLPEndpoint,
	})
	if err != nil {
		log.Printf("Warning: Failed to initialize telemetry: %v", err)
	}
	if tel != nil {
		defer tel.Shutdown(ctx)
	}

	router := newRouter(cfg, dbPool, natsClient, tel)
	serve(cfg.APIHost, cfg.APIPort, router)
}

// setupDatabase creates a database connection pool.
func setupDatabase(databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
