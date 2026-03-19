package repo

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"kerplan/internal/model"
)

// CapexRepo handles capital expenditure data operations
type CapexRepo struct {
	db *gorm.DB
}

// NewCapexRepo creates a new CapexRepo
func NewCapexRepo(db *gorm.DB) *CapexRepo {
	return &CapexRepo{db: db}
}

// ListByScenario retrieves all capex entries for a scenario
func (r *CapexRepo) ListByScenario(tenantID, scenarioID uuid.UUID) ([]*model.CapexEntry, error) {
	var entries []*model.CapexEntry
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Order("category_index, year").
		Find(&entries).Error
	return entries, err
}

// BatchUpsert creates or updates capex entries
func (r *CapexRepo) BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.CapexEntry) error {
	for i := range entries {
		entries[i].TenantID = tenantID
	}
	return r.db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&entries).Error
}
