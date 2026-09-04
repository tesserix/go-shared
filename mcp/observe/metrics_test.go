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
