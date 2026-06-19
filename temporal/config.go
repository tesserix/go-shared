// Package temporal is the tesserix shared bootstrap for Temporal (durable
// execution): client/worker setup, sane retry defaults, and the platform naming
// conventions. Every tesserix product (HomeChef, marketplace, FanZone, …) wires
// in through this one package so onboarding a service is uniform — see the
// Temporal adoption epic (tesserix/Home-Chef-App#116, sub-issue #120).
//
// Typical worker main:
//
//	temporal.RunWorkers(
//	    temporal.Queue(temporal.TaskQueue("homechef", "orders")).
//	        Workflows(workflows.OrderWorkflow).
//	        Activities(act.CapturePayment, act.DispatchDelivery),
//	)
//
// Typical producer:
//
//	rt, _ := temporal.NewRuntime()
//	defer rt.Close()
//	rt.Start(ctx, temporal.TaskQueue("homechef", "orders"),
//	    temporal.WorkflowID("homechef", "order", id), workflows.OrderWorkflow, in)
package temporal

import "os"

// Config holds the connection settings for the Temporal frontend. Defaults are
// the local dev cluster; production values come from the environment (mounted via
// External Secrets).
type Config struct {
	HostPort   string // TEMPORAL_HOSTPORT (e.g. temporal-frontend.temporal-system:7233)
	Namespace  string // TEMPORAL_NAMESPACE (one per product, e.g. "homechef")
	TLSEnabled bool   // TEMPORAL_TLS=true for mTLS / Temporal Cloud
	enabled    bool   // explicitly configured (see Enabled)
}

// LoadConfig reads Temporal settings from the environment with dev-safe defaults.
// It is opt-in: a service only touches Temporal when TEMPORAL_HOSTPORT (or
// TEMPORAL_ENABLED=true) is set, so it keeps booting with inline fallbacks until
// the cluster exists in its environment.
func LoadConfig() Config {
	hostPort := os.Getenv("TEMPORAL_HOSTPORT")
	return Config{
		enabled:    hostPort != "" || os.Getenv("TEMPORAL_ENABLED") == "true",
		HostPort:   orDefault(hostPort, "localhost:7233"),
		Namespace:  env("TEMPORAL_NAMESPACE", "default"),
		TLSEnabled: os.Getenv("TEMPORAL_TLS") == "true",
	}
}

// Enabled reports whether Temporal is configured for this process. When false,
// callers fall back to inline execution so the service stays up.
func (c Config) Enabled() bool { return c.enabled }

func orDefault(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}
