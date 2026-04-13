package repository

import (
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

func TestNewDBWithInvalidDSN(t *testing.T) {
	_, err := NewDB("file:///nonexistent/path/to/db.sqlite")
	if err == nil {
		t.Error("Expected error for invalid path, got nil")
	}
}

func TestDBCloseNilPool(t *testing.T) {
	db := &DB{Pool: nil}
	// Should not panic
	db.Close()
}

func TestNewDBInMemory(t *testing.T) {
	// Create an in-memory SQLite database for testing
	db, err := NewDB("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// Verify we can ping the database
	if db.Pool == nil {
		t.Error("Expected non-nil pool")
	}
}

func TestDBClose(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	db, err := NewDB("file:" + dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}
}
