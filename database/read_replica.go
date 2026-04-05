package database

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ReplicaConfig holds configuration for read replica routing
type ReplicaConfig struct {
	// Primary database DSN
	PrimaryDSN string

	// Replica DSNs (can be empty for single-node setup)
	ReplicaDSNs []string

	// MaxOpenConns for each connection
	MaxOpenConns int

	// MaxIdleConns for each connection
	MaxIdleConns int

	// ConnMaxLifetime for each connection
	ConnMaxLifetime time.Duration

	// ConnMaxIdleTime for each connection
	ConnMaxIdleTime time.Duration

	// ReplicaLagThreshold - max acceptable replication lag in seconds
	ReplicaLagThreshold int

	// EnableLogging enables query logging
	EnableLogging bool

	// HealthCheckInterval is the interval for health checks
	HealthCheckInterval time.Duration

	// ReadFromPrimaryRatio is the ratio of reads to route to primary (0.0-1.0)
	// Useful for ensuring some reads hit the primary for consistency
	ReadFromPrimaryRatio float64
}

// DefaultReplicaConfig returns default replica configuration
func DefaultReplicaConfig(primaryDSN string) ReplicaConfig {
	return ReplicaConfig{
		PrimaryDSN:           primaryDSN,
		ReplicaDSNs:          []string{},
		MaxOpenConns:         100,
		MaxIdleConns:         25,
		ConnMaxLifetime:      time.Hour,
		ConnMaxIdleTime:      10 * time.Minute,
		ReplicaLagThreshold:  5,
		EnableLogging:        false,
		HealthCheckInterval:  30 * time.Second,
		ReadFromPrimaryRatio: 0.0, // By default, route all reads to replicas
	}
}

// Replica represents a single database replica
type Replica struct {
	db        *gorm.DB
	dsn       string
	healthy   bool
	lag       int64 // replication lag in seconds
	lastCheck time.Time
	mu        sync.RWMutex
}

// DBRouter handles routing between primary and read replicas
type DBRouter struct {
	config   ReplicaConfig
	primary  *gorm.DB
	replicas []*Replica
	mu       sync.RWMutex

	// Round-robin counter for replica selection
	counter uint64

	// Context for background goroutines
	ctx    context.Context
	cancel context.CancelFunc
}

// NewDBRouter creates a new database router
func NewDBRouter(config ReplicaConfig) (*DBRouter, error) {
	if config.PrimaryDSN == "" {
		return nil, errors.New("primary DSN is required")
	}

	ctx, cancel := context.WithCancel(context.Background())

	router := &DBRouter{
		config:   config,
		replicas: make([]*Replica, 0),
		ctx:      ctx,
		cancel:   cancel,
	}

	// Initialize primary
	primary, err := router.initDB(config.PrimaryDSN)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to primary: %w", err)
	}
	router.primary = primary

	// Initialize replicas
	for _, dsn := range config.ReplicaDSNs {
		replica, err := router.initReplica(dsn)
		if err != nil {
			// Log warning but continue - replica might be temporarily unavailable
			continue
		}
		router.replicas = append(router.replicas, replica)
	}

	// Start health check goroutine
	if len(router.replicas) > 0 {
		go router.healthCheckLoop()
	}

	return router, nil
}

// initDB initializes a database connection
func (r *DBRouter) initDB(dsn string) (*gorm.DB, error) {
	logLevel := logger.Silent
	if r.config.EnableLogging {
		logLevel = logger.Info
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(r.config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(r.config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(r.config.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(r.config.ConnMaxIdleTime)

	return db, nil
}

// initReplica initializes a read replica
func (r *DBRouter) initReplica(dsn string) (*Replica, error) {
	db, err := r.initDB(dsn)
	if err != nil {
		return nil, err
	}

	return &Replica{
		db:        db,
		dsn:       dsn,
		healthy:   true,
		lag:       0,
		lastCheck: time.Now(),
	}, nil
}

// Primary returns the primary database connection
func (r *DBRouter) Primary() *gorm.DB {
	return r.primary
}

// Read returns a database connection suitable for read operations
// Routes to replicas when available and healthy, falls back to primary
func (r *DBRouter) Read() *gorm.DB {
	// Check if we should route to primary based on ratio
	if r.config.ReadFromPrimaryRatio > 0 {
		count := atomic.LoadUint64(&r.counter)
		if float64(count%100)/100.0 < r.config.ReadFromPrimaryRatio {
			return r.primary
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Get healthy replicas
	healthyReplicas := make([]*Replica, 0)
	for _, replica := range r.replicas {
		if replica.IsHealthy() && replica.GetLag() <= int64(r.config.ReplicaLagThreshold) {
			healthyReplicas = append(healthyReplicas, replica)
		}
	}

	// If no healthy replicas, fall back to primary
	if len(healthyReplicas) == 0 {
		return r.primary
	}

	// Round-robin selection
	count := atomic.AddUint64(&r.counter, 1)
	selected := healthyReplicas[count%uint64(len(healthyReplicas))]

	return selected.db
}

// Write returns the primary database connection for write operations
func (r *DBRouter) Write() *gorm.DB {
	return r.primary
}

// DB returns the appropriate database based on the forWrite flag
func (r *DBRouter) DB(forWrite bool) *gorm.DB {
	if forWrite {
		return r.Write()
	}
	return r.Read()
}

// Transaction executes a function within a transaction (always on primary)
func (r *DBRouter) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.primary.WithContext(ctx).Transaction(fn)
}

// IsHealthy returns the health status of a replica
func (replica *Replica) IsHealthy() bool {
	replica.mu.RLock()
	defer replica.mu.RUnlock()
	return replica.healthy
}

// SetHealthy sets the health status of a replica
func (replica *Replica) SetHealthy(healthy bool) {
	replica.mu.Lock()
	defer replica.mu.Unlock()
	replica.healthy = healthy
	replica.lastCheck = time.Now()
}

// GetLag returns the replication lag in seconds
func (replica *Replica) GetLag() int64 {
	replica.mu.RLock()
	defer replica.mu.RUnlock()
	return replica.lag
}

// SetLag sets the replication lag
func (replica *Replica) SetLag(lag int64) {
	replica.mu.Lock()
	defer replica.mu.Unlock()
	replica.lag = lag
}

// healthCheckLoop runs health checks on all replicas periodically
func (r *DBRouter) healthCheckLoop() {
	interval := r.config.HealthCheckInterval
	if interval == 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.checkReplicas()
		}
	}
}

// checkReplicas performs health checks on all replicas
func (r *DBRouter) checkReplicas() {
	r.mu.RLock()
	replicas := make([]*Replica, len(r.replicas))
	copy(replicas, r.replicas)
	r.mu.RUnlock()

	for _, replica := range replicas {
		r.checkReplica(replica)
	}
}

// checkReplica performs a health check on a single replica
func (r *DBRouter) checkReplica(replica *Replica) {
	ctx, cancel := context.WithTimeout(r.ctx, 5*time.Second)
	defer cancel()

	sqlDB, err := replica.db.DB()
	if err != nil {
		replica.SetHealthy(false)
		return
	}

	// Ping check
	if err := sqlDB.PingContext(ctx); err != nil {
		replica.SetHealthy(false)
		return
	}

	// Check replication lag (PostgreSQL specific)
	var lag int64
	row := sqlDB.QueryRowContext(ctx, `
		SELECT COALESCE(
			EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp()))::bigint,
			0
		)
	`)
	if err := row.Scan(&lag); err != nil {
		// If we can't get lag, assume it's healthy but log warning
		lag = 0
	}

	replica.SetLag(lag)
	replica.SetHealthy(lag <= int64(r.config.ReplicaLagThreshold))
}

// AddReplica adds a new replica at runtime
func (r *DBRouter) AddReplica(dsn string) error {
	replica, err := r.initReplica(dsn)
	if err != nil {
		return err
	}

	r.mu.Lock()
	r.replicas = append(r.replicas, replica)
	r.mu.Unlock()

	return nil
}

// RemoveReplica removes a replica by DSN
func (r *DBRouter) RemoveReplica(dsn string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, replica := range r.replicas {
		if replica.dsn == dsn {
			// Close connection
			if sqlDB, err := replica.db.DB(); err == nil {
				sqlDB.Close()
			}

			// Remove from slice
			r.replicas = append(r.replicas[:i], r.replicas[i+1:]...)
			return nil
		}
	}

	return errors.New("replica not found")
}

// Close closes all database connections
func (r *DBRouter) Close() error {
	r.cancel()

	var errs []error

	// Close primary
	if sqlDB, err := r.primary.DB(); err == nil {
		if err := sqlDB.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	// Close replicas
	r.mu.Lock()
	for _, replica := range r.replicas {
		if sqlDB, err := replica.db.DB(); err == nil {
			if err := sqlDB.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	r.mu.Unlock()

	if len(errs) > 0 {
		return fmt.Errorf("errors closing connections: %v", errs)
	}

	return nil
}

// ReplicaStats holds statistics for a replica
type ReplicaStats struct {
	DSN        string
	Healthy    bool
	Lag        int64
	LastCheck  time.Time
	OpenConns  int
	IdleConns  int
	InUseConns int
}

// Stats returns statistics for all connections
type RouterStats struct {
	Primary  ReplicaStats
	Replicas []ReplicaStats
}

// GetStats returns statistics for the router
func (r *DBRouter) GetStats() RouterStats {
	stats := RouterStats{
		Replicas: make([]ReplicaStats, 0),
	}

	// Primary stats
	if sqlDB, err := r.primary.DB(); err == nil {
		dbStats := sqlDB.Stats()
		stats.Primary = ReplicaStats{
			DSN:        r.config.PrimaryDSN,
			Healthy:    true,
			Lag:        0,
			LastCheck:  time.Now(),
			OpenConns:  dbStats.OpenConnections,
			IdleConns:  dbStats.Idle,
			InUseConns: dbStats.InUse,
		}
	}

	// Replica stats
	r.mu.RLock()
	for _, replica := range r.replicas {
		stat := ReplicaStats{
			DSN:       replica.dsn,
			Healthy:   replica.IsHealthy(),
			Lag:       replica.GetLag(),
			LastCheck: replica.lastCheck,
		}

		if sqlDB, err := replica.db.DB(); err == nil {
			dbStats := sqlDB.Stats()
			stat.OpenConns = dbStats.OpenConnections
			stat.IdleConns = dbStats.Idle
			stat.InUseConns = dbStats.InUse
		}

		stats.Replicas = append(stats.Replicas, stat)
	}
	r.mu.RUnlock()

	return stats
}

// HealthyReplicaCount returns the number of healthy replicas
func (r *DBRouter) HealthyReplicaCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, replica := range r.replicas {
		if replica.IsHealthy() {
			count++
		}
	}
	return count
}

// TotalReplicaCount returns the total number of replicas
func (r *DBRouter) TotalReplicaCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.replicas)
}

// Middleware helper for Gin to inject the router into context
type contextKey string

const dbRouterKey contextKey = "db_router"

// SetDBRouter stores the DBRouter in context
func SetDBRouter(ctx context.Context, router *DBRouter) context.Context {
	return context.WithValue(ctx, dbRouterKey, router)
}

// GetDBRouter retrieves the DBRouter from context
func GetDBRouter(ctx context.Context) (*DBRouter, bool) {
	router, ok := ctx.Value(dbRouterKey).(*DBRouter)
	return router, ok
}
