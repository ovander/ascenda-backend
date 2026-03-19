package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"kerplan/internal/pkg/ctxutil"
	"kerplan/internal/service"
)

// BSheetHandler handles balance sheet operations (read-only).
type BSheetHandler struct {
	svc    *service.BSheetService
	logger *logrus.Entry
}

// NewBSheetHandler creates a new BSheetHandler.
func NewBSheetHandler(svc *service.BSheetService, logger *logrus.Entry) *BSheetHandler {
	return &BSheetHandler{
		svc:    svc,
		logger: logger,
	}
}

// GetReport returns the balance sheet report.
func (h *BSheetHandler) GetReport(w http.ResponseWriter, r *http.Request) {
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

// GetChartData returns chart-formatted balance sheet data.
func (h *BSheetHandler) GetChartData(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	chartData, err := h.svc.GetChartData(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, chartData)
}
