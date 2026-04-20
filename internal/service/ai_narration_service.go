// Package service — AI financial narration service.
// Architecture ported from GPWA internal/service/ai_narration_service.go and
// adapted for Ascenda financial intelligence.
//
// # IMPORTANT BOUNDARIES
//
//   - The narrator EXPLAINS financial data — it does NOT make business decisions
//   - The narrator uses ONLY data from NarrationContext — NO database access
//   - NO business logic recalculation inside this service
//   - Fallback narration is always deterministic (no AI required)
//   - Prompts are designed to produce deterministic JSON — never free prose
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"ascenda/internal/config"
	aiClient "ascenda/internal/pkg/ai"
	"github.com/ovander/backendkit/ctxutil"
)

// ============================================================================
// Types — Narration Context
// ============================================================================

// NarrationUserRole describes the audience for the narration.
type NarrationUserRole string

const (
	NarrationRoleAdmin  NarrationUserRole = "admin"
	NarrationRoleOwner  NarrationUserRole = "owner"
	NarrationRoleUser   NarrationUserRole = "user"
	NarrationRoleViewer NarrationUserRole = "viewer"
)

// NarrationType identifies the kind of financial narration requested.
type NarrationType string

const (
	// ── Standard-tier narration types ────────────────────────────────────────
	NarrationTypePlanSummary        NarrationType = "plan_summary"
	NarrationTypeVarianceAnalysis   NarrationType = "variance_analysis"
	NarrationTypeScenarioComparison NarrationType = "scenario_comparison"
	NarrationTypeAnomalyDetection   NarrationType = "anomaly_detection"
	NarrationTypeCashRunway         NarrationType = "cash_runway"

	// ── Pro-tier narration types (driver-aware intelligence) ─────────────────
	NarrationTypeUnitEconomics       NarrationType = "unit_economics"
	NarrationTypeAssumptionReview    NarrationType = "assumption_review"
	NarrationTypeBenchmarkCommentary NarrationType = "benchmark_commentary"
	NarrationTypePortfolioMix        NarrationType = "portfolio_mix"
	NarrationTypeDriverAdvisor       NarrationType = "driver_advisor"
	NarrationTypeScenarioSuggestion  NarrationType = "scenario_suggestion"
	NarrationTypeSensitivityNarrative NarrationType = "sensitivity_narrative"

	// ── Enterprise narration type ─────────────────────────────────────────────
	NarrationTypeInvestorMemo NarrationType = "investor_memo"
)

// FinancialMetric holds a single labelled numeric metric.
type FinancialMetric struct {
	Label    string  `json:"label"`
	Value    float64 `json:"value"`
	Currency string  `json:"currency,omitempty"` // e.g. "EUR", "USD"
	Unit     string  `json:"unit,omitempty"`     // e.g. "%", "months"
	Note     string  `json:"note,omitempty"`
}

// VarianceLine captures a single budget vs actual line.
type VarianceLine struct {
	Label    string  `json:"label"`
	Budget   float64 `json:"budget"`
	Actual   float64 `json:"actual"`
	Variance float64 `json:"variance"`          // actual - budget
	VarPct   float64 `json:"variance_pct"`      // (actual - budget) / |budget| × 100
	Favourable bool  `json:"favourable"`
}

// ScenarioSummary holds key metrics for one scenario (used in comparisons).
type ScenarioSummary struct {
	ScenarioName string            `json:"scenario_name"`
	IsDefault    bool              `json:"is_default"`
	Metrics      []FinancialMetric `json:"metrics"`
}

// ProductEconomics holds driver-specific per-product economic data for Pro narrations.
type ProductEconomics struct {
	ProductName string                 `json:"product_name"`
	DriverType  string                 `json:"driver_type"` // saas, consulting, marketplace, industry, media, generic
	Metrics     []FinancialMetric      `json:"metrics"`     // driver-specific KPIs (ARPU, take rate, eCPM, etc.)
	DriverParams map[string]interface{} `json:"driver_params,omitempty"`
}

// SensitivityLever represents one sensitivity analysis lever and its P&L impact.
type SensitivityLever struct {
	LeverName     string  `json:"lever_name"`
	DriverType    string  `json:"driver_type"`
	BaseValue     float64 `json:"base_value"`
	StressValue   float64 `json:"stress_value"`
	RevenueImpact float64 `json:"revenue_impact"`  // absolute change in revenue
	EBITDAImpact  float64 `json:"ebitda_impact"`   // absolute change in EBITDA
	Unit          string  `json:"unit,omitempty"`  // "%", "€", "units", etc.
}

// AssumptionFlag is a driver-specific flagged assumption requiring review.
type AssumptionFlag struct {
	ProductName string      `json:"product_name"`
	DriverType  string      `json:"driver_type"`
	FieldName   string      `json:"field_name"`
	Value       interface{} `json:"value"`
	Reason      string      `json:"reason"` // why it is flagged (e.g. "ARPU > 3× industry benchmark")
}

// ScenarioParamDiff represents a single parameter change suggested for a scenario variant.
type ScenarioParamDiff struct {
	ProductName    string      `json:"product_name"`
	DriverType     string      `json:"driver_type"`
	ParamName      string      `json:"param_name"`
	CurrentValue   interface{} `json:"current_value"`
	SuggestedValue interface{} `json:"suggested_value"`
	Rationale      string      `json:"rationale"`
}

// NarrationContext carries all the financial data the AI narrator needs.
// No database access is allowed inside AINarrationService — all data must
// be populated by the calling handler/service before passing it here.
type NarrationContext struct {
	// Identification
	PlanName     string            `json:"plan_name"`
	ScenarioName string            `json:"scenario_name"`
	PeriodLabel  string            `json:"period_label"` // e.g. "FY 2026", "Q1 2026"
	Currency     string            `json:"currency"`     // e.g. "EUR"
	UserRole     NarrationUserRole `json:"user_role"`

	// Language is the BCP-47 language tag in which the narration should be
	// produced.  Defaults to "en" when empty or unsupported.
	// Callers should populate this from PlanConfig.Language so that AI output
	// matches the user's configured language preference.
	Language string `json:"language,omitempty"` // NEW — "en" | "fr" | …

	// Narration type (optional — inferred if empty)
	NarrationType NarrationType `json:"narration_type,omitempty"`

	// Core P&L metrics
	Revenue     *FinancialMetric `json:"revenue,omitempty"`
	GrossProfit *FinancialMetric `json:"gross_profit,omitempty"`
	EBITDA      *FinancialMetric `json:"ebitda,omitempty"`
	NetIncome   *FinancialMetric `json:"net_income,omitempty"`

	// Cash metrics
	CashPosition     *FinancialMetric `json:"cash_position,omitempty"`
	CashRunwayMonths *FinancialMetric `json:"cash_runway_months,omitempty"`
	BurnRate         *FinancialMetric `json:"burn_rate,omitempty"`

	// Additional KPIs (optional)
	KeyMetrics []FinancialMetric `json:"key_metrics,omitempty"`

	// Variance data (for NarrationTypeVarianceAnalysis)
	VarianceLines []VarianceLine `json:"variance_lines,omitempty"`

	// Scenario comparison data (for NarrationTypeScenarioComparison)
	Scenarios []ScenarioSummary `json:"scenarios,omitempty"`

	// Anomalies already flagged by the compute engine (for NarrationTypeAnomalyDetection)
	Anomalies []string `json:"anomalies,omitempty"`

	// ── Pro-tier driver-aware fields ──────────────────────────────────────────

	// PrimaryDriverType is the dominant driver for the plan (saas, consulting, etc.).
	// Used for benchmark and unit-economics narrations.
	PrimaryDriverType string `json:"primary_driver_type,omitempty"`

	// ProductEconomics holds per-product driver-specific KPIs (unit economics, portfolio mix).
	ProductEconomics []ProductEconomics `json:"product_economics,omitempty"`

	// AssumptionFlags holds pre-computed flagged assumptions (assumption review).
	AssumptionFlags []AssumptionFlag `json:"assumption_flags,omitempty"`

	// SensitivityLevers holds pre-computed sensitivity levers (sensitivity narrative).
	SensitivityLevers []SensitivityLever `json:"sensitivity_levers,omitempty"`

	// ScenarioType qualifies the scenario being suggested: "bear", "bull", "stress".
	// Used for NarrationTypeScenarioSuggestion.
	ScenarioType string `json:"scenario_type,omitempty"`
}

// ============================================================================
// Types — Narration Output
// ============================================================================

// NarrationParagraph is one section of the narration.
type NarrationParagraph struct {
	Heading string `json:"heading,omitempty"`
	Content string `json:"content"`
	Type    string `json:"type"` // "info", "positive", "warning", "neutral"
}

// NarrationOutput is the final structured result returned to the caller.
type NarrationOutput struct {
	Title         string                 `json:"title"`
	Summary       string                 `json:"summary"`
	Paragraphs    []NarrationParagraph   `json:"paragraphs"`
	KeyTakeaways  []string               `json:"key_takeaways"`
	IsAIGenerated bool                   `json:"is_ai_generated"`
	// StructuredData holds machine-readable output for features that produce
	// structured results (e.g. ScenarioSuggestion returns param diffs).
	StructuredData map[string]interface{} `json:"structured_data,omitempty"`
}

// ============================================================================
// Service
// ============================================================================

// AINarrationService generates human-readable financial narrations.
// It wraps the shared AIClient and enforces strict narrator boundaries.
type AINarrationService struct {
	client *aiClient.Client
	log    *logrus.Entry
	cache  NarrationCacher // nil = caching disabled
}

// NewAINarrationService creates a new AINarrationService with a pure in-memory
// cache (LRU 200 entries, 2-hour TTL).  Use NewAINarrationServiceWithCache to
// inject a DB-backed or layered cache instead.
func NewAINarrationService(aiCfg config.AIConfig, log *logrus.Entry) *AINarrationService {
	return &AINarrationService{
		client: aiClient.NewClient(aiCfg, log),
		log:    log,
		cache:  NewNarrationCache(DefaultNarrationCacheConfig()),
	}
}

// NewAINarrationServiceWithCache creates an AINarrationService using the
// supplied cache implementation.  Pass a *LayeredNarrationCache in production
// to get both in-memory speed and DB persistence.
func NewAINarrationServiceWithCache(aiCfg config.AIConfig, log *logrus.Entry, cache NarrationCacher) *AINarrationService {
	return &AINarrationService{
		client: aiClient.NewClient(aiCfg, log),
		log:    log,
		cache:  cache,
	}
}

// SetCache replaces the narration cache. Pass nil to disable caching entirely.
// Accepts any NarrationCacher implementation (in-memory, DB, layered, mock).
func (s *AINarrationService) SetCache(c NarrationCacher) {
	s.cache = c
}

// IsConfigured returns true when the underlying AI client has an API key.
func (s *AINarrationService) IsConfigured() bool {
	return s.client.IsConfigured()
}

// ============================================================================
// GenerateNarration — main entry point
// ============================================================================

// GenerateNarration creates a human-readable explanation of financial data.
// Falls back to a deterministic narrative when AI is unavailable or fails.
//
// Cache behaviour:
//   - The cache key is derived from the full NarrationContext content hash
//     (featureType:role:sha256[:16]).  If any input value changes the hash
//     changes and the cached entry is never served.
//   - Only AI-generated responses are cached; deterministic fallbacks are not
//     (they are cheap to produce and should reflect the "no AI configured" state).
//   - Cache hits return the stored NarrationOutput as-is (IsAIGenerated = true,
//     same payload the AI originally produced).
func (s *AINarrationService) GenerateNarration(ctx context.Context, nCtx *NarrationContext) (*NarrationOutput, error) {
	if nCtx == nil {
		return nil, fmt.Errorf("narration context is required")
	}
	if nCtx.PlanName == "" {
		return nil, fmt.Errorf("plan name is required in narration context")
	}

	// Infer narration type when not specified (must happen before cache key computation
	// so that inferred types are cached under the correct key).
	narrationType := nCtx.NarrationType
	if narrationType == "" {
		narrationType = inferNarrationType(nCtx)
		// Write back so the cache key captures the resolved type.
		nCtx.NarrationType = narrationType
	}

	// Tenant ID is required for DB-scoped cache storage.
	tenantID := ctxutil.GetTenantID(ctx)
	if tenantID == uuid.Nil {
		// Platform admins (no tenant) or tests without a tenant context: still
		// allow narration but use a sentinel so cache rows are distinguishable.
		tenantID = uuid.MustParse("00000000-0000-0000-0000-000000000000")
	}

	// ── Cache lookup ─────────────────────────────────────────────────────────
	var cacheKey string
	if s.cache != nil {
		cacheKey = NarrationCacheKey(nCtx)
		if cached, ok := s.cache.Get(tenantID, cacheKey); ok {
			s.log.WithField("cache_key", cacheKey).Debug("AI narration cache hit")
			return cached, nil
		}
	}

	// ── Fallback path (no AI configured) ─────────────────────────────────────
	if !s.client.IsConfigured() {
		s.log.Warn("AI not configured — using deterministic fallback narration")
		return s.generateFallback(nCtx, narrationType), nil
	}

	// ── AI call ───────────────────────────────────────────────────────────────
	prompt := s.buildPrompt(nCtx, narrationType)
	aiResponse, err := s.client.Call(ctx, prompt)
	if err != nil {
		s.log.WithError(err).Warn("AI narration call failed — using deterministic fallback")
		return s.generateFallback(nCtx, narrationType), nil
	}

	output, err := s.parseAIResponse(aiResponse, nCtx, narrationType)
	if err != nil {
		s.log.WithError(err).Warn("failed to parse AI narration response — using fallback")
		return s.generateFallback(nCtx, narrationType), nil
	}

	output.IsAIGenerated = true

	// ── Cache store ───────────────────────────────────────────────────────────
	if s.cache != nil && cacheKey != "" {
		s.cache.Put(tenantID, cacheKey, output)
		s.log.WithField("cache_key", cacheKey).Debug("AI narration result cached")
	}

	return output, nil
}

// ============================================================================
// Prompt Building
// ============================================================================

func (s *AINarrationService) buildPrompt(nCtx *NarrationContext, nt NarrationType) string {
	var sb strings.Builder
	sb.WriteString(systemPrompt(nCtx.UserRole, normalizeLanguage(nCtx.Language)))
	sb.WriteString("\n\n=== FINANCIAL DATA (use ONLY this data — do not invent figures) ===\n")
	sb.WriteString(buildDataContext(nCtx))
	sb.WriteString("\n\n=== NARRATION REQUEST ===\n")
	sb.WriteString(narrationInstructions(nt, nCtx.UserRole))
	return sb.String()
}

// systemPrompt returns the strict system prompt for the narrator role.
// lang is the normalised 2-letter language code ("en", "fr", …).
// When lang != "en", a language instruction is appended to force the model
// to produce output in the requested language.
func systemPrompt(role NarrationUserRole, lang string) string {
	base := `You are a financial narrator for Ascenda, a business planning application.

CRITICAL RULES:
1. You EXPLAIN financial data that has already been computed — you do NOT make business decisions
2. You use ONLY the data provided — do NOT invent figures, percentages, or projections
3. You do NOT suggest strategy changes not supported by the provided data
4. You do NOT override or contradict figures in the data
5. Your role is to translate financial metrics into clear, human-friendly language

OUTPUT FORMAT:
Respond with a single JSON object — no markdown, no prose outside the JSON:
{
  "title": "short descriptive title",
  "summary": "1–2 sentence executive summary",
  "paragraphs": [
    {"heading": "optional section heading", "content": "paragraph text", "type": "info|positive|warning|neutral"}
  ],
  "key_takeaways": ["concise bullet 1", "concise bullet 2", "concise bullet 3"]
}`

	// Language instruction: appended after role / tone section so the language
	// constraint is processed last (highest priority for the model).
	langInstruction := ""
	switch lang {
	case "fr":
		langInstruction = "\n\nLANGUAGE: Respond strictly in French (fr). All title, summary, paragraphs, and key_takeaways MUST be in French."
	case "en", "":
		// default — no extra instruction needed
	default:
		langInstruction = fmt.Sprintf("\n\nLANGUAGE: Respond strictly in %s. All output fields MUST be in that language.", lang)
	}

	switch role {
	case NarrationRoleAdmin, NarrationRoleOwner:
		return base + `

AUDIENCE: Business owner / plan administrator
TONE: Analytical, precise, executive-level. Include specific figures. Highlight risks and opportunities. Use professional financial language.` + langInstruction
	case NarrationRoleUser:
		return base + `

AUDIENCE: Team member / plan contributor
TONE: Clear, structured, informative. Explain what the figures mean for the team. Avoid overly technical jargon while keeping precision.` + langInstruction
	case NarrationRoleViewer:
		return base + `

AUDIENCE: Read-only stakeholder
TONE: High-level, accessible, balanced. Focus on the overall story. Avoid sensitive internal details. Keep it concise and non-technical.` + langInstruction
	default:
		return base + langInstruction
	}
}

// buildDataContext serialises the NarrationContext fields into a compact JSON block.
func buildDataContext(nCtx *NarrationContext) string {
	data := map[string]interface{}{
		"plan":         nCtx.PlanName,
		"scenario":     nCtx.ScenarioName,
		"period":       nCtx.PeriodLabel,
		"currency":     nCtx.Currency,
	}

	if nCtx.Revenue != nil {
		data["revenue"] = nCtx.Revenue
	}
	if nCtx.GrossProfit != nil {
		data["gross_profit"] = nCtx.GrossProfit
	}
	if nCtx.EBITDA != nil {
		data["ebitda"] = nCtx.EBITDA
	}
	if nCtx.NetIncome != nil {
		data["net_income"] = nCtx.NetIncome
	}
	if nCtx.CashPosition != nil {
		data["cash_position"] = nCtx.CashPosition
	}
	if nCtx.CashRunwayMonths != nil {
		data["cash_runway_months"] = nCtx.CashRunwayMonths
	}
	if nCtx.BurnRate != nil {
		data["burn_rate"] = nCtx.BurnRate
	}
	if len(nCtx.KeyMetrics) > 0 {
		data["key_metrics"] = nCtx.KeyMetrics
	}
	if len(nCtx.VarianceLines) > 0 {
		data["variance_lines"] = nCtx.VarianceLines
	}
	if len(nCtx.Scenarios) > 0 {
		data["scenarios"] = nCtx.Scenarios
	}
	if len(nCtx.Anomalies) > 0 {
		data["anomalies"] = nCtx.Anomalies
	}

	// Pro-tier driver-aware fields
	if nCtx.PrimaryDriverType != "" {
		data["primary_driver_type"] = nCtx.PrimaryDriverType
	}
	if len(nCtx.ProductEconomics) > 0 {
		data["product_economics"] = nCtx.ProductEconomics
	}
	if len(nCtx.AssumptionFlags) > 0 {
		data["assumption_flags"] = nCtx.AssumptionFlags
	}
	if len(nCtx.SensitivityLevers) > 0 {
		data["sensitivity_levers"] = nCtx.SensitivityLevers
	}
	if nCtx.ScenarioType != "" {
		data["scenario_type"] = nCtx.ScenarioType
	}

	b, _ := json.MarshalIndent(data, "", "  ")
	return string(b)
}

// narrationInstructions returns task-specific instructions for the AI.
func narrationInstructions(nt NarrationType, role NarrationUserRole) string {
	switch nt {
	// ── Standard-tier narration types ─────────────────────────────────────────
	case NarrationTypeVarianceAnalysis:
		return `Generate a variance analysis narration.
Focus on: which lines are over/under budget, the magnitude of variances, whether they are favourable or unfavourable, and any patterns across the data.
Do NOT explain causes not present in the data.`

	case NarrationTypeScenarioComparison:
		return `Generate a scenario comparison narration.
Focus on: key differences in financial outcomes between the scenarios, which scenario performs better on each major KPI, and trade-offs.
Do NOT recommend which scenario to choose.`

	case NarrationTypeAnomalyDetection:
		return `Generate an anomaly detection narration.
Focus on: what anomalies were flagged, why they stand out against the surrounding data, and their potential financial significance.
Do NOT diagnose root causes not present in the data.`

	case NarrationTypeCashRunway:
		return `Generate a cash runway narration.
Focus on: current cash position, burn rate, projected runway in months, and whether the outlook is comfortable, cautious, or critical.
Do NOT provide investment or fundraising advice.`

	// ── Pro-tier driver-aware narration types ──────────────────────────────────
	case NarrationTypeUnitEconomics:
		return `Generate a unit economics narration using the product_economics data.
For each product, explain its driver-specific KPIs (e.g. ARPU for SaaS, revenue-per-consultant for consulting, take rate for marketplace, eCPM for media).
Focus on: which products have the strongest unit economics, how they contribute to overall revenue, and any notable KPI trends.
Do NOT benchmark against external data not present in the context.`

	case NarrationTypeAssumptionReview:
		return `Generate an assumption review narration using the assumption_flags data.
For each flagged assumption: explain why it stands out, what it means for the plan if correct, and what it would mean if it is an error.
Rank the flags by financial significance.
Do NOT suggest corrections — only explain the implications.`

	case NarrationTypeBenchmarkCommentary:
		return `Generate a benchmark commentary narration.
Using the primary_driver_type and product_economics data, comment on how the plan's KPIs compare to typical industry benchmarks for that driver type.
For SaaS: reference CAC payback, churn, ARR growth. For consulting: utilisation rate, revenue-per-FTE. For marketplace: take rate, GMV growth. For media: eCPM, ARPU. For industry: gross margin, inventory turn. For session_based: fill rate, revenue-per-session, trainer utilisation, participant yield. For generic: revenue growth, EBITDA margin.
Be explicit that benchmarks are general references, not guarantees.`

	case NarrationTypePortfolioMix:
		return `Generate a portfolio mix narration using the product_economics data.
Focus on: revenue-mix evolution across driver types, margin contribution by product, and which products drive growth vs. which are mature.
Highlight any concentration risk (e.g. one product > 80% of revenue) or diversification opportunities visible in the data.
Do NOT recommend adding or removing products.`

	case NarrationTypeDriverAdvisor:
		return `Generate a driver advisor narration.
Based on the product_economics data and any generic-driver products present, suggest which structured driver type (saas, consulting, marketplace, industry, media, session_based) best fits each generic product.
Use session_based when the product delivers discrete sessions or events (training, workshops, seminars) where revenue scales with participants × fill rate and cost splits between fixed-per-session and variable-per-participant.
Explain the reasoning for each recommendation using the available assumption and KPI data.
Structure your output JSON with an additional "structured_data" field containing an object: {"recommendations": [{"product_name": "...", "recommended_driver": "...", "confidence": "high|medium|low", "rationale": "..."}]}.`

	case NarrationTypeScenarioSuggestion:
		return `Generate a scenario suggestion narration.
Based on the key_metrics, product_economics, and scenario_type ("bear", "bull", or "stress"), suggest a coherent set of parameter modifications.
For each change: name the parameter, the product, the direction and magnitude of change, and the business rationale.
Structure your output JSON with an additional "structured_data" field containing {"scenario_type": "...", "diffs": [{"product_name": "...", "driver_type": "...", "param_name": "...", "current_value": ..., "suggested_value": ..., "rationale": "..."}]}.
Do NOT modify structural assumptions (e.g. number of years, currency). Only suggest plausible operating parameter changes.`

	case NarrationTypeSensitivityNarrative:
		return `Generate a sensitivity narrative using the sensitivity_levers data.
Rank the levers by absolute EBITDA impact. For the top 3–5 levers, explain: what the lever is, what drives it, the magnitude of its impact, and whether it is primarily a revenue or cost driver.
Do NOT recommend specific lever values. Only explain the data provided.`

	// ── Enterprise narration type ──────────────────────────────────────────────
	case NarrationTypeInvestorMemo:
		return `Generate a full investor-ready plan narrative with the following sections:
1. Business Model — describe the revenue model and driver types present in the plan
2. Unit Economics — highlight key per-unit or per-customer metrics
3. Financial Projections — summarise revenue, EBITDA, and cash trajectory
4. Key Risks — identify the top 2–3 financial risks visible in the data
5. Investment Highlights — 3 concise bullet points on why the plan is compelling
Be analytical and investor-grade. Use precise figures from the data. Do NOT use speculative language.`

	// ── Default: standard plan summary ────────────────────────────────────────
	default:
		switch role {
		case NarrationRoleOwner, NarrationRoleAdmin:
			return `Generate an executive plan summary narration.
Cover: revenue trajectory, profitability (gross profit, EBITDA, net income), cash position, and the top 2–3 strategic highlights.`
		case NarrationRoleViewer:
			return `Generate a high-level plan overview narration.
Cover: the overall direction of the plan and whether the key financial goals appear to be on track. Keep it concise (2–3 paragraphs).`
		default:
			return `Generate a plan summary narration covering the key financial metrics and their significance for the team.`
		}
	}
}

// ============================================================================
// AI Response Parsing
// ============================================================================

func (s *AINarrationService) parseAIResponse(raw string, nCtx *NarrationContext, nt NarrationType) (*NarrationOutput, error) {
	jsonStr, err := aiClient.ExtractJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("extractJSON: %w", err)
	}

	var out NarrationOutput
	if err := json.Unmarshal([]byte(jsonStr), &out); err != nil {
		return nil, fmt.Errorf("unmarshal narration output: %w", err)
	}

	// Validate minimum required fields.
	if out.Summary == "" {
		return nil, fmt.Errorf("AI response missing 'summary' field")
	}
	if out.Title == "" {
		out.Title = fallbackTitle(nCtx, nt)
	}

	return &out, nil
}

// ============================================================================
// Deterministic Fallback
// ============================================================================

// generateFallback returns a simple, deterministic narration when AI is unavailable.
// This ensures the feature degrades gracefully rather than returning an error.
func (s *AINarrationService) generateFallback(nCtx *NarrationContext, nt NarrationType) *NarrationOutput {
	title := fallbackTitle(nCtx, nt)
	summary := buildFallbackSummary(nCtx, nt)

	paragraphs := []NarrationParagraph{{
		Content: summary,
		Type:    "info",
	}}

	takeaways := buildFallbackTakeaways(nCtx, nt)

	return &NarrationOutput{
		Title:         title,
		Summary:       summary,
		Paragraphs:    paragraphs,
		KeyTakeaways:  takeaways,
		IsAIGenerated: false,
	}
}

// fallbackTitle returns a simple title without AI.
// The type label is localised using nCtx.Language; plan/scenario names are kept
// as-is since they are user-entered data and should not be translated.
func fallbackTitle(nCtx *NarrationContext, nt NarrationType) string {
	lang := normalizeLanguage(nCtx.Language)
	label := fallbackTitleLabel(nt, lang, nCtx.ScenarioType)
	switch nt {
	case NarrationTypeScenarioComparison, NarrationTypeDriverAdvisor, NarrationTypeInvestorMemo:
		return fmt.Sprintf("%s — %s", label, nCtx.PlanName)
	default:
		return fmt.Sprintf("%s — %s / %s", label, nCtx.ScenarioName, nCtx.PeriodLabel)
	}
}

// fallbackTitleLabel returns the localised type label for a fallback title.
func fallbackTitleLabel(nt NarrationType, lang, scenarioType string) string {
	labels := map[NarrationType]map[string]string{
		NarrationTypePlanSummary:        {"en": "Plan Summary", "fr": "Résumé du plan"},
		NarrationTypeVarianceAnalysis:   {"en": "Variance Analysis", "fr": "Analyse des écarts"},
		NarrationTypeScenarioComparison: {"en": "Scenario Comparison", "fr": "Comparaison de scénarios"},
		NarrationTypeAnomalyDetection:   {"en": "Anomaly Report", "fr": "Rapport d'anomalies"},
		NarrationTypeCashRunway:         {"en": "Cash Runway", "fr": "Autonomie de trésorerie"},
		NarrationTypeUnitEconomics:      {"en": "Unit Economics", "fr": "Économie unitaire"},
		NarrationTypeAssumptionReview:   {"en": "Assumption Review", "fr": "Revue des hypothèses"},
		NarrationTypeBenchmarkCommentary:{"en": "Benchmark Commentary", "fr": "Commentaire benchmark"},
		NarrationTypePortfolioMix:       {"en": "Portfolio Mix", "fr": "Mix portefeuille"},
		NarrationTypeDriverAdvisor:      {"en": "Driver Advisor", "fr": "Conseiller de performance"},
		NarrationTypeSensitivityNarrative:{"en": "Sensitivity Narrative", "fr": "Analyse de sensibilité"},
		NarrationTypeInvestorMemo:       {"en": "Investor Memo", "fr": "Note investisseur"},
	}

	// ScenarioSuggestion needs the scenario type injected.
	if nt == NarrationTypeScenarioSuggestion {
		sType := scenarioType
		if sType == "" {
			if lang == "fr" {
				sType = "variante"
			} else {
				sType = "variant"
			}
		}
		if lang == "fr" {
			return fmt.Sprintf("Suggestion de scénario (%s)", sType)
		}
		return fmt.Sprintf("Scenario Suggestion (%s)", sType)
	}

	if m, ok := labels[nt]; ok {
		if l, ok2 := m[lang]; ok2 {
			return l
		}
		return m["en"]
	}
	if lang == "fr" {
		return "Résumé du plan"
	}
	return "Plan Summary"
}

// buildFallbackSummary constructs a brief summary using only available fields.
// For non-English languages the English content-specific branches are bypassed
// in favour of a generic localised message that avoids mixed-language output
// (the fallback path is only reached when AI is unconfigured — a rare case
// in production).
func buildFallbackSummary(nCtx *NarrationContext, nt NarrationType) string {
	lang := normalizeLanguage(nCtx.Language)
	if lang == "fr" {
		return fmt.Sprintf(
			"Résumé financier pour le plan « %s », scénario « %s », période %s. La narration AI n'est pas disponible.",
			nCtx.PlanName, nCtx.ScenarioName, nCtx.PeriodLabel,
		)
	}
	switch nt {
	case NarrationTypeVarianceAnalysis:
		if len(nCtx.VarianceLines) == 0 {
			return fmt.Sprintf("No variance data is available for %s.", nCtx.PeriodLabel)
		}
		fav, unfav := 0, 0
		for _, v := range nCtx.VarianceLines {
			if v.Favourable {
				fav++
			} else {
				unfav++
			}
		}
		return fmt.Sprintf(
			"For %s, %d out of %d variance lines are favourable and %d are unfavourable.",
			nCtx.PeriodLabel, fav, len(nCtx.VarianceLines), unfav,
		)
	case NarrationTypeCashRunway:
		if nCtx.CashRunwayMonths != nil {
			return fmt.Sprintf(
				"Based on the current plan, %s has a projected cash runway of %.1f months as of %s.",
				nCtx.PlanName, nCtx.CashRunwayMonths.Value, nCtx.PeriodLabel,
			)
		}
		return fmt.Sprintf("Cash runway data is not available for %s.", nCtx.PeriodLabel)
	case NarrationTypeAnomalyDetection:
		if len(nCtx.Anomalies) == 0 {
			return fmt.Sprintf("No anomalies were flagged for %s in %s.", nCtx.ScenarioName, nCtx.PeriodLabel)
		}
		return fmt.Sprintf(
			"%d anomal%s flagged for %s in %s.",
			len(nCtx.Anomalies),
			map[bool]string{true: "y was", false: "ies were"}[len(nCtx.Anomalies) == 1],
			nCtx.ScenarioName, nCtx.PeriodLabel,
		)
	// Pro-tier fallbacks
	case NarrationTypeUnitEconomics:
		if len(nCtx.ProductEconomics) > 0 {
			return fmt.Sprintf(
				"Unit economics computed for %d product(s) in %s.",
				len(nCtx.ProductEconomics), nCtx.PeriodLabel,
			)
		}
		return fmt.Sprintf("No product economics data available for %s.", nCtx.PeriodLabel)

	case NarrationTypeAssumptionReview:
		n := len(nCtx.AssumptionFlags)
		if n == 0 {
			return fmt.Sprintf("No assumption flags were detected for %s.", nCtx.ScenarioName)
		}
		return fmt.Sprintf(
			"%d assumption flag(s) detected for %s — review required.",
			n, nCtx.ScenarioName,
		)

	case NarrationTypeBenchmarkCommentary:
		driver := nCtx.PrimaryDriverType
		if driver == "" {
			driver = "mixed"
		}
		return fmt.Sprintf(
			"Benchmark commentary for %s plan (%s driver) — AI narration unavailable.",
			nCtx.PlanName, driver,
		)

	case NarrationTypePortfolioMix:
		n := len(nCtx.ProductEconomics)
		if n > 0 {
			return fmt.Sprintf(
				"Portfolio mix covers %d product(s) across the %s period.",
				n, nCtx.PeriodLabel,
			)
		}
		return "Portfolio mix data unavailable."

	case NarrationTypeDriverAdvisor:
		return fmt.Sprintf(
			"Driver advisor analysis for plan '%s' — AI narration unavailable.",
			nCtx.PlanName,
		)

	case NarrationTypeScenarioSuggestion:
		label := nCtx.ScenarioType
		if label == "" {
			label = "variant"
		}
		return fmt.Sprintf(
			"Scenario suggestion (%s) for '%s' — AI narration unavailable.",
			label, nCtx.ScenarioName,
		)

	case NarrationTypeSensitivityNarrative:
		n := len(nCtx.SensitivityLevers)
		if n > 0 {
			return fmt.Sprintf(
				"%d sensitivity lever(s) identified for %s.",
				n, nCtx.ScenarioName,
			)
		}
		return fmt.Sprintf("No sensitivity lever data available for %s.", nCtx.ScenarioName)

	// Enterprise fallback
	case NarrationTypeInvestorMemo:
		return fmt.Sprintf(
			"Investor memo for plan '%s' — AI narration unavailable. Contact support.",
			nCtx.PlanName,
		)

	default: // plan summary
		if nCtx.Revenue != nil && nCtx.EBITDA != nil {
			return fmt.Sprintf(
				"Plan '%s', scenario '%s' (%s): revenue %.0f %s, EBITDA %.0f %s.",
				nCtx.PlanName, nCtx.ScenarioName, nCtx.PeriodLabel,
				nCtx.Revenue.Value, nCtx.Currency,
				nCtx.EBITDA.Value, nCtx.Currency,
			)
		}
		return fmt.Sprintf(
			"Financial summary for plan '%s', scenario '%s', period %s.",
			nCtx.PlanName, nCtx.ScenarioName, nCtx.PeriodLabel,
		)
	}
}

// buildFallbackTakeaways returns concise takeaways constructed from available data.
// For non-English languages, a single localised takeaway is returned since the
// English-specific content branches are not translated in the fallback path.
func buildFallbackTakeaways(nCtx *NarrationContext, nt NarrationType) []string {
	lang := normalizeLanguage(nCtx.Language)
	if lang == "fr" {
		return []string{"La narration AI n'est pas disponible. Activez la narration AI pour obtenir des analyses détaillées."}
	}

	var items []string

	switch nt {
	case NarrationTypeCashRunway:
		if nCtx.CashRunwayMonths != nil {
			items = append(items, fmt.Sprintf("Projected cash runway: %.1f months", nCtx.CashRunwayMonths.Value))
		}
		if nCtx.CashPosition != nil {
			items = append(items, fmt.Sprintf("Cash position: %.0f %s", nCtx.CashPosition.Value, nCtx.Currency))
		}
		if nCtx.BurnRate != nil {
			items = append(items, fmt.Sprintf("Monthly burn rate: %.0f %s", nCtx.BurnRate.Value, nCtx.Currency))
		}
	case NarrationTypeVarianceAnalysis:
		for i, v := range nCtx.VarianceLines {
			if i >= 3 {
				break
			}
			dir := "over"
			if v.Favourable {
				dir = "under"
			}
			items = append(items, fmt.Sprintf("%s: %s budget by %.0f %s (%.1f%%)",
				v.Label, dir, abs(v.Variance), nCtx.Currency, abs(v.VarPct)))
		}

	case NarrationTypeUnitEconomics:
		for i, pe := range nCtx.ProductEconomics {
			if i >= 3 {
				break
			}
			if len(pe.Metrics) > 0 {
				items = append(items, fmt.Sprintf("%s (%s): %s = %.2f",
					pe.ProductName, pe.DriverType, pe.Metrics[0].Label, pe.Metrics[0].Value))
			}
		}

	case NarrationTypeAssumptionReview:
		for i, f := range nCtx.AssumptionFlags {
			if i >= 3 {
				break
			}
			items = append(items, fmt.Sprintf("%s / %s: %s", f.ProductName, f.FieldName, f.Reason))
		}

	case NarrationTypeSensitivityNarrative:
		for i, lv := range nCtx.SensitivityLevers {
			if i >= 3 {
				break
			}
			items = append(items, fmt.Sprintf(
				"%s (%s): EBITDA impact %.0f %s",
				lv.LeverName, lv.DriverType, lv.EBITDAImpact, nCtx.Currency,
			))
		}

	default:
		if nCtx.Revenue != nil {
			items = append(items, fmt.Sprintf("Revenue: %.0f %s", nCtx.Revenue.Value, nCtx.Currency))
		}
		if nCtx.EBITDA != nil {
			items = append(items, fmt.Sprintf("EBITDA: %.0f %s", nCtx.EBITDA.Value, nCtx.Currency))
		}
		if nCtx.NetIncome != nil {
			items = append(items, fmt.Sprintf("Net income: %.0f %s", nCtx.NetIncome.Value, nCtx.Currency))
		}
	}

	if len(items) == 0 {
		items = append(items, fmt.Sprintf("AI narration is not available — review the %s data directly.", nCtx.PeriodLabel))
	}
	return items
}

// ============================================================================
// Helpers
// ============================================================================

// inferNarrationType picks a sensible default from available context fields.
// Callers should set NarrationContext.NarrationType explicitly for Pro features.
func inferNarrationType(nCtx *NarrationContext) NarrationType {
	switch {
	// Pro-tier types inferred from context
	case len(nCtx.AssumptionFlags) > 0:
		return NarrationTypeAssumptionReview
	case len(nCtx.SensitivityLevers) > 0:
		return NarrationTypeSensitivityNarrative
	case len(nCtx.ProductEconomics) > 1:
		return NarrationTypePortfolioMix
	case len(nCtx.ProductEconomics) == 1:
		return NarrationTypeUnitEconomics
	// Standard-tier types
	case len(nCtx.VarianceLines) > 0:
		return NarrationTypeVarianceAnalysis
	case len(nCtx.Scenarios) > 1:
		return NarrationTypeScenarioComparison
	case len(nCtx.Anomalies) > 0:
		return NarrationTypeAnomalyDetection
	case nCtx.CashRunwayMonths != nil || nCtx.BurnRate != nil:
		return NarrationTypeCashRunway
	default:
		return NarrationTypePlanSummary
	}
}

// abs returns the absolute value of a float64.
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
