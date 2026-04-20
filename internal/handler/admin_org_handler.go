package handler

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/ovander/backendkit/apierror"
	"ascenda/internal/service"
)

// OrgServicer is the interface that AdminOrgHandler depends on.
type OrgServicer interface {
	ListOrganizations(ctx context.Context, page, pageSize int) (*service.OrgListResponse, error)
	GetOrganization(ctx context.Context, id uuid.UUID) (*service.OrgDTO, error)
	CreateOrganization(ctx context.Context, req service.CreateOrganizationRequest) (*service.OrgDTO, error)
	UpdateOrganization(ctx context.Context, id uuid.UUID, req service.UpdateOrganizationRequest) (*service.OrgDTO, error)
	DeleteOrganization(ctx context.Context, id uuid.UUID) error
	AddTenant(ctx context.Context, orgID uuid.UUID, req service.AddOrgTenantRequest) (*service.OrgTenantDTO, error)
	ListTenants(ctx context.Context, orgID uuid.UUID) ([]service.OrgTenantDTO, error)
}

// AdminOrgHandler handles enterprise organization management for the platform admin role.
type AdminOrgHandler struct {
	orgService OrgServicer
	logger     *logrus.Entry
}

// NewAdminOrgHandler creates a new AdminOrgHandler.
func NewAdminOrgHandler(orgService OrgServicer, logger *logrus.Entry) *AdminOrgHandler {
	return &AdminOrgHandler{
		orgService: orgService,
		logger:     logger,
	}
}

// ── Organization CRUD ─────────────────────────────────────────────────────────

// ListOrganizations handles GET /api/v1/admin/organizations
func (h *AdminOrgHandler) ListOrganizations(w http.ResponseWriter, r *http.Request) {
	page := queryIntDefault(r, "page", 1)
	pageSize := queryIntDefault(r, "pageSize", 20)
	if pageSize > 100 {
		pageSize = 100
	}

	result, err := h.orgService.ListOrganizations(r.Context(), page, pageSize)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, result)
}

// GetOrganization handles GET /api/v1/admin/organizations/{orgId}
func (h *AdminOrgHandler) GetOrganization(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUIDParam(chi.URLParam(r, "orgId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	org, err := h.orgService.GetOrganization(r.Context(), orgID)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, org)
}

// CreateOrganization handles POST /api/v1/admin/organizations
func (h *AdminOrgHandler) CreateOrganization(w http.ResponseWriter, r *http.Request) {
	var req service.CreateOrganizationRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	org, err := h.orgService.CreateOrganization(r.Context(), req)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, org)
}

// UpdateOrganization handles PUT /api/v1/admin/organizations/{orgId}
func (h *AdminOrgHandler) UpdateOrganization(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUIDParam(chi.URLParam(r, "orgId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	var req service.UpdateOrganizationRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	org, err := h.orgService.UpdateOrganization(r.Context(), orgID, req)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, org)
}

// DeleteOrganization handles DELETE /api/v1/admin/organizations/{orgId}
func (h *AdminOrgHandler) DeleteOrganization(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUIDParam(chi.URLParam(r, "orgId"))
	if err != nil {
		handleError(w, r, apierror.BadRequest("invalid organization ID"))
		return
	}

	if err := h.orgService.DeleteOrganization(r.Context(), orgID); err != nil {
		handleError(w, r, err)
		return
	}
	respondNoContent(w)
}

// ── Department tenants ────────────────────────────────────────────────────────

// ListOrgTenants handles GET /api/v1/admin/organizations/{orgId}/tenants
func (h *AdminOrgHandler) ListOrgTenants(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUIDParam(chi.URLParam(r, "orgId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenants, err := h.orgService.ListTenants(r.Context(), orgID)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tenants": tenants,
	})
}

// AddOrgTenant handles POST /api/v1/admin/organizations/{orgId}/tenants
func (h *AdminOrgHandler) AddOrgTenant(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUIDParam(chi.URLParam(r, "orgId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	var req service.AddOrgTenantRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	tenant, err := h.orgService.AddTenant(r.Context(), orgID, req)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, tenant)
}
