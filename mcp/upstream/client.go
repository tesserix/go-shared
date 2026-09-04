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
	// ErrInvalidPath is returned when a request path contains a segment Get
	// refuses to interpret: ".", "..", or an interior empty segment (a "//"
	// anywhere but a single trailing slash). See Get's doc comment.
	ErrInvalidPath = errors.New("upstream: invalid path")
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
//
// The request path is joined to the base URL's path prefix, and it is never
// silently rewritten: every segment of reqPath is validated before any
// cleaning happens. A segment equal to "." or ".." is refused, as is an
// interior empty segment (a "//" anywhere inside the path). A single
// trailing slash is fine — "/stores/bondi/" is a normal, benign path — only
// interior empties are rejected. Rejections return an error wrapping
// ErrInvalidPath. path.Clean and the base-path-prefix guard still run after
// validation as defence in depth; on a validated path they are close to a
// no-op, and that is the point.
func (c *Client) Get(ctx context.Context, reqPath string, params url.Values, out any) error {
	if err := validateReqPath(reqPath); err != nil {
		return err
	}

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
	hadTrailingSlash := strings.HasSuffix(fullPath, "/") && fullPath != "/"
	fullPath = path.Clean(fullPath)
	// path.Clean strips a trailing slash; validateReqPath already established
	// that reqPath's trailing slash (if any) was benign, not an escape
	// attempt, so restore it — the request must reach the server exactly as
	// the caller asked, per Get's doc comment.
	if hadTrailingSlash && !strings.HasSuffix(fullPath, "/") {
		fullPath += "/"
	}

	// Guard against escape: the result must be within the base path. If base is
	// /api/v1 and reqPath is /../admin, fullPath becomes /admin, which is
	// outside /api/v1. Comparing fullPath+"/" against basePath+"/" also admits
	// the exact-match case (fullPath == basePath), so that needs no clause of
	// its own.
	if basePath != "" && !strings.HasPrefix(fullPath+"/", basePath+"/") {
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

// validateReqPath rejects a request path before any cleaning is done to it,
// so Get never silently rewrites what the caller asked for. It refuses a
// segment equal to "." or ".." and any interior empty segment. The leading
// empty segment produced by a leading "/" is not interior and is allowed,
// and so is a single trailing empty segment (a trailing slash) — only an
// empty segment strictly between two others is rejected.
//
// The returned error names the offending segment but never the full path: a
// base URL can carry credentials in userinfo, and a path can carry
// identifiers that should not be echoed back wholesale into logs or errors.
func validateReqPath(reqPath string) error {
	segments := strings.Split(reqPath, "/")
	last := len(segments) - 1
	for i, seg := range segments {
		switch {
		case seg == "." || seg == "..":
			return fmt.Errorf("%w: segment %q", ErrInvalidPath, seg)
		case seg == "" && i != 0 && i != last:
			return fmt.Errorf("%w: empty segment", ErrInvalidPath)
		}
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
