// Package auth verifies the shared key a connector is called with.
package auth

import (
	"crypto/subtle"
	"net/http"
)

// HeaderName is the header every product MCP server in the estate is called
// with; the registry records name it in their credentialRef.
const HeaderName = "X-MCP-Key"

// RequireKey wraps next, rejecting any request whose HeaderName does not match
// expected.
//
// An empty expected key rejects everything. That is deliberate: the key comes
// from a mounted secret, and an absent secret means the deployment is
// misconfigured — serving an open endpoint would turn a configuration mistake
// into an exposure.
func RequireKey(expected string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if expected == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		got := r.Header.Get(HeaderName)
		if subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
