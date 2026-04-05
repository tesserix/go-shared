// Package gdpr provides comprehensive GDPR compliance functionality including
// consent management, data export, data retention, and right to deletion.
// This module is designed to be shared across services to ensure consistent
// GDPR compliance across the entire platform.
package gdpr

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// RequestType represents the type of GDPR request
type RequestType string

const (
	RequestTypeAccess    RequestType = "ACCESS"    // Article 15 - Right of access
	RequestTypeRectify   RequestType = "RECTIFY"   // Article 16 - Right to rectification
	RequestTypeErasure   RequestType = "ERASURE"   // Article 17 - Right to erasure
	RequestTypeRestrict  RequestType = "RESTRICT"  // Article 18 - Right to restriction
	RequestTypePortable  RequestType = "PORTABLE"  // Article 20 - Right to data portability
	RequestTypeObject    RequestType = "OBJECT"    // Article 21 - Right to object
	RequestTypeAutomated RequestType = "AUTOMATED" // Article 22 - Automated decision-making
)

// RequestStatus represents the status of a GDPR request
type RequestStatus string

const (
	RequestStatusPending    RequestStatus = "PENDING"
	RequestStatusProcessing RequestStatus = "PROCESSING"
	RequestStatusCompleted  RequestStatus = "COMPLETED"
	RequestStatusRejected   RequestStatus = "REJECTED"
	RequestStatusFailed     RequestStatus = "FAILED"
	RequestStatusCancelled  RequestStatus = "CANCELLED"
)

// ConsentPurpose represents predefined consent purposes
type ConsentPurpose string

const (
	ConsentPurposeMarketing       ConsentPurpose = "MARKETING"
	ConsentPurposeAnalytics       ConsentPurpose = "ANALYTICS"
	ConsentPurposePersonalization ConsentPurpose = "PERSONALIZATION"
	ConsentPurposeThirdParty      ConsentPurpose = "THIRD_PARTY"
	ConsentPurposeEssential       ConsentPurpose = "ESSENTIAL"
	ConsentPurposePerformance     ConsentPurpose = "PERFORMANCE"
	ConsentPurposeFunctional      ConsentPurpose = "FUNCTIONAL"
)

// DataCategory represents categories of personal data
type DataCategory string

const (
	DataCategoryIdentity      DataCategory = "IDENTITY"      // Name, email, phone, etc.
	DataCategoryContact       DataCategory = "CONTACT"       // Address, contact details
	DataCategoryFinancial     DataCategory = "FINANCIAL"     // Payment info, transactions
	DataCategoryTransactional DataCategory = "TRANSACTIONAL" // Orders, purchases
	DataCategoryTechnical     DataCategory = "TECHNICAL"     // IP, device info, cookies
	DataCategoryUsage         DataCategory = "USAGE"         // Activity logs, preferences
	DataCategoryMarketing     DataCategory = "MARKETING"     // Marketing preferences
	DataCategorySensitive     DataCategory = "SENSITIVE"     // Special category data
)

// AuditAction represents the type of audit action
type AuditAction string

const (
	AuditActionConsentGranted   AuditAction = "CONSENT_GRANTED"
	AuditActionConsentWithdrawn AuditAction = "CONSENT_WITHDRAWN"
	AuditActionConsentUpdated   AuditAction = "CONSENT_UPDATED"
	AuditActionDataExported     AuditAction = "DATA_EXPORTED"
	AuditActionDataDeleted      AuditAction = "DATA_DELETED"
	AuditActionDataAnonymized   AuditAction = "DATA_ANONYMIZED"
	AuditActionDeletionRequest  AuditAction = "DELETION_REQUEST"
	AuditActionRetentionApplied AuditAction = "RETENTION_APPLIED"
	AuditActionAccessRequest    AuditAction = "ACCESS_REQUEST"
)

// DataSubjectRequest represents a GDPR data subject request
type DataSubjectRequest struct {
	ID              string          `json:"id" gorm:"primaryKey;type:varchar(36)"`
	TenantID        string          `json:"tenant_id" gorm:"type:varchar(36);index"`
	UserID          string          `json:"user_id" gorm:"type:varchar(36);index"`
	RequestType     RequestType     `json:"request_type" gorm:"type:varchar(50);index"`
	Status          RequestStatus   `json:"status" gorm:"type:varchar(50);index"`
	RequestedAt     time.Time       `json:"requested_at" gorm:"autoCreateTime"`
	ProcessedAt     *time.Time      `json:"processed_at,omitempty"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`
	ExpiresAt       *time.Time      `json:"expires_at,omitempty"`
	RequestDetails  json.RawMessage `json:"request_details,omitempty" gorm:"type:jsonb"`
	ResponseDetails json.RawMessage `json:"response_details,omitempty" gorm:"type:jsonb"`
	ProcessedBy     string          `json:"processed_by,omitempty" gorm:"type:varchar(36)"`
	Notes           string          `json:"notes,omitempty" gorm:"type:text"`
	IPAddress       string          `json:"ip_address,omitempty" gorm:"type:varchar(45)"`
	UserAgent       string          `json:"user_agent,omitempty" gorm:"type:varchar(500)"`
	CreatedAt       time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the table name for DataSubjectRequest
func (DataSubjectRequest) TableName() string {
	return "gdpr_data_subject_requests"
}

// ConsentRecord represents a user's consent for a specific purpose
type ConsentRecord struct {
	ID                string          `json:"id" gorm:"primaryKey;type:varchar(36)"`
	TenantID          string          `json:"tenant_id" gorm:"type:varchar(36);index:idx_consent_tenant_user"`
	UserID            string          `json:"user_id" gorm:"type:varchar(36);index:idx_consent_tenant_user"`
	Purpose           ConsentPurpose  `json:"purpose" gorm:"type:varchar(50);index:idx_consent_purpose"`
	Granted           bool            `json:"granted" gorm:"index"`
	GrantedAt         *time.Time      `json:"granted_at,omitempty"`
	WithdrawnAt       *time.Time      `json:"withdrawn_at,omitempty"`
	ExpiresAt         *time.Time      `json:"expires_at,omitempty"`
	Version           string          `json:"version" gorm:"type:varchar(50)"`
	Source            string          `json:"source" gorm:"type:varchar(100)"`
	IPAddress         string          `json:"ip_address,omitempty" gorm:"type:varchar(45)"`
	UserAgent         string          `json:"user_agent,omitempty" gorm:"type:varchar(500)"`
	LegalBasis        string          `json:"legal_basis,omitempty" gorm:"type:varchar(100)"`
	DataCategories    json.RawMessage `json:"data_categories,omitempty" gorm:"type:jsonb"`
	ProcessingPurpose string          `json:"processing_purpose,omitempty" gorm:"type:text"`
	CreatedAt         time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the table name for ConsentRecord
func (ConsentRecord) TableName() string {
	return "gdpr_consent_records"
}

// IsExpired checks if the consent has expired
func (c *ConsentRecord) IsExpired() bool {
	if c.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*c.ExpiresAt)
}

// IsActive checks if the consent is currently active (granted and not expired)
func (c *ConsentRecord) IsActive() bool {
	return c.Granted && !c.IsExpired()
}

// RetentionPolicy defines data retention rules for specific data categories
type RetentionPolicy struct {
	ID                string          `json:"id" gorm:"primaryKey;type:varchar(36)"`
	TenantID          string          `json:"tenant_id" gorm:"type:varchar(36);index"`
	Name              string          `json:"name" gorm:"type:varchar(100)"`
	Description       string          `json:"description" gorm:"type:text"`
	DataCategory      DataCategory    `json:"data_category" gorm:"type:varchar(50);index"`
	RetentionDays     int             `json:"retention_days"`
	Action            string          `json:"action" gorm:"type:varchar(50)"` // DELETE, ANONYMIZE, ARCHIVE
	IsActive          bool            `json:"is_active" gorm:"default:true"`
	LegalBasis        string          `json:"legal_basis,omitempty" gorm:"type:varchar(200)"`
	ApplicableTables  json.RawMessage `json:"applicable_tables,omitempty" gorm:"type:jsonb"`
	ExclusionCriteria json.RawMessage `json:"exclusion_criteria,omitempty" gorm:"type:jsonb"`
	CreatedBy         string          `json:"created_by" gorm:"type:varchar(36)"`
	LastModifiedBy    string          `json:"last_modified_by" gorm:"type:varchar(36)"`
	CreatedAt         time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the table name for RetentionPolicy
func (RetentionPolicy) TableName() string {
	return "gdpr_retention_policies"
}

// CalculateExpiryDate calculates when data should expire based on creation date
func (r *RetentionPolicy) CalculateExpiryDate(createdAt time.Time) time.Time {
	return createdAt.AddDate(0, 0, r.RetentionDays)
}

// GDPRAuditLog represents an audit log entry for GDPR-related actions
type GDPRAuditLog struct {
	ID          string          `json:"id" gorm:"primaryKey;type:varchar(36)"`
	TenantID    string          `json:"tenant_id" gorm:"type:varchar(36);index"`
	UserID      string          `json:"user_id" gorm:"type:varchar(36);index"`
	Action      AuditAction     `json:"action" gorm:"type:varchar(50);index"`
	EntityType  string          `json:"entity_type" gorm:"type:varchar(100)"`
	EntityID    string          `json:"entity_id" gorm:"type:varchar(36)"`
	OldValue    json.RawMessage `json:"old_value,omitempty" gorm:"type:jsonb"`
	NewValue    json.RawMessage `json:"new_value,omitempty" gorm:"type:jsonb"`
	Metadata    json.RawMessage `json:"metadata,omitempty" gorm:"type:jsonb"`
	IPAddress   string          `json:"ip_address,omitempty" gorm:"type:varchar(45)"`
	UserAgent   string          `json:"user_agent,omitempty" gorm:"type:varchar(500)"`
	PerformedBy string          `json:"performed_by" gorm:"type:varchar(36)"`
	PerformedAt time.Time       `json:"performed_at" gorm:"autoCreateTime"`
	RequestID   string          `json:"request_id,omitempty" gorm:"type:varchar(36)"`
	ServiceName string          `json:"service_name" gorm:"type:varchar(100)"`
	Checksum    string          `json:"checksum" gorm:"type:varchar(64)"` // SHA256 hash for integrity
}

// TableName returns the table name for GDPRAuditLog
func (GDPRAuditLog) TableName() string {
	return "gdpr_audit_logs"
}

// DataDeletionRequest represents a request to delete user data
type DataDeletionRequest struct {
	ID               string          `json:"id" gorm:"primaryKey;type:varchar(36)"`
	TenantID         string          `json:"tenant_id" gorm:"type:varchar(36);index"`
	UserID           string          `json:"user_id" gorm:"type:varchar(36);index"`
	Status           RequestStatus   `json:"status" gorm:"type:varchar(50);index"`
	RequestedAt      time.Time       `json:"requested_at" gorm:"autoCreateTime"`
	ScheduledFor     *time.Time      `json:"scheduled_for,omitempty"`
	ProcessedAt      *time.Time      `json:"processed_at,omitempty"`
	CompletedAt      *time.Time      `json:"completed_at,omitempty"`
	DataCategories   json.RawMessage `json:"data_categories,omitempty" gorm:"type:jsonb"`
	ExcludedData     json.RawMessage `json:"excluded_data,omitempty" gorm:"type:jsonb"`
	DeletionResults  json.RawMessage `json:"deletion_results,omitempty" gorm:"type:jsonb"`
	VerificationData json.RawMessage `json:"verification_data,omitempty" gorm:"type:jsonb"`
	ProcessedBy      string          `json:"processed_by,omitempty" gorm:"type:varchar(36)"`
	Reason           string          `json:"reason,omitempty" gorm:"type:text"`
	Notes            string          `json:"notes,omitempty" gorm:"type:text"`
	IPAddress        string          `json:"ip_address,omitempty" gorm:"type:varchar(45)"`
	UserAgent        string          `json:"user_agent,omitempty" gorm:"type:varchar(500)"`
	CreatedAt        time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the table name for DataDeletionRequest
func (DataDeletionRequest) TableName() string {
	return "gdpr_data_deletion_requests"
}

// ScheduledDeletion represents a scheduled data deletion job
type ScheduledDeletion struct {
	ID           string       `json:"id" gorm:"primaryKey;type:varchar(36)"`
	TenantID     string       `json:"tenant_id" gorm:"type:varchar(36);index"`
	UserID       string       `json:"user_id" gorm:"type:varchar(36);index"`
	DataCategory DataCategory `json:"data_category" gorm:"type:varchar(50)"`
	TargetTable  string       `json:"target_table" gorm:"type:varchar(100)"`
	RecordID     string       `json:"record_id" gorm:"type:varchar(36)"`
	ScheduledFor time.Time    `json:"scheduled_for" gorm:"index"`
	Executed     bool         `json:"executed" gorm:"default:false"`
	ExecutedAt   *time.Time   `json:"executed_at,omitempty"`
	PolicyID     string       `json:"policy_id,omitempty" gorm:"type:varchar(36)"`
	CreatedAt    time.Time    `json:"created_at" gorm:"autoCreateTime"`
}

// TableName returns the table name for ScheduledDeletion
func (ScheduledDeletion) TableName() string {
	return "gdpr_scheduled_deletions"
}

// GDPRService defines the interface for GDPR compliance operations
type GDPRService interface {
	// Consent management
	RecordConsent(ctx context.Context, req ConsentRequest) (*ConsentRecord, error)
	WithdrawConsent(ctx context.Context, userID string, purpose ConsentPurpose) error
	GetConsentStatus(ctx context.Context, userID string, purpose ConsentPurpose) (*ConsentRecord, error)
	ListConsents(ctx context.Context, userID string) ([]ConsentRecord, error)
	GetConsentHistory(ctx context.Context, userID string, purpose ConsentPurpose) ([]GDPRAuditLog, error)

	// Data export (Article 20)
	ExportUserData(ctx context.Context, userID string) (*DataExport, error)
	GenerateDataPackage(ctx context.Context, userID string) ([]byte, error)

	// Data retention
	GetRetentionPolicies(ctx context.Context) ([]RetentionPolicy, error)
	CreateRetentionPolicy(ctx context.Context, policy RetentionPolicy) (*RetentionPolicy, error)
	UpdateRetentionPolicy(ctx context.Context, policy RetentionPolicy) (*RetentionPolicy, error)
	ScheduleDataDeletion(ctx context.Context, userID string, category DataCategory, days int) error
	CleanupExpiredData(ctx context.Context) (*CleanupResult, error)
	AnonymizeData(ctx context.Context, userID string) (*AnonymizationResult, error)

	// Right to deletion (Article 17)
	RequestDeletion(ctx context.Context, req DeletionRequest) (*DataDeletionRequest, error)
	ProcessDeletionRequest(ctx context.Context, requestID string) error
	VerifyDeletion(ctx context.Context, requestID string) (*DeletionVerification, error)
	GetDeletionRequest(ctx context.Context, requestID string) (*DataDeletionRequest, error)
	ListDeletionRequests(ctx context.Context, userID string) ([]DataDeletionRequest, error)

	// Audit
	GetAuditLog(ctx context.Context, userID string, filter AuditFilter) ([]GDPRAuditLog, error)
	CreateAuditEntry(ctx context.Context, entry GDPRAuditLog) error
}

// ConsentRequest represents a request to record consent
type ConsentRequest struct {
	TenantID          string         `json:"tenant_id"`
	UserID            string         `json:"user_id"`
	Purpose           ConsentPurpose `json:"purpose"`
	Granted           bool           `json:"granted"`
	Version           string         `json:"version"`
	Source            string         `json:"source"`
	IPAddress         string         `json:"ip_address"`
	UserAgent         string         `json:"user_agent"`
	LegalBasis        string         `json:"legal_basis"`
	ExpiresInDays     *int           `json:"expires_in_days,omitempty"`
	DataCategories    []DataCategory `json:"data_categories,omitempty"`
	ProcessingPurpose string         `json:"processing_purpose,omitempty"`
}

// DeletionRequest represents a request to delete user data
type DeletionRequest struct {
	TenantID       string         `json:"tenant_id"`
	UserID         string         `json:"user_id"`
	DataCategories []DataCategory `json:"data_categories,omitempty"`
	Reason         string         `json:"reason,omitempty"`
	ScheduleFor    *time.Time     `json:"schedule_for,omitempty"`
	IPAddress      string         `json:"ip_address"`
	UserAgent      string         `json:"user_agent"`
}

// DataExport represents exported user data
type DataExport struct {
	UserID      string                     `json:"user_id"`
	ExportedAt  time.Time                  `json:"exported_at"`
	DataSources map[string]json.RawMessage `json:"data_sources"`
	Metadata    ExportMetadata             `json:"metadata"`
}

// ExportMetadata contains metadata about the export
type ExportMetadata struct {
	Format       string    `json:"format"`
	Version      string    `json:"version"`
	GeneratedAt  time.Time `json:"generated_at"`
	RequestID    string    `json:"request_id,omitempty"`
	TotalRecords int       `json:"total_records"`
	DataTypes    []string  `json:"data_types"`
}

// CleanupResult represents the result of a data cleanup operation
type CleanupResult struct {
	ProcessedAt     time.Time            `json:"processed_at"`
	TotalProcessed  int                  `json:"total_processed"`
	TotalDeleted    int                  `json:"total_deleted"`
	TotalAnonymized int                  `json:"total_anonymized"`
	TotalFailed     int                  `json:"total_failed"`
	ByCategory      map[DataCategory]int `json:"by_category"`
	Errors          []string             `json:"errors,omitempty"`
}

// AnonymizationResult represents the result of data anonymization
type AnonymizationResult struct {
	UserID           string         `json:"user_id"`
	ProcessedAt      time.Time      `json:"processed_at"`
	TotalRecords     int            `json:"total_records"`
	ByTable          map[string]int `json:"by_table"`
	AnonymizedFields []string       `json:"anonymized_fields"`
}

// DeletionVerification represents the verification of a deletion request
type DeletionVerification struct {
	RequestID       string               `json:"request_id"`
	UserID          string               `json:"user_id"`
	Verified        bool                 `json:"verified"`
	VerifiedAt      time.Time            `json:"verified_at"`
	RemainingData   map[string]int       `json:"remaining_data,omitempty"`
	DeletedSources  []string             `json:"deleted_sources"`
	RetainedSources []RetainedDataSource `json:"retained_sources,omitempty"`
}

// RetainedDataSource represents data that was retained for legal reasons
type RetainedDataSource struct {
	Source      string `json:"source"`
	Reason      string `json:"reason"`
	LegalBasis  string `json:"legal_basis"`
	RetainUntil string `json:"retain_until,omitempty"`
}

// AuditFilter represents filters for audit log queries
type AuditFilter struct {
	Actions     []AuditAction `json:"actions,omitempty"`
	StartDate   *time.Time    `json:"start_date,omitempty"`
	EndDate     *time.Time    `json:"end_date,omitempty"`
	EntityType  string        `json:"entity_type,omitempty"`
	PerformedBy string        `json:"performed_by,omitempty"`
	Limit       int           `json:"limit,omitempty"`
	Offset      int           `json:"offset,omitempty"`
}

// DataSourceProvider defines an interface for services to provide user data
type DataSourceProvider interface {
	GetServiceName() string
	GetUserData(ctx context.Context, userID string) (json.RawMessage, error)
	DeleteUserData(ctx context.Context, userID string) error
	AnonymizeUserData(ctx context.Context, userID string) error
	GetDataCategories() []DataCategory
}

// Config holds configuration for the GDPR service
type Config struct {
	DB                    *gorm.DB
	ServiceName           string
	DefaultRetentionDays  int
	DataSourceProviders   []DataSourceProvider
	EnableAuditLog        bool
	AuditLogRetentionDays int
}

// DefaultConfig returns a default GDPR configuration
func DefaultConfig(db *gorm.DB, serviceName string) Config {
	return Config{
		DB:                    db,
		ServiceName:           serviceName,
		DefaultRetentionDays:  365 * 3, // 3 years default retention
		EnableAuditLog:        true,
		AuditLogRetentionDays: 365 * 7, // 7 years for audit logs
	}
}

// MigrateModels runs database migrations for GDPR models
func MigrateModels(db *gorm.DB) error {
	return db.AutoMigrate(
		&DataSubjectRequest{},
		&ConsentRecord{},
		&RetentionPolicy{},
		&GDPRAuditLog{},
		&DataDeletionRequest{},
		&ScheduledDeletion{},
	)
}
