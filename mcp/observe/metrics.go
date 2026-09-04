// Package observe carries a connector's per-tool metrics.
//
// Outcome is a label rather than separate metrics because the distinction that
// matters is between a server that has STOPPED SERVING and one that KEEPS
// FAILING — and both are invisible if success and failure share a counter.
package observe

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Outcome is the controlled vocabulary for how a tool call ended.
type Outcome string

const (
	OutcomeOK           Outcome = "ok"
	OutcomeNotFound     Outcome = "not_found"
	OutcomeUnavailable  Outcome = "unavailable"
	OutcomeDeadline     Outcome = "deadline"
	OutcomeInvalidInput Outcome = "invalid_input"
	// OutcomeError is the catch-all: a handler failure that is none of the
	// above — a projection bug, a marshal failure. Without it every connector
	// invents its own "error"/"internal"/"failed" and the label stops being a
	// vocabulary.
	OutcomeError Outcome = "error"
)

// ToolMetrics records tool call counts and latency.
type ToolMetrics struct {
	calls    *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// defaultBuckets are bucketed around the 400ms per-tool contract rather than
// Prometheus defaults.
var defaultBuckets = []float64{0.01, 0.025, 0.05, 0.1, 0.2, 0.4, 0.8, 2}

// Option adjusts the metrics NewToolMetrics builds. It exists so the first
// connector that needs something different does not force a signature change in
// a release ~30 services have already taken.
type Option func(*options)

type options struct {
	buckets []float64
}

// WithBuckets overrides the latency histogram's buckets. Use it only when a
// connector's budget genuinely differs from the 400ms contract — buckets that
// vary per service make latency across the estate harder to compare.
func WithBuckets(b []float64) Option {
	return func(o *options) {
		o.buckets = append([]float64(nil), b...)
	}
}

// NewToolMetrics registers the metrics on reg.
func NewToolMetrics(reg prometheus.Registerer, service string, opts ...Option) (*ToolMetrics, error) {
	o := options{buckets: defaultBuckets}
	for _, opt := range opts {
		opt(&o)
	}
	labels := prometheus.Labels{"service": service}

	calls := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "tesserix_mcp_tool_calls_total",
		Help:        "Tool calls by tool and outcome. Read outcome!=ok against outcome=ok — a tool that only fails and a tool nobody calls look identical in a single total.",
		ConstLabels: labels,
	}, []string{"tool", "outcome"})

	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:        "tesserix_mcp_tool_duration_seconds",
		Help:        "Tool call latency. The engine budgets 400ms p99 per tool; this is how you know whether that contract holds.",
		Buckets:     o.buckets,
		ConstLabels: labels,
	}, []string{"tool", "outcome"})

	if err := reg.Register(calls); err != nil {
		return nil, err
	}
	if err := reg.Register(duration); err != nil {
		reg.Unregister(calls)
		return nil, err
	}
	return &ToolMetrics{calls: calls, duration: duration}, nil
}

// Observe records one completed tool call.
//
// tool MUST be a REGISTERED tool name, never a value taken off the wire. The
// name in an MCP tools/call request is supplied by the caller, so feeding it
// straight in lets a caller mint an unbounded number of label values and
// inflate metric cardinality at will. Look the name up in the registry first
// and record only what came back.
func (m *ToolMetrics) Observe(tool string, outcome Outcome, d time.Duration) {
	m.calls.WithLabelValues(tool, string(outcome)).Inc()
	m.duration.WithLabelValues(tool, string(outcome)).Observe(d.Seconds())
}
