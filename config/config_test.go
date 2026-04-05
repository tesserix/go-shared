package config

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tesserix/go-shared/database"
)

// --- Validate() tests ---

func TestValidate_PassesForValidDevelopmentConfig(t *testing.T) {
	cfg := validDevelopmentConfig()
	assert.NoError(t, cfg.Validate())
}

func TestValidate_PassesForValidStagingConfig(t *testing.T) {
	cfg := validDevelopmentConfig()
	cfg.Environment = "staging"
	assert.NoError(t, cfg.Validate())
}

func TestValidate_PassesForValidProductionConfig(t *testing.T) {
	cfg := validProductionConfig()
	assert.NoError(t, cfg.Validate())
}

func TestValidate_MissingServiceName(t *testing.T) {
	cfg := validDevelopmentConfig()
	cfg.ServiceName = ""

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SERVICE_NAME is required")
}

func TestValidate_MissingDBHost(t *testing.T) {
	cfg := validDevelopmentConfig()
	cfg.Database.Host = ""

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DB_HOST is required")
}

func TestValidate_MissingDBUser(t *testing.T) {
	cfg := validDevelopmentConfig()
	cfg.Database.User = ""

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DB_USER is required")
}

func TestValidate_MissingDBName(t *testing.T) {
	cfg := validDevelopmentConfig()
	cfg.Database.DBName = ""

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DB_NAME is required")
}

func TestValidate_InvalidEnvironment(t *testing.T) {
	tests := []struct {
		name string
		env  string
	}{
		{"empty string", ""},
		{"typo", "producton"},
		{"uppercase", "Development"},
		{"arbitrary value", "local"},
		{"dev shorthand", "dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validDevelopmentConfig()
			cfg.Environment = tt.env

			err := cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "ENVIRONMENT must be one of")
		})
	}
}

func TestValidate_AccumulatesMultipleErrors(t *testing.T) {
	// A config that fails several validations at once.
	cfg := &AppConfig{
		// ServiceName intentionally blank
		Environment: "development",
		Database: database.Config{
			// Host and User and DBName intentionally blank
		},
	}

	err := cfg.Validate()
	require.Error(t, err)

	errMsg := err.Error()
	assert.Contains(t, errMsg, "SERVICE_NAME is required")
	assert.Contains(t, errMsg, "DB_HOST is required")
	assert.Contains(t, errMsg, "DB_USER is required")
	assert.Contains(t, errMsg, "DB_NAME is required")
}

// --- Production-specific Validate() tests ---

func TestValidate_Production_RequiresJWTSecret(t *testing.T) {
	tests := []struct {
		name      string
		jwtSecret string
	}{
		{"empty JWT secret", ""},
		{"default placeholder secret", "default-secret-change-in-production"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validProductionConfig()
			cfg.JWTSecret = tt.jwtSecret

			err := cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "JWT_SECRET must be set in production")
		})
	}
}

func TestValidate_NonProduction_AllowsDefaultJWTSecret(t *testing.T) {
	for _, env := range []string{"development", "staging"} {
		t.Run(env, func(t *testing.T) {
			cfg := validDevelopmentConfig()
			cfg.Environment = env
			cfg.JWTSecret = "default-secret-change-in-production"

			assert.NoError(t, cfg.Validate())
		})
	}
}

func TestValidate_Production_RequiresHTTPS(t *testing.T) {
	cfg := validProductionConfig()
	cfg.RequireHTTPS = false

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "REQUIRE_HTTPS must be true in production")
}

func TestValidate_Production_RejectsWildcardCORS(t *testing.T) {
	cfg := validProductionConfig()
	cfg.CORSAllowedOrigins = []string{"*"}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CORS_ALLOWED_ORIGINS should not be '*' in production")
}

func TestValidate_Production_AllowsMultipleCORSOrigins(t *testing.T) {
	// Even if one of the origins happens to be "*" among multiple entries the
	// current implementation only checks the single-entry wildcard case.
	// More importantly, a legitimate multi-origin list must pass validation.
	cfg := validProductionConfig()
	cfg.CORSAllowedOrigins = []string{
		"https://app.example.com",
		"https://admin.example.com",
	}

	assert.NoError(t, cfg.Validate())
}

func TestValidate_Production_RejectsAllThreeProductionRules(t *testing.T) {
	// Combine all three production failures to verify they all surface.
	cfg := &AppConfig{
		ServiceName: "my-service",
		Environment: "production",
		JWTSecret:   "", // empty → JWT error
		RequireHTTPS: false,
		CORSAllowedOrigins: []string{"*"},
		Database: database.Config{
			Host:   "db.example.com",
			User:   "admin",
			DBName: "mydb",
		},
	}

	err := cfg.Validate()
	require.Error(t, err)

	errMsg := err.Error()
	assert.Contains(t, errMsg, "JWT_SECRET must be set in production")
	assert.Contains(t, errMsg, "REQUIRE_HTTPS must be true in production")
	assert.Contains(t, errMsg, "CORS_ALLOWED_ORIGINS should not be '*' in production")
}

// --- IsProduction / IsDevelopment / IsStaging ---

func TestEnvironmentHelpers(t *testing.T) {
	tests := []struct {
		env          string
		isProduction bool
		isDevelopment bool
		isStaging    bool
	}{
		{"production", true, false, false},
		{"development", false, true, false},
		{"staging", false, false, true},
		{"unknown", false, false, false},
		{"", false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			cfg := &AppConfig{Environment: tt.env}

			assert.Equal(t, tt.isProduction, cfg.IsProduction(), "IsProduction()")
			assert.Equal(t, tt.isDevelopment, cfg.IsDevelopment(), "IsDevelopment()")
			assert.Equal(t, tt.isStaging, cfg.IsStaging(), "IsStaging()")
		})
	}
}

// --- Load() reads from env vars ---

func TestLoad_DefaultValues(t *testing.T) {
	// Ensure none of the relevant env vars are set so we exercise defaults.
	// t.Setenv handles cleanup automatically.
	t.Setenv("PORT", "")
	t.Setenv("ENVIRONMENT", "")
	t.Setenv("SERVICE_NAME", "")
	t.Setenv("SERVICE_VERSION", "")
	t.Setenv("DB_HOST", "")
	t.Setenv("DB_PORT", "")
	t.Setenv("DB_USER", "")
	t.Setenv("DB_NAME", "")
	t.Setenv("DB_SSL_MODE", "")
	t.Setenv("DB_MAX_OPEN_CONNS", "")
	t.Setenv("DB_MAX_IDLE_CONNS", "")
	t.Setenv("DB_CONN_MAX_LIFETIME", "")
	t.Setenv("DB_CONN_MAX_IDLE_TIME", "")
	t.Setenv("JWT_EXPIRATION", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	t.Setenv("DOCUMENT_SERVICE_URL", "")
	t.Setenv("AUTH_SERVICE_URL", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("METRICS_ENABLED", "")
	t.Setenv("TRACING_ENABLED", "")
	t.Setenv("REQUIRE_HTTPS", "")
	// Make sure GCP Secret Manager is disabled so Load() uses env vars.
	t.Setenv("USE_GCP_SECRET_MANAGER", "")

	cfg := Load()

	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "development", cfg.Environment)
	assert.Equal(t, "unknown-service", cfg.ServiceName)
	assert.Equal(t, "1.0.0", cfg.Version)

	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "postgres", cfg.Database.User)
	assert.Equal(t, "testdb", cfg.Database.DBName)
	assert.Equal(t, "disable", cfg.Database.SSLMode)
	assert.Equal(t, 5, cfg.Database.MaxOpenConns)
	assert.Equal(t, 2, cfg.Database.MaxIdleConns)
	assert.Equal(t, 5*time.Minute, cfg.Database.ConnMaxLifetime)
	assert.Equal(t, 5*time.Minute, cfg.Database.ConnMaxIdleTime)

	assert.Equal(t, 15*time.Minute, cfg.JWTExpiration)

	assert.Equal(t, []string{"*"}, cfg.CORSAllowedOrigins)

	assert.Equal(t, "", cfg.DocumentServiceURL)
	assert.Equal(t, "", cfg.AuthServiceURL)

	assert.Equal(t, "info", cfg.LogLevel)
	assert.True(t, cfg.MetricsEnabled)
	assert.False(t, cfg.TracingEnabled)
	assert.False(t, cfg.RequireHTTPS)
}

func TestLoad_ReadsStringEnvVars(t *testing.T) {
	t.Setenv("USE_GCP_SECRET_MANAGER", "")
	t.Setenv("PORT", "9090")
	t.Setenv("ENVIRONMENT", "staging")
	t.Setenv("SERVICE_NAME", "order-service")
	t.Setenv("SERVICE_VERSION", "2.3.1")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("DOCUMENT_SERVICE_URL", "http://docs:9000")
	t.Setenv("AUTH_SERVICE_URL", "http://auth:9001")

	cfg := Load()

	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "staging", cfg.Environment)
	assert.Equal(t, "order-service", cfg.ServiceName)
	assert.Equal(t, "2.3.1", cfg.Version)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "http://docs:9000", cfg.DocumentServiceURL)
	assert.Equal(t, "http://auth:9001", cfg.AuthServiceURL)
}

func TestLoad_ReadsDatabaseEnvVars(t *testing.T) {
	t.Setenv("USE_GCP_SECRET_MANAGER", "")
	t.Setenv("DB_HOST", "postgres.internal")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("DB_USER", "myuser")
	t.Setenv("DB_PASSWORD", "mysecret")
	t.Setenv("DB_NAME", "myapp")
	t.Setenv("DB_SSL_MODE", "require")
	t.Setenv("DB_MAX_OPEN_CONNS", "50")
	t.Setenv("DB_MAX_IDLE_CONNS", "10")
	t.Setenv("DB_CONN_MAX_LIFETIME", "10m")
	t.Setenv("DB_CONN_MAX_IDLE_TIME", "3m")

	cfg := Load()

	assert.Equal(t, "postgres.internal", cfg.Database.Host)
	assert.Equal(t, 5433, cfg.Database.Port)
	assert.Equal(t, "myuser", cfg.Database.User)
	assert.Equal(t, "mysecret", cfg.Database.Password)
	assert.Equal(t, "myapp", cfg.Database.DBName)
	assert.Equal(t, "require", cfg.Database.SSLMode)
	assert.Equal(t, 50, cfg.Database.MaxOpenConns)
	assert.Equal(t, 10, cfg.Database.MaxIdleConns)
	assert.Equal(t, 10*time.Minute, cfg.Database.ConnMaxLifetime)
	assert.Equal(t, 3*time.Minute, cfg.Database.ConnMaxIdleTime)
}

func TestLoad_ReadsBooleanEnvVars(t *testing.T) {
	tests := []struct {
		name             string
		metricsEnabled   string
		tracingEnabled   string
		requireHTTPS     string
		wantMetrics      bool
		wantTracing      bool
		wantRequireHTTPS bool
	}{
		{
			name:             "all true",
			metricsEnabled:   "true",
			tracingEnabled:   "true",
			requireHTTPS:     "true",
			wantMetrics:      true,
			wantTracing:      true,
			wantRequireHTTPS: true,
		},
		{
			name:             "all false",
			metricsEnabled:   "false",
			tracingEnabled:   "false",
			requireHTTPS:     "false",
			wantMetrics:      false,
			wantTracing:      false,
			wantRequireHTTPS: false,
		},
		{
			name:             "numeric true (1)",
			metricsEnabled:   "1",
			tracingEnabled:   "1",
			requireHTTPS:     "1",
			wantMetrics:      true,
			wantTracing:      true,
			wantRequireHTTPS: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("USE_GCP_SECRET_MANAGER", "")
			t.Setenv("METRICS_ENABLED", tt.metricsEnabled)
			t.Setenv("TRACING_ENABLED", tt.tracingEnabled)
			t.Setenv("REQUIRE_HTTPS", tt.requireHTTPS)

			cfg := Load()

			assert.Equal(t, tt.wantMetrics, cfg.MetricsEnabled, "MetricsEnabled")
			assert.Equal(t, tt.wantTracing, cfg.TracingEnabled, "TracingEnabled")
			assert.Equal(t, tt.wantRequireHTTPS, cfg.RequireHTTPS, "RequireHTTPS")
		})
	}
}

func TestLoad_ReadsDurationEnvVar(t *testing.T) {
	t.Setenv("USE_GCP_SECRET_MANAGER", "")
	t.Setenv("JWT_EXPIRATION", "30m")

	cfg := Load()

	assert.Equal(t, 30*time.Minute, cfg.JWTExpiration)
}

func TestLoad_ReadsCORSOriginsFromEnvVar(t *testing.T) {
	t.Setenv("USE_GCP_SECRET_MANAGER", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com,https://admin.example.com")

	cfg := Load()

	require.Len(t, cfg.CORSAllowedOrigins, 2)
	assert.Equal(t, "https://app.example.com", cfg.CORSAllowedOrigins[0])
	assert.Equal(t, "https://admin.example.com", cfg.CORSAllowedOrigins[1])
}

func TestLoad_InvalidIntFallsBackToDefault(t *testing.T) {
	t.Setenv("USE_GCP_SECRET_MANAGER", "")
	t.Setenv("DB_PORT", "not-a-number")

	cfg := Load()

	// Should fall back to the default port of 5432.
	assert.Equal(t, 5432, cfg.Database.Port)
}

func TestLoad_InvalidBoolFallsBackToDefault(t *testing.T) {
	t.Setenv("USE_GCP_SECRET_MANAGER", "")
	t.Setenv("METRICS_ENABLED", "not-a-bool")

	cfg := Load()

	// METRICS_ENABLED default is true.
	assert.True(t, cfg.MetricsEnabled)
}

func TestLoad_InvalidDurationFallsBackToDefault(t *testing.T) {
	t.Setenv("USE_GCP_SECRET_MANAGER", "")
	t.Setenv("JWT_EXPIRATION", "not-a-duration")

	cfg := Load()

	assert.Equal(t, 15*time.Minute, cfg.JWTExpiration)
}

// --- Validate() error message format ---

func TestValidate_ErrorMessageFormat(t *testing.T) {
	// The error message must start with the known prefix.
	cfg := validDevelopmentConfig()
	cfg.ServiceName = ""

	err := cfg.Validate()
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "configuration validation failed:"),
		"error should start with 'configuration validation failed:', got: %q", err.Error())
}

// --- Validate() passes with an empty JWTSecret in non-prod ---

func TestValidate_AllowsEmptyJWTSecretInDevelopment(t *testing.T) {
	cfg := validDevelopmentConfig()
	cfg.JWTSecret = ""

	// Empty secret is only an error in production, not in development.
	assert.NoError(t, cfg.Validate())
}

// --- helpers ---

// validDevelopmentConfig returns a fully valid development AppConfig.
func validDevelopmentConfig() *AppConfig {
	return &AppConfig{
		Port:        "8080",
		Environment: "development",
		ServiceName: "test-service",
		Version:     "1.0.0",
		Database: database.Config{
			Host:   "localhost",
			Port:   5432,
			User:   "postgres",
			DBName: "testdb",
		},
		JWTSecret:          "any-secret-is-fine-in-dev",
		JWTExpiration:      15 * time.Minute,
		CORSAllowedOrigins: []string{"*"},
		LogLevel:           "info",
		MetricsEnabled:     true,
	}
}

// validProductionConfig returns a fully valid production AppConfig.
func validProductionConfig() *AppConfig {
	return &AppConfig{
		Port:        "8080",
		Environment: "production",
		ServiceName: "test-service",
		Version:     "1.0.0",
		Database: database.Config{
			Host:   "db.production.internal",
			Port:   5432,
			User:   "appuser",
			DBName: "appdb",
		},
		JWTSecret:     "super-long-production-secret-key-that-is-not-the-default",
		JWTExpiration: 15 * time.Minute,
		CORSAllowedOrigins: []string{
			"https://app.tesserix.app",
			"https://admin.tesserix.app",
		},
		RequireHTTPS:   true,
		LogLevel:       "warn",
		MetricsEnabled: true,
	}
}
