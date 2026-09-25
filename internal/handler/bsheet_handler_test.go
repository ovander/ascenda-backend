package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── mockBSheetService ─────────────────────────────────────────────────────

type mockBSheetService struct {
	getReportFn    func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.BSheetReport, error)
	getChartDataFn func(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]interface{}, error)
}

func (m *mockBSheetService) GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.BSheetReport, error) {
	if m.getReportFn != nil {
		return m.getReportFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("get report not implemented")
}

func (m *mockBSheetService) GetChartData(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]interface{}, error) {
	if m.getChartDataFn != nil {
		return m.getChartDataFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("get chart data not implemented")
}

// ── Helper ────────────────────────────────────────────────────────────────

func newBSheetHandler(svc *mockBSheetService) *BSheetHandler {
	return NewBSheetHandler(svc, logrus.NewEntry(logrus.New()))
}

// ── GetReport ─────────────────────────────────────────────────────────────

func TestBSheetHandler_GetReport_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	expectedReport := &model.BSheetReport{}

	svc := &mockBSheetService{
		getReportFn: func(_ context.Context, tid, sid uuid.UUID) (*model.BSheetReport, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return expectedReport, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newBSheetHandler(svc).GetReport(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got model.BSheetReport
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
}

func TestBSheetHandler_GetReport_BadScenarioUUID(t *testing.T) {
	svc := &mockBSheetService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "not-a-uuid"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBSheetHandler(svc).GetReport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBSheetHandler_GetReport_ServiceError(t *testing.T) {
	svc := &mockBSheetService{
		getReportFn: func(_ context.Context, _, _ uuid.UUID) (*model.BSheetReport, error) {
			return nil, apierror.NotFound("scenario", "missing")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBSheetHandler(svc).GetReport(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestBSheetHandler_GetReport_InternalServerError(t *testing.T) {
	svc := &mockBSheetService{
		getReportFn: func(_ context.Context, _, _ uuid.UUID) (*model.BSheetReport, error) {
			return nil, apierror.Internal("database error")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBSheetHandler(svc).GetReport(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ── GetChartData ──────────────────────────────────────────────────────────

func TestBSheetHandler_GetChartData_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	expectedData := map[string]interface{}{
		"assets": map[string]float64{
			"current": 2000.0,
			"fixed":   3000.0,
		},
		"liabilities": map[string]float64{
			"current":   500.0,
			"long_term": 1500.0,
		},
	}

	svc := &mockBSheetService{
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

	newBSheetHandler(svc).GetChartData(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got["assets"])
	assert.NotNil(t, got["liabilities"])
}

func TestBSheetHandler_GetChartData_BadScenarioUUID(t *testing.T) {
	svc := &mockBSheetService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "invalid"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBSheetHandler(svc).GetChartData(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBSheetHandler_GetChartData_ServiceError(t *testing.T) {
	svc := &mockBSheetService{
		getChartDataFn: func(_ context.Context, _, _ uuid.UUID) (map[string]interface{}, error) {
			return nil, apierror.Internal("calculation error")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBSheetHandler(svc).GetChartData(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestBSheetHandler_GetChartData_NotFound(t *testing.T) {
	svc := &mockBSheetService{
		getChartDataFn: func(_ context.Context, _, _ uuid.UUID) (map[string]interface{}, error) {
			return nil, apierror.NotFound("balance sheet", "not found")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBSheetHandler(svc).GetChartData(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
