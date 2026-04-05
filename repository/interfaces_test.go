package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- BaseEntity ---

func TestBaseEntity_GetSetID(t *testing.T) {
	e := &BaseEntity{}
	assert.Equal(t, "", e.GetID())

	e.SetID("abc-123")
	assert.Equal(t, "abc-123", e.GetID())
}

func TestBaseEntity_GetCreatedAt(t *testing.T) {
	now := time.Now()
	e := &BaseEntity{CreatedAt: now}
	assert.Equal(t, now, e.GetCreatedAt())
}

func TestBaseEntity_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	e := &BaseEntity{UpdatedAt: now}
	assert.Equal(t, now, e.GetUpdatedAt())
}

func TestBaseEntity_DeletedAt_Nullable(t *testing.T) {
	e := &BaseEntity{}
	assert.Nil(t, e.DeletedAt)

	ts := time.Now()
	e.DeletedAt = &ts
	assert.NotNil(t, e.DeletedAt)
	assert.Equal(t, ts, *e.DeletedAt)
}

// --- TenantEntity ---

func TestTenantEntity_EmbeddsBaseEntity(t *testing.T) {
	te := &TenantEntity{}
	te.SetID("entity-id")
	assert.Equal(t, "entity-id", te.GetID())
}

func TestTenantEntity_GetSetTenantID(t *testing.T) {
	te := &TenantEntity{}
	assert.Equal(t, "", te.GetTenantID())

	te.SetTenantID("tenant-xyz")
	assert.Equal(t, "tenant-xyz", te.GetTenantID())
}

// --- QueryOptions ---

func TestDefaultQueryOptions(t *testing.T) {
	opts := DefaultQueryOptions()

	assert.Equal(t, 1, opts.Page)
	assert.Equal(t, 10, opts.PageSize)
	assert.Equal(t, "created_at", opts.SortBy)
	assert.Equal(t, "desc", opts.SortOrder)
	assert.NotNil(t, opts.Filters)
	assert.Empty(t, opts.Filters)
}

func TestQueryOptions_ZeroValue(t *testing.T) {
	var opts QueryOptions
	assert.Equal(t, 0, opts.Page)
	assert.Equal(t, 0, opts.PageSize)
	assert.Equal(t, "", opts.SortBy)
	assert.Equal(t, "", opts.SortOrder)
	assert.Nil(t, opts.Filters)
	assert.False(t, opts.IncludeDeleted)
}

func TestQueryOptions_FilterAssignment(t *testing.T) {
	opts := DefaultQueryOptions()
	opts.Filters["status"] = "active"
	opts.Filters["type"] = "premium"

	assert.Equal(t, "active", opts.Filters["status"])
	assert.Equal(t, "premium", opts.Filters["type"])
}

func TestQueryOptions_SearchFields(t *testing.T) {
	opts := QueryOptions{
		Search:       "laptop",
		SearchFields: []string{"name", "description", "sku"},
	}

	assert.Equal(t, "laptop", opts.Search)
	assert.Len(t, opts.SearchFields, 3)
}

func TestQueryOptions_Preload(t *testing.T) {
	opts := QueryOptions{
		Preload: []string{"Category", "Images"},
	}
	assert.Equal(t, []string{"Category", "Images"}, opts.Preload)
}

// --- PaginatedResult ---

func TestPaginatedResult_HasNext_HasPrev(t *testing.T) {
	tests := []struct {
		name       string
		page       int
		totalPages int
		hasNext    bool
		hasPrev    bool
	}{
		{
			name:       "first page of many",
			page:       1,
			totalPages: 5,
			hasNext:    true,
			hasPrev:    false,
		},
		{
			name:       "middle page",
			page:       3,
			totalPages: 5,
			hasNext:    true,
			hasPrev:    true,
		},
		{
			name:       "last page",
			page:       5,
			totalPages: 5,
			hasNext:    false,
			hasPrev:    true,
		},
		{
			name:       "single page",
			page:       1,
			totalPages: 1,
			hasNext:    false,
			hasPrev:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := &PaginatedResult[*BaseEntity]{
				Page:       tc.page,
				TotalPages: tc.totalPages,
				HasNext:    tc.page < tc.totalPages,
				HasPrev:    tc.page > 1,
			}

			assert.Equal(t, tc.hasNext, result.HasNext)
			assert.Equal(t, tc.hasPrev, result.HasPrev)
		})
	}
}

func TestPaginatedResult_TotalPages_Calculation(t *testing.T) {
	tests := []struct {
		name      string
		total     int64
		pageSize  int
		wantPages int
	}{
		{"exact division", 100, 10, 10},
		{"with remainder", 101, 10, 11},
		{"single record", 1, 10, 1},
		{"empty set", 0, 10, 0},
		{"page size 1", 5, 1, 5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var totalPages int
			if tc.pageSize > 0 && tc.total > 0 {
				totalPages = int((tc.total + int64(tc.pageSize) - 1) / int64(tc.pageSize))
			}
			assert.Equal(t, tc.wantPages, totalPages)
		})
	}
}

// --- AuditEntry ---

func TestAuditEntry_StructFields(t *testing.T) {
	now := time.Now()
	entry := AuditEntry{
		ID:        "audit-1",
		EntityID:  "entity-1",
		Action:    "UPDATE",
		UserID:    "user-1",
		Changes:   map[string]interface{}{"salary": 50000},
		CreatedAt: now,
	}

	assert.Equal(t, "audit-1", entry.ID)
	assert.Equal(t, "entity-1", entry.EntityID)
	assert.Equal(t, "UPDATE", entry.Action)
	assert.Equal(t, "user-1", entry.UserID)
	assert.Equal(t, 50000, entry.Changes["salary"])
	assert.Equal(t, now, entry.CreatedAt)
}

// --- RepositoryConfig ---

func TestRepositoryConfig_ZeroValue(t *testing.T) {
	var cfg RepositoryConfig
	assert.Nil(t, cfg.DB)
	assert.Equal(t, "", cfg.TableName)
	assert.False(t, cfg.EnableSoftDelete)
	assert.False(t, cfg.EnableAudit)
	assert.False(t, cfg.EnableCache)
	assert.Equal(t, time.Duration(0), cfg.CacheTTL)
	assert.False(t, cfg.TenantAware)
}

// --- TenantContext ---

func TestWithTenantContext_And_GetTenantContext(t *testing.T) {
	tenantCtx := TenantContext{
		TenantID:   "tenant-42",
		TenantName: "Acme Corp",
		Schema:     "acme",
	}

	ctx := WithTenantContext(context.Background(), tenantCtx)
	retrieved, ok := GetTenantContext(ctx)

	assert.True(t, ok)
	assert.Equal(t, "tenant-42", retrieved.TenantID)
	assert.Equal(t, "Acme Corp", retrieved.TenantName)
	assert.Equal(t, "acme", retrieved.Schema)
}

func TestGetTenantContext_MissingFromContext(t *testing.T) {
	ctx := context.Background()
	_, ok := GetTenantContext(ctx)
	assert.False(t, ok)
}
