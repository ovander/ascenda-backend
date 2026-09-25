package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newLimiter(t *testing.T, rps float64, burst int, keyFn KeyFunc, trusted TrustedProxies) *RateLimiter {
	t.Helper()
	rl := NewRateLimiter(rps, burst, keyFn, trusted)
	t.Cleanup(rl.Stop)
	return rl
}

func hit(rl *RateLimiter, req *http.Request) int {
	w := httptest.NewRecorder()
	rl.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(w, req)
	return w.Code
}

func reqFrom(remote string, headers map[string]string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/auth/register", nil)
	req.RemoteAddr = remote
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

// ── ClientIP ──────────────────────────────────────────────────────────────────

func TestClientIP(t *testing.T) {
	loopback, err := ParseTrustedProxies([]string{"127.0.0.1/32", "::1"})
	require.NoError(t, err)

	cases := []struct {
		name    string
		remote  string
		headers map[string]string
		trusted TrustedProxies
		want    string
	}{
		{"direct peer, no proxy", "203.0.113.7:5555", nil, loopback, "203.0.113.7"},
		{"forged XFF from untrusted peer is ignored", "203.0.113.7:5555", map[string]string{"X-Forwarded-For": "10.0.0.1"}, loopback, "203.0.113.7"},
		{"XFF via trusted proxy", "127.0.0.1:1234", map[string]string{"X-Forwarded-For": "198.51.100.9"}, loopback, "198.51.100.9"},
		{"rightmost untrusted XFF entry wins", "127.0.0.1:1234", map[string]string{"X-Forwarded-For": "1.1.1.1, 198.51.100.9, 127.0.0.1"}, loopback, "198.51.100.9"},
		{"X-Real-IP via trusted proxy", "127.0.0.1:1234", map[string]string{"X-Real-IP": "198.51.100.10"}, loopback, "198.51.100.10"},
		{"trusted proxy without headers falls back to peer", "127.0.0.1:1234", nil, loopback, "127.0.0.1"},
		{"garbage XFF via trusted proxy falls back to peer", "127.0.0.1:1234", map[string]string{"X-Forwarded-For": "not-an-ip"}, loopback, "127.0.0.1"},
		{"ipv6 peer", "[2001:db8::1]:443", nil, loopback, "2001:db8::1"},
		{"no trusted proxies configured", "127.0.0.1:1234", map[string]string{"X-Forwarded-For": "198.51.100.9"}, nil, "127.0.0.1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ClientIP(reqFrom(tc.remote, tc.headers), tc.trusted))
		})
	}
}

func TestParseTrustedProxies(t *testing.T) {
	tp, err := ParseTrustedProxies([]string{" 10.0.0.0/8 ", "", "192.168.1.5", "fd00::/8"})
	require.NoError(t, err)
	assert.Len(t, tp, 3)

	_, err = ParseTrustedProxies([]string{"nope"})
	assert.Error(t, err)
	_, err = ParseTrustedProxies([]string{"10.0.0.0/99"})
	assert.Error(t, err)
}

// ── Keying ────────────────────────────────────────────────────────────────────

func TestRateLimiter_UnauthenticatedRequestsAreLimitedPerIP(t *testing.T) {
	// Regression for audit S-H3: the old limiter passed every request without
	// a tenant ID straight through, so /auth/register was never limited.
	rl := newLimiter(t, 0, 2, KeyByClientIP(nil), nil) // burst 2, no refill

	a := reqFrom("203.0.113.7:1", nil)
	assert.Equal(t, 200, hit(rl, a))
	assert.Equal(t, 200, hit(rl, a))
	assert.Equal(t, 429, hit(rl, a), "third request from the same IP is rejected")

	b := reqFrom("203.0.113.8:1", nil)
	assert.Equal(t, 200, hit(rl, b), "another IP has its own bucket")
}

func TestRateLimiter_ForgedForwardedForCannotEscapeBucket(t *testing.T) {
	rl := newLimiter(t, 0, 1, KeyByClientIP(nil), nil)
	assert.Equal(t, 200, hit(rl, reqFrom("203.0.113.7:1", map[string]string{"X-Forwarded-For": "1.1.1.1"})))
	assert.Equal(t, 429, hit(rl, reqFrom("203.0.113.7:1", map[string]string{"X-Forwarded-For": "2.2.2.2"})),
		"changing a forged header must not change the key")
}

func TestRateLimiter_KeyByUserFallsBackToIP(t *testing.T) {
	rl := newLimiter(t, 0, 1, KeyByUser, nil)

	userA, userB := uuid.New(), uuid.New()
	withUser := func(id uuid.UUID, remote string) *http.Request {
		req := reqFrom(remote, nil)
		return req.WithContext(ctxutil.WithUserID(req.Context(), id))
	}

	assert.Equal(t, 200, hit(rl, withUser(userA, "203.0.113.7:1")))
	assert.Equal(t, 429, hit(rl, withUser(userA, "203.0.113.9:1")), "same user from another IP shares the bucket")
	assert.Equal(t, 200, hit(rl, withUser(userB, "203.0.113.7:1")), "another user has its own bucket")

	// No user in context → per-IP bucket, never pass-through.
	assert.Equal(t, 200, hit(rl, reqFrom("198.51.100.1:1", nil)))
	assert.Equal(t, 429, hit(rl, reqFrom("198.51.100.1:1", nil)))
}

func TestRateLimiter_KeyByTenantFallsBackToUserThenIP(t *testing.T) {
	rl := newLimiter(t, 0, 1, KeyByTenant, nil)

	tenant := uuid.New()
	withTenant := func(remote string) *http.Request {
		req := reqFrom(remote, nil)
		ctx := ctxutil.WithTenantID(req.Context(), tenant)
		ctx = ctxutil.WithUserID(ctx, uuid.New())
		return req.WithContext(ctx)
	}
	assert.Equal(t, 200, hit(rl, withTenant("203.0.113.7:1")))
	assert.Equal(t, 429, hit(rl, withTenant("203.0.113.8:1")), "different users of one tenant share the tenant bucket")

	user := uuid.New()
	withUserOnly := func() *http.Request {
		req := reqFrom("203.0.113.7:1", nil)
		return req.WithContext(ctxutil.WithUserID(req.Context(), user))
	}
	assert.Equal(t, 200, hit(rl, withUserOnly()))
	assert.Equal(t, 429, hit(rl, withUserOnly()))

	assert.Equal(t, 200, hit(rl, reqFrom("198.51.100.1:1", nil)))
	assert.Equal(t, 429, hit(rl, reqFrom("198.51.100.1:1", nil)))
}

// ── Behaviour ─────────────────────────────────────────────────────────────────

func TestRateLimiter_RejectionHeadersAndBody(t *testing.T) {
	rl := newLimiter(t, 0, 1, KeyByClientIP(nil), nil)
	req := reqFrom("203.0.113.7:1", nil)
	hit(rl, req)

	w := httptest.NewRecorder()
	rl.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("must not run") })).ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "1", w.Header().Get("Retry-After"))
	assert.Contains(t, w.Body.String(), `"code":"rate_limited"`)
}

func TestRateLimiter_RefillsOverTime(t *testing.T) {
	rl := newLimiter(t, 100, 1, KeyByClientIP(nil), nil) // 1 token every 10 ms
	req := reqFrom("203.0.113.7:1", nil)
	assert.Equal(t, 200, hit(rl, req))
	assert.Equal(t, 429, hit(rl, req))
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, 200, hit(rl, req))
}

func TestRateLimiter_EvictsIdleBuckets(t *testing.T) {
	rl := newLimiter(t, 0, 1, KeyByClientIP(nil), nil)
	base := time.Now()
	rl.now = func() time.Time { return base }

	req := reqFrom("203.0.113.7:1", nil)
	assert.Equal(t, 200, hit(rl, req))
	assert.Equal(t, 429, hit(rl, req))
	assert.Len(t, rl.buckets, 1)

	rl.now = func() time.Time { return base.Add(rateLimiterTTL + time.Second) }
	rl.evictStale()
	assert.Empty(t, rl.buckets)

	assert.Equal(t, 200, hit(rl, req), "a fresh bucket is created after eviction")
}
