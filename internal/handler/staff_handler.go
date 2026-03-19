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

// StaffHandler handles staff operations.
type StaffHandler struct {
	svc    *service.StaffService
	logger *logrus.Entry
}

// NewStaffHandler creates a new StaffHandler.
func NewStaffHandler(svc *service.StaffService, logger *logrus.Entry) *StaffHandler {
	return &StaffHandler{
		svc:    svc,
		logger: logger,
	}
}

// ListHeadcounts returns all headcount records for a scenario.
func (h *StaffHandler) ListHeadcounts(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	headcounts, err := h.svc.ListHeadcounts(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.HeadcountsFromModels(headcounts))
}

// ListSalaries returns all salary records for a scenario.
func (h *StaffHandler) ListSalaries(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	salaries, err := h.svc.ListSalaries(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.SalariesFromModels(salaries))
}

// ListIncentives returns all incentive records for a scenario.
func (h *StaffHandler) ListIncentives(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	incentives, err := h.svc.ListIncentives(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.IncentivesFromModels(incentives))
}

// UpdateHeadcounts updates headcount records.
func (h *StaffHandler) UpdateHeadcounts(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	var headcounts []model.StaffHeadcount
	if err := decodeAndValidate(r, &headcounts); err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateHeadcounts(r.Context(), tenantID, scenarioID, headcounts); err != nil {
		handleError(w, err)
		return
	}

	respondNoContent(w)
}

// UpdateSalaries updates salary records.
func (h *StaffHandler) UpdateSalaries(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	var salaries []model.StaffSalary
	if err := decodeAndValidate(r, &salaries); err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateSalaries(r.Context(), tenantID, scenarioID, salaries); err != nil {
		handleError(w, err)
		return
	}

	respondNoContent(w)
}

// UpdateIncentives updates incentive records.
func (h *StaffHandler) UpdateIncentives(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	var incentives []model.StaffIncentive
	if err := decodeAndValidate(r, &incentives); err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateIncentives(r.Context(), tenantID, scenarioID, incentives); err != nil {
		handleError(w, err)
		return
	}

	respondNoContent(w)
}

// GetPayrollSummary returns aggregated payroll metrics.
func (h *StaffHandler) GetPayrollSummary(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	summary, err := h.svc.GetPayrollSummary(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, summary)
}
