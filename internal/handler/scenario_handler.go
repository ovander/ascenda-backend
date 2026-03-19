package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"kerplan/internal/dto"
	"kerplan/internal/pkg/ctxutil"
	"kerplan/internal/service"
)

// ScenarioHandler handles scenario operations.
type ScenarioHandler struct {
	svc    *service.PlanService
	logger *logrus.Entry
}

// NewScenarioHandler creates a new ScenarioHandler.
func NewScenarioHandler(svc *service.PlanService, logger *logrus.Entry) *ScenarioHandler {
	return &ScenarioHandler{
		svc:    svc,
		logger: logger,
	}
}

// extractPlanID extracts the plan ID from chi params, with fallback to URL path parsing.
// This handles cases where nested chi subrouters don't propagate parent params correctly.
func extractPlanID(r *http.Request) string {
	if id := chi.URLParam(r, "planId"); id != "" {
		return id
	}
	if id := chi.URLParam(r, "id"); id != "" {
		return id
	}
	// Fallback: extract from URL path /api/v1/plans/{planId}/...
	parts := strings.Split(r.URL.Path, "/")
	for i, p := range parts {
		if p == "plans" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// List returns all scenarios for a plan.
func (h *ScenarioHandler) List(w http.ResponseWriter, r *http.Request) {
	planID, err := parseUUIDParam(extractPlanID(r))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	scenarios, err := h.svc.ListScenarios(r.Context(), tenantID, planID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.ScenariosFromModels(scenarios))
}

// CreateRequest represents a scenario creation request.
type CreateScenarioRequest struct {
	Name string `json:"name" validate:"required"`
}

// Create creates a new scenario.
func (h *ScenarioHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateScenarioRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, err)
		return
	}

	planID, err := parseUUIDParam(extractPlanID(r))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	scenario, err := h.svc.CreateScenario(r.Context(), tenantID, planID, req.Name)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, dto.ScenarioFromModel(*scenario))
}

// Get retrieves a scenario by ID.
func (h *ScenarioHandler) Get(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	scenario, err := h.svc.GetScenario(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.ScenarioFromModel(*scenario))
}

// UpdateRequest represents a scenario update request.
type UpdateScenarioRequest struct {
	Name string `json:"name"`
}

// Update updates a scenario.
func (h *ScenarioHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req UpdateScenarioRequest
	if err := decodeJSON(r, &req); err != nil {
		handleError(w, err)
		return
	}

	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateScenario(r.Context(), tenantID, scenarioID, req.Name); err != nil {
		handleError(w, err)
		return
	}

	respondNoContent(w)
}

// Delete removes a scenario.
func (h *ScenarioHandler) Delete(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.DeleteScenario(r.Context(), tenantID, scenarioID); err != nil {
		handleError(w, err)
		return
	}

	respondNoContent(w)
}

// CloneRequest represents a scenario cloning request.
type CloneScenarioRequest struct {
	Name string `json:"name" validate:"required"`
}

// Clone clones a scenario to a new one.
func (h *ScenarioHandler) Clone(w http.ResponseWriter, r *http.Request) {
	var req CloneScenarioRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, err)
		return
	}

	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	newScenario, err := h.svc.CloneScenario(r.Context(), tenantID, scenarioID, req.Name)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, dto.ScenarioFromModel(*newScenario))
}
