package router

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/sirupsen/logrus"
	"ascenda/internal/handler"
	"ascenda/internal/middleware"
	"ascenda/internal/model"
)

// NewRouter creates and configures the main router with all routes and middleware.
// allowedOrigins is sourced from cfg.AllowedOrigins (env: CORS_ORIGINS) so that
// production deployments never rely on hardcoded localhost values.
// maxBodyBytes caps all incoming request bodies (env: MAX_REQUEST_BODY_BYTES).
// metricsEnabled controls whether GET /metrics (Prometheus) is registered.
func NewRouter(
	handlers *handler.HandlerBundle,
	authMW *middleware.AuthMiddleware,
	tenantMW *middleware.TenantMiddleware,
	rbacMW *middleware.RBACMiddleware,
	planAccessMW *middleware.PlanAccessMiddleware,
	tierGateMW *middleware.TierGateMiddleware,
	aiAccessMW *middleware.AIAccessMiddleware,
	loggerMW *middleware.LoggerMiddleware,
	recoverMW *middleware.RecoverMiddleware,
	requestIDMW *middleware.RequestIDMiddleware,
	securityMW *middleware.SecurityHeadersMiddleware,
	logger *logrus.Entry,
	allowedOrigins []string,
	maxBodyBytes int64,
	metricsEnabled bool,
) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware (applied to all routes)
	r.Use(middleware.CORSMiddleware(allowedOrigins))
	r.Use(middleware.BodyLimitMiddleware(maxBodyBytes))
	r.Use(chimw.Compress(5)) // gzip compression (level 5 = balanced)
	r.Use(securityMW.Handler)
	r.Use(recoverMW.Handler)
	r.Use(requestIDMW.Handler)
	r.Use(loggerMW.Handler)

	// Default CRUD timeout
	crudTimeout := middleware.TimeoutMiddleware(5 * time.Second)
	// Report timeout (longer for heavy computation)
	reportTimeout := middleware.TimeoutMiddleware(30 * time.Second)
	// Snapshot timeout (longest — captures/restores entire scenario)
	snapshotTimeout := middleware.TimeoutMiddleware(60 * time.Second)
	// AI timeout — LLM inference can take 30–90 s; give ample headroom.
	// This value (120 s) MUST remain less than the HTTP server's WriteTimeout
	// (150 s defined in cmd/server/bootstrap.go) so that this context-based
	// timeout always fires first and can write a proper JSON error response.
	// Raising this value above WriteTimeout would cause ERR_EMPTY_RESPONSE.
	aiTimeout := middleware.TimeoutMiddleware(120 * time.Second)

	// Rate limiters
	generalLimiter := middleware.NewRateLimiter(100, 20) // 100 req/s, burst 20
	reportLimiter := middleware.NewRateLimiter(10, 5)    // 10 req/s, burst 5

	// Health check routes (no auth required)
	r.Route("/health", func(r chi.Router) {
		r.Get("/", handlers.Admin.Health.Check)
	})

	r.Route("/ready", func(r chi.Router) {
		r.Get("/", handlers.Admin.Health.Ready)
	})

	// Prometheus metrics — only registered when METRICS_ENABLED=true.
	// Restrict this endpoint to an internal network interface in production
	// (e.g. via Caddy's bind directive or a VPS firewall rule) to avoid
	// leaking internal counters to the public internet.
	if metricsEnabled {
		r.Get("/metrics", handlers.Admin.Metrics.ServeHTTP)
		logger.WithField("phase", "router").Info("Prometheus /metrics endpoint enabled")
	}

	// Auth routes (no tenant context required)
	r.Route("/auth", func(r chi.Router) {
		// Self-service registration — unauthenticated, rate-limited.
		r.With(middleware.NewRateLimiter(5, 2).Handler).Post("/register", handlers.Admin.Auth.Register)
		r.Post("/login", handlers.Admin.Auth.Login)
		r.Post("/callback", handlers.Admin.Auth.Callback)
		r.Post("/refresh", handlers.Admin.Auth.Refresh)
		r.Post("/logout", handlers.Admin.Auth.Logout)

		// Passwordless magic-link sign-in.
		// POST /auth/magic-link   — request a sign-in link (always 202, anti-enumeration).
		// GET  /auth/magic-link/verify — browser follows link from email; redirects to Socrate PKCE flow.
		r.With(middleware.NewRateLimiter(5, 2).Handler).Post("/magic-link", handlers.Admin.MagicLink.Send)
		r.Get("/magic-link/verify", handlers.Admin.MagicLink.Verify)
	})

	// Version endpoint (no auth required — useful for deploy checks)
	r.Get("/api/v1/version", handlers.Admin.Metadata.GetVersion)

	// API routes that require auth but NOT tenant context
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(authMW.Handler)
		r.Use(generalLimiter.Handler)
		r.Use(crudTimeout)

		// ── Platform-admin routes — no tenant context required ────────────────
		// Only accessible to users with the platform:admin permission (role=admin).
		r.Route("/admin", func(r chi.Router) {
			r.Use(rbacMW.RequirePermission(middleware.PermPlatformAdmin))

			// Aggregate stats
			r.Get("/stats", handlers.Admin.AdminStats.GetStats)
			r.Get("/ai-usage", handlers.Admin.AdminStats.GetAIUsage)

			// Platform-wide user management (via Socrate proxy)
			r.Route("/users", func(r chi.Router) {
				r.Get("/", handlers.Admin.AdminUser.ListUsers)
				r.Post("/", handlers.Admin.AdminUser.CreateUser)
				r.Route("/{userId}", func(r chi.Router) {
					r.Get("/", handlers.Admin.AdminUser.GetUser)
					r.Put("/", handlers.Admin.AdminUser.UpdateUser)
					r.Delete("/", handlers.Admin.AdminUser.DeleteUser)
					r.Post("/resend-verification", handlers.Admin.AdminUser.ResendVerification)
					r.Post("/reset-password", handlers.Admin.AdminUser.ResetPassword)
				})
			})

			// Tenant management
			r.Route("/tenants", func(r chi.Router) {
				r.Get("/", handlers.Admin.AdminUser.ListTenants)
				r.Post("/", handlers.Admin.AdminUser.CreateTenant)
				r.Route("/{tenantId}", func(r chi.Router) {
					r.Get("/", handlers.Admin.AdminUser.GetTenant)
					r.Put("/", handlers.Admin.AdminUser.UpdateTenant)
				})
			})

			// Enterprise organization management
			// Each org is the billing umbrella for N department tenants.
			r.Route("/organizations", func(r chi.Router) {
				r.Get("/", handlers.Admin.AdminOrg.ListOrganizations)
				r.Post("/", handlers.Admin.AdminOrg.CreateOrganization)
				r.Route("/{orgId}", func(r chi.Router) {
					r.Get("/", handlers.Admin.AdminOrg.GetOrganization)
					r.Put("/", handlers.Admin.AdminOrg.UpdateOrganization)
					r.Delete("/", handlers.Admin.AdminOrg.DeleteOrganization)
					// Department tenants within the org
					r.Get("/tenants", handlers.Admin.AdminOrg.ListOrgTenants)
					r.Post("/tenants", handlers.Admin.AdminOrg.AddOrgTenant)
				})
			})

			// Country rate config management
			r.Route("/country-configs", func(r chi.Router) {
				r.Get("/", handlers.Admin.AdminCountryConfig.List)
				r.Post("/", handlers.Admin.AdminCountryConfig.Create)
				r.Route("/{code}", func(r chi.Router) {
					r.Get("/", handlers.Admin.AdminCountryConfig.Get)
					r.Put("/", handlers.Admin.AdminCountryConfig.Update)
					r.Post("/reset", handlers.Admin.AdminCountryConfig.Reset)
				})
			})

			// Feature policy management — move features between tiers without redeploy
			r.Route("/feature-policies", func(r chi.Router) {
				r.Put("/{feature}", handlers.Admin.FeaturePolicy.Update)
			})
		})

		// All data routes require tenant context (resolves Ascenda role from DB).
		// Note: the tenant middleware has a platform-admin fast-path — if the JWT
		// role is "admin" it skips DB provisioning and preserves the role, so
		// platform admins can safely call /users/me through this group too.
		r.Group(func(r chi.Router) {
			r.Use(tenantMW.Handler)

			// User profile — works for both business users (DB role) and platform admins (JWT role).
			r.Get("/users/me", handlers.Admin.User.GetMe)
			r.Put("/users/me", handlers.Admin.User.UpdateMe)

			// Metadata (enum lists for frontend)
			r.Get("/metadata", handlers.Admin.Metadata.GetMetadata)

			// Feature policies — public read so every client can evaluate tier limits
			r.Get("/feature-policies", handlers.Admin.FeaturePolicy.List)

			// User management routes (require manage:users permission)
			r.Route("/users", func(r chi.Router) {
				r.Use(rbacMW.RequirePermission(middleware.PermManageUsers))
				r.Get("/", handlers.Admin.User.List)
				r.Post("/invite", handlers.Admin.User.Invite)
				r.Get("/{userId}/impact", handlers.Admin.User.GetImpact)
				r.Delete("/{userId}", handlers.Admin.User.Delete)
				r.Put("/{userId}/role", handlers.Admin.User.UpdateRole)
				r.Post("/{userId}/deactivate", handlers.Admin.User.Deactivate)
				r.Post("/{userId}/reactivate", handlers.Admin.User.Reactivate)
			})

			// Tenant management routes (require manage:tenant permission)
			r.Route("/tenant", func(r chi.Router) {
				r.Use(rbacMW.RequirePermission(middleware.PermManageTenant))
				r.Get("/", handlers.Admin.Tenant.Get)
				r.Put("/", handlers.Admin.Tenant.Update)
			})

		// Audit trail — tenant-scoped, all authenticated users can read.
		r.Route("/audit", func(r chi.Router) {
			r.Get("/", handlers.Plans.Audit.List)
			r.Post("/export", handlers.Plans.Audit.RecordExport)
			r.With(snapshotTimeout).Get("/{entryId}/detail", handlers.Plans.Audit.GetDetail)
		})

		// Plan routes — access is checked per-plan by PlanAccessMiddleware.
		// Admin role is blocked by PlanAccessMiddleware on individual plan routes.
		// List endpoint returns only plans the user has access to.
		r.Route("/plans", func(r chi.Router) {
			r.Get("/", handlers.Plans.Plan.List)
			r.With(rbacMW.RequirePermission(middleware.PermManagePlan)).Post("/", handlers.Plans.Plan.Create)
			r.With(rbacMW.RequirePermission(middleware.PermManagePlan)).Post("/reset-demo", handlers.Plans.Plan.ResetDemo)

			// All routes under /{planId} require plan access check
			r.Route("/{planId}", func(r chi.Router) {
				r.Use(planAccessMW.RequirePlanAccess)
				r.Get("/", handlers.Plans.Plan.Get)
				r.With(planAccessMW.RequirePlanEdit).Put("/", handlers.Plans.Plan.Update)
				r.With(rbacMW.RequirePermission(middleware.PermManagePlan)).Delete("/", handlers.Plans.Plan.Delete)
				// Lifecycle transitions — require PermManagePlan (owner/admin)
				r.Get("/impact", handlers.Plans.Plan.GetImpact)
				r.With(rbacMW.RequirePermission(middleware.PermManagePlan)).Post("/lock", handlers.Plans.Plan.Lock)
				r.With(rbacMW.RequirePermission(middleware.PermManagePlan)).Post("/unlock", handlers.Plans.Plan.Unlock)
				r.With(rbacMW.RequirePermission(middleware.PermManagePlan)).Post("/archive", handlers.Plans.Plan.Archive)

				// Plan members (owner only — manages who has access)
				r.Route("/members", func(r chi.Router) {
					r.Get("/", handlers.Plans.PlanMember.List)
					r.With(rbacMW.RequirePermission(middleware.PermManagePlan)).Post("/", handlers.Plans.PlanMember.Grant)
					r.With(rbacMW.RequirePermission(middleware.PermManagePlan)).Put("/{userId}", handlers.Plans.PlanMember.UpdateRole)
					r.With(rbacMW.RequirePermission(middleware.PermManagePlan)).Delete("/{userId}", handlers.Plans.PlanMember.Revoke)
				})

				// Plan-level cap table (Pro tier) — scoped to plan, no scenarioId required.
				r.Route("/cap-table", func(r chi.Router) {
					r.Use(tierGateMW.Require(middleware.TierPro))
					r.Get("/", handlers.Finance.PlanCapTable.GetSummary)
					r.With(planAccessMW.RequirePlanEdit).Post("/shareholders", handlers.Finance.PlanCapTable.CreateShareholder)
					r.With(planAccessMW.RequirePlanEdit).Put("/shareholders/{id}", handlers.Finance.PlanCapTable.UpdateShareholder)
					r.With(planAccessMW.RequirePlanEdit).Delete("/shareholders/{id}", handlers.Finance.PlanCapTable.DeleteShareholder)
				})

				// Scenario routes (nested under plan)
				r.Route("/scenarios", func(r chi.Router) {
					r.Get("/", handlers.Plans.Scenario.List)
					r.With(planAccessMW.RequirePlanEdit).Post("/", handlers.Plans.Scenario.Create)

					// All routes under /{scenarioId} in a single subrouter
					r.Route("/{scenarioId}", func(r chi.Router) {
						r.Get("/", handlers.Plans.Scenario.Get)
						r.With(planAccessMW.RequirePlanEdit).Put("/", handlers.Plans.Scenario.Update)
						r.With(planAccessMW.RequirePlanEdit).Delete("/", handlers.Plans.Scenario.Delete)
						r.With(planAccessMW.RequirePlanEdit).Post("/clone", handlers.Plans.Scenario.Clone)
						r.Get("/impact", handlers.Plans.Plan.GetScenarioImpact)

						// Scenario analysis — decision-intelligence layer (viability, risks, drivers)
						r.Group(func(r chi.Router) {
							r.Use(reportTimeout)
							r.Use(reportLimiter.Handler)
							r.Get("/analysis", handlers.Plans.ScenarioAnalysis.Analyze)
						})

						// Dev audit download — full audit trail for this scenario as JSON attachment.
						r.Get("/audit/download", handlers.Plans.Audit.DownloadByScenario)

						// Settings
						r.Route("/settings", func(r chi.Router) {
							r.Get("/config", handlers.Plans.Settings.GetConfig)
							r.With(planAccessMW.RequirePlanEdit).Put("/config", handlers.Plans.Settings.UpdateConfig)
							r.Get("/opening-balance", handlers.Plans.Settings.GetOpeningBalance)
							r.With(planAccessMW.RequirePlanEdit).Put("/opening-balance", handlers.Plans.Settings.UpdateOpeningBalance)
							r.Get("/wc-config", handlers.Plans.Settings.GetWCConfig)
							r.With(planAccessMW.RequirePlanEdit).Put("/wc-config", handlers.Plans.Settings.UpdateWCConfig)
							r.Get("/opex-per-hire", handlers.Plans.Settings.GetOpexPerHire)
							r.With(planAccessMW.RequirePlanEdit).Put("/opex-per-hire", handlers.Plans.Settings.UpdateOpexPerHire)
							r.Get("/capex-per-hire", handlers.Plans.Settings.GetCapexPerHire)
							r.With(planAccessMW.RequirePlanEdit).Put("/capex-per-hire", handlers.Plans.Settings.UpdateCapexPerHire)
						})

						// Products
						r.Route("/products", func(r chi.Router) {
							r.Get("/", handlers.Finance.Product.List)
							r.With(planAccessMW.RequirePlanEdit).Post("/", handlers.Finance.Product.Create)
							r.Get("/{productId}", handlers.Finance.Product.Get)
							r.With(planAccessMW.RequirePlanEdit).Put("/{productId}", handlers.Finance.Product.Update)
							r.With(planAccessMW.RequirePlanEdit).Delete("/{productId}", handlers.Finance.Product.Delete)
							r.Get("/{productId}/assumptions", handlers.Finance.Product.GetAssumptions)
							r.With(planAccessMW.RequirePlanEdit).Put("/{productId}/assumptions", handlers.Finance.Product.UpdateAssumptions)
							r.Get("/{productId}/volumes", handlers.Finance.Product.GetVolumes)
							r.With(planAccessMW.RequirePlanEdit).Put("/{productId}/volumes", handlers.Finance.Product.UpdateVolumes)
							r.Get("/{productId}/margins", handlers.Finance.Product.GetMargins)
							r.With(planAccessMW.RequirePlanEdit).Put("/{productId}/margins", handlers.Finance.Product.UpdateMargins)

							// Business Driver Framework: derived volumes + assumptions for typed drivers.
							r.Get("/{productId}/driver/bundle", handlers.Finance.Product.GetDerivedBundle)

							// Revenue endpoints use report timeout + report rate limit
							r.Group(func(r chi.Router) {
								r.Use(reportTimeout)
								r.Use(reportLimiter.Handler)
								r.Get("/{productId}/revenue", handlers.Finance.Product.GetRevenueByProduct)
								r.Get("/revenue/consolidated", handlers.Finance.Product.GetConsolidatedRevenue)
							})
						})

						// Staff
						r.Route("/staff", func(r chi.Router) {
							r.Get("/headcounts", handlers.Finance.Staff.ListHeadcounts)
							r.With(planAccessMW.RequirePlanEdit).Put("/headcounts", handlers.Finance.Staff.UpdateHeadcounts)
							r.Get("/salaries", handlers.Finance.Staff.ListSalaries)
							r.With(planAccessMW.RequirePlanEdit).Put("/salaries", handlers.Finance.Staff.UpdateSalaries)
							r.Get("/incentives", handlers.Finance.Staff.ListIncentives)
							r.With(planAccessMW.RequirePlanEdit).Put("/incentives", handlers.Finance.Staff.UpdateIncentives)

							// Summary uses report timeout
							r.Group(func(r chi.Router) {
								r.Use(reportTimeout)
								r.Use(reportLimiter.Handler)
								r.Get("/summary", handlers.Finance.Staff.GetPayrollSummary)
							})
						})

						// Capex
						r.Route("/capex", func(r chi.Router) {
							r.Get("/", handlers.Finance.Capex.ListEntries)
							r.With(planAccessMW.RequirePlanEdit).Put("/", handlers.Finance.Capex.UpdateEntries)
							r.Group(func(r chi.Router) {
								r.Use(reportTimeout)
								r.Use(reportLimiter.Handler)
								r.Get("/summary", handlers.Finance.Capex.GetSummary)
							})
						})

						// Opex
						r.Route("/opex", func(r chi.Router) {
							r.Get("/", handlers.Finance.Opex.ListEntries)
							r.With(planAccessMW.RequirePlanEdit).Put("/", handlers.Finance.Opex.UpdateEntries)
							r.Group(func(r chi.Router) {
								r.Use(reportTimeout)
								r.Use(reportLimiter.Handler)
								r.Get("/summary", handlers.Finance.Opex.GetSummary)
							})
						})

						// P&L
						r.Route("/pnl", func(r chi.Router) {
							r.Get("/", handlers.Finance.PnL.ListManualEntries)
							r.With(planAccessMW.RequirePlanEdit).Put("/", handlers.Finance.PnL.UpdateManualEntries)
							r.Group(func(r chi.Router) {
								r.Use(reportTimeout)
								r.Use(reportLimiter.Handler)
								r.Get("/report", handlers.Finance.PnL.GetReport)
								r.Get("/chart", handlers.Finance.PnL.GetChartData)
							})
						})

						// Financial Plan
						r.Route("/fiplan", func(r chi.Router) {
							r.Get("/", handlers.Finance.FiPlan.ListEntries)
							r.With(planAccessMW.RequirePlanEdit).Put("/", handlers.Finance.FiPlan.UpdateEntries)
							r.Group(func(r chi.Router) {
								r.Use(reportTimeout)
								r.Use(reportLimiter.Handler)
								r.Get("/report", handlers.Finance.FiPlan.GetReport)
								r.Get("/grants", handlers.Finance.FiPlan.GetGrantsForPnL)
							})
							// Flow A: FiPlan → Cap Table (Pro tier) — create a round from a planned capital increase.
							r.With(tierGateMW.Require(middleware.TierPro)).
								With(planAccessMW.RequirePlanEdit).
								Post("/capital-increase/{yearIndex}/create-round", handlers.Finance.FiPlan.CreateRoundFromCapitalIncrease)
						})

						// P&L + Cash
						r.Route("/pnl-cash", func(r chi.Router) {
							r.Get("/", handlers.Finance.PnlCash.ListEntries)
							r.With(planAccessMW.RequirePlanEdit).Put("/", handlers.Finance.PnlCash.UpdateEntries)
							r.Group(func(r chi.Router) {
								r.Use(reportTimeout)
								r.Use(reportLimiter.Handler)
								r.Get("/report", handlers.Finance.PnlCash.GetReport)
								r.Get("/chart", handlers.Finance.PnlCash.GetChartData)
							})
						})

					// Balance Sheet (report only)
					r.Route("/bsheet", func(r chi.Router) {
						r.Use(reportTimeout)
						r.Use(reportLimiter.Handler)
						r.Get("/report", handlers.Finance.BSheet.GetReport)
						r.Get("/chart", handlers.Finance.BSheet.GetChartData)
					})

					// Ratios (report only)
					r.Route("/ratios", func(r chi.Router) {
						r.Use(reportTimeout)
						r.Use(reportLimiter.Handler)
						r.Get("/report", handlers.Finance.Ratios.GetReport)
						r.Get("/chart", handlers.Finance.Ratios.GetChartData)
					})

					// Working Capital
						r.Route("/wcr", func(r chi.Router) {
							r.Get("/", handlers.Finance.WCR.ListEntries)
							r.With(planAccessMW.RequirePlanEdit).Put("/", handlers.Finance.WCR.UpdateEntries)
							r.Group(func(r chi.Router) {
								r.Use(reportTimeout)
								r.Use(reportLimiter.Handler)
								r.Get("/report", handlers.Finance.WCR.GetReport)
								r.Get("/chart", handlers.Finance.WCR.GetChartData)
							})
						})

						// Cash
						r.Route("/cash", func(r chi.Router) {
							r.Get("/", handlers.Finance.Cash.ListOverrides)
							r.With(planAccessMW.RequirePlanEdit).Put("/", handlers.Finance.Cash.UpdateOverrides)
							r.Group(func(r chi.Router) {
								r.Use(reportTimeout)
								r.Use(reportLimiter.Handler)
								r.Get("/report", handlers.Finance.Cash.GetReport)
							})
						})

						// Budget
						r.Route("/budget", func(r chi.Router) {
							r.Get("/", handlers.Finance.Budget.ListOverrides)
							r.With(planAccessMW.RequirePlanEdit).Put("/", handlers.Finance.Budget.UpdateOverrides)
							r.Group(func(r chi.Router) {
								r.Use(reportTimeout)
								r.Use(reportLimiter.Handler)
								r.Get("/year1", handlers.Finance.Budget.GetBudget1Report)
								r.Get("/year2", handlers.Finance.Budget.GetBudget2Report)
							})
						})

						// Graphs (chart data for dashboard)
						r.Route("/graphs", func(r chi.Router) {
							r.Use(reportTimeout)
							r.Use(reportLimiter.Handler)
							r.Get("/annual/all", handlers.Finance.Graph.GetAllAnnualCharts)
							r.Get("/annual", handlers.Finance.Graph.GetAnnualChart)
							r.Get("/monthly", handlers.Finance.Graph.GetMonthlyChart)
						})

					// Full Report
					r.Group(func(r chi.Router) {
						r.Use(reportTimeout)
						r.Use(reportLimiter.Handler)
						r.Get("/report", handlers.Finance.Report.GetFullReport)
					})

					// BEP module (Pro tier and above)
					r.Route("/bep", func(r chi.Router) {
						r.Use(tierGateMW.Require(middleware.TierPro))

						// Static reference data — no per-request overhead
						r.Get("/pcg-accounts", handlers.Finance.BEP.GetPCGAccounts)

						// Read-only plan preview (used to pre-populate the New Snapshot form)
						// and multi-year BEP report (derived from plan, no snapshot required)
						r.Group(func(r chi.Router) {
							r.Use(reportTimeout)
							r.Use(reportLimiter.Handler)
							r.Get("/plan-preview", handlers.Finance.BEP.PreviewFromPlan)
							r.Get("/multi-year-report", handlers.Finance.BEP.GetMultiYearBEPReport)
						})

						// Snapshots
						r.Get("/snapshots", handlers.Finance.BEP.ListSnapshots)
						r.With(planAccessMW.RequirePlanEdit).Post("/snapshots", handlers.Finance.BEP.CreateSnapshot)

						r.Route("/snapshots/{snapshotId}", func(r chi.Router) {
							r.Get("/", handlers.Finance.BEP.GetSnapshot)
							r.With(planAccessMW.RequirePlanEdit).Put("/", handlers.Finance.BEP.UpdateSnapshot)
							r.With(planAccessMW.RequirePlanEdit).Delete("/", handlers.Finance.BEP.DeleteSnapshot)
							r.With(planAccessMW.RequirePlanEdit).Post("/import-from-plan", handlers.Finance.BEP.ImportFromPlan)

							// BEP report (compute timeout + rate limiter)
							r.Group(func(r chi.Router) {
								r.Use(reportTimeout)
								r.Use(reportLimiter.Handler)
								r.Get("/report", handlers.Finance.BEP.GetBEPReport)
							})

							// Fixed and variable cost lines
							r.Get("/fixed-costs", handlers.Finance.BEP.ListFixedCostLines)
							r.With(planAccessMW.RequirePlanEdit).Put("/fixed-costs", handlers.Finance.BEP.UpsertFixedCostLines)
							r.Get("/variable-costs", handlers.Finance.BEP.ListVariableCostLines)
							r.With(planAccessMW.RequirePlanEdit).Put("/variable-costs", handlers.Finance.BEP.UpsertVariableCostLines)

							// Sensitivity configs
							r.Get("/sensitivity-config", handlers.Finance.BEP.ListSensitivityConfigs)
							r.With(planAccessMW.RequirePlanEdit).Put("/sensitivity-config/{analysisType}", handlers.Finance.BEP.UpsertSensitivityConfig)

							// Optimisation plans
							r.Get("/plans", handlers.Finance.BEP.ListOptimisationPlans)
							r.With(planAccessMW.RequirePlanEdit).Post("/plans", handlers.Finance.BEP.CreateOptimisationPlan)

							r.Route("/plans/{planId}", func(r chi.Router) {
								r.Get("/", handlers.Finance.BEP.GetOptimisationPlan)
								r.With(planAccessMW.RequirePlanEdit).Put("/", handlers.Finance.BEP.UpdateOptimisationPlan)
								r.With(planAccessMW.RequirePlanEdit).Delete("/", handlers.Finance.BEP.DeleteOptimisationPlan)

								// Optimised BEP report
								r.Group(func(r chi.Router) {
									r.Use(reportTimeout)
									r.Use(reportLimiter.Handler)
									r.Get("/report", handlers.Finance.BEP.GetOptimisedBEPReport)
								})

								// Savings
								r.Get("/savings/fixed", handlers.Finance.BEP.ListFixedCostSavings)
								r.With(planAccessMW.RequirePlanEdit).Put("/savings/fixed", handlers.Finance.BEP.UpsertFixedCostSavings)
								r.Get("/savings/variable", handlers.Finance.BEP.ListVariableCostSavings)
								r.With(planAccessMW.RequirePlanEdit).Put("/savings/variable", handlers.Finance.BEP.UpsertVariableCostSavings)

								// PCG review checklist
								r.Get("/pcg-review", handlers.Finance.BEP.ListPCGReviewItems)
								r.With(planAccessMW.RequirePlanEdit).Put("/pcg-review", handlers.Finance.BEP.UpsertPCGReviewItems)
							})
						})
					})

					// Cap Table module (Pro tier and above)
					r.Route("/cap-table", func(r chi.Router) {
						r.Use(tierGateMW.Require(middleware.TierPro))
						r.Use(reportTimeout)

						// Company & country profile
						r.Get("/company", handlers.Finance.CapTable.GetCapTableCompany)
						r.With(planAccessMW.RequirePlanEdit).Put("/company", handlers.Finance.CapTable.UpsertCapTableCompany)
						r.Get("/country-profile", handlers.Finance.CapTable.GetCapTableCountryProfile)

						// Share classes
						r.Get("/classes", handlers.Finance.CapTable.ListShareClasses)
						r.With(planAccessMW.RequirePlanEdit).Put("/classes", handlers.Finance.CapTable.UpsertShareClasses)

						// Shareholders
						r.Get("/shareholders", handlers.Finance.CapTable.ListShareholders)
						r.With(planAccessMW.RequirePlanEdit).Put("/shareholders", handlers.Finance.CapTable.UpsertShareholders)
						r.With(planAccessMW.RequirePlanEdit).Delete("/shareholders/{id}", handlers.Finance.CapTable.DeleteShareholder)

						// Funding rounds
						r.Get("/rounds", handlers.Finance.CapTable.ListCapTableRounds)
						r.With(planAccessMW.RequirePlanEdit).Put("/rounds", handlers.Finance.CapTable.UpsertCapTableRounds)
						r.With(planAccessMW.RequirePlanEdit).Delete("/rounds/{id}", handlers.Finance.CapTable.DeleteCapTableRound)
						// FiPlan sync — one-way push of a round's AmountRaisedK → capital_increase line
						r.With(planAccessMW.RequirePlanEdit).Post("/rounds/{roundId}/sync-to-fiplan", handlers.Finance.CapTable.SyncRoundToFiplan)
						r.With(planAccessMW.RequirePlanEdit).Delete("/rounds/{roundId}/sync-to-fiplan", handlers.Finance.CapTable.UnlinkFromFiplan)
						// Opening balance sync — founding capital (capital social bloqué avant immatriculation)
						r.With(planAccessMW.RequirePlanEdit).Post("/rounds/{roundId}/sync-to-opening-balance", handlers.Finance.CapTable.SyncRoundToOpeningBalance)
						r.With(planAccessMW.RequirePlanEdit).Delete("/rounds/{roundId}/sync-to-opening-balance", handlers.Finance.CapTable.UnsyncRoundFromOpeningBalance)

						// Stock option plans
						r.Get("/option-plans", handlers.Finance.CapTable.ListOptionPlans)
						r.With(planAccessMW.RequirePlanEdit).Put("/option-plans", handlers.Finance.CapTable.UpsertOptionPlans)
						r.With(planAccessMW.RequirePlanEdit).Delete("/option-plans/{id}", handlers.Finance.CapTable.DeleteOptionPlan)

						// Option grants
						r.Get("/option-grants", handlers.Finance.CapTable.ListOptionGrants)
						r.With(planAccessMW.RequirePlanEdit).Put("/option-grants", handlers.Finance.CapTable.UpsertOptionGrants)
						r.With(planAccessMW.RequirePlanEdit).Delete("/option-grants/{id}", handlers.Finance.CapTable.DeleteOptionGrant)

						// FastValo scenarios
						r.Get("/valuation", handlers.Finance.CapTable.ListValuationScenarios)
						r.With(planAccessMW.RequirePlanEdit).Post("/valuation", handlers.Finance.CapTable.CreateValuationScenario)
						r.With(planAccessMW.RequirePlanEdit).Put("/valuation/{id}", handlers.Finance.CapTable.UpdateValuationScenario)
						r.With(planAccessMW.RequirePlanEdit).Delete("/valuation/{id}", handlers.Finance.CapTable.DeleteValuationScenario)
						r.Post("/valuation/{id}/compute", handlers.Finance.CapTable.ComputeValuationScenario)

						// Scenario branches
						r.Get("/branches", handlers.Finance.CapTable.ListCapTableBranches)
						r.With(planAccessMW.RequirePlanEdit).Post("/branches", handlers.Finance.CapTable.CreateCapTableBranch)
						r.With(planAccessMW.RequirePlanEdit).Put("/branches/{id}", handlers.Finance.CapTable.UpdateCapTableBranch)

						// Computed reports (report timeout already applied above)
						r.Group(func(r chi.Router) {
							r.Use(reportLimiter.Handler)
							r.Get("/report", handlers.Finance.CapTable.GetCapTableReport)
							r.Get("/report/ingefie", handlers.Finance.CapTable.GetIngeFiReport)
							r.Get("/report/stock-options", handlers.Finance.CapTable.GetStockOptionReport)
							r.Get("/report/valuation", handlers.Finance.CapTable.GetFastValoReport)
							r.Get("/report/waterfall", handlers.Finance.CapTable.GetDilutionWaterfall)
							r.Get("/report/matrix", handlers.Finance.CapTable.GetCapTableMatrix)
						})
					})

					// Snapshots (use snapshot timeout)
						r.Route("/snapshots", func(r chi.Router) {
							r.Use(snapshotTimeout)
							r.Get("/", handlers.Plans.Snapshot.List)
							r.With(planAccessMW.RequirePlanEdit).Post("/", handlers.Plans.Snapshot.Create)
							r.Get("/{snapshotId}", handlers.Plans.Snapshot.Get)
							r.Get("/{snapshotId}/data", handlers.Plans.Snapshot.GetData)
							r.With(planAccessMW.RequirePlanEdit).Post("/{snapshotId}/restore", handlers.Plans.Snapshot.Restore)
							r.With(planAccessMW.RequirePlanEdit).Post("/{snapshotId}/clone", handlers.Plans.Snapshot.Clone)
							r.With(planAccessMW.RequirePlanEdit).Delete("/{snapshotId}", handlers.Plans.Snapshot.Delete)
						r.Get("/{snapshot1Id}/diff/{snapshot2Id}", handlers.Plans.Snapshot.Diff)
						})

						// ── AI narration routes ───────────────────────────────────────────────────────────────────
						// AI timeout applied — LLM inference needs up to 120 s.
						// Standard-tier: quota enforced by AI access middleware.
						// Pro/Enterprise: structural tier gate + AI access middleware.
						r.Route("/ai", func(r chi.Router) {
							r.Use(aiTimeout)
							r.Use(reportLimiter.Handler)
							// Resolve enriches every AI request with the tenant's real subscription
							// tier before any access or recording middleware runs.  This ensures
							// usage records always reflect the actual plan (not a default "standard")
							// even for features accessible to all tiers.
							// Pro/Enterprise sub-groups call Require() next; it reuses the tier
							// already in context — no extra DB round-trip.
							r.Use(tierGateMW.Resolve())

							// Standard-tier narration (AI access MW enforces per-feature quotas).
							r.With(aiAccessMW.RequireAIAccessWithRecording(model.AIFeaturePlanNarration)).
								Post("/narrate", handlers.AI.AI.Narrate)

							// Pro-tier driver-aware features.
							r.Group(func(r chi.Router) {
								r.Use(tierGateMW.Require(middleware.TierPro))

								r.With(aiAccessMW.RequireAIAccessWithRecording(model.AIFeatureUnitEconomics)).
									Post("/unit-economics", handlers.AI.AI.UnitEconomics)

								r.With(aiAccessMW.RequireAIAccessWithRecording(model.AIFeatureAssumptionReview)).
									Post("/assumption-review", handlers.AI.AI.AssumptionReview)

								r.With(aiAccessMW.RequireAIAccessWithRecording(model.AIFeatureBenchmarkCommentary)).
									Post("/benchmark-commentary", handlers.AI.AI.BenchmarkCommentary)

								r.With(aiAccessMW.RequireAIAccessWithRecording(model.AIFeaturePortfolioMix)).
									Post("/portfolio-mix", handlers.AI.AI.PortfolioMix)

								r.With(aiAccessMW.RequireAIAccessWithRecording(model.AIFeatureDriverAdvisor)).
									Post("/driver-advisor", handlers.AI.AI.DriverAdvisor)

								r.With(aiAccessMW.RequireAIAccessWithRecording(model.AIFeatureScenarioSuggestion)).
									Post("/scenario-suggestion", handlers.AI.AI.ScenarioSuggestion)

								r.With(aiAccessMW.RequireAIAccessWithRecording(model.AIFeatureSensitivityNarrative)).
									Post("/sensitivity-narrative", handlers.AI.AI.SensitivityNarrative)
							})

							// Enterprise-only investor memo (owner-only enforced by AI access policy).
							r.Group(func(r chi.Router) {
								r.Use(tierGateMW.Require(middleware.TierEnterprise))

								r.With(aiAccessMW.RequireAIAccessWithRecording(model.AIFeatureInvestorMemo)).
									Post("/investor-memo", handlers.AI.AI.InvestorMemo)
							})
						})
					})
				})
			})
		})
		}) // end tenant-required group
	}) // end /api/v1

	// 404 handler
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":{"code":"not_found","message":"route not found"}}`))
	})

	logger.WithField("phase", "router").Info("routes configured")
	return r
}
