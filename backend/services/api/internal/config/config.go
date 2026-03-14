// Package config provides configuration management for the API service.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration for the API service.
type Config struct {
	// Database configuration
	DatabaseURL         string
	DatabaseMaxConns    int32
	DatabaseMinConns    int32
	DatabaseMaxConnLifetime int
	DatabaseMaxConnIdleTime int

	// API configuration
	APIHost            string
	APIPort            string

	// CORS configuration
	CORSAllowedOrigins []string

	// Logging configuration
	LogLevel           string

	// JWT configuration
	JWTSecret          string
	JWTExpirationHours int
}

// Load loads configuration from environment variables.
func Load() (*Config, error) {
	// Database configuration
	dbURL, err := getDatabaseURL()
	if err != nil {
		return nil, fmt.Errorf("database configuration: %w", err)
	}

	maxConns := getEnvInt("DATABASE_MAX_CONNS", 25)
	minConns := getEnvInt("DATABASE_MIN_CONNS", 5)
	maxConnLifetime := getEnvInt("DATABASE_MAX_CONN_LIFETIME", 3600) // 1 hour
	maxConnIdleTime := getEnvInt("DATABASE_MAX_CONN_IDLE_TIME", 1800) // 30 minutes

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

	// JWT configuration
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if len(jwtSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	jwtExpirationHours := getEnvInt("JWT_EXPIRATION_HOURS", 24)

	return &Config{
		DatabaseURL:            dbURL,
		DatabaseMaxConns:       maxConns,
		DatabaseMinConns:       minConns,
		DatabaseMaxConnLifetime: maxConnLifetime,
		DatabaseMaxConnIdleTime: maxConnIdleTime,
		APIHost:                apiHost,
		APIPort:                apiPort,
		CORSAllowedOrigins:     origins,
		LogLevel:               logLevel,
		JWTSecret:              jwtSecret,
		JWTExpirationHours:    jwtExpirationHours,
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
	sslmode := os.Getenv("POSTGRES_SSLMODE", "disable")

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
