package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenEmbedded_CreatesDatabaseFile(t *testing.T) {
	tmpDir := t.TempDir()
	expectedDBPath := filepath.Join(tmpDir, "ace.db")

	cfg := &Config{
		Mode:    "embedded",
		DataDir: tmpDir,
	}

	// Database file should not exist yet
	if _, err := os.Stat(expectedDBPath); !os.IsNotExist(err) {
		t.Fatal("Database file should not exist before Open")
	}

	db, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	db.Close()

	// Database file should now exist
	if _, err := os.Stat(expectedDBPath); os.IsNotExist(err) {
		t.Fatal("Database file should exist at ace.db")
	}
}

func TestOpenEmbedded_InvalidMode(t *testing.T) {
	cfg := &Config{
		Mode: "invalid",
	}

	_, err := Open(cfg)
	if err == nil {
		t.Fatal("Expected error for invalid mode")
	}
}

func TestOpenEmbedded_EmptyDataDir(t *testing.T) {
	// Empty data dir should use XDG default
	cfg := &Config{
		Mode:    "embedded",
		DataDir: "",
	}

	db, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	db.Close()
}
