package handler

import (
	"net/http"

	"github.com/sirupsen/logrus"
	"ascenda/internal/dto"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"ascenda/internal/service"
)

// TenantHandler handles tenant management operations.
type TenantHandler struct {
	svc    service.TenantServicer
	logger *logrus.Entry
}

// NewTenantHandler creates a new TenantHandler.
func NewTenantHandler(svc service.TenantServicer, logger *logrus.Entry) *TenantHandler {
	return &TenantHandler{
		svc:    svc,
		logger: logger,
	}
}

// Get returns the current tenant info.
func (h *TenantHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxutil.GetTenantID(r.Context())

	tenant, err := h.svc.GetTenant(r.Context(), tenantID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.TenantFromModel(*tenant))
}

// UpdateTenantRequest represents a tenant update request.
type UpdateTenantRequest struct {
	Name string `json:"name" validate:"required"`
}

// Update updates tenant details.
func (h *TenantHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req UpdateTenantRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	role := ctxutil.GetUserRole(r.Context())

	// Belt-and-suspenders RBAC check (route-level PermManageTenant already applied).
	if role != "admin" && role != "owner" {
		handleError(w, r, apierror.Forbidden("insufficient permissions to manage tenant"))
		return
	}

	if err := h.svc.UpdateTenant(r.Context(), tenantID, req.Name); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}
