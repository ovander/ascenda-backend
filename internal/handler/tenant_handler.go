package handler

import (
	"net/http"

	"github.com/sirupsen/logrus"
	"kerplan/internal/pkg/apierror"
	"kerplan/internal/pkg/ctxutil"
)

// TenantHandler handles tenant management operations.
type TenantHandler struct {
	logger *logrus.Entry
}

// NewTenantHandler creates a new TenantHandler.
func NewTenantHandler(logger *logrus.Entry) *TenantHandler {
	return &TenantHandler{
		logger: logger,
	}
}

// TenantDTO represents a tenant.
type TenantDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Get returns the current tenant info.
func (h *TenantHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxutil.GetTenantID(r.Context())

	tenant := TenantDTO{
		ID:    tenantID.String(),
		Name:  "Tenant",
		Email: "tenant@example.com",
	}

	respondJSON(w, http.StatusOK, tenant)
}

// UpdateRequest represents a tenant update request.
type UpdateTenantRequest struct {
	Name string `json:"name"`
}

// Update updates tenant details.
func (h *TenantHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req UpdateTenantRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	role := ctxutil.GetUserRole(r.Context())

	// Stub: check RBAC permission for PermManageTenant
	if role != "admin" && role != "owner" {
		handleError(w, apierror.Forbidden("insufficient permissions to manage tenant"))
		return
	}

	_ = tenantID
	respondNoContent(w)
}
