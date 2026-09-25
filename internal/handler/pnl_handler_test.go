package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ascenda/internal/dto"
	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── mockPnLService ────────────────────────────────────────────────────────

type mockPnLService struct {
	listManualEntriesFn   func(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.PnlManualEntry, error)
	updateManualEntriesFn func(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.PnlManualEntry) error
	getReportFn           func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.PnlReport, error)
	getChartDataFn        func(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]interface{}, error)
}

func (m *mockPnLService) ListManualEntries(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.PnlManualEntry, error) {
	if m.listManualEntriesFn != nil {
		return m.listManualEntriesFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("list manual entries not implemented")
}

func (m *mockPnLService) UpdateManualEntries(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.PnlManualEntry) error {
	if m.updateManualEntriesFn != nil {
		return m.updateManualEntriesFn(ctx, tenantID, scenarioID, entries)
	}
	return apierror.Internal("update manual entries not implemented")
}

func (m *mockPnLService) GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.PnlReport, error) {
	if m.getReportFn != nil {
		return m.getReportFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("get report not implemented")
}

func (m *mockPnLService) GetChartData(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]interface{}, error) {
	if m.getChartDataFn != nil {
		return m.getChartDataFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("get chart data not implemented")
}

// ── Helper ────────────────────────────────────────────────────────────────

func newPnLHandler(svc *mockPnLService) *PnLHandler {
	return NewPnLHandler(svc, logrus.NewEntry(logrus.New()))
}

// ── ListManualEntries ─────────────────────────────────────────────────────

func TestPnLHandler_ListManualEntries_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	entries := []model.PnlManualEntry{
		{LineID: model.PnlLineID("other_revenue"), YearIndex: 1, Amount: decimal.NewFromInt(100)},
		{LineID: model.PnlLineID("other_charges"), YearIndex: 1, Amount: decimal.NewFromInt(50)},
	}

	svc := &mockPnLService{
		listManualEntriesFn: func(_ context.Context, tid, sid uuid.UUID) ([]model.PnlManualEntry, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return entries, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newPnLHandler(svc).ListManualEntries(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []dto.PnlEntryResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 2)
}

func TestPnLHandler_ListManualEntries_BadScenarioUUID(t *testing.T) {
	svc := &mockPnLService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "not-a-uuid"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPnLHandler(svc).ListManualEntries(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPnLHandler_ListManualEntries_ServiceError(t *testing.T) {
	svc := &mockPnLService{
		listManualEntriesFn: func(_ context.Context, _, _ uuid.UUID) ([]model.PnlManualEntry, error) {
			return nil, apierror.NotFound("scenario", "missing")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPnLHandler(svc).ListManualEntries(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── UpdateManualEntries ───────────────────────────────────────────────────

func TestPnLHandler_UpdateManualEntries_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	var capturedEntries []model.PnlManualEntry
	svc := &mockPnLService{
		updateManualEntriesFn: func(_ context.Context, tid, sid uuid.UUID, entries []model.PnlManualEntry) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			capturedEntries = entries
			return nil
		},
	}

	entries := []model.PnlManualEntry{
		{LineID: model.PnlLineID("other_revenue"), YearIndex: 1, Amount: decimal.NewFromInt(100)},
		{LineID: model.PnlLineID("other_charges"), YearIndex: 1, Amount: decimal.NewFromInt(50)},
	}
	body, _ := json.Marshal(entries)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newPnLHandler(svc).UpdateManualEntries(w, r)

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Len(t, capturedEntries, 2)
}

func TestPnLHandler_UpdateManualEntries_BadScenarioUUID(t *testing.T) {
	svc := &mockPnLService{}
	entries := []model.PnlManualEntry{{LineID: model.PnlLineID("other_revenue"), YearIndex: 1, Amount: decimal.NewFromInt(100)}}
	body, _ := json.Marshal(entries)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPnLHandler(svc).UpdateManualEntries(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPnLHandler_UpdateManualEntries_InvalidBody(t *testing.T) {
	svc := &mockPnLService{}
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader([]byte("invalid json")))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPnLHandler(svc).UpdateManualEntries(w, r)

	// Invalid body should result in 400 or 422
	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusUnprocessableEntity)
}

func TestPnLHandler_UpdateManualEntries_ServiceError(t *testing.T) {
	svc := &mockPnLService{
		updateManualEntriesFn: func(_ context.Context, _, _ uuid.UUID, _ []model.PnlManualEntry) error {
			return apierror.Internal("database error")
		},
	}

	entries := []model.PnlManualEntry{{LineID: model.PnlLineID("other_revenue"), YearIndex: 1, Amount: decimal.NewFromInt(100)}}
	body, _ := json.Marshal(entries)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPnLHandler(svc).UpdateManualEntries(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ── GetReport ─────────────────────────────────────────────────────────────

func TestPnLHandler_GetReport_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	expectedReport := &model.PnlReport{}

	svc := &mockPnLService{
		getReportFn: func(_ context.Context, tid, sid uuid.UUID) (*model.PnlReport, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return expectedReport, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newPnLHandler(svc).GetReport(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

func TestPnLHandler_GetReport_BadScenarioUUID(t *testing.T) {
	svc := &mockPnLService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPnLHandler(svc).GetReport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPnLHandler_GetReport_ServiceError(t *testing.T) {
	svc := &mockPnLService{
		getReportFn: func(_ context.Context, _, _ uuid.UUID) (*model.PnlReport, error) {
			return nil, apierror.NotFound("scenario", "missing")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPnLHandler(svc).GetReport(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── GetChartData ──────────────────────────────────────────────────────────

func TestPnLHandler_GetChartData_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	expectedData := map[string]interface{}{
		"labels": []string{"Q1", "Q2", "Q3", "Q4"},
		"values": []float64{100, 200, 150, 300},
	}

	svc := &mockPnLService{
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

	newPnLHandler(svc).GetChartData(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

func TestPnLHandler_GetChartData_BadScenarioUUID(t *testing.T) {
	svc := &mockPnLService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "invalid"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPnLHandler(svc).GetChartData(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPnLHandler_GetChartData_ServiceError(t *testing.T) {
	svc := &mockPnLService{
		getChartDataFn: func(_ context.Context, _, _ uuid.UUID) (map[string]interface{}, error) {
			return nil, apierror.Internal("calculation error")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPnLHandler(svc).GetChartData(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
