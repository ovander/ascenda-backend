package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"ascenda/internal/event"
	"ascenda/internal/model"
	"ascenda/internal/pkg/apierror"
	"ascenda/internal/pkg/ctxutil"
	"ascenda/internal/repo"
)

// SettingsService orchestrates settings CRUD operations.
type SettingsService struct {
	settingsRepo repo.SettingsRepository
	emitter      *event.Emitter
	logger       *logrus.Entry
}

// NewSettingsService creates a new SettingsService.
func NewSettingsService(settingsRepo repo.SettingsRepository, emitter *event.Emitter, logger *logrus.Entry) *SettingsService {
	return &SettingsService{
		settingsRepo: settingsRepo,
		emitter:      emitter,
		logger:       logger,
	}
}

// GetConfig retrieves the plan configuration for a scenario.
func (s *SettingsService) GetConfig(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.PlanConfig, error) {
	config, err := s.settingsRepo.GetConfig(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to get config")
		return nil, apierror.Internal("failed to get config")
	}

	if config == nil {
		return nil, apierror.NotFound("config", scenarioID.String())
	}

	return config, nil
}

// UpdateConfig updates the plan configuration for a scenario.
func (s *SettingsService) UpdateConfig(ctx context.Context, tenantID, scenarioID uuid.UUID, config *model.PlanConfig) error {
	if config.CompanyName == "" {
		return apierror.ValidationError("companyName is required", nil)
	}

	config.TenantID = tenantID
	config.ScenarioID = scenarioID

	if err := s.settingsRepo.UpsertConfig(config); err != nil {
		s.logger.WithError(err).Error("failed to update config")
		return apierror.Internal("failed to update config")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("config updated")
	s.emitter.Publish(event.Event{
		Type: event.DataChanged, TenantID: tenantID, UserID: ctxutil.GetUserID(ctx), ScenarioID: scenarioID,
		EntityType: "config", Action: event.ActionUpdate,
		Changes: marshalChanges(map[string]any{
			"companyName":   config.CompanyName,
			"country":       config.Country,
			"forecastStart": config.ForecastStart,
		}),
	})
	return nil
}

// GetOpeningBalance retrieves the opening balance for a scenario.
// Returns empty defaults if no record exists yet.
func (s *SettingsService) GetOpeningBalance(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.OpeningBalance, error) {
	balance, err := s.settingsRepo.GetOpeningBalance(tenantID, scenarioID)
	if err != nil || balance == nil {
		// Return empty defaults when record doesn't exist
		return &model.OpeningBalance{
			TenantScoped: model.TenantScoped{ID: uuid.New()},
			ScenarioID:   scenarioID,
		}, nil
	}

	return balance, nil
}

// UpdateOpeningBalance updates the opening balance for a scenario.
func (s *SettingsService) UpdateOpeningBalance(ctx context.Context, tenantID, scenarioID uuid.UUID, balance *model.OpeningBalance) error {
	balance.TenantID = tenantID
	balance.ScenarioID = scenarioID

	if err := s.settingsRepo.UpsertOpeningBalance(balance); err != nil {
		s.logger.WithError(err).Error("failed to update opening balance")
		return apierror.Internal("failed to update opening balance")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("opening balance updated")
	s.emitter.Publish(event.Event{
		Type: event.DataChanged, TenantID: tenantID, UserID: ctxutil.GetUserID(ctx), ScenarioID: scenarioID,
		EntityType: "opening_balance", Action: event.ActionUpdate,
		Changes: marshalChanges(map[string]any{
			"cashAndSecurities":   balance.CashAndSecurities,
			"shareCapital":        balance.ShareCapital,
			"loansAndDebt":        balance.LoansAndDebt,
			"noncurrentAssets":    balance.NoncurrentAssets,
			"customerReceivables": balance.CustomerReceivables,
		}),
	})
	return nil
}

// wcConfigB2BDefaults returns the canonical B2B default working-capital
// configuration.  It is applied whenever the stored record is absent OR when
// all payment-tranche percentages are still at their zero value — which
// happens for any scenario created before explicit WC settings were saved.
// This is the single source of truth for default WC parameters.
func wcConfigB2BDefaults(tenantID, scenarioID uuid.UUID) *model.WorkingCapitalConfig {
	half   := decimal.NewFromFloat(0.5)
	tenPct := decimal.NewFromFloat(0.1)
	return &model.WorkingCapitalConfig{
		TenantScoped:      model.TenantScoped{TenantID: tenantID, ID: uuid.New()},
		ScenarioID:        scenarioID,
		// Payment tranches: 50 % at 30 days + 50 % at 60 days → DSO = DPO = 45 days
		CustomerPct0Days:  decimal.Zero,
		CustomerPct30Days: half,
		CustomerPct60Days: half,
		CustomerPct90Days: decimal.Zero,
		SupplierPct0Days:  decimal.Zero,
		SupplierPct30Days: half,
		SupplierPct60Days: half,
		SupplierPct90Days: decimal.Zero,
		// Inventory: 10 % of COGS for all plan years
		InventoryPctYear1: tenPct,
		InventoryPctYear2: tenPct,
		InventoryPctYear3: tenPct,
		InventoryPctYear4: tenPct,
		InventoryPctYear5: tenPct,
	}
}

// wcConfigIsUnconfigured returns true when all customer and supplier
// payment-tranche percentages are zero, which indicates that the row was
// created by an earlier code path before B2B defaults were introduced.
func wcConfigIsUnconfigured(cfg *model.WorkingCapitalConfig) bool {
	fields := []decimal.Decimal{
		cfg.CustomerPct0Days, cfg.CustomerPct30Days,
		cfg.CustomerPct60Days, cfg.CustomerPct90Days,
		cfg.SupplierPct0Days, cfg.SupplierPct30Days,
		cfg.SupplierPct60Days, cfg.SupplierPct90Days,
	}
	for _, f := range fields {
		if !f.IsZero() {
			return false
		}
	}
	return true
}

// GetWCConfig retrieves the working capital configuration for a scenario.
// Returns B2B defaults when no record exists, or when the stored record was
// written before payment-tranche defaults were introduced (all pcts zero).
func (s *SettingsService) GetWCConfig(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.WorkingCapitalConfig, error) {
	wcConfig, err := s.settingsRepo.GetWCConfig(tenantID, scenarioID)
	if err != nil || wcConfig == nil || wcConfigIsUnconfigured(wcConfig) {
		return wcConfigB2BDefaults(tenantID, scenarioID), nil
	}
	return wcConfig, nil
}

// UpdateWCConfig updates the working capital configuration for a scenario.
func (s *SettingsService) UpdateWCConfig(ctx context.Context, tenantID, scenarioID uuid.UUID, wcConfig *model.WorkingCapitalConfig) error {
	wcConfig.TenantID = tenantID
	wcConfig.ScenarioID = scenarioID

	// Carry over the existing row's primary key so GORM performs an UPDATE
	// rather than an INSERT.  We bypass the service-level GetWCConfig (which
	// applies the B2B-default fallback and may return a synthetic UUID) and
	// call the repo directly to get the real stored ID.
	if existing, err := s.settingsRepo.GetWCConfig(tenantID, scenarioID); err == nil && existing != nil {
		wcConfig.ID = existing.ID
	}

	if err := s.settingsRepo.UpsertWCConfig(wcConfig); err != nil {
		s.logger.WithError(err).Error("failed to update wc config")
		return apierror.Internal("failed to update wc config")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("wc config updated")
	s.emitter.Publish(event.Event{
		Type: event.DataChanged, TenantID: tenantID, UserID: ctxutil.GetUserID(ctx), ScenarioID: scenarioID,
		EntityType: "wc_config", Action: event.ActionUpdate,
		Changes: marshalChanges(map[string]any{
			"customerPct30Days": wcConfig.CustomerPct30Days,
			"customerPct60Days": wcConfig.CustomerPct60Days,
			"supplierPct30Days": wcConfig.SupplierPct30Days,
			"supplierPct60Days": wcConfig.SupplierPct60Days,
			"inventoryPctYear1": wcConfig.InventoryPctYear1,
		}),
	})
	return nil
}

// GetOpexPerHire retrieves the operating expense per hire settings for a scenario.
// When no record exists yet, sensible defaults are returned so formula-driven
// OPEX lines render non-zero values out of the box.
func (s *SettingsService) GetOpexPerHire(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.OpexPerHire, error) {
	opexPerHire, err := s.settingsRepo.GetOpexPerHire(tenantID, scenarioID)
	if err != nil || opexPerHire == nil || isZeroOpexPerHire(opexPerHire) {
		return defaultOpexPerHire(tenantID, scenarioID), nil
	}

	return opexPerHire, nil
}

// isZeroOpexPerHire returns true when every numeric field is zero,
// which indicates the row was created before defaults were introduced.
func isZeroOpexPerHire(o *model.OpexPerHire) bool {
	return o.PropertyRentals.IsZero() &&
		o.PostageTelecom.IsZero() &&
		o.SuppliesPurchases.IsZero() &&
		o.StudiesDocumentation.IsZero() &&
		o.InsuranceCostsPctSales.IsZero() &&
		o.RoyaltyPaymentsPctSales.IsZero() &&
		o.TravelTransportation.IsZero() &&
		o.MissionRepresentation.IsZero() &&
		o.RecruitTrainingPctPayroll.IsZero()
}

// defaultOpexPerHire returns typical values for a French SaaS startup.
//   - Per-capita fields are in base € per person per year
//   - Percentage fields are decimals (0.005 = 0.5 %)
func defaultOpexPerHire(tenantID, scenarioID uuid.UUID) *model.OpexPerHire {
	d := func(s string) decimal.Decimal { v, _ := decimal.NewFromString(s); return v }
	return &model.OpexPerHire{
		TenantScoped:            model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:              scenarioID,
		PropertyRentals:         d("40000"),  // 40 000 € base annual rent (year 1)
		PostageTelecom:          d("1000"),   // 1 000 €/person/year
		SuppliesPurchases:       d("500"),    // 500 €/person/year
		StudiesDocumentation:    d("300"),    // 300 €/person/year
		InsuranceCostsPctSales:  d("0.005"),  // 0.5 % of sales
		RoyaltyPaymentsPctSales: d("0.01"),   // 1 % of sales
		TravelTransportation:    d("2000"),   // 2 000 €/person/year
		MissionRepresentation:   d("1000"),   // 1 000 €/person/year
		RecruitTrainingPctPayroll: d("0.03"), // 3 % of payroll
	}
}

// UpdateOpexPerHire updates the operating expense per hire settings for a scenario.
func (s *SettingsService) UpdateOpexPerHire(ctx context.Context, tenantID, scenarioID uuid.UUID, opexPerHire *model.OpexPerHire) error {
	opexPerHire.TenantID = tenantID
	opexPerHire.ScenarioID = scenarioID

	if err := s.settingsRepo.UpsertOpexPerHire(opexPerHire); err != nil {
		s.logger.WithError(err).Error("failed to update opex per hire")
		return apierror.Internal("failed to update opex per hire")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("opex per hire updated")
	s.emitter.Publish(event.Event{
		Type: event.DataChanged, TenantID: tenantID, UserID: ctxutil.GetUserID(ctx), ScenarioID: scenarioID,
		EntityType: "opex_per_hire", Action: event.ActionUpdate,
		Changes: marshalChanges(map[string]any{
			"propertyRentals":           opexPerHire.PropertyRentals,
			"postageTelecom":            opexPerHire.PostageTelecom,
			"travelTransportation":      opexPerHire.TravelTransportation,
			"insuranceCostsPctSales":    opexPerHire.InsuranceCostsPctSales,
			"recruitTrainingPctPayroll": opexPerHire.RecruitTrainingPctPayroll,
		}),
	})
	return nil
}

// GetCapexPerHire retrieves the capital expenditure per hire settings for a scenario.
func (s *SettingsService) GetCapexPerHire(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.CapexPerHire, error) {
	capexPerHire, err := s.settingsRepo.GetCapexPerHire(tenantID, scenarioID)
	if err != nil || capexPerHire == nil {
		return &model.CapexPerHire{
			TenantScoped: model.TenantScoped{ID: uuid.New()},
			ScenarioID:   scenarioID,
		}, nil
	}

	return capexPerHire, nil
}

// UpdateCapexPerHire updates the capital expenditure per hire settings for a scenario.
func (s *SettingsService) UpdateCapexPerHire(ctx context.Context, tenantID, scenarioID uuid.UUID, capexPerHire *model.CapexPerHire) error {
	capexPerHire.TenantID = tenantID
	capexPerHire.ScenarioID = scenarioID

	if err := s.settingsRepo.UpsertCapexPerHire(capexPerHire); err != nil {
		s.logger.WithError(err).Error("failed to update capex per hire")
		return apierror.Internal("failed to update capex per hire")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("capex per hire updated")
	s.emitter.Publish(event.Event{
		Type: event.DataChanged, TenantID: tenantID, UserID: ctxutil.GetUserID(ctx), ScenarioID: scenarioID,
		EntityType: "capex_per_hire", Action: event.ActionUpdate,
		Changes: marshalChanges(map[string]any{
			"furniturePerHire": capexPerHire.FurniturePerHire,
			"itEquipPerHire":   capexPerHire.ITEquipPerHire,
		}),
	})
	return nil
}
