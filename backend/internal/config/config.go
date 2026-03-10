package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	JWT        JWTConfig
	Log        LogConfig
	LLM        LLMConfig
	NATS       NATSConfig
	Telemetry TelemetryConfig
}

type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type JWTConfig struct {
	Secret          string
	AccessExpiry    time.Duration
	RefreshExpiry   time.Duration
}

type LogConfig struct {
	Level  string
	Format string
}

type LLMConfig struct {
	Provider      string
	APIKey        string
	BaseURL       string
	DefaultModel  string
	MaxRetries    int
	Timeout       int
}

type NATSConfig struct {
	URL         string
	UseInMemory bool
}

type TelemetryConfig struct {
	Enabled        bool
	Endpoint       string
	ServiceName    string
	ServiceVersion string
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8080"),
			ReadTimeout:  getDurationEnv("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getDurationEnv("SERVER_WRITE_TIMEOUT", 15*time.Second),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DATABASE_HOST", "localhost"),
			Port:     getIntEnv("DATABASE_PORT", 5432),
			User:     getEnv("DATABASE_USER", "ace"),
			Password: getEnv("DATABASE_PASSWORD", "ace"),
			DBName:   getEnv("DATABASE_NAME", "ace_framework"),
			SSLMode:  getEnv("DATABASE_SSLMODE", "disable"),
		},
		JWT: JWTConfig{
			Secret:        getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
			AccessExpiry:  getDurationEnv("JWT_ACCESS_EXPIRY", 15*time.Minute),
			RefreshExpiry: getDurationEnv("JWT_REFRESH_EXPIRY", 7*24*time.Hour),
		},
		Log: LogConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
		LLM: LLMConfig{
			Provider:      getEnv("LLM_PROVIDER", "openrouter"),
			APIKey:        getEnv("OPENROUTER_API_KEY", ""),
			BaseURL:       getEnv("LLM_BASE_URL", "https://openrouter.ai/api/v1"),
			DefaultModel:  getEnv("LLM_DEFAULT_MODEL", "openrouter/free"),
			MaxRetries:    getIntEnv("LLM_MAX_RETRIES", 3),
			Timeout:       getIntEnv("LLM_TIMEOUT", 120),
		},
		NATS: NATSConfig{
			URL:         getEnv("NATS_URL", "nats://localhost:4222"),
			UseInMemory: getEnv("NATS_USE_IN_MEMORY", "true") == "true",
		},
		Telemetry: TelemetryConfig{
			Enabled:        getEnv("TELEMETRY_ENABLED", "true") == "true",
			Endpoint:       getEnv("TELEMETRY_ENDPOINT", "localhost:4317"),
			ServiceName:    getEnv("TELEMETRY_SERVICE_NAME", "ace-framework"),
			ServiceVersion: getEnv("TELEMETRY_SERVICE_VERSION", "1.0.0"),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
