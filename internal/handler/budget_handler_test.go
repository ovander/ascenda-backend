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
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
)

// ── mockBudgetService ─────────────────────────────────────────────────────────

type mockBudgetService struct {
	listOverridesFn    func(ctx context.Context, tenantID, scenarioID uuid.UUID, year int) ([]model.BudgetMonthlyOverride, error)
	updateOverridesFn  func(ctx context.Context, tenantID, scenarioID uuid.UUID, year int, overrides []model.BudgetMonthlyOverride) error
	getBudget1ReportFn func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.Budget1Report, error)
	getBudget2ReportFn func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.Budget2Report, error)
}

func (m *mockBudgetService) ListOverrides(ctx context.Context, tenantID, scenarioID uuid.UUID, year int) ([]model.BudgetMonthlyOverride, error) {
	if m.listOverridesFn != nil {
		return m.listOverridesFn(ctx, tenantID, scenarioID, year)
	}
	return nil, apierror.Internal("list overrides not implemented")
}

func (m *mockBudgetService) UpdateOverrides(ctx context.Context, tenantID, scenarioID uuid.UUID, year int, overrides []model.BudgetMonthlyOverride) error {
	if m.updateOverridesFn != nil {
		return m.updateOverridesFn(ctx, tenantID, scenarioID, year, overrides)
	}
	return apierror.Internal("update overrides not implemented")
}

func (m *mockBudgetService) GetBudget1Report(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.Budget1Report, error) {
	if m.getBudget1ReportFn != nil {
		return m.getBudget1ReportFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("get budget 1 report not implemented")
}

func (m *mockBudgetService) GetBudget2Report(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.Budget2Report, error) {
	if m.getBudget2ReportFn != nil {
		return m.getBudget2ReportFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("get budget 2 report not implemented")
}

// ── fixtures ──────────────────────────────────────────────────────────────────

func newBudgetHandler(svc *mockBudgetService) *BudgetHandler {
	return NewBudgetHandler(svc, logrus.NewEntry(logrus.New()))
}

// ── ListOverrides ─────────────────────────────────────────────────────────────

func TestBudgetHandler_ListOverrides_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	overrides := []model.BudgetMonthlyOverride{
		{
			Month: 1,
			Amount: decimal.NewFromInt(5000),
		},
	}

	svc := &mockBudgetService{
		listOverridesFn: func(_ context.Context, tid, sid uuid.UUID, year int) ([]model.BudgetMonthlyOverride, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			assert.Equal(t, 1, year)
			return overrides, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/?year=1", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newBudgetHandler(svc).ListOverrides(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

func TestBudgetHandler_ListOverrides_BadScenarioUUID(t *testing.T) {
	svc := &mockBudgetService{}
	r := httptest.NewRequest(http.MethodGet, "/?year=1", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBudgetHandler(svc).ListOverrides(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBudgetHandler_ListOverrides_ServiceError(t *testing.T) {
	svc := &mockBudgetService{
		listOverridesFn: func(_ context.Context, _, _ uuid.UUID, _ int) ([]model.BudgetMonthlyOverride, error) {
			return nil, apierror.NotFound("scenario", "x")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/?year=1", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBudgetHandler(svc).ListOverrides(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── UpdateOverrides ───────────────────────────────────────────────────────────

func TestBudgetHandler_UpdateOverrides_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	overrides := []model.BudgetMonthlyOverride{
		{
			Month: 1,
			Amount: decimal.NewFromInt(7500),
		},
	}

	svc := &mockBudgetService{
		updateOverridesFn: func(_ context.Context, tid, sid uuid.UUID, year int, oo []model.BudgetMonthlyOverride) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			assert.Equal(t, 1, year)
			assert.Equal(t, 1, len(oo))
			return nil
		},
	}

	body, _ := json.Marshal(overrides)
	r := httptest.NewRequest(http.MethodPut, "/?year=1", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newBudgetHandler(svc).UpdateOverrides(w, r)

	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestBudgetHandler_UpdateOverrides_BadScenarioUUID(t *testing.T) {
	svc := &mockBudgetService{}
	body, _ := json.Marshal([]model.BudgetMonthlyOverride{})
	r := httptest.NewRequest(http.MethodPut, "/?year=1", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBudgetHandler(svc).UpdateOverrides(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBudgetHandler_UpdateOverrides_InvalidBody(t *testing.T) {
	svc := &mockBudgetService{}
	r := httptest.NewRequest(http.MethodPut, "/?year=1", bytes.NewReader([]byte("invalid json")))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBudgetHandler(svc).UpdateOverrides(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBudgetHandler_UpdateOverrides_ServiceError(t *testing.T) {
	svc := &mockBudgetService{
		updateOverridesFn: func(_ context.Context, _, _ uuid.UUID, _ int, _ []model.BudgetMonthlyOverride) error {
			return apierror.NotFound("scenario", "x")
		},
	}

	overrides := []model.BudgetMonthlyOverride{}
	body, _ := json.Marshal(overrides)
	r := httptest.NewRequest(http.MethodPut, "/?year=1", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBudgetHandler(svc).UpdateOverrides(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── GetBudget1Report ──────────────────────────────────────────────────────────

func TestBudgetHandler_GetBudget1Report_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	report := &model.Budget1Report{}

	svc := &mockBudgetService{
		getBudget1ReportFn: func(_ context.Context, tid, sid uuid.UUID) (*model.Budget1Report, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return report, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newBudgetHandler(svc).GetBudget1Report(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

func TestBudgetHandler_GetBudget1Report_BadScenarioUUID(t *testing.T) {
	svc := &mockBudgetService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBudgetHandler(svc).GetBudget1Report(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBudgetHandler_GetBudget1Report_ServiceError(t *testing.T) {
	svc := &mockBudgetService{
		getBudget1ReportFn: func(_ context.Context, _, _ uuid.UUID) (*model.Budget1Report, error) {
			return nil, apierror.NotFound("scenario", "x")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBudgetHandler(svc).GetBudget1Report(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── GetBudget2Report ──────────────────────────────────────────────────────────

func TestBudgetHandler_GetBudget2Report_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	report := &model.Budget2Report{}

	svc := &mockBudgetService{
		getBudget2ReportFn: func(_ context.Context, tid, sid uuid.UUID) (*model.Budget2Report, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return report, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newBudgetHandler(svc).GetBudget2Report(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

func TestBudgetHandler_GetBudget2Report_BadScenarioUUID(t *testing.T) {
	svc := &mockBudgetService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBudgetHandler(svc).GetBudget2Report(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBudgetHandler_GetBudget2Report_ServiceError(t *testing.T) {
	svc := &mockBudgetService{
		getBudget2ReportFn: func(_ context.Context, _, _ uuid.UUID) (*model.Budget2Report, error) {
			return nil, apierror.NotFound("scenario", "x")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBudgetHandler(svc).GetBudget2Report(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
