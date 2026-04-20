package handler

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/ovander/backendkit/ctxutil"
	"ascenda/internal/service"
)

// GraphServicer defines the interface the GraphHandler depends on.
type GraphServicer interface {
	GetAllAnnualCharts(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]*service.ChartData, error)
	GetAnnualChart(ctx context.Context, tenantID, scenarioID uuid.UUID, name string) (*service.ChartData, error)
	GetMonthlyChart(ctx context.Context, tenantID, scenarioID uuid.UUID, name string) (*service.ChartData, error)
}

// GraphHandler serves computed chart data for the Graphs dashboard.
type GraphHandler struct {
	svc    GraphServicer
	logger *logrus.Entry
}

// NewGraphHandler creates a new GraphHandler.
func NewGraphHandler(svc GraphServicer, logger *logrus.Entry) *GraphHandler {
	return &GraphHandler{svc: svc, logger: logger}
}

// GetAllAnnualCharts handles GET /graphs/annual/all and returns every annual
// chart in a single response, avoiding per-request rate-limit exhaustion.
func (h *GraphHandler) GetAllAnnualCharts(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	data, err := h.svc.GetAllAnnualCharts(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, data)
}

// GetAnnualChart handles GET /graphs/annual?name=<chartName>
func (h *GraphHandler) GetAnnualChart(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "name query parameter is required"})
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	data, err := h.svc.GetAnnualChart(r.Context(), tenantID, scenarioID, name)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, data)
}

// GetMonthlyChart handles GET /graphs/monthly?name=<chartName>
func (h *GraphHandler) GetMonthlyChart(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "name query parameter is required"})
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	data, err := h.svc.GetMonthlyChart(r.Context(), tenantID, scenarioID, name)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, data)
}
