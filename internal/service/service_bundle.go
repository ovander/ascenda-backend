package service

import (
	"github.com/sirupsen/logrus"
	"kerplan/internal/config"
	"kerplan/internal/event"
	"kerplan/internal/repo"
)

// ServiceBundle contains all service instances.
type ServiceBundle struct {
	Plan      *PlanService
	Settings  *SettingsService
	Product   *ProductService
	Staff     *StaffService
	Capex     *CapexService
	Opex      *OpexService
	PnL       *PnLService
	FiPlan    *FiplanService
	PnlCash   *PnlCashService
	BSheet    *BSheetService
	Ratios    *RatiosService
	WCR       *WCRService
	Cash      *CashService
	Budget    *BudgetService
	Report    *ReportService
	Snapshot  *SnapshotService
	User      *UserService

	// Emitter is the in-process event bus shared by all services.
	Emitter *event.Emitter

	// ReportInvalidator tracks stale report caches.
	ReportInvalidator *event.ReportInvalidator
}

// NewServiceBundle creates a new ServiceBundle with all services initialized.
func NewServiceBundle(repos *repo.RepoBundle, cfg *config.Config, logger *logrus.Entry) *ServiceBundle {
	// Create event emitter and subscribers
	emitter := event.NewEmitter(logger)

	// Audit subscriber: writes every event to the audit_logs table
	auditSub := event.NewAuditSubscriber(repos.Audit, logger)
	emitter.SubscribeAsync(auditSub)

	// Report invalidator: marks cached reports stale on data mutations
	invalidator, invalidatorSub := event.NewReportInvalidator(logger)
	emitter.Subscribe(invalidatorSub)

	// Create ReportService first, as other services depend on it
	reportService := NewReportService(repos, logger)

	// Wire report cache (LRU, max 50 scenarios) using the invalidator
	reportCache := NewReportCache(50, invalidator)
	reportService.SetCache(reportCache)

	return &ServiceBundle{
		Report:            reportService,
		Plan:              NewPlanService(repos.Plan, repos.Settings, repos.Audit, repos, emitter, logger),
		Settings:          NewSettingsService(repos.Settings, emitter, logger),
		Product:           NewProductService(repos.Product, reportService, emitter, logger),
		Staff:             NewStaffService(repos.Staff, reportService, emitter, logger),
		Capex:             NewCapexService(repos.Capex, reportService, emitter, logger),
		Opex:              NewOpexService(repos.Opex, reportService, emitter, logger),
		PnL:               NewPnLService(repos.PnL, reportService, logger),
		FiPlan:            NewFiplanService(repos.FiPlan, reportService, logger),
		PnlCash:           NewPnlCashService(repos.PnlCash, reportService, logger),
		BSheet:            NewBSheetService(reportService, logger),
		Ratios:            NewRatiosService(reportService, logger),
		WCR:               NewWCRService(repos.WCR, reportService, logger),
		Cash:              NewCashService(repos.Cash, reportService, logger),
		Budget:            NewBudgetService(repos.Budget, reportService, logger),
		Snapshot:          NewSnapshotService(repos.Snapshot, repos, emitter, logger),
		User:              NewUserService(repos.User, repos.Tenant, emitter, logger),
		Emitter:           emitter,
		ReportInvalidator: invalidator,
	}
}
