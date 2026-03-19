package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"kerplan/internal/pkg/ctxutil"
	"kerplan/internal/service"
)

// RatiosHandler handles financial ratios operations (read-only).
type RatiosHandler struct {
	svc    *service.RatiosService
	logger *logrus.Entry
}

// NewRatiosHandler creates a new RatiosHandler.
func NewRatiosHandler(svc *service.RatiosService, logger *logrus.Entry) *RatiosHandler {
	return &RatiosHandler{
		svc:    svc,
		logger: logger,
	}
}

// GetReport returns the financial ratios report.
func (h *RatiosHandler) GetReport(w http.ResponseWriter, r *http.Request) {
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

// GetChartData returns chart-formatted ratios data.
func (h *RatiosHandler) GetChartData(w http.ResponseWriter, r *http.Request) {
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
