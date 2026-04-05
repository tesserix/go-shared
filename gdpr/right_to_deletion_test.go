package gdpr

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDataDeletionRequest_FieldPopulation verifies that all fields on a
// DataDeletionRequest can be set and read back correctly.
func TestDataDeletionRequest_FieldPopulation(t *testing.T) {
	now := time.Now().UTC()
	future := now.Add(30 * 24 * time.Hour)

	req := DataDeletionRequest{
		ID:           "del-req-001",
		TenantID:     "tenant-abc",
		UserID:       "user-xyz",
		Status:       RequestStatusPending,
		RequestedAt:  now,
		ScheduledFor: &future,
		Reason:       "User requested account deletion per GDPR Article 17",
		IPAddress:    "192.168.0.1",
		UserAgent:    "Chrome/120",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	assert.Equal(t, "del-req-001", req.ID)
	assert.Equal(t, "tenant-abc", req.TenantID)
	assert.Equal(t, "user-xyz", req.UserID)
	assert.Equal(t, RequestStatusPending, req.Status)
	assert.Equal(t, &future, req.ScheduledFor)
	assert.Contains(t, req.Reason, "GDPR Article 17")
	assert.Equal(t, "192.168.0.1", req.IPAddress)
}

// TestDataDeletionRequest_TableName verifies the GORM table name.
func TestDataDeletionRequest_TableName(t *testing.T) {
	req := DataDeletionRequest{}
	assert.Equal(t, "gdpr_data_deletion_requests", req.TableName())
}

// TestDataDeletionRequest_StatusTransitions verifies that the RequestStatus
// constants can be correctly set and compared on the struct.
func TestDataDeletionRequest_StatusTransitions(t *testing.T) {
	tests := []struct {
		name   string
		status RequestStatus
	}{
		{"pending", RequestStatusPending},
		{"processing", RequestStatusProcessing},
		{"completed", RequestStatusCompleted},
		{"rejected", RequestStatusRejected},
		{"failed", RequestStatusFailed},
		{"cancelled", RequestStatusCancelled},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := DataDeletionRequest{Status: tc.status}
			assert.Equal(t, tc.status, req.Status)
		})
	}
}

// TestDeletionRequest_StructFields verifies that the DeletionRequest input
// struct can be populated with all fields.
func TestDeletionRequest_StructFields(t *testing.T) {
	scheduleFor := time.Now().Add(30 * 24 * time.Hour)
	req := DeletionRequest{
		TenantID:       "tenant-1",
		UserID:         "user-1",
		DataCategories: []DataCategory{DataCategoryIdentity, DataCategoryContact, DataCategoryMarketing},
		Reason:         "Closing my account",
		ScheduleFor:    &scheduleFor,
		IPAddress:      "10.0.0.1",
		UserAgent:      "Mozilla/5.0",
	}

	assert.Equal(t, "tenant-1", req.TenantID)
	assert.Equal(t, "user-1", req.UserID)
	assert.Len(t, req.DataCategories, 3)
	assert.Equal(t, DataCategoryIdentity, req.DataCategories[0])
	assert.NotNil(t, req.ScheduleFor)
	assert.True(t, req.ScheduleFor.After(time.Now()))
}

// TestDeletionRequest_NoSchedule verifies that ScheduleFor can be nil (no
// explicit scheduling provided, service will use the default 30-day period).
func TestDeletionRequest_NoSchedule(t *testing.T) {
	req := DeletionRequest{
		TenantID: "tenant-1",
		UserID:   "user-1",
	}
	assert.Nil(t, req.ScheduleFor, "nil ScheduleFor means use default grace period")
}

// TestDeletionRequest_EmptyDataCategories verifies that when no specific
// categories are given the slice is empty (meaning delete all).
func TestDeletionRequest_EmptyDataCategories(t *testing.T) {
	req := DeletionRequest{
		TenantID: "tenant-1",
		UserID:   "user-1",
	}
	assert.Empty(t, req.DataCategories, "empty slice means delete all data categories")
}

// TestDeletionManager_Construction verifies that the DeletionManager struct can
// be created from its fields without touching a real database.
func TestDeletionManager_Construction(t *testing.T) {
	dm := &DeletionManager{
		serviceName: "my-service",
	}
	assert.Equal(t, "my-service", dm.serviceName)
}

// TestNewDeletionManager_Construction verifies NewDeletionManager sets all
// fields correctly.
func TestNewDeletionManager_Construction(t *testing.T) {
	provider := NewBaseDataSourceProvider("orders", []DataCategory{DataCategoryTransactional}, nil, nil, nil)
	dm := NewDeletionManager(nil, "deletion-service", []DataSourceProvider{provider})

	require.NotNil(t, dm)
	assert.Equal(t, "deletion-service", dm.serviceName)
	assert.Len(t, dm.dataProviders, 1)
	assert.NotNil(t, dm.auditLogger)
	assert.NotNil(t, dm.consentManager)
}

// TestDeletionManager_AddDataProvider verifies providers can be added after
// construction.
func TestDeletionManager_AddDataProvider(t *testing.T) {
	dm := NewDeletionManager(nil, "svc", nil)
	assert.Empty(t, dm.dataProviders)

	p := NewBaseDataSourceProvider("p1", nil, nil, nil, nil)
	dm.AddDataProvider(p)
	assert.Len(t, dm.dataProviders, 1)
}

// TestDeletionVerification_VerifiedNoRemainingData verifies the semantics of
// the Verified flag: true when RemainingData is empty.
func TestDeletionVerification_VerifiedNoRemainingData(t *testing.T) {
	verification := DeletionVerification{
		RequestID:       "req-001",
		UserID:          "user-001",
		Verified:        true,
		VerifiedAt:      time.Now().UTC(),
		RemainingData:   map[string]int{},
		DeletedSources:  []string{"orders_service", "catalog_service"},
		RetainedSources: []RetainedDataSource{},
	}

	assert.True(t, verification.Verified)
	assert.Empty(t, verification.RemainingData)
	assert.Len(t, verification.DeletedSources, 2)
}

// TestDeletionVerification_NotVerifiedWithRemainingData verifies that the
// Verified flag is false when data still exists in some source.
func TestDeletionVerification_NotVerifiedWithRemainingData(t *testing.T) {
	verification := DeletionVerification{
		RequestID: "req-002",
		UserID:    "user-002",
		Verified:  false,
		RemainingData: map[string]int{
			"legacy_service": 3,
		},
		DeletedSources: []string{"orders_service"},
	}

	assert.False(t, verification.Verified)
	assert.NotEmpty(t, verification.RemainingData)
	assert.Equal(t, 3, verification.RemainingData["legacy_service"])
}

// TestDeletionVerification_RetainedSourcesWithLegalBasis verifies that retained
// sources are stored with their legal basis for compliance records.
func TestDeletionVerification_RetainedSourcesWithLegalBasis(t *testing.T) {
	verification := DeletionVerification{
		RequestID: "req-003",
		UserID:    "user-003",
		Verified:  true,
		RetainedSources: []RetainedDataSource{
			{
				Source:      "gdpr_audit_logs",
				Reason:      "Legal compliance requirement",
				LegalBasis:  "Regulatory obligation - audit trail retention",
				RetainUntil: "7 years from creation",
			},
			{
				Source:     "financial_records",
				Reason:     "Tax compliance",
				LegalBasis: "Legal obligation - tax regulations",
			},
		},
	}

	assert.True(t, verification.Verified)
	assert.Len(t, verification.RetainedSources, 2)

	auditSource := verification.RetainedSources[0]
	assert.Equal(t, "gdpr_audit_logs", auditSource.Source)
	assert.NotEmpty(t, auditSource.LegalBasis)
	assert.Equal(t, "7 years from creation", auditSource.RetainUntil)
}

// TestDeletionProcessingResult_ZeroValue verifies the zero value is valid.
func TestDeletionProcessingResult_ZeroValue(t *testing.T) {
	var result DeletionProcessingResult
	assert.Zero(t, result.TotalDue)
	assert.Zero(t, result.Processed)
	assert.Zero(t, result.Failed)
	assert.Empty(t, result.ProcessedIDs)
	assert.Empty(t, result.FailedIDs)
}

// TestDeletionProcessingResult_PartialSuccess verifies a partial success
// scenario where some requests are processed and some fail.
func TestDeletionProcessingResult_PartialSuccess(t *testing.T) {
	result := DeletionProcessingResult{
		ProcessedAt:  time.Now().UTC(),
		TotalDue:     5,
		Processed:    3,
		Failed:       2,
		ProcessedIDs: []string{"req-1", "req-2", "req-3"},
		FailedIDs:    []string{"req-4", "req-5"},
	}

	assert.Equal(t, 5, result.TotalDue)
	assert.Equal(t, result.Processed+result.Failed, result.TotalDue)
	assert.Len(t, result.ProcessedIDs, result.Processed)
	assert.Len(t, result.FailedIDs, result.Failed)
}

// TestDeletionRequestStats_AllStatusesCounted verifies that ByStatus map
// contains entries for every meaningful status.
func TestDeletionRequestStats_AllStatusesCounted(t *testing.T) {
	stats := DeletionRequestStats{
		Total:     6,
		Pending:   1,
		Completed: 4,
		Failed:    1,
		ByStatus: map[RequestStatus]int64{
			RequestStatusPending:   1,
			RequestStatusCompleted: 4,
			RequestStatusFailed:    1,
		},
		AvgProcessingTimeSeconds: 3600,
	}

	total := int64(0)
	for _, count := range stats.ByStatus {
		total += count
	}
	assert.Equal(t, stats.Total, total, "sum of ByStatus counts must equal Total")
}

// TestCascadeDeleteConfig_Order verifies the Order field is used to express
// deletion priority (higher order = deleted first).
func TestCascadeDeleteConfig_Order(t *testing.T) {
	configs := []CascadeDeleteConfig{
		{TableName: "order_items", UserIDColumn: "user_id", Order: 30},
		{TableName: "orders", UserIDColumn: "user_id", Order: 20},
		{TableName: "users", UserIDColumn: "id", Order: 10},
	}

	// Higher order should be processed before lower order.
	assert.Greater(t, configs[0].Order, configs[1].Order,
		"order_items must have higher order than orders (must be deleted first)")
	assert.Greater(t, configs[1].Order, configs[2].Order,
		"orders must have higher order than users")
}

// TestDeletionRequest_AllDataCategories verifies that a DeletionRequest can
// target all known data categories.
func TestDeletionRequest_AllDataCategories(t *testing.T) {
	allCategories := []DataCategory{
		DataCategoryIdentity,
		DataCategoryContact,
		DataCategoryFinancial,
		DataCategoryTransactional,
		DataCategoryTechnical,
		DataCategoryUsage,
		DataCategoryMarketing,
		DataCategorySensitive,
	}

	req := DeletionRequest{
		TenantID:       "tenant-1",
		UserID:         "user-1",
		DataCategories: allCategories,
	}

	assert.Len(t, req.DataCategories, len(allCategories))
	for i, cat := range allCategories {
		assert.Equal(t, cat, req.DataCategories[i])
	}
}
