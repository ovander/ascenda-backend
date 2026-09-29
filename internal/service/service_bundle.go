package service

import (
	"ascenda/internal/config"
	"ascenda/internal/event"
	"ascenda/internal/repo"
	"github.com/ovander/backendkit/socrate"
	"github.com/sirupsen/logrus"
)

// ServiceBundle contains all service instances.
type ServiceBundle struct {
	Admin        *AdminService
	AdminUser    *AdminUserService
	Organization *OrganizationService
	Registration *RegistrationService
	MagicLink    *MagicLinkService
	Token        *TokenService
	Tenant       *TenantService
	Plan         *PlanService
	Settings     *SettingsService
	Product      *ProductService
	Staff        *StaffService
	Capex        *CapexService
	Opex         *OpexService
	PnL          *PnLService
	FiPlan       *FiplanService
	PnlCash      *PnlCashService
	BSheet       *BSheetService
	Ratios       *RatiosService
	WCR          *WCRService
	Cash         *CashService
	Budget       *BudgetService
	Graph        *GraphService
	Report       *ReportService
	Snapshot     *SnapshotService
	User         *UserService
	Seed         *SeedService

	// AI services
	AIUsagePolicy    *AIUsagePolicyService
	AINarration      *AINarrationService
	PlanOrchestrator *PlanComputeOrchestrator

	// Cap Table module (Enterprise tier)
	CapTable *CapTableService

	// ScenarioAnalysis — decision-intelligence layer (viability, risks, drivers)
	ScenarioAnalysis *ScenarioAnalysisService

	// BEP module (Pro tier and above)
	BEP *BEPService

	// Platform-wide country rate configs (admin-editable)
	CountryRateConfig *CountryRateConfigService

	// Feature policies — tier-to-feature access rules (admin-editable, cached)
	FeaturePolicy *FeaturePolicyService

	// Emitter is the in-process event bus shared by all services.
	Emitter *event.Emitter

	// ReportInvalidator tracks stale report caches.
	ReportInvalidator *event.ReportInvalidator

	// SocrateClient is the raw Socrate API client. Exposed so that the
	// composition root (bootstrap.go) can wire it into TenantMiddleware for
	// OIDC userinfo enrichment without building a second client instance.
	// Nil when Socrate is not configured.
	SocrateClient *socrate.Client
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

	// Create the shared compute orchestrator — single owner of FullPlanInput
	// assembly logic.  Both ReportService and ScenarioAnalysisService use it.
	orchestrator := NewPlanComputeOrchestrator(repos, logger)

	// Create ReportService (wraps orchestrator with an LRU cache)
	reportService := NewReportService(orchestrator, logger)

	// Wire report cache (LRU, max 50 scenarios) using the invalidator
	reportCache := NewReportCache(50, invalidator)
	reportService.SetCache(reportCache)

	// Build optional Socrate client (gracefully skipped if not configured).
	// SocrateUserManager is the interface; *socrate.Client satisfies it.
	// The same *socrate.Client also satisfies SocrateRegistrar for self-service registration.
	var socrateClient SocrateUserManager
	var socrateRegistrar SocrateRegistrar
	var socrateInviter SocrateInviter
	var socrateMagicLink SocrateMagicLink
	var socrateTokens SocrateTokens
	var socrateProfiler SocrateProfileFetcher
	var rawSocrateClient *socrate.Client // exported on bundle for TenantMiddleware wiring
	if cfg.Socrate.BaseURL != "" && cfg.Socrate.ClientID != "" {
		if sc, err := socrate.NewClient(socrate.ClientConfig{
			BaseURL:      cfg.Socrate.BaseURL,
			AdminBaseURL: cfg.Socrate.AdminBaseURL,
			ClientID:     cfg.Socrate.ClientID,
			ClientSecret: cfg.Socrate.ClientSecret,
			AppID:        cfg.Socrate.AppID,
		}); err == nil {
			socrateClient = sc
			socrateRegistrar = sc
			socrateInviter = sc
			socrateMagicLink = sc
			socrateTokens = sc
			socrateProfiler = sc
			rawSocrateClient = sc
		} else {
			logger.WithError(err).Warn("Socrate client could not be initialised — admin user management will be unavailable")
		}
	} else {
		logger.Debug("Socrate client disabled (SOCRATE_BASE_URL or SOCRATE_CLIENT_ID not set)")
	}

	// Build FiplanService first so it can be injected into CapTableService
	// for the one-way cap-table → FiPlan sync feature.
	fiplanSvc := NewFiplanService(repos.FiPlan, reportService, emitter, logger)

	// CountryRateConfigService — seed hard-coded defaults into DB on first boot.
	countryRateSvc := NewCountryRateConfigService(repos.CountryRateConfig, logger)
	if err := countryRateSvc.SeedDefaults(); err != nil {
		logger.WithError(err).Warn("country rate config seed failed — falling back to hard-coded defaults")
	}

	// FeaturePolicyService — seed baseline tier rules on first boot.
	featurePolicySvc := NewFeaturePolicyService(repos.FeaturePolicy, logger)
	if err := featurePolicySvc.SeedDefaults(); err != nil {
		logger.WithError(err).Warn("feature policy seed failed — limits will fall back to deny-all")
	}

	// SeedService is needed by RegistrationService; build it early.
	seedSvc := NewSeedService(repos, countryRateSvc, logger)

	bundle := &ServiceBundle{
		Admin:        NewAdminService(repos.AdminStats, socrateClient, logger),
		AdminUser:    NewAdminUserService(socrateClient, repos.User, repos.Tenant, logger),
		Organization: NewOrganizationService(repos.Org, repos.Tenant, repos.User, socrateInviter, logger),
		Registration: NewRegistrationService(socrateRegistrar, repos.User, repos.Tenant, seedSvc, logger),
		MagicLink:    NewMagicLinkService(socrateMagicLink, logger),
		Token:        NewTokenService(socrateTokens, logger),
		Tenant:       NewTenantService(repos.Tenant, logger),
		Report:       reportService,
		Plan:         NewPlanService(repos.Plan, repos.Settings, repos.Audit, repos, countryRateSvc, emitter, logger).WithFeaturePolicyService(featurePolicySvc),
		Settings:     NewSettingsService(repos.Settings, emitter, logger),
		Product:      NewProductService(repos.Product, reportService, emitter, logger),
		Staff:        NewStaffService(repos.Staff, reportService, emitter, logger),
		Capex:        NewCapexService(repos.Capex, reportService, emitter, logger),
		Opex:         NewOpexService(repos.Opex, reportService, emitter, logger),
		PnL:          NewPnLService(repos.PnL, reportService, emitter, logger),
		FiPlan:       fiplanSvc,
		PnlCash:      NewPnlCashService(repos.PnlCash, reportService, emitter, logger),
		BSheet:       NewBSheetService(reportService, logger),
		Ratios:       NewRatiosService(reportService, logger),
		WCR:          NewWCRService(repos.WCR, reportService, emitter, logger),
		Cash:         NewCashService(repos.Cash, reportService, emitter, logger),
		Budget:       NewBudgetService(repos.Budget, reportService, emitter, logger),
		Graph:        NewGraphService(reportService, logger),
		Snapshot:     NewSnapshotService(repos.Snapshot, repos, emitter, logger),
		User:         NewUserService(repos.User, repos.Tenant, socrateInviter, socrateProfiler, emitter, logger),
		Seed:         seedSvc,
		// AI services
		AIUsagePolicy:    NewAIUsagePolicyService(repos.AIUsagePolicy, repos.AIUsageRecord, logger),
		PlanOrchestrator: orchestrator,
		AINarration: NewAINarrationServiceWithCache(cfg.AI, logger,
			NewLayeredNarrationCache(DefaultNarrationCacheConfig(), repos.AINarrationCache),
		),
		Emitter:           emitter,
		ReportInvalidator: invalidator,
		SocrateClient:     rawSocrateClient,

		// Cap Table module (Enterprise tier) — fiplanSvc injected for round→FiPlan sync
		CapTable: NewCapTableService(repos.CapTable, repos.Settings, fiplanSvc, emitter, logger),

		// ScenarioAnalysis — decision-intelligence layer
		ScenarioAnalysis: NewScenarioAnalysisService(reportService, logger),

		// BEP module (Pro tier and above)
		BEP: NewBEPService(repos.BEP, reportService, emitter, logger),

		// Platform-wide country rate configs (admin-editable)
		CountryRateConfig: countryRateSvc,

		// Feature policies (admin-editable, in-process cached)
		FeaturePolicy: featurePolicySvc,
	}

	logger.WithFields(logrus.Fields{
		"event_bus":      "emitter + audit subscriber + report invalidator",
		"report_cache":   "LRU (max 50 scenarios)",
		"socrate_client": socrateClient != nil,
	}).Info("service layer initialised")

	return bundle
}
