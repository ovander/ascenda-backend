package middleware

import (
	"net/http"
	"time"

	"github.com/ovander/backendkit/httpware"
)

// TimeoutMiddleware cancels the request context after the given duration.
// Delegates to httpware.Timeout from backendkit.
func TimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return httpware.Timeout(timeout)
}
