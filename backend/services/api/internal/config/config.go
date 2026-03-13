// Package config provides configuration management for the API service.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the API service.
type Config struct {
	Database DatabaseConfig
	API      APIConfig
}

// DatabaseConfig holds PostgreSQL connection configuration.
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DB       string
	// Connection pool settings
	MaxConns           int32
	MinConns           int32
	MaxConnLifetime    int // in seconds
	MaxConnIdleTime    int // in seconds
}

// APIConfig holds API server configuration.
type APIConfig struct {
	Port     string
	LogLevel string
}

// Load loads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		Database: DatabaseConfig{
			Host:            getEnv("POSTGRES_HOST", "localhost"),
			Port:            getEnvAsInt("POSTGRES_PORT", 5432),
			User:            getEnv("POSTGRES_USER", "postgres"),
			Password:        getEnv("POSTGRES_PASSWORD", ""),
			DB:              getEnv("POSTGRES_DB", "ace"),
			MaxConns:        getEnvAsInt32("POSTGRES_MAX_CONNS", 25),
			MinConns:        getEnvAsInt32("POSTGRES_MIN_CONNS", 5),
			MaxConnLifetime: getEnvAsInt("POSTGRES_MAX_CONN_LIFETIME", 3600),
			MaxConnIdleTime: getEnvAsInt("POSTGRES_MAX_CONN_IDLE_TIME", 1800),
		},
		API: APIConfig{
			Port:     getEnv("API_PORT", "8080"),
			LogLevel: getEnv("LOG_LEVEL", "info"),
		},
	}

	// Validate required fields
	if cfg.Database.Host == "" {
		return nil, fmt.Errorf("POSTGRES_HOST is required")
	}
	if cfg.Database.User == "" {
		return nil, fmt.Errorf("POSTGRES_USER is required")
	}
	if cfg.Database.Password == "" {
		return nil, fmt.Errorf("POSTGRES_PASSWORD is required")
	}
	if cfg.Database.DB == "" {
		return nil, fmt.Errorf("POSTGRES_DB is required")
	}

	return cfg, nil
}

// DSN returns the PostgreSQL connection string.
func (d *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		d.User,
		d.Password,
		d.Host,
		d.Port,
		d.DB,
	)
}

// getEnv retrieves an environment variable or returns a default value.
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getEnvAsInt retrieves an environment variable as an integer or returns a default value.
func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvAsInt32 retrieves an environment variable as an int32 or returns a default value.
func getEnvAsInt32(key string, defaultValue int32) int32 {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return int32(intValue)
		}
	}
	return defaultValue
}
