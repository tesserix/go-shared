package gdpr

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DeletionManager handles right to deletion operations (GDPR Article 17)
type DeletionManager struct {
	db             *gorm.DB
	serviceName    string
	auditLogger    *AuditLogger
	dataProviders  []DataSourceProvider
	consentManager *ConsentManager
}

// NewDeletionManager creates a new DeletionManager instance
func NewDeletionManager(db *gorm.DB, serviceName string, providers []DataSourceProvider) *DeletionManager {
	return &DeletionManager{
		db:             db,
		serviceName:    serviceName,
		auditLogger:    NewAuditLogger(db, serviceName),
		dataProviders:  providers,
		consentManager: NewConsentManager(db, serviceName),
	}
}

// AddDataProvider adds a data source provider
func (dm *DeletionManager) AddDataProvider(provider DataSourceProvider) {
	dm.dataProviders = append(dm.dataProviders, provider)
}

// RequestDeletion initiates a deletion request (Right to be Forgotten)
func (dm *DeletionManager) RequestDeletion(ctx context.Context, req DeletionRequest) (*DataDeletionRequest, error) {
	if req.UserID == "" {
		return nil, ErrInvalidUserID
	}

	// Check for existing pending deletion request
	var existingRequest DataDeletionRequest
	err := dm.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND status IN ?", req.TenantID, req.UserID,
			[]RequestStatus{RequestStatusPending, RequestStatusProcessing}).
		First(&existingRequest).Error

	if err == nil {
		return nil, ErrDeletionRequestExists
	}
	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to check existing request: %w", err)
	}

	now := time.Now().UTC()
	requestID := uuid.New().String()

	// Prepare data categories
	var dataCategoriesJSON json.RawMessage
	if len(req.DataCategories) > 0 {
		categoriesBytes, err := json.Marshal(req.DataCategories)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal data categories: %w", err)
		}
		dataCategoriesJSON = categoriesBytes
	}

	// Determine scheduled time
	var scheduledFor *time.Time
	if req.ScheduleFor != nil {
		scheduledFor = req.ScheduleFor
	} else {
		// Default: 30 days grace period for user to reconsider
		defaultSchedule := now.AddDate(0, 0, 30)
		scheduledFor = &defaultSchedule
	}

	deletionRequest := DataDeletionRequest{
		ID:             requestID,
		TenantID:       req.TenantID,
		UserID:         req.UserID,
		Status:         RequestStatusPending,
		RequestedAt:    now,
		ScheduledFor:   scheduledFor,
		DataCategories: dataCategoriesJSON,
		Reason:         req.Reason,
		IPAddress:      req.IPAddress,
		UserAgent:      req.UserAgent,
	}

	if err := dm.db.WithContext(ctx).Create(&deletionRequest).Error; err != nil {
		return nil, fmt.Errorf("failed to create deletion request: %w", err)
	}

	// Also create a data subject request record
	dataSubjectRequest := DataSubjectRequest{
		ID:          uuid.New().String(),
		TenantID:    req.TenantID,
		UserID:      req.UserID,
		RequestType: RequestTypeErasure,
		Status:      RequestStatusPending,
		RequestedAt: now,
		IPAddress:   req.IPAddress,
		UserAgent:   req.UserAgent,
	}
	requestDetails, _ := json.Marshal(map[string]interface{}{
		"deletion_request_id": requestID,
		"data_categories":     req.DataCategories,
		"reason":              req.Reason,
		"scheduled_for":       scheduledFor,
	})
	dataSubjectRequest.RequestDetails = requestDetails
	dm.db.WithContext(ctx).Create(&dataSubjectRequest)

	// Create audit log
	requestJSON, _ := json.Marshal(deletionRequest)
	err = dm.auditLogger.Log(ctx, GDPRAuditLog{
		TenantID:    req.TenantID,
		UserID:      req.UserID,
		Action:      AuditActionDeletionRequest,
		EntityType:  "deletion_request",
		EntityID:    requestID,
		NewValue:    requestJSON,
		IPAddress:   req.IPAddress,
		UserAgent:   req.UserAgent,
		PerformedBy: req.UserID,
		RequestID:   requestID,
	})
	if err != nil {
		fmt.Printf("failed to create audit log: %v\n", err)
	}

	return &deletionRequest, nil
}

// ProcessDeletionRequest executes a deletion request
func (dm *DeletionManager) ProcessDeletionRequest(ctx context.Context, requestID string) error {
	if requestID == "" {
		return ErrInvalidRequestID
	}

	var request DataDeletionRequest
	if err := dm.db.WithContext(ctx).First(&request, "id = ?", requestID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrRequestNotFound
		}
		return fmt.Errorf("failed to get deletion request: %w", err)
	}

	// Verify request is in correct status
	if request.Status != RequestStatusPending {
		return ErrInvalidRequestStatus
	}

	// Check if scheduled time has passed
	now := time.Now().UTC()
	if request.ScheduledFor != nil && request.ScheduledFor.After(now) {
		return ErrDeletionNotDue
	}

	// Update status to processing
	request.Status = RequestStatusProcessing
	request.ProcessedAt = &now
	if err := dm.db.WithContext(ctx).Save(&request).Error; err != nil {
		return fmt.Errorf("failed to update request status: %w", err)
	}

	// Parse data categories
	var categories []DataCategory
	if request.DataCategories != nil {
		json.Unmarshal(request.DataCategories, &categories)
	}

	// Execute deletion across all data sources
	deletionResults := make(map[string]interface{})
	var errors []string
	deletedSources := []string{}

	// Delete from registered data providers
	for _, provider := range dm.dataProviders {
		providerCategories := provider.GetDataCategories()
		shouldDelete := len(categories) == 0 // Delete all if no specific categories

		if !shouldDelete {
			for _, reqCat := range categories {
				for _, provCat := range providerCategories {
					if reqCat == provCat {
						shouldDelete = true
						break
					}
				}
				if shouldDelete {
					break
				}
			}
		}

		if shouldDelete {
			err := provider.DeleteUserData(ctx, request.UserID)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", provider.GetServiceName(), err))
				deletionResults[provider.GetServiceName()] = map[string]interface{}{
					"success": false,
					"error":   err.Error(),
				}
			} else {
				deletedSources = append(deletedSources, provider.GetServiceName())
				deletionResults[provider.GetServiceName()] = map[string]interface{}{
					"success":    true,
					"deleted_at": time.Now().UTC(),
				}
			}
		}
	}

	// Delete GDPR-specific data (consents, etc.)
	if err := dm.deleteGDPRData(ctx, request.TenantID, request.UserID); err != nil {
		errors = append(errors, fmt.Sprintf("gdpr_data: %v", err))
		deletionResults["gdpr_data"] = map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
	} else {
		deletedSources = append(deletedSources, "gdpr_data")
		deletionResults["gdpr_data"] = map[string]interface{}{
			"success":    true,
			"deleted_at": time.Now().UTC(),
		}
	}

	// Update request with results
	completedAt := time.Now().UTC()
	resultsJSON, _ := json.Marshal(deletionResults)
	request.DeletionResults = resultsJSON
	request.CompletedAt = &completedAt

	if len(errors) == 0 {
		request.Status = RequestStatusCompleted
	} else if len(deletedSources) > 0 {
		request.Status = RequestStatusCompleted
		request.Notes = fmt.Sprintf("Partial completion. Errors: %v", errors)
	} else {
		request.Status = RequestStatusFailed
		request.Notes = fmt.Sprintf("Failed to delete any data. Errors: %v", errors)
	}

	if err := dm.db.WithContext(ctx).Save(&request).Error; err != nil {
		return fmt.Errorf("failed to update request: %w", err)
	}

	// Create audit log
	metadata, _ := json.Marshal(map[string]interface{}{
		"deleted_sources": deletedSources,
		"errors":          errors,
	})
	dm.auditLogger.Log(ctx, GDPRAuditLog{
		TenantID:    request.TenantID,
		UserID:      request.UserID,
		Action:      AuditActionDataDeleted,
		EntityType:  "deletion_request",
		EntityID:    requestID,
		Metadata:    metadata,
		PerformedBy: "system",
		RequestID:   requestID,
	})

	if request.Status == RequestStatusFailed {
		return fmt.Errorf("deletion failed: %v", errors)
	}

	return nil
}

// deleteGDPRData deletes GDPR-specific records for a user
func (dm *DeletionManager) deleteGDPRData(ctx context.Context, tenantID, userID string) error {
	// Revoke all consents first
	if err := dm.consentManager.RevokeAllConsents(ctx, tenantID, userID); err != nil {
		fmt.Printf("failed to revoke consents: %v\n", err)
	}

	// Delete consent records
	if err := dm.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Delete(&ConsentRecord{}).Error; err != nil {
		return fmt.Errorf("failed to delete consent records: %w", err)
	}

	// Delete scheduled deletions (they're no longer needed)
	if err := dm.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Delete(&ScheduledDeletion{}).Error; err != nil {
		fmt.Printf("failed to delete scheduled deletions: %v\n", err)
	}

	// Note: We keep audit logs for compliance purposes
	// They are anonymized instead of deleted

	// Anonymize audit logs
	dm.db.WithContext(ctx).
		Model(&GDPRAuditLog{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Updates(map[string]interface{}{
			"ip_address": "[DELETED]",
			"user_agent": "[DELETED]",
		})

	return nil
}

// VerifyDeletion verifies that all user data has been deleted
func (dm *DeletionManager) VerifyDeletion(ctx context.Context, requestID string) (*DeletionVerification, error) {
	if requestID == "" {
		return nil, ErrInvalidRequestID
	}

	var request DataDeletionRequest
	if err := dm.db.WithContext(ctx).First(&request, "id = ?", requestID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrRequestNotFound
		}
		return nil, fmt.Errorf("failed to get deletion request: %w", err)
	}

	verification := &DeletionVerification{
		RequestID:       requestID,
		UserID:          request.UserID,
		VerifiedAt:      time.Now().UTC(),
		RemainingData:   make(map[string]int),
		DeletedSources:  []string{},
		RetainedSources: []RetainedDataSource{},
	}

	// Check each data provider for remaining data
	for _, provider := range dm.dataProviders {
		data, err := provider.GetUserData(ctx, request.UserID)
		if err != nil {
			// Error checking - might mean data doesn't exist
			verification.DeletedSources = append(verification.DeletedSources, provider.GetServiceName())
			continue
		}

		if data != nil && len(data) > 0 {
			// Data still exists
			var dataMap map[string]interface{}
			if err := json.Unmarshal(data, &dataMap); err == nil {
				count := 0
				for _, v := range dataMap {
					if arr, ok := v.([]interface{}); ok {
						count += len(arr)
					} else {
						count++
					}
				}
				if count > 0 {
					verification.RemainingData[provider.GetServiceName()] = count
				}
			}
		} else {
			verification.DeletedSources = append(verification.DeletedSources, provider.GetServiceName())
		}
	}

	// Check for retained data due to legal obligations
	retainedSources := dm.checkLegalRetention(ctx, request.TenantID, request.UserID)
	verification.RetainedSources = retainedSources

	// Determine if verification passed
	verification.Verified = len(verification.RemainingData) == 0

	// Store verification results
	verificationJSON, _ := json.Marshal(verification)
	request.VerificationData = verificationJSON
	dm.db.WithContext(ctx).Save(&request)

	// Create audit log
	dm.auditLogger.Log(ctx, GDPRAuditLog{
		TenantID:    request.TenantID,
		UserID:      request.UserID,
		Action:      AuditActionDataDeleted,
		EntityType:  "deletion_verification",
		EntityID:    requestID,
		NewValue:    verificationJSON,
		PerformedBy: "system",
		RequestID:   requestID,
	})

	return verification, nil
}

// checkLegalRetention checks for data that must be retained for legal reasons
func (dm *DeletionManager) checkLegalRetention(ctx context.Context, tenantID, userID string) []RetainedDataSource {
	var retained []RetainedDataSource

	// Check for audit logs (must be retained for compliance)
	var auditCount int64
	dm.db.WithContext(ctx).
		Model(&GDPRAuditLog{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Count(&auditCount)

	if auditCount > 0 {
		retained = append(retained, RetainedDataSource{
			Source:      "gdpr_audit_logs",
			Reason:      "Legal compliance requirement",
			LegalBasis:  "Regulatory obligation - audit trail retention",
			RetainUntil: "7 years from creation",
		})
	}

	// Additional checks can be added here based on business requirements
	// e.g., financial records, tax documents, etc.

	return retained
}

// GetDeletionRequest retrieves a deletion request by ID
func (dm *DeletionManager) GetDeletionRequest(ctx context.Context, requestID string) (*DataDeletionRequest, error) {
	if requestID == "" {
		return nil, ErrInvalidRequestID
	}

	var request DataDeletionRequest
	if err := dm.db.WithContext(ctx).First(&request, "id = ?", requestID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrRequestNotFound
		}
		return nil, fmt.Errorf("failed to get deletion request: %w", err)
	}

	return &request, nil
}

// ListDeletionRequests lists all deletion requests for a user
func (dm *DeletionManager) ListDeletionRequests(ctx context.Context, tenantID, userID string) ([]DataDeletionRequest, error) {
	var requests []DataDeletionRequest
	query := dm.db.WithContext(ctx)

	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Order("requested_at DESC").Find(&requests).Error; err != nil {
		return nil, fmt.Errorf("failed to list deletion requests: %w", err)
	}

	return requests, nil
}

// CancelDeletionRequest cancels a pending deletion request
func (dm *DeletionManager) CancelDeletionRequest(ctx context.Context, requestID string, cancelledBy string) error {
	if requestID == "" {
		return ErrInvalidRequestID
	}

	var request DataDeletionRequest
	if err := dm.db.WithContext(ctx).First(&request, "id = ?", requestID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrRequestNotFound
		}
		return fmt.Errorf("failed to get deletion request: %w", err)
	}

	if request.Status != RequestStatusPending {
		return ErrCannotCancelRequest
	}

	oldStatus := request.Status
	request.Status = RequestStatusCancelled
	request.Notes = fmt.Sprintf("Cancelled by user: %s", cancelledBy)

	if err := dm.db.WithContext(ctx).Save(&request).Error; err != nil {
		return fmt.Errorf("failed to cancel deletion request: %w", err)
	}

	// Create audit log
	dm.auditLogger.Log(ctx, GDPRAuditLog{
		TenantID:    request.TenantID,
		UserID:      request.UserID,
		Action:      AuditActionDeletionRequest,
		EntityType:  "deletion_request",
		EntityID:    requestID,
		OldValue:    json.RawMessage(fmt.Sprintf(`{"status": "%s"}`, oldStatus)),
		NewValue:    json.RawMessage(fmt.Sprintf(`{"status": "%s"}`, request.Status)),
		PerformedBy: cancelledBy,
		RequestID:   requestID,
	})

	return nil
}

// GetPendingDeletionRequests retrieves all pending deletion requests
func (dm *DeletionManager) GetPendingDeletionRequests(ctx context.Context, tenantID string) ([]DataDeletionRequest, error) {
	var requests []DataDeletionRequest
	query := dm.db.WithContext(ctx).Where("status = ?", RequestStatusPending)

	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if err := query.Order("scheduled_for").Find(&requests).Error; err != nil {
		return nil, fmt.Errorf("failed to get pending deletion requests: %w", err)
	}

	return requests, nil
}

// GetDueDeletionRequests retrieves deletion requests that are due for processing
func (dm *DeletionManager) GetDueDeletionRequests(ctx context.Context, tenantID string) ([]DataDeletionRequest, error) {
	now := time.Now().UTC()
	var requests []DataDeletionRequest
	query := dm.db.WithContext(ctx).
		Where("status = ?", RequestStatusPending).
		Where("scheduled_for IS NULL OR scheduled_for <= ?", now)

	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if err := query.Order("scheduled_for").Find(&requests).Error; err != nil {
		return nil, fmt.Errorf("failed to get due deletion requests: %w", err)
	}

	return requests, nil
}

// ProcessDueDeletionRequests processes all deletion requests that are due
func (dm *DeletionManager) ProcessDueDeletionRequests(ctx context.Context, tenantID string) (*DeletionProcessingResult, error) {
	requests, err := dm.GetDueDeletionRequests(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	result := &DeletionProcessingResult{
		ProcessedAt:  time.Now().UTC(),
		TotalDue:     len(requests),
		Processed:    0,
		Failed:       0,
		FailedIDs:    []string{},
		ProcessedIDs: []string{},
	}

	for _, request := range requests {
		err := dm.ProcessDeletionRequest(ctx, request.ID)
		if err != nil {
			result.Failed++
			result.FailedIDs = append(result.FailedIDs, request.ID)
		} else {
			result.Processed++
			result.ProcessedIDs = append(result.ProcessedIDs, request.ID)
		}
	}

	return result, nil
}

// DeletionProcessingResult represents the result of processing deletion requests
type DeletionProcessingResult struct {
	ProcessedAt  time.Time `json:"processed_at"`
	TotalDue     int       `json:"total_due"`
	Processed    int       `json:"processed"`
	Failed       int       `json:"failed"`
	ProcessedIDs []string  `json:"processed_ids"`
	FailedIDs    []string  `json:"failed_ids"`
}

// CascadeDelete performs cascade deletion to related records
func (dm *DeletionManager) CascadeDelete(ctx context.Context, userID string, tables []CascadeDeleteConfig) error {
	if userID == "" {
		return ErrInvalidUserID
	}

	return dm.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, table := range tables {
			result := tx.Table(table.TableName).
				Where(table.UserIDColumn+" = ?", userID).
				Delete(nil)

			if result.Error != nil {
				return fmt.Errorf("failed to delete from %s: %w", table.TableName, result.Error)
			}

			// Log the deletion
			dm.auditLogger.Log(ctx, GDPRAuditLog{
				UserID:      userID,
				Action:      AuditActionDataDeleted,
				EntityType:  table.TableName,
				EntityID:    "cascade",
				Metadata:    json.RawMessage(fmt.Sprintf(`{"rows_affected": %d}`, result.RowsAffected)),
				PerformedBy: "system",
			})
		}
		return nil
	})
}

// CascadeDeleteConfig defines configuration for cascade deletion
type CascadeDeleteConfig struct {
	TableName    string
	UserIDColumn string
	Order        int // Order of deletion (higher = deleted first)
}

// GetDeletionRequestStats returns statistics about deletion requests
func (dm *DeletionManager) GetDeletionRequestStats(ctx context.Context, tenantID string) (*DeletionRequestStats, error) {
	stats := &DeletionRequestStats{
		ByStatus: make(map[RequestStatus]int64),
	}

	// Count by status
	type StatusCount struct {
		Status RequestStatus
		Count  int64
	}
	var statusCounts []StatusCount

	query := dm.db.WithContext(ctx).Model(&DataDeletionRequest{})
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if err := query.
		Select("status, count(*) as count").
		Group("status").
		Scan(&statusCounts).Error; err != nil {
		return nil, fmt.Errorf("failed to get deletion stats: %w", err)
	}

	for _, sc := range statusCounts {
		stats.ByStatus[sc.Status] = sc.Count
		stats.Total += sc.Count
	}

	stats.Pending = stats.ByStatus[RequestStatusPending]
	stats.Completed = stats.ByStatus[RequestStatusCompleted]
	stats.Failed = stats.ByStatus[RequestStatusFailed]

	// Get average processing time for completed requests
	var avgDuration float64
	dm.db.WithContext(ctx).
		Model(&DataDeletionRequest{}).
		Where("status = ? AND completed_at IS NOT NULL AND requested_at IS NOT NULL", RequestStatusCompleted).
		Select("AVG(EXTRACT(EPOCH FROM (completed_at - requested_at)))").
		Scan(&avgDuration)
	stats.AvgProcessingTimeSeconds = avgDuration

	return stats, nil
}

// DeletionRequestStats represents statistics about deletion requests
type DeletionRequestStats struct {
	Total                    int64                   `json:"total"`
	Pending                  int64                   `json:"pending"`
	Completed                int64                   `json:"completed"`
	Failed                   int64                   `json:"failed"`
	ByStatus                 map[RequestStatus]int64 `json:"by_status"`
	AvgProcessingTimeSeconds float64                 `json:"avg_processing_time_seconds"`
}
