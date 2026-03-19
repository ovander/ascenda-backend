package middleware

import (
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"kerplan/internal/pkg/ctxutil"
)

// statusWriter wraps http.ResponseWriter to capture status code.
type statusWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// LoggerMiddleware creates request-scoped logging.
type LoggerMiddleware struct {
	logger *logrus.Logger
}

// NewLoggerMiddleware creates a new LoggerMiddleware.
func NewLoggerMiddleware(logger *logrus.Logger) *LoggerMiddleware {
	return &LoggerMiddleware{
		logger: logger,
	}
}

// Handler wraps an HTTP handler with request-scoped logging.
func (m *LoggerMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := ctxutil.GetRequestID(r.Context())

		// Create request-scoped logger entry
		entry := m.logger.WithField("request_id", requestID).
			WithField("method", r.Method).
			WithField("path", r.URL.Path)

		// Wrap response writer to capture status code
		wrapped := &statusWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Inject logger into context
		ctx := ctxutil.WithLogger(r.Context(), entry)

		start := time.Now()

		// Call next handler
		next.ServeHTTP(wrapped, r.WithContext(ctx))

		// Log request completion
		duration := time.Since(start)
		entry.WithField("status", wrapped.statusCode).
			WithField("duration_ms", duration.Milliseconds()).
			WithField("tenant_id", ctxutil.GetTenantIDStr(r.Context())).
			Info("request completed")
	})
}
