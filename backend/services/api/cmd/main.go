// Package main is the entry point for the API service.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"ace/api/internal/config"
	"ace/api/internal/handler"
	"ace/api/internal/middleware"
	"ace/api/internal/repository"
	"ace/shared/messaging"
	"ace/shared/telemetry"
	_ "ace/shared/telemetry/migrations"

	// _ "ace/api/migrations"
	"ace/shared"
)

// NOTE: Commented out code to be enabled once needed

func migrate(databaseURL string) {
	goose.SetTableName("schema_migrations")
	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatalf("Failed to open database for migrations: %v", err)
	}
	defer sqlDB.Close()

	if err := goose.Up(sqlDB, "migrations"); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Migrations completed successfully")
}

func newRouter(cfg *config.Config, pool *pgxpool.Pool, nats messaging.Client, tel *telemetry.Telemetry) *chi.Mux {
	// Create SQLC queries instance
	// queries := queries.New(pool)

	// Service layers

	// Handlers
	healthHandler := handler.NewHealthHandler(pool, nats, tel)
	exampleHandler := handler.NewExampleHandler()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recovery)
	r.Use(middleware.CORS(cfg.CORSAllowedOrigins))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"message": "ACE API Server"}`)
	})

	r.Get("/health/live", healthHandler.Live)
	r.Get("/health/ready", healthHandler.Ready)
	r.Get("/health/exporters", healthHandler.Exporters)

	// Example routes demonstrating validation
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
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
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

	db, err := repository.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to create database connection: %v", err)
	}
	defer db.Close()

	// Run database migrations
	migrate(cfg.DatabaseURL)

	// Create NATS client
	natsClient, err := messaging.NewClient(messaging.Config{
		URLs:          cfg.NATSURL,
		Name:          "ace-api",
		Timeout:       10 * time.Second,
		MaxReconnect:  5,
		ReconnectWait: 2 * time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to create NATS client: %v", err)
	}
	defer natsClient.Close()

	// Initialize telemetry
	tel, err := telemetry.Init(ctx, telemetry.Config{
		ServiceName:  "api",
		Environment:  cfg.Environment,
		OTLPEndpoint: cfg.OTLPEndpoint,
	})
	if err != nil {
		log.Printf("Warning: Failed to initialize telemetry: %v", err)
		// Continue without telemetry - it's optional
		tel = nil
	}
	defer func() {
		if tel != nil {
			tel.Shutdown(ctx)
		}
	}()

	shared.Hello()

	router := newRouter(cfg, db.Pool, natsClient, tel)
	serve(cfg.APIHost, cfg.APIPort, router)
}
