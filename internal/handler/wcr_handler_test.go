package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ascenda/internal/model"
	"ascenda/internal/pkg/apierror"
	"ascenda/internal/pkg/ctxutil"
)

// ── mockWCRService ────────────────────────────────────────────────────────────

type mockWCRService struct {
	listEntriesFn   func(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.WCREntry, error)
	updateEntriesFn func(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.WCREntry) error
	getReportFn     func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.WCRReport, error)
	getChartDataFn  func(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]interface{}, error)
}

func (m *mockWCRService) ListEntries(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.WCREntry, error) {
	if m.listEntriesFn != nil {
		return m.listEntriesFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("list entries not implemented")
}

func (m *mockWCRService) UpdateEntries(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.WCREntry) error {
	if m.updateEntriesFn != nil {
		return m.updateEntriesFn(ctx, tenantID, scenarioID, entries)
	}
	return apierror.Internal("update entries not implemented")
}

func (m *mockWCRService) GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.WCRReport, error) {
	if m.getReportFn != nil {
		return m.getReportFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("get report not implemented")
}

func (m *mockWCRService) GetChartData(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]interface{}, error) {
	if m.getChartDataFn != nil {
		return m.getChartDataFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("get chart data not implemented")
}

// ── fixtures ──────────────────────────────────────────────────────────────────

func newWCRHandler(svc *mockWCRService) *WCRHandler {
	return NewWCRHandler(svc, logrus.NewEntry(logrus.New()))
}

// ── ListEntries ───────────────────────────────────────────────────────────────

func TestWCRHandler_ListEntries_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	entries := []model.WCREntry{
		{
			LineID:    model.WCRLineID("receivables_override"),
			YearIndex: 1,
			Amount:    decimal.NewFromInt(25000),
		},
	}

	svc := &mockWCRService{
		listEntriesFn: func(_ context.Context, tid, sid uuid.UUID) ([]model.WCREntry, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return entries, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newWCRHandler(svc).ListEntries(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 1)
}

func TestWCRHandler_ListEntries_BadScenarioUUID(t *testing.T) {
	svc := &mockWCRService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newWCRHandler(svc).ListEntries(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWCRHandler_ListEntries_ServiceError(t *testing.T) {
	svc := &mockWCRService{
		listEntriesFn: func(_ context.Context, _, _ uuid.UUID) ([]model.WCREntry, error) {
			return nil, apierror.NotFound("scenario", "x")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newWCRHandler(svc).ListEntries(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── UpdateEntries ─────────────────────────────────────────────────────────────

func TestWCRHandler_UpdateEntries_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	entries := []model.WCREntry{
		{
			LineID:    model.WCRLineID("payables_override"),
			YearIndex: 1,
			Amount:    decimal.NewFromInt(30000),
		},
	}

	svc := &mockWCRService{
		updateEntriesFn: func(_ context.Context, tid, sid uuid.UUID, ee []model.WCREntry) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			assert.Equal(t, 1, len(ee))
			assert.Equal(t, model.WCRLineID("payables_override"), ee[0].LineID)
			return nil
		},
	}

	body, _ := json.Marshal(entries)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newWCRHandler(svc).UpdateEntries(w, r)

	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestWCRHandler_UpdateEntries_BadScenarioUUID(t *testing.T) {
	svc := &mockWCRService{}
	body, _ := json.Marshal([]model.WCREntry{})
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newWCRHandler(svc).UpdateEntries(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWCRHandler_UpdateEntries_InvalidBody(t *testing.T) {
	svc := &mockWCRService{}
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader([]byte("invalid json")))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newWCRHandler(svc).UpdateEntries(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWCRHandler_UpdateEntries_ServiceError(t *testing.T) {
	svc := &mockWCRService{
		updateEntriesFn: func(_ context.Context, _, _ uuid.UUID, _ []model.WCREntry) error {
			return apierror.NotFound("scenario", "x")
		},
	}

	entries := []model.WCREntry{}
	body, _ := json.Marshal(entries)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newWCRHandler(svc).UpdateEntries(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── GetReport ─────────────────────────────────────────────────────────────────

func TestWCRHandler_GetReport_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	report := &model.WCRReport{}

	svc := &mockWCRService{
		getReportFn: func(_ context.Context, tid, sid uuid.UUID) (*model.WCRReport, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return report, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newWCRHandler(svc).GetReport(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

func TestWCRHandler_GetReport_BadScenarioUUID(t *testing.T) {
	svc := &mockWCRService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newWCRHandler(svc).GetReport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWCRHandler_GetReport_ServiceError(t *testing.T) {
	svc := &mockWCRService{
		getReportFn: func(_ context.Context, _, _ uuid.UUID) (*model.WCRReport, error) {
			return nil, apierror.NotFound("scenario", "x")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newWCRHandler(svc).GetReport(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── GetChartData ──────────────────────────────────────────────────────────────

func TestWCRHandler_GetChartData_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	chartData := map[string]interface{}{"data": []interface{}{}}

	svc := &mockWCRService{
		getChartDataFn: func(_ context.Context, tid, sid uuid.UUID) (map[string]interface{}, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return chartData, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newWCRHandler(svc).GetChartData(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

func TestWCRHandler_GetChartData_BadScenarioUUID(t *testing.T) {
	svc := &mockWCRService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newWCRHandler(svc).GetChartData(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWCRHandler_GetChartData_ServiceError(t *testing.T) {
	svc := &mockWCRService{
		getChartDataFn: func(_ context.Context, _, _ uuid.UUID) (map[string]interface{}, error) {
			return nil, apierror.NotFound("scenario", "x")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newWCRHandler(svc).GetChartData(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
