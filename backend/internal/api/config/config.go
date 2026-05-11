// Package config provides configuration management for the API service.
package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the API service.
type Config struct {
	// Database configuration
	DatabaseURL string

	// API configuration
	APIHost string
	APIPort string

	// CORS configuration
	CORSAllowedOrigins []string

	// Logging configuration
	LogLevel string

	// JWT configuration (legacy - replaced by AccessTokenTTL, RefreshTokenTTL, JWTAudience, JWTIssuer)
	JWTSecret          string
	JWTExpirationHours int

	// Auth JWT configuration
	AccessTokenTTL  time.Duration // default 15 minutes
	RefreshTokenTTL time.Duration // default 7 days
	JWTAudience     []string      // default ["ace-api"]
	JWTIssuer       string        // default "ace-auth"

	// Rate limiting
	RateLimitPerIP        int           // requests per window
	RateLimitPerEmail     int           // requests per window
	RateLimitWindow       time.Duration // window duration
	LoginLockoutThreshold int           // failed attempts before lockout
	LoginLockoutDuration  time.Duration // lockout duration

	// Password requirements
	PasswordMinLength     int
	PasswordRequireUpper  bool
	PasswordRequireLower  bool
	PasswordRequireNumber bool
	PasswordRequireSymbol bool

	// Token TTLs
	EmailTokenTTL time.Duration // magic link default 15 min
	ResetTokenTTL time.Duration // password reset default 1 hour

	// Deployment mode
	DeploymentMode string // "single" or "multi"
	BaseURL        string // for email links

	// NATS configuration
	NATSURL string

	// Telemetry configuration
	Environment  string
	OTLPEndpoint string

	// Provider encryption configuration
	ProviderEncryptionKey string
}

// Load loads configuration from environment variables.
func Load() (*Config, error) {
	// Database configuration
	dbURL, err := getDatabaseURL()
	if err != nil {
		return nil, fmt.Errorf("database configuration: %w", err)
	}

	// API configuration
	apiHost := getEnvString("API_HOST", "0.0.0.0")
	apiPort := os.Getenv("API_PORT")
	if apiPort == "" {
		return nil, fmt.Errorf("API_PORT is required")
	}

	// CORS configuration
	cors := os.Getenv("CORS_ALLOWED_ORIGINS")
	if cors == "" {
		return nil, fmt.Errorf("CORS_ALLOWED_ORIGINS is required")
	}
	origins := parseStringList(cors)

	// Log level configuration
	logLevel := getEnvString("LOG_LEVEL", "info")
	if !isValidLogLevel(logLevel) {
		return nil, fmt.Errorf("LOG_LEVEL must be one of: debug, info, warn, error")
	}

	// JWT configuration (legacy)
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if len(jwtSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	jwtExpirationHours := getEnvInt("JWT_EXPIRATION_HOURS", 24)

	// Auth JWT configuration
	accessTokenTTL := getEnvDuration("ACCESS_TOKEN_TTL", 15*time.Minute)
	refreshTokenTTL := getEnvDuration("REFRESH_TOKEN_TTL", 7*24*time.Hour)
	jwtAudience := parseStringList(getEnvString("JWT_AUDIENCE", "ace-api"))
	jwtIssuer := getEnvString("JWT_ISSUER", "ace-auth")

	// Rate limiting configuration
	rateLimitPerIP := getEnvInt("RATE_LIMIT_PER_IP", 100)
	rateLimitPerEmail := getEnvInt("RATE_LIMIT_PER_EMAIL", 10)
	rateLimitWindow := getEnvDuration("RATE_LIMIT_WINDOW", time.Minute)
	loginLockoutThreshold := getEnvInt("LOGIN_LOCKOUT_THRESHOLD", 5)
	loginLockoutDuration := getEnvDuration("LOGIN_LOCKOUT_DURATION", 15*time.Minute)

	// Password requirements
	passwordMinLength := getEnvInt("PASSWORD_MIN_LENGTH", 8)
	passwordRequireUpper := getEnvBool("PASSWORD_REQUIRE_UPPER", true)
	passwordRequireLower := getEnvBool("PASSWORD_REQUIRE_LOWER", true)
	passwordRequireNumber := getEnvBool("PASSWORD_REQUIRE_NUMBER", true)
	passwordRequireSymbol := getEnvBool("PASSWORD_REQUIRE_SYMBOL", false)

	// Token TTLs
	emailTokenTTL := getEnvDuration("EMAIL_TOKEN_TTL", 15*time.Minute)
	resetTokenTTL := getEnvDuration("RESET_TOKEN_TTL", time.Hour)

	// Deployment mode
	deploymentMode := getEnvString("DEPLOYMENT_MODE", "single")
	baseURL := getEnvString("BASE_URL", "http://localhost:3000")

	// NATS configuration
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	// Telemetry configuration
	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		return nil, fmt.Errorf("ENVIRONMENT is required")
	}
	otlpEndpoint := os.Getenv("OTLP_ENDPOINT")
	if otlpEndpoint == "" {
		return nil, fmt.Errorf("OTLP_ENDPOINT is required")
	}

	// Provider encryption key
	providerEncryptionKey := os.Getenv("PROVIDER_ENCRYPTION_KEY")
	if providerEncryptionKey == "" {
		return nil, fmt.Errorf("PROVIDER_ENCRYPTION_KEY is required")
	}
	decodedKey, err := hex.DecodeString(providerEncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("PROVIDER_ENCRYPTION_KEY must be hex-encoded: %w", err)
	}
	if len(decodedKey) != 32 {
		return nil, fmt.Errorf("PROVIDER_ENCRYPTION_KEY must decode to exactly 32 bytes, got %d", len(decodedKey))
	}

	return &Config{
		DatabaseURL:           dbURL,
		APIHost:               apiHost,
		APIPort:               apiPort,
		CORSAllowedOrigins:    origins,
		LogLevel:              logLevel,
		JWTSecret:             jwtSecret,
		JWTExpirationHours:    jwtExpirationHours,
		AccessTokenTTL:        accessTokenTTL,
		RefreshTokenTTL:       refreshTokenTTL,
		JWTAudience:           jwtAudience,
		JWTIssuer:             jwtIssuer,
		RateLimitPerIP:        rateLimitPerIP,
		RateLimitPerEmail:     rateLimitPerEmail,
		RateLimitWindow:       rateLimitWindow,
		LoginLockoutThreshold: loginLockoutThreshold,
		LoginLockoutDuration:  loginLockoutDuration,
		PasswordMinLength:     passwordMinLength,
		PasswordRequireUpper:  passwordRequireUpper,
		PasswordRequireLower:  passwordRequireLower,
		PasswordRequireNumber: passwordRequireNumber,
		PasswordRequireSymbol: passwordRequireSymbol,
		EmailTokenTTL:         emailTokenTTL,
		ResetTokenTTL:         resetTokenTTL,
		DeploymentMode:        deploymentMode,
		BaseURL:               baseURL,
		NATSURL:               natsURL,
		Environment:           environment,
		OTLPEndpoint:          otlpEndpoint,
		ProviderEncryptionKey: providerEncryptionKey,
	}, nil
}

// getDatabaseURL constructs the database URL from environment variables.
func getDatabaseURL() (string, error) {
	// Check if DATABASE_URL is provided directly
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		return dbURL, nil
	}

	// Build from individual POSTGRES_* variables
	host := os.Getenv("POSTGRES_HOST")
	port := os.Getenv("POSTGRES_PORT")
	user := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")
	db := os.Getenv("POSTGRES_DB")
	sslmode := os.Getenv("POSTGRES_SSLMODE")
	if sslmode == "" {
		sslmode = "disable"
	}

	missing := []string{}
	if host == "" {
		missing = append(missing, "POSTGRES_HOST")
	}
	if port == "" {
		missing = append(missing, "POSTGRES_PORT")
	}
	if user == "" {
		missing = append(missing, "POSTGRES_USER")
	}
	if password == "" {
		missing = append(missing, "POSTGRES_PASSWORD")
	}
	if db == "" {
		missing = append(missing, "POSTGRES_DB")
	}

	if len(missing) > 0 {
		return "", fmt.Errorf("missing required environment variables: %s (or provide DATABASE_URL)", strings.Join(missing, ", "))
	}

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, password, host, port, db, sslmode), nil
}

// getEnvString returns the environment variable or default value.
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt returns the environment variable as an int or default value.
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getEnvBool returns the environment variable as a bool or default value.
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		lower := strings.ToLower(value)
		if lower == "true" || lower == "1" || lower == "yes" {
			return true
		}
		if lower == "false" || lower == "0" || lower == "no" {
			return false
		}
	}
	return defaultValue
}

// getEnvDuration returns the environment variable as a duration or default value.
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// parseStringList parses a comma-separated string into a slice of trimmed strings.
func parseStringList(s string) []string {
	var result []string
	for _, item := range strings.Split(s, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// isValidLogLevel checks if the log level is valid.
func isValidLogLevel(level string) bool {
	switch strings.ToLower(level) {
	case "debug", "info", "warn", "warning", "error":
		return true
	}
	return false
}
