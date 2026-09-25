package repo

import (
	"time"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StaffRepo handles staff-related data operations
type StaffRepo struct {
	db *gorm.DB
}

// NewStaffRepo creates a new StaffRepo
func NewStaffRepo(db *gorm.DB) *StaffRepo {
	return &StaffRepo{db: db}
}

// ListHeadcountsByScenario retrieves all headcounts for a scenario
func (r *StaffRepo) ListHeadcountsByScenario(tenantID, scenarioID uuid.UUID) ([]*model.StaffHeadcount, error) {
	var headcounts []*model.StaffHeadcount
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Order("category, year").
		Find(&headcounts).Error
	return headcounts, err
}

// BatchUpsertHeadcounts creates or updates headcount entries.
// Conflict target: (scenario_id, category, year) — the composite unique key.
func (r *StaffRepo) BatchUpsertHeadcounts(tenantID, scenarioID uuid.UUID, headcounts []model.StaffHeadcount) error {
	now := time.Now()
	for i := range headcounts {
		headcounts[i].TenantID = tenantID
		headcounts[i].ScenarioID = scenarioID
		headcounts[i].UpdatedAt = now
		if headcounts[i].ID == uuid.Nil {
			headcounts[i].ID = uuid.New()
		}
	}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "scenario_id"},
			{Name: "category"},
			{Name: "year"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"fte", "is_overridden", "updated_at"}),
	}).Create(&headcounts).Error
}

// ListSalariesByScenario retrieves all salaries for a scenario
func (r *StaffRepo) ListSalariesByScenario(tenantID, scenarioID uuid.UUID) ([]*model.StaffSalary, error) {
	var salaries []*model.StaffSalary
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Order("category, year").
		Find(&salaries).Error
	return salaries, err
}

// BatchUpsertSalaries creates or updates salary entries.
// Conflict target: (scenario_id, category, year) — the composite unique key.
func (r *StaffRepo) BatchUpsertSalaries(tenantID, scenarioID uuid.UUID, salaries []model.StaffSalary) error {
	now := time.Now()
	for i := range salaries {
		salaries[i].TenantID = tenantID
		salaries[i].ScenarioID = scenarioID
		salaries[i].UpdatedAt = now
		if salaries[i].ID == uuid.Nil {
			salaries[i].ID = uuid.New()
		}
	}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "scenario_id"},
			{Name: "category"},
			{Name: "year"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"monthly_gross_salary", "annual_increase_pct", "is_overridden", "updated_at"}),
	}).Create(&salaries).Error
}

// ListIncentivesByScenario retrieves all incentives for a scenario
func (r *StaffRepo) ListIncentivesByScenario(tenantID, scenarioID uuid.UUID) ([]*model.StaffIncentive, error) {
	var incentives []*model.StaffIncentive
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Order("year").
		Find(&incentives).Error
	return incentives, err
}

// BatchUpsertIncentives creates or updates incentive entries.
// Conflict target: (scenario_id, year) — the composite unique key.
func (r *StaffRepo) BatchUpsertIncentives(tenantID, scenarioID uuid.UUID, incentives []model.StaffIncentive) error {
	now := time.Now()
	for i := range incentives {
		incentives[i].TenantID = tenantID
		incentives[i].ScenarioID = scenarioID
		incentives[i].UpdatedAt = now
		if incentives[i].ID == uuid.Nil {
			incentives[i].ID = uuid.New()
		}
	}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "scenario_id"},
			{Name: "year"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"incentive_pct", "specific_incentives", "is_overridden", "updated_at"}),
	}).Create(&incentives).Error
}
