package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// CoalesceConfig holds configuration for request coalescing
type CoalesceConfig struct {
	// Enabled controls whether coalescing is active
	Enabled bool

	// MaxWaitTime is the maximum time to wait for a coalesced result
	MaxWaitTime time.Duration

	// ExcludedPaths are paths that should not be coalesced
	ExcludedPaths []string

	// ExcludedHeaders are headers to exclude from the coalesce key
	ExcludedHeaders []string

	// IncludeTenantID includes X-Tenant-ID in the coalesce key (recommended)
	IncludeTenantID bool

	// IncludeUserID includes X-User-ID in the coalesce key
	IncludeUserID bool

	// CacheResult caches the result for a short duration after coalescing
	CacheResult bool

	// CacheTTL is how long to cache coalesced results
	CacheTTL time.Duration
}

// DefaultCoalesceConfig returns default coalescing configuration
func DefaultCoalesceConfig() CoalesceConfig {
	return CoalesceConfig{
		Enabled:     true,
		MaxWaitTime: 100 * time.Millisecond,
		ExcludedPaths: []string{
			"/health",
			"/ready",
			"/metrics",
		},
		ExcludedHeaders: []string{
			"Authorization",
			"Cookie",
			"X-Request-ID",
			"X-Correlation-ID",
		},
		IncludeTenantID: true,
		IncludeUserID:   false,
		CacheResult:     true,
		CacheTTL:        50 * time.Millisecond,
	}
}

// coalesceResult holds the result of a coalesced request
type coalesceResult struct {
	statusCode int
	headers    http.Header
	body       []byte
	ready      chan struct{}
	err        error
}

// resultCache holds cached results with expiration
type resultCache struct {
	result    *coalesceResult
	expiresAt time.Time
}

// RequestCoalescer deduplicates concurrent identical GET requests
type RequestCoalescer struct {
	config   CoalesceConfig
	inflight sync.Map // key -> *coalesceResult
	cache    sync.Map // key -> *resultCache
	mu       sync.Mutex
}

// NewRequestCoalescer creates a new request coalescer
func NewRequestCoalescer(config CoalesceConfig) *RequestCoalescer {
	c := &RequestCoalescer{
		config: config,
	}

	// Start cache cleanup goroutine
	if config.CacheResult {
		go c.cleanupLoop()
	}

	return c
}

// cleanupLoop periodically removes expired cache entries
func (c *RequestCoalescer) cleanupLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		c.cache.Range(func(key, value interface{}) bool {
			if cached, ok := value.(*resultCache); ok {
				if now.After(cached.expiresAt) {
					c.cache.Delete(key)
				}
			}
			return true
		})
	}
}

// buildKey creates a unique key for request coalescing
func (c *RequestCoalescer) buildKey(r *http.Request) string {
	var keyParts []string

	// Method and path
	keyParts = append(keyParts, r.Method)
	keyParts = append(keyParts, r.URL.Path)

	// Query parameters (sorted for consistency)
	if r.URL.RawQuery != "" {
		queryParams := make([]string, 0)
		for key, values := range r.URL.Query() {
			for _, value := range values {
				queryParams = append(queryParams, key+"="+value)
			}
		}
		sort.Strings(queryParams)
		keyParts = append(keyParts, strings.Join(queryParams, "&"))
	}

	// Include tenant ID if configured
	if c.config.IncludeTenantID {
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			tenantID = r.Header.Get("X-Vendor-ID")
		}
		if tenantID != "" {
			keyParts = append(keyParts, "t:"+tenantID)
		}
	}

	// Include user ID if configured
	if c.config.IncludeUserID {
		userID := r.Header.Get("X-User-ID")
		if userID != "" {
			keyParts = append(keyParts, "u:"+userID)
		}
	}

	// Hash the key to keep it compact
	keyStr := strings.Join(keyParts, "|")
	hash := sha256.Sum256([]byte(keyStr))
	return hex.EncodeToString(hash[:16]) // First 16 bytes = 32 hex chars
}

// isExcludedPath checks if the path should bypass coalescing
func (c *RequestCoalescer) isExcludedPath(path string) bool {
	for _, excluded := range c.config.ExcludedPaths {
		if path == excluded || strings.HasPrefix(path, excluded) {
			return true
		}
	}
	return false
}

// responseWriter captures the response for coalescing
type coalesceResponseWriter struct {
	gin.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func newCoalesceResponseWriter(w gin.ResponseWriter) *coalesceResponseWriter {
	return &coalesceResponseWriter{
		ResponseWriter: w,
		body:           &bytes.Buffer{},
		statusCode:     http.StatusOK,
	}
}

func (w *coalesceResponseWriter) Write(data []byte) (int, error) {
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

func (w *coalesceResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// Middleware returns a Gin middleware for request coalescing
func (c *RequestCoalescer) Middleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// Only coalesce GET requests
		if ctx.Request.Method != http.MethodGet || !c.config.Enabled {
			ctx.Next()
			return
		}

		// Skip excluded paths
		if c.isExcludedPath(ctx.Request.URL.Path) {
			ctx.Next()
			return
		}

		// Build unique key for this request
		key := c.buildKey(ctx.Request)

		// Check result cache first
		if c.config.CacheResult {
			if cached, ok := c.cache.Load(key); ok {
				if rc, ok := cached.(*resultCache); ok && time.Now().Before(rc.expiresAt) {
					// Serve from cache
					for k, v := range rc.result.headers {
						for _, vv := range v {
							ctx.Header(k, vv)
						}
					}
					ctx.Header("X-Coalesce-Cache", "HIT")
					ctx.Data(rc.result.statusCode, ctx.Writer.Header().Get("Content-Type"), rc.result.body)
					ctx.Abort()
					return
				}
			}
		}

		// Try to join an existing in-flight request
		result := &coalesceResult{
			ready: make(chan struct{}),
		}

		actual, loaded := c.inflight.LoadOrStore(key, result)
		if loaded {
			// Another request is already in flight, wait for it
			existingResult := actual.(*coalesceResult)
			select {
			case <-existingResult.ready:
				// Serve the coalesced result
				if existingResult.err == nil {
					for k, v := range existingResult.headers {
						for _, vv := range v {
							ctx.Header(k, vv)
						}
					}
					ctx.Header("X-Coalesce", "JOINED")
					ctx.Data(existingResult.statusCode, ctx.Writer.Header().Get("Content-Type"), existingResult.body)
					ctx.Abort()
					return
				}
				// If there was an error, fall through and make our own request
			case <-time.After(c.config.MaxWaitTime):
				// Timeout waiting, proceed with our own request
				ctx.Header("X-Coalesce", "TIMEOUT")
			}
			ctx.Next()
			return
		}

		// We're the leader - execute the request
		crw := newCoalesceResponseWriter(ctx.Writer)
		ctx.Writer = crw

		ctx.Next()

		// Capture the result
		result.statusCode = crw.statusCode
		result.body = crw.body.Bytes()
		result.headers = make(http.Header)
		for k, v := range crw.Header() {
			result.headers[k] = v
		}

		// Signal that result is ready
		close(result.ready)

		// Clean up in-flight and optionally cache
		c.inflight.Delete(key)

		// Cache successful responses
		if c.config.CacheResult && result.statusCode >= 200 && result.statusCode < 300 {
			c.cache.Store(key, &resultCache{
				result:    result,
				expiresAt: time.Now().Add(c.config.CacheTTL),
			})
		}

		ctx.Header("X-Coalesce", "LEADER")
	}
}

// CoalesceMiddleware creates a request coalescing middleware with default configuration
func CoalesceMiddleware() gin.HandlerFunc {
	coalescer := NewRequestCoalescer(DefaultCoalesceConfig())
	return coalescer.Middleware()
}

// CoalesceMiddlewareWithConfig creates a request coalescing middleware with custom configuration
func CoalesceMiddlewareWithConfig(config CoalesceConfig) gin.HandlerFunc {
	coalescer := NewRequestCoalescer(config)
	return coalescer.Middleware()
}

// SingleFlight provides a simpler interface for deduplicating expensive operations
// (not HTTP-specific, can be used for any expensive computation)
type SingleFlight struct {
	mu      sync.Mutex
	calls   map[string]*singleFlightCall
	results sync.Map // key -> *singleFlightResult
	resultTTL time.Duration
}

type singleFlightCall struct {
	wg     sync.WaitGroup
	result interface{}
	err    error
}

type singleFlightResult struct {
	value     interface{}
	err       error
	expiresAt time.Time
}

// NewSingleFlight creates a new single-flight deduplicator
func NewSingleFlight(resultTTL time.Duration) *SingleFlight {
	sf := &SingleFlight{
		calls:     make(map[string]*singleFlightCall),
		resultTTL: resultTTL,
	}

	// Start cleanup goroutine
	if resultTTL > 0 {
		go sf.cleanupLoop()
	}

	return sf
}

func (sf *SingleFlight) cleanupLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		sf.results.Range(func(key, value interface{}) bool {
			if cached, ok := value.(*singleFlightResult); ok {
				if now.After(cached.expiresAt) {
					sf.results.Delete(key)
				}
			}
			return true
		})
	}
}

// Do executes fn only once for a given key, sharing the result with all callers
func (sf *SingleFlight) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	// Check result cache first
	if sf.resultTTL > 0 {
		if cached, ok := sf.results.Load(key); ok {
			if result, ok := cached.(*singleFlightResult); ok && time.Now().Before(result.expiresAt) {
				return result.value, result.err
			}
		}
	}

	sf.mu.Lock()

	// Check if there's an in-flight call
	if call, ok := sf.calls[key]; ok {
		sf.mu.Unlock()
		call.wg.Wait()
		return call.result, call.err
	}

	// Create a new call
	call := &singleFlightCall{}
	call.wg.Add(1)
	sf.calls[key] = call
	sf.mu.Unlock()

	// Execute the function
	call.result, call.err = fn()
	call.wg.Done()

	// Clean up and cache
	sf.mu.Lock()
	delete(sf.calls, key)
	sf.mu.Unlock()

	if sf.resultTTL > 0 && call.err == nil {
		sf.results.Store(key, &singleFlightResult{
			value:     call.result,
			err:       call.err,
			expiresAt: time.Now().Add(sf.resultTTL),
		})
	}

	return call.result, call.err
}

// DoBytes is a convenience method for byte slice results
func (sf *SingleFlight) DoBytes(key string, fn func() ([]byte, error)) ([]byte, error) {
	result, err := sf.Do(key, func() (interface{}, error) {
		return fn()
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return result.([]byte), nil
}

// BatchCoalescer coalesces multiple individual requests into batch requests
type BatchCoalescer struct {
	batchSize     int
	batchTimeout  time.Duration
	mu            sync.Mutex
	pending       map[string][]batchRequest
	batchHandlers map[string]BatchHandler
}

type batchRequest struct {
	id       string
	response chan interface{}
}

// BatchHandler processes a batch of IDs and returns a map of results
type BatchHandler func(ids []string) (map[string]interface{}, error)

// NewBatchCoalescer creates a new batch coalescer
func NewBatchCoalescer(batchSize int, batchTimeout time.Duration) *BatchCoalescer {
	return &BatchCoalescer{
		batchSize:     batchSize,
		batchTimeout:  batchTimeout,
		pending:       make(map[string][]batchRequest),
		batchHandlers: make(map[string]BatchHandler),
	}
}

// RegisterHandler registers a batch handler for a resource type
func (bc *BatchCoalescer) RegisterHandler(resourceType string, handler BatchHandler) {
	bc.mu.Lock()
	bc.batchHandlers[resourceType] = handler
	bc.mu.Unlock()
}

// Get retrieves a single item, automatically batching with other concurrent requests
func (bc *BatchCoalescer) Get(resourceType, id string) (interface{}, error) {
	responseChan := make(chan interface{}, 1)

	bc.mu.Lock()
	handler, ok := bc.batchHandlers[resourceType]
	if !ok {
		bc.mu.Unlock()
		return nil, io.EOF // No handler registered
	}

	// Add to pending batch
	bc.pending[resourceType] = append(bc.pending[resourceType], batchRequest{
		id:       id,
		response: responseChan,
	})

	// Check if we should trigger a batch
	pendingCount := len(bc.pending[resourceType])
	if pendingCount >= bc.batchSize {
		// Trigger batch immediately
		bc.executeBatch(resourceType, handler)
	} else if pendingCount == 1 {
		// First request - start timer
		go func() {
			time.Sleep(bc.batchTimeout)
			bc.mu.Lock()
			if len(bc.pending[resourceType]) > 0 {
				bc.executeBatch(resourceType, handler)
			}
			bc.mu.Unlock()
		}()
	}
	bc.mu.Unlock()

	// Wait for result
	result := <-responseChan
	return result, nil
}

func (bc *BatchCoalescer) executeBatch(resourceType string, handler BatchHandler) {
	requests := bc.pending[resourceType]
	bc.pending[resourceType] = nil

	if len(requests) == 0 {
		return
	}

	// Collect unique IDs
	ids := make([]string, 0, len(requests))
	for _, req := range requests {
		ids = append(ids, req.id)
	}

	// Execute batch handler
	go func() {
		results, err := handler(ids)
		if err != nil {
			for _, req := range requests {
				req.response <- nil
			}
			return
		}

		for _, req := range requests {
			if result, ok := results[req.id]; ok {
				req.response <- result
			} else {
				req.response <- nil
			}
		}
	}()
}
