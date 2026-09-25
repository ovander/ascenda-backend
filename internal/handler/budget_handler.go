package handler

import (
	"context"
	"net/http"
	"strconv"

	"ascenda/internal/dto"
	"ascenda/internal/model"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"
)

// BudgetServicer interface for dependency injection.
type BudgetServicer interface {
	ListOverrides(ctx context.Context, tenantID, scenarioID uuid.UUID, year int) ([]model.BudgetMonthlyOverride, error)
	UpdateOverrides(ctx context.Context, tenantID, scenarioID uuid.UUID, year int, overrides []model.BudgetMonthlyOverride) error
	GetBudget1Report(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.Budget1Report, error)
	GetBudget2Report(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.Budget2Report, error)
}

// BudgetHandler handles budget operations.
type BudgetHandler struct {
	svc    BudgetServicer
	logger *logrus.Entry
}

// NewBudgetHandler creates a new BudgetHandler.
func NewBudgetHandler(svc BudgetServicer, logger *logrus.Entry) *BudgetHandler {
	return &BudgetHandler{
		svc:    svc,
		logger: logger,
	}
}

// ListOverrides returns all budget overrides for a scenario and year.
func (h *BudgetHandler) ListOverrides(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	if year < 1 || year > 5 {
		year = 1
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	overrides, err := h.svc.ListOverrides(r.Context(), tenantID, scenarioID, year)
	if err != nil {
		handleError(w, r, err)
		return
	}

	dtoOverrides := dto.BudgetOverridesFromModels(overrides)
	respondJSON(w, http.StatusOK, dto.NewPagedResponse(dtoOverrides, int64(len(dtoOverrides)), 0, len(dtoOverrides)))
}

// UpdateOverrides updates budget overrides.
func (h *BudgetHandler) UpdateOverrides(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	if year < 1 || year > 5 {
		year = 1
	}

	var overrides []model.BudgetMonthlyOverride
	if err := decodeAndValidate(r, &overrides); err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateOverrides(r.Context(), tenantID, scenarioID, year, overrides); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// GetBudget1Report returns year 1 budget analysis.
func (h *BudgetHandler) GetBudget1Report(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	report, err := h.svc.GetBudget1Report(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, report)
}

// GetBudget2Report returns year 2 budget analysis.
func (h *BudgetHandler) GetBudget2Report(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	report, err := h.svc.GetBudget2Report(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, report)
}
