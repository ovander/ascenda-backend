package middleware

import (
	"net/http"

	"github.com/ovander/backendkit/httpware"
	"github.com/sirupsen/logrus"
)

// RecoverMiddleware wraps httpware.Recover for backwards compatibility with
// the struct+Handler pattern used in router.go.
// Note: backendkit's Recover takes a *logrus.Entry; we create one from the Logger.
type RecoverMiddleware struct {
	fn func(http.Handler) http.Handler
}

// NewRecoverMiddleware creates a RecoverMiddleware backed by backendkit.
func NewRecoverMiddleware(logger *logrus.Logger) *RecoverMiddleware {
	return &RecoverMiddleware{fn: httpware.Recover(logrus.NewEntry(logger))}
}

// Handler wraps an HTTP handler with panic recovery.
func (m *RecoverMiddleware) Handler(next http.Handler) http.Handler {
	return m.fn(next)
}
