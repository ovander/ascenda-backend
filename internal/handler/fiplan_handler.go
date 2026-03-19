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

// FiplanHandler handles financial plan operations.
type FiplanHandler struct {
	svc    *service.FiplanService
	logger *logrus.Entry
}

// NewFiplanHandler creates a new FiplanHandler.
func NewFiplanHandler(svc *service.FiplanService, logger *logrus.Entry) *FiplanHandler {
	return &FiplanHandler{
		svc:    svc,
		logger: logger,
	}
}

// ListEntries returns all fiplan entries for a scenario.
func (h *FiplanHandler) ListEntries(w http.ResponseWriter, r *http.Request) {
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

	respondJSON(w, http.StatusOK, dto.FiplanEntriesFromModels(entries))
}

// UpdateEntries updates fiplan entries.
func (h *FiplanHandler) UpdateEntries(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	var entries []model.FiplanEntry
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

// GetReport returns the financing plan report.
func (h *FiplanHandler) GetReport(w http.ResponseWriter, r *http.Request) {
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

// GetGrantsForPnL returns grants for P&L inclusion.
func (h *FiplanHandler) GetGrantsForPnL(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	grants, err := h.svc.GetGrantsForPnL(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, grants)
}
