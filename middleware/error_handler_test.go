package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apperrors "github.com/tesserix/go-shared/errors"
)

// decodeErrorBody is a helper that unmarshals the recorder body into a map for
// assertions in this test file.
func decodeErrorBody(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err, "response body must be valid JSON")
	return body
}

func TestErrorHandler_PassesThroughNoErrors(t *testing.T) {
	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)

	router.Use(ErrorHandler())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

func TestErrorHandler_HandlesAppError(t *testing.T) {
	tests := []struct {
		name       string
		appErr     apperrors.AppError
		wantStatus int
		wantCode   string
	}{
		{
			name:       "bad request",
			appErr:     apperrors.NewBadRequestError("invalid input", nil),
			wantStatus: http.StatusBadRequest,
			wantCode:   apperrors.ErrCodeBadRequest,
		},
		{
			name:       "unauthorized",
			appErr:     apperrors.NewUnauthorizedError("token expired"),
			wantStatus: http.StatusUnauthorized,
			wantCode:   apperrors.ErrCodeUnauthorized,
		},
		{
			name:       "not found",
			appErr:     apperrors.NewNotFoundError("User"),
			wantStatus: http.StatusNotFound,
			wantCode:   apperrors.ErrCodeNotFound,
		},
		{
			name:       "forbidden",
			appErr:     apperrors.NewForbiddenError("access denied"),
			wantStatus: http.StatusForbidden,
			wantCode:   apperrors.ErrCodeForbidden,
		},
		{
			name:       "internal server error",
			appErr:     apperrors.NewInternalServerError("something went wrong"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   apperrors.ErrCodeInternalServer,
		},
		{
			name:       "conflict",
			appErr:     apperrors.NewConflictError("duplicate entry", nil),
			wantStatus: http.StatusConflict,
			wantCode:   apperrors.ErrCodeConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			_, router := gin.CreateTestContext(w)

			router.Use(ErrorHandler())
			router.GET("/test", func(c *gin.Context) {
				_ = c.Error(tt.appErr)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			body := decodeErrorBody(t, w)
			assert.Equal(t, false, body["success"])

			errObj, ok := body["error"].(map[string]interface{})
			require.True(t, ok, "error field must be a JSON object")
			assert.Equal(t, tt.wantCode, errObj["code"])
		})
	}
}

func TestErrorHandler_HandlesUnknownErrorAs500(t *testing.T) {
	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)

	router.Use(ErrorHandler())
	router.GET("/test", func(c *gin.Context) {
		_ = c.Error(errors.New("some unexpected low-level error"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	body := decodeErrorBody(t, w)
	assert.Equal(t, false, body["success"])

	errObj, ok := body["error"].(map[string]interface{})
	require.True(t, ok, "error field must be a JSON object")
	assert.Equal(t, apperrors.ErrCodeInternalServer, errObj["code"])
}

func TestRecovery_HandlesStringPanic(t *testing.T) {
	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)

	router.Use(Recovery())
	router.GET("/test", func(c *gin.Context) {
		panic("something went terribly wrong")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	body := decodeErrorBody(t, w)
	assert.Equal(t, false, body["success"])

	errObj, ok := body["error"].(map[string]interface{})
	require.True(t, ok, "error field must be a JSON object")
	assert.Equal(t, apperrors.ErrCodeInternalServer, errObj["code"])

	// The panic message should be included in the error message for string panics.
	msg, _ := errObj["message"].(string)
	assert.Contains(t, msg, "Panic recovered")
}

func TestRecovery_HandlesNonStringPanic(t *testing.T) {
	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)

	router.Use(Recovery())
	router.GET("/test", func(c *gin.Context) {
		// Panic with a non-string value (e.g. a struct or integer).
		panic(42)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	body := decodeErrorBody(t, w)
	assert.Equal(t, false, body["success"])

	errObj, ok := body["error"].(map[string]interface{})
	require.True(t, ok, "error field must be a JSON object")
	assert.Equal(t, apperrors.ErrCodeInternalServer, errObj["code"])
}

func TestRecovery_HandlesErrorPanic(t *testing.T) {
	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)

	router.Use(Recovery())
	router.GET("/test", func(c *gin.Context) {
		panic(errors.New("wrapped error value"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	body := decodeErrorBody(t, w)
	assert.Equal(t, false, body["success"])
}

func TestRecovery_ContinuesServingAfterPanic(t *testing.T) {
	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)

	router.Use(Recovery())
	router.GET("/panic", func(c *gin.Context) {
		panic("deliberate panic")
	})
	router.GET("/ok", func(c *gin.Context) {
		c.String(http.StatusOK, "healthy")
	})

	// First request panics.
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// Second request should be served normally — recovery must not break the router.
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/ok", nil)
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, "healthy", w2.Body.String())
}
