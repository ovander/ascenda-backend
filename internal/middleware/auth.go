package middleware

// Auth middleware: backendkit/jwtauth validates the Socrate access token
// (RS256 against the JWKS, issuer, expiry), then the role is scoped to this
// application.

import (
	"net/http"

	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/backendkit/jwtauth"
	"github.com/sirupsen/logrus"
)

// SocrateClaims re-exports the JWT claims type from backendkit.
type SocrateClaims = jwtauth.SocrateClaims

// AuthMiddleware validates the bearer token and scopes its role to Ascenda.
type AuthMiddleware struct {
	jwt      *jwtauth.Middleware
	clientID string
}

// NewAuthMiddleware creates the auth middleware.
//
// Socrate serves several applications from one issuer and one JWKS, so a
// token minted for another application verifies here too. With a clientID:
//   - the token's aud claim must contain it (unless verifyAudience is false),
//     so a token for another application is rejected outright;
//   - the role seen by every later check (platform-admin routes, the tenant
//     middleware) is app_roles[clientID], the role Socrate grants for Ascenda,
//     never the top-level role claim, which may belong to another application.
//     A user without an Ascenda app role is a plain "user".
//
// Without a clientID (development without Socrate) tokens are taken as they are.
func NewAuthMiddleware(jwksURL, issuer, clientID string, verifyAudience bool, logger *logrus.Entry) *AuthMiddleware {
	var opts []jwtauth.Option
	if clientID != "" && verifyAudience {
		opts = append(opts, jwtauth.WithAudience(clientID))
	}
	return &AuthMiddleware{jwt: jwtauth.New(jwksURL, issuer, logger, opts...), clientID: clientID}
}

// Handler is the chi-compatible middleware function.
func (m *AuthMiddleware) Handler(next http.Handler) http.Handler {
	if m.clientID == "" {
		return m.jwt.Handler(next)
	}
	return m.jwt.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role := ctxutil.GetAppRoles(r.Context())[m.clientID]
		if role == "" {
			role = "user"
		}
		next.ServeHTTP(w, r.WithContext(ctxutil.WithUserRole(r.Context(), role)))
	}))
}
