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

// Finding 1: Body-read timeout must be classified as ErrDeadlineExceeded, not ErrUnavailable.
func TestGet_BodyReadTimeoutIsDeadlineExceeded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Write headers and flush to pass the Do() successfully
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		flusher.Flush()
		// Then sleep past the client timeout before writing body
		time.Sleep(150 * time.Millisecond)
		_, _ = w.Write([]byte(`{"handle":"mug"}`))
	}))
	defer srv.Close()

	c, err := New(srv.URL, WithTimeout(20*time.Millisecond))
	require.NoError(t, err)

	var got result
	err = c.Get(context.Background(), "/stall", nil, &got)
	require.ErrorIs(t, err, ErrDeadlineExceeded,
		"body-read timeout must be ErrDeadlineExceeded, not ErrUnavailable")
	assert.NotErrorIs(t, err, ErrUnavailable)
}

// Finding 2: WithHTTPClient must not mutate the caller's *http.Client.
func TestWithHTTPClient_DoesNotMutateCaller(t *testing.T) {
	callerClient := &http.Client{Timeout: 0}
	originalTimeout := callerClient.Timeout

	_, err := New("http://example.com", WithTimeout(500*time.Millisecond), WithHTTPClient(callerClient))
	require.NoError(t, err)

	assert.Equal(t, originalTimeout, callerClient.Timeout,
		"WithHTTPClient must not mutate the caller's *http.Client.Timeout")
}

// Finding 3a: Base URL path prefix must be preserved.
func TestGet_PreservesBaseURLPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/products", r.URL.Path,
			"base path /api/v1 must be preserved when requesting /products")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"handle":"item"}`))
	}))
	defer srv.Close()

	c, err := New(srv.URL + "/api/v1")
	require.NoError(t, err)

	var got result
	require.NoError(t, c.Get(context.Background(), "/products", nil, &got))
	assert.Equal(t, "item", got.Handle)
}

// Finding 3b: Path traversal with .. must be rejected.
func TestGet_RejectsPathTraversal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server must not receive request with path %s", r.URL.Path)
	}))
	defer srv.Close()

	c, err := New(srv.URL + "/api/v1/products")
	require.NoError(t, err)

	var got result
	err = c.Get(context.Background(), "/../admin", nil, &got)
	require.Error(t, err, "path traversal must be rejected")
	assert.NotErrorIs(t, err, ErrNotFound)
	assert.NotErrorIs(t, err, ErrUnavailable)
	assert.NotErrorIs(t, err, ErrDeadlineExceeded)
}

// A redirect must never carry the credential to a host the operator did not
// configure. Go's stdlib strips Authorization/Cookie across hosts but copies
// everything WithHeader added, so the only safe policy is not to follow.
func TestGet_DoesNotFollowRedirect(t *testing.T) {
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("redirect target must never be contacted; got %s with key %q",
			r.URL.Path, r.Header.Get("X-Storefront-Key"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"handle":"pwned"}`))
	}))
	defer evil.Close()

	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, evil.URL+"/admin/secrets", http.StatusFound)
	}))
	defer good.Close()

	c, err := New(good.URL+"/api/v1", WithHeader("X-Storefront-Key", "secret"))
	require.NoError(t, err)

	var got result
	err = c.Get(context.Background(), "/products", nil, &got)
	require.Error(t, err, "a 3xx is not a result; it must not decode as one")
	assert.ErrorIs(t, err, ErrUnavailable)
	assert.Empty(t, got.Handle)
}

// The redirect policy must survive a caller-supplied client, or the hole
// reopens through WithHTTPClient.
func TestGet_DoesNotFollowRedirectWithCallerClient(t *testing.T) {
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("redirect target must never be contacted; got %s", r.URL.Path)
	}))
	defer evil.Close()

	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, evil.URL+"/admin/secrets", http.StatusFound)
	}))
	defer good.Close()

	c, err := New(good.URL, WithHTTPClient(&http.Client{}))
	require.NoError(t, err)

	var got result
	require.Error(t, c.Get(context.Background(), "/products", nil, &got))
}

// The per-request budget is a contract, so option ORDER must not be able to
// change it. WithHTTPClient(clientWith30s) silently winning over WithTimeout is
// a 75x overrun of a 400ms budget.
func TestWithTimeout_SurvivesWithHTTPClientInEitherOrder(t *testing.T) {
	want := time.Second

	callerFirst, err := New("http://example.com",
		WithHTTPClient(&http.Client{Timeout: 30 * time.Second}), WithTimeout(want))
	require.NoError(t, err)

	timeoutFirst, err := New("http://example.com",
		WithTimeout(want), WithHTTPClient(&http.Client{Timeout: 30 * time.Second}))
	require.NoError(t, err)

	assert.Equal(t, want, callerFirst.http.Timeout)
	assert.Equal(t, want, timeoutFirst.http.Timeout,
		"a caller-supplied client must not silently discard WithTimeout")
}

// With no WithTimeout, the default budget still applies to a supplied client.
func TestWithHTTPClient_DefaultTimeoutStillApplies(t *testing.T) {
	c, err := New("http://example.com", WithHTTPClient(&http.Client{Timeout: 30 * time.Second}))
	require.NoError(t, err)
	assert.Equal(t, defaultTimeout, c.http.Timeout)
}
