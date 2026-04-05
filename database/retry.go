package database

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"gorm.io/gorm"
)

const (
	defaultMaxRetries   = 5
	defaultInitialDelay = time.Second
)

// RetryOption configures the behaviour of ConnectWithRetry.
type RetryOption func(*retryConfig)

// retryConfig holds the resolved retry parameters.
type retryConfig struct {
	maxRetries   int
	initialDelay time.Duration
}

func defaultRetryConfig() retryConfig {
	return retryConfig{
		maxRetries:   defaultMaxRetries,
		initialDelay: defaultInitialDelay,
	}
}

// WithMaxRetries sets the maximum number of connection attempts (first attempt
// plus up to maxRetries-1 retries). Values below 1 are ignored.
func WithMaxRetries(n int) RetryOption {
	return func(c *retryConfig) {
		if n >= 1 {
			c.maxRetries = n
		}
	}
}

// WithInitialDelay sets the base delay for the first retry backoff. The delay
// doubles on each subsequent attempt and a random jitter in [0, initialDelay)
// is added. Zero or negative values are ignored.
func WithInitialDelay(d time.Duration) RetryOption {
	return func(c *retryConfig) {
		if d > 0 {
			c.initialDelay = d
		}
	}
}

// openAndPingFn is the function used by ConnectWithRetry to open a connection
// and verify it. It is a package-level variable so tests can inject a fake
// without importing gorm driver packages.
var openAndPingFn = defaultOpenAndPing

// defaultOpenAndPing is the production implementation: open via gorm.Open then
// verify the underlying connection is reachable with Ping.
func defaultOpenAndPing(dialector gorm.Dialector, config *gorm.Config) (*gorm.DB, error) {
	db, err := gorm.Open(dialector, config)
	if err != nil {
		return nil, fmt.Errorf("gorm.Open: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("db.DB(): %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		// Close before returning to avoid leaking the connection handle.
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}

	return db, nil
}

// ConnectWithRetry opens a GORM database connection and verifies it with a
// Ping, retrying on failure using jittered exponential backoff.
//
// This is the recommended constructor for services deployed on Cloud Run with
// a Cloud SQL Auth Proxy sidecar. The sidecar may not be ready when the
// application container starts, so the first connection attempt often fails.
//
// Backoff formula for attempt i (0-indexed, i > 0):
//
//	delay = initialDelay * 2^(i-1) + rand[0, initialDelay)
//
// The context is checked before each attempt and during the wait period, so
// callers can enforce a deadline across all retries.
//
// Example:
//
//	db, err := database.ConnectWithRetry(
//	    ctx,
//	    postgres.Open(dsn),
//	    &gorm.Config{},
//	    database.WithMaxRetries(5),
//	    database.WithInitialDelay(time.Second),
//	)
func ConnectWithRetry(ctx context.Context, dialector gorm.Dialector, config *gorm.Config, opts ...RetryOption) (*gorm.DB, error) {
	cfg := defaultRetryConfig()
	for _, o := range opts {
		o(&cfg)
	}

	var lastErr error

	for attempt := 0; attempt < cfg.maxRetries; attempt++ {
		// Honour context cancellation before every attempt.
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("database connect retry cancelled: %w", err)
		}

		if attempt > 0 {
			delay := jitteredDelay(cfg.initialDelay, attempt)
			slog.Info("retrying database connection",
				"attempt", attempt+1,
				"max_retries", cfg.maxRetries,
				"delay", delay.String(),
				"last_error", lastErr.Error(),
			)

			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("database connect retry cancelled: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		db, err := openAndPingFn(dialector, config)
		if err != nil {
			lastErr = err
			slog.Warn("database connection attempt failed",
				"attempt", attempt+1,
				"max_retries", cfg.maxRetries,
				"error", err.Error(),
			)
			continue
		}

		slog.Info("database connection established", "attempt", attempt+1)
		return db, nil
	}

	return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", cfg.maxRetries, lastErr)
}

// jitteredDelay computes a backoff duration for the given 0-indexed attempt
// counter. The first retry corresponds to attempt=1.
//
//	delay = initialDelay * 2^(attempt-1) + rand[0, initialDelay)
//
// The exponent is capped at 30 to prevent int64 overflow on very large attempt
// counts.
func jitteredDelay(initial time.Duration, attempt int) time.Duration {
	shift := attempt - 1
	if shift > 30 {
		shift = 30
	}
	base := initial * time.Duration(1<<uint(shift))

	// Jitter in [0, initial) distributes concurrent service restarts.
	jitter := time.Duration(rand.Int64N(int64(initial)))

	return base + jitter
}
