package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"kerplan/internal/dto"
	"kerplan/internal/model"
	"kerplan/internal/pkg/ctxutil"
	"kerplan/internal/service"
)

// BudgetHandler handles budget operations.
type BudgetHandler struct {
	svc    *service.BudgetService
	logger *logrus.Entry
}

// NewBudgetHandler creates a new BudgetHandler.
func NewBudgetHandler(svc *service.BudgetService, logger *logrus.Entry) *BudgetHandler {
	return &BudgetHandler{
		svc:    svc,
		logger: logger,
	}
}

// ListOverrides returns all budget overrides for a scenario and year.
func (h *BudgetHandler) ListOverrides(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	if year < 1 || year > 5 {
		year = 1
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	overrides, err := h.svc.ListOverrides(r.Context(), tenantID, scenarioID, year)
	if err != nil {
		handleError(w, err)
		return
	}

	dtoOverrides := dto.BudgetOverridesFromModels(overrides)
	respondJSON(w, http.StatusOK, dto.NewPagedResponse(dtoOverrides, int64(len(dtoOverrides)), 0, len(dtoOverrides)))
}

// UpdateOverrides updates budget overrides.
func (h *BudgetHandler) UpdateOverrides(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	if year < 1 || year > 5 {
		year = 1
	}

	var overrides []model.BudgetMonthlyOverride
	if err := decodeAndValidate(r, &overrides); err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateOverrides(r.Context(), tenantID, scenarioID, year, overrides); err != nil {
		handleError(w, err)
		return
	}

	respondNoContent(w)
}

// GetBudget1Report returns year 1 budget analysis.
func (h *BudgetHandler) GetBudget1Report(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	report, err := h.svc.GetBudget1Report(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, report)
}

// GetBudget2Report returns year 2 budget analysis.
func (h *BudgetHandler) GetBudget2Report(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	report, err := h.svc.GetBudget2Report(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, report)
}
