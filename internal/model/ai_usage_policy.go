// Package model — AI usage control layer.
// Manages AI feature access based on user role (and future subscription tier).
// Ported from GPWA internal/model/ai_usage_policy.go and adapted for Ascenda.
package model

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// AI Feature Types — Ascenda financial intelligence features
// ============================================================================

// AIFeatureType identifies an AI-powered feature.
type AIFeatureType string

const (
	// AIFeaturePlanNarration — Natural language narrative of a plan/scenario.
	AIFeaturePlanNarration AIFeatureType = "AI_PLAN_NARRATION"

	// AIFeatureVarianceAnalysis — Explain budget vs actuals variances.
	AIFeatureVarianceAnalysis AIFeatureType = "AI_VARIANCE_ANALYSIS"

	// AIFeatureScenarioComparison — Compare two or more scenarios.
	AIFeatureScenarioComparison AIFeatureType = "AI_SCENARIO_COMPARISON"

	// AIFeatureAnomalyDetection — Flag unusual values in financial data.
	AIFeatureAnomalyDetection AIFeatureType = "AI_ANOMALY_DETECTION"

	// AIFeatureCashRunway — Narrative of cash runway and liquidity outlook.
	AIFeatureCashRunway AIFeatureType = "AI_CASH_RUNWAY"

	// ── Pro-tier features — driver-aware intelligence ─────────────────────────

	// AIFeatureUnitEconomics — Driver-specific unit-level KPI narration.
	// Computes ARPU, revenue-per-FTE, take-rate contribution, eCPM, etc.
	AIFeatureUnitEconomics AIFeatureType = "AI_UNIT_ECONOMICS"

	// AIFeatureAssumptionReview — Driver-aware red-flag detection on plan inputs.
	// Flags physically or commercially implausible assumptions per driver type.
	AIFeatureAssumptionReview AIFeatureType = "AI_ASSUMPTION_REVIEW"

	// AIFeatureBenchmarkCommentary — Positions plan KPIs against industry benchmarks.
	// Benchmarks are injected per driver type (SaaS, consulting, marketplace, etc.).
	AIFeatureBenchmarkCommentary AIFeatureType = "AI_BENCHMARK_COMMENTARY"

	// AIFeaturePortfolioMix — Multi-driver product portfolio commentary.
	// Narrates revenue-mix evolution and margin contribution by segment.
	AIFeaturePortfolioMix AIFeatureType = "AI_PORTFOLIO_MIX"

	// AIFeatureDriverAdvisor — Suggests a structured driver for generic products.
	// Analyses stored assumptions and recommends the best-fit driver type.
	AIFeatureDriverAdvisor AIFeatureType = "AI_DRIVER_ADVISOR"

	// AIFeatureScenarioSuggestion — Generates bear / bull / stress driverParam diffs.
	// Returns structured parameter modifications with per-change rationale.
	AIFeatureScenarioSuggestion AIFeatureType = "AI_SCENARIO_SUGGESTION"

	// AIFeatureSensitivityNarrative — Driver-aware sensitivity lever narration.
	// Identifies the top levers and quantifies their impact on revenue and EBITDA.
	AIFeatureSensitivityNarrative AIFeatureType = "AI_SENSITIVITY_NARRATIVE"

	// AIFeatureInvestorMemo — Full investor-ready plan section (Enterprise).
	// Covers business model, unit economics, projections, risks. Higher token budget.
	AIFeatureInvestorMemo AIFeatureType = "AI_INVESTOR_MEMO"
)

// AIFeatureCost represents the resource-cost tier of a feature.
type AIFeatureCost string

const (
	AIFeatureCostFree   AIFeatureCost = "free"
	AIFeatureCostLow    AIFeatureCost = "low"
	AIFeatureCostMedium AIFeatureCost = "medium"
	AIFeatureCostHigh   AIFeatureCost = "high"
)

// AIAccessDeniedReason indicates why access was denied.
type AIAccessDeniedReason string

const (
	AIAccessDeniedFeatureDisabled     AIAccessDeniedReason = "feature_disabled"
	AIAccessDeniedRoleNotAllowed      AIAccessDeniedReason = "role_not_allowed"
	AIAccessDeniedTierNotAllowed      AIAccessDeniedReason = "tier_not_allowed"
	AIAccessDeniedDailyLimitReached   AIAccessDeniedReason = "daily_limit_reached"
	AIAccessDeniedWeeklyLimitReached  AIAccessDeniedReason = "weekly_limit_reached"
	AIAccessDeniedMonthlyLimitReached AIAccessDeniedReason = "monthly_limit_reached"
	AIAccessDeniedMaintenanceMode     AIAccessDeniedReason = "maintenance_mode"
)

// ============================================================================
// AI Feature Metadata
// ============================================================================

// AIFeatureMetadata describes an AI feature's characteristics.
type AIFeatureMetadata struct {
	FeatureType AIFeatureType `json:"feature_type"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	CostLevel   AIFeatureCost `json:"cost_level"`
	RequiresLLM bool          `json:"requires_llm"`
}

// GetAIFeatureMetadata returns metadata for all Ascenda AI features.
func GetAIFeatureMetadata() map[AIFeatureType]AIFeatureMetadata {
	return map[AIFeatureType]AIFeatureMetadata{
		// ── Standard features ────────────────────────────────────────────────
		AIFeaturePlanNarration: {
			FeatureType: AIFeaturePlanNarration,
			Name:        "Plan Narration",
			Description: "Natural language narrative of a business plan scenario",
			CostLevel:   AIFeatureCostMedium,
			RequiresLLM: true,
		},
		AIFeatureVarianceAnalysis: {
			FeatureType: AIFeatureVarianceAnalysis,
			Name:        "Variance Analysis",
			Description: "AI-powered explanation of budget vs actuals variances",
			CostLevel:   AIFeatureCostMedium,
			RequiresLLM: true,
		},
		AIFeatureScenarioComparison: {
			FeatureType: AIFeatureScenarioComparison,
			Name:        "Scenario Comparison",
			Description: "Side-by-side AI analysis of two or more scenarios",
			CostLevel:   AIFeatureCostHigh,
			RequiresLLM: true,
		},
		AIFeatureAnomalyDetection: {
			FeatureType: AIFeatureAnomalyDetection,
			Name:        "Anomaly Detection",
			Description: "AI-powered flagging of unusual values in financial data",
			CostLevel:   AIFeatureCostMedium,
			RequiresLLM: true,
		},
		AIFeatureCashRunway: {
			FeatureType: AIFeatureCashRunway,
			Name:        "Cash Runway Narrative",
			Description: "Natural language liquidity and cash runway outlook",
			CostLevel:   AIFeatureCostLow,
			RequiresLLM: true,
		},
		// ── Pro-tier features ─────────────────────────────────────────────────
		AIFeatureUnitEconomics: {
			FeatureType: AIFeatureUnitEconomics,
			Name:        "Unit Economics",
			Description: "Driver-specific unit-level KPI narration (ARPU, revenue/FTE, eCPM, etc.)",
			CostLevel:   AIFeatureCostMedium,
			RequiresLLM: true,
		},
		AIFeatureAssumptionReview: {
			FeatureType: AIFeatureAssumptionReview,
			Name:        "Assumption Review",
			Description: "Driver-aware red-flag detection on plan inputs",
			CostLevel:   AIFeatureCostMedium,
			RequiresLLM: true,
		},
		AIFeatureBenchmarkCommentary: {
			FeatureType: AIFeatureBenchmarkCommentary,
			Name:        "Benchmark Commentary",
			Description: "Positions plan KPIs against driver-specific industry benchmarks",
			CostLevel:   AIFeatureCostMedium,
			RequiresLLM: true,
		},
		AIFeaturePortfolioMix: {
			FeatureType: AIFeaturePortfolioMix,
			Name:        "Portfolio Mix",
			Description: "Multi-driver product portfolio revenue mix and margin commentary",
			CostLevel:   AIFeatureCostMedium,
			RequiresLLM: true,
		},
		AIFeatureDriverAdvisor: {
			FeatureType: AIFeatureDriverAdvisor,
			Name:        "Driver Advisor",
			Description: "Suggests the best-fit business driver for generic products",
			CostLevel:   AIFeatureCostLow,
			RequiresLLM: true,
		},
		AIFeatureScenarioSuggestion: {
			FeatureType: AIFeatureScenarioSuggestion,
			Name:        "Scenario Suggestion",
			Description: "Generates bear / bull / stress driverParam diffs with rationale",
			CostLevel:   AIFeatureCostHigh,
			RequiresLLM: true,
		},
		AIFeatureSensitivityNarrative: {
			FeatureType: AIFeatureSensitivityNarrative,
			Name:        "Sensitivity Narrative",
			Description: "Identifies top driver-specific levers and their P&L impact",
			CostLevel:   AIFeatureCostMedium,
			RequiresLLM: true,
		},
		// ── Enterprise feature ────────────────────────────────────────────────
		AIFeatureInvestorMemo: {
			FeatureType: AIFeatureInvestorMemo,
			Name:        "Investor Memo",
			Description: "Full investor-ready plan narrative: model, unit economics, projections, risks",
			CostLevel:   AIFeatureCostHigh,
			RequiresLLM: true,
		},
	}
}

// ============================================================================
// AI Usage Policy — access matrix (role × tier × feature)
// ============================================================================

// AIUsagePolicy defines access rules for an AI feature by role and subscription tier.
// TenantID is stored as a UUID string to remain consistent with TenantScoped.
type AIUsagePolicy struct {
	TenantScoped

	// Policy identification
	UserRole         string        `json:"user_role" gorm:"type:varchar(20);index;not null"`          // admin, owner, user, viewer
	SubscriptionTier string        `json:"subscription_tier" gorm:"type:varchar(20);index;not null"`  // standard, pro, enterprise
	FeatureType      AIFeatureType `json:"feature_type" gorm:"type:varchar(40);index;not null"`

	// Access control
	Allowed bool `json:"allowed" gorm:"default:false"`

	// Usage limits (nil = unlimited)
	DailyLimit   *int `json:"daily_limit,omitempty"`
	WeeklyLimit  *int `json:"weekly_limit,omitempty"`
	MonthlyLimit *int `json:"monthly_limit,omitempty"`

	// Priority and cost management
	Priority       int     `json:"priority" gorm:"default:0"`
	CostMultiplier float64 `json:"cost_multiplier" gorm:"default:1.0"`
}

// TableName specifies the table name for GORM.
func (AIUsagePolicy) TableName() string {
	return "ai_usage_policies"
}

// ============================================================================
// AI Usage Record — per-request audit trail
// ============================================================================

// AIUsageRecord tracks individual AI feature usage for quota enforcement and billing.
type AIUsageRecord struct {
	TenantScoped

	UserID      uuid.UUID     `json:"user_id" gorm:"type:uuid;index;not null"`
	FeatureType AIFeatureType `json:"feature_type" gorm:"type:varchar(40);index;not null"`
	UsedAt      time.Time     `json:"used_at" gorm:"index;not null"`
	Success     bool          `json:"success" gorm:"default:true"`

	// User context (for analytics)
	UserRole         string `json:"user_role" gorm:"type:varchar(20);index"`
	SubscriptionTier string `json:"subscription_tier" gorm:"type:varchar(20);index"`

	// Request context
	RequestID  string `json:"request_id,omitempty" gorm:"type:varchar(50)"`
	DurationMs int    `json:"duration_ms,omitempty"`
	TokensUsed int    `json:"tokens_used,omitempty"`
	ErrorMsg   string `json:"error_msg,omitempty" gorm:"type:text"`

	// Cost tracking
	EstimatedCostCents float64 `json:"estimated_cost_cents,omitempty"`

	// Aggregation keys for efficient quota queries
	DailyKey   string `json:"daily_key" gorm:"type:varchar(10);index;not null"`   // "2026-01-29"
	WeeklyKey  string `json:"weekly_key" gorm:"type:varchar(10);index;not null"`  // "2026-W05"
	MonthlyKey string `json:"monthly_key" gorm:"type:varchar(7);index;not null"`  // "2026-01"
}

// TableName specifies the table name for GORM.
func (AIUsageRecord) TableName() string {
	return "ai_usage_records"
}

// NewAIUsageRecord creates a usage record with proper aggregation keys.
func NewAIUsageRecord(tenantID uuid.UUID, userID uuid.UUID, feature AIFeatureType, userRole, subscriptionTier string) *AIUsageRecord {
	now := time.Now()
	year, week := now.ISOWeek()

	return &AIUsageRecord{
		TenantScoped:     TenantScoped{TenantID: tenantID},
		UserID:           userID,
		FeatureType:      feature,
		UsedAt:           now,
		Success:          true,
		UserRole:         userRole,
		SubscriptionTier: subscriptionTier,
		DailyKey:         now.Format("2006-01-02"),
		WeeklyKey:        aiFormatWeekKey(year, week),
		MonthlyKey:       now.Format("2006-01"),
	}
}

// aiFormatWeekKey formats year and week number into a key (e.g. "2026-W05").
func aiFormatWeekKey(year, week int) string {
	return fmt.Sprintf("%d-W%02d", year, week)
}

// ============================================================================
// AI Access Result
// ============================================================================

// AIAccessResult contains the outcome of an access check.
type AIAccessResult struct {
	Allowed       bool                 `json:"allowed"`
	DeniedReason  AIAccessDeniedReason `json:"denied_reason,omitempty"`
	DeniedMessage string               `json:"denied_message,omitempty"`

	DailyUsed    int  `json:"daily_used"`
	DailyLimit   *int `json:"daily_limit,omitempty"`
	WeeklyUsed   int  `json:"weekly_used"`
	WeeklyLimit  *int `json:"weekly_limit,omitempty"`
	MonthlyUsed  int  `json:"monthly_used"`
	MonthlyLimit *int `json:"monthly_limit,omitempty"`

	FeatureType AIFeatureType `json:"feature_type"`
	UserRole    string        `json:"user_role"`
	Tier        string        `json:"tier"`
}

// ============================================================================
// AI Usage Stats
// ============================================================================

// AIUsageStats provides aggregated usage statistics.
type AIUsageStats struct {
	TenantID    uuid.UUID `json:"tenant_id"`
	UserID      uuid.UUID `json:"user_id,omitempty"`
	Period      string    `json:"period"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`

	UsageByFeature  map[AIFeatureType]int `json:"usage_by_feature"`
	TotalCalls      int                   `json:"total_calls"`
	SuccessfulCalls int                   `json:"successful_calls"`
	FailedCalls     int                   `json:"failed_calls"`
	TotalTokens     int                   `json:"total_tokens"`
	EstimatedCostCents float64            `json:"estimated_cost_cents"`
}

// ============================================================================
// Policy Lookup Cache
// ============================================================================

// AIUsagePolicyLookup provides O(1) policy lookup from a slice of policies.
type AIUsagePolicyLookup struct {
	policies map[string]*AIUsagePolicy // key: "role:tier:feature"
}

// NewAIUsagePolicyLookup builds a lookup index from a slice of policies.
func NewAIUsagePolicyLookup(policies []AIUsagePolicy) *AIUsagePolicyLookup {
	l := &AIUsagePolicyLookup{
		policies: make(map[string]*AIUsagePolicy, len(policies)),
	}
	for i := range policies {
		key := policies[i].UserRole + ":" + policies[i].SubscriptionTier + ":" + string(policies[i].FeatureType)
		l.policies[key] = &policies[i]
	}
	return l
}

// GetPolicy retrieves a policy for the given role, tier, and feature.
func (l *AIUsagePolicyLookup) GetPolicy(role, tier string, feature AIFeatureType) *AIUsagePolicy {
	return l.policies[role+":"+tier+":"+string(feature)]
}

// ============================================================================
// Default Policies — access matrix seed
// ============================================================================

// Ascenda roles (Socrate app roles + plan membership roles).
const (
	AIRoleAdmin  = "admin"  // Socrate platform admin
	AIRoleOwner  = "owner"  // Plan owner (plan membership)
	AIRoleUser   = "user"   // Plan member (plan membership)
	AIRoleViewer = "viewer" // Read-only plan member
)

// Ascenda subscription tiers (infrastructure ready for billing).
const (
	AITierFreemium  = "freemium" // No AI access — base tier before any paid plan.
	AITierStandard  = "standard"
	AITierPro       = "pro"
	AITierEnterprise = "enterprise"
)

// AllAIRoles returns all supported Ascenda roles for AI access.
func AllAIRoles() []string {
	return []string{AIRoleAdmin, AIRoleOwner, AIRoleUser, AIRoleViewer}
}

// AllAISubscriptionTiers returns all supported subscription tiers.
func AllAISubscriptionTiers() []string {
	return []string{AITierFreemium, AITierStandard, AITierPro, AITierEnterprise}
}

// AllAIFeatures returns all Ascenda AI features (standard + pro + enterprise).
func AllAIFeatures() []AIFeatureType {
	return []AIFeatureType{
		// Standard features
		AIFeaturePlanNarration,
		AIFeatureVarianceAnalysis,
		AIFeatureScenarioComparison,
		AIFeatureAnomalyDetection,
		AIFeatureCashRunway,
		// Pro-tier features
		AIFeatureUnitEconomics,
		AIFeatureAssumptionReview,
		AIFeatureBenchmarkCommentary,
		AIFeaturePortfolioMix,
		AIFeatureDriverAdvisor,
		AIFeatureScenarioSuggestion,
		AIFeatureSensitivityNarrative,
		// Enterprise feature
		AIFeatureInvestorMemo,
	}
}

// DefaultAIUsagePolicies returns the default role × tier × feature access matrix.
// This is used to seed the database and as the in-memory fallback.
//
// Policy summary:
//   - admin:       full access on all tiers (no limits)
//   - owner:       full access on standard, higher limits on pro/enterprise
//   - user:        read narration features on all tiers, anomaly/comparison only on pro+
//   - viewer:      plan narration only (read-only insight), never editing features
func DefaultAIUsagePolicies() []AIUsagePolicy {
	policies := []AIUsagePolicy{}

	add := func(role, tier string, feature AIFeatureType, allowed bool, daily, weekly, monthly *int, priority int) {
		policies = append(policies, AIUsagePolicy{
			UserRole:         role,
			SubscriptionTier: tier,
			FeatureType:      feature,
			Allowed:          allowed,
			DailyLimit:       daily,
			WeeklyLimit:      weekly,
			MonthlyLimit:     monthly,
			Priority:         priority,
			CostMultiplier:   1.0,
		})
	}

	ptr := func(i int) *int { return &i }

	// standardFeatures are the original 5 features available on all tiers.
	// The Pro/Enterprise extended features are seeded separately below so that
	// the enterprise-owner loop here does not create duplicate entries.
	standardFeatures := []AIFeatureType{
		AIFeaturePlanNarration,
		AIFeatureVarianceAnalysis,
		AIFeatureScenarioComparison,
		AIFeatureAnomalyDetection,
		AIFeatureCashRunway,
	}

	// ── Freemium: all AI features blocked for non-admin roles ────────────────
	// The freemium plan includes no AI access. Users must upgrade to at least
	// the standard tier to use any AI feature. Admin is excluded here because
	// it has unrestricted access on all tiers (handled by the admin block below).
	for _, role := range []string{AIRoleOwner, AIRoleUser, AIRoleViewer} {
		for _, feat := range AllAIFeatures() {
			add(role, AITierFreemium, feat, false, nil, nil, nil, 0)
		}
	}

	// ── Admin: unlimited access on ALL features and ALL tiers ─────────────────
	// Admins bypass tier restrictions so they can manage the platform on any plan.
	for _, tier := range AllAISubscriptionTiers() {
		for _, feat := range AllAIFeatures() {
			add(AIRoleAdmin, tier, feat, true, nil, nil, nil, 100)
		}
	}

	// ── Owner: full access; limits tighten on standard tier ──────────────────
	for _, tier := range AllAISubscriptionTiers() {
		switch tier {
		case AITierStandard:
			add(AIRoleOwner, tier, AIFeaturePlanNarration,     true, ptr(10), ptr(50), nil, 10)
			add(AIRoleOwner, tier, AIFeatureVarianceAnalysis,  true, ptr(5),  ptr(25), nil, 10)
			add(AIRoleOwner, tier, AIFeatureScenarioComparison, false, nil, nil, nil, 10)
			add(AIRoleOwner, tier, AIFeatureAnomalyDetection,  false, nil, nil, nil, 10)
			add(AIRoleOwner, tier, AIFeatureCashRunway,        true, ptr(10), ptr(50), nil, 10)
		case AITierPro:
			add(AIRoleOwner, tier, AIFeaturePlanNarration,     true, nil, nil, nil, 15)
			add(AIRoleOwner, tier, AIFeatureVarianceAnalysis,  true, nil, nil, nil, 15)
			add(AIRoleOwner, tier, AIFeatureScenarioComparison, true, ptr(10), ptr(40), nil, 15)
			add(AIRoleOwner, tier, AIFeatureAnomalyDetection,  true, ptr(5),  ptr(20), nil, 15)
			add(AIRoleOwner, tier, AIFeatureCashRunway,        true, nil, nil, nil, 15)
		case AITierEnterprise:
			// Only the 5 standard features here; Pro/Enterprise extended features
			// are handled by the proDriverFeatures and InvestorMemo blocks below.
			for _, feat := range standardFeatures {
				add(AIRoleOwner, tier, feat, true, nil, nil, nil, 20)
			}
		}
	}

	// ── User: narration and cash runway always; analysis features on pro+ ────
	for _, tier := range AllAISubscriptionTiers() {
		switch tier {
		case AITierStandard:
			add(AIRoleUser, tier, AIFeaturePlanNarration,      true, ptr(5), ptr(20), nil, 5)
			add(AIRoleUser, tier, AIFeatureVarianceAnalysis,   false, nil, nil, nil, 5)
			add(AIRoleUser, tier, AIFeatureScenarioComparison, false, nil, nil, nil, 5)
			add(AIRoleUser, tier, AIFeatureAnomalyDetection,   false, nil, nil, nil, 5)
			add(AIRoleUser, tier, AIFeatureCashRunway,         true, ptr(5), ptr(20), nil, 5)
		case AITierPro:
			add(AIRoleUser, tier, AIFeaturePlanNarration,      true, ptr(10), ptr(40), nil, 8)
			add(AIRoleUser, tier, AIFeatureVarianceAnalysis,   true, ptr(5),  ptr(20), nil, 8)
			add(AIRoleUser, tier, AIFeatureScenarioComparison, false, nil, nil, nil, 8)
			add(AIRoleUser, tier, AIFeatureAnomalyDetection,   true, ptr(3),  ptr(15), nil, 8)
			add(AIRoleUser, tier, AIFeatureCashRunway,         true, ptr(10), ptr(40), nil, 8)
		case AITierEnterprise:
			add(AIRoleUser, tier, AIFeaturePlanNarration,      true, nil, nil, nil, 10)
			add(AIRoleUser, tier, AIFeatureVarianceAnalysis,   true, nil, nil, nil, 10)
			add(AIRoleUser, tier, AIFeatureScenarioComparison, true, ptr(10), ptr(40), nil, 10)
			add(AIRoleUser, tier, AIFeatureAnomalyDetection,   true, nil, nil, nil, 10)
			add(AIRoleUser, tier, AIFeatureCashRunway,         true, nil, nil, nil, 10)
		}
	}

	// ── Viewer: plan narration only (read-only insight) ──────────────────────
	// Freemium viewer entries are handled by the freemium block above.
	for _, tier := range []string{AITierStandard, AITierPro, AITierEnterprise} {
		add(AIRoleViewer, tier, AIFeaturePlanNarration,      true, ptr(3), ptr(10), nil, 2)
		add(AIRoleViewer, tier, AIFeatureVarianceAnalysis,   false, nil, nil, nil, 2)
		add(AIRoleViewer, tier, AIFeatureScenarioComparison, false, nil, nil, nil, 2)
		add(AIRoleViewer, tier, AIFeatureAnomalyDetection,   false, nil, nil, nil, 2)
		add(AIRoleViewer, tier, AIFeatureCashRunway,         false, nil, nil, nil, 2)
	}

	// ── Pro-tier driver-aware features (7) ───────────────────────────────────
	//   Standard: blocked for all non-admin roles.
	//   Pro:      owner 5/day 20/week; user 3/day 15/week; viewer blocked.
	//   Enterprise: unlimited; viewer blocked.
	proDriverFeatures := []AIFeatureType{
		AIFeatureUnitEconomics,
		AIFeatureAssumptionReview,
		AIFeatureBenchmarkCommentary,
		AIFeaturePortfolioMix,
		AIFeatureDriverAdvisor,
		AIFeatureScenarioSuggestion,
		AIFeatureSensitivityNarrative,
	}
	for _, feat := range proDriverFeatures {
		add(AIRoleOwner,  AITierStandard,   feat, false, nil,     nil,     nil, 10)
		add(AIRoleUser,   AITierStandard,   feat, false, nil,     nil,     nil, 5)
		add(AIRoleViewer, AITierStandard,   feat, false, nil,     nil,     nil, 2)

		add(AIRoleOwner,  AITierPro,        feat, true,  ptr(5),  ptr(20), nil, 15)
		add(AIRoleUser,   AITierPro,        feat, true,  ptr(3),  ptr(15), nil, 8)
		add(AIRoleViewer, AITierPro,        feat, false, nil,     nil,     nil, 2)

		add(AIRoleOwner,  AITierEnterprise, feat, true,  nil,     nil,     nil, 20)
		add(AIRoleUser,   AITierEnterprise, feat, true,  nil,     nil,     nil, 10)
		add(AIRoleViewer, AITierEnterprise, feat, false, nil,     nil,     nil, 2)
	}

	// ── InvestorMemo: Enterprise, plan owner only ─────────────────────────────
	//   Standard/Pro: blocked for all roles.
	//   Enterprise:   owner unlimited; user and viewer blocked.
	add(AIRoleOwner,  AITierStandard,   AIFeatureInvestorMemo, false, nil, nil, nil, 10)
	add(AIRoleOwner,  AITierPro,        AIFeatureInvestorMemo, false, nil, nil, nil, 10)
	add(AIRoleOwner,  AITierEnterprise, AIFeatureInvestorMemo, true,  nil, nil, nil, 20)
	// Freemium entries for user/viewer are handled by the freemium block above.
	for _, tier := range []string{AITierStandard, AITierPro, AITierEnterprise} {
		add(AIRoleUser,   tier, AIFeatureInvestorMemo, false, nil, nil, nil, 5)
		add(AIRoleViewer, tier, AIFeatureInvestorMemo, false, nil, nil, nil, 2)
	}

	return policies
}

// ============================================================================
// Denied Message Helper
// ============================================================================

// GetDeniedMessage returns a user-friendly message for the denial reason.
func GetDeniedMessage(reason AIAccessDeniedReason, feature AIFeatureType) string {
	meta := GetAIFeatureMetadata()[feature]
	featureName := string(feature)
	if meta.Name != "" {
		featureName = meta.Name
	}

	switch reason {
	case AIAccessDeniedFeatureDisabled:
		return featureName + " is currently disabled"
	case AIAccessDeniedRoleNotAllowed:
		return "Your role does not have access to " + featureName
	case AIAccessDeniedTierNotAllowed:
		return "Upgrade your subscription to access " + featureName
	case AIAccessDeniedDailyLimitReached:
		return "Daily limit reached for " + featureName + ". Try again tomorrow"
	case AIAccessDeniedWeeklyLimitReached:
		return "Weekly limit reached for " + featureName + ". Try again next week"
	case AIAccessDeniedMonthlyLimitReached:
		return "Monthly limit reached for " + featureName
	case AIAccessDeniedMaintenanceMode:
		return featureName + " is temporarily unavailable for maintenance"
	default:
		return "Access denied to " + featureName
	}
}
