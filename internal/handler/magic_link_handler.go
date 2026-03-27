package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
	"ascenda/internal/config"
	"ascenda/internal/pkg/apierror"
	"ascenda/internal/service"
)

// MagicLinkHandler manages passwordless sign-in via email.
type MagicLinkHandler struct {
	svc    *service.MagicLinkService
	cfg    *config.Config
	logger *logrus.Entry
}

// NewMagicLinkHandler creates a MagicLinkHandler.
func NewMagicLinkHandler(svc *service.MagicLinkService, cfg *config.Config, logger *logrus.Entry) *MagicLinkHandler {
	return &MagicLinkHandler{svc: svc, cfg: cfg, logger: logger}
}

// ── Send ─────────────────────────────────────────────────────────────────────

// SendMagicLinkRequest is the JSON body for POST /auth/magic-link.
type SendMagicLinkRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Redirect string `json:"redirect"` // optional post-auth destination
}

// Send handles POST /auth/magic-link.
// Always returns 202 Accepted to prevent account enumeration.
func (h *MagicLinkHandler) Send(w http.ResponseWriter, r *http.Request) {
	var req SendMagicLinkRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	// Sanitise and validate the redirect URL — must be same origin or empty.
	redirect := sanitiseRedirect(req.Redirect, h.cfg.AllowedOrigins)

	if err := h.svc.SendMagicLink(r.Context(), req.Email, redirect); err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusAccepted, map[string]string{
		"message": "If an account exists for that email, a sign-in link is on its way.",
	})
}

// ── Verify ───────────────────────────────────────────────────────────────────

// Verify handles GET /auth/magic-link/verify?token=xxx.
// On success it redirects the browser to Socrate's OAuth2 authorize endpoint
// with login_hint pre-filled, which completes the normal PKCE sign-in flow.
// On failure it redirects to the frontend login page with an error query param.
func (h *MagicLinkHandler) Verify(w http.ResponseWriter, r *http.Request) {
	rawToken := strings.TrimSpace(r.URL.Query().Get("token"))
	if rawToken == "" {
		h.redirectWithError(w, r, "missing token")
		return
	}

	result, err := h.svc.VerifyMagicLink(r.Context(), rawToken)
	if err != nil {
		appErr, ok := err.(*apierror.AppError)
		if ok && appErr.StatusCode == http.StatusUnauthorized {
			h.redirectWithError(w, r, "link expired or already used")
			return
		}
		h.redirectWithError(w, r, "verification failed")
		return
	}

	// Build the Socrate OAuth2 authorize URL with login_hint so the user's
	// email is pre-filled and they can confirm sign-in with one click.
	authURL, err := h.buildAuthURL(result.Email, result.RedirectURL)
	if err != nil {
		h.logger.WithError(err).Error("magic-link: failed to build auth URL")
		h.redirectWithError(w, r, "internal error")
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

// ── helpers ──────────────────────────────────────────────────────────────────

// buildAuthURL constructs the Socrate OAuth2 authorize URL with login_hint.
func (h *MagicLinkHandler) buildAuthURL(email, postAuthRedirect string) (string, error) {
	u, err := url.Parse(h.cfg.Socrate.BaseURL + "/oauth/authorize")
	if err != nil {
		return "", fmt.Errorf("parse OAuth2 base URL: %w", err)
	}

	q := u.Query()
	q.Set("client_id", h.cfg.Socrate.ClientID)
	q.Set("redirect_uri", h.cfg.Socrate.RedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", "openid profile email")
	q.Set("login_hint", email)

	// Pass the post-auth redirect so the frontend callback can honour it.
	if postAuthRedirect != "" {
		q.Set("state", postAuthRedirect)
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

// redirectWithError sends the browser to the frontend login page with an error
// query parameter so the user sees a human-readable message.
func (h *MagicLinkHandler) redirectWithError(w http.ResponseWriter, r *http.Request, msg string) {
	// Best-effort: derive frontend origin from AllowedOrigins.
	frontendBase := "/"
	if len(h.cfg.AllowedOrigins) > 0 {
		frontendBase = strings.TrimRight(h.cfg.AllowedOrigins[0], "/")
	}
	dest := frontendBase + "/?magic_error=" + url.QueryEscape(msg)
	http.Redirect(w, r, dest, http.StatusFound)
}

// sanitiseRedirect validates that a redirect URL is same-origin (or relative).
// Any URL pointing outside the allowed origins is replaced with "/".
func sanitiseRedirect(redirect string, allowedOrigins []string) string {
	if redirect == "" || redirect == "/" {
		return redirect
	}
	// Allow relative paths.
	if strings.HasPrefix(redirect, "/") && !strings.HasPrefix(redirect, "//") {
		return redirect
	}
	// Allow only URLs matching an allowed origin.
	parsed, err := url.Parse(redirect)
	if err != nil {
		return "/"
	}
	origin := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
	for _, allowed := range allowedOrigins {
		if strings.TrimRight(allowed, "/") == strings.TrimRight(origin, "/") {
			return redirect
		}
	}
	return "/"
}
