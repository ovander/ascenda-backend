package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── mockStaffService ──────────────────────────────────────────────────────────

type mockStaffService struct {
	listHeadcountsFn    func(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.StaffHeadcount, error)
	listSalariesFn      func(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.StaffSalary, error)
	listIncentivesFn    func(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.StaffIncentive, error)
	updateHeadcountsFn  func(ctx context.Context, tenantID, scenarioID uuid.UUID, headcounts []model.StaffHeadcount) error
	updateSalariesFn    func(ctx context.Context, tenantID, scenarioID uuid.UUID, salaries []model.StaffSalary) error
	updateIncentivesFn  func(ctx context.Context, tenantID, scenarioID uuid.UUID, incentives []model.StaffIncentive) error
	getPayrollSummaryFn func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.StaffPayrollSummary, error)
}

func (m *mockStaffService) ListHeadcounts(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.StaffHeadcount, error) {
	if m.listHeadcountsFn != nil {
		return m.listHeadcountsFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("list headcounts not implemented")
}

func (m *mockStaffService) ListSalaries(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.StaffSalary, error) {
	if m.listSalariesFn != nil {
		return m.listSalariesFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("list salaries not implemented")
}

func (m *mockStaffService) ListIncentives(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.StaffIncentive, error) {
	if m.listIncentivesFn != nil {
		return m.listIncentivesFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("list incentives not implemented")
}

func (m *mockStaffService) UpdateHeadcounts(ctx context.Context, tenantID, scenarioID uuid.UUID, headcounts []model.StaffHeadcount) error {
	if m.updateHeadcountsFn != nil {
		return m.updateHeadcountsFn(ctx, tenantID, scenarioID, headcounts)
	}
	return apierror.Internal("update headcounts not implemented")
}

func (m *mockStaffService) UpdateSalaries(ctx context.Context, tenantID, scenarioID uuid.UUID, salaries []model.StaffSalary) error {
	if m.updateSalariesFn != nil {
		return m.updateSalariesFn(ctx, tenantID, scenarioID, salaries)
	}
	return apierror.Internal("update salaries not implemented")
}

func (m *mockStaffService) UpdateIncentives(ctx context.Context, tenantID, scenarioID uuid.UUID, incentives []model.StaffIncentive) error {
	if m.updateIncentivesFn != nil {
		return m.updateIncentivesFn(ctx, tenantID, scenarioID, incentives)
	}
	return apierror.Internal("update incentives not implemented")
}

func (m *mockStaffService) GetPayrollSummary(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.StaffPayrollSummary, error) {
	if m.getPayrollSummaryFn != nil {
		return m.getPayrollSummaryFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("get payroll summary not implemented")
}

// ── fixtures ──────────────────────────────────────────────────────────────────

func newStaffHandler(svc *mockStaffService) *StaffHandler {
	return NewStaffHandler(svc, logrus.NewEntry(logrus.New()))
}

// ── ListHeadcounts ────────────────────────────────────────────────────────────

func TestStaffHandler_ListHeadcounts_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	headcounts := []model.StaffHeadcount{
		{
			Category:  model.CategoryRnDEngineers,
			YearIndex: 1,
			FTE:       decimal.NewFromInt(5),
		},
	}

	svc := &mockStaffService{
		listHeadcountsFn: func(_ context.Context, tid, sid uuid.UUID) ([]model.StaffHeadcount, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return headcounts, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newStaffHandler(svc).ListHeadcounts(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 1)
}

func TestStaffHandler_ListHeadcounts_BadScenarioUUID(t *testing.T) {
	svc := &mockStaffService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newStaffHandler(svc).ListHeadcounts(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStaffHandler_ListHeadcounts_ServiceError(t *testing.T) {
	svc := &mockStaffService{
		listHeadcountsFn: func(_ context.Context, _, _ uuid.UUID) ([]model.StaffHeadcount, error) {
			return nil, apierror.NotFound("scenario", "x")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newStaffHandler(svc).ListHeadcounts(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── ListSalaries ──────────────────────────────────────────────────────────────

func TestStaffHandler_ListSalaries_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	salaries := []model.StaffSalary{
		{
			Category:           model.CategoryAdminManagers,
			YearIndex:          1,
			MonthlyGrossSalary: decimal.NewFromInt(6667),
		},
	}

	svc := &mockStaffService{
		listSalariesFn: func(_ context.Context, tid, sid uuid.UUID) ([]model.StaffSalary, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return salaries, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newStaffHandler(svc).ListSalaries(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 1)
}

func TestStaffHandler_ListSalaries_BadScenarioUUID(t *testing.T) {
	svc := &mockStaffService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newStaffHandler(svc).ListSalaries(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStaffHandler_ListSalaries_ServiceError(t *testing.T) {
	svc := &mockStaffService{
		listSalariesFn: func(_ context.Context, _, _ uuid.UUID) ([]model.StaffSalary, error) {
			return nil, apierror.NotFound("scenario", "x")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newStaffHandler(svc).ListSalaries(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── ListIncentives ────────────────────────────────────────────────────────────

func TestStaffHandler_ListIncentives_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	incentives := []model.StaffIncentive{
		{
			YearIndex:          1,
			IncentivePct:       decimal.NewFromFloat(0.1),
			SpecificIncentives: decimal.NewFromInt(10000),
		},
	}

	svc := &mockStaffService{
		listIncentivesFn: func(_ context.Context, tid, sid uuid.UUID) ([]model.StaffIncentive, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return incentives, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newStaffHandler(svc).ListIncentives(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 1)
}

func TestStaffHandler_ListIncentives_BadScenarioUUID(t *testing.T) {
	svc := &mockStaffService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newStaffHandler(svc).ListIncentives(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStaffHandler_ListIncentives_ServiceError(t *testing.T) {
	svc := &mockStaffService{
		listIncentivesFn: func(_ context.Context, _, _ uuid.UUID) ([]model.StaffIncentive, error) {
			return nil, apierror.NotFound("scenario", "x")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newStaffHandler(svc).ListIncentives(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── UpdateHeadcounts ──────────────────────────────────────────────────────────

func TestStaffHandler_UpdateHeadcounts_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	headcounts := []model.StaffHeadcount{
		{
			Category:  model.CategoryRnDEngineers,
			YearIndex: 1,
			FTE:       decimal.NewFromInt(10),
		},
	}

	svc := &mockStaffService{
		updateHeadcountsFn: func(_ context.Context, tid, sid uuid.UUID, hh []model.StaffHeadcount) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			assert.Equal(t, 1, len(hh))
			assert.Equal(t, model.CategoryRnDEngineers, hh[0].Category)
			return nil
		},
	}

	body, _ := json.Marshal(headcounts)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newStaffHandler(svc).UpdateHeadcounts(w, r)

	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestStaffHandler_UpdateHeadcounts_BadScenarioUUID(t *testing.T) {
	svc := &mockStaffService{}
	body, _ := json.Marshal([]model.StaffHeadcount{})
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newStaffHandler(svc).UpdateHeadcounts(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStaffHandler_UpdateHeadcounts_InvalidBody(t *testing.T) {
	svc := &mockStaffService{}
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader([]byte("invalid json")))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newStaffHandler(svc).UpdateHeadcounts(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStaffHandler_UpdateHeadcounts_ServiceError(t *testing.T) {
	svc := &mockStaffService{
		updateHeadcountsFn: func(_ context.Context, _, _ uuid.UUID, _ []model.StaffHeadcount) error {
			return apierror.NotFound("scenario", "x")
		},
	}

	headcounts := []model.StaffHeadcount{}
	body, _ := json.Marshal(headcounts)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newStaffHandler(svc).UpdateHeadcounts(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── UpdateSalaries ────────────────────────────────────────────────────────────

func TestStaffHandler_UpdateSalaries_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	salaries := []model.StaffSalary{
		{
			Category:           model.CategoryAdminManagers,
			YearIndex:          1,
			MonthlyGrossSalary: decimal.NewFromInt(8333),
		},
	}

	svc := &mockStaffService{
		updateSalariesFn: func(_ context.Context, tid, sid uuid.UUID, ss []model.StaffSalary) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			assert.Equal(t, 1, len(ss))
			return nil
		},
	}

	body, _ := json.Marshal(salaries)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newStaffHandler(svc).UpdateSalaries(w, r)

	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestStaffHandler_UpdateSalaries_BadScenarioUUID(t *testing.T) {
	svc := &mockStaffService{}
	body, _ := json.Marshal([]model.StaffSalary{})
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newStaffHandler(svc).UpdateSalaries(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStaffHandler_UpdateSalaries_ServiceError(t *testing.T) {
	svc := &mockStaffService{
		updateSalariesFn: func(_ context.Context, _, _ uuid.UUID, _ []model.StaffSalary) error {
			return apierror.NotFound("scenario", "x")
		},
	}

	salaries := []model.StaffSalary{}
	body, _ := json.Marshal(salaries)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newStaffHandler(svc).UpdateSalaries(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── UpdateIncentives ──────────────────────────────────────────────────────────

func TestStaffHandler_UpdateIncentives_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	incentives := []model.StaffIncentive{
		{
			YearIndex:          1,
			IncentivePct:       decimal.NewFromFloat(0.05),
			SpecificIncentives: decimal.NewFromInt(15000),
		},
	}

	svc := &mockStaffService{
		updateIncentivesFn: func(_ context.Context, tid, sid uuid.UUID, ii []model.StaffIncentive) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			assert.Equal(t, 1, len(ii))
			return nil
		},
	}

	body, _ := json.Marshal(incentives)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newStaffHandler(svc).UpdateIncentives(w, r)

	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestStaffHandler_UpdateIncentives_BadScenarioUUID(t *testing.T) {
	svc := &mockStaffService{}
	body, _ := json.Marshal([]model.StaffIncentive{})
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newStaffHandler(svc).UpdateIncentives(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStaffHandler_UpdateIncentives_ServiceError(t *testing.T) {
	svc := &mockStaffService{
		updateIncentivesFn: func(_ context.Context, _, _ uuid.UUID, _ []model.StaffIncentive) error {
			return apierror.NotFound("scenario", "x")
		},
	}

	incentives := []model.StaffIncentive{}
	body, _ := json.Marshal(incentives)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newStaffHandler(svc).UpdateIncentives(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── GetPayrollSummary ─────────────────────────────────────────────────────────

func TestStaffHandler_GetPayrollSummary_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	summary := &model.StaffPayrollSummary{}

	svc := &mockStaffService{
		getPayrollSummaryFn: func(_ context.Context, tid, sid uuid.UUID) (*model.StaffPayrollSummary, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return summary, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newStaffHandler(svc).GetPayrollSummary(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

func TestStaffHandler_GetPayrollSummary_BadScenarioUUID(t *testing.T) {
	svc := &mockStaffService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newStaffHandler(svc).GetPayrollSummary(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStaffHandler_GetPayrollSummary_ServiceError(t *testing.T) {
	svc := &mockStaffService{
		getPayrollSummaryFn: func(_ context.Context, _, _ uuid.UUID) (*model.StaffPayrollSummary, error) {
			return nil, apierror.NotFound("scenario", "x")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newStaffHandler(svc).GetPayrollSummary(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
