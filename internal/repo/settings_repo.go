package repo

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"ascenda/internal/model"
)

// SettingsRepo handles plan configuration and settings
type SettingsRepo struct {
	db *gorm.DB
}

// NewSettingsRepo creates a new SettingsRepo
func NewSettingsRepo(db *gorm.DB) *SettingsRepo {
	return &SettingsRepo{db: db}
}

// GetConfig retrieves plan configuration
func (r *SettingsRepo) GetConfig(tenantID, scenarioID uuid.UUID) (*model.PlanConfig, error) {
	var config model.PlanConfig
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// UpsertConfig creates or updates plan configuration
func (r *SettingsRepo) UpsertConfig(config *model.PlanConfig) error {
	return r.db.Save(config).Error
}

// GetOpeningBalance retrieves opening balance sheet
func (r *SettingsRepo) GetOpeningBalance(tenantID, scenarioID uuid.UUID) (*model.OpeningBalance, error) {
	var balance model.OpeningBalance
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).First(&balance).Error
	if err != nil {
		return nil, err
	}
	return &balance, nil
}

// UpsertOpeningBalance creates or updates opening balance
func (r *SettingsRepo) UpsertOpeningBalance(balance *model.OpeningBalance) error {
	return r.db.Save(balance).Error
}

// GetWCConfig retrieves working capital configuration
func (r *SettingsRepo) GetWCConfig(tenantID, scenarioID uuid.UUID) (*model.WorkingCapitalConfig, error) {
	var config model.WorkingCapitalConfig
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// UpsertWCConfig creates or updates working capital configuration
func (r *SettingsRepo) UpsertWCConfig(config *model.WorkingCapitalConfig) error {
	return r.db.Save(config).Error
}

// GetOpexPerHire retrieves operating expense per hire settings
func (r *SettingsRepo) GetOpexPerHire(tenantID, scenarioID uuid.UUID) (*model.OpexPerHire, error) {
	var opexPerHire model.OpexPerHire
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).First(&opexPerHire).Error
	if err != nil {
		return nil, err
	}
	return &opexPerHire, nil
}

// UpsertOpexPerHire creates or updates operating expense per hire settings
func (r *SettingsRepo) UpsertOpexPerHire(opexPerHire *model.OpexPerHire) error {
	return r.db.Save(opexPerHire).Error
}

// GetCapexPerHire retrieves capital expenditure per hire settings
func (r *SettingsRepo) GetCapexPerHire(tenantID, scenarioID uuid.UUID) (*model.CapexPerHire, error) {
	var capexPerHire model.CapexPerHire
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).First(&capexPerHire).Error
	if err != nil {
		return nil, err
	}
	return &capexPerHire, nil
}

// UpsertCapexPerHire creates or updates capital expenditure per hire settings
func (r *SettingsRepo) UpsertCapexPerHire(capexPerHire *model.CapexPerHire) error {
	return r.db.Save(capexPerHire).Error
}

// ListMultiYearAdjustments retrieves all multi-year adjustments for a scenario
func (r *SettingsRepo) ListMultiYearAdjustments(tenantID, scenarioID uuid.UUID) ([]*model.MultiYearAdjustment, error) {
	var adjustments []*model.MultiYearAdjustment
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Order("year_index").
		Find(&adjustments).Error
	return adjustments, err
}

// BatchUpsertMultiYearAdjustments creates or updates multi-year adjustments
func (r *SettingsRepo) BatchUpsertMultiYearAdjustments(tenantID, scenarioID uuid.UUID, adjustments []model.MultiYearAdjustment) error {
	for i := range adjustments {
		adjustments[i].TenantID = tenantID
		adjustments[i].ScenarioID = scenarioID
	}
	return r.db.Save(&adjustments).Error
}
