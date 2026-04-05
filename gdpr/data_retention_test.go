package gdpr

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRetentionPolicy_FieldPopulation verifies that a RetentionPolicy struct
// can be created with all fields and read back without loss.
func TestRetentionPolicy_FieldPopulation(t *testing.T) {
	now := time.Now().UTC()
	policy := RetentionPolicy{
		ID:            "policy-001",
		TenantID:      "tenant-1",
		Name:          "Identity Data Retention",
		Description:   "Retention policy for PII",
		DataCategory:  DataCategoryIdentity,
		RetentionDays: 365 * 3,
		Action:        "ANONYMIZE",
		IsActive:      true,
		LegalBasis:    "Legitimate interest",
		CreatedBy:     "admin-001",
		LastModifiedBy: "admin-002",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	assert.Equal(t, "policy-001", policy.ID)
	assert.Equal(t, "tenant-1", policy.TenantID)
	assert.Equal(t, "Identity Data Retention", policy.Name)
	assert.Equal(t, DataCategoryIdentity, policy.DataCategory)
	assert.Equal(t, 365*3, policy.RetentionDays)
	assert.Equal(t, "ANONYMIZE", policy.Action)
	assert.True(t, policy.IsActive)
	assert.Equal(t, "Legitimate interest", policy.LegalBasis)
}

// TestRetentionPolicy_CalculateExpiryDate_Detailed provides additional
// table-driven cases for the CalculateExpiryDate method.
func TestRetentionPolicy_CalculateExpiryDate_Detailed(t *testing.T) {
	tests := []struct {
		name          string
		retentionDays int
		createdAt     time.Time
		wantDiff      time.Duration // approximate, tested within a day
	}{
		{
			name:          "1 year retention",
			retentionDays: 365,
			createdAt:     time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:          "7 years retention (financial)",
			retentionDays: 7 * 365,
			createdAt:     time.Date(2020, 3, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:          "30 days retention",
			retentionDays: 30,
			createdAt:     time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			policy := &RetentionPolicy{RetentionDays: tc.retentionDays}
			expiry := policy.CalculateExpiryDate(tc.createdAt)

			expectedExpiry := tc.createdAt.AddDate(0, 0, tc.retentionDays)
			assert.Equal(t, expectedExpiry, expiry,
				"CalculateExpiryDate must return createdAt + RetentionDays calendar days")
		})
	}
}

// TestScheduledDeletion_StructFields verifies ScheduledDeletion fields.
func TestScheduledDeletion_StructFields(t *testing.T) {
	now := time.Now().UTC()
	future := now.Add(30 * 24 * time.Hour)
	sd := ScheduledDeletion{
		ID:           "sched-001",
		TenantID:     "tenant-abc",
		UserID:       "user-xyz",
		DataCategory: DataCategoryTechnical,
		TargetTable:  "sessions",
		RecordID:     "session-999",
		ScheduledFor: future,
		Executed:     false,
		ExecutedAt:   nil,
		PolicyID:     "policy-001",
		CreatedAt:    now,
	}

	assert.Equal(t, "sched-001", sd.ID)
	assert.Equal(t, DataCategoryTechnical, sd.DataCategory)
	assert.Equal(t, future, sd.ScheduledFor)
	assert.False(t, sd.Executed)
	assert.Nil(t, sd.ExecutedAt)
}

// TestScheduledDeletion_TableName verifies the GORM table name.
func TestScheduledDeletion_TableName(t *testing.T) {
	sd := ScheduledDeletion{}
	assert.Equal(t, "gdpr_scheduled_deletions", sd.TableName())
}

// TestDefaultRetentionPolicies_Count verifies that DefaultRetentionPolicies
// returns the expected number of policies.
func TestDefaultRetentionPolicies_Count(t *testing.T) {
	policies := DefaultRetentionPolicies("tenant-1", "admin-001")
	assert.Len(t, policies, 6, "six default retention policies must be defined")
}

// TestDefaultRetentionPolicies_AllHaveRequiredFields verifies that every
// default policy has non-zero required fields.
func TestDefaultRetentionPolicies_AllHaveRequiredFields(t *testing.T) {
	policies := DefaultRetentionPolicies("tenant-abc", "creator-001")

	for _, policy := range policies {
		t.Run(string(policy.DataCategory), func(t *testing.T) {
			assert.NotEmpty(t, policy.Name, "policy must have a Name")
			assert.NotEmpty(t, policy.Description, "policy must have a Description")
			assert.NotEmpty(t, policy.DataCategory, "policy must have a DataCategory")
			assert.Greater(t, policy.RetentionDays, 0, "RetentionDays must be positive")
			assert.NotEmpty(t, policy.Action, "policy must specify an Action")
			assert.NotEmpty(t, policy.LegalBasis, "policy must specify a LegalBasis")
			assert.Equal(t, "tenant-abc", policy.TenantID)
			assert.Equal(t, "creator-001", policy.CreatedBy)
		})
	}
}

// TestDefaultRetentionPolicies_DataCategoryCoverage verifies that the default
// policies cover all major data categories.
func TestDefaultRetentionPolicies_DataCategoryCoverage(t *testing.T) {
	policies := DefaultRetentionPolicies("tenant-1", "admin")

	covered := make(map[DataCategory]bool)
	for _, p := range policies {
		covered[p.DataCategory] = true
	}

	expected := []DataCategory{
		DataCategoryIdentity,
		DataCategoryFinancial,
		DataCategoryTransactional,
		DataCategoryTechnical,
		DataCategoryUsage,
		DataCategoryMarketing,
	}

	for _, cat := range expected {
		assert.True(t, covered[cat], "default policies must cover category %q", cat)
	}
}

// TestDefaultRetentionPolicies_RetentionPeriods verifies the specific
// retention periods mandated by GDPR best-practice defaults.
func TestDefaultRetentionPolicies_RetentionPeriods(t *testing.T) {
	policies := DefaultRetentionPolicies("tenant-1", "admin")

	byCategory := make(map[DataCategory]RetentionPolicy)
	for _, p := range policies {
		byCategory[p.DataCategory] = p
	}

	tests := []struct {
		category      DataCategory
		wantDays      int
		wantAction    string
	}{
		{DataCategoryIdentity, 365 * 3, "ANONYMIZE"},
		{DataCategoryFinancial, 365 * 7, "ARCHIVE"},
		{DataCategoryTransactional, 365 * 5, "ANONYMIZE"},
		{DataCategoryTechnical, 365, "DELETE"},
		{DataCategoryUsage, 365 * 2, "ANONYMIZE"},
		{DataCategoryMarketing, 365 * 2, "DELETE"},
	}

	for _, tc := range tests {
		t.Run(string(tc.category), func(t *testing.T) {
			policy, ok := byCategory[tc.category]
			require.True(t, ok, "policy for category %q must exist", tc.category)
			assert.Equal(t, tc.wantDays, policy.RetentionDays,
				"retention period for %q", tc.category)
			assert.Equal(t, tc.wantAction, policy.Action,
				"action for %q", tc.category)
		})
	}
}

// TestDefaultRetentionPolicies_FinancialHasLongestRetention verifies that
// financial data has the longest mandated retention period (7 years) to satisfy
// tax and accounting regulatory requirements.
func TestDefaultRetentionPolicies_FinancialHasLongestRetention(t *testing.T) {
	policies := DefaultRetentionPolicies("tenant-1", "admin")

	var maxDays int
	var maxCategory DataCategory
	for _, p := range policies {
		if p.RetentionDays > maxDays {
			maxDays = p.RetentionDays
			maxCategory = p.DataCategory
		}
	}

	assert.Equal(t, DataCategoryFinancial, maxCategory,
		"financial data must have the longest retention period")
	assert.Equal(t, 365*7, maxDays)
}

// TestDefaultRetentionPolicies_TechnicalHasShortestRetention verifies that
// technical data (IP addresses, device info) has the shortest default retention.
func TestDefaultRetentionPolicies_TechnicalHasShortestRetention(t *testing.T) {
	policies := DefaultRetentionPolicies("tenant-1", "admin")

	var minDays int = 999999
	var minCategory DataCategory
	for _, p := range policies {
		if p.RetentionDays < minDays {
			minDays = p.RetentionDays
			minCategory = p.DataCategory
		}
	}

	assert.Equal(t, DataCategoryTechnical, minCategory,
		"technical data must have the shortest default retention")
	assert.Equal(t, 365, minDays)
}

// TestRetentionManager_Construction verifies the struct can be created from
// its fields without requiring a database.
func TestRetentionManager_Construction(t *testing.T) {
	rm := &RetentionManager{
		serviceName: "test-service",
	}
	assert.Equal(t, "test-service", rm.serviceName)
}

// TestRetentionManager_AddDataProvider verifies that AddDataProvider appends
// to the provider list.
func TestRetentionManager_AddDataProvider(t *testing.T) {
	rm := &RetentionManager{
		serviceName:   "test-service",
		dataProviders: nil,
	}

	provider := NewBaseDataSourceProvider(
		"orders-service",
		[]DataCategory{DataCategoryTransactional},
		nil, nil, nil,
	)

	rm.AddDataProvider(provider)
	assert.Len(t, rm.dataProviders, 1)

	rm.AddDataProvider(provider)
	assert.Len(t, rm.dataProviders, 2)
}

// TestNewRetentionManager_Construction verifies NewRetentionManager initialises
// the struct correctly (no DB calls).
func TestNewRetentionManager_Construction(t *testing.T) {
	provider := NewBaseDataSourceProvider(
		"catalog-service",
		[]DataCategory{DataCategoryIdentity},
		nil, nil, nil,
	)

	rm := NewRetentionManager(nil, "my-service", []DataSourceProvider{provider})
	require.NotNil(t, rm)
	assert.Equal(t, "my-service", rm.serviceName)
	assert.Len(t, rm.dataProviders, 1)
	assert.NotNil(t, rm.auditLogger)
}

// TestDeletionProcessingResult_StructFields verifies the result struct.
func TestDeletionProcessingResult_StructFields(t *testing.T) {
	result := DeletionProcessingResult{
		ProcessedAt:  time.Now().UTC(),
		TotalDue:     10,
		Processed:    8,
		Failed:       2,
		ProcessedIDs: []string{"id-1", "id-2", "id-3"},
		FailedIDs:    []string{"id-bad-1", "id-bad-2"},
	}

	assert.Equal(t, 10, result.TotalDue)
	assert.Equal(t, 8, result.Processed)
	assert.Equal(t, 2, result.Failed)
	assert.Len(t, result.ProcessedIDs, 3)
	assert.Len(t, result.FailedIDs, 2)
}

// TestCascadeDeleteConfig_StructFields verifies the CascadeDeleteConfig struct.
func TestCascadeDeleteConfig_StructFields(t *testing.T) {
	cfg := CascadeDeleteConfig{
		TableName:    "order_items",
		UserIDColumn: "customer_id",
		Order:        10,
	}

	assert.Equal(t, "order_items", cfg.TableName)
	assert.Equal(t, "customer_id", cfg.UserIDColumn)
	assert.Equal(t, 10, cfg.Order)
}

// TestDeletionRequestStats_StructFields verifies the DeletionRequestStats
// struct can be populated and all fields accessed.
func TestDeletionRequestStats_StructFields(t *testing.T) {
	stats := DeletionRequestStats{
		Total:     100,
		Pending:   10,
		Completed: 85,
		Failed:    5,
		ByStatus: map[RequestStatus]int64{
			RequestStatusPending:   10,
			RequestStatusCompleted: 85,
			RequestStatusFailed:    5,
		},
		AvgProcessingTimeSeconds: 43200.5,
	}

	assert.Equal(t, int64(100), stats.Total)
	assert.Equal(t, int64(10), stats.Pending)
	assert.Equal(t, int64(85), stats.Completed)
	assert.Equal(t, int64(5), stats.Failed)
	assert.Equal(t, int64(10), stats.ByStatus[RequestStatusPending])
	assert.InDelta(t, 43200.5, stats.AvgProcessingTimeSeconds, 0.001)
}
