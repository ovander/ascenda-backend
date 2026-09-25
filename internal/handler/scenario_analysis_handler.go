// Package handler — Scenario Analysis HTTP handler.
package handler

import (
	"context"
	"net/http"

	"ascenda/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"
)

// ScenarioAnalysisSvcer is the narrow interface the handler depends on.
// Depending on this interface (not the concrete *service.ScenarioAnalysisService)
// keeps the handler fully testable with a mock.
type ScenarioAnalysisSvcer interface {
	AnalyzeScenario(ctx context.Context, tenantID, scenarioID uuid.UUID) (*service.ScenarioAnalysisResult, error)
}

// Compile-time check that *service.ScenarioAnalysisService satisfies the interface.
var _ ScenarioAnalysisSvcer = (*service.ScenarioAnalysisService)(nil)

// ScenarioAnalysisHandler handles GET /plans/{planId}/scenarios/{scenarioId}/analysis.
type ScenarioAnalysisHandler struct {
	svc    ScenarioAnalysisSvcer
	logger *logrus.Entry
}

// NewScenarioAnalysisHandler creates a ScenarioAnalysisHandler.
func NewScenarioAnalysisHandler(svc ScenarioAnalysisSvcer, logger *logrus.Entry) *ScenarioAnalysisHandler {
	return &ScenarioAnalysisHandler{svc: svc, logger: logger}
}

// Analyze godoc
//
//	GET /api/v1/plans/{planId}/scenarios/{scenarioId}/analysis
//
// Returns a structured decision-intelligence analysis for the scenario,
// including viability scoring, projections, risks, drivers, and trend series.
func (h *ScenarioAnalysisHandler) Analyze(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())

	result, err := h.svc.AnalyzeScenario(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, result)
}
