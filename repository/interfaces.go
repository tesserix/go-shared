package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// Entity defines the interface that all domain entities must implement
type Entity interface {
	GetID() string
	SetID(string)
	GetCreatedAt() time.Time
	GetUpdatedAt() time.Time
}

// BaseEntity provides common fields for all entities
type BaseEntity struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

// GetID returns the entity ID
func (b *BaseEntity) GetID() string {
	return b.ID
}

// SetID sets the entity ID
func (b *BaseEntity) SetID(id string) {
	b.ID = id
}

// GetCreatedAt returns the creation time
func (b *BaseEntity) GetCreatedAt() time.Time {
	return b.CreatedAt
}

// GetUpdatedAt returns the last update time
func (b *BaseEntity) GetUpdatedAt() time.Time {
	return b.UpdatedAt
}

// TenantEntity adds tenant support to entities
type TenantEntity struct {
	BaseEntity
	TenantID string `json:"tenant_id" gorm:"type:varchar(36);index"`
}

// GetTenantID returns the tenant ID
func (t *TenantEntity) GetTenantID() string {
	return t.TenantID
}

// SetTenantID sets the tenant ID
func (t *TenantEntity) SetTenantID(tenantID string) {
	t.TenantID = tenantID
}

// QueryOptions defines options for querying data
type QueryOptions struct {
	// Pagination
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	
	// Sorting
	SortBy    string `json:"sort_by"`
	SortOrder string `json:"sort_order"` // "asc" or "desc"
	
	// Filtering
	Filters map[string]interface{} `json:"filters"`
	
	// Search
	Search       string   `json:"search"`
	SearchFields []string `json:"search_fields"`
	
	// Preloading
	Preload []string `json:"preload"`
	
	// Additional options
	IncludeDeleted bool `json:"include_deleted"`
	TenantID       string `json:"tenant_id"`
}

// DefaultQueryOptions returns default query options
func DefaultQueryOptions() QueryOptions {
	return QueryOptions{
		Page:      1,
		PageSize:  10,
		SortBy:    "created_at",
		SortOrder: "desc",
		Filters:   make(map[string]interface{}),
	}
}

// PaginatedResult represents a paginated result set
type PaginatedResult[T Entity] struct {
	Data       []T   `json:"data"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

// Repository defines the generic repository interface
type Repository[T Entity] interface {
	// Basic CRUD operations
	Create(ctx context.Context, entity T) (T, error)
	GetByID(ctx context.Context, id string) (T, error)
	Update(ctx context.Context, entity T) (T, error)
	Delete(ctx context.Context, id string) error
	SoftDelete(ctx context.Context, id string) error
	
	// Bulk operations
	CreateBatch(ctx context.Context, entities []T) ([]T, error)
	UpdateBatch(ctx context.Context, entities []T) ([]T, error)
	DeleteBatch(ctx context.Context, ids []string) error
	
	// Query operations
	List(ctx context.Context, options QueryOptions) (*PaginatedResult[T], error)
	Find(ctx context.Context, options QueryOptions) ([]T, error)
	FindOne(ctx context.Context, conditions map[string]interface{}) (T, error)
	Count(ctx context.Context, conditions map[string]interface{}) (int64, error)
	Exists(ctx context.Context, conditions map[string]interface{}) (bool, error)
	
	// Transaction support
	WithTransaction(tx *gorm.DB) Repository[T]
	
	// Raw query support
	Raw(ctx context.Context, sql string, args ...interface{}) *gorm.DB
	
	// Relationship preloading
	WithPreload(associations ...string) Repository[T]
}

// TenantRepository adds tenant-aware operations
type TenantRepository[T Entity] interface {
	Repository[T]
	
	// Tenant-specific operations
	CreateForTenant(ctx context.Context, tenantID string, entity T) (T, error)
	GetByIDForTenant(ctx context.Context, tenantID, id string) (T, error)
	ListForTenant(ctx context.Context, tenantID string, options QueryOptions) (*PaginatedResult[T], error)
	DeleteForTenant(ctx context.Context, tenantID, id string) error
}

// ReadOnlyRepository provides read-only operations
type ReadOnlyRepository[T Entity] interface {
	GetByID(ctx context.Context, id string) (T, error)
	List(ctx context.Context, options QueryOptions) (*PaginatedResult[T], error)
	Find(ctx context.Context, options QueryOptions) ([]T, error)
	FindOne(ctx context.Context, conditions map[string]interface{}) (T, error)
	Count(ctx context.Context, conditions map[string]interface{}) (int64, error)
	Exists(ctx context.Context, conditions map[string]interface{}) (bool, error)
}

// CacheableRepository adds caching capabilities
type CacheableRepository[T Entity] interface {
	Repository[T]
	
	// Cache operations
	SetCache(key string, value interface{}, ttl time.Duration) error
	GetCache(key string, dest interface{}) error
	DeleteCache(key string) error
	ClearCache(pattern string) error
}

// AuditableRepository adds audit logging
type AuditableRepository[T Entity] interface {
	Repository[T]
	
	// Audit operations
	GetAuditLog(ctx context.Context, entityID string) ([]AuditEntry, error)
	CreateWithAudit(ctx context.Context, entity T, userID string) (T, error)
	UpdateWithAudit(ctx context.Context, entity T, userID string) (T, error)
	DeleteWithAudit(ctx context.Context, id, userID string) error
}

// AuditEntry represents an audit log entry
type AuditEntry struct {
	ID        string                 `json:"id" gorm:"primaryKey;type:varchar(36)"`
	EntityID  string                 `json:"entity_id" gorm:"type:varchar(36);index"`
	Action    string                 `json:"action" gorm:"type:varchar(50)"`
	UserID    string                 `json:"user_id" gorm:"type:varchar(36)"`
	Changes   map[string]interface{} `json:"changes" gorm:"serializer:json"`
	CreatedAt time.Time              `json:"created_at" gorm:"autoCreateTime"`
}

// QueryBuilder provides a fluent interface for building queries
type QueryBuilder[T Entity] interface {
	Where(condition string, args ...interface{}) QueryBuilder[T]
	WhereIn(column string, values []interface{}) QueryBuilder[T]
	WhereBetween(column string, start, end interface{}) QueryBuilder[T]
	WhereNull(column string) QueryBuilder[T]
	WhereNotNull(column string) QueryBuilder[T]
	
	// Joins
	Join(table, condition string) QueryBuilder[T]
	LeftJoin(table, condition string) QueryBuilder[T]
	RightJoin(table, condition string) QueryBuilder[T]
	
	// Ordering
	OrderBy(column, direction string) QueryBuilder[T]
	OrderByRaw(sql string) QueryBuilder[T]
	
	// Grouping
	GroupBy(columns ...string) QueryBuilder[T]
	Having(condition string, args ...interface{}) QueryBuilder[T]
	
	// Limiting
	Limit(limit int) QueryBuilder[T]
	Offset(offset int) QueryBuilder[T]
	
	// Preloading
	Preload(associations ...string) QueryBuilder[T]
	
	// Execution
	Find() ([]T, error)
	First() (T, error)
	Count() (int64, error)
	Exists() (bool, error)
	
	// Raw SQL
	Raw(sql string, args ...interface{}) QueryBuilder[T]
	
	// Debugging
	Debug() QueryBuilder[T]
}

// RepositoryConfig holds configuration for repositories
type RepositoryConfig struct {
	DB               *gorm.DB
	TableName        string
	EnableSoftDelete bool
	EnableAudit      bool
	EnableCache      bool
	CacheTTL         time.Duration
	TenantAware      bool
}

// RepositoryFactory creates repository instances
// Note: Go does not support generic methods on interfaces, so factory methods
// are provided as package-level functions: CreateRepository[T], CreateTenantRepository[T], etc.
type RepositoryFactory interface {
	// GetDB returns the underlying database connection for creating repositories
	GetDB() *gorm.DB
}

// TransactionManager handles database transactions
type TransactionManager interface {
	WithTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error
	BeginTransaction(ctx context.Context) (*gorm.DB, error)
	CommitTransaction(tx *gorm.DB) error
	RollbackTransaction(tx *gorm.DB) error
}