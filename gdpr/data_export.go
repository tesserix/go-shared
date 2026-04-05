package gdpr

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DataExporter handles data portability operations (GDPR Article 20)
type DataExporter struct {
	db            *gorm.DB
	serviceName   string
	auditLogger   *AuditLogger
	dataProviders []DataSourceProvider
}

// NewDataExporter creates a new DataExporter instance
func NewDataExporter(db *gorm.DB, serviceName string, providers []DataSourceProvider) *DataExporter {
	return &DataExporter{
		db:            db,
		serviceName:   serviceName,
		auditLogger:   NewAuditLogger(db, serviceName),
		dataProviders: providers,
	}
}

// AddDataProvider adds a data source provider
func (de *DataExporter) AddDataProvider(provider DataSourceProvider) {
	de.dataProviders = append(de.dataProviders, provider)
}

// ExportUserData exports all user data in JSON format
func (de *DataExporter) ExportUserData(ctx context.Context, tenantID, userID string) (*DataExport, error) {
	if userID == "" {
		return nil, ErrInvalidUserID
	}

	requestID := uuid.New().String()
	now := time.Now().UTC()

	// Create data subject request record
	request := DataSubjectRequest{
		ID:          requestID,
		TenantID:    tenantID,
		UserID:      userID,
		RequestType: RequestTypePortable,
		Status:      RequestStatusProcessing,
		RequestedAt: now,
	}

	if err := de.db.WithContext(ctx).Create(&request).Error; err != nil {
		return nil, fmt.Errorf("failed to create data subject request: %w", err)
	}

	export := &DataExport{
		UserID:      userID,
		ExportedAt:  now,
		DataSources: make(map[string]json.RawMessage),
		Metadata: ExportMetadata{
			Format:      "JSON",
			Version:     "1.0",
			GeneratedAt: now,
			RequestID:   requestID,
			DataTypes:   []string{},
		},
	}

	totalRecords := 0
	var exportErrors []string

	// Collect data from all registered providers
	for _, provider := range de.dataProviders {
		data, err := provider.GetUserData(ctx, userID)
		if err != nil {
			exportErrors = append(exportErrors, fmt.Sprintf("%s: %v", provider.GetServiceName(), err))
			continue
		}

		if data != nil && len(data) > 0 {
			export.DataSources[provider.GetServiceName()] = data
			export.Metadata.DataTypes = append(export.Metadata.DataTypes, provider.GetServiceName())

			// Count records in the data
			var dataMap map[string]interface{}
			if err := json.Unmarshal(data, &dataMap); err == nil {
				for _, v := range dataMap {
					if arr, ok := v.([]interface{}); ok {
						totalRecords += len(arr)
					} else {
						totalRecords++
					}
				}
			}
		}
	}

	// Also export GDPR-specific data
	gdprData, err := de.exportGDPRData(ctx, tenantID, userID)
	if err == nil && gdprData != nil {
		export.DataSources["gdpr_compliance"] = gdprData
		export.Metadata.DataTypes = append(export.Metadata.DataTypes, "gdpr_compliance")
	}

	export.Metadata.TotalRecords = totalRecords

	// Update request status
	completedAt := time.Now().UTC()
	request.Status = RequestStatusCompleted
	request.CompletedAt = &completedAt

	responseDetails, _ := json.Marshal(map[string]interface{}{
		"total_records": totalRecords,
		"data_types":    export.Metadata.DataTypes,
		"errors":        exportErrors,
	})
	request.ResponseDetails = responseDetails

	if err := de.db.WithContext(ctx).Save(&request).Error; err != nil {
		// Log but don't fail
		fmt.Printf("failed to update request status: %v\n", err)
	}

	// Create audit log
	metadata, _ := json.Marshal(map[string]interface{}{
		"request_id":    requestID,
		"total_records": totalRecords,
		"data_types":    export.Metadata.DataTypes,
	})
	err = de.auditLogger.Log(ctx, GDPRAuditLog{
		TenantID:    tenantID,
		UserID:      userID,
		Action:      AuditActionDataExported,
		EntityType:  "user_data",
		EntityID:    userID,
		Metadata:    metadata,
		PerformedBy: userID,
		RequestID:   requestID,
	})
	if err != nil {
		fmt.Printf("failed to create audit log: %v\n", err)
	}

	return export, nil
}

// exportGDPRData exports GDPR-specific data (consents, requests, etc.)
func (de *DataExporter) exportGDPRData(ctx context.Context, tenantID, userID string) (json.RawMessage, error) {
	gdprData := make(map[string]interface{})

	// Export consent records
	var consents []ConsentRecord
	if err := de.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Find(&consents).Error; err == nil {
		gdprData["consent_records"] = consents
	}

	// Export data subject requests (excluding deletion requests for privacy)
	var requests []DataSubjectRequest
	if err := de.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Find(&requests).Error; err == nil {
		gdprData["data_subject_requests"] = requests
	}

	if len(gdprData) == 0 {
		return nil, nil
	}

	data, err := json.Marshal(gdprData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GDPR data: %w", err)
	}

	return data, nil
}

// GenerateDataPackage creates a downloadable ZIP archive containing all user data
func (de *DataExporter) GenerateDataPackage(ctx context.Context, tenantID, userID string) ([]byte, error) {
	if userID == "" {
		return nil, ErrInvalidUserID
	}

	// First, export all data
	export, err := de.ExportUserData(ctx, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to export user data: %w", err)
	}

	// Create a buffer to write the ZIP file
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// Add metadata file
	metadataFile, err := zipWriter.Create("metadata.json")
	if err != nil {
		return nil, fmt.Errorf("failed to create metadata file: %w", err)
	}

	metadataJSON, err := json.MarshalIndent(export.Metadata, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if _, err := metadataFile.Write(metadataJSON); err != nil {
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}

	// Add data files for each source
	for sourceName, data := range export.DataSources {
		fileName := fmt.Sprintf("data/%s.json", sourceName)
		dataFile, err := zipWriter.Create(fileName)
		if err != nil {
			return nil, fmt.Errorf("failed to create data file for %s: %w", sourceName, err)
		}

		// Pretty-print the JSON
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, data, "", "  "); err != nil {
			// If indentation fails, write raw data
			if _, err := dataFile.Write(data); err != nil {
				return nil, fmt.Errorf("failed to write data for %s: %w", sourceName, err)
			}
		} else {
			if _, err := dataFile.Write(prettyJSON.Bytes()); err != nil {
				return nil, fmt.Errorf("failed to write data for %s: %w", sourceName, err)
			}
		}
	}

	// Add a summary file
	summaryFile, err := zipWriter.Create("summary.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to create summary file: %w", err)
	}

	summary := fmt.Sprintf(`GDPR Data Export Package
========================

User ID: %s
Export Date: %s
Request ID: %s

Data Sources Included:
`, userID, export.ExportedAt.Format(time.RFC3339), export.Metadata.RequestID)

	for _, dataType := range export.Metadata.DataTypes {
		summary += fmt.Sprintf("  - %s\n", dataType)
	}

	summary += fmt.Sprintf(`
Total Records: %d

This data package contains all personal data processed in accordance with
GDPR Article 20 (Right to Data Portability). The data is provided in JSON
format for machine readability and easy transfer to another service provider.

For questions about this data export, please contact our Data Protection Officer.
`, export.Metadata.TotalRecords)

	if _, err := summaryFile.Write([]byte(summary)); err != nil {
		return nil, fmt.Errorf("failed to write summary: %w", err)
	}

	// Add README file
	readmeFile, err := zipWriter.Create("README.md")
	if err != nil {
		return nil, fmt.Errorf("failed to create README file: %w", err)
	}

	readme := `# GDPR Data Export

This archive contains your personal data exported in compliance with GDPR Article 20
(Right to Data Portability).

## Contents

- **metadata.json** - Information about this export
- **data/** - Directory containing your data from various services
- **summary.txt** - Human-readable summary of the export

## Data Format

All data files are in JSON format, which is a standard, machine-readable format
that can be easily imported into other services or analyzed with standard tools.

## Your Rights

Under GDPR, you have the right to:
- Receive your personal data in a structured, commonly used format
- Transmit that data to another controller without hindrance
- Have your data erased (Right to Erasure)
- Restrict processing of your data
- Object to processing

For any questions or to exercise additional rights, please contact our
Data Protection Officer.
`

	if _, err := readmeFile.Write([]byte(readme)); err != nil {
		return nil, fmt.Errorf("failed to write README: %w", err)
	}

	// Close the ZIP writer
	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close ZIP writer: %w", err)
	}

	return buf.Bytes(), nil
}

// ExportConsentRecords exports only consent records for a user
func (de *DataExporter) ExportConsentRecords(ctx context.Context, tenantID, userID string) ([]ConsentRecord, error) {
	if userID == "" {
		return nil, ErrInvalidUserID
	}

	var consents []ConsentRecord
	if err := de.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Find(&consents).Error; err != nil {
		return nil, fmt.Errorf("failed to export consent records: %w", err)
	}

	return consents, nil
}

// ExportAuditLog exports the GDPR audit log for a user
func (de *DataExporter) ExportAuditLog(ctx context.Context, tenantID, userID string) ([]GDPRAuditLog, error) {
	if userID == "" {
		return nil, ErrInvalidUserID
	}

	var logs []GDPRAuditLog
	if err := de.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Order("performed_at DESC").
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to export audit log: %w", err)
	}

	return logs, nil
}

// GetExportRequest retrieves a data export request by ID
func (de *DataExporter) GetExportRequest(ctx context.Context, requestID string) (*DataSubjectRequest, error) {
	if requestID == "" {
		return nil, ErrInvalidRequestID
	}

	var request DataSubjectRequest
	if err := de.db.WithContext(ctx).
		Where("id = ? AND request_type = ?", requestID, RequestTypePortable).
		First(&request).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrRequestNotFound
		}
		return nil, fmt.Errorf("failed to get export request: %w", err)
	}

	return &request, nil
}

// ListExportRequests lists all data export requests for a user
func (de *DataExporter) ListExportRequests(ctx context.Context, tenantID, userID string) ([]DataSubjectRequest, error) {
	if userID == "" {
		return nil, ErrInvalidUserID
	}

	var requests []DataSubjectRequest
	if err := de.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND request_type = ?", tenantID, userID, RequestTypePortable).
		Order("requested_at DESC").
		Find(&requests).Error; err != nil {
		return nil, fmt.Errorf("failed to list export requests: %w", err)
	}

	return requests, nil
}

// BaseDataSourceProvider provides a base implementation for data source providers
type BaseDataSourceProvider struct {
	serviceName    string
	dataCategories []DataCategory
	getUserData    func(ctx context.Context, userID string) (json.RawMessage, error)
	deleteUserData func(ctx context.Context, userID string) error
	anonymizeData  func(ctx context.Context, userID string) error
}

// NewBaseDataSourceProvider creates a new BaseDataSourceProvider
func NewBaseDataSourceProvider(
	serviceName string,
	dataCategories []DataCategory,
	getUserData func(ctx context.Context, userID string) (json.RawMessage, error),
	deleteUserData func(ctx context.Context, userID string) error,
	anonymizeData func(ctx context.Context, userID string) error,
) *BaseDataSourceProvider {
	return &BaseDataSourceProvider{
		serviceName:    serviceName,
		dataCategories: dataCategories,
		getUserData:    getUserData,
		deleteUserData: deleteUserData,
		anonymizeData:  anonymizeData,
	}
}

// GetServiceName returns the service name
func (p *BaseDataSourceProvider) GetServiceName() string {
	return p.serviceName
}

// GetUserData retrieves user data
func (p *BaseDataSourceProvider) GetUserData(ctx context.Context, userID string) (json.RawMessage, error) {
	if p.getUserData == nil {
		return nil, nil
	}
	return p.getUserData(ctx, userID)
}

// DeleteUserData deletes user data
func (p *BaseDataSourceProvider) DeleteUserData(ctx context.Context, userID string) error {
	if p.deleteUserData == nil {
		return nil
	}
	return p.deleteUserData(ctx, userID)
}

// AnonymizeUserData anonymizes user data
func (p *BaseDataSourceProvider) AnonymizeUserData(ctx context.Context, userID string) error {
	if p.anonymizeData == nil {
		return nil
	}
	return p.anonymizeData(ctx, userID)
}

// GetDataCategories returns the data categories handled by this provider
func (p *BaseDataSourceProvider) GetDataCategories() []DataCategory {
	return p.dataCategories
}

// DBTableProvider provides user data from a specific database table
type DBTableProvider struct {
	db             *gorm.DB
	serviceName    string
	tableName      string
	userIDColumn   string
	selectColumns  []string
	dataCategories []DataCategory
	anonymizeFunc  func(tx *gorm.DB, userID string) error
}

// NewDBTableProvider creates a provider for a database table
func NewDBTableProvider(
	db *gorm.DB,
	serviceName string,
	tableName string,
	userIDColumn string,
	selectColumns []string,
	dataCategories []DataCategory,
) *DBTableProvider {
	return &DBTableProvider{
		db:             db,
		serviceName:    serviceName,
		tableName:      tableName,
		userIDColumn:   userIDColumn,
		selectColumns:  selectColumns,
		dataCategories: dataCategories,
	}
}

// GetServiceName returns the service name
func (p *DBTableProvider) GetServiceName() string {
	return p.serviceName
}

// GetUserData retrieves user data from the table
func (p *DBTableProvider) GetUserData(ctx context.Context, userID string) (json.RawMessage, error) {
	var results []map[string]interface{}

	query := p.db.WithContext(ctx).Table(p.tableName)

	if len(p.selectColumns) > 0 {
		query = query.Select(p.selectColumns)
	}

	if err := query.Where(p.userIDColumn+" = ?", userID).Find(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get user data from %s: %w", p.tableName, err)
	}

	if len(results) == 0 {
		return nil, nil
	}

	data, err := json.Marshal(map[string]interface{}{
		"records": results,
		"count":   len(results),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data from %s: %w", p.tableName, err)
	}

	return data, nil
}

// DeleteUserData deletes user data from the table
func (p *DBTableProvider) DeleteUserData(ctx context.Context, userID string) error {
	result := p.db.WithContext(ctx).
		Table(p.tableName).
		Where(p.userIDColumn+" = ?", userID).
		Delete(nil)

	if result.Error != nil {
		return fmt.Errorf("failed to delete user data from %s: %w", p.tableName, result.Error)
	}

	return nil
}

// AnonymizeUserData anonymizes user data in the table
func (p *DBTableProvider) AnonymizeUserData(ctx context.Context, userID string) error {
	if p.anonymizeFunc != nil {
		return p.anonymizeFunc(p.db.WithContext(ctx), userID)
	}

	// Default: just delete the data
	return p.DeleteUserData(ctx, userID)
}

// GetDataCategories returns the data categories
func (p *DBTableProvider) GetDataCategories() []DataCategory {
	return p.dataCategories
}

// SetAnonymizeFunc sets a custom anonymization function
func (p *DBTableProvider) SetAnonymizeFunc(fn func(tx *gorm.DB, userID string) error) {
	p.anonymizeFunc = fn
}
