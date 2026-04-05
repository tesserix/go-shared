//go:build examples
// +build examples

// This file contains example code showing repository usage patterns.
// It is excluded from normal builds due to the build constraint above.
// Run tests with -tags=examples to include this file.

package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Example domain entities

// Staff represents a staff member
type Staff struct {
	TenantEntity
	FirstName    string `json:"first_name" gorm:"type:varchar(100);not null"`
	LastName     string `json:"last_name" gorm:"type:varchar(100);not null"`
	Email        string `json:"email" gorm:"type:varchar(255);uniqueIndex;not null"`
	Phone        string `json:"phone" gorm:"type:varchar(20)"`
	Department   string `json:"department" gorm:"type:varchar(100)"`
	Position     string `json:"position" gorm:"type:varchar(100)"`
	Salary       int64  `json:"salary" gorm:"type:bigint"`
	HireDate     time.Time `json:"hire_date"`
	IsActive     bool   `json:"is_active" gorm:"default:true"`
	
	// Relationships
	Manager   *Staff  `json:"manager,omitempty" gorm:"foreignKey:ManagerID"`
	ManagerID *string `json:"manager_id" gorm:"type:varchar(36)"`
}

// Product represents a product
type Product struct {
	TenantEntity
	Name        string  `json:"name" gorm:"type:varchar(255);not null"`
	Description string  `json:"description" gorm:"type:text"`
	SKU         string  `json:"sku" gorm:"type:varchar(100);uniqueIndex;not null"`
	Price       float64 `json:"price" gorm:"type:decimal(10,2);not null"`
	Stock       int     `json:"stock" gorm:"type:integer;default:0"`
	CategoryID  string  `json:"category_id" gorm:"type:varchar(36)"`
	IsActive    bool    `json:"is_active" gorm:"default:true"`
	
	// Relationships
	Category *Category `json:"category,omitempty" gorm:"foreignKey:CategoryID"`
}

// Category represents a product category
type Category struct {
	TenantEntity
	Name        string     `json:"name" gorm:"type:varchar(255);not null"`
	Description string     `json:"description" gorm:"type:text"`
	ParentID    *string    `json:"parent_id" gorm:"type:varchar(36)"`
	IsActive    bool       `json:"is_active" gorm:"default:true"`
	
	// Relationships
	Parent   *Category  `json:"parent,omitempty" gorm:"foreignKey:ParentID"`
	Children []Category `json:"children,omitempty" gorm:"foreignKey:ParentID"`
	Products []Product  `json:"products,omitempty" gorm:"foreignKey:CategoryID"`
}

// Example service showing repository usage
type StaffService struct {
	staffRepo    TenantRepository[*Staff]
	registry     *RepositoryRegistry
}

// NewStaffService creates a new staff service
func NewStaffService(db *gorm.DB) *StaffService {
	// Create repository registry
	registry := NewRepositoryRegistry(db).
		RegisterTenant[*Staff]("staff", NewRepositoryConfig().
			WithTableName("staff").
			WithSoftDelete(true).
			WithAudit(true).
			WithTenantAware(true).
			Build())

	return &StaffService{
		staffRepo: registry.GetTenant[*Staff]("staff"),
		registry:  registry,
	}
}

// CreateStaff creates a new staff member
func (s *StaffService) CreateStaff(ctx context.Context, tenantID string, staff *Staff) (*Staff, error) {
	// Validate business rules
	if staff.Email == "" {
		return nil, fmt.Errorf("email is required")
	}
	
	// Check if email already exists
	exists, err := s.staffRepo.ExistsForTenant(ctx, tenantID, map[string]interface{}{
		"email": staff.Email,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to check email existence: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("staff with email %s already exists", staff.Email)
	}
	
	// Create staff member
	createdStaff, err := s.staffRepo.CreateForTenant(ctx, tenantID, staff)
	if err != nil {
		return nil, fmt.Errorf("failed to create staff: %w", err)
	}
	
	return &createdStaff, nil
}

// GetStaffList retrieves staff members with pagination and filtering
func (s *StaffService) GetStaffList(ctx context.Context, tenantID string, options QueryOptions) (*PaginatedResult[*Staff], error) {
	// Set preload relationships
	options.Preload = []string{"Manager"}
	
	return s.staffRepo.ListForTenant(ctx, tenantID, options)
}

// SearchStaff searches staff members by name or email
func (s *StaffService) SearchStaff(ctx context.Context, tenantID, query string) ([]*Staff, error) {
	options := QueryOptions{
		Search:       query,
		SearchFields: []string{"first_name", "last_name", "email"},
		Preload:      []string{"Manager"},
	}
	
	return s.staffRepo.FindForTenant(ctx, tenantID, nil)
}

// UpdateStaffSalary updates staff salary with audit trail
func (s *StaffService) UpdateStaffSalary(ctx context.Context, tenantID, staffID string, newSalary int64, userID string) error {
	return s.registry.WithTx(ctx, func(txRegistry *RepositoryRegistry) error {
		staffRepo := txRegistry.GetTenant[*Staff]("staff")
		
		// Get current staff
		staff, err := staffRepo.GetByIDForTenant(ctx, tenantID, staffID)
		if err != nil {
			return err
		}
		
		// Update salary
		oldSalary := staff.Salary
		staff.Salary = newSalary
		
		// Save with audit
		_, err = staffRepo.UpdateForTenant(ctx, tenantID, staff)
		if err != nil {
			return err
		}
		
		// Log business event (this would typically be in a separate audit service)
		fmt.Printf("Salary updated for staff %s: %d -> %d by user %s", 
			staffID, oldSalary, newSalary, userID)
		
		return nil
	})
}

// Example query builder usage
func (s *StaffService) GetStaffByDepartmentAndSalaryRange(ctx context.Context, tenantID, department string, minSalary, maxSalary int64) ([]*Staff, error) {
	qb := NewTenantAwareQueryBuilder[*Staff](s.staffRepo.(*TenantBaseRepository[*Staff]).db, tenantID)
	
	return qb.Where("department = ?", department).
		WhereBetween("salary", minSalary, maxSalary).
		Where("is_active = ?", true).
		OrderBy("salary", "DESC").
		Preload("Manager").
		Find()
}

// Example aggregation usage
func (s *StaffService) GetDepartmentSalaryStats(ctx context.Context, tenantID, department string) (map[string]float64, error) {
	qb := NewTenantAwareQueryBuilder[*Staff](s.staffRepo.(*TenantBaseRepository[*Staff]).db, tenantID)
	
	baseQuery := qb.Where("department = ?", department).Where("is_active = ?", true)
	
	avgSalary, err := baseQuery.Clone().Avg("salary")
	if err != nil {
		return nil, err
	}
	
	maxSalary, err := baseQuery.Clone().Max("salary")
	if err != nil {
		return nil, err
	}
	
	minSalary, err := baseQuery.Clone().Min("salary")
	if err != nil {
		return nil, err
	}
	
	return map[string]float64{
		"average": avgSalary,
		"maximum": maxSalary.(float64),
		"minimum": minSalary.(float64),
	}, nil
}

// Example batch operations
func (s *StaffService) BulkUpdateDepartment(ctx context.Context, tenantID string, staffIDs []string, newDepartment string) error {
	return s.registry.WithTx(ctx, func(txRegistry *RepositoryRegistry) error {
		staffRepo := txRegistry.GetTenant[*Staff]("staff")
		
		// Process in batches to avoid memory issues
		batchSize := 100
		for i := 0; i < len(staffIDs); i += batchSize {
			end := i + batchSize
			if end > len(staffIDs) {
				end = len(staffIDs)
			}
			
			batch := staffIDs[i:end]
			
			// Get staff members in this batch
			options := QueryOptions{
				Filters: map[string]interface{}{
					"id": batch,
				},
			}
			
			staffList, err := staffRepo.FindForTenant(ctx, tenantID, nil)
			if err != nil {
				return err
			}
			
			// Update department
			for _, staff := range staffList {
				staff.Department = newDepartment
			}
			
			// Batch update
			_, err = staffRepo.UpdateBatch(ctx, staffList)
			if err != nil {
				return err
			}
		}
		
		return nil
	})
}

// Example of custom repository methods
type StaffRepository struct {
	*TenantBaseRepository[*Staff]
}

// NewStaffRepository creates a new staff repository with custom methods
func NewStaffRepository(db *gorm.DB) *StaffRepository {
	baseRepo := NewTenantRepository[*Staff](db, "staff")
	return &StaffRepository{
		TenantBaseRepository: baseRepo,
	}
}

// GetStaffHierarchy returns staff hierarchy for a tenant
func (r *StaffRepository) GetStaffHierarchy(ctx context.Context, tenantID string) ([]*Staff, error) {
	// Using raw SQL for complex hierarchical query
	var staff []*Staff
	
	query := `
		WITH RECURSIVE staff_hierarchy AS (
			-- Base case: top-level managers (no manager_id)
			SELECT id, first_name, last_name, email, manager_id, 0 as level
			FROM staff 
			WHERE tenant_id = ? AND manager_id IS NULL AND deleted_at IS NULL
			
			UNION ALL
			
			-- Recursive case: employees with managers
			SELECT s.id, s.first_name, s.last_name, s.email, s.manager_id, sh.level + 1
			FROM staff s
			INNER JOIN staff_hierarchy sh ON s.manager_id = sh.id
			WHERE s.tenant_id = ? AND s.deleted_at IS NULL
		)
		SELECT * FROM staff_hierarchy ORDER BY level, last_name, first_name
	`
	
	if err := r.db.WithContext(ctx).Raw(query, tenantID, tenantID).Scan(&staff).Error; err != nil {
		return nil, fmt.Errorf("failed to get staff hierarchy: %w", err)
	}
	
	return staff, nil
}

// GetActiveStaffCount returns count of active staff by department
func (r *StaffRepository) GetActiveStaffCount(ctx context.Context, tenantID string) (map[string]int64, error) {
	type Result struct {
		Department string `json:"department"`
		Count      int64  `json:"count"`
	}
	
	var results []Result
	
	query := r.db.WithContext(ctx).
		Model(&Staff{}).
		Select("department, COUNT(*) as count").
		Where("tenant_id = ? AND is_active = ? AND deleted_at IS NULL", tenantID, true).
		Group("department")
	
	if err := query.Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get staff count by department: %w", err)
	}
	
	countMap := make(map[string]int64)
	for _, result := range results {
		countMap[result.Department] = result.Count
	}
	
	return countMap, nil
}