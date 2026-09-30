package handler

import (
	"net/http"
	"strings"
	"testing"

	"ascenda/internal/service"
	"github.com/ovander/backendkit/socrate"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The browser can send any X-Forwarded-For or X-Real-IP. Socrate's Caddy trusts
// X-Forwarded-For from the apps VPS, so whatever Ascenda sends is what Socrate
// audits: it must be one address Ascenda resolved itself, never the browser's.
const forged = "6.6.6.6"

func TestAttribution_ForgedHeadersNeverReachSocrate(t *testing.T) {
	cases := []struct {
		name    string
		peer    string
		headers map[string]string
		wantXFF string // exact outgoing X-Forwarded-For; "" means none
	}{
		{
			name:    "direct peer (not loopback) with X-Forwarded-For: the peer is sent",
			peer:    "198.51.100.9:5555",
			headers: map[string]string{"X-Forwarded-For": forged},
			wantXFF: "198.51.100.9",
		},
		{
			name:    "direct peer with X-Real-IP: the peer is sent, X-Real-IP is not",
			peer:    "198.51.100.9:5555",
			headers: map[string]string{"X-Forwarded-For": forged, "X-Real-IP": forged},
			wantXFF: "198.51.100.9",
		},
		{
			name:    "from Caddy on loopback: the forwarded client is sent",
			peer:    "127.0.0.1:40000",
			headers: map[string]string{"X-Forwarded-For": "203.0.113.7"},
			wantXFF: "203.0.113.7",
		},
		{
			name:    "from Caddy, forged entry on the left: only the entry Caddy appended is sent",
			peer:    "127.0.0.1:40000",
			headers: map[string]string{"X-Forwarded-For": forged + ", 203.0.113.7"},
			wantXFF: "203.0.113.7",
		},
		{
			name:    "from Caddy with X-Real-IP: X-Real-IP is neither used nor forwarded",
			peer:    "127.0.0.1:40000",
			headers: map[string]string{"X-Forwarded-For": "203.0.113.7", "X-Real-IP": forged},
			wantXFF: "203.0.113.7",
		},
		{
			name:    "loopback without X-Forwarded-For but with X-Real-IP: nothing is sent",
			peer:    "127.0.0.1:40000",
			headers: map[string]string{"X-Real-IP": forged},
			wantXFF: "",
		},
		{
			name:    "from Caddy over IPv6 loopback",
			peer:    "[::1]:40000",
			headers: map[string]string{"X-Forwarded-For": "2001:db8::7"},
			wantXFF: "2001:db8::7",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newSocrateServer(t, map[string]socrateReply{"/oauth/token": {http.StatusOK, socrateTokenJSON}})
			h := newSocrateAuthHandler(t, f)

			w := serveWithHeaders(t, h.Refresh, tc.peer, tc.headers, RefreshRequest{RefreshToken: "rt"})

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			got := f.only(t)
			assert.Equal(t, tc.wantXFF, got.header.Get("X-Forwarded-For"))
			assert.NotContains(t, strings.Join(got.header.Values("X-Forwarded-For"), ","), forged)
			assert.Empty(t, got.header.Values("X-Real-IP"), "X-Real-IP is never forwarded")
			assert.Equal(t, "Mozilla/5.0 (Ascenda test)", got.header.Get("User-Agent"))
		})
	}
}

// Every call made on the user's behalf is attributed the same way.
func TestAttribution_EverySocrateCallSendsThePeerNotTheForgedAddress(t *testing.T) {
	f := newSocrateServer(t, map[string]socrateReply{
		"/oauth/token":                {http.StatusOK, socrateTokenJSON},
		"/oauth/revoke":               {http.StatusOK, `{}`},
		"/api/auth/magic-link/verify": {http.StatusOK, `{"access_token":"at","refresh_token":"rt","expires_in":900}`},
	})
	h := newSocrateAuthHandler(t, f)
	sc, err := socrate.NewClient(socrate.ClientConfig{BaseURL: f.srv.URL, ClientID: "ascenda-client", ClientSecret: "ascenda-secret"})
	require.NoError(t, err)
	logger := logrus.NewEntry(logrus.New())
	ml := NewMagicLinkHandler(service.NewMagicLinkService(sc, logger), nil, logger)

	forgedHeaders := map[string]string{"X-Forwarded-For": forged, "X-Real-IP": forged}
	const peer = "198.51.100.9:5555"
	serveWithHeaders(t, h.Callback, peer, forgedHeaders, CallbackRequest{Code: "c"})
	serveWithHeaders(t, h.Refresh, peer, forgedHeaders, RefreshRequest{RefreshToken: "rt"})
	serveWithHeaders(t, h.Logout, peer, forgedHeaders, LogoutRequest{Token: "rt"})
	serveWithHeaders(t, ml.Verify, peer, forgedHeaders, VerifyMagicLinkRequest{Token: "ml"})

	f.mu.Lock()
	defer f.mu.Unlock()
	require.Len(t, f.requests, 4)
	for _, req := range f.requests {
		assert.Equal(t, "198.51.100.9", req.header.Get("X-Forwarded-For"), req.path)
		assert.Empty(t, req.header.Values("X-Real-IP"), req.path)
	}
}
