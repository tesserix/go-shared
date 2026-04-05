package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "localhost", cfg.Host)
	assert.Equal(t, 5432, cfg.Port)
	assert.Equal(t, "postgres", cfg.User)
	assert.Equal(t, "password", cfg.Password)
	assert.Equal(t, "testdb", cfg.DBName)
	assert.Equal(t, "disable", cfg.SSLMode)
	assert.Equal(t, 5, cfg.MaxOpenConns)
	assert.Equal(t, 2, cfg.MaxIdleConns)
	assert.Equal(t, 5*time.Minute, cfg.ConnMaxLifetime)
	assert.Equal(t, 5*time.Minute, cfg.ConnMaxIdleTime)
}

func TestPoolProfileConstants(t *testing.T) {
	assert.Equal(t, PoolProfile("default"), PoolProfileDefault)
	assert.Equal(t, PoolProfile("high_throughput"), PoolProfileHighThroughput)
	assert.Equal(t, PoolProfile("low_latency"), PoolProfileLowLatency)
	assert.Equal(t, PoolProfile("background"), PoolProfileBackground)
}

func TestPoolProfileConfigs_AllProfilesPresent(t *testing.T) {
	profiles := PoolProfileConfigs()

	expectedProfiles := []PoolProfile{
		PoolProfileDefault,
		PoolProfileHighThroughput,
		PoolProfileLowLatency,
		PoolProfileBackground,
	}

	for _, profile := range expectedProfiles {
		_, ok := profiles[profile]
		assert.True(t, ok, "expected profile %q to be present in PoolProfileConfigs", profile)
	}
}

func TestPoolProfileConfigs_Values(t *testing.T) {
	profiles := PoolProfileConfigs()

	tests := []struct {
		name            string
		profile         PoolProfile
		maxOpenConns    int
		maxIdleConns    int
		connMaxLifetime time.Duration
		connMaxIdleTime time.Duration
	}{
		{
			name:            "high throughput profile",
			profile:         PoolProfileHighThroughput,
			maxOpenConns:    8,
			maxIdleConns:    4,
			connMaxLifetime: 5 * time.Minute,
			connMaxIdleTime: 5 * time.Minute,
		},
		{
			name:            "default profile",
			profile:         PoolProfileDefault,
			maxOpenConns:    5,
			maxIdleConns:    2,
			connMaxLifetime: 5 * time.Minute,
			connMaxIdleTime: 5 * time.Minute,
		},
		{
			name:            "low latency profile",
			profile:         PoolProfileLowLatency,
			maxOpenConns:    5,
			maxIdleConns:    3,
			connMaxLifetime: 5 * time.Minute,
			connMaxIdleTime: 5 * time.Minute,
		},
		{
			name:            "background profile",
			profile:         PoolProfileBackground,
			maxOpenConns:    3,
			maxIdleConns:    1,
			connMaxLifetime: 10 * time.Minute,
			connMaxIdleTime: 5 * time.Minute,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, ok := profiles[tc.profile]
			assert.True(t, ok)
			assert.Equal(t, tc.maxOpenConns, p.MaxOpenConns)
			assert.Equal(t, tc.maxIdleConns, p.MaxIdleConns)
			assert.Equal(t, tc.connMaxLifetime, p.ConnMaxLifetime)
			assert.Equal(t, tc.connMaxIdleTime, p.ConnMaxIdleTime)
		})
	}
}

func TestApplyPoolProfile(t *testing.T) {
	tests := []struct {
		name            string
		profile         PoolProfile
		maxOpenConns    int
		maxIdleConns    int
		connMaxLifetime time.Duration
		connMaxIdleTime time.Duration
	}{
		{
			name:            "apply high throughput",
			profile:         PoolProfileHighThroughput,
			maxOpenConns:    8,
			maxIdleConns:    4,
			connMaxLifetime: 5 * time.Minute,
			connMaxIdleTime: 5 * time.Minute,
		},
		{
			name:            "apply default",
			profile:         PoolProfileDefault,
			maxOpenConns:    5,
			maxIdleConns:    2,
			connMaxLifetime: 5 * time.Minute,
			connMaxIdleTime: 5 * time.Minute,
		},
		{
			name:            "apply low latency",
			profile:         PoolProfileLowLatency,
			maxOpenConns:    5,
			maxIdleConns:    3,
			connMaxLifetime: 5 * time.Minute,
			connMaxIdleTime: 5 * time.Minute,
		},
		{
			name:            "apply background",
			profile:         PoolProfileBackground,
			maxOpenConns:    3,
			maxIdleConns:    1,
			connMaxLifetime: 10 * time.Minute,
			connMaxIdleTime: 5 * time.Minute,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.ApplyPoolProfile(tc.profile)

			assert.Equal(t, tc.maxOpenConns, cfg.MaxOpenConns)
			assert.Equal(t, tc.maxIdleConns, cfg.MaxIdleConns)
			assert.Equal(t, tc.connMaxLifetime, cfg.ConnMaxLifetime)
			assert.Equal(t, tc.connMaxIdleTime, cfg.ConnMaxIdleTime)
		})
	}
}

func TestApplyPoolProfile_UnknownProfile(t *testing.T) {
	cfg := DefaultConfig()
	originalMaxOpen := cfg.MaxOpenConns
	originalMaxIdle := cfg.MaxIdleConns

	// Applying an unknown profile should leave the config unchanged.
	cfg.ApplyPoolProfile(PoolProfile("nonexistent"))

	assert.Equal(t, originalMaxOpen, cfg.MaxOpenConns)
	assert.Equal(t, originalMaxIdle, cfg.MaxIdleConns)
}

func TestConfigWithProfile(t *testing.T) {
	cfg := ConfigWithProfile(PoolProfileBackground)

	// Connection identity fields come from DefaultConfig.
	assert.Equal(t, "localhost", cfg.Host)
	assert.Equal(t, 5432, cfg.Port)

	// Pool settings come from the selected profile.
	profiles := PoolProfileConfigs()
	bg := profiles[PoolProfileBackground]
	assert.Equal(t, bg.MaxOpenConns, cfg.MaxOpenConns)
	assert.Equal(t, bg.MaxIdleConns, cfg.MaxIdleConns)
	assert.Equal(t, bg.ConnMaxLifetime, cfg.ConnMaxLifetime)
	assert.Equal(t, bg.ConnMaxIdleTime, cfg.ConnMaxIdleTime)
}

func TestNewPagination(t *testing.T) {
	tests := []struct {
		name           string
		page           int
		perPage        int
		expectedPage   int
		expectedPer    int
		expectedOffset int
	}{
		{
			name:           "normal values",
			page:           2,
			perPage:        20,
			expectedPage:   2,
			expectedPer:    20,
			expectedOffset: 20,
		},
		{
			name:           "page zero defaults to 1",
			page:           0,
			perPage:        10,
			expectedPage:   1,
			expectedPer:    10,
			expectedOffset: 0,
		},
		{
			name:           "negative page defaults to 1",
			page:           -5,
			perPage:        10,
			expectedPage:   1,
			expectedPer:    10,
			expectedOffset: 0,
		},
		{
			name:           "perPage zero defaults to 10",
			page:           1,
			perPage:        0,
			expectedPage:   1,
			expectedPer:    10,
			expectedOffset: 0,
		},
		{
			name:           "perPage above 100 clamped to 100",
			page:           1,
			perPage:        200,
			expectedPage:   1,
			expectedPer:    100,
			expectedOffset: 0,
		},
		{
			name:           "first page gives zero offset",
			page:           1,
			perPage:        25,
			expectedPage:   1,
			expectedPer:    25,
			expectedOffset: 0,
		},
		{
			name:           "third page offset calculation",
			page:           3,
			perPage:        15,
			expectedPage:   3,
			expectedPer:    15,
			expectedOffset: 30,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := NewPagination(tc.page, tc.perPage)

			assert.Equal(t, tc.expectedPage, p.Page)
			assert.Equal(t, tc.expectedPer, p.PerPage)
			assert.Equal(t, tc.expectedOffset, p.Offset)
		})
	}
}

func TestBuildPaginationQuery(t *testing.T) {
	tests := []struct {
		name      string
		page      int
		perPage   int
		baseQuery string
		expected  string
	}{
		{
			name:      "appends limit and offset",
			page:      1,
			perPage:   10,
			baseQuery: "SELECT * FROM products",
			expected:  "SELECT * FROM products LIMIT 10 OFFSET 0",
		},
		{
			name:      "second page offset",
			page:      2,
			perPage:   25,
			baseQuery: "SELECT id FROM orders WHERE status = 'open'",
			expected:  "SELECT id FROM orders WHERE status = 'open' LIMIT 25 OFFSET 25",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := NewPagination(tc.page, tc.perPage)
			result := p.BuildPaginationQuery(tc.baseQuery)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestCountQuery(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "wraps select in count subquery",
			input:    "SELECT * FROM users",
			expected: "SELECT COUNT(*) FROM (SELECT * FROM users) as count_query",
		},
		{
			name:     "wraps complex query",
			input:    "SELECT id, name FROM products WHERE active = true",
			expected: "SELECT COUNT(*) FROM (SELECT id, name FROM products WHERE active = true) as count_query",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CountQuery(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestHealthCheck_NilDB(t *testing.T) {
	err := HealthCheck(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database connection is nil")
}

func TestClose_NilDB(t *testing.T) {
	err := Close(nil)
	assert.NoError(t, err)
}

func TestHealthCheckGORM_NilDB(t *testing.T) {
	err := HealthCheckGORM(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database connection is nil")
}

func TestCloseGORM_NilDB(t *testing.T) {
	err := CloseGORM(nil)
	assert.NoError(t, err)
}
