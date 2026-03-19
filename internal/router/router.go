package router

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/sirupsen/logrus"
	"kerplan/internal/handler"
	"kerplan/internal/middleware"
)

// NewRouter creates and configures the main router with all routes and middleware.
func NewRouter(
	handlers *handler.HandlerBundle,
	authMW *middleware.AuthMiddleware,
	tenantMW *middleware.TenantMiddleware,
	rbacMW *middleware.RBACMiddleware,
	planAccessMW *middleware.PlanAccessMiddleware,
	loggerMW *middleware.LoggerMiddleware,
	recoverMW *middleware.RecoverMiddleware,
	requestIDMW *middleware.RequestIDMiddleware,
	securityMW *middleware.SecurityHeadersMiddleware,
	logger *logrus.Entry,
) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware (applied to all routes)
	r.Use(middleware.CORSMiddleware())
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

	// Auth routes (no tenant context required)
	r.Route("/auth", func(r chi.Router) {
		r.Post("/login", handlers.Admin.Auth.Login)
		r.Post("/callback", handlers.Admin.Auth.Callback)
		r.Post("/refresh", handlers.Admin.Auth.Refresh)
		r.Post("/logout", handlers.Admin.Auth.Logout)
	})

	// Version endpoint (no auth required — useful for deploy checks)
	r.Get("/api/v1/version", handlers.Admin.Metadata.GetVersion)

	// API routes that require auth but NOT tenant context
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(authMW.Handler)
		r.Use(generalLimiter.Handler)
		r.Use(crudTimeout)

		// All data routes require tenant context (resolves KerPlan role from DB)
		r.Group(func(r chi.Router) {
			r.Use(tenantMW.Handler)

			// User profile — needs tenant middleware to resolve KerPlan role
			r.Get("/users/me", handlers.Admin.User.GetMe)
			r.Put("/users/me", handlers.Admin.User.UpdateMe)

			// Metadata (enum lists for frontend)
			r.Get("/metadata", handlers.Admin.Metadata.GetMetadata)

			// User management routes (require manage:users permission)
			r.Route("/users", func(r chi.Router) {
				r.Use(rbacMW.RequirePermission(middleware.PermManageUsers))
				r.Get("/", handlers.Admin.User.List)
				r.Post("/invite", handlers.Admin.User.Invite)
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

		// Plan routes — access is checked per-plan by PlanAccessMiddleware.
		// Admin role is blocked by PlanAccessMiddleware on individual plan routes.
		// List endpoint returns only plans the user has access to.
		r.Route("/plans", func(r chi.Router) {
			r.Get("/", handlers.Plans.Plan.List)
			r.With(rbacMW.RequirePermission(middleware.PermManagePlan)).Post("/", handlers.Plans.Plan.Create)

			// All routes under /{planId} require plan access check
			r.Route("/{planId}", func(r chi.Router) {
				r.Use(planAccessMW.RequirePlanAccess)
				r.Get("/", handlers.Plans.Plan.Get)
				r.With(planAccessMW.RequirePlanEdit).Put("/", handlers.Plans.Plan.Update)
				r.With(rbacMW.RequirePermission(middleware.PermManagePlan)).Delete("/", handlers.Plans.Plan.Delete)

				// Plan members (owner only — manages who has access)
				r.Route("/members", func(r chi.Router) {
					r.Get("/", handlers.Plans.PlanMember.List)
					r.With(rbacMW.RequirePermission(middleware.PermManagePlan)).Post("/", handlers.Plans.PlanMember.Grant)
					r.With(rbacMW.RequirePermission(middleware.PermManagePlan)).Put("/{userId}", handlers.Plans.PlanMember.UpdateRole)
					r.With(rbacMW.RequirePermission(middleware.PermManagePlan)).Delete("/{userId}", handlers.Plans.PlanMember.Revoke)
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

							// Consolidated revenue uses report timeout + report rate limit
							r.Group(func(r chi.Router) {
								r.Use(reportTimeout)
								r.Use(reportLimiter.Handler)
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

					// Full Report
					r.Group(func(r chi.Router) {
						r.Use(reportTimeout)
						r.Use(reportLimiter.Handler)
						r.Get("/report", handlers.Finance.Report.GetFullReport)
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

	logger.Info("routes configured successfully")
	return r
}
