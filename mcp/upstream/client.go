// Package upstream is the only way a connector reaches a product API.
//
// It exposes exactly one verb. That is D6 made structural: a connector cannot
// perform a write because there is no method that issues one, not because an
// author remembered to avoid it.
package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Failure modes a caller must be able to tell apart. A missing store and an
// unreachable API produce very different agent behaviour: one is an answer,
// the other must degrade to a document-only reply with disclosure.
var (
	ErrNotFound         = errors.New("upstream: not found")
	ErrUnavailable      = errors.New("upstream: unavailable")
	ErrDeadlineExceeded = errors.New("upstream: deadline exceeded")
)

const defaultTimeout = 400 * time.Millisecond

// Client performs GET requests against one product API.
type Client struct {
	base    *url.URL
	headers map[string]string
	http    *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithHeader adds a header sent on every request — the credential path.
func WithHeader(k, v string) Option {
	return func(c *Client) { c.headers[k] = v }
}

// WithTimeout overrides the per-request deadline. Defaults to 400ms, the
// per-tool budget the engine contracts for.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.http.Timeout = d }
}

// WithHTTPClient replaces the underlying client. The timeout is preserved.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) {
		t := c.http.Timeout
		c.http = h
		if c.http.Timeout == 0 {
			c.http.Timeout = t
		}
	}
}

// New returns a Client rooted at baseURL.
func New(baseURL string, opts ...Option) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("upstream: base URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("upstream: base URL %q must be absolute", baseURL)
	}
	c := &Client{
		base:    u,
		headers: map[string]string{},
		http:    &http.Client{Timeout: defaultTimeout},
	}
	for _, o := range opts {
		o(c)
	}
	return c, nil
}

// Get fetches path and decodes the JSON body into out.
func (c *Client) Get(ctx context.Context, path string, params url.Values, out any) error {
	ref := &url.URL{Path: path}
	if params != nil {
		ref.RawQuery = params.Encode()
	}
	target := c.base.ResolveReference(ref)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrUnavailable, err)
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			return fmt.Errorf("%w: %s", ErrDeadlineExceeded, err)
		}
		return fmt.Errorf("%w: %s", ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case resp.StatusCode >= 400:
		// Everything else — including a 4xx that means our credential is
		// wrong — is "we could not ask". It must never be reported as an
		// empty catalogue.
		return fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%w: reading body: %s", ErrUnavailable, err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%w: decoding body: %s", ErrUnavailable, err)
	}
	return nil
}

func isTimeout(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) {
		return ne.Timeout()
	}
	return strings.Contains(err.Error(), "Client.Timeout")
}
