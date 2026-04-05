// Package http tests live in the same package so they can access unexported
// fields.  Because the package itself is named "http", we alias net/http as
// "stdhttp" throughout to avoid the name collision.
package http

import (
	"encoding/json"
	"fmt"
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// DefaultConfig
// -----------------------------------------------------------------------------

func TestDefaultConfig_CorrectValues(t *testing.T) {
	baseURL := "https://api.example.com"
	cfg := DefaultConfig(baseURL)

	assert.Equal(t, baseURL, cfg.BaseURL)
	assert.Equal(t, 30*time.Second, cfg.Timeout, "Timeout should be 30s")
	assert.Equal(t, 3, cfg.MaxRetries, "MaxRetries should be 3")
	assert.Equal(t, 1*time.Second, cfg.RetryDelay, "RetryDelay should be 1s")

	assert.Equal(t, "application/json", cfg.DefaultHeaders["Content-Type"])
	assert.Equal(t, "application/json", cfg.DefaultHeaders["Accept"])
	assert.Equal(t, "tesseract-hub-client/1.0", cfg.DefaultHeaders["User-Agent"])
}

func TestDefaultConfig_NilLoggerAndMetrics(t *testing.T) {
	cfg := DefaultConfig("http://example.com")

	assert.Nil(t, cfg.Logger, "Logger should be nil in default config")
	assert.Nil(t, cfg.Metrics, "Metrics should be nil in default config")
}

// -----------------------------------------------------------------------------
// NewClient
// -----------------------------------------------------------------------------

func TestNewClient_TrimsTrailingSlashFromBaseURL(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedURL string
	}{
		{"single trailing slash", "https://api.example.com/", "https://api.example.com"},
		{"multiple trailing slashes", "https://api.example.com///", "https://api.example.com"},
		{"no trailing slash", "https://api.example.com", "https://api.example.com"},
		{"path with trailing slash", "https://api.example.com/v1/", "https://api.example.com/v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig(tt.input)
			client := NewClient(cfg)

			assert.Equal(t, tt.expectedURL, client.baseURL)
		})
	}
}

func TestNewClient_SetsTimeout(t *testing.T) {
	cfg := DefaultConfig("https://example.com")
	cfg.Timeout = 15 * time.Second

	client := NewClient(cfg)

	assert.Equal(t, 15*time.Second, client.httpClient.Timeout)
	assert.Equal(t, 15*time.Second, client.timeout)
}

func TestNewClient_SetsDefaultHeaders(t *testing.T) {
	cfg := DefaultConfig("https://example.com")
	client := NewClient(cfg)

	assert.Equal(t, "application/json", client.headers["Content-Type"])
	assert.Equal(t, "application/json", client.headers["Accept"])
	assert.Equal(t, "tesseract-hub-client/1.0", client.headers["User-Agent"])
}

// -----------------------------------------------------------------------------
// Response helpers
// -----------------------------------------------------------------------------

func TestResponse_IsSuccess(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   bool
	}{
		{200, true},
		{201, true},
		{204, true},
		{299, true},
		{300, false},
		{400, false},
		{404, false},
		{500, false},
		{199, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.statusCode), func(t *testing.T) {
			resp := &Response{StatusCode: tt.statusCode}
			assert.Equal(t, tt.expected, resp.IsSuccess())
		})
	}
}

func TestResponse_IsClientError(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   bool
	}{
		{400, true},
		{401, true},
		{403, true},
		{404, true},
		{422, true},
		{499, true},
		{200, false},
		{201, false},
		{299, false},
		{300, false},
		{399, false},
		{500, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.statusCode), func(t *testing.T) {
			resp := &Response{StatusCode: tt.statusCode}
			assert.Equal(t, tt.expected, resp.IsClientError())
		})
	}
}

func TestResponse_IsServerError(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   bool
	}{
		{500, true},
		{501, true},
		{502, true},
		{503, true},
		{504, true},
		{599, true},
		{200, false},
		{201, false},
		{400, false},
		{404, false},
		{499, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.statusCode), func(t *testing.T) {
			resp := &Response{StatusCode: tt.statusCode}
			assert.Equal(t, tt.expected, resp.IsServerError())
		})
	}
}

func TestResponse_JSON_UnmarshalsCorrectly(t *testing.T) {
	type payload struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	data := payload{Name: "test", Value: 42}
	raw, err := json.Marshal(data)
	require.NoError(t, err)

	resp := &Response{Body: raw}

	var result payload
	err = resp.JSON(&result)

	assert.NoError(t, err)
	assert.Equal(t, data.Name, result.Name)
	assert.Equal(t, data.Value, result.Value)
}

func TestResponse_JSON_ReturnsErrorOnInvalidJSON(t *testing.T) {
	resp := &Response{Body: []byte("not json {")}

	var result map[string]interface{}
	err := resp.JSON(&result)

	assert.Error(t, err)
}

func TestResponse_String_ReturnsByteBodyAsString(t *testing.T) {
	tests := []struct {
		name     string
		body     []byte
		expected string
	}{
		{"simple string", []byte("hello world"), "hello world"},
		{"empty body", []byte{}, ""},
		{"json body", []byte(`{"key":"value"}`), `{"key":"value"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &Response{Body: tt.body}
			assert.Equal(t, tt.expected, resp.String())
		})
	}
}

// -----------------------------------------------------------------------------
// Client.SetHeader
// -----------------------------------------------------------------------------

func TestClient_SetHeader_SetsHeader(t *testing.T) {
	client := NewClient(DefaultConfig("https://example.com"))

	client.SetHeader("X-Custom-Header", "custom-value")

	assert.Equal(t, "custom-value", client.headers["X-Custom-Header"])
}

func TestClient_SetHeader_OverwritesExistingHeader(t *testing.T) {
	client := NewClient(DefaultConfig("https://example.com"))
	client.SetHeader("Content-Type", "text/plain")

	assert.Equal(t, "text/plain", client.headers["Content-Type"])
}

func TestClient_SetHeader_InitializesNilHeaders(t *testing.T) {
	// Manually construct a Client with nil headers to test the lazy init path.
	client := &Client{
		httpClient: &stdhttp.Client{},
		headers:    nil,
	}

	client.SetHeader("X-Test", "value")

	assert.NotNil(t, client.headers)
	assert.Equal(t, "value", client.headers["X-Test"])
}

// -----------------------------------------------------------------------------
// Client.SetAuthToken
// -----------------------------------------------------------------------------

func TestClient_SetAuthToken_SetsBearerHeader(t *testing.T) {
	client := NewClient(DefaultConfig("https://example.com"))

	client.SetAuthToken("my-secret-token")

	assert.Equal(t, "Bearer my-secret-token", client.headers["Authorization"])
}

func TestClient_SetAuthToken_OverwritesPreviousToken(t *testing.T) {
	client := NewClient(DefaultConfig("https://example.com"))
	client.SetAuthToken("first-token")
	client.SetAuthToken("second-token")

	assert.Equal(t, "Bearer second-token", client.headers["Authorization"])
}

// -----------------------------------------------------------------------------
// buildURL
// -----------------------------------------------------------------------------

func TestClient_BuildURL_WithoutQueryParams(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		path        string
		expectedURL string
	}{
		{
			name:        "simple path",
			baseURL:     "https://api.example.com",
			path:        "/users",
			expectedURL: "https://api.example.com/users",
		},
		{
			name:        "path without leading slash",
			baseURL:     "https://api.example.com",
			path:        "users",
			expectedURL: "https://api.example.com/users",
		},
		{
			name:        "nested path",
			baseURL:     "https://api.example.com",
			path:        "/v1/users/123",
			expectedURL: "https://api.example.com/v1/users/123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(DefaultConfig(tt.baseURL))
			result := client.buildURL(tt.path, nil)
			assert.Equal(t, tt.expectedURL, result)
		})
	}
}

func TestClient_BuildURL_WithQueryParams(t *testing.T) {
	client := NewClient(DefaultConfig("https://api.example.com"))

	rawURL := client.buildURL("/search", map[string]string{"q": "test"})

	assert.True(t, strings.HasPrefix(rawURL, "https://api.example.com/search?"),
		"URL should start with base + path + '?'")
	assert.Contains(t, rawURL, "q=test", "URL should contain the query parameter")
}

func TestClient_BuildURL_WithMultipleQueryParams(t *testing.T) {
	client := NewClient(DefaultConfig("https://api.example.com"))

	rawURL := client.buildURL("/items", map[string]string{
		"page":  "1",
		"limit": "20",
	})

	assert.Contains(t, rawURL, "page=1")
	assert.Contains(t, rawURL, "limit=20")
	assert.Contains(t, rawURL, "?")
}

func TestClient_BuildURL_EmptyQueryParamsOmitsQueryString(t *testing.T) {
	client := NewClient(DefaultConfig("https://api.example.com"))

	rawURL := client.buildURL("/items", map[string]string{})

	assert.NotContains(t, rawURL, "?", "Empty query params should not add a '?'")
}

// -----------------------------------------------------------------------------
// Client.Do — using httptest.NewServer
// -----------------------------------------------------------------------------

func TestClient_Do_GETRequest(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/resource", r.URL.Path)
		w.WriteHeader(stdhttp.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewClient(DefaultConfig(server.URL))
	resp, err := client.Do(Request{
		Method: "GET",
		Path:   "/api/resource",
	})

	require.NoError(t, err)
	assert.Equal(t, stdhttp.StatusOK, resp.StatusCode)
	assert.Equal(t, `{"status":"ok"}`, resp.String())
}

func TestClient_Do_GETWithQueryParams(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		assert.Equal(t, "search_term", r.URL.Query().Get("q"))
		assert.Equal(t, "10", r.URL.Query().Get("limit"))
		w.WriteHeader(stdhttp.StatusOK)
	}))
	defer server.Close()

	client := NewClient(DefaultConfig(server.URL))
	resp, err := client.Do(Request{
		Method: "GET",
		Path:   "/search",
		QueryParams: map[string]string{
			"q":     "search_term",
			"limit": "10",
		},
	})

	require.NoError(t, err)
	assert.Equal(t, stdhttp.StatusOK, resp.StatusCode)
}

func TestClient_Do_POSTWithJSONBody_Struct(t *testing.T) {
	type payload struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	var received payload

	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		assert.Equal(t, "POST", r.Method)
		err := json.NewDecoder(r.Body).Decode(&received)
		require.NoError(t, err)
		w.WriteHeader(stdhttp.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(DefaultConfig(server.URL))
	resp, err := client.Do(Request{
		Method: "POST",
		Path:   "/users",
		Body:   payload{Name: "Alice", Email: "alice@example.com"},
	})

	require.NoError(t, err)
	assert.Equal(t, stdhttp.StatusCreated, resp.StatusCode)
	assert.Equal(t, "Alice", received.Name)
	assert.Equal(t, "alice@example.com", received.Email)
}

func TestClient_Do_POSTWithStringBody(t *testing.T) {
	var receivedBody string

	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		raw, _ := io.ReadAll(r.Body)
		receivedBody = string(raw)
		w.WriteHeader(stdhttp.StatusOK)
	}))
	defer server.Close()

	client := NewClient(DefaultConfig(server.URL))
	resp, err := client.Do(Request{
		Method: "POST",
		Path:   "/data",
		Body:   "raw string body",
	})

	require.NoError(t, err)
	assert.Equal(t, stdhttp.StatusOK, resp.StatusCode)
	assert.Equal(t, "raw string body", receivedBody)
}

func TestClient_Do_POSTWithBytesBody(t *testing.T) {
	var receivedBody []byte

	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		w.WriteHeader(stdhttp.StatusOK)
	}))
	defer server.Close()

	client := NewClient(DefaultConfig(server.URL))
	resp, err := client.Do(Request{
		Method: "POST",
		Path:   "/data",
		Body:   []byte("byte slice body"),
	})

	require.NoError(t, err)
	assert.Equal(t, stdhttp.StatusOK, resp.StatusCode)
	assert.Equal(t, []byte("byte slice body"), receivedBody)
}

func TestClient_Do_SendsDefaultHeaders(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.Equal(t, "tesseract-hub-client/1.0", r.Header.Get("User-Agent"))
		w.WriteHeader(stdhttp.StatusOK)
	}))
	defer server.Close()

	client := NewClient(DefaultConfig(server.URL))
	_, err := client.Do(Request{Method: "GET", Path: "/"})
	require.NoError(t, err)
}

func TestClient_Do_SendsRequestLevelHeaders(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		assert.Equal(t, "request-value", r.Header.Get("X-Request-Header"))
		w.WriteHeader(stdhttp.StatusOK)
	}))
	defer server.Close()

	client := NewClient(DefaultConfig(server.URL))
	_, err := client.Do(Request{
		Method: "GET",
		Path:   "/",
		Headers: map[string]string{
			"X-Request-Header": "request-value",
		},
	})
	require.NoError(t, err)
}

func TestClient_Do_RequestLevelHeadersOverrideDefaults(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		assert.Equal(t, "text/plain", r.Header.Get("Content-Type"))
		w.WriteHeader(stdhttp.StatusOK)
	}))
	defer server.Close()

	client := NewClient(DefaultConfig(server.URL))
	_, err := client.Do(Request{
		Method: "POST",
		Path:   "/",
		Headers: map[string]string{
			"Content-Type": "text/plain",
		},
	})
	require.NoError(t, err)
}

func TestClient_Do_ResponseBodyIsRead(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(stdhttp.StatusOK)
		_, _ = w.Write([]byte(`{"message":"hello"}`))
	}))
	defer server.Close()

	client := NewClient(DefaultConfig(server.URL))
	resp, err := client.Do(Request{Method: "GET", Path: "/"})

	require.NoError(t, err)
	assert.Equal(t, `{"message":"hello"}`, resp.String())

	var result map[string]string
	err = resp.JSON(&result)
	require.NoError(t, err)
	assert.Equal(t, "hello", result["message"])
}

func TestClient_Do_Returns4xxResponseWithoutError(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.WriteHeader(stdhttp.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer server.Close()

	client := NewClient(DefaultConfig(server.URL))
	resp, err := client.Do(Request{Method: "GET", Path: "/missing"})

	// The client does not convert HTTP error status codes into Go errors.
	require.NoError(t, err)
	assert.Equal(t, stdhttp.StatusNotFound, resp.StatusCode)
	assert.True(t, resp.IsClientError())
	assert.False(t, resp.IsSuccess())
}

func TestClient_Do_Returns5xxResponseWithoutError(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.WriteHeader(stdhttp.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(DefaultConfig(server.URL))
	resp, err := client.Do(Request{Method: "GET", Path: "/"})

	require.NoError(t, err)
	assert.Equal(t, stdhttp.StatusInternalServerError, resp.StatusCode)
	assert.True(t, resp.IsServerError())
}

func TestClient_Do_ResponseIncludesDuration(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.WriteHeader(stdhttp.StatusOK)
	}))
	defer server.Close()

	client := NewClient(DefaultConfig(server.URL))
	resp, err := client.Do(Request{Method: "GET", Path: "/"})

	require.NoError(t, err)
	assert.Greater(t, resp.Duration, time.Duration(0), "Response duration should be positive")
}

// -----------------------------------------------------------------------------
// Convenience methods: Get, Post, Put, Patch, Delete
// -----------------------------------------------------------------------------

func TestClient_Get_SendsGETRequest(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/items", r.URL.Path)
		w.WriteHeader(stdhttp.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := NewClient(DefaultConfig(server.URL))
	resp, err := client.Get("/items", nil)

	require.NoError(t, err)
	assert.Equal(t, stdhttp.StatusOK, resp.StatusCode)
}

func TestClient_Get_SendsQueryParams(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		assert.Equal(t, "active", r.URL.Query().Get("status"))
		w.WriteHeader(stdhttp.StatusOK)
	}))
	defer server.Close()

	client := NewClient(DefaultConfig(server.URL))
	_, err := client.Get("/items", map[string]string{"status": "active"})
	require.NoError(t, err)
}

func TestClient_Post_SendsPOSTRequest(t *testing.T) {
	type body struct {
		Title string `json:"title"`
	}

	var received body

	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		assert.Equal(t, "POST", r.Method)
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(stdhttp.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(DefaultConfig(server.URL))
	resp, err := client.Post("/articles", body{Title: "Hello"})

	require.NoError(t, err)
	assert.Equal(t, stdhttp.StatusCreated, resp.StatusCode)
	assert.Equal(t, "Hello", received.Title)
}

func TestClient_Put_SendsPUTRequest(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		assert.Equal(t, "PUT", r.Method)
		w.WriteHeader(stdhttp.StatusOK)
	}))
	defer server.Close()

	client := NewClient(DefaultConfig(server.URL))
	resp, err := client.Put("/items/1", map[string]string{"name": "updated"})

	require.NoError(t, err)
	assert.Equal(t, stdhttp.StatusOK, resp.StatusCode)
}

func TestClient_Patch_SendsPATCHRequest(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		assert.Equal(t, "PATCH", r.Method)
		w.WriteHeader(stdhttp.StatusOK)
	}))
	defer server.Close()

	client := NewClient(DefaultConfig(server.URL))
	resp, err := client.Patch("/items/1", map[string]string{"status": "inactive"})

	require.NoError(t, err)
	assert.Equal(t, stdhttp.StatusOK, resp.StatusCode)
}

func TestClient_Delete_SendsDELETERequest(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/items/99", r.URL.Path)
		w.WriteHeader(stdhttp.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(DefaultConfig(server.URL))
	resp, err := client.Delete("/items/99")

	require.NoError(t, err)
	assert.Equal(t, stdhttp.StatusNoContent, resp.StatusCode)
}

// -----------------------------------------------------------------------------
// RetryableClient.Do
// -----------------------------------------------------------------------------

func TestRetryableClient_Do_SucceedsOnFirstAttempt(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		callCount++
		w.WriteHeader(stdhttp.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	cfg := DefaultConfig(server.URL)
	cfg.MaxRetries = 3
	cfg.RetryDelay = 1 * time.Millisecond // short delay to keep the test fast

	rc := NewRetryableClient(cfg)
	resp, err := rc.Do(Request{Method: "GET", Path: "/"})

	require.NoError(t, err)
	assert.Equal(t, stdhttp.StatusOK, resp.StatusCode)
	assert.Equal(t, 1, callCount, "Should only make 1 call on immediate success")
}

func TestRetryableClient_Do_RetriesOnServerError(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		callCount++
		if callCount < 3 {
			// Fail the first two attempts with a 5xx so the client retries.
			w.WriteHeader(stdhttp.StatusInternalServerError)
			return
		}
		// Succeed on the third attempt.
		w.WriteHeader(stdhttp.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	cfg := DefaultConfig(server.URL)
	cfg.MaxRetries = 3
	cfg.RetryDelay = 1 * time.Millisecond

	rc := NewRetryableClient(cfg)
	resp, err := rc.Do(Request{Method: "GET", Path: "/"})

	require.NoError(t, err)
	assert.Equal(t, stdhttp.StatusOK, resp.StatusCode)
	assert.Equal(t, 3, callCount, "Should have made exactly 3 calls")
}

func TestRetryableClient_Do_ExhaustsMaxRetries(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		callCount++
		w.WriteHeader(stdhttp.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := DefaultConfig(server.URL)
	cfg.MaxRetries = 2
	cfg.RetryDelay = 1 * time.Millisecond

	rc := NewRetryableClient(cfg)
	// We only assert that all attempts were made; the exact return values
	// depend on whether lastErr is nil (transport succeeded but 5xx) vs
	// non-nil (transport error).  Either way, MaxRetries+1 calls must happen.
	_, _ = rc.Do(Request{Method: "GET", Path: "/"})

	assert.Equal(t, cfg.MaxRetries+1, callCount,
		"Should attempt initial request + MaxRetries retries")
}

func TestRetryableClient_Do_DoesNotRetryOn4xx(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		callCount++
		w.WriteHeader(stdhttp.StatusNotFound)
	}))
	defer server.Close()

	cfg := DefaultConfig(server.URL)
	cfg.MaxRetries = 3
	cfg.RetryDelay = 1 * time.Millisecond

	rc := NewRetryableClient(cfg)
	resp, err := rc.Do(Request{Method: "GET", Path: "/"})

	require.NoError(t, err)
	assert.Equal(t, stdhttp.StatusNotFound, resp.StatusCode)
	assert.Equal(t, 1, callCount, "Should not retry on 4xx responses")
}
