package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

func newMagicLinkTestHandler(f *fakeSocrate) *MagicLinkHandler {
	logger := logrus.NewEntry(logrus.New())
	return NewMagicLinkHandler(service.NewMagicLinkService(f, logger), logger)
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

// ── Send ─────────────────────────────────────────────────────────────────────

func TestMagicLinkHandler_Send_AsksSocrateAndReturns202(t *testing.T) {
	f := &fakeSocrate{}
	w := postJSON(t, newMagicLinkTestHandler(f).Send, "/auth/magic-link", map[string]string{"email": "User@Example.com"})

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Contains(t, resp["message"], "sign-in link")
	assert.Equal(t, []string{"user@example.com"}, f.sentTo)
}

func TestMagicLinkHandler_Send_InvalidEmailIs422(t *testing.T) {
	for _, body := range []map[string]string{{}, {"email": "not-an-email"}} {
		f := &fakeSocrate{}
		w := postJSON(t, newMagicLinkTestHandler(f).Send, "/auth/magic-link", body)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		assert.Empty(t, f.sentTo)
	}
}
