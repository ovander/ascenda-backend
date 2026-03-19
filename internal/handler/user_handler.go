package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"kerplan/internal/pkg/apierror"
	"kerplan/internal/pkg/ctxutil"
	"kerplan/internal/service"
)

// UserHandler handles user profile operations.
type UserHandler struct {
	userService *service.UserService
	logger      *logrus.Entry
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(userService *service.UserService, logger *logrus.Entry) *UserHandler {
	return &UserHandler{
		userService: userService,
		logger:      logger,
	}
}

// UserDTO represents a user profile.
type UserDTO struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

// GetMe returns the authenticated user's profile.
func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := ctxutil.GetUserID(ctx)
	email := ctxutil.GetUserEmail(ctx)
	name := ctxutil.GetUserName(ctx)
	role := ctxutil.GetUserRole(ctx)

	if role == "" {
		role = "user"
	}

	user := UserDTO{
		ID:    userID.String(),
		Email: email,
		Name:  name,
		Role:  role,
	}

	respondJSON(w, http.StatusOK, user)
}

// UpdateMeRequest represents a user profile update request.
type UpdateMeRequest struct {
	Name string `json:"name"`
}

// UpdateMe updates the authenticated user's profile.
func (h *UserHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	var req UpdateMeRequest
	if err := decodeJSON(r, &req); err != nil {
		handleError(w, err)
		return
	}

	// Stub: actual implementation would update user profile
	respondNoContent(w)
}

// List returns all users in a tenant.
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxutil.GetTenantID(r.Context())
	role := ctxutil.GetUserRole(r.Context())

	// Stub: check RBAC permission for PermManageUsers
	if role != "admin" && role != "owner" {
		handleError(w, apierror.Forbidden("insufficient permissions"))
		return
	}

	_ = tenantID
	respondJSON(w, http.StatusOK, []UserDTO{})
}

// InviteRequest represents a user invitation request.
type InviteRequest struct {
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role" validate:"required"`
}

// Invite sends an invitation to a new user.
func (h *UserHandler) Invite(w http.ResponseWriter, r *http.Request) {
	var req InviteRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	role := ctxutil.GetUserRole(r.Context())

	// Stub: check RBAC permission
	if role != "admin" && role != "owner" {
		handleError(w, apierror.Forbidden("insufficient permissions"))
		return
	}

	_ = tenantID
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"email": req.Email,
		"role":  req.Role,
	})
}

// UpdateRoleRequest represents a role update request.
type UpdateRoleRequest struct {
	Role string `json:"role" validate:"required"`
}

// UpdateRole changes a user's role.
func (h *UserHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	var req UpdateRoleRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, err)
		return
	}

	userID, err := parseUUIDParam(chi.URLParam(r, "userId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	role := ctxutil.GetUserRole(r.Context())

	// Stub: check RBAC permission
	if role != "admin" && role != "owner" {
		handleError(w, apierror.Forbidden("insufficient permissions"))
		return
	}

	_ = tenantID
	_ = userID
	respondNoContent(w)
}

// Deactivate marks a user as inactive.
func (h *UserHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUUIDParam(chi.URLParam(r, "userId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())

	if err := h.userService.DeactivateUser(r.Context(), tenantID, userID); err != nil {
		handleError(w, err)
		return
	}

	respondNoContent(w)
}

// Reactivate marks a user as active.
func (h *UserHandler) Reactivate(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUUIDParam(chi.URLParam(r, "userId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())

	if err := h.userService.ReactivateUser(r.Context(), tenantID, userID); err != nil {
		handleError(w, err)
		return
	}

	respondNoContent(w)
}
