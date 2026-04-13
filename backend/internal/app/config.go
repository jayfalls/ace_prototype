// Package app provides the core application structure and lifecycle.
package app

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration values for the ACE application.
// Fields are ordered by category matching the FSD §1.2-1.3.
type Config struct {
	// Server configuration
	Host string
	Port int

	// Data directory override
	DataDir string

	// Config file path
	ConfigFile string

	// Database configuration
	DBMode string
	DBURL  string

	// Messaging (NATS) configuration
	NATSMode string
	NATSURL  string

	// Cache configuration
	CacheMode    string
	CacheURL     string
	CacheMaxCost int

	// Telemetry configuration
	TelemetryMode string
	OTLPEndpoint  string

	// Development mode
	Dev bool

	// Auth configuration (from config file only, not CLI flags)
	Auth AuthConfig
}

// AuthConfig holds authentication configuration loaded from config file.
type AuthConfig struct {
	JWTSecret             string
	AccessTokenTTL        string
	RefreshTokenTTL       string
	JWTAudience           []string
	JWTIssuer             string
	RateLimitPerIP        int
	RateLimitPerEmail     int
	RateLimitWindow       string
	LoginLockoutThreshold int
	LoginLockoutDuration  string
	PasswordMinLength     int
	PasswordRequireUpper  bool
	PasswordRequireLower  bool
	PasswordRequireNumber bool
	PasswordRequireSymbol bool
}

// configFile represents the YAML config file structure.
type configFile struct {
	Server      serverConfig      `yaml:"server"`
	Data        dataConfig        `yaml:"data"`
	Database    databaseConfig    `yaml:"database"`
	Messaging   messagingConfig   `yaml:"messaging"`
	Cache       cacheConfig       `yaml:"cache"`
	Telemetry   telemetryConfig   `yaml:"telemetry"`
	Auth        authConfig        `yaml:"auth"`
	Logging     loggingConfig     `yaml:"logging"`
	Development developmentConfig `yaml:"development"`
}

type serverConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type dataConfig struct {
	Dir string `yaml:"dir"`
}

type databaseConfig struct {
	Mode string `yaml:"mode"`
	URL  string `yaml:"url"`
}

type messagingConfig struct {
	Mode string `yaml:"mode"`
	URL  string `yaml:"url"`
}

type cacheConfig struct {
	Mode    string `yaml:"mode"`
	URL     string `yaml:"url"`
	MaxCost int    `yaml:"max_cost"`
}

type telemetryConfig struct {
	Mode         string `yaml:"mode"`
	OTLPEndpoint string `yaml:"otlp_endpoint"`
}

type authConfig struct {
	JWTSecret             string   `yaml:"jwt_secret"`
	AccessTokenTTL        string   `yaml:"access_token_ttl"`
	RefreshTokenTTL       string   `yaml:"refresh_token_ttl"`
	JWTAudience           []string `yaml:"jwt_audience"`
	JWTIssuer             string   `yaml:"jwt_issuer"`
	RateLimitPerIP        int      `yaml:"rate_limit_per_ip"`
	RateLimitPerEmail     int      `yaml:"rate_limit_per_email"`
	RateLimitWindow       string   `yaml:"rate_limit_window"`
	LoginLockoutThreshold int      `yaml:"login_lockout_threshold"`
	LoginLockoutDuration  string   `yaml:"login_lockout_duration"`
	PasswordMinLength     int      `yaml:"password_min_length"`
	PasswordRequireUpper  bool     `yaml:"password_require_upper"`
	PasswordRequireLower  bool     `yaml:"password_require_lower"`
	PasswordRequireNumber bool     `yaml:"password_require_number"`
	PasswordRequireSymbol bool     `yaml:"password_require_symbol"`
}

type loggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type developmentConfig struct {
	Dev bool `yaml:"dev"`
}

// defaultConfig returns the hardcoded defaults for all configuration values.
func defaultConfig() *Config {
	return &Config{
		Host:          "0.0.0.0",
		Port:          8080,
		DataDir:       "",
		ConfigFile:    "",
		DBMode:        "embedded",
		DBURL:         "",
		NATSMode:      "embedded",
		NATSURL:       "",
		CacheMode:     "embedded",
		CacheURL:      "",
		CacheMaxCost:  52428800,
		TelemetryMode: "embedded",
		OTLPEndpoint:  "",
		Dev:           false,
	}
}

// ResolveConfig resolves configuration using priority: CLI flags > env vars > config file > defaults.
// The provided cliConfig contains values from CLI flags (zero values mean "not set").
// Note: This does NOT validate the configuration. Use ValidateConfig separately for server startup.
func ResolveConfig(cliConfig *Config) (*Config, error) {
	cfg := defaultConfig()

	// 1. Load config file if specified
	configFilePath := cliConfig.ConfigFile
	if configFilePath == "" {
		configFilePath = getDefaultConfigPath()
	}
	if configFilePath != "" {
		fileCfg, err := loadConfigFile(configFilePath)
		if err == nil {
			applyFileConfig(cfg, fileCfg)
		}
	}

	// 2. Apply environment variable overrides
	applyEnvConfig(cfg)

	// 3. Apply CLI flag overrides (only non-zero/non-empty values override)
	applyCLIConfig(cfg, cliConfig)

	// 4. Auto-generate JWT secret if not provided
	if cfg.Auth.JWTSecret == "" {
		generatedSecret, err := generateJWTSecret()
		if err != nil {
			return nil, fmt.Errorf("config: failed to generate JWT secret: %w", err)
		}
		cfg.Auth.JWTSecret = generatedSecret
		log.Printf("[WARNING] JWT secret was auto-generated. For production, set jwt_secret in config file or ACE_JWT_SECRET env var")

		// Store the generated secret in config file for persistence
		if configFilePath != "" {
			if err := saveJWTSecretToFile(configFilePath, generatedSecret); err != nil {
				log.Printf("[WARNING] Failed to persist generated JWT secret to config file: %v", err)
			}
		}
	}

	return cfg, nil
}

// ValidateConfig validates the configuration for server startup.
// This should be called after ResolveConfig and before starting the server.
func ValidateConfig(cfg *Config) error {
	return validateConfig(cfg)
}

// getDefaultConfigPath returns the default config file path based on XDG_CONFIG_HOME.
func getDefaultConfigPath() string {
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		xdgConfigHome = filepath.Join(home, ".config")
	}
	return filepath.Join(xdgConfigHome, "ace", "config.yaml")
}

func loadConfigFile(path string) (*configFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var cfg configFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	return &cfg, nil
}

func applyFileConfig(cfg *Config, fileCfg *configFile) {
	// Server config
	if fileCfg.Server.Host != "" {
		cfg.Host = fileCfg.Server.Host
	}
	if fileCfg.Server.Port != 0 {
		cfg.Port = fileCfg.Server.Port
	}

	// Data config
	if fileCfg.Data.Dir != "" {
		cfg.DataDir = fileCfg.Data.Dir
	}

	// Database config
	if fileCfg.Database.Mode != "" {
		cfg.DBMode = fileCfg.Database.Mode
	}
	if fileCfg.Database.URL != "" {
		cfg.DBURL = fileCfg.Database.URL
	}

	// Messaging config
	if fileCfg.Messaging.Mode != "" {
		cfg.NATSMode = fileCfg.Messaging.Mode
	}
	if fileCfg.Messaging.URL != "" {
		cfg.NATSURL = fileCfg.Messaging.URL
	}

	// Cache config
	if fileCfg.Cache.Mode != "" {
		cfg.CacheMode = fileCfg.Cache.Mode
	}
	if fileCfg.Cache.URL != "" {
		cfg.CacheURL = fileCfg.Cache.URL
	}
	if fileCfg.Cache.MaxCost != 0 {
		cfg.CacheMaxCost = fileCfg.Cache.MaxCost
	}

	// Telemetry config
	if fileCfg.Telemetry.Mode != "" {
		cfg.TelemetryMode = fileCfg.Telemetry.Mode
	}
	if fileCfg.Telemetry.OTLPEndpoint != "" {
		cfg.OTLPEndpoint = fileCfg.Telemetry.OTLPEndpoint
	}

	// Auth config
	cfg.Auth.JWTSecret = fileCfg.Auth.JWTSecret
	cfg.Auth.AccessTokenTTL = fileCfg.Auth.AccessTokenTTL
	cfg.Auth.RefreshTokenTTL = fileCfg.Auth.RefreshTokenTTL
	cfg.Auth.JWTAudience = fileCfg.Auth.JWTAudience
	cfg.Auth.JWTIssuer = fileCfg.Auth.JWTIssuer
	cfg.Auth.RateLimitPerIP = fileCfg.Auth.RateLimitPerIP
	cfg.Auth.RateLimitPerEmail = fileCfg.Auth.RateLimitPerEmail
	cfg.Auth.RateLimitWindow = fileCfg.Auth.RateLimitWindow
	cfg.Auth.LoginLockoutThreshold = fileCfg.Auth.LoginLockoutThreshold
	cfg.Auth.LoginLockoutDuration = fileCfg.Auth.LoginLockoutDuration
	cfg.Auth.PasswordMinLength = fileCfg.Auth.PasswordMinLength
	cfg.Auth.PasswordRequireUpper = fileCfg.Auth.PasswordRequireUpper
	cfg.Auth.PasswordRequireLower = fileCfg.Auth.PasswordRequireLower
	cfg.Auth.PasswordRequireNumber = fileCfg.Auth.PasswordRequireNumber
	cfg.Auth.PasswordRequireSymbol = fileCfg.Auth.PasswordRequireSymbol

	// Development config
	if fileCfg.Development.Dev {
		cfg.Dev = true
	}
}

func applyEnvConfig(cfg *Config) {
	// Data dir
	if v := os.Getenv("ACE_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}

	// Config file
	if v := os.Getenv("ACE_CONFIG"); v != "" {
		cfg.ConfigFile = v
	}

	// Server config
	if v := os.Getenv("ACE_HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("ACE_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Port = port
		}
	}

	// Database config
	if v := os.Getenv("ACE_DB_MODE"); v != "" {
		cfg.DBMode = v
	}
	if v := os.Getenv("ACE_DB_URL"); v != "" {
		cfg.DBURL = v
	}

	// Messaging config
	if v := os.Getenv("ACE_NATS_MODE"); v != "" {
		cfg.NATSMode = v
	}
	if v := os.Getenv("ACE_NATS_URL"); v != "" {
		cfg.NATSURL = v
	}

	// Cache config
	if v := os.Getenv("ACE_CACHE_MODE"); v != "" {
		cfg.CacheMode = v
	}
	if v := os.Getenv("ACE_CACHE_URL"); v != "" {
		cfg.CacheURL = v
	}
	if v := os.Getenv("ACE_CACHE_MAX_COST"); v != "" {
		if cost, err := strconv.Atoi(v); err == nil {
			cfg.CacheMaxCost = cost
		}
	}

	// Telemetry config
	if v := os.Getenv("ACE_TELEMETRY_MODE"); v != "" {
		cfg.TelemetryMode = v
	}
	if v := os.Getenv("ACE_OTLP_ENDPOINT"); v != "" {
		cfg.OTLPEndpoint = v
	}

	// Development mode
	if v := os.Getenv("ACE_DEV"); v != "" {
		cfg.Dev = parseBool(v)
	}
}

func applyCLIConfig(cfg *Config, cli *Config) {
	// Only override if CLI value is set (non-zero/non-empty)
	if cli.DataDir != "" {
		cfg.DataDir = cli.DataDir
	}
	if cli.ConfigFile != "" {
		cfg.ConfigFile = cli.ConfigFile
	}
	if cli.Host != "" {
		cfg.Host = cli.Host
	}
	if cli.Port != 0 {
		cfg.Port = cli.Port
	}
	if cli.DBMode != "" {
		cfg.DBMode = cli.DBMode
	}
	if cli.DBURL != "" {
		cfg.DBURL = cli.DBURL
	}
	if cli.NATSMode != "" {
		cfg.NATSMode = cli.NATSMode
	}
	if cli.NATSURL != "" {
		cfg.NATSURL = cli.NATSURL
	}
	if cli.CacheMode != "" {
		cfg.CacheMode = cli.CacheMode
	}
	if cli.CacheURL != "" {
		cfg.CacheURL = cli.CacheURL
	}
	if cli.CacheMaxCost != 0 {
		cfg.CacheMaxCost = cli.CacheMaxCost
	}
	if cli.TelemetryMode != "" {
		cfg.TelemetryMode = cli.TelemetryMode
	}
	if cli.OTLPEndpoint != "" {
		cfg.OTLPEndpoint = cli.OTLPEndpoint
	}
	if cli.Dev {
		cfg.Dev = cli.Dev
	}
}

func validateConfig(cfg *Config) error {
	// Validate port range
	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("config: invalid port: %d (must be between 1 and 65535)", cfg.Port)
	}

	// Validate mode values
	if cfg.DBMode != "embedded" && cfg.DBMode != "external" {
		return fmt.Errorf("config: invalid db-mode: %q (must be \"embedded\" or \"external\")", cfg.DBMode)
	}
	if cfg.NATSMode != "embedded" && cfg.NATSMode != "external" {
		return fmt.Errorf("config: invalid nats-mode: %q (must be \"embedded\" or \"external\")", cfg.NATSMode)
	}
	if cfg.CacheMode != "embedded" && cfg.CacheMode != "external" {
		return fmt.Errorf("config: invalid cache-mode: %q (must be \"embedded\" or \"external\")", cfg.CacheMode)
	}
	if cfg.TelemetryMode != "embedded" && cfg.TelemetryMode != "external" {
		return fmt.Errorf("config: invalid telemetry-mode: %q (must be \"embedded\" or \"external\")", cfg.TelemetryMode)
	}

	// Validate external mode URLs
	if cfg.DBMode == "external" && cfg.DBURL == "" {
		return fmt.Errorf("config: db-url is required when db-mode is \"external\"")
	}
	if cfg.NATSMode == "external" && cfg.NATSURL == "" {
		return fmt.Errorf("config: nats-url is required when nats-mode is \"external\"")
	}
	if cfg.CacheMode == "external" && cfg.CacheURL == "" {
		return fmt.Errorf("config: cache-url is required when cache-mode is \"external\"")
	}
	if cfg.TelemetryMode == "external" && cfg.OTLPEndpoint == "" {
		return fmt.Errorf("config: otlp-endpoint is required when telemetry-mode is \"external\"")
	}

	// Validate auth config
	if cfg.Auth.JWTSecret == "" {
		return fmt.Errorf("config: jwt_secret is required")
	}
	if len(cfg.Auth.JWTSecret) < 32 {
		return fmt.Errorf("config: jwt_secret must be at least 32 characters (got %d)", len(cfg.Auth.JWTSecret))
	}

	return nil
}

func parseBool(v string) bool {
	v = strings.ToLower(v)
	return v == "true" || v == "1" || v == "yes"
}

// generateJWTSecret generates a cryptographically secure random string for JWT signing.
func generateJWTSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// saveJWTSecretToFile saves the JWT secret to the config file.
func saveJWTSecretToFile(configPath, secret string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Config file might not exist yet, create it
		data = []byte{}
	}

	var cfg configFile
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("parse existing config: %w", err)
		}
	}

	// Set the JWT secret
	cfg.Auth.JWTSecret = secret

	// Marshal back to YAML
	updatedData, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, updatedData, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}
