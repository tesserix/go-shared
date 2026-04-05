package middleware

import (
	"net/http"
	"os"
)

// SetInternalServiceHeader sets the X-Internal-Service header on an outgoing HTTP request
// using the shared secret from INTERNAL_SERVICE_SECRET env var.
// If the env var is not set, falls back to the provided serviceName for backward compatibility.
func SetInternalServiceHeader(req *http.Request, serviceName string) {
	secret := os.Getenv("INTERNAL_SERVICE_SECRET")
	if secret != "" {
		req.Header.Set(InternalServiceHeader, secret)
	} else {
		req.Header.Set(InternalServiceHeader, serviceName)
	}
}
