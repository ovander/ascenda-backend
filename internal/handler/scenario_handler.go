package handler

import (
	"context"
	"net/http"
	"strings"

	"ascenda/internal/dto"
	"ascenda/internal/model"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"
)

// ScenarioServicer is the narrow interface the ScenarioHandler depends on.
// Using an interface instead of the concrete *service.PlanService makes the
// handler independently unit-testable without standing up a full service graph.
type ScenarioServicer interface {
	ListScenarios(ctx context.Context, tenantID, planID uuid.UUID) ([]model.Scenario, error)
	GetScenario(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.Scenario, error)
	CreateScenario(ctx context.Context, tenantID, planID uuid.UUID, name, description string) (*model.Scenario, error)
	UpdateScenario(ctx context.Context, tenantID, scenarioID uuid.UUID, name, description string) error
	DeleteScenario(ctx context.Context, tenantID, scenarioID uuid.UUID) error
	CloneScenario(ctx context.Context, tenantID, scenarioID uuid.UUID, newName string) (*model.Scenario, error)
}

// ScenarioHandler handles scenario operations.
type ScenarioHandler struct {
	svc    ScenarioServicer
	logger *logrus.Entry
}

// NewScenarioHandler creates a new ScenarioHandler.
func NewScenarioHandler(svc ScenarioServicer, logger *logrus.Entry) *ScenarioHandler {
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
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	scenarios, err := h.svc.ListScenarios(r.Context(), tenantID, planID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.ScenariosFromModels(scenarios))
}

// CreateRequest represents a scenario creation request.
type CreateScenarioRequest struct {
	Name        string `json:"name"        validate:"required"`
	Description string `json:"description"`
}

// Create creates a new scenario.
func (h *ScenarioHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateScenarioRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	planID, err := parseUUIDParam(extractPlanID(r))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	scenario, err := h.svc.CreateScenario(r.Context(), tenantID, planID, req.Name, req.Description)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusCreated, dto.ScenarioFromModel(*scenario))
}

// Get retrieves a scenario by ID.
func (h *ScenarioHandler) Get(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	scenario, err := h.svc.GetScenario(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.ScenarioFromModel(*scenario))
}

// UpdateRequest represents a scenario update request.
type UpdateScenarioRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Update updates a scenario.
func (h *ScenarioHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req UpdateScenarioRequest
	if err := decodeJSON(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateScenario(r.Context(), tenantID, scenarioID, req.Name, req.Description); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// Delete removes a scenario.
func (h *ScenarioHandler) Delete(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.DeleteScenario(r.Context(), tenantID, scenarioID); err != nil {
		handleError(w, r, err)
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
		handleError(w, r, err)
		return
	}

	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	newScenario, err := h.svc.CloneScenario(r.Context(), tenantID, scenarioID, req.Name)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusCreated, dto.ScenarioFromModel(*newScenario))
}
