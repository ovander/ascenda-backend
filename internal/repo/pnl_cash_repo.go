package repo

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"kerplan/internal/model"
)

// PnlCashRepo handles simplified P&L cash data operations
type PnlCashRepo struct {
	db *gorm.DB
}

// NewPnlCashRepo creates a new PnlCashRepo
func NewPnlCashRepo(db *gorm.DB) *PnlCashRepo {
	return &PnlCashRepo{db: db}
}

// ListByScenario retrieves all P&L cash entries for a scenario
func (r *PnlCashRepo) ListByScenario(tenantID, scenarioID uuid.UUID) ([]*model.PnlCashEntry, error) {
	var entries []*model.PnlCashEntry
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Order("line_id, year").
		Find(&entries).Error
	return entries, err
}

// BatchUpsert creates or updates P&L cash entries
func (r *PnlCashRepo) BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.PnlCashEntry) error {
	for i := range entries {
		entries[i].TenantID = tenantID
	}
	return r.db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&entries).Error
}
