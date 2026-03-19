package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"kerplan/internal/dto"
	"kerplan/internal/pkg/apierror"
	"kerplan/internal/pkg/ctxutil"
	"kerplan/internal/pkg/pagination"
	"kerplan/internal/service"
)

// PlanHandler handles business plan operations.
type PlanHandler struct {
	svc    *service.PlanService
	logger *logrus.Entry
}

// NewPlanHandler creates a new PlanHandler.
func NewPlanHandler(svc *service.PlanService, logger *logrus.Entry) *PlanHandler {
	return &PlanHandler{
		svc:    svc,
		logger: logger,
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
		handleError(w, apierror.Forbidden("missing tenant context"))
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 10
	}

	params := pagination.Params{
		Offset: page * limit,
		PerPage: limit,
	}

	plans, total, err := h.svc.ListPlans(r.Context(), tenantID, params)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.NewPagedResponse(dto.PlansFromModels(plans), total, page, limit))
}

// CreateRequest represents a plan creation request.
type CreatePlanRequest struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description"`
}

// Create creates a new plan.
func (h *PlanHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreatePlanRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	userID := ctxutil.GetUserID(r.Context())
	if tenantID.String() == "00000000-0000-0000-0000-000000000000" {
		handleError(w, apierror.Forbidden("missing tenant context"))
		return
	}

	plan, err := h.svc.CreatePlan(r.Context(), tenantID, userID, req.Name, req.Description)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, dto.PlanFromModel(*plan))
}

// Get retrieves a plan by ID.
func (h *PlanHandler) Get(w http.ResponseWriter, r *http.Request) {
	planID, err := parseUUIDParam(chi.URLParam(r, "planId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	plan, err := h.svc.GetPlan(r.Context(), tenantID, planID)
	if err != nil {
		handleError(w, err)
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
		handleError(w, err)
		return
	}

	planID, err := parseUUIDParam(chi.URLParam(r, "planId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdatePlan(r.Context(), tenantID, planID, req.Name, req.Description, req.Status); err != nil {
		handleError(w, err)
		return
	}

	respondNoContent(w)
}

// Delete removes a plan.
func (h *PlanHandler) Delete(w http.ResponseWriter, r *http.Request) {
	planID, err := parseUUIDParam(chi.URLParam(r, "planId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.DeletePlan(r.Context(), tenantID, planID); err != nil {
		handleError(w, err)
		return
	}

	respondNoContent(w)
}
