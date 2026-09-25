package service

// Tests for:
//   - Cache integration inside GenerateNarration (hit, miss, store, disabled)
//   - The 8 new Pro/Enterprise narration types: fallback titles, fallback summaries,
//     fallback takeaways, narration instruction keywords
//   - inferNarrationType detection of Pro context fields
//   - buildDataContext serialisation of Pro context fields

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	aiClient "ascenda/internal/pkg/ai"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Helpers (shared with ai_narration_service_test.go via same package)
// ============================================================================

// proCtx builds a NarrationContext pre-loaded with Pro-tier driver-aware data.
func proCtx(nt NarrationType) *NarrationContext {
	ctx := minimalCtx()
	ctx.NarrationType = nt
	ctx.PrimaryDriverType = "saas"
	ctx.ProductEconomics = []ProductEconomics{
		{
			ProductName: "Starter Plan",
			DriverType:  "saas",
			Metrics: []FinancialMetric{
				{Label: "ARPU", Value: 49.0, Currency: "EUR"},
				{Label: "Churn Rate", Value: 0.03, Unit: "%"},
			},
			DriverParams: map[string]interface{}{"mrr_growth": 0.12},
		},
	}
	return ctx
}

// unconfiguredSvc returns a service with no AI client and no cache.
func unconfiguredSvc() *AINarrationService {
	return &AINarrationService{
		client: &aiClient.Client{},
		log:    logrus.NewEntry(logrus.New()),
	}
}

// mockNarrationServerWithCounter is like mockNarrationServer but increments
// callCount on every request received.
func mockNarrationServerWithCounter(t *testing.T, provider string, callCount *int) (*httptest.Server, *string) {
	t.Helper()
	var lastBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*callCount++

		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err == nil {
			b, _ := json.Marshal(reqBody)
			lastBody = string(b)
		}

		narration := NarrationOutput{
			Title:        "AI-Generated Title",
			Summary:      "AI-generated executive summary.",
			Paragraphs:   []NarrationParagraph{{Content: "Detailed paragraph.", Type: "info"}},
			KeyTakeaways: []string{"Takeaway one", "Takeaway two"},
		}
		aiContent, _ := json.Marshal(narration)

		w.Header().Set("Content-Type", "application/json")
		switch provider {
		case "claude":
			resp := map[string]interface{}{
				"content": []map[string]string{
					{"type": "text", "text": string(aiContent)},
				},
			}
			json.NewEncoder(w).Encode(resp) //nolint:errcheck
		default: // openai
			resp := map[string]interface{}{
				"choices": []map[string]interface{}{
					{"message": map[string]string{"content": string(aiContent)}},
				},
			}
			json.NewEncoder(w).Encode(resp) //nolint:errcheck
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &lastBody
}

// ============================================================================
// Cache integration — GenerateNarration flow
// ============================================================================

func TestGenerateNarration_CacheMiss_CallsAI_ThenCaches(t *testing.T) {
	srv, _ := mockNarrationServer(t, "claude")
	c := aiClient.ClientForTest("claude", "test-key", srv.URL)
	svc := &AINarrationService{client: &c, log: logrus.NewEntry(logrus.New())}
	freshCache := NewNarrationCache(DefaultNarrationCacheConfig())
	svc.SetCache(freshCache)

	nCtx := minimalCtx()
	nCtx.NarrationType = NarrationTypePlanSummary

	out, err := svc.GenerateNarration(context.Background(), nCtx)
	require.NoError(t, err)
	assert.True(t, out.IsAIGenerated)

	// The result must now be in the cache.
	key := NarrationCacheKey(nCtx)
	cached, ok := freshCache.Get(uuid.Nil, key)
	require.True(t, ok, "result must be stored in cache after AI call")
	assert.Equal(t, out.Title, cached.Title)
}

func TestGenerateNarration_CacheHit_NoAICall(t *testing.T) {
	callCount := 0
	srv, _ := mockNarrationServerWithCounter(t, "claude", &callCount)
	c := aiClient.ClientForTest("claude", "test-key", srv.URL)
	svc := &AINarrationService{client: &c, log: logrus.NewEntry(logrus.New())}
	preloadedCache := NewNarrationCache(DefaultNarrationCacheConfig())
	svc.SetCache(preloadedCache)

	nCtx := minimalCtx()
	nCtx.NarrationType = NarrationTypePlanSummary
	cachedResult := stubOutput("Cached Result")

	// Pre-populate the cache with the exact key the service will look up.
	key := NarrationCacheKey(nCtx)
	preloadedCache.Put(uuid.Nil, key, cachedResult)

	out, err := svc.GenerateNarration(context.Background(), nCtx)
	require.NoError(t, err)
	assert.Equal(t, "Cached Result", out.Title, "should return cached result")
	assert.Equal(t, 0, callCount, "AI provider must NOT be called on cache hit")
}

func TestGenerateNarration_FallbackNotCached(t *testing.T) {
	// Service with no AI configured — produces deterministic fallback.
	svc := unconfiguredSvc()
	freshCache := NewNarrationCache(DefaultNarrationCacheConfig())
	svc.SetCache(freshCache)

	nCtx := minimalCtx()
	nCtx.NarrationType = NarrationTypePlanSummary

	out, err := svc.GenerateNarration(context.Background(), nCtx)
	require.NoError(t, err)
	assert.False(t, out.IsAIGenerated, "deterministic fallback must not claim AI authorship")
	assert.Equal(t, 0, freshCache.Size(), "fallback results must not be stored in cache")
}

func TestGenerateNarration_NilCache_StillWorks(t *testing.T) {
	svc := unconfiguredSvc()
	svc.SetCache(nil)

	nCtx := minimalCtx()
	nCtx.NarrationType = NarrationTypePlanSummary

	out, err := svc.GenerateNarration(context.Background(), nCtx)
	require.NoError(t, err, "nil cache must not panic or error")
	assert.NotNil(t, out)
}

func TestGenerateNarration_InferTypeWrittenBackBeforeCacheKey(t *testing.T) {
	srv, _ := mockNarrationServer(t, "claude")
	c := aiClient.ClientForTest("claude", "test-key", srv.URL)
	svc := &AINarrationService{client: &c, log: logrus.NewEntry(logrus.New())}
	freshCache := NewNarrationCache(DefaultNarrationCacheConfig())
	svc.SetCache(freshCache)

	// No NarrationType set — service infers plan_summary.
	nCtx := minimalCtx()
	nCtx.NarrationType = "" // will be inferred

	_, err := svc.GenerateNarration(context.Background(), nCtx)
	require.NoError(t, err)

	// After the call the type should have been written back.
	assert.Equal(t, NarrationTypePlanSummary, nCtx.NarrationType)

	// The cache key must reflect the resolved type.
	key := NarrationCacheKey(nCtx)
	assert.Contains(t, key, "plan_summary")
}

func TestGenerateNarration_CacheKey_ProType_PrePopulated(t *testing.T) {
	// Verify that a Pro-type context can be pre-seeded and served from cache.
	svc := unconfiguredSvc()
	c := NewNarrationCache(DefaultNarrationCacheConfig())
	svc.SetCache(c)

	nCtx := proCtx(NarrationTypeUnitEconomics)
	want := stubOutput("Unit Economics — Starter Plan")
	c.Put(uuid.Nil, NarrationCacheKey(nCtx), want)

	out, err := svc.GenerateNarration(context.Background(), nCtx)
	require.NoError(t, err)
	assert.Equal(t, "Unit Economics — Starter Plan", out.Title)
}

func TestGenerateNarration_CacheTTLExpiry_AICalledAgain(t *testing.T) {
	callCount := 0
	srv, _ := mockNarrationServerWithCounter(t, "claude", &callCount)
	c := aiClient.ClientForTest("claude", "test-key", srv.URL)
	svc := &AINarrationService{client: &c, log: logrus.NewEntry(logrus.New())}
	shortTTLCache := NewNarrationCache(NarrationCacheConfig{
		MaxSize: 10,
		TTL:     5 * time.Millisecond,
	})
	svc.SetCache(shortTTLCache)

	nCtx := minimalCtx()
	nCtx.NarrationType = NarrationTypePlanSummary

	// First call — AI is called, result cached.
	_, err := svc.GenerateNarration(context.Background(), nCtx)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Wait for TTL to expire.
	time.Sleep(20 * time.Millisecond)

	// Second call — cache miss, AI called again.
	_, err = svc.GenerateNarration(context.Background(), nCtx)
	require.NoError(t, err)
	assert.Equal(t, 2, callCount, "AI must be called again after cache TTL expiry")
}

// ============================================================================
// inferNarrationType — Pro context fields
// ============================================================================

func TestInferNarrationType_AssumptionFlags_ReturnsAssumptionReview(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.AssumptionFlags = []AssumptionFlag{
		{ProductName: "P1", FieldName: "churnRate", Value: 0.99, Reason: "implausibly high"},
	}
	assert.Equal(t, NarrationTypeAssumptionReview, inferNarrationType(nCtx))
}

func TestInferNarrationType_SensitivityLevers_ReturnsSensitivityNarrative(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.SensitivityLevers = []SensitivityLever{
		{LeverName: "Churn", DriverType: "saas", EBITDAImpact: -150_000},
	}
	assert.Equal(t, NarrationTypeSensitivityNarrative, inferNarrationType(nCtx))
}

func TestInferNarrationType_MultipleProducts_ReturnsPortfolioMix(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.ProductEconomics = []ProductEconomics{
		{ProductName: "A", DriverType: "saas"},
		{ProductName: "B", DriverType: "marketplace"},
	}
	assert.Equal(t, NarrationTypePortfolioMix, inferNarrationType(nCtx))
}

func TestInferNarrationType_SingleProduct_ReturnsUnitEconomics(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.ProductEconomics = []ProductEconomics{
		{ProductName: "Starter", DriverType: "saas"},
	}
	assert.Equal(t, NarrationTypeUnitEconomics, inferNarrationType(nCtx))
}

func TestInferNarrationType_ProFieldsOverrideStandardFields(t *testing.T) {
	// When assumption flags AND variance lines are both present, Pro type wins.
	nCtx := minimalCtx()
	nCtx.AssumptionFlags = []AssumptionFlag{
		{ProductName: "P1", FieldName: "churnRate", Value: 0.5, Reason: "high"},
	}
	nCtx.VarianceLines = []VarianceLine{
		{Label: "Revenue", Budget: 100, Actual: 80, Variance: -20},
	}
	assert.Equal(t, NarrationTypeAssumptionReview, inferNarrationType(nCtx),
		"Pro context fields should take priority over standard variance lines")
}

// ============================================================================
// Fallback titles — Pro + Enterprise types
// ============================================================================

func TestFallbackTitle_ProTypes_AllCovered(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.ScenarioName = "Base Case"
	nCtx.PeriodLabel = "FY 2026"

	cases := []struct {
		nt      NarrationType
		wantSub string // substring that must appear in the title
	}{
		{NarrationTypeUnitEconomics, "Unit Economics"},
		{NarrationTypeAssumptionReview, "Assumption Review"},
		{NarrationTypeBenchmarkCommentary, "Benchmark Commentary"},
		{NarrationTypePortfolioMix, "Portfolio Mix"},
		{NarrationTypeDriverAdvisor, "Driver Advisor"},
		{NarrationTypeScenarioSuggestion, "Scenario Suggestion"},
		{NarrationTypeSensitivityNarrative, "Sensitivity Narrative"},
		{NarrationTypeInvestorMemo, "Investor Memo"},
	}

	for _, tc := range cases {
		t.Run(string(tc.nt), func(t *testing.T) {
			title := fallbackTitle(nCtx, tc.nt)
			assert.NotEmpty(t, title)
			assert.Contains(t, title, tc.wantSub)
		})
	}
}

func TestFallbackTitle_ScenarioSuggestion_IncludesScenarioType(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.ScenarioType = "bear"
	title := fallbackTitle(nCtx, NarrationTypeScenarioSuggestion)
	assert.Contains(t, title, "bear")
}

func TestFallbackTitle_ScenarioSuggestion_DefaultsToVariantWhenEmpty(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.ScenarioType = ""
	title := fallbackTitle(nCtx, NarrationTypeScenarioSuggestion)
	assert.Contains(t, title, "variant")
}

// ============================================================================
// Fallback summaries — Pro + Enterprise types
// ============================================================================

func TestBuildFallbackSummary_UnitEconomics_WithProducts(t *testing.T) {
	nCtx := proCtx(NarrationTypeUnitEconomics)
	summary := buildFallbackSummary(nCtx, NarrationTypeUnitEconomics)
	assert.Contains(t, summary, "1 product")
}

func TestBuildFallbackSummary_UnitEconomics_NoProducts(t *testing.T) {
	nCtx := minimalCtx()
	summary := buildFallbackSummary(nCtx, NarrationTypeUnitEconomics)
	assert.Contains(t, summary, "No product economics")
}

func TestBuildFallbackSummary_AssumptionReview_WithFlags(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.AssumptionFlags = []AssumptionFlag{
		{ProductName: "P1", FieldName: "churnRate", Value: 0.99},
		{ProductName: "P2", FieldName: "arpu", Value: 999.0},
	}
	summary := buildFallbackSummary(nCtx, NarrationTypeAssumptionReview)
	assert.Contains(t, summary, "2 assumption flag")
}

func TestBuildFallbackSummary_AssumptionReview_NoFlags(t *testing.T) {
	nCtx := minimalCtx()
	summary := buildFallbackSummary(nCtx, NarrationTypeAssumptionReview)
	assert.Contains(t, summary, "No assumption flags")
}

func TestBuildFallbackSummary_BenchmarkCommentary_IncludesDriverType(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.PrimaryDriverType = "marketplace"
	summary := buildFallbackSummary(nCtx, NarrationTypeBenchmarkCommentary)
	assert.Contains(t, summary, "marketplace")
}

func TestBuildFallbackSummary_SensitivityNarrative_WithLevers(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.SensitivityLevers = []SensitivityLever{
		{LeverName: "Churn", DriverType: "saas", EBITDAImpact: -200_000},
		{LeverName: "ARPU", DriverType: "saas", EBITDAImpact: 150_000},
	}
	summary := buildFallbackSummary(nCtx, NarrationTypeSensitivityNarrative)
	assert.Contains(t, summary, "2 sensitivity lever")
}

func TestBuildFallbackSummary_InvestorMemo_ContainsPlanName(t *testing.T) {
	nCtx := minimalCtx()
	summary := buildFallbackSummary(nCtx, NarrationTypeInvestorMemo)
	assert.Contains(t, summary, "Budget 2026")
	assert.Contains(t, summary, "Investor memo")
}

// ============================================================================
// Fallback takeaways — Pro types produce driver-aware items
// ============================================================================

func TestBuildFallbackTakeaways_UnitEconomics_IncludesMetricValues(t *testing.T) {
	nCtx := proCtx(NarrationTypeUnitEconomics)
	items := buildFallbackTakeaways(nCtx, NarrationTypeUnitEconomics)
	require.NotEmpty(t, items)
	// First takeaway should mention the product and first metric
	assert.Contains(t, items[0], "Starter Plan")
}

func TestBuildFallbackTakeaways_AssumptionReview_IncludesFieldName(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.AssumptionFlags = []AssumptionFlag{
		{ProductName: "Starter", FieldName: "churnRate", Reason: "above benchmark"},
		{ProductName: "Pro", FieldName: "arpu", Reason: "very high"},
		{ProductName: "Enterprise", FieldName: "seats", Reason: "low"},
		{ProductName: "Extra", FieldName: "extra", Reason: "extra"},
	}
	items := buildFallbackTakeaways(nCtx, NarrationTypeAssumptionReview)
	assert.Len(t, items, 3, "capped at 3 takeaways")
	assert.Contains(t, items[0], "churnRate")
}

func TestBuildFallbackTakeaways_SensitivityNarrative_TopThreeByEBITDA(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.SensitivityLevers = []SensitivityLever{
		{LeverName: "Churn", EBITDAImpact: -300_000},
		{LeverName: "ARPU", EBITDAImpact: 200_000},
		{LeverName: "CAC", EBITDAImpact: -150_000},
		{LeverName: "Headcount", EBITDAImpact: -100_000},
	}
	items := buildFallbackTakeaways(nCtx, NarrationTypeSensitivityNarrative)
	assert.Len(t, items, 3, "capped at 3 takeaways")
	assert.Contains(t, items[0], "Churn")
}

// ============================================================================
// narrationInstructions — Pro type keywords
// ============================================================================

func TestNarrationInstructions_UnitEconomics_ContainsDriverKeywords(t *testing.T) {
	instr := narrationInstructions(NarrationTypeUnitEconomics, NarrationRoleOwner)
	assert.Contains(t, instr, "product_economics")
	assert.Contains(t, instr, "ARPU")
	assert.Contains(t, instr, "eCPM")
}

func TestNarrationInstructions_AssumptionReview_NoSuggestCorrections(t *testing.T) {
	instr := narrationInstructions(NarrationTypeAssumptionReview, NarrationRoleOwner)
	assert.Contains(t, instr, "assumption_flags")
	assert.Contains(t, instr, "Do NOT suggest corrections")
}

func TestNarrationInstructions_BenchmarkCommentary_PerDriverBenchmarks(t *testing.T) {
	instr := narrationInstructions(NarrationTypeBenchmarkCommentary, NarrationRoleOwner)
	for _, driver := range []string{"SaaS", "consulting", "marketplace", "media"} {
		assert.True(t, strings.Contains(instr, driver),
			"benchmark commentary should mention driver: %s", driver)
	}
}

func TestNarrationInstructions_DriverAdvisor_StructuredDataField(t *testing.T) {
	instr := narrationInstructions(NarrationTypeDriverAdvisor, NarrationRoleOwner)
	assert.Contains(t, instr, "structured_data")
	assert.Contains(t, instr, "recommended_driver")
}

func TestNarrationInstructions_ScenarioSuggestion_StructuredDataDiffs(t *testing.T) {
	instr := narrationInstructions(NarrationTypeScenarioSuggestion, NarrationRoleOwner)
	assert.Contains(t, instr, "structured_data")
	assert.Contains(t, instr, "diffs")
	assert.Contains(t, instr, "Do NOT modify structural assumptions")
}

func TestNarrationInstructions_SensitivityNarrative_LeverRanking(t *testing.T) {
	instr := narrationInstructions(NarrationTypeSensitivityNarrative, NarrationRoleOwner)
	assert.Contains(t, instr, "sensitivity_levers")
	assert.Contains(t, instr, "EBITDA impact")
}

func TestNarrationInstructions_InvestorMemo_ContainsSections(t *testing.T) {
	instr := narrationInstructions(NarrationTypeInvestorMemo, NarrationRoleOwner)
	for _, section := range []string{"Business Model", "Unit Economics", "Financial Projections", "Key Risks", "Investment Highlights"} {
		assert.Contains(t, instr, section, "investor memo instruction must define section: %s", section)
	}
}

// ============================================================================
// buildDataContext — Pro fields are serialised
// ============================================================================

func TestBuildDataContext_ProFields_IncludedWhenSet(t *testing.T) {
	nCtx := proCtx(NarrationTypeUnitEconomics)
	nCtx.AssumptionFlags = []AssumptionFlag{
		{ProductName: "P1", FieldName: "churnRate", Value: 0.5},
	}
	nCtx.SensitivityLevers = []SensitivityLever{
		{LeverName: "ARPU", EBITDAImpact: 100_000},
	}
	nCtx.ScenarioType = "bear"

	data := buildDataContext(nCtx)
	assert.Contains(t, data, "primary_driver_type")
	assert.Contains(t, data, "saas")
	assert.Contains(t, data, "product_economics")
	assert.Contains(t, data, "Starter Plan")
	assert.Contains(t, data, "assumption_flags")
	assert.Contains(t, data, "churnRate")
	assert.Contains(t, data, "sensitivity_levers")
	assert.Contains(t, data, "ARPU")
	assert.Contains(t, data, "scenario_type")
	assert.Contains(t, data, "bear")
}

func TestBuildDataContext_ProFields_AbsentWhenEmpty(t *testing.T) {
	nCtx := minimalCtx()
	data := buildDataContext(nCtx)
	assert.NotContains(t, data, "primary_driver_type")
	assert.NotContains(t, data, "product_economics")
	assert.NotContains(t, data, "assumption_flags")
	assert.NotContains(t, data, "sensitivity_levers")
}
