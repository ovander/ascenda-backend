package repo

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"kerplan/internal/model"
)

// FiplanRepo handles financial planning data operations
type FiplanRepo struct {
	db *gorm.DB
}

// NewFiplanRepo creates a new FiplanRepo
func NewFiplanRepo(db *gorm.DB) *FiplanRepo {
	return &FiplanRepo{db: db}
}

// ListByScenario retrieves all fiplan entries for a scenario
func (r *FiplanRepo) ListByScenario(tenantID, scenarioID uuid.UUID) ([]*model.FiplanEntry, error) {
	var entries []*model.FiplanEntry
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Order("line_id, year").
		Find(&entries).Error
	return entries, err
}

// BatchUpsert creates or updates fiplan entries
func (r *FiplanRepo) BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.FiplanEntry) error {
	for i := range entries {
		entries[i].TenantID = tenantID
	}
	return r.db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&entries).Error
}
