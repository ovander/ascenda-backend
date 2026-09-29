package router

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAuthRoutesAreCodeFlowOnly pins the /auth surface to the Socrate
// contract: sign-in is the authorization-code flow with PKCE (started by the
// SPA, exchanged at /auth/callback) or a Socrate magic link. There is no
// password or implicit login route, and no route that builds an authorize URL
// without PKCE (the former POST /auth/login).
func TestAuthRoutesAreCodeFlowOnly(t *testing.T) {
	var got []string
	for op := range routerOperations(t) {
		if strings.Contains(op, " /auth/") {
			got = append(got, op)
		}
	}
	sort.Strings(got)

	assert.Equal(t, []string{
		"POST /auth/callback",
		"POST /auth/logout",
		"POST /auth/magic-link",
		"POST /auth/magic-link/verify",
		"POST /auth/refresh",
		"POST /auth/register",
	}, got)
}
