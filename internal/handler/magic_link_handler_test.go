package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ascenda/internal/config"
	"ascenda/internal/model"
	"ascenda/internal/service"
)

// ── in-memory token store (satisfies the private magicLinkTokenRepo interface
// via Go's structural typing — method signatures must match exactly) ───────────

type handlerTestTokenStore struct {
	mu     sync.Mutex
	tokens map[string]*model.MagicLinkToken
}

func newHandlerTestTokenStore() *handlerTestTokenStore {
	return &handlerTestTokenStore{tokens: make(map[string]*model.MagicLinkToken)}
}

func (s *handlerTestTokenStore) Create(t *model.MagicLinkToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[t.TokenHash] = t
	return nil
}

func (s *handlerTestTokenStore) FindByTokenHash(hash string) (*model.MagicLinkToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tokens[hash]
	if !ok || t.UsedAt != nil || time.Now().After(t.ExpiresAt) {
		return nil, nil
	}
	return t, nil
}

func (s *handlerTestTokenStore) MarkUsed(id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for _, t := range s.tokens {
		if t.ID == id {
			t.UsedAt = &now
			return nil
		}
	}
	return nil
}

// ── capturing mailer (satisfies service.SocrateMailer) ───────────────────────

type capturingMailer struct {
	mu       sync.Mutex
	lastURL  string
	lastMail string
}

func (m *capturingMailer) SendMagicLink(_ context.Context, email, callbackURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastMail = email
	m.lastURL = callbackURL
	return nil
}

func (m *capturingMailer) capturedToken() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	// URL format: http://localhost:8080/auth/magic-link/verify?token=<rawToken>
	idx := strings.LastIndex(m.lastURL, "?token=")
	if idx < 0 {
		return ""
	}
	return m.lastURL[idx+7:]
}

// ── test config ───────────────────────────────────────────────────────────────

func testMagicLinkCfg() *config.Config {
	return &config.Config{
		Socrate: config.SocrateConfig{
			BaseURL:     "https://auth.example.com",
			ClientID:    "test-client-id",
			RedirectURL: "http://localhost:5173/callback",
		},
		AllowedOrigins: []string{"http://localhost:5173"},
	}
}

// newHandlerWithMailer wires a MagicLinkHandler with a capturing mailer
// so the raw token can be recovered from the sent email URL.
func newHandlerWithMailer() (*MagicLinkHandler, *capturingMailer, *handlerTestTokenStore) {
	store := newHandlerTestTokenStore()
	mailer := &capturingMailer{}
	logger := logrus.NewEntry(logrus.New())
	svc := service.NewMagicLinkService(mailer, store, "http://localhost:8080", logger)
	h := NewMagicLinkHandler(svc, testMagicLinkCfg(), logger)
	return h, mailer, store
}

// newHandlerNoMailer is used where we only need to test validation / error paths.
func newHandlerNoMailer() *MagicLinkHandler {
	store := newHandlerTestTokenStore()
	logger := logrus.NewEntry(logrus.New())
	svc := service.NewMagicLinkService(nil, store, "http://localhost:8080", logger)
	return NewMagicLinkHandler(svc, testMagicLinkCfg(), logger)
}

// ── Send tests ────────────────────────────────────────────────────────────────

func TestMagicLinkHandler_Send_ValidRequest_Returns202(t *testing.T) {
	h := newHandlerNoMailer()

	body, _ := json.Marshal(map[string]string{
		"email":    "user@example.com",
		"redirect": "/dashboard",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/magic-link", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Send(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Contains(t, resp["message"], "sign-in link")
}

func TestMagicLinkHandler_Send_MissingEmail_Returns422(t *testing.T) {
	h := newHandlerNoMailer()

	body, _ := json.Marshal(map[string]string{"redirect": "/"})
	req := httptest.NewRequest(http.MethodPost, "/auth/magic-link", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Send(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestMagicLinkHandler_Send_InvalidEmail_Returns422(t *testing.T) {
	h := newHandlerNoMailer()

	body, _ := json.Marshal(map[string]string{"email": "not-an-email"})
	req := httptest.NewRequest(http.MethodPost, "/auth/magic-link", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Send(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestMagicLinkHandler_Send_InvalidJSON_Returns4xx(t *testing.T) {
	h := newHandlerNoMailer()

	req := httptest.NewRequest(http.MethodPost, "/auth/magic-link", bytes.NewReader([]byte("{bad")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Send(w, req)

	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusUnprocessableEntity)
}

// ── Verify tests ──────────────────────────────────────────────────────────────

func TestMagicLinkHandler_Verify_MissingToken_RedirectsWithError(t *testing.T) {
	h := newHandlerNoMailer()

	req := httptest.NewRequest(http.MethodGet, "/auth/magic-link/verify", nil)
	w := httptest.NewRecorder()

	h.Verify(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "magic_error=")
}

func TestMagicLinkHandler_Verify_InvalidToken_RedirectsWithError(t *testing.T) {
	h := newHandlerNoMailer()

	req := httptest.NewRequest(http.MethodGet, "/auth/magic-link/verify?token=doesnotexist", nil)
	w := httptest.NewRecorder()

	h.Verify(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "magic_error=")
}

func TestMagicLinkHandler_Verify_ValidToken_RedirectsToSocrateOAuth(t *testing.T) {
	h, mailer, _ := newHandlerWithMailer()

	// Step 1: send a magic link for the email.
	sendBody, _ := json.Marshal(map[string]string{
		"email":    "user@example.com",
		"redirect": "/reports",
	})
	sendReq := httptest.NewRequest(http.MethodPost, "/auth/magic-link", bytes.NewReader(sendBody))
	sendReq.Header.Set("Content-Type", "application/json")
	sendW := httptest.NewRecorder()
	h.Send(sendW, sendReq)
	require.Equal(t, http.StatusAccepted, sendW.Code)

	// Step 2: recover the raw token from the captured email URL.
	rawToken := mailer.capturedToken()
	require.NotEmpty(t, rawToken, "mailer should have captured the token")

	// Step 3: verify the token.
	verifyReq := httptest.NewRequest(http.MethodGet, "/auth/magic-link/verify?token="+rawToken, nil)
	verifyW := httptest.NewRecorder()
	h.Verify(verifyW, verifyReq)

	assert.Equal(t, http.StatusFound, verifyW.Code)
	loc := verifyW.Header().Get("Location")
	assert.Contains(t, loc, "https://auth.example.com/oauth/authorize")
	assert.Contains(t, loc, "login_hint=user%40example.com")
	assert.Contains(t, loc, "client_id=test-client-id")
	assert.Contains(t, loc, "state=") // redirect URL passed as state
}

func TestMagicLinkHandler_Verify_TokenSingleUse(t *testing.T) {
	h, mailer, _ := newHandlerWithMailer()

	sendBody, _ := json.Marshal(map[string]string{"email": "u@x.com"})
	sendReq := httptest.NewRequest(http.MethodPost, "/auth/magic-link", bytes.NewReader(sendBody))
	sendReq.Header.Set("Content-Type", "application/json")
	h.Send(httptest.NewRecorder(), sendReq)

	rawToken := mailer.capturedToken()
	require.NotEmpty(t, rawToken)

	// First use: succeeds.
	r1 := httptest.NewRequest(http.MethodGet, "/auth/magic-link/verify?token="+rawToken, nil)
	w1 := httptest.NewRecorder()
	h.Verify(w1, r1)
	assert.Equal(t, http.StatusFound, w1.Code)
	assert.NotContains(t, w1.Header().Get("Location"), "magic_error=")

	// Second use: must redirect with error.
	r2 := httptest.NewRequest(http.MethodGet, "/auth/magic-link/verify?token="+rawToken, nil)
	w2 := httptest.NewRecorder()
	h.Verify(w2, r2)
	assert.Equal(t, http.StatusFound, w2.Code)
	assert.Contains(t, w2.Header().Get("Location"), "magic_error=")
}

// ── sanitiseRedirect tests ────────────────────────────────────────────────────

func TestSanitiseRedirect_Empty(t *testing.T) {
	assert.Equal(t, "", sanitiseRedirect("", nil))
}

func TestSanitiseRedirect_Slash(t *testing.T) {
	assert.Equal(t, "/", sanitiseRedirect("/", nil))
}

func TestSanitiseRedirect_RelativePath(t *testing.T) {
	assert.Equal(t, "/dashboard", sanitiseRedirect("/dashboard", nil))
}

func TestSanitiseRedirect_AllowedAbsoluteURL(t *testing.T) {
	origins := []string{"http://localhost:5173"}
	result := sanitiseRedirect("http://localhost:5173/app", origins)
	assert.Equal(t, "http://localhost:5173/app", result)
}

func TestSanitiseRedirect_DisallowedOrigin(t *testing.T) {
	origins := []string{"http://localhost:5173"}
	result := sanitiseRedirect("https://evil.example.com/steal", origins)
	assert.Equal(t, "/", result)
}

func TestSanitiseRedirect_DoubleSlashOpenRedirect(t *testing.T) {
	// //evil.com is a classic open-redirect vector.
	assert.Equal(t, "/", sanitiseRedirect("//evil.example.com", nil))
}

func TestSanitiseRedirect_InvalidURL(t *testing.T) {
	assert.Equal(t, "/", sanitiseRedirect("://no-scheme", nil))
}
