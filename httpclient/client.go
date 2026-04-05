package httpclient

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// ClientConfig holds HTTP client configuration for connection pooling
type ClientConfig struct {
	// Timeout is the maximum total time for a request (connection + read + write)
	Timeout time.Duration

	// MaxIdleConns controls the maximum number of idle (keep-alive) connections across all hosts
	MaxIdleConns int

	// MaxIdleConnsPerHost controls the maximum idle (keep-alive) connections per host
	MaxIdleConnsPerHost int

	// MaxConnsPerHost limits the total number of connections per host (0 = no limit)
	MaxConnsPerHost int

	// IdleConnTimeout is the maximum time an idle connection will remain idle before closing
	IdleConnTimeout time.Duration

	// DialTimeout is the maximum time to wait for a dial to complete
	DialTimeout time.Duration

	// KeepAlive specifies the interval between keep-alive probes
	KeepAlive time.Duration

	// TLSHandshakeTimeout is the maximum time to wait for a TLS handshake
	TLSHandshakeTimeout time.Duration

	// ResponseHeaderTimeout is the maximum time to wait for a server's response headers
	ResponseHeaderTimeout time.Duration

	// ExpectContinueTimeout is the maximum time to wait for a 100-continue response
	ExpectContinueTimeout time.Duration

	// DisableKeepAlives disables HTTP keep-alives
	DisableKeepAlives bool

	// DisableCompression disables automatic decompression of response body
	DisableCompression bool

	// ForceAttemptHTTP2 controls whether HTTP/2 is enabled
	ForceAttemptHTTP2 bool
}

// ClientProfile represents different client configurations for different use cases
type ClientProfile string

const (
	// ProfileDefault - Standard service-to-service communication
	ProfileDefault ClientProfile = "default"

	// ProfileHighThroughput - For high-volume APIs (products, orders)
	ProfileHighThroughput ClientProfile = "high_throughput"

	// ProfileLowLatency - For latency-sensitive APIs (auth, tenant resolution)
	ProfileLowLatency ClientProfile = "low_latency"

	// ProfileBatch - For batch/bulk operations
	ProfileBatch ClientProfile = "batch"

	// ProfileExternal - For external API calls (third-party services)
	ProfileExternal ClientProfile = "external"
)

// DefaultConfig returns a default HTTP client configuration
// Optimized for internal service-to-service communication
func DefaultConfig() ClientConfig {
	return ClientConfig{
		Timeout:               30 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       100,
		IdleConnTimeout:       90 * time.Second,
		DialTimeout:           5 * time.Second,
		KeepAlive:             30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     false,
		DisableCompression:    false,
		ForceAttemptHTTP2:     true,
	}
}

// ProfileConfigs returns optimized configurations for different profiles
func ProfileConfigs() map[ClientProfile]ClientConfig {
	return map[ClientProfile]ClientConfig{
		// Default: Balanced configuration for most services
		ProfileDefault: {
			Timeout:               30 * time.Second,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			MaxConnsPerHost:       100,
			IdleConnTimeout:       90 * time.Second,
			DialTimeout:           5 * time.Second,
			KeepAlive:             30 * time.Second,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ForceAttemptHTTP2:     true,
		},

		// High-throughput: For APIs handling 1000+ RPS
		// Maximizes connection reuse and parallel connections
		ProfileHighThroughput: {
			Timeout:               15 * time.Second,
			MaxIdleConns:          500,
			MaxIdleConnsPerHost:   50,
			MaxConnsPerHost:       500,
			IdleConnTimeout:       120 * time.Second,
			DialTimeout:           3 * time.Second,
			KeepAlive:             30 * time.Second,
			TLSHandshakeTimeout:   3 * time.Second,
			ResponseHeaderTimeout: 5 * time.Second,
			ExpectContinueTimeout: 500 * time.Millisecond,
			ForceAttemptHTTP2:     true,
		},

		// Low-latency: For fast lookups (auth, tenant)
		// Prioritizes quick responses and warm connections
		ProfileLowLatency: {
			Timeout:               5 * time.Second,
			MaxIdleConns:          200,
			MaxIdleConnsPerHost:   30,
			MaxConnsPerHost:       200,
			IdleConnTimeout:       60 * time.Second,
			DialTimeout:           2 * time.Second,
			KeepAlive:             15 * time.Second,
			TLSHandshakeTimeout:   2 * time.Second,
			ResponseHeaderTimeout: 3 * time.Second,
			ExpectContinueTimeout: 500 * time.Millisecond,
			ForceAttemptHTTP2:     true,
		},

		// Batch: For bulk operations
		// Longer timeouts, fewer connections
		ProfileBatch: {
			Timeout:               120 * time.Second,
			MaxIdleConns:          50,
			MaxIdleConnsPerHost:   10,
			MaxConnsPerHost:       50,
			IdleConnTimeout:       120 * time.Second,
			DialTimeout:           10 * time.Second,
			KeepAlive:             60 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
			ExpectContinueTimeout: 2 * time.Second,
			ForceAttemptHTTP2:     true,
		},

		// External: For third-party API calls
		// Longer timeouts, separate connection pool
		ProfileExternal: {
			Timeout:               60 * time.Second,
			MaxIdleConns:          50,
			MaxIdleConnsPerHost:   5,
			MaxConnsPerHost:       20,
			IdleConnTimeout:       60 * time.Second,
			DialTimeout:           10 * time.Second,
			KeepAlive:             30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			ExpectContinueTimeout: 2 * time.Second,
			ForceAttemptHTTP2:     true,
		},
	}
}

// GetProfileConfig returns the configuration for a specific profile
func GetProfileConfig(profile ClientProfile) ClientConfig {
	profiles := ProfileConfigs()
	if config, ok := profiles[profile]; ok {
		return config
	}
	return DefaultConfig()
}

// NewClient creates a new HTTP client with the given configuration
func NewClient(config ClientConfig) *http.Client {
	transport := NewTransport(config)

	return &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
	}
}

// NewClientWithProfile creates a new HTTP client with a predefined profile
func NewClientWithProfile(profile ClientProfile) *http.Client {
	return NewClient(GetProfileConfig(profile))
}

// NewTransport creates a new HTTP transport with the given configuration
func NewTransport(config ClientConfig) *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   config.DialTimeout,
			KeepAlive: config.KeepAlive,
		}).DialContext,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ResponseHeaderTimeout: config.ResponseHeaderTimeout,
		ExpectContinueTimeout: config.ExpectContinueTimeout,
		DisableKeepAlives:     config.DisableKeepAlives,
		DisableCompression:    config.DisableCompression,
		ForceAttemptHTTP2:     config.ForceAttemptHTTP2,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
}

// Pool manages a pool of HTTP clients for different purposes
type Pool struct {
	clients map[ClientProfile]*http.Client
}

// NewPool creates a new HTTP client pool with all profiles
func NewPool() *Pool {
	pool := &Pool{
		clients: make(map[ClientProfile]*http.Client),
	}

	// Pre-create clients for all profiles
	for profile := range ProfileConfigs() {
		pool.clients[profile] = NewClientWithProfile(profile)
	}

	return pool
}

// Get returns the HTTP client for the given profile
func (p *Pool) Get(profile ClientProfile) *http.Client {
	if client, ok := p.clients[profile]; ok {
		return client
	}
	return p.clients[ProfileDefault]
}

// Default returns the default HTTP client
func (p *Pool) Default() *http.Client {
	return p.Get(ProfileDefault)
}

// HighThroughput returns the high-throughput HTTP client
func (p *Pool) HighThroughput() *http.Client {
	return p.Get(ProfileHighThroughput)
}

// LowLatency returns the low-latency HTTP client
func (p *Pool) LowLatency() *http.Client {
	return p.Get(ProfileLowLatency)
}

// Batch returns the batch HTTP client
func (p *Pool) Batch() *http.Client {
	return p.Get(ProfileBatch)
}

// External returns the external HTTP client
func (p *Pool) External() *http.Client {
	return p.Get(ProfileExternal)
}

// Global pool instance for convenience
var globalPool *Pool

// GetPool returns the global HTTP client pool (lazy initialization)
func GetPool() *Pool {
	if globalPool == nil {
		globalPool = NewPool()
	}
	return globalPool
}

// GetClient returns a client from the global pool
func GetClient(profile ClientProfile) *http.Client {
	return GetPool().Get(profile)
}

// ContextWithTimeout creates a context with the given timeout
func ContextWithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, timeout)
}

// RetryConfig holds configuration for retry behavior
type RetryConfig struct {
	MaxRetries      int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	Multiplier      float64
	RetryableStatus []int
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		RetryableStatus: []int{
			http.StatusRequestTimeout,
			http.StatusTooManyRequests,
			http.StatusInternalServerError,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout,
		},
	}
}

// IsRetryableStatus checks if the status code is retryable
func (r RetryConfig) IsRetryableStatus(statusCode int) bool {
	for _, s := range r.RetryableStatus {
		if s == statusCode {
			return true
		}
	}
	return false
}

// CalculateDelay calculates the delay for a retry attempt
func (r RetryConfig) CalculateDelay(attempt int) time.Duration {
	delay := float64(r.InitialDelay)
	for i := 0; i < attempt; i++ {
		delay *= r.Multiplier
	}
	if delay > float64(r.MaxDelay) {
		delay = float64(r.MaxDelay)
	}
	return time.Duration(delay)
}
