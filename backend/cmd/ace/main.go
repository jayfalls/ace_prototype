// Package main is the entry point for the ACE binary.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"

	"ace/internal/app"
	"ace/internal/platform"
	"ace/internal/platform/database"
	"ace/internal/platform/telemetry"

	"go.uber.org/zap"
	_ "modernc.org/sqlite"
)

var (
	version   = "0.1.0"
	commit    = "abc1234"
	buildDate = "2026-04-12T10:00:00Z"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

}

func run(args []string) error {
	// Parse all flags first
	if err := flag.CommandLine.Parse(args[1:]); err != nil {
		return err
	}

	// Find subcommand in remaining non-flag args
	cmd := ""
	remainingArgs := flag.CommandLine.Args()
	if len(remainingArgs) > 0 {
		cmd = remainingArgs[0]
	}

	switch cmd {
	case "paths":
		return runPaths()
	case "version":
		return runVersion()
	case "migrate":
		return runMigrate()
	case "help":
		return runHelp()
	case "":
		// No subcommand - start the server
		return runServer()
	default:
		// Unknown subcommand
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

// CLI flag definitions
var (
	dataDir       = flag.String("data-dir", "", "Data root directory (env: ACE_DATA_DIR)")
	configFile    = flag.String("config", "", "Config file path (env: ACE_CONFIG)")
	port          = flag.Int("port", 0, "HTTP listen port (env: ACE_PORT)")
	host          = flag.String("host", "", "HTTP listen address (env: ACE_HOST)")
	dbMode        = flag.String("db-mode", "", "Database mode: embedded|external (env: ACE_DB_MODE)")
	dbURL         = flag.String("db-url", "", "Database URL (env: ACE_DB_URL)")
	natsMode      = flag.String("nats-mode", "", "NATS mode: embedded|external (env: ACE_NATS_MODE)")
	natsURL       = flag.String("nats-url", "", "NATS URL (env: ACE_NATS_URL)")
	cacheMode     = flag.String("cache-mode", "", "Cache mode: embedded|external (env: ACE_CACHE_MODE)")
	cacheURL      = flag.String("cache-url", "", "Cache URL (env: ACE_CACHE_URL)")
	cacheMaxCost  = flag.Int("cache-max-cost", 0, "Max cache cost in bytes (env: ACE_CACHE_MAX_COST)")
	telemetryMode = flag.String("telemetry-mode", "", "Telemetry mode: embedded|external (env: ACE_TELEMETRY_MODE)")
	otlpEndpoint  = flag.String("otlp-endpoint", "", "OTLP collector URL (env: ACE_OTLP_ENDPOINT)")
	dev           = flag.Bool("dev", false, "Enable development mode: proxy frontend to Vite dev server (env: ACE_DEV)")
)

func runServer() error {
	// Create logger for CLI startup messages
	logger, err := telemetry.NewLoggerWithStdout("ace", "development")
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}

	// Build CLI config from flags
	cliCfg := &app.Config{
		DataDir:       *dataDir,
		ConfigFile:    *configFile,
		Host:          *host,
		Port:          *port,
		DBMode:        *dbMode,
		DBURL:         *dbURL,
		NATSMode:      *natsMode,
		NATSURL:       *natsURL,
		CacheMode:     *cacheMode,
		CacheURL:      *cacheURL,
		CacheMaxCost:  *cacheMaxCost,
		TelemetryMode: *telemetryMode,
		OTLPEndpoint:  *otlpEndpoint,
		Dev:           *dev,
	}

	// Resolve configuration
	cfg, err := app.ResolveConfig(cliCfg)
	if err != nil {
		return fmt.Errorf("resolve config: %w", err)
	}

	// Validate configuration for server startup
	if err := app.ValidateConfig(cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	logger.Info("starting ACE",
		zap.String("version", version),
		zap.String("commit", commit),
		zap.String("build_date", buildDate),
	)

	// Create app
	ace, err := app.New(cfg)
	if err != nil {
		return fmt.Errorf("create app: %w", err)
	}

	// Start HTTP server
	if err := ace.Serve(); err != nil {
		return fmt.Errorf("serve: %w", err)
	}

	// Wait for shutdown signal
	sig := app.WaitForSignal()
	logger.Info("received signal", zap.Stringer("signal", sig))

	// Graceful shutdown
	if err := ace.Shutdown(); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	return nil
}

func runPaths() error {
	// Resolve paths directly - paths command doesn't need full config validation
	// CLI flag --data-dir takes priority, then env var, then XDG default
	dataDirOverride := *dataDir
	if dataDirOverride == "" {
		dataDirOverride = os.Getenv("ACE_DATA_DIR")
	}

	paths, err := platform.ResolvePaths(dataDirOverride)
	if err != nil {
		return err
	}

	paths.PrintPaths()
	return nil
}

func runVersion() error {
	fmt.Printf("ace v%s\n", version)
	fmt.Printf("go%s\n", "1.26.0")
	fmt.Printf("build: %s\n", buildDate)
	fmt.Printf("commit: %s\n", commit)
	return nil
}

func runMigrate() error {
	fmt.Println("Running migrations...")

	// Resolve paths to get data directory
	dataDirOverride := *dataDir
	if dataDirOverride == "" {
		dataDirOverride = os.Getenv("ACE_DATA_DIR")
	}

	paths, err := platform.ResolvePaths(dataDirOverride)
	if err != nil {
		return fmt.Errorf("resolve paths: %w", err)
	}

	// Open database
	dbCfg := &database.Config{
		Mode:    "embedded",
		DataDir: paths.DataDir,
	}

	// If db-mode flag was provided, use it
	if *dbMode != "" {
		dbCfg.Mode = *dbMode
		dbCfg.URL = *dbURL
	}

	db, err := database.Open(dbCfg)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	// Run migrations
	if err := database.Migrate(db); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	// Count applied migrations
	count, err := countMigrations(db)
	if err != nil {
		return fmt.Errorf("count migrations: %w", err)
	}

	fmt.Printf("Migrations complete: %d applied, 0 pending.\n", count)
	return nil
}

func countMigrations(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM goose_db_version").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func runHelp() error {
	printHelp()
	return nil
}

func printHelp() {
	fmt.Println(`ACE — Agent Configuration Engine

Usage:
  ace [flags] [command]

Commands:
  ace           Start the ACE server (default)
  ace paths     Print configured filesystem paths
  ace version   Print version information
  ace migrate   Run database migrations and exit
  ace help      Print this help message

Flags:
  --data-dir string         Data root directory (env: ACE_DATA_DIR)
  --config string           Config file path (env: ACE_CONFIG)
  --port int                HTTP listen port (env: ACE_PORT, default: 8080)
  --host string             HTTP listen address (env: ACE_HOST, default: 0.0.0.0)
  --db-mode string          Database mode: embedded|external (env: ACE_DB_MODE, default: embedded)
  --db-url string           Database URL (env: ACE_DB_URL, required when db-mode=external)
  --nats-mode string        NATS mode: embedded|external (env: ACE_NATS_MODE, default: embedded)
  --nats-url string         NATS URL (env: ACE_NATS_URL, required when nats-mode=external)
  --cache-mode string       Cache mode: embedded|external (env: ACE_CACHE_MODE, default: embedded)
  --cache-url string        Cache URL (env: ACE_CACHE_URL, required when cache-mode=external)
  --cache-max-cost int      Max cache cost in bytes (env: ACE_CACHE_MAX_COST, default: 52428800)
  --telemetry-mode string   Telemetry mode: embedded|external (env: ACE_TELEMETRY_MODE, default: embedded)
  --otlp-endpoint string    OTLP collector URL (env: ACE_OTLP_ENDPOINT, required when telemetry-mode=external)
  --dev                     Enable development mode: proxy frontend to Vite dev server (env: ACE_DEV)

Configuration priority: CLI flags > environment variables > config file > XDG defaults`)
}
