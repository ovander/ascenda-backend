package repo

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"ascenda/internal/model"
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

// BatchUpsert creates or updates P&L cash entries.
// Conflict target: (scenario_id, line_id, year) — enforced by uix_pnl_cash_entries.
func (r *PnlCashRepo) BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.PnlCashEntry) error {
	now := time.Now()
	for i := range entries {
		entries[i].TenantID = tenantID
		entries[i].ScenarioID = scenarioID
		entries[i].UpdatedAt = now
		if entries[i].ID == uuid.Nil {
			entries[i].ID = uuid.New()
		}
	}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "scenario_id"},
			{Name: "line_id"},
			{Name: "year"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"amount", "updated_at"}),
	}).Create(&entries).Error
}
