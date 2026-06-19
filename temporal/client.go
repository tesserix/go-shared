package temporal

import (
	"crypto/tls"

	"go.temporal.io/sdk/client"
)

// NewClient dials the Temporal frontend using environment configuration.
// Callers must Close() the returned client.
func NewClient() (client.Client, error) {
	return NewClientWith(LoadConfig())
}

// NewClientWith dials using an explicit Config (useful for tests / multi-namespace).
func NewClientWith(cfg Config) (client.Client, error) {
	opts := client.Options{
		HostPort:  cfg.HostPort,
		Namespace: cfg.Namespace,
	}
	if cfg.TLSEnabled {
		// TLS material (mTLS certs / Temporal Cloud) is mounted via External
		// Secrets; an empty tls.Config uses the system roots + SNI for Temporal
		// Cloud.
		opts.ConnectionOptions = client.ConnectionOptions{TLS: &tls.Config{}}
	}
	return client.Dial(opts)
}
