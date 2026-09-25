package middleware

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"golang.org/x/time/rate"
)

// Rate limiting.
//
// Every limiter is a token bucket per key. The key is chosen by a KeyFunc so
// the same limiter type can protect public routes (per client IP), the
// authenticated API (per user) and compute-heavy reports (per tenant).
//
// A limiter never passes a request through unkeyed: when the preferred key is
// unavailable it falls back to the next one, ending with the client IP. The
// previous implementation (backendkit/httpware) keyed on the tenant ID only
// and let every request without one straight through, which left the public
// auth routes and the general API limiter without any effect (audit S-H3).

const (
	rateLimiterTTL             = 10 * time.Minute
	rateLimiterCleanupInterval = 2 * time.Minute
)

// KeyFunc derives the bucket key for a request. ok=false means "no key of
// this kind is available", and the limiter falls back to the client IP.
type KeyFunc func(r *http.Request) (key string, ok bool)

// TrustedProxies is the set of peer networks whose X-Forwarded-For /
// X-Real-IP headers are believed. Requests arriving directly from any other
// peer are keyed on the TCP peer address, so a client cannot pick its own key
// by sending a forged header.
type TrustedProxies []*net.IPNet

// ParseTrustedProxies parses a list of CIDRs (a bare IP is accepted as a /32
// or /128). Empty or whitespace entries are skipped.
func ParseTrustedProxies(cidrs []string) (TrustedProxies, error) {
	var out TrustedProxies
	for _, raw := range cidrs {
		c := strings.TrimSpace(raw)
		if c == "" {
			continue
		}
		if !strings.Contains(c, "/") {
			ip := net.ParseIP(c)
			if ip == nil {
				return nil, &net.ParseError{Type: "IP address", Text: c}
			}
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			c = c + "/" + strconv.Itoa(bits)
		}
		_, ipnet, err := net.ParseCIDR(c)
		if err != nil {
			return nil, err
		}
		out = append(out, ipnet)
	}
	return out, nil
}

func (t TrustedProxies) contains(ip net.IP) bool {
	if ip == nil {
		return false
	}
	for _, n := range t {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// ClientIP returns the originating client address. The TCP peer is used
// unless it is a trusted proxy, in which case the rightmost X-Forwarded-For
// entry not belonging to a trusted proxy (or X-Real-IP) is used. The result
// is a canonical textual IP, never empty.
func ClientIP(r *http.Request, trusted TrustedProxies) string {
	peerHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peerHost = r.RemoteAddr
	}
	peer := net.ParseIP(strings.TrimSpace(peerHost))

	if peer != nil && trusted.contains(peer) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			for i := len(parts) - 1; i >= 0; i-- {
				ip := net.ParseIP(strings.TrimSpace(parts[i]))
				if ip == nil {
					continue
				}
				if !trusted.contains(ip) {
					return ip.String()
				}
			}
		}
		if real := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); real != nil {
			return real.String()
		}
	}
	if peer != nil {
		return peer.String()
	}
	if peerHost == "" {
		return "unknown"
	}
	return peerHost
}

// KeyByClientIP keys every request on the originating client address.
func KeyByClientIP(trusted TrustedProxies) KeyFunc {
	return func(r *http.Request) (string, bool) {
		return "ip:" + ClientIP(r, trusted), true
	}
}

// KeyByUser keys on the authenticated user ID (set by the JWT middleware from
// the subject claim). Unauthenticated requests fall back to the client IP.
func KeyByUser(r *http.Request) (string, bool) {
	if id := ctxutil.GetUserID(r.Context()); id != uuid.Nil {
		return "user:" + id.String(), true
	}
	return "", false
}

// KeyByTenant keys on the resolved tenant (fair share of compute-heavy
// endpoints between workspaces); falls back to the user, then the client IP.
func KeyByTenant(r *http.Request) (string, bool) {
	if id := ctxutil.GetTenantID(r.Context()); id != uuid.Nil {
		return "tenant:" + id.String(), true
	}
	return KeyByUser(r)
}

type bucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter is a keyed token-bucket limiter with idle-entry eviction.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rps     rate.Limit
	burst   int
	keyFn   KeyFunc
	trusted TrustedProxies
	now     func() time.Time
	stop    chan struct{}
}

// NewRateLimiter creates a limiter allowing rps requests per second with the
// given burst, per key produced by keyFn. trusted is used for the client-IP
// fallback (and by KeyByClientIP keys created through this limiter).
func NewRateLimiter(rps float64, burst int, keyFn KeyFunc, trusted TrustedProxies) *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*bucket),
		rps:     rate.Limit(rps),
		burst:   burst,
		keyFn:   keyFn,
		trusted: trusted,
		now:     time.Now,
		stop:    make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

// Stop ends the background eviction goroutine.
func (rl *RateLimiter) Stop() { close(rl.stop) }

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

func (rl *RateLimiter) evictStale() {
	cutoff := rl.now().Add(-rateLimiterTTL)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for k, b := range rl.buckets {
		if b.lastSeen.Before(cutoff) {
			delete(rl.buckets, k)
		}
	}
}

// keyFor resolves the bucket key, falling back to the client IP.
func (rl *RateLimiter) keyFor(r *http.Request) string {
	if rl.keyFn != nil {
		if key, ok := rl.keyFn(r); ok && key != "" {
			return key
		}
	}
	return "ip:" + ClientIP(r, rl.trusted)
}

func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	b, ok := rl.buckets[key]
	if !ok {
		b = &bucket{limiter: rate.NewLimiter(rl.rps, rl.burst)}
		rl.buckets[key] = b
	}
	b.lastSeen = rl.now()
	return b.limiter.Allow()
}

// Handler enforces the limit. Rejected requests get 429 with Retry-After.
func (rl *RateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.allow(rl.keyFor(r)) {
			w.Header().Set("Retry-After", "1")
			writeJSONError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
			return
		}
		next.ServeHTTP(w, r)
	})
}
