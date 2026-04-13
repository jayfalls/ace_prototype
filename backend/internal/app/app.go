// Package app provides the core application structure and lifecycle.
package app

import (
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"ace/internal/platform"
	"ace/internal/platform/database"
)

// App represents the main ACE application.
type App struct {
	Config *Config
	Paths  *platform.Paths
	DB     *sql.DB
}

// New creates a new App instance with the given configuration.
// It resolves paths, creates necessary directories, and opens the database.
func New(cfg *Config) (*App, error) {
	// Resolve filesystem paths
	paths, err := platform.ResolvePaths(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve paths: %w", err)
	}

	// Create data directories
	if err := paths.EnsureDirs(); err != nil {
		return nil, fmt.Errorf("ensure directories: %w", err)
	}

	// Open database
	dbCfg := &database.Config{
		Mode:    cfg.DBMode,
		URL:     cfg.DBURL,
		DataDir: paths.DataDir,
	}

	db, err := database.Open(dbCfg)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Run migrations
	if err := database.Migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return &App{
		Config: cfg,
		Paths:  &paths,
		DB:     db,
	}, nil
}

// Serve starts the ACE server.
// Currently a stub that just logs "serving" - full implementation comes in later slices.
func (a *App) Serve() error {
	// TODO: Implement HTTP server in later slices
	fmt.Println("serving")
	return nil
}

// Shutdown gracefully shuts down the application.
func (a *App) Shutdown() error {
	// Close database connection
	if a.DB != nil {
		if err := a.DB.Close(); err != nil {
			return fmt.Errorf("close database: %w", err)
		}
	}
	fmt.Println("shutting down")
	return nil
}

// WaitForSignal blocks until a SIGINT or SIGTERM is received.
func WaitForSignal() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
