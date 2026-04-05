package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- RepositoryConfigBuilder ---

func TestNewRepositoryConfig_Defaults(t *testing.T) {
	cfg := NewRepositoryConfig().Build()

	assert.True(t, cfg.EnableSoftDelete, "soft delete should be enabled by default")
	assert.False(t, cfg.EnableAudit, "audit should be disabled by default")
	assert.False(t, cfg.EnableCache, "cache should be disabled by default")
	assert.Equal(t, time.Duration(5*60), cfg.CacheTTL, "default cache TTL should be 5 minutes in seconds")
	assert.False(t, cfg.TenantAware, "tenant awareness should be disabled by default")
	assert.Nil(t, cfg.DB, "DB should be nil by default")
	assert.Equal(t, "", cfg.TableName, "table name should be empty by default")
}

func TestRepositoryConfigBuilder_WithTableName(t *testing.T) {
	cfg := NewRepositoryConfig().WithTableName("products").Build()
	assert.Equal(t, "products", cfg.TableName)
}

func TestRepositoryConfigBuilder_WithSoftDelete(t *testing.T) {
	cfgEnabled := NewRepositoryConfig().WithSoftDelete(true).Build()
	assert.True(t, cfgEnabled.EnableSoftDelete)

	cfgDisabled := NewRepositoryConfig().WithSoftDelete(false).Build()
	assert.False(t, cfgDisabled.EnableSoftDelete)
}

func TestRepositoryConfigBuilder_WithAudit(t *testing.T) {
	cfg := NewRepositoryConfig().WithAudit(true).Build()
	assert.True(t, cfg.EnableAudit)

	cfg2 := NewRepositoryConfig().WithAudit(false).Build()
	assert.False(t, cfg2.EnableAudit)
}

func TestRepositoryConfigBuilder_WithCache(t *testing.T) {
	cfg := NewRepositoryConfig().WithCache(true).Build()
	assert.True(t, cfg.EnableCache)

	cfg2 := NewRepositoryConfig().WithCache(false).Build()
	assert.False(t, cfg2.EnableCache)
}

func TestRepositoryConfigBuilder_WithTenantAware(t *testing.T) {
	cfg := NewRepositoryConfig().WithTenantAware(true).Build()
	assert.True(t, cfg.TenantAware)

	cfg2 := NewRepositoryConfig().WithTenantAware(false).Build()
	assert.False(t, cfg2.TenantAware)
}

func TestRepositoryConfigBuilder_Chaining(t *testing.T) {
	cfg := NewRepositoryConfig().
		WithTableName("staff").
		WithSoftDelete(true).
		WithAudit(true).
		WithCache(false).
		WithTenantAware(true).
		Build()

	assert.Equal(t, "staff", cfg.TableName)
	assert.True(t, cfg.EnableSoftDelete)
	assert.True(t, cfg.EnableAudit)
	assert.False(t, cfg.EnableCache)
	assert.True(t, cfg.TenantAware)
}

func TestRepositoryConfigBuilder_WithDB_NilAccepted(t *testing.T) {
	// Passing nil should not panic; it is a valid initial value before a real
	// DB is provided later.
	cfg := NewRepositoryConfig().WithDB(nil).Build()
	assert.Nil(t, cfg.DB)
}

// --- RepositoryFactory (via DefaultRepositoryFactory) ---

func TestNewRepositoryFactory_GetDB_ReturnsNil_WhenNilPassed(t *testing.T) {
	factory := NewRepositoryFactory(nil)
	assert.NotNil(t, factory, "factory should not be nil")
	assert.Nil(t, factory.GetDB(), "GetDB should return the db that was passed in")
}

// --- ServiceRepositoryManager ---

func TestNewServiceRepositoryManager_NonNil(t *testing.T) {
	mgr := NewServiceRepositoryManager(nil)
	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.repositories)
	assert.Empty(t, mgr.repositories)
}

func TestGetRepository_NotFound(t *testing.T) {
	mgr := NewServiceRepositoryManager(nil)

	_, err := GetRepository[*BaseEntity](mgr, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestGetTenantRepository_NotFound(t *testing.T) {
	mgr := NewServiceRepositoryManager(nil)

	_, err := GetTenantRepository[*BaseEntity](mgr, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

// --- RepositoryRegistry ---

func TestNewRepositoryRegistry_NonNil(t *testing.T) {
	rr := NewRepositoryRegistry(nil)
	assert.NotNil(t, rr)
	assert.NotNil(t, rr.manager)
}

// TestGet_PanicsWhenMissing verifies that Get() panics for unknown repositories,
// which is the documented contract in the source.
func TestGet_PanicsWhenMissing(t *testing.T) {
	rr := NewRepositoryRegistry(nil)

	assert.Panics(t, func() {
		Get[*BaseEntity](rr, "missing")
	})
}

// TestGetTenant_PanicsWhenMissing verifies that GetTenant() panics for unknown
// repositories.
func TestGetTenant_PanicsWhenMissing(t *testing.T) {
	rr := NewRepositoryRegistry(nil)

	assert.Panics(t, func() {
		GetTenant[*BaseEntity](rr, "missing")
	})
}

// --- ReadOnlyRepositoryAdapter ---

func TestReadOnlyRepositoryAdapter_IsNotNil(t *testing.T) {
	// Verify that wrapping a nil inner repository does not panic at
	// construction time.
	adapter := &ReadOnlyRepositoryAdapter[*BaseEntity]{
		repository: nil,
	}
	assert.NotNil(t, adapter)
}

// --- TenantContext helpers (factory layer) ---

func TestTenantMigrator_NonNil(t *testing.T) {
	// NewTenantMigrator wraps a *gorm.DB; verify it doesn't panic with nil.
	tm := NewTenantMigrator(nil)
	assert.NotNil(t, tm)
}
