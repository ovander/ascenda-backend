package service

// ScenarioAnalysisService — decision-intelligence layer for Ascenda.
//
// It sits between the HTTP handlers and the compute engine:
//
//	HTTP Handlers
//	     ↓
//	ScenarioAnalysisService   ← this file
//	     ↓
//	ReportService (compute + cache)
//	     ↓
//	PlanComputeOrchestrator / Compute Engine
//
// Design rule: this service NEVER modifies compute engine internals.
// It only reads FullPlanOutput and derives decision-oriented insights.

import (
	"context"
	"fmt"
	"math"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

// ──────────────────────────────────────────────────────────────────────────────
// Thresholds (v1 heuristics — promote to config when needed)
// ──────────────────────────────────────────────────────────────────────────────

const (
	// breakEvenRiskyMonths — break-even beyond this threshold is flagged medium risk.
	breakEvenRiskyMonths = 24

	// unrealisticGrowthThreshold — year-over-year revenue growth above 100 %
	// (i.e. doubling) is considered unrealistic for v1 heuristics.
	unrealisticGrowthThreshold = 100.0

	// highHiringRatio — payroll / revenue above this ratio triggers the
	// "hiring too aggressive" driver.
	highHiringRatio = 0.50

	// lowGrossMarginThreshold — gross margin below 20 % triggers the
	// "pricing too low" driver.
	lowGrossMarginThreshold = 0.20

	// highBurnRatio — operating losses exceeding 50 % of revenue in year 1
	// combined with slow growth triggers the "inefficient growth model" driver.
	highBurnRatio = 0.50

	// slowGrowthThreshold — year-over-year revenue growth below 20 % is
	// considered "slow" when combined with a high burn rate.
	slowGrowthThreshold = 0.20

	// viabilityStrong — minimum score for a "strong" classification.
	viabilityStrong = 80

	// viabilityModerate — minimum score for a "moderate" classification.
	viabilityModerate = 50

	// ── Viability scoring — cash health (max 30 pts) ─────────────────────────

	// minMeaningfulRevenue — revenue5Y below this value causes the cash-health
	// scorer to use absolute thresholds instead of a cash/revenue ratio.
	// Avoids ratio explosion in pre-revenue scenarios.
	minMeaningfulRevenue = 10_000.0

	// cashHealthCeiling — relative ratio (cashMin/rev5Y) at which the maximum
	// cash-health score is awarded (30 pts).
	cashHealthCeiling = 0.10

	// cashHealthFloor — relative ratio at which the cash-health score is zero.
	cashHealthFloor = -0.50

	// cashHealthAbsCeiling — absolute cashMin (currency) for max score when
	// revenue5Y < minMeaningfulRevenue.
	cashHealthAbsCeiling = 50_000.0

	// cashHealthAbsFloor — absolute cashMin (currency) for zero score when
	// revenue5Y < minMeaningfulRevenue.
	cashHealthAbsFloor = -200_000.0

	// ── Viability scoring — break-even (max 25 pts) ───────────────────────────

	// bepScoreHorizon — the month horizon over which BEP is scored.
	// bep=1 → 25 pts, bep=bepScoreHorizon → 0 pts.
	bepScoreHorizon = 60

	// ── Viability scoring — EBITDA margin (max 20 pts) ───────────────────────

	// ebitdaMarginBaseline — score contribution when year-5 EBITDA margin = 0.
	ebitdaMarginBaseline = 10.0

	// ebitdaMarginAnchor — year-5 EBITDA margin fraction that earns the
	// maximum score (20 pts).
	ebitdaMarginAnchor = 0.25

	// ── Viability scoring — burn control (max 10 pts) ────────────────────────

	// burnControlCeiling — burn ratio (-cashMin/rev5Y) at which burn score = 0.
	burnControlCeiling = 0.20

	// burnControlAbsFloor — absolute cashMin at which burn score = 0 in the
	// absolute branch (revenue < minMeaningfulRevenue).
	burnControlAbsFloor = -50_000.0

	// ── Risk summary — global risk level classification ───────────────────────

	// riskSummaryNone … riskSummaryCritical are the ordered tiers for
	// GlobalRiskLevel in RiskSummary, derived from the weighted risk score.
	riskSummaryNone     = "none"
	riskSummaryLow      = "low"
	riskSummaryModerate = "moderate"
	riskSummaryElevated = "elevated"
	riskSummaryCritical = "critical"

	// Severity weights used in computeRiskSummary.
	riskWeightHigh   = 3
	riskWeightMedium = 2
	riskWeightLow    = 1

	// riskScoreLow … riskScoreCritical are the lower-bounds (inclusive) of each
	// tier's weighted score range.  0 = "none"; 1–2 = "low"; 3–5 = "moderate";
	// 6–8 = "elevated"; ≥9 = "critical".
	riskScoreLow      = 1
	riskScoreModerate = 3
	riskScoreElevated = 6
	riskScoreCritical = 9

	// viabilityCriticalCap — maximum viability score allowed when
	// GlobalRiskLevel == "critical".  Forces critical-risk scenarios into the
	// "moderate" band at most, making "strong" + "critical" structurally
	// impossible.
	viabilityCriticalCap = 60

	// highLeverageRatio — NetDebt / FreeCashFlow above this threshold (year 5)
	// is flagged as high leverage.
	highLeverageRatio = 3.0

	// highPayrollCashFraction — payroll fraction of year-1 operating outflow
	// above this threshold triggers the "cash contribution: payroll dominant"
	// driver when the hiring driver was not already emitted.
	highPayrollCashFraction = 0.60

	// ── Trend intelligence constants (Sprint 5) ───────────────────────────────

	// minScaleRevenue — monthly revenue must exceed this value before the
	// time-to-scale growth-rate analysis is meaningful.
	minScaleRevenue = 5_000.0

	// minScaleVariation — minimum month-over-month revenue change (as a
	// fraction) required before a growth-rate acceleration is considered
	// significant.  Both consecutive months must individually exceed this.
	minScaleVariation = 0.02

	// volatilityThreshold — RevenueVolatility (coefficient of variation)
	// above this level overrides the improving/deteriorating classification.
	volatilityThreshold = 0.50

	// cagrImprovingFloor — minimum 5-year CAGR for the "improving" trend
	// signal classification.
	cagrImprovingFloor = 0.10

	// ── Break-even status constants ───────────────────────────────────────────

	// BEPReached — break-even was reached within the 60-month horizon.
	BEPReached = "reached"

	// BEPApproaching — break-even not reached yet but projected via slope
	// extrapolation.
	BEPApproaching = "approaching"

	// BEPNotReached — break-even not reached and trend does not project it.
	BEPNotReached = "not_reached"

	// minBEPSlopeRevenueFraction — proportional floor for the "approaching"
	// slope guard: slope must be at least 1 % of average monthly revenue.
	minBEPSlopeRevenueFraction = 0.01

	// minAbsoluteBEPSlope — absolute floor for the "approaching" slope guard:
	// slope must be at least 100 (currency units per month) regardless of
	// revenue size.  Both guards must pass together.
	minAbsoluteBEPSlope = 100.0

	// bepForwardSimCap — maximum months to simulate when projecting an
	// "approaching" break-even beyond month 36.
	bepForwardSimCap = 120

	// ── Funding urgency constants ─────────────────────────────────────────────

	// fundingUrgencyImmediate — minimum cash is reached within 6 months.
	fundingUrgencyImmediate = "immediate"

	// fundingUrgencyNearTerm — minimum cash is reached within 7–18 months.
	fundingUrgencyNearTerm = "near_term"

	// fundingUrgencyLongTerm — minimum cash is reached beyond 18 months.
	fundingUrgencyLongTerm = "long_term"
)

// ──────────────────────────────────────────────────────────────────────────────
// API contract
//
// ScenarioAnalysisResult is serialised directly into the HTTP response by the
// handler (no separate DTO layer).  The JSON tags on every field and nested
// struct define the public API contract.
//
// Consumer guidance:
//   - Primary decision fields: viability, projections, risks, risk_summary,
//     drivers, insights, highlights.
//   - projections.ebitda_margin_y5 and projections.max_yoy_growth_pct are
//     advanced scoring metrics exposed for transparency.  They are useful for
//     tooling and debugging but are not part of the stable business contract
//     and may evolve with the scoring model.
//   - metadata carries version and scoring-model identifiers.  Consumers
//     should use this to detect model changes without parsing field shapes.
//
// Stability contract:
//   - JSON keys are never renamed or removed (additive-only).
//   - Slice fields (risks, drivers) always serialize as [] — never null.
// ──────────────────────────────────────────────────────────────────────────────

// ScenarioAnalysisResult is the top-level output of AnalyzeScenario.
// It is serialised directly as the HTTP response body — see the API contract
// comment block above for consumer guidance.
type ScenarioAnalysisResult struct {
	Viability   ViabilityResult           `json:"viability"`
	Projections ProjectionResult          `json:"projections"`
	Risks       []Risk                    `json:"risks"`
	RiskSummary RiskSummary               `json:"risk_summary"` // global coherence signal (Sprint 3)
	Drivers     []Driver                  `json:"drivers"`
	Trends      Trends                    `json:"trends"`
	Insights    TrendInsights             `json:"insights"`           // trend intelligence layer (Sprint 5)
	Highlights  Highlights                `json:"highlights"`         // AI-ready narrative layer (Sprint 6)
	Metadata    *ScenarioAnalysisMetadata `json:"metadata,omitempty"` // versioning + scoring transparency
}

// ScenarioAnalysisMetadata carries version and scoring-model identifiers.
// It is an optional field (omitted when nil) and is always populated by
// AnalyzeScenario using static constants — no configuration dependency.
type ScenarioAnalysisMetadata struct {
	// Version identifies the analysis pipeline version.
	Version string `json:"version"`
	// ScoreModel identifies the viability scoring algorithm.
	// Consumers can use this to detect model changes across deployments.
	ScoreModel string `json:"score_model"`
}

// analysisVersion and analysisScoreModel are the static identifiers embedded
// in every ScenarioAnalysisResult.  Update these constants — not the struct —
// when the scoring model changes.
const (
	analysisVersion    = "v2"
	analysisScoreModel = "v2_continuous"
)

// Highlights is the AI-ready narrative layer.  It contains a varied headline,
// derived strength/weakness lists, and the top risks and drivers in prose form.
//
// Design rule: buildHighlights performs ZERO decimal arithmetic — all numeric
// signals are already pre-computed in the upstream fields.  Enforcement: the
// function only calls decimal comparison methods (.IsNegative, .IsPositive),
// never arithmetic methods (.Add, .Sub, .Mul, .Div).
//
// i18n: The *Localized fields carry translated versions of the base fields.
// They are omitted (json:"…,omitempty") when the plan language is "en".
// Consumers should prefer the localised fields when present.
type Highlights struct {
	// Headline is one of 7 distinct template strings chosen by
	// (viability.Status, globalRiskLevel, bepStatus, rootCauseDriver).
	Headline string `json:"headline"`
	// HeadlineLocalized carries Headline in the plan's configured language.
	HeadlineLocalized string `json:"headline_localized,omitempty"` // NEW — i18n

	// Strengths contains up to 2 positive signals: high-impact root-cause
	// drivers first, supplemented by projection signals (positive cash /
	// positive EBITDA).  Falls back to a single explanatory string — never empty.
	Strengths []string `json:"strengths"`
	// StrengthsLocalized carries Strengths in the plan's configured language.
	StrengthsLocalized []string `json:"strengths_localized,omitempty"` // NEW — i18n

	// Weaknesses lists the top 2 risk messages ordered high → medium → low.
	// Falls back to a single explanatory string — never empty.
	Weaknesses []string `json:"weaknesses"`
	// WeaknessesLocalized carries Weaknesses in the plan's configured language.
	WeaknessesLocalized []string `json:"weaknesses_localized,omitempty"` // NEW — i18n

	// TopRisks lists the top 2 risk messages, severity-ordered.
	// Falls back to a single explanatory string — never empty.
	TopRisks []string `json:"top_risks"`

	// TopDrivers lists the Effect strings of the top 2 ranked drivers.
	// Falls back to a single explanatory string — never empty.
	TopDrivers []string `json:"top_drivers"`
}

// ViabilityResult is a synthesised go/no-go score for the scenario.
type ViabilityResult struct {
	Score  int    `json:"score"`  // 0–100
	Status string `json:"status"` // strong | moderate | risky
}

// ProjectionResult aggregates the most decision-relevant KPIs.
type ProjectionResult struct {
	Revenue5Y       decimal.Decimal `json:"revenue_5y"`
	EbitdaPeak      decimal.Decimal `json:"ebitda_peak"`
	BreakEvenMonth  int             `json:"break_even_month"`  // 1-indexed; 0 = not_reached
	BreakEvenStatus string          `json:"break_even_status"` // reached | approaching | not_reached
	BreakEvenMethod string          `json:"break_even_method"` // economic_cash | pnl_approx | projected | ""
	CashMin         decimal.Decimal `json:"cash_min"`
	CashMinMonth    int             `json:"cash_min_month"`   // 1-indexed month of minimum cash
	FundingRequired decimal.Decimal `json:"funding_required"` // max(-CashMin, 0)
	FundingMonth    int             `json:"funding_month"`    // 0 when no funding required
	FundingUrgency  string          `json:"funding_urgency"`  // immediate | near_term | long_term | ""
	// Advanced scoring metrics — intentionally exposed for transparency and
	// tooling.  These are precomputed during projection extraction so that
	// computeViability can reference them without re-deriving from raw data.
	// They are NOT part of the stable business contract: their values and
	// interpretation may evolve when the scoring model is updated.
	// See ScenarioAnalysisMetadata.ScoreModel to detect such changes.
	EbitdaMarginY5  decimal.Decimal `json:"ebitda_margin_y5"`   // year-5 EBITDA / year-5 Sales; 0 if no sales
	MaxYoYGrowthPct float64         `json:"max_yoy_growth_pct"` // max year-over-year revenue growth %
}

// Risk describes a specific financial risk detected in the scenario.
type Risk struct {
	Type     string `json:"type"`
	Severity string `json:"severity"` // high | medium | low
	Urgency  string `json:"urgency"`  // immediate | near_term | long_term
	Message  string `json:"message"`  // English — always populated
	// MessageLocalized carries the same message in the plan's configured language.
	// Omitted when the plan language is "en" (Message is already English).
	// Consumers should prefer MessageLocalized when present.
	MessageLocalized string          `json:"message_localized,omitempty"` // NEW — i18n
	When             *int            `json:"when,omitempty"`              // 1-indexed month if time-specific
	Value            decimal.Decimal `json:"value,omitempty"`
}

// RiskSummary provides a global coherence signal over the full risk slice.
// It prevents a scenario with three medium-severity risks reading as "medium
// risk" when the aggregate picture is materially worse.
type RiskSummary struct {
	GlobalRiskLevel string `json:"global_risk_level"` // none | low | moderate | elevated | critical
	RiskScore       int    `json:"risk_score"`        // weighted aggregate (high=3, med=2, low=1)
	RiskCount       int    `json:"risk_count"`
}

// Driver identifies a key assumption that materially drives an outcome.
type Driver struct {
	// Code is a stable machine-readable identifier for this driver type.
	// It is always populated and language-neutral.
	// Use Code for localisation lookups and analytics.
	Code   string `json:"code"`   // NEW — stable i18n key
	Name   string `json:"name"`   // English label; kept for backward-compat
	Impact string `json:"impact"` // high | medium | low
	Effect string `json:"effect"` // English effect description
	// EffectLocalized carries the same effect in the plan's configured language.
	// Omitted when the plan language is "en".
	EffectLocalized string `json:"effect_localized,omitempty"` // NEW — i18n
	RootCause       bool   `json:"root_cause"`                 // true = root cause; false = symptom/consequence
}

// candidateDriver is an internal struct used by identifyDrivers and rankDrivers.
// It carries an integer priority to enable stable multi-key sorting without
// encoding priority implicitly in Impact strings.
// effectParams holds the raw numeric values used in Effect so that localised
// effect templates can be rendered without re-running the heuristics.
type candidateDriver struct {
	Driver
	priority     int                    // higher number = more important
	effectParams map[string]interface{} // params for localised effect template
}

// TrendInsights contains six derived scalars that characterise the trajectory
// of the scenario.  All fields are computed from FullPlanOutput — no additional
// DB reads are required.
//
// Sprint 5 additions: TimeToScale and TrendSignal.
type TrendInsights struct {
	// RevenueCAGR is the compound annual growth rate from year 1 to year 5.
	// Computed as (Year5Sales / Year1Sales)^(1/4) − 1.  Returns 0 when
	// Year1Sales ≤ 0.
	RevenueCAGR decimal.Decimal `json:"revenue_cagr"`

	// BurnRateMonthly is the average absolute value of negative
	// Economic.Total[m] months across the 36-month cash horizon.
	// Returns 0 when no negative months are observed.
	BurnRateMonthly decimal.Decimal `json:"burn_rate_monthly"`

	// RevenueVolatility is the coefficient of variation (stddev / mean) of
	// the 5 annual Sales values.  Returns 0 when mean = 0.
	RevenueVolatility decimal.Decimal `json:"revenue_volatility"`

	// InflectionMonth is the first 1-indexed month where NetCashFlow turns
	// positive after a negative period.  Returns 0 when no inflection is
	// observed within the 36-month horizon.
	InflectionMonth int `json:"inflection_month"`

	// TimeToScale is the first 1-indexed month where the month-over-month
	// revenue growth rate increases for two consecutive periods.  Guards:
	//   monthly revenue ≥ minScaleRevenue (5,000) for both qualifying months,
	//   month-over-month change ≥ minScaleVariation (2%) for both.
	// Returns 0 if no qualifying inflection is found within 36 months.
	TimeToScale int `json:"time_to_scale"`

	// TrendSignal is a synthesised direction label derived from the other
	// fields: "improving" | "deteriorating" | "volatile".
	// "volatile" overrides the direction signal when RevenueVolatility > 0.50.
	TrendSignal string `json:"trend_signal"`
}

// Trends carries simplified time-series for charts/mobile.
type Trends struct {
	Revenue []decimal.Decimal `json:"revenue"` // 5 annual data points
	Cash    []decimal.Decimal `json:"cash"`    // up to 36 monthly closing balances
}

// ──────────────────────────────────────────────────────────────────────────────
// Service interface + implementation
// ──────────────────────────────────────────────────────────────────────────────

// ScenarioAnalysisSvc is the interface consumed by the HTTP handler.
// Using an interface keeps the handler independently testable.
type ScenarioAnalysisSvc interface {
	AnalyzeScenario(ctx context.Context, tenantID, scenarioID uuid.UUID) (*ScenarioAnalysisResult, error)
}

// Compile-time check.
var _ ScenarioAnalysisSvc = (*ScenarioAnalysisService)(nil)

// ScenarioAnalysisService is the concrete implementation of ScenarioAnalysisSvc.
type ScenarioAnalysisService struct {
	reportSvc *ReportService
	logger    *logrus.Entry
}

// NewScenarioAnalysisService creates a ScenarioAnalysisService.
// It depends on ReportService (which already provides caching + compute)
// so that analysis results benefit from the existing LRU cache for free.
func NewScenarioAnalysisService(reportSvc *ReportService, logger *logrus.Entry) *ScenarioAnalysisService {
	return &ScenarioAnalysisService{
		reportSvc: reportSvc,
		logger:    logger,
	}
}

// AnalyzeScenario runs the full analysis pipeline for a scenario.
//
//  1. Load + compute via ReportService (cache-aware)
//  2. Extract KPI projections
//  3. Score viability
//  4. Detect risks
//  5. Identify key drivers
//  6. Build trend series
//  7. Post-populate i18n localised fields when plan language ≠ "en"
func (s *ScenarioAnalysisService) AnalyzeScenario(
	ctx context.Context,
	tenantID, scenarioID uuid.UUID,
) (*ScenarioAnalysisResult, error) {
	output, err := s.reportSvc.GetFullReport(ctx, tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).
			WithField("scenario_id", scenarioID).
			Error("scenario analysis: failed to get full report")
		return nil, apierror.Internal("failed to compute scenario report")
	}

	// ── Language resolution ────────────────────────────────────────────────
	// Load the plan config to extract the configured language.
	// Non-critical: a failure falls back to "en" without aborting the analysis.
	lang := "en"
	if planConfig, cfgErr := s.reportSvc.GetPlanConfig(ctx, tenantID, scenarioID); cfgErr == nil && planConfig != nil {
		lang = normalizeLanguage(planConfig.Language)
	} else if cfgErr != nil {
		s.logger.WithField("scenario_id", scenarioID).
			Debug("scenario analysis: plan config unavailable — defaulting to 'en'")
	}

	projections := computeProjections(output)

	// Execution order: projections → viability (raw) → risks → riskSummary →
	// cap if critical → re-classify.
	// Cap applied after risk summary: prevents strong/critical contradiction.
	viability := computeViability(projections)
	risks := detectRisks(output, projections)
	riskSummary := computeRiskSummary(risks)
	if riskSummary.GlobalRiskLevel == riskSummaryCritical {
		if viability.Score > viabilityCriticalCap {
			viability.Score = viabilityCriticalCap
		}
		viability.Status = classifyViability(viability.Score)
	}

	// identifyDriversWithParams returns the params map alongside the drivers
	// for use by the localisation layer.  identifyDrivers (called here) is
	// unchanged and retained for compatibility with existing tests.
	drivers, driverEffectParamsMap := identifyDriversWithParams(output)

	trends := extractTrends(output)
	insights := computeInsights(output)

	s.logger.WithFields(logrus.Fields{
		"scenario_id":       scenarioID,
		"viability_score":   viability.Score,
		"risk_count":        len(risks),
		"global_risk_level": riskSummary.GlobalRiskLevel,
		"lang":              lang,
	}).Info("scenario analysis complete")

	// Nil-guard: ensure slices always serialise as [] (never null) so clients
	// can iterate unconditionally without null-checking.
	if risks == nil {
		risks = []Risk{}
	}
	if drivers == nil {
		drivers = []Driver{}
	}
	if trends.Revenue == nil {
		trends.Revenue = []decimal.Decimal{}
	}
	if trends.Cash == nil {
		trends.Cash = []decimal.Decimal{}
	}

	// ── i18n: populate localised fields (non-English plans only) ─────────
	if lang != "en" {
		// Risk.MessageLocalized — uses re-derived leverage ratio from output.
		populateRiskLocalizationsWithOutput(
			risks, lang, projections,
			output.BSheet.Analysis.NetDebt[4],
			output.Ratios.Profitability.FreeCashFlow[4],
		)

		// Driver.EffectLocalized — uses pre-captured effectParams.
		populateDriverLocalizations(drivers, lang, driverEffectParamsMap)
	}

	// Build the result first so buildHighlights can reference all pre-computed
	// fields without re-deriving them.
	result := &ScenarioAnalysisResult{
		Viability:   viability,
		Projections: projections,
		Risks:       risks,
		RiskSummary: riskSummary,
		Drivers:     drivers,
		Trends:      trends,
		Insights:    insights,
		Metadata: &ScenarioAnalysisMetadata{
			Version:    analysisVersion,
			ScoreModel: analysisScoreModel,
		},
	}
	result.Highlights = buildHighlights(result)

	// ── i18n: populate localised Highlights fields ────────────────────────
	if lang != "en" {
		buildHighlightsLocalized(result, lang)
	}

	return result, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// 1. Projections
// ──────────────────────────────────────────────────────────────────────────────

// computeProjections extracts the headline KPIs from a FullPlanOutput.
// It is a pure function so it can be tested without a database.
func computeProjections(output *model.FullPlanOutput) ProjectionResult {
	cashMin := computeCashMin(output.Cash)
	cashMinMonth := computeCashMinMonth(output.Cash)

	var fundingRequired decimal.Decimal
	var fundingMonth int
	var fundingUrgency string
	if cashMin.IsNegative() {
		fundingRequired = cashMin.Neg()
		fundingMonth = cashMinMonth
		fundingUrgency = fundingUrgencyFromMonth(cashMinMonth)
	}

	bepMonth, bepStatus, bepMethod := computeBreakEvenV2(output.PnL, output.Cash)

	// Precompute year-5 EBITDA margin for viability scoring.
	var ebitdaMarginY5 decimal.Decimal
	year5 := output.PnL.Years[4] // index 4 = year 5
	if year5.Sales.GreaterThan(decimal.Zero) {
		ebitdaMarginY5 = year5.EBITDA.Div(year5.Sales)
	}

	return ProjectionResult{
		Revenue5Y:       computeRevenue5Y(output.PnL),
		EbitdaPeak:      computeEbitdaPeak(output.PnL),
		BreakEvenMonth:  bepMonth,
		BreakEvenStatus: bepStatus,
		BreakEvenMethod: bepMethod,
		CashMin:         cashMin,
		CashMinMonth:    cashMinMonth,
		FundingRequired: fundingRequired,
		FundingMonth:    fundingMonth,
		FundingUrgency:  fundingUrgency,
		EbitdaMarginY5:  ebitdaMarginY5,
		MaxYoYGrowthPct: maxYoYGrowthPct(output.PnL),
	}
}

// computeRevenue5Y sums the annual Sales figure across all 5 forecast years.
func computeRevenue5Y(pnl model.PnlReport) decimal.Decimal {
	total := decimal.Zero
	for i := 0; i < len(pnl.Years); i++ {
		total = total.Add(pnl.Years[i].Sales)
	}
	return total
}

// computeEbitdaPeak returns the highest annual EBITDA across the 5-year horizon.
func computeEbitdaPeak(pnl model.PnlReport) decimal.Decimal {
	if len(pnl.Years) == 0 {
		return decimal.Zero
	}
	peak := pnl.Years[0].EBITDA
	for i := 1; i < len(pnl.Years); i++ {
		if pnl.Years[i].EBITDA.GreaterThan(peak) {
			peak = pnl.Years[i].EBITDA
		}
	}
	return peak
}

// computeBreakEvenV2 determines when cumulative cash flow first turns positive
// using a three-phase strategy:
//
//	Phase 1 (months 1–36): Uses CashReport.Economic.Total[m] — the most
//	accurate monthly economic cash flow from the compute engine.
//
//	Phase 2 (months 37–60): Approximates from PnlReport.Years[3..4].CashFlow
//	divided evenly over 12 months.  Less granular but covers the full horizon.
//
//	Phase 3 (approaching): If no break-even is found in 60 months, uses a
//	linear slope of the last 12 economic monthly values to project whether
//	break-even is converging.  Dual guard prevents noise triggering:
//	  slope >= minBEPSlopeRevenueFraction * avgMonthlyRevenue  (proportional)
//	  slope >= minAbsoluteBEPSlope                             (absolute floor)
//	Both must be satisfied.
//
// Returns: (month 1-indexed, status, method).
// month = 0 and method = "" when status is BEPNotReached.
func computeBreakEvenV2(pnl model.PnlReport, cash model.CashReport) (month int, status, method string) {
	// Collect economic monthly values for all 36 months.
	var economicMonthly [36]decimal.Decimal
	for y := 0; y < 3; y++ {
		for m := 0; m < 12; m++ {
			economicMonthly[y*12+m] = cash.Years[y].Economic.Total[m]
		}
	}

	// ── Phase 1: Economic cash (months 1–36) ──────────────────────────────
	cumulative := decimal.Zero
	for i := 0; i < 36; i++ {
		cumulative = cumulative.Add(economicMonthly[i])
		if cumulative.GreaterThan(decimal.Zero) {
			return i + 1, BEPReached, "economic_cash"
		}
	}

	// Save cumulative at month 36 for the approaching projection (phase 3
	// starts from this point rather than from the PnL-adjusted cumulative so
	// that the projection is anchored to real economic cash data).
	cumulativeAt36 := cumulative

	// ── Phase 2: PnL cash-flow approximation (months 37–60) ──────────────
	twelve := decimal.NewFromInt(12)
	for y := 3; y < 5; y++ {
		monthly := pnl.Years[y].CashFlow.Div(twelve)
		for m := 0; m < 12; m++ {
			cumulative = cumulative.Add(monthly)
			if cumulative.GreaterThan(decimal.Zero) {
				return y*12 + m + 1, BEPReached, "pnl_approx"
			}
		}
	}

	// ── Phase 3: Approaching detection ────────────────────────────────────
	avgMonthlyRevenue := computeAvgMonthlyRevenue(pnl)
	if projMonth, ok := computeApproachingBEP(economicMonthly, cumulativeAt36, avgMonthlyRevenue); ok {
		return projMonth, BEPApproaching, "projected"
	}

	return 0, BEPNotReached, ""
}

// computeAvgMonthlyRevenue returns the average monthly revenue across the full
// 5-year P&L horizon (Revenue5Y / 60).  Used as the proportional base for the
// BEP slope guard.
func computeAvgMonthlyRevenue(pnl model.PnlReport) float64 {
	total := decimal.Zero
	for _, y := range pnl.Years {
		total = total.Add(y.Sales)
	}
	if total.IsZero() {
		return 0
	}
	avg, _ := total.Div(decimal.NewFromInt(60)).Float64()
	return avg
}

// linearSlope12 computes the ordinary-least-squares slope for a 12-element
// time series with x-indices 0..11.  Pre-computed constants:
//
//	x̄ = 5.5,  Σ(xᵢ − x̄)² = 143
func linearSlope12(y [12]float64) float64 {
	const xMean = 5.5
	const xVar = 143.0 // Σ(i − 5.5)² for i = 0..11

	var yMean float64
	for _, v := range y {
		yMean += v
	}
	yMean /= 12

	var cov float64
	for i, v := range y {
		cov += (float64(i) - xMean) * (v - yMean)
	}
	return cov / xVar
}

// computeApproachingBEP returns (projectedMonth, true) when the slope of the
// last 12 economic monthly values passes the dual guard and a forward
// simulation projects cumulative break-even within bepForwardSimCap months.
//
// The forward simulation starts from cumulativeAt36 (not the PnL-adjusted
// cumulative) and extrapolates the monthly value using the computed slope.
func computeApproachingBEP(
	economicMonthly [36]decimal.Decimal,
	cumulativeAt36 decimal.Decimal,
	avgMonthlyRevenue float64,
) (int, bool) {
	// Extract last 12 economic monthly incremental values (months 25–36).
	var last12 [12]float64
	for i := 0; i < 12; i++ {
		last12[i], _ = economicMonthly[24+i].Float64()
	}

	slope := linearSlope12(last12)

	// Dual guard: both conditions must be satisfied.
	proportionalFloor := minBEPSlopeRevenueFraction * avgMonthlyRevenue
	if slope < proportionalFloor || slope < minAbsoluteBEPSlope {
		return 0, false
	}

	// Forward simulation: extrapolate monthly values from month 36 onward.
	// currentMonthly is the projected value for the next step; it starts from
	// the last known monthly value and grows by slope each period.
	currentMonthly, _ := economicMonthly[35].Float64()
	cumulative, _ := cumulativeAt36.Float64()

	for step := 1; step <= bepForwardSimCap; step++ {
		currentMonthly += slope
		cumulative += currentMonthly
		if cumulative > 0 {
			return 36 + step, true
		}
	}

	return 0, false
}

// computeCashMinMonth returns the 1-indexed month number at which the minimum
// monthly closing cash balance is reached.  Returns 1 if the cash report is
// empty (degenerate case).
func computeCashMinMonth(cash model.CashReport) int {
	minVal := cash.Years[0].ClosingBalance[0]
	minMonth := 1
	for y := 0; y < len(cash.Years); y++ {
		for m := 0; m < 12; m++ {
			v := cash.Years[y].ClosingBalance[m]
			if v.LessThan(minVal) {
				minVal = v
				minMonth = y*12 + m + 1
			}
		}
	}
	return minMonth
}

// fundingUrgencyFromMonth classifies how urgently external funding is needed
// based on when the minimum cash position occurs.
//
//	≤  6 months → "immediate"
//	≤ 18 months → "near_term"
//	>  18 months → "long_term"
func fundingUrgencyFromMonth(month int) string {
	switch {
	case month <= 6:
		return fundingUrgencyImmediate
	case month <= 18:
		return fundingUrgencyNearTerm
	default:
		return fundingUrgencyLongTerm
	}
}

// computeCashMin returns the minimum monthly closing cash balance over the
// 3-year cash report horizon (36 months).
func computeCashMin(cash model.CashReport) decimal.Decimal {
	if len(cash.Years) == 0 {
		return decimal.Zero
	}
	// Seed with the first value so min is meaningful even for all-positive data.
	minVal := cash.Years[0].ClosingBalance[0]
	for y := 0; y < len(cash.Years); y++ {
		for m := 0; m < 12; m++ {
			v := cash.Years[y].ClosingBalance[m]
			if v.LessThan(minVal) {
				minVal = v
			}
		}
	}
	return minVal
}

// ──────────────────────────────────────────────────────────────────────────────
// 2. Viability scoring
// ──────────────────────────────────────────────────────────────────────────────

// computeViability derives a 0–100 continuous viability score and classifies
// it using five independent scoring functions.
//
// The five components and their maximum contributions (sum = 100):
//
//	scoreCashHealth       → max 30 pts
//	scoreBreakEven        → max 25 pts
//	scoreEbitdaMargin     → max 20 pts
//	scoreRevenueRealism   → max 15 pts
//	scoreBurnControl      → max 10 pts
//
// The function is pure and database-free.  The caller is responsible for
// applying the viability cap when GlobalRiskLevel == "critical" (Sprint 3).
func computeViability(p ProjectionResult) ViabilityResult {
	total := scoreCashHealth(p.CashMin, p.Revenue5Y) +
		scoreBreakEven(p.BreakEvenMonth) +
		scoreEbitdaMargin(p.EbitdaPeak, p.EbitdaMarginY5) +
		scoreRevenueRealism(p.MaxYoYGrowthPct, p.Revenue5Y) +
		scoreBurnControl(p.CashMin, p.Revenue5Y)

	score := int(math.Round(total))
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return ViabilityResult{
		Score:  score,
		Status: classifyViability(score),
	}
}

// clamp64 clamps a float64 value to [lo, hi].
func clamp64(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// scoreCashHealth returns a 0–30 score based on the minimum closing cash
// balance relative to 5-year revenue.
//
// When revenue5Y ≥ minMeaningfulRevenue (relative branch):
//
//	ratio = cashMin / revenue5Y
//	score = 30 × (ratio − floor) / (ceiling − floor), clamped to [0, 30]
//	anchor points: ratio ≤ −50% → 0 pts, ratio ≥ +10% → 30 pts
//
// When revenue5Y < minMeaningfulRevenue (absolute branch — avoids ratio explosion):
//
//	score = 30 × (cashMin − absCeiling) / (absCeiling − absFloor), clamped to [0, 30]
//	anchor points: cashMin ≤ −200K → 0 pts, cashMin ≥ +50K → 30 pts
func scoreCashHealth(cashMin, rev5Y decimal.Decimal) float64 {
	rev, _ := rev5Y.Float64()
	cash, _ := cashMin.Float64()

	if rev >= minMeaningfulRevenue {
		ratio := cash / rev
		return clamp64(30.0*(ratio-cashHealthFloor)/(cashHealthCeiling-cashHealthFloor), 0, 30)
	}
	// Absolute branch: pre-revenue / micro-revenue scenario.
	return clamp64(30.0*(cash-cashHealthAbsFloor)/(cashHealthAbsCeiling-cashHealthAbsFloor), 0, 30)
}

// scoreBreakEven returns a 0–25 score that decays linearly from the shortest
// (month 1 = 25 pts) to the longest plausible BEP (month 60 = 0 pts).
// bep = 0 (not reached / not_reached) always returns 0.
func scoreBreakEven(bep int) float64 {
	if bep <= 0 {
		return 0
	}
	// 25 × (60 − bep) / 59  clamped to [0, 25]
	// bep=1 → 25*59/59 = 25;  bep=60 → 0;  bep>60 → clamped to 0
	return clamp64(25.0*float64(bepScoreHorizon-bep)/float64(bepScoreHorizon-1), 0, 25)
}

// scoreEbitdaMargin returns a 0–20 score based on the year-5 EBITDA margin.
// If peak EBITDA is never positive, score is 0 regardless of margin.
//
//	margin = ebitdaMarginY5 (precomputed decimal, year5.EBITDA / year5.Sales)
//	score  = 10 + 10 × margin / 0.25,  clamped to [0, 20]
//	anchor points: margin = −25% → 0, margin = 0% → 10 pts, margin = +25% → 20 pts
func scoreEbitdaMargin(ebitdaPeak, ebitdaMarginY5 decimal.Decimal) float64 {
	// A scenario where EBITDA never turns positive earns no margin score.
	if ebitdaPeak.LessThanOrEqual(decimal.Zero) {
		return 0
	}
	margin, _ := ebitdaMarginY5.Float64()
	return clamp64(ebitdaMarginBaseline+ebitdaMarginBaseline*margin/ebitdaMarginAnchor, 0, 20)
}

// scoreRevenueRealism returns a 0–15 score that rewards modest growth and
// penalises implausible hypergrowth.
//
//	score = 15 × (1 − maxYoYPct / 100),  clamped to [0, 15]
//	anchor points: 0% growth → 15 pts, 100% growth → 0 pts, >100% → 0 pts
//
// Zero revenue (revenue5Y ≤ 0) always returns 0 — the business has no
// commercial traction to score.
func scoreRevenueRealism(maxYoYPct float64, revenue5Y decimal.Decimal) float64 {
	if revenue5Y.IsZero() || revenue5Y.IsNegative() {
		return 0
	}
	return clamp64(15.0*(1.0-maxYoYPct/unrealisticGrowthThreshold), 0, 15)
}

// scoreBurnControl returns a 0–10 score that rewards efficient use of capital
// by measuring how much cash is consumed relative to total revenue.
//
// Relative branch (revenue5Y ≥ minMeaningfulRevenue):
//
//	burnRatio = −cashMin / revenue5Y  (positive = cash consumed)
//	score = 10 × (1 − burnRatio / 0.20),  clamped to [0, 10]
//	anchor points: cashMin ≥ 0 → 10 pts, burn ratio = 20% → 0 pts
//
// Absolute branch (revenue5Y < minMeaningfulRevenue):
//
//	cashMin ≥ 0 → 10 pts; cashMin = −50K → 0 pts; linear in between.
func scoreBurnControl(cashMin, rev5Y decimal.Decimal) float64 {
	rev, _ := rev5Y.Float64()
	cash, _ := cashMin.Float64()

	if rev >= minMeaningfulRevenue {
		burnRatio := -cash / rev
		return clamp64(10.0*(1.0-burnRatio/burnControlCeiling), 0, 10)
	}
	// Absolute branch.
	if cash >= 0 {
		return 10
	}
	return clamp64(10.0*(cash-burnControlAbsFloor)/(-burnControlAbsFloor), 0, 10)
}

func classifyViability(score int) string {
	switch {
	case score >= viabilityStrong:
		return "strong"
	case score >= viabilityModerate:
		return "moderate"
	default:
		return "risky"
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// 3. Risk detection + summary
// ──────────────────────────────────────────────────────────────────────────────

// urgencyFromMonth maps a 1-indexed month pointer to a funding-urgency tier.
// A nil pointer means "no specific timing" and returns "" (empty string).
// Callers that need a non-empty urgency regardless should substitute a
// severity-based default after calling this function.
//
//	≤  6 months → "immediate"
//	≤ 18 months → "near_term"
//	>  18 months → "long_term"
//	nil          → ""
func urgencyFromMonth(when *int) string {
	if when == nil {
		return ""
	}
	switch {
	case *when <= 6:
		return fundingUrgencyImmediate
	case *when <= 18:
		return fundingUrgencyNearTerm
	default:
		return fundingUrgencyLongTerm
	}
}

// detectRisks returns a slice of Risk objects derived from the computed output
// and pre-calculated projections.  It is a pure function.
//
// Sprint 3 changes:
//   - cash_gap gains When (= CashMinMonth) and Urgency.
//   - late_profitability gains When and Urgency.
//   - no_profitability and unrealistic_growth gain Urgency.
//   - high_leverage risk added (NetDebt[4] vs FreeCashFlow[4]).
//   - maxYoYGrowthPct double-call eliminated: use p.MaxYoYGrowthPct directly.
func detectRisks(output *model.FullPlanOutput, p ProjectionResult) []Risk {
	var risks []Risk

	// ── cash_gap ───────────────────────────────────────────────────────────
	if p.CashMin.LessThan(decimal.Zero) {
		when := intPtr(p.CashMinMonth)
		risks = append(risks, Risk{
			Type:     "cash_gap",
			Severity: "high",
			Urgency:  urgencyFromMonth(when),
			Message:  fmt.Sprintf("Cash position drops to %s at month %d — the scenario may require additional financing.", p.CashMin.StringFixed(0), p.CashMinMonth),
			When:     when,
			Value:    p.CashMin,
		})
	}

	// ── late_profitability ─────────────────────────────────────────────────
	if p.BreakEvenMonth > breakEvenRiskyMonths {
		when := intPtr(p.BreakEvenMonth)
		risks = append(risks, Risk{
			Type:     "late_profitability",
			Severity: "medium",
			Urgency:  urgencyFromMonth(when),
			Message:  fmt.Sprintf("Break-even is reached at month %d, which is beyond the 24-month threshold.", p.BreakEvenMonth),
			When:     when,
		})
	} else if p.BreakEvenMonth == 0 {
		risks = append(risks, Risk{
			Type:     "late_profitability",
			Severity: "high",
			Urgency:  fundingUrgencyImmediate, // structural problem — urgent regardless of timing
			Message:  "The scenario does not reach profitability within the 5-year horizon.",
		})
	}

	// ── no_profitability ───────────────────────────────────────────────────
	if p.EbitdaPeak.LessThanOrEqual(decimal.Zero) {
		risks = append(risks, Risk{
			Type:     "no_profitability",
			Severity: "high",
			Urgency:  fundingUrgencyImmediate, // existential — requires structural revision
			Message:  "EBITDA never turns positive — the scenario generates structural operating losses.",
			Value:    p.EbitdaPeak,
		})
	}

	// ── unrealistic_growth ─────────────────────────────────────────────────
	// Use p.MaxYoYGrowthPct (precomputed in computeProjections) to avoid
	// calling maxYoYGrowthPct twice.
	if p.MaxYoYGrowthPct > unrealisticGrowthThreshold {
		risks = append(risks, Risk{
			Type:     "unrealistic_growth",
			Severity: "medium",
			Urgency:  fundingUrgencyLongTerm, // modelling issue, not time-critical
			Message:  fmt.Sprintf("Revenue growth reaches %.0f %% year-over-year, which may be difficult to achieve and sustain.", p.MaxYoYGrowthPct),
			Value:    decimal.NewFromFloat(p.MaxYoYGrowthPct),
		})
	}

	// ── high_leverage ──────────────────────────────────────────────────────
	// Compare year-5 net debt (BSheet.Analysis.NetDebt[4]) against year-5
	// free cash flow (Ratios.Profitability.FreeCashFlow[4]).
	// Index 4 in the 5-element Ratios arrays = year 5.
	// Index 4 in the 6-element BSheet arrays = year 5 (index 0 = opening balance).
	netDebt5 := output.BSheet.Analysis.NetDebt[4]
	fcf5 := output.Ratios.Profitability.FreeCashFlow[4]
	if netDebt5.GreaterThan(decimal.Zero) {
		if fcf5.LessThanOrEqual(decimal.Zero) {
			// Cannot service debt from cash flow — highest severity.
			risks = append(risks, Risk{
				Type:     "high_leverage",
				Severity: "high",
				Urgency:  fundingUrgencyLongTerm,
				Message:  fmt.Sprintf("Net debt of %s in year 5 cannot be serviced from free cash flow — the scenario carries structural refinancing risk.", netDebt5.StringFixed(0)),
				Value:    netDebt5,
			})
		} else {
			nd, _ := netDebt5.Float64()
			fcf, _ := fcf5.Float64()
			ratio := nd / fcf
			if ratio > highLeverageRatio {
				risks = append(risks, Risk{
					Type:     "high_leverage",
					Severity: "medium",
					Urgency:  fundingUrgencyLongTerm,
					Message:  fmt.Sprintf("Net debt is %.1fx free cash flow in year 5 — leverage is elevated and may constrain future financing.", ratio),
					Value:    netDebt5,
				})
			}
		}
	}

	return risks
}

// computeRiskSummary aggregates a risk slice into a global coherence signal.
// Weights: high=3, medium=2, low=1.
// Score thresholds: 0="none", 1–2="low", 3–5="moderate", 6–8="elevated", ≥9="critical".
func computeRiskSummary(risks []Risk) RiskSummary {
	score := 0
	for _, r := range risks {
		switch r.Severity {
		case "high":
			score += riskWeightHigh
		case "medium":
			score += riskWeightMedium
		case "low":
			score += riskWeightLow
		}
	}

	var level string
	switch {
	case score == 0:
		level = riskSummaryNone
	case score < riskScoreModerate:
		level = riskSummaryLow
	case score < riskScoreElevated:
		level = riskSummaryModerate
	case score < riskScoreCritical:
		level = riskSummaryElevated
	default:
		level = riskSummaryCritical
	}

	return RiskSummary{
		GlobalRiskLevel: level,
		RiskScore:       score,
		RiskCount:       len(risks),
	}
}

// maxYoYGrowthPct returns the maximum year-over-year revenue growth rate (as a
// percentage) across the 5-year P&L.  Returns 0 if revenue is zero or negative
// in the base year.
func maxYoYGrowthPct(pnl model.PnlReport) float64 {
	maxGrowth := 0.0
	for y := 1; y < len(pnl.Years); y++ {
		prev := pnl.Years[y-1].Sales
		curr := pnl.Years[y].Sales
		if prev.IsZero() || prev.IsNegative() {
			continue
		}
		growth := curr.Sub(prev).Div(prev).Mul(decimal.NewFromInt(100))
		g, _ := growth.Float64()
		if g > maxGrowth {
			maxGrowth = g
		}
	}
	return maxGrowth
}

// ──────────────────────────────────────────────────────────────────────────────
// 4. Driver identification (v2 — ranked, causal, top-3)
// ──────────────────────────────────────────────────────────────────────────────

// rankDrivers sorts candidate drivers by (priority desc, rootCause desc) and
// returns the top 3 as plain Driver values.  Deterministic: equal-priority
// root causes precede symptoms.
func rankDrivers(candidates []candidateDriver) []Driver {
	// Simple insertion sort — candidate count is always small (≤ ~6).
	for i := 1; i < len(candidates); i++ {
		for j := i; j > 0; j-- {
			a, b := candidates[j-1], candidates[j]
			// Primary: priority descending.
			if a.priority < b.priority ||
				// Secondary: root cause before symptom on equal priority.
				(a.priority == b.priority && !a.RootCause && b.RootCause) {
				candidates[j-1], candidates[j] = candidates[j], candidates[j-1]
			} else {
				break
			}
		}
	}

	limit := len(candidates)
	if limit > 3 {
		limit = 3
	}
	out := make([]Driver, limit)
	for i := 0; i < limit; i++ {
		out[i] = candidates[i].Driver
	}
	return out
}

// rankDriversWithParams is like rankDrivers but also returns a map of
// code → effectParams for use by the i18n layer.
func rankDriversWithParams(candidates []candidateDriver) ([]Driver, map[string]map[string]interface{}) {
	// Re-use rankDrivers for the sort + slice — it operates by value so
	// we sort a copy first to recover the effectParams after ranking.
	for i := 1; i < len(candidates); i++ {
		for j := i; j > 0; j-- {
			a, b := candidates[j-1], candidates[j]
			if a.priority < b.priority ||
				(a.priority == b.priority && !a.RootCause && b.RootCause) {
				candidates[j-1], candidates[j] = candidates[j], candidates[j-1]
			} else {
				break
			}
		}
	}
	limit := len(candidates)
	if limit > 3 {
		limit = 3
	}
	drivers := make([]Driver, limit)
	params := make(map[string]map[string]interface{}, limit)
	for i := 0; i < limit; i++ {
		drivers[i] = candidates[i].Driver
		if candidates[i].Driver.Code != "" {
			params[candidates[i].Driver.Code] = candidates[i].effectParams
		}
	}
	return drivers, params
}

// identifyDrivers returns the top-3 ranked key assumption-to-outcome links.
// It is a pure function.
//
// Sprint 4 changes:
//   - All drivers carry RootCause (true = root cause, false = symptom).
//   - Hiring driver upgraded: trajectory-based impact (worsening → high,
//     improving → medium); compares year-1 vs year-3 payroll/revenue ratio.
//   - Cash-contribution driver added as fallback for payroll dominance when
//     the hiring driver was not already emitted (dedup guard).
//   - All drivers are collected as candidateDrivers and passed through
//     rankDrivers to enforce the top-3 ceiling with stable ordering.
func identifyDrivers(output *model.FullPlanOutput) []Driver {
	var candidates []candidateDriver

	// Guard: avoid division by zero for empty scenarios.
	if len(output.PnL.Years) == 0 {
		return nil
	}

	year1 := output.PnL.Years[0]
	hiringDriverEmitted := false

	// ── Hiring aggressiveness (trajectory-aware) ───────────────────────────
	// Payroll/revenue in year 1 vs year 3.  A ratio above 50 % in year 1 is
	// the entry condition.  If the ratio is still worsening by year 3 (ratio
	// increases), the driver has high impact; if improving, medium impact.
	if year1.Sales.GreaterThan(decimal.Zero) {
		hiringRatioY1, _ := year1.PayrollExpenses.Div(year1.Sales).Float64()
		if hiringRatioY1 > highHiringRatio {
			impact := "medium"
			if len(output.PnL.Years) >= 3 {
				year3 := output.PnL.Years[2]
				if year3.Sales.GreaterThan(decimal.Zero) {
					hiringRatioY3, _ := year3.PayrollExpenses.Div(year3.Sales).Float64()
					if hiringRatioY3 > hiringRatioY1 {
						impact = "high" // trajectory is worsening
					}
				}
			}
			priority := 2
			if impact == "high" {
				priority = 3
			}
			candidates = append(candidates, candidateDriver{
				Driver: Driver{
					Code:      "hiring_aggressive",
					Name:      "Hiring too aggressive",
					Impact:    impact,
					Effect:    fmt.Sprintf("Payroll represents %.0f %% of year-1 revenue — reduce hiring pace or increase revenue to improve the ratio.", hiringRatioY1*100),
					RootCause: true,
				},
				priority:     priority,
				effectParams: map[string]interface{}{"pct": fmt.Sprintf("%.0f", hiringRatioY1*100)},
			})
			hiringDriverEmitted = true
		}
	}

	// ── Low gross margin (pricing signal) ─────────────────────────────────
	if year1.Sales.GreaterThan(decimal.Zero) {
		grossMargin := year1.Sales.Sub(year1.COGS).Div(year1.Sales)
		gm, _ := grossMargin.Float64()
		if gm < lowGrossMarginThreshold {
			candidates = append(candidates, candidateDriver{
				Driver: Driver{
					Code:      "pricing_low",
					Name:      "Pricing too low",
					Impact:    "high",
					Effect:    fmt.Sprintf("Gross margin is %.0f %% in year 1 — consider increasing prices or reducing COGS.", gm*100),
					RootCause: true,
				},
				priority:     3,
				effectParams: map[string]interface{}{"pct": fmt.Sprintf("%.0f", gm*100)},
			})
		}
	}

	// ── Inefficient growth model (symptom, not root cause) ────────────────
	if year1.Sales.GreaterThan(decimal.Zero) && len(output.PnL.Years) >= 2 {
		burnRatio, _ := year1.CashFlow.Neg().Div(year1.Sales).Float64()
		year2Sales := output.PnL.Years[1].Sales
		var yoyGrowth float64
		g := year2Sales.Sub(year1.Sales).Div(year1.Sales)
		yoyGrowth, _ = g.Float64()

		if burnRatio > highBurnRatio && yoyGrowth < slowGrowthThreshold {
			candidates = append(candidates, candidateDriver{
				Driver: Driver{
					Code:      "inefficient_growth",
					Name:      "Inefficient growth model",
					Impact:    "medium",
					Effect:    fmt.Sprintf("Year-1 burn is %.0f %% of revenue but year-2 growth is only %.0f %% — costs are scaling faster than revenue.", burnRatio*100, yoyGrowth*100),
					RootCause: false, // symptom of root causes (hiring + pricing)
				},
				priority:     1,
				effectParams: map[string]interface{}{"burn": fmt.Sprintf("%.0f", burnRatio*100), "growth": fmt.Sprintf("%.0f", yoyGrowth*100)},
			})
		}
	}

	// ── High EBITDA leverage (positive signal) ────────────────────────────
	if len(output.PnL.Years) == 5 {
		year5 := output.PnL.Years[4]
		if year5.Sales.GreaterThan(decimal.Zero) {
			ebitdaMargin, _ := year5.EBITDA.Div(year5.Sales).Float64()
			if ebitdaMargin > 0.25 {
				candidates = append(candidates, candidateDriver{
					Driver: Driver{
						Code:      "strong_leverage",
						Name:      "Strong operating leverage",
						Impact:    "high",
						Effect:    fmt.Sprintf("EBITDA margin reaches %.0f %% by year 5 — costs are scaling sub-linearly relative to revenue.", ebitdaMargin*100),
						RootCause: true,
					},
					priority:     3,
					effectParams: map[string]interface{}{"pct": fmt.Sprintf("%.0f", ebitdaMargin*100)},
				})
			}
		}
	}

	// ── Cash contribution: payroll dominant (fallback root cause) ──────────
	// Fires only when the P&L-ratio hiring driver was NOT emitted (dedup).
	// Uses Cash.Years[0].Operating.Total to measure actual cash outflow.
	if !hiringDriverEmitted {
		var operatingOut float64
		for _, m := range output.Cash.Years[0].Operating.Total {
			v, _ := m.Float64()
			if v < 0 {
				operatingOut += -v // accumulate magnitude of outflows only
			}
		}
		if operatingOut > 0 {
			payroll, _ := year1.PayrollExpenses.Float64()
			if payroll/operatingOut > highPayrollCashFraction {
				candidates = append(candidates, candidateDriver{
					Driver: Driver{
						Code:      "payroll_dominant",
						Name:      "Cash contribution: payroll dominant",
						Impact:    "high",
						Effect:    fmt.Sprintf("Payroll accounts for %.0f %% of year-1 operating cash outflow — headcount cost is the primary cash drain.", (payroll/operatingOut)*100),
						RootCause: true,
					},
					priority:     3,
					effectParams: map[string]interface{}{"pct": fmt.Sprintf("%.0f", (payroll/operatingOut)*100)},
				})
			}
		}
	}

	return rankDrivers(candidates)
}

// identifyDriversWithParams is like identifyDrivers but additionally returns
// a map of driver code → effect params for use by the i18n localisation layer.
// Called from AnalyzeScenario instead of identifyDrivers when lang != "en".
func identifyDriversWithParams(output *model.FullPlanOutput) ([]Driver, map[string]map[string]interface{}) {
	var candidates []candidateDriver

	if len(output.PnL.Years) == 0 {
		return nil, nil
	}

	year1 := output.PnL.Years[0]
	hiringDriverEmitted := false

	if year1.Sales.GreaterThan(decimal.Zero) {
		hiringRatioY1, _ := year1.PayrollExpenses.Div(year1.Sales).Float64()
		if hiringRatioY1 > highHiringRatio {
			impact := "medium"
			if len(output.PnL.Years) >= 3 {
				year3 := output.PnL.Years[2]
				if year3.Sales.GreaterThan(decimal.Zero) {
					hiringRatioY3, _ := year3.PayrollExpenses.Div(year3.Sales).Float64()
					if hiringRatioY3 > hiringRatioY1 {
						impact = "high"
					}
				}
			}
			priority := 2
			if impact == "high" {
				priority = 3
			}
			candidates = append(candidates, candidateDriver{
				Driver: Driver{
					Code:      "hiring_aggressive",
					Name:      "Hiring too aggressive",
					Impact:    impact,
					Effect:    fmt.Sprintf("Payroll represents %.0f %% of year-1 revenue — reduce hiring pace or increase revenue to improve the ratio.", hiringRatioY1*100),
					RootCause: true,
				},
				priority:     priority,
				effectParams: map[string]interface{}{"pct": fmt.Sprintf("%.0f", hiringRatioY1*100)},
			})
			hiringDriverEmitted = true
		}
	}

	if year1.Sales.GreaterThan(decimal.Zero) {
		grossMargin := year1.Sales.Sub(year1.COGS).Div(year1.Sales)
		gm, _ := grossMargin.Float64()
		if gm < lowGrossMarginThreshold {
			candidates = append(candidates, candidateDriver{
				Driver: Driver{
					Code:      "pricing_low",
					Name:      "Pricing too low",
					Impact:    "high",
					Effect:    fmt.Sprintf("Gross margin is %.0f %% in year 1 — consider increasing prices or reducing COGS.", gm*100),
					RootCause: true,
				},
				priority:     3,
				effectParams: map[string]interface{}{"pct": fmt.Sprintf("%.0f", gm*100)},
			})
		}
	}

	if year1.Sales.GreaterThan(decimal.Zero) && len(output.PnL.Years) >= 2 {
		burnRatio, _ := year1.CashFlow.Neg().Div(year1.Sales).Float64()
		year2Sales := output.PnL.Years[1].Sales
		var yoyGrowth float64
		g := year2Sales.Sub(year1.Sales).Div(year1.Sales)
		yoyGrowth, _ = g.Float64()
		if burnRatio > highBurnRatio && yoyGrowth < slowGrowthThreshold {
			candidates = append(candidates, candidateDriver{
				Driver: Driver{
					Code:      "inefficient_growth",
					Name:      "Inefficient growth model",
					Impact:    "medium",
					Effect:    fmt.Sprintf("Year-1 burn is %.0f %% of revenue but year-2 growth is only %.0f %% — costs are scaling faster than revenue.", burnRatio*100, yoyGrowth*100),
					RootCause: false,
				},
				priority:     1,
				effectParams: map[string]interface{}{"burn": fmt.Sprintf("%.0f", burnRatio*100), "growth": fmt.Sprintf("%.0f", yoyGrowth*100)},
			})
		}
	}

	if len(output.PnL.Years) == 5 {
		year5 := output.PnL.Years[4]
		if year5.Sales.GreaterThan(decimal.Zero) {
			ebitdaMargin, _ := year5.EBITDA.Div(year5.Sales).Float64()
			if ebitdaMargin > 0.25 {
				candidates = append(candidates, candidateDriver{
					Driver: Driver{
						Code:      "strong_leverage",
						Name:      "Strong operating leverage",
						Impact:    "high",
						Effect:    fmt.Sprintf("EBITDA margin reaches %.0f %% by year 5 — costs are scaling sub-linearly relative to revenue.", ebitdaMargin*100),
						RootCause: true,
					},
					priority:     3,
					effectParams: map[string]interface{}{"pct": fmt.Sprintf("%.0f", ebitdaMargin*100)},
				})
			}
		}
	}

	if !hiringDriverEmitted {
		var operatingOut float64
		for _, m := range output.Cash.Years[0].Operating.Total {
			v, _ := m.Float64()
			if v < 0 {
				operatingOut += -v
			}
		}
		if operatingOut > 0 {
			payroll, _ := year1.PayrollExpenses.Float64()
			if payroll/operatingOut > highPayrollCashFraction {
				candidates = append(candidates, candidateDriver{
					Driver: Driver{
						Code:      "payroll_dominant",
						Name:      "Cash contribution: payroll dominant",
						Impact:    "high",
						Effect:    fmt.Sprintf("Payroll accounts for %.0f %% of year-1 operating cash outflow — headcount cost is the primary cash drain.", (payroll/operatingOut)*100),
						RootCause: true,
					},
					priority:     3,
					effectParams: map[string]interface{}{"pct": fmt.Sprintf("%.0f", (payroll/operatingOut)*100)},
				})
			}
		}
	}

	return rankDriversWithParams(candidates)
}

// ──────────────────────────────────────────────────────────────────────────────
// 5. Trend extraction
// ──────────────────────────────────────────────────────────────────────────────

// extractTrends builds the simplified time-series payloads for the mobile UX
// and AI narration layer.  It is a pure function.
func extractTrends(output *model.FullPlanOutput) Trends {
	return Trends{
		Revenue: extractRevenueTrend(output.PnL),
		Cash:    extractCashTrend(output.Cash),
	}
}

// extractRevenueTrend returns the 5 annual Sales figures.
func extractRevenueTrend(pnl model.PnlReport) []decimal.Decimal {
	series := make([]decimal.Decimal, len(pnl.Years))
	for i, y := range pnl.Years {
		series[i] = y.Sales
	}
	return series
}

// extractCashTrend returns monthly closing cash balances across the available
// cash report years (up to 36 months = 3 years).
func extractCashTrend(cash model.CashReport) []decimal.Decimal {
	months := make([]decimal.Decimal, 0, len(cash.Years)*12)
	for y := 0; y < len(cash.Years); y++ {
		for m := 0; m < 12; m++ {
			months = append(months, cash.Years[y].ClosingBalance[m])
		}
	}
	return months
}

// ──────────────────────────────────────────────────────────────────────────────
// 6. Trend insights (Sprint 5)
// ──────────────────────────────────────────────────────────────────────────────

// computeRevenueCAGR returns the compound annual growth rate from year 1 to
// year 5 of the P&L.  CAGR = (Year5 / Year1)^(1/4) − 1.
// Returns decimal.Zero when Year1Sales ≤ 0 (undefined base).
func computeRevenueCAGR(pnl model.PnlReport) decimal.Decimal {
	if len(pnl.Years) < 5 {
		return decimal.Zero
	}
	y1, _ := pnl.Years[0].Sales.Float64()
	if y1 <= 0 {
		return decimal.Zero
	}
	y5, _ := pnl.Years[4].Sales.Float64()
	if y5 < 0 {
		return decimal.Zero
	}
	cagr := math.Pow(y5/y1, 0.25) - 1
	return decimal.NewFromFloat(cagr)
}

// computeBurnRateMonthly returns the average absolute monthly economic outflow
// across all months where Economic.Total[m] < 0 in the 36-month cash horizon.
// Returns decimal.Zero when no negative months are observed (no burn).
func computeBurnRateMonthly(cash model.CashReport) decimal.Decimal {
	var totalBurn float64
	count := 0
	for y := 0; y < len(cash.Years); y++ {
		for m := 0; m < 12; m++ {
			v, _ := cash.Years[y].Economic.Total[m].Float64()
			if v < 0 {
				totalBurn += -v
				count++
			}
		}
	}
	if count == 0 {
		return decimal.Zero
	}
	return decimal.NewFromFloat(totalBurn / float64(count))
}

// computeRevenueVolatility returns the coefficient of variation (stddev / mean)
// of the 5 annual P&L Sales values.  Returns decimal.Zero when mean = 0 or
// when all values are identical (flat revenue).
func computeRevenueVolatility(pnl model.PnlReport) decimal.Decimal {
	n := len(pnl.Years)
	if n < 2 {
		return decimal.Zero
	}
	if n > 5 {
		n = 5
	}

	values := make([]float64, n)
	var sum float64
	for i := 0; i < n; i++ {
		v, _ := pnl.Years[i].Sales.Float64()
		values[i] = v
		sum += v
	}

	mean := sum / float64(n)
	if mean == 0 {
		return decimal.Zero
	}

	var variance float64
	for i := 0; i < n; i++ {
		diff := values[i] - mean
		variance += diff * diff
	}
	variance /= float64(n)

	cv := math.Sqrt(variance) / math.Abs(mean)
	return decimal.NewFromFloat(cv)
}

// computeInflectionMonth returns the 1-indexed month at which NetCashFlow
// first turns positive after at least one negative month, within the 36-month
// cash horizon.  Returns 0 when no such inflection is observed.
func computeInflectionMonth(cash model.CashReport) int {
	seenNegative := false
	for y := 0; y < len(cash.Years); y++ {
		for m := 0; m < 12; m++ {
			v, _ := cash.Years[y].NetCashFlow[m].Float64()
			if v < 0 {
				seenNegative = true
			} else if seenNegative && v > 0 {
				return y*12 + m + 1 // 1-indexed
			}
		}
	}
	return 0
}

// computeTimeToScale returns the 1-indexed month at which the month-over-month
// revenue growth rate first increases for two consecutive periods, subject to
// both guards being satisfied by those two periods:
//
//   - Monthly revenue ≥ minScaleRevenue (5,000) for both qualifying months.
//   - Month-over-month change ≥ minScaleVariation (2%) for both qualifying months.
//
// The function uses Cash.Years[y].Revenue.Total[m] as the monthly revenue source.
// Returns 0 when no qualifying inflection is found within 36 months, or when
// the guards cannot be satisfied.
//
// When revenue accelerates monotonically from the first month, the result is 1.
func computeTimeToScale(cash model.CashReport) int {
	// Collect flat monthly revenue array (36 months).
	var rev [36]float64
	for y := 0; y < len(cash.Years); y++ {
		for m := 0; m < 12; m++ {
			rev[y*12+m], _ = cash.Years[y].Revenue.Total[m].Float64()
		}
	}

	// Pre-compute month-over-month growth rates.
	// rate[i] = (rev[i] - rev[i-1]) / rev[i-1] for i in 1..35.
	// rate[0] is unused (index 0 = first month, no previous).
	var rate [36]float64
	for i := 1; i < 36; i++ {
		if rev[i-1] > 0 {
			rate[i] = (rev[i] - rev[i-1]) / rev[i-1]
		}
	}

	// Scan for the first i (2 ≤ i ≤ 34) where:
	//   rate[i]   > rate[i-1]   (first  consecutive growth-rate increase)
	//   rate[i+1] > rate[i]     (second consecutive growth-rate increase)
	// with both qualifying months (months i and i+1) passing the revenue and
	// variation guards.
	//
	// TimeToScale = i − 1.  When i=2 (the earliest possible), TimeToScale = 1.
	for i := 2; i <= 34; i++ {
		// Revenue guard: both qualifying months must clear the minimum threshold.
		if rev[i] < minScaleRevenue || rev[i+1] < minScaleRevenue {
			continue
		}
		// Variation guard: both growth rates must exceed the minimum fraction.
		if rate[i] < minScaleVariation || rate[i+1] < minScaleVariation {
			continue
		}
		// Two consecutive increases in growth rate.
		if rate[i] > rate[i-1] && rate[i+1] > rate[i] {
			return i - 1
		}
	}
	return 0
}

// computeTrendSignal derives a single direction label from the pre-computed
// TrendInsights scalars.
//
//	"volatile"      — RevenueVolatility > 0.50 (overrides all other signals)
//	"improving"     — RevenueCAGR > 0.10 AND burn appears to be decreasing
//	                  (proxied by InflectionMonth > 0)
//	"deteriorating" — RevenueCAGR < 0 OR burn is increasing
//	                  (proxied by BurnRateMonthly > 0 AND InflectionMonth = 0)
//	"improving"     — default when no negative signal is present
func computeTrendSignal(insights TrendInsights) string {
	vol, _ := insights.RevenueVolatility.Float64()
	if vol > volatilityThreshold {
		return "volatile"
	}

	cagr, _ := insights.RevenueCAGR.Float64()
	burn, _ := insights.BurnRateMonthly.Float64()

	// "Burn decreasing" proxy: cash flow inflected to positive → burn is winding down.
	burnDecreasing := insights.InflectionMonth > 0
	// "Burn increasing" proxy: there is a burn rate but cash flow never inflected.
	burnIncreasing := burn > 0 && insights.InflectionMonth == 0

	if cagr > cagrImprovingFloor && burnDecreasing {
		return "improving"
	}
	if cagr < 0 || burnIncreasing {
		return "deteriorating"
	}
	return "improving"
}

// computeInsights composes the six derived scalars into a TrendInsights value.
// It is a pure function that reads only from FullPlanOutput.
func computeInsights(output *model.FullPlanOutput) TrendInsights {
	insights := TrendInsights{
		RevenueCAGR:       computeRevenueCAGR(output.PnL),
		BurnRateMonthly:   computeBurnRateMonthly(output.Cash),
		RevenueVolatility: computeRevenueVolatility(output.PnL),
		InflectionMonth:   computeInflectionMonth(output.Cash),
		TimeToScale:       computeTimeToScale(output.Cash),
	}
	// TrendSignal must be computed after the other fields are populated.
	insights.TrendSignal = computeTrendSignal(insights)
	return insights
}

// ──────────────────────────────────────────────────────────────────────────────
// 7. AI-ready highlights (Sprint 6)
// ──────────────────────────────────────────────────────────────────────────────

// buildHighlights derives the narrative layer from pre-computed result fields.
//
// Design rule: ZERO decimal arithmetic is performed here.  All numeric signals
// are pre-computed upstream.  Only comparison methods (.IsNegative, .IsPositive)
// are used — never arithmetic methods (.Add, .Sub, .Mul, .Div).
func buildHighlights(result *ScenarioAnalysisResult) Highlights {
	return Highlights{
		Headline:   buildHeadline(result),
		Strengths:  buildStrengths(result),
		Weaknesses: buildWeaknesses(result),
		TopRisks:   buildTopRisks(result),
		TopDrivers: buildTopDrivers(result),
	}
}

// buildHeadline selects one of 7 template headlines using an ordered decision
// tree over (viability.Status, globalRiskLevel, bepStatus, rootCauseDriver).
//
// Branch map:
//  1. strong  + none/low          → "Healthy scenario: …"
//  2. strong  + moderate/elevated → "Strong viability (N/100) but …"
//  3. moderate + rootCause driver → "[Driver] limits growth: …"
//  4. moderate + no root cause    → "Moderate scenario: …"
//  5. risky   + critical risk     → "Critical: N compounding risks …"
//  6. risky   + BEP approaching   → "Approaching viability but …"
//  7. risky   + other             → "Not fundable under current assumptions …"
func buildHeadline(result *ScenarioAnalysisResult) string {
	v := result.Viability
	rs := result.RiskSummary
	p := result.Projections

	switch v.Status {
	case "strong":
		if rs.GlobalRiskLevel == riskSummaryNone || rs.GlobalRiskLevel == riskSummaryLow {
			// Branch 1: healthy, low-risk scenario.
			driver := "strong fundamentals"
			if len(result.Drivers) > 0 {
				driver = result.Drivers[0].Name
			}
			return fmt.Sprintf("Healthy scenario: %s drives profitability by month %d.", driver, p.BreakEvenMonth)
		}
		// Branch 2: strong viability but notable risks present.
		riskType := "identified risks"
		riskWhen := ""
		if len(result.Risks) > 0 {
			r := result.Risks[0]
			riskType = r.Type
			if r.When != nil {
				riskWhen = fmt.Sprintf(" at month %d", *r.When)
			}
		}
		return fmt.Sprintf("Strong viability (%d/100) but %s poses a risk%s.", v.Score, riskType, riskWhen)

	case "moderate":
		// Branch 3: root cause driver explains the limitation.
		for _, d := range result.Drivers {
			if d.RootCause {
				bepStr := fmt.Sprintf("month %d", p.BreakEvenMonth)
				if p.BreakEvenMonth == 0 {
					bepStr = "not projected within 5 years"
				}
				return fmt.Sprintf("%s limits growth: break-even %s, viability %d/100.", d.Name, bepStr, v.Score)
			}
		}
		// Branch 4: moderate scenario without a dominant root cause.
		return fmt.Sprintf("Moderate scenario: %s at month %d, %d risk(s) identified.", p.BreakEvenStatus, p.BreakEvenMonth, rs.RiskCount)

	default: // "risky"
		// Branch 5: critical aggregate risk.
		if rs.GlobalRiskLevel == riskSummaryCritical {
			return fmt.Sprintf("Critical: %d compounding risks — model requires structural revision before funding.", rs.RiskCount)
		}
		// Branch 6: not yet viable but approaching break-even.
		if p.BreakEvenStatus == BEPApproaching {
			return fmt.Sprintf("Approaching viability but not yet fundable: projected break-even at month %d.", p.BreakEvenMonth)
		}
		// Branch 7: no viable path within the 5-year horizon.
		return "Not fundable under current assumptions: no break-even path within 5 years."
	}
}

// severityRank maps severity strings to a numeric rank for ordering.
// higher number = more severe.
func severityRank(s string) int {
	switch s {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	}
	return 0
}

// topRiskMessages returns up to n risk messages ordered by severity
// (high first), as a string slice.  Returns nil when the input is empty.
func topRiskMessages(risks []Risk, n int) []string {
	if len(risks) == 0 {
		return nil
	}
	// Partition by severity — insertion sort on a small slice.
	ordered := make([]Risk, len(risks))
	copy(ordered, risks)
	for i := 1; i < len(ordered); i++ {
		for j := i; j > 0 && severityRank(ordered[j].Severity) > severityRank(ordered[j-1].Severity); j-- {
			ordered[j], ordered[j-1] = ordered[j-1], ordered[j]
		}
	}
	limit := len(ordered)
	if limit > n {
		limit = n
	}
	msgs := make([]string, limit)
	for i := 0; i < limit; i++ {
		msgs[i] = ordered[i].Message
	}
	return msgs
}

// buildStrengths collects positive signals: high-impact root-cause driver
// names (max 2), supplemented by projection signals.  Fallback if none found.
// No decimal arithmetic: uses only .IsNegative() and .IsPositive() comparisons.
func buildStrengths(result *ScenarioAnalysisResult) []string {
	var strengths []string

	// High-impact root-cause drivers (capped at 2).
	for _, d := range result.Drivers {
		if len(strengths) >= 2 {
			break
		}
		if d.Impact == "high" && d.RootCause {
			strengths = append(strengths, d.Name)
		}
	}

	// Supplement with projection signals when fewer than 2 driver strengths.
	if len(strengths) < 2 && !result.Projections.CashMin.IsNegative() {
		strengths = append(strengths, "Cash position is positive throughout the 5-year horizon.")
	}
	if len(strengths) < 2 && result.Projections.EbitdaPeak.IsPositive() {
		strengths = append(strengths, "EBITDA turns positive during the plan period.")
	}

	if len(strengths) == 0 {
		return []string{"No dominant positive signal detected — scenario appears balanced but inconclusive."}
	}
	return strengths
}

// buildWeaknesses returns the top 2 risk messages severity-ordered.
// Fallback if no risks exist.
func buildWeaknesses(result *ScenarioAnalysisResult) []string {
	msgs := topRiskMessages(result.Risks, 2)
	if len(msgs) == 0 {
		return []string{"No structural risks detected — scenario appears stable under current assumptions."}
	}
	return msgs
}

// buildTopRisks returns the top 2 risk messages severity-ordered.
// Fallback if no risks exist.
func buildTopRisks(result *ScenarioAnalysisResult) []string {
	msgs := topRiskMessages(result.Risks, 2)
	if len(msgs) == 0 {
		return []string{"No risks flagged — model appears robust at current assumptions."}
	}
	return msgs
}

// buildTopDrivers returns the Effect strings of the top 2 ranked drivers.
// Fallback if no drivers exist.
func buildTopDrivers(result *ScenarioAnalysisResult) []string {
	if len(result.Drivers) == 0 {
		return []string{"No dominant signal detected — scenario appears balanced but inconclusive."}
	}
	limit := len(result.Drivers)
	if limit > 2 {
		limit = 2
	}
	effects := make([]string, limit)
	for i := 0; i < limit; i++ {
		effects[i] = result.Drivers[i].Effect
	}
	return effects
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

func intPtr(v int) *int { return &v }

// ──────────────────────────────────────────────────────────────────────────────
// i18n post-processing
//
// These functions run AFTER the core analysis is complete and populate the
// optional *Localized fields on Risk, Driver, and Highlights structs.
// They are no-ops when lang == "en" (base fields are already English).
// ──────────────────────────────────────────────────────────────────────────────

// populateRiskLocalizations fills Risk.MessageLocalized for each risk using
// the stable risk type as a lookup key into the translation map.
// The function inspects each risk's type and reconstructs the template params
// from the already-computed Risk fields (Value, When) — no new arithmetic.
func populateRiskLocalizations(risks []Risk, lang string, p ProjectionResult) {
	for i := range risks {
		r := &risks[i]
		var key string
		var params map[string]interface{}

		switch r.Type {
		case "cash_gap":
			key = "cash_gap"
			params = map[string]interface{}{
				"cash":  p.CashMin.StringFixed(0),
				"month": p.CashMinMonth,
			}
		case "late_profitability":
			if r.When != nil {
				key = "late_profitability_late"
				params = map[string]interface{}{"month": *r.When}
			} else {
				key = "late_profitability_none"
			}
		case "no_profitability":
			key = "no_profitability"
		case "unrealistic_growth":
			key = "unrealistic_growth"
			params = map[string]interface{}{"pct": fmt.Sprintf("%.0f", p.MaxYoYGrowthPct)}
		case "high_leverage":
			// Distinguish no-FCF branch (Urgency == "long_term" with r.Value > debt-threshold)
			// by checking whether original message contained "cannot be serviced" — but
			// to avoid string parsing we re-derive from p (same logic as detectRisks).
			// If FCF was ≤ 0 at the time of risk detection the message had no ratio value;
			// we detect this branch by checking if the formatted Value appears in both messages.
			// Simpler: re-compute like detectRisks did, using p which we have.
			// We don't have the raw FCF/netDebt here, but we CAN discriminate by Severity:
			// no_fcf branch always produces Severity=="high"; elevated branch == "medium".
			if r.Severity == "high" {
				key = "high_leverage_no_fcf"
				params = map[string]interface{}{"debt": r.Value.StringFixed(0)}
			} else {
				// Extract ratio from the English message is fragile; re-compute is cleaner.
				// We store the Value (netDebt5) in the Risk. We can't recover FCF here
				// without the output, so we use a generic param placeholder.
				// The ratio string is embedded in the English message — parse it via
				// a safe fallback: produce a generic localized message without the ratio.
				// Full ratio localisation requires passing output to this function, which
				// we avoid to keep the signature minimal. Accept this as a known limitation:
				// the high_leverage_elevated message will be localised without the ratio value.
				key = "high_leverage_elevated"
				params = map[string]interface{}{"ratio": "N/A"} // see note above
			}
		default:
			// Unknown risk type — skip localisation.
			continue
		}

		r.MessageLocalized = localize(lang, key, params)
	}
}

// populateRiskLocalizationsWithOutput is a richer variant that restores the
// leverage ratio by re-deriving it from FullPlanOutput.  It is called from
// AnalyzeScenario when the output is available.
func populateRiskLocalizationsWithOutput(
	risks []Risk,
	lang string,
	p ProjectionResult,
	netDebt5 decimal.Decimal,
	fcf5 decimal.Decimal,
) {
	// Use populateRiskLocalizations for all types; then fix up high_leverage.
	populateRiskLocalizations(risks, lang, p)

	for i := range risks {
		r := &risks[i]
		if r.Type != "high_leverage" || r.Severity != "medium" {
			continue
		}
		nd, _ := netDebt5.Float64()
		fcf, _ := fcf5.Float64()
		if fcf > 0 {
			ratio := nd / fcf
			r.MessageLocalized = localize(lang, "high_leverage_elevated", map[string]interface{}{
				"ratio": fmt.Sprintf("%.1f", ratio),
			})
		}
	}
}

// populateDriverLocalizations fills Driver.EffectLocalized for each driver
// using its stable Code as a lookup key.
// The numeric params are extracted from the English Effect string — to avoid
// re-running the heuristics, we re-derive the fmt param by reverse-parsing is
// fragile, so instead we call fmt.Sprintf with the same numeric signals.
// Since we don't have the raw signals here, we store them inline in the Code
// via a separate approach: the Effect already contains the formatted numbers,
// so we use the Driver.Code to determine the template and pass numeric params
// that were stored in a parallel effectParams map built during identifyDrivers.
//
// Simpler alternative used here: carry params alongside the code in candidateDriver.
// The helper is called from AnalyzeScenario with a pre-built params table.
func populateDriverLocalizations(drivers []Driver, lang string, params map[string]map[string]interface{}) {
	for i := range drivers {
		d := &drivers[i]
		if d.Code == "" {
			continue
		}
		effectKey := "effect_" + d.Code
		p := params[d.Code]
		d.EffectLocalized = localize(lang, effectKey, p)
	}
}

// driverLocalizedName returns the localised driver name for the given code.
// Used when building localised Highlights.Strengths.
func driverLocalizedName(code, lang string) string {
	if code == "" {
		return ""
	}
	return localize(lang, "driver_"+code, nil)
}

// buildHighlightsLocalized builds the *Localized variants of Highlights fields.
// It is called after buildHighlights and populateDriverLocalizations.
func buildHighlightsLocalized(result *ScenarioAnalysisResult, lang string) {
	h := &result.Highlights

	// ── HeadlineLocalized ────────────────────────────────────────────────────
	h.HeadlineLocalized = buildHeadlineLocalized(result, lang)

	// ── StrengthsLocalized ───────────────────────────────────────────────────
	h.StrengthsLocalized = buildStrengthsLocalized(result, lang)

	// ── WeaknessesLocalized ──────────────────────────────────────────────────
	h.WeaknessesLocalized = buildWeaknessesLocalized(result, lang)
}

// buildHeadlineLocalized re-runs the headline decision tree using localised templates.
func buildHeadlineLocalized(result *ScenarioAnalysisResult, lang string) string {
	v := result.Viability
	rs := result.RiskSummary
	p := result.Projections

	switch v.Status {
	case "strong":
		if rs.GlobalRiskLevel == riskSummaryNone || rs.GlobalRiskLevel == riskSummaryLow {
			driver := localize(lang, "strong_fundamentals", nil) // fallback when no drivers
			if len(result.Drivers) > 0 {
				d := result.Drivers[0]
				driver = driverLocalizedName(d.Code, lang)
				if driver == "" {
					driver = d.Name
				}
			}
			return localize(lang, "headline_healthy", map[string]interface{}{
				"driver": driver,
				"bep":    p.BreakEvenMonth,
			})
		}
		riskType := "identified risks"
		riskWhen := ""
		if len(result.Risks) > 0 {
			r := result.Risks[0]
			riskType = r.Type
			if r.When != nil {
				riskWhen = fmt.Sprintf(" at month %d", *r.When)
			}
		}
		return localize(lang, "headline_strong_risk", map[string]interface{}{
			"score":     v.Score,
			"risk_type": riskType,
			"risk_when": riskWhen,
		})

	case "moderate":
		for _, d := range result.Drivers {
			if d.RootCause {
				bepStr := fmt.Sprintf("month %d", p.BreakEvenMonth)
				if p.BreakEvenMonth == 0 {
					// Localise "not projected within 5 years" — use English for now (no key defined).
					bepStr = "not projected within 5 years"
				}
				driverName := driverLocalizedName(d.Code, lang)
				if driverName == "" {
					driverName = d.Name
				}
				return localize(lang, "headline_moderate_root", map[string]interface{}{
					"driver":  driverName,
					"bep_str": bepStr,
					"score":   v.Score,
				})
			}
		}
		return localize(lang, "headline_moderate_no_root", map[string]interface{}{
			"bep_status": p.BreakEvenStatus,
			"bep":        p.BreakEvenMonth,
			"risk_count": rs.RiskCount,
		})

	default: // "risky"
		if rs.GlobalRiskLevel == riskSummaryCritical {
			return localize(lang, "headline_critical", map[string]interface{}{
				"risk_count": rs.RiskCount,
			})
		}
		if p.BreakEvenStatus == BEPApproaching {
			return localize(lang, "headline_approaching", map[string]interface{}{
				"bep": p.BreakEvenMonth,
			})
		}
		return localize(lang, "headline_not_fundable", nil)
	}
}

// buildStrengthsLocalized builds the localised Strengths slice.
func buildStrengthsLocalized(result *ScenarioAnalysisResult, lang string) []string {
	var strengths []string

	for _, d := range result.Drivers {
		if len(strengths) >= 2 {
			break
		}
		if d.Impact == "high" && d.RootCause {
			name := driverLocalizedName(d.Code, lang)
			if name == "" {
				name = d.Name
			}
			strengths = append(strengths, name)
		}
	}

	if len(strengths) < 2 && !result.Projections.CashMin.IsNegative() {
		strengths = append(strengths, localize(lang, "hl_cash_positive", nil))
	}
	if len(strengths) < 2 && result.Projections.EbitdaPeak.IsPositive() {
		strengths = append(strengths, localize(lang, "hl_ebitda_positive", nil))
	}
	if len(strengths) == 0 {
		return []string{localize(lang, "hl_no_signal", nil)}
	}
	return strengths
}

// buildWeaknessesLocalized builds the localised Weaknesses slice using
// Risk.MessageLocalized where available, falling back to Risk.Message.
func buildWeaknessesLocalized(result *ScenarioAnalysisResult, lang string) []string {
	if len(result.Risks) == 0 {
		return []string{localize(lang, "hl_weaknesses_none", nil)}
	}
	ordered := make([]Risk, len(result.Risks))
	copy(ordered, result.Risks)
	for i := 1; i < len(ordered); i++ {
		for j := i; j > 0 && severityRank(ordered[j].Severity) > severityRank(ordered[j-1].Severity); j-- {
			ordered[j], ordered[j-1] = ordered[j-1], ordered[j]
		}
	}
	limit := len(ordered)
	if limit > 2 {
		limit = 2
	}
	msgs := make([]string, limit)
	for i := 0; i < limit; i++ {
		if ordered[i].MessageLocalized != "" {
			msgs[i] = ordered[i].MessageLocalized
		} else {
			msgs[i] = ordered[i].Message
		}
	}
	return msgs
}

// driverEffectParamsFromEffect extracts the numeric argument from a pre-formatted
// effect string for a given driver code. This is used to re-build localised
// effect text without re-running the full heuristics.
//
// Since the numbers are already embedded in the English Effect string via
// fmt.Sprintf("%.0f %%", x), this helper uses a parallel params-capture
// approach: identifyDrivers now stores params in candidateDriver.effectParams.
// This function is a companion to that change.
// NOTE: The params are passed directly from AnalyzeScenario via the effectParams map.
func driverEffectParams(code string, params map[string]map[string]interface{}) map[string]interface{} {
	if params == nil {
		return nil
	}
	return params[code]
}
