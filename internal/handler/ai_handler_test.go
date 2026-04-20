package handler

// Tests for AIHandler — all 9 AI narration endpoints.
//
// Strategy: use a real AINarrationService with no AI client configured so
// every call produces a deterministic fallback (IsAIGenerated = false).
// This avoids HTTP mocking complexity inside the handler layer and keeps
// tests focused on HTTP mechanics: request decoding, NarrationType injection,
// and response encoding.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ascenda/internal/config"
	"ascenda/internal/service"
)

// ============================================================================
// Test helpers
// ============================================================================

// newTestAIHandler creates an AIHandler backed by an unconfigured narration
// service (no AI API key → always returns deterministic fallback).
func newTestAIHandler() *AIHandler {
	svc := service.NewAINarrationService(config.AIConfig{}, logrus.NewEntry(logrus.New()))
	// nil SensitivityEngine: no DB is available in unit tests; the handler
	// gracefully logs a warning and proceeds without lever data.
	return NewAIHandler(svc, nil, logrus.NewEntry(logrus.New()))
}

// minimalAIRequest builds a minimal valid AIFeatureRequest JSON body.
func minimalAIRequest() []byte {
	req := AIFeatureRequest{
		Context: service.NarrationContext{
			PlanName:     "Budget 2026",
			ScenarioName: "Base",
			PeriodLabel:  "FY 2026",
			Currency:     "EUR",
			UserRole:     "owner",
		},
	}
	b, _ := json.Marshal(req)
	return b
}

// doRequest sends body to the given handler function and returns the recorder.
func doRequest(t *testing.T, handlerFn http.HandlerFunc, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/ai/test", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handlerFn(w, r)
	return w
}

// decodeAIResponse parses the response body into an AIFeatureResponse.
func decodeAIResponse(t *testing.T, w *httptest.ResponseRecorder) AIFeatureResponse {
	t.Helper()
	var resp AIFeatureResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

// ============================================================================
// Narrate — standard tier general endpoint
// ============================================================================

func TestAIHandler_Narrate_Success(t *testing.T) {
	h := newTestAIHandler()
	w := doRequest(t, h.Narrate, minimalAIRequest())

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	resp := decodeAIResponse(t, w)
	require.NotNil(t, resp.Narration)
	assert.NotEmpty(t, resp.Narration.Title)
	assert.NotEmpty(t, resp.Narration.Summary)
}

func TestAIHandler_Narrate_InvalidJSON_Returns400(t *testing.T) {
	h := newTestAIHandler()
	r := httptest.NewRequest(http.MethodPost, "/ai/narrate", bytes.NewReader([]byte("not json")))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Narrate(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAIHandler_Narrate_EmptyBody_Returns400(t *testing.T) {
	h := newTestAIHandler()
	r := httptest.NewRequest(http.MethodPost, "/ai/narrate", bytes.NewReader([]byte{}))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Narrate(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// UnitEconomics — Pro endpoint sets NarrationType
// ============================================================================

func TestAIHandler_UnitEconomics_Success(t *testing.T) {
	h := newTestAIHandler()
	w := doRequest(t, h.UnitEconomics, minimalAIRequest())

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeAIResponse(t, w)
	require.NotNil(t, resp.Narration)
	assert.NotEmpty(t, resp.Narration.Title)
	assert.Contains(t, resp.Narration.Title, "Unit Economics",
		"UnitEconomics handler must produce a unit_economics narration")
}

func TestAIHandler_UnitEconomics_InvalidJSON_Returns400(t *testing.T) {
	h := newTestAIHandler()
	r := httptest.NewRequest(http.MethodPost, "/ai/unit-economics", bytes.NewReader([]byte("{bad}")))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.UnitEconomics(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// AssumptionReview
// ============================================================================

func TestAIHandler_AssumptionReview_Success(t *testing.T) {
	h := newTestAIHandler()
	w := doRequest(t, h.AssumptionReview, minimalAIRequest())

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeAIResponse(t, w)
	require.NotNil(t, resp.Narration)
	assert.Contains(t, resp.Narration.Title, "Assumption Review")
}

func TestAIHandler_AssumptionReview_InvalidJSON_Returns400(t *testing.T) {
	h := newTestAIHandler()
	r := httptest.NewRequest(http.MethodPost, "/ai/assumption-review", bytes.NewReader([]byte(`{bad json`)))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.AssumptionReview(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// BenchmarkCommentary
// ============================================================================

func TestAIHandler_BenchmarkCommentary_Success(t *testing.T) {
	h := newTestAIHandler()
	w := doRequest(t, h.BenchmarkCommentary, minimalAIRequest())

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeAIResponse(t, w)
	require.NotNil(t, resp.Narration)
	assert.Contains(t, resp.Narration.Title, "Benchmark Commentary")
}

// ============================================================================
// PortfolioMix
// ============================================================================

func TestAIHandler_PortfolioMix_Success(t *testing.T) {
	h := newTestAIHandler()
	w := doRequest(t, h.PortfolioMix, minimalAIRequest())

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeAIResponse(t, w)
	require.NotNil(t, resp.Narration)
	assert.Contains(t, resp.Narration.Title, "Portfolio Mix")
}

// ============================================================================
// DriverAdvisor
// ============================================================================

func TestAIHandler_DriverAdvisor_Success(t *testing.T) {
	h := newTestAIHandler()
	w := doRequest(t, h.DriverAdvisor, minimalAIRequest())

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeAIResponse(t, w)
	require.NotNil(t, resp.Narration)
	assert.Contains(t, resp.Narration.Title, "Driver Advisor")
}

// ============================================================================
// ScenarioSuggestion
// ============================================================================

func TestAIHandler_ScenarioSuggestion_Success(t *testing.T) {
	h := newTestAIHandler()
	w := doRequest(t, h.ScenarioSuggestion, minimalAIRequest())

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeAIResponse(t, w)
	require.NotNil(t, resp.Narration)
	assert.Contains(t, resp.Narration.Title, "Scenario Suggestion")
}

func TestAIHandler_ScenarioSuggestion_WithScenarioType(t *testing.T) {
	h := newTestAIHandler()
	req := AIFeatureRequest{
		Context: service.NarrationContext{
			PlanName:     "Budget 2026",
			ScenarioName: "Bear Case",
			PeriodLabel:  "FY 2026",
			Currency:     "EUR",
			UserRole:     "owner",
			ScenarioType: "bear",
		},
	}
	body, _ := json.Marshal(req)
	w := doRequest(t, h.ScenarioSuggestion, body)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeAIResponse(t, w)
	require.NotNil(t, resp.Narration)
	// The fallback title for scenario_suggestion includes the scenario type.
	assert.Contains(t, resp.Narration.Title, "bear")
}

// ============================================================================
// SensitivityNarrative
// ============================================================================

func TestAIHandler_SensitivityNarrative_Success(t *testing.T) {
	h := newTestAIHandler()
	w := doRequest(t, h.SensitivityNarrative, minimalAIRequest())

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeAIResponse(t, w)
	require.NotNil(t, resp.Narration)
	assert.Contains(t, resp.Narration.Title, "Sensitivity Narrative")
}

// ============================================================================
// InvestorMemo — Enterprise endpoint
// ============================================================================

func TestAIHandler_InvestorMemo_Success(t *testing.T) {
	h := newTestAIHandler()
	w := doRequest(t, h.InvestorMemo, minimalAIRequest())

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeAIResponse(t, w)
	require.NotNil(t, resp.Narration)
	assert.Contains(t, resp.Narration.Title, "Investor Memo")
}

func TestAIHandler_InvestorMemo_InvalidJSON_Returns400(t *testing.T) {
	h := newTestAIHandler()
	r := httptest.NewRequest(http.MethodPost, "/ai/investor-memo", bytes.NewReader([]byte(`{"context": "not an object"}`)))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.InvestorMemo(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// NarrationType injection — Pro handlers always override context type
// ============================================================================

func TestAIHandler_ProHandlers_OverrideNarrationType(t *testing.T) {
	// If the caller sets a different NarrationType in the context, the Pro
	// handler must replace it with the endpoint-specific type.
	cases := []struct {
		name    string
		handler func(h *AIHandler) http.HandlerFunc
		wantSub string
	}{
		{"UnitEconomics", func(h *AIHandler) http.HandlerFunc { return h.UnitEconomics }, "Unit Economics"},
		{"AssumptionReview", func(h *AIHandler) http.HandlerFunc { return h.AssumptionReview }, "Assumption Review"},
		{"BenchmarkCommentary", func(h *AIHandler) http.HandlerFunc { return h.BenchmarkCommentary }, "Benchmark Commentary"},
		{"PortfolioMix", func(h *AIHandler) http.HandlerFunc { return h.PortfolioMix }, "Portfolio Mix"},
		{"DriverAdvisor", func(h *AIHandler) http.HandlerFunc { return h.DriverAdvisor }, "Driver Advisor"},
		{"SensitivityNarrative", func(h *AIHandler) http.HandlerFunc { return h.SensitivityNarrative }, "Sensitivity Narrative"},
		{"InvestorMemo", func(h *AIHandler) http.HandlerFunc { return h.InvestorMemo }, "Investor Memo"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestAIHandler()
			// Send a request with NarrationType set to something different.
			req := AIFeatureRequest{
				Context: service.NarrationContext{
					PlanName:        "Budget 2026",
					ScenarioName:    "Base",
					PeriodLabel:     "FY 2026",
					Currency:        "EUR",
					UserRole:        "owner",
					NarrationType:   "plan_summary", // should be overridden
				},
			}
			body, _ := json.Marshal(req)
			w := doRequest(t, tc.handler(h), body)

			assert.Equal(t, http.StatusOK, w.Code)
			resp := decodeAIResponse(t, w)
			require.NotNil(t, resp.Narration)
			assert.Contains(t, resp.Narration.Title, tc.wantSub,
				"%s handler must produce narration of correct type", tc.name)
		})
	}
}

// ============================================================================
// Response structure — all handlers return a consistent JSON shape
// ============================================================================

func TestAIHandler_AllHandlers_ReturnValidStructure(t *testing.T) {
	h := newTestAIHandler()

	handlers := []struct {
		name string
		fn   http.HandlerFunc
	}{
		{"Narrate", h.Narrate},
		{"UnitEconomics", h.UnitEconomics},
		{"AssumptionReview", h.AssumptionReview},
		{"BenchmarkCommentary", h.BenchmarkCommentary},
		{"PortfolioMix", h.PortfolioMix},
		{"DriverAdvisor", h.DriverAdvisor},
		{"ScenarioSuggestion", h.ScenarioSuggestion},
		{"SensitivityNarrative", h.SensitivityNarrative},
		{"InvestorMemo", h.InvestorMemo},
	}

	for _, hh := range handlers {
		t.Run(hh.name, func(t *testing.T) {
			w := doRequest(t, hh.fn, minimalAIRequest())
			assert.Equal(t, http.StatusOK, w.Code, "%s must return 200", hh.name)

			var raw map[string]interface{}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw),
				"%s response must be valid JSON", hh.name)
			assert.Contains(t, raw, "narration",
				"%s response must contain 'narration' key", hh.name)

			narration, ok := raw["narration"].(map[string]interface{})
			require.True(t, ok, "%s narration field must be an object", hh.name)
			assert.Contains(t, narration, "title")
			assert.Contains(t, narration, "summary")
			assert.Contains(t, narration, "paragraphs")
			assert.Contains(t, narration, "key_takeaways")
		})
	}
}
