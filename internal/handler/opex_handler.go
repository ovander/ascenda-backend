package handler

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"ascenda/internal/dto"
	"ascenda/internal/model"
	"ascenda/internal/pkg/ctxutil"
)

// OpexServicer interface for dependency injection.
type OpexServicer interface {
	ListEntries(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.OpexManualEntry, error)
	UpdateEntries(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.OpexManualEntry) error
	GetSummary(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.OpexSummary, error)
}

// OpexHandler handles operating expense operations.
type OpexHandler struct {
	svc    OpexServicer
	logger *logrus.Entry
}

// NewOpexHandler creates a new OpexHandler.
func NewOpexHandler(svc OpexServicer, logger *logrus.Entry) *OpexHandler {
	return &OpexHandler{
		svc:    svc,
		logger: logger,
	}
}

// ListEntries returns all opex entries for a scenario.
func (h *OpexHandler) ListEntries(w http.ResponseWriter, r *http.Request) {
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

	respondJSON(w, http.StatusOK, dto.OpexEntriesFromModels(entries))
}

// UpdateEntries updates opex entries.
func (h *OpexHandler) UpdateEntries(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	var entries []model.OpexManualEntry
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

// GetSummary returns opex aggregation.
func (h *OpexHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	summary, err := h.svc.GetSummary(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, summary)
}
