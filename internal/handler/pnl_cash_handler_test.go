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

// ── mockPnlCashService ────────────────────────────────────────────────────

type mockPnlCashService struct {
	listEntriesFn   func(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.PnlCashEntry, error)
	updateEntriesFn func(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.PnlCashEntry) error
	getReportFn     func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.PnlCashReport, error)
	getChartDataFn  func(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]interface{}, error)
}

func (m *mockPnlCashService) ListEntries(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.PnlCashEntry, error) {
	if m.listEntriesFn != nil {
		return m.listEntriesFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("list entries not implemented")
}

func (m *mockPnlCashService) UpdateEntries(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.PnlCashEntry) error {
	if m.updateEntriesFn != nil {
		return m.updateEntriesFn(ctx, tenantID, scenarioID, entries)
	}
	return apierror.Internal("update entries not implemented")
}

func (m *mockPnlCashService) GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.PnlCashReport, error) {
	if m.getReportFn != nil {
		return m.getReportFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("get report not implemented")
}

func (m *mockPnlCashService) GetChartData(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]interface{}, error) {
	if m.getChartDataFn != nil {
		return m.getChartDataFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("get chart data not implemented")
}

// ── Helper ────────────────────────────────────────────────────────────────

func newPnlCashHandler(svc *mockPnlCashService) *PnlCashHandler {
	return NewPnlCashHandler(svc, logrus.NewEntry(logrus.New()))
}

// ── ListEntries ───────────────────────────────────────────────────────────

func TestPnlCashHandler_ListEntries_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	entries := []model.PnlCashEntry{
		{LineID: model.PnlCashLineID("rd_expenses"), YearIndex: 1, Amount: decimal.NewFromInt(1000)},
		{LineID: model.PnlCashLineID("admin_expenses"), YearIndex: 1, Amount: decimal.NewFromInt(500)},
	}

	svc := &mockPnlCashService{
		listEntriesFn: func(_ context.Context, tid, sid uuid.UUID) ([]model.PnlCashEntry, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return entries, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newPnlCashHandler(svc).ListEntries(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []dto.PnlCashEntryResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 2)
}

func TestPnlCashHandler_ListEntries_Empty(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	svc := &mockPnlCashService{
		listEntriesFn: func(_ context.Context, tid, sid uuid.UUID) ([]model.PnlCashEntry, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return []model.PnlCashEntry{}, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newPnlCashHandler(svc).ListEntries(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []dto.PnlCashEntryResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Empty(t, got)
}

func TestPnlCashHandler_ListEntries_BadScenarioUUID(t *testing.T) {
	svc := &mockPnlCashService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "not-a-uuid"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPnlCashHandler(svc).ListEntries(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPnlCashHandler_ListEntries_ServiceError(t *testing.T) {
	svc := &mockPnlCashService{
		listEntriesFn: func(_ context.Context, _, _ uuid.UUID) ([]model.PnlCashEntry, error) {
			return nil, apierror.NotFound("scenario", "not found")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPnlCashHandler(svc).ListEntries(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── UpdateEntries ─────────────────────────────────────────────────────────

func TestPnlCashHandler_UpdateEntries_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	var capturedEntries []model.PnlCashEntry
	svc := &mockPnlCashService{
		updateEntriesFn: func(_ context.Context, tid, sid uuid.UUID, entries []model.PnlCashEntry) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			capturedEntries = entries
			return nil
		},
	}

	entries := []model.PnlCashEntry{
		{LineID: model.PnlCashLineID("rd_expenses"), YearIndex: 1, Amount: decimal.NewFromInt(1000)},
		{LineID: model.PnlCashLineID("admin_expenses"), YearIndex: 1, Amount: decimal.NewFromInt(500)},
	}
	body, _ := json.Marshal(entries)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newPnlCashHandler(svc).UpdateEntries(w, r)

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Len(t, capturedEntries, 2)
}

func TestPnlCashHandler_UpdateEntries_BadScenarioUUID(t *testing.T) {
	svc := &mockPnlCashService{}
	entries := []model.PnlCashEntry{{LineID: model.PnlCashLineID("rd_expenses"), YearIndex: 1, Amount: decimal.NewFromInt(1000)}}
	body, _ := json.Marshal(entries)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPnlCashHandler(svc).UpdateEntries(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPnlCashHandler_UpdateEntries_InvalidBody(t *testing.T) {
	svc := &mockPnlCashService{}
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader([]byte("invalid json")))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPnlCashHandler(svc).UpdateEntries(w, r)

	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusUnprocessableEntity)
}

func TestPnlCashHandler_UpdateEntries_ServiceError(t *testing.T) {
	svc := &mockPnlCashService{
		updateEntriesFn: func(_ context.Context, _, _ uuid.UUID, _ []model.PnlCashEntry) error {
			return apierror.Internal("database error")
		},
	}

	entries := []model.PnlCashEntry{{LineID: model.PnlCashLineID("rd_expenses"), YearIndex: 1, Amount: decimal.NewFromInt(1000)}}
	body, _ := json.Marshal(entries)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPnlCashHandler(svc).UpdateEntries(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ── GetReport ─────────────────────────────────────────────────────────────

func TestPnlCashHandler_GetReport_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	expectedReport := &model.PnlCashReport{}

	svc := &mockPnlCashService{
		getReportFn: func(_ context.Context, tid, sid uuid.UUID) (*model.PnlCashReport, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return expectedReport, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newPnlCashHandler(svc).GetReport(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

func TestPnlCashHandler_GetReport_BadScenarioUUID(t *testing.T) {
	svc := &mockPnlCashService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPnlCashHandler(svc).GetReport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPnlCashHandler_GetReport_ServiceError(t *testing.T) {
	svc := &mockPnlCashService{
		getReportFn: func(_ context.Context, _, _ uuid.UUID) (*model.PnlCashReport, error) {
			return nil, apierror.NotFound("scenario", "missing")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPnlCashHandler(svc).GetReport(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── GetChartData ──────────────────────────────────────────────────────────

func TestPnlCashHandler_GetChartData_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	expectedData := map[string]interface{}{
		"months": []string{"Jan", "Feb", "Mar"},
		"cash":   []float64{100, 200, 300},
	}

	svc := &mockPnlCashService{
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

	newPnlCashHandler(svc).GetChartData(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

func TestPnlCashHandler_GetChartData_BadScenarioUUID(t *testing.T) {
	svc := &mockPnlCashService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "invalid"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPnlCashHandler(svc).GetChartData(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPnlCashHandler_GetChartData_ServiceError(t *testing.T) {
	svc := &mockPnlCashService{
		getChartDataFn: func(_ context.Context, _, _ uuid.UUID) (map[string]interface{}, error) {
			return nil, apierror.Internal("calculation error")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPnlCashHandler(svc).GetChartData(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
