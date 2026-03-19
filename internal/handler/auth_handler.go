package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"
	"kerplan/internal/config"
	"kerplan/internal/pkg/apierror"
)

// AuthHandler manages OAuth2 authentication flows.
type AuthHandler struct {
	config     *config.Config
	httpClient *http.Client
	logger     *logrus.Entry
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(cfg *config.Config, logger *logrus.Entry) *AuthHandler {
	return &AuthHandler{
		config:     cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger,
	}
}

// LoginRequest represents a login request.
type LoginRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// LoginResponse represents a login response with OAuth2 redirect.
type LoginResponse struct {
	AuthURL string `json:"authUrl"`
}

// Login initiates OAuth2 authentication flow.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, err)
		return
	}

	u, err := url.Parse(h.config.Socrate.BaseURL + "/oauth/authorize")
	if err != nil {
		handleError(w, apierror.Internal("invalid OAuth2 base URL"))
		return
	}
	q := u.Query()
	q.Set("client_id", h.config.Socrate.ClientID)
	q.Set("redirect_uri", h.config.Socrate.RedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", "openid profile email")
	q.Set("login_hint", req.Email)
	u.RawQuery = q.Encode()
	respondJSON(w, http.StatusOK, LoginResponse{AuthURL: u.String()})
}

// CallbackRequest represents the OAuth2 callback.
type CallbackRequest struct {
	Code         string `json:"code" validate:"required"`
	CodeVerifier string `json:"codeVerifier,omitempty"`
	RedirectURI  string `json:"redirectUri,omitempty"`
}

// socrateTokenResponse represents the OAuth2 token response from Socrate (snake_case).
type socrateTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// TokenResponse represents token exchange response for the frontend (camelCase).
type TokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int    `json:"expiresIn"`
}

// Callback handles OAuth2 callback and exchanges code for tokens.
func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	var req CallbackRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, err)
		return
	}

	redirectURI := req.RedirectURI
	if redirectURI == "" {
		redirectURI = h.config.Socrate.RedirectURL
	}

	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {req.Code},
		"client_id":     {h.config.Socrate.ClientID},
		"client_secret": {h.config.Socrate.ClientSecret},
		"redirect_uri":  {redirectURI},
	}
	if req.CodeVerifier != "" {
		data.Set("code_verifier", req.CodeVerifier)
	}
	resp, err := h.httpClient.PostForm(h.config.Socrate.BaseURL+"/oauth/token", data)
	if err != nil {
		h.logger.WithError(err).Error("failed to exchange authorization code")
		handleError(w, apierror.Internal("failed to exchange authorization code"))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		h.logger.WithField("status", resp.StatusCode).Error("token exchange failed")
		handleError(w, apierror.Unauthorized("token exchange failed"))
		return
	}
	var socrateTokens socrateTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&socrateTokens); err != nil {
		h.logger.WithError(err).Error("failed to decode token response")
		handleError(w, apierror.Internal("failed to decode token response"))
		return
	}
	respondJSON(w, http.StatusOK, TokenResponse{
		AccessToken:  socrateTokens.AccessToken,
		RefreshToken: socrateTokens.RefreshToken,
		ExpiresIn:    socrateTokens.ExpiresIn,
	})
}

// RefreshRequest represents a token refresh request.
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" validate:"required"`
}

// Refresh refreshes an access token.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, err)
		return
	}

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {req.RefreshToken},
		"client_id":     {h.config.Socrate.ClientID},
		"client_secret": {h.config.Socrate.ClientSecret},
	}
	resp, err := h.httpClient.PostForm(h.config.Socrate.BaseURL+"/oauth/token", data)
	if err != nil {
		h.logger.WithError(err).Error("failed to refresh token")
		handleError(w, apierror.Internal("failed to refresh token"))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		h.logger.WithField("status", resp.StatusCode).Error("token refresh failed")
		handleError(w, apierror.Unauthorized("token refresh failed"))
		return
	}
	var socrateTokens socrateTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&socrateTokens); err != nil {
		h.logger.WithError(err).Error("failed to decode token response")
		handleError(w, apierror.Internal("failed to decode token response"))
		return
	}
	respondJSON(w, http.StatusOK, TokenResponse{
		AccessToken:  socrateTokens.AccessToken,
		RefreshToken: socrateTokens.RefreshToken,
		ExpiresIn:    socrateTokens.ExpiresIn,
	})
}

// LogoutRequest represents a logout request.
type LogoutRequest struct {
	Token string `json:"token" validate:"required"`
}

// Logout revokes tokens.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req LogoutRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, err)
		return
	}

	data := url.Values{
		"token":         {req.Token},
		"client_id":     {h.config.Socrate.ClientID},
		"client_secret": {h.config.Socrate.ClientSecret},
	}
	resp, err := h.httpClient.PostForm(h.config.Socrate.BaseURL+"/oauth/revoke", data)
	if err != nil {
		h.logger.WithError(err).Error("failed to revoke token")
		handleError(w, apierror.Internal("failed to revoke token"))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		h.logger.WithField("status", resp.StatusCode).Error("token revocation failed")
		handleError(w, apierror.Internal("token revocation failed"))
		return
	}
	respondNoContent(w)
}
