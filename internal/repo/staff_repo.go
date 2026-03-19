package repo

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"kerplan/internal/model"
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
		Order("role_index, year").
		Find(&headcounts).Error
	return headcounts, err
}

// BatchUpsertHeadcounts creates or updates headcount entries
func (r *StaffRepo) BatchUpsertHeadcounts(tenantID, scenarioID uuid.UUID, headcounts []model.StaffHeadcount) error {
	for i := range headcounts {
		headcounts[i].TenantID = tenantID
	}
	return r.db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&headcounts).Error
}

// ListSalariesByScenario retrieves all salaries for a scenario
func (r *StaffRepo) ListSalariesByScenario(tenantID, scenarioID uuid.UUID) ([]*model.StaffSalary, error) {
	var salaries []*model.StaffSalary
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Order("role_index, year").
		Find(&salaries).Error
	return salaries, err
}

// BatchUpsertSalaries creates or updates salary entries
func (r *StaffRepo) BatchUpsertSalaries(tenantID, scenarioID uuid.UUID, salaries []model.StaffSalary) error {
	for i := range salaries {
		salaries[i].TenantID = tenantID
	}
	return r.db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&salaries).Error
}

// ListIncentivesByScenario retrieves all incentives for a scenario
func (r *StaffRepo) ListIncentivesByScenario(tenantID, scenarioID uuid.UUID) ([]*model.StaffIncentive, error) {
	var incentives []*model.StaffIncentive
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Order("role_index, year").
		Find(&incentives).Error
	return incentives, err
}

// BatchUpsertIncentives creates or updates incentive entries
func (r *StaffRepo) BatchUpsertIncentives(tenantID, scenarioID uuid.UUID, incentives []model.StaffIncentive) error {
	for i := range incentives {
		incentives[i].TenantID = tenantID
	}
	return r.db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&incentives).Error
}
