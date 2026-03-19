package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"kerplan/internal/pkg/ctxutil"
	"kerplan/internal/service"
)

// ReportHandler handles full plan report operations.
type ReportHandler struct {
	svc    *service.ReportService
	logger *logrus.Entry
}

// NewReportHandler creates a new ReportHandler.
func NewReportHandler(svc *service.ReportService, logger *logrus.Entry) *ReportHandler {
	return &ReportHandler{
		svc:    svc,
		logger: logger,
	}
}

// GetFullReport returns the complete plan report with all sections.
func (h *ReportHandler) GetFullReport(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	fullReport, err := h.svc.GetFullReport(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, fullReport)
}
