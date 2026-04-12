package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePaths_Defaults(t *testing.T) {
	// Use t.Setenv for proper env var handling in tests (Go 1.17+)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")

	paths, err := ResolvePaths("")
	if err != nil {
		t.Fatalf("ResolvePaths failed: %v", err)
	}

	home, _ := os.UserHomeDir()
	expectedDataDir := filepath.Join(home, ".local", "share", "ace")
	expectedConfigDir := filepath.Join(home, ".config", "ace")
	expectedLogDir := filepath.Join(home, ".local", "state", "ace", "logs")

	if paths.DataDir != expectedDataDir {
		t.Errorf("DataDir: got %q, want %q", paths.DataDir, expectedDataDir)
	}
	if paths.ConfigDir != expectedConfigDir {
		t.Errorf("ConfigDir: got %q, want %q", paths.ConfigDir, expectedConfigDir)
	}
	if paths.LogDir != expectedLogDir {
		t.Errorf("LogDir: got %q, want %q", paths.LogDir, expectedLogDir)
	}
	if paths.DBPath != filepath.Join(expectedDataDir, "ace.db") {
		t.Errorf("DBPath: got %q, want %q", paths.DBPath, filepath.Join(expectedDataDir, "ace.db"))
	}
	if paths.NATSPath != filepath.Join(expectedDataDir, "nats") {
		t.Errorf("NATSPath: got %q, want %q", paths.NATSPath, filepath.Join(expectedDataDir, "nats"))
	}
	if paths.TelemetryPath != filepath.Join(expectedDataDir, "telemetry") {
		t.Errorf("TelemetryPath: got %q, want %q", paths.TelemetryPath, filepath.Join(expectedDataDir, "telemetry"))
	}
}

func TestResolvePaths_EnvOverride(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/custom/data")

	paths, err := ResolvePaths("")
	if err != nil {
		t.Fatalf("ResolvePaths failed: %v", err)
	}

	if paths.DataDir != "/custom/data/ace" {
		t.Errorf("DataDir: got %q, want %q", paths.DataDir, "/custom/data/ace")
	}
}

func TestResolvePaths_CLIFlag(t *testing.T) {
	paths, err := ResolvePaths("/tmp/ace-cli-test")
	if err != nil {
		t.Fatalf("ResolvePaths failed: %v", err)
	}

	if paths.DataDir != "/tmp/ace-cli-test" {
		t.Errorf("DataDir: got %q, want %q", paths.DataDir, "/tmp/ace-cli-test")
	}
	if paths.DBPath != "/tmp/ace-cli-test/ace.db" {
		t.Errorf("DBPath: got %q, want %q", paths.DBPath, "/tmp/ace-cli-test/ace.db")
	}
}

func TestEnsureDirs_CreatesTree(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "ace-test")

	paths := Paths{
		DataDir:       dataDir,
		ConfigDir:     filepath.Join(dataDir, "config"),
		LogDir:        filepath.Join(dataDir, "logs"),
		DBPath:        filepath.Join(dataDir, "ace.db"),
		NATSPath:      filepath.Join(dataDir, "nats"),
		TelemetryPath: filepath.Join(dataDir, "telemetry"),
	}

	err := paths.EnsureDirs()
	if err != nil {
		t.Fatalf("EnsureDirs failed: %v", err)
	}

	// Verify directories were created
	for _, dir := range []string{paths.DataDir, paths.ConfigDir, paths.LogDir, paths.NATSPath, paths.TelemetryPath} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("directory %q was not created: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("path %q exists but is not a directory", dir)
		}
		// Check permissions (0700)
		if info.Mode().Perm() != 0700 {
			t.Errorf("directory %q has permissions %o, want 0700", dir, info.Mode().Perm())
		}
	}

	// Verify file paths are within data dir
	if paths.DBPath != filepath.Join(dataDir, "ace.db") {
		t.Errorf("DBPath: got %q, want %q", paths.DBPath, filepath.Join(dataDir, "ace.db"))
	}
}
