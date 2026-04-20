// Package middleware — AI feature access middleware.
// Ported from GPWA internal/middleware/ai_access.go and adapted for Ascenda
// (ctxutil for context helpers, UUID IDs, no subscription-tier in JWT →
// defaults to model.AITierStandard).
package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/sirupsen/logrus"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/ctxutil"
	"ascenda/internal/service"
)

// ============================================================================
// AI Access Middleware
// ============================================================================

// AIAccessMiddleware enforces AI feature access based on user role and tier.
type AIAccessMiddleware struct {
	policyService *service.AIUsagePolicyService
	log           *logrus.Entry
	// wg tracks in-flight async usage-recording goroutines so that graceful
	// shutdown can drain them before the database connection pool is closed.
	// Call Wait() between HTTP drain and database close during shutdown.
	wg sync.WaitGroup
}

// NewAIAccessMiddleware creates a new AIAccessMiddleware.
func NewAIAccessMiddleware(policyService *service.AIUsagePolicyService, log *logrus.Entry) *AIAccessMiddleware {
	return &AIAccessMiddleware{
		policyService: policyService,
		log:           log,
	}
}

// Wait blocks until all async usage-recording goroutines spawned by
// AIUsageRecorder have completed. Call this during graceful shutdown —
// after the HTTP server has stopped accepting new requests but before
// the database connection pool is closed.
func (m *AIAccessMiddleware) Wait() {
	m.wg.Wait()
}

// AIAccessErrorResponse is the JSON body returned when access is denied.
type AIAccessErrorResponse struct {
	Error            string `json:"error"`
	Code             string `json:"code"`
	Feature          string `json:"feature"`
	DailyUsed        int    `json:"daily_used,omitempty"`
	DailyLimit       *int   `json:"daily_limit,omitempty"`
	DailyRemaining   *int   `json:"daily_remaining,omitempty"`
	WeeklyUsed       int    `json:"weekly_used,omitempty"`
	WeeklyLimit      *int   `json:"weekly_limit,omitempty"`
	WeeklyRemaining  *int   `json:"weekly_remaining,omitempty"`
	MonthlyUsed      int    `json:"monthly_used,omitempty"`
	MonthlyLimit     *int   `json:"monthly_limit,omitempty"`
	MonthlyRemaining *int   `json:"monthly_remaining,omitempty"`
	UpgradeURL       string `json:"upgrade_url,omitempty"`
	UpgradeRequired  bool   `json:"upgrade_required,omitempty"`
}

// RequireAIAccess returns a middleware that enforces access to a specific AI feature.
// If the user's subscription tier is not stored in context (current default),
// model.AITierStandard is used.
func (m *AIAccessMiddleware) RequireAIAccess(feature model.AIFeatureType) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			tenantID := ctxutil.GetTenantID(ctx)
			userID := ctxutil.GetUserID(ctx)
			role := ctxutil.GetUserRole(ctx)

			// TierGateMiddleware stores the tenant tier in context after its own
			// DB lookup. Map it to the AI policy tier vocabulary. Routes without a
			// tier gate (e.g. /narrate) have no tier in context and default to
			// "standard" — which is the correct baseline for free/unverified users.
			tier := tenantTierToAITier(ctxutil.GetTenantTier(ctx))

			result, err := m.policyService.CheckAccess(ctx, tenantID, userID, role, tier, feature)
			if err != nil {
				m.log.WithError(err).Error("AI access check failed")
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			if !result.Allowed {
				m.log.WithFields(logrus.Fields{
					"user_id": userID,
					"role":    role,
					"tier":    tier,
					"feature": feature,
					"reason":  result.DeniedReason,
				}).Warn("AI feature access denied")

				statusCode := accessErrHTTPStatus(result.DeniedReason)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(statusCode)

				resp := AIAccessErrorResponse{
					Error:           result.DeniedMessage,
					Code:            string(result.DeniedReason),
					Feature:         string(feature),
					DailyUsed:       result.DailyUsed,
					DailyLimit:      result.DailyLimit,
					DailyRemaining:  quotaRemaining(result.DailyLimit, result.DailyUsed),
					WeeklyUsed:      result.WeeklyUsed,
					WeeklyLimit:     result.WeeklyLimit,
					WeeklyRemaining: quotaRemaining(result.WeeklyLimit, result.WeeklyUsed),
					MonthlyUsed:     result.MonthlyUsed,
					MonthlyLimit:    result.MonthlyLimit,
					MonthlyRemaining: quotaRemaining(result.MonthlyLimit, result.MonthlyUsed),
					UpgradeRequired: result.DeniedReason == model.AIAccessDeniedTierNotAllowed,
				}
				if resp.UpgradeRequired {
					resp.UpgradeURL = "/settings/subscription"
				}
				json.NewEncoder(w).Encode(resp) //nolint:errcheck
				return
			}

			// Store access result in context so handlers can inspect quota.
			ctx = context.WithValue(ctx, aiAccessResultKey{}, result)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AIUsageRecorder wraps a handler to record AI usage after a successful response.
// Usage is recorded asynchronously so it does not block the response.
func (m *AIAccessMiddleware) AIUsageRecorder(feature model.AIFeatureType) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := &aiResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rw, r)

			if rw.statusCode >= 200 && rw.statusCode < 300 {
				// Do not consume quota for deterministic fallback responses.
				// When the AI call fails (timeout, API error, parse failure) the
				// handler returns HTTP 200 with a fallback narration and sets the
				// X-AI-Generated: false header.  Counting these as successful
				// quota uses would exhaust the user's limit on failed attempts.
				if rw.Header().Get("X-AI-Generated") == "false" {
					return
				}

				ctx := r.Context()
				tenantID := ctxutil.GetTenantID(ctx)
				userID := ctxutil.GetUserID(ctx)
				role := ctxutil.GetUserRole(ctx)
				tier := model.AITierStandard

				m.wg.Add(1)
				go func() {
					defer m.wg.Done()
					if err := m.policyService.RecordUsage(
						context.Background(), tenantID, userID, feature, role, tier,
					); err != nil {
						m.log.WithError(err).Warn("failed to record AI usage")
					}
				}()
			}
		})
	}
}

// RequireAIAccessWithRecording combines access check and usage recording.
// Equivalent to RequireAIAccess → AIUsageRecorder → handler.
func (m *AIAccessMiddleware) RequireAIAccessWithRecording(feature model.AIFeatureType) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return m.RequireAIAccess(feature)(m.AIUsageRecorder(feature)(next))
	}
}

// ── Context helpers ───────────────────────────────────────────────────────────

type aiAccessResultKey struct{}

// GetAIAccessResult retrieves the AI access result stored by RequireAIAccess.
func GetAIAccessResult(ctx context.Context) *model.AIAccessResult {
	if v, ok := ctx.Value(aiAccessResultKey{}).(*model.AIAccessResult); ok {
		return v
	}
	return nil
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// aiResponseWriter captures the HTTP status code written by a handler.
type aiResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *aiResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// quotaRemaining computes remaining quota from limit and used counts.
func quotaRemaining(limit *int, used int) *int {
	if limit == nil {
		return nil
	}
	r := *limit - used
	if r < 0 {
		r = 0
	}
	return &r
}

// tenantTierToAITier maps the user's commercial plan ("freemium", "pro", "enterprise")
// to the AI usage-policy tier ("freemium", "standard", "pro", "enterprise").
// Unrecognised values default to "freemium" (most restrictive) for safety.
func tenantTierToAITier(userPlan string) string {
	switch userPlan {
	case TierPro:
		return model.AITierPro
	case TierEnterprise:
		return model.AITierEnterprise
	case "standard":
		return model.AITierStandard
	default: // "freemium", "" or any unknown value → no AI access
		return model.AITierFreemium
	}
}

// accessErrHTTPStatus maps a denial reason to an HTTP status code.
func accessErrHTTPStatus(reason model.AIAccessDeniedReason) int {
	switch reason {
	case model.AIAccessDeniedTierNotAllowed:
		return 402
	case model.AIAccessDeniedRoleNotAllowed:
		return 403
	case model.AIAccessDeniedDailyLimitReached,
		model.AIAccessDeniedWeeklyLimitReached,
		model.AIAccessDeniedMonthlyLimitReached:
		return 429
	case model.AIAccessDeniedFeatureDisabled, model.AIAccessDeniedMaintenanceMode:
		return 503
	default:
		return 403
	}
}
