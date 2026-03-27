package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"ascenda/internal/model"
	"ascenda/internal/pkg/apierror"
	"ascenda/internal/pkg/ctxutil"
)

// UserServiceIface defines the service methods used by UserHandler.
// Using an interface keeps the handler testable without a real DB.
type UserServiceIface interface {
	ListUsers(ctx context.Context, tenantID uuid.UUID, offset, limit int) ([]*model.User, int64, error)
	InviteUser(ctx context.Context, tenantID, invitedBy uuid.UUID, email, name, role string) (*model.User, error)
	UpdateRole(ctx context.Context, tenantID, targetUserID uuid.UUID, callerRole, newRole string) error
	DeactivateUser(ctx context.Context, tenantID, targetUserID uuid.UUID) error
	ReactivateUser(ctx context.Context, tenantID, targetUserID uuid.UUID) error
	DeleteUser(ctx context.Context, tenantID, targetUserID uuid.UUID) error
}

// UserHandler handles tenant-level user management operations.
type UserHandler struct {
	userService UserServiceIface
	logger      *logrus.Entry
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(userService UserServiceIface, logger *logrus.Entry) *UserHandler {
	return &UserHandler{
		userService: userService,
		logger:      logger,
	}
}

// UserDTO represents a tenant user in API responses.
type UserDTO struct {
	ID         string  `json:"id"`
	Email      string  `json:"email"`
	Name       string  `json:"name"`
	Role       string  `json:"role"`
	IsActive   bool    `json:"isActive"`
	JoinedAt   *string `json:"joinedAt,omitempty"`
	InvitedBy  *string `json:"invitedBy,omitempty"`
	ExternalID string  `json:"externalId,omitempty"`
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

	respondJSON(w, http.StatusOK, UserDTO{
		ID:    userID.String(),
		Email: email,
		Name:  name,
		Role:  role,
	})
}

// UpdateMeRequest represents a user profile update request.
type UpdateMeRequest struct {
	Name string `json:"name"`
}

// UpdateMe updates the authenticated user's profile.
func (h *UserHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	var req UpdateMeRequest
	if err := decodeJSON(r, &req); err != nil {
		handleError(w, r, err)
		return
	}
	respondNoContent(w)
}

// requireOwnerOrAdmin checks that the caller has owner or admin role.
func requireOwnerOrAdmin(role string) error {
	if role != "admin" && role != "owner" {
		return apierror.Forbidden("insufficient permissions: owner or admin required")
	}
	return nil
}

// List returns all users in the caller's tenant.
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)
	role := ctxutil.GetUserRole(ctx)

	if err := requireOwnerOrAdmin(role); err != nil {
		handleError(w, r, err)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	offset := page * limit

	users, _, err := h.userService.ListUsers(ctx, tenantID, offset, limit)
	if err != nil {
		handleError(w, r, err)
		return
	}

	dtos := make([]UserDTO, len(users))
	for i, u := range users {
		dto := UserDTO{
			ID:         u.ID.String(),
			Email:      u.Email,
			Name:       u.Name,
			Role:       u.Role,
			IsActive:   u.IsActive,
			ExternalID: u.ExternalID,
		}
		if u.JoinedAt != nil {
			s := u.JoinedAt.Format("2006-01-02T15:04:05Z07:00")
			dto.JoinedAt = &s
		}
		if u.InvitedBy != nil {
			s := u.InvitedBy.String()
			dto.InvitedBy = &s
		}
		dtos[i] = dto
	}

	respondJSON(w, http.StatusOK, dtos)
}

// InviteRequest represents a user invitation request.
type InviteRequest struct {
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"name"`
	Role  string `json:"role" validate:"required"`
}

// Invite sends an invitation to a new user and creates a pending record.
func (h *UserHandler) Invite(w http.ResponseWriter, r *http.Request) {
	var req InviteRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)
	callerID := ctxutil.GetUserID(ctx)
	role := ctxutil.GetUserRole(ctx)

	if err := requireOwnerOrAdmin(role); err != nil {
		handleError(w, r, err)
		return
	}

	user, err := h.userService.InviteUser(ctx, tenantID, callerID, req.Email, req.Name, req.Role)
	if err != nil {
		handleError(w, r, err)
		return
	}

	dto := UserDTO{
		ID:       user.ID.String(),
		Email:    user.Email,
		Name:     user.Name,
		Role:     user.Role,
		IsActive: user.IsActive,
	}
	respondJSON(w, http.StatusCreated, dto)
}

// UpdateRoleRequest represents a role update request.
type UpdateRoleRequest struct {
	Role string `json:"role" validate:"required"`
}

// UpdateRole changes a tenant user's role.
func (h *UserHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	var req UpdateRoleRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	userID, err := parseUUIDParam(chi.URLParam(r, "userId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)
	callerRole := ctxutil.GetUserRole(ctx)

	if err := requireOwnerOrAdmin(callerRole); err != nil {
		handleError(w, r, err)
		return
	}

	if err := h.userService.UpdateRole(ctx, tenantID, userID, callerRole, req.Role); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// Deactivate marks a user as inactive (session still valid until next auth check).
func (h *UserHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUUIDParam(chi.URLParam(r, "userId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	if err := h.userService.DeactivateUser(ctx, tenantID, userID); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// Reactivate marks a user as active again.
func (h *UserHandler) Reactivate(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUUIDParam(chi.URLParam(r, "userId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	if err := h.userService.ReactivateUser(ctx, tenantID, userID); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// Delete soft-deletes a user from the tenant. Owner cannot be deleted.
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUUIDParam(chi.URLParam(r, "userId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)
	callerRole := ctxutil.GetUserRole(ctx)

	if err := requireOwnerOrAdmin(callerRole); err != nil {
		handleError(w, r, err)
		return
	}

	if err := h.userService.DeleteUser(ctx, tenantID, userID); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// UserImpactDTO summarises the dependencies that would be affected by deleting a user.
type UserImpactDTO struct {
	UserID   string `json:"userId"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	IsOwner  bool   `json:"isOwner"`
	CanDelete bool  `json:"canDelete"`
	Message  string `json:"message,omitempty"`
}

// GetImpact returns a dependency preview before deleting a user.
func (h *UserHandler) GetImpact(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUUIDParam(chi.URLParam(r, "userId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)
	callerRole := ctxutil.GetUserRole(ctx)

	if err := requireOwnerOrAdmin(callerRole); err != nil {
		handleError(w, r, err)
		return
	}

	users, _, err := h.userService.ListUsers(ctx, tenantID, 0, 1000)
	if err != nil {
		handleError(w, r, err)
		return
	}

	// Find the target user in the tenant user list
	for _, u := range users {
		if u.ID == userID {
			isOwner := u.Role == "owner"
			canDelete := !isOwner
			msg := ""
			if isOwner {
				msg = "Cannot delete the owner. Transfer ownership first."
			}
			respondJSON(w, http.StatusOK, UserImpactDTO{
				UserID:    u.ID.String(),
				Email:     u.Email,
				Name:      u.Name,
				Role:      u.Role,
				IsOwner:   isOwner,
				CanDelete: canDelete,
				Message:   msg,
			})
			return
		}
	}

	handleError(w, r, apierror.NotFound("user", userID.String()))
}
