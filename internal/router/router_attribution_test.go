package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ascenda/internal/config"
	"ascenda/internal/handler"
	"ascenda/internal/middleware"
	"ascenda/internal/service"
	"github.com/ovander/backendkit/socrate"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuthRoutesAttributeSocrateCalls checks the wiring end to end: a refresh
// through the production router reaches Socrate with the browser's address
// (resolved through the trusted proxy) and User-Agent.
func TestAuthRoutesAttributeSocrateCalls(t *testing.T) {
	var gotXFF, gotUA string
	socrateSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotXFF, gotUA = r.Header.Get("X-Forwarded-For"), r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at","refresh_token":"rt-new","expires_in":900}`))
	}))
	defer socrateSrv.Close()

	sc, err := socrate.NewClient(socrate.ClientConfig{BaseURL: socrateSrv.URL, ClientID: "c", ClientSecret: "s"})
	require.NoError(t, err)
	le := logrus.NewEntry(logrus.New())
	handlers := &handler.HandlerBundle{}
	handlers.Admin.Auth = handler.NewAuthHandler(&config.Config{}, service.NewTokenService(sc, le), nil, nil, le)
	trusted, err := middleware.ParseTrustedProxies([]string{"127.0.0.1/32"})
	require.NoError(t, err)
	r := newProductionRouter(handlers, trusted)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(`{"refreshToken":"rt-old"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (router test)")
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	req.RemoteAddr = "127.0.0.1:40000" // Caddy on loopback
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"refreshToken":"rt-new"`)
	assert.Equal(t, "203.0.113.7", gotXFF)
	assert.Equal(t, "Mozilla/5.0 (router test)", gotUA)
}
