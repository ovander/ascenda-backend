package handler

import (
	"net/http"

	"ascenda/internal/service"
	"github.com/sirupsen/logrus"
)

// AuthHandler serves self-service registration. Sign-in, token refresh and
// sign-out run in the Backend-for-Frontend (BFFHandler, /bff): no route
// returns an OAuth token to the browser.
type AuthHandler struct {
	registration *service.RegistrationService
	logger       *logrus.Entry
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(registration *service.RegistrationService, logger *logrus.Entry) *AuthHandler {
	return &AuthHandler{registration: registration, logger: logger}
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
