package middleware

// Auth middleware delegated to github.com/ovander/backendkit/jwtauth.
// Re-exported so all call sites within ascenda/internal/ continue using
// the "middleware" package without changes.

import (
	"github.com/ovander/backendkit/jwtauth"
	"github.com/sirupsen/logrus"
)

// SocrateClaims re-exports the JWT claims type from backendkit.
type SocrateClaims = jwtauth.SocrateClaims

// AuthMiddleware re-exports the JWKS-backed JWT middleware from backendkit.
type AuthMiddleware = jwtauth.Middleware

// NewAuthMiddleware creates a new AuthMiddleware using backendkit's implementation.
func NewAuthMiddleware(jwksURL string, issuer string, logger *logrus.Entry) *AuthMiddleware {
	return jwtauth.New(jwksURL, issuer, logger)
}
