package repo

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"ascenda/internal/model"
)

// PnLRepo handles profit and loss data operations
type PnLRepo struct {
	db *gorm.DB
}

// NewPnLRepo creates a new PnLRepo
func NewPnLRepo(db *gorm.DB) *PnLRepo {
	return &PnLRepo{db: db}
}

// ListByScenario retrieves all P&L entries for a scenario
func (r *PnLRepo) ListByScenario(tenantID, scenarioID uuid.UUID) ([]*model.PnlManualEntry, error) {
	var entries []*model.PnlManualEntry
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Order("line_id, year_index").
		Find(&entries).Error
	return entries, err
}

// BatchUpsert creates or updates P&L entries
func (r *PnLRepo) BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.PnlManualEntry) error {
	for i := range entries {
		entries[i].TenantID = tenantID
	}
	return r.db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&entries).Error
}
