// Package app provides the core application structure and lifecycle.
package app

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

	"ace/internal/api/router"
	"ace/internal/caching"
	"ace/internal/platform"
	"ace/internal/platform/cache"
	"ace/internal/platform/database"
	"ace/internal/platform/frontend"
	"ace/internal/platform/messaging"
	"ace/internal/platform/telemetry"

	"github.com/go-chi/chi/v5"
	"github.com/nats-io/nats.go"
)

// App represents the main ACE application.
type App struct {
	Config      *Config
	Paths       *platform.Paths
	DB          *sql.DB
	NATSConn    *nats.Conn
	natsCleanup func() error
	Cache       caching.CacheBackend
	Telemetry   *telemetry.Telemetry
	HTTPServer  *http.Server
	Router      *chi.Mux
}

// New creates a new App instance with the given configuration.
// It resolves paths, creates necessary directories, and initializes all subsystems
// in order: database → NATS → cache → telemetry.
func New(cfg *Config) (*App, error) {
	ctx := context.Background()

	// Resolve filesystem paths
	paths, err := platform.ResolvePaths(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve paths: %w", err)
	}

	// Create data directories
	if err := paths.EnsureDirs(); err != nil {
		return nil, fmt.Errorf("ensure directories: %w", err)
	}

	log.Printf("[ACE] initializing subsystems...")

	// 1. Open database (order: 1st)
	dbCfg := &database.Config{
		Mode:    cfg.DBMode,
		URL:     cfg.DBURL,
		DataDir: paths.DataDir,
	}

	db, err := database.Open(dbCfg)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	log.Printf("[ACE]   database: opened (%s)", paths.DBPath)

	// Run migrations
	if err := database.Migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	log.Printf("[ACE]   database: migrated")

	// 2. Initialize NATS messaging (order: 2nd)
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
	log.Printf("[ACE]   nats: connected")

	// 3. Initialize cache (order: 3rd)
	cacheCfg := &cache.Config{
		Mode:        cfg.CacheMode,
		URL:         cfg.CacheURL,
		MaxCost:     int64(cfg.CacheMaxCost),
		BufferItems: 64,
	}
	cacheBackend, err := cache.Init(cacheCfg)
	if err != nil {
		natsCleanup()
		db.Close()
		return nil, fmt.Errorf("init cache: %w", err)
	}
	log.Printf("[ACE]   cache: initialized")

	// 4. Initialize telemetry (order: 4th)
	telemetryCfg := &telemetry.Config{
		Mode:          cfg.TelemetryMode,
		OTLPEndpoint:  cfg.OTLPEndpoint,
		ServiceName:   "ace",
		Environment:   "development",
		LogDir:        paths.LogDir,
		PruneInterval: 6 * time.Hour,
	}
	appTelemetry, err := telemetry.Init(ctx, telemetryCfg, db)
	if err != nil {
		cacheBackend.Close()
		natsCleanup()
		db.Close()
		return nil, fmt.Errorf("init telemetry: %w", err)
	}
	log.Printf("[ACE]   telemetry: initialized")

	log.Printf("[ACE] all subsystems initialized successfully")

	return &App{
		Config:      cfg,
		Paths:       &paths,
		DB:          db,
		NATSConn:    nc,
		natsCleanup: natsCleanup,
		Cache:       cacheBackend,
		Telemetry:   appTelemetry,
	}, nil
}

// Serve starts the HTTP server and listens for connections.
func (a *App) Serve() error {
	if a.HTTPServer != nil {
		return fmt.Errorf("server already running")
	}

	// Determine host and port
	host := a.Config.Host
	if host == "" {
		host = "0.0.0.0"
	}
	port := a.Config.Port
	if port == 0 {
		port = 8080
	}

	addr := fmt.Sprintf("%s:%d", host, port)

	// Create router
	routeCfg := &router.Config{
		App: &router.AppConfig{
			Host:               host,
			Port:               port,
			CORSAllowedOrigins: []string{"*"}, // TODO: make this configurable
		},
		DB:       a.DB,
		NATSConn: a.NATSConn,
		Cache:    a.Cache,
	}

	r, err := router.New(routeCfg)
	if err != nil {
		return fmt.Errorf("create router: %w", err)
	}
	a.Router = r

	// Create HTTP server
	a.HTTPServer = &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("[ACE] HTTP server listening on %s", addr)
		if err := a.HTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[ACE] HTTP server error: %v", err)
		}
	}()

	return nil
}

// Shutdown gracefully shuts down the application.
// Order: HTTP drain → telemetry flush → NATS drain → cache close → database close.
func (a *App) Shutdown() error {
	var errs []error

	log.Printf("[ACE] shutting down...")

	// 1. Shutdown HTTP server (drain connections)
	if a.HTTPServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := a.HTTPServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown http: %w", err))
		} else {
			log.Printf("[ACE]   http: drained")
		}
	}

	// 2. Stop pruning goroutine
	if a.Telemetry != nil && a.Telemetry.PruneStop != nil {
		a.Telemetry.PruneStop()
		log.Printf("[ACE]   telemetry: pruning stopped")
	}

	// 3. Flush and shutdown telemetry
	if a.Telemetry != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.Telemetry.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown telemetry: %w", err))
		} else {
			log.Printf("[ACE]   telemetry: flushed")
		}
	}

	// 4. Close cache
	if a.Cache != nil {
		if err := a.Cache.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close cache: %w", err))
		} else {
			log.Printf("[ACE]   cache: closed")
		}
	}

	// 5. Cleanup NATS (drain client then shutdown server)
	if a.natsCleanup != nil {
		if err := a.natsCleanup(); err != nil {
			errs = append(errs, fmt.Errorf("cleanup messaging: %w", err))
		} else {
			log.Printf("[ACE]   nats: drained and stopped")
		}
	}

	// 6. Close database connection
	if a.DB != nil {
		if err := a.DB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close database: %w", err))
		} else {
			log.Printf("[ACE]   database: closed")
		}
	}

	log.Printf("[ACE] shutdown complete")

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}

// WaitForSignal blocks until a SIGINT or SIGTERM is received.
func WaitForSignal() os.Signal {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	return <-sigCh
}

// DevProxyHandler returns a handler that proxies to Vite dev server.
func DevProxyHandler() http.Handler {
	return frontend.DevProxy("http://localhost:5173")
}
