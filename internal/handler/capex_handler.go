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

// CapexHandler handles capital expenditure operations.
type CapexHandler struct {
	svc    *service.CapexService
	logger *logrus.Entry
}

// NewCapexHandler creates a new CapexHandler.
func NewCapexHandler(svc *service.CapexService, logger *logrus.Entry) *CapexHandler {
	return &CapexHandler{
		svc:    svc,
		logger: logger,
	}
}

// ListEntries returns all capex entries for a scenario.
func (h *CapexHandler) ListEntries(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	entries, err := h.svc.ListEntries(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.CapexEntriesFromModels(entries))
}

// UpdateEntries updates capex entries.
func (h *CapexHandler) UpdateEntries(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	var entries []model.CapexEntry
	if err := decodeAndValidate(r, &entries); err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateEntries(r.Context(), tenantID, scenarioID, entries); err != nil {
		handleError(w, err)
		return
	}

	respondNoContent(w)
}

// GetSummary returns capex summary and depreciation.
func (h *CapexHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	summary, err := h.svc.GetSummary(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, summary)
}
