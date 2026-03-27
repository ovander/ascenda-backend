package repo

import (
	"gorm.io/gorm"
)

// RepoBundle contains all repository instances
type RepoBundle struct {
	// DB is the underlying database handle. Exposed so services can wrap
	// multi-step operations in a single transaction via DB.Transaction(fn).
	DB *gorm.DB

	Plan        *PlanRepo
	Scenario    ScenarioRepository
	Tenant      *TenantRepo
	User        *UserRepo
	AdminStats  *AdminStatsRepo
	Settings   *SettingsRepo
	Product    *ProductRepo
	Staff      *StaffRepo
	Capex      *CapexRepo
	Opex       *OpexRepo
	PnL        *PnLRepo
	FiPlan     *FiplanRepo
	PnlCash    *PnlCashRepo
	WCR        *WCRRepo
	Cash       *CashRepo
	Budget     *BudgetRepo
	Snapshot   *SnapshotRepo
	Audit      *AuditRepo
	PlanMember *PlanMemberRepo

	// AI usage control
	AIUsagePolicy *AIUsagePolicyRepo
	AIUsageRecord *AIUsageRecordRepo

	// Cap Table module (Enterprise tier)
	CapTable *CapTableRepo

	// BEP module (Pro tier and above)
	BEP *BEPRepo

	// Plan-level cap table (Pro tier and above)
	PlanShareholder *PlanShareholderRepo

	// Magic-link sign-in tokens
	MagicLink *MagicLinkRepo

	// Platform-wide country rate configs (admin-editable)
	CountryRateConfig *CountryRateConfigRepo
}

// ── PlanDeps interface implementation ─────────────────────────────────────────
// These thin getters let *RepoBundle satisfy the PlanDeps interface, decoupling
// PlanService from the concrete bundle type.

func (r *RepoBundle) GetDB() *gorm.DB              { return r.DB }
func (r *RepoBundle) GetScenario() ScenarioRepository { return r.Scenario }
func (r *RepoBundle) GetProduct() ProductRepository   { return r.Product }
func (r *RepoBundle) GetStaff() StaffRepository       { return r.Staff }
func (r *RepoBundle) GetCapex() CapexRepository       { return r.Capex }
func (r *RepoBundle) GetOpex() OpexRepository         { return r.Opex }
func (r *RepoBundle) GetPnL() PnLRepository           { return r.PnL }
func (r *RepoBundle) GetFiPlan() FiplanRepository     { return r.FiPlan }
func (r *RepoBundle) GetPnlCash() PnlCashRepository   { return r.PnlCash }
func (r *RepoBundle) GetWCR() WCRRepository           { return r.WCR }
func (r *RepoBundle) GetCash() CashRepository         { return r.Cash }
func (r *RepoBundle) GetBudget() BudgetRepository     { return r.Budget }

// NewRepoBundle creates a new RepoBundle with all repositories initialized
func NewRepoBundle(db *gorm.DB) *RepoBundle {
	return &RepoBundle{
		DB:          db,
		Plan:        NewPlanRepo(db),
		Scenario:    NewScenarioRepo(db),
		Tenant:      NewTenantRepo(db),
		User:        NewUserRepo(db),
		AdminStats:  NewAdminStatsRepo(db),
		Settings:   NewSettingsRepo(db),
		Product:    NewProductRepo(db),
		Staff:      NewStaffRepo(db),
		Capex:      NewCapexRepo(db),
		Opex:       NewOpexRepo(db),
		PnL:        NewPnLRepo(db),
		FiPlan:     NewFiplanRepo(db),
		PnlCash:    NewPnlCashRepo(db),
		WCR:        NewWCRRepo(db),
		Cash:       NewCashRepo(db),
		Budget:     NewBudgetRepo(db),
		Snapshot:   NewSnapshotRepo(db),
		Audit:      NewAuditRepo(db),
		PlanMember: NewPlanMemberRepo(db),
		// AI usage control
		AIUsagePolicy: NewAIUsagePolicyRepo(db),
		AIUsageRecord: NewAIUsageRecordRepo(db),

		// Cap Table module (Enterprise tier)
		CapTable: NewCapTableRepo(db),

		// BEP module (Pro tier and above)
		BEP: NewBEPRepo(db),

		// Plan-level cap table (Pro tier and above)
		PlanShareholder: NewPlanShareholderRepo(db),

		// Magic-link sign-in tokens
		MagicLink: NewMagicLinkRepo(db),

		// Platform-wide country rate configs (admin-editable)
		CountryRateConfig: NewCountryRateConfigRepo(db),
	}
}
