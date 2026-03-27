package repo

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"ascenda/internal/model"
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

// BatchUpsert creates or updates fiplan entries.
// Conflict target: (scenario_id, line_id, year) — enforced by uix_fiplan_entries.
// Note: does NOT update cap_table_round_id / cap_table_round_label — those are
// managed exclusively via UpsertCapitalIncreaseEntry / ClearCapTableLink.
func (r *FiplanRepo) BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.FiplanEntry) error {
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

// UpsertCapitalIncreaseEntry inserts or updates the capital_increase FiplanEntry
// for the given fiscalYearIndex (0-based, 0 = Year 1 … 4 = Year 5), also setting
// the cap table link fields.
// FiplanEntry.YearIndex is stored 1-based to match the frontend convention, so
// the conversion fiscalYearIndex → yearIndex+1 happens here at the repo boundary.
// Conflict target: (scenario_id, line_id, year).
func (r *FiplanRepo) UpsertCapitalIncreaseEntry(tenantID, scenarioID uuid.UUID, yearIndex int, amount decimal.Decimal, roundID uuid.UUID, roundLabel string) error {
	entry := model.FiplanEntry{
		ScenarioID:         scenarioID,
		LineID:             model.FiplanCapitalIncrease,
		YearIndex:          yearIndex + 1, // convert 0-based fiscal year → 1-based FiPlan year
		Amount:             amount,
		CapTableRoundID:    &roundID,
		CapTableRoundLabel: roundLabel,
	}
	entry.ID = uuid.New()
	entry.TenantID = tenantID
	entry.UpdatedAt = time.Now()

	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "scenario_id"},
			{Name: "line_id"},
			{Name: "year"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"amount", "cap_table_round_id", "cap_table_round_label", "updated_at",
		}),
	}).Create(&entry).Error
}

// ClearCapTableLink removes the cap table link fields from the capital_increase entry
// for the given fiscalYearIndex (0-based). The amount is left unchanged.
// yearIndex+1 converts to the 1-based FiPlan convention stored in the DB.
func (r *FiplanRepo) ClearCapTableLink(tenantID, scenarioID uuid.UUID, yearIndex int) error {
	return r.db.Model(&model.FiplanEntry{}).
		Where("tenant_id = ? AND scenario_id = ? AND line_id = ? AND year = ?",
			tenantID, scenarioID, model.FiplanCapitalIncrease, yearIndex+1).
		Updates(map[string]any{
			"cap_table_round_id":    nil,
			"cap_table_round_label": "",
			"updated_at":            time.Now(),
		}).Error
}
