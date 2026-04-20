package middleware

import (
	"net/http"

	"github.com/ovander/backendkit/httpware"
)

// SecurityHeadersMiddleware wraps httpware.SecurityHeaders for backwards
// compatibility with the struct+Handler pattern used in router.go.
type SecurityHeadersMiddleware struct{}

// NewSecurityHeadersMiddleware returns a SecurityHeadersMiddleware.
func NewSecurityHeadersMiddleware() *SecurityHeadersMiddleware {
	return &SecurityHeadersMiddleware{}
}

// Handler applies security headers to every response.
func (m *SecurityHeadersMiddleware) Handler(next http.Handler) http.Handler {
	return httpware.SecurityHeaders(next)
}
