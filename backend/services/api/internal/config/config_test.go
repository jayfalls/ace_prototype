package config

import (
	"os"
	"testing"
)

func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"DATABASE_URL", "POSTGRES_HOST", "POSTGRES_PORT",
		"POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB",
		"API_PORT", "CORS_ALLOWED_ORIGINS",
	} {
		os.Unsetenv(key)
	}
}

func setPostgresVars(t *testing.T) {
	t.Helper()
	os.Setenv("POSTGRES_HOST", "localhost")
	os.Setenv("POSTGRES_PORT", "5432")
	os.Setenv("POSTGRES_USER", "testuser")
	os.Setenv("POSTGRES_PASSWORD", "testpass")
	os.Setenv("POSTGRES_DB", "testdb")
	os.Setenv("API_PORT", "8080")
	os.Setenv("CORS_ALLOWED_ORIGINS", "*")
}

func TestLoadWithPostgresVars(t *testing.T) {
	clearEnv(t)
	setPostgresVars(t)
	defer clearEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	expected := "postgres://testuser:testpass@localhost:5432/testdb?sslmode=disable"
	if cfg.DatabaseURL != expected {
		t.Errorf("Expected DatabaseURL '%s', got '%s'", expected, cfg.DatabaseURL)
	}
	if cfg.APIPort != "8080" {
		t.Errorf("Expected APIPort '8080', got '%s'", cfg.APIPort)
	}
}

func TestLoadWithDatabaseURL(t *testing.T) {
	clearEnv(t)
	defer clearEnv(t)

	os.Setenv("DATABASE_URL", "postgres://u:p@host:5432/db")
	os.Setenv("API_PORT", "9090")
	os.Setenv("CORS_ALLOWED_ORIGINS", "*")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DatabaseURL != "postgres://u:p@host:5432/db" {
		t.Errorf("Expected DATABASE_URL to be used, got '%s'", cfg.DatabaseURL)
	}
}

func TestLoadMissingDatabase(t *testing.T) {
	clearEnv(t)
	defer clearEnv(t)

	os.Setenv("API_PORT", "8080")
	os.Setenv("CORS_ALLOWED_ORIGINS", "*")

	_, err := Load()
	if err == nil {
		t.Error("Expected error for missing database config, got nil")
	}
}

func TestLoadMissingPartialPostgresVars(t *testing.T) {
	clearEnv(t)
	defer clearEnv(t)

	os.Setenv("POSTGRES_HOST", "localhost")
	os.Setenv("API_PORT", "8080")
	os.Setenv("CORS_ALLOWED_ORIGINS", "*")

	_, err := Load()
	if err == nil {
		t.Error("Expected error for incomplete POSTGRES_* vars, got nil")
	}
}

func TestLoadMissingAPIPort(t *testing.T) {
	clearEnv(t)
	defer clearEnv(t)

	os.Setenv("DATABASE_URL", "postgres://u:p@host:5432/db")
	os.Setenv("CORS_ALLOWED_ORIGINS", "*")

	_, err := Load()
	if err == nil {
		t.Error("Expected error for missing API_PORT, got nil")
	}
}

func TestLoadMissingCORS(t *testing.T) {
	clearEnv(t)
	defer clearEnv(t)

	os.Setenv("DATABASE_URL", "postgres://u:p@host:5432/db")
	os.Setenv("API_PORT", "8080")

	_, err := Load()
	if err == nil {
		t.Error("Expected error for missing CORS_ALLOWED_ORIGINS, got nil")
	}
}

func TestLoadWithCORSOrigins(t *testing.T) {
	clearEnv(t)
	defer clearEnv(t)

	os.Setenv("DATABASE_URL", "postgres://u:p@host:5432/db")
	os.Setenv("API_PORT", "8080")
	os.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173, https://example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.CORSAllowedOrigins) != 2 {
		t.Fatalf("Expected 2 CORS origins, got %d", len(cfg.CORSAllowedOrigins))
	}
	if cfg.CORSAllowedOrigins[0] != "http://localhost:5173" {
		t.Errorf("Expected first origin 'http://localhost:5173', got '%s'", cfg.CORSAllowedOrigins[0])
	}
	if cfg.CORSAllowedOrigins[1] != "https://example.com" {
		t.Errorf("Expected second origin 'https://example.com', got '%s'", cfg.CORSAllowedOrigins[1])
	}
}
