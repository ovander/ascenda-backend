package handler

import (
	"context"
	"net/http"

	"ascenda/internal/dto"
	"ascenda/internal/model"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"
)

// PnlCashServicer interface for dependency injection.
type PnlCashServicer interface {
	ListEntries(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.PnlCashEntry, error)
	UpdateEntries(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.PnlCashEntry) error
	GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.PnlCashReport, error)
	GetChartData(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]interface{}, error)
}

// PnlCashHandler handles P&L + Cash operations.
type PnlCashHandler struct {
	svc    PnlCashServicer
	logger *logrus.Entry
}

// NewPnlCashHandler creates a new PnlCashHandler.
func NewPnlCashHandler(svc PnlCashServicer, logger *logrus.Entry) *PnlCashHandler {
	return &PnlCashHandler{
		svc:    svc,
		logger: logger,
	}
}

// ListEntries returns all P&L + Cash entries for a scenario.
func (h *PnlCashHandler) ListEntries(w http.ResponseWriter, r *http.Request) {
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

	respondJSON(w, http.StatusOK, dto.PnlCashEntriesFromModels(entries))
}

// UpdateEntries updates P&L + Cash entries.
func (h *PnlCashHandler) UpdateEntries(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	var entries []model.PnlCashEntry
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

// GetReport returns the P&L + Cash report.
func (h *PnlCashHandler) GetReport(w http.ResponseWriter, r *http.Request) {
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

// GetChartData returns chart-formatted P&L + Cash data.
func (h *PnlCashHandler) GetChartData(w http.ResponseWriter, r *http.Request) {
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
