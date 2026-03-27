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

// ── mockCashService ───────────────────────────────────────────────────────

type mockCashService struct {
	listOverridesFn   func(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CashMonthlyOverride, error)
	updateOverridesFn func(ctx context.Context, tenantID, scenarioID uuid.UUID, overrides []model.CashMonthlyOverride) error
	getReportFn       func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.CashReport, error)
}

func (m *mockCashService) ListOverrides(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CashMonthlyOverride, error) {
	if m.listOverridesFn != nil {
		return m.listOverridesFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("list overrides not implemented")
}

func (m *mockCashService) UpdateOverrides(ctx context.Context, tenantID, scenarioID uuid.UUID, overrides []model.CashMonthlyOverride) error {
	if m.updateOverridesFn != nil {
		return m.updateOverridesFn(ctx, tenantID, scenarioID, overrides)
	}
	return apierror.Internal("update overrides not implemented")
}

func (m *mockCashService) GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.CashReport, error) {
	if m.getReportFn != nil {
		return m.getReportFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("get report not implemented")
}

// ── Helper ────────────────────────────────────────────────────────────────

func newCashHandler(svc *mockCashService) *CashHandler {
	return NewCashHandler(svc, logrus.NewEntry(logrus.New()))
}

// ── ListOverrides ─────────────────────────────────────────────────────────

func TestCashHandler_ListOverrides_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	overrides := []model.CashMonthlyOverride{
		{Month: 1, Amount: decimal.NewFromInt(5000)},
		{Month: 2, Amount: decimal.NewFromInt(3000)},
	}

	svc := &mockCashService{
		listOverridesFn: func(_ context.Context, tid, sid uuid.UUID) ([]model.CashMonthlyOverride, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return overrides, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newCashHandler(svc).ListOverrides(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp["data"])
}

func TestCashHandler_ListOverrides_Empty(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	svc := &mockCashService{
		listOverridesFn: func(_ context.Context, tid, sid uuid.UUID) ([]model.CashMonthlyOverride, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return []model.CashMonthlyOverride{}, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newCashHandler(svc).ListOverrides(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(0), resp["total"])
}

func TestCashHandler_ListOverrides_BadScenarioUUID(t *testing.T) {
	svc := &mockCashService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "not-a-uuid"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newCashHandler(svc).ListOverrides(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCashHandler_ListOverrides_ServiceError(t *testing.T) {
	svc := &mockCashService{
		listOverridesFn: func(_ context.Context, _, _ uuid.UUID) ([]model.CashMonthlyOverride, error) {
			return nil, apierror.NotFound("scenario", "missing")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newCashHandler(svc).ListOverrides(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── UpdateOverrides ───────────────────────────────────────────────────────

func TestCashHandler_UpdateOverrides_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	var capturedOverrides []model.CashMonthlyOverride
	svc := &mockCashService{
		updateOverridesFn: func(_ context.Context, tid, sid uuid.UUID, overrides []model.CashMonthlyOverride) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			capturedOverrides = overrides
			return nil
		},
	}

	overrides := []model.CashMonthlyOverride{
		{Month: 1, Amount: decimal.NewFromInt(5000)},
		{Month: 2, Amount: decimal.NewFromInt(3000)},
	}
	body, _ := json.Marshal(overrides)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newCashHandler(svc).UpdateOverrides(w, r)

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Len(t, capturedOverrides, 2)
}

func TestCashHandler_UpdateOverrides_BadScenarioUUID(t *testing.T) {
	svc := &mockCashService{}
	overrides := []model.CashMonthlyOverride{{Month: 1, Amount: decimal.NewFromInt(5000)}}
	body, _ := json.Marshal(overrides)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newCashHandler(svc).UpdateOverrides(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCashHandler_UpdateOverrides_InvalidBody(t *testing.T) {
	svc := &mockCashService{}
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader([]byte("invalid json")))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newCashHandler(svc).UpdateOverrides(w, r)

	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusUnprocessableEntity)
}

func TestCashHandler_UpdateOverrides_ServiceError(t *testing.T) {
	svc := &mockCashService{
		updateOverridesFn: func(_ context.Context, _, _ uuid.UUID, _ []model.CashMonthlyOverride) error {
			return apierror.Internal("database error")
		},
	}

	overrides := []model.CashMonthlyOverride{{Month: 1, Amount: decimal.NewFromInt(5000)}}
	body, _ := json.Marshal(overrides)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newCashHandler(svc).UpdateOverrides(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ── GetReport ─────────────────────────────────────────────────────────────

func TestCashHandler_GetReport_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	expectedReport := &model.CashReport{}

	svc := &mockCashService{
		getReportFn: func(_ context.Context, tid, sid uuid.UUID) (*model.CashReport, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return expectedReport, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newCashHandler(svc).GetReport(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

func TestCashHandler_GetReport_BadScenarioUUID(t *testing.T) {
	svc := &mockCashService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newCashHandler(svc).GetReport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCashHandler_GetReport_ServiceError(t *testing.T) {
	svc := &mockCashService{
		getReportFn: func(_ context.Context, _, _ uuid.UUID) (*model.CashReport, error) {
			return nil, apierror.NotFound("scenario", "missing")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newCashHandler(svc).GetReport(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCashHandler_GetReport_InternalServerError(t *testing.T) {
	svc := &mockCashService{
		getReportFn: func(_ context.Context, _, _ uuid.UUID) (*model.CashReport, error) {
			return nil, apierror.Internal("calculation failed")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newCashHandler(svc).GetReport(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
