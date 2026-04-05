package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// TenantBaseRepository implements tenant-aware repository operations
type TenantBaseRepository[T Entity] struct {
	*BaseRepository[T]
}

// NewTenantRepository creates a new tenant-aware repository
func NewTenantRepository[T Entity](db *gorm.DB, tableName string) *TenantBaseRepository[T] {
	baseRepo := NewBaseRepository[T](db, tableName)
	return &TenantBaseRepository[T]{
		BaseRepository: baseRepo,
	}
}

// CreateForTenant creates an entity for a specific tenant
func (r *TenantBaseRepository[T]) CreateForTenant(ctx context.Context, tenantID string, entity T) (T, error) {
	// Set tenant ID if the entity supports it
	if tenantEntity, ok := any(entity).(interface{ SetTenantID(string) }); ok {
		tenantEntity.SetTenantID(tenantID)
	}
	
	return r.Create(ctx, entity)
}

// GetByIDForTenant retrieves an entity by ID for a specific tenant
func (r *TenantBaseRepository[T]) GetByIDForTenant(ctx context.Context, tenantID, id string) (T, error) {
	var entity T
	
	query := r.db.WithContext(ctx)
	
	// Apply preloads
	for _, field := range r.preloadFields {
		query = query.Preload(field)
	}
	
	// Add tenant and ID conditions
	if err := query.Where("id = ? AND tenant_id = ?", id, tenantID).First(&entity).Error; err != nil {
		var zero T
		if err == gorm.ErrRecordNotFound {
			return zero, fmt.Errorf("entity with id %s not found for tenant %s", id, tenantID)
		}
		return zero, fmt.Errorf("failed to get entity for tenant: %w", err)
	}

	return entity, nil
}

// ListForTenant retrieves entities for a specific tenant
func (r *TenantBaseRepository[T]) ListForTenant(ctx context.Context, tenantID string, options QueryOptions) (*PaginatedResult[T], error) {
	// Ensure tenant ID is set in options
	options.TenantID = tenantID
	return r.List(ctx, options)
}

// DeleteForTenant deletes an entity for a specific tenant
func (r *TenantBaseRepository[T]) DeleteForTenant(ctx context.Context, tenantID, id string) error {
	var entity T
	
	result := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&entity)
	if result.Error != nil {
		return fmt.Errorf("failed to delete entity for tenant: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		return fmt.Errorf("entity with id %s not found for tenant %s", id, tenantID)
	}

	return nil
}

// UpdateForTenant updates an entity for a specific tenant
func (r *TenantBaseRepository[T]) UpdateForTenant(ctx context.Context, tenantID string, entity T) (T, error) {
	// Verify the entity belongs to the tenant
	if tenantEntity, ok := any(entity).(interface{ GetTenantID() string }); ok {
		if tenantEntity.GetTenantID() != tenantID {
			var zero T
			return zero, fmt.Errorf("entity does not belong to tenant %s", tenantID)
		}
	}
	
	return r.Update(ctx, entity)
}

// FindForTenant finds entities for a specific tenant
func (r *TenantBaseRepository[T]) FindForTenant(ctx context.Context, tenantID string, conditions map[string]interface{}) ([]T, error) {
	// Add tenant condition
	if conditions == nil {
		conditions = make(map[string]interface{})
	}
	conditions["tenant_id"] = tenantID
	
	options := QueryOptions{
		Filters: conditions,
	}
	
	return r.Find(ctx, options)
}

// CountForTenant counts entities for a specific tenant
func (r *TenantBaseRepository[T]) CountForTenant(ctx context.Context, tenantID string, conditions map[string]interface{}) (int64, error) {
	// Add tenant condition
	if conditions == nil {
		conditions = make(map[string]interface{})
	}
	conditions["tenant_id"] = tenantID
	
	return r.Count(ctx, conditions)
}

// ExistsForTenant checks if an entity exists for a specific tenant
func (r *TenantBaseRepository[T]) ExistsForTenant(ctx context.Context, tenantID string, conditions map[string]interface{}) (bool, error) {
	count, err := r.CountForTenant(ctx, tenantID, conditions)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// WithTransaction returns a new tenant repository instance with the given transaction
// Note: Returns Repository[T] to satisfy the interface, but the underlying type is still TenantBaseRepository.
// Use type assertion to access tenant-specific methods in transaction: repo.WithTransaction(tx).(TenantRepository[T])
func (r *TenantBaseRepository[T]) WithTransaction(tx *gorm.DB) Repository[T] {
	newRepo := *r
	newRepo.BaseRepository = newRepo.BaseRepository.WithTransaction(tx).(*BaseRepository[T])
	return &newRepo
}

// TenantAwareQueryBuilder provides tenant-aware query building
type TenantAwareQueryBuilder[T Entity] struct {
	*QueryBuilderImpl[T]
	tenantID string
}

// NewTenantAwareQueryBuilder creates a new tenant-aware query builder
func NewTenantAwareQueryBuilder[T Entity](db *gorm.DB, tenantID string) *TenantAwareQueryBuilder[T] {
	baseBuilder := NewQueryBuilder[T](db)
	
	// Automatically add tenant filter
	baseBuilder = baseBuilder.Where("tenant_id = ?", tenantID).(*QueryBuilderImpl[T])
	
	return &TenantAwareQueryBuilder[T]{
		QueryBuilderImpl: baseBuilder,
		tenantID:         tenantID,
	}
}

// GetTenantID returns the tenant ID for this query builder
func (qb *TenantAwareQueryBuilder[T]) GetTenantID() string {
	return qb.tenantID
}

// CreateForCurrentTenant creates an entity for the current tenant
func (qb *TenantAwareQueryBuilder[T]) CreateForCurrentTenant(ctx context.Context, entity T) (T, error) {
	// Set tenant ID if the entity supports it
	if tenantEntity, ok := any(entity).(interface{ SetTenantID(string) }); ok {
		tenantEntity.SetTenantID(qb.tenantID)
	}
	
	if err := qb.db.WithContext(ctx).Create(&entity).Error; err != nil {
		var zero T
		return zero, fmt.Errorf("failed to create entity for tenant: %w", err)
	}
	
	return entity, nil
}

// Multi-tenant utilities

// TenantMigrator handles tenant-specific migrations
type TenantMigrator struct {
	db *gorm.DB
}

// NewTenantMigrator creates a new tenant migrator
func NewTenantMigrator(db *gorm.DB) *TenantMigrator {
	return &TenantMigrator{db: db}
}

// MigrateForTenant runs migrations for a specific tenant
func (tm *TenantMigrator) MigrateForTenant(tenantID string, models ...interface{}) error {
	// Set tenant-specific table naming
	for _, model := range models {
		if err := tm.db.Table(tm.getTenantTableName(tenantID, model)).AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate for tenant %s: %w", tenantID, err)
		}
	}
	return nil
}

// getTenantTableName generates tenant-specific table names
func (tm *TenantMigrator) getTenantTableName(tenantID string, model interface{}) string {
	// Get base table name
	stmt := &gorm.Statement{DB: tm.db}
	stmt.Parse(model)
	baseTableName := stmt.Schema.Table
	
	// Return tenant-specific table name
	return fmt.Sprintf("%s_%s", tenantID, baseTableName)
}

// TenantContext provides tenant information in context
type TenantContext struct {
	TenantID   string
	TenantName string
	Schema     string
}

// WithTenantContext adds tenant context to the given context
func WithTenantContext(ctx context.Context, tenantCtx TenantContext) context.Context {
	return context.WithValue(ctx, "tenant_context", tenantCtx)
}

// GetTenantContext retrieves tenant context from the given context
func GetTenantContext(ctx context.Context) (TenantContext, bool) {
	tenantCtx, ok := ctx.Value("tenant_context").(TenantContext)
	return tenantCtx, ok
}

// TenantAwareMiddleware provides middleware for extracting tenant information
func TenantAwareMiddleware() func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if db.Statement != nil && db.Statement.Context != nil {
			if tenantCtx, ok := GetTenantContext(db.Statement.Context); ok {
				// Automatically add tenant filter for tenant-aware entities
				return db.Where("tenant_id = ?", tenantCtx.TenantID)
			}
		}
		return db
	}
}