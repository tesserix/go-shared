package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---- RetryOption / retryConfig unit tests -----------------------------------

func TestDefaultRetryConfig(t *testing.T) {
	cfg := defaultRetryConfig()
	assert.Equal(t, defaultMaxRetries, cfg.maxRetries)
	assert.Equal(t, defaultInitialDelay, cfg.initialDelay)
}

func TestWithMaxRetries(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"valid positive", 10, 10},
		{"minimum value 1", 1, 1},
		{"zero is ignored", 0, defaultMaxRetries},
		{"negative is ignored", -3, defaultMaxRetries},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := defaultRetryConfig()
			WithMaxRetries(tc.input)(&cfg)
			assert.Equal(t, tc.expected, cfg.maxRetries)
		})
	}
}

func TestWithInitialDelay(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Duration
		expected time.Duration
	}{
		{"positive value", 500 * time.Millisecond, 500 * time.Millisecond},
		{"zero is ignored", 0, defaultInitialDelay},
		{"negative is ignored", -time.Second, defaultInitialDelay},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := defaultRetryConfig()
			WithInitialDelay(tc.input)(&cfg)
			assert.Equal(t, tc.expected, cfg.initialDelay)
		})
	}
}

// ---- jitteredDelay unit tests -----------------------------------------------

func TestJitteredDelay_BaseScaling(t *testing.T) {
	initial := 100 * time.Millisecond

	// For each attempt the base doubles: base = initial * 2^(attempt-1).
	// Jitter is in [0, initial), so the result lies in [base, base+initial).
	tests := []struct {
		attempt int
		minVal  time.Duration
		maxVal  time.Duration // exclusive upper bound
	}{
		{1, 100 * time.Millisecond, 200 * time.Millisecond},
		{2, 200 * time.Millisecond, 300 * time.Millisecond},
		{3, 400 * time.Millisecond, 500 * time.Millisecond},
		{4, 800 * time.Millisecond, 900 * time.Millisecond},
	}

	for _, tc := range tests {
		tc := tc
		t.Run("attempt_"+string(rune('0'+tc.attempt)), func(t *testing.T) {
			// Run many iterations to exercise jitter randomness.
			for i := 0; i < 100; i++ {
				d := jitteredDelay(initial, tc.attempt)
				assert.GreaterOrEqual(t, d, tc.minVal)
				assert.Less(t, d, tc.maxVal)
			}
		})
	}
}

func TestJitteredDelay_LargeAttemptNoOverflow(t *testing.T) {
	// Exponents > 30 would overflow int64 without the cap; verify the result
	// is a positive, finite duration.
	d := jitteredDelay(time.Second, 40)
	assert.Positive(t, d)
}

// ---- ConnectWithRetry behavioural tests -------------------------------------
//
// These tests inject a fake openAndPingFn to drive retry logic without
// requiring a real database or a compilable gorm.Dialector stub.

// withFakeOpener replaces the package-level openAndPingFn with fn for the
// duration of the test and restores it afterwards.
func withFakeOpener(t *testing.T, fn func(gorm.Dialector, *gorm.Config) (*gorm.DB, error)) {
	t.Helper()
	original := openAndPingFn
	openAndPingFn = fn
	t.Cleanup(func() { openAndPingFn = original })
}

func TestConnectWithRetry_ContextAlreadyCancelled(t *testing.T) {
	calls := 0
	withFakeOpener(t, func(_ gorm.Dialector, _ *gorm.Config) (*gorm.DB, error) {
		calls++
		return nil, errors.New("should not reach db")
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the first attempt

	_, err := ConnectWithRetry(ctx, nil, nil,
		WithMaxRetries(3),
		WithInitialDelay(10*time.Millisecond),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
	assert.Equal(t, 0, calls, "no connection attempt should have been made")
}

func TestConnectWithRetry_ExhaustsAllRetries(t *testing.T) {
	const maxRetries = 3
	calls := 0
	withFakeOpener(t, func(_ gorm.Dialector, _ *gorm.Config) (*gorm.DB, error) {
		calls++
		return nil, errors.New("proxy not ready")
	})

	_, err := ConnectWithRetry(context.Background(), nil, nil,
		WithMaxRetries(maxRetries),
		WithInitialDelay(time.Millisecond), // keep the test fast
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "3 attempts")
	assert.Contains(t, err.Error(), "proxy not ready")
	assert.Equal(t, maxRetries, calls, "should attempt exactly maxRetries times")
}

func TestConnectWithRetry_SucceedsOnFirstAttempt(t *testing.T) {
	calls := 0
	fakeDB := &gorm.DB{}
	withFakeOpener(t, func(_ gorm.Dialector, _ *gorm.Config) (*gorm.DB, error) {
		calls++
		return fakeDB, nil
	})

	db, err := ConnectWithRetry(context.Background(), nil, nil,
		WithMaxRetries(5),
		WithInitialDelay(time.Millisecond),
	)

	require.NoError(t, err)
	assert.Equal(t, fakeDB, db)
	assert.Equal(t, 1, calls, "should succeed without any retry")
}

func TestConnectWithRetry_SucceedsAfterSomeFailures(t *testing.T) {
	const succeedOnAttempt = 3
	calls := 0
	fakeDB := &gorm.DB{}
	withFakeOpener(t, func(_ gorm.Dialector, _ *gorm.Config) (*gorm.DB, error) {
		calls++
		if calls < succeedOnAttempt {
			return nil, errors.New("transient error")
		}
		return fakeDB, nil
	})

	db, err := ConnectWithRetry(context.Background(), nil, nil,
		WithMaxRetries(5),
		WithInitialDelay(time.Millisecond),
	)

	require.NoError(t, err)
	assert.Equal(t, fakeDB, db)
	assert.Equal(t, succeedOnAttempt, calls)
}

func TestConnectWithRetry_ContextCancelledDuringWait(t *testing.T) {
	withFakeOpener(t, func(_ gorm.Dialector, _ *gorm.Config) (*gorm.DB, error) {
		return nil, errors.New("not ready")
	})

	// The context times out before the first retry delay (100 ms) elapses.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()

	_, err := ConnectWithRetry(ctx, nil, nil,
		WithMaxRetries(5),
		WithInitialDelay(100*time.Millisecond),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestConnectWithRetry_DefaultMaxRetries(t *testing.T) {
	// Verify that omitting WithMaxRetries uses defaultMaxRetries (5).
	calls := 0
	withFakeOpener(t, func(_ gorm.Dialector, _ *gorm.Config) (*gorm.DB, error) {
		calls++
		return nil, errors.New("always fails")
	})

	_, err := ConnectWithRetry(context.Background(), nil, nil,
		WithInitialDelay(time.Millisecond), // keep fast
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "5 attempts")
	assert.Equal(t, defaultMaxRetries, calls)
}

func TestConnectWithRetry_WrapsLastError(t *testing.T) {
	sentinel := errors.New("sentinel error")
	withFakeOpener(t, func(_ gorm.Dialector, _ *gorm.Config) (*gorm.DB, error) {
		return nil, sentinel
	})

	_, err := ConnectWithRetry(context.Background(), nil, nil,
		WithMaxRetries(2),
		WithInitialDelay(time.Millisecond),
	)

	require.Error(t, err)
	// The last error must be reachable via errors.Is.
	assert.True(t, errors.Is(err, sentinel), "last error should be wrapped and unwrappable")
}
