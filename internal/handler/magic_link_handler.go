package handler

import (
	"net/http"

	"ascenda/internal/service"
	"github.com/sirupsen/logrus"
)

// MagicLinkHandler manages passwordless sign-in via Socrate's magic links.
//
// Flow: the landing page posts the e-mail to Send; Socrate e-mails a link to
// the magic-link URL configured on the Socrate application (the frontend's
// /magic-link page); that page posts the token from the link to
// POST /bff/magic-link/verify (BFFHandler.MagicLinkVerify), which redeems it
// into a session. No token reaches the browser.
type MagicLinkHandler struct {
	svc    *service.MagicLinkService
	logger *logrus.Entry
}

// NewMagicLinkHandler creates a MagicLinkHandler.
func NewMagicLinkHandler(svc *service.MagicLinkService, logger *logrus.Entry) *MagicLinkHandler {
	return &MagicLinkHandler{svc: svc, logger: logger}
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

// VerifyMagicLinkRequest is the JSON body for POST /bff/magic-link/verify.
type VerifyMagicLinkRequest struct {
	Token string `json:"token" validate:"required"`
}
