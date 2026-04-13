// Package repository provides database access for the API service.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// DB represents a database connection wrapper with connection pool.
type DB struct {
	Pool *sql.DB
}

// NewDB creates a new database connection with connection pool.
func NewDB(databaseURL string) (*DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	pool.SetMaxOpenConns(25)
	pool.SetMaxIdleConns(5)
	pool.SetConnMaxLifetime(5 * time.Minute)

	if err := pool.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Close closes the database connection pool.
func (db *DB) Close() error {
	if db.Pool != nil {
		return db.Pool.Close()
	}
	return nil
}
