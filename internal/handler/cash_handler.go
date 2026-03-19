package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"kerplan/internal/dto"
	"kerplan/internal/model"
	"kerplan/internal/pkg/ctxutil"
	"kerplan/internal/service"
)

// CashHandler handles monthly cash flow operations.
type CashHandler struct {
	svc    *service.CashService
	logger *logrus.Entry
}

// NewCashHandler creates a new CashHandler.
func NewCashHandler(svc *service.CashService, logger *logrus.Entry) *CashHandler {
	return &CashHandler{
		svc:    svc,
		logger: logger,
	}
}

// ListOverrides returns all cash flow overrides for a scenario.
func (h *CashHandler) ListOverrides(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	overrides, err := h.svc.ListOverrides(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, err)
		return
	}

	dtoOverrides := dto.CashOverridesFromModels(overrides)
	respondJSON(w, http.StatusOK, dto.NewPagedResponse(dtoOverrides, int64(len(dtoOverrides)), 0, len(dtoOverrides)))
}

// UpdateOverrides updates cash flow overrides.
func (h *CashHandler) UpdateOverrides(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	var overrides []model.CashMonthlyOverride
	if err := decodeAndValidate(r, &overrides); err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateOverrides(r.Context(), tenantID, scenarioID, overrides); err != nil {
		handleError(w, err)
		return
	}

	respondNoContent(w)
}

// GetReport returns the monthly cash flow report.
func (h *CashHandler) GetReport(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	report, err := h.svc.GetReport(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, report)
}
