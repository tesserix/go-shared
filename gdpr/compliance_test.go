package gdpr

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRequestTypeConstants verifies all GDPR request type constants correspond
// to the correct GDPR article identifiers.
func TestRequestTypeConstants(t *testing.T) {
	tests := []struct {
		name    string
		reqType RequestType
		want    string
	}{
		{"Access (Article 15)", RequestTypeAccess, "ACCESS"},
		{"Rectify (Article 16)", RequestTypeRectify, "RECTIFY"},
		{"Erasure (Article 17)", RequestTypeErasure, "ERASURE"},
		{"Restrict (Article 18)", RequestTypeRestrict, "RESTRICT"},
		{"Portable (Article 20)", RequestTypePortable, "PORTABLE"},
		{"Object (Article 21)", RequestTypeObject, "OBJECT"},
		{"Automated (Article 22)", RequestTypeAutomated, "AUTOMATED"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, RequestType(tc.want), tc.reqType)
			assert.NotEmpty(t, string(tc.reqType))
		})
	}
}

// TestRequestTypeDistinctness verifies that every RequestType constant is unique.
func TestRequestTypeDistinctness(t *testing.T) {
	types := []RequestType{
		RequestTypeAccess,
		RequestTypeRectify,
		RequestTypeErasure,
		RequestTypeRestrict,
		RequestTypePortable,
		RequestTypeObject,
		RequestTypeAutomated,
	}

	seen := make(map[RequestType]bool)
	for _, rt := range types {
		assert.False(t, seen[rt], "RequestType %q must be unique", rt)
		seen[rt] = true
	}
}

// TestRequestStatusConstants verifies all GDPR request status constants are
// defined with the expected string values.
func TestRequestStatusConstants(t *testing.T) {
	tests := []struct {
		name   string
		status RequestStatus
		want   string
	}{
		{"Pending", RequestStatusPending, "PENDING"},
		{"Processing", RequestStatusProcessing, "PROCESSING"},
		{"Completed", RequestStatusCompleted, "COMPLETED"},
		{"Rejected", RequestStatusRejected, "REJECTED"},
		{"Failed", RequestStatusFailed, "FAILED"},
		{"Cancelled", RequestStatusCancelled, "CANCELLED"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, RequestStatus(tc.want), tc.status)
			assert.NotEmpty(t, string(tc.status))
		})
	}
}

// TestRequestStatusDistinctness verifies every RequestStatus constant is unique.
func TestRequestStatusDistinctness(t *testing.T) {
	statuses := []RequestStatus{
		RequestStatusPending,
		RequestStatusProcessing,
		RequestStatusCompleted,
		RequestStatusRejected,
		RequestStatusFailed,
		RequestStatusCancelled,
	}

	seen := make(map[RequestStatus]bool)
	for _, s := range statuses {
		assert.False(t, seen[s], "RequestStatus %q must be unique", s)
		seen[s] = true
	}
}

// TestDataCategoryConstants verifies each DataCategory constant has the correct
// string representation.
func TestDataCategoryConstants(t *testing.T) {
	tests := []struct {
		name     string
		category DataCategory
		want     string
	}{
		{"Identity", DataCategoryIdentity, "IDENTITY"},
		{"Contact", DataCategoryContact, "CONTACT"},
		{"Financial", DataCategoryFinancial, "FINANCIAL"},
		{"Transactional", DataCategoryTransactional, "TRANSACTIONAL"},
		{"Technical", DataCategoryTechnical, "TECHNICAL"},
		{"Usage", DataCategoryUsage, "USAGE"},
		{"Marketing", DataCategoryMarketing, "MARKETING"},
		{"Sensitive", DataCategorySensitive, "SENSITIVE"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, DataCategory(tc.want), tc.category)
			assert.NotEmpty(t, string(tc.category))
		})
	}
}

// TestDataCategoryDistinctness verifies every DataCategory constant is unique.
func TestDataCategoryDistinctness(t *testing.T) {
	categories := []DataCategory{
		DataCategoryIdentity,
		DataCategoryContact,
		DataCategoryFinancial,
		DataCategoryTransactional,
		DataCategoryTechnical,
		DataCategoryUsage,
		DataCategoryMarketing,
		DataCategorySensitive,
	}

	seen := make(map[DataCategory]bool)
	for _, c := range categories {
		assert.False(t, seen[c], "DataCategory %q must be unique", c)
		seen[c] = true
	}
}

// TestDataSubjectRequest_TableName verifies the correct GORM table name.
func TestDataSubjectRequest_TableName(t *testing.T) {
	req := DataSubjectRequest{}
	assert.Equal(t, "gdpr_data_subject_requests", req.TableName())
}

// TestRetentionPolicy_TableName verifies the correct GORM table name.
func TestRetentionPolicy_TableName(t *testing.T) {
	policy := RetentionPolicy{}
	assert.Equal(t, "gdpr_retention_policies", policy.TableName())
}

// TestGDPRAuditLog_TableName verifies the correct GORM table name.
func TestGDPRAuditLog_TableName(t *testing.T) {
	log := GDPRAuditLog{}
	assert.Equal(t, "gdpr_audit_logs", log.TableName())
}

// TestDataDeletionRequestModel_TableName verifies the correct GORM table name.
func TestDataDeletionRequestModel_TableName(t *testing.T) {
	req := DataDeletionRequest{}
	assert.Equal(t, "gdpr_data_deletion_requests", req.TableName())
}

// TestScheduledDeletionModel_TableName verifies the correct GORM table name.
func TestScheduledDeletionModel_TableName(t *testing.T) {
	sd := ScheduledDeletion{}
	assert.Equal(t, "gdpr_scheduled_deletions", sd.TableName())
}

// TestRetentionPolicy_CalculateExpiryDate verifies that the expiry date is
// computed as creation date plus the retention period in calendar days.
func TestRetentionPolicy_CalculateExpiryDate(t *testing.T) {
	tests := []struct {
		name          string
		retentionDays int
		createdAt     time.Time
		wantYear      int
		wantMonth     time.Month
		wantDay       int
	}{
		{
			name:          "365 days from Jan 1",
			retentionDays: 365,
			createdAt:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			wantYear:      2024,
			wantMonth:     12,
			wantDay:       31,
		},
		{
			name:          "7 years (2555 days) from Jan 1 2020",
			retentionDays: 7 * 365,
			createdAt:     time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			wantYear:      2026,
			wantMonth:     12,
			wantDay:       30,
		},
		{
			name:          "zero retention days returns same day",
			retentionDays: 0,
			createdAt:     time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
			wantYear:      2024,
			wantMonth:     6,
			wantDay:       15,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			policy := &RetentionPolicy{RetentionDays: tc.retentionDays}
			expiry := policy.CalculateExpiryDate(tc.createdAt)
			assert.Equal(t, tc.wantYear, expiry.Year())
			assert.Equal(t, tc.wantMonth, expiry.Month())
			assert.Equal(t, tc.wantDay, expiry.Day())
		})
	}
}

// TestDefaultConfig verifies that DefaultConfig returns sensible defaults.
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig(nil, "my-service")

	assert.Equal(t, "my-service", config.ServiceName)
	assert.Nil(t, config.DB) // nil is passed in, should be stored as-is
	assert.True(t, config.EnableAuditLog, "audit log must be enabled by default")
	assert.Greater(t, config.DefaultRetentionDays, 0, "default retention must be positive")
	assert.Greater(t, config.AuditLogRetentionDays, 0, "audit log retention must be positive")
	// 3 years = 1095 days
	assert.Equal(t, 365*3, config.DefaultRetentionDays)
	// 7 years = 2555 days
	assert.Equal(t, 365*7, config.AuditLogRetentionDays)
}

// TestContextWithTenant verifies that ContextWithTenant stores the tenant ID
// under the expected context key.
func TestContextWithTenant(t *testing.T) {
	ctx := context.Background()
	ctx = ContextWithTenant(ctx, "tenant-abc")

	tenantID, ok := ctx.Value("tenant_id").(string)
	assert.True(t, ok, "tenant_id must be stored as a string")
	assert.Equal(t, "tenant-abc", tenantID)
}

// TestExtractTenantID verifies the extractTenantID helper reads the correct
// key from context, and returns an empty string when the key is absent.
func TestExtractTenantID(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{
			name:     "tenant present",
			ctx:      ContextWithTenant(context.Background(), "tenant-xyz"),
			expected: "tenant-xyz",
		},
		{
			name:     "tenant absent",
			ctx:      context.Background(),
			expected: "",
		},
		{
			name:     "empty tenant string",
			ctx:      ContextWithTenant(context.Background(), ""),
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractTenantID(tc.ctx)
			assert.Equal(t, tc.expected, got)
		})
	}
}

// TestDataExportFromCompliance_StructFields verifies DataExport struct fields are accessible.
func TestDataExportFromCompliance_StructFields(t *testing.T) {
	now := time.Now().UTC()
	export := DataExport{
		UserID:      "user-1",
		ExportedAt:  now,
		DataSources: map[string]json.RawMessage{"orders": json.RawMessage(`[]`)},
		Metadata: ExportMetadata{
			Format:       "JSON",
			Version:      "1.0",
			GeneratedAt:  now,
			RequestID:    "req-001",
			TotalRecords: 10,
			DataTypes:    []string{"orders", "gdpr_compliance"},
		},
	}

	assert.Equal(t, "user-1", export.UserID)
	assert.Equal(t, "JSON", export.Metadata.Format)
	assert.Equal(t, "1.0", export.Metadata.Version)
	assert.Equal(t, "req-001", export.Metadata.RequestID)
	assert.Equal(t, 10, export.Metadata.TotalRecords)
	assert.Len(t, export.Metadata.DataTypes, 2)
}

// TestCleanupResult_StructFields verifies the CleanupResult struct.
func TestCleanupResult_StructFields(t *testing.T) {
	result := CleanupResult{
		ProcessedAt:     time.Now().UTC(),
		TotalProcessed:  100,
		TotalDeleted:    80,
		TotalAnonymized: 15,
		TotalFailed:     5,
		ByCategory: map[DataCategory]int{
			DataCategoryIdentity: 40,
			DataCategoryTechnical: 60,
		},
		Errors: []string{"user-1: delete failed"},
	}

	assert.Equal(t, 100, result.TotalProcessed)
	assert.Equal(t, 80, result.TotalDeleted)
	assert.Equal(t, 15, result.TotalAnonymized)
	assert.Equal(t, 5, result.TotalFailed)
	assert.Equal(t, 40, result.ByCategory[DataCategoryIdentity])
	assert.Len(t, result.Errors, 1)
}

// TestAnonymizationResult_StructFields verifies the AnonymizationResult struct.
func TestAnonymizationResult_StructFields(t *testing.T) {
	result := AnonymizationResult{
		UserID:           "user-xyz",
		ProcessedAt:      time.Now().UTC(),
		TotalRecords:     50,
		ByTable:          map[string]int{"users": 1, "orders": 49},
		AnonymizedFields: []string{"email", "phone", "ip_address"},
	}

	assert.Equal(t, "user-xyz", result.UserID)
	assert.Equal(t, 50, result.TotalRecords)
	assert.Equal(t, 1, result.ByTable["users"])
	assert.Len(t, result.AnonymizedFields, 3)
}

// TestDeletionVerification_StructFields verifies the DeletionVerification struct.
func TestDeletionVerification_StructFields(t *testing.T) {
	verification := DeletionVerification{
		RequestID:      "req-001",
		UserID:         "user-001",
		Verified:       true,
		VerifiedAt:     time.Now().UTC(),
		DeletedSources: []string{"orders_service", "products_service"},
		RetainedSources: []RetainedDataSource{
			{
				Source:      "gdpr_audit_logs",
				Reason:      "Legal compliance",
				LegalBasis:  "Regulatory obligation",
				RetainUntil: "7 years from creation",
			},
		},
	}

	assert.Equal(t, "req-001", verification.RequestID)
	assert.True(t, verification.Verified)
	assert.Len(t, verification.DeletedSources, 2)
	assert.Len(t, verification.RetainedSources, 1)
	assert.Equal(t, "gdpr_audit_logs", verification.RetainedSources[0].Source)
}

// TestGDPRStats_StructFields verifies the GDPRStats and nested stats structs.
func TestGDPRStats_StructFields(t *testing.T) {
	stats := GDPRStats{
		ConsentStats: ConsentStats{
			TotalConsents:   100,
			ActiveConsents:  80,
			ExpiredConsents: 5,
			ByPurpose: map[ConsentPurpose]int64{
				ConsentPurposeMarketing: 40,
				ConsentPurposeAnalytics: 60,
			},
		},
		RetentionStats: RetentionStats{
			ActivePolicies:     6,
			ScheduledDeletions: 10,
			CompletedDeletions: 200,
		},
	}

	assert.Equal(t, int64(100), stats.ConsentStats.TotalConsents)
	assert.Equal(t, int64(80), stats.ConsentStats.ActiveConsents)
	assert.Equal(t, int64(40), stats.ConsentStats.ByPurpose[ConsentPurposeMarketing])
	assert.Equal(t, int64(6), stats.RetentionStats.ActivePolicies)
}

// TestComplianceReport_StructFields verifies the ComplianceReport struct.
func TestComplianceReport_StructFields(t *testing.T) {
	report := ComplianceReport{
		GeneratedAt: "2024-01-01",
		TenantID:    "tenant-1",
		Period:      "2024-Q1",
		AuditSummary: AuditSummary{
			TotalEntries: 500,
			ByAction: map[AuditAction]int64{
				AuditActionConsentGranted: 200,
				AuditActionDataExported:   50,
			},
		},
	}

	assert.Equal(t, "tenant-1", report.TenantID)
	assert.Equal(t, "2024-Q1", report.Period)
	assert.Equal(t, int64(500), report.AuditSummary.TotalEntries)
	assert.Equal(t, int64(200), report.AuditSummary.ByAction[AuditActionConsentGranted])
}

// TestDeletionRequestFromCompliance_StructFields verifies DeletionRequest can be fully
// populated and read back correctly.
func TestDeletionRequestFromCompliance_StructFields(t *testing.T) {
	scheduleFor := time.Now().Add(30 * 24 * time.Hour)
	req := DeletionRequest{
		TenantID:       "tenant-abc",
		UserID:         "user-xyz",
		DataCategories: []DataCategory{DataCategoryIdentity, DataCategoryMarketing},
		Reason:         "User requested account deletion",
		ScheduleFor:    &scheduleFor,
		IPAddress:      "10.0.0.1",
		UserAgent:      "Chrome/120",
	}

	assert.Equal(t, "tenant-abc", req.TenantID)
	assert.Equal(t, "user-xyz", req.UserID)
	assert.Len(t, req.DataCategories, 2)
	assert.Equal(t, DataCategoryIdentity, req.DataCategories[0])
	assert.Contains(t, req.Reason, "deletion")
	assert.NotNil(t, req.ScheduleFor)
}

// TestGDPRServiceInterface verifies that the Service type satisfies the
// GDPRService interface at compile time.
func TestGDPRServiceInterface(t *testing.T) {
	// This test is enforced at compile time by the type assertion below.
	// If Service does not implement GDPRService, the build will fail.
	var _ GDPRService = (*Service)(nil)
	// A nil pointer of *Service satisfies the interface — this is the standard
	// Go compile-time interface check pattern.
	require.True(t, true, "Service satisfies GDPRService interface (compile-time check)")
}

// TestNewServiceConstruction verifies that NewService initialises all internal
// managers when given a Config with a nil DB (managers are constructed, not nil).
func TestNewServiceConstruction(t *testing.T) {
	config := Config{
		DB:                   nil, // nil DB; we test construction only
		ServiceName:          "test-service",
		DefaultRetentionDays: 365,
		EnableAuditLog:       true,
	}

	svc := NewService(config)
	require.NotNil(t, svc)
	assert.Equal(t, "test-service", svc.config.ServiceName)
	assert.NotNil(t, svc.consentManager)
	assert.NotNil(t, svc.dataExporter)
	assert.NotNil(t, svc.retentionManager)
	assert.NotNil(t, svc.deletionManager)
	assert.NotNil(t, svc.auditLogger)
}
