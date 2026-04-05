package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// DefaultRepositoryFactory implements RepositoryFactory
type DefaultRepositoryFactory struct {
	db *gorm.DB
}

// NewRepositoryFactory creates a new repository factory
func NewRepositoryFactory(db *gorm.DB) RepositoryFactory {
	return &DefaultRepositoryFactory{
		db: db,
	}
}

// GetDB returns the underlying database connection
func (f *DefaultRepositoryFactory) GetDB() *gorm.DB {
	return f.db
}

// CreateRepository creates a new base repository (package-level generic function)
func CreateRepository[T Entity](f RepositoryFactory, config RepositoryConfig) Repository[T] {
	db := config.DB
	if db == nil {
		db = f.GetDB()
	}

	repo := NewBaseRepository[T](db, config.TableName)
	repo.SetSoftDelete(config.EnableSoftDelete)
	repo.SetAudit(config.EnableAudit)

	return repo
}

// CreateTenantRepository creates a new tenant-aware repository (package-level generic function)
func CreateTenantRepository[T Entity](f RepositoryFactory, config RepositoryConfig) TenantRepository[T] {
	db := config.DB
	if db == nil {
		db = f.GetDB()
	}

	repo := NewTenantRepository[T](db, config.TableName)
	repo.SetSoftDelete(config.EnableSoftDelete)
	repo.SetAudit(config.EnableAudit)

	return repo
}

// CreateReadOnlyRepository creates a new read-only repository (package-level generic function)
func CreateReadOnlyRepository[T Entity](f RepositoryFactory, config RepositoryConfig) ReadOnlyRepository[T] {
	db := config.DB
	if db == nil {
		db = f.GetDB()
	}

	repo := NewBaseRepository[T](db, config.TableName)

	// Wrap in read-only adapter
	return &ReadOnlyRepositoryAdapter[T]{
		repository: repo,
	}
}

// ReadOnlyRepositoryAdapter adapts a full repository to read-only interface
type ReadOnlyRepositoryAdapter[T Entity] struct {
	repository Repository[T]
}

// GetByID retrieves an entity by its ID
func (r *ReadOnlyRepositoryAdapter[T]) GetByID(ctx context.Context, id string) (T, error) {
	return r.repository.GetByID(ctx, id)
}

// List retrieves entities with pagination and filtering
func (r *ReadOnlyRepositoryAdapter[T]) List(ctx context.Context, options QueryOptions) (*PaginatedResult[T], error) {
	return r.repository.List(ctx, options)
}

// Find retrieves entities without pagination
func (r *ReadOnlyRepositoryAdapter[T]) Find(ctx context.Context, options QueryOptions) ([]T, error) {
	return r.repository.Find(ctx, options)
}

// FindOne retrieves a single entity based on conditions
func (r *ReadOnlyRepositoryAdapter[T]) FindOne(ctx context.Context, conditions map[string]interface{}) (T, error) {
	return r.repository.FindOne(ctx, conditions)
}

// Count counts entities based on conditions
func (r *ReadOnlyRepositoryAdapter[T]) Count(ctx context.Context, conditions map[string]interface{}) (int64, error) {
	return r.repository.Count(ctx, conditions)
}

// Exists checks if an entity exists based on conditions
func (r *ReadOnlyRepositoryAdapter[T]) Exists(ctx context.Context, conditions map[string]interface{}) (bool, error) {
	return r.repository.Exists(ctx, conditions)
}

// DefaultTransactionManager implements TransactionManager
type DefaultTransactionManager struct {
	db *gorm.DB
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager(db *gorm.DB) TransactionManager {
	return &DefaultTransactionManager{
		db: db,
	}
}

// WithTransaction executes a function within a database transaction
func (tm *DefaultTransactionManager) WithTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return tm.db.WithContext(ctx).Transaction(fn)
}

// BeginTransaction starts a new transaction
func (tm *DefaultTransactionManager) BeginTransaction(ctx context.Context) (*gorm.DB, error) {
	tx := tm.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}
	return tx, nil
}

// CommitTransaction commits a transaction
func (tm *DefaultTransactionManager) CommitTransaction(tx *gorm.DB) error {
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// RollbackTransaction rolls back a transaction
func (tm *DefaultTransactionManager) RollbackTransaction(tx *gorm.DB) error {
	if err := tx.Rollback().Error; err != nil {
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}
	return nil
}

// ServiceRepositoryManager manages all repositories for a service
type ServiceRepositoryManager struct {
	factory            RepositoryFactory
	transactionManager TransactionManager
	repositories       map[string]interface{}
}

// NewServiceRepositoryManager creates a new service repository manager
func NewServiceRepositoryManager(db *gorm.DB) *ServiceRepositoryManager {
	return &ServiceRepositoryManager{
		factory:            NewRepositoryFactory(db),
		transactionManager: NewTransactionManager(db),
		repositories:       make(map[string]interface{}),
	}
}

// RegisterRepository registers a repository with the manager (package-level generic function)
func RegisterRepository[T Entity](srm *ServiceRepositoryManager, name string, config RepositoryConfig) {
	repo := CreateRepository[T](srm.factory, config)
	srm.repositories[name] = repo
}

// RegisterTenantRepository registers a tenant repository with the manager (package-level generic function)
func RegisterTenantRepository[T Entity](srm *ServiceRepositoryManager, name string, config RepositoryConfig) {
	repo := CreateTenantRepository[T](srm.factory, config)
	srm.repositories[name] = repo
}

// GetRepository retrieves a registered repository (package-level generic function)
func GetRepository[T Entity](srm *ServiceRepositoryManager, name string) (Repository[T], error) {
	repo, exists := srm.repositories[name]
	if !exists {
		var zero Repository[T]
		return zero, fmt.Errorf("repository %s not found", name)
	}

	typedRepo, ok := repo.(Repository[T])
	if !ok {
		var zero Repository[T]
		return zero, fmt.Errorf("repository %s has incorrect type", name)
	}

	return typedRepo, nil
}

// GetTenantRepository retrieves a registered tenant repository (package-level generic function)
func GetTenantRepository[T Entity](srm *ServiceRepositoryManager, name string) (TenantRepository[T], error) {
	repo, exists := srm.repositories[name]
	if !exists {
		var zero TenantRepository[T]
		return zero, fmt.Errorf("tenant repository %s not found", name)
	}

	typedRepo, ok := repo.(TenantRepository[T])
	if !ok {
		var zero TenantRepository[T]
		return zero, fmt.Errorf("tenant repository %s has incorrect type", name)
	}

	return typedRepo, nil
}

// WithTransaction executes a function with all repositories in a transaction
func (srm *ServiceRepositoryManager) WithTransaction(ctx context.Context, fn func(map[string]interface{}) error) error {
	return srm.transactionManager.WithTransaction(ctx, func(tx *gorm.DB) error {
		// Create transaction-scoped repositories
		txRepositories := make(map[string]interface{})

		for name, repo := range srm.repositories {
			switch r := repo.(type) {
			case Repository[Entity]:
				txRepositories[name] = r.WithTransaction(tx)
			case TenantRepository[Entity]:
				txRepositories[name] = r.WithTransaction(tx)
			default:
				txRepositories[name] = repo
			}
		}

		return fn(txRepositories)
	})
}

// Repository Registration Helpers

// RepositoryRegistry provides a centralized way to register and access repositories
type RepositoryRegistry struct {
	manager *ServiceRepositoryManager
}

// NewRepositoryRegistry creates a new repository registry
func NewRepositoryRegistry(db *gorm.DB) *RepositoryRegistry {
	return &RepositoryRegistry{
		manager: NewServiceRepositoryManager(db),
	}
}

// Register registers a repository (package-level generic function)
func Register[T Entity](rr *RepositoryRegistry, name string, config RepositoryConfig) *RepositoryRegistry {
	RegisterRepository[T](rr.manager, name, config)
	return rr
}

// RegisterTenant registers a tenant repository (package-level generic function)
func RegisterTenant[T Entity](rr *RepositoryRegistry, name string, config RepositoryConfig) *RepositoryRegistry {
	RegisterTenantRepository[T](rr.manager, name, config)
	return rr
}

// Get retrieves a repository (package-level generic function)
func Get[T Entity](rr *RepositoryRegistry, name string) Repository[T] {
	repo, err := GetRepository[T](rr.manager, name)
	if err != nil {
		panic(fmt.Sprintf("Repository %s not found: %v", name, err))
	}
	return repo
}

// GetTenant retrieves a tenant repository (package-level generic function)
func GetTenant[T Entity](rr *RepositoryRegistry, name string) TenantRepository[T] {
	repo, err := GetTenantRepository[T](rr.manager, name)
	if err != nil {
		panic(fmt.Sprintf("Tenant repository %s not found: %v", name, err))
	}
	return repo
}

// WithTx executes a function in a transaction
func (rr *RepositoryRegistry) WithTx(ctx context.Context, fn func(*RepositoryRegistry) error) error {
	return rr.manager.WithTransaction(ctx, func(txRepos map[string]interface{}) error {
		txRegistry := &RepositoryRegistry{
			manager: &ServiceRepositoryManager{
				repositories: txRepos,
			},
		}
		return fn(txRegistry)
	})
}

// Configuration Builders

// RepositoryConfigBuilder provides a fluent interface for building repository configurations
type RepositoryConfigBuilder struct {
	config RepositoryConfig
}

// NewRepositoryConfig creates a new repository configuration builder
func NewRepositoryConfig() *RepositoryConfigBuilder {
	return &RepositoryConfigBuilder{
		config: RepositoryConfig{
			EnableSoftDelete: true,
			EnableAudit:      false,
			EnableCache:      false,
			CacheTTL:         5 * 60, // 5 minutes
			TenantAware:      false,
		},
	}
}

// WithDB sets the database connection
func (rcb *RepositoryConfigBuilder) WithDB(db *gorm.DB) *RepositoryConfigBuilder {
	rcb.config.DB = db
	return rcb
}

// WithTableName sets the table name
func (rcb *RepositoryConfigBuilder) WithTableName(tableName string) *RepositoryConfigBuilder {
	rcb.config.TableName = tableName
	return rcb
}

// WithSoftDelete enables or disables soft delete
func (rcb *RepositoryConfigBuilder) WithSoftDelete(enabled bool) *RepositoryConfigBuilder {
	rcb.config.EnableSoftDelete = enabled
	return rcb
}

// WithAudit enables or disables audit logging
func (rcb *RepositoryConfigBuilder) WithAudit(enabled bool) *RepositoryConfigBuilder {
	rcb.config.EnableAudit = enabled
	return rcb
}

// WithCache enables or disables caching
func (rcb *RepositoryConfigBuilder) WithCache(enabled bool) *RepositoryConfigBuilder {
	rcb.config.EnableCache = enabled
	return rcb
}

// WithTenantAware enables or disables tenant awareness
func (rcb *RepositoryConfigBuilder) WithTenantAware(enabled bool) *RepositoryConfigBuilder {
	rcb.config.TenantAware = enabled
	return rcb
}

// Build returns the configuration
func (rcb *RepositoryConfigBuilder) Build() RepositoryConfig {
	return rcb.config
}
