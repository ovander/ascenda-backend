package middleware

import (
	"net/http"

	"github.com/ovander/backendkit/httpware"
	"github.com/sirupsen/logrus"
)

// LoggerMiddleware wraps httpware.Logger for backwards compatibility with the
// struct+Handler pattern used in router.go.
type LoggerMiddleware struct {
	fn func(http.Handler) http.Handler
}

// NewLoggerMiddleware creates a LoggerMiddleware backed by backendkit.
func NewLoggerMiddleware(logger *logrus.Logger) *LoggerMiddleware {
	return &LoggerMiddleware{fn: httpware.Logger(logger)}
}

// Handler wraps an HTTP handler with request-scoped logging.
func (m *LoggerMiddleware) Handler(next http.Handler) http.Handler {
	return m.fn(next)
}
