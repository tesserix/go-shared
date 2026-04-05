package gdpr

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestConsentPurposeConstants verifies every predefined ConsentPurpose constant
// is defined and has a non-empty string value.
func TestConsentPurposeConstants(t *testing.T) {
	tests := []struct {
		name    string
		purpose ConsentPurpose
		want    string
	}{
		{"Marketing", ConsentPurposeMarketing, "MARKETING"},
		{"Analytics", ConsentPurposeAnalytics, "ANALYTICS"},
		{"Personalization", ConsentPurposePersonalization, "PERSONALIZATION"},
		{"ThirdParty", ConsentPurposeThirdParty, "THIRD_PARTY"},
		{"Essential", ConsentPurposeEssential, "ESSENTIAL"},
		{"Performance", ConsentPurposePerformance, "PERFORMANCE"},
		{"Functional", ConsentPurposeFunctional, "FUNCTIONAL"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, ConsentPurpose(tc.want), tc.purpose)
			assert.NotEmpty(t, string(tc.purpose))
		})
	}
}

// TestConsentPurposeDistinctness verifies every ConsentPurpose constant is
// unique so accidental equality comparisons behave correctly.
func TestConsentPurposeDistinctness(t *testing.T) {
	purposes := []ConsentPurpose{
		ConsentPurposeMarketing,
		ConsentPurposeAnalytics,
		ConsentPurposePersonalization,
		ConsentPurposeThirdParty,
		ConsentPurposeEssential,
		ConsentPurposePerformance,
		ConsentPurposeFunctional,
	}

	seen := make(map[ConsentPurpose]bool)
	for _, p := range purposes {
		assert.False(t, seen[p], "ConsentPurpose %q must be unique", p)
		seen[p] = true
	}
}

// TestConsentRecord_IsExpired tests the IsExpired method across several
// expiry-time scenarios.
func TestConsentRecord_IsExpired(t *testing.T) {
	now := time.Now().UTC()
	past := now.Add(-24 * time.Hour)
	future := now.Add(24 * time.Hour)

	tests := []struct {
		name      string
		expiresAt *time.Time
		want      bool
	}{
		{
			name:      "nil ExpiresAt never expires",
			expiresAt: nil,
			want:      false,
		},
		{
			name:      "future ExpiresAt not expired",
			expiresAt: &future,
			want:      false,
		},
		{
			name:      "past ExpiresAt is expired",
			expiresAt: &past,
			want:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			record := &ConsentRecord{ExpiresAt: tc.expiresAt}
			assert.Equal(t, tc.want, record.IsExpired())
		})
	}
}

// TestConsentRecord_IsActive tests the IsActive method which combines the
// Granted flag and the IsExpired check.
func TestConsentRecord_IsActive(t *testing.T) {
	now := time.Now().UTC()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	tests := []struct {
		name      string
		granted   bool
		expiresAt *time.Time
		want      bool
	}{
		{
			name:      "granted with no expiry is active",
			granted:   true,
			expiresAt: nil,
			want:      true,
		},
		{
			name:      "granted with future expiry is active",
			granted:   true,
			expiresAt: &future,
			want:      true,
		},
		{
			name:      "granted but expired is not active",
			granted:   true,
			expiresAt: &past,
			want:      false,
		},
		{
			name:      "not granted with no expiry is not active",
			granted:   false,
			expiresAt: nil,
			want:      false,
		},
		{
			name:      "not granted with future expiry is not active",
			granted:   false,
			expiresAt: &future,
			want:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			record := &ConsentRecord{
				Granted:   tc.granted,
				ExpiresAt: tc.expiresAt,
			}
			assert.Equal(t, tc.want, record.IsActive())
		})
	}
}

// TestConsentRecord_TableName verifies the model uses the correct database
// table name.
func TestConsentRecord_TableName(t *testing.T) {
	r := ConsentRecord{}
	assert.Equal(t, "gdpr_consent_records", r.TableName())
}

// TestConsentRecord_ZeroValueIsInactive verifies that the zero value of a
// ConsentRecord (unset fields) is treated as inactive and not expired.
func TestConsentRecord_ZeroValueIsInactive(t *testing.T) {
	record := &ConsentRecord{}
	assert.False(t, record.IsExpired(), "zero value must not report as expired (no ExpiresAt)")
	assert.False(t, record.IsActive(), "zero value must be inactive (Granted is false)")
}

// TestConsentRequest_StructFields verifies that a ConsentRequest struct can be
// populated and read back without field-ordering or tagging issues.
func TestConsentRequest_StructFields(t *testing.T) {
	expiresIn := 30
	req := ConsentRequest{
		TenantID:          "tenant-1",
		UserID:            "user-abc",
		Purpose:           ConsentPurposeMarketing,
		Granted:           true,
		Version:           "v1.0",
		Source:            "web",
		IPAddress:         "192.168.1.1",
		UserAgent:         "Mozilla/5.0",
		LegalBasis:        "consent",
		ExpiresInDays:     &expiresIn,
		DataCategories:    []DataCategory{DataCategoryMarketing, DataCategoryTechnical},
		ProcessingPurpose: "Send promotional emails",
	}

	assert.Equal(t, "tenant-1", req.TenantID)
	assert.Equal(t, "user-abc", req.UserID)
	assert.Equal(t, ConsentPurposeMarketing, req.Purpose)
	assert.True(t, req.Granted)
	assert.Equal(t, "v1.0", req.Version)
	assert.Equal(t, "web", req.Source)
	assert.Equal(t, "192.168.1.1", req.IPAddress)
	assert.Equal(t, 30, *req.ExpiresInDays)
	assert.Len(t, req.DataCategories, 2)
	assert.Equal(t, DataCategoryMarketing, req.DataCategories[0])
}

// TestConsentSummary_StructFields verifies the ConsentSummary struct populates
// correctly for use in response serialisation.
func TestConsentSummary_StructFields(t *testing.T) {
	now := time.Now().UTC()
	summary := ConsentSummary{
		UserID:       "user-abc",
		TotalCount:   5,
		GrantedCount: 3,
		ExpiredCount: 1,
		ByPurpose: map[ConsentPurpose]bool{
			ConsentPurposeMarketing: true,
			ConsentPurposeAnalytics: false,
		},
		LastUpdated: &now,
	}

	assert.Equal(t, "user-abc", summary.UserID)
	assert.Equal(t, 5, summary.TotalCount)
	assert.Equal(t, 3, summary.GrantedCount)
	assert.Equal(t, 1, summary.ExpiredCount)
	assert.True(t, summary.ByPurpose[ConsentPurposeMarketing])
	assert.False(t, summary.ByPurpose[ConsentPurposeAnalytics])
	assert.NotNil(t, summary.LastUpdated)
}

// TestDataAuditActionConstants verifies all AuditAction constants.
func TestDataAuditActionConstants(t *testing.T) {
	tests := []struct {
		name   string
		action AuditAction
		want   string
	}{
		{"ConsentGranted", AuditActionConsentGranted, "CONSENT_GRANTED"},
		{"ConsentWithdrawn", AuditActionConsentWithdrawn, "CONSENT_WITHDRAWN"},
		{"ConsentUpdated", AuditActionConsentUpdated, "CONSENT_UPDATED"},
		{"DataExported", AuditActionDataExported, "DATA_EXPORTED"},
		{"DataDeleted", AuditActionDataDeleted, "DATA_DELETED"},
		{"DataAnonymized", AuditActionDataAnonymized, "DATA_ANONYMIZED"},
		{"DeletionRequest", AuditActionDeletionRequest, "DELETION_REQUEST"},
		{"RetentionApplied", AuditActionRetentionApplied, "RETENTION_APPLIED"},
		{"AccessRequest", AuditActionAccessRequest, "ACCESS_REQUEST"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, AuditAction(tc.want), tc.action)
			assert.NotEmpty(t, string(tc.action))
		})
	}
}

// TestAuditFilter_StructFields verifies that AuditFilter can be populated and
// its zero value does not panic or cause unexpected behaviour.
func TestAuditFilter_StructFields(t *testing.T) {
	start := time.Now().Add(-24 * time.Hour)
	end := time.Now()

	filter := AuditFilter{
		Actions:     []AuditAction{AuditActionConsentGranted, AuditActionDataExported},
		StartDate:   &start,
		EndDate:     &end,
		EntityType:  "consent",
		PerformedBy: "admin-user",
		Limit:       50,
		Offset:      10,
	}

	assert.Len(t, filter.Actions, 2)
	assert.Equal(t, "consent", filter.EntityType)
	assert.Equal(t, "admin-user", filter.PerformedBy)
	assert.Equal(t, 50, filter.Limit)
	assert.Equal(t, 10, filter.Offset)
	assert.NotNil(t, filter.StartDate)
	assert.NotNil(t, filter.EndDate)
}

// TestAuditFilter_ZeroValue verifies the zero value of AuditFilter is safe.
func TestAuditFilter_ZeroValue(t *testing.T) {
	var filter AuditFilter
	assert.Empty(t, filter.Actions)
	assert.Nil(t, filter.StartDate)
	assert.Nil(t, filter.EndDate)
	assert.Equal(t, 0, filter.Limit)
}

// TestAuditLogger_VerifyIntegrity verifies that VerifyAuditLogIntegrity accepts
// a valid checksum and rejects a tampered entry.
func TestAuditLogger_VerifyIntegrity(t *testing.T) {
	logger := &AuditLogger{serviceName: "svc"}

	performedAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	entry := GDPRAuditLog{
		ID:          "test-id-001",
		UserID:      "user-xyz",
		Action:      AuditActionConsentGranted,
		EntityType:  "consent",
		EntityID:    "entity-001",
		PerformedAt: performedAt,
	}

	// Compute the checksum using the same formula as the production code.
	checksumData := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		entry.ID, entry.UserID, entry.Action, entry.EntityType, entry.EntityID,
		entry.PerformedAt.Format(time.RFC3339))
	hash := sha256.Sum256([]byte(checksumData))
	entry.Checksum = hex.EncodeToString(hash[:])

	assert.True(t, logger.VerifyAuditLogIntegrity(entry), "integrity check must pass for valid checksum")

	// Tamper with the entry and verify detection.
	tamperedEntry := entry
	tamperedEntry.UserID = "hacker"
	assert.False(t, logger.VerifyAuditLogIntegrity(tamperedEntry), "integrity check must fail for tampered entry")
}

// TestConsentManagerConstruction verifies ConsentManager can be created from
// the underlying struct fields without requiring a real database.
func TestConsentManagerConstruction(t *testing.T) {
	cm := &ConsentManager{serviceName: "order-service"}
	assert.Equal(t, "order-service", cm.serviceName)
}

// TestNewAuditLogger verifies NewAuditLogger sets the service name correctly.
func TestNewAuditLogger(t *testing.T) {
	logger := NewAuditLogger(nil, "test-service")
	assert.NotNil(t, logger)
	assert.Equal(t, "test-service", logger.serviceName)
}
