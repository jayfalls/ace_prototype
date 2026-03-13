// Package repository provides database access for the API service.
package repository

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ace/api/internal/config"
)

// DB represents a database connection wrapper with connection pool.
type DB struct {
	Pool   *pgxpool.Pool
	Config *config.DatabaseConfig
	mu     sync.RWMutex
}

// NewDB creates a new database connection with connection pool.
func NewDB(cfg *config.DatabaseConfig) (*DB, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// Configure connection pool
	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = time.Duration(cfg.MaxConnLifetime) * time.Second
	poolConfig.MaxConnIdleTime = time.Duration(cfg.MaxConnIdleTime) * time.Second

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Printf("Connected to PostgreSQL database: %s", cfg.DB)

	return &DB{
		Pool:   pool,
		Config: cfg,
	}, nil
}

// Close closes the database connection pool gracefully.
func (db *DB) Close() {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.Pool != nil {
		db.Pool.Close()
		log.Println("Database connection pool closed")
	}
}

// WaitForConnection waits for the database to become available.
// It retries with exponential backoff until success or timeout.
func WaitForConnection(cfg *config.DatabaseConfig, maxRetries int, retryInterval time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(maxRetries)*retryInterval)
	defer cancel()

	var lastErr error
	retryCount := 0

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("database connection timeout after %d retries: %w", retryCount, lastErr)
		default:
			connStr := cfg.DSN()
			conn, err := pgx.Connect(ctx, connStr)
			if err != nil {
				lastErr = err
				retryCount++
				log.Printf("Failed to connect to database (attempt %d/%d): %v", retryCount, maxRetries, err)
				time.Sleep(retryInterval)
				continue
			}
			conn.Close(ctx)
			log.Println("Database connection established")
			return nil
		}
	}
}

// RunWithGracefulShutdown runs the database operations with graceful shutdown handling.
// It waits for termination signals and properly closes connections.
func RunWithGracefulShutdown(db *DB, runFunc func(ctx context.Context) error) error {
	// Create context that cancels on SIGINT/SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel to listen for termination signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Channel to capture errors from runFunc
	errChan := make(chan error, 1)

	// Start the main function in a goroutine
	go func() {
		errChan <- runFunc(ctx)
	}()

	// Wait for either a termination signal or an error
	select {
	case err := <-errChan:
		if err != nil {
			log.Printf("Error running database operations: %v", err)
		}
		// Initiate graceful shutdown
		log.Println("Initiating graceful shutdown...")
		cancel()

		// Wait for connections to close with timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		db.Pool.Reset()
		<-shutdownCtx.Done()
		db.Close()

		return err

	case sig := <-sigChan:
		log.Printf("Received signal: %v", sig)
		log.Println("Initiating graceful shutdown...")

		// Cancel main context
		cancel()

		// Wait for connections to close with timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		// Wait for pool to drain
		db.Pool.Reset()
		<-shutdownCtx.Done()
		db.Close()

		return nil
	}
}

// HealthCheck performs a health check on the database connection.
func (db *DB) HealthCheck(ctx context.Context) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.Pool == nil {
		return fmt.Errorf("database pool is nil")
	}

	// Check if we can ping the database
	if err := db.Pool.Ping(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Check pool stats
	stats := db.Pool.Stat()
	if stats == nil {
		return fmt.Errorf("failed to get pool stats")
	}

	// Check for network issues
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer conn.Release()

	// Check underlying connection
	if conn.Conn() == nil {
		return fmt.Errorf("connection is nil")
	}

	// Check if connection is alive
	if !conn.Conn().IsAlive() {
		return fmt.Errorf("connection is not alive")
	}

	// Check for network reachability
	netConn, err := conn.Conn().Conn().NetConn()
	if err != nil {
		return fmt.Errorf("failed to get network connection: %w", err)
	}

	// Check if connection is not closed
	if _, ok := netConn.(*net.TCPConn); !ok {
		return fmt.Errorf("unexpected connection type")
	}

	return nil
}
