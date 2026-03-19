package repo

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"kerplan/internal/model"
)

// OpexRepo handles operating expense data operations
type OpexRepo struct {
	db *gorm.DB
}

// NewOpexRepo creates a new OpexRepo
func NewOpexRepo(db *gorm.DB) *OpexRepo {
	return &OpexRepo{db: db}
}

// ListByScenario retrieves all opex entries for a scenario
func (r *OpexRepo) ListByScenario(tenantID, scenarioID uuid.UUID) ([]*model.OpexManualEntry, error) {
	var entries []*model.OpexManualEntry
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Order("line_id, year").
		Find(&entries).Error
	return entries, err
}

// BatchUpsert creates or updates opex entries
func (r *OpexRepo) BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.OpexManualEntry) error {
	for i := range entries {
		entries[i].TenantID = tenantID
	}
	return r.db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&entries).Error
}
