package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
	"ascenda/internal/pkg/ctxutil"
)

const (
	// rateLimiterTTL is the idle duration after which a per-tenant limiter entry
	// is evicted from the map. A tenant that makes no requests for this long will
	// have its bucket removed, reclaiming memory. When the tenant next sends a
	// request a fresh bucket is created (it starts full, which is the desired
	// behaviour for a long-idle tenant).
	rateLimiterTTL = 10 * time.Minute

	// rateLimiterCleanupInterval controls how often the background goroutine
	// sweeps the limiter map for stale entries. Must be less than rateLimiterTTL.
	rateLimiterCleanupInterval = 2 * time.Minute
)

// tenantLimiter couples a token-bucket limiter with the last-seen timestamp
// used for TTL-based eviction.
type tenantLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter provides per-tenant token-bucket rate limiting with automatic
// memory reclamation for idle tenants.
//
// Before this fix the limiters map grew without bound — one entry per unique
// tenant UUID that ever sent a request. With rateLimiterTTL=10 min, entries
// are removed after 10 minutes of inactivity, bounding peak memory to
// (active tenant count) × ~200 bytes.
type RateLimiter struct {
	mu       sync.Mutex
	limiters map[uuid.UUID]*tenantLimiter
	rps      rate.Limit
	burst    int
	stop     chan struct{} // closed by Stop() to halt the cleanup goroutine
}

// NewRateLimiter creates a RateLimiter with the given requests-per-second and
// burst and starts the background cleanup goroutine.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[uuid.UUID]*tenantLimiter),
		rps:      rate.Limit(rps),
		burst:    burst,
		stop:     make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

// Stop shuts down the background cleanup goroutine. Call this during graceful
// shutdown if the RateLimiter outlives a request (rarely necessary in practice
// since the process is about to exit, but keeps the linter happy).
func (rl *RateLimiter) Stop() {
	close(rl.stop)
}

// cleanupLoop runs at rateLimiterCleanupInterval and evicts any entry that has
// not been accessed in rateLimiterTTL. It exits when rl.stop is closed.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rateLimiterCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.evictStale()
		case <-rl.stop:
			return
		}
	}
}

// evictStale removes entries whose lastSeen is older than rateLimiterTTL.
func (rl *RateLimiter) evictStale() {
	cutoff := time.Now().Add(-rateLimiterTTL)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for id, tl := range rl.limiters {
		if tl.lastSeen.Before(cutoff) {
			delete(rl.limiters, id)
		}
	}
}

// getLimiter returns or creates a limiter for the given tenant, updating its
// lastSeen timestamp on every access.
func (rl *RateLimiter) getLimiter(tenantID uuid.UUID) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	tl, ok := rl.limiters[tenantID]
	if !ok {
		tl = &tenantLimiter{
			limiter: rate.NewLimiter(rl.rps, rl.burst),
		}
		rl.limiters[tenantID] = tl
	}
	tl.lastSeen = time.Now()
	return tl.limiter
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
