package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// End-to-end tests of the role scoping (Socrate compatibility report W2):
// real RS256 tokens verified against a local JWKS, as Socrate issues them.

const (
	testIssuer     = "https://idp.example"
	ascendaClient  = "ascenda-client"
	otherAppClient = "other-app-client"
	testKid        = "test-key"
)

type testIdP struct {
	key  *rsa.PrivateKey
	jwks *httptest.Server
}

func newTestIdP(t *testing.T) *testIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	enc := base64.RawURLEncoding.EncodeToString
	body, err := json.Marshal(map[string]any{"keys": []map[string]string{{
		"kty": "RSA", "use": "sig", "alg": "RS256", "kid": testKid,
		"n": enc(key.N.Bytes()), "e": enc(big.NewInt(int64(key.E)).Bytes()),
	}}})
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return &testIdP{key: key, jwks: srv}
}

// token signs an access token; aud may be nil (no aud claim).
func (p *testIdP) token(t *testing.T, aud []string, role string, appRoles map[string]string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"iss": testIssuer, "sub": "42",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"role": role, "app_roles": appRoles,
	}
	if aud != nil {
		claims["aud"] = aud
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = testKid
	s, err := tok.SignedString(p.key)
	require.NoError(t, err)
	return s
}

// call runs the token through the auth middleware and, when it passes, the
// platform-admin permission check of the /api/v1/admin routes.
func (p *testIdP) call(t *testing.T, verifyAudience bool, token string) (status int, role string) {
	t.Helper()
	logger := logrus.NewEntry(logrus.New())
	auth := NewAuthMiddleware(p.jwks.URL, testIssuer, ascendaClient, verifyAudience, logger)
	admin := NewRBACMiddleware(logger).RequirePermission(PermPlatformAdmin)
	h := auth.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role = ctxutil.GetUserRole(r.Context())
		admin(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(w, r)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Code, role
}

func TestAuth_AscendaAdminIsPlatformAdmin(t *testing.T) {
	p := newTestIdP(t)
	status, role := p.call(t, true, p.token(t, []string{ascendaClient}, "admin",
		map[string]string{ascendaClient: "admin"}))
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "admin", role)
}

func TestAuth_AdminOfAnotherAppIsNotAscendaAdmin(t *testing.T) {
	// A token for Ascenda whose top-level role is another application's.
	p := newTestIdP(t)
	status, role := p.call(t, true, p.token(t, []string{ascendaClient}, "admin",
		map[string]string{otherAppClient: "admin", ascendaClient: "user"}))
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "user", role)
}

func TestAuth_NoAscendaAppRoleIsPlainUser(t *testing.T) {
	p := newTestIdP(t)
	status, role := p.call(t, true, p.token(t, []string{ascendaClient}, "admin", nil))
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "user", role)
}

func TestAuth_TokenForAnotherAppIsRejected(t *testing.T) {
	p := newTestIdP(t)
	status, _ := p.call(t, true, p.token(t, []string{otherAppClient}, "admin",
		map[string]string{otherAppClient: "admin"}))
	assert.Equal(t, http.StatusUnauthorized, status)
}

func TestAuth_TokenWithoutAudienceIsRejected(t *testing.T) {
	p := newTestIdP(t)
	status, _ := p.call(t, true, p.token(t, nil, "admin", map[string]string{ascendaClient: "admin"}))
	assert.Equal(t, http.StatusUnauthorized, status)
}

func TestAuth_AudienceCheckOff_RoleStillScoped(t *testing.T) {
	// SOCRATE_VERIFY_AUDIENCE=false accepts another app's token, but its
	// admin role there grants nothing here.
	p := newTestIdP(t)
	status, role := p.call(t, false, p.token(t, []string{otherAppClient}, "admin",
		map[string]string{otherAppClient: "admin"}))
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "user", role)
}
