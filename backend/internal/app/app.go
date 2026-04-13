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
	"ace/internal/platform/messaging"

	"github.com/nats-io/nats.go"
)

// App represents the main ACE application.
type App struct {
	Config      *Config
	Paths       *platform.Paths
	DB          *sql.DB
	NATSConn    *nats.Conn
	natsCleanup func() error
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

	// Initialize NATS messaging
	messagingCfg := &messaging.Config{
		Mode: cfg.NATSMode,
		URL:  cfg.NATSURL,
	}
	messagingPaths := &messaging.MessagingPaths{
		NATSPath: paths.NATSPath,
	}

	nc, natsCleanup, err := messaging.Init(messagingCfg, messagingPaths)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("init messaging: %w", err)
	}

	return &App{
		Config:      cfg,
		Paths:       &paths,
		DB:          db,
		NATSConn:    nc,
		natsCleanup: natsCleanup,
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
	// Cleanup NATS (drain client then shutdown server)
	if a.natsCleanup != nil {
		if err := a.natsCleanup(); err != nil {
			return fmt.Errorf("cleanup messaging: %w", err)
		}
	}

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
