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

// OpexHandler handles operating expense operations.
type OpexHandler struct {
	svc    *service.OpexService
	logger *logrus.Entry
}

// NewOpexHandler creates a new OpexHandler.
func NewOpexHandler(svc *service.OpexService, logger *logrus.Entry) *OpexHandler {
	return &OpexHandler{
		svc:    svc,
		logger: logger,
	}
}

// ListEntries returns all opex entries for a scenario.
func (h *OpexHandler) ListEntries(w http.ResponseWriter, r *http.Request) {
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

	respondJSON(w, http.StatusOK, dto.OpexEntriesFromModels(entries))
}

// UpdateEntries updates opex entries.
func (h *OpexHandler) UpdateEntries(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	var entries []model.OpexManualEntry
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

// GetSummary returns opex aggregation.
func (h *OpexHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
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
