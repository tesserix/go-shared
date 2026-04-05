package gdpr

import (
	"context"
	"encoding/json"

	"gorm.io/gorm"
)

// Service implements the GDPRService interface and provides a unified
// access point for all GDPR compliance operations
type Service struct {
	db               *gorm.DB
	config           Config
	consentManager   *ConsentManager
	dataExporter     *DataExporter
	retentionManager *RetentionManager
	deletionManager  *DeletionManager
	auditLogger      *AuditLogger
}

// NewService creates a new GDPR service instance
func NewService(config Config) *Service {
	service := &Service{
		db:               config.DB,
		config:           config,
		consentManager:   NewConsentManager(config.DB, config.ServiceName),
		dataExporter:     NewDataExporter(config.DB, config.ServiceName, config.DataSourceProviders),
		retentionManager: NewRetentionManager(config.DB, config.ServiceName, config.DataSourceProviders),
		deletionManager:  NewDeletionManager(config.DB, config.ServiceName, config.DataSourceProviders),
		auditLogger:      NewAuditLogger(config.DB, config.ServiceName),
	}

	return service
}

// RegisterDataProvider registers a data source provider for GDPR operations
func (s *Service) RegisterDataProvider(provider DataSourceProvider) {
	s.dataExporter.AddDataProvider(provider)
	s.retentionManager.AddDataProvider(provider)
	s.deletionManager.AddDataProvider(provider)
}

// === Consent Management (implements GDPRService) ===

// RecordConsent records a user's consent
func (s *Service) RecordConsent(ctx context.Context, req ConsentRequest) (*ConsentRecord, error) {
	return s.consentManager.RecordConsent(ctx, req)
}

// WithdrawConsent withdraws a user's consent
func (s *Service) WithdrawConsent(ctx context.Context, userID string, purpose ConsentPurpose) error {
	// Extract tenant ID from context if available
	tenantID := extractTenantID(ctx)
	return s.consentManager.WithdrawConsent(ctx, tenantID, userID, purpose)
}

// GetConsentStatus retrieves consent status
func (s *Service) GetConsentStatus(ctx context.Context, userID string, purpose ConsentPurpose) (*ConsentRecord, error) {
	tenantID := extractTenantID(ctx)
	return s.consentManager.GetConsentStatus(ctx, tenantID, userID, purpose)
}

// ListConsents lists all consents for a user
func (s *Service) ListConsents(ctx context.Context, userID string) ([]ConsentRecord, error) {
	tenantID := extractTenantID(ctx)
	return s.consentManager.ListConsents(ctx, tenantID, userID)
}

// GetConsentHistory retrieves consent history
func (s *Service) GetConsentHistory(ctx context.Context, userID string, purpose ConsentPurpose) ([]GDPRAuditLog, error) {
	tenantID := extractTenantID(ctx)
	return s.consentManager.GetConsentHistory(ctx, tenantID, userID, purpose)
}

// HasActiveConsent checks if user has active consent for a purpose
func (s *Service) HasActiveConsent(ctx context.Context, userID string, purpose ConsentPurpose) (bool, error) {
	tenantID := extractTenantID(ctx)
	return s.consentManager.HasActiveConsent(ctx, tenantID, userID, purpose)
}

// GetActiveConsents returns all active consents for a user
func (s *Service) GetActiveConsents(ctx context.Context, userID string) ([]ConsentRecord, error) {
	tenantID := extractTenantID(ctx)
	return s.consentManager.GetActiveConsents(ctx, tenantID, userID)
}

// GetConsentSummary returns a summary of user's consents
func (s *Service) GetConsentSummary(ctx context.Context, userID string) (*ConsentSummary, error) {
	tenantID := extractTenantID(ctx)
	return s.consentManager.GetConsentSummary(ctx, tenantID, userID)
}

// RevokeAllConsents revokes all consents for a user
func (s *Service) RevokeAllConsents(ctx context.Context, userID string) error {
	tenantID := extractTenantID(ctx)
	return s.consentManager.RevokeAllConsents(ctx, tenantID, userID)
}

// ProcessExpiredConsents processes expired consents
func (s *Service) ProcessExpiredConsents(ctx context.Context) (int64, error) {
	tenantID := extractTenantID(ctx)
	return s.consentManager.ProcessExpiredConsents(ctx, tenantID)
}

// === Data Export (Article 20) ===

// ExportUserData exports all user data
func (s *Service) ExportUserData(ctx context.Context, userID string) (*DataExport, error) {
	tenantID := extractTenantID(ctx)
	return s.dataExporter.ExportUserData(ctx, tenantID, userID)
}

// GenerateDataPackage generates a downloadable data package
func (s *Service) GenerateDataPackage(ctx context.Context, userID string) ([]byte, error) {
	tenantID := extractTenantID(ctx)
	return s.dataExporter.GenerateDataPackage(ctx, tenantID, userID)
}

// ExportConsentRecords exports consent records
func (s *Service) ExportConsentRecords(ctx context.Context, userID string) ([]ConsentRecord, error) {
	tenantID := extractTenantID(ctx)
	return s.dataExporter.ExportConsentRecords(ctx, tenantID, userID)
}

// ExportAuditLog exports audit log
func (s *Service) ExportAuditLog(ctx context.Context, userID string) ([]GDPRAuditLog, error) {
	tenantID := extractTenantID(ctx)
	return s.dataExporter.ExportAuditLog(ctx, tenantID, userID)
}

// === Data Retention ===

// GetRetentionPolicies retrieves all retention policies
func (s *Service) GetRetentionPolicies(ctx context.Context) ([]RetentionPolicy, error) {
	tenantID := extractTenantID(ctx)
	return s.retentionManager.GetRetentionPolicies(ctx, tenantID)
}

// GetActiveRetentionPolicies retrieves active retention policies
func (s *Service) GetActiveRetentionPolicies(ctx context.Context) ([]RetentionPolicy, error) {
	tenantID := extractTenantID(ctx)
	return s.retentionManager.GetActiveRetentionPolicies(ctx, tenantID)
}

// CreateRetentionPolicy creates a new retention policy
func (s *Service) CreateRetentionPolicy(ctx context.Context, policy RetentionPolicy) (*RetentionPolicy, error) {
	return s.retentionManager.CreateRetentionPolicy(ctx, policy)
}

// UpdateRetentionPolicy updates a retention policy
func (s *Service) UpdateRetentionPolicy(ctx context.Context, policy RetentionPolicy) (*RetentionPolicy, error) {
	return s.retentionManager.UpdateRetentionPolicy(ctx, policy)
}

// GetRetentionPolicy retrieves a retention policy by ID
func (s *Service) GetRetentionPolicy(ctx context.Context, policyID string) (*RetentionPolicy, error) {
	return s.retentionManager.GetRetentionPolicy(ctx, policyID)
}

// GetRetentionPolicyByCategory retrieves retention policy by category
func (s *Service) GetRetentionPolicyByCategory(ctx context.Context, category DataCategory) (*RetentionPolicy, error) {
	tenantID := extractTenantID(ctx)
	return s.retentionManager.GetRetentionPolicyByCategory(ctx, tenantID, category)
}

// ScheduleDataDeletion schedules data deletion
func (s *Service) ScheduleDataDeletion(ctx context.Context, userID string, category DataCategory, days int) error {
	tenantID := extractTenantID(ctx)
	return s.retentionManager.ScheduleDataDeletion(ctx, tenantID, userID, category, days)
}

// CleanupExpiredData cleans up expired data
func (s *Service) CleanupExpiredData(ctx context.Context) (*CleanupResult, error) {
	tenantID := extractTenantID(ctx)
	return s.retentionManager.CleanupExpiredData(ctx, tenantID)
}

// AnonymizeData anonymizes user data
func (s *Service) AnonymizeData(ctx context.Context, userID string) (*AnonymizationResult, error) {
	tenantID := extractTenantID(ctx)
	return s.retentionManager.AnonymizeData(ctx, tenantID, userID)
}

// GetScheduledDeletions retrieves scheduled deletions
func (s *Service) GetScheduledDeletions(ctx context.Context, userID string) ([]ScheduledDeletion, error) {
	tenantID := extractTenantID(ctx)
	return s.retentionManager.GetScheduledDeletions(ctx, tenantID, userID)
}

// === Right to Deletion (Article 17) ===

// RequestDeletion initiates a deletion request
func (s *Service) RequestDeletion(ctx context.Context, req DeletionRequest) (*DataDeletionRequest, error) {
	return s.deletionManager.RequestDeletion(ctx, req)
}

// ProcessDeletionRequest processes a deletion request
func (s *Service) ProcessDeletionRequest(ctx context.Context, requestID string) error {
	return s.deletionManager.ProcessDeletionRequest(ctx, requestID)
}

// VerifyDeletion verifies deletion completion
func (s *Service) VerifyDeletion(ctx context.Context, requestID string) (*DeletionVerification, error) {
	return s.deletionManager.VerifyDeletion(ctx, requestID)
}

// GetDeletionRequest retrieves a deletion request
func (s *Service) GetDeletionRequest(ctx context.Context, requestID string) (*DataDeletionRequest, error) {
	return s.deletionManager.GetDeletionRequest(ctx, requestID)
}

// ListDeletionRequests lists deletion requests
func (s *Service) ListDeletionRequests(ctx context.Context, userID string) ([]DataDeletionRequest, error) {
	tenantID := extractTenantID(ctx)
	return s.deletionManager.ListDeletionRequests(ctx, tenantID, userID)
}

// CancelDeletionRequest cancels a deletion request
func (s *Service) CancelDeletionRequest(ctx context.Context, requestID string, cancelledBy string) error {
	return s.deletionManager.CancelDeletionRequest(ctx, requestID, cancelledBy)
}

// GetPendingDeletionRequests retrieves pending deletion requests
func (s *Service) GetPendingDeletionRequests(ctx context.Context) ([]DataDeletionRequest, error) {
	tenantID := extractTenantID(ctx)
	return s.deletionManager.GetPendingDeletionRequests(ctx, tenantID)
}

// ProcessDueDeletionRequests processes due deletion requests
func (s *Service) ProcessDueDeletionRequests(ctx context.Context) (*DeletionProcessingResult, error) {
	tenantID := extractTenantID(ctx)
	return s.deletionManager.ProcessDueDeletionRequests(ctx, tenantID)
}

// GetDeletionRequestStats retrieves deletion request statistics
func (s *Service) GetDeletionRequestStats(ctx context.Context) (*DeletionRequestStats, error) {
	tenantID := extractTenantID(ctx)
	return s.deletionManager.GetDeletionRequestStats(ctx, tenantID)
}

// === Audit ===

// GetAuditLog retrieves audit logs
func (s *Service) GetAuditLog(ctx context.Context, userID string, filter AuditFilter) ([]GDPRAuditLog, error) {
	tenantID := extractTenantID(ctx)
	return s.auditLogger.GetAuditLogs(ctx, tenantID, userID, filter)
}

// CreateAuditEntry creates an audit entry
func (s *Service) CreateAuditEntry(ctx context.Context, entry GDPRAuditLog) error {
	return s.auditLogger.Log(ctx, entry)
}

// VerifyAuditLogIntegrity verifies audit log integrity
func (s *Service) VerifyAuditLogIntegrity(entry GDPRAuditLog) bool {
	return s.auditLogger.VerifyAuditLogIntegrity(entry)
}

// === Database Migrations ===

// Migrate runs database migrations for GDPR models
func (s *Service) Migrate() error {
	return MigrateModels(s.db)
}

// === Default Policies ===

// InitializeDefaultPolicies creates default retention policies
func (s *Service) InitializeDefaultPolicies(ctx context.Context, createdBy string) error {
	tenantID := extractTenantID(ctx)
	policies := DefaultRetentionPolicies(tenantID, createdBy)

	for _, policy := range policies {
		// Check if policy already exists
		existing, _ := s.retentionManager.GetRetentionPolicyByCategory(ctx, tenantID, policy.DataCategory)
		if existing != nil {
			continue
		}

		_, err := s.retentionManager.CreateRetentionPolicy(ctx, policy)
		if err != nil {
			return err
		}
	}

	return nil
}

// === Health Check ===

// HealthCheck performs a health check on the GDPR service
func (s *Service) HealthCheck(ctx context.Context) error {
	// Test database connection
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// === Utility Functions ===

// extractTenantID extracts tenant ID from context
func extractTenantID(ctx context.Context) string {
	if tenantID, ok := ctx.Value("tenant_id").(string); ok {
		return tenantID
	}
	return ""
}

// ContextWithTenant creates a context with tenant ID
func ContextWithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, "tenant_id", tenantID)
}

// === Batch Operations ===

// BatchRecordConsent records multiple consents
func (s *Service) BatchRecordConsent(ctx context.Context, requests []ConsentRequest) ([]ConsentRecord, error) {
	return s.consentManager.BulkRecordConsent(ctx, requests)
}

// === Statistics and Reporting ===

// GDPRStats represents overall GDPR statistics
type GDPRStats struct {
	ConsentStats   ConsentStats          `json:"consent_stats"`
	DeletionStats  *DeletionRequestStats `json:"deletion_stats"`
	RetentionStats RetentionStats        `json:"retention_stats"`
}

// ConsentStats represents consent statistics
type ConsentStats struct {
	TotalConsents   int64                    `json:"total_consents"`
	ActiveConsents  int64                    `json:"active_consents"`
	ExpiredConsents int64                    `json:"expired_consents"`
	ByPurpose       map[ConsentPurpose]int64 `json:"by_purpose"`
}

// RetentionStats represents retention statistics
type RetentionStats struct {
	ActivePolicies     int64 `json:"active_policies"`
	ScheduledDeletions int64 `json:"scheduled_deletions"`
	CompletedDeletions int64 `json:"completed_deletions"`
}

// GetStats retrieves overall GDPR statistics
func (s *Service) GetStats(ctx context.Context) (*GDPRStats, error) {
	tenantID := extractTenantID(ctx)
	stats := &GDPRStats{
		ConsentStats: ConsentStats{
			ByPurpose: make(map[ConsentPurpose]int64),
		},
		RetentionStats: RetentionStats{},
	}

	// Get consent stats
	var totalConsents, activeConsents, expiredConsents int64

	s.db.WithContext(ctx).
		Model(&ConsentRecord{}).
		Where("tenant_id = ?", tenantID).
		Count(&totalConsents)
	stats.ConsentStats.TotalConsents = totalConsents

	s.db.WithContext(ctx).
		Model(&ConsentRecord{}).
		Where("tenant_id = ? AND granted = ? AND (expires_at IS NULL OR expires_at > NOW())", tenantID, true).
		Count(&activeConsents)
	stats.ConsentStats.ActiveConsents = activeConsents

	s.db.WithContext(ctx).
		Model(&ConsentRecord{}).
		Where("tenant_id = ? AND granted = ? AND expires_at IS NOT NULL AND expires_at < NOW()", tenantID, true).
		Count(&expiredConsents)
	stats.ConsentStats.ExpiredConsents = expiredConsents

	// Get consent by purpose
	type PurposeCount struct {
		Purpose ConsentPurpose
		Count   int64
	}
	var purposeCounts []PurposeCount
	s.db.WithContext(ctx).
		Model(&ConsentRecord{}).
		Where("tenant_id = ? AND granted = ?", tenantID, true).
		Select("purpose, count(*) as count").
		Group("purpose").
		Scan(&purposeCounts)
	for _, pc := range purposeCounts {
		stats.ConsentStats.ByPurpose[pc.Purpose] = pc.Count
	}

	// Get deletion stats
	deletionStats, err := s.deletionManager.GetDeletionRequestStats(ctx, tenantID)
	if err == nil {
		stats.DeletionStats = deletionStats
	}

	// Get retention stats
	var activePolicies, scheduledDeletions, completedDeletions int64

	s.db.WithContext(ctx).
		Model(&RetentionPolicy{}).
		Where("tenant_id = ? AND is_active = ?", tenantID, true).
		Count(&activePolicies)
	stats.RetentionStats.ActivePolicies = activePolicies

	s.db.WithContext(ctx).
		Model(&ScheduledDeletion{}).
		Where("tenant_id = ? AND executed = ?", tenantID, false).
		Count(&scheduledDeletions)
	stats.RetentionStats.ScheduledDeletions = scheduledDeletions

	s.db.WithContext(ctx).
		Model(&ScheduledDeletion{}).
		Where("tenant_id = ? AND executed = ?", tenantID, true).
		Count(&completedDeletions)
	stats.RetentionStats.CompletedDeletions = completedDeletions

	return stats, nil
}

// === Compliance Report Generation ===

// ComplianceReport represents a GDPR compliance report
type ComplianceReport struct {
	GeneratedAt     string               `json:"generated_at"`
	TenantID        string               `json:"tenant_id"`
	Period          string               `json:"period"`
	Stats           *GDPRStats           `json:"stats"`
	ConsentRecords  []ConsentReportItem  `json:"consent_records"`
	DeletionRecords []DeletionReportItem `json:"deletion_records"`
	AuditSummary    AuditSummary         `json:"audit_summary"`
}

// ConsentReportItem represents a consent item in the report
type ConsentReportItem struct {
	Purpose   ConsentPurpose `json:"purpose"`
	Total     int64          `json:"total"`
	Active    int64          `json:"active"`
	Withdrawn int64          `json:"withdrawn"`
	Expired   int64          `json:"expired"`
}

// DeletionReportItem represents a deletion item in the report
type DeletionReportItem struct {
	Status RequestStatus `json:"status"`
	Count  int64         `json:"count"`
}

// AuditSummary represents an audit summary
type AuditSummary struct {
	TotalEntries int64                 `json:"total_entries"`
	ByAction     map[AuditAction]int64 `json:"by_action"`
}

// GenerateComplianceReport generates a GDPR compliance report
func (s *Service) GenerateComplianceReport(ctx context.Context, period string) (*ComplianceReport, error) {
	tenantID := extractTenantID(ctx)

	stats, err := s.GetStats(ctx)
	if err != nil {
		return nil, err
	}

	report := &ComplianceReport{
		GeneratedAt:     "now",
		TenantID:        tenantID,
		Period:          period,
		Stats:           stats,
		ConsentRecords:  []ConsentReportItem{},
		DeletionRecords: []DeletionReportItem{},
		AuditSummary: AuditSummary{
			ByAction: make(map[AuditAction]int64),
		},
	}

	// Build consent report items
	purposes := []ConsentPurpose{
		ConsentPurposeMarketing,
		ConsentPurposeAnalytics,
		ConsentPurposePersonalization,
		ConsentPurposeThirdParty,
		ConsentPurposeEssential,
		ConsentPurposePerformance,
		ConsentPurposeFunctional,
	}

	for _, purpose := range purposes {
		var total, active, withdrawn, expired int64

		s.db.WithContext(ctx).
			Model(&ConsentRecord{}).
			Where("tenant_id = ? AND purpose = ?", tenantID, purpose).
			Count(&total)

		s.db.WithContext(ctx).
			Model(&ConsentRecord{}).
			Where("tenant_id = ? AND purpose = ? AND granted = ? AND (expires_at IS NULL OR expires_at > NOW())", tenantID, purpose, true).
			Count(&active)

		s.db.WithContext(ctx).
			Model(&ConsentRecord{}).
			Where("tenant_id = ? AND purpose = ? AND granted = ? AND withdrawn_at IS NOT NULL", tenantID, purpose, false).
			Count(&withdrawn)

		s.db.WithContext(ctx).
			Model(&ConsentRecord{}).
			Where("tenant_id = ? AND purpose = ? AND granted = ? AND expires_at < NOW()", tenantID, purpose, true).
			Count(&expired)

		if total > 0 {
			report.ConsentRecords = append(report.ConsentRecords, ConsentReportItem{
				Purpose:   purpose,
				Total:     total,
				Active:    active,
				Withdrawn: withdrawn,
				Expired:   expired,
			})
		}
	}

	// Build deletion report items
	statuses := []RequestStatus{
		RequestStatusPending,
		RequestStatusProcessing,
		RequestStatusCompleted,
		RequestStatusRejected,
		RequestStatusFailed,
		RequestStatusCancelled,
	}

	for _, status := range statuses {
		var count int64
		s.db.WithContext(ctx).
			Model(&DataDeletionRequest{}).
			Where("tenant_id = ? AND status = ?", tenantID, status).
			Count(&count)

		if count > 0 {
			report.DeletionRecords = append(report.DeletionRecords, DeletionReportItem{
				Status: status,
				Count:  count,
			})
		}
	}

	// Build audit summary
	var totalAuditEntries int64
	s.db.WithContext(ctx).
		Model(&GDPRAuditLog{}).
		Where("tenant_id = ?", tenantID).
		Count(&totalAuditEntries)
	report.AuditSummary.TotalEntries = totalAuditEntries

	type ActionCount struct {
		Action AuditAction
		Count  int64
	}
	var actionCounts []ActionCount
	s.db.WithContext(ctx).
		Model(&GDPRAuditLog{}).
		Where("tenant_id = ?", tenantID).
		Select("action, count(*) as count").
		Group("action").
		Scan(&actionCounts)
	for _, ac := range actionCounts {
		report.AuditSummary.ByAction[ac.Action] = ac.Count
	}

	// Create audit log for report generation
	reportJSON, _ := json.Marshal(report)
	s.auditLogger.Log(ctx, GDPRAuditLog{
		TenantID:    tenantID,
		UserID:      "system",
		Action:      AuditActionAccessRequest,
		EntityType:  "compliance_report",
		EntityID:    period,
		NewValue:    reportJSON,
		PerformedBy: "system",
	})

	return report, nil
}
