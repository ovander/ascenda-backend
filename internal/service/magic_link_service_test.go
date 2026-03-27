package service

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ascenda/internal/model"
	"ascenda/internal/pkg/apierror"
)

// ── in-memory mock token repo ─────────────────────────────────────────────────

type mockTokenRepo struct {
	mu     sync.Mutex
	tokens map[string]*model.MagicLinkToken // key: tokenHash
}

func newMockTokenRepo() *mockTokenRepo {
	return &mockTokenRepo{tokens: make(map[string]*model.MagicLinkToken)}
}

func (m *mockTokenRepo) Create(t *model.MagicLinkToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[t.TokenHash] = t
	return nil
}

func (m *mockTokenRepo) FindByTokenHash(hash string) (*model.MagicLinkToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tokens[hash]
	if !ok {
		return nil, nil // not found → (nil, nil) matches real repo behaviour
	}
	if t.UsedAt != nil {
		return nil, nil // already used
	}
	if time.Now().After(t.ExpiresAt) {
		return nil, nil // expired
	}
	return t, nil
}

func (m *mockTokenRepo) MarkUsed(id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for _, t := range m.tokens {
		if t.ID == id {
			t.UsedAt = &now
			return nil
		}
	}
	return nil
}

// ── mock mailer ───────────────────────────────────────────────────────────────

type mockMailer struct {
	mu       sync.Mutex
	calls    []mockMailCall
	failNext bool
}

type mockMailCall struct {
	email       string
	callbackURL string
}

func (m *mockMailer) SendMagicLink(_ context.Context, email, callbackURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failNext {
		m.failNext = false
		return apierror.Internal("mailer error")
	}
	m.calls = append(m.calls, mockMailCall{email: email, callbackURL: callbackURL})
	return nil
}

func (m *mockMailer) lastCall() (mockMailCall, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.calls) == 0 {
		return mockMailCall{}, false
	}
	return m.calls[len(m.calls)-1], true
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newTestMagicLinkService(mailer SocrateMailer) (*MagicLinkService, *mockTokenRepo) {
	repo := newMockTokenRepo()
	logger := logrus.NewEntry(logrus.New())
	svc := NewMagicLinkService(mailer, repo, "http://localhost:8080", logger)
	return svc, repo
}

// ── SendMagicLink tests ───────────────────────────────────────────────────────

func TestMagicLinkService_Send_CallsMailer(t *testing.T) {
	mailer := &mockMailer{}
	svc, repo := newTestMagicLinkService(mailer)

	err := svc.SendMagicLink(context.Background(), "user@example.com", "/dashboard")
	require.NoError(t, err)

	// Token should be persisted.
	assert.Len(t, repo.tokens, 1)

	// Mailer should have been called once with the right email.
	call, ok := mailer.lastCall()
	require.True(t, ok)
	assert.Equal(t, "user@example.com", call.email)
	assert.Contains(t, call.callbackURL, "http://localhost:8080/auth/magic-link/verify?token=")
}

func TestMagicLinkService_Send_EmailNormalisedToLowercase(t *testing.T) {
	mailer := &mockMailer{}
	svc, repo := newTestMagicLinkService(mailer)

	err := svc.SendMagicLink(context.Background(), "  User@EXAMPLE.COM  ", "")
	require.NoError(t, err)

	// Stored email should be lowercase and trimmed.
	for _, tok := range repo.tokens {
		assert.Equal(t, "user@example.com", tok.Email)
	}
}

func TestMagicLinkService_Send_NoMailer_DoesNotPanic(t *testing.T) {
	// mailer = nil → dev mode, token still stored, no panic.
	svc, repo := newTestMagicLinkService(nil)

	err := svc.SendMagicLink(context.Background(), "dev@local.test", "")
	require.NoError(t, err)
	assert.Len(t, repo.tokens, 1)
}

func TestMagicLinkService_Send_MailerError_IsNonFatal(t *testing.T) {
	// Mailer failure must not bubble up — token is still stored.
	mailer := &mockMailer{failNext: true}
	svc, repo := newTestMagicLinkService(mailer)

	err := svc.SendMagicLink(context.Background(), "user@example.com", "")
	require.NoError(t, err) // error from mailer is swallowed
	assert.Len(t, repo.tokens, 1)
}

func TestMagicLinkService_Send_TTL(t *testing.T) {
	svc, repo := newTestMagicLinkService(nil)
	_ = svc.SendMagicLink(context.Background(), "u@x.com", "")
	for _, tok := range repo.tokens {
		// ExpiresAt should be roughly now + 15 minutes.
		diff := time.Until(tok.ExpiresAt)
		assert.Greater(t, diff, 14*time.Minute)
		assert.LessOrEqual(t, diff, 15*time.Minute+time.Second)
	}
}

// ── VerifyMagicLink tests ─────────────────────────────────────────────────────

func TestMagicLinkService_Verify_HappyPath(t *testing.T) {
	mailer := &mockMailer{}
	svc, _ := newTestMagicLinkService(mailer)

	// Send to obtain a real raw token via the verify URL.
	err := svc.SendMagicLink(context.Background(), "user@example.com", "/reports")
	require.NoError(t, err)

	// Extract raw token from the callback URL.
	call, _ := mailer.lastCall()
	rawToken := extractToken(call.callbackURL)
	require.NotEmpty(t, rawToken)

	result, err := svc.VerifyMagicLink(context.Background(), rawToken)
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", result.Email)
	assert.Equal(t, "/reports", result.RedirectURL)
}

func TestMagicLinkService_Verify_TokenSingleUse(t *testing.T) {
	mailer := &mockMailer{}
	svc, _ := newTestMagicLinkService(mailer)
	_ = svc.SendMagicLink(context.Background(), "u@x.com", "")
	call, _ := mailer.lastCall()
	rawToken := extractToken(call.callbackURL)

	// First verify: OK.
	_, err := svc.VerifyMagicLink(context.Background(), rawToken)
	require.NoError(t, err)

	// Second verify: must fail — token already used.
	_, err = svc.VerifyMagicLink(context.Background(), rawToken)
	require.Error(t, err)
	appErr, ok := err.(*apierror.AppError)
	require.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, appErr.StatusCode)
}

func TestMagicLinkService_Verify_UnknownToken_Returns401(t *testing.T) {
	svc, _ := newTestMagicLinkService(nil)
	_, err := svc.VerifyMagicLink(context.Background(), "deadbeefdeadbeef")
	require.Error(t, err)
	appErr, ok := err.(*apierror.AppError)
	require.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, appErr.StatusCode)
}

func TestMagicLinkService_Verify_ExpiredToken_Returns401(t *testing.T) {
	repo := newMockTokenRepo()
	logger := logrus.NewEntry(logrus.New())
	svc := NewMagicLinkService(nil, repo, "http://localhost", logger)

	// Manually insert an expired token.
	rawToken := "aabbccddeeff00112233445566778899aabbccddeeff001122334455667788990"
	tokenHash := hashToken(rawToken)
	expired := time.Now().Add(-1 * time.Minute)
	repo.tokens[tokenHash] = &model.MagicLinkToken{
		ID:        uuid.New(),
		Email:     "user@example.com",
		TokenHash: tokenHash,
		ExpiresAt: expired, // already expired
	}

	_, err := svc.VerifyMagicLink(context.Background(), rawToken)
	require.Error(t, err)
	appErr, ok := err.(*apierror.AppError)
	require.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, appErr.StatusCode)
}

// ── generateToken / hashToken unit tests ─────────────────────────────────────

func TestGenerateToken_UniqueEachCall(t *testing.T) {
	raw1, hash1, err := generateToken()
	require.NoError(t, err)
	raw2, hash2, err := generateToken()
	require.NoError(t, err)

	assert.NotEqual(t, raw1, raw2, "raw tokens should be unique")
	assert.NotEqual(t, hash1, hash2, "hashes should be unique")
	assert.Len(t, raw1, 64, "raw token should be 64 hex chars (32 bytes)")
	assert.Len(t, hash1, 64, "SHA-256 hex hash should be 64 chars")
}

func TestHashToken_Deterministic(t *testing.T) {
	h1 := hashToken("sometoken")
	h2 := hashToken("sometoken")
	assert.Equal(t, h1, h2)
}

func TestHashToken_DifferentInputs_DifferentOutputs(t *testing.T) {
	assert.NotEqual(t, hashToken("aaa"), hashToken("bbb"))
}

// ── helpers ───────────────────────────────────────────────────────────────────

// extractToken parses the raw token out of a verify URL such as:
// http://localhost:8080/auth/magic-link/verify?token=<rawToken>
func extractToken(verifyURL string) string {
	const prefix = "?token="
	idx := len(verifyURL) - 1
	for i, c := range verifyURL {
		if string(c) == "?" {
			idx = i
			break
		}
	}
	if idx == len(verifyURL)-1 {
		return ""
	}
	q := verifyURL[idx+1:]
	for _, part := range splitQuery(q) {
		if len(part) > 6 && part[:6] == "token=" {
			return part[6:]
		}
	}
	return ""
}

func splitQuery(q string) []string {
	var parts []string
	start := 0
	for i, c := range q {
		if c == '&' {
			parts = append(parts, q[start:i])
			start = i + 1
		}
	}
	parts = append(parts, q[start:])
	return parts
}
