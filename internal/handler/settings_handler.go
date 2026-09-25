package handler

import (
	"context"
	"net/http"
	"time"

	"ascenda/internal/model"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

// computePlanConfig wraps a PlanConfig with computed derived fields.
// If ForecastStart is zero (unset), the current year is used as a safe fallback.
func computePlanConfig(config *model.PlanConfig) *model.PlanConfigComputed {
	startYear := config.ForecastStart.Year()
	if startYear < 2000 {
		startYear = time.Now().Year()
	}
	currencySymbol := config.CurrencySymbol
	if currencySymbol == "" {
		currencySymbol = "€"
	}
	return &model.PlanConfigComputed{
		PlanConfig:     *config,
		CurrentDate:    time.Now(),
		FirstCivilYear: startYear,
		OverdraftRate:  config.MLTInterestRate.Add(decimal.NewFromFloat(0.03)),
		YearHeaders:    [5]int{startYear, startYear + 1, startYear + 2, startYear + 3, startYear + 4},
		UnitLabel:      "k" + currencySymbol,
	}
}

// SettingsServicer defines the interface the SettingsHandler depends on.
type SettingsServicer interface {
	GetConfig(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.PlanConfig, error)
	UpdateConfig(ctx context.Context, tenantID, scenarioID uuid.UUID, config *model.PlanConfig) error
	GetOpeningBalance(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.OpeningBalance, error)
	UpdateOpeningBalance(ctx context.Context, tenantID, scenarioID uuid.UUID, balance *model.OpeningBalance) error
	GetWCConfig(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.WorkingCapitalConfig, error)
	UpdateWCConfig(ctx context.Context, tenantID, scenarioID uuid.UUID, wcConfig *model.WorkingCapitalConfig) error
	GetOpexPerHire(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.OpexPerHire, error)
	UpdateOpexPerHire(ctx context.Context, tenantID, scenarioID uuid.UUID, opexPerHire *model.OpexPerHire) error
	GetCapexPerHire(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.CapexPerHire, error)
	UpdateCapexPerHire(ctx context.Context, tenantID, scenarioID uuid.UUID, capexPerHire *model.CapexPerHire) error
}

// SettingsHandler handles plan configuration operations.
type SettingsHandler struct {
	svc    SettingsServicer
	logger *logrus.Entry
}

// NewSettingsHandler creates a new SettingsHandler.
func NewSettingsHandler(svc SettingsServicer, logger *logrus.Entry) *SettingsHandler {
	return &SettingsHandler{
		svc:    svc,
		logger: logger,
	}
}

// GetConfig retrieves the plan configuration.
func (h *SettingsHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	config, err := h.svc.GetConfig(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, computePlanConfig(config))
}

// UpdateConfig updates the plan configuration and returns the computed result.
func (h *SettingsHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	var config model.PlanConfig
	if err := decodeAndValidate(r, &config); err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateConfig(r.Context(), tenantID, scenarioID, &config); err != nil {
		handleError(w, r, err)
		return
	}

	// Re-fetch to return the persisted config with computed fields
	updated, err := h.svc.GetConfig(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, computePlanConfig(updated))
}

// GetOpeningBalance retrieves the opening balance.
func (h *SettingsHandler) GetOpeningBalance(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	balance, err := h.svc.GetOpeningBalance(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, balance)
}

// UpdateOpeningBalance updates the opening balance.
func (h *SettingsHandler) UpdateOpeningBalance(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	var balance model.OpeningBalance
	if err := decodeAndValidate(r, &balance); err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateOpeningBalance(r.Context(), tenantID, scenarioID, &balance); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// GetWCConfig retrieves the working capital configuration.
func (h *SettingsHandler) GetWCConfig(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	wcConfig, err := h.svc.GetWCConfig(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, wcConfig)
}

// UpdateWCConfig updates the working capital configuration.
func (h *SettingsHandler) UpdateWCConfig(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	var wcConfig model.WorkingCapitalConfig
	if err := decodeAndValidate(r, &wcConfig); err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateWCConfig(r.Context(), tenantID, scenarioID, &wcConfig); err != nil {
		handleError(w, r, err)
		return
	}

	// Return the saved config so the frontend store can update wcConfig.value
	// with the real persisted state (including the resolved primary key).
	respondJSON(w, http.StatusOK, wcConfig)
}

// GetOpexPerHire retrieves the operating expense per hire settings.
func (h *SettingsHandler) GetOpexPerHire(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	opexPerHire, err := h.svc.GetOpexPerHire(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, opexPerHire)
}

// UpdateOpexPerHire updates the operating expense per hire settings.
func (h *SettingsHandler) UpdateOpexPerHire(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	var opexPerHire model.OpexPerHire
	if err := decodeAndValidate(r, &opexPerHire); err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateOpexPerHire(r.Context(), tenantID, scenarioID, &opexPerHire); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// GetCapexPerHire retrieves the capital expenditure per hire settings.
func (h *SettingsHandler) GetCapexPerHire(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	capexPerHire, err := h.svc.GetCapexPerHire(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, capexPerHire)
}

// UpdateCapexPerHire updates the capital expenditure per hire settings.
func (h *SettingsHandler) UpdateCapexPerHire(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	var capexPerHire model.CapexPerHire
	if err := decodeAndValidate(r, &capexPerHire); err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateCapexPerHire(r.Context(), tenantID, scenarioID, &capexPerHire); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}
