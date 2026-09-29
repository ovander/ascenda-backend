package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ascenda/internal/config"
	"ascenda/internal/service"
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
	return NewAuthHandler(cfg, service.NewTokenService(nil, logger), nil, nil, logger)
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

// The mapping from Socrate's snake_case token response to this camelCase one is
// exercised end to end in auth_handler_socrate_test.go.
func TestTokenResponseMapping(t *testing.T) {
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
