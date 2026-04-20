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

// PnLServicer interface for dependency injection.
type PnLServicer interface {
	ListManualEntries(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.PnlManualEntry, error)
	UpdateManualEntries(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.PnlManualEntry) error
	GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.PnlReport, error)
	GetChartData(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]interface{}, error)
}

// PnLHandler handles P&L operations.
type PnLHandler struct {
	svc    PnLServicer
	logger *logrus.Entry
}

// NewPnLHandler creates a new PnLHandler.
func NewPnLHandler(svc PnLServicer, logger *logrus.Entry) *PnLHandler {
	return &PnLHandler{
		svc:    svc,
		logger: logger,
	}
}

// ListManualEntries returns all manual P&L entries for a scenario.
func (h *PnLHandler) ListManualEntries(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	entries, err := h.svc.ListManualEntries(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.PnlEntriesFromModels(entries))
}

// UpdateManualEntries updates P&L entries.
func (h *PnLHandler) UpdateManualEntries(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	var entries []model.PnlManualEntry
	if err := decodeAndValidate(r, &entries); err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateManualEntries(r.Context(), tenantID, scenarioID, entries); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// GetReport returns the full P&L statement.
func (h *PnLHandler) GetReport(w http.ResponseWriter, r *http.Request) {
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

// GetChartData returns chart-formatted P&L data.
func (h *PnLHandler) GetChartData(w http.ResponseWriter, r *http.Request) {
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
