package handler

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"ascenda/internal/dto"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/ctxutil"
)

// WCRServicer interface for dependency injection.
type WCRServicer interface {
	ListEntries(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.WCREntry, error)
	UpdateEntries(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.WCREntry) error
	GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.WCRReport, error)
	GetChartData(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]interface{}, error)
}

// WCRHandler handles working capital requirement operations.
type WCRHandler struct {
	svc    WCRServicer
	logger *logrus.Entry
}

// NewWCRHandler creates a new WCRHandler.
func NewWCRHandler(svc WCRServicer, logger *logrus.Entry) *WCRHandler {
	return &WCRHandler{
		svc:    svc,
		logger: logger,
	}
}

// ListEntries returns all WCR entries for a scenario.
func (h *WCRHandler) ListEntries(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	entries, err := h.svc.ListEntries(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.WCREntriesFromModels(entries))
}

// UpdateEntries updates WCR entries.
func (h *WCRHandler) UpdateEntries(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	var entries []model.WCREntry
	if err := decodeAndValidate(r, &entries); err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateEntries(r.Context(), tenantID, scenarioID, entries); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// GetReport returns the WCR analysis report.
func (h *WCRHandler) GetReport(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	report, err := h.svc.GetReport(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, report)
}

// GetChartData returns chart-formatted WCR data.
func (h *WCRHandler) GetChartData(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	chartData, err := h.svc.GetChartData(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, chartData)
}
