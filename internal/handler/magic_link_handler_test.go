package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"ascenda/internal/config"
	"ascenda/internal/model"
	"ascenda/internal/service"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/socrate"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	lastMail string
}

func (m *capturingMailer) SendMagicLink(_ context.Context, email string) (*socrate.MagicLinkResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastMail = email
	return &socrate.MagicLinkResponse{}, nil
}

// seedToken inserts a known raw token directly into a handlerTestTokenStore
// so verify tests can use a predictable token without relying on mailer capture.
func seedToken(store *handlerTestTokenStore, rawToken, email, redirectURL string) {
	h := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(h[:])
	store.mu.Lock()
	store.tokens[tokenHash] = &model.MagicLinkToken{
		ID:          uuid.New(),
		Email:       email,
		TokenHash:   tokenHash,
		RedirectURL: redirectURL,
		ExpiresAt:   time.Now().Add(15 * time.Minute),
	}
	store.mu.Unlock()
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
	h, _, store := newHandlerWithMailer()

	// Pre-seed a known raw token directly into the store.
	const rawToken = "aabbccdd1122334455667788aabbccdd1122334455667788aabbccdd11223344"
	seedToken(store, rawToken, "user@example.com", "/reports")

	// Verify the token.
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
	h, _, store := newHandlerWithMailer()

	const rawToken = "deadbeef1234567890abcdef1234567890abcdef1234567890abcdef12345678"
	seedToken(store, rawToken, "u@x.com", "")
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
