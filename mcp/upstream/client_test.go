package upstream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type result struct {
	Handle string `json:"handle"`
}

func TestGet_HappyPathSendsHeadersAndParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "secret", r.Header.Get("X-Storefront-Key"))
		assert.Equal(t, "20", r.URL.Query().Get("limit"))
		assert.Equal(t, "/api/v1/storefront/stores/bondi/products", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"handle":"mug"}`))
	}))
	defer srv.Close()

	c, err := New(srv.URL, WithHeader("X-Storefront-Key", "secret"))
	require.NoError(t, err)

	var got result
	require.NoError(t, c.Get(context.Background(),
		"/api/v1/storefront/stores/bondi/products", url.Values{"limit": {"20"}}, &got))
	assert.Equal(t, "mug", got.Handle)
}

func TestGet_404IsNotFoundNotUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	require.NoError(t, err)

	var got result
	err = c.Get(context.Background(), "/missing", nil, &got)
	// These must stay distinct: "this store does not exist" and "we could not
	// ask" lead to different agent behaviour.
	require.ErrorIs(t, err, ErrNotFound)
	assert.NotErrorIs(t, err, ErrUnavailable)
}

func TestGet_5xxIsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	require.NoError(t, err)

	var got result
	require.ErrorIs(t, c.Get(context.Background(), "/x", nil, &got), ErrUnavailable)
}

func TestGet_TimeoutIsDeadlineExceeded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(150 * time.Millisecond)
	}))
	defer srv.Close()

	c, err := New(srv.URL, WithTimeout(20*time.Millisecond))
	require.NoError(t, err)

	var got result
	require.ErrorIs(t, c.Get(context.Background(), "/slow", nil, &got), ErrDeadlineExceeded)
}

// D6, asserted structurally rather than by review. If someone adds Post, this
// fails — which is the entire point: a write must not be something an author
// merely remembers not to do.
func TestClient_ExposesOnlyGet(t *testing.T) {
	var methods []string
	ct := reflect.TypeOf(&Client{})
	for i := range ct.NumMethod() {
		methods = append(methods, ct.Method(i).Name)
	}
	assert.Equal(t, []string{"Get"}, methods,
		"the upstream client must be incapable of expressing a write")
}
