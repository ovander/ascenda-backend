package repo

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"ascenda/internal/model"
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

// ListByTenant retrieves a page of plans for a tenant.
func (r *PlanRepo) ListByTenant(tenantID uuid.UUID, offset, limit int) ([]*model.BusinessPlan, error) {
	var plans []*model.BusinessPlan
	err := r.db.Where("tenant_id = ?", tenantID).
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&plans).Error
	return plans, err
}

// CountByTenant returns the total number of plans for a tenant (used for pagination totals).
func (r *PlanRepo) CountByTenant(tenantID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&model.BusinessPlan{}).
		Where("tenant_id = ?", tenantID).
		Count(&count).Error
	return count, err
}

// Update updates a business plan
func (r *PlanRepo) Update(plan *model.BusinessPlan) error {
	return r.db.Save(plan).Error
}

// Delete deletes a business plan.
// Returns gorm.ErrRecordNotFound when no row matching both tenant_id and id is
// found, which the handler layer maps to HTTP 404.
func (r *PlanRepo) Delete(tenantID, planID uuid.UUID) error {
	result := r.db.Where("tenant_id = ? AND id = ?", tenantID, planID).Delete(&model.BusinessPlan{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// PurgeDemoPlans deletes all demo plans for a tenant and every child record
// they own (scenarios, products, staff, capex, opex, settings, members …).
// All deletes run inside a single transaction in dependency order so they
// are safe whether or not the database has FK constraints.
func (r *PlanRepo) PurgeDemoPlans(tenantID uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 1. Collect scenario IDs that belong to demo plans.
		var scenarioIDs []uuid.UUID
		if err := tx.Raw(
			`SELECT s.id FROM scenarios s
			 JOIN business_plans p ON p.id = s.plan_id
			 WHERE p.tenant_id = ? AND p.is_demo = true`, tenantID,
		).Scan(&scenarioIDs).Error; err != nil {
			return err
		}

		if len(scenarioIDs) > 0 {
			// 2. Products first — child tables (assumptions, volumes, margins) depend on product_id.
			var productIDs []uuid.UUID
			if err := tx.Raw(
				`SELECT id FROM products WHERE tenant_id = ? AND scenario_id IN (?)`,
				tenantID, scenarioIDs,
			).Scan(&productIDs).Error; err != nil {
				return err
			}
			if len(productIDs) > 0 {
				for _, tbl := range []string{
					"product_assumptions",
					"product_sales_volumes",
					"product_distributor_margins",
				} {
					if err := tx.Exec(
						`DELETE FROM `+tbl+` WHERE tenant_id = ? AND product_id IN (?)`,
						tenantID, productIDs,
					).Error; err != nil {
						return err
					}
				}
				if err := tx.Exec(
					`DELETE FROM products WHERE tenant_id = ? AND id IN (?)`,
					tenantID, productIDs,
				).Error; err != nil {
					return err
				}
			}

			// 3. All other scenario-scoped tables.
			// Use SAVEPOINT per table so a missing table doesn't abort the transaction.
			for _, tbl := range []string{
				"plan_configs", "opening_balances", "working_capital_configs",
				"opex_per_hire", "capex_per_hire", "multi_year_adjustments",
				"staff_headcounts", "staff_salaries", "staff_incentives",
				"capex_entries", "opex_manual_entries",
				"pnl_manual_entries", "fiplan_entries", "pnl_cash_entries",
				"wcr_entries", "cash_monthly_overrides", "budget_monthly_overrides",
				"reports", "plan_snapshots",
				"bep_snapshots", "bep_fixed_cost_lines", "bep_variable_cost_lines",
				"bep_sensitivity_configs", "bep_pcg_review_items",
			} {
				sp := "sp_purge_" + tbl
				_ = tx.Exec("SAVEPOINT " + sp).Error
				if err := tx.Exec(
					`DELETE FROM `+tbl+` WHERE tenant_id = ? AND scenario_id IN (?)`,
					tenantID, scenarioIDs,
				).Error; err != nil {
					_ = tx.Exec("ROLLBACK TO SAVEPOINT " + sp).Error
				} else {
					_ = tx.Exec("RELEASE SAVEPOINT " + sp).Error
				}
			}

			// 4. Delete the scenarios themselves.
			if err := tx.Exec(
				`DELETE FROM scenarios WHERE tenant_id = ? AND id IN (?)`,
				tenantID, scenarioIDs,
			).Error; err != nil {
				return err
			}
		}

		// 5. Delete plan_members, then the plans.
		if err := tx.Exec(
			`DELETE FROM plan_members WHERE plan_id IN
			 (SELECT id FROM business_plans WHERE tenant_id = ? AND is_demo = true)`,
			tenantID,
		).Error; err != nil {
			return err
		}

		return tx.Exec(
			`DELETE FROM business_plans WHERE tenant_id = ? AND is_demo = true`, tenantID,
		).Error
	})
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

// Delete deletes a scenario.
// Returns gorm.ErrRecordNotFound when no row matching both tenant_id and id is
// found, which the handler layer maps to HTTP 404.
func (r *ScenarioRepo) Delete(tenantID, scenarioID uuid.UUID) error {
	result := r.db.Where("tenant_id = ? AND id = ?", tenantID, scenarioID).Delete(&model.Scenario{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
