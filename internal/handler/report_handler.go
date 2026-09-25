package handler

import (
	"context"
	"net/http"

	"ascenda/internal/model"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"
)

// ReportServicer interface for dependency injection.
type ReportServicer interface {
	GetFullReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.FullPlanOutput, error)
}

// ReportHandler handles full plan report operations.
type ReportHandler struct {
	svc    ReportServicer
	logger *logrus.Entry
}

// NewReportHandler creates a new ReportHandler.
func NewReportHandler(svc ReportServicer, logger *logrus.Entry) *ReportHandler {
	return &ReportHandler{
		svc:    svc,
		logger: logger,
	}
}

// GetFullReport returns the complete plan report with all sections.
func (h *ReportHandler) GetFullReport(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	fullReport, err := h.svc.GetFullReport(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, fullReport)
}
