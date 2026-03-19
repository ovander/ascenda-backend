package repo

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"kerplan/internal/model"
)

// PlanRepo handles business plan data operations
type PlanRepo struct {
	db *gorm.DB
}

// NewPlanRepo creates a new PlanRepo
func NewPlanRepo(db *gorm.DB) *PlanRepo {
	return &PlanRepo{db: db}
}

// Create creates a new business plan
func (r *PlanRepo) Create(plan *model.BusinessPlan) error {
	return r.db.Create(plan).Error
}

// GetByID retrieves a plan by ID with tenant filtering
func (r *PlanRepo) GetByID(tenantID, planID uuid.UUID) (*model.BusinessPlan, error) {
	var plan model.BusinessPlan
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, planID).First(&plan).Error
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

// ListByTenant retrieves all plans for a tenant
func (r *PlanRepo) ListByTenant(tenantID uuid.UUID, offset, limit int) ([]*model.BusinessPlan, error) {
	var plans []*model.BusinessPlan
	err := r.db.Where("tenant_id = ?", tenantID).
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&plans).Error
	return plans, err
}

// Update updates a business plan
func (r *PlanRepo) Update(plan *model.BusinessPlan) error {
	return r.db.Save(plan).Error
}

// Delete deletes a business plan
func (r *PlanRepo) Delete(tenantID, planID uuid.UUID) error {
	return r.db.Where("tenant_id = ? AND id = ?", tenantID, planID).Delete(&model.BusinessPlan{}).Error
}

// ScenarioRepo handles scenario data operations
type ScenarioRepo struct {
	db *gorm.DB
}

// NewScenarioRepo creates a new ScenarioRepo
func NewScenarioRepo(db *gorm.DB) *ScenarioRepo {
	return &ScenarioRepo{db: db}
}

// Create creates a new scenario
func (r *ScenarioRepo) Create(scenario *model.Scenario) error {
	return r.db.Create(scenario).Error
}

// GetByID retrieves a scenario by ID with tenant filtering
func (r *ScenarioRepo) GetByID(tenantID, scenarioID uuid.UUID) (*model.Scenario, error) {
	var scenario model.Scenario
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, scenarioID).First(&scenario).Error
	if err != nil {
		return nil, err
	}
	return &scenario, nil
}

// ListByPlan retrieves all scenarios for a plan
func (r *ScenarioRepo) ListByPlan(tenantID, planID uuid.UUID) ([]*model.Scenario, error) {
	var scenarios []*model.Scenario
	err := r.db.Where("tenant_id = ? AND plan_id = ?", tenantID, planID).
		Order("created_at").
		Find(&scenarios).Error
	return scenarios, err
}

// Update updates a scenario
func (r *ScenarioRepo) Update(scenario *model.Scenario) error {
	return r.db.Save(scenario).Error
}

// Delete deletes a scenario
func (r *ScenarioRepo) Delete(tenantID, scenarioID uuid.UUID) error {
	return r.db.Where("tenant_id = ? AND id = ?", tenantID, scenarioID).Delete(&model.Scenario{}).Error
}
