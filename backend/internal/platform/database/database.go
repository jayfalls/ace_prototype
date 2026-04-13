// Package database provides database connectivity and migration support.
package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
)

// Config holds database configuration.
type Config struct {
	// Mode is the database mode: "embedded" (SQLite) or "external" (PostgreSQL).
	Mode string
	// URL is the database connection URL (required for external mode).
	URL string
	// DataDir is the data directory path (used for embedded SQLite).
	DataDir string
}

// Open opens a database connection based on the configuration.
// For embedded mode, it opens a SQLite database at {DataDir}/ace.db.
// For external mode, it opens a PostgreSQL connection using the URL.
func Open(cfg *Config) (*sql.DB, error) {
	var db *sql.DB
	var err error

	switch cfg.Mode {
	case "embedded":
		db, err = openEmbedded(cfg.DataDir)
	case "external":
		db, err = openExternal(cfg.URL)
	default:
		return nil, fmt.Errorf("database: invalid mode: %q (must be \"embedded\" or \"external\")", cfg.Mode)
	}

	if err != nil {
		return nil, fmt.Errorf("database: open failed: %w", err)
	}

	return db, nil
}

// Migrate runs all pending Goose migrations against the database.
func Migrate(db *sql.DB) error {
	migrationsDir := detectMigrationsDir()

	// Set goose options
	goose.SetDialect("sqlite3")

	if err := goose.Up(db, migrationsDir); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	return nil
}

// detectMigrationsDir tries to find the migrations directory.
// It checks multiple locations in order of priority.
func detectMigrationsDir() string {
	// List of possible migration directory locations
	possibleDirs := []string{
		"migrations", // Current working directory
		"backend/migrations",
		"../migrations",
	}

	// Try to find the executable for additional detection
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		possibleDirs = append(possibleDirs,
			filepath.Join(execDir, "migrations"),
			filepath.Join(execDir, "backend", "migrations"),
			filepath.Join(execDir, "..", "migrations"),
			filepath.Join(execDir, "..", "backend", "migrations"),
		)
	}

	// Try each possible directory
	for _, dir := range possibleDirs {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			absPath, _ := filepath.Abs(dir)
			return absPath
		}
	}

	// Default to current directory
	return "migrations"
}
