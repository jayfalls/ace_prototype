//go:build !external

// Package database provides database connectivity for embedded (SQLite) mode.
package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// openEmbedded opens a SQLite database with WAL mode and foreign keys enabled.
func openEmbedded(dataDir string) (*sql.DB, error) {
	// Ensure data directory is set
	if dataDir == "" {
		// Use XDG default
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		xdgDataHome := os.Getenv("XDG_DATA_HOME")
		if xdgDataHome == "" {
			xdgDataHome = filepath.Join(home, ".local", "share")
		}
		dataDir = filepath.Join(xdgDataHome, "ace")
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	// Build connection string with pragmas
	dbPath := filepath.Join(dataDir, "ace.db")
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)",
		dbPath,
	)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	return db, nil
}

// openExternal is not available in embedded mode.
func openExternal(url string) (*sql.DB, error) {
	return nil, fmt.Errorf("external mode not compiled: use -tags=external")
}
