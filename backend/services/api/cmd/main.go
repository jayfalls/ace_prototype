// Package main is the entry point for the API service.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ace/api/internal/config"
	"ace/api/internal/handler"
	"ace/api/internal/middleware"
	"ace/api/internal/repository"
	"ace/api/internal/repository/generated"
	"ace/api/internal/service"
	"ace/shared"
	"github.com/go-chi/chi/v5"
	"github.com/pressly/goose/v3"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Connecting to database: %s@%s:%d/%s", cfg.Database.User, cfg.Database.Host, cfg.Database.Port, cfg.Database.DB)

	// Wait for database to be available (with retry)
	err = repository.WaitForConnection(&cfg.Database, 10, 2*time.Second)
	if err != nil {
		log.Printf("Warning: Could not connect to database: %v", err)
		log.Printf("Starting server without database connection...")
	} else {
		log.Println("Database connection established")
	}

	// Create database connection pool
	var db *repository.DB
	db, err = repository.NewDB(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to create database connection: %v", err)
	}
	defer db.Close()

	// Run migrations
	log.Println("Running database migrations...")
	goose.SetTableName("schema_migrations")
	if err := goose.Up(db.Pool, "migrations"); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Migrations completed successfully")

	// Create SQLC queries instance
	queries := generated.New(db.Pool)

	// Initialize service layer
	healthService := service.NewHealthService(queries)

	// Initialize handlers
	healthHandler := handler.NewHealthHandler(healthService)

	r := chi.NewRouter()

	// Middleware stack
	r.Use(middleware.Logger)
	r.Use(middleware.Recovery)
	r.Use(middleware.CORS(cfg.API.CORSAllowedOrigins))

	// Health check endpoint using SQLC-generated queries
	r.Get("/health", healthHandler.Health)
	r.Get("/health/history", healthHandler.ListHealthChecks)

	// Root endpoint
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"message": "ACE API Server"}`)
	})

	shared.Hello()

	// Create server
	addr := ":" + cfg.API.Port
	server := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting ACE API server on port %s...", cfg.API.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Wait for active connections
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
