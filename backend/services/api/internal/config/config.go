// Package config provides configuration management for the API service.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds all configuration for the API service.
type Config struct {
	DatabaseURL        string
	APIPort            string
	CORSAllowedOrigins []string
}

// Load loads configuration from environment variables.
func Load() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		host := os.Getenv("POSTGRES_HOST")
		port := os.Getenv("POSTGRES_PORT")
		user := os.Getenv("POSTGRES_USER")
		pass := os.Getenv("POSTGRES_PASSWORD")
		db := os.Getenv("POSTGRES_DB")

		if host == "" || port == "" || user == "" || pass == "" || db == "" {
			return nil, fmt.Errorf("DATABASE_URL or all POSTGRES_* variables (HOST, PORT, USER, PASSWORD, DB) are required")
		}

		dbURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, db)
	}

	apiPort := os.Getenv("API_PORT")
	if apiPort == "" {
		return nil, fmt.Errorf("API_PORT is required")
	}

	cors := os.Getenv("CORS_ALLOWED_ORIGINS")
	if cors == "" {
		return nil, fmt.Errorf("CORS_ALLOWED_ORIGINS is required")
	}

	var origins []string
	for _, item := range strings.Split(cors, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}

	return &Config{
		DatabaseURL:        dbURL,
		APIPort:            apiPort,
		CORSAllowedOrigins: origins,
	}, nil
}
