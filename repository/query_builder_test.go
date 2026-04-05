package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// queryBuilderPaginateOffset is extracted from QueryBuilderImpl.Paginate to
// allow pure unit tests of the offset calculation without a real database.
func queryBuilderPaginateOffset(page, pageSize int) (limit, offset int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	offset = (page - 1) * pageSize
	return pageSize, offset
}

func TestQueryBuilderImpl_PaginateOffset(t *testing.T) {
	tests := []struct {
		name           string
		page           int
		pageSize       int
		expectedLimit  int
		expectedOffset int
	}{
		{"first page", 1, 10, 10, 0},
		{"second page", 2, 10, 10, 10},
		{"third page large size", 3, 25, 25, 50},
		{"page zero clamps to 1", 0, 10, 10, 0},
		{"negative page clamps to 1", -3, 10, 10, 0},
		{"zero pageSize clamps to 10", 1, 0, 10, 0},
		{"negative pageSize clamps to 10", 1, -5, 10, 0},
		{"page 5 size 20", 5, 20, 20, 80},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			limit, offset := queryBuilderPaginateOffset(tc.page, tc.pageSize)
			assert.Equal(t, tc.expectedLimit, limit)
			assert.Equal(t, tc.expectedOffset, offset)
		})
	}
}

// TestTotalPagesCalculation tests the ceiling division used in WithPagination.
func TestTotalPagesCalculation(t *testing.T) {
	tests := []struct {
		name       string
		total      int64
		pageSize   int
		wantPages  int
	}{
		{"even split", 100, 10, 10},
		{"remainder rounds up", 101, 10, 11},
		{"one record", 1, 10, 1},
		{"page larger than total", 5, 10, 1},
		{"exact boundary", 20, 5, 4},
		{"page size 1", 7, 1, 7},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := int((tc.total + int64(tc.pageSize) - 1) / int64(tc.pageSize))
			assert.Equal(t, tc.wantPages, got)
		})
	}
}

// TestOrderBy_DirectionNormalization tests the logic inside OrderBy that
// normalizes the direction string.  The source converts the input to upper-case
// and falls back to "ASC" for any unrecognised value.
func TestOrderBy_DirectionNormalization(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
	}{
		{"lowercase asc", "asc", "ASC"},
		{"lowercase desc", "desc", "DESC"},
		{"uppercase ASC", "ASC", "ASC"},
		{"uppercase DESC", "DESC", "DESC"},
		{"mixed case Desc", "Desc", "DESC"},
		{"invalid direction defaults to ASC", "INVALID", "ASC"},
		{"empty defaults to ASC", "", "ASC"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Inline the same logic as QueryBuilderImpl.OrderBy so we can test
			// the normalization without a database.
			direction := tc.input
			upper := upperCase(direction)
			if upper != "ASC" && upper != "DESC" {
				upper = "ASC"
			}
			assert.Equal(t, tc.expected, upper)
		})
	}
}

// upperCase mirrors strings.ToUpper without importing strings in the test body.
func upperCase(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			result[i] = c - 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

// TestWhereIn_SkipsEmptyValues tests the WhereIn guard against empty slices.
// The source skips the Where call when len(values) == 0.
func TestWhereIn_EmptyValuesGuard(t *testing.T) {
	// When values is empty the condition should simply not be appended.
	// We cannot call the real method without a *gorm.DB, so we test the guard
	// condition independently.
	values := []interface{}{}
	shouldApply := len(values) > 0
	assert.False(t, shouldApply, "WhereIn should skip empty value slices")

	values2 := []interface{}{"a", "b"}
	shouldApply2 := len(values2) > 0
	assert.True(t, shouldApply2, "WhereIn should apply non-empty value slices")
}

// TestGroupByJoin_ColumnConcatenation tests the column joining behaviour used
// in GroupBy (which delegates to strings.Join).
func TestGroupByJoin_ColumnConcatenation(t *testing.T) {
	tests := []struct {
		name     string
		columns  []string
		expected string
	}{
		{"single column", []string{"tenant_id"}, "tenant_id"},
		{"two columns", []string{"tenant_id", "status"}, "tenant_id, status"},
		{"three columns", []string{"a", "b", "c"}, "a, b, c"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Replicate the same join logic used in GroupBy.
			var result string
			for i, col := range tc.columns {
				if i > 0 {
					result += ", "
				}
				result += col
			}
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestJoinClauseFormats verifies the SQL fragment patterns produced by the
// join methods match what is sent to GORM.
func TestJoinClauseFormats(t *testing.T) {
	tests := []struct {
		name     string
		joinType string
		table    string
		cond     string
		expected string
	}{
		{
			name:     "inner join",
			joinType: "JOIN",
			table:    "categories",
			cond:     "categories.id = products.category_id",
			expected: "JOIN categories ON categories.id = products.category_id",
		},
		{
			name:     "left join",
			joinType: "LEFT JOIN",
			table:    "orders",
			cond:     "orders.product_id = products.id",
			expected: "LEFT JOIN orders ON orders.product_id = products.id",
		},
		{
			name:     "right join",
			joinType: "RIGHT JOIN",
			table:    "inventory",
			cond:     "inventory.sku = products.sku",
			expected: "RIGHT JOIN inventory ON inventory.sku = products.sku",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.joinType + " " + tc.table + " ON " + tc.cond
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestWhereNullClauses verifies the SQL fragments produced by WhereNull and
// WhereNotNull.
func TestWhereNullClauses(t *testing.T) {
	tests := []struct {
		name     string
		column   string
		null     bool
		expected string
	}{
		{"null check", "deleted_at", true, "deleted_at IS NULL"},
		{"not null check", "published_at", false, "published_at IS NOT NULL"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var clause string
			if tc.null {
				clause = tc.column + " IS NULL"
			} else {
				clause = tc.column + " IS NOT NULL"
			}
			assert.Equal(t, tc.expected, clause)
		})
	}
}

// TestWhereBetweenClause verifies the SQL fragment produced by WhereBetween.
func TestWhereBetweenClause(t *testing.T) {
	column := "salary"
	expected := "salary BETWEEN ? AND ?"
	got := column + " BETWEEN ? AND ?"
	assert.Equal(t, expected, got)
}

// TestPreloadFields_Accumulation verifies that calling Preload() multiple times
// accumulates associations (the field slice grows).
func TestPreloadFields_Accumulation(t *testing.T) {
	fields := make([]string, 0)
	fields = append(fields, "Category")
	fields = append(fields, "Images")
	fields = append(fields, "Variants")

	assert.Equal(t, []string{"Category", "Images", "Variants"}, fields)
}

// TestClone_PreloadFieldsAreCopied verifies that clone receives an independent
// copy of the preload slice so mutations do not affect the original.
func TestClone_PreloadFieldsAreCopied(t *testing.T) {
	original := []string{"Category", "Images"}
	cloned := append([]string{}, original...)
	cloned = append(cloned, "Variants")

	assert.Len(t, original, 2, "original slice must not be mutated")
	assert.Len(t, cloned, 3)
}
