package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tesserix/go-shared/logger"
	"github.com/tesserix/go-shared/metrics"
)

// Client wraps http.Client with additional functionality
type Client struct {
	httpClient *http.Client
	baseURL    string
	headers    map[string]string
	logger     *logger.Logger
	metrics    *metrics.Metrics
	timeout    time.Duration
}

// Config holds HTTP client configuration
type Config struct {
	BaseURL        string
	Timeout        time.Duration
	MaxRetries     int
	RetryDelay     time.Duration
	DefaultHeaders map[string]string
	Logger         *logger.Logger
	Metrics        *metrics.Metrics
}

// DefaultConfig returns a default HTTP client configuration
func DefaultConfig(baseURL string) Config {
	return Config{
		BaseURL:    baseURL,
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		RetryDelay: 1 * time.Second,
		DefaultHeaders: map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
			"User-Agent":   "tesseract-hub-client/1.0",
		},
	}
}

// NewClient creates a new HTTP client
func NewClient(config Config) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		baseURL: strings.TrimRight(config.BaseURL, "/"),
		headers: config.DefaultHeaders,
		logger:  config.Logger,
		metrics: config.Metrics,
		timeout: config.Timeout,
	}
}

// Request represents an HTTP request
type Request struct {
	Method      string
	Path        string
	Body        interface{}
	Headers     map[string]string
	QueryParams map[string]string
	Context     context.Context
}

// Response represents an HTTP response
type Response struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
	Duration   time.Duration
}

// IsSuccess returns true if the response indicates success
func (r *Response) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// IsClientError returns true if the response indicates a client error
func (r *Response) IsClientError() bool {
	return r.StatusCode >= 400 && r.StatusCode < 500
}

// IsServerError returns true if the response indicates a server error
func (r *Response) IsServerError() bool {
	return r.StatusCode >= 500
}

// JSON unmarshals the response body as JSON
func (r *Response) JSON(v interface{}) error {
	return json.Unmarshal(r.Body, v)
}

// String returns the response body as a string
func (r *Response) String() string {
	return string(r.Body)
}

// SetHeader sets a default header for all requests
func (c *Client) SetHeader(key, value string) {
	if c.headers == nil {
		c.headers = make(map[string]string)
	}
	c.headers[key] = value
}

// SetAuthToken sets the Authorization header with a Bearer token
func (c *Client) SetAuthToken(token string) {
	c.SetHeader("Authorization", "Bearer "+token)
}

// SetBasicAuth sets basic authentication
func (c *Client) SetBasicAuth(username, password string) {
	c.httpClient.Transport = &basicAuthTransport{
		username: username,
		password: password,
		base:     http.DefaultTransport,
	}
}

// Do executes an HTTP request
func (c *Client) Do(req Request) (*Response, error) {
	start := time.Now()
	
	// Build URL
	requestURL := c.buildURL(req.Path, req.QueryParams)
	
	// Prepare request body
	var bodyReader io.Reader
	var bodyBytes []byte
	
	if req.Body != nil {
		switch body := req.Body.(type) {
		case string:
			bodyBytes = []byte(body)
		case []byte:
			bodyBytes = body
		default:
			var err error
			bodyBytes, err = json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal request body: %w", err)
			}
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}
	
	// Create HTTP request
	httpReq, err := http.NewRequest(req.Method, requestURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set context
	if req.Context != nil {
		httpReq = httpReq.WithContext(req.Context)
	}
	
	// Set headers
	for key, value := range c.headers {
		httpReq.Header.Set(key, value)
	}
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}
	
	// Log request
	if c.logger != nil {
		c.logger.WithFields(map[string]interface{}{
			"method":      req.Method,
			"url":         requestURL,
			"body_size":   len(bodyBytes),
		}).Info("HTTP request")
	}
	
	// Execute request
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		duration := time.Since(start)
		
		// Log error
		if c.logger != nil {
			c.logger.WithError(err).WithFields(map[string]interface{}{
				"method":      req.Method,
				"url":         requestURL,
				"duration_ms": duration.Milliseconds(),
			}).Error("HTTP request failed")
		}
		
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()
	
	// Read response body
	responseBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	duration := time.Since(start)
	
	response := &Response{
		StatusCode: httpResp.StatusCode,
		Body:       responseBody,
		Headers:    httpResp.Header,
		Duration:   duration,
	}
	
	// Log response
	if c.logger != nil {
		logLevel := "info"
		if response.IsClientError() {
			logLevel = "warn"
		} else if response.IsServerError() {
			logLevel = "error"
		}
		
		c.logger.WithFields(map[string]interface{}{
			"method":        req.Method,
			"url":           requestURL,
			"status_code":   response.StatusCode,
			"duration_ms":   duration.Milliseconds(),
			"response_size": len(responseBody),
		}).Info(fmt.Sprintf("HTTP response [%s]", logLevel))
	}
	
	// Record metrics
	if c.metrics != nil {
		c.metrics.RecordHTTPRequest(
			req.Method,
			req.Path,
			fmt.Sprintf("%d", response.StatusCode),
			duration,
			int64(len(bodyBytes)),
			int64(len(responseBody)),
		)
	}
	
	return response, nil
}

// Convenience methods

// Get performs a GET request
func (c *Client) Get(path string, queryParams map[string]string) (*Response, error) {
	return c.Do(Request{
		Method:      http.MethodGet,
		Path:        path,
		QueryParams: queryParams,
	})
}

// GetWithContext performs a GET request with context
func (c *Client) GetWithContext(ctx context.Context, path string, queryParams map[string]string) (*Response, error) {
	return c.Do(Request{
		Method:      http.MethodGet,
		Path:        path,
		QueryParams: queryParams,
		Context:     ctx,
	})
}

// Post performs a POST request
func (c *Client) Post(path string, body interface{}) (*Response, error) {
	return c.Do(Request{
		Method: http.MethodPost,
		Path:   path,
		Body:   body,
	})
}

// PostWithContext performs a POST request with context
func (c *Client) PostWithContext(ctx context.Context, path string, body interface{}) (*Response, error) {
	return c.Do(Request{
		Method:  http.MethodPost,
		Path:    path,
		Body:    body,
		Context: ctx,
	})
}

// Put performs a PUT request
func (c *Client) Put(path string, body interface{}) (*Response, error) {
	return c.Do(Request{
		Method: http.MethodPut,
		Path:   path,
		Body:   body,
	})
}

// PutWithContext performs a PUT request with context
func (c *Client) PutWithContext(ctx context.Context, path string, body interface{}) (*Response, error) {
	return c.Do(Request{
		Method:  http.MethodPut,
		Path:    path,
		Body:    body,
		Context: ctx,
	})
}

// Patch performs a PATCH request
func (c *Client) Patch(path string, body interface{}) (*Response, error) {
	return c.Do(Request{
		Method: http.MethodPatch,
		Path:   path,
		Body:   body,
	})
}

// PatchWithContext performs a PATCH request with context
func (c *Client) PatchWithContext(ctx context.Context, path string, body interface{}) (*Response, error) {
	return c.Do(Request{
		Method:  http.MethodPatch,
		Path:    path,
		Body:    body,
		Context: ctx,
	})
}

// Delete performs a DELETE request
func (c *Client) Delete(path string) (*Response, error) {
	return c.Do(Request{
		Method: http.MethodDelete,
		Path:   path,
	})
}

// DeleteWithContext performs a DELETE request with context
func (c *Client) DeleteWithContext(ctx context.Context, path string) (*Response, error) {
	return c.Do(Request{
		Method:  http.MethodDelete,
		Path:    path,
		Context: ctx,
	})
}

// Helper methods

// buildURL builds the complete URL
func (c *Client) buildURL(path string, queryParams map[string]string) string {
	fullURL := c.baseURL + "/" + strings.TrimLeft(path, "/")
	
	if len(queryParams) > 0 {
		params := url.Values{}
		for key, value := range queryParams {
			params.Add(key, value)
		}
		fullURL += "?" + params.Encode()
	}
	
	return fullURL
}

// Service-specific clients

// AuthClient provides authentication service client
type AuthClient struct {
	*Client
}

// NewAuthClient creates a new authentication service client
func NewAuthClient(baseURL string, logger *logger.Logger, metrics *metrics.Metrics) *AuthClient {
	config := DefaultConfig(baseURL)
	config.Logger = logger
	config.Metrics = metrics
	
	return &AuthClient{
		Client: NewClient(config),
	}
}

// Login authenticates a user
func (ac *AuthClient) Login(email, password string) (*Response, error) {
	return ac.Post("/login", map[string]string{
		"email":    email,
		"password": password,
	})
}

// ValidateToken validates a JWT token
func (ac *AuthClient) ValidateToken(token string) (*Response, error) {
	return ac.Post("/validate", map[string]string{
		"token": token,
	})
}

// DocumentClient provides document service client
type DocumentClient struct {
	*Client
}

// NewDocumentClient creates a new document service client
func NewDocumentClient(baseURL string, logger *logger.Logger, metrics *metrics.Metrics) *DocumentClient {
	config := DefaultConfig(baseURL)
	config.Logger = logger
	config.Metrics = metrics
	
	return &DocumentClient{
		Client: NewClient(config),
	}
}

// UploadDocument uploads a document
func (dc *DocumentClient) UploadDocument(entityType, entityID string, document interface{}) (*Response, error) {
	path := fmt.Sprintf("/%s/%s/documents", entityType, entityID)
	return dc.Post(path, document)
}

// GetDocuments retrieves documents for an entity
func (dc *DocumentClient) GetDocuments(entityType, entityID string) (*Response, error) {
	path := fmt.Sprintf("/%s/%s/documents", entityType, entityID)
	return dc.Get(path, nil)
}

// Middleware and utilities

// basicAuthTransport implements basic authentication
type basicAuthTransport struct {
	username string
	password string
	base     http.RoundTripper
}

func (t *basicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.SetBasicAuth(t.username, t.password)
	return t.base.RoundTrip(req)
}

// RetryableClient wraps Client with retry logic
type RetryableClient struct {
	*Client
	maxRetries int
	retryDelay time.Duration
}

// NewRetryableClient creates a client with retry logic
func NewRetryableClient(config Config) *RetryableClient {
	return &RetryableClient{
		Client:     NewClient(config),
		maxRetries: config.MaxRetries,
		retryDelay: config.RetryDelay,
	}
}

// Do executes a request with retry logic
func (rc *RetryableClient) Do(req Request) (*Response, error) {
	var lastErr error
	
	for attempt := 0; attempt <= rc.maxRetries; attempt++ {
		resp, err := rc.Client.Do(req)
		
		if err == nil && !resp.IsServerError() {
			return resp, nil
		}
		
		lastErr = err
		
		if attempt < rc.maxRetries {
			if rc.logger != nil {
				rc.logger.WithFields(map[string]interface{}{
					"attempt":     attempt + 1,
					"max_retries": rc.maxRetries,
					"delay_ms":    rc.retryDelay.Milliseconds(),
				}).Warn("Retrying HTTP request")
			}
			
			time.Sleep(rc.retryDelay)
		}
	}
	
	return nil, fmt.Errorf("request failed after %d retries: %w", rc.maxRetries, lastErr)
}