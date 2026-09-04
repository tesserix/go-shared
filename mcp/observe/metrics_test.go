package observe

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestObserve_CountsPerToolAndOutcome(t *testing.T) {
	reg := prometheus.NewRegistry()
	m, err := NewToolMetrics(reg, "mcp-catalog")
	require.NoError(t, err)

	m.Observe("list_store_products", OutcomeOK, 12*time.Millisecond)
	m.Observe("list_store_products", OutcomeUnavailable, 8*time.Millisecond)
	m.Observe("get_store_product", OutcomeNotFound, 3*time.Millisecond)

	require.Equal(t, 1.0, testutil.ToFloat64(
		m.calls.WithLabelValues("list_store_products", string(OutcomeOK))))
	require.Equal(t, 1.0, testutil.ToFloat64(
		m.calls.WithLabelValues("list_store_products", string(OutcomeUnavailable))))
	require.Equal(t, 1.0, testutil.ToFloat64(
		m.calls.WithLabelValues("get_store_product", string(OutcomeNotFound))))
}

// Registering twice against one registry is a startup bug, and it must be an
// error rather than a panic in a library ~30 services import.
func TestNewToolMetrics_DuplicateRegistrationIsAnError(t *testing.T) {
	reg := prometheus.NewRegistry()
	_, err := NewToolMetrics(reg, "mcp-catalog")
	require.NoError(t, err)
	_, err = NewToolMetrics(reg, "mcp-catalog")
	require.Error(t, err)
}

// On partial failure (e.g., duration registration fails after calls succeeds),
// the registry must be left clean so retry works. Unregister calls if duration fails.
func TestNewToolMetrics_CleansUpOnPartialFailure(t *testing.T) {
	reg := prometheus.NewRegistry()

	// Pre-register a duration collector with the same name and const labels
	// so that NewToolMetrics's second Register (duration) will fail.
	collidingDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:        "tesserix_mcp_tool_duration_seconds",
		Help:        "Tool call latency. The engine budgets 400ms p99 per tool; this is how you know whether that contract holds.",
		ConstLabels: prometheus.Labels{"service": "mcp-catalog"},
	}, []string{"tool", "outcome"})
	err := reg.Register(collidingDuration)
	require.NoError(t, err)

	// Now NewToolMetrics should fail (duration collision). Without the fix, calls
	// would be left registered. With the fix, it should clean up.
	_, err = NewToolMetrics(reg, "mcp-catalog")
	require.Error(t, err)

	// Verify the registry is clean by attempting to register another calls collector.
	// If the first calls was cleaned up, this should succeed. If not, it will fail.
	testCalls := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "tesserix_mcp_tool_calls_total",
		Help:        "Tool calls by tool and outcome. Read outcome!=ok against outcome=ok — a tool that only fails and a tool nobody calls look identical in a single total.",
		ConstLabels: prometheus.Labels{"service": "mcp-catalog"},
	}, []string{"tool", "outcome"})

	err = reg.Register(testCalls)
	require.NoError(t, err, "registry was not cleaned up after partial failure; calls collector was left behind")
}
