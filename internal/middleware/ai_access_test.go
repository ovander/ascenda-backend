package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/ctxutil"
	"ascenda/internal/service"
)

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// newTestPolicySvc builds an AIUsagePolicyService with the full default policy
// matrix seeded in the in-memory cache for a given tenantID.
// No DB connection is required; callers must avoid role+tier combinations that
// have quota limits (those would trigger a nil usageRepo panic). Safe combos:
//   - AIRoleAdmin:  all features, all tiers → nil limits
//   - AIRoleOwner + AITierPro: narration/cash-runway → nil limits
//   - AIRoleUser/Owner + AITierEnterprise for Pro features → nil limits
//   - Any role + standard tier + Pro feature → Allowed=false → no DB call
//   - AIRoleOwner + AITierEnterprise + InvestorMemo → nil limits
func newTestPolicySvc(tenantID uuid.UUID) *service.AIUsagePolicyService {
	return service.NewAIUsagePolicyServiceWithCache(
		tenantID,
		model.DefaultAIUsagePolicies(),
		logrus.NewEntry(logrus.New()),
	)
}

// newAIAccessRequest creates an HTTP request with tenant/user/role/tier set in context.
func newAIAccessRequest(tenantID, userID uuid.UUID, role, tier string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/ai/narrate", nil)
	ctx := req.Context()
	ctx = ctxutil.WithTenantID(ctx, tenantID)
	ctx = ctxutil.WithUserID(ctx, userID)
	ctx = ctxutil.WithUserRole(ctx, role)
	ctx = ctxutil.WithTenantTier(ctx, tier)
	return req.WithContext(ctx)
}

// ─────────────────────────────────────────────────────────────────────────────
// tenantTierToAITier — pure mapping function
// ─────────────────────────────────────────────────────────────────────────────

func TestTenantTierToAITier_Free_MapsToFreemium(t *testing.T) {
	// Freemium commercial plan maps to the freemium AI tier (no AI access).
	assert.Equal(t, model.AITierFreemium, tenantTierToAITier(TierFree))
}

func TestTenantTierToAITier_Pro_MapsToPro(t *testing.T) {
	assert.Equal(t, model.AITierPro, tenantTierToAITier(TierPro))
}

func TestTenantTierToAITier_Enterprise_MapsToEnterprise(t *testing.T) {
	assert.Equal(t, model.AITierEnterprise, tenantTierToAITier(TierEnterprise))
}

func TestTenantTierToAITier_EmptyString_MapsToFreemium(t *testing.T) {
	// Unrecognised or missing plan defaults to freemium (most restrictive).
	assert.Equal(t, model.AITierFreemium, tenantTierToAITier(""))
}

func TestTenantTierToAITier_Unknown_MapsToFreemium(t *testing.T) {
	assert.Equal(t, model.AITierFreemium, tenantTierToAITier("gold"))
	assert.Equal(t, model.AITierFreemium, tenantTierToAITier("premium"))
	assert.Equal(t, model.AITierFreemium, tenantTierToAITier("ENTERPRISE")) // case-sensitive
}

func TestTenantTierToAITier_AllCanonicalValues(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{TierFree, model.AITierFreemium},
		{TierPro, model.AITierPro},
		{TierEnterprise, model.AITierEnterprise},
		{"", model.AITierFreemium},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, tenantTierToAITier(tc.input))
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// accessErrHTTPStatus — denial reason → HTTP status
// ─────────────────────────────────────────────────────────────────────────────

func TestAccessErrHTTPStatus_TierNotAllowed_Returns402(t *testing.T) {
	assert.Equal(t, 402, accessErrHTTPStatus(model.AIAccessDeniedTierNotAllowed))
}

func TestAccessErrHTTPStatus_RoleNotAllowed_Returns403(t *testing.T) {
	assert.Equal(t, http.StatusForbidden, accessErrHTTPStatus(model.AIAccessDeniedRoleNotAllowed))
}

func TestAccessErrHTTPStatus_QuotaExceeded_Returns429(t *testing.T) {
	assert.Equal(t, http.StatusTooManyRequests, accessErrHTTPStatus(model.AIAccessDeniedDailyLimitReached))
	assert.Equal(t, http.StatusTooManyRequests, accessErrHTTPStatus(model.AIAccessDeniedWeeklyLimitReached))
	assert.Equal(t, http.StatusTooManyRequests, accessErrHTTPStatus(model.AIAccessDeniedMonthlyLimitReached))
}

func TestAccessErrHTTPStatus_FeatureDisabled_Returns503(t *testing.T) {
	assert.Equal(t, http.StatusServiceUnavailable, accessErrHTTPStatus(model.AIAccessDeniedFeatureDisabled))
}

func TestAccessErrHTTPStatus_MaintenanceMode_Returns503(t *testing.T) {
	assert.Equal(t, http.StatusServiceUnavailable, accessErrHTTPStatus(model.AIAccessDeniedMaintenanceMode))
}

func TestAccessErrHTTPStatus_UnknownReason_Returns403(t *testing.T) {
	assert.Equal(t, http.StatusForbidden, accessErrHTTPStatus("some_unknown_reason"))
}

// ─────────────────────────────────────────────────────────────────────────────
// quotaRemaining — arithmetic helper
// ─────────────────────────────────────────────────────────────────────────────

func TestQuotaRemaining_NilLimit_ReturnsNil(t *testing.T) {
	assert.Nil(t, quotaRemaining(nil, 5))
}

func TestQuotaRemaining_Positive_ReturnsCorrectRemainder(t *testing.T) {
	limit := 10
	result := quotaRemaining(&limit, 3)
	require.NotNil(t, result)
	assert.Equal(t, 7, *result)
}

func TestQuotaRemaining_ZeroUsed_ReturnsFullLimit(t *testing.T) {
	limit := 20
	result := quotaRemaining(&limit, 0)
	require.NotNil(t, result)
	assert.Equal(t, 20, *result)
}

func TestQuotaRemaining_UsedEqualsLimit_ReturnsZero(t *testing.T) {
	limit := 5
	result := quotaRemaining(&limit, 5)
	require.NotNil(t, result)
	assert.Equal(t, 0, *result)
}

func TestQuotaRemaining_OverLimit_ClampsToZero(t *testing.T) {
	// usage can temporarily exceed limit (e.g. concurrency); must not go negative.
	limit := 5
	result := quotaRemaining(&limit, 10)
	require.NotNil(t, result)
	assert.Equal(t, 0, *result)
}

// ─────────────────────────────────────────────────────────────────────────────
// GetAIAccessResult — context helper
// ─────────────────────────────────────────────────────────────────────────────

func TestGetAIAccessResult_ReturnsNilWhenNotSet(t *testing.T) {
	ctx := context.Background()
	assert.Nil(t, GetAIAccessResult(ctx))
}

func TestGetAIAccessResult_ReturnsStoredResult(t *testing.T) {
	expected := &model.AIAccessResult{
		Allowed:     true,
		FeatureType: model.AIFeaturePlanNarration,
	}
	ctx := context.WithValue(context.Background(), aiAccessResultKey{}, expected)
	got := GetAIAccessResult(ctx)
	require.NotNil(t, got)
	assert.Equal(t, expected, got)
}

// ─────────────────────────────────────────────────────────────────────────────
// RequireAIAccess — tier-aware access control
//
// NOTE ON SAFE ROLE+TIER COMBINATIONS
// The default policy matrix assigns quota limits to some role+tier entries.
// When limits are present, CheckAccess calls usageRepo.GetUsageCounts(), which
// panics with a nil repo in these unit tests.  We restrict to combos where:
//   • The policy is Allowed=false  →  service short-circuits before the DB call
//   • The policy has nil limits    →  service returns Allowed=true without DB
//
// Safe nil-limit combos from the default matrix:
//   AIRoleAdmin  + any tier  + any feature → Allowed=true,  nil limits
//   AIRoleUser   + Enterprise + Pro feature → Allowed=true,  nil limits
//   AIRoleOwner  + Enterprise + InvestorMemo → Allowed=true, nil limits
//   Any non-admin role + Standard + Pro feature → Allowed=false (no DB)
//   AIRoleOwner  + Standard/Pro + InvestorMemo → Allowed=false (no DB)
//
// These tests also serve as a regression guard for the 402 fix:
//   Before: AIAccessMiddleware always used model.AITierStandard regardless of
//           the tier stored in context by TierGateMiddleware → Pro users got 402.
//   After:  Reads tenantTierToAITier(ctxutil.GetTenantTier(ctx)) → correct tier.
// ─────────────────────────────────────────────────────────────────────────────

func TestRequireAIAccess_AdminRole_AllowsNarration(t *testing.T) {
	// Admin on any tier → nil limits → safe for no-DB test.
	tenantID := uuid.New()
	svc := newTestPolicySvc(tenantID)
	mw := NewAIAccessMiddleware(svc, logrus.NewEntry(logrus.New()))

	nextCalled := false
	handler := mw.RequireAIAccess(model.AIFeaturePlanNarration)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := newAIAccessRequest(tenantID, uuid.New(), model.AIRoleAdmin, TierFree)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, nextCalled, "admin must reach the handler for narration")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireAIAccess_StandardTier_BlocksProFeature(t *testing.T) {
	// A free-tier user hitting a Pro-only endpoint must get 402 (Allowed=false
	// short-circuits before any DB call, so nil usageRepo is safe here).
	tenantID := uuid.New()
	svc := newTestPolicySvc(tenantID)
	mw := NewAIAccessMiddleware(svc, logrus.NewEntry(logrus.New()))

	handler := mw.RequireAIAccess(model.AIFeatureSensitivityNarrative)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler must not be called for a blocked standard-tier user")
		}),
	)

	// Simulate context as set by TierGateMiddleware for a free tenant.
	req := newAIAccessRequest(tenantID, uuid.New(), model.AIRoleUser, TierFree)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 402, w.Code, "free-tier user must get 402 on Pro feature")

	var body AIAccessErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, string(model.AIAccessDeniedTierNotAllowed), body.Code)
	assert.True(t, body.UpgradeRequired)
	assert.NotEmpty(t, body.UpgradeURL)
}

func TestRequireAIAccess_ElevatedTier_AllowsProFeature(t *testing.T) {
	// Core regression guard: when the middleware reads the tier from context
	// (set by TierGateMiddleware) instead of hardcoding "standard", a tenant
	// with a higher tier must gain access to Pro features.
	//
	// We use Enterprise tier here because Enterprise + Pro feature → nil limits
	// in the default policy matrix, so no usageRepo call is made.  The mapping
	// "enterprise" → AITierEnterprise is verified separately by
	// TestTenantTierToAITier_Enterprise_MapsToEnterprise.
	tenantID := uuid.New()
	svc := newTestPolicySvc(tenantID)
	mw := NewAIAccessMiddleware(svc, logrus.NewEntry(logrus.New()))

	nextCalled := false
	handler := mw.RequireAIAccess(model.AIFeatureSensitivityNarrative)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		}),
	)

	// Enterprise tier stored in context (as TierGateMiddleware would set it).
	req := newAIAccessRequest(tenantID, uuid.New(), model.AIRoleUser, TierEnterprise)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, nextCalled,
		"elevated-tier user must reach handler for Pro feature (regression guard for 402 fix)")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireAIAccess_ElevatedTier_AllowsAllProFeatures(t *testing.T) {
	// All 7 Pro driver-aware features are accessible when an elevated tier is
	// stored in context.  Enterprise + pro feature → nil limits (safe).
	proFeatures := []model.AIFeatureType{
		model.AIFeatureUnitEconomics,
		model.AIFeatureAssumptionReview,
		model.AIFeatureBenchmarkCommentary,
		model.AIFeaturePortfolioMix,
		model.AIFeatureDriverAdvisor,
		model.AIFeatureScenarioSuggestion,
		model.AIFeatureSensitivityNarrative,
	}

	for _, feat := range proFeatures {
		feat := feat
		t.Run(string(feat), func(t *testing.T) {
			tenantID := uuid.New()
			svc := newTestPolicySvc(tenantID)
			mw := NewAIAccessMiddleware(svc, logrus.NewEntry(logrus.New()))

			nextCalled := false
			handler := mw.RequireAIAccess(feat)(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					nextCalled = true
					w.WriteHeader(http.StatusOK)
				}),
			)

			req := newAIAccessRequest(tenantID, uuid.New(), model.AIRoleUser, TierEnterprise)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.True(t, nextCalled, "Pro feature %s must be accessible on Enterprise tier", feat)
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestRequireAIAccess_StandardTier_BlocksAllProFeatures(t *testing.T) {
	// All 7 Pro features must return 402 for a standard-tier user.
	// Allowed=false → no usageRepo call → nil repo is safe.
	proFeatures := []model.AIFeatureType{
		model.AIFeatureUnitEconomics,
		model.AIFeatureAssumptionReview,
		model.AIFeatureBenchmarkCommentary,
		model.AIFeaturePortfolioMix,
		model.AIFeatureDriverAdvisor,
		model.AIFeatureScenarioSuggestion,
		model.AIFeatureSensitivityNarrative,
	}

	for _, feat := range proFeatures {
		feat := feat
		t.Run(string(feat), func(t *testing.T) {
			tenantID := uuid.New()
			svc := newTestPolicySvc(tenantID)
			mw := NewAIAccessMiddleware(svc, logrus.NewEntry(logrus.New()))

			handler := mw.RequireAIAccess(feat)(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					t.Fatalf("handler must not be called for free-tier user on Pro feature %s", feat)
				}),
			)

			req := newAIAccessRequest(tenantID, uuid.New(), model.AIRoleUser, TierFree)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, 402, w.Code,
				"free-tier user must get 402 on Pro feature %s", feat)
		})
	}
}

func TestRequireAIAccess_EnterpriseTier_AllowsInvestorMemo(t *testing.T) {
	// InvestorMemo: Enterprise, plan owner only → nil limits → safe.
	tenantID := uuid.New()
	svc := newTestPolicySvc(tenantID)
	mw := NewAIAccessMiddleware(svc, logrus.NewEntry(logrus.New()))

	nextCalled := false
	handler := mw.RequireAIAccess(model.AIFeatureInvestorMemo)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		}),
	)

	// Owner role + Enterprise tier — the only allowed combination.
	req := newAIAccessRequest(tenantID, uuid.New(), model.AIRoleOwner, TierEnterprise)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, nextCalled, "Enterprise owner must reach handler for Investor Memo")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireAIAccess_ProTier_BlocksInvestorMemo(t *testing.T) {
	// Investor Memo is Enterprise-only; Pro tier must still get 402.
	// Allowed=false → no usageRepo call → safe.
	tenantID := uuid.New()
	svc := newTestPolicySvc(tenantID)
	mw := NewAIAccessMiddleware(svc, logrus.NewEntry(logrus.New()))

	handler := mw.RequireAIAccess(model.AIFeatureInvestorMemo)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler must not be called for Pro user on Enterprise-only feature")
		}),
	)

	req := newAIAccessRequest(tenantID, uuid.New(), model.AIRoleOwner, TierPro)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 402, w.Code, "Pro owner must get 402 on Enterprise-only Investor Memo")

	var body AIAccessErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, string(model.AIAccessDeniedTierNotAllowed), body.Code)
}

func TestRequireAIAccess_StandardTier_BlocksInvestorMemo(t *testing.T) {
	tenantID := uuid.New()
	svc := newTestPolicySvc(tenantID)
	mw := NewAIAccessMiddleware(svc, logrus.NewEntry(logrus.New()))

	handler := mw.RequireAIAccess(model.AIFeatureInvestorMemo)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler must not be called for standard user on Enterprise-only feature")
		}),
	)

	req := newAIAccessRequest(tenantID, uuid.New(), model.AIRoleOwner, TierFree)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 402, w.Code)
}

func TestRequireAIAccess_AdminRole_AllowsAllFeatures(t *testing.T) {
	// Admin bypasses tier restrictions across all features and all tiers.
	// Admin policies always have nil limits → no DB call → safe.
	allFeatures := model.AllAIFeatures()

	for _, feat := range allFeatures {
		feat := feat
		t.Run(string(feat), func(t *testing.T) {
			tenantID := uuid.New()
			svc := newTestPolicySvc(tenantID)
			mw := NewAIAccessMiddleware(svc, logrus.NewEntry(logrus.New()))

			nextCalled := false
			handler := mw.RequireAIAccess(feat)(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					nextCalled = true
					w.WriteHeader(http.StatusOK)
				}),
			)

			// Admin on standard (free) tier — should still be allowed.
			req := newAIAccessRequest(tenantID, uuid.New(), model.AIRoleAdmin, TierFree)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.True(t, nextCalled,
				"admin must be allowed for feature %s regardless of tier", feat)
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestRequireAIAccess_AccessResultStoredInContext(t *testing.T) {
	// RequireAIAccess must store the AIAccessResult in context so handlers can
	// inspect quota details without repeating the policy lookup.
	// Using admin role → nil limits → no DB call → safe.
	tenantID := uuid.New()
	svc := newTestPolicySvc(tenantID)
	mw := NewAIAccessMiddleware(svc, logrus.NewEntry(logrus.New()))

	var resultInCtx *model.AIAccessResult
	handler := mw.RequireAIAccess(model.AIFeaturePlanNarration)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resultInCtx = GetAIAccessResult(r.Context())
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := newAIAccessRequest(tenantID, uuid.New(), model.AIRoleAdmin, TierFree)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, resultInCtx,
		"AIAccessResult must be stored in context on an allowed request")
	assert.True(t, resultInCtx.Allowed)
	assert.Equal(t, model.AIFeaturePlanNarration, resultInCtx.FeatureType)
}

func TestRequireAIAccess_DeniedResponse_ContainsFeatureName(t *testing.T) {
	// The JSON error body must include the feature name so the client can
	// display a targeted upgrade prompt.
	// user+standard+ProFeature → Allowed=false → no DB call → safe.
	tenantID := uuid.New()
	svc := newTestPolicySvc(tenantID)
	mw := NewAIAccessMiddleware(svc, logrus.NewEntry(logrus.New()))

	handler := mw.RequireAIAccess(model.AIFeatureUnitEconomics)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)

	req := newAIAccessRequest(tenantID, uuid.New(), model.AIRoleUser, TierFree)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, 402, w.Code)

	var body AIAccessErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, string(model.AIFeatureUnitEconomics), body.Feature)
}

// ─────────────────────────────────────────────────────────────────────────────
// RequireAIAccessWithRecording — combined middleware (denied paths only)
//
// The recording path fires only on 2xx responses and calls
// policyService.RecordUsage → usageRepo.Create, which requires a real DB.
// We therefore only test the denied path here, where the access check
// short-circuits before the recorder fires (Allowed=false → no recording).
// The full recording path is covered by integration/DB tests elsewhere.
// ─────────────────────────────────────────────────────────────────────────────

func TestRequireAIAccessWithRecording_StandardTier_BlocksProFeature(t *testing.T) {
	// Allowed=false → recording goroutine never fires → nil usageRepo is safe.
	tenantID := uuid.New()
	svc := newTestPolicySvc(tenantID)
	mw := NewAIAccessMiddleware(svc, logrus.NewEntry(logrus.New()))

	handler := mw.RequireAIAccessWithRecording(model.AIFeatureAssumptionReview)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler must not be called for blocked standard-tier user")
		}),
	)

	req := newAIAccessRequest(tenantID, uuid.New(), model.AIRoleUser, TierFree)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	mw.Wait()
	assert.Equal(t, 402, w.Code)
}

func TestRequireAIAccessWithRecording_StandardTier_BlocksInvestorMemo(t *testing.T) {
	// Denied at access-check stage → recorder never fires → safe with nil usageRepo.
	tenantID := uuid.New()
	svc := newTestPolicySvc(tenantID)
	mw := NewAIAccessMiddleware(svc, logrus.NewEntry(logrus.New()))

	handler := mw.RequireAIAccessWithRecording(model.AIFeatureInvestorMemo)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler must not be called for Pro user on Enterprise-only feature")
		}),
	)

	req := newAIAccessRequest(tenantID, uuid.New(), model.AIRoleOwner, TierPro)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	mw.Wait()
	assert.Equal(t, 402, w.Code)
}

func TestRequireAIAccessWithRecording_AccessCheckPrecedesRecorder(t *testing.T) {
	// Structural test: the combined middleware must run the access check BEFORE
	// the recorder.  If the check denies, the handler (and its subsequent
	// recorder) must never execute.
	tenantID := uuid.New()
	svc := newTestPolicySvc(tenantID)
	mw := NewAIAccessMiddleware(svc, logrus.NewEntry(logrus.New()))

	handlerReached := false
	handler := mw.RequireAIAccessWithRecording(model.AIFeaturePortfolioMix)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerReached = true
			w.WriteHeader(http.StatusOK)
		}),
	)

	// Standard tier → Pro feature blocked by access check.
	req := newAIAccessRequest(tenantID, uuid.New(), model.AIRoleUser, TierFree)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	mw.Wait()

	assert.False(t, handlerReached,
		"handler (and recorder) must not fire when access check denies the request")
	assert.Equal(t, 402, w.Code)
}
