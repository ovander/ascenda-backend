package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ascenda/internal/config"
	"ascenda/internal/repo"
	"ascenda/internal/service"
	"github.com/ovander/backendkit/apierror"
	"github.com/sirupsen/logrus"
)

// AuthHandler manages OAuth2 authentication flows and self-service registration.
type AuthHandler struct {
	config       *config.Config
	httpClient   *http.Client
	registration *service.RegistrationService
	userRepo     repo.UserRepository // optional; used to enrich email/name from id_token
	logger       *logrus.Entry
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(cfg *config.Config, registration *service.RegistrationService, userRepo repo.UserRepository, logger *logrus.Entry) *AuthHandler {
	return &AuthHandler{
		config:       cfg,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		registration: registration,
		userRepo:     userRepo,
		logger:       logger,
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
		handleError(w, r, err)
		return
	}

	u, err := url.Parse(h.config.Socrate.BaseURL + "/oauth/authorize")
	if err != nil {
		handleError(w, r, apierror.Internal("invalid OAuth2 base URL"))
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

// socrateTokenResponse represents the OAuth2/OIDC token response from Socrate (snake_case).
// id_token is an OIDC ID token (JWT) that contains user claims (email, name, sub, etc.)
// and is issued when the authorization request includes the "openid email profile" scopes.
type socrateTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"` // OIDC ID token — contains email, name, sub
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
		handleError(w, r, err)
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
		handleError(w, r, apierror.Internal("failed to exchange authorization code"))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		handleError(w, r, apierror.Unauthorized("token exchange failed"))
		return
	}
	var socrateTokens socrateTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&socrateTokens); err != nil {
		handleError(w, r, apierror.Internal("failed to decode token response"))
		return
	}

	// If Socrate returned an OIDC id_token, decode its payload to extract email/name
	// and update the user record eagerly. The id_token is issued when the frontend
	// requests the "openid email profile" scopes. No signature verification is needed
	// here because the token came directly from Socrate over HTTPS (trusted channel).
	if socrateTokens.IDToken != "" && h.userRepo != nil {
		enrichUserFromIDToken(h.userRepo, h.logger, socrateTokens.IDToken)
	}

	respondJSON(w, http.StatusOK, TokenResponse{
		AccessToken:  socrateTokens.AccessToken,
		RefreshToken: socrateTokens.RefreshToken,
		ExpiresIn:    socrateTokens.ExpiresIn,
	})
}

// enrichUserFromIDToken decodes the OIDC id_token payload (no sig verification —
// token came directly from Socrate over HTTPS) and updates the matching user record
// with email and name if those fields are currently empty. Used after the code
// exchange and after a magic-link sign-in.
func enrichUserFromIDToken(userRepo repo.UserRepository, logger *logrus.Entry, idToken string) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return
	}

	sub, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)
	name, _ := claims["name"].(string)
	if name == "" {
		given, _ := claims["given_name"].(string)
		family, _ := claims["family_name"].(string)
		name = strings.TrimSpace(given + " " + family)
	}

	if sub == "" || (email == "" && name == "") {
		return
	}

	user, err := userRepo.GetByExternalID(sub)
	if err != nil || user == nil {
		return // user hasn't been auto-provisioned yet — TenantMiddleware will do it on first request
	}

	changed := false
	if email != "" && user.Email != email {
		user.Email = email
		changed = true
	}
	if name != "" && user.Name != name {
		user.Name = name
		changed = true
	}
	if changed {
		if updateErr := userRepo.Update(user); updateErr != nil {
			logger.WithError(updateErr).Warn("failed to persist user profile from id_token")
		} else {
			logger.WithFields(logrus.Fields{"sub": sub, "email": email}).
				Debug("user profile enriched from OIDC id_token")
		}
	}
}

// RefreshRequest represents a token refresh request.
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" validate:"required"`
}

// Refresh refreshes an access token.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
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
		handleError(w, r, apierror.Internal("failed to refresh token"))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		handleError(w, r, apierror.Unauthorized("token refresh failed"))
		return
	}
	var socrateTokens socrateTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&socrateTokens); err != nil {
		handleError(w, r, apierror.Internal("failed to decode token response"))
		return
	}
	respondJSON(w, http.StatusOK, TokenResponse{
		AccessToken:  socrateTokens.AccessToken,
		RefreshToken: socrateTokens.RefreshToken,
		ExpiresIn:    socrateTokens.ExpiresIn,
	})
}

// RegisterRequest is the payload from the public registration form.
// It mirrors service.RegisterRequest but lives in the handler layer for
// decoding and validation before being forwarded to the service.
type RegisterRequest struct {
	FirstName   string `json:"firstName"   validate:"required"`
	LastName    string `json:"lastName"    validate:"required"`
	CompanyName string `json:"companyName" validate:"required"`
	Email       string `json:"email"       validate:"required,email"`
	Country     string `json:"country"     validate:"required"`
}

// Register handles POST /auth/register.
// It is unauthenticated and rate-limited at the router level.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	result, err := h.registration.Register(r.Context(), service.RegisterRequest{
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		CompanyName: req.CompanyName,
		Email:       req.Email,
		Country:     req.Country,
	})
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusAccepted, result)
}

// LogoutRequest represents a logout request.
type LogoutRequest struct {
	Token string `json:"token" validate:"required"`
}

// Logout revokes tokens.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req LogoutRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	data := url.Values{
		"token":         {req.Token},
		"client_id":     {h.config.Socrate.ClientID},
		"client_secret": {h.config.Socrate.ClientSecret},
	}
	resp, err := h.httpClient.PostForm(h.config.Socrate.BaseURL+"/oauth/revoke", data)
	if err != nil {
		handleError(w, r, apierror.Internal("failed to revoke token"))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		handleError(w, r, apierror.Internal("token revocation failed"))
		return
	}
	respondNoContent(w)
}
