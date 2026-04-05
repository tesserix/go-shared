package httpclient

import (
	"context"
	"crypto/tls"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// -----------------------------------------------------------------------------
// DefaultConfig
// -----------------------------------------------------------------------------

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, 30*time.Second, cfg.Timeout, "Timeout should be 30s")
	assert.Equal(t, 100, cfg.MaxIdleConns, "MaxIdleConns should be 100")
	assert.Equal(t, 10, cfg.MaxIdleConnsPerHost, "MaxIdleConnsPerHost should be 10")
	assert.Equal(t, 100, cfg.MaxConnsPerHost, "MaxConnsPerHost should be 100")
	assert.Equal(t, 90*time.Second, cfg.IdleConnTimeout, "IdleConnTimeout should be 90s")
	assert.Equal(t, 5*time.Second, cfg.DialTimeout, "DialTimeout should be 5s")
	assert.Equal(t, 30*time.Second, cfg.KeepAlive, "KeepAlive should be 30s")
	assert.Equal(t, 5*time.Second, cfg.TLSHandshakeTimeout, "TLSHandshakeTimeout should be 5s")
	assert.Equal(t, 10*time.Second, cfg.ResponseHeaderTimeout, "ResponseHeaderTimeout should be 10s")
	assert.Equal(t, 1*time.Second, cfg.ExpectContinueTimeout, "ExpectContinueTimeout should be 1s")
	assert.False(t, cfg.DisableKeepAlives, "DisableKeepAlives should be false")
	assert.False(t, cfg.DisableCompression, "DisableCompression should be false")
	assert.True(t, cfg.ForceAttemptHTTP2, "ForceAttemptHTTP2 should be true")
}

// -----------------------------------------------------------------------------
// ProfileConfigs
// -----------------------------------------------------------------------------

func TestProfileConfigs_ReturnsAllFiveProfiles(t *testing.T) {
	profiles := ProfileConfigs()

	expectedProfiles := []ClientProfile{
		ProfileDefault,
		ProfileHighThroughput,
		ProfileLowLatency,
		ProfileBatch,
		ProfileExternal,
	}

	assert.Len(t, profiles, len(expectedProfiles), "ProfileConfigs should return exactly 5 profiles")

	for _, p := range expectedProfiles {
		_, ok := profiles[p]
		assert.True(t, ok, "Profile %q should be present in ProfileConfigs", p)
	}
}

func TestProfileConfigs_DefaultProfile(t *testing.T) {
	cfg := ProfileConfigs()[ProfileDefault]

	assert.Equal(t, 30*time.Second, cfg.Timeout)
	assert.Equal(t, 100, cfg.MaxIdleConns)
	assert.Equal(t, 10, cfg.MaxIdleConnsPerHost)
	assert.Equal(t, 100, cfg.MaxConnsPerHost)
	assert.Equal(t, 90*time.Second, cfg.IdleConnTimeout)
	assert.Equal(t, 5*time.Second, cfg.DialTimeout)
	assert.Equal(t, 30*time.Second, cfg.KeepAlive)
	assert.Equal(t, 5*time.Second, cfg.TLSHandshakeTimeout)
	assert.Equal(t, 10*time.Second, cfg.ResponseHeaderTimeout)
	assert.Equal(t, 1*time.Second, cfg.ExpectContinueTimeout)
	assert.True(t, cfg.ForceAttemptHTTP2)
}

func TestProfileConfigs_HighThroughputProfile(t *testing.T) {
	cfg := ProfileConfigs()[ProfileHighThroughput]

	assert.Equal(t, 15*time.Second, cfg.Timeout)
	assert.Equal(t, 500, cfg.MaxIdleConns)
	assert.Equal(t, 50, cfg.MaxIdleConnsPerHost)
	assert.Equal(t, 500, cfg.MaxConnsPerHost)
	assert.Equal(t, 120*time.Second, cfg.IdleConnTimeout)
	assert.Equal(t, 3*time.Second, cfg.DialTimeout)
	assert.Equal(t, 30*time.Second, cfg.KeepAlive)
	assert.Equal(t, 3*time.Second, cfg.TLSHandshakeTimeout)
	assert.Equal(t, 5*time.Second, cfg.ResponseHeaderTimeout)
	assert.Equal(t, 500*time.Millisecond, cfg.ExpectContinueTimeout)
	assert.True(t, cfg.ForceAttemptHTTP2)
}

func TestProfileConfigs_LowLatencyProfile(t *testing.T) {
	cfg := ProfileConfigs()[ProfileLowLatency]

	assert.Equal(t, 5*time.Second, cfg.Timeout)
	assert.Equal(t, 200, cfg.MaxIdleConns)
	assert.Equal(t, 30, cfg.MaxIdleConnsPerHost)
	assert.Equal(t, 200, cfg.MaxConnsPerHost)
	assert.Equal(t, 60*time.Second, cfg.IdleConnTimeout)
	assert.Equal(t, 2*time.Second, cfg.DialTimeout)
	assert.Equal(t, 15*time.Second, cfg.KeepAlive)
	assert.Equal(t, 2*time.Second, cfg.TLSHandshakeTimeout)
	assert.Equal(t, 3*time.Second, cfg.ResponseHeaderTimeout)
	assert.Equal(t, 500*time.Millisecond, cfg.ExpectContinueTimeout)
	assert.True(t, cfg.ForceAttemptHTTP2)
}

func TestProfileConfigs_BatchProfile(t *testing.T) {
	cfg := ProfileConfigs()[ProfileBatch]

	assert.Equal(t, 120*time.Second, cfg.Timeout)
	assert.Equal(t, 50, cfg.MaxIdleConns)
	assert.Equal(t, 10, cfg.MaxIdleConnsPerHost)
	assert.Equal(t, 50, cfg.MaxConnsPerHost)
	assert.Equal(t, 120*time.Second, cfg.IdleConnTimeout)
	assert.Equal(t, 10*time.Second, cfg.DialTimeout)
	assert.Equal(t, 60*time.Second, cfg.KeepAlive)
	assert.Equal(t, 10*time.Second, cfg.TLSHandshakeTimeout)
	assert.Equal(t, 60*time.Second, cfg.ResponseHeaderTimeout)
	assert.Equal(t, 2*time.Second, cfg.ExpectContinueTimeout)
	assert.True(t, cfg.ForceAttemptHTTP2)
}

func TestProfileConfigs_ExternalProfile(t *testing.T) {
	cfg := ProfileConfigs()[ProfileExternal]

	assert.Equal(t, 60*time.Second, cfg.Timeout)
	assert.Equal(t, 50, cfg.MaxIdleConns)
	assert.Equal(t, 5, cfg.MaxIdleConnsPerHost)
	assert.Equal(t, 20, cfg.MaxConnsPerHost)
	assert.Equal(t, 60*time.Second, cfg.IdleConnTimeout)
	assert.Equal(t, 10*time.Second, cfg.DialTimeout)
	assert.Equal(t, 30*time.Second, cfg.KeepAlive)
	assert.Equal(t, 10*time.Second, cfg.TLSHandshakeTimeout)
	assert.Equal(t, 30*time.Second, cfg.ResponseHeaderTimeout)
	assert.Equal(t, 2*time.Second, cfg.ExpectContinueTimeout)
	assert.True(t, cfg.ForceAttemptHTTP2)
}

// -----------------------------------------------------------------------------
// GetProfileConfig
// -----------------------------------------------------------------------------

func TestGetProfileConfig_KnownProfiles(t *testing.T) {
	tests := []struct {
		name            string
		profile         ClientProfile
		expectedTimeout time.Duration
	}{
		{"default", ProfileDefault, 30 * time.Second},
		{"high_throughput", ProfileHighThroughput, 15 * time.Second},
		{"low_latency", ProfileLowLatency, 5 * time.Second},
		{"batch", ProfileBatch, 120 * time.Second},
		{"external", ProfileExternal, 60 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := GetProfileConfig(tt.profile)
			assert.Equal(t, tt.expectedTimeout, cfg.Timeout,
				"Profile %q should have timeout %v", tt.profile, tt.expectedTimeout)
		})
	}
}

func TestGetProfileConfig_UnknownProfileFallsBackToDefault(t *testing.T) {
	cfg := GetProfileConfig("nonexistent_profile")
	defaultCfg := DefaultConfig()

	assert.Equal(t, defaultCfg.Timeout, cfg.Timeout)
	assert.Equal(t, defaultCfg.MaxIdleConns, cfg.MaxIdleConns)
	assert.Equal(t, defaultCfg.MaxIdleConnsPerHost, cfg.MaxIdleConnsPerHost)
}

// -----------------------------------------------------------------------------
// NewClient
// -----------------------------------------------------------------------------

func TestNewClient_CreatesClientWithCorrectTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{"30s timeout", 30 * time.Second},
		{"5s timeout", 5 * time.Second},
		{"2m timeout", 2 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Timeout = tt.timeout

			client := NewClient(cfg)

			assert.NotNil(t, client)
			assert.Equal(t, tt.timeout, client.Timeout)
		})
	}
}

func TestNewClient_HasTransport(t *testing.T) {
	cfg := DefaultConfig()
	client := NewClient(cfg)

	assert.NotNil(t, client.Transport, "client should have a non-nil transport")
}

// -----------------------------------------------------------------------------
// NewClientWithProfile
// -----------------------------------------------------------------------------

func TestNewClientWithProfile_CreatesClientWithProfileTimeout(t *testing.T) {
	tests := []struct {
		profile         ClientProfile
		expectedTimeout time.Duration
	}{
		{ProfileDefault, 30 * time.Second},
		{ProfileHighThroughput, 15 * time.Second},
		{ProfileLowLatency, 5 * time.Second},
		{ProfileBatch, 120 * time.Second},
		{ProfileExternal, 60 * time.Second},
	}

	for _, tt := range tests {
		t.Run(string(tt.profile), func(t *testing.T) {
			client := NewClientWithProfile(tt.profile)

			assert.NotNil(t, client)
			assert.Equal(t, tt.expectedTimeout, client.Timeout)
		})
	}
}

// -----------------------------------------------------------------------------
// NewTransport
// -----------------------------------------------------------------------------

func TestNewTransport_TLSMinVersionIsTLS12(t *testing.T) {
	cfg := DefaultConfig()
	transport := NewTransport(cfg)

	assert.NotNil(t, transport.TLSClientConfig)
	assert.Equal(t, uint16(tls.VersionTLS12), transport.TLSClientConfig.MinVersion,
		"TLS minimum version should be TLS 1.2")
}

func TestNewTransport_ConnectionPoolSettings(t *testing.T) {
	cfg := DefaultConfig()
	transport := NewTransport(cfg)

	assert.Equal(t, cfg.MaxIdleConns, transport.MaxIdleConns)
	assert.Equal(t, cfg.MaxIdleConnsPerHost, transport.MaxIdleConnsPerHost)
	assert.Equal(t, cfg.MaxConnsPerHost, transport.MaxConnsPerHost)
	assert.Equal(t, cfg.IdleConnTimeout, transport.IdleConnTimeout)
}

func TestNewTransport_TimeoutSettings(t *testing.T) {
	cfg := DefaultConfig()
	transport := NewTransport(cfg)

	assert.Equal(t, cfg.TLSHandshakeTimeout, transport.TLSHandshakeTimeout)
	assert.Equal(t, cfg.ResponseHeaderTimeout, transport.ResponseHeaderTimeout)
	assert.Equal(t, cfg.ExpectContinueTimeout, transport.ExpectContinueTimeout)
}

func TestNewTransport_BooleanFlags(t *testing.T) {
	// With defaults (DisableKeepAlives=false, DisableCompression=false, ForceAttemptHTTP2=true)
	cfg := DefaultConfig()
	transport := NewTransport(cfg)

	assert.False(t, transport.DisableKeepAlives)
	assert.False(t, transport.DisableCompression)
	assert.True(t, transport.ForceAttemptHTTP2)
}

func TestNewTransport_DisableKeepAlives(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DisableKeepAlives = true
	transport := NewTransport(cfg)

	assert.True(t, transport.DisableKeepAlives)
}

func TestNewTransport_DisableCompression(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DisableCompression = true
	transport := NewTransport(cfg)

	assert.True(t, transport.DisableCompression)
}

// -----------------------------------------------------------------------------
// NewPool
// -----------------------------------------------------------------------------

func TestNewPool_PreCreatesAllFiveProfiles(t *testing.T) {
	pool := NewPool()

	assert.NotNil(t, pool)
	assert.Len(t, pool.clients, 5, "Pool should have exactly 5 pre-created clients")

	profiles := []ClientProfile{
		ProfileDefault,
		ProfileHighThroughput,
		ProfileLowLatency,
		ProfileBatch,
		ProfileExternal,
	}
	for _, p := range profiles {
		client, ok := pool.clients[p]
		assert.True(t, ok, "Pool should have a client for profile %q", p)
		assert.NotNil(t, client, "Client for profile %q should not be nil", p)
	}
}

// -----------------------------------------------------------------------------
// Pool.Get
// -----------------------------------------------------------------------------

func TestPool_Get_ReturnsCorrectClientPerProfile(t *testing.T) {
	pool := NewPool()

	tests := []struct {
		profile         ClientProfile
		expectedTimeout time.Duration
	}{
		{ProfileDefault, 30 * time.Second},
		{ProfileHighThroughput, 15 * time.Second},
		{ProfileLowLatency, 5 * time.Second},
		{ProfileBatch, 120 * time.Second},
		{ProfileExternal, 60 * time.Second},
	}

	for _, tt := range tests {
		t.Run(string(tt.profile), func(t *testing.T) {
			client := pool.Get(tt.profile)
			assert.NotNil(t, client)
			assert.Equal(t, tt.expectedTimeout, client.Timeout,
				"Profile %q client should have timeout %v", tt.profile, tt.expectedTimeout)
		})
	}
}

func TestPool_Get_UnknownProfileFallsBackToDefault(t *testing.T) {
	pool := NewPool()

	client := pool.Get("unknown_profile")
	defaultClient := pool.Get(ProfileDefault)

	assert.NotNil(t, client)
	// Both should be the same pointer (same pre-created default client)
	assert.Same(t, defaultClient, client,
		"Unknown profile should return the same default client instance")
}

// -----------------------------------------------------------------------------
// Pool convenience methods
// -----------------------------------------------------------------------------

func TestPool_ConvenienceMethods(t *testing.T) {
	pool := NewPool()

	tests := []struct {
		name            string
		method          func() *http.Client
		expectedTimeout time.Duration
	}{
		{"Default", pool.Default, 30 * time.Second},
		{"HighThroughput", pool.HighThroughput, 15 * time.Second},
		{"LowLatency", pool.LowLatency, 5 * time.Second},
		{"Batch", pool.Batch, 120 * time.Second},
		{"External", pool.External, 60 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.method()
			assert.NotNil(t, client)
			assert.Equal(t, tt.expectedTimeout, client.Timeout)
		})
	}
}

func TestPool_ConvenienceMethods_ReturnSameInstanceAsGet(t *testing.T) {
	pool := NewPool()

	assert.Same(t, pool.Get(ProfileDefault), pool.Default())
	assert.Same(t, pool.Get(ProfileHighThroughput), pool.HighThroughput())
	assert.Same(t, pool.Get(ProfileLowLatency), pool.LowLatency())
	assert.Same(t, pool.Get(ProfileBatch), pool.Batch())
	assert.Same(t, pool.Get(ProfileExternal), pool.External())
}

// -----------------------------------------------------------------------------
// DefaultRetryConfig
// -----------------------------------------------------------------------------

func TestDefaultRetryConfig_CorrectDefaults(t *testing.T) {
	cfg := DefaultRetryConfig()

	assert.Equal(t, 3, cfg.MaxRetries, "MaxRetries should be 3")
	assert.Equal(t, 100*time.Millisecond, cfg.InitialDelay, "InitialDelay should be 100ms")
	assert.Equal(t, 5*time.Second, cfg.MaxDelay, "MaxDelay should be 5s")
	assert.Equal(t, 2.0, cfg.Multiplier, "Multiplier should be 2.0")

	expectedStatuses := []int{
		http.StatusRequestTimeout,        // 408
		http.StatusTooManyRequests,        // 429
		http.StatusInternalServerError,    // 500
		http.StatusBadGateway,            // 502
		http.StatusServiceUnavailable,    // 503
		http.StatusGatewayTimeout,        // 504
	}
	assert.ElementsMatch(t, expectedStatuses, cfg.RetryableStatus,
		"RetryableStatus should contain the standard retryable HTTP status codes")
}

// -----------------------------------------------------------------------------
// RetryConfig.IsRetryableStatus
// -----------------------------------------------------------------------------

func TestRetryConfig_IsRetryableStatus(t *testing.T) {
	cfg := DefaultRetryConfig()

	tests := []struct {
		name       string
		statusCode int
		retryable  bool
	}{
		// Retryable statuses
		{"408 Request Timeout", http.StatusRequestTimeout, true},
		{"429 Too Many Requests", http.StatusTooManyRequests, true},
		{"500 Internal Server Error", http.StatusInternalServerError, true},
		{"502 Bad Gateway", http.StatusBadGateway, true},
		{"503 Service Unavailable", http.StatusServiceUnavailable, true},
		{"504 Gateway Timeout", http.StatusGatewayTimeout, true},
		// Non-retryable statuses
		{"200 OK", http.StatusOK, false},
		{"201 Created", http.StatusCreated, false},
		{"204 No Content", http.StatusNoContent, false},
		{"301 Moved Permanently", http.StatusMovedPermanently, false},
		{"400 Bad Request", http.StatusBadRequest, false},
		{"401 Unauthorized", http.StatusUnauthorized, false},
		{"403 Forbidden", http.StatusForbidden, false},
		{"404 Not Found", http.StatusNotFound, false},
		{"422 Unprocessable Entity", http.StatusUnprocessableEntity, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cfg.IsRetryableStatus(tt.statusCode)
			assert.Equal(t, tt.retryable, result,
				"IsRetryableStatus(%d) should return %v", tt.statusCode, tt.retryable)
		})
	}
}

// -----------------------------------------------------------------------------
// RetryConfig.CalculateDelay
// -----------------------------------------------------------------------------

func TestRetryConfig_CalculateDelay_ExponentialBackoff(t *testing.T) {
	cfg := RetryConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
	}

	tests := []struct {
		attempt       int
		expectedDelay time.Duration
	}{
		// attempt 0: delay = 100ms * 2^0 = 100ms
		{0, 100 * time.Millisecond},
		// attempt 1: delay = 100ms * 2^1 = 200ms
		{1, 200 * time.Millisecond},
		// attempt 2: delay = 100ms * 2^2 = 400ms
		{2, 400 * time.Millisecond},
		// attempt 3: delay = 100ms * 2^3 = 800ms
		{3, 800 * time.Millisecond},
		// attempt 4: delay = 100ms * 2^4 = 1600ms
		{4, 1600 * time.Millisecond},
		// attempt 5: delay = 100ms * 2^5 = 3200ms
		{5, 3200 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run("attempt_"+string(rune('0'+tt.attempt)), func(t *testing.T) {
			delay := cfg.CalculateDelay(tt.attempt)
			assert.Equal(t, tt.expectedDelay, delay,
				"CalculateDelay(%d) should return %v", tt.attempt, tt.expectedDelay)
		})
	}
}

func TestRetryConfig_CalculateDelay_CapsAtMaxDelay(t *testing.T) {
	cfg := RetryConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}

	// attempt 10: 100ms * 2^10 = 102400ms — well beyond MaxDelay of 1s
	delay := cfg.CalculateDelay(10)
	assert.Equal(t, cfg.MaxDelay, delay,
		"CalculateDelay should cap at MaxDelay when exponential exceeds it")
}

func TestRetryConfig_CalculateDelay_AtMaxDelayBoundary(t *testing.T) {
	cfg := RetryConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
	}

	// attempt 6: 100ms * 2^6 = 6400ms > 5000ms → should cap at 5s
	delay := cfg.CalculateDelay(6)
	assert.Equal(t, 5*time.Second, delay,
		"Delay exceeding MaxDelay should be capped at MaxDelay")
}

func TestRetryConfig_CalculateDelay_DefaultConfig(t *testing.T) {
	cfg := DefaultRetryConfig()

	// Verify the full progression with defaults: 100ms initial, 2x multiplier, 5s max
	// attempt 0 → 100ms
	assert.Equal(t, 100*time.Millisecond, cfg.CalculateDelay(0))
	// attempt 1 → 200ms
	assert.Equal(t, 200*time.Millisecond, cfg.CalculateDelay(1))
	// attempt 2 → 400ms
	assert.Equal(t, 400*time.Millisecond, cfg.CalculateDelay(2))
	// attempt 3 → 800ms
	assert.Equal(t, 800*time.Millisecond, cfg.CalculateDelay(3))
	// attempt 4 → 1600ms
	assert.Equal(t, 1600*time.Millisecond, cfg.CalculateDelay(4))
	// attempt 5 → 3200ms
	assert.Equal(t, 3200*time.Millisecond, cfg.CalculateDelay(5))
	// attempt 6 → would be 6400ms but capped at 5000ms
	assert.Equal(t, 5*time.Second, cfg.CalculateDelay(6))
	// attempt 100 → still capped at 5s
	assert.Equal(t, 5*time.Second, cfg.CalculateDelay(100))
}

// -----------------------------------------------------------------------------
// ContextWithTimeout
// -----------------------------------------------------------------------------

func TestContextWithTimeout_CreatesContextWithDeadline(t *testing.T) {
	parent := context.Background()
	timeout := 500 * time.Millisecond

	ctx, cancel := ContextWithTimeout(parent, timeout)
	defer cancel()

	assert.NotNil(t, ctx)
	assert.NotNil(t, cancel)

	deadline, ok := ctx.Deadline()
	assert.True(t, ok, "Context should have a deadline")
	assert.WithinDuration(t, time.Now().Add(timeout), deadline, 50*time.Millisecond,
		"Deadline should be approximately now + timeout")
}

func TestContextWithTimeout_CancelFunctionCancelsContext(t *testing.T) {
	parent := context.Background()

	ctx, cancel := ContextWithTimeout(parent, 10*time.Second)
	cancel()

	assert.Error(t, ctx.Err(), "Cancelled context should have a non-nil error")
	assert.Equal(t, context.Canceled, ctx.Err())
}

func TestContextWithTimeout_RespectsParentCancellation(t *testing.T) {
	parent, parentCancel := context.WithCancel(context.Background())

	ctx, cancel := ContextWithTimeout(parent, 10*time.Second)
	defer cancel()

	parentCancel()

	assert.Error(t, ctx.Err(), "Child context should be cancelled when parent is cancelled")
}
