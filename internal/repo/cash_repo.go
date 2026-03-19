package repo

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"kerplan/internal/model"
)

// CashRepo handles cash monthly override data operations
type CashRepo struct {
	db *gorm.DB
}

// NewCashRepo creates a new CashRepo
func NewCashRepo(db *gorm.DB) *CashRepo {
	return &CashRepo{db: db}
}

// ListByScenario retrieves all cash overrides for a scenario
func (r *CashRepo) ListByScenario(tenantID, scenarioID uuid.UUID) ([]*model.CashMonthlyOverride, error) {
	var entries []*model.CashMonthlyOverride
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Order("year_index, month").
		Find(&entries).Error
	return entries, err
}

// BatchUpsert creates or updates cash override entries
func (r *CashRepo) BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.CashMonthlyOverride) error {
	for i := range entries {
		entries[i].TenantID = tenantID
	}
	return r.db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&entries).Error
}
