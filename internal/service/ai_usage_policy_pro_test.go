package service

// Tests for the Pro and Enterprise AI usage policy matrix added in the
// 8-feature expansion. These tests complement the existing standard-tier
// tests in ai_usage_policy_service_test.go.

import (
	"testing"

	"ascenda/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// proLookup is a shorthand for the full default policy lookup table.
func proLookup() *model.AIUsagePolicyLookup {
	return model.NewAIUsagePolicyLookup(model.DefaultAIUsagePolicies())
}

// ============================================================================
// Policy count — full matrix after 8-feature expansion
// ============================================================================

func TestDefaultAIUsagePolicies_TotalCount_FullMatrix(t *testing.T) {
	policies := model.DefaultAIUsagePolicies()
	// 4 roles × 4 tiers × 13 features = 208 entries.
	// Freemium tier: 3 non-admin roles × 13 features = 39 (admin bypasses tier restrictions).
	// Admin: 4 tiers × 13 features = 52.
	// Owner/User: 3 paid tiers × 13 features each = 26 (5 standard + 7 pro driver + 1 investor) × 3 tiers.
	// Viewer: 3 paid tiers × 13 features each.
	// Total: 4 roles × 4 tiers × 13 features = 208 unique entries.
	assert.Equal(t, 208, len(policies),
		"expected 4 roles × 4 tiers × 13 features = 208 policy entries (no duplicates)")
}

// ============================================================================
// Pro driver features — owner access matrix
// ============================================================================

func TestProDriverFeatures_OwnerStandard_Blocked(t *testing.T) {
	lkp := proLookup()
	for _, feat := range proDriverFeatureList() {
		p := lkp.GetPolicy(model.AIRoleOwner, model.AITierStandard, feat)
		require.NotNil(t, p, "owner/standard/%s policy must exist", feat)
		assert.False(t, p.Allowed,
			"owner/standard/%s must be blocked (Pro-only feature)", feat)
	}
}

func TestProDriverFeatures_OwnerPro_AllowedWithLimits(t *testing.T) {
	lkp := proLookup()
	for _, feat := range proDriverFeatureList() {
		p := lkp.GetPolicy(model.AIRoleOwner, model.AITierPro, feat)
		require.NotNil(t, p, "owner/pro/%s policy must exist", feat)
		assert.True(t, p.Allowed, "owner/pro/%s must be allowed", feat)
		require.NotNil(t, p.DailyLimit, "owner/pro/%s must have a daily limit", feat)
		assert.Equal(t, 5, *p.DailyLimit,
			"owner/pro/%s daily limit must be 5", feat)
		require.NotNil(t, p.WeeklyLimit, "owner/pro/%s must have a weekly limit", feat)
		assert.Equal(t, 20, *p.WeeklyLimit,
			"owner/pro/%s weekly limit must be 20", feat)
	}
}

func TestProDriverFeatures_OwnerEnterprise_Unlimited(t *testing.T) {
	lkp := proLookup()
	for _, feat := range proDriverFeatureList() {
		p := lkp.GetPolicy(model.AIRoleOwner, model.AITierEnterprise, feat)
		require.NotNil(t, p, "owner/enterprise/%s policy must exist", feat)
		assert.True(t, p.Allowed, "owner/enterprise/%s must be allowed", feat)
		assert.Nil(t, p.DailyLimit,
			"owner/enterprise/%s must have no daily limit (unlimited)", feat)
		assert.Nil(t, p.WeeklyLimit,
			"owner/enterprise/%s must have no weekly limit (unlimited)", feat)
	}
}

// ============================================================================
// Pro driver features — user access matrix
// ============================================================================

func TestProDriverFeatures_UserStandard_Blocked(t *testing.T) {
	lkp := proLookup()
	for _, feat := range proDriverFeatureList() {
		p := lkp.GetPolicy(model.AIRoleUser, model.AITierStandard, feat)
		require.NotNil(t, p)
		assert.False(t, p.Allowed,
			"user/standard/%s must be blocked", feat)
	}
}

func TestProDriverFeatures_UserPro_AllowedWithTighterLimits(t *testing.T) {
	lkp := proLookup()
	for _, feat := range proDriverFeatureList() {
		p := lkp.GetPolicy(model.AIRoleUser, model.AITierPro, feat)
		require.NotNil(t, p)
		assert.True(t, p.Allowed, "user/pro/%s must be allowed", feat)
		require.NotNil(t, p.DailyLimit)
		assert.Equal(t, 3, *p.DailyLimit,
			"user/pro/%s daily limit must be 3 (tighter than owner)", feat)
		require.NotNil(t, p.WeeklyLimit)
		assert.Equal(t, 15, *p.WeeklyLimit,
			"user/pro/%s weekly limit must be 15", feat)
	}
}

func TestProDriverFeatures_UserEnterprise_AllowedUnlimited(t *testing.T) {
	lkp := proLookup()
	for _, feat := range proDriverFeatureList() {
		p := lkp.GetPolicy(model.AIRoleUser, model.AITierEnterprise, feat)
		require.NotNil(t, p)
		assert.True(t, p.Allowed)
		assert.Nil(t, p.DailyLimit)
	}
}

// ============================================================================
// Pro driver features — viewer always blocked
// ============================================================================

func TestProDriverFeatures_ViewerAllTiers_AlwaysBlocked(t *testing.T) {
	lkp := proLookup()
	for _, feat := range proDriverFeatureList() {
		for _, tier := range model.AllAISubscriptionTiers() {
			p := lkp.GetPolicy(model.AIRoleViewer, tier, feat)
			require.NotNil(t, p)
			assert.False(t, p.Allowed,
				"viewer/%s/%s must always be blocked for Pro features", tier, feat)
		}
	}
}

// ============================================================================
// InvestorMemo — Enterprise, plan owner only
// ============================================================================

func TestInvestorMemo_OwnerEnterprise_Allowed_Unlimited(t *testing.T) {
	lkp := proLookup()
	p := lkp.GetPolicy(model.AIRoleOwner, model.AITierEnterprise, model.AIFeatureInvestorMemo)
	require.NotNil(t, p)
	assert.True(t, p.Allowed, "owner/enterprise/investor_memo must be allowed")
	assert.Nil(t, p.DailyLimit, "enterprise owner investor memo must be unlimited")
}

func TestInvestorMemo_OwnerStandard_Blocked(t *testing.T) {
	lkp := proLookup()
	p := lkp.GetPolicy(model.AIRoleOwner, model.AITierStandard, model.AIFeatureInvestorMemo)
	require.NotNil(t, p)
	assert.False(t, p.Allowed, "owner/standard/investor_memo must be blocked")
}

func TestInvestorMemo_OwnerPro_Blocked(t *testing.T) {
	lkp := proLookup()
	p := lkp.GetPolicy(model.AIRoleOwner, model.AITierPro, model.AIFeatureInvestorMemo)
	require.NotNil(t, p)
	assert.False(t, p.Allowed, "owner/pro/investor_memo must be blocked (Enterprise-only)")
}

func TestInvestorMemo_UserAllTiers_Blocked(t *testing.T) {
	lkp := proLookup()
	for _, tier := range model.AllAISubscriptionTiers() {
		p := lkp.GetPolicy(model.AIRoleUser, tier, model.AIFeatureInvestorMemo)
		require.NotNil(t, p)
		assert.False(t, p.Allowed,
			"user/%s/investor_memo must be blocked (owner-only feature)", tier)
	}
}

func TestInvestorMemo_ViewerAllTiers_Blocked(t *testing.T) {
	lkp := proLookup()
	for _, tier := range model.AllAISubscriptionTiers() {
		p := lkp.GetPolicy(model.AIRoleViewer, tier, model.AIFeatureInvestorMemo)
		require.NotNil(t, p)
		assert.False(t, p.Allowed,
			"viewer/%s/investor_memo must be blocked", tier)
	}
}

// ============================================================================
// AllAIFeatures — completeness check
// ============================================================================

func TestAllAIFeatures_Contains13Features(t *testing.T) {
	features := model.AllAIFeatures()
	assert.Len(t, features, 13, "AllAIFeatures must return all 13 features (5 standard + 7 Pro + 1 Enterprise)")
}

func TestAllAIFeatures_ContainsAllProFeatures(t *testing.T) {
	featureSet := make(map[model.AIFeatureType]bool)
	for _, f := range model.AllAIFeatures() {
		featureSet[f] = true
	}
	for _, feat := range proDriverFeatureList() {
		assert.True(t, featureSet[feat], "AllAIFeatures must include Pro feature %s", feat)
	}
	assert.True(t, featureSet[model.AIFeatureInvestorMemo],
		"AllAIFeatures must include AIFeatureInvestorMemo")
}

// ============================================================================
// Priority values
// ============================================================================

func TestProDriverFeatures_PriorityValues(t *testing.T) {
	lkp := proLookup()
	feat := model.AIFeatureUnitEconomics

	ownerPro := lkp.GetPolicy(model.AIRoleOwner, model.AITierPro, feat)
	ownerEnt := lkp.GetPolicy(model.AIRoleOwner, model.AITierEnterprise, feat)
	require.NotNil(t, ownerPro)
	require.NotNil(t, ownerEnt)

	assert.Equal(t, 15, ownerPro.Priority, "owner/pro priority must be 15")
	assert.Equal(t, 20, ownerEnt.Priority, "owner/enterprise priority must be 20")
}

// ============================================================================
// Admin has full access to all 13 features (including Pro + Enterprise)
// ============================================================================

func TestAdmin_HasAccessToAllProAndEnterpriseFeatures(t *testing.T) {
	lkp := proLookup()
	for _, feat := range model.AllAIFeatures() {
		for _, tier := range model.AllAISubscriptionTiers() {
			p := lkp.GetPolicy(model.AIRoleAdmin, tier, feat)
			require.NotNil(t, p, "admin/%s/%s policy must exist", tier, feat)
			assert.True(t, p.Allowed, "admin must have access to %s/%s", tier, feat)
			assert.Nil(t, p.DailyLimit, "admin must have no limit on %s/%s", tier, feat)
		}
	}
}

// ============================================================================
// NoDuplicates — policy matrix has no duplicate (role, tier, feature) tuples
// ============================================================================

func TestDefaultAIUsagePolicies_NoDuplicateEntries(t *testing.T) {
	policies := model.DefaultAIUsagePolicies()
	seen := make(map[string]bool)
	for _, p := range policies {
		key := string(p.UserRole) + "|" + string(p.SubscriptionTier) + "|" + string(p.FeatureType)
		assert.False(t, seen[key],
			"duplicate policy entry found for role=%s tier=%s feature=%s",
			p.UserRole, p.SubscriptionTier, p.FeatureType)
		seen[key] = true
	}
}

// ============================================================================
// Helpers
// ============================================================================

// proDriverFeatureList returns the 7 Pro driver-aware feature types.
func proDriverFeatureList() []model.AIFeatureType {
	return []model.AIFeatureType{
		model.AIFeatureUnitEconomics,
		model.AIFeatureAssumptionReview,
		model.AIFeatureBenchmarkCommentary,
		model.AIFeaturePortfolioMix,
		model.AIFeatureDriverAdvisor,
		model.AIFeatureScenarioSuggestion,
		model.AIFeatureSensitivityNarrative,
	}
}
