package repo

import (
	"gorm.io/gorm"
)

// RepoBundle contains all repository instances
type RepoBundle struct {
	Plan       *PlanRepo
	Scenario   *ScenarioRepo
	Tenant     *TenantRepo
	User       *UserRepo
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
}

// NewRepoBundle creates a new RepoBundle with all repositories initialized
func NewRepoBundle(db *gorm.DB) *RepoBundle {
	return &RepoBundle{
		Plan:       NewPlanRepo(db),
		Scenario:   NewScenarioRepo(db),
		Tenant:     NewTenantRepo(db),
		User:       NewUserRepo(db),
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
	}
}
