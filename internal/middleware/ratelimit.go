package middleware

import "github.com/ovander/backendkit/httpware"

// RateLimiter re-exports the type from backendkit.
type RateLimiter = httpware.RateLimiter

// NewRateLimiter creates a per-tenant token-bucket rate limiter.
// Delegates to httpware.NewRateLimiter from backendkit.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	return httpware.NewRateLimiter(rps, burst)
}
