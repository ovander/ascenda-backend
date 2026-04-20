package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"ascenda/internal/service"
)

// ── Mock ─────────────────────────────────────────────────────────────────────

type MockGraphService struct {
	GetAllAnnualChartsFunc func(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]*service.ChartData, error)
	GetAnnualChartFunc     func(ctx context.Context, tenantID, scenarioID uuid.UUID, name string) (*service.ChartData, error)
	GetMonthlyChartFunc    func(ctx context.Context, tenantID, scenarioID uuid.UUID, name string) (*service.ChartData, error)
}

func (m *MockGraphService) GetAllAnnualCharts(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]*service.ChartData, error) {
	if m.GetAllAnnualChartsFunc != nil {
		return m.GetAllAnnualChartsFunc(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("mock not implemented")
}

func (m *MockGraphService) GetAnnualChart(ctx context.Context, tenantID, scenarioID uuid.UUID, name string) (*service.ChartData, error) {
	if m.GetAnnualChartFunc != nil {
		return m.GetAnnualChartFunc(ctx, tenantID, scenarioID, name)
	}
	return nil, apierror.Internal("mock not implemented")
}

func (m *MockGraphService) GetMonthlyChart(ctx context.Context, tenantID, scenarioID uuid.UUID, name string) (*service.ChartData, error) {
	if m.GetMonthlyChartFunc != nil {
		return m.GetMonthlyChartFunc(ctx, tenantID, scenarioID, name)
	}
	return nil, apierror.Internal("mock not implemented")
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newGraphRequest(method, path string, scenarioID uuid.UUID, queryName string) *http.Request {
	r := httptest.NewRequest(method, path+"?name="+queryName, nil)
	ctx := ctxutil.WithTenantID(r.Context(), uuid.New())

	// Inject chi URL params
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("scenarioId", scenarioID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

	return r.WithContext(ctx)
}

func fixedChartData() *service.ChartData {
	return &service.ChartData{
		Labels: []string{"2025", "2026", "2027", "2028", "2029"},
		Datasets: []service.ChartDataset{
			{Label: "Revenue", Data: []float64{100, 200, 300, 400, 500}},
		},
	}
}

// ── GetAllAnnualCharts ────────────────────────────────────────────────────────

func TestGraphHandler_GetAllAnnualCharts_Success(t *testing.T) {
	scenarioID := uuid.New()
	allCharts := map[string]*service.ChartData{
		"sales-analysis": fixedChartData(),
		"pnl-cascade":    fixedChartData(),
	}
	mock := &MockGraphService{
		GetAllAnnualChartsFunc: func(_ context.Context, _, _ uuid.UUID) (map[string]*service.ChartData, error) {
			return allCharts, nil
		},
	}
	h := NewGraphHandler(mock, logrus.NewEntry(logrus.StandardLogger()))

	r := httptest.NewRequest("GET", "/graphs/annual/all", nil)
	ctx := ctxutil.WithTenantID(r.Context(), uuid.New())
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("scenarioId", scenarioID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()
	h.GetAllAnnualCharts(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp map[string]*service.ChartData
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp, "sales-analysis")
	assert.Contains(t, resp, "pnl-cascade")
}

func TestGraphHandler_GetAllAnnualCharts_ServiceError(t *testing.T) {
	scenarioID := uuid.New()
	mock := &MockGraphService{
		GetAllAnnualChartsFunc: func(_ context.Context, _, _ uuid.UUID) (map[string]*service.ChartData, error) {
			return nil, apierror.Internal("compute failed")
		},
	}
	h := NewGraphHandler(mock, logrus.NewEntry(logrus.StandardLogger()))

	r := httptest.NewRequest("GET", "/graphs/annual/all", nil)
	ctx := ctxutil.WithTenantID(r.Context(), uuid.New())
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("scenarioId", scenarioID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()
	h.GetAllAnnualCharts(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ── GetAnnualChart ────────────────────────────────────────────────────────────

func TestGraphHandler_GetAnnualChart_Success(t *testing.T) {
	scenarioID := uuid.New()
	mock := &MockGraphService{
		GetAnnualChartFunc: func(_ context.Context, _, _ uuid.UUID, name string) (*service.ChartData, error) {
			assert.Equal(t, "sales-analysis", name)
			return fixedChartData(), nil
		},
	}
	h := NewGraphHandler(mock, logrus.NewEntry(logrus.StandardLogger()))

	r := newGraphRequest("GET", "/graphs/annual", scenarioID, "sales-analysis")
	w := httptest.NewRecorder()
	h.GetAnnualChart(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp service.ChartData
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, []string{"2025", "2026", "2027", "2028", "2029"}, resp.Labels)
	assert.Len(t, resp.Datasets, 1)
	assert.Equal(t, "Revenue", resp.Datasets[0].Label)
}

func TestGraphHandler_GetAnnualChart_MissingName(t *testing.T) {
	h := NewGraphHandler(&MockGraphService{}, logrus.NewEntry(logrus.StandardLogger()))

	scenarioID := uuid.New()
	r := httptest.NewRequest("GET", "/graphs/annual", nil) // no ?name=
	ctx := ctxutil.WithTenantID(r.Context(), uuid.New())
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("scenarioId", scenarioID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()
	h.GetAnnualChart(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGraphHandler_GetAnnualChart_InvalidScenarioID(t *testing.T) {
	h := NewGraphHandler(&MockGraphService{}, logrus.NewEntry(logrus.StandardLogger()))

	r := httptest.NewRequest("GET", "/graphs/annual?name=sales-analysis", nil)
	ctx := ctxutil.WithTenantID(r.Context(), uuid.New())
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("scenarioId", "not-a-uuid")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()
	h.GetAnnualChart(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGraphHandler_GetAnnualChart_UnknownName(t *testing.T) {
	scenarioID := uuid.New()
	mock := &MockGraphService{
		GetAnnualChartFunc: func(_ context.Context, _, _ uuid.UUID, name string) (*service.ChartData, error) {
			return nil, apierror.NotFound("annual chart", name)
		},
	}
	h := NewGraphHandler(mock, logrus.NewEntry(logrus.StandardLogger()))

	r := newGraphRequest("GET", "/graphs/annual", scenarioID, "nonexistent-chart")
	w := httptest.NewRecorder()
	h.GetAnnualChart(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGraphHandler_GetAnnualChart_ServiceError(t *testing.T) {
	scenarioID := uuid.New()
	mock := &MockGraphService{
		GetAnnualChartFunc: func(_ context.Context, _, _ uuid.UUID, _ string) (*service.ChartData, error) {
			return nil, apierror.Internal("compute failed")
		},
	}
	h := NewGraphHandler(mock, logrus.NewEntry(logrus.StandardLogger()))

	r := newGraphRequest("GET", "/graphs/annual", scenarioID, "sales-analysis")
	w := httptest.NewRecorder()
	h.GetAnnualChart(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ── GetMonthlyChart ───────────────────────────────────────────────────────────

func TestGraphHandler_GetMonthlyChart_Success(t *testing.T) {
	scenarioID := uuid.New()

	monthlyData := &service.ChartData{
		Labels: make([]string, 36),
		Datasets: []service.ChartDataset{
			{Label: "Cash Balance", Data: make([]float64, 36)},
		},
	}
	for i := range monthlyData.Labels {
		monthlyData.Labels[i] = "Jan 2025"
	}

	mock := &MockGraphService{
		GetMonthlyChartFunc: func(_ context.Context, _, _ uuid.UUID, name string) (*service.ChartData, error) {
			assert.Equal(t, "cash-equity-debt", name)
			return monthlyData, nil
		},
	}
	h := NewGraphHandler(mock, logrus.NewEntry(logrus.StandardLogger()))

	r := newGraphRequest("GET", "/graphs/monthly", scenarioID, "cash-equity-debt")
	w := httptest.NewRecorder()
	h.GetMonthlyChart(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp service.ChartData
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Labels, 36)
	assert.Len(t, resp.Datasets, 1)
}

func TestGraphHandler_GetMonthlyChart_MissingName(t *testing.T) {
	h := NewGraphHandler(&MockGraphService{}, logrus.NewEntry(logrus.StandardLogger()))

	scenarioID := uuid.New()
	r := httptest.NewRequest("GET", "/graphs/monthly", nil)
	ctx := ctxutil.WithTenantID(r.Context(), uuid.New())
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("scenarioId", scenarioID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()
	h.GetMonthlyChart(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGraphHandler_GetMonthlyChart_UnknownName(t *testing.T) {
	scenarioID := uuid.New()
	mock := &MockGraphService{
		GetMonthlyChartFunc: func(_ context.Context, _, _ uuid.UUID, name string) (*service.ChartData, error) {
			return nil, apierror.NotFound("monthly chart", name)
		},
	}
	h := NewGraphHandler(mock, logrus.NewEntry(logrus.StandardLogger()))

	r := newGraphRequest("GET", "/graphs/monthly", scenarioID, "bogus-chart")
	w := httptest.NewRecorder()
	h.GetMonthlyChart(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGraphHandler_GetMonthlyChart_PropagatesTenantID(t *testing.T) {
	scenarioID := uuid.New()
	tenantID := uuid.New()
	capturedTenantID := uuid.Nil

	mock := &MockGraphService{
		GetMonthlyChartFunc: func(_ context.Context, tID, _ uuid.UUID, _ string) (*service.ChartData, error) {
			capturedTenantID = tID
			return fixedChartData(), nil
		},
	}
	h := NewGraphHandler(mock, logrus.NewEntry(logrus.StandardLogger()))

	r := httptest.NewRequest("GET", "/graphs/monthly?name=headcount", nil)
	ctx := ctxutil.WithTenantID(r.Context(), tenantID)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("scenarioId", scenarioID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()
	h.GetMonthlyChart(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, tenantID, capturedTenantID)
}
