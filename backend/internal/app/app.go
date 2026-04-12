// Package app provides the core application structure and lifecycle.
package app

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"ace/internal/platform"
)

// App represents the main ACE application.
type App struct {
	Config *Config
	Paths  *platform.Paths
}

// New creates a new App instance with the given configuration.
// It resolves paths, creates necessary directories, but does not start any servers.
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

	return &App{
		Config: cfg,
		Paths:  &paths,
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
// Currently a stub that just logs "shutting down" - full implementation comes in later slices.
func (a *App) Shutdown() error {
	// TODO: Implement graceful shutdown in later slices
	fmt.Println("shutting down")
	return nil
}

// WaitForSignal blocks until a SIGINT or SIGTERM is received.
func WaitForSignal() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
