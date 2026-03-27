package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"ascenda/internal/pkg/apierror"
	"ascenda/internal/pkg/socrate"
	"ascenda/internal/service"
)

// AdminUserServicer interface for dependency injection.
type AdminUserServicer interface {
	ListUsers(ctx context.Context, search string, page, pageSize int) (*service.AdminUserListResponse, error)
	GetUser(ctx context.Context, socrateID string) (*service.AdminUserDTO, error)
	CreateUser(ctx context.Context, req service.CreateUserRequest) (*service.AdminUserDTO, error)
	UpdateUser(ctx context.Context, socrateID string, req service.UpdateUserRequest) (*service.AdminUserDTO, error)
	DeleteUser(ctx context.Context, socrateID string) error
	ResendVerification(ctx context.Context, socrateID string) error
	ResetPassword(ctx context.Context, socrateID string) error
	ListTenants(ctx context.Context, page, pageSize int) (*service.AdminTenantListResponse, error)
	GetTenant(ctx context.Context, id uuid.UUID) (*service.AdminTenantDTO, error)
	CreateTenant(ctx context.Context, req service.CreateTenantRequest) (*service.AdminTenantDTO, error)
	UpdateTenant(ctx context.Context, id uuid.UUID, req service.UpdateTenantRequest) (*service.AdminTenantDTO, error)
}

// AdminUserHandler handles platform-wide user and tenant management for the admin role.
type AdminUserHandler struct {
	adminUserService AdminUserServicer
	logger           *logrus.Entry
}

// NewAdminUserHandler creates a new AdminUserHandler.
func NewAdminUserHandler(adminUserService AdminUserServicer, logger *logrus.Entry) *AdminUserHandler {
	return &AdminUserHandler{
		adminUserService: adminUserService,
		logger:           logger,
	}
}

// withSocrateJWT extracts the Bearer JWT from the Authorization header and injects
// it into the request context so the Socrate client can forward it upstream.
// Auth middleware has already validated the token — we are just re-using it here.
func withSocrateJWT(r *http.Request) *http.Request {
	authHeader := r.Header.Get("Authorization")
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
		ctx := socrate.WithJWT(r.Context(), parts[1])
		return r.WithContext(ctx)
	}
	return r
}

// ============================================================================
// User endpoints
// ============================================================================

// ListUsers handles GET /api/v1/admin/users
func (h *AdminUserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	r = withSocrateJWT(r)

	search := r.URL.Query().Get("search")
	page := queryIntDefault(r, "page", 1)
	pageSize := queryIntDefault(r, "pageSize", 20)
	if pageSize > 100 {
		pageSize = 100
	}

	result, err := h.adminUserService.ListUsers(r.Context(), search, page, pageSize)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// GetUser handles GET /api/v1/admin/users/{userId}
func (h *AdminUserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	r = withSocrateJWT(r)

	userID := chi.URLParam(r, "userId")
	if userID == "" {
		handleError(w, r, apierror.BadRequest("missing userId parameter"))
		return
	}

	user, err := h.adminUserService.GetUser(r.Context(), userID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, user)
}

// CreateUser handles POST /api/v1/admin/users
func (h *AdminUserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	r = withSocrateJWT(r)

	var req service.CreateUserRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	user, err := h.adminUserService.CreateUser(r.Context(), req)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusCreated, user)
}

// UpdateUser handles PUT /api/v1/admin/users/{userId}
func (h *AdminUserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	r = withSocrateJWT(r)

	userID := chi.URLParam(r, "userId")
	if userID == "" {
		handleError(w, r, apierror.BadRequest("missing userId parameter"))
		return
	}

	var req service.UpdateUserRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	user, err := h.adminUserService.UpdateUser(r.Context(), userID, req)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, user)
}

// DeleteUser handles DELETE /api/v1/admin/users/{userId}
func (h *AdminUserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	r = withSocrateJWT(r)

	userID := chi.URLParam(r, "userId")
	if userID == "" {
		handleError(w, r, apierror.BadRequest("missing userId parameter"))
		return
	}

	if err := h.adminUserService.DeleteUser(r.Context(), userID); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// ResendVerification handles POST /api/v1/admin/users/{userId}/resend-verification
func (h *AdminUserHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	r = withSocrateJWT(r)

	userID := chi.URLParam(r, "userId")
	if userID == "" {
		handleError(w, r, apierror.BadRequest("missing userId parameter"))
		return
	}

	if err := h.adminUserService.ResendVerification(r.Context(), userID); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// ResetPassword handles POST /api/v1/admin/users/{userId}/reset-password
func (h *AdminUserHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	r = withSocrateJWT(r)

	userID := chi.URLParam(r, "userId")
	if userID == "" {
		handleError(w, r, apierror.BadRequest("missing userId parameter"))
		return
	}

	if err := h.adminUserService.ResetPassword(r.Context(), userID); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// ============================================================================
// Tenant endpoints
// ============================================================================

// ListTenants handles GET /api/v1/admin/tenants
func (h *AdminUserHandler) ListTenants(w http.ResponseWriter, r *http.Request) {
	page := queryIntDefault(r, "page", 1)
	pageSize := queryIntDefault(r, "pageSize", 20)
	if pageSize > 100 {
		pageSize = 100
	}

	result, err := h.adminUserService.ListTenants(r.Context(), page, pageSize)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// GetTenant handles GET /api/v1/admin/tenants/{tenantId}
func (h *AdminUserHandler) GetTenant(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseUUIDParam(chi.URLParam(r, "tenantId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenant, err := h.adminUserService.GetTenant(r.Context(), tenantID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, tenant)
}

// CreateTenant handles POST /api/v1/admin/tenants
func (h *AdminUserHandler) CreateTenant(w http.ResponseWriter, r *http.Request) {
	var req service.CreateTenantRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	tenant, err := h.adminUserService.CreateTenant(r.Context(), req)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusCreated, tenant)
}

// UpdateTenant handles PUT /api/v1/admin/tenants/{tenantId}
func (h *AdminUserHandler) UpdateTenant(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseUUIDParam(chi.URLParam(r, "tenantId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	var req service.UpdateTenantRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	tenant, err := h.adminUserService.UpdateTenant(r.Context(), tenantID, req)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, tenant)
}

// ============================================================================
// Helpers
// ============================================================================

// queryIntDefault returns the query param as int, or defaultVal if missing/invalid.
func queryIntDefault(r *http.Request, param string, defaultVal int) int {
	v := r.URL.Query().Get(param)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return defaultVal
	}
	return n
}
