package repo

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"kerplan/internal/model"
)

// BudgetRepo handles budget monthly override data operations
type BudgetRepo struct {
	db *gorm.DB
}

// NewBudgetRepo creates a new BudgetRepo
func NewBudgetRepo(db *gorm.DB) *BudgetRepo {
	return &BudgetRepo{db: db}
}

// ListByScenario retrieves all budget overrides for a scenario and year
func (r *BudgetRepo) ListByScenario(tenantID, scenarioID uuid.UUID, year int) ([]*model.BudgetMonthlyOverride, error) {
	var entries []*model.BudgetMonthlyOverride
	err := r.db.Where("tenant_id = ? AND scenario_id = ? AND year = ?", tenantID, scenarioID, year).
		Order("month").
		Find(&entries).Error
	return entries, err
}

// ListAllByScenario retrieves all budget overrides for a scenario across all years
func (r *BudgetRepo) ListAllByScenario(tenantID, scenarioID uuid.UUID) ([]*model.BudgetMonthlyOverride, error) {
	var entries []*model.BudgetMonthlyOverride
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Order("year, month").
		Find(&entries).Error
	return entries, err
}

// BatchUpsert creates or updates budget override entries
func (r *BudgetRepo) BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.BudgetMonthlyOverride) error {
	for i := range entries {
		entries[i].TenantID = tenantID
	}
	return r.db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&entries).Error
}
