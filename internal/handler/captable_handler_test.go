package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ascenda/internal/model"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// Mock CapTableServicer
// ─────────────────────────────────────────────────────────────────────────────

type mockCapTableService struct {
	GetCompanyFunc                    func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.CapTableCompany, error)
	UpsertCompanyFunc                 func(ctx context.Context, company *model.CapTableCompany) error
	GetCountryProfileFunc             func(code string) model.CapTableCountryProfile
	ListShareClassesFunc              func(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CapTableShareClass, error)
	UpsertShareClassFunc              func(ctx context.Context, sc *model.CapTableShareClass) error
	DeleteShareClassFunc              func(ctx context.Context, tenantID, id uuid.UUID) error
	ListShareholdersFunc              func(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CapTableShareholder, error)
	CreateShareholderFunc             func(ctx context.Context, sh *model.CapTableShareholder) error
	UpdateShareholderFunc             func(ctx context.Context, sh *model.CapTableShareholder) error
	DeleteShareholderFunc             func(ctx context.Context, tenantID, id uuid.UUID) error
	ListRoundsFunc                    func(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CapTableRound, error)
	CreateRoundFunc                   func(ctx context.Context, rnd *model.CapTableRound) error
	UpdateRoundFunc                   func(ctx context.Context, rnd *model.CapTableRound) error
	DeleteRoundFunc                   func(ctx context.Context, tenantID, id uuid.UUID) error
	ListPlansFunc                     func(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.StockOptionPlan, error)
	CreatePlanFunc                    func(ctx context.Context, plan *model.StockOptionPlan) error
	UpdatePlanFunc                    func(ctx context.Context, plan *model.StockOptionPlan) error
	DeletePlanFunc                    func(ctx context.Context, tenantID, id uuid.UUID) error
	ListGrantsFunc                    func(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.OptionGrant, error)
	CreateGrantFunc                   func(ctx context.Context, grant *model.OptionGrant) error
	UpdateGrantFunc                   func(ctx context.Context, grant *model.OptionGrant) error
	DeleteGrantFunc                   func(ctx context.Context, tenantID, id uuid.UUID) error
	ListValuationScenariosFunc        func(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.ValuationScenario, error)
	CreateValuationScenarioFunc       func(ctx context.Context, vs *model.ValuationScenario) error
	UpdateValuationScenarioFunc       func(ctx context.Context, vs *model.ValuationScenario) error
	DeleteValuationScenarioFunc       func(ctx context.Context, tenantID, id uuid.UUID) error
	ComputeValuationScenarioFunc      func(ctx context.Context, tenantID, id uuid.UUID) (*model.ValuationScenarioResult, error)
	ListBranchesFunc                  func(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CapTableScenarioBranch, error)
	CreateBranchFunc                  func(ctx context.Context, branch *model.CapTableScenarioBranch) error
	UpdateBranchFunc                  func(ctx context.Context, branch *model.CapTableScenarioBranch) error
	GetReportFunc                     func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.CapTableReport, error)
	SyncRoundToFiplanFunc             func(ctx context.Context, tenantID uuid.UUID, roundID uuid.UUID, fiscalYearIndex int) (*model.CapTableRound, error)
	UnlinkFromFiplanFunc              func(ctx context.Context, tenantID uuid.UUID, roundID uuid.UUID) (*model.CapTableRound, error)
	SyncRoundToOpeningBalanceFunc     func(ctx context.Context, tenantID, roundID uuid.UUID) (*model.CapTableRound, error)
	UnsyncRoundFromOpeningBalanceFunc func(ctx context.Context, tenantID, roundID uuid.UUID) (*model.CapTableRound, error)
}

func (m *mockCapTableService) GetCompany(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.CapTableCompany, error) {
	if m.GetCompanyFunc != nil {
		return m.GetCompanyFunc(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("not implemented")
}
func (m *mockCapTableService) UpsertCompany(ctx context.Context, company *model.CapTableCompany) error {
	if m.UpsertCompanyFunc != nil {
		return m.UpsertCompanyFunc(ctx, company)
	}
	return nil
}
func (m *mockCapTableService) GetCountryProfile(code string) model.CapTableCountryProfile {
	if m.GetCountryProfileFunc != nil {
		return m.GetCountryProfileFunc(code)
	}
	return model.CapTableCountryProfile{}
}
func (m *mockCapTableService) ListShareClasses(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CapTableShareClass, error) {
	if m.ListShareClassesFunc != nil {
		return m.ListShareClassesFunc(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("not implemented")
}
func (m *mockCapTableService) UpsertShareClass(ctx context.Context, sc *model.CapTableShareClass) error {
	if m.UpsertShareClassFunc != nil {
		return m.UpsertShareClassFunc(ctx, sc)
	}
	return nil
}
func (m *mockCapTableService) DeleteShareClass(ctx context.Context, tenantID, id uuid.UUID) error {
	if m.DeleteShareClassFunc != nil {
		return m.DeleteShareClassFunc(ctx, tenantID, id)
	}
	return nil
}
func (m *mockCapTableService) ListShareholders(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CapTableShareholder, error) {
	if m.ListShareholdersFunc != nil {
		return m.ListShareholdersFunc(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("not implemented")
}
func (m *mockCapTableService) CreateShareholder(ctx context.Context, sh *model.CapTableShareholder) error {
	if m.CreateShareholderFunc != nil {
		return m.CreateShareholderFunc(ctx, sh)
	}
	return nil
}
func (m *mockCapTableService) UpdateShareholder(ctx context.Context, sh *model.CapTableShareholder) error {
	if m.UpdateShareholderFunc != nil {
		return m.UpdateShareholderFunc(ctx, sh)
	}
	return nil
}
func (m *mockCapTableService) DeleteShareholder(ctx context.Context, tenantID, id uuid.UUID) error {
	if m.DeleteShareholderFunc != nil {
		return m.DeleteShareholderFunc(ctx, tenantID, id)
	}
	return nil
}
func (m *mockCapTableService) ListRounds(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CapTableRound, error) {
	if m.ListRoundsFunc != nil {
		return m.ListRoundsFunc(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("not implemented")
}
func (m *mockCapTableService) CreateRound(ctx context.Context, rnd *model.CapTableRound) error {
	if m.CreateRoundFunc != nil {
		return m.CreateRoundFunc(ctx, rnd)
	}
	return nil
}
func (m *mockCapTableService) UpdateRound(ctx context.Context, rnd *model.CapTableRound) error {
	if m.UpdateRoundFunc != nil {
		return m.UpdateRoundFunc(ctx, rnd)
	}
	return nil
}
func (m *mockCapTableService) DeleteRound(ctx context.Context, tenantID, id uuid.UUID) error {
	if m.DeleteRoundFunc != nil {
		return m.DeleteRoundFunc(ctx, tenantID, id)
	}
	return nil
}
func (m *mockCapTableService) ListPlans(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.StockOptionPlan, error) {
	if m.ListPlansFunc != nil {
		return m.ListPlansFunc(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("not implemented")
}
func (m *mockCapTableService) CreatePlan(ctx context.Context, plan *model.StockOptionPlan) error {
	if m.CreatePlanFunc != nil {
		return m.CreatePlanFunc(ctx, plan)
	}
	return nil
}
func (m *mockCapTableService) UpdatePlan(ctx context.Context, plan *model.StockOptionPlan) error {
	if m.UpdatePlanFunc != nil {
		return m.UpdatePlanFunc(ctx, plan)
	}
	return nil
}
func (m *mockCapTableService) DeletePlan(ctx context.Context, tenantID, id uuid.UUID) error {
	if m.DeletePlanFunc != nil {
		return m.DeletePlanFunc(ctx, tenantID, id)
	}
	return nil
}
func (m *mockCapTableService) ListGrants(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.OptionGrant, error) {
	if m.ListGrantsFunc != nil {
		return m.ListGrantsFunc(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("not implemented")
}
func (m *mockCapTableService) CreateGrant(ctx context.Context, grant *model.OptionGrant) error {
	if m.CreateGrantFunc != nil {
		return m.CreateGrantFunc(ctx, grant)
	}
	return nil
}
func (m *mockCapTableService) UpdateGrant(ctx context.Context, grant *model.OptionGrant) error {
	if m.UpdateGrantFunc != nil {
		return m.UpdateGrantFunc(ctx, grant)
	}
	return nil
}
func (m *mockCapTableService) DeleteGrant(ctx context.Context, tenantID, id uuid.UUID) error {
	if m.DeleteGrantFunc != nil {
		return m.DeleteGrantFunc(ctx, tenantID, id)
	}
	return nil
}
func (m *mockCapTableService) ListValuationScenarios(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.ValuationScenario, error) {
	if m.ListValuationScenariosFunc != nil {
		return m.ListValuationScenariosFunc(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("not implemented")
}
func (m *mockCapTableService) CreateValuationScenario(ctx context.Context, vs *model.ValuationScenario) error {
	if m.CreateValuationScenarioFunc != nil {
		return m.CreateValuationScenarioFunc(ctx, vs)
	}
	return nil
}
func (m *mockCapTableService) UpdateValuationScenario(ctx context.Context, vs *model.ValuationScenario) error {
	if m.UpdateValuationScenarioFunc != nil {
		return m.UpdateValuationScenarioFunc(ctx, vs)
	}
	return nil
}
func (m *mockCapTableService) DeleteValuationScenario(ctx context.Context, tenantID, id uuid.UUID) error {
	if m.DeleteValuationScenarioFunc != nil {
		return m.DeleteValuationScenarioFunc(ctx, tenantID, id)
	}
	return nil
}
func (m *mockCapTableService) ComputeValuationScenario(ctx context.Context, tenantID, id uuid.UUID) (*model.ValuationScenarioResult, error) {
	if m.ComputeValuationScenarioFunc != nil {
		return m.ComputeValuationScenarioFunc(ctx, tenantID, id)
	}
	return nil, apierror.Internal("not implemented")
}
func (m *mockCapTableService) ListBranches(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CapTableScenarioBranch, error) {
	if m.ListBranchesFunc != nil {
		return m.ListBranchesFunc(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("not implemented")
}
func (m *mockCapTableService) CreateBranch(ctx context.Context, branch *model.CapTableScenarioBranch) error {
	if m.CreateBranchFunc != nil {
		return m.CreateBranchFunc(ctx, branch)
	}
	return nil
}
func (m *mockCapTableService) UpdateBranch(ctx context.Context, branch *model.CapTableScenarioBranch) error {
	if m.UpdateBranchFunc != nil {
		return m.UpdateBranchFunc(ctx, branch)
	}
	return nil
}
func (m *mockCapTableService) GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.CapTableReport, error) {
	if m.GetReportFunc != nil {
		return m.GetReportFunc(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("not implemented")
}
func (m *mockCapTableService) SyncRoundToFiplan(ctx context.Context, tenantID uuid.UUID, roundID uuid.UUID, fiscalYearIndex int) (*model.CapTableRound, error) {
	if m.SyncRoundToFiplanFunc != nil {
		return m.SyncRoundToFiplanFunc(ctx, tenantID, roundID, fiscalYearIndex)
	}
	return nil, apierror.Internal("not implemented")
}
func (m *mockCapTableService) UnlinkFromFiplan(ctx context.Context, tenantID uuid.UUID, roundID uuid.UUID) (*model.CapTableRound, error) {
	if m.UnlinkFromFiplanFunc != nil {
		return m.UnlinkFromFiplanFunc(ctx, tenantID, roundID)
	}
	return nil, apierror.Internal("not implemented")
}
func (m *mockCapTableService) SyncRoundToOpeningBalance(ctx context.Context, tenantID, roundID uuid.UUID) (*model.CapTableRound, error) {
	if m.SyncRoundToOpeningBalanceFunc != nil {
		return m.SyncRoundToOpeningBalanceFunc(ctx, tenantID, roundID)
	}
	return nil, apierror.Internal("not implemented")
}
func (m *mockCapTableService) UnsyncRoundFromOpeningBalance(ctx context.Context, tenantID, roundID uuid.UUID) (*model.CapTableRound, error) {
	if m.UnsyncRoundFromOpeningBalanceFunc != nil {
		return m.UnsyncRoundFromOpeningBalanceFunc(ctx, tenantID, roundID)
	}
	return nil, apierror.Internal("not implemented")
}

// ─────────────────────────────────────────────────────────────────────────────
// Test helpers
// ─────────────────────────────────────────────────────────────────────────────

func newCapTableHandler(svc CapTableServicer) *CapTableHandler {
	return NewCapTableHandler(svc, logrus.NewEntry(logrus.New()))
}

// capTableRequest builds an httptest request with chi route context for {scenarioId}.
func capTableRequest(method, path string, body interface{}, scenarioID, tenantID uuid.UUID) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body) //nolint:errcheck
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	ctx := ctxutil.WithTenantID(req.Context(), tenantID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("scenarioId", scenarioID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

	return req.WithContext(ctx)
}

// capTableRequestWithID builds a request that also sets a chi {id} param.
func capTableRequestWithID(method, path string, body interface{}, scenarioID, tenantID, id uuid.UUID) *http.Request {
	req := capTableRequest(method, path, body, scenarioID, tenantID)
	rctx := chi.RouteContext(req.Context())
	rctx.URLParams.Add("id", id.String())
	return req
}

// ─────────────────────────────────────────────────────────────────────────────
// GetCapTableCompany
// ─────────────────────────────────────────────────────────────────────────────

func TestGetCapTableCompany_OK(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	svc := &mockCapTableService{
		GetCompanyFunc: func(_ context.Context, tid, sid uuid.UUID) (*model.CapTableCompany, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return &model.CapTableCompany{
				CompanyName: "ACME SAS",
				Currency:    "EUR",
			}, nil
		},
	}

	h := newCapTableHandler(svc)
	req := capTableRequest(http.MethodGet, "/captable/company", nil, scenarioID, tenantID)
	w := httptest.NewRecorder()
	h.GetCapTableCompany(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result model.CapTableCompany
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "ACME SAS", result.CompanyName)
}

func TestGetCapTableCompany_InvalidScenarioID(t *testing.T) {
	h := newCapTableHandler(&mockCapTableService{})

	req := httptest.NewRequest(http.MethodGet, "/captable/company", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("scenarioId", "not-a-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.GetCapTableCompany(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetCapTableCompany_ServiceError(t *testing.T) {
	svc := &mockCapTableService{
		GetCompanyFunc: func(_ context.Context, _, _ uuid.UUID) (*model.CapTableCompany, error) {
			return nil, apierror.Internal("database error")
		},
	}
	h := newCapTableHandler(svc)
	req := capTableRequest(http.MethodGet, "/captable/company", nil, uuid.New(), uuid.New())
	w := httptest.NewRecorder()
	h.GetCapTableCompany(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ─────────────────────────────────────────────────────────────────────────────
// UpsertCapTableCompany
// ─────────────────────────────────────────────────────────────────────────────

func TestUpsertCapTableCompany_OK(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	var captured model.CapTableCompany
	svc := &mockCapTableService{
		UpsertCompanyFunc: func(_ context.Context, c *model.CapTableCompany) error {
			captured = *c
			return nil
		},
	}

	payload := map[string]interface{}{
		"companyName": "TestCo",
		"currency":    "EUR",
	}
	h := newCapTableHandler(svc)
	req := capTableRequest(http.MethodPut, "/captable/company", payload, scenarioID, tenantID)
	w := httptest.NewRecorder()
	h.UpsertCapTableCompany(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, tenantID, captured.TenantID)
	assert.Equal(t, scenarioID, captured.ScenarioID)
	assert.Equal(t, "TestCo", captured.CompanyName)
}

// ─────────────────────────────────────────────────────────────────────────────
// GetCapTableCountryProfile
// ─────────────────────────────────────────────────────────────────────────────

func TestGetCapTableCountryProfile_WithCode(t *testing.T) {
	svc := &mockCapTableService{
		GetCountryProfileFunc: func(code string) model.CapTableCountryProfile {
			return model.CapTableCountryProfile{CountryCode: code, Currency: "EUR"}
		},
	}

	h := newCapTableHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/captable/country-profile?code=FR", nil)
	w := httptest.NewRecorder()
	h.GetCapTableCountryProfile(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var profile model.CapTableCountryProfile
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &profile))
	assert.Equal(t, "FR", profile.CountryCode)
}

func TestGetCapTableCountryProfile_DefaultCodeBE(t *testing.T) {
	called := ""
	svc := &mockCapTableService{
		GetCountryProfileFunc: func(code string) model.CapTableCountryProfile {
			called = code
			return model.CapTableCountryProfile{CountryCode: code}
		},
	}

	h := newCapTableHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/captable/country-profile", nil)
	w := httptest.NewRecorder()
	h.GetCapTableCountryProfile(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "BE", called, "default country code should be BE")
}

// ─────────────────────────────────────────────────────────────────────────────
// ListShareholders
// ─────────────────────────────────────────────────────────────────────────────

func TestListShareholders_OK(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	svc := &mockCapTableService{
		ListShareholdersFunc: func(_ context.Context, tid, sid uuid.UUID) ([]model.CapTableShareholder, error) {
			return []model.CapTableShareholder{
				{Name: "Alice", Type: model.ShareholderFounder},
				{Name: "Bob", Type: model.ShareholderFounder},
			}, nil
		},
	}

	h := newCapTableHandler(svc)
	req := capTableRequest(http.MethodGet, "/captable/shareholders", nil, scenarioID, tenantID)
	w := httptest.NewRecorder()
	h.ListShareholders(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result []model.CapTableShareholder
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Len(t, result, 2)
}

// ─────────────────────────────────────────────────────────────────────────────
// UpsertShareholders — create path (nil UUID)
// ─────────────────────────────────────────────────────────────────────────────

func TestUpsertShareholders_Create(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	createCalled := false
	svc := &mockCapTableService{
		CreateShareholderFunc: func(_ context.Context, sh *model.CapTableShareholder) error {
			createCalled = true
			return nil
		},
	}

	payload := map[string]interface{}{
		"id":   "00000000-0000-0000-0000-000000000000",
		"name": "Charlie",
		"type": "founder",
	}
	h := newCapTableHandler(svc)
	req := capTableRequest(http.MethodPut, "/captable/shareholders", payload, scenarioID, tenantID)
	w := httptest.NewRecorder()
	h.UpsertShareholders(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.True(t, createCalled)
}

// ─────────────────────────────────────────────────────────────────────────────
// UpsertShareholders — update path (existing UUID)
// ─────────────────────────────────────────────────────────────────────────────

func TestUpsertShareholders_Update(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	existingID := uuid.New()

	updateCalled := false
	svc := &mockCapTableService{
		UpdateShareholderFunc: func(_ context.Context, sh *model.CapTableShareholder) error {
			updateCalled = true
			return nil
		},
	}

	payload := map[string]interface{}{
		"id":   existingID.String(),
		"name": "Charlie Updated",
		"type": "founder",
	}
	h := newCapTableHandler(svc)
	req := capTableRequest(http.MethodPut, "/captable/shareholders", payload, scenarioID, tenantID)
	w := httptest.NewRecorder()
	h.UpsertShareholders(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, updateCalled)
}

// ─────────────────────────────────────────────────────────────────────────────
// DeleteShareholder
// ─────────────────────────────────────────────────────────────────────────────

func TestDeleteShareholder_OK(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	shareholderID := uuid.New()

	deleteCalled := false
	svc := &mockCapTableService{
		DeleteShareholderFunc: func(_ context.Context, tid, id uuid.UUID) error {
			deleteCalled = true
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, shareholderID, id)
			return nil
		},
	}

	h := newCapTableHandler(svc)
	req := capTableRequestWithID(http.MethodDelete, "/captable/shareholders/"+shareholderID.String(),
		nil, scenarioID, tenantID, shareholderID)
	w := httptest.NewRecorder()
	h.DeleteShareholder(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, deleteCalled)
}

func TestDeleteShareholder_InvalidID(t *testing.T) {
	h := newCapTableHandler(&mockCapTableService{})
	req := httptest.NewRequest(http.MethodDelete, "/captable/shareholders/bad-id", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "not-a-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.DeleteShareholder(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ─────────────────────────────────────────────────────────────────────────────
// ListCapTableRounds
// ─────────────────────────────────────────────────────────────────────────────

func TestListCapTableRounds_OK(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	svc := &mockCapTableService{
		ListRoundsFunc: func(_ context.Context, _, _ uuid.UUID) ([]model.CapTableRound, error) {
			return []model.CapTableRound{
				{Label: "Série A", PhaseNumber: 1},
			}, nil
		},
	}

	h := newCapTableHandler(svc)
	req := capTableRequest(http.MethodGet, "/captable/rounds", nil, scenarioID, tenantID)
	w := httptest.NewRecorder()
	h.ListCapTableRounds(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result []model.CapTableRound
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Len(t, result, 1)
	assert.Equal(t, "Série A", result[0].Label)
}

// ─────────────────────────────────────────────────────────────────────────────
// UpsertCapTableRounds — create path
// ─────────────────────────────────────────────────────────────────────────────

func TestUpsertCapTableRounds_Create(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	createCalled := false
	svc := &mockCapTableService{
		CreateRoundFunc: func(_ context.Context, rnd *model.CapTableRound) error {
			createCalled = true
			assert.Equal(t, "Seed", rnd.Label)
			return nil
		},
	}

	payload := map[string]interface{}{
		"id":            "00000000-0000-0000-0000-000000000000",
		"label":         "Seed",
		"phaseNumber":   1,
		"eventType":     "funding_round",
		"amountRaisedK": "100",
	}
	h := newCapTableHandler(svc)
	req := capTableRequest(http.MethodPut, "/captable/rounds", payload, scenarioID, tenantID)
	w := httptest.NewRecorder()
	h.UpsertCapTableRounds(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.True(t, createCalled)
}

// ─────────────────────────────────────────────────────────────────────────────
// ListOptionPlans
// ─────────────────────────────────────────────────────────────────────────────

func TestListOptionPlans_OK(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	svc := &mockCapTableService{
		ListPlansFunc: func(_ context.Context, _, _ uuid.UUID) ([]model.StockOptionPlan, error) {
			return []model.StockOptionPlan{
				{PlanLabel: "BSPCE 2024", Instrument: model.SOIBSPCE},
			}, nil
		},
	}

	h := newCapTableHandler(svc)
	req := capTableRequest(http.MethodGet, "/captable/option-plans", nil, scenarioID, tenantID)
	w := httptest.NewRecorder()
	h.ListOptionPlans(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result []model.StockOptionPlan
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Len(t, result, 1)
	assert.Equal(t, "BSPCE 2024", result[0].PlanLabel)
}

// ─────────────────────────────────────────────────────────────────────────────
// ListValuationScenarios
// ─────────────────────────────────────────────────────────────────────────────

func TestListValuationScenarios_OK(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	svc := &mockCapTableService{
		ListValuationScenariosFunc: func(_ context.Context, _, _ uuid.UUID) ([]model.ValuationScenario, error) {
			return []model.ValuationScenario{
				{CalcType: model.ValScenMultipleToIRR, Label: "Bull case"},
			}, nil
		},
	}

	h := newCapTableHandler(svc)
	req := capTableRequest(http.MethodGet, "/captable/valuation", nil, scenarioID, tenantID)
	w := httptest.NewRecorder()
	h.ListValuationScenarios(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result []model.ValuationScenario
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Len(t, result, 1)
}

// ─────────────────────────────────────────────────────────────────────────────
// CreateValuationScenario
// ─────────────────────────────────────────────────────────────────────────────

func TestCreateValuationScenario_OK(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	createCalled := false
	svc := &mockCapTableService{
		CreateValuationScenarioFunc: func(_ context.Context, vs *model.ValuationScenario) error {
			createCalled = true
			assert.Equal(t, model.ValScenMultipleToIRR, vs.CalcType)
			return nil
		},
	}

	payload := map[string]interface{}{
		"calcType": "multiple_to_irr",
		"label":    "Base case",
	}
	h := newCapTableHandler(svc)
	req := capTableRequest(http.MethodPost, "/captable/valuation", payload, scenarioID, tenantID)
	w := httptest.NewRecorder()
	h.CreateValuationScenario(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.True(t, createCalled)
}

// ─────────────────────────────────────────────────────────────────────────────
// ComputeValuationScenario
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeValuationScenario_OK(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	vsID := uuid.New()

	irr := decimal.NewFromFloat(0.25)
	svc := &mockCapTableService{
		ComputeValuationScenarioFunc: func(_ context.Context, tid, id uuid.UUID) (*model.ValuationScenarioResult, error) {
			return &model.ValuationScenarioResult{
				ScenarioID: id,
				CalcType:   model.ValScenMultipleToIRR,
				IRRPct:     &irr,
			}, nil
		},
	}

	h := newCapTableHandler(svc)
	req := capTableRequestWithID(http.MethodPost, "/captable/valuation/"+vsID.String()+"/compute",
		nil, scenarioID, tenantID, vsID)
	w := httptest.NewRecorder()
	h.ComputeValuationScenario(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result model.ValuationScenarioResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.NotNil(t, result.IRRPct)
}

// ─────────────────────────────────────────────────────────────────────────────
// GetCapTableReport
// ─────────────────────────────────────────────────────────────────────────────

func TestGetCapTableReport_OK(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	svc := &mockCapTableService{
		GetReportFunc: func(_ context.Context, _, _ uuid.UUID) (*model.CapTableReport, error) {
			return &model.CapTableReport{
				IngeFi: model.IngeFiReport{
					Phases: []model.IngeFiPhase{{PhaseNumber: 0, Label: "Création"}},
				},
			}, nil
		},
	}

	h := newCapTableHandler(svc)
	req := capTableRequest(http.MethodGet, "/captable/report", nil, scenarioID, tenantID)
	w := httptest.NewRecorder()
	h.GetCapTableReport(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result model.CapTableReport
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.Len(t, result.IngeFi.Phases, 1)
	assert.Equal(t, "Création", result.IngeFi.Phases[0].Label)
}

func TestGetCapTableReport_ServiceError(t *testing.T) {
	svc := &mockCapTableService{
		GetReportFunc: func(_ context.Context, _, _ uuid.UUID) (*model.CapTableReport, error) {
			return nil, apierror.Internal("compute failed")
		},
	}

	h := newCapTableHandler(svc)
	req := capTableRequest(http.MethodGet, "/captable/report", nil, uuid.New(), uuid.New())
	w := httptest.NewRecorder()
	h.GetCapTableReport(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ─────────────────────────────────────────────────────────────────────────────
// GetIngeFiReport
// ─────────────────────────────────────────────────────────────────────────────

func TestGetIngeFiReport_ReturnsIngefiSubReport(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	svc := &mockCapTableService{
		GetReportFunc: func(_ context.Context, _, _ uuid.UUID) (*model.CapTableReport, error) {
			return &model.CapTableReport{
				IngeFi: model.IngeFiReport{
					Phases: []model.IngeFiPhase{{PhaseNumber: 0}, {PhaseNumber: 1}},
				},
			}, nil
		},
	}

	h := newCapTableHandler(svc)
	req := capTableRequest(http.MethodGet, "/captable/report/ingefie", nil, scenarioID, tenantID)
	w := httptest.NewRecorder()
	h.GetIngeFiReport(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result model.IngeFiReport
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Len(t, result.Phases, 2)
}

// ─────────────────────────────────────────────────────────────────────────────
// GetDilutionWaterfall
// ─────────────────────────────────────────────────────────────────────────────

func TestGetDilutionWaterfall_ReturnsWaterfallSubReport(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	svc := &mockCapTableService{
		GetReportFunc: func(_ context.Context, _, _ uuid.UUID) (*model.CapTableReport, error) {
			return &model.CapTableReport{
				IngeFi: model.IngeFiReport{
					Waterfall: model.DilutionWaterfall{
						Holders: []model.DilutionWaterfallHolder{
							{Name: "Alice", Type: model.ShareholderFounder},
						},
					},
				},
			}, nil
		},
	}

	h := newCapTableHandler(svc)
	req := capTableRequest(http.MethodGet, "/captable/report/waterfall", nil, scenarioID, tenantID)
	w := httptest.NewRecorder()
	h.GetDilutionWaterfall(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result model.DilutionWaterfall
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.Len(t, result.Holders, 1)
	assert.Equal(t, "Alice", result.Holders[0].Name)
}

// ─────────────────────────────────────────────────────────────────────────────
// ListCapTableBranches
// ─────────────────────────────────────────────────────────────────────────────

func TestListCapTableBranches_OK(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	svc := &mockCapTableService{
		ListBranchesFunc: func(_ context.Context, _, _ uuid.UUID) ([]model.CapTableScenarioBranch, error) {
			return []model.CapTableScenarioBranch{
				{Name: "Bear case", Status: "draft"},
			}, nil
		},
	}

	h := newCapTableHandler(svc)
	req := capTableRequest(http.MethodGet, "/captable/branches", nil, scenarioID, tenantID)
	w := httptest.NewRecorder()
	h.ListCapTableBranches(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result []model.CapTableScenarioBranch
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Len(t, result, 1)
	assert.Equal(t, "Bear case", result[0].Name)
}
