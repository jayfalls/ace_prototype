// Package platform provides platform-level concerns: filesystem paths and directories.
package platform

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
)

// Paths holds all resolved filesystem paths for the ACE application.
// These paths follow XDG base directory specification by default.
type Paths struct {
	DataDir       string // Root directory for persistent data
	ConfigDir     string // Root directory for configuration
	LogDir        string // Root directory for log files
	DBPath        string // SQLite database file path
	NATSPath      string // NATS JetStream storage path
	TelemetryPath string // Telemetry storage path
}

// ResolvePaths resolves filesystem paths using XDG base directory specification.
// If dataDir is non-empty, it overrides the XDG_DATA_HOME default.
// The config dir is derived from XDG_CONFIG_HOME.
// The log dir is derived from XDG_STATE_HOME.
func ResolvePaths(dataDir string) (Paths, error) {
	// Reload XDG paths from environment to pick up any changes
	xdg.Reload()

	var paths Paths

	// Resolve DataDir
	if dataDir != "" {
		paths.DataDir = dataDir
	} else {
		xdgDataHome := xdg.DataHome
		paths.DataDir = filepath.Join(xdgDataHome, "ace")
	}

	// Resolve ConfigDir (from XDG_CONFIG_HOME)
	xdgConfigHome := xdg.ConfigHome
	paths.ConfigDir = filepath.Join(xdgConfigHome, "ace")

	// Resolve LogDir (from XDG_STATE_HOME)
	xdgStateHome := xdg.StateHome
	paths.LogDir = filepath.Join(xdgStateHome, "ace", "logs")

	// Resolve specific file/directory paths within DataDir
	paths.DBPath = filepath.Join(paths.DataDir, "ace.db")
	paths.NATSPath = filepath.Join(paths.DataDir, "nats")
	paths.TelemetryPath = filepath.Join(paths.DataDir, "telemetry")

	return paths, nil
}

// EnsureDirs creates all required directories with 0700 permissions.
// Directories are created recursively if they don't exist.
func (p *Paths) EnsureDirs() error {
	dirs := []string{
		p.DataDir,
		p.ConfigDir,
		p.LogDir,
		p.NATSPath,
		p.TelemetryPath,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	return nil
}

// PrintPaths outputs the resolved paths in a human-readable format.
// Used by the `ace paths` command.
func (p *Paths) PrintPaths() {
	fmt.Println("Data Dir:     ", p.DataDir)
	fmt.Println("Config Dir:   ", p.ConfigDir)
	fmt.Println("Log Dir:      ", p.LogDir)
	fmt.Println("Database:     ", p.DBPath)
	fmt.Println("NATS Store:   ", p.NATSPath)
	fmt.Println("Telemetry:    ", p.TelemetryPath)
	fmt.Println("Cache:        (in-process, 50MB max)")
}
