package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ascenda/internal/model"
	"ascenda/internal/repo"
	"ascenda/internal/service"
	"github.com/ovander/backendkit/socrate"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSocrate satisfies service.SocrateMagicLink.
type fakeSocrate struct {
	sentTo    []string
	verified  []string
	result    *socrate.LoginResult
	verifyErr error
}

func (f *fakeSocrate) SendMagicLink(_ context.Context, email string) (*socrate.MagicLinkResponse, error) {
	f.sentTo = append(f.sentTo, email)
	return &socrate.MagicLinkResponse{}, nil
}

func (f *fakeSocrate) VerifyMagicLink(_ context.Context, token string) (*socrate.LoginResult, error) {
	f.verified = append(f.verified, token)
	return f.result, f.verifyErr
}

// magicLinkUserRepo implements the two UserRepository methods the ID-token
// enrichment uses; any other call panics on the nil embedded interface.
type magicLinkUserRepo struct {
	repo.UserRepository
	user    *model.User
	updated *model.User
}

func (r *magicLinkUserRepo) GetByExternalID(externalID string) (*model.User, error) {
	if r.user != nil && r.user.ExternalID == externalID {
		return r.user, nil
	}
	return nil, nil
}

func (r *magicLinkUserRepo) Update(u *model.User) error {
	r.updated = u
	return nil
}

func newMagicLinkTestHandler(f *fakeSocrate, users repo.UserRepository) *MagicLinkHandler {
	logger := logrus.NewEntry(logrus.New())
	return NewMagicLinkHandler(service.NewMagicLinkService(f, logger), users, logger)
}

func postJSON(t *testing.T, fn http.HandlerFunc, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	fn(w, req)
	return w
}

// unsignedIDToken builds a JWT-shaped string whose payload carries claims; the
// enrichment decodes the payload only.
func unsignedIDToken(t *testing.T, claims map[string]string) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"RS256"}`)) + "." + enc(payload) + ".sig"
}

// ── Send ─────────────────────────────────────────────────────────────────────

func TestMagicLinkHandler_Send_AsksSocrateAndReturns202(t *testing.T) {
	f := &fakeSocrate{}
	w := postJSON(t, newMagicLinkTestHandler(f, nil).Send, "/auth/magic-link", map[string]string{"email": "User@Example.com"})

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Contains(t, resp["message"], "sign-in link")
	assert.Equal(t, []string{"user@example.com"}, f.sentTo)
}

func TestMagicLinkHandler_Send_InvalidEmailIs422(t *testing.T) {
	for _, body := range []map[string]string{{}, {"email": "not-an-email"}} {
		f := &fakeSocrate{}
		w := postJSON(t, newMagicLinkTestHandler(f, nil).Send, "/auth/magic-link", body)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		assert.Empty(t, f.sentTo)
	}
}

// ── Verify ───────────────────────────────────────────────────────────────────

func TestMagicLinkHandler_Verify_ReturnsTokensLikeCallback(t *testing.T) {
	f := &fakeSocrate{result: &socrate.LoginResult{AccessToken: "at", RefreshToken: "rt", ExpiresIn: 900}}
	w := postJSON(t, newMagicLinkTestHandler(f, nil).Verify, "/auth/magic-link/verify", map[string]string{"token": "tok"})

	require.Equal(t, http.StatusOK, w.Code)
	var resp TokenResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, TokenResponse{AccessToken: "at", RefreshToken: "rt", ExpiresIn: 900}, resp)
	assert.Equal(t, []string{"tok"}, f.verified)
}

func TestMagicLinkHandler_Verify_InvalidTokenIs401(t *testing.T) {
	for _, err := range []error{socrate.ErrMagicLinkInvalid, socrate.ErrMagicLinkAlreadyUsed} {
		f := &fakeSocrate{verifyErr: err}
		w := postJSON(t, newMagicLinkTestHandler(f, nil).Verify, "/auth/magic-link/verify", map[string]string{"token": "tok"})
		assert.Equal(t, http.StatusUnauthorized, w.Code, "for %v", err)
	}
}

func TestMagicLinkHandler_Verify_MissingTokenIs422(t *testing.T) {
	f := &fakeSocrate{}
	w := postJSON(t, newMagicLinkTestHandler(f, nil).Verify, "/auth/magic-link/verify", map[string]string{})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Empty(t, f.verified)
}

func TestMagicLinkHandler_Verify_EnrichesUserFromIDToken(t *testing.T) {
	users := &magicLinkUserRepo{user: &model.User{ExternalID: "42"}}
	f := &fakeSocrate{result: &socrate.LoginResult{
		AccessToken:  "at",
		RefreshToken: "rt",
		IDToken:      unsignedIDToken(t, map[string]string{"sub": "42", "email": "ada@example.com", "name": "Ada"}),
	}}
	w := postJSON(t, newMagicLinkTestHandler(f, users).Verify, "/auth/magic-link/verify", map[string]string{"token": "tok"})

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, users.updated)
	assert.Equal(t, "ada@example.com", users.updated.Email)
	assert.Equal(t, "Ada", users.updated.Name)
}
