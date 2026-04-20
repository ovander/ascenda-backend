package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
)

// ── mockRatiosService ─────────────────────────────────────────────────────

type mockRatiosService struct {
	getReportFn    func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.RatiosReport, error)
	getChartDataFn func(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]interface{}, error)
}

func (m *mockRatiosService) GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.RatiosReport, error) {
	if m.getReportFn != nil {
		return m.getReportFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("get report not implemented")
}

func (m *mockRatiosService) GetChartData(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]interface{}, error) {
	if m.getChartDataFn != nil {
		return m.getChartDataFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("get chart data not implemented")
}

// ── Helper ────────────────────────────────────────────────────────────────

func newRatiosHandler(svc *mockRatiosService) *RatiosHandler {
	return NewRatiosHandler(svc, logrus.NewEntry(logrus.New()))
}

// ── GetReport ─────────────────────────────────────────────────────────────

func TestRatiosHandler_GetReport_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	expectedReport := &model.RatiosReport{}

	svc := &mockRatiosService{
		getReportFn: func(_ context.Context, tid, sid uuid.UUID) (*model.RatiosReport, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return expectedReport, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newRatiosHandler(svc).GetReport(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

func TestRatiosHandler_GetReport_BadScenarioUUID(t *testing.T) {
	svc := &mockRatiosService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad-uuid"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newRatiosHandler(svc).GetReport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRatiosHandler_GetReport_ServiceError(t *testing.T) {
	svc := &mockRatiosService{
		getReportFn: func(_ context.Context, _, _ uuid.UUID) (*model.RatiosReport, error) {
			return nil, apierror.NotFound("scenario", "missing")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newRatiosHandler(svc).GetReport(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRatiosHandler_GetReport_InternalServerError(t *testing.T) {
	svc := &mockRatiosService{
		getReportFn: func(_ context.Context, _, _ uuid.UUID) (*model.RatiosReport, error) {
			return nil, apierror.Internal("calculation error")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newRatiosHandler(svc).GetReport(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ── GetChartData ──────────────────────────────────────────────────────────

func TestRatiosHandler_GetChartData_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	expectedData := map[string]interface{}{
		"ratios": map[string]float64{
			"current":      2.0,
			"quick":        1.8,
			"debt_to_equity": 1.5,
		},
		"years": []int{2024, 2025, 2026},
	}

	svc := &mockRatiosService{
		getChartDataFn: func(_ context.Context, tid, sid uuid.UUID) (map[string]interface{}, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return expectedData, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newRatiosHandler(svc).GetChartData(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got["ratios"])
	assert.NotNil(t, got["years"])
}

func TestRatiosHandler_GetChartData_BadScenarioUUID(t *testing.T) {
	svc := &mockRatiosService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "invalid-uuid"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newRatiosHandler(svc).GetChartData(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRatiosHandler_GetChartData_ServiceError(t *testing.T) {
	svc := &mockRatiosService{
		getChartDataFn: func(_ context.Context, _, _ uuid.UUID) (map[string]interface{}, error) {
			return nil, apierror.Internal("ratio calculation failed")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newRatiosHandler(svc).GetChartData(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRatiosHandler_GetChartData_NotFound(t *testing.T) {
	svc := &mockRatiosService{
		getChartDataFn: func(_ context.Context, _, _ uuid.UUID) (map[string]interface{}, error) {
			return nil, apierror.NotFound("ratios", "not found")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newRatiosHandler(svc).GetChartData(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
