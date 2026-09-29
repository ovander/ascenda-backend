package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"ascenda/internal/config"
	"ascenda/internal/repo"
	"ascenda/internal/service"
	"github.com/sirupsen/logrus"
)

// AuthHandler manages OAuth2 authentication flows and self-service registration.
// The token grants go through TokenService (backendkit's socrate.Client), so the
// browser's address and User-Agent reach Socrate with every sign-in, refresh and
// logout (middleware.SocrateClientAttribution on the /auth routes).
type AuthHandler struct {
	config       *config.Config
	tokens       *service.TokenService
	registration *service.RegistrationService
	userRepo     repo.UserRepository // optional; used to enrich email/name from id_token
	logger       *logrus.Entry
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(cfg *config.Config, tokens *service.TokenService, registration *service.RegistrationService, userRepo repo.UserRepository, logger *logrus.Entry) *AuthHandler {
	return &AuthHandler{
		config:       cfg,
		tokens:       tokens,
		registration: registration,
		userRepo:     userRepo,
		logger:       logger,
	}
}

// CallbackRequest represents the OAuth2 callback.
type CallbackRequest struct {
	Code         string `json:"code" validate:"required"`
	CodeVerifier string `json:"codeVerifier,omitempty"`
	RedirectURI  string `json:"redirectUri,omitempty"`
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

	tokens, err := h.tokens.ExchangeCode(r.Context(), req.Code, redirectURI, req.CodeVerifier)
	if err != nil {
		handleError(w, r, err)
		return
	}

	// If Socrate returned an OIDC id_token, decode its payload to extract email/name
	// and update the user record eagerly. The id_token is issued when the frontend
	// requests the "openid email profile" scopes. No signature verification is needed
	// here because the token came directly from Socrate over HTTPS (trusted channel).
	if tokens.IDToken != "" && h.userRepo != nil {
		enrichUserFromIDToken(h.userRepo, h.logger, tokens.IDToken)
	}

	respondJSON(w, http.StatusOK, TokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
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

	tokens, err := h.tokens.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, TokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
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

	if err := h.tokens.RevokeToken(r.Context(), req.Token); err != nil {
		handleError(w, r, err)
		return
	}
	respondNoContent(w)
}
