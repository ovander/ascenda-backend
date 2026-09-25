package repo

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"ascenda/internal/model"
)

// This file adds DeleteByScenario to every repository whose tables a plan
// snapshot captures. SnapshotService calls them, inside one transaction, to
// empty a scenario before writing a snapshot back, so that a restore replaces
// the scenario's data instead of merging into it (duplicated products,
// leftover entries the snapshot never contained).

// deleteByScenario removes every row of the given models that belongs to the
// scenario. All models must carry tenant_id and scenario_id columns.
func deleteByScenario(db *gorm.DB, tenantID, scenarioID uuid.UUID, models ...interface{}) error {
	for _, m := range models {
		if err := db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).Delete(m).Error; err != nil {
			return err
		}
	}
	return nil
}

// DeleteCoreByScenario removes the scenario's plan config, opening balance and
// working-capital config — the three settings tables a snapshot captures. The
// per-hire and multi-year adjustment tables are left untouched.
func (r *SettingsRepo) DeleteCoreByScenario(tenantID, scenarioID uuid.UUID) error {
	return deleteByScenario(r.db, tenantID, scenarioID,
		&model.PlanConfig{}, &model.OpeningBalance{}, &model.WorkingCapitalConfig{})
}

// DeleteByScenario removes every product of the scenario together with its
// assumptions, sales volumes and distributor margins.
func (r *ProductRepo) DeleteByScenario(tenantID, scenarioID uuid.UUID) error {
	products := r.db.Model(&model.Product{}).Select("id").
		Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID)
	for _, child := range []interface{}{
		&model.ProductAssumption{}, &model.ProductSalesVolume{}, &model.ProductDistributorMargin{},
	} {
		if err := r.db.Where("tenant_id = ? AND product_id IN (?)", tenantID, products).Delete(child).Error; err != nil {
			return err
		}
	}
	return deleteByScenario(r.db, tenantID, scenarioID, &model.Product{})
}

// DeleteByScenario removes the scenario's headcounts, salaries and incentives.
func (r *StaffRepo) DeleteByScenario(tenantID, scenarioID uuid.UUID) error {
	return deleteByScenario(r.db, tenantID, scenarioID,
		&model.StaffHeadcount{}, &model.StaffSalary{}, &model.StaffIncentive{})
}

// DeleteByScenario removes the scenario's capex entries.
func (r *CapexRepo) DeleteByScenario(tenantID, scenarioID uuid.UUID) error {
	return deleteByScenario(r.db, tenantID, scenarioID, &model.CapexEntry{})
}

// DeleteByScenario removes the scenario's manual opex entries.
func (r *OpexRepo) DeleteByScenario(tenantID, scenarioID uuid.UUID) error {
	return deleteByScenario(r.db, tenantID, scenarioID, &model.OpexManualEntry{})
}

// DeleteByScenario removes the scenario's manual P&L entries.
func (r *PnLRepo) DeleteByScenario(tenantID, scenarioID uuid.UUID) error {
	return deleteByScenario(r.db, tenantID, scenarioID, &model.PnlManualEntry{})
}

// DeleteByScenario removes the scenario's financing plan entries.
func (r *FiplanRepo) DeleteByScenario(tenantID, scenarioID uuid.UUID) error {
	return deleteByScenario(r.db, tenantID, scenarioID, &model.FiplanEntry{})
}

// DeleteByScenario removes the scenario's P&L-to-cash entries.
func (r *PnlCashRepo) DeleteByScenario(tenantID, scenarioID uuid.UUID) error {
	return deleteByScenario(r.db, tenantID, scenarioID, &model.PnlCashEntry{})
}

// DeleteByScenario removes the scenario's working-capital requirement entries.
func (r *WCRRepo) DeleteByScenario(tenantID, scenarioID uuid.UUID) error {
	return deleteByScenario(r.db, tenantID, scenarioID, &model.WCREntry{})
}

// DeleteByScenario removes the scenario's monthly cash overrides.
func (r *CashRepo) DeleteByScenario(tenantID, scenarioID uuid.UUID) error {
	return deleteByScenario(r.db, tenantID, scenarioID, &model.CashMonthlyOverride{})
}

// DeleteByScenario removes the scenario's monthly budget overrides for every year.
func (r *BudgetRepo) DeleteByScenario(tenantID, scenarioID uuid.UUID) error {
	return deleteByScenario(r.db, tenantID, scenarioID, &model.BudgetMonthlyOverride{})
}
