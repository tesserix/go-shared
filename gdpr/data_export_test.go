package gdpr

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestBaseDataSourceProvider_GetServiceName verifies that the service name is
// returned correctly.
func TestBaseDataSourceProvider_GetServiceName(t *testing.T) {
	p := NewBaseDataSourceProvider("my-service", nil, nil, nil, nil)
	assert.Equal(t, "my-service", p.GetServiceName())
}

// TestBaseDataSourceProvider_GetDataCategories verifies the categories list is
// returned correctly.
func TestBaseDataSourceProvider_GetDataCategories(t *testing.T) {
	categories := []DataCategory{DataCategoryIdentity, DataCategoryMarketing}
	p := NewBaseDataSourceProvider("svc", categories, nil, nil, nil)
	assert.Equal(t, categories, p.GetDataCategories())
}

// TestBaseDataSourceProvider_GetUserData_NilHandler verifies that when no
// getUserData handler is provided the method returns nil without error.
func TestBaseDataSourceProvider_GetUserData_NilHandler(t *testing.T) {
	p := NewBaseDataSourceProvider("svc", nil, nil, nil, nil)
	data, err := p.GetUserData(context.Background(), "user-1")
	assert.NoError(t, err)
	assert.Nil(t, data)
}

// TestBaseDataSourceProvider_GetUserData_WithHandler verifies that the provided
// handler is invoked and its result is returned.
func TestBaseDataSourceProvider_GetUserData_WithHandler(t *testing.T) {
	want := json.RawMessage(`{"orders":[{"id":"1"}]}`)

	p := NewBaseDataSourceProvider("order-service", nil,
		func(_ context.Context, userID string) (json.RawMessage, error) {
			assert.Equal(t, "user-abc", userID)
			return want, nil
		},
		nil, nil,
	)

	got, err := p.GetUserData(context.Background(), "user-abc")
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// TestBaseDataSourceProvider_DeleteUserData_NilHandler verifies no-op when no
// delete handler is configured.
func TestBaseDataSourceProvider_DeleteUserData_NilHandler(t *testing.T) {
	p := NewBaseDataSourceProvider("svc", nil, nil, nil, nil)
	err := p.DeleteUserData(context.Background(), "user-1")
	assert.NoError(t, err)
}

// TestBaseDataSourceProvider_DeleteUserData_WithHandler verifies the delete
// handler is called with the correct user ID.
func TestBaseDataSourceProvider_DeleteUserData_WithHandler(t *testing.T) {
	deleted := false
	p := NewBaseDataSourceProvider("svc", nil, nil,
		func(_ context.Context, userID string) error {
			assert.Equal(t, "user-42", userID)
			deleted = true
			return nil
		},
		nil,
	)

	err := p.DeleteUserData(context.Background(), "user-42")
	require.NoError(t, err)
	assert.True(t, deleted)
}

// TestBaseDataSourceProvider_AnonymizeUserData_NilHandler verifies no-op when
// no anonymize handler is configured.
func TestBaseDataSourceProvider_AnonymizeUserData_NilHandler(t *testing.T) {
	p := NewBaseDataSourceProvider("svc", nil, nil, nil, nil)
	err := p.AnonymizeUserData(context.Background(), "user-1")
	assert.NoError(t, err)
}

// TestBaseDataSourceProvider_AnonymizeUserData_WithHandler verifies the
// anonymize handler is invoked correctly.
func TestBaseDataSourceProvider_AnonymizeUserData_WithHandler(t *testing.T) {
	anonymized := false
	p := NewBaseDataSourceProvider("svc", nil, nil, nil,
		func(_ context.Context, userID string) error {
			assert.Equal(t, "user-7", userID)
			anonymized = true
			return nil
		},
	)

	err := p.AnonymizeUserData(context.Background(), "user-7")
	require.NoError(t, err)
	assert.True(t, anonymized)
}

// TestNewBaseDataSourceProvider_Construction verifies all constructor arguments
// are stored and accessible.
func TestNewBaseDataSourceProvider_Construction(t *testing.T) {
	categories := []DataCategory{DataCategoryFinancial, DataCategoryTransactional}
	p := NewBaseDataSourceProvider("payment-service", categories, nil, nil, nil)

	require.NotNil(t, p)
	assert.Equal(t, "payment-service", p.GetServiceName())
	assert.Len(t, p.GetDataCategories(), 2)
}

// TestDBTableProvider_GetServiceName verifies the service name accessor.
func TestDBTableProvider_GetServiceName(t *testing.T) {
	p := NewDBTableProvider(nil, "catalog-service", "products", "user_id", nil, nil)
	assert.Equal(t, "catalog-service", p.GetServiceName())
}

// TestDBTableProvider_GetDataCategories verifies the categories accessor.
func TestDBTableProvider_GetDataCategories(t *testing.T) {
	categories := []DataCategory{DataCategoryTransactional}
	p := NewDBTableProvider(nil, "svc", "orders", "customer_id", nil, categories)
	assert.Equal(t, categories, p.GetDataCategories())
}

// TestDBTableProvider_SetAnonymizeFunc verifies that SetAnonymizeFunc replaces
// the anonymization function.
func TestDBTableProvider_SetAnonymizeFunc(t *testing.T) {
	p := NewDBTableProvider(nil, "svc", "users", "id", nil, nil)
	assert.Nil(t, p.anonymizeFunc)

	p.SetAnonymizeFunc(func(_ *gorm.DB, _ string) error {
		return nil
	})
	// After setting, the field should be non-nil (same package, direct access).
	assert.NotNil(t, p.anonymizeFunc)
}

// TestDataExporter_Construction verifies NewDataExporter sets all fields.
func TestDataExporter_Construction(t *testing.T) {
	provider := NewBaseDataSourceProvider("svc", nil, nil, nil, nil)
	de := NewDataExporter(nil, "export-service", []DataSourceProvider{provider})

	require.NotNil(t, de)
	assert.Equal(t, "export-service", de.serviceName)
	assert.Len(t, de.dataProviders, 1)
	assert.NotNil(t, de.auditLogger)
}

// TestDataExporter_AddDataProvider verifies that AddDataProvider appends to the
// provider list.
func TestDataExporter_AddDataProvider(t *testing.T) {
	de := NewDataExporter(nil, "svc", nil)
	require.Len(t, de.dataProviders, 0)

	p := NewBaseDataSourceProvider("p1", nil, nil, nil, nil)
	de.AddDataProvider(p)
	assert.Len(t, de.dataProviders, 1)

	q := NewBaseDataSourceProvider("p2", nil, nil, nil, nil)
	de.AddDataProvider(q)
	assert.Len(t, de.dataProviders, 2)
}

// TestExportMetadata_StructFields verifies that ExportMetadata can be fully
// populated and its fields read back.
func TestExportMetadata_StructFields(t *testing.T) {
	now := time.Now().UTC()
	meta := ExportMetadata{
		Format:       "JSON",
		Version:      "1.0",
		GeneratedAt:  now,
		RequestID:    "req-123",
		TotalRecords: 500,
		DataTypes:    []string{"orders", "profiles", "gdpr_compliance"},
	}

	assert.Equal(t, "JSON", meta.Format)
	assert.Equal(t, "1.0", meta.Version)
	assert.Equal(t, "req-123", meta.RequestID)
	assert.Equal(t, 500, meta.TotalRecords)
	assert.Len(t, meta.DataTypes, 3)
	assert.Contains(t, meta.DataTypes, "orders")
}

// TestDataExport_StructFields verifies DataExport struct field access.
func TestDataExport_StructFields(t *testing.T) {
	now := time.Now().UTC()
	rawOrders := json.RawMessage(`{"records":[{"id":"1"}],"count":1}`)

	export := DataExport{
		UserID:     "user-001",
		ExportedAt: now,
		DataSources: map[string]json.RawMessage{
			"order-service": rawOrders,
		},
		Metadata: ExportMetadata{
			Format:       "JSON",
			Version:      "1.0",
			GeneratedAt:  now,
			TotalRecords: 1,
			DataTypes:    []string{"order-service"},
		},
	}

	assert.Equal(t, "user-001", export.UserID)
	assert.Equal(t, rawOrders, export.DataSources["order-service"])
	assert.Equal(t, "JSON", export.Metadata.Format)
	assert.Equal(t, 1, export.Metadata.TotalRecords)
}

// TestDataSubjectRequest_RequestTypes verifies that a DataSubjectRequest can be
// created for each GDPR article request type.
func TestDataSubjectRequest_RequestTypes(t *testing.T) {
	requestTypes := []RequestType{
		RequestTypeAccess,
		RequestTypeRectify,
		RequestTypeErasure,
		RequestTypeRestrict,
		RequestTypePortable,
		RequestTypeObject,
		RequestTypeAutomated,
	}

	for _, rt := range requestTypes {
		t.Run(string(rt), func(t *testing.T) {
			req := DataSubjectRequest{
				ID:          "req-001",
				TenantID:    "tenant-1",
				UserID:      "user-1",
				RequestType: rt,
				Status:      RequestStatusPending,
			}
			assert.Equal(t, rt, req.RequestType)
			assert.Equal(t, RequestStatusPending, req.Status)
		})
	}
}

// TestDataSubjectRequest_AllStatuses verifies that every RequestStatus can be
// assigned to a DataSubjectRequest.
func TestDataSubjectRequest_AllStatuses(t *testing.T) {
	statuses := []RequestStatus{
		RequestStatusPending,
		RequestStatusProcessing,
		RequestStatusCompleted,
		RequestStatusRejected,
		RequestStatusFailed,
		RequestStatusCancelled,
	}

	for _, s := range statuses {
		t.Run(string(s), func(t *testing.T) {
			req := DataSubjectRequest{Status: s}
			assert.Equal(t, s, req.Status)
		})
	}
}

// TestRetainedDataSource_StructFields verifies the RetainedDataSource struct.
func TestRetainedDataSource_StructFields(t *testing.T) {
	rds := RetainedDataSource{
		Source:      "gdpr_audit_logs",
		Reason:      "Legal compliance requirement",
		LegalBasis:  "Regulatory obligation",
		RetainUntil: "7 years from creation",
	}

	assert.Equal(t, "gdpr_audit_logs", rds.Source)
	assert.Equal(t, "Legal compliance requirement", rds.Reason)
	assert.Equal(t, "Regulatory obligation", rds.LegalBasis)
	assert.Equal(t, "7 years from creation", rds.RetainUntil)
}

// TestDBTableProvider_Constructor verifies NewDBTableProvider initialises all
// fields correctly.
func TestDBTableProvider_Constructor(t *testing.T) {
	cols := []string{"id", "email", "created_at"}
	cats := []DataCategory{DataCategoryIdentity, DataCategoryContact}

	p := NewDBTableProvider(nil, "user-service", "users", "id", cols, cats)

	require.NotNil(t, p)
	assert.Equal(t, "user-service", p.serviceName)
	assert.Equal(t, "users", p.tableName)
	assert.Equal(t, "id", p.userIDColumn)
	assert.Equal(t, cols, p.selectColumns)
	assert.Equal(t, cats, p.dataCategories)
	assert.Nil(t, p.anonymizeFunc)
}
