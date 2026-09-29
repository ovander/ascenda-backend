package handler

import (
	"ascenda/internal/config"
	"ascenda/internal/repo"
	"ascenda/internal/service"
	"github.com/ovander/backendkit/buildinfo"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// AdminHandlers groups system and user management handlers.
type AdminHandlers struct {
	Health             *HealthHandler
	Auth               *AuthHandler
	MagicLink          *MagicLinkHandler
	User               *UserHandler
	Tenant             *TenantHandler
	Metadata           *MetadataHandler
	AdminStats         *AdminStatsHandler
	AdminUser          *AdminUserHandler
	AdminOrg           *AdminOrgHandler
	AdminCountryConfig *AdminCountryConfigHandler
	FeaturePolicy      *FeaturePolicyHandler
	Metrics            *MetricsHandler
}

// PlanHandlers groups plan lifecycle handlers.
type PlanHandlers struct {
	Plan             *PlanHandler
	Scenario         *ScenarioHandler
	ScenarioAnalysis *ScenarioAnalysisHandler
	Settings         *SettingsHandler
	Snapshot         *SnapshotHandler
	PlanMember       *PlanMemberHandler
	Audit            *AuditHandler
}

// FinanceHandlers groups financial data and report handlers.
type FinanceHandlers struct {
	Product      *ProductHandler
	Staff        *StaffHandler
	Capex        *CapexHandler
	Opex         *OpexHandler
	PnL          *PnLHandler
	FiPlan       *FiplanHandler
	PnlCash      *PnlCashHandler
	BSheet       *BSheetHandler
	Ratios       *RatiosHandler
	WCR          *WCRHandler
	Cash         *CashHandler
	Budget       *BudgetHandler
	Graph        *GraphHandler
	Report       *ReportHandler
	CapTable     *CapTableHandler
	PlanCapTable *PlanCapTableHandler
	BEP          *BEPHandler
}

// AIHandlers groups AI-powered narration handlers.
type AIHandlers struct {
	AI *AIHandler
}

// HandlerBundle contains all handler instances grouped by domain.
type HandlerBundle struct {
	Admin   AdminHandlers
	Plans   PlanHandlers
	Finance FinanceHandlers
	AI      AIHandlers
}

// NewHandlerBundle creates a new HandlerBundle with all handlers initialized.
func NewHandlerBundle(services *service.ServiceBundle, repos *repo.RepoBundle, db *gorm.DB, cfg *config.Config, logger *logrus.Entry) *HandlerBundle {
	return &HandlerBundle{
		Admin: AdminHandlers{
			Health:             NewHealthHandler(db, logger),
			Auth:               NewAuthHandler(cfg, services.Registration, repos.User, logger),
			MagicLink:          NewMagicLinkHandler(services.MagicLink, repos.User, logger),
			User:               NewUserHandler(services.User, logger),
			Tenant:             NewTenantHandler(services.Tenant, logger),
			Metadata:           NewMetadataHandler(buildinfo.Version, buildinfo.BuildTime, buildinfo.GitCommit, logger),
			AdminStats:         NewAdminStatsHandler(services.Admin, logger),
			AdminUser:          NewAdminUserHandler(services.AdminUser, logger),
			AdminOrg:           NewAdminOrgHandler(services.Organization, logger),
			AdminCountryConfig: NewAdminCountryConfigHandler(services.CountryRateConfig, logger),
			FeaturePolicy:      NewFeaturePolicyHandler(services.FeaturePolicy, logger),
			Metrics:            NewMetricsHandler(),
		},
		Plans: PlanHandlers{
			Plan:             NewPlanHandler(services.Plan, services.Seed, logger),
			Scenario:         NewScenarioHandler(services.Plan, logger),
			ScenarioAnalysis: NewScenarioAnalysisHandler(services.ScenarioAnalysis, logger),
			Settings:         NewSettingsHandler(services.Settings, logger),
			Snapshot:         NewSnapshotHandler(services.Snapshot, logger),
			PlanMember:       NewPlanMemberHandler(repos.PlanMember, logger),
			Audit:            NewAuditHandler(repos.Audit, services.Snapshot, service.NewPlanAccessResolver(repos.Plan, repos.PlanMember, repos.Scenario), repos.Scenario, logger),
		},
		Finance: FinanceHandlers{
			Product:      NewProductHandler(services.Product, logger),
			Staff:        NewStaffHandler(services.Staff, logger),
			Capex:        NewCapexHandler(services.Capex, logger),
			Opex:         NewOpexHandler(services.Opex, logger),
			PnL:          NewPnLHandler(services.PnL, logger),
			FiPlan:       NewFiplanHandler(services.FiPlan, services.CapTable, logger),
			PnlCash:      NewPnlCashHandler(services.PnlCash, logger),
			BSheet:       NewBSheetHandler(services.BSheet, logger),
			Ratios:       NewRatiosHandler(services.Ratios, logger),
			WCR:          NewWCRHandler(services.WCR, logger),
			Cash:         NewCashHandler(services.Cash, logger),
			Budget:       NewBudgetHandler(services.Budget, logger),
			Graph:        NewGraphHandler(services.Graph, logger),
			Report:       NewReportHandler(services.Report, logger),
			CapTable:     NewCapTableHandler(services.CapTable, logger),
			PlanCapTable: NewPlanCapTableHandler(repos.PlanShareholder, logger),
			BEP:          NewBEPHandler(services.BEP, logger),
		},
		AI: AIHandlers{
			AI: NewAIHandler(
				services.AINarration,
				service.NewSensitivityEngine(services.PlanOrchestrator, logger),
				logger,
			),
		},
	}
}
