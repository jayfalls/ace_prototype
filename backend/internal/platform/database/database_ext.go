//go:build external

// Package database provides database connectivity for external (PostgreSQL) mode.
package database

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// openExternal opens a PostgreSQL database connection using the given URL.
func openExternal(url string) (*sql.DB, error) {
	if url == "" {
		return nil, fmt.Errorf("db-url is required for external mode")
	}

	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}
