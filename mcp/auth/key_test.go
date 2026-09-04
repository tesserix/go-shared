package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func ok() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func do(t *testing.T, h http.Handler, key string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	if key != "" {
		req.Header.Set(HeaderName, key)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestRequireKey_AcceptsMatching(t *testing.T) {
	assert.Equal(t, http.StatusOK, do(t, RequireKey("s3cret", ok()), "s3cret"))
}

func TestRequireKey_RejectsWrongAndMissing(t *testing.T) {
	h := RequireKey("s3cret", ok())
	assert.Equal(t, http.StatusUnauthorized, do(t, h, "wrong"))
	assert.Equal(t, http.StatusUnauthorized, do(t, h, ""))
}

// The case that matters most: an unset secret must not become an open door.
// A missing ExternalSecret is a deployment mistake, and the safe reading of it
// is "nobody may call", never "everybody may".
func TestRequireKey_EmptyExpectedFailsClosed(t *testing.T) {
	h := RequireKey("", ok())
	assert.Equal(t, http.StatusUnauthorized, do(t, h, ""))
	assert.Equal(t, http.StatusUnauthorized, do(t, h, "anything"))
}
