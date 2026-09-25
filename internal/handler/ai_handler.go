// Package handler — AI narration handler.
// Exposes HTTP endpoints for all AI-powered financial intelligence features.
// Each endpoint is protected by the AI access middleware (quota + tier gate);
// the handler itself only validates the request and delegates to the narration service.
//
// Architecture note:
//
//	The caller (frontend or integration) is responsible for building the
//	NarrationContext from plan data before calling these endpoints.
//	The handler does NOT re-fetch financial data from the database —
//	this keeps the AI layer stateless and independently testable.
package handler

import (
	"net/http"

	"ascenda/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"
)

// AIHandler handles all AI-powered narration requests.
type AIHandler struct {
	narration  *service.AINarrationService
	sensEngine *service.SensitivityEngine
	logger     *logrus.Entry
}

// NewAIHandler creates a new AIHandler.
func NewAIHandler(
	narration *service.AINarrationService,
	sensEngine *service.SensitivityEngine,
	logger *logrus.Entry,
) *AIHandler {
	return &AIHandler{
		narration:  narration,
		sensEngine: sensEngine,
		logger:     logger,
	}
}

// ============================================================================
// Request / Response types
// ============================================================================

// AIFeatureRequest is the common request body for all AI narration endpoints.
// The caller pre-computes the NarrationContext from its local plan data.
type AIFeatureRequest struct {
	Context service.NarrationContext `json:"context" validate:"required"`
}

// AIFeatureResponse wraps the narration output returned to the client.
type AIFeatureResponse struct {
	Narration *service.NarrationOutput `json:"narration"`
}

// respondAINarration writes a JSON response for an AI narration result.
// When the output is a deterministic fallback (IsAIGenerated = false) it sets
// the X-AI-Generated: false response header so that AIUsageRecorder can skip
// quota consumption — a failed AI call should not count against the user's
// daily/weekly/monthly limit.
func respondAINarration(w http.ResponseWriter, out *service.NarrationOutput) {
	if out != nil && !out.IsAIGenerated {
		w.Header().Set("X-AI-Generated", "false")
	}
	respondJSON(w, http.StatusOK, AIFeatureResponse{Narration: out})
}

// ============================================================================
// Standard-tier endpoints
// ============================================================================

// Narrate handles POST /ai/narrate — general plan narration (standard tier).
// The NarrationType in the context selects the narration mode; if absent,
// the service infers a sensible default from the context fields.
func (h *AIHandler) Narrate(w http.ResponseWriter, r *http.Request) {
	var req AIFeatureRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	out, err := h.narration.GenerateNarration(r.Context(), &req.Context)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondAINarration(w, out)
}

// ============================================================================
// Pro-tier driver-aware endpoints
// ============================================================================

// UnitEconomics handles POST /ai/unit-economics.
// Expects product_economics in the context for driver-specific KPI narration.
func (h *AIHandler) UnitEconomics(w http.ResponseWriter, r *http.Request) {
	var req AIFeatureRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	req.Context.NarrationType = service.NarrationTypeUnitEconomics
	out, err := h.narration.GenerateNarration(r.Context(), &req.Context)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondAINarration(w, out)
}

// AssumptionReview handles POST /ai/assumption-review.
// Expects assumption_flags in the context (pre-computed by the caller).
func (h *AIHandler) AssumptionReview(w http.ResponseWriter, r *http.Request) {
	var req AIFeatureRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	req.Context.NarrationType = service.NarrationTypeAssumptionReview
	out, err := h.narration.GenerateNarration(r.Context(), &req.Context)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondAINarration(w, out)
}

// BenchmarkCommentary handles POST /ai/benchmark-commentary.
// Expects primary_driver_type and product_economics in the context.
func (h *AIHandler) BenchmarkCommentary(w http.ResponseWriter, r *http.Request) {
	var req AIFeatureRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	req.Context.NarrationType = service.NarrationTypeBenchmarkCommentary
	out, err := h.narration.GenerateNarration(r.Context(), &req.Context)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondAINarration(w, out)
}

// PortfolioMix handles POST /ai/portfolio-mix.
// Expects product_economics (multiple products) in the context.
func (h *AIHandler) PortfolioMix(w http.ResponseWriter, r *http.Request) {
	var req AIFeatureRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	req.Context.NarrationType = service.NarrationTypePortfolioMix
	out, err := h.narration.GenerateNarration(r.Context(), &req.Context)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondAINarration(w, out)
}

// DriverAdvisor handles POST /ai/driver-advisor.
// Analyses generic-driver products and recommends structured driver types.
// Returns structured_data in the narration output with recommendations.
func (h *AIHandler) DriverAdvisor(w http.ResponseWriter, r *http.Request) {
	var req AIFeatureRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	req.Context.NarrationType = service.NarrationTypeDriverAdvisor
	out, err := h.narration.GenerateNarration(r.Context(), &req.Context)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondAINarration(w, out)
}

// ScenarioSuggestion handles POST /ai/scenario-suggestion.
// Generates a bear/bull/stress parameter diff for the scenario.
// Expects scenario_type ("bear", "bull", "stress") and key_metrics in the context.
// Returns structured_data with a list of ScenarioParamDiff in the narration output.
func (h *AIHandler) ScenarioSuggestion(w http.ResponseWriter, r *http.Request) {
	var req AIFeatureRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	req.Context.NarrationType = service.NarrationTypeScenarioSuggestion
	out, err := h.narration.GenerateNarration(r.Context(), &req.Context)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondAINarration(w, out)
}

// SensitivityNarrative handles POST /ai/sensitivity-narrative.
// Computes sensitivity levers server-side by perturbing the financial model,
// then passes them to the AI narration service.
func (h *AIHandler) SensitivityNarrative(w http.ResponseWriter, r *http.Request) {
	var req AIFeatureRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	// Compute sensitivity levers from the live financial model — the backend
	// owns this calculation; the frontend does not need to send lever data.
	if h.sensEngine != nil {
		scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
		if err != nil {
			handleError(w, r, err)
			return
		}
		tenantID := ctxutil.GetTenantID(r.Context())
		levers, err := h.sensEngine.ComputeSensitivityLevers(r.Context(), tenantID, scenarioID)
		if err != nil {
			// Non-fatal: proceed without lever data rather than failing the whole request.
			h.logger.WithError(err).Warn("failed to compute sensitivity levers — proceeding without them")
		} else {
			req.Context.SensitivityLevers = levers
		}
	}

	req.Context.NarrationType = service.NarrationTypeSensitivityNarrative
	out, err := h.narration.GenerateNarration(r.Context(), &req.Context)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondAINarration(w, out)
}

// ============================================================================
// Enterprise endpoint
// ============================================================================

// InvestorMemo handles POST /ai/investor-memo (Enterprise, owner only).
// Generates a full investor-ready plan narrative.
func (h *AIHandler) InvestorMemo(w http.ResponseWriter, r *http.Request) {
	var req AIFeatureRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	req.Context.NarrationType = service.NarrationTypeInvestorMemo
	out, err := h.narration.GenerateNarration(r.Context(), &req.Context)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondAINarration(w, out)
}
