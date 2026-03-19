package handler

import (
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"kerplan/internal/config"
	"kerplan/internal/repo"
	"kerplan/internal/service"
)

// AdminHandlers groups system and user management handlers.
type AdminHandlers struct {
	Health   *HealthHandler
	Auth     *AuthHandler
	User     *UserHandler
	Tenant   *TenantHandler
	Metadata *MetadataHandler
}

// PlanHandlers groups plan lifecycle handlers.
type PlanHandlers struct {
	Plan       *PlanHandler
	Scenario   *ScenarioHandler
	Settings   *SettingsHandler
	Snapshot   *SnapshotHandler
	PlanMember *PlanMemberHandler
}

// FinanceHandlers groups financial data and report handlers.
type FinanceHandlers struct {
	Product *ProductHandler
	Staff   *StaffHandler
	Capex   *CapexHandler
	Opex    *OpexHandler
	PnL     *PnLHandler
	FiPlan  *FiplanHandler
	PnlCash *PnlCashHandler
	BSheet  *BSheetHandler
	Ratios  *RatiosHandler
	WCR     *WCRHandler
	Cash    *CashHandler
	Budget  *BudgetHandler
	Report  *ReportHandler
}

// HandlerBundle contains all handler instances grouped by domain.
type HandlerBundle struct {
	Admin   AdminHandlers
	Plans   PlanHandlers
	Finance FinanceHandlers
}

// NewHandlerBundle creates a new HandlerBundle with all handlers initialized.
func NewHandlerBundle(services *service.ServiceBundle, repos *repo.RepoBundle, db *gorm.DB, cfg *config.Config, logger *logrus.Entry) *HandlerBundle {
	return &HandlerBundle{
		Admin: AdminHandlers{
			Health:   NewHealthHandler(db, logger),
			Auth:     NewAuthHandler(cfg, logger),
			User:     NewUserHandler(services.User, logger),
			Tenant:   NewTenantHandler(logger),
			Metadata: NewMetadataHandler(cfg.Version, cfg.BuildTime, cfg.GitCommit, logger),
		},
		Plans: PlanHandlers{
			Plan:       NewPlanHandler(services.Plan, logger),
			Scenario:   NewScenarioHandler(services.Plan, logger),
			Settings:   NewSettingsHandler(services.Settings, logger),
			Snapshot:   NewSnapshotHandler(services.Snapshot, logger),
			PlanMember: NewPlanMemberHandler(repos.PlanMember, logger),
		},
		Finance: FinanceHandlers{
			Product: NewProductHandler(services.Product, logger),
			Staff:   NewStaffHandler(services.Staff, logger),
			Capex:   NewCapexHandler(services.Capex, logger),
			Opex:    NewOpexHandler(services.Opex, logger),
			PnL:     NewPnLHandler(services.PnL, logger),
			FiPlan:  NewFiplanHandler(services.FiPlan, logger),
			PnlCash: NewPnlCashHandler(services.PnlCash, logger),
			BSheet:  NewBSheetHandler(services.BSheet, logger),
			Ratios:  NewRatiosHandler(services.Ratios, logger),
			WCR:     NewWCRHandler(services.WCR, logger),
			Cash:    NewCashHandler(services.Cash, logger),
			Budget:  NewBudgetHandler(services.Budget, logger),
			Report:  NewReportHandler(services.Report, logger),
		},
	}
}
