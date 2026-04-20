package middleware

import (
	"net/http"

	"github.com/ovander/backendkit/httpware"
)

// RequestIDMiddleware wraps httpware.RequestID for backwards compatibility
// with the struct+Handler pattern used in router.go.
type RequestIDMiddleware struct{}

// NewRequestIDMiddleware returns a RequestIDMiddleware.
func NewRequestIDMiddleware() *RequestIDMiddleware {
	return &RequestIDMiddleware{}
}

// Handler injects or propagates the X-Request-ID header.
func (m *RequestIDMiddleware) Handler(next http.Handler) http.Handler {
	return httpware.RequestID(next)
}
