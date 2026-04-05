package database

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ShardConfig holds configuration for a single shard
type ShardConfig struct {
	// DSN is the database connection string
	DSN string

	// ShardID is the unique identifier for this shard
	ShardID int

	// Weight for load balancing (higher = more traffic)
	Weight int

	// MaxOpenConns is the max number of open connections
	MaxOpenConns int

	// MaxIdleConns is the max number of idle connections
	MaxIdleConns int

	// ConnMaxLifetime is the max lifetime of a connection
	ConnMaxLifetime time.Duration

	// ConnMaxIdleTime is the max idle time for connections
	ConnMaxIdleTime time.Duration
}

// ShardingConfig holds the complete sharding configuration
type ShardingConfig struct {
	// Shards is the list of shard configurations
	Shards []ShardConfig

	// TotalShards is the total number of shards (for consistent hashing)
	TotalShards int

	// ReadReplicas per shard (optional)
	ReadReplicas map[int][]ShardConfig

	// ShardKeyColumn is the column used for sharding (default: tenant_id)
	ShardKeyColumn string

	// EnableLogging enables query logging
	EnableLogging bool
}

// DefaultShardConfig returns default shard configuration
func DefaultShardConfig(dsn string, shardID int) ShardConfig {
	return ShardConfig{
		DSN:             dsn,
		ShardID:         shardID,
		Weight:          1,
		MaxOpenConns:    100,
		MaxIdleConns:    25,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 10 * time.Minute,
	}
}

// Shard represents a database shard with its connections
type Shard struct {
	config   ShardConfig
	primary  *gorm.DB
	replicas []*gorm.DB

	// Read replica round-robin counter
	replicaCounter uint64

	// Health status
	healthy     bool
	healthMutex sync.RWMutex
	lastCheck   time.Time
}

// ShardRouter handles routing queries to the correct shard
type ShardRouter struct {
	config ShardingConfig
	shards map[int]*Shard
	mu     sync.RWMutex

	// For consistent hashing
	hashRing []int

	// Context for background goroutines
	ctx    context.Context
	cancel context.CancelFunc
}

// NewShardRouter creates a new shard router
func NewShardRouter(config ShardingConfig) (*ShardRouter, error) {
	if len(config.Shards) == 0 {
		return nil, errors.New("at least one shard is required")
	}

	if config.TotalShards == 0 {
		config.TotalShards = len(config.Shards)
	}

	if config.ShardKeyColumn == "" {
		config.ShardKeyColumn = "tenant_id"
	}

	ctx, cancel := context.WithCancel(context.Background())

	router := &ShardRouter{
		config: config,
		shards: make(map[int]*Shard),
		ctx:    ctx,
		cancel: cancel,
	}

	// Initialize shards
	for _, shardConfig := range config.Shards {
		shard, err := router.initShard(shardConfig)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to initialize shard %d: %w", shardConfig.ShardID, err)
		}
		router.shards[shardConfig.ShardID] = shard
	}

	// Initialize read replicas
	for shardID, replicaConfigs := range config.ReadReplicas {
		shard, exists := router.shards[shardID]
		if !exists {
			continue
		}

		for _, replicaConfig := range replicaConfigs {
			replica, err := router.initDB(replicaConfig)
			if err != nil {
				continue // Log warning but don't fail
			}
			shard.replicas = append(shard.replicas, replica)
		}
	}

	// Build hash ring for consistent hashing
	router.buildHashRing()

	// Start health check goroutine
	go router.healthCheckLoop()

	return router, nil
}

// initShard initializes a single shard
func (r *ShardRouter) initShard(config ShardConfig) (*Shard, error) {
	db, err := r.initDB(config)
	if err != nil {
		return nil, err
	}

	return &Shard{
		config:    config,
		primary:   db,
		replicas:  make([]*gorm.DB, 0),
		healthy:   true,
		lastCheck: time.Now(),
	}, nil
}

// initDB initializes a database connection
func (r *ShardRouter) initDB(config ShardConfig) (*gorm.DB, error) {
	logLevel := logger.Silent
	if r.config.EnableLogging {
		logLevel = logger.Info
	}

	db, err := gorm.Open(postgres.Open(config.DSN), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	return db, nil
}

// buildHashRing builds the consistent hash ring
func (r *ShardRouter) buildHashRing() {
	r.hashRing = make([]int, r.config.TotalShards)
	shardIDs := make([]int, 0, len(r.shards))

	for shardID := range r.shards {
		shardIDs = append(shardIDs, shardID)
	}

	// Distribute shards across the hash ring based on weights
	for i := 0; i < r.config.TotalShards; i++ {
		// Simple modulo distribution - in production, use consistent hashing
		r.hashRing[i] = shardIDs[i%len(shardIDs)]
	}
}

// GetShardID returns the shard ID for a given tenant ID
func (r *ShardRouter) GetShardID(tenantID string) int {
	// Use SHA256 for consistent hashing
	hash := sha256.Sum256([]byte(tenantID))
	hashValue := binary.BigEndian.Uint64(hash[:8])

	// Map to shard using modulo
	shardIndex := int(hashValue % uint64(r.config.TotalShards))
	return r.hashRing[shardIndex]
}

// GetPrimary returns the primary database connection for a tenant
func (r *ShardRouter) GetPrimary(tenantID string) (*gorm.DB, error) {
	shardID := r.GetShardID(tenantID)

	r.mu.RLock()
	shard, exists := r.shards[shardID]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("shard %d not found", shardID)
	}

	if !shard.IsHealthy() {
		return nil, fmt.Errorf("shard %d is unhealthy", shardID)
	}

	return shard.primary, nil
}

// GetReplica returns a read replica for a tenant (round-robin)
func (r *ShardRouter) GetReplica(tenantID string) (*gorm.DB, error) {
	shardID := r.GetShardID(tenantID)

	r.mu.RLock()
	shard, exists := r.shards[shardID]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("shard %d not found", shardID)
	}

	// If no replicas, fall back to primary
	if len(shard.replicas) == 0 {
		return shard.primary, nil
	}

	// Round-robin selection
	count := atomic.AddUint64(&shard.replicaCounter, 1)
	replica := shard.replicas[count%uint64(len(shard.replicas))]

	return replica, nil
}

// GetDB returns the appropriate database connection based on operation type
func (r *ShardRouter) GetDB(tenantID string, forWrite bool) (*gorm.DB, error) {
	if forWrite {
		return r.GetPrimary(tenantID)
	}
	return r.GetReplica(tenantID)
}

// ForTenant returns a database connection scoped to a specific tenant
func (r *ShardRouter) ForTenant(tenantID string, forWrite bool) (*gorm.DB, error) {
	db, err := r.GetDB(tenantID, forWrite)
	if err != nil {
		return nil, err
	}

	// Add tenant_id filter to all queries
	return db.Where(r.config.ShardKeyColumn+" = ?", tenantID), nil
}

// ExecuteOnAllShards executes a function on all shards
func (r *ShardRouter) ExecuteOnAllShards(ctx context.Context, fn func(db *gorm.DB) error) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var wg sync.WaitGroup
	errChan := make(chan error, len(r.shards))

	for _, shard := range r.shards {
		if !shard.IsHealthy() {
			continue
		}

		wg.Add(1)
		go func(db *gorm.DB) {
			defer wg.Done()
			if err := fn(db); err != nil {
				errChan <- err
			}
		}(shard.primary)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors executing on shards: %v", errs)
	}

	return nil
}

// IsHealthy returns the health status of a shard
func (s *Shard) IsHealthy() bool {
	s.healthMutex.RLock()
	defer s.healthMutex.RUnlock()
	return s.healthy
}

// SetHealthy sets the health status of a shard
func (s *Shard) SetHealthy(healthy bool) {
	s.healthMutex.Lock()
	defer s.healthMutex.Unlock()
	s.healthy = healthy
	s.lastCheck = time.Now()
}

// healthCheckLoop runs health checks on all shards periodically
func (r *ShardRouter) healthCheckLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.checkAllShards()
		}
	}
}

// checkAllShards performs health checks on all shards
func (r *ShardRouter) checkAllShards() {
	r.mu.RLock()
	shards := make([]*Shard, 0, len(r.shards))
	for _, shard := range r.shards {
		shards = append(shards, shard)
	}
	r.mu.RUnlock()

	for _, shard := range shards {
		r.checkShard(shard)
	}
}

// checkShard performs a health check on a single shard
func (r *ShardRouter) checkShard(shard *Shard) {
	ctx, cancel := context.WithTimeout(r.ctx, 5*time.Second)
	defer cancel()

	sqlDB, err := shard.primary.DB()
	if err != nil {
		shard.SetHealthy(false)
		return
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		shard.SetHealthy(false)
		return
	}

	shard.SetHealthy(true)

	// Also check replicas
	for _, replica := range shard.replicas {
		sqlDB, err := replica.DB()
		if err != nil {
			continue
		}
		_ = sqlDB.PingContext(ctx) // Log but don't affect primary health
	}
}

// Close closes all database connections
func (r *ShardRouter) Close() error {
	r.cancel()

	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for _, shard := range r.shards {
		if sqlDB, err := shard.primary.DB(); err == nil {
			if err := sqlDB.Close(); err != nil {
				errs = append(errs, err)
			}
		}

		for _, replica := range shard.replicas {
			if sqlDB, err := replica.DB(); err == nil {
				if err := sqlDB.Close(); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing connections: %v", errs)
	}

	return nil
}

// Stats returns statistics about the shard router
type ShardStats struct {
	ShardID      int
	Healthy      bool
	LastCheck    time.Time
	OpenConns    int
	IdleConns    int
	InUseConns   int
	ReplicaCount int
}

// GetStats returns statistics for all shards
func (r *ShardRouter) GetStats() []ShardStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make([]ShardStats, 0, len(r.shards))

	for shardID, shard := range r.shards {
		stat := ShardStats{
			ShardID:      shardID,
			Healthy:      shard.IsHealthy(),
			LastCheck:    shard.lastCheck,
			ReplicaCount: len(shard.replicas),
		}

		if sqlDB, err := shard.primary.DB(); err == nil {
			dbStats := sqlDB.Stats()
			stat.OpenConns = dbStats.OpenConnections
			stat.IdleConns = dbStats.Idle
			stat.InUseConns = dbStats.InUse
		}

		stats = append(stats, stat)
	}

	return stats
}

// ShardedRepository provides a base for sharded repository implementations
type ShardedRepository struct {
	router *ShardRouter
}

// NewShardedRepository creates a new sharded repository
func NewShardedRepository(router *ShardRouter) *ShardedRepository {
	return &ShardedRepository{router: router}
}

// GetDB returns the database for a tenant
func (r *ShardedRepository) GetDB(tenantID string, forWrite bool) (*gorm.DB, error) {
	return r.router.GetDB(tenantID, forWrite)
}

// WithTenant returns a database scoped to a tenant
func (r *ShardedRepository) WithTenant(ctx context.Context, tenantID string, forWrite bool) (*gorm.DB, error) {
	db, err := r.GetDB(tenantID, forWrite)
	if err != nil {
		return nil, err
	}
	return db.WithContext(ctx).Where("tenant_id = ?", tenantID), nil
}

// Transaction executes a function within a transaction on the correct shard
func (r *ShardedRepository) Transaction(ctx context.Context, tenantID string, fn func(tx *gorm.DB) error) error {
	db, err := r.router.GetPrimary(tenantID)
	if err != nil {
		return err
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(tx.Where("tenant_id = ?", tenantID))
	})
}
