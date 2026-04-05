package gdpr

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RetentionManager handles data retention policy operations
type RetentionManager struct {
	db            *gorm.DB
	serviceName   string
	auditLogger   *AuditLogger
	dataProviders []DataSourceProvider
}

// NewRetentionManager creates a new RetentionManager instance
func NewRetentionManager(db *gorm.DB, serviceName string, providers []DataSourceProvider) *RetentionManager {
	return &RetentionManager{
		db:            db,
		serviceName:   serviceName,
		auditLogger:   NewAuditLogger(db, serviceName),
		dataProviders: providers,
	}
}

// AddDataProvider adds a data source provider
func (rm *RetentionManager) AddDataProvider(provider DataSourceProvider) {
	rm.dataProviders = append(rm.dataProviders, provider)
}

// CreateRetentionPolicy creates a new retention policy
func (rm *RetentionManager) CreateRetentionPolicy(ctx context.Context, policy RetentionPolicy) (*RetentionPolicy, error) {
	if policy.DataCategory == "" {
		return nil, ErrInvalidDataCategory
	}
	if policy.RetentionDays <= 0 {
		return nil, ErrInvalidRetentionPeriod
	}

	policy.ID = uuid.New().String()
	policy.IsActive = true

	if err := rm.db.WithContext(ctx).Create(&policy).Error; err != nil {
		return nil, fmt.Errorf("failed to create retention policy: %w", err)
	}

	// Create audit log
	policyJSON, _ := json.Marshal(policy)
	err := rm.auditLogger.Log(ctx, GDPRAuditLog{
		TenantID:    policy.TenantID,
		UserID:      policy.CreatedBy,
		Action:      AuditActionRetentionApplied,
		EntityType:  "retention_policy",
		EntityID:    policy.ID,
		NewValue:    policyJSON,
		PerformedBy: policy.CreatedBy,
	})
	if err != nil {
		fmt.Printf("failed to create audit log: %v\n", err)
	}

	return &policy, nil
}

// UpdateRetentionPolicy updates an existing retention policy
func (rm *RetentionManager) UpdateRetentionPolicy(ctx context.Context, policy RetentionPolicy) (*RetentionPolicy, error) {
	if policy.ID == "" {
		return nil, ErrInvalidPolicyID
	}

	var existingPolicy RetentionPolicy
	if err := rm.db.WithContext(ctx).First(&existingPolicy, "id = ?", policy.ID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrPolicyNotFound
		}
		return nil, fmt.Errorf("failed to find policy: %w", err)
	}

	oldPolicyJSON, _ := json.Marshal(existingPolicy)

	// Update fields
	existingPolicy.Name = policy.Name
	existingPolicy.Description = policy.Description
	existingPolicy.RetentionDays = policy.RetentionDays
	existingPolicy.Action = policy.Action
	existingPolicy.IsActive = policy.IsActive
	existingPolicy.LegalBasis = policy.LegalBasis
	existingPolicy.ApplicableTables = policy.ApplicableTables
	existingPolicy.ExclusionCriteria = policy.ExclusionCriteria
	existingPolicy.LastModifiedBy = policy.LastModifiedBy

	if err := rm.db.WithContext(ctx).Save(&existingPolicy).Error; err != nil {
		return nil, fmt.Errorf("failed to update retention policy: %w", err)
	}

	// Create audit log
	newPolicyJSON, _ := json.Marshal(existingPolicy)
	err := rm.auditLogger.Log(ctx, GDPRAuditLog{
		TenantID:    existingPolicy.TenantID,
		UserID:      policy.LastModifiedBy,
		Action:      AuditActionRetentionApplied,
		EntityType:  "retention_policy",
		EntityID:    existingPolicy.ID,
		OldValue:    oldPolicyJSON,
		NewValue:    newPolicyJSON,
		PerformedBy: policy.LastModifiedBy,
	})
	if err != nil {
		fmt.Printf("failed to create audit log: %v\n", err)
	}

	return &existingPolicy, nil
}

// GetRetentionPolicy retrieves a retention policy by ID
func (rm *RetentionManager) GetRetentionPolicy(ctx context.Context, policyID string) (*RetentionPolicy, error) {
	if policyID == "" {
		return nil, ErrInvalidPolicyID
	}

	var policy RetentionPolicy
	if err := rm.db.WithContext(ctx).First(&policy, "id = ?", policyID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrPolicyNotFound
		}
		return nil, fmt.Errorf("failed to get retention policy: %w", err)
	}

	return &policy, nil
}

// GetRetentionPolicies retrieves all retention policies for a tenant
func (rm *RetentionManager) GetRetentionPolicies(ctx context.Context, tenantID string) ([]RetentionPolicy, error) {
	var policies []RetentionPolicy
	query := rm.db.WithContext(ctx)

	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if err := query.Order("data_category").Find(&policies).Error; err != nil {
		return nil, fmt.Errorf("failed to get retention policies: %w", err)
	}

	return policies, nil
}

// GetActiveRetentionPolicies retrieves all active retention policies
func (rm *RetentionManager) GetActiveRetentionPolicies(ctx context.Context, tenantID string) ([]RetentionPolicy, error) {
	var policies []RetentionPolicy
	query := rm.db.WithContext(ctx).Where("is_active = ?", true)

	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if err := query.Order("data_category").Find(&policies).Error; err != nil {
		return nil, fmt.Errorf("failed to get active retention policies: %w", err)
	}

	return policies, nil
}

// GetRetentionPolicyByCategory retrieves a retention policy by data category
func (rm *RetentionManager) GetRetentionPolicyByCategory(ctx context.Context, tenantID string, category DataCategory) (*RetentionPolicy, error) {
	var policy RetentionPolicy
	query := rm.db.WithContext(ctx).Where("data_category = ? AND is_active = ?", category, true)

	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if err := query.First(&policy).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrPolicyNotFound
		}
		return nil, fmt.Errorf("failed to get retention policy: %w", err)
	}

	return &policy, nil
}

// DeleteRetentionPolicy soft-deletes a retention policy
func (rm *RetentionManager) DeleteRetentionPolicy(ctx context.Context, policyID string, deletedBy string) error {
	if policyID == "" {
		return ErrInvalidPolicyID
	}

	var policy RetentionPolicy
	if err := rm.db.WithContext(ctx).First(&policy, "id = ?", policyID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrPolicyNotFound
		}
		return fmt.Errorf("failed to find policy: %w", err)
	}

	policy.IsActive = false
	policy.LastModifiedBy = deletedBy

	if err := rm.db.WithContext(ctx).Save(&policy).Error; err != nil {
		return fmt.Errorf("failed to delete retention policy: %w", err)
	}

	return nil
}

// ScheduleDataDeletion schedules data for deletion after a retention period
func (rm *RetentionManager) ScheduleDataDeletion(ctx context.Context, tenantID, userID string, category DataCategory, retentionDays int) error {
	if userID == "" {
		return ErrInvalidUserID
	}
	if category == "" {
		return ErrInvalidDataCategory
	}
	if retentionDays <= 0 {
		return ErrInvalidRetentionPeriod
	}

	scheduledFor := time.Now().UTC().AddDate(0, 0, retentionDays)

	scheduled := ScheduledDeletion{
		ID:           uuid.New().String(),
		TenantID:     tenantID,
		UserID:       userID,
		DataCategory: category,
		ScheduledFor: scheduledFor,
		Executed:     false,
	}

	if err := rm.db.WithContext(ctx).Create(&scheduled).Error; err != nil {
		return fmt.Errorf("failed to schedule data deletion: %w", err)
	}

	// Create audit log
	metadata, _ := json.Marshal(map[string]interface{}{
		"data_category":  category,
		"retention_days": retentionDays,
		"scheduled_for":  scheduledFor,
	})
	err := rm.auditLogger.Log(ctx, GDPRAuditLog{
		TenantID:    tenantID,
		UserID:      userID,
		Action:      AuditActionRetentionApplied,
		EntityType:  "scheduled_deletion",
		EntityID:    scheduled.ID,
		Metadata:    metadata,
		PerformedBy: "system",
	})
	if err != nil {
		fmt.Printf("failed to create audit log: %v\n", err)
	}

	return nil
}

// ScheduleDataDeletionWithPolicy schedules data deletion based on a retention policy
func (rm *RetentionManager) ScheduleDataDeletionWithPolicy(ctx context.Context, tenantID, userID string, policy RetentionPolicy) error {
	if userID == "" {
		return ErrInvalidUserID
	}
	if policy.ID == "" {
		return ErrInvalidPolicyID
	}

	scheduledFor := time.Now().UTC().AddDate(0, 0, policy.RetentionDays)

	scheduled := ScheduledDeletion{
		ID:           uuid.New().String(),
		TenantID:     tenantID,
		UserID:       userID,
		DataCategory: policy.DataCategory,
		ScheduledFor: scheduledFor,
		Executed:     false,
		PolicyID:     policy.ID,
	}

	if err := rm.db.WithContext(ctx).Create(&scheduled).Error; err != nil {
		return fmt.Errorf("failed to schedule data deletion: %w", err)
	}

	return nil
}

// CleanupExpiredData processes all scheduled deletions that are due
func (rm *RetentionManager) CleanupExpiredData(ctx context.Context, tenantID string) (*CleanupResult, error) {
	now := time.Now().UTC()
	result := &CleanupResult{
		ProcessedAt: now,
		ByCategory:  make(map[DataCategory]int),
		Errors:      []string{},
	}

	// Find all scheduled deletions that are due
	var scheduled []ScheduledDeletion
	query := rm.db.WithContext(ctx).
		Where("scheduled_for <= ? AND executed = ?", now, false)

	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if err := query.Find(&scheduled).Error; err != nil {
		return nil, fmt.Errorf("failed to get scheduled deletions: %w", err)
	}

	result.TotalProcessed = len(scheduled)

	// Group by user for efficient processing
	userDeletions := make(map[string][]ScheduledDeletion)
	for _, s := range scheduled {
		userDeletions[s.UserID] = append(userDeletions[s.UserID], s)
	}

	// Process each user's deletions
	for userID, deletions := range userDeletions {
		for _, deletion := range deletions {
			var deleted bool
			var err error

			// Find the policy to determine action
			var policy *RetentionPolicy
			if deletion.PolicyID != "" {
				policy, _ = rm.GetRetentionPolicy(ctx, deletion.PolicyID)
			}

			action := "DELETE"
			if policy != nil && policy.Action != "" {
				action = policy.Action
			}

			switch action {
			case "ANONYMIZE":
				err = rm.anonymizeDataForCategory(ctx, userID, deletion.DataCategory)
				if err == nil {
					result.TotalAnonymized++
					deleted = true
				}
			case "ARCHIVE":
				// Archive would be implemented based on specific requirements
				err = fmt.Errorf("archive action not yet implemented")
			default: // DELETE
				err = rm.deleteDataForCategory(ctx, userID, deletion.DataCategory)
				if err == nil {
					result.TotalDeleted++
					deleted = true
				}
			}

			if err != nil {
				result.TotalFailed++
				result.Errors = append(result.Errors, fmt.Sprintf("user %s category %s: %v", userID, deletion.DataCategory, err))
			} else if deleted {
				result.ByCategory[deletion.DataCategory]++

				// Mark as executed
				executedAt := time.Now().UTC()
				deletion.Executed = true
				deletion.ExecutedAt = &executedAt
				rm.db.WithContext(ctx).Save(&deletion)

				// Create audit log
				rm.auditLogger.Log(ctx, GDPRAuditLog{
					TenantID:    deletion.TenantID,
					UserID:      userID,
					Action:      AuditActionDataDeleted,
					EntityType:  "scheduled_deletion",
					EntityID:    deletion.ID,
					Metadata:    json.RawMessage(fmt.Sprintf(`{"category": "%s", "action": "%s"}`, deletion.DataCategory, action)),
					PerformedBy: "system",
				})
			}
		}
	}

	return result, nil
}

// deleteDataForCategory deletes data for a specific category using registered providers
func (rm *RetentionManager) deleteDataForCategory(ctx context.Context, userID string, category DataCategory) error {
	for _, provider := range rm.dataProviders {
		categories := provider.GetDataCategories()
		for _, c := range categories {
			if c == category {
				if err := provider.DeleteUserData(ctx, userID); err != nil {
					return fmt.Errorf("failed to delete from %s: %w", provider.GetServiceName(), err)
				}
			}
		}
	}
	return nil
}

// anonymizeDataForCategory anonymizes data for a specific category
func (rm *RetentionManager) anonymizeDataForCategory(ctx context.Context, userID string, category DataCategory) error {
	for _, provider := range rm.dataProviders {
		categories := provider.GetDataCategories()
		for _, c := range categories {
			if c == category {
				if err := provider.AnonymizeUserData(ctx, userID); err != nil {
					return fmt.Errorf("failed to anonymize in %s: %w", provider.GetServiceName(), err)
				}
			}
		}
	}
	return nil
}

// AnonymizeData anonymizes all user data instead of deleting it
func (rm *RetentionManager) AnonymizeData(ctx context.Context, tenantID, userID string) (*AnonymizationResult, error) {
	if userID == "" {
		return nil, ErrInvalidUserID
	}

	result := &AnonymizationResult{
		UserID:           userID,
		ProcessedAt:      time.Now().UTC(),
		ByTable:          make(map[string]int),
		AnonymizedFields: []string{},
	}

	var errors []string

	// Anonymize data from all providers
	for _, provider := range rm.dataProviders {
		err := provider.AnonymizeUserData(ctx, userID)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", provider.GetServiceName(), err))
		} else {
			result.ByTable[provider.GetServiceName()]++
			result.TotalRecords++
		}
	}

	// Anonymize consent records
	err := rm.anonymizeConsentRecords(ctx, tenantID, userID)
	if err != nil {
		errors = append(errors, fmt.Sprintf("consent_records: %v", err))
	} else {
		result.ByTable["consent_records"]++
		result.AnonymizedFields = append(result.AnonymizedFields, "ip_address", "user_agent")
	}

	if len(errors) > 0 && result.TotalRecords == 0 {
		return nil, fmt.Errorf("failed to anonymize any data: %v", errors)
	}

	// Create audit log
	resultJSON, _ := json.Marshal(result)
	rm.auditLogger.Log(ctx, GDPRAuditLog{
		TenantID:    tenantID,
		UserID:      userID,
		Action:      AuditActionDataAnonymized,
		EntityType:  "user_data",
		EntityID:    userID,
		NewValue:    resultJSON,
		PerformedBy: "system",
	})

	return result, nil
}

// anonymizeConsentRecords anonymizes PII in consent records
func (rm *RetentionManager) anonymizeConsentRecords(ctx context.Context, tenantID, userID string) error {
	result := rm.db.WithContext(ctx).
		Model(&ConsentRecord{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Updates(map[string]interface{}{
			"ip_address": "[ANONYMIZED]",
			"user_agent": "[ANONYMIZED]",
		})

	if result.Error != nil {
		return result.Error
	}

	return nil
}

// GetScheduledDeletions retrieves all scheduled deletions for a user
func (rm *RetentionManager) GetScheduledDeletions(ctx context.Context, tenantID, userID string) ([]ScheduledDeletion, error) {
	var scheduled []ScheduledDeletion
	query := rm.db.WithContext(ctx).Where("executed = ?", false)

	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Order("scheduled_for").Find(&scheduled).Error; err != nil {
		return nil, fmt.Errorf("failed to get scheduled deletions: %w", err)
	}

	return scheduled, nil
}

// CancelScheduledDeletion cancels a scheduled deletion
func (rm *RetentionManager) CancelScheduledDeletion(ctx context.Context, deletionID string) error {
	if deletionID == "" {
		return ErrInvalidDeletionID
	}

	result := rm.db.WithContext(ctx).Delete(&ScheduledDeletion{}, "id = ?", deletionID)
	if result.Error != nil {
		return fmt.Errorf("failed to cancel scheduled deletion: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrDeletionNotFound
	}

	return nil
}

// GetRetentionAuditLog retrieves audit logs for retention actions
func (rm *RetentionManager) GetRetentionAuditLog(ctx context.Context, tenantID string, filter AuditFilter) ([]GDPRAuditLog, error) {
	// Default to retention-related actions
	if len(filter.Actions) == 0 {
		filter.Actions = []AuditAction{
			AuditActionRetentionApplied,
			AuditActionDataDeleted,
			AuditActionDataAnonymized,
		}
	}

	return rm.auditLogger.GetAuditLogs(ctx, tenantID, "", filter)
}

// ApplyRetentionPolicyToExistingData applies a retention policy to existing data
func (rm *RetentionManager) ApplyRetentionPolicyToExistingData(ctx context.Context, policy RetentionPolicy, dataCreatedBefore time.Time) (*CleanupResult, error) {
	if policy.ID == "" {
		return nil, ErrInvalidPolicyID
	}

	// Calculate which data should be deleted based on the retention period
	cutoffDate := dataCreatedBefore.AddDate(0, 0, -policy.RetentionDays)

	result := &CleanupResult{
		ProcessedAt: time.Now().UTC(),
		ByCategory:  make(map[DataCategory]int),
		Errors:      []string{},
	}

	// This is a placeholder - actual implementation would depend on specific table structure
	// Each service would need to implement its own logic for identifying and processing old data

	// Create audit log
	metadata, _ := json.Marshal(map[string]interface{}{
		"policy_id":     policy.ID,
		"cutoff_date":   cutoffDate,
		"data_category": policy.DataCategory,
	})
	rm.auditLogger.Log(ctx, GDPRAuditLog{
		TenantID:    policy.TenantID,
		UserID:      "system",
		Action:      AuditActionRetentionApplied,
		EntityType:  "bulk_retention",
		EntityID:    policy.ID,
		Metadata:    metadata,
		PerformedBy: "system",
	})

	return result, nil
}

// DefaultRetentionPolicies returns a set of default retention policies
func DefaultRetentionPolicies(tenantID, createdBy string) []RetentionPolicy {
	return []RetentionPolicy{
		{
			TenantID:      tenantID,
			Name:          "Identity Data Retention",
			Description:   "Retention policy for identity information (name, email, etc.)",
			DataCategory:  DataCategoryIdentity,
			RetentionDays: 365 * 3, // 3 years
			Action:        "ANONYMIZE",
			LegalBasis:    "Legitimate interest - account management",
			CreatedBy:     createdBy,
		},
		{
			TenantID:      tenantID,
			Name:          "Financial Data Retention",
			Description:   "Retention policy for financial records",
			DataCategory:  DataCategoryFinancial,
			RetentionDays: 365 * 7, // 7 years for tax compliance
			Action:        "ARCHIVE",
			LegalBasis:    "Legal obligation - tax and accounting regulations",
			CreatedBy:     createdBy,
		},
		{
			TenantID:      tenantID,
			Name:          "Transaction Data Retention",
			Description:   "Retention policy for transaction history",
			DataCategory:  DataCategoryTransactional,
			RetentionDays: 365 * 5, // 5 years
			Action:        "ANONYMIZE",
			LegalBasis:    "Legal obligation - consumer protection",
			CreatedBy:     createdBy,
		},
		{
			TenantID:      tenantID,
			Name:          "Technical Data Retention",
			Description:   "Retention policy for technical data (IP addresses, device info)",
			DataCategory:  DataCategoryTechnical,
			RetentionDays: 365, // 1 year
			Action:        "DELETE",
			LegalBasis:    "Legitimate interest - security",
			CreatedBy:     createdBy,
		},
		{
			TenantID:      tenantID,
			Name:          "Usage Data Retention",
			Description:   "Retention policy for usage and activity data",
			DataCategory:  DataCategoryUsage,
			RetentionDays: 365 * 2, // 2 years
			Action:        "ANONYMIZE",
			LegalBasis:    "Legitimate interest - service improvement",
			CreatedBy:     createdBy,
		},
		{
			TenantID:      tenantID,
			Name:          "Marketing Data Retention",
			Description:   "Retention policy for marketing preferences and campaigns",
			DataCategory:  DataCategoryMarketing,
			RetentionDays: 365 * 2, // 2 years after consent withdrawal
			Action:        "DELETE",
			LegalBasis:    "Consent",
			CreatedBy:     createdBy,
		},
	}
}
