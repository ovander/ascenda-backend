package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"kerplan/internal/event"
	"kerplan/internal/model"
	"kerplan/internal/pkg/apierror"
	"kerplan/internal/repo"
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
	config.TenantID = tenantID
	config.ScenarioID = scenarioID

	if err := s.settingsRepo.UpsertConfig(config); err != nil {
		s.logger.WithError(err).Error("failed to update config")
		return apierror.Internal("failed to update config")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("config updated")
	s.emitter.Publish(event.Event{Type: event.DataChanged, TenantID: tenantID, ScenarioID: scenarioID, EntityType: "config", Action: event.ActionUpdate})
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
	s.emitter.Publish(event.Event{Type: event.DataChanged, TenantID: tenantID, ScenarioID: scenarioID, EntityType: "opening_balance", Action: event.ActionUpdate})
	return nil
}

// GetWCConfig retrieves the working capital configuration for a scenario.
// Returns empty defaults if no record exists yet.
func (s *SettingsService) GetWCConfig(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.WorkingCapitalConfig, error) {
	wcConfig, err := s.settingsRepo.GetWCConfig(tenantID, scenarioID)
	if err != nil || wcConfig == nil {
		// Return empty defaults when record doesn't exist
		return &model.WorkingCapitalConfig{
			TenantScoped: model.TenantScoped{ID: uuid.New()},
			ScenarioID:   scenarioID,
		}, nil
	}

	return wcConfig, nil
}

// UpdateWCConfig updates the working capital configuration for a scenario.
func (s *SettingsService) UpdateWCConfig(ctx context.Context, tenantID, scenarioID uuid.UUID, wcConfig *model.WorkingCapitalConfig) error {
	wcConfig.TenantID = tenantID
	wcConfig.ScenarioID = scenarioID

	if err := s.settingsRepo.UpsertWCConfig(wcConfig); err != nil {
		s.logger.WithError(err).Error("failed to update wc config")
		return apierror.Internal("failed to update wc config")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("wc config updated")
	s.emitter.Publish(event.Event{Type: event.DataChanged, TenantID: tenantID, ScenarioID: scenarioID, EntityType: "wc_config", Action: event.ActionUpdate})
	return nil
}

// GetOpexPerHire retrieves the operating expense per hire settings for a scenario.
func (s *SettingsService) GetOpexPerHire(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.OpexPerHire, error) {
	opexPerHire, err := s.settingsRepo.GetOpexPerHire(tenantID, scenarioID)
	if err != nil || opexPerHire == nil {
		return &model.OpexPerHire{
			TenantScoped: model.TenantScoped{ID: uuid.New()},
			ScenarioID:   scenarioID,
		}, nil
	}

	return opexPerHire, nil
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
	s.emitter.Publish(event.Event{Type: event.DataChanged, TenantID: tenantID, ScenarioID: scenarioID, EntityType: "opex_per_hire", Action: event.ActionUpdate})
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
	s.emitter.Publish(event.Event{Type: event.DataChanged, TenantID: tenantID, ScenarioID: scenarioID, EntityType: "capex_per_hire", Action: event.ActionUpdate})
	return nil
}
