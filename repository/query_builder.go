package repository

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// QueryBuilderImpl implements the QueryBuilder interface
type QueryBuilderImpl[T Entity] struct {
	db           *gorm.DB
	query        *gorm.DB
	entityType   T
	preloadFields []string
}

// NewQueryBuilder creates a new query builder instance
func NewQueryBuilder[T Entity](db *gorm.DB) *QueryBuilderImpl[T] {
	var entity T
	return &QueryBuilderImpl[T]{
		db:    db,
		query: db.Model(&entity),
		entityType: entity,
		preloadFields: make([]string, 0),
	}
}

// Where adds a WHERE condition
func (qb *QueryBuilderImpl[T]) Where(condition string, args ...interface{}) QueryBuilder[T] {
	qb.query = qb.query.Where(condition, args...)
	return qb
}

// WhereIn adds a WHERE IN condition
func (qb *QueryBuilderImpl[T]) WhereIn(column string, values []interface{}) QueryBuilder[T] {
	if len(values) > 0 {
		qb.query = qb.query.Where(fmt.Sprintf("%s IN ?", column), values)
	}
	return qb
}

// WhereBetween adds a WHERE BETWEEN condition
func (qb *QueryBuilderImpl[T]) WhereBetween(column string, start, end interface{}) QueryBuilder[T] {
	qb.query = qb.query.Where(fmt.Sprintf("%s BETWEEN ? AND ?", column), start, end)
	return qb
}

// WhereNull adds a WHERE IS NULL condition
func (qb *QueryBuilderImpl[T]) WhereNull(column string) QueryBuilder[T] {
	qb.query = qb.query.Where(fmt.Sprintf("%s IS NULL", column))
	return qb
}

// WhereNotNull adds a WHERE IS NOT NULL condition
func (qb *QueryBuilderImpl[T]) WhereNotNull(column string) QueryBuilder[T] {
	qb.query = qb.query.Where(fmt.Sprintf("%s IS NOT NULL", column))
	return qb
}

// Join adds an INNER JOIN
func (qb *QueryBuilderImpl[T]) Join(table, condition string) QueryBuilder[T] {
	qb.query = qb.query.Joins(fmt.Sprintf("JOIN %s ON %s", table, condition))
	return qb
}

// LeftJoin adds a LEFT JOIN
func (qb *QueryBuilderImpl[T]) LeftJoin(table, condition string) QueryBuilder[T] {
	qb.query = qb.query.Joins(fmt.Sprintf("LEFT JOIN %s ON %s", table, condition))
	return qb
}

// RightJoin adds a RIGHT JOIN
func (qb *QueryBuilderImpl[T]) RightJoin(table, condition string) QueryBuilder[T] {
	qb.query = qb.query.Joins(fmt.Sprintf("RIGHT JOIN %s ON %s", table, condition))
	return qb
}

// OrderBy adds an ORDER BY clause
func (qb *QueryBuilderImpl[T]) OrderBy(column, direction string) QueryBuilder[T] {
	direction = strings.ToUpper(direction)
	if direction != "ASC" && direction != "DESC" {
		direction = "ASC"
	}
	qb.query = qb.query.Order(fmt.Sprintf("%s %s", column, direction))
	return qb
}

// OrderByRaw adds a raw ORDER BY clause
func (qb *QueryBuilderImpl[T]) OrderByRaw(sql string) QueryBuilder[T] {
	qb.query = qb.query.Order(sql)
	return qb
}

// GroupBy adds a GROUP BY clause
func (qb *QueryBuilderImpl[T]) GroupBy(columns ...string) QueryBuilder[T] {
	qb.query = qb.query.Group(strings.Join(columns, ", "))
	return qb
}

// Having adds a HAVING clause
func (qb *QueryBuilderImpl[T]) Having(condition string, args ...interface{}) QueryBuilder[T] {
	qb.query = qb.query.Having(condition, args...)
	return qb
}

// Limit adds a LIMIT clause
func (qb *QueryBuilderImpl[T]) Limit(limit int) QueryBuilder[T] {
	qb.query = qb.query.Limit(limit)
	return qb
}

// Offset adds an OFFSET clause
func (qb *QueryBuilderImpl[T]) Offset(offset int) QueryBuilder[T] {
	qb.query = qb.query.Offset(offset)
	return qb
}

// Preload adds preload associations
func (qb *QueryBuilderImpl[T]) Preload(associations ...string) QueryBuilder[T] {
	qb.preloadFields = append(qb.preloadFields, associations...)
	for _, association := range associations {
		qb.query = qb.query.Preload(association)
	}
	return qb
}

// Find executes the query and returns all matching records
func (qb *QueryBuilderImpl[T]) Find() ([]T, error) {
	var entities []T
	if err := qb.query.Find(&entities).Error; err != nil {
		return nil, fmt.Errorf("failed to execute find query: %w", err)
	}
	return entities, nil
}

// First executes the query and returns the first matching record
func (qb *QueryBuilderImpl[T]) First() (T, error) {
	var entity T
	if err := qb.query.First(&entity).Error; err != nil {
		var zero T
		if err == gorm.ErrRecordNotFound {
			return zero, fmt.Errorf("no record found")
		}
		return zero, fmt.Errorf("failed to execute first query: %w", err)
	}
	return entity, nil
}

// Count executes a count query
func (qb *QueryBuilderImpl[T]) Count() (int64, error) {
	var count int64
	if err := qb.query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to execute count query: %w", err)
	}
	return count, nil
}

// Exists checks if any records match the query
func (qb *QueryBuilderImpl[T]) Exists() (bool, error) {
	count, err := qb.Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Raw executes a raw SQL query
func (qb *QueryBuilderImpl[T]) Raw(sql string, args ...interface{}) QueryBuilder[T] {
	qb.query = qb.db.Raw(sql, args...)
	return qb
}

// Debug enables debug mode for the query
func (qb *QueryBuilderImpl[T]) Debug() QueryBuilder[T] {
	qb.query = qb.query.Debug()
	return qb
}

// Advanced query methods

// Distinct adds DISTINCT to the query
func (qb *QueryBuilderImpl[T]) Distinct(columns ...string) QueryBuilder[T] {
	if len(columns) > 0 {
		qb.query = qb.query.Distinct(columns)
	} else {
		qb.query = qb.query.Distinct()
	}
	return qb
}

// Select specifies which columns to select
func (qb *QueryBuilderImpl[T]) Select(columns ...string) QueryBuilder[T] {
	qb.query = qb.query.Select(columns)
	return qb
}

// Omit specifies which columns to omit
func (qb *QueryBuilderImpl[T]) Omit(columns ...string) QueryBuilder[T] {
	qb.query = qb.query.Omit(columns...)
	return qb
}

// Scopes applies scopes to the query
func (qb *QueryBuilderImpl[T]) Scopes(funcs ...func(*gorm.DB) *gorm.DB) QueryBuilder[T] {
	qb.query = qb.query.Scopes(funcs...)
	return qb
}

// Unscoped removes the default scope (useful for soft-deleted records)
func (qb *QueryBuilderImpl[T]) Unscoped() QueryBuilder[T] {
	qb.query = qb.query.Unscoped()
	return qb
}

// Pagination methods

// Paginate applies pagination to the query
func (qb *QueryBuilderImpl[T]) Paginate(page, pageSize int) QueryBuilder[T] {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	
	offset := (page - 1) * pageSize
	qb.query = qb.query.Limit(pageSize).Offset(offset)
	return qb
}

// WithPagination executes the query with pagination
func (qb *QueryBuilderImpl[T]) WithPagination(page, pageSize int) (*PaginatedResult[T], error) {
	// Count total records first
	var total int64
	countQuery := qb.db.Model(&qb.entityType)
	
	// Apply all conditions from the current query to the count query
	// This is a simplified approach - in a real implementation, you'd want to
	// copy all WHERE conditions but exclude ORDER BY, LIMIT, OFFSET
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count records: %w", err)
	}
	
	// Apply pagination
	qb.Paginate(page, pageSize)
	
	// Execute query
	entities, err := qb.Find()
	if err != nil {
		return nil, err
	}
	
	// Calculate pagination info
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	
	return &PaginatedResult[T]{
		Data:       entities,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}, nil
}

// Aggregation methods

// Sum calculates the sum of a column
func (qb *QueryBuilderImpl[T]) Sum(column string) (float64, error) {
	var result float64
	row := qb.query.Select(fmt.Sprintf("SUM(%s)", column)).Row()
	if err := row.Scan(&result); err != nil {
		return 0, fmt.Errorf("failed to calculate sum: %w", err)
	}
	return result, nil
}

// Avg calculates the average of a column
func (qb *QueryBuilderImpl[T]) Avg(column string) (float64, error) {
	var result float64
	row := qb.query.Select(fmt.Sprintf("AVG(%s)", column)).Row()
	if err := row.Scan(&result); err != nil {
		return 0, fmt.Errorf("failed to calculate average: %w", err)
	}
	return result, nil
}

// Max finds the maximum value of a column
func (qb *QueryBuilderImpl[T]) Max(column string) (interface{}, error) {
	var result interface{}
	row := qb.query.Select(fmt.Sprintf("MAX(%s)", column)).Row()
	if err := row.Scan(&result); err != nil {
		return nil, fmt.Errorf("failed to find maximum: %w", err)
	}
	return result, nil
}

// Min finds the minimum value of a column
func (qb *QueryBuilderImpl[T]) Min(column string) (interface{}, error) {
	var result interface{}
	row := qb.query.Select(fmt.Sprintf("MIN(%s)", column)).Row()
	if err := row.Scan(&result); err != nil {
		return nil, fmt.Errorf("failed to find minimum: %w", err)
	}
	return result, nil
}

// Batch operations

// FindInBatches processes records in batches
func (qb *QueryBuilderImpl[T]) FindInBatches(batchSize int, fn func([]T) error) error {
	var entities []T
	result := qb.query.FindInBatches(&entities, batchSize, func(tx *gorm.DB, batch int) error {
		return fn(entities)
	})
	return result.Error
}

// Clone creates a copy of the query builder
func (qb *QueryBuilderImpl[T]) Clone() QueryBuilder[T] {
	return &QueryBuilderImpl[T]{
		db:            qb.db,
		query:         qb.query.Session(&gorm.Session{}),
		entityType:    qb.entityType,
		preloadFields: append([]string{}, qb.preloadFields...),
	}
}

// ToSQL returns the SQL query and arguments
func (qb *QueryBuilderImpl[T]) ToSQL() (string, []interface{}) {
	// Create a dry-run session to get the SQL without executing
	stmt := qb.query.Session(&gorm.Session{DryRun: true}).Statement
	var entities []T
	qb.query.Session(&gorm.Session{DryRun: true}).Find(&entities)
	return stmt.SQL.String(), stmt.Vars
}

// Explain executes EXPLAIN on the query
func (qb *QueryBuilderImpl[T]) Explain() (string, error) {
	sql, args := qb.ToSQL()
	explainSQL := fmt.Sprintf("EXPLAIN %s", sql)
	
	rows, err := qb.db.Raw(explainSQL, args...).Rows()
	if err != nil {
		return "", fmt.Errorf("failed to explain query: %w", err)
	}
	defer rows.Close()
	
	var result strings.Builder
	columns, _ := rows.Columns()
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	
	for rows.Next() {
		for i := range columns {
			valuePtrs[i] = &values[i]
		}
		
		rows.Scan(valuePtrs...)
		
		for i, col := range columns {
			result.WriteString(fmt.Sprintf("%s: %v ", col, values[i]))
		}
		result.WriteString("\n")
	}
	
	return result.String(), nil
}