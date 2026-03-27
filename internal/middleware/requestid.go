package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"ascenda/internal/pkg/ctxutil"
)

// RequestIDMiddleware generates or extracts X-Request-ID header.
type RequestIDMiddleware struct{}

// NewRequestIDMiddleware creates a new RequestIDMiddleware.
func NewRequestIDMiddleware() *RequestIDMiddleware {
	return &RequestIDMiddleware{}
}

// Handler wraps an HTTP handler with request ID injection.
func (m *RequestIDMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for existing X-Request-ID header
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			// Generate new request ID
			requestID = uuid.New().String()
		}

		// Inject into context
		ctx := ctxutil.WithRequestID(r.Context(), requestID)

		// Set response header
		w.Header().Set("X-Request-ID", requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
