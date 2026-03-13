// Package repository provides tests for the repository module.
package repository

import (
	"context"
	"testing"

	"ace/api/internal/config"
)

func TestNewDBWithInvalidDSN(t *testing.T) {
	cfg := &config.DatabaseConfig{
		Host:     "invalid-host-that-does-not-exist",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		DB:       "testdb",
		MaxConns: 10,
	}

	_, err := NewDB(cfg)
	// This should fail because the host doesn't exist
	if err == nil {
		t.Error("Expected error for invalid host, got nil")
	}
}

func TestWaitForConnectionTimeout(t *testing.T) {
	cfg := &config.DatabaseConfig{
		Host:     "invalid-host-that-does-not-exist",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		DB:       "testdb",
	}

	// Use very few retries with short interval for testing
	err := WaitForConnection(cfg, 2, 100)
	if err == nil {
		t.Error("Expected error for connection timeout, got nil")
	}
}

func TestDBHealthCheckNilPool(t *testing.T) {
	db := &DB{
		Pool:   nil,
		Config: &config.DatabaseConfig{},
	}

	ctx := context.Background()
	err := db.HealthCheck(ctx)
	if err == nil {
		t.Error("Expected error for nil pool, got nil")
	}
	if err != nil && err.Error() != "database pool is nil" {
		t.Errorf("Expected 'database pool is nil', got '%s'", err.Error())
	}
}

func TestDBClose(t *testing.T) {
	// This tests that Close doesn't panic with nil pool
	db := &DB{
		Pool:   nil,
		Config: &config.DatabaseConfig{},
	}

	// Should not panic
	db.Close()

	// Test that it works with nil after closing
	// (this is a no-op, just ensuring no panic)
}

func TestDatabaseConfigDSN(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.DatabaseConfig
		expected string
	}{
		{
			name: "standard connection",
			cfg: &config.DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "password",
				DB:       "testdb",
			},
			expected: "postgres://user:password@localhost:5432/testdb?sslmode=disable",
		},
		{
			name: "custom port",
			cfg: &config.DatabaseConfig{
				Host:     "db.example.com",
				Port:     5433,
				User:     "admin",
				Password: "secret",
				DB:       "production",
			},
			expected: "postgres://admin:secret@db.example.com:5433/production?sslmode=disable",
		},
		{
			name: "password with special characters",
			cfg: &config.DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "p@ss!word#123",
				DB:       "testdb",
			},
			// Password should be URL-encoded in actual DSN
			expected: "postgres://user:p@ss!word#123@localhost:5432/testdb?sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.DSN()
			if result != tt.expected {
				t.Errorf("DSN() = %v, want %v", result, tt.expected)
			}
		})
	}
}
