package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveConfig_Defaults(t *testing.T) {
	// Set a valid JWT secret for validation
	os.Setenv("ACE_JWT_SECRET", "this-is-a-valid-secret-that-is-at-least-32-chars")

	cliCfg := &Config{}
	cfg, err := ResolveConfig(cliCfg)
	if err != nil {
		t.Fatalf("ResolveConfig failed: %v", err)
	}

	if cfg.Host != "0.0.0.0" {
		t.Errorf("Host: got %q, want %q", cfg.Host, "0.0.0.0")
	}
	if cfg.Port != 8080 {
		t.Errorf("Port: got %d, want %d", cfg.Port, 8080)
	}
	if cfg.DBMode != "embedded" {
		t.Errorf("DBMode: got %q, want %q", cfg.DBMode, "embedded")
	}
	if cfg.NATSMode != "embedded" {
		t.Errorf("NATSMode: got %q, want %q", cfg.NATSMode, "embedded")
	}
	if cfg.CacheMode != "embedded" {
		t.Errorf("CacheMode: got %q, want %q", cfg.CacheMode, "embedded")
	}
	if cfg.TelemetryMode != "embedded" {
		t.Errorf("TelemetryMode: got %q, want %q", cfg.TelemetryMode, "embedded")
	}
	if cfg.CacheMaxCost != 52428800 {
		t.Errorf("CacheMaxCost: got %d, want %d", cfg.CacheMaxCost, 52428800)
	}
}

func TestResolveConfig_EnvOverrides(t *testing.T) {
	// Set env vars
	os.Setenv("ACE_HOST", "127.0.0.1")
	os.Setenv("ACE_PORT", "9000")
	os.Setenv("ACE_DB_MODE", "external")
	os.Setenv("ACE_DB_URL", "postgres://localhost/acedb")
	os.Setenv("ACE_DEV", "true")
	os.Setenv("ACE_JWT_SECRET", "this-is-a-valid-secret-that-is-at-least-32-chars")
	defer func() {
		os.Unsetenv("ACE_HOST")
		os.Unsetenv("ACE_PORT")
		os.Unsetenv("ACE_DB_MODE")
		os.Unsetenv("ACE_DB_URL")
		os.Unsetenv("ACE_DEV")
		os.Unsetenv("ACE_JWT_SECRET")
	}()

	cliCfg := &Config{}
	cfg, err := ResolveConfig(cliCfg)
	if err != nil {
		t.Fatalf("ResolveConfig failed: %v", err)
	}

	if cfg.Host != "127.0.0.1" {
		t.Errorf("Host: got %q, want %q", cfg.Host, "127.0.0.1")
	}
	if cfg.Port != 9000 {
		t.Errorf("Port: got %d, want %d", cfg.Port, 9000)
	}
	if cfg.DBMode != "external" {
		t.Errorf("DBMode: got %q, want %q", cfg.DBMode, "external")
	}
	if cfg.DBURL != "postgres://localhost/acedb" {
		t.Errorf("DBURL: got %q, want %q", cfg.DBURL, "postgres://localhost/acedb")
	}
	if !cfg.Dev {
		t.Error("Dev: got false, want true")
	}
}

func TestResolveConfig_CLIOverridesEnv(t *testing.T) {
	// Set env vars
	os.Setenv("ACE_HOST", "127.0.0.1")
	os.Setenv("ACE_PORT", "9000")
	os.Setenv("ACE_JWT_SECRET", "this-is-a-valid-secret-that-is-at-least-32-chars")
	defer func() {
		os.Unsetenv("ACE_HOST")
		os.Unsetenv("ACE_PORT")
		os.Unsetenv("ACE_JWT_SECRET")
	}()

	// CLI config should override env
	cliCfg := &Config{
		Host: "0.0.0.0",
		Port: 8080,
	}
	cfg, err := ResolveConfig(cliCfg)
	if err != nil {
		t.Fatalf("ResolveConfig failed: %v", err)
	}

	// CLI should win
	if cfg.Host != "0.0.0.0" {
		t.Errorf("Host: got %q, want %q", cfg.Host, "0.0.0.0")
	}
	if cfg.Port != 8080 {
		t.Errorf("Port: got %d, want %d", cfg.Port, 8080)
	}
}

func TestResolveConfig_FileLoading(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  host: "localhost"
  port: 9999
database:
  mode: "external"
  url: "postgres://localhost/testdb"
cache:
  max_cost: 12345
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	os.Setenv("ACE_JWT_SECRET", "this-is-a-valid-secret-that-is-at-least-32-chars")
	defer os.Unsetenv("ACE_JWT_SECRET")

	cliCfg := &Config{
		ConfigFile: configPath,
	}

	cfg, err := ResolveConfig(cliCfg)
	if err != nil {
		t.Fatalf("ResolveConfig failed: %v", err)
	}

	if cfg.Host != "localhost" {
		t.Errorf("Host: got %q, want %q", cfg.Host, "localhost")
	}
	if cfg.Port != 9999 {
		t.Errorf("Port: got %d, want %d", cfg.Port, 9999)
	}
	if cfg.DBMode != "external" {
		t.Errorf("DBMode: got %q, want %q", cfg.DBMode, "external")
	}
	if cfg.DBURL != "postgres://localhost/testdb" {
		t.Errorf("DBURL: got %q, want %q", cfg.DBURL, "postgres://localhost/testdb")
	}
	if cfg.CacheMaxCost != 12345 {
		t.Errorf("CacheMaxCost: got %d, want %d", cfg.CacheMaxCost, 12345)
	}
}

func TestValidateConfig_Errors(t *testing.T) {
	validJWTSecret := "this-is-a-valid-secret-that-is-32-chars"

	tests := []struct {
		name   string
		config *Config
		errMsg string
	}{
		{
			name: "invalid port too low",
			config: &Config{
				Port: 0,
				Auth: AuthConfig{JWTSecret: validJWTSecret},
			},
			errMsg: "invalid port: 0",
		},
		{
			name: "invalid port too high",
			config: &Config{
				Port: 99999,
				Auth: AuthConfig{JWTSecret: validJWTSecret},
			},
			errMsg: "invalid port: 99999",
		},
		{
			name: "invalid db mode",
			config: &Config{
				Port:   8080,
				DBMode: "invalid",
				Auth:   AuthConfig{JWTSecret: validJWTSecret},
			},
			errMsg: "invalid db-mode",
		},
		{
			name: "external db without url",
			config: &Config{
				Port:          8080,
				DBMode:        "external",
				NATSMode:      "embedded",
				CacheMode:     "embedded",
				TelemetryMode: "embedded",
				Auth:          AuthConfig{JWTSecret: validJWTSecret},
			},
			errMsg: "db-url is required",
		},
		{
			name: "external nats without url",
			config: &Config{
				Port:          8080,
				DBMode:        "embedded",
				NATSMode:      "external",
				CacheMode:     "embedded",
				TelemetryMode: "embedded",
				Auth:          AuthConfig{JWTSecret: validJWTSecret},
			},
			errMsg: "nats-url is required",
		},
		{
			name: "external cache without url",
			config: &Config{
				Port:          8080,
				DBMode:        "embedded",
				NATSMode:      "embedded",
				CacheMode:     "external",
				TelemetryMode: "embedded",
				Auth:          AuthConfig{JWTSecret: validJWTSecret},
			},
			errMsg: "cache-url is required",
		},
		{
			name: "external telemetry without endpoint",
			config: &Config{
				Port:          8080,
				DBMode:        "embedded",
				NATSMode:      "embedded",
				CacheMode:     "embedded",
				TelemetryMode: "external",
				Auth:          AuthConfig{JWTSecret: validJWTSecret},
			},
			errMsg: "otlp-endpoint is required",
		},
		{
			name: "missing jwt secret",
			config: &Config{
				Port:          8080,
				DBMode:        "embedded",
				NATSMode:      "embedded",
				CacheMode:     "embedded",
				TelemetryMode: "embedded",
				Auth:          AuthConfig{JWTSecret: ""},
			},
			errMsg: "jwt_secret is required",
		},
		{
			name: "jwt secret too short",
			config: &Config{
				Port:          8080,
				DBMode:        "embedded",
				NATSMode:      "embedded",
				CacheMode:     "embedded",
				TelemetryMode: "embedded",
				Auth:          AuthConfig{JWTSecret: "short"},
			},
			errMsg: "jwt_secret must be at least 32 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			if err == nil {
				t.Fatalf("ValidateConfig expected error, got nil")
			}
			if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
				t.Errorf("error message %q does not contain %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestValidateConfig_InvalidModeValues(t *testing.T) {
	tests := []struct {
		modeField string
		value     string
	}{
		{"db-mode", "invalid"},
		{"nats-mode", "invalid"},
		{"cache-mode", "invalid"},
		{"telemetry-mode", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.modeField, func(t *testing.T) {
			cfg := &Config{
				Auth: AuthConfig{JWTSecret: "this-is-a-valid-secret-that-is-32-chars"},
			}
			switch tt.modeField {
			case "db-mode":
				cfg.DBMode = tt.value
			case "nats-mode":
				cfg.NATSMode = tt.value
			case "cache-mode":
				cfg.CacheMode = tt.value
			case "telemetry-mode":
				cfg.TelemetryMode = tt.value
			}

			err := ValidateConfig(cfg)
			if err == nil {
				t.Errorf("ValidateConfig expected error for %s=%s, got nil", tt.modeField, tt.value)
			}
		})
	}
}

func TestResolveAndValidate_Integration(t *testing.T) {
	// Test that resolve + validate works correctly for a valid config
	os.Setenv("ACE_JWT_SECRET", "this-is-a-valid-secret-that-is-at-least-32-chars")
	defer os.Unsetenv("ACE_JWT_SECRET")

	// CLI config with port 0 (zero value, won't override defaults)
	cliCfg := &Config{
		Port: 0, // Zero value, will use default 8080
	}
	cfg, err := ResolveConfig(cliCfg)
	if err != nil {
		t.Fatalf("ResolveConfig failed: %v", err)
	}

	// Port 0 is the zero value, so it won't override the default 8080
	// The resolved config should have port=8080 (the default) which is valid
	if cfg.Port != 8080 {
		t.Errorf("expected port 8080 after resolve, got %d", cfg.Port)
	}

	// ValidateConfig should pass for a valid resolved config
	err = ValidateConfig(cfg)
	if err != nil {
		t.Fatalf("ValidateConfig failed for valid resolved config: %v", err)
	}
}

func TestResolveAndValidate_InvalidPort(t *testing.T) {
	// Test that an explicitly invalid port fails validation
	os.Setenv("ACE_JWT_SECRET", "this-is-a-valid-secret-that-is-at-least-32-chars")
	defer os.Unsetenv("ACE_JWT_SECRET")

	cfg := &Config{
		Port: 99999, // Invalid port
		Auth: AuthConfig{JWTSecret: "this-is-a-valid-secret-that-is-32-chars"},
	}

	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("ValidateConfig expected error for port=99999, got nil")
	}
	if !contains(err.Error(), "invalid port: 99999") {
		t.Errorf("error message %q does not contain %q", err.Error(), "invalid port: 99999")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
