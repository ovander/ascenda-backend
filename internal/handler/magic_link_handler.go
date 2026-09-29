package handler

import (
	"net/http"

	"ascenda/internal/repo"
	"ascenda/internal/service"
	"github.com/sirupsen/logrus"
)

// MagicLinkHandler manages passwordless sign-in via Socrate's magic links.
//
// Flow: the landing page posts the e-mail to Send; Socrate e-mails a link to
// the magic-link URL configured on the Socrate application (the frontend's
// /magic-link page); that page posts the token from the link to Verify, which
// redeems it at Socrate and returns the same tokens as /auth/callback.
type MagicLinkHandler struct {
	svc      *service.MagicLinkService
	userRepo repo.UserRepository // optional; used to enrich email/name from id_token
	logger   *logrus.Entry
}

// NewMagicLinkHandler creates a MagicLinkHandler.
func NewMagicLinkHandler(svc *service.MagicLinkService, userRepo repo.UserRepository, logger *logrus.Entry) *MagicLinkHandler {
	return &MagicLinkHandler{svc: svc, userRepo: userRepo, logger: logger}
}

// SendMagicLinkRequest is the JSON body for POST /auth/magic-link.
type SendMagicLinkRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// Send handles POST /auth/magic-link.
// Always returns 202 Accepted to prevent account enumeration.
func (h *MagicLinkHandler) Send(w http.ResponseWriter, r *http.Request) {
	var req SendMagicLinkRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	h.svc.SendMagicLink(r.Context(), req.Email)

	respondJSON(w, http.StatusAccepted, map[string]string{
		"message": "If an account exists for that email, a sign-in link is on its way.",
	})
}

// VerifyMagicLinkRequest is the JSON body for POST /auth/magic-link/verify.
type VerifyMagicLinkRequest struct {
	Token string `json:"token" validate:"required"`
}

// Verify handles POST /auth/magic-link/verify. It redeems the single-use token
// from the e-mailed link at Socrate and returns the token set, like Callback.
// POST only: a GET would let e-mail link scanners spend the token.
func (h *MagicLinkHandler) Verify(w http.ResponseWriter, r *http.Request) {
	var req VerifyMagicLinkRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	result, err := h.svc.VerifyMagicLink(r.Context(), req.Token)
	if err != nil {
		handleError(w, r, err)
		return
	}

	if result.IDToken != "" && h.userRepo != nil {
		enrichUserFromIDToken(h.userRepo, h.logger, result.IDToken)
	}

	respondJSON(w, http.StatusOK, TokenResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresIn,
	})
}
