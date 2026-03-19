package repo

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"kerplan/internal/model"
)

// WCRRepo handles working capital requirement data operations
type WCRRepo struct {
	db *gorm.DB
}

// NewWCRRepo creates a new WCRRepo
func NewWCRRepo(db *gorm.DB) *WCRRepo {
	return &WCRRepo{db: db}
}

// ListByScenario retrieves all WCR entries for a scenario
func (r *WCRRepo) ListByScenario(tenantID, scenarioID uuid.UUID) ([]*model.WCREntry, error) {
	var entries []*model.WCREntry
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Order("line_id, year").
		Find(&entries).Error
	return entries, err
}

// BatchUpsert creates or updates WCR entries
func (r *WCRRepo) BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.WCREntry) error {
	for i := range entries {
		entries[i].TenantID = tenantID
	}
	return r.db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&entries).Error
}
