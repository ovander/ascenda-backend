package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/backendkit/tiering"
	"github.com/sirupsen/logrus"
)

// ─────────────────────────────────────────────────────────────────────────────
// Plan constants — forwarded from backendkit/tiering.
// ─────────────────────────────────────────────────────────────────────────────

const (
	TierFree       = tiering.PlanFreemium
	TierPro        = tiering.PlanPro
	TierEnterprise = tiering.PlanEnterprise

	PlanFreemium   = tiering.PlanFreemium
	PlanPro        = tiering.PlanPro
	PlanEnterprise = tiering.PlanEnterprise
)

// defaultRegistry is the shared plan ordering for Ascenda: freemium < pro < enterprise.
var defaultRegistry = tiering.DefaultRegistry()

// AllTiers returns the ordered list of canonical plan values (lowest → highest).
func AllTiers() []string { return defaultRegistry.Plans() }

// TierAtLeast returns true when current plan >= required plan.
func TierAtLeast(current, required string) bool {
	return defaultRegistry.TierAtLeast(current, required)
}

// ─────────────────────────────────────────────────────────────────────────────
// TierGateMiddleware
// ─────────────────────────────────────────────────────────────────────────────

// TierGateMiddleware enforces plan requirements on route groups.
// Plan comparison is delegated to backendkit's tiering.PlanRegistry.
type TierGateMiddleware struct {
	registry   *tiering.PlanRegistry
	logger     *logrus.Entry
	upgradeURL string
}

// NewTierGateMiddleware creates a TierGateMiddleware backed by the default plan registry.
func NewTierGateMiddleware(logger *logrus.Entry) *TierGateMiddleware {
	return &TierGateMiddleware{
		registry:   tiering.DefaultRegistry(),
		logger:     logger,
		upgradeURL: "/settings/billing",
	}
}

// tierGateErrorResponse is the JSON body returned when plan access is denied.
// Field names match what the Ascenda frontend expects.
type tierGateErrorResponse struct {
	Error        string `json:"error"`
	Message      string `json:"message"`
	Tier         string `json:"tier"`
	RequiredTier string `json:"requiredTier"`
	UpgradeURL   string `json:"upgradeUrl"`
}

// Resolve is a no-op pass-through middleware. The user's plan is already stored
// in context by TenantMiddleware; this method exists for composability with
// route groups that mix Resolve + Require.
func (m *TierGateMiddleware) Resolve() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}

// Require returns an HTTP middleware that allows only users whose plan is >=
// minPlan. Unknown / missing plan values are normalised to the lowest tier
// (freemium) by the registry.
func (m *TierGateMiddleware) Require(minPlan string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			userPlan := m.registry.Normalise(ctxutil.GetUserPlan(ctx))

			if !m.registry.TierAtLeast(userPlan, minPlan) {
				m.logger.WithFields(logrus.Fields{
					"user_plan":     userPlan,
					"required_plan": minPlan,
				}).Warn("plan gate: upgrade required")

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(tierGateErrorResponse{ //nolint:errcheck
					Error:        "upgrade_required",
					Message:      "This feature requires the " + minPlan + " plan or above",
					Tier:         userPlan,
					RequiredTier: minPlan,
					UpgradeURL:   m.upgradeURL,
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
