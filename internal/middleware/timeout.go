package middleware

import (
	"context"
	"net/http"
	"time"
)

// TimeoutMiddleware returns a middleware that cancels the request context
// after the given duration. Handlers should respect ctx.Done() to abort
// long-running operations.
func TimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
