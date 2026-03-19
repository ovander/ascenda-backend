package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
	"kerplan/internal/pkg/ctxutil"
)

// RateLimiter provides per-tenant token-bucket rate limiting.
type RateLimiter struct {
	mu       sync.Mutex
	limiters map[uuid.UUID]*rate.Limiter
	rps      rate.Limit
	burst    int
}

// NewRateLimiter creates a RateLimiter with the given requests-per-second and burst.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[uuid.UUID]*rate.Limiter),
		rps:      rate.Limit(rps),
		burst:    burst,
	}
}

// getLimiter returns or creates a limiter for the given tenant.
func (rl *RateLimiter) getLimiter(tenantID uuid.UUID) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if l, ok := rl.limiters[tenantID]; ok {
		return l
	}

	l := rate.NewLimiter(rl.rps, rl.burst)
	rl.limiters[tenantID] = l
	return l
}

// Handler returns an http middleware that enforces the rate limit.
// It must be applied after auth middleware (tenantID must be in context).
func (rl *RateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := ctxutil.GetTenantID(r.Context())
		if tenantID == uuid.Nil {
			// No tenant in context — let the request through (auth will catch it)
			next.ServeHTTP(w, r)
			return
		}

		limiter := rl.getLimiter(tenantID)
		if !limiter.Allow() {
			retryAfter := time.Second // approximate
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":{"code":"rate_limited","message":"too many requests"}}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}
