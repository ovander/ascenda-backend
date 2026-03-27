package repo

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"ascenda/internal/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// CapTableRepo — single repo struct covering all nine cap table tables.
// ─────────────────────────────────────────────────────────────────────────────

// CapTableRepo handles all cap table data operations.
type CapTableRepo struct {
	db *gorm.DB
}

// NewCapTableRepo creates a new CapTableRepo.
func NewCapTableRepo(db *gorm.DB) *CapTableRepo {
	return &CapTableRepo{db: db}
}

// ─── CapTableCompany ──────────────────────────────────────────────────────────

// GetCompany retrieves the company record for a scenario (one per scenario).
func (r *CapTableRepo) GetCompany(tenantID, scenarioID uuid.UUID) (*model.CapTableCompany, error) {
	var company model.CapTableCompany
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		First(&company).Error
	if err != nil {
		return nil, err
	}
	return &company, nil
}

// UpsertCompany creates or updates the company record.
func (r *CapTableRepo) UpsertCompany(company *model.CapTableCompany) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "scenario_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"company_name", "legal_form", "creation_date",
			"currency", "currency_symbol", "nominal_value_cents",
			"initial_shares", "initial_capital_k",
			"display_language", "date_format",
			"book_equity_term", "share_capital_term",
			"max_phases", "founder_alert_pct", "updated_at",
		}),
	}).Create(company).Error
}

// ─── CapTableShareClass ───────────────────────────────────────────────────────

// ListShareClasses retrieves all share classes for a scenario.
func (r *CapTableRepo) ListShareClasses(tenantID, scenarioID uuid.UUID) ([]*model.CapTableShareClass, error) {
	var rows []*model.CapTableShareClass
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Order("sort_order, created_at").
		Find(&rows).Error
	return rows, err
}

// UpsertShareClass creates or updates a share class.
func (r *CapTableRepo) UpsertShareClass(sc *model.CapTableShareClass) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "scenario_id"}, {Name: "class_type"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"label", "voting_rights", "voting_multiple",
			"liquidation_pref", "liquidation_multiple", "participation_cap",
			"anti_dilution", "conversion_ratio", "dividend_rate_pct",
			"sort_order", "updated_at",
		}),
	}).Create(sc).Error
}

// deleteTenantRow is a helper that deletes a single row filtered by tenant_id
// and id, returning gorm.ErrRecordNotFound when no row was matched (so the
// handler layer can return HTTP 404 instead of 200).
func (r *CapTableRepo) deleteTenantRow(tenantID, id uuid.UUID, dest interface{}) error {
	result := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).Delete(dest)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteShareClass deletes a share class by ID.
func (r *CapTableRepo) DeleteShareClass(tenantID, id uuid.UUID) error {
	return r.deleteTenantRow(tenantID, id, &model.CapTableShareClass{})
}

// ─── CapTableShareholder ──────────────────────────────────────────────────────

// ListShareholders retrieves all shareholders for a scenario.
func (r *CapTableRepo) ListShareholders(tenantID, scenarioID uuid.UUID) ([]*model.CapTableShareholder, error) {
	var rows []*model.CapTableShareholder
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Order("sort_order, created_at").
		Find(&rows).Error
	return rows, err
}

// CreateShareholder inserts a new shareholder.
func (r *CapTableRepo) CreateShareholder(sh *model.CapTableShareholder) error {
	return r.db.Create(sh).Error
}

// UpdateShareholder saves changes to an existing shareholder.
func (r *CapTableRepo) UpdateShareholder(sh *model.CapTableShareholder) error {
	return r.db.Save(sh).Error
}

// DeleteShareholder removes a shareholder by ID.
func (r *CapTableRepo) DeleteShareholder(tenantID, id uuid.UUID) error {
	return r.deleteTenantRow(tenantID, id, &model.CapTableShareholder{})
}

// ─── CapTableRound ────────────────────────────────────────────────────────────

// ListRounds retrieves all rounds for a scenario, ordered by phase number.
func (r *CapTableRepo) ListRounds(tenantID, scenarioID uuid.UUID) ([]*model.CapTableRound, error) {
	var rows []*model.CapTableRound
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Order("phase_number, sort_order").
		Find(&rows).Error
	return rows, err
}

// GetRound retrieves a single round by ID.
func (r *CapTableRepo) GetRound(tenantID, id uuid.UUID) (*model.CapTableRound, error) {
	var row model.CapTableRound
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// CreateRound inserts a new round.
func (r *CapTableRepo) CreateRound(rnd *model.CapTableRound) error {
	return r.db.Create(rnd).Error
}

// UpdateRound saves changes to an existing round.
func (r *CapTableRepo) UpdateRound(rnd *model.CapTableRound) error {
	return r.db.Save(rnd).Error
}

// DeleteRound removes a round by ID.
func (r *CapTableRepo) DeleteRound(tenantID, id uuid.UUID) error {
	return r.deleteTenantRow(tenantID, id, &model.CapTableRound{})
}

// ─── CapTablePosition ─────────────────────────────────────────────────────────

// ListPositionsByRound retrieves all shareholder positions for a given round.
func (r *CapTableRepo) ListPositionsByRound(tenantID, roundID uuid.UUID) ([]*model.CapTablePosition, error) {
	var rows []*model.CapTablePosition
	err := r.db.Where("tenant_id = ? AND round_id = ?", tenantID, roundID).
		Find(&rows).Error
	return rows, err
}

// ListPositionsByScenario retrieves all positions across all rounds for a scenario.
// Joins via cap_table_rounds to filter by scenario.
func (r *CapTableRepo) ListPositionsByScenario(tenantID, scenarioID uuid.UUID) ([]*model.CapTablePosition, error) {
	var rows []*model.CapTablePosition
	err := r.db.
		Joins("JOIN cap_table_rounds ON cap_table_rounds.id = cap_table_positions.round_id").
		Where("cap_table_positions.tenant_id = ? AND cap_table_rounds.scenario_id = ?", tenantID, scenarioID).
		Find(&rows).Error
	return rows, err
}

// BatchUpsertPositions creates or updates positions for one round.
func (r *CapTableRepo) BatchUpsertPositions(tenantID, roundID uuid.UUID, positions []model.CapTablePosition) error {
	for i := range positions {
		positions[i].TenantID = tenantID
		positions[i].RoundID = roundID
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "round_id"}, {Name: "shareholder_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"shares_before_split", "shares_after_split",
			"new_shares_received", "shares_after_round",
			"pct_basic_before", "pct_basic_after",
			"pct_fully_diluted", "dilution_delta",
			"implied_value_k", "amount_invested_k", "updated_at",
		}),
	}).Create(&positions).Error
}

// ─── StockOptionPlan ─────────────────────────────────────────────────────────

// ListPlans retrieves all stock option plans for a scenario.
func (r *CapTableRepo) ListPlans(tenantID, scenarioID uuid.UUID) ([]*model.StockOptionPlan, error) {
	var rows []*model.StockOptionPlan
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Order("sort_order, created_at").
		Find(&rows).Error
	return rows, err
}

// GetPlan retrieves a single stock option plan by ID.
func (r *CapTableRepo) GetPlan(tenantID, id uuid.UUID) (*model.StockOptionPlan, error) {
	var row model.StockOptionPlan
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// CreatePlan inserts a new stock option plan.
func (r *CapTableRepo) CreatePlan(plan *model.StockOptionPlan) error {
	return r.db.Create(plan).Error
}

// UpdatePlan saves changes to an existing plan.
func (r *CapTableRepo) UpdatePlan(plan *model.StockOptionPlan) error {
	return r.db.Save(plan).Error
}

// DeletePlan removes a stock option plan by ID.
func (r *CapTableRepo) DeletePlan(tenantID, id uuid.UUID) error {
	return r.deleteTenantRow(tenantID, id, &model.StockOptionPlan{})
}

// ─── OptionGrant ─────────────────────────────────────────────────────────────

// ListGrantsByPlan retrieves all grants for one plan.
func (r *CapTableRepo) ListGrantsByPlan(tenantID, planID uuid.UUID) ([]*model.OptionGrant, error) {
	var rows []*model.OptionGrant
	err := r.db.Where("tenant_id = ? AND plan_id = ?", tenantID, planID).
		Find(&rows).Error
	return rows, err
}

// ListGrantsByScenario retrieves all grants across all plans for a scenario.
func (r *CapTableRepo) ListGrantsByScenario(tenantID, scenarioID uuid.UUID) ([]*model.OptionGrant, error) {
	var rows []*model.OptionGrant
	err := r.db.
		Joins("JOIN stock_option_plans ON stock_option_plans.id = option_grants.plan_id").
		Where("option_grants.tenant_id = ? AND stock_option_plans.scenario_id = ?", tenantID, scenarioID).
		Find(&rows).Error
	return rows, err
}

// CreateGrant inserts a new option grant.
func (r *CapTableRepo) CreateGrant(grant *model.OptionGrant) error {
	return r.db.Create(grant).Error
}

// UpdateGrant saves changes to an existing grant.
func (r *CapTableRepo) UpdateGrant(grant *model.OptionGrant) error {
	return r.db.Save(grant).Error
}

// DeleteGrant removes an option grant by ID.
func (r *CapTableRepo) DeleteGrant(tenantID, id uuid.UUID) error {
	return r.deleteTenantRow(tenantID, id, &model.OptionGrant{})
}

// ─── ValuationScenario ────────────────────────────────────────────────────────

// ListValuationScenarios retrieves all valuation scenarios for a Ascenda scenario.
func (r *CapTableRepo) ListValuationScenarios(tenantID, scenarioID uuid.UUID) ([]*model.ValuationScenario, error) {
	var rows []*model.ValuationScenario
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Order("created_at").
		Find(&rows).Error
	return rows, err
}

// GetValuationScenario retrieves a single valuation scenario by ID.
func (r *CapTableRepo) GetValuationScenario(tenantID, id uuid.UUID) (*model.ValuationScenario, error) {
	var row model.ValuationScenario
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// CreateValuationScenario inserts a new valuation scenario.
func (r *CapTableRepo) CreateValuationScenario(vs *model.ValuationScenario) error {
	return r.db.Create(vs).Error
}

// UpdateValuationScenario saves changes to an existing valuation scenario.
func (r *CapTableRepo) UpdateValuationScenario(vs *model.ValuationScenario) error {
	return r.db.Save(vs).Error
}

// DeleteValuationScenario removes a valuation scenario by ID.
func (r *CapTableRepo) DeleteValuationScenario(tenantID, id uuid.UUID) error {
	return r.deleteTenantRow(tenantID, id, &model.ValuationScenario{})
}

// ─── CapTableScenarioBranch ───────────────────────────────────────────────────

// ListBranches retrieves all scenario branches for a Ascenda scenario.
func (r *CapTableRepo) ListBranches(tenantID, scenarioID uuid.UUID) ([]*model.CapTableScenarioBranch, error) {
	var rows []*model.CapTableScenarioBranch
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Order("created_at").
		Find(&rows).Error
	return rows, err
}

// CreateBranch inserts a new scenario branch.
func (r *CapTableRepo) CreateBranch(branch *model.CapTableScenarioBranch) error {
	return r.db.Create(branch).Error
}

// UpdateBranch saves changes to a scenario branch.
func (r *CapTableRepo) UpdateBranch(branch *model.CapTableScenarioBranch) error {
	return r.db.Save(branch).Error
}

// DeleteBranch removes a scenario branch by ID.
func (r *CapTableRepo) DeleteBranch(tenantID, id uuid.UUID) error {
	return r.deleteTenantRow(tenantID, id, &model.CapTableScenarioBranch{})
}
