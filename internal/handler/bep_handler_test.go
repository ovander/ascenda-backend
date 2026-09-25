package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ascenda/internal/model"
	"ascenda/internal/service"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── mockBEPService ────────────────────────────────────────────────────────────

type mockBEPService struct {
	listSnapshotsFn             func(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]*model.BEPSnapshot, error)
	createSnapshotFn            func(ctx context.Context, snap *model.BEPSnapshot) error
	getSnapshotFn               func(ctx context.Context, tenantID, id uuid.UUID) (*model.BEPSnapshot, error)
	updateSnapshotFn            func(ctx context.Context, snap *model.BEPSnapshot) error
	deleteSnapshotFn            func(ctx context.Context, tenantID, scenarioID, id uuid.UUID) error
	listFixedCostLinesFn        func(ctx context.Context, tenantID, snapshotID uuid.UUID) ([]*model.FixedCostLine, error)
	upsertFixedCostLinesFn      func(ctx context.Context, tenantID, snapshotID uuid.UUID, lines []model.FixedCostLine) error
	listVariableCostLinesFn     func(ctx context.Context, tenantID, snapshotID uuid.UUID) ([]*model.VariableCostLine, error)
	upsertVariableCostLinesFn   func(ctx context.Context, tenantID, snapshotID uuid.UUID, lines []model.VariableCostLine) error
	listSensitivityConfigsFn    func(ctx context.Context, tenantID, snapshotID uuid.UUID) ([]*model.SensitivityConfig, error)
	upsertSensitivityConfigFn   func(ctx context.Context, cfg *model.SensitivityConfig) error
	getBEPReportFn              func(ctx context.Context, tenantID, snapshotID uuid.UUID) (*model.BEPReport, error)
	getPCGAccountsFn            func() []model.PCGAccount
	listOptimisationPlansFn     func(ctx context.Context, tenantID, snapshotID uuid.UUID) ([]*model.OptimisationPlan, error)
	createOptimisationPlanFn    func(ctx context.Context, plan *model.OptimisationPlan) error
	getOptimisationPlanFn       func(ctx context.Context, tenantID, id uuid.UUID) (*model.OptimisationPlan, error)
	updateOptimisationPlanFn    func(ctx context.Context, plan *model.OptimisationPlan) error
	deleteOptimisationPlanFn    func(ctx context.Context, tenantID, id uuid.UUID) error
	listFixedCostSavingsFn      func(ctx context.Context, tenantID, planID uuid.UUID) ([]*model.FixedCostSaving, error)
	upsertFixedCostSavingsFn    func(ctx context.Context, tenantID, planID uuid.UUID, savings []model.FixedCostSaving) error
	listVariableCostSavingsFn   func(ctx context.Context, tenantID, planID uuid.UUID) ([]*model.VariableCostSaving, error)
	upsertVariableCostSavingsFn func(ctx context.Context, tenantID, planID uuid.UUID, savings []model.VariableCostSaving) error
	listPCGReviewItemsFn        func(ctx context.Context, tenantID, planID uuid.UUID) ([]*model.PCGReviewItem, error)
	upsertPCGReviewItemsFn      func(ctx context.Context, tenantID, planID uuid.UUID, items []model.PCGReviewItem) error
	getOptimisedBEPReportFn     func(ctx context.Context, tenantID, planID uuid.UUID) (*model.OptimisedBEPReport, error)
	importFromPlanFn            func(ctx context.Context, tenantID, scenarioID, snapshotID uuid.UUID, yearIndex int) (*model.BEPSnapshot, error)
	previewFromPlanFn           func(ctx context.Context, tenantID, scenarioID uuid.UUID, yearIndex int) (*service.BEPPlanPreview, error)
	getMultiYearBEPReportFn     func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.MultiYearBEPReport, error)
}

// Snapshots
func (m *mockBEPService) ListSnapshots(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]*model.BEPSnapshot, error) {
	if m.listSnapshotsFn != nil {
		return m.listSnapshotsFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("list snapshots not implemented")
}

func (m *mockBEPService) CreateSnapshot(ctx context.Context, snap *model.BEPSnapshot) error {
	if m.createSnapshotFn != nil {
		return m.createSnapshotFn(ctx, snap)
	}
	return apierror.Internal("create snapshot not implemented")
}

func (m *mockBEPService) GetSnapshot(ctx context.Context, tenantID, id uuid.UUID) (*model.BEPSnapshot, error) {
	if m.getSnapshotFn != nil {
		return m.getSnapshotFn(ctx, tenantID, id)
	}
	return nil, apierror.Internal("get snapshot not implemented")
}

func (m *mockBEPService) UpdateSnapshot(ctx context.Context, snap *model.BEPSnapshot) error {
	if m.updateSnapshotFn != nil {
		return m.updateSnapshotFn(ctx, snap)
	}
	return apierror.Internal("update snapshot not implemented")
}

func (m *mockBEPService) DeleteSnapshot(ctx context.Context, tenantID, scenarioID, id uuid.UUID) error {
	if m.deleteSnapshotFn != nil {
		return m.deleteSnapshotFn(ctx, tenantID, scenarioID, id)
	}
	return apierror.Internal("delete snapshot not implemented")
}

// Fixed cost lines
func (m *mockBEPService) ListFixedCostLines(ctx context.Context, tenantID, snapshotID uuid.UUID) ([]*model.FixedCostLine, error) {
	if m.listFixedCostLinesFn != nil {
		return m.listFixedCostLinesFn(ctx, tenantID, snapshotID)
	}
	return nil, apierror.Internal("list fixed cost lines not implemented")
}

func (m *mockBEPService) UpsertFixedCostLines(ctx context.Context, tenantID, snapshotID uuid.UUID, lines []model.FixedCostLine) error {
	if m.upsertFixedCostLinesFn != nil {
		return m.upsertFixedCostLinesFn(ctx, tenantID, snapshotID, lines)
	}
	return apierror.Internal("upsert fixed cost lines not implemented")
}

// Variable cost lines
func (m *mockBEPService) ListVariableCostLines(ctx context.Context, tenantID, snapshotID uuid.UUID) ([]*model.VariableCostLine, error) {
	if m.listVariableCostLinesFn != nil {
		return m.listVariableCostLinesFn(ctx, tenantID, snapshotID)
	}
	return nil, apierror.Internal("list variable cost lines not implemented")
}

func (m *mockBEPService) UpsertVariableCostLines(ctx context.Context, tenantID, snapshotID uuid.UUID, lines []model.VariableCostLine) error {
	if m.upsertVariableCostLinesFn != nil {
		return m.upsertVariableCostLinesFn(ctx, tenantID, snapshotID, lines)
	}
	return apierror.Internal("upsert variable cost lines not implemented")
}

// Sensitivity configs
func (m *mockBEPService) ListSensitivityConfigs(ctx context.Context, tenantID, snapshotID uuid.UUID) ([]*model.SensitivityConfig, error) {
	if m.listSensitivityConfigsFn != nil {
		return m.listSensitivityConfigsFn(ctx, tenantID, snapshotID)
	}
	return nil, apierror.Internal("list sensitivity configs not implemented")
}

func (m *mockBEPService) UpsertSensitivityConfig(ctx context.Context, cfg *model.SensitivityConfig) error {
	if m.upsertSensitivityConfigFn != nil {
		return m.upsertSensitivityConfigFn(ctx, cfg)
	}
	return apierror.Internal("upsert sensitivity config not implemented")
}

// Computed reports
func (m *mockBEPService) GetBEPReport(ctx context.Context, tenantID, snapshotID uuid.UUID) (*model.BEPReport, error) {
	if m.getBEPReportFn != nil {
		return m.getBEPReportFn(ctx, tenantID, snapshotID)
	}
	return nil, apierror.Internal("get BEP report not implemented")
}

func (m *mockBEPService) GetPCGAccounts() []model.PCGAccount {
	if m.getPCGAccountsFn != nil {
		return m.getPCGAccountsFn()
	}
	return []model.PCGAccount{}
}

// Optimisation plans
func (m *mockBEPService) ListOptimisationPlans(ctx context.Context, tenantID, snapshotID uuid.UUID) ([]*model.OptimisationPlan, error) {
	if m.listOptimisationPlansFn != nil {
		return m.listOptimisationPlansFn(ctx, tenantID, snapshotID)
	}
	return nil, apierror.Internal("list optimisation plans not implemented")
}

func (m *mockBEPService) CreateOptimisationPlan(ctx context.Context, plan *model.OptimisationPlan) error {
	if m.createOptimisationPlanFn != nil {
		return m.createOptimisationPlanFn(ctx, plan)
	}
	return apierror.Internal("create optimisation plan not implemented")
}

func (m *mockBEPService) GetOptimisationPlan(ctx context.Context, tenantID, id uuid.UUID) (*model.OptimisationPlan, error) {
	if m.getOptimisationPlanFn != nil {
		return m.getOptimisationPlanFn(ctx, tenantID, id)
	}
	return nil, apierror.Internal("get optimisation plan not implemented")
}

func (m *mockBEPService) UpdateOptimisationPlan(ctx context.Context, plan *model.OptimisationPlan) error {
	if m.updateOptimisationPlanFn != nil {
		return m.updateOptimisationPlanFn(ctx, plan)
	}
	return apierror.Internal("update optimisation plan not implemented")
}

func (m *mockBEPService) DeleteOptimisationPlan(ctx context.Context, tenantID, id uuid.UUID) error {
	if m.deleteOptimisationPlanFn != nil {
		return m.deleteOptimisationPlanFn(ctx, tenantID, id)
	}
	return apierror.Internal("delete optimisation plan not implemented")
}

// Savings
func (m *mockBEPService) ListFixedCostSavings(ctx context.Context, tenantID, planID uuid.UUID) ([]*model.FixedCostSaving, error) {
	if m.listFixedCostSavingsFn != nil {
		return m.listFixedCostSavingsFn(ctx, tenantID, planID)
	}
	return nil, apierror.Internal("list fixed cost savings not implemented")
}

func (m *mockBEPService) UpsertFixedCostSavings(ctx context.Context, tenantID, planID uuid.UUID, savings []model.FixedCostSaving) error {
	if m.upsertFixedCostSavingsFn != nil {
		return m.upsertFixedCostSavingsFn(ctx, tenantID, planID, savings)
	}
	return apierror.Internal("upsert fixed cost savings not implemented")
}

func (m *mockBEPService) ListVariableCostSavings(ctx context.Context, tenantID, planID uuid.UUID) ([]*model.VariableCostSaving, error) {
	if m.listVariableCostSavingsFn != nil {
		return m.listVariableCostSavingsFn(ctx, tenantID, planID)
	}
	return nil, apierror.Internal("list variable cost savings not implemented")
}

func (m *mockBEPService) UpsertVariableCostSavings(ctx context.Context, tenantID, planID uuid.UUID, savings []model.VariableCostSaving) error {
	if m.upsertVariableCostSavingsFn != nil {
		return m.upsertVariableCostSavingsFn(ctx, tenantID, planID, savings)
	}
	return apierror.Internal("upsert variable cost savings not implemented")
}

// PCG review
func (m *mockBEPService) ListPCGReviewItems(ctx context.Context, tenantID, planID uuid.UUID) ([]*model.PCGReviewItem, error) {
	if m.listPCGReviewItemsFn != nil {
		return m.listPCGReviewItemsFn(ctx, tenantID, planID)
	}
	return nil, apierror.Internal("list PCG review items not implemented")
}

func (m *mockBEPService) UpsertPCGReviewItems(ctx context.Context, tenantID, planID uuid.UUID, items []model.PCGReviewItem) error {
	if m.upsertPCGReviewItemsFn != nil {
		return m.upsertPCGReviewItemsFn(ctx, tenantID, planID, items)
	}
	return apierror.Internal("upsert PCG review items not implemented")
}

// Optimised report
func (m *mockBEPService) GetOptimisedBEPReport(ctx context.Context, tenantID, planID uuid.UUID) (*model.OptimisedBEPReport, error) {
	if m.getOptimisedBEPReportFn != nil {
		return m.getOptimisedBEPReportFn(ctx, tenantID, planID)
	}
	return nil, apierror.Internal("get optimised BEP report not implemented")
}

// Plan import
func (m *mockBEPService) ImportFromPlan(ctx context.Context, tenantID, scenarioID, snapshotID uuid.UUID, yearIndex int) (*model.BEPSnapshot, error) {
	if m.importFromPlanFn != nil {
		return m.importFromPlanFn(ctx, tenantID, scenarioID, snapshotID, yearIndex)
	}
	return nil, apierror.Internal("import from plan not implemented")
}

// Plan preview
func (m *mockBEPService) PreviewFromPlan(ctx context.Context, tenantID, scenarioID uuid.UUID, yearIndex int) (*service.BEPPlanPreview, error) {
	if m.previewFromPlanFn != nil {
		return m.previewFromPlanFn(ctx, tenantID, scenarioID, yearIndex)
	}
	return nil, apierror.Internal("preview from plan not implemented")
}

// Multi-year BEP report
func (m *mockBEPService) GetMultiYearBEPReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.MultiYearBEPReport, error) {
	if m.getMultiYearBEPReportFn != nil {
		return m.getMultiYearBEPReportFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("get multi-year BEP report not implemented")
}

// ── fixtures ──────────────────────────────────────────────────────────────────

func newBEPHandler(svc *mockBEPService) *BEPHandler {
	return NewBEPHandler(svc, logrus.NewEntry(logrus.New()))
}

// ── ListSnapshots ─────────────────────────────────────────────────────────────

func TestBEPHandler_ListSnapshots_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	snapshots := []*model.BEPSnapshot{
		{
			TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
			ScenarioID:   scenarioID,
			Label:        "Q1 2024",
		},
	}

	svc := &mockBEPService{
		listSnapshotsFn: func(_ context.Context, tid, sid uuid.UUID) ([]*model.BEPSnapshot, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return snapshots, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newBEPHandler(svc).ListSnapshots(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 1)
}

func TestBEPHandler_ListSnapshots_BadScenarioUUID(t *testing.T) {
	svc := &mockBEPService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBEPHandler(svc).ListSnapshots(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBEPHandler_ListSnapshots_ServiceError(t *testing.T) {
	svc := &mockBEPService{
		listSnapshotsFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.BEPSnapshot, error) {
			return nil, apierror.NotFound("scenario", "x")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBEPHandler(svc).ListSnapshots(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── CreateSnapshot ────────────────────────────────────────────────────────────

func TestBEPHandler_CreateSnapshot_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	snap := model.BEPSnapshot{
		Label:                 "Q1 2024",
		FixedCostsTotal:       decimal.NewFromInt(50000),
		ContributionMarginPct: decimal.NewFromInt(40),
	}

	svc := &mockBEPService{
		createSnapshotFn: func(_ context.Context, s *model.BEPSnapshot) error {
			assert.Equal(t, tenantID, s.TenantID)
			assert.Equal(t, scenarioID, s.ScenarioID)
			assert.Equal(t, "Q1 2024", s.Label)
			return nil
		},
	}

	body, _ := json.Marshal(snap)
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newBEPHandler(svc).CreateSnapshot(w, r)

	require.Equal(t, http.StatusCreated, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

func TestBEPHandler_CreateSnapshot_BadScenarioUUID(t *testing.T) {
	svc := &mockBEPService{}
	snap := model.BEPSnapshot{Label: "Q1"}
	body, _ := json.Marshal(snap)
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBEPHandler(svc).CreateSnapshot(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBEPHandler_CreateSnapshot_InvalidBody(t *testing.T) {
	svc := &mockBEPService{}
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("invalid json")))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBEPHandler(svc).CreateSnapshot(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── GetSnapshot ───────────────────────────────────────────────────────────────

func TestBEPHandler_GetSnapshot_Success(t *testing.T) {
	tenantID := uuid.New()
	snapID := uuid.New()
	snap := &model.BEPSnapshot{
		TenantScoped: model.TenantScoped{ID: snapID, TenantID: tenantID},
		Label:        "Q1 2024",
	}

	svc := &mockBEPService{
		getSnapshotFn: func(_ context.Context, tid, id uuid.UUID) (*model.BEPSnapshot, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, snapID, id)
			return snap, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"snapshotId": snapID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newBEPHandler(svc).GetSnapshot(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

func TestBEPHandler_GetSnapshot_BadSnapshotID(t *testing.T) {
	svc := &mockBEPService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"snapshotId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBEPHandler(svc).GetSnapshot(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBEPHandler_GetSnapshot_NotFound(t *testing.T) {
	svc := &mockBEPService{
		getSnapshotFn: func(_ context.Context, _, _ uuid.UUID) (*model.BEPSnapshot, error) {
			return nil, apierror.NotFound("snapshot", "x")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"snapshotId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBEPHandler(svc).GetSnapshot(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── UpdateSnapshot ────────────────────────────────────────────────────────────

func TestBEPHandler_UpdateSnapshot_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	snapID := uuid.New()
	snap := model.BEPSnapshot{
		Label:                 "Q1 Updated",
		FixedCostsTotal:       decimal.NewFromInt(60000),
		ContributionMarginPct: decimal.NewFromInt(45),
	}

	svc := &mockBEPService{
		updateSnapshotFn: func(_ context.Context, s *model.BEPSnapshot) error {
			assert.Equal(t, tenantID, s.TenantID)
			assert.Equal(t, snapID, s.ID)
			return nil
		},
	}

	body, _ := json.Marshal(snap)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{
		"snapshotId": snapID.String(),
		"scenarioId": scenarioID.String(),
	})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newBEPHandler(svc).UpdateSnapshot(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestBEPHandler_UpdateSnapshot_BadSnapshotID(t *testing.T) {
	svc := &mockBEPService{}
	snap := model.BEPSnapshot{Label: "Q1"}
	body, _ := json.Marshal(snap)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"snapshotId": "bad", "scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBEPHandler(svc).UpdateSnapshot(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── DeleteSnapshot ────────────────────────────────────────────────────────────

func TestBEPHandler_DeleteSnapshot_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	snapID := uuid.New()
	called := false

	svc := &mockBEPService{
		deleteSnapshotFn: func(_ context.Context, tid, sid, id uuid.UUID) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			assert.Equal(t, snapID, id)
			called = true
			return nil
		},
	}

	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r = withChiParams(r, map[string]string{
		"snapshotId": snapID.String(),
		"scenarioId": scenarioID.String(),
	})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newBEPHandler(svc).DeleteSnapshot(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, called)
}

func TestBEPHandler_DeleteSnapshot_BadSnapshotID(t *testing.T) {
	svc := &mockBEPService{}
	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r = withChiParams(r, map[string]string{
		"snapshotId": "bad",
		"scenarioId": uuid.New().String(),
	})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBEPHandler(svc).DeleteSnapshot(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── ListFixedCostLines ────────────────────────────────────────────────────────

func TestBEPHandler_ListFixedCostLines_Success(t *testing.T) {
	tenantID := uuid.New()
	snapID := uuid.New()
	lines := []*model.FixedCostLine{
		{
			TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
			SnapshotID:   snapID,
			Category:     model.FixedCostPayroll,
			Label:        "Salaries",
			AmountAnnual: decimal.NewFromInt(120000),
		},
	}

	svc := &mockBEPService{
		listFixedCostLinesFn: func(_ context.Context, tid, sid uuid.UUID) ([]*model.FixedCostLine, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, snapID, sid)
			return lines, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"snapshotId": snapID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newBEPHandler(svc).ListFixedCostLines(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 1)
}

func TestBEPHandler_ListFixedCostLines_BadSnapshotID(t *testing.T) {
	svc := &mockBEPService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"snapshotId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBEPHandler(svc).ListFixedCostLines(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── UpsertFixedCostLines ──────────────────────────────────────────────────────

func TestBEPHandler_UpsertFixedCostLines_Success(t *testing.T) {
	tenantID := uuid.New()
	snapID := uuid.New()
	lines := []model.FixedCostLine{
		{
			Category:     model.FixedCostPayroll,
			Label:        "Salaries",
			AmountAnnual: decimal.NewFromInt(120000),
		},
	}

	svc := &mockBEPService{
		upsertFixedCostLinesFn: func(_ context.Context, tid, sid uuid.UUID, ll []model.FixedCostLine) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, snapID, sid)
			assert.Len(t, ll, 1)
			return nil
		},
	}

	body, _ := json.Marshal(lines)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"snapshotId": snapID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newBEPHandler(svc).UpsertFixedCostLines(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestBEPHandler_UpsertFixedCostLines_BadSnapshotID(t *testing.T) {
	svc := &mockBEPService{}
	lines := []model.FixedCostLine{}
	body, _ := json.Marshal(lines)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"snapshotId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBEPHandler(svc).UpsertFixedCostLines(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── ListVariableCostLines ─────────────────────────────────────────────────────

func TestBEPHandler_ListVariableCostLines_Success(t *testing.T) {
	tenantID := uuid.New()
	snapID := uuid.New()
	lines := []*model.VariableCostLine{
		{
			TenantScoped:  model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
			SnapshotID:    snapID,
			Category:      model.VarCostMaterials,
			Label:         "Raw materials",
			AmountPerUnit: decimal.NewFromFloat(5.5),
		},
	}

	svc := &mockBEPService{
		listVariableCostLinesFn: func(_ context.Context, tid, sid uuid.UUID) ([]*model.VariableCostLine, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, snapID, sid)
			return lines, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"snapshotId": snapID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newBEPHandler(svc).ListVariableCostLines(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 1)
}

func TestBEPHandler_ListVariableCostLines_BadSnapshotID(t *testing.T) {
	svc := &mockBEPService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"snapshotId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBEPHandler(svc).ListVariableCostLines(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── UpsertVariableCostLines ───────────────────────────────────────────────────

func TestBEPHandler_UpsertVariableCostLines_Success(t *testing.T) {
	tenantID := uuid.New()
	snapID := uuid.New()
	lines := []model.VariableCostLine{
		{
			Category:      model.VarCostMaterials,
			Label:         "Raw materials",
			AmountPerUnit: decimal.NewFromFloat(5.5),
		},
	}

	svc := &mockBEPService{
		upsertVariableCostLinesFn: func(_ context.Context, tid, sid uuid.UUID, ll []model.VariableCostLine) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, snapID, sid)
			assert.Len(t, ll, 1)
			return nil
		},
	}

	body, _ := json.Marshal(lines)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"snapshotId": snapID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newBEPHandler(svc).UpsertVariableCostLines(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestBEPHandler_UpsertVariableCostLines_BadSnapshotID(t *testing.T) {
	svc := &mockBEPService{}
	lines := []model.VariableCostLine{}
	body, _ := json.Marshal(lines)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"snapshotId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBEPHandler(svc).UpsertVariableCostLines(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── GetBEPReport ──────────────────────────────────────────────────────────────

func TestBEPHandler_GetBEPReport_Success(t *testing.T) {
	tenantID := uuid.New()
	snapID := uuid.New()
	bepRev := decimal.NewFromInt(100000)
	report := &model.BEPReport{
		SnapshotID: snapID,
		Core: model.BEPCoreResult{
			BEPRevenue:      &bepRev,
			VariableCostPct: decimal.NewFromInt(60),
		},
	}

	svc := &mockBEPService{
		getBEPReportFn: func(_ context.Context, tid, sid uuid.UUID) (*model.BEPReport, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, snapID, sid)
			return report, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"snapshotId": snapID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newBEPHandler(svc).GetBEPReport(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

func TestBEPHandler_GetBEPReport_BadSnapshotID(t *testing.T) {
	svc := &mockBEPService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"snapshotId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBEPHandler(svc).GetBEPReport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── GetPCGAccounts ────────────────────────────────────────────────────────────

func TestBEPHandler_GetPCGAccounts_Success(t *testing.T) {
	svc := &mockBEPService{
		getPCGAccountsFn: func() []model.PCGAccount {
			return []model.PCGAccount{
				{Code: "601", Label: "Achats de matières premières et fournitures", Group: "60"},
				{Code: "641", Label: "Rémunérations du personnel", Group: "64"},
			}
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBEPHandler(svc).GetPCGAccounts(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 2)
}

// ── ListOptimisationPlans ─────────────────────────────────────────────────────

func TestBEPHandler_ListOptimisationPlans_Success(t *testing.T) {
	tenantID := uuid.New()
	snapID := uuid.New()
	plans := []*model.OptimisationPlan{
		{
			TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
			SnapshotID:   snapID,
			Name:         "Headcount reduction",
			Status:       model.BEPPlanDraft,
		},
	}

	svc := &mockBEPService{
		listOptimisationPlansFn: func(_ context.Context, tid, sid uuid.UUID) ([]*model.OptimisationPlan, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, snapID, sid)
			return plans, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"snapshotId": snapID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newBEPHandler(svc).ListOptimisationPlans(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 1)
}

// ── CreateOptimisationPlan ────────────────────────────────────────────────────

func TestBEPHandler_CreateOptimisationPlan_Success(t *testing.T) {
	tenantID := uuid.New()
	snapID := uuid.New()
	userID := uuid.New()
	plan := model.OptimisationPlan{
		Name:   "Headcount reduction",
		Status: model.BEPPlanDraft,
	}

	svc := &mockBEPService{
		createOptimisationPlanFn: func(_ context.Context, p *model.OptimisationPlan) error {
			assert.Equal(t, tenantID, p.TenantID)
			assert.Equal(t, snapID, p.SnapshotID)
			assert.Equal(t, userID, p.CreatedBy)
			return nil
		},
	}

	body, _ := json.Marshal(plan)
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"snapshotId": snapID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	r = r.WithContext(ctxutil.WithUserID(r.Context(), userID))
	w := httptest.NewRecorder()

	newBEPHandler(svc).CreateOptimisationPlan(w, r)

	require.Equal(t, http.StatusCreated, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

// ── GetMultiYearBEPReport ─────────────────────────────────────────────────────

func TestBEPHandler_GetMultiYearBEPReport_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	report := &model.MultiYearBEPReport{
		TotalCumulativeEBE: decimal.NewFromInt(500000),
	}

	svc := &mockBEPService{
		getMultiYearBEPReportFn: func(_ context.Context, tid, sid uuid.UUID) (*model.MultiYearBEPReport, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return report, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newBEPHandler(svc).GetMultiYearBEPReport(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

func TestBEPHandler_GetMultiYearBEPReport_BadScenarioID(t *testing.T) {
	svc := &mockBEPService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBEPHandler(svc).GetMultiYearBEPReport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── ImportFromPlan ────────────────────────────────────────────────────────────

func TestBEPHandler_ImportFromPlan_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	snapID := uuid.New()
	snap := &model.BEPSnapshot{
		TenantScoped: model.TenantScoped{ID: snapID, TenantID: tenantID},
		Label:        "Imported from plan",
	}

	svc := &mockBEPService{
		importFromPlanFn: func(_ context.Context, tid, sid, snapid uuid.UUID, yearIndex int) (*model.BEPSnapshot, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			assert.Equal(t, snapID, snapid)
			assert.Equal(t, 0, yearIndex) // year=1 becomes yearIndex=0
			return snap, nil
		},
	}

	r := httptest.NewRequest(http.MethodPost, "/?year=1", nil)
	r = withChiParams(r, map[string]string{
		"snapshotId": snapID.String(),
		"scenarioId": scenarioID.String(),
	})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newBEPHandler(svc).ImportFromPlan(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

func TestBEPHandler_ImportFromPlan_MissingYear(t *testing.T) {
	svc := &mockBEPService{}
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r = withChiParams(r, map[string]string{
		"snapshotId": uuid.New().String(),
		"scenarioId": uuid.New().String(),
	})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBEPHandler(svc).ImportFromPlan(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestBEPHandler_ImportFromPlan_YearOutOfRange(t *testing.T) {
	svc := &mockBEPService{}
	r := httptest.NewRequest(http.MethodPost, "/?year=6", nil)
	r = withChiParams(r, map[string]string{
		"snapshotId": uuid.New().String(),
		"scenarioId": uuid.New().String(),
	})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBEPHandler(svc).ImportFromPlan(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// ── PreviewFromPlan ───────────────────────────────────────────────────────────

func TestBEPHandler_PreviewFromPlan_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	preview := &service.BEPPlanPreview{
		FixedCostsTotal:       decimal.NewFromInt(50000),
		ContributionMarginPct: decimal.NewFromInt(40),
	}

	svc := &mockBEPService{
		previewFromPlanFn: func(_ context.Context, tid, sid uuid.UUID, yearIndex int) (*service.BEPPlanPreview, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			assert.Equal(t, 0, yearIndex)
			return preview, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/?year=1", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newBEPHandler(svc).PreviewFromPlan(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

func TestBEPHandler_PreviewFromPlan_MissingYear(t *testing.T) {
	svc := &mockBEPService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newBEPHandler(svc).PreviewFromPlan(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}
