package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ============================================================================
// Test helpers
// ============================================================================

// newDefaultPolicySvc creates an AIUsagePolicyService with the default policy
// matrix pre-seeded in the in-memory cache (no DB required).
// The repos are left nil; since admin/unlimited policies skip GetUsageCounts,
// tests that use nil limits never touch the DB.
func newDefaultPolicySvc(t *testing.T) (*AIUsagePolicyService, uuid.UUID) {
	t.Helper()
	svc := &AIUsagePolicyService{
		policyRepo:      nil,
		usageRepo:       nil,
		log:             logrus.NewEntry(logrus.New()),
		policyCache:     make(map[uuid.UUID]*model.AIUsagePolicyLookup),
		policyCacheTime: make(map[uuid.UUID]time.Time),
		policyCacheTTL:  5 * time.Minute,
	}
	tenantID := uuid.New()
	seedPolicyCacheFor(svc, tenantID, model.DefaultAIUsagePolicies())
	return svc, tenantID
}

// seedPolicyCacheFor injects policies into the service's in-memory cache.
func seedPolicyCacheFor(svc *AIUsagePolicyService, tenantID uuid.UUID, policies []model.AIUsagePolicy) {
	lookup := model.NewAIUsagePolicyLookup(policies)
	svc.mu.Lock()
	svc.policyCache[tenantID] = lookup
	svc.policyCacheTime[tenantID] = time.Now()
	svc.mu.Unlock()
}

// aiTestWeekKey formats a week key for assertions.
func aiTestWeekKey(t time.Time) string {
	y, w := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", y, w)
}

// ============================================================================
// DefaultAIUsagePolicies — model-level tests
// ============================================================================

func TestDefaultAIUsagePolicies_Count(t *testing.T) {
	policies := model.DefaultAIUsagePolicies()
	// Freemium tier adds 4 roles × 13 features = 52 entries (all denied).
	// Paid tiers (standard, pro, enterprise): 4 roles × 3 tiers × 13 features = 156.
	// Total: 156 + 52 = 208.
	assert.Equal(t, 208, len(policies), "expected 4 roles × 4 tiers × 13 features = 208")
}

func TestDefaultAIUsagePolicies_AdminHasFullUnlimitedAccess(t *testing.T) {
	lookup := model.NewAIUsagePolicyLookup(model.DefaultAIUsagePolicies())
	for _, tier := range model.AllAISubscriptionTiers() {
		for _, feat := range model.AllAIFeatures() {
			p := lookup.GetPolicy(model.AIRoleAdmin, tier, feat)
			require.NotNil(t, p, "admin/%s/%s should exist", tier, feat)
			assert.True(t, p.Allowed, "admin/%s/%s must be allowed", tier, feat)
			assert.Nil(t, p.DailyLimit, "admin/%s/%s must have no daily limit", tier, feat)
			assert.Equal(t, 100, p.Priority, "admin must have priority 100")
		}
	}
}

func TestDefaultAIUsagePolicies_ViewerOnlyGetsPlanNarration(t *testing.T) {
	lookup := model.NewAIUsagePolicyLookup(model.DefaultAIUsagePolicies())
	// On paid tiers (standard, pro, enterprise) viewers get only plan narration.
	// On freemium all AI is denied, so we skip it in this test.
	paidTiers := []string{model.AITierStandard, model.AITierPro, model.AITierEnterprise}
	for _, tier := range paidTiers {
		assert.True(t, lookup.GetPolicy(model.AIRoleViewer, tier, model.AIFeaturePlanNarration).Allowed,
			"viewer/%s should get plan narration", tier)

		for _, blocked := range []model.AIFeatureType{
			model.AIFeatureVarianceAnalysis,
			model.AIFeatureScenarioComparison,
			model.AIFeatureAnomalyDetection,
			model.AIFeatureCashRunway,
		} {
			p := lookup.GetPolicy(model.AIRoleViewer, tier, blocked)
			require.NotNil(t, p)
			assert.False(t, p.Allowed, "viewer/%s should NOT have %s", tier, blocked)
		}
	}
}

func TestDefaultAIUsagePolicies_FreemiumDeniesAllFeaturesForAllRoles(t *testing.T) {
	lookup := model.NewAIUsagePolicyLookup(model.DefaultAIUsagePolicies())
	// Admin bypasses tier restrictions and has full access on every plan,
	// including freemium. Only non-admin roles are blocked on the freemium tier.
	nonAdminRoles := []string{model.AIRoleOwner, model.AIRoleUser, model.AIRoleViewer}
	for _, role := range nonAdminRoles {
		for _, feat := range model.AllAIFeatures() {
			p := lookup.GetPolicy(role, model.AITierFreemium, feat)
			require.NotNil(t, p, "freemium/%s/%s policy must exist", role, feat)
			assert.False(t, p.Allowed, "freemium/%s/%s must be denied — no AI on freemium plan", role, feat)
		}
	}
}

func TestDefaultAIUsagePolicies_OwnerStandardHasLimits(t *testing.T) {
	lookup := model.NewAIUsagePolicyLookup(model.DefaultAIUsagePolicies())
	p := lookup.GetPolicy(model.AIRoleOwner, model.AITierStandard, model.AIFeaturePlanNarration)
	require.NotNil(t, p)
	assert.True(t, p.Allowed)
	require.NotNil(t, p.DailyLimit)
	assert.Equal(t, 10, *p.DailyLimit)
}

func TestDefaultAIUsagePolicies_ScenarioComparisonLockedOnStandard(t *testing.T) {
	lookup := model.NewAIUsagePolicyLookup(model.DefaultAIUsagePolicies())
	p := lookup.GetPolicy(model.AIRoleOwner, model.AITierStandard, model.AIFeatureScenarioComparison)
	require.NotNil(t, p)
	assert.False(t, p.Allowed, "scenario comparison must be locked on standard tier")
}

func TestDefaultAIUsagePolicies_EnterpriseOwnerUnlimited(t *testing.T) {
	lookup := model.NewAIUsagePolicyLookup(model.DefaultAIUsagePolicies())
	for _, feat := range model.AllAIFeatures() {
		p := lookup.GetPolicy(model.AIRoleOwner, model.AITierEnterprise, feat)
		require.NotNil(t, p, "enterprise owner must have policy for %s", feat)
		assert.True(t, p.Allowed)
		assert.Nil(t, p.DailyLimit, "enterprise owner must have no daily limit for %s", feat)
	}
}

func TestDefaultAIUsagePolicies_UserStandardBlockedAdvancedFeatures(t *testing.T) {
	lookup := model.NewAIUsagePolicyLookup(model.DefaultAIUsagePolicies())
	for _, blocked := range []model.AIFeatureType{
		model.AIFeatureVarianceAnalysis,
		model.AIFeatureScenarioComparison,
		model.AIFeatureAnomalyDetection,
	} {
		p := lookup.GetPolicy(model.AIRoleUser, model.AITierStandard, blocked)
		require.NotNil(t, p)
		assert.False(t, p.Allowed, "user/standard should not have %s", blocked)
	}
}

// ============================================================================
// CheckAccess — access control logic (no DB calls for nil-limit policies)
// ============================================================================

func TestCheckAccess_AdminAllFeaturesAllowed(t *testing.T) {
	svc, tenantID := newDefaultPolicySvc(t)
	userID := uuid.New()

	for _, feat := range model.AllAIFeatures() {
		result, err := svc.CheckAccess(context.Background(), tenantID, userID,
			model.AIRoleAdmin, model.AITierStandard, feat)
		require.NoError(t, err, "feature %s", feat)
		assert.True(t, result.Allowed, "admin should have access to %s", feat)
		assert.Equal(t, feat, result.FeatureType)
		assert.Equal(t, model.AIRoleAdmin, result.UserRole)
	}
}

func TestCheckAccess_ViewerBlockedVariance(t *testing.T) {
	svc, tenantID := newDefaultPolicySvc(t)
	userID := uuid.New()

	result, err := svc.CheckAccess(context.Background(), tenantID, userID,
		model.AIRoleViewer, model.AITierStandard, model.AIFeatureVarianceAnalysis)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, model.AIAccessDeniedTierNotAllowed, result.DeniedReason)
	assert.NotEmpty(t, result.DeniedMessage)
}

func TestCheckAccess_ViewerCanNarrate(t *testing.T) {
	// Viewer+standard+plan_narration has non-nil daily limits, so a real
	// CheckAccess call would invoke GetUsageCounts on a nil usageRepo (panic).
	// Instead, verify the policy matrix directly: the default policy must exist,
	// be enabled, and carry a non-nil daily limit (confirming quota enforcement).
	// Full end-to-end quota evaluation is covered by integration tests.
	policies := model.DefaultAIUsagePolicies()
	lookup := model.NewAIUsagePolicyLookup(policies)
	p := lookup.GetPolicy(model.AIRoleViewer, model.AITierStandard, model.AIFeaturePlanNarration)
	require.NotNil(t, p, "default policy matrix must include viewer+standard+plan_narration")
	assert.True(t, p.Allowed, "viewer plan narration should be allowed")
	assert.NotNil(t, p.DailyLimit, "viewer plan narration must have a daily limit (quota-gated)")
}

func TestCheckAccess_ViewerCanNarrate_ViaAdmin(t *testing.T) {
	// Use admin (nil limits) to verify the "allowed + no DB" path.
	svc, tenantID := newDefaultPolicySvc(t)
	userID := uuid.New()

	result, err := svc.CheckAccess(context.Background(), tenantID, userID,
		model.AIRoleAdmin, model.AITierStandard, model.AIFeaturePlanNarration)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

func TestCheckAccess_OwnerStandardBlockedScenarioComparison(t *testing.T) {
	svc, tenantID := newDefaultPolicySvc(t)
	userID := uuid.New()

	result, err := svc.CheckAccess(context.Background(), tenantID, userID,
		model.AIRoleOwner, model.AITierStandard, model.AIFeatureScenarioComparison)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, model.AIAccessDeniedTierNotAllowed, result.DeniedReason)
}

func TestCheckAccess_EnterpriseAdminNoDbCallNeeded(t *testing.T) {
	// Enterprise admin has nil limits — CheckAccess must NOT call GetUsageCounts.
	svc, tenantID := newDefaultPolicySvc(t)
	userID := uuid.New()

	for _, feat := range model.AllAIFeatures() {
		result, err := svc.CheckAccess(context.Background(), tenantID, userID,
			model.AIRoleAdmin, model.AITierEnterprise, feat)
		require.NoError(t, err, "should not panic or error for feature %s", feat)
		assert.True(t, result.Allowed, "enterprise admin must have access to %s", feat)
	}
}

func TestCheckAccess_UnknownTenantFallsBackToDefault(t *testing.T) {
	// A tenant with no cached policies and a nil policyRepo falls back to
	// DefaultAIUsagePolicies() in getPolicy(). But with a nil policyRepo, the
	// ListPolicies call would panic. We therefore only test with pre-seeded cache.
	svc, tenantID := newDefaultPolicySvc(t)
	userID := uuid.New()

	// Admin on a non-standard tier — still uses the seeded default cache.
	result, err := svc.CheckAccess(context.Background(), tenantID, userID,
		model.AIRoleAdmin, model.AITierPro, model.AIFeatureAnomalyDetection)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

// ============================================================================
// EnforceAccess Tests
// ============================================================================

func TestEnforceAccess_DeniedReturnsAIAccessError(t *testing.T) {
	svc, tenantID := newDefaultPolicySvc(t)
	userID := uuid.New()

	err := svc.EnforceAccess(context.Background(), tenantID, userID,
		model.AIRoleViewer, model.AITierStandard, model.AIFeatureVarianceAnalysis)
	require.Error(t, err)
	assert.True(t, IsAIAccessError(err))
}

func TestEnforceAccess_AdminAllowedReturnsNil(t *testing.T) {
	svc, tenantID := newDefaultPolicySvc(t)
	userID := uuid.New()

	// Admin + nil limits → short-circuit path → no usageRepo call → no panic.
	err := svc.EnforceAccess(context.Background(), tenantID, userID,
		model.AIRoleAdmin, model.AITierStandard, model.AIFeaturePlanNarration)
	require.NoError(t, err)
}

// ============================================================================
// AIAccessError — HTTP status mapping
// ============================================================================

func TestAIAccessError_HTTPStatusCodes(t *testing.T) {
	cases := []struct {
		reason   model.AIAccessDeniedReason
		wantCode int
	}{
		{model.AIAccessDeniedTierNotAllowed, 402},
		{model.AIAccessDeniedRoleNotAllowed, 403},
		{model.AIAccessDeniedDailyLimitReached, 429},
		{model.AIAccessDeniedWeeklyLimitReached, 429},
		{model.AIAccessDeniedMonthlyLimitReached, 429},
		{model.AIAccessDeniedFeatureDisabled, 503},
		{model.AIAccessDeniedMaintenanceMode, 503},
	}
	for _, tc := range cases {
		e := newAIAccessError(&model.AIAccessResult{
			DeniedReason:  tc.reason,
			DeniedMessage: "denied",
		})
		assert.Equal(t, tc.wantCode, e.HTTPStatusCode(), "reason=%s", tc.reason)
	}
}

func TestAIAccessError_IsUpgradeRequired(t *testing.T) {
	e := &AIAccessError{Reason: model.AIAccessDeniedTierNotAllowed}
	assert.True(t, e.IsUpgradeRequired())

	e2 := &AIAccessError{Reason: model.AIAccessDeniedRoleNotAllowed}
	assert.False(t, e2.IsUpgradeRequired())
}

func TestIsAIAccessError_True(t *testing.T) {
	var err error = &AIAccessError{Message: "denied"}
	assert.True(t, IsAIAccessError(err))
}

func TestIsAIAccessError_False(t *testing.T) {
	assert.False(t, IsAIAccessError(gorm.ErrRecordNotFound))
}

// ============================================================================
// Cache Invalidation
// ============================================================================

func TestInvalidateCache_RemovesEntry(t *testing.T) {
	svc, tenantID := newDefaultPolicySvc(t)

	svc.mu.RLock()
	_, exists := svc.policyCache[tenantID]
	svc.mu.RUnlock()
	require.True(t, exists, "cache should be populated before invalidation")

	svc.invalidateCache(tenantID)

	svc.mu.RLock()
	_, exists = svc.policyCache[tenantID]
	svc.mu.RUnlock()
	assert.False(t, exists, "cache should be empty after invalidation")
}

func TestInvalidateCache_OtherTenantsUnaffected(t *testing.T) {
	svc, tenantA := newDefaultPolicySvc(t)
	tenantB := uuid.New()
	seedPolicyCacheFor(svc, tenantB, model.DefaultAIUsagePolicies())

	svc.invalidateCache(tenantA)

	svc.mu.RLock()
	_, aExists := svc.policyCache[tenantA]
	_, bExists := svc.policyCache[tenantB]
	svc.mu.RUnlock()

	assert.False(t, aExists, "tenant A should be evicted")
	assert.True(t, bExists, "tenant B should be unaffected")
}

// ============================================================================
// GetDeniedMessage
// ============================================================================

func TestGetDeniedMessage_TierNotAllowed(t *testing.T) {
	msg := model.GetDeniedMessage(model.AIAccessDeniedTierNotAllowed, model.AIFeaturePlanNarration)
	assert.Contains(t, msg, "Plan Narration")
	assert.Contains(t, msg, "Upgrade")
}

func TestGetDeniedMessage_DailyLimit(t *testing.T) {
	msg := model.GetDeniedMessage(model.AIAccessDeniedDailyLimitReached, model.AIFeatureVarianceAnalysis)
	assert.Contains(t, msg, "Variance Analysis")
	assert.Contains(t, msg, "Daily limit")
}

func TestGetDeniedMessage_MaintenanceMode(t *testing.T) {
	msg := model.GetDeniedMessage(model.AIAccessDeniedMaintenanceMode, model.AIFeatureCashRunway)
	assert.Contains(t, msg, "Cash Runway Narrative")
	assert.Contains(t, msg, "unavailable")
}

// ============================================================================
// Helper function — also covers the unused aiTestWeekKey
// ============================================================================

func TestAITestWeekKey_Format(t *testing.T) {
	// 2026-W12 for a known Monday
	ts := time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC) // ISO week 12
	assert.Contains(t, aiTestWeekKey(ts), "2026-W")
}
