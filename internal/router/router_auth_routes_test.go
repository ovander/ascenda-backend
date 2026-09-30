package router

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAuthRoutesAreCodeFlowOnly pins the sign-in surface. Sign-in is the
// authorization-code flow with PKCE run by the BFF (/bff/login, /bff/callback)
// or a Socrate magic link redeemed into a session (/bff/magic-link/verify).
// /auth keeps only what returns no token: registration and the request for a
// magic-link e-mail. There is no password or implicit login route.
func TestAuthRoutesAreCodeFlowOnly(t *testing.T) {
	var got []string
	for op := range routerOperations(t) {
		if strings.Contains(op, " /auth/") || strings.Contains(op, " /bff/") {
			got = append(got, op)
		}
	}
	sort.Strings(got)

	assert.Equal(t, []string{
		"GET /bff/callback",
		"GET /bff/login",
		"GET /bff/session",
		"POST /auth/magic-link",
		"POST /auth/register",
		"POST /bff/logout",
		"POST /bff/magic-link/verify",
	}, got)
}
