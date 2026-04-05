package gdpr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ConsentManager handles all consent-related operations
type ConsentManager struct {
	db          *gorm.DB
	serviceName string
	auditLogger *AuditLogger
}

// NewConsentManager creates a new ConsentManager instance
func NewConsentManager(db *gorm.DB, serviceName string) *ConsentManager {
	return &ConsentManager{
		db:          db,
		serviceName: serviceName,
		auditLogger: NewAuditLogger(db, serviceName),
	}
}

// RecordConsent records a user's consent for a specific purpose
func (cm *ConsentManager) RecordConsent(ctx context.Context, req ConsentRequest) (*ConsentRecord, error) {
	if req.UserID == "" {
		return nil, ErrInvalidUserID
	}
	if req.Purpose == "" {
		return nil, ErrInvalidPurpose
	}

	now := time.Now().UTC()

	// Check for existing consent record
	var existingConsent ConsentRecord
	err := cm.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND purpose = ?", req.TenantID, req.UserID, req.Purpose).
		First(&existingConsent).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to check existing consent: %w", err)
	}

	// Prepare data categories JSON
	var dataCategoriesJSON json.RawMessage
	if len(req.DataCategories) > 0 {
		categoriesBytes, err := json.Marshal(req.DataCategories)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal data categories: %w", err)
		}
		dataCategoriesJSON = categoriesBytes
	}

	// Calculate expiry if specified
	var expiresAt *time.Time
	if req.ExpiresInDays != nil && *req.ExpiresInDays > 0 {
		expiry := now.AddDate(0, 0, *req.ExpiresInDays)
		expiresAt = &expiry
	}

	var consent ConsentRecord
	var oldValue json.RawMessage
	var auditAction AuditAction

	if existingConsent.ID != "" {
		// Update existing consent
		oldValueBytes, _ := json.Marshal(existingConsent)
		oldValue = oldValueBytes

		consent = existingConsent
		consent.Granted = req.Granted
		consent.Version = req.Version
		consent.Source = req.Source
		consent.IPAddress = req.IPAddress
		consent.UserAgent = req.UserAgent
		consent.LegalBasis = req.LegalBasis
		consent.DataCategories = dataCategoriesJSON
		consent.ProcessingPurpose = req.ProcessingPurpose
		consent.ExpiresAt = expiresAt

		if req.Granted {
			consent.GrantedAt = &now
			consent.WithdrawnAt = nil
			auditAction = AuditActionConsentGranted
		} else {
			consent.WithdrawnAt = &now
			auditAction = AuditActionConsentWithdrawn
		}

		if err := cm.db.WithContext(ctx).Save(&consent).Error; err != nil {
			return nil, fmt.Errorf("failed to update consent: %w", err)
		}
	} else {
		// Create new consent record
		consent = ConsentRecord{
			ID:                uuid.New().String(),
			TenantID:          req.TenantID,
			UserID:            req.UserID,
			Purpose:           req.Purpose,
			Granted:           req.Granted,
			Version:           req.Version,
			Source:            req.Source,
			IPAddress:         req.IPAddress,
			UserAgent:         req.UserAgent,
			LegalBasis:        req.LegalBasis,
			DataCategories:    dataCategoriesJSON,
			ProcessingPurpose: req.ProcessingPurpose,
			ExpiresAt:         expiresAt,
		}

		if req.Granted {
			consent.GrantedAt = &now
			auditAction = AuditActionConsentGranted
		} else {
			consent.WithdrawnAt = &now
			auditAction = AuditActionConsentWithdrawn
		}

		if err := cm.db.WithContext(ctx).Create(&consent).Error; err != nil {
			return nil, fmt.Errorf("failed to create consent: %w", err)
		}
	}

	// Create audit log entry
	newValueBytes, _ := json.Marshal(consent)
	err = cm.auditLogger.Log(ctx, GDPRAuditLog{
		TenantID:    req.TenantID,
		UserID:      req.UserID,
		Action:      auditAction,
		EntityType:  "consent",
		EntityID:    consent.ID,
		OldValue:    oldValue,
		NewValue:    newValueBytes,
		IPAddress:   req.IPAddress,
		UserAgent:   req.UserAgent,
		PerformedBy: req.UserID,
	})
	if err != nil {
		// Log error but don't fail the operation
		fmt.Printf("failed to create audit log: %v\n", err)
	}

	return &consent, nil
}

// WithdrawConsent withdraws consent for a specific purpose
func (cm *ConsentManager) WithdrawConsent(ctx context.Context, tenantID, userID string, purpose ConsentPurpose) error {
	if userID == "" {
		return ErrInvalidUserID
	}
	if purpose == "" {
		return ErrInvalidPurpose
	}

	var consent ConsentRecord
	err := cm.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND purpose = ?", tenantID, userID, purpose).
		First(&consent).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrConsentNotFound
		}
		return fmt.Errorf("failed to find consent: %w", err)
	}

	if !consent.Granted {
		return ErrConsentAlreadyWithdrawn
	}

	oldValueBytes, _ := json.Marshal(consent)

	now := time.Now().UTC()
	consent.Granted = false
	consent.WithdrawnAt = &now

	if err := cm.db.WithContext(ctx).Save(&consent).Error; err != nil {
		return fmt.Errorf("failed to withdraw consent: %w", err)
	}

	// Create audit log entry
	newValueBytes, _ := json.Marshal(consent)
	err = cm.auditLogger.Log(ctx, GDPRAuditLog{
		TenantID:    tenantID,
		UserID:      userID,
		Action:      AuditActionConsentWithdrawn,
		EntityType:  "consent",
		EntityID:    consent.ID,
		OldValue:    oldValueBytes,
		NewValue:    newValueBytes,
		PerformedBy: userID,
	})
	if err != nil {
		fmt.Printf("failed to create audit log: %v\n", err)
	}

	return nil
}

// GetConsentStatus retrieves the current consent status for a user and purpose
func (cm *ConsentManager) GetConsentStatus(ctx context.Context, tenantID, userID string, purpose ConsentPurpose) (*ConsentRecord, error) {
	if userID == "" {
		return nil, ErrInvalidUserID
	}
	if purpose == "" {
		return nil, ErrInvalidPurpose
	}

	var consent ConsentRecord
	err := cm.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND purpose = ?", tenantID, userID, purpose).
		First(&consent).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrConsentNotFound
		}
		return nil, fmt.Errorf("failed to get consent status: %w", err)
	}

	return &consent, nil
}

// ListConsents returns all consents for a user
func (cm *ConsentManager) ListConsents(ctx context.Context, tenantID, userID string) ([]ConsentRecord, error) {
	if userID == "" {
		return nil, ErrInvalidUserID
	}

	var consents []ConsentRecord
	err := cm.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Order("created_at DESC").
		Find(&consents).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list consents: %w", err)
	}

	return consents, nil
}

// GetActiveConsents returns all active (granted and not expired) consents for a user
func (cm *ConsentManager) GetActiveConsents(ctx context.Context, tenantID, userID string) ([]ConsentRecord, error) {
	if userID == "" {
		return nil, ErrInvalidUserID
	}

	var consents []ConsentRecord
	now := time.Now().UTC()

	err := cm.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND granted = ?", tenantID, userID, true).
		Where("expires_at IS NULL OR expires_at > ?", now).
		Order("created_at DESC").
		Find(&consents).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list active consents: %w", err)
	}

	return consents, nil
}

// HasActiveConsent checks if a user has active consent for a specific purpose
func (cm *ConsentManager) HasActiveConsent(ctx context.Context, tenantID, userID string, purpose ConsentPurpose) (bool, error) {
	consent, err := cm.GetConsentStatus(ctx, tenantID, userID, purpose)
	if err != nil {
		if err == ErrConsentNotFound {
			return false, nil
		}
		return false, err
	}

	return consent.IsActive(), nil
}

// GetConsentHistory retrieves the audit history for a user's consent
func (cm *ConsentManager) GetConsentHistory(ctx context.Context, tenantID, userID string, purpose ConsentPurpose) ([]GDPRAuditLog, error) {
	if userID == "" {
		return nil, ErrInvalidUserID
	}

	// First, find the consent record to get its ID
	var consent ConsentRecord
	err := cm.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND purpose = ?", tenantID, userID, purpose).
		First(&consent).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to find consent: %w", err)
	}

	// Get all audit logs for consent actions by this user
	var logs []GDPRAuditLog
	query := cm.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND entity_type = ?", tenantID, userID, "consent")

	if consent.ID != "" {
		query = query.Where("entity_id = ?", consent.ID)
	}

	err = query.
		Where("action IN ?", []AuditAction{
			AuditActionConsentGranted,
			AuditActionConsentWithdrawn,
			AuditActionConsentUpdated,
		}).
		Order("performed_at DESC").
		Find(&logs).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get consent history: %w", err)
	}

	return logs, nil
}

// RevokeAllConsents revokes all consents for a user
func (cm *ConsentManager) RevokeAllConsents(ctx context.Context, tenantID, userID string) error {
	if userID == "" {
		return ErrInvalidUserID
	}

	now := time.Now().UTC()

	result := cm.db.WithContext(ctx).
		Model(&ConsentRecord{}).
		Where("tenant_id = ? AND user_id = ? AND granted = ?", tenantID, userID, true).
		Updates(map[string]interface{}{
			"granted":      false,
			"withdrawn_at": now,
			"updated_at":   now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to revoke all consents: %w", result.Error)
	}

	// Create audit log entry
	err := cm.auditLogger.Log(ctx, GDPRAuditLog{
		TenantID:    tenantID,
		UserID:      userID,
		Action:      AuditActionConsentWithdrawn,
		EntityType:  "consent",
		EntityID:    "all",
		Metadata:    json.RawMessage(fmt.Sprintf(`{"count": %d}`, result.RowsAffected)),
		PerformedBy: userID,
	})
	if err != nil {
		fmt.Printf("failed to create audit log: %v\n", err)
	}

	return nil
}

// GetExpiredConsents returns all expired consents
func (cm *ConsentManager) GetExpiredConsents(ctx context.Context, tenantID string) ([]ConsentRecord, error) {
	var consents []ConsentRecord
	now := time.Now().UTC()

	err := cm.db.WithContext(ctx).
		Where("tenant_id = ? AND granted = ? AND expires_at IS NOT NULL AND expires_at < ?", tenantID, true, now).
		Find(&consents).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get expired consents: %w", err)
	}

	return consents, nil
}

// ProcessExpiredConsents automatically withdraws expired consents
func (cm *ConsentManager) ProcessExpiredConsents(ctx context.Context, tenantID string) (int64, error) {
	now := time.Now().UTC()

	result := cm.db.WithContext(ctx).
		Model(&ConsentRecord{}).
		Where("tenant_id = ? AND granted = ? AND expires_at IS NOT NULL AND expires_at < ?", tenantID, true, now).
		Updates(map[string]interface{}{
			"granted":      false,
			"withdrawn_at": now,
			"updated_at":   now,
		})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to process expired consents: %w", result.Error)
	}

	return result.RowsAffected, nil
}

// BulkRecordConsent records multiple consents at once
func (cm *ConsentManager) BulkRecordConsent(ctx context.Context, requests []ConsentRequest) ([]ConsentRecord, error) {
	if len(requests) == 0 {
		return nil, nil
	}

	var results []ConsentRecord
	var errors []error

	// Process each request in a transaction
	err := cm.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txManager := &ConsentManager{
			db:          tx,
			serviceName: cm.serviceName,
			auditLogger: &AuditLogger{db: tx, serviceName: cm.serviceName},
		}

		for _, req := range requests {
			consent, err := txManager.RecordConsent(ctx, req)
			if err != nil {
				errors = append(errors, fmt.Errorf("user %s purpose %s: %w", req.UserID, req.Purpose, err))
				continue
			}
			results = append(results, *consent)
		}

		if len(errors) > 0 && len(results) == 0 {
			return fmt.Errorf("all consent recordings failed")
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return results, nil
}

// ConsentSummary represents a summary of a user's consents
type ConsentSummary struct {
	UserID       string                  `json:"user_id"`
	TotalCount   int                     `json:"total_count"`
	GrantedCount int                     `json:"granted_count"`
	ExpiredCount int                     `json:"expired_count"`
	ByPurpose    map[ConsentPurpose]bool `json:"by_purpose"`
	LastUpdated  *time.Time              `json:"last_updated,omitempty"`
}

// GetConsentSummary returns a summary of a user's consents
func (cm *ConsentManager) GetConsentSummary(ctx context.Context, tenantID, userID string) (*ConsentSummary, error) {
	if userID == "" {
		return nil, ErrInvalidUserID
	}

	consents, err := cm.ListConsents(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}

	summary := &ConsentSummary{
		UserID:     userID,
		TotalCount: len(consents),
		ByPurpose:  make(map[ConsentPurpose]bool),
	}

	var lastUpdated time.Time
	now := time.Now().UTC()

	for _, consent := range consents {
		if consent.IsActive() {
			summary.GrantedCount++
			summary.ByPurpose[consent.Purpose] = true
		} else if consent.Granted && consent.ExpiresAt != nil && consent.ExpiresAt.Before(now) {
			summary.ExpiredCount++
			summary.ByPurpose[consent.Purpose] = false
		} else {
			summary.ByPurpose[consent.Purpose] = false
		}

		if consent.UpdatedAt.After(lastUpdated) {
			lastUpdated = consent.UpdatedAt
		}
	}

	if !lastUpdated.IsZero() {
		summary.LastUpdated = &lastUpdated
	}

	return summary, nil
}

// AuditLogger handles audit log creation
type AuditLogger struct {
	db          *gorm.DB
	serviceName string
}

// NewAuditLogger creates a new AuditLogger instance
func NewAuditLogger(db *gorm.DB, serviceName string) *AuditLogger {
	return &AuditLogger{
		db:          db,
		serviceName: serviceName,
	}
}

// Log creates an audit log entry
func (al *AuditLogger) Log(ctx context.Context, entry GDPRAuditLog) error {
	entry.ID = uuid.New().String()
	entry.PerformedAt = time.Now().UTC()
	entry.ServiceName = al.serviceName

	// Calculate checksum for integrity
	checksumData := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		entry.ID, entry.UserID, entry.Action, entry.EntityType, entry.EntityID, entry.PerformedAt.Format(time.RFC3339))
	hash := sha256.Sum256([]byte(checksumData))
	entry.Checksum = hex.EncodeToString(hash[:])

	if err := al.db.WithContext(ctx).Create(&entry).Error; err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	return nil
}

// GetAuditLogs retrieves audit logs based on filter criteria
func (al *AuditLogger) GetAuditLogs(ctx context.Context, tenantID, userID string, filter AuditFilter) ([]GDPRAuditLog, error) {
	query := al.db.WithContext(ctx).Where("tenant_id = ?", tenantID)

	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	if len(filter.Actions) > 0 {
		query = query.Where("action IN ?", filter.Actions)
	}

	if filter.StartDate != nil {
		query = query.Where("performed_at >= ?", filter.StartDate)
	}

	if filter.EndDate != nil {
		query = query.Where("performed_at <= ?", filter.EndDate)
	}

	if filter.EntityType != "" {
		query = query.Where("entity_type = ?", filter.EntityType)
	}

	if filter.PerformedBy != "" {
		query = query.Where("performed_by = ?", filter.PerformedBy)
	}

	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	} else {
		query = query.Limit(100) // Default limit
	}

	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	var logs []GDPRAuditLog
	if err := query.Order("performed_at DESC").Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to get audit logs: %w", err)
	}

	return logs, nil
}

// VerifyAuditLogIntegrity verifies the integrity of an audit log entry
func (al *AuditLogger) VerifyAuditLogIntegrity(entry GDPRAuditLog) bool {
	checksumData := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		entry.ID, entry.UserID, entry.Action, entry.EntityType, entry.EntityID, entry.PerformedAt.Format(time.RFC3339))
	hash := sha256.Sum256([]byte(checksumData))
	expectedChecksum := hex.EncodeToString(hash[:])

	return entry.Checksum == expectedChecksum
}
