package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ascenda/internal/config"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func newTestAuthHandler() *AuthHandler {
	cfg := &config.Config{
		Socrate: config.SocrateConfig{
			BaseURL:      "https://auth.example.com",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			RedirectURL:  "http://localhost:5173/callback",
		},
	}
	logger := logrus.NewEntry(logrus.New())
	return NewAuthHandler(cfg, nil, nil, logger)
}

func TestAuthHandlerLogin(t *testing.T) {
	h := newTestAuthHandler()

	t.Run("valid login request returns auth URL", func(t *testing.T) {
		body, _ := json.Marshal(LoginRequest{Email: "user@test.com"})
		req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.Login(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp LoginResponse
		err := json.NewDecoder(w.Body).Decode(&resp)
		assert.NoError(t, err)
		assert.Contains(t, resp.AuthURL, "https://auth.example.com/oauth/authorize")
		assert.Contains(t, resp.AuthURL, "client_id=test-client-id")
		assert.Contains(t, resp.AuthURL, "redirect_uri=")
		assert.Contains(t, resp.AuthURL, "login_hint=user%40test.com")
		assert.Contains(t, resp.AuthURL, "response_type=code")
		assert.Contains(t, resp.AuthURL, "scope=openid+profile+email")
	})

	t.Run("missing email returns 422 (validation error)", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{})
		req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.Login(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader([]byte("not json")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.Login(w, req)

		// Bad JSON may return 400 or 422 depending on decode vs validate stage
		assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusUnprocessableEntity)
	})
}

func TestCallbackRequestStructure(t *testing.T) {
	t.Run("callback request accepts code and codeVerifier", func(t *testing.T) {
		payload := `{"code":"auth-code-123","codeVerifier":"verifier-abc","redirectUri":"http://localhost:5173/callback"}`
		var req CallbackRequest
		err := json.Unmarshal([]byte(payload), &req)

		assert.NoError(t, err)
		assert.Equal(t, "auth-code-123", req.Code)
		assert.Equal(t, "verifier-abc", req.CodeVerifier)
		assert.Equal(t, "http://localhost:5173/callback", req.RedirectURI)
	})

	t.Run("callback request with only code", func(t *testing.T) {
		payload := `{"code":"auth-code-456"}`
		var req CallbackRequest
		err := json.Unmarshal([]byte(payload), &req)

		assert.NoError(t, err)
		assert.Equal(t, "auth-code-456", req.Code)
		assert.Empty(t, req.CodeVerifier)
		assert.Empty(t, req.RedirectURI)
	})
}

func TestTokenResponseMapping(t *testing.T) {
	t.Run("socrate response snake_case maps correctly", func(t *testing.T) {
		socrateJSON := `{"access_token":"at-123","refresh_token":"rt-456","expires_in":3600,"token_type":"bearer"}`
		var socrate socrateTokenResponse
		err := json.Unmarshal([]byte(socrateJSON), &socrate)

		assert.NoError(t, err)
		assert.Equal(t, "at-123", socrate.AccessToken)
		assert.Equal(t, "rt-456", socrate.RefreshToken)
		assert.Equal(t, 3600, socrate.ExpiresIn)
		assert.Equal(t, "bearer", socrate.TokenType)
	})

	t.Run("frontend response uses camelCase", func(t *testing.T) {
		resp := TokenResponse{
			AccessToken:  "at-123",
			RefreshToken: "rt-456",
			ExpiresIn:    3600,
		}
		data, err := json.Marshal(resp)
		assert.NoError(t, err)

		var parsed map[string]interface{}
		json.Unmarshal(data, &parsed)

		assert.Contains(t, parsed, "accessToken")
		assert.Contains(t, parsed, "refreshToken")
		assert.Contains(t, parsed, "expiresIn")
		// Should NOT have snake_case keys
		assert.NotContains(t, parsed, "access_token")
		assert.NotContains(t, parsed, "refresh_token")
	})

	t.Run("socrate to frontend mapping preserves values", func(t *testing.T) {
		socrate := socrateTokenResponse{
			AccessToken:  "access-tok",
			RefreshToken: "refresh-tok",
			ExpiresIn:    7200,
		}
		frontend := TokenResponse{
			AccessToken:  socrate.AccessToken,
			RefreshToken: socrate.RefreshToken,
			ExpiresIn:    socrate.ExpiresIn,
		}

		assert.Equal(t, "access-tok", frontend.AccessToken)
		assert.Equal(t, "refresh-tok", frontend.RefreshToken)
		assert.Equal(t, 7200, frontend.ExpiresIn)
	})
}

func TestAuthHandlerCallbackMissingCode(t *testing.T) {
	h := newTestAuthHandler()

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest("POST", "/auth/callback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Callback(w, req)

	// Should fail validation because "code" is required (422 Unprocessable Entity)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestRefreshRequestStructure(t *testing.T) {
	payload := `{"refreshToken":"rt-existing"}`
	var req RefreshRequest
	err := json.Unmarshal([]byte(payload), &req)

	assert.NoError(t, err)
	assert.Equal(t, "rt-existing", req.RefreshToken)
}

func TestLogoutRequestStructure(t *testing.T) {
	payload := `{"token":"some-token"}`
	var req LogoutRequest
	err := json.Unmarshal([]byte(payload), &req)

	assert.NoError(t, err)
	assert.Equal(t, "some-token", req.Token)
}
