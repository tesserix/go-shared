package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tesserix/go-shared/errors"
)

// newTestContext creates a gin.Context backed by an httptest.ResponseRecorder.
// The returned recorder is used to inspect the HTTP response written by the
// response helpers under test.
func newTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// Attach a synthetic request so Gin does not dereference a nil pointer
	// when helpers call methods on c.Request.
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
	return c, w
}

// decodeBody unmarshals the recorder body into a map for field-level assertions.
func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err, "response body must be valid JSON")
	return body
}

// decodeStandardResponse unmarshals the recorder body into a StandardResponse.
func decodeStandardResponse(t *testing.T, w *httptest.ResponseRecorder) StandardResponse {
	t.Helper()
	var resp StandardResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "response body must be valid JSON")
	return resp
}

// ---- SuccessResponse --------------------------------------------------------

func TestSuccessResponse_StatusCode(t *testing.T) {
	c, w := newTestContext()
	SuccessResponse(c, map[string]string{"key": "value"})
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSuccessResponse_SuccessTrue(t *testing.T) {
	c, w := newTestContext()
	SuccessResponse(c, map[string]string{"key": "value"})
	body := decodeBody(t, w)
	assert.Equal(t, true, body["success"])
}

func TestSuccessResponse_DataPresent(t *testing.T) {
	c, w := newTestContext()
	SuccessResponse(c, map[string]string{"msg": "hello"})
	body := decodeBody(t, w)
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok, "data field must be a JSON object")
	assert.Equal(t, "hello", data["msg"])
}

func TestSuccessResponse_ErrorAbsent(t *testing.T) {
	c, w := newTestContext()
	SuccessResponse(c, "payload")
	body := decodeBody(t, w)
	assert.Nil(t, body["error"], "error field must be absent on a success response")
}

func TestSuccessResponse_TimestampPresent(t *testing.T) {
	c, w := newTestContext()
	before := time.Now().UTC().Add(-time.Second)
	SuccessResponse(c, nil)
	after := time.Now().UTC().Add(time.Second)

	body := decodeBody(t, w)
	tsRaw, ok := body["timestamp"]
	require.True(t, ok, "timestamp field must be present")
	tsStr, ok := tsRaw.(string)
	require.True(t, ok, "timestamp must be a string")
	ts, err := time.Parse(time.RFC3339Nano, tsStr)
	require.NoError(t, err, "timestamp must be parseable as RFC3339")
	assert.True(t, ts.After(before) && ts.Before(after), "timestamp must be within the test window")
}

func TestSuccessResponse_NoRequestID_OmittedFromBody(t *testing.T) {
	c, w := newTestContext()
	SuccessResponse(c, nil)
	body := decodeBody(t, w)
	// request_id is tagged omitempty; when not set it must not appear in JSON.
	_, exists := body["request_id"]
	assert.False(t, exists, "request_id must be omitted when not set in context")
}

func TestSuccessResponse_WithRequestID(t *testing.T) {
	c, w := newTestContext()
	c.Set("request_id", "test-req-abc-123")
	SuccessResponse(c, nil)
	body := decodeBody(t, w)
	assert.Equal(t, "test-req-abc-123", body["request_id"])
}

func TestSuccessResponse_NilData(t *testing.T) {
	c, w := newTestContext()
	SuccessResponse(c, nil)
	assert.Equal(t, http.StatusOK, w.Code)
	body := decodeBody(t, w)
	assert.Equal(t, true, body["success"])
}

// ---- SuccessResponseWithMeta ------------------------------------------------

func TestSuccessResponseWithMeta_StatusCode(t *testing.T) {
	c, w := newTestContext()
	meta := &MetaData{Page: 1, PerPage: 10, Total: 50, TotalPages: 5}
	SuccessResponseWithMeta(c, []string{"a", "b"}, meta)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSuccessResponseWithMeta_SuccessTrue(t *testing.T) {
	c, w := newTestContext()
	meta := &MetaData{Page: 1, PerPage: 10, Total: 50, TotalPages: 5}
	SuccessResponseWithMeta(c, nil, meta)
	body := decodeBody(t, w)
	assert.Equal(t, true, body["success"])
}

func TestSuccessResponseWithMeta_MetaFields(t *testing.T) {
	c, w := newTestContext()
	meta := &MetaData{Page: 2, PerPage: 5, Total: 42, TotalPages: 9}
	SuccessResponseWithMeta(c, nil, meta)

	body := decodeBody(t, w)
	metaRaw, ok := body["meta"].(map[string]interface{})
	require.True(t, ok, "meta must be a JSON object")

	assert.EqualValues(t, 2, metaRaw["page"])
	assert.EqualValues(t, 5, metaRaw["per_page"])
	assert.EqualValues(t, 42, metaRaw["total"])
	assert.EqualValues(t, 9, metaRaw["total_pages"])
}

func TestSuccessResponseWithMeta_WithRequestID(t *testing.T) {
	c, w := newTestContext()
	c.Set("request_id", "meta-req-id-xyz")
	SuccessResponseWithMeta(c, nil, &MetaData{})
	body := decodeBody(t, w)
	assert.Equal(t, "meta-req-id-xyz", body["request_id"])
}

// ---- CreatedResponse --------------------------------------------------------

func TestCreatedResponse_StatusCode(t *testing.T) {
	c, w := newTestContext()
	CreatedResponse(c, map[string]string{"id": "new-resource"})
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreatedResponse_SuccessTrue(t *testing.T) {
	c, w := newTestContext()
	CreatedResponse(c, map[string]string{"id": "new-resource"})
	body := decodeBody(t, w)
	assert.Equal(t, true, body["success"])
}

func TestCreatedResponse_DataPresent(t *testing.T) {
	c, w := newTestContext()
	CreatedResponse(c, map[string]string{"id": "abc"})
	body := decodeBody(t, w)
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok, "data must be a JSON object")
	assert.Equal(t, "abc", data["id"])
}

func TestCreatedResponse_ErrorAbsent(t *testing.T) {
	c, w := newTestContext()
	CreatedResponse(c, nil)
	body := decodeBody(t, w)
	assert.Nil(t, body["error"])
}

func TestCreatedResponse_WithRequestID(t *testing.T) {
	c, w := newTestContext()
	c.Set("request_id", "created-req-001")
	CreatedResponse(c, nil)
	body := decodeBody(t, w)
	assert.Equal(t, "created-req-001", body["request_id"])
}

// ---- NoContentResponse ------------------------------------------------------

func TestNoContentResponse_StatusCode(t *testing.T) {
	c, _ := newTestContext()
	NoContentResponse(c)
	assert.Equal(t, http.StatusNoContent, c.Writer.Status())
}

func TestNoContentResponse_EmptyBody(t *testing.T) {
	c, w := newTestContext()
	NoContentResponse(c)
	assert.Empty(t, w.Body.Bytes(), "204 No Content must have no response body")
}

// ---- ErrorResponse ----------------------------------------------------------

func TestErrorResponse_SuccessFalse(t *testing.T) {
	c, w := newTestContext()
	appErr := errors.NewInternalServerError("something went wrong")
	ErrorResponse(c, appErr)
	body := decodeBody(t, w)
	assert.Equal(t, false, body["success"])
}

func TestErrorResponse_StatusCode(t *testing.T) {
	c, w := newTestContext()
	appErr := errors.NewInternalServerError("something went wrong")
	ErrorResponse(c, appErr)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestErrorResponse_ErrorFieldPresent(t *testing.T) {
	c, w := newTestContext()
	appErr := errors.NewBadRequestError("bad input", map[string]interface{}{"field": "name"})
	ErrorResponse(c, appErr)
	body := decodeBody(t, w)
	errObj, ok := body["error"].(map[string]interface{})
	require.True(t, ok, "error must be a JSON object")
	assert.Equal(t, errors.ErrCodeBadRequest, errObj["code"])
	assert.Equal(t, "bad input", errObj["message"])
}

func TestErrorResponse_ErrorDetails(t *testing.T) {
	c, w := newTestContext()
	details := map[string]interface{}{"field": "email", "reason": "invalid format"}
	appErr := errors.NewBadRequestError("validation failed", details)
	ErrorResponse(c, appErr)

	body := decodeBody(t, w)
	errObj, ok := body["error"].(map[string]interface{})
	require.True(t, ok)
	detailsRaw, ok := errObj["details"].(map[string]interface{})
	require.True(t, ok, "error.details must be a JSON object")
	assert.Equal(t, "email", detailsRaw["field"])
	assert.Equal(t, "invalid format", detailsRaw["reason"])
}

func TestErrorResponse_DataAbsent(t *testing.T) {
	c, w := newTestContext()
	appErr := errors.NewInternalServerError("err")
	ErrorResponse(c, appErr)
	body := decodeBody(t, w)
	assert.Nil(t, body["data"], "data must be absent on an error response")
}

func TestErrorResponse_TimestampPresent(t *testing.T) {
	c, w := newTestContext()
	ErrorResponse(c, errors.NewInternalServerError("err"))
	body := decodeBody(t, w)
	_, ok := body["timestamp"]
	assert.True(t, ok, "timestamp must be present on error responses")
}

func TestErrorResponse_WithRequestID(t *testing.T) {
	c, w := newTestContext()
	c.Set("request_id", "err-req-007")
	ErrorResponse(c, errors.NewInternalServerError("err"))
	body := decodeBody(t, w)
	assert.Equal(t, "err-req-007", body["request_id"])
}

func TestErrorResponse_UsesAppErrStatusCode(t *testing.T) {
	tests := []struct {
		name     string
		appErr   errors.AppError
		wantCode int
	}{
		{"unauthorized", errors.NewUnauthorizedError("denied"), http.StatusUnauthorized},
		{"forbidden", errors.NewForbiddenError("no access"), http.StatusForbidden},
		{"not found", errors.NewNotFoundError("User"), http.StatusNotFound},
		{"conflict", errors.NewConflictError("duplicate", nil), http.StatusConflict},
		{"internal", errors.NewInternalServerError("crash"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := newTestContext()
			ErrorResponse(c, tt.appErr)
			assert.Equal(t, tt.wantCode, w.Code)
		})
	}
}

// ---- ValidationErrorResponse ------------------------------------------------

func TestValidationErrorResponse_StatusCode(t *testing.T) {
	c, w := newTestContext()
	ValidationErrorResponse(c, map[string]interface{}{"username": "required"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestValidationErrorResponse_SuccessFalse(t *testing.T) {
	c, w := newTestContext()
	ValidationErrorResponse(c, map[string]interface{}{"email": "invalid"})
	body := decodeBody(t, w)
	assert.Equal(t, false, body["success"])
}

func TestValidationErrorResponse_ErrorCode(t *testing.T) {
	c, w := newTestContext()
	ValidationErrorResponse(c, map[string]interface{}{"field": "required"})
	body := decodeBody(t, w)
	errObj, ok := body["error"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, errors.ErrCodeValidationFailed, errObj["code"])
}

func TestValidationErrorResponse_ValidationDetailsPassedThrough(t *testing.T) {
	c, w := newTestContext()
	validationErrs := map[string]interface{}{
		"email":    "must be a valid email address",
		"password": "must be at least 8 characters",
	}
	ValidationErrorResponse(c, validationErrs)

	body := decodeBody(t, w)
	errObj := body["error"].(map[string]interface{})
	details := errObj["details"].(map[string]interface{})
	assert.Equal(t, "must be a valid email address", details["email"])
	assert.Equal(t, "must be at least 8 characters", details["password"])
}

// ---- UnauthorizedResponse ---------------------------------------------------

func TestUnauthorizedResponse_StatusCode(t *testing.T) {
	c, w := newTestContext()
	UnauthorizedResponse(c, "token expired")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUnauthorizedResponse_ErrorCode(t *testing.T) {
	c, w := newTestContext()
	UnauthorizedResponse(c, "token expired")
	body := decodeBody(t, w)
	errObj := body["error"].(map[string]interface{})
	assert.Equal(t, errors.ErrCodeUnauthorized, errObj["code"])
}

func TestUnauthorizedResponse_MessagePropagated(t *testing.T) {
	c, w := newTestContext()
	UnauthorizedResponse(c, "session has expired")
	body := decodeBody(t, w)
	errObj := body["error"].(map[string]interface{})
	assert.Equal(t, "session has expired", errObj["message"])
}

func TestUnauthorizedResponse_SuccessFalse(t *testing.T) {
	c, w := newTestContext()
	UnauthorizedResponse(c, "unauthorized")
	body := decodeBody(t, w)
	assert.Equal(t, false, body["success"])
}

// ---- ForbiddenResponse ------------------------------------------------------

func TestForbiddenResponse_StatusCode(t *testing.T) {
	c, w := newTestContext()
	ForbiddenResponse(c, "insufficient permissions")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestForbiddenResponse_ErrorCode(t *testing.T) {
	c, w := newTestContext()
	ForbiddenResponse(c, "access denied")
	body := decodeBody(t, w)
	errObj := body["error"].(map[string]interface{})
	assert.Equal(t, errors.ErrCodeForbidden, errObj["code"])
}

func TestForbiddenResponse_MessagePropagated(t *testing.T) {
	c, w := newTestContext()
	ForbiddenResponse(c, "you are not allowed here")
	body := decodeBody(t, w)
	errObj := body["error"].(map[string]interface{})
	assert.Equal(t, "you are not allowed here", errObj["message"])
}

// ---- NotFoundResponse -------------------------------------------------------

func TestNotFoundResponse_StatusCode(t *testing.T) {
	c, w := newTestContext()
	NotFoundResponse(c, "Order")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestNotFoundResponse_ErrorCode(t *testing.T) {
	c, w := newTestContext()
	NotFoundResponse(c, "Order")
	body := decodeBody(t, w)
	errObj := body["error"].(map[string]interface{})
	assert.Equal(t, errors.ErrCodeNotFound, errObj["code"])
}

func TestNotFoundResponse_MessageContainsResource(t *testing.T) {
	c, w := newTestContext()
	NotFoundResponse(c, "Product")
	body := decodeBody(t, w)
	errObj := body["error"].(map[string]interface{})
	// errors.NewNotFoundError formats as "<resource> not found"
	assert.Contains(t, errObj["message"], "Product")
}

// ---- BadRequestResponse -----------------------------------------------------

func TestBadRequestResponse_StatusCode(t *testing.T) {
	c, w := newTestContext()
	BadRequestResponse(c, "malformed JSON", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBadRequestResponse_ErrorCode(t *testing.T) {
	c, w := newTestContext()
	BadRequestResponse(c, "invalid payload", nil)
	body := decodeBody(t, w)
	errObj := body["error"].(map[string]interface{})
	assert.Equal(t, errors.ErrCodeBadRequest, errObj["code"])
}

func TestBadRequestResponse_MessagePropagated(t *testing.T) {
	c, w := newTestContext()
	BadRequestResponse(c, "the field X is required", nil)
	body := decodeBody(t, w)
	errObj := body["error"].(map[string]interface{})
	assert.Equal(t, "the field X is required", errObj["message"])
}

func TestBadRequestResponse_DetailsPropagated(t *testing.T) {
	c, w := newTestContext()
	details := map[string]interface{}{"field": "start_date", "hint": "use ISO 8601"}
	BadRequestResponse(c, "invalid date", details)
	body := decodeBody(t, w)
	errObj := body["error"].(map[string]interface{})
	d := errObj["details"].(map[string]interface{})
	assert.Equal(t, "start_date", d["field"])
	assert.Equal(t, "use ISO 8601", d["hint"])
}

func TestBadRequestResponse_NilDetails(t *testing.T) {
	c, w := newTestContext()
	BadRequestResponse(c, "bad request", nil)
	// Must not panic and must still return 400.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---- ConflictResponse -------------------------------------------------------

func TestConflictResponse_StatusCode(t *testing.T) {
	c, w := newTestContext()
	ConflictResponse(c, "email already registered", nil)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestConflictResponse_ErrorCode(t *testing.T) {
	c, w := newTestContext()
	ConflictResponse(c, "duplicate entry", nil)
	body := decodeBody(t, w)
	errObj := body["error"].(map[string]interface{})
	assert.Equal(t, errors.ErrCodeConflict, errObj["code"])
}

func TestConflictResponse_MessageAndDetails(t *testing.T) {
	c, w := newTestContext()
	details := map[string]interface{}{"conflicting_id": "usr-42"}
	ConflictResponse(c, "username taken", details)
	body := decodeBody(t, w)
	errObj := body["error"].(map[string]interface{})
	assert.Equal(t, "username taken", errObj["message"])
	d := errObj["details"].(map[string]interface{})
	assert.Equal(t, "usr-42", d["conflicting_id"])
}

// ---- InternalServerErrorResponse --------------------------------------------

func TestInternalServerErrorResponse_StatusCode(t *testing.T) {
	c, w := newTestContext()
	InternalServerErrorResponse(c, "database connection lost")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestInternalServerErrorResponse_ErrorCode(t *testing.T) {
	c, w := newTestContext()
	InternalServerErrorResponse(c, "unexpected failure")
	body := decodeBody(t, w)
	errObj := body["error"].(map[string]interface{})
	assert.Equal(t, errors.ErrCodeInternalServer, errObj["code"])
}

func TestInternalServerErrorResponse_MessagePropagated(t *testing.T) {
	c, w := newTestContext()
	InternalServerErrorResponse(c, "disk full")
	body := decodeBody(t, w)
	errObj := body["error"].(map[string]interface{})
	assert.Equal(t, "disk full", errObj["message"])
}

func TestInternalServerErrorResponse_SuccessFalse(t *testing.T) {
	c, w := newTestContext()
	InternalServerErrorResponse(c, "fatal error")
	body := decodeBody(t, w)
	assert.Equal(t, false, body["success"])
}

// ---- PaginatedResponse (pure calculation, no HTTP) --------------------------

func TestPaginatedResponse_BasicCalculation(t *testing.T) {
	meta := PaginatedResponse(1, 10, 100)
	assert.Equal(t, 1, meta.Page)
	assert.Equal(t, 10, meta.PerPage)
	assert.EqualValues(t, 100, meta.Total)
	assert.Equal(t, 10, meta.TotalPages)
}

func TestPaginatedResponse_TotalPagesRoundsUp(t *testing.T) {
	// 11 items with page size 10 requires 2 pages.
	meta := PaginatedResponse(1, 10, 11)
	assert.Equal(t, 2, meta.TotalPages)
}

func TestPaginatedResponse_ExactDivision(t *testing.T) {
	// 20 items with page size 10 is exactly 2 pages.
	meta := PaginatedResponse(1, 10, 20)
	assert.Equal(t, 2, meta.TotalPages)
}

func TestPaginatedResponse_SingleItem(t *testing.T) {
	// 1 item with any page size > 0 is 1 page.
	meta := PaginatedResponse(1, 25, 1)
	assert.Equal(t, 1, meta.TotalPages)
}

func TestPaginatedResponse_ZeroTotal(t *testing.T) {
	// Zero total items means 0 pages.
	meta := PaginatedResponse(1, 10, 0)
	assert.EqualValues(t, 0, meta.Total)
	assert.Equal(t, 0, meta.TotalPages)
}

func TestPaginatedResponse_OneLessThanFullPage(t *testing.T) {
	// 9 items with page size 10 still requires 1 page.
	meta := PaginatedResponse(1, 10, 9)
	assert.Equal(t, 1, meta.TotalPages)
}

func TestPaginatedResponse_LargeDataset(t *testing.T) {
	// 1,000,001 items with page size 100 requires 10,001 pages.
	meta := PaginatedResponse(1, 100, 1_000_001)
	assert.Equal(t, 10_001, meta.TotalPages)
}

func TestPaginatedResponse_PageSizeOne(t *testing.T) {
	// Each item is its own page when per_page is 1.
	meta := PaginatedResponse(3, 1, 7)
	assert.Equal(t, 7, meta.TotalPages)
}

func TestPaginatedResponse_FieldsPassedThrough(t *testing.T) {
	meta := PaginatedResponse(5, 25, 300)
	assert.Equal(t, 5, meta.Page)
	assert.Equal(t, 25, meta.PerPage)
	assert.EqualValues(t, 300, meta.Total)
	// ceil(300/25) = 12
	assert.Equal(t, 12, meta.TotalPages)
}

// ---- RequestID across helper functions (table-driven) -----------------------

func TestRequestID_PropagatedForAllHelpers(t *testing.T) {
	const rid = "global-request-id-99"

	tests := []struct {
		name     string
		call     func(c *gin.Context)
		wantCode int
	}{
		{
			name:     "SuccessResponse",
			call:     func(c *gin.Context) { SuccessResponse(c, nil) },
			wantCode: http.StatusOK,
		},
		{
			name: "SuccessResponseWithMeta",
			call: func(c *gin.Context) {
				SuccessResponseWithMeta(c, nil, &MetaData{})
			},
			wantCode: http.StatusOK,
		},
		{
			name:     "CreatedResponse",
			call:     func(c *gin.Context) { CreatedResponse(c, nil) },
			wantCode: http.StatusCreated,
		},
		{
			name:     "ErrorResponse",
			call:     func(c *gin.Context) { ErrorResponse(c, errors.NewInternalServerError("err")) },
			wantCode: http.StatusInternalServerError,
		},
		{
			name: "ValidationErrorResponse",
			call: func(c *gin.Context) {
				ValidationErrorResponse(c, map[string]interface{}{"f": "required"})
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "UnauthorizedResponse",
			call:     func(c *gin.Context) { UnauthorizedResponse(c, "denied") },
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "ForbiddenResponse",
			call:     func(c *gin.Context) { ForbiddenResponse(c, "forbidden") },
			wantCode: http.StatusForbidden,
		},
		{
			name:     "NotFoundResponse",
			call:     func(c *gin.Context) { NotFoundResponse(c, "Resource") },
			wantCode: http.StatusNotFound,
		},
		{
			name:     "BadRequestResponse",
			call:     func(c *gin.Context) { BadRequestResponse(c, "bad", nil) },
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "ConflictResponse",
			call:     func(c *gin.Context) { ConflictResponse(c, "conflict", nil) },
			wantCode: http.StatusConflict,
		},
		{
			name:     "InternalServerErrorResponse",
			call:     func(c *gin.Context) { InternalServerErrorResponse(c, "err") },
			wantCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := newTestContext()
			c.Set("request_id", rid)
			tt.call(c)

			assert.Equal(t, tt.wantCode, w.Code)
			body := decodeBody(t, w)
			assert.Equal(t, rid, body["request_id"],
				"request_id must be echoed back for %s", tt.name)
		})
	}
}

// ---- Timestamp presence across all JSON-emitting helpers --------------------

func TestTimestamp_PresentForAllHelpers(t *testing.T) {
	tests := []struct {
		name string
		call func(c *gin.Context)
	}{
		{"SuccessResponse", func(c *gin.Context) { SuccessResponse(c, nil) }},
		{"SuccessResponseWithMeta", func(c *gin.Context) {
			SuccessResponseWithMeta(c, nil, &MetaData{})
		}},
		{"CreatedResponse", func(c *gin.Context) { CreatedResponse(c, nil) }},
		{"ErrorResponse", func(c *gin.Context) {
			ErrorResponse(c, errors.NewInternalServerError("err"))
		}},
		{"UnauthorizedResponse", func(c *gin.Context) { UnauthorizedResponse(c, "deny") }},
		{"ForbiddenResponse", func(c *gin.Context) { ForbiddenResponse(c, "no") }},
		{"NotFoundResponse", func(c *gin.Context) { NotFoundResponse(c, "X") }},
		{"BadRequestResponse", func(c *gin.Context) { BadRequestResponse(c, "bad", nil) }},
		{"ConflictResponse", func(c *gin.Context) { ConflictResponse(c, "dup", nil) }},
		{"InternalServerErrorResponse", func(c *gin.Context) {
			InternalServerErrorResponse(c, "fatal")
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := newTestContext()
			tt.call(c)

			body := decodeBody(t, w)
			tsRaw, exists := body["timestamp"]
			assert.True(t, exists, "timestamp must be present in response from %s", tt.name)
			tsStr, ok := tsRaw.(string)
			require.True(t, ok, "timestamp must be a JSON string")
			_, err := time.Parse(time.RFC3339Nano, tsStr)
			assert.NoError(t, err, "timestamp must be a valid RFC3339 time string")
		})
	}
}

// ---- Content-Type -----------------------------------------------------------

func TestResponseHelpers_ContentTypeJSON(t *testing.T) {
	tests := []struct {
		name string
		call func(c *gin.Context)
	}{
		{"SuccessResponse", func(c *gin.Context) { SuccessResponse(c, "data") }},
		{"CreatedResponse", func(c *gin.Context) { CreatedResponse(c, "data") }},
		{"ErrorResponse", func(c *gin.Context) {
			ErrorResponse(c, errors.NewBadRequestError("bad", nil))
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := newTestContext()
			tt.call(c)
			assert.Contains(t, w.Header().Get("Content-Type"), "application/json",
				"Content-Type must be application/json for %s", tt.name)
		})
	}
}

// ---- Struct-level assertions using StandardResponse -------------------------

func TestSuccessResponse_StructDecoding(t *testing.T) {
	c, w := newTestContext()
	c.Set("request_id", "struct-test-id")
	SuccessResponse(c, map[string]int{"count": 3})

	resp := decodeStandardResponse(t, w)
	assert.True(t, resp.Success)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Data)
	assert.Equal(t, "struct-test-id", resp.RequestID)
	assert.False(t, resp.Timestamp.IsZero(), "Timestamp must be non-zero")
}

func TestErrorResponse_StructDecoding(t *testing.T) {
	c, w := newTestContext()
	c.Set("request_id", "struct-err-id")
	appErr := errors.NewForbiddenError("access denied")
	ErrorResponse(c, appErr)

	resp := decodeStandardResponse(t, w)
	assert.False(t, resp.Success)
	assert.Nil(t, resp.Data)
	require.NotNil(t, resp.Error)
	assert.Equal(t, errors.ErrCodeForbidden, resp.Error.Code)
	assert.Equal(t, "access denied", resp.Error.Message)
	assert.Equal(t, "struct-err-id", resp.RequestID)
	assert.False(t, resp.Timestamp.IsZero())
}

func TestSuccessResponseWithMeta_StructDecoding(t *testing.T) {
	c, w := newTestContext()
	meta := PaginatedResponse(2, 15, 45)
	SuccessResponseWithMeta(c, []string{"x"}, meta)

	resp := decodeStandardResponse(t, w)
	assert.True(t, resp.Success)
	require.NotNil(t, resp.Meta)
	assert.Equal(t, 2, resp.Meta.Page)
	assert.Equal(t, 15, resp.Meta.PerPage)
	assert.EqualValues(t, 45, resp.Meta.Total)
	assert.Equal(t, 3, resp.Meta.TotalPages) // ceil(45/15) = 3
}
