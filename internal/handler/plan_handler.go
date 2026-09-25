package handler

import (
	"net/http"
	"strconv"

	"ascenda/internal/dto"
	"ascenda/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/backendkit/pagination"
	"github.com/sirupsen/logrus"
)

// PlanHandler handles business plan operations.
type PlanHandler struct {
	svc     service.PlanServicer
	seedSvc *service.SeedService
	logger  *logrus.Entry
}

// NewPlanHandler creates a new PlanHandler.
func NewPlanHandler(svc service.PlanServicer, seedSvc *service.SeedService, logger *logrus.Entry) *PlanHandler {
	return &PlanHandler{
		svc:     svc,
		seedSvc: seedSvc,
		logger:  logger,
	}
}

// ListRequest represents list parameters.
type ListPlanRequest struct {
	Page  int `validate:"min=0"`
	Limit int `validate:"min=1,max=100"`
}

// List returns paginated list of plans.
func (h *PlanHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxutil.GetTenantID(r.Context())
	if tenantID.String() == "00000000-0000-0000-0000-000000000000" {
		handleError(w, r, apierror.Forbidden("missing tenant context"))
		return
	}

	// Lazy-seed the three built-in demo plans the first time a tenant lists plans.
	// EnsureDemoPlans is idempotent and a no-op after the first run.
	userID := ctxutil.GetUserID(r.Context())
	if h.seedSvc != nil {
		if err := h.seedSvc.EnsureDemoPlans(r.Context(), tenantID, userID); err != nil {
			h.logger.WithError(err).Warn("demo plan seeding failed (non-fatal)")
			// Do not block the list response; continue even if seeding failed.
		}
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 10
	}

	params := pagination.Params{
		Offset:  page * limit,
		PerPage: limit,
	}

	plans, total, err := h.svc.ListPlans(r.Context(), tenantID, params)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.NewPagedResponse(dto.PlansFromModels(plans), total, page, limit))
}

// CreateRequest represents a plan creation request.
type CreatePlanRequest struct {
	Name        string `json:"name"    validate:"required"`
	Description string `json:"description"`
	// Country is an ISO 3166-1 alpha-2 code (e.g. "BE", "FR", "DE").
	// Defaults to "BE" when omitted. Drives the initial statutory-rate defaults
	// (corporate tax, VAT, employer contributions, etc.) for the plan config.
	Country string `json:"country"`
}

// Create creates a new plan.
func (h *PlanHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreatePlanRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	userID := ctxutil.GetUserID(r.Context())
	if tenantID.String() == "00000000-0000-0000-0000-000000000000" {
		handleError(w, r, apierror.Forbidden("missing tenant context"))
		return
	}

	plan, err := h.svc.CreatePlan(r.Context(), tenantID, userID, req.Name, req.Description, req.Country)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusCreated, dto.PlanFromModel(*plan))
}

// Get retrieves a plan by ID.
func (h *PlanHandler) Get(w http.ResponseWriter, r *http.Request) {
	planID, err := parseUUIDParam(chi.URLParam(r, "planId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	plan, err := h.svc.GetPlan(r.Context(), tenantID, planID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.PlanFromModel(*plan))
}

// UpdateRequest represents a plan update request.
type UpdatePlanRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

// Update updates a plan.
func (h *PlanHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req UpdatePlanRequest
	if err := decodeJSON(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	planID, err := parseUUIDParam(chi.URLParam(r, "planId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdatePlan(r.Context(), tenantID, planID, req.Name, req.Description, req.Status); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// ResetDemo deletes all demo plans for the tenant and re-creates them from the
// current seed definitions.  Useful after data-model or seed changes.
func (h *PlanHandler) ResetDemo(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxutil.GetTenantID(r.Context())
	userID := ctxutil.GetUserID(r.Context())
	if tenantID.String() == "00000000-0000-0000-0000-000000000000" {
		handleError(w, r, apierror.Forbidden("missing tenant context"))
		return
	}

	if err := h.seedSvc.ResetDemoPlans(r.Context(), tenantID, userID); err != nil {
		h.logger.WithError(err).Error("demo plan reset failed")
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// Delete removes a plan.
func (h *PlanHandler) Delete(w http.ResponseWriter, r *http.Request) {
	planID, err := parseUUIDParam(chi.URLParam(r, "planId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.DeletePlan(r.Context(), tenantID, planID); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// GetImpact returns a deletion impact preview for a plan.
func (h *PlanHandler) GetImpact(w http.ResponseWriter, r *http.Request) {
	planID, err := parseUUIDParam(chi.URLParam(r, "planId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	impact, err := h.svc.GetPlanImpact(r.Context(), tenantID, planID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, impact)
}

// Lock transitions a plan to the "approved" (locked/frozen) status.
// Only owners and platform admins may lock a plan.
func (h *PlanHandler) Lock(w http.ResponseWriter, r *http.Request) {
	planID, err := parseUUIDParam(chi.URLParam(r, "planId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)
	callerRole := ctxutil.GetUserRole(ctx)

	if err := h.svc.TransitionPlanStatus(ctx, tenantID, planID, "approved", callerRole); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// Unlock transitions a locked plan back to "review" status.
func (h *PlanHandler) Unlock(w http.ResponseWriter, r *http.Request) {
	planID, err := parseUUIDParam(chi.URLParam(r, "planId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)
	callerRole := ctxutil.GetUserRole(ctx)

	if err := h.svc.TransitionPlanStatus(ctx, tenantID, planID, "review", callerRole); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// Archive transitions a plan to the "archived" status (read-only, hidden from active list).
func (h *PlanHandler) Archive(w http.ResponseWriter, r *http.Request) {
	planID, err := parseUUIDParam(chi.URLParam(r, "planId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)
	callerRole := ctxutil.GetUserRole(ctx)

	if err := h.svc.TransitionPlanStatus(ctx, tenantID, planID, "archived", callerRole); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// GetScenarioImpact returns a deletion impact preview for a scenario.
func (h *PlanHandler) GetScenarioImpact(w http.ResponseWriter, r *http.Request) {
	planID, err := parseUUIDParam(chi.URLParam(r, "planId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	impact, err := h.svc.GetScenarioImpact(r.Context(), tenantID, planID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, impact)
}
