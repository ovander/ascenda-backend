package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/sirupsen/logrus"
)

// RecoverMiddleware recovers from panics and logs them.
type RecoverMiddleware struct {
	logger *logrus.Logger
}

// NewRecoverMiddleware creates a new RecoverMiddleware.
func NewRecoverMiddleware(logger *logrus.Logger) *RecoverMiddleware {
	return &RecoverMiddleware{
		logger: logger,
	}
}

// Handler wraps an HTTP handler with panic recovery.
func (m *RecoverMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				m.logger.WithField("panic", err).
					WithField("stack", string(debug.Stack())).
					Error("panic recovered")

				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
