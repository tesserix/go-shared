package repository

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BaseRepository implements the Repository interface using GORM
type BaseRepository[T Entity] struct {
	db               *gorm.DB
	tableName        string
	entityType       reflect.Type
	enableSoftDelete bool
	enableAudit      bool
	preloadFields    []string
}

// NewBaseRepository creates a new base repository instance
func NewBaseRepository[T Entity](db *gorm.DB, tableName string) *BaseRepository[T] {
	var entity T
	entityType := reflect.TypeOf(entity)
	if entityType.Kind() == reflect.Ptr {
		entityType = entityType.Elem()
	}

	return &BaseRepository[T]{
		db:               db,
		tableName:        tableName,
		entityType:       entityType,
		enableSoftDelete: true,
		enableAudit:      false,
		preloadFields:    make([]string, 0),
	}
}

// Create creates a new entity
func (r *BaseRepository[T]) Create(ctx context.Context, entity T) (T, error) {
	// Generate ID if not set
	if entity.GetID() == "" {
		entity.SetID(uuid.New().String())
	}

	// Execute create
	if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
		var zero T
		return zero, fmt.Errorf("failed to create entity: %w", err)
	}

	return entity, nil
}

// GetByID retrieves an entity by its ID
func (r *BaseRepository[T]) GetByID(ctx context.Context, id string) (T, error) {
	var entity T
	
	query := r.db.WithContext(ctx)
	
	// Apply preloads
	for _, field := range r.preloadFields {
		query = query.Preload(field)
	}
	
	if err := query.First(&entity, "id = ?", id).Error; err != nil {
		var zero T
		if err == gorm.ErrRecordNotFound {
			return zero, fmt.Errorf("entity with id %s not found", id)
		}
		return zero, fmt.Errorf("failed to get entity: %w", err)
	}

	return entity, nil
}

// Update updates an existing entity
func (r *BaseRepository[T]) Update(ctx context.Context, entity T) (T, error) {
	if entity.GetID() == "" {
		var zero T
		return zero, fmt.Errorf("entity ID is required for update")
	}

	if err := r.db.WithContext(ctx).Save(&entity).Error; err != nil {
		var zero T
		return zero, fmt.Errorf("failed to update entity: %w", err)
	}

	return entity, nil
}

// Delete permanently deletes an entity
func (r *BaseRepository[T]) Delete(ctx context.Context, id string) error {
	var entity T
	
	result := r.db.WithContext(ctx).Unscoped().Delete(&entity, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete entity: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		return fmt.Errorf("entity with id %s not found", id)
	}

	return nil
}

// SoftDelete soft deletes an entity (if soft delete is enabled)
func (r *BaseRepository[T]) SoftDelete(ctx context.Context, id string) error {
	if !r.enableSoftDelete {
		return r.Delete(ctx, id)
	}

	var entity T
	
	result := r.db.WithContext(ctx).Delete(&entity, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to soft delete entity: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		return fmt.Errorf("entity with id %s not found", id)
	}

	return nil
}

// CreateBatch creates multiple entities in a single transaction
func (r *BaseRepository[T]) CreateBatch(ctx context.Context, entities []T) ([]T, error) {
	if len(entities) == 0 {
		return entities, nil
	}

	// Generate IDs for entities that don't have them
	for i := range entities {
		if entities[i].GetID() == "" {
			entities[i].SetID(uuid.New().String())
		}
	}

	if err := r.db.WithContext(ctx).CreateInBatches(entities, 100).Error; err != nil {
		return nil, fmt.Errorf("failed to create entities in batch: %w", err)
	}

	return entities, nil
}

// UpdateBatch updates multiple entities
func (r *BaseRepository[T]) UpdateBatch(ctx context.Context, entities []T) ([]T, error) {
	if len(entities) == 0 {
		return entities, nil
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, entity := range entities {
			if err := tx.Save(&entity).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to update entities in batch: %w", err)
	}

	return entities, nil
}

// DeleteBatch deletes multiple entities by IDs
func (r *BaseRepository[T]) DeleteBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	var entity T
	
	result := r.db.WithContext(ctx).Delete(&entity, "id IN ?", ids)
	if result.Error != nil {
		return fmt.Errorf("failed to delete entities in batch: %w", result.Error)
	}

	return nil
}

// List retrieves entities with pagination and filtering
func (r *BaseRepository[T]) List(ctx context.Context, options QueryOptions) (*PaginatedResult[T], error) {
	query := r.buildQuery(ctx, options)
	
	// Count total records
	var total int64
	countQuery := r.buildCountQuery(ctx, options)
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count entities: %w", err)
	}

	// Calculate pagination
	if options.PageSize <= 0 {
		options.PageSize = 10
	}
	if options.Page <= 0 {
		options.Page = 1
	}

	offset := (options.Page - 1) * options.PageSize
	totalPages := int((total + int64(options.PageSize) - 1) / int64(options.PageSize))

	// Apply pagination
	query = query.Limit(options.PageSize).Offset(offset)

	// Execute query
	var entities []T
	if err := query.Find(&entities).Error; err != nil {
		return nil, fmt.Errorf("failed to list entities: %w", err)
	}

	return &PaginatedResult[T]{
		Data:       entities,
		Total:      total,
		Page:       options.Page,
		PageSize:   options.PageSize,
		TotalPages: totalPages,
		HasNext:    options.Page < totalPages,
		HasPrev:    options.Page > 1,
	}, nil
}

// Find retrieves entities without pagination
func (r *BaseRepository[T]) Find(ctx context.Context, options QueryOptions) ([]T, error) {
	query := r.buildQuery(ctx, options)
	
	var entities []T
	if err := query.Find(&entities).Error; err != nil {
		return nil, fmt.Errorf("failed to find entities: %w", err)
	}

	return entities, nil
}

// FindOne retrieves a single entity based on conditions
func (r *BaseRepository[T]) FindOne(ctx context.Context, conditions map[string]interface{}) (T, error) {
	var entity T
	
	query := r.db.WithContext(ctx)
	
	// Apply preloads
	for _, field := range r.preloadFields {
		query = query.Preload(field)
	}
	
	// Apply conditions
	for key, value := range conditions {
		query = query.Where(fmt.Sprintf("%s = ?", key), value)
	}
	
	if err := query.First(&entity).Error; err != nil {
		var zero T
		if err == gorm.ErrRecordNotFound {
			return zero, fmt.Errorf("entity not found")
		}
		return zero, fmt.Errorf("failed to find entity: %w", err)
	}

	return entity, nil
}

// Count counts entities based on conditions
func (r *BaseRepository[T]) Count(ctx context.Context, conditions map[string]interface{}) (int64, error) {
	var count int64
	var entity T
	
	query := r.db.WithContext(ctx).Model(&entity)
	
	// Apply conditions
	for key, value := range conditions {
		query = query.Where(fmt.Sprintf("%s = ?", key), value)
	}
	
	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count entities: %w", err)
	}

	return count, nil
}

// Exists checks if an entity exists based on conditions
func (r *BaseRepository[T]) Exists(ctx context.Context, conditions map[string]interface{}) (bool, error) {
	count, err := r.Count(ctx, conditions)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// WithTransaction returns a new repository instance with the given transaction
func (r *BaseRepository[T]) WithTransaction(tx *gorm.DB) Repository[T] {
	newRepo := *r
	newRepo.db = tx
	return &newRepo
}

// Raw executes a raw SQL query
func (r *BaseRepository[T]) Raw(ctx context.Context, sql string, args ...interface{}) *gorm.DB {
	return r.db.WithContext(ctx).Raw(sql, args...)
}

// WithPreload sets preload associations
func (r *BaseRepository[T]) WithPreload(associations ...string) Repository[T] {
	newRepo := *r
	newRepo.preloadFields = associations
	return &newRepo
}

// Helper methods

// buildQuery builds a GORM query based on options
func (r *BaseRepository[T]) buildQuery(ctx context.Context, options QueryOptions) *gorm.DB {
	var entity T
	query := r.db.WithContext(ctx).Model(&entity)
	
	// Apply preloads
	for _, field := range r.preloadFields {
		query = query.Preload(field)
	}
	
	// Apply additional preloads from options
	for _, field := range options.Preload {
		query = query.Preload(field)
	}
	
	// Apply filters
	for key, value := range options.Filters {
		query = query.Where(fmt.Sprintf("%s = ?", key), value)
	}
	
	// Apply tenant filter
	if options.TenantID != "" {
		query = query.Where("tenant_id = ?", options.TenantID)
	}
	
	// Apply search
	if options.Search != "" && len(options.SearchFields) > 0 {
		searchConditions := make([]string, len(options.SearchFields))
		searchArgs := make([]interface{}, len(options.SearchFields))
		
		for i, field := range options.SearchFields {
			searchConditions[i] = fmt.Sprintf("%s ILIKE ?", field)
			searchArgs[i] = "%" + options.Search + "%"
		}
		
		query = query.Where(strings.Join(searchConditions, " OR "), searchArgs...)
	}
	
	// Apply sorting
	if options.SortBy != "" {
		order := "ASC"
		if strings.ToLower(options.SortOrder) == "desc" {
			order = "DESC"
		}
		query = query.Order(fmt.Sprintf("%s %s", options.SortBy, order))
	}
	
	// Include deleted records if requested
	if options.IncludeDeleted {
		query = query.Unscoped()
	}
	
	return query
}

// buildCountQuery builds a query for counting records
func (r *BaseRepository[T]) buildCountQuery(ctx context.Context, options QueryOptions) *gorm.DB {
	var entity T
	query := r.db.WithContext(ctx).Model(&entity)
	
	// Apply filters (but not preloads for counting)
	for key, value := range options.Filters {
		query = query.Where(fmt.Sprintf("%s = ?", key), value)
	}
	
	// Apply tenant filter
	if options.TenantID != "" {
		query = query.Where("tenant_id = ?", options.TenantID)
	}
	
	// Apply search
	if options.Search != "" && len(options.SearchFields) > 0 {
		searchConditions := make([]string, len(options.SearchFields))
		searchArgs := make([]interface{}, len(options.SearchFields))
		
		for i, field := range options.SearchFields {
			searchConditions[i] = fmt.Sprintf("%s ILIKE ?", field)
			searchArgs[i] = "%" + options.Search + "%"
		}
		
		query = query.Where(strings.Join(searchConditions, " OR "), searchArgs...)
	}
	
	// Include deleted records if requested
	if options.IncludeDeleted {
		query = query.Unscoped()
	}
	
	return query
}

// SetSoftDelete enables or disables soft delete
func (r *BaseRepository[T]) SetSoftDelete(enabled bool) {
	r.enableSoftDelete = enabled
}

// SetAudit enables or disables audit logging
func (r *BaseRepository[T]) SetAudit(enabled bool) {
	r.enableAudit = enabled
}