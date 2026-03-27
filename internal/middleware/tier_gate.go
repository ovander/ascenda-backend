package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"
	"ascenda/internal/pkg/ctxutil"
	"ascenda/internal/repo"
)

// ─────────────────────────────────────────────────────────────────────────────
// Tier constants
// ─────────────────────────────────────────────────────────────────────────────

const (
	TierFree       = "free"
	TierPro        = "pro"
	TierEnterprise = "enterprise"
)

// AllTiers returns the ordered list of canonical subscription tiers.
func AllTiers() []string {
	return []string{TierFree, TierPro, TierEnterprise}
}

// TierAtLeast returns true when current tier >= required tier.
func TierAtLeast(current, required string) bool {
	order := map[string]int{TierFree: 0, TierPro: 1, TierEnterprise: 2}
	cur, ok1 := order[current]
	req, ok2 := order[required]
	if !ok1 || !ok2 {
		return false
	}
	return cur >= req
}

// ─────────────────────────────────────────────────────────────────────────────
// TierGateMiddleware
// ─────────────────────────────────────────────────────────────────────────────

// TierGateMiddleware enforces subscription-tier requirements on route groups.
type TierGateMiddleware struct {
	tenantRepo repo.TenantRepository
	logger     *logrus.Entry
}

// NewTierGateMiddleware creates a TierGateMiddleware backed by the tenant repo.
func NewTierGateMiddleware(tenantRepo repo.TenantRepository, logger *logrus.Entry) *TierGateMiddleware {
	return &TierGateMiddleware{
		tenantRepo: tenantRepo,
		logger:     logger,
	}
}

// tierGateErrorResponse is the JSON body returned when tier access is denied.
type tierGateErrorResponse struct {
	Error        string `json:"error"`
	Message      string `json:"message"`
	Tier         string `json:"tier"`
	RequiredTier string `json:"requiredTier"`
	UpgradeURL   string `json:"upgradeUrl"`
}

// Resolve returns an HTTP middleware that enriches the request context with the
// tenant's subscription tier without blocking any requests.  Apply it to route
// groups that are accessible to all tiers but need accurate tier attribution
// for quota recording or usage analytics.
//
// When a Require() gate follows on a sub-group it reuses the tier already
// stored in context, so no extra DB round-trip occurs.
//
//	r.Route("/ai", func(r chi.Router) {
//	    r.Use(tierGateMW.Resolve())           // enriches, never blocks
//	    r.Post("/narrate", ...)               // standard — tier now correct
//	    r.Group(func(r chi.Router) {
//	        r.Use(tierGateMW.Require(TierPro)) // gates + reuses resolved tier
//	        r.Post("/unit-economics", ...)
//	    })
//	})
func (m *TierGateMiddleware) Resolve() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			tenantID := ctxutil.GetTenantID(ctx)

			if tenant, err := m.tenantRepo.GetByID(tenantID); err == nil && tenant != nil {
				ctx = ctxutil.WithTenantTier(ctx, tenant.Tier)
				r = r.WithContext(ctx)
			}
			// Never block — always forward, even on lookup failure.
			next.ServeHTTP(w, r)
		})
	}
}

// Require returns an HTTP middleware that allows only tenants whose tier is >=
// minTier. Unknown tier values are treated as TierFree.
//
// If Resolve() already ran for this request the tier is read directly from
// context, avoiding a second DB round-trip.
//
//	r.Group(func(r chi.Router) {
//	    r.Use(tierGate.Require(middleware.TierEnterprise))
//	    r.Mount("/captable", captableRouter)
//	})
func (m *TierGateMiddleware) Require(minTier string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			tenantID := ctxutil.GetTenantID(ctx)

			// Reuse tier already resolved by Resolve() to avoid a second DB
			// lookup when both middlewares apply to the same request.
			tenantTier := ctxutil.GetTenantTier(ctx)
			if tenantTier == "" {
				tenant, err := m.tenantRepo.GetByID(tenantID)
				if err != nil || tenant == nil {
					m.logger.WithField("tenant_id", tenantID).Warn("tier gate: tenant lookup failed")
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					json.NewEncoder(w).Encode(tierGateErrorResponse{ //nolint:errcheck
						Error:        "forbidden",
						Message:      "tenant not found",
						Tier:         "",
						RequiredTier: minTier,
						UpgradeURL:   "/settings/billing",
					})
					return
				}
				tenantTier = tenant.Tier
				ctx = ctxutil.WithTenantTier(ctx, tenantTier)
				r = r.WithContext(ctx)
			}

			if !TierAtLeast(tenantTier, minTier) {
				m.logger.WithFields(logrus.Fields{
					"tenant_id":     tenantID,
					"tenant_tier":   tenantTier,
					"required_tier": minTier,
				}).Warn("tier gate: upgrade required")

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(tierGateErrorResponse{ //nolint:errcheck
					Error:        "upgrade_required",
					Message:      "This feature requires the " + minTier + " plan or above",
					Tier:         tenantTier,
					RequiredTier: minTier,
					UpgradeURL:   "/settings/billing",
				})
				return
			}

			// Tier is already in context (set above or by a prior Resolve()).
			next.ServeHTTP(w, r)
		})
	}
}
