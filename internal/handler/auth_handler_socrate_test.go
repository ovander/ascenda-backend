package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"ascenda/internal/config"
	"ascenda/internal/middleware"
	"ascenda/internal/service"
	"github.com/ovander/backendkit/socrate"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// socrateServer is an HTTP stand-in for Socrate: it records the requests Socrate receives and answers with the
// status and body set for each path.
type socrateServer struct {
	srv       *httptest.Server
	mu        sync.Mutex
	requests  []recordedRequest
	responses map[string]socrateReply
}

type recordedRequest struct {
	path   string
	header http.Header
	form   url.Values
	body   []byte
}

type socrateReply struct {
	status int
	body   string
}

func newSocrateServer(t *testing.T, responses map[string]socrateReply) *socrateServer {
	t.Helper()
	f := &socrateServer{responses: responses}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		f.mu.Lock()
		f.requests = append(f.requests, recordedRequest{path: r.URL.Path, header: r.Header.Clone(), form: form, body: body})
		f.mu.Unlock()
		resp, ok := f.responses[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.status)
		_, _ = w.Write([]byte(resp.body))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *socrateServer) only(t *testing.T) recordedRequest {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	require.Len(t, f.requests, 1, "Socrate should receive exactly one request")
	return f.requests[0]
}

const socrateTokenJSON = `{"access_token":"at-123","refresh_token":"rt-456","id_token":"","expires_in":900,"token_type":"Bearer"}`

// newSocrateAuthHandler wires AuthHandler to the fake Socrate through the real
// backendkit client, the way service.NewServiceBundle does.
func newSocrateAuthHandler(t *testing.T, f *socrateServer) *AuthHandler {
	t.Helper()
	sc, err := socrate.NewClient(socrate.ClientConfig{
		BaseURL:      f.srv.URL,
		ClientID:     "ascenda-client",
		ClientSecret: "ascenda-secret",
	})
	require.NoError(t, err)
	logger := logrus.NewEntry(logrus.New())
	cfg := &config.Config{Socrate: config.SocrateConfig{
		BaseURL:      f.srv.URL,
		ClientID:     "ascenda-client",
		ClientSecret: "ascenda-secret",
		RedirectURL:  "https://app.example.com/callback",
	}}
	return NewAuthHandler(cfg, service.NewTokenService(sc, logger), nil, nil, logger)
}

// serveWithAttribution runs h behind middleware.SocrateClientAttribution, with
// loopback as the only trusted proxy (the production default).
func serveWithAttribution(t *testing.T, h http.HandlerFunc, remoteAddr, xff string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	trusted, err := middleware.ParseTrustedProxies([]string{"127.0.0.1/32"})
	require.NoError(t, err)
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/auth/x", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Ascenda test)")
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	w := httptest.NewRecorder()
	middleware.SocrateClientAttribution(trusted)(h).ServeHTTP(w, req)
	return w
}

func decodeTokenResponse(t *testing.T, w *httptest.ResponseRecorder) TokenResponse {
	t.Helper()
	var got TokenResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	return got
}

func TestCallback_ExchangesCodeThroughSocrateClient(t *testing.T) {
	f := newSocrateServer(t, map[string]socrateReply{"/oauth/token": {http.StatusOK, socrateTokenJSON}})
	h := newSocrateAuthHandler(t, f)

	w := serveWithAttribution(t, h.Callback, "127.0.0.1:40000", "203.0.113.7",
		CallbackRequest{Code: "code-1", CodeVerifier: "verifier-1"})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, TokenResponse{AccessToken: "at-123", RefreshToken: "rt-456", ExpiresIn: 900}, decodeTokenResponse(t, w))

	got := f.only(t)
	assert.Equal(t, "/oauth/token", got.path)
	assert.Equal(t, "authorization_code", got.form.Get("grant_type"))
	assert.Equal(t, "code-1", got.form.Get("code"))
	assert.Equal(t, "verifier-1", got.form.Get("code_verifier"))
	assert.Equal(t, "https://app.example.com/callback", got.form.Get("redirect_uri"), "defaults to SOCRATE_REDIRECT_URL")
	assert.Equal(t, "ascenda-client", got.form.Get("client_id"))
	assert.Equal(t, "ascenda-secret", got.form.Get("client_secret"))
	// Client attribution: Socrate sees the browser, not this server.
	assert.Equal(t, "203.0.113.7", got.header.Get("X-Forwarded-For"))
	assert.Equal(t, "Mozilla/5.0 (Ascenda test)", got.header.Get("User-Agent"))
}

func TestCallback_AttributionIgnoresForwardedForFromUntrustedPeer(t *testing.T) {
	f := newSocrateServer(t, map[string]socrateReply{"/oauth/token": {http.StatusOK, socrateTokenJSON}})
	h := newSocrateAuthHandler(t, f)

	// A browser connecting directly cannot choose the address Socrate audits it as.
	w := serveWithAttribution(t, h.Callback, "198.51.100.9:5555", "1.2.3.4",
		CallbackRequest{Code: "code-1", RedirectURI: "https://app.example.com/other"})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	got := f.only(t)
	assert.Equal(t, "198.51.100.9", got.header.Get("X-Forwarded-For"))
	assert.Equal(t, "https://app.example.com/other", got.form.Get("redirect_uri"))
	assert.Empty(t, got.form.Get("code_verifier"))
}

func TestCallback_RejectedCodeIs401(t *testing.T) {
	f := newSocrateServer(t, map[string]socrateReply{
		"/oauth/token": {http.StatusBadRequest, `{"error":"invalid_grant","error_description":"code expired"}`},
	})
	h := newSocrateAuthHandler(t, f)

	w := serveWithAttribution(t, h.Callback, "127.0.0.1:40000", "203.0.113.7", CallbackRequest{Code: "stale"})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "token exchange failed")
	assert.NotContains(t, w.Body.String(), "code expired", "Socrate's error detail stays in the log")
}

func TestRefresh_ReturnsRotatedTokens(t *testing.T) {
	f := newSocrateServer(t, map[string]socrateReply{"/oauth/token": {http.StatusOK, socrateTokenJSON}})
	h := newSocrateAuthHandler(t, f)

	w := serveWithAttribution(t, h.Refresh, "127.0.0.1:40000", "203.0.113.7", RefreshRequest{RefreshToken: "rt-old"})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, TokenResponse{AccessToken: "at-123", RefreshToken: "rt-456", ExpiresIn: 900}, decodeTokenResponse(t, w))
	got := f.only(t)
	assert.Equal(t, "refresh_token", got.form.Get("grant_type"))
	assert.Equal(t, "rt-old", got.form.Get("refresh_token"))
	assert.Equal(t, "ascenda-secret", got.form.Get("client_secret"))
	assert.Equal(t, "203.0.113.7", got.header.Get("X-Forwarded-For"))
	assert.Equal(t, "Mozilla/5.0 (Ascenda test)", got.header.Get("User-Agent"))
}

func TestRefresh_SpentRefreshTokenIs401(t *testing.T) {
	f := newSocrateServer(t, map[string]socrateReply{
		"/oauth/token": {http.StatusBadRequest, `{"error":"invalid_grant"}`},
	})
	h := newSocrateAuthHandler(t, f)

	w := serveWithAttribution(t, h.Refresh, "127.0.0.1:40000", "", RefreshRequest{RefreshToken: "rt-spent"})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "token refresh failed")
}

func TestRefresh_SocrateUnreachableIs500(t *testing.T) {
	f := newSocrateServer(t, nil)
	h := newSocrateAuthHandler(t, f)
	f.srv.Close()

	w := serveWithAttribution(t, h.Refresh, "127.0.0.1:40000", "", RefreshRequest{RefreshToken: "rt"})

	assert.Equal(t, http.StatusInternalServerError, w.Code) // apierror keeps 5xx messages generic
}

func TestLogout_RevokesThroughSocrateClient(t *testing.T) {
	f := newSocrateServer(t, map[string]socrateReply{"/oauth/revoke": {http.StatusOK, `{}`}})
	h := newSocrateAuthHandler(t, f)

	w := serveWithAttribution(t, h.Logout, "127.0.0.1:40000", "203.0.113.7", LogoutRequest{Token: "rt-456"})

	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	got := f.only(t)
	assert.Equal(t, "/oauth/revoke", got.path)
	assert.Equal(t, "rt-456", got.form.Get("token"))
	assert.Equal(t, "ascenda-client", got.form.Get("client_id"))
	assert.Equal(t, "ascenda-secret", got.form.Get("client_secret"))
	assert.Equal(t, "203.0.113.7", got.header.Get("X-Forwarded-For"))
	assert.Equal(t, "Mozilla/5.0 (Ascenda test)", got.header.Get("User-Agent"))
}

func TestLogout_RevocationFailureIs500(t *testing.T) {
	f := newSocrateServer(t, map[string]socrateReply{"/oauth/revoke": {http.StatusInternalServerError, `oops`}})
	h := newSocrateAuthHandler(t, f)

	w := serveWithAttribution(t, h.Logout, "127.0.0.1:40000", "", LogoutRequest{Token: "rt"})

	assert.Equal(t, http.StatusInternalServerError, w.Code) // apierror keeps 5xx messages generic
}

func TestMagicLinkVerify_IsAttributed(t *testing.T) {
	f := newSocrateServer(t, map[string]socrateReply{
		"/api/auth/magic-link/verify": {http.StatusOK, `{"access_token":"at-ml","refresh_token":"rt-ml","expires_in":900,"user_id":7}`},
	})
	sc, err := socrate.NewClient(socrate.ClientConfig{BaseURL: f.srv.URL, ClientID: "ascenda-client", ClientSecret: "ascenda-secret"})
	require.NoError(t, err)
	logger := logrus.NewEntry(logrus.New())
	h := NewMagicLinkHandler(service.NewMagicLinkService(sc, logger), nil, logger)

	w := serveWithAttribution(t, h.Verify, "127.0.0.1:40000", "203.0.113.7", VerifyMagicLinkRequest{Token: "ml-token"})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "at-ml", decodeTokenResponse(t, w).AccessToken)
	got := f.only(t)
	assert.Equal(t, "203.0.113.7", got.header.Get("X-Forwarded-For"))
	assert.Equal(t, "Mozilla/5.0 (Ascenda test)", got.header.Get("User-Agent"))
}
