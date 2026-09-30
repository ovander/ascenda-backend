package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ascenda/internal/handler"
	"ascenda/internal/middleware"
	"github.com/stretchr/testify/assert"
)

// TestRemovedLoginRouteIsGone checks, through the production router, that the
// former POST /auth/login (an authorize URL without PKCE or state, which Socrate
// refuses with require_pkce=true) is no longer served, whatever the method.
func TestRemovedLoginRouteIsGone(t *testing.T) {
	r := newProductionRouter(&handler.HandlerBundle{}, middleware.TrustedProxies{})

	for _, method := range []string{http.MethodPost, http.MethodGet} {
		req := httptest.NewRequest(method, "/auth/login", strings.NewReader(`{"email":"user@example.com"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Contains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed}, w.Code,
			"%s /auth/login answered %d: %s", method, w.Code, w.Body.String())
	}
}

// TestNoRouteReturnsTokensToTheBrowser checks, through the production router,
// that the endpoints that used to hand OAuth tokens to the browser are gone,
// whatever the method: the code exchange, the refresh proxy, the logout that
// took a token, and the magic-link redemption. The BFF (/bff) replaces them.
func TestNoRouteReturnsTokensToTheBrowser(t *testing.T) {
	r := newProductionRouter(&handler.HandlerBundle{}, middleware.TrustedProxies{})

	for _, path := range []string{"/auth/callback", "/auth/refresh", "/auth/logout", "/auth/magic-link/verify"} {
		for _, method := range []string{http.MethodPost, http.MethodGet} {
			req := httptest.NewRequest(method, path, strings.NewReader(`{"code":"c","refreshToken":"r","token":"t"}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Contains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed}, w.Code,
				"%s %s answered %d: %s", method, path, w.Code, w.Body.String())
			assert.NotContains(t, strings.ToLower(w.Body.String()), "token\":")
		}
	}
}
