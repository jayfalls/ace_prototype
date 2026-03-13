// Package config provides tests for the configuration module.
package config

import (
	"os"
	"testing"
)

func TestLoadWithDefaults(t *testing.T) {
	// Set required environment variables
	os.Setenv("POSTGRES_HOST", "localhost")
	os.Setenv("POSTGRES_PORT", "5432")
	os.Setenv("POSTGRES_USER", "testuser")
	os.Setenv("POSTGRES_PASSWORD", "testpass")
	os.Setenv("POSTGRES_DB", "testdb")
	defer os.Unsetenv("POSTGRES_HOST")
	defer os.Unsetenv("POSTGRES_PORT")
	defer os.Unsetenv("POSTGRES_USER")
	defer os.Unsetenv("POSTGRES_PASSWORD")
	defer os.Unsetenv("POSTGRES_DB")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Database.Host != "localhost" {
		t.Errorf("Expected host 'localhost', got '%s'", cfg.Database.Host)
	}
	if cfg.Database.Port != 5432 {
		t.Errorf("Expected port 5432, got %d", cfg.Database.Port)
	}
	if cfg.Database.User != "testuser" {
		t.Errorf("Expected user 'testuser', got '%s'", cfg.Database.User)
	}
	if cfg.Database.Password != "testpass" {
		t.Errorf("Expected password 'testpass', got '%s'", cfg.Database.Password)
	}
	if cfg.Database.DB != "testdb" {
		t.Errorf("Expected DB 'testdb', got '%s'", cfg.Database.DB)
	}
	if cfg.API.Port != "8080" {
		t.Errorf("Expected API port '8080', got '%s'", cfg.API.Port)
	}
}

func TestLoadMissingRequiredField(t *testing.T) {
	// Clear all environment variables
	os.Unsetenv("POSTGRES_HOST")
	os.Unsetenv("POSTGRES_PORT")
	os.Unsetenv("POSTGRES_USER")
	os.Unsetenv("POSTGRES_PASSWORD")
	os.Unsetenv("POSTGRES_DB")

	_, err := Load()
	if err == nil {
		t.Error("Expected error for missing required fields, got nil")
	}
}

func TestDatabaseConfigDSN(t *testing.T) {
	cfg := &DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "user",
		Password: "password",
		DB:       "testdb",
	}

	dsn := cfg.DSN()
	expected := "postgres://user:password@localhost:5432/testdb?sslmode=disable"
	if dsn != expected {
		t.Errorf("Expected DSN '%s', got '%s'", expected, dsn)
	}
}

func TestGetEnv(t *testing.T) {
	// Test with existing env var
	os.Setenv("TEST_KEY", "test_value")
	defer os.Unsetenv("TEST_KEY")

	result := getEnv("TEST_KEY", "default")
	if result != "test_value" {
		t.Errorf("Expected 'test_value', got '%s'", result)
	}

	// Test with missing env var
	result = getEnv("NON_EXISTENT_KEY", "default_value")
	if result != "default_value" {
		t.Errorf("Expected 'default_value', got '%s'", result)
	}
}

func TestGetEnvAsInt(t *testing.T) {
	// Test with valid int
	os.Setenv("TEST_INT", "123")
	defer os.Unsetenv("TEST_INT")

	result := getEnvAsInt("TEST_INT", 0)
	if result != 123 {
		t.Errorf("Expected 123, got %d", result)
	}

	// Test with invalid int
	os.Setenv("TEST_INVALID_INT", "abc")
	defer os.Unsetenv("TEST_INVALID_INT")

	result = getEnvAsInt("TEST_INVALID_INT", 99)
	if result != 99 {
		t.Errorf("Expected 99 for invalid int, got %d", result)
	}

	// Test with missing env var
	result = getEnvAsInt("NON_EXISTENT_INT", 42)
	if result != 42 {
		t.Errorf("Expected 42 for missing env var, got %d", result)
	}
}

func TestGetEnvAsInt32(t *testing.T) {
	// Test with valid int32
	os.Setenv("TEST_INT32", "456")
	defer os.Unsetenv("TEST_INT32")

	result := getEnvAsInt32("TEST_INT32", 0)
	if result != 456 {
		t.Errorf("Expected 456, got %d", result)
	}

	// Test with missing env var
	result = getEnvAsInt32("NON_EXISTENT_INT32", 100)
	if result != 100 {
		t.Errorf("Expected 100 for missing env var, got %d", result)
	}
}

func TestLoadWithCustomPoolSettings(t *testing.T) {
	os.Setenv("POSTGRES_HOST", "localhost")
	os.Setenv("POSTGRES_PORT", "5432")
	os.Setenv("POSTGRES_USER", "testuser")
	os.Setenv("POSTGRES_PASSWORD", "testpass")
	os.Setenv("POSTGRES_DB", "testdb")
	os.Setenv("POSTGRES_MAX_CONNS", "50")
	os.Setenv("POSTGRES_MIN_CONNS", "10")
	defer os.Unsetenv("POSTGRES_HOST")
	defer os.Unsetenv("POSTGRES_PORT")
	defer os.Unsetenv("POSTGRES_USER")
	defer os.Unsetenv("POSTGRES_PASSWORD")
	defer os.Unsetenv("POSTGRES_DB")
	defer os.Unsetenv("POSTGRES_MAX_CONNS")
	defer os.Unsetenv("POSTGRES_MIN_CONNS")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Database.MaxConns != 50 {
		t.Errorf("Expected MaxConns 50, got %d", cfg.Database.MaxConns)
	}
	if cfg.Database.MinConns != 10 {
		t.Errorf("Expected MinConns 10, got %d", cfg.Database.MinConns)
	}
}

func TestLoadWithCORSOrigins(t *testing.T) {
	os.Setenv("POSTGRES_HOST", "localhost")
	os.Setenv("POSTGRES_PORT", "5432")
	os.Setenv("POSTGRES_USER", "testuser")
	os.Setenv("POSTGRES_PASSWORD", "testpass")
	os.Setenv("POSTGRES_DB", "testdb")
	os.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173, https://example.com")
	defer os.Unsetenv("POSTGRES_HOST")
	defer os.Unsetenv("POSTGRES_PORT")
	defer os.Unsetenv("POSTGRES_USER")
	defer os.Unsetenv("POSTGRES_PASSWORD")
	defer os.Unsetenv("POSTGRES_DB")
	defer os.Unsetenv("CORS_ALLOWED_ORIGINS")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.API.CORSAllowedOrigins) != 2 {
		t.Errorf("Expected 2 CORS origins, got %d", len(cfg.API.CORSAllowedOrigins))
	}
	if cfg.API.CORSAllowedOrigins[0] != "http://localhost:5173" {
		t.Errorf("Expected first origin 'http://localhost:5173', got '%s'", cfg.API.CORSAllowedOrigins[0])
	}
	if cfg.API.CORSAllowedOrigins[1] != "https://example.com" {
		t.Errorf("Expected second origin 'https://example.com', got '%s'", cfg.API.CORSAllowedOrigins[1])
	}
}

func TestLoadWithDefaultCORSOrigins(t *testing.T) {
	os.Setenv("POSTGRES_HOST", "localhost")
	os.Setenv("POSTGRES_PORT", "5432")
	os.Setenv("POSTGRES_USER", "testuser")
	os.Setenv("POSTGRES_PASSWORD", "testpass")
	os.Setenv("POSTGRES_DB", "testdb")
	defer os.Unsetenv("POSTGRES_HOST")
	defer os.Unsetenv("POSTGRES_PORT")
	defer os.Unsetenv("POSTGRES_USER")
	defer os.Unsetenv("POSTGRES_PASSWORD")
	defer os.Unsetenv("POSTGRES_DB")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Default should be ["*"]
	if len(cfg.API.CORSAllowedOrigins) != 1 || cfg.API.CORSAllowedOrigins[0] != "*" {
		t.Errorf("Expected default CORS origins ['*'], got %v", cfg.API.CORSAllowedOrigins)
	}
}
