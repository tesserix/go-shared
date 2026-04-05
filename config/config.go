package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/tesserix/go-shared/database"
	"github.com/tesserix/go-shared/secrets"
)

// AppConfig holds the application configuration
type AppConfig struct {
	// Server configuration
	Port        string
	Environment string
	ServiceName string
	Version     string

	// Database configuration
	Database database.Config

	// JWT configuration
	JWTSecret     string
	JWTExpiration time.Duration

	// CORS configuration
	CORSAllowedOrigins []string

	// External services
	DocumentServiceURL string
	AuthServiceURL     string

	// Observability
	LogLevel        string
	MetricsEnabled  bool
	TracingEnabled  bool

	// Security
	RequireHTTPS bool
}

// Load loads configuration from environment variables
func Load() *AppConfig {
	// Load .env file if exists (ignore errors)
	godotenv.Load()

	// Try to initialize GCP Secret Manager for secrets
	ctx := context.Background()
	secretFetcher, err := secrets.NewEnvSecretFetcher(ctx)
	if err != nil {
		log.Printf("Warning: Failed to initialize GCP Secret Manager: %v (using env vars)", err)
	}
	defer func() {
		if secretFetcher != nil {
			secretFetcher.Close()
		}
	}()

	// Get database password from GCP Secret Manager or env var
	dbPassword := getEnv("DB_PASSWORD", "password")
	if secretFetcher != nil {
		dbPassword = secrets.LoadDatabasePassword(ctx, secretFetcher)
	}

	// Get JWT secret from GCP Secret Manager or env var
	jwtSecret := getEnv("JWT_SECRET", "default-secret-change-in-production")
	if secretFetcher != nil {
		jwtSecret = secrets.LoadJWTSecret(ctx, secretFetcher)
	}

	return &AppConfig{
		// Server
		Port:        getEnv("PORT", "8080"),
		Environment: getEnv("ENVIRONMENT", "development"),
		ServiceName: getEnv("SERVICE_NAME", "unknown-service"),
		Version:     getEnv("SERVICE_VERSION", "1.0.0"),

		// Database
		Database: database.Config{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnvAsInt("DB_PORT", 5432),
			User:            getEnv("DB_USER", "postgres"),
			Password:        dbPassword,
			DBName:          getEnv("DB_NAME", "testdb"),
			SSLMode:         getEnv("DB_SSL_MODE", "disable"),
			MaxOpenConns:    getEnvAsInt("DB_MAX_OPEN_CONNS", 5),
			MaxIdleConns:    getEnvAsInt("DB_MAX_IDLE_CONNS", 2),
			ConnMaxLifetime: getEnvAsDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
			ConnMaxIdleTime: getEnvAsDuration("DB_CONN_MAX_IDLE_TIME", 5*time.Minute),
		},

		// JWT
		JWTSecret:     jwtSecret,
		JWTExpiration: getEnvAsDuration("JWT_EXPIRATION", 15*time.Minute),

		// CORS
		CORSAllowedOrigins: getEnvAsSlice("CORS_ALLOWED_ORIGINS", []string{"*"}),

		// External services
		DocumentServiceURL: getEnv("DOCUMENT_SERVICE_URL", ""),
		AuthServiceURL:     getEnv("AUTH_SERVICE_URL", ""),

		// Observability
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		MetricsEnabled: getEnvAsBool("METRICS_ENABLED", true),
		TracingEnabled: getEnvAsBool("TRACING_ENABLED", false),

		// Security
		RequireHTTPS: getEnvAsBool("REQUIRE_HTTPS", false),
	}
}

// LoadWithContext loads configuration with a custom context for secret fetching
func LoadWithContext(ctx context.Context) *AppConfig {
	// Load .env file if exists (ignore errors)
	godotenv.Load()

	// Try to initialize GCP Secret Manager for secrets
	secretFetcher, err := secrets.NewEnvSecretFetcher(ctx)
	if err != nil {
		log.Printf("Warning: Failed to initialize GCP Secret Manager: %v (using env vars)", err)
	}
	defer func() {
		if secretFetcher != nil {
			secretFetcher.Close()
		}
	}()

	// Get database password from GCP Secret Manager or env var
	dbPassword := getEnv("DB_PASSWORD", "password")
	if secretFetcher != nil {
		dbPassword = secrets.LoadDatabasePassword(ctx, secretFetcher)
	}

	// Get JWT secret from GCP Secret Manager or env var
	jwtSecret := getEnv("JWT_SECRET", "default-secret-change-in-production")
	if secretFetcher != nil {
		jwtSecret = secrets.LoadJWTSecret(ctx, secretFetcher)
	}

	return &AppConfig{
		// Server
		Port:        getEnv("PORT", "8080"),
		Environment: getEnv("ENVIRONMENT", "development"),
		ServiceName: getEnv("SERVICE_NAME", "unknown-service"),
		Version:     getEnv("SERVICE_VERSION", "1.0.0"),

		// Database
		Database: database.Config{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnvAsInt("DB_PORT", 5432),
			User:            getEnv("DB_USER", "postgres"),
			Password:        dbPassword,
			DBName:          getEnv("DB_NAME", "testdb"),
			SSLMode:         getEnv("DB_SSL_MODE", "disable"),
			MaxOpenConns:    getEnvAsInt("DB_MAX_OPEN_CONNS", 5),
			MaxIdleConns:    getEnvAsInt("DB_MAX_IDLE_CONNS", 2),
			ConnMaxLifetime: getEnvAsDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
			ConnMaxIdleTime: getEnvAsDuration("DB_CONN_MAX_IDLE_TIME", 5*time.Minute),
		},

		// JWT
		JWTSecret:     jwtSecret,
		JWTExpiration: getEnvAsDuration("JWT_EXPIRATION", 15*time.Minute),

		// CORS
		CORSAllowedOrigins: getEnvAsSlice("CORS_ALLOWED_ORIGINS", []string{"*"}),

		// External services
		DocumentServiceURL: getEnv("DOCUMENT_SERVICE_URL", ""),
		AuthServiceURL:     getEnv("AUTH_SERVICE_URL", ""),

		// Observability
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		MetricsEnabled: getEnvAsBool("METRICS_ENABLED", true),
		TracingEnabled: getEnvAsBool("TRACING_ENABLED", false),

		// Security
		RequireHTTPS: getEnvAsBool("REQUIRE_HTTPS", false),
	}
}

// Validate validates the configuration
func (c *AppConfig) Validate() error {
	var errors []string

	// Validate required fields
	if c.ServiceName == "" {
		errors = append(errors, "SERVICE_NAME is required")
	}

	if c.JWTSecret == "" || c.JWTSecret == "default-secret-change-in-production" {
		if c.Environment == "production" {
			errors = append(errors, "JWT_SECRET must be set in production")
		}
	}

	if c.Database.Host == "" {
		errors = append(errors, "DB_HOST is required")
	}

	if c.Database.User == "" {
		errors = append(errors, "DB_USER is required")
	}

	if c.Database.DBName == "" {
		errors = append(errors, "DB_NAME is required")
	}

	// Validate environment
	validEnvs := []string{"development", "staging", "production"}
	isValidEnv := false
	for _, env := range validEnvs {
		if c.Environment == env {
			isValidEnv = true
			break
		}
	}
	if !isValidEnv {
		errors = append(errors, fmt.Sprintf("ENVIRONMENT must be one of: %s", strings.Join(validEnvs, ", ")))
	}

	// Production-specific validations
	if c.Environment == "production" {
		if !c.RequireHTTPS {
			errors = append(errors, "REQUIRE_HTTPS must be true in production")
		}

		if len(c.CORSAllowedOrigins) == 1 && c.CORSAllowedOrigins[0] == "*" {
			errors = append(errors, "CORS_ALLOWED_ORIGINS should not be '*' in production")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed: %s", strings.Join(errors, ", "))
	}

	return nil
}

// IsProduction checks if the environment is production
func (c *AppConfig) IsProduction() bool {
	return c.Environment == "production"
}

// IsDevelopment checks if the environment is development
func (c *AppConfig) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsStaging checks if the environment is staging
func (c *AppConfig) IsStaging() bool {
	return c.Environment == "staging"
}

// Helper functions

// getEnv gets an environment variable with a fallback value
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// getEnvAsInt gets an environment variable as an integer with a fallback
func getEnvAsInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return fallback
}

// getEnvAsBool gets an environment variable as a boolean with a fallback
func getEnvAsBool(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return fallback
}

// getEnvAsDuration gets an environment variable as a duration with a fallback
func getEnvAsDuration(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return fallback
}

// getEnvAsSlice gets an environment variable as a slice with a fallback
func getEnvAsSlice(key string, fallback []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return fallback
}