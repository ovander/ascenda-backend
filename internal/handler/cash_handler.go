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

// CashServicer interface for dependency injection.
type CashServicer interface {
	ListOverrides(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CashMonthlyOverride, error)
	UpdateOverrides(ctx context.Context, tenantID, scenarioID uuid.UUID, overrides []model.CashMonthlyOverride) error
	GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.CashReport, error)
}

// CashHandler handles monthly cash flow operations.
type CashHandler struct {
	svc    CashServicer
	logger *logrus.Entry
}

// NewCashHandler creates a new CashHandler.
func NewCashHandler(svc CashServicer, logger *logrus.Entry) *CashHandler {
	return &CashHandler{
		svc:    svc,
		logger: logger,
	}
}

// ListOverrides returns all cash flow overrides for a scenario.
func (h *CashHandler) ListOverrides(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	overrides, err := h.svc.ListOverrides(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	dtoOverrides := dto.CashOverridesFromModels(overrides)
	respondJSON(w, http.StatusOK, dto.NewPagedResponse(dtoOverrides, int64(len(dtoOverrides)), 0, len(dtoOverrides)))
}

// UpdateOverrides updates cash flow overrides.
func (h *CashHandler) UpdateOverrides(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	var overrides []model.CashMonthlyOverride
	if err := decodeAndValidate(r, &overrides); err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateOverrides(r.Context(), tenantID, scenarioID, overrides); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// GetReport returns the monthly cash flow report.
func (h *CashHandler) GetReport(w http.ResponseWriter, r *http.Request) {
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
