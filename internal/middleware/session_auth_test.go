package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovander/backendkit/bff"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/backendkit/socrate"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests of the BFF session middleware in front of the API: a session cookie
// becomes the bearer that the real auth middleware (jwtauth against a local
// JWKS) validates, so the handlers see the user as before.

const sessionCookie = "__Host-ascenda_session"

// fakeRefresher stands in for Socrate's refresh grant. Like Socrate it rotates:
// the refresh token it accepts is spent and a new one is returned.
type fakeRefresher struct {
	mu    sync.Mutex
	valid string // the one refresh token it accepts
	next  func() *socrate.TokenSet
	err   error
	calls atomic.Int32
	delay time.Duration
}

func (f *fakeRefresher) RefreshToken(_ context.Context, rt string) (*socrate.TokenSet, error) {
	f.calls.Add(1)
	time.Sleep(f.delay)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	if rt != f.valid {
		return nil, &socrate.OAuthError{StatusCode: 400, Code: "invalid_grant", Description: "refresh token reused"}
	}
	ts := f.next()
	f.valid = ts.RefreshToken
	return ts, nil
}

type sessionFixture struct {
	idp   *testIdP
	gw    *bff.Gateway
	store *bff.MemoryStore
	ref   *fakeRefresher
	// what the handler saw
	ran     atomic.Int32
	gotSub  string
	gotRole string
	gotJWT  string
	mu      sync.Mutex
}

// newSessionFixture wires SessionAuth → AuthMiddleware → a probe handler.
func newSessionFixture(t *testing.T, allowBearer bool) (*sessionFixture, http.Handler) {
	t.Helper()
	f := &sessionFixture{idp: newTestIdP(t)}
	f.store = bff.NewMemoryStore(30*time.Minute, 8*time.Hour)
	f.ref = &fakeRefresher{valid: "rt-1"}
	n := 1
	f.ref.next = func() *socrate.TokenSet {
		n++
		return &socrate.TokenSet{
			AccessToken:  f.idp.token(t, []string{ascendaClient}, "user", map[string]string{ascendaClient: "admin"}),
			RefreshToken: "rt-" + string(rune('0'+n)),
			ExpiresIn:    900,
		}
	}
	f.gw = &bff.Gateway{
		Store:     f.store,
		Cookie:    bff.CookieConfig{Name: "ascenda_session", Secure: true},
		Refresher: f.ref,
	}
	le := logrus.NewEntry(logrus.New())
	auth := NewAuthMiddleware(f.idp.jwks.URL, testIssuer, ascendaClient, true, le)
	probe := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.ran.Add(1)
		f.mu.Lock()
		f.gotSub = ctxutil.GetUserSub(r.Context())
		f.gotRole = ctxutil.GetUserRole(r.Context())
		f.gotJWT = ctxutil.GetRawJWT(r.Context())
		f.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
	return f, NewSessionAuth(f.gw, allowBearer, le).Handler(auth.Handler(probe))
}

// session stores a session whose access token expires in expiresIn seconds.
func (f *sessionFixture) session(t *testing.T, expiresIn int) *bff.Session {
	t.Helper()
	s := bff.NewSession(bff.RandomToken(32), bff.RandomToken(32), &socrate.TokenSet{
		AccessToken:  f.idp.token(t, []string{ascendaClient}, "user", map[string]string{ascendaClient: "admin"}),
		RefreshToken: "rt-1",
		ExpiresIn:    expiresIn,
	}, bff.UserInfo{Sub: "42"}, time.Now())
	f.store.Put(s)
	return s
}

func sessionRequest(method string, s *bff.Session, csrf string) *http.Request {
	r := httptest.NewRequest(method, "/api/v1/plans", nil)
	if s != nil {
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: s.ID()})
	}
	if csrf != "" {
		r.Header.Set("X-CSRF-Token", csrf)
	}
	return r
}

func serveSession(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestSessionAuth_NoSessionIs401AndHandlerNeverRuns(t *testing.T) {
	for _, allowBearer := range []bool{true, false} {
		f, h := newSessionFixture(t, allowBearer)

		w := serveSession(h, sessionRequest(http.MethodGet, nil, ""))
		assert.Equal(t, http.StatusUnauthorized, w.Code)

		// A cookie naming no session is no better.
		r := sessionRequest(http.MethodGet, nil, "")
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "forged"})
		w = serveSession(h, r)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Header().Get("Set-Cookie"), sessionCookie+"=;", "a dead cookie is cleared")

		assert.Zero(t, f.ran.Load())
	}
}

func TestSessionAuth_SessionReachesHandlerAsTheUser(t *testing.T) {
	f, h := newSessionFixture(t, false)
	s := f.session(t, 900)

	// A bearer the browser sends is replaced by the session's.
	r := sessionRequest(http.MethodGet, s, "")
	r.Header.Set("Authorization", "Bearer forged.token.value")
	w := serveSession(h, r)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "42", f.gotSub)
	assert.Equal(t, "admin", f.gotRole, "role scoped to Ascenda from app_roles")
	assert.Equal(t, s.AccessToken(), f.gotJWT)
	assert.Zero(t, f.ref.calls.Load(), "a fresh token is not refreshed")
}

func TestSessionAuth_CSRF(t *testing.T) {
	f, h := newSessionFixture(t, false)
	s := f.session(t, 900)

	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		assert.Equal(t, http.StatusForbidden, serveSession(h, sessionRequest(m, s, "")).Code, m+" without a token")
		assert.Equal(t, http.StatusForbidden, serveSession(h, sessionRequest(m, s, "wrong-"+s.CSRF())).Code, m+" with a wrong token")
		assert.Equal(t, http.StatusOK, serveSession(h, sessionRequest(m, s, s.CSRF())).Code, m+" with the token")
	}
	assert.Equal(t, int32(4), f.ran.Load(), "only the requests with the token ran")

	// Another session's token does not do.
	other := f.session(t, 900)
	assert.Equal(t, http.StatusForbidden, serveSession(h, sessionRequest(http.MethodPost, s, other.CSRF())).Code)

	// Safe methods need none.
	assert.Equal(t, http.StatusOK, serveSession(h, sessionRequest(http.MethodGet, s, "")).Code)
	assert.Equal(t, http.StatusOK, serveSession(h, sessionRequest(http.MethodHead, s, "")).Code)
}

func TestSessionAuth_RefreshesOnceUnderConcurrency(t *testing.T) {
	f, h := newSessionFixture(t, false)
	f.ref.delay = 50 * time.Millisecond
	// Warm jwtauth's key cache, as any earlier request would in production.
	require.Equal(t, http.StatusOK, serveSession(h, sessionRequest(http.MethodGet, f.session(t, 900), "")).Code)
	s := f.session(t, 5) // inside the 30 s refresh leeway

	var wg sync.WaitGroup
	codes := make([]int, 20)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = serveSession(h, sessionRequest(http.MethodGet, s, "")).Code
		}(i)
	}
	wg.Wait()

	for _, c := range codes {
		assert.Equal(t, http.StatusOK, c)
	}
	assert.Equal(t, int32(1), f.ref.calls.Load(), "one refresh for all concurrent requests")
	assert.Equal(t, "rt-2", s.RefreshToken(), "the rotated refresh token is kept")
	got, ok := f.store.Get(s.ID())
	require.True(t, ok)
	assert.Equal(t, "rt-2", got.RefreshToken(), "and written to the store")
	assert.Equal(t, s.AccessToken(), f.gotJWT, "the handler got the new access token")
}

func TestSessionAuth_RejectedRefreshEndsSession(t *testing.T) {
	f, h := newSessionFixture(t, false)
	s := f.session(t, 5)
	f.ref.valid = "someone-else" // Socrate answers invalid_grant (spent or revoked)

	w := serveSession(h, sessionRequest(http.MethodGet, s, ""))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Header().Get("Set-Cookie"), sessionCookie+"=;")
	_, ok := f.store.Get(s.ID())
	assert.False(t, ok, "session deleted")

	assert.Equal(t, http.StatusUnauthorized, serveSession(h, sessionRequest(http.MethodGet, s, "")).Code, "the old cookie stays dead")
	assert.Zero(t, f.ran.Load())
}

func TestSessionAuth_UnreachableSocrateKeepsSession(t *testing.T) {
	f, h := newSessionFixture(t, false)
	s := f.session(t, 5)
	f.ref.err = context.DeadlineExceeded

	assert.Equal(t, http.StatusBadGateway, serveSession(h, sessionRequest(http.MethodGet, s, "")).Code)
	_, ok := f.store.Get(s.ID())
	assert.True(t, ok, "a transient failure keeps the session")
	assert.Zero(t, f.ran.Load())
}

func TestSessionAuth_BearerDuringTransitionOnly(t *testing.T) {
	good := func(f *sessionFixture) string {
		return f.idp.token(t, []string{ascendaClient}, "user", map[string]string{ascendaClient: "editor"})
	}

	f, h := newSessionFixture(t, true)
	r := sessionRequest(http.MethodGet, nil, "")
	r.Header.Set("Authorization", "Bearer "+good(f))
	require.Equal(t, http.StatusOK, serveSession(h, r).Code, "today's SPA keeps working")
	assert.Equal(t, "editor", f.gotRole)

	r = sessionRequest(http.MethodGet, nil, "")
	r.Header.Set("Authorization", "Bearer not-a-jwt")
	assert.Equal(t, http.StatusUnauthorized, serveSession(h, r).Code, "the bearer is still validated")

	f, h = newSessionFixture(t, false)
	r = sessionRequest(http.MethodGet, nil, "")
	r.Header.Set("Authorization", "Bearer "+good(f))
	assert.Equal(t, http.StatusUnauthorized, serveSession(h, r).Code, "without the transition, a bearer alone is refused")
	assert.Zero(t, f.ran.Load())
}

func TestSessionAuth_NilGatewayPassesThrough(t *testing.T) {
	ran := false
	h := NewSessionAuth(nil, false, logrus.NewEntry(logrus.New())).Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { ran = true }))
	serveSession(h, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.True(t, ran)
}

func TestHasBearer(t *testing.T) {
	for h, want := range map[string]bool{"": false, "Bearer ": false, "Bearer x": true, "bearer x": true, "Basic x": false} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if h != "" {
			r.Header.Set("Authorization", h)
		}
		assert.Equal(t, want, hasBearer(r), strings.TrimSpace(h))
	}
}

func TestEndWithoutRefreshToken(t *testing.T) {
	inner := &fakeRefresher{valid: "rt-1", next: func() *socrate.TokenSet { return &socrate.TokenSet{AccessToken: "at", RefreshToken: "rt-2"} }}
	r := EndWithoutRefreshToken(inner)

	_, err := r.RefreshToken(context.Background(), "")
	assert.True(t, bff.IsFatalRefreshError(err), "no refresh token ends the session")
	assert.Zero(t, inner.calls.Load(), "Socrate is not asked")

	ts, err := r.RefreshToken(context.Background(), "rt-1")
	require.NoError(t, err)
	assert.Equal(t, "rt-2", ts.RefreshToken)
}
