package model

import (
	"encoding/json"
	"time"
)

// FeatureType distinguishes how a policy rule is evaluated.
type FeatureType string

const (
	// FeatureTypeAccess gates a boolean yes/no feature per tier.
	FeatureTypeAccess FeatureType = "access"
	// FeatureTypeNumericLimit gates a countable resource with a per-tier ceiling.
	FeatureTypeNumericLimit FeatureType = "numeric_limit"
)

// FeaturePolicy stores the access rules for a named platform feature across all
// commercial tiers.  One row per feature; the primary key is the feature slug.
//
// The Freemium / Pro / Enterprise columns hold small JSONB objects whose shape
// depends on FeatureType:
//
//	access:        {"allowed": true}  or {"allowed": false}
//	numeric_limit: {"limit": 3}       or {"limit": -1}  (-1 = unlimited)
type FeaturePolicy struct {
	Feature     string          `gorm:"primaryKey"                json:"feature"`
	Category    string          `gorm:"not null;index"            json:"category"`
	Label       string          `gorm:"not null"                  json:"label"`
	FeatureType FeatureType     `gorm:"column:feature_type;not null" json:"featureType"`
	Freemium    json.RawMessage `gorm:"type:jsonb;not null"       json:"freemium"`
	Pro         json.RawMessage `gorm:"type:jsonb;not null"       json:"pro"`
	Enterprise  json.RawMessage `gorm:"type:jsonb;not null"       json:"enterprise"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// TierRule is the unmarshalled form of one per-tier policy cell.
type TierRule struct {
	// Allowed is populated for FeatureTypeAccess rows.
	Allowed *bool `json:"allowed,omitempty"`
	// Limit is populated for FeatureTypeNumericLimit rows.
	// -1 means unlimited.
	Limit *int `json:"limit,omitempty"`
}

// MarshalAccess encodes a boolean access rule as JSON.
func MarshalAccess(allowed bool) json.RawMessage {
	b, _ := json.Marshal(TierRule{Allowed: &allowed})
	return b
}

// MarshalLimit encodes a numeric limit rule as JSON.
func MarshalLimit(n int) json.RawMessage {
	b, _ := json.Marshal(TierRule{Limit: &n})
	return b
}

// ──────────────────────────────────────────────────────────────────────────────
// Well-known feature slugs
// ──────────────────────────────────────────────────────────────────────────────

const (
	// Plans / scenarios
	FeatureMaxPlans        = "max_plans"
	FeatureMaxScenarios    = "max_scenarios"
	FeaturePlanSharing     = "plan_sharing"

	// AI features
	FeatureAIPlanNarration       = "ai_plan_narration"
	FeatureAIUnitEconomics       = "ai_unit_economics"
	FeatureAIAssumptionReview    = "ai_assumption_review"
	FeatureAIBenchmarkCommentary = "ai_benchmark_commentary"
	FeatureAIPortfolioMix        = "ai_portfolio_mix"
	FeatureAIDriverAdvisor       = "ai_driver_advisor"
	FeatureAIScenarioSuggestion  = "ai_scenario_suggestion"
	FeatureAISensitivityNarrative = "ai_sensitivity_narrative"
	FeatureAIInvestorMemo        = "ai_investor_memo"

	// Advanced modules
	FeatureBreakEven  = "break_even"
	FeatureCapTable   = "cap_table"
)

// DefaultFeaturePolicies returns the seed rows that represent the platform's
// baseline rules.  These are inserted with ON CONFLICT DO NOTHING so that
// admin overrides made at runtime are never overwritten on restart.
func DefaultFeaturePolicies() []FeaturePolicy {
	yes  := MarshalAccess(true)
	no   := MarshalAccess(false)
	lim1 := MarshalLimit(1)
	lim3 := MarshalLimit(3)
	unlimited := MarshalLimit(-1)

	return []FeaturePolicy{
		// ── Plans / scenarios ────────────────────────────────────────────────
		{
			Feature: FeatureMaxPlans, Category: "plans",
			Label: "Business Plan count", FeatureType: FeatureTypeNumericLimit,
			Freemium: lim1, Pro: lim3, Enterprise: unlimited,
		},
		{
			Feature: FeatureMaxScenarios, Category: "scenarios",
			Label: "Scenarios per plan", FeatureType: FeatureTypeNumericLimit,
			Freemium: lim1, Pro: lim3, Enterprise: unlimited,
		},
		{
			Feature: FeaturePlanSharing, Category: "plans",
			Label: "Plan sharing / collaborators", FeatureType: FeatureTypeAccess,
			Freemium: no, Pro: yes, Enterprise: yes,
		},

		// ── AI — narration (freemium blocked) ────────────────────────────────
		{
			Feature: FeatureAIPlanNarration, Category: "ai",
			Label: "Plan Narration", FeatureType: FeatureTypeAccess,
			Freemium: no, Pro: yes, Enterprise: yes,
		},

		// ── AI — pro tier ────────────────────────────────────────────────────
		{
			Feature: FeatureAIUnitEconomics, Category: "ai",
			Label: "Unit Economics", FeatureType: FeatureTypeAccess,
			Freemium: no, Pro: yes, Enterprise: yes,
		},
		{
			Feature: FeatureAIAssumptionReview, Category: "ai",
			Label: "Assumption Review", FeatureType: FeatureTypeAccess,
			Freemium: no, Pro: yes, Enterprise: yes,
		},
		{
			Feature: FeatureAIBenchmarkCommentary, Category: "ai",
			Label: "Benchmark Commentary", FeatureType: FeatureTypeAccess,
			Freemium: no, Pro: yes, Enterprise: yes,
		},
		{
			Feature: FeatureAIPortfolioMix, Category: "ai",
			Label: "Portfolio Mix", FeatureType: FeatureTypeAccess,
			Freemium: no, Pro: yes, Enterprise: yes,
		},
		{
			Feature: FeatureAIDriverAdvisor, Category: "ai",
			Label: "Driver Advisor", FeatureType: FeatureTypeAccess,
			Freemium: no, Pro: yes, Enterprise: yes,
		},
		{
			Feature: FeatureAIScenarioSuggestion, Category: "ai",
			Label: "Scenario Suggestion", FeatureType: FeatureTypeAccess,
			Freemium: no, Pro: yes, Enterprise: yes,
		},
		{
			Feature: FeatureAISensitivityNarrative, Category: "ai",
			Label: "Sensitivity Narrative", FeatureType: FeatureTypeAccess,
			Freemium: no, Pro: yes, Enterprise: yes,
		},

		// ── AI — enterprise only ─────────────────────────────────────────────
		{
			Feature: FeatureAIInvestorMemo, Category: "ai",
			Label: "Investor Memo", FeatureType: FeatureTypeAccess,
			Freemium: no, Pro: no, Enterprise: yes,
		},

		// ── Advanced modules ─────────────────────────────────────────────────
		{
			Feature: FeatureBreakEven, Category: "advanced",
			Label: "Break-Even Analysis", FeatureType: FeatureTypeAccess,
			Freemium: no, Pro: yes, Enterprise: yes,
		},
		{
			Feature: FeatureCapTable, Category: "advanced",
			Label: "Cap Table", FeatureType: FeatureTypeAccess,
			Freemium: no, Pro: yes, Enterprise: yes,
		},
	}
}
