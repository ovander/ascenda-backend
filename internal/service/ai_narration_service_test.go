package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	aiClient "ascenda/internal/pkg/ai"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Test helpers
// ============================================================================

// minimalCtx returns a NarrationContext with required fields set.
func minimalCtx() *NarrationContext {
	return &NarrationContext{
		PlanName:     "Budget 2026",
		ScenarioName: "Base",
		PeriodLabel:  "FY 2026",
		Currency:     "EUR",
		UserRole:     NarrationRoleOwner,
	}
}

// mockNarrationServer returns an httptest server that always responds with
// a valid NarrationOutput JSON and records the last received request body.
func mockNarrationServer(t *testing.T, provider string) (*httptest.Server, *string) {
	t.Helper()
	var lastBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Decode the request body to capture it for assertions.
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err == nil {
			b, _ := json.Marshal(reqBody)
			lastBody = string(b)
		}

		narration := NarrationOutput{
			Title:         "AI-Generated Title",
			Summary:       "AI-generated executive summary.",
			Paragraphs:    []NarrationParagraph{{Content: "Detailed paragraph.", Type: "info"}},
			KeyTakeaways:  []string{"Takeaway one", "Takeaway two"},
			IsAIGenerated: false, // set by service after parsing
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
// GenerateNarration — guard conditions
// ============================================================================

func TestGenerateNarration_NilContextReturnsError(t *testing.T) {
	svc := &AINarrationService{
		client: &aiClient.Client{},
		log:    logrus.NewEntry(logrus.New()),
	}
	_, err := svc.GenerateNarration(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "narration context is required")
}

func TestGenerateNarration_EmptyPlanNameReturnsError(t *testing.T) {
	svc := &AINarrationService{
		client: &aiClient.Client{},
		log:    logrus.NewEntry(logrus.New()),
	}
	nCtx := minimalCtx()
	nCtx.PlanName = ""
	_, err := svc.GenerateNarration(context.Background(), nCtx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plan name is required")
}

// ============================================================================
// GenerateNarration — deterministic fallback (AI not configured)
// ============================================================================

func TestGenerateNarration_FallbackWhenNotConfigured(t *testing.T) {
	svc := &AINarrationService{
		client: &aiClient.Client{},
		log:    logrus.NewEntry(logrus.New()),
	}
	nCtx := minimalCtx()
	out, err := svc.GenerateNarration(context.Background(), nCtx)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.False(t, out.IsAIGenerated, "fallback must not set IsAIGenerated=true")
	assert.NotEmpty(t, out.Title)
	assert.NotEmpty(t, out.Summary)
}

func TestGenerateNarration_FallbackContainsPlanName(t *testing.T) {
	svc := &AINarrationService{
		client: &aiClient.Client{},
		log:    logrus.NewEntry(logrus.New()),
	}
	nCtx := minimalCtx()
	nCtx.Revenue = &FinancialMetric{Label: "Revenue", Value: 500_000, Currency: "EUR"}
	nCtx.EBITDA = &FinancialMetric{Label: "EBITDA", Value: 80_000, Currency: "EUR"}

	out, err := svc.GenerateNarration(context.Background(), nCtx)
	require.NoError(t, err)
	assert.Contains(t, out.Summary, "Budget 2026")
}

// ============================================================================
// GenerateNarration — AI success path (OpenAI)
// ============================================================================

func TestGenerateNarration_AISuccessPath_OpenAI(t *testing.T) {
	srv, _ := mockNarrationServer(t, "openai")
	c := aiClient.ClientForTest("openai", "test-key", srv.URL)
	svc := &AINarrationService{client: &c, log: logrus.NewEntry(logrus.New())}

	nCtx := minimalCtx()
	out, err := svc.GenerateNarration(context.Background(), nCtx)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.True(t, out.IsAIGenerated)
	assert.Equal(t, "AI-Generated Title", out.Title)
	assert.Equal(t, "AI-generated executive summary.", out.Summary)
}

func TestGenerateNarration_AISuccessPath_Claude(t *testing.T) {
	srv, _ := mockNarrationServer(t, "claude")
	c := aiClient.ClientForTest("claude", "test-key", srv.URL)
	svc := &AINarrationService{client: &c, log: logrus.NewEntry(logrus.New())}

	nCtx := minimalCtx()
	out, err := svc.GenerateNarration(context.Background(), nCtx)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.True(t, out.IsAIGenerated)
}

// ============================================================================
// GenerateNarration — AI failure path (falls back gracefully)
// ============================================================================

func TestGenerateNarration_AIErrorFallsBack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	c := aiClient.ClientForTest("openai", "test-key", srv.URL)
	svc := &AINarrationService{client: &c, log: logrus.NewEntry(logrus.New())}

	nCtx := minimalCtx()
	out, err := svc.GenerateNarration(context.Background(), nCtx)
	require.NoError(t, err, "AI errors should not propagate — fallback used instead")
	assert.False(t, out.IsAIGenerated)
}

func TestGenerateNarration_AIBadJSONFallsBack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "not json at all"}},
			},
		}
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)

	c := aiClient.ClientForTest("openai", "test-key", srv.URL)
	svc := &AINarrationService{client: &c, log: logrus.NewEntry(logrus.New())}

	nCtx := minimalCtx()
	out, err := svc.GenerateNarration(context.Background(), nCtx)
	require.NoError(t, err, "bad AI JSON should fall back, not error")
	assert.False(t, out.IsAIGenerated)
}

func TestGenerateNarration_AIMissingSummaryFallsBack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Valid JSON but summary is empty → parseAIResponse returns error.
		payload := `{"title":"T","summary":"","paragraphs":[],"key_takeaways":[]}`
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": payload}},
			},
		}
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)

	c := aiClient.ClientForTest("openai", "test-key", srv.URL)
	svc := &AINarrationService{client: &c, log: logrus.NewEntry(logrus.New())}

	nCtx := minimalCtx()
	out, err := svc.GenerateNarration(context.Background(), nCtx)
	require.NoError(t, err)
	assert.False(t, out.IsAIGenerated)
}

// ============================================================================
// GenerateNarration — AI response with empty title uses fallback title
// ============================================================================

func TestGenerateNarration_AIEmptyTitleUsesFallbackTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// title is empty — service should fill it with fallbackTitle.
		payload := `{"title":"","summary":"Good summary here.","paragraphs":[],"key_takeaways":[]}`
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": payload}},
			},
		}
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)

	c := aiClient.ClientForTest("openai", "test-key", srv.URL)
	svc := &AINarrationService{client: &c, log: logrus.NewEntry(logrus.New())}

	nCtx := minimalCtx()
	nCtx.NarrationType = NarrationTypePlanSummary
	out, err := svc.GenerateNarration(context.Background(), nCtx)
	require.NoError(t, err)
	assert.True(t, out.IsAIGenerated)
	assert.NotEmpty(t, out.Title, "empty AI title must be replaced by fallback")
	assert.Contains(t, out.Title, "Plan Summary")
}

// ============================================================================
// inferNarrationType
// ============================================================================

func TestInferNarrationType_VarianceLinesFirst(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.VarianceLines = []VarianceLine{{Label: "Revenue", Budget: 100, Actual: 90}}
	nCtx.Anomalies = []string{"anomaly"}
	assert.Equal(t, NarrationTypeVarianceAnalysis, inferNarrationType(nCtx))
}

func TestInferNarrationType_ScenarioComparison(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.Scenarios = []ScenarioSummary{
		{ScenarioName: "Base"},
		{ScenarioName: "Optimistic"},
	}
	assert.Equal(t, NarrationTypeScenarioComparison, inferNarrationType(nCtx))
}

func TestInferNarrationType_SingleScenarioNotComparison(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.Scenarios = []ScenarioSummary{{ScenarioName: "Base"}}
	// Only one scenario — should not infer scenario comparison.
	typ := inferNarrationType(nCtx)
	assert.NotEqual(t, NarrationTypeScenarioComparison, typ)
}

func TestInferNarrationType_AnomalyDetection(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.Anomalies = []string{"revenue spike"}
	assert.Equal(t, NarrationTypeAnomalyDetection, inferNarrationType(nCtx))
}

func TestInferNarrationType_CashRunwayFromMonths(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.CashRunwayMonths = &FinancialMetric{Label: "Runway", Value: 12}
	assert.Equal(t, NarrationTypeCashRunway, inferNarrationType(nCtx))
}

func TestInferNarrationType_CashRunwayFromBurnRate(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.BurnRate = &FinancialMetric{Label: "Burn", Value: 50_000}
	assert.Equal(t, NarrationTypeCashRunway, inferNarrationType(nCtx))
}

func TestInferNarrationType_DefaultPlanSummary(t *testing.T) {
	nCtx := minimalCtx()
	assert.Equal(t, NarrationTypePlanSummary, inferNarrationType(nCtx))
}

// ============================================================================
// fallbackTitle
// ============================================================================

func TestFallbackTitle_AllTypes(t *testing.T) {
	nCtx := &NarrationContext{
		PlanName:     "My Plan",
		ScenarioName: "Base",
		PeriodLabel:  "Q1 2026",
	}
	cases := []struct {
		nt      NarrationType
		contain string
	}{
		{NarrationTypeVarianceAnalysis, "Variance Analysis"},
		{NarrationTypeScenarioComparison, "Scenario Comparison"},
		{NarrationTypeAnomalyDetection, "Anomaly Report"},
		{NarrationTypeCashRunway, "Cash Runway"},
		{NarrationTypePlanSummary, "Plan Summary"},
	}
	for _, tc := range cases {
		t.Run(string(tc.nt), func(t *testing.T) {
			title := fallbackTitle(nCtx, tc.nt)
			assert.Contains(t, title, tc.contain, "title for %s should contain %q", tc.nt, tc.contain)
		})
	}
}

// ============================================================================
// buildFallbackSummary
// ============================================================================

func TestBuildFallbackSummary_PlanWithMetrics(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.Revenue = &FinancialMetric{Label: "Revenue", Value: 1_000_000}
	nCtx.EBITDA = &FinancialMetric{Label: "EBITDA", Value: 200_000}

	summary := buildFallbackSummary(nCtx, NarrationTypePlanSummary)
	assert.Contains(t, summary, "Budget 2026")
	assert.Contains(t, summary, "1000000")
}

func TestBuildFallbackSummary_PlanWithoutMetrics(t *testing.T) {
	nCtx := minimalCtx()
	summary := buildFallbackSummary(nCtx, NarrationTypePlanSummary)
	assert.Contains(t, summary, "Budget 2026")
	assert.Contains(t, summary, "Base")
}

func TestBuildFallbackSummary_VarianceWithLines(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.VarianceLines = []VarianceLine{
		{Label: "Revenue", Favourable: true},
		{Label: "Costs", Favourable: false},
		{Label: "EBITDA", Favourable: false},
	}
	summary := buildFallbackSummary(nCtx, NarrationTypeVarianceAnalysis)
	assert.Contains(t, summary, "1 out of 3")
	assert.Contains(t, summary, "2 are unfavourable")
}

func TestBuildFallbackSummary_VarianceNoLines(t *testing.T) {
	nCtx := minimalCtx()
	summary := buildFallbackSummary(nCtx, NarrationTypeVarianceAnalysis)
	assert.Contains(t, summary, "No variance data")
}

func TestBuildFallbackSummary_CashRunwayWithData(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.CashRunwayMonths = &FinancialMetric{Value: 14.5}
	summary := buildFallbackSummary(nCtx, NarrationTypeCashRunway)
	assert.Contains(t, summary, "14.5 months")
	assert.Contains(t, summary, "Budget 2026")
}

func TestBuildFallbackSummary_CashRunwayNoData(t *testing.T) {
	nCtx := minimalCtx()
	summary := buildFallbackSummary(nCtx, NarrationTypeCashRunway)
	assert.Contains(t, summary, "not available")
}

func TestBuildFallbackSummary_AnomalyWithItems(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.Anomalies = []string{"spike in COGS", "negative GP"}
	summary := buildFallbackSummary(nCtx, NarrationTypeAnomalyDetection)
	assert.Contains(t, summary, "2 anomalies were flagged")
}

func TestBuildFallbackSummary_AnomalySingular(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.Anomalies = []string{"spike in COGS"}
	summary := buildFallbackSummary(nCtx, NarrationTypeAnomalyDetection)
	assert.Contains(t, summary, "1 anomaly was flagged")
}

func TestBuildFallbackSummary_AnomalyNone(t *testing.T) {
	nCtx := minimalCtx()
	summary := buildFallbackSummary(nCtx, NarrationTypeAnomalyDetection)
	assert.Contains(t, summary, "No anomalies")
}

// ============================================================================
// buildFallbackTakeaways
// ============================================================================

func TestBuildFallbackTakeaways_PlanWithMetrics(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.Revenue = &FinancialMetric{Value: 500_000}
	nCtx.EBITDA = &FinancialMetric{Value: 80_000}
	nCtx.NetIncome = &FinancialMetric{Value: 40_000}

	items := buildFallbackTakeaways(nCtx, NarrationTypePlanSummary)
	require.Len(t, items, 3)
	assert.Contains(t, items[0], "Revenue")
	assert.Contains(t, items[1], "EBITDA")
	assert.Contains(t, items[2], "Net income")
}

func TestBuildFallbackTakeaways_PlanNoMetrics(t *testing.T) {
	nCtx := minimalCtx()
	items := buildFallbackTakeaways(nCtx, NarrationTypePlanSummary)
	require.Len(t, items, 1)
	assert.Contains(t, items[0], "AI narration is not available")
}

func TestBuildFallbackTakeaways_CashRunway(t *testing.T) {
	nCtx := minimalCtx()
	nCtx.CashRunwayMonths = &FinancialMetric{Value: 18}
	nCtx.CashPosition = &FinancialMetric{Value: 900_000}
	nCtx.BurnRate = &FinancialMetric{Value: 50_000}

	items := buildFallbackTakeaways(nCtx, NarrationTypeCashRunway)
	require.Len(t, items, 3)
	assert.Contains(t, items[0], "18.0 months")
	assert.Contains(t, items[1], "900000")
	assert.Contains(t, items[2], "50000")
}

func TestBuildFallbackTakeaways_VarianceMaxThree(t *testing.T) {
	nCtx := minimalCtx()
	for i := 0; i < 5; i++ {
		nCtx.VarianceLines = append(nCtx.VarianceLines, VarianceLine{
			Label:      fmt.Sprintf("Line %d", i),
			Variance:   float64(i * -1000),
			VarPct:     float64(i * -5),
			Favourable: false,
		})
	}
	items := buildFallbackTakeaways(nCtx, NarrationTypeVarianceAnalysis)
	assert.LessOrEqual(t, len(items), 3, "variance takeaways must be capped at 3")
}

// ============================================================================
// parseAIResponse — direct unit tests
// ============================================================================

func TestParseAIResponse_ValidJSON(t *testing.T) {
	svc := &AINarrationService{log: logrus.NewEntry(logrus.New())}
	raw := `{"title":"T","summary":"S","paragraphs":[{"content":"P","type":"info"}],"key_takeaways":["K1"]}`
	out, err := svc.parseAIResponse(raw, minimalCtx(), NarrationTypePlanSummary)
	require.NoError(t, err)
	assert.Equal(t, "T", out.Title)
	assert.Equal(t, "S", out.Summary)
	require.Len(t, out.Paragraphs, 1)
	assert.Equal(t, "P", out.Paragraphs[0].Content)
}

func TestParseAIResponse_JSONWithProse(t *testing.T) {
	// AI often wraps JSON in prose or markdown — ExtractJSON should strip it.
	svc := &AINarrationService{log: logrus.NewEntry(logrus.New())}
	raw := `Here is your narration:
` + "```json" + `
{"title":"T","summary":"S","paragraphs":[],"key_takeaways":[]}
` + "```"
	out, err := svc.parseAIResponse(raw, minimalCtx(), NarrationTypePlanSummary)
	require.NoError(t, err)
	assert.Equal(t, "S", out.Summary)
}

func TestParseAIResponse_NoJSON(t *testing.T) {
	svc := &AINarrationService{log: logrus.NewEntry(logrus.New())}
	_, err := svc.parseAIResponse("totally free prose, no JSON", minimalCtx(), NarrationTypePlanSummary)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "extractJSON")
}

func TestParseAIResponse_InvalidJSON(t *testing.T) {
	svc := &AINarrationService{log: logrus.NewEntry(logrus.New())}
	_, err := svc.parseAIResponse(`{invalid json`, minimalCtx(), NarrationTypePlanSummary)
	require.Error(t, err)
}

func TestParseAIResponse_MissingSummary(t *testing.T) {
	svc := &AINarrationService{log: logrus.NewEntry(logrus.New())}
	raw := `{"title":"T","summary":"","paragraphs":[],"key_takeaways":[]}`
	_, err := svc.parseAIResponse(raw, minimalCtx(), NarrationTypePlanSummary)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "summary")
}

func TestParseAIResponse_EmptyTitleGotsFallback(t *testing.T) {
	svc := &AINarrationService{log: logrus.NewEntry(logrus.New())}
	raw := `{"title":"","summary":"Summary here.","paragraphs":[],"key_takeaways":[]}`
	nCtx := minimalCtx()
	nCtx.NarrationType = NarrationTypeCashRunway
	nCtx.ScenarioName = "Base"
	nCtx.PeriodLabel = "FY 2026"
	out, err := svc.parseAIResponse(raw, nCtx, NarrationTypeCashRunway)
	require.NoError(t, err)
	assert.Contains(t, out.Title, "Cash Runway", "empty title should be replaced by fallback")
}

// ============================================================================
// systemPrompt — audience text
// ============================================================================

func TestSystemPrompt_AdminOwnerAudience(t *testing.T) {
	for _, role := range []NarrationUserRole{NarrationRoleAdmin, NarrationRoleOwner} {
		p := systemPrompt(role, "en")
		assert.Contains(t, p, "Business owner", "role %s prompt should mention business owner", role)
		assert.Contains(t, p, "Analytical", "role %s prompt should mention analytical tone", role)
	}
}

func TestSystemPrompt_UserAudience(t *testing.T) {
	p := systemPrompt(NarrationRoleUser, "en")
	assert.Contains(t, p, "Team member")
}

func TestSystemPrompt_ViewerAudience(t *testing.T) {
	p := systemPrompt(NarrationRoleViewer, "en")
	assert.Contains(t, p, "Read-only stakeholder")
}

func TestSystemPrompt_UnknownRoleUsesBase(t *testing.T) {
	p := systemPrompt("unknown_role", "en")
	assert.Contains(t, p, "Ascenda")
	// Should NOT contain audience-specific sections.
	assert.NotContains(t, p, "AUDIENCE:")
}

func TestSystemPrompt_AllRolesContainOutputFormat(t *testing.T) {
	roles := []NarrationUserRole{NarrationRoleAdmin, NarrationRoleOwner, NarrationRoleUser, NarrationRoleViewer}
	for _, role := range roles {
		p := systemPrompt(role, "en")
		assert.Contains(t, p, "OUTPUT FORMAT", "role %s prompt must describe expected JSON format", role)
	}
}

func TestSystemPrompt_FrenchLanguageInstruction(t *testing.T) {
	p := systemPrompt(NarrationRoleAdmin, "fr")
	assert.Contains(t, p, "LANGUAGE:", "French prompt must include LANGUAGE instruction")
	assert.Contains(t, p, "French", "French prompt must name the language")
}

func TestSystemPrompt_EnglishNoLanguageInstruction(t *testing.T) {
	p := systemPrompt(NarrationRoleAdmin, "en")
	assert.NotContains(t, p, "LANGUAGE:", "English prompt must not add LANGUAGE instruction")
}

// ============================================================================
// narrationInstructions — type-specific content
// ============================================================================

func TestNarrationInstructions_VarianceAnalysis(t *testing.T) {
	instr := narrationInstructions(NarrationTypeVarianceAnalysis, NarrationRoleOwner)
	assert.Contains(t, instr, "variance")
	assert.Contains(t, instr, "favourable")
}

func TestNarrationInstructions_ScenarioComparison(t *testing.T) {
	instr := narrationInstructions(NarrationTypeScenarioComparison, NarrationRoleOwner)
	assert.Contains(t, instr, "scenario")
	// The prohibition must be stated explicitly, not absent entirely.
	assert.Contains(t, instr, "Do NOT recommend which scenario to choose",
		"scenario comparison must explicitly prohibit recommending a scenario")
}

func TestNarrationInstructions_AnomalyDetection(t *testing.T) {
	instr := narrationInstructions(NarrationTypeAnomalyDetection, NarrationRoleOwner)
	assert.Contains(t, instr, "anomal")
}

func TestNarrationInstructions_CashRunway(t *testing.T) {
	instr := narrationInstructions(NarrationTypeCashRunway, NarrationRoleOwner)
	assert.Contains(t, instr, "runway")
	// The prohibition must be stated explicitly in the prompt.
	assert.Contains(t, instr, "Do NOT provide investment or fundraising advice",
		"cash runway prompt must explicitly prohibit fundraising advice")
}

func TestNarrationInstructions_PlanSummaryOwner(t *testing.T) {
	instr := narrationInstructions(NarrationTypePlanSummary, NarrationRoleOwner)
	assert.Contains(t, instr, "executive")
}

func TestNarrationInstructions_PlanSummaryViewer(t *testing.T) {
	instr := narrationInstructions(NarrationTypePlanSummary, NarrationRoleViewer)
	assert.Contains(t, instr, "high-level")
}

func TestNarrationInstructions_PlanSummaryUser(t *testing.T) {
	instr := narrationInstructions(NarrationTypePlanSummary, NarrationRoleUser)
	assert.Contains(t, instr, "team")
}

// ============================================================================
// abs helper
// ============================================================================

func TestAbs_Positive(t *testing.T) {
	assert.Equal(t, 5.0, abs(5.0))
}

func TestAbs_Negative(t *testing.T) {
	assert.Equal(t, 5.0, abs(-5.0))
}

func TestAbs_Zero(t *testing.T) {
	assert.Equal(t, 0.0, abs(0))
}

// ============================================================================
// IsConfigured
// ============================================================================

func TestAINarrationService_IsConfiguredFalseWithoutKey(t *testing.T) {
	svc := &AINarrationService{
		client: &aiClient.Client{},
		log:    logrus.NewEntry(logrus.New()),
	}
	assert.False(t, svc.IsConfigured())
}

func TestAINarrationService_IsConfiguredTrueWithKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(srv.Close)
	c := aiClient.ClientForTest("openai", "my-key", srv.URL)
	svc := &AINarrationService{client: &c, log: logrus.NewEntry(logrus.New())}
	assert.True(t, svc.IsConfigured())
}

// ============================================================================
// generateFallback — explicit NarrationType propagation
// ============================================================================

func TestGenerateFallback_CashRunwayType(t *testing.T) {
	svc := &AINarrationService{log: logrus.NewEntry(logrus.New())}
	nCtx := minimalCtx()
	nCtx.CashRunwayMonths = &FinancialMetric{Value: 6.0}

	out := svc.generateFallback(nCtx, NarrationTypeCashRunway)
	assert.False(t, out.IsAIGenerated)
	assert.Contains(t, out.Title, "Cash Runway")
	assert.Contains(t, out.Summary, "6.0 months")
	require.NotEmpty(t, out.KeyTakeaways)
	assert.Contains(t, out.KeyTakeaways[0], "6.0 months")
}

func TestGenerateFallback_ScenarioComparison(t *testing.T) {
	svc := &AINarrationService{log: logrus.NewEntry(logrus.New())}
	nCtx := minimalCtx()
	nCtx.Scenarios = []ScenarioSummary{
		{ScenarioName: "Base"},
		{ScenarioName: "Optimistic"},
	}

	out := svc.generateFallback(nCtx, NarrationTypeScenarioComparison)
	assert.Contains(t, out.Title, "Scenario Comparison")
	// Scenario comparison falls through to default takeaways (no specific logic).
	assert.NotEmpty(t, out.Summary)
}

func TestGenerateFallback_AlwaysHasParagraph(t *testing.T) {
	svc := &AINarrationService{log: logrus.NewEntry(logrus.New())}
	nCtx := minimalCtx()

	for _, nt := range []NarrationType{
		NarrationTypePlanSummary,
		NarrationTypeVarianceAnalysis,
		NarrationTypeScenarioComparison,
		NarrationTypeAnomalyDetection,
		NarrationTypeCashRunway,
	} {
		out := svc.generateFallback(nCtx, nt)
		assert.NotEmpty(t, out.Paragraphs, "fallback for %s must include at least one paragraph", nt)
		assert.Equal(t, "info", out.Paragraphs[0].Type)
	}
}
