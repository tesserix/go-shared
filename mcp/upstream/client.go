// Package upstream is the only way a connector reaches a product API.
//
// It exposes exactly one verb. That is D6 made structural: a connector cannot
// perform a write because there is no method that issues one, not because an
// author remembered to avoid it.
//
// The base URL may include a path prefix (e.g., http://svc:8080/api/v1).
// When Get() is called, the request path is joined to the base path, and
// the result must remain within the base path prefix. Path traversal
// attempts (e.g., "/../admin") are rejected with an error.
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
	"path"
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
	// timeout is the intended per-request budget. It is held separately from
	// http.Timeout so New can stamp it after every option has run — see New.
	timeout time.Duration
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
	return func(c *Client) { c.timeout = d }
}

// WithHTTPClient replaces the underlying transport, dialer and so on. The
// per-request budget is NOT taken from the supplied client: New stamps the
// Client's own timeout (WithTimeout, else the 400ms default) onto the copy
// after every option has run, so option order cannot change the effective
// deadline. The no-redirect policy is re-applied there for the same reason.
// The supplied client is shallow-copied to avoid mutating the caller's object.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) {
		// Shallow-copy to avoid mutating the caller's client.
		cp := *h
		c.http = &cp
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
		http:    &http.Client{},
		timeout: defaultTimeout,
	}
	for _, o := range opts {
		o(c)
	}
	// Stamped AFTER the options so WithTimeout and WithHTTPClient commute: the
	// budget is a contract, and whichever order a caller writes them in, the
	// effective deadline is the same.
	c.http.Timeout = c.timeout
	// Applied AFTER the options so a caller-supplied client cannot reopen the
	// hole. Headers added by WithHeader are the credential path, and the
	// stdlib only strips Authorization/Cookie across hosts — every other
	// header is copied to whatever host a Location names. Not following also
	// keeps the path guard in Get from being bypassed by a redirect. A 3xx
	// then falls through Get's status switch to a decode failure, which
	// classifies as ErrUnavailable: correct for "what I was pointed at moved".
	c.http.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return c, nil
}

// Get fetches path and decodes the JSON body into out.
// The request path is joined to the base URL's path prefix.
// Path traversal attempts that would escape the base path are rejected.
func (c *Client) Get(ctx context.Context, reqPath string, params url.Values, out any) error {
	// Join base path with request path by string concatenation, not path.Join,
	// to avoid absolute request paths replacing the base path entirely.
	// Normalize base: if empty, use "/".
	basePath := c.base.Path
	if basePath == "" {
		basePath = "/"
	}
	// Ensure reqPath starts with /
	if !strings.HasPrefix(reqPath, "/") {
		reqPath = "/" + reqPath
	}
	// Concatenate: trim trailing / from base, append request path
	basePath = strings.TrimSuffix(basePath, "/")
	fullPath := basePath + reqPath
	fullPath = path.Clean(fullPath)

	// Guard against escape: the result must be within the base path.
	// If base is /api/v1, and reqPath is /../admin, fullPath becomes /admin,
	// which is outside /api/v1. Reject it.
	basePath = strings.TrimSuffix(basePath, "/")
	if basePath != "" && !strings.HasPrefix(fullPath+"/", basePath+"/") && fullPath != basePath {
		return fmt.Errorf("upstream: path traversal rejected")
	}

	target := &url.URL{
		Scheme:   c.base.Scheme,
		Host:     c.base.Host,
		Path:     fullPath,
		RawQuery: params.Encode(),
	}

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
		if classifyError(err) == ErrDeadlineExceeded {
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
		if classifyError(err) == ErrDeadlineExceeded {
			return fmt.Errorf("%w: %s", ErrDeadlineExceeded, err)
		}
		return fmt.Errorf("%w: reading body: %s", ErrUnavailable, err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%w: decoding body: %s", ErrUnavailable, err)
	}
	return nil
}

// classifyError returns the sentinel for this error, or nil if not classified.
// Used consistently to avoid misclassifying deadline/timeout errors as unavailable.
func classifyError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
		return ErrDeadlineExceeded
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
