package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

// Config holds database configuration
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string

	// Connection pool settings - sized for Cloud SQL db-f1-micro shared instance
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// PoolProfile represents different connection pool configurations for different service types
// All profiles are sized for Cloud SQL db-f1-micro (~25 total connections shared across services)
type PoolProfile string

const (
	// PoolProfileDefault - Standard services
	PoolProfileDefault PoolProfile = "default"
	// PoolProfileHighThroughput - Higher-traffic services like products, orders
	PoolProfileHighThroughput PoolProfile = "high_throughput"
	// PoolProfileLowLatency - Services requiring fast response times
	PoolProfileLowLatency PoolProfile = "low_latency"
	// PoolProfileBackground - Background jobs, analytics
	PoolProfileBackground PoolProfile = "background"
)

// DefaultConfig returns a default database configuration
// Conservative defaults for Cloud SQL db-f1-micro (~25 total connections across all services)
func DefaultConfig() Config {
	return Config{
		Host:            "localhost",
		Port:            5432,
		User:            "postgres",
		Password:        "password",
		DBName:          "testdb",
		SSLMode:         "disable",
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}
}

// PoolProfileConfigs returns pool configurations sized for Cloud SQL db-f1-micro
// (~25 total connections shared across all services)
func PoolProfileConfigs() map[PoolProfile]struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
} {
	return map[PoolProfile]struct {
		MaxOpenConns    int
		MaxIdleConns    int
		ConnMaxLifetime time.Duration
		ConnMaxIdleTime time.Duration
	}{
		// High-throughput services: products, orders, customers
		PoolProfileHighThroughput: {
			MaxOpenConns:    8,
			MaxIdleConns:    4,
			ConnMaxLifetime: 5 * time.Minute,
			ConnMaxIdleTime: 5 * time.Minute,
		},
		// Default services: most microservices
		PoolProfileDefault: {
			MaxOpenConns:    5,
			MaxIdleConns:    2,
			ConnMaxLifetime: 5 * time.Minute,
			ConnMaxIdleTime: 5 * time.Minute,
		},
		// Low-latency services: auth, tenant resolution
		PoolProfileLowLatency: {
			MaxOpenConns:    5,
			MaxIdleConns:    3, // Higher ratio of idle to open for warm connections
			ConnMaxLifetime: 5 * time.Minute,
			ConnMaxIdleTime: 5 * time.Minute,
		},
		// Background services: analytics, audit, notifications
		PoolProfileBackground: {
			MaxOpenConns:    3,
			MaxIdleConns:    1,
			ConnMaxLifetime: 10 * time.Minute,
			ConnMaxIdleTime: 5 * time.Minute,
		},
	}
}

// ApplyPoolProfile applies a pool profile to a Config
func (c *Config) ApplyPoolProfile(profile PoolProfile) {
	profiles := PoolProfileConfigs()
	if p, ok := profiles[profile]; ok {
		c.MaxOpenConns = p.MaxOpenConns
		c.MaxIdleConns = p.MaxIdleConns
		c.ConnMaxLifetime = p.ConnMaxLifetime
		c.ConnMaxIdleTime = p.ConnMaxIdleTime
	}
}

// ConfigWithProfile creates a Config with a specific pool profile
func ConfigWithProfile(profile PoolProfile) Config {
	config := DefaultConfig()
	config.ApplyPoolProfile(profile)
	return config
}

// Connect establishes a connection to PostgreSQL database
func Connect(config Config) (*sql.DB, error) {
	// Build connection string
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host,
		config.Port,
		config.User,
		config.Password,
		config.DBName,
		config.SSLMode,
	)

	// Open database connection
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Printf("Successfully connected to PostgreSQL database: %s", config.DBName)
	return db, nil
}

// HealthCheck performs a health check on the database connection
func HealthCheck(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}
	
	if err := db.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	
	return nil
}

// Close closes the database connection
func Close(db *sql.DB) error {
	if db == nil {
		return nil
	}
	
	if err := db.Close(); err != nil {
		return fmt.Errorf("failed to close database connection: %w", err)
	}
	
	log.Println("Database connection closed")
	return nil
}

// Transaction executes a function within a database transaction
func Transaction(db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()
	
	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("transaction error: %v, rollback error: %v", err, rbErr)
		}
		return err
	}
	
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	return nil
}

// Pagination represents pagination parameters
type Pagination struct {
	Page    int
	PerPage int
	Offset  int
}

// NewPagination creates a new pagination instance
func NewPagination(page, perPage int) Pagination {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}
	if perPage > 100 {
		perPage = 100
	}
	
	offset := (page - 1) * perPage
	
	return Pagination{
		Page:    page,
		PerPage: perPage,
		Offset:  offset,
	}
}

// BuildPaginationQuery adds pagination to a SQL query
func (p Pagination) BuildPaginationQuery(baseQuery string) string {
	return fmt.Sprintf("%s LIMIT %d OFFSET %d", baseQuery, p.PerPage, p.Offset)
}

// CountQuery returns a count query from a select query
func CountQuery(selectQuery string) string {
	// Simple approach - replace SELECT ... FROM with SELECT COUNT(*) FROM
	// This might need to be more sophisticated for complex queries
	return fmt.Sprintf("SELECT COUNT(*) FROM (%s) as count_query", selectQuery)
}