package middleware

import (
	"net/http"

	"github.com/ovander/backendkit/httpware"
)

// BodyLimitMiddleware caps incoming request body size.
// Delegates to httpware.BodyLimit from backendkit.
func BodyLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return httpware.BodyLimit(maxBytes)
}
