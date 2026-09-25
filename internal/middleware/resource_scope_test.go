package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"ascenda/internal/model"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// ── Fakes for the narrow lookup interfaces ────────────────────────────────────

type fakeProducts map[uuid.UUID]*model.Product

func (f fakeProducts) GetByID(tenantID, id uuid.UUID) (*model.Product, error) {
	if p, ok := f[id]; ok && p.TenantID == tenantID {
		return p, nil
	}
	return nil, gorm.ErrRecordNotFound
}

type fakeBEP struct {
	snapshots map[uuid.UUID]*model.BEPSnapshot
	plans     map[uuid.UUID]*model.OptimisationPlan
}

func (f *fakeBEP) GetSnapshot(tenantID, id uuid.UUID) (*model.BEPSnapshot, error) {
	if s, ok := f.snapshots[id]; ok && s.TenantID == tenantID {
		return s, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeBEP) GetOptimisationPlan(tenantID, id uuid.UUID) (*model.OptimisationPlan, error) {
	if p, ok := f.plans[id]; ok && p.TenantID == tenantID {
		return p, nil
	}
	return nil, gorm.ErrRecordNotFound
}

type fakeCapTable struct {
	shareholders map[uuid.UUID]*model.CapTableShareholder
	rounds       map[uuid.UUID]*model.CapTableRound
	plans        map[uuid.UUID]*model.StockOptionPlan
	grants       map[uuid.UUID]*model.OptionGrant
	valuations   map[uuid.UUID]*model.ValuationScenario
	branches     map[uuid.UUID]*model.CapTableScenarioBranch
}

func (f *fakeCapTable) GetShareholder(tenantID, id uuid.UUID) (*model.CapTableShareholder, error) {
	if e, ok := f.shareholders[id]; ok && e.TenantID == tenantID {
		return e, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (f *fakeCapTable) GetRound(tenantID, id uuid.UUID) (*model.CapTableRound, error) {
	if e, ok := f.rounds[id]; ok && e.TenantID == tenantID {
		return e, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (f *fakeCapTable) GetPlan(tenantID, id uuid.UUID) (*model.StockOptionPlan, error) {
	if e, ok := f.plans[id]; ok && e.TenantID == tenantID {
		return e, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (f *fakeCapTable) GetGrant(tenantID, id uuid.UUID) (*model.OptionGrant, error) {
	if e, ok := f.grants[id]; ok && e.TenantID == tenantID {
		return e, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (f *fakeCapTable) GetValuationScenario(tenantID, id uuid.UUID) (*model.ValuationScenario, error) {
	if e, ok := f.valuations[id]; ok && e.TenantID == tenantID {
		return e, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (f *fakeCapTable) GetBranch(tenantID, id uuid.UUID) (*model.CapTableScenarioBranch, error) {
	if e, ok := f.branches[id]; ok && e.TenantID == tenantID {
		return e, nil
	}
	return nil, gorm.ErrRecordNotFound
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func scopedReq(tenantID uuid.UUID, params map[string]string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := ctxutil.WithTenantID(req.Context(), tenantID)
	ctx = ctxutil.WithUserID(ctx, uuid.New())
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))
}

func serve(t *testing.T, mw func(http.Handler) http.Handler, req *http.Request) (int, bool) {
	t.Helper()
	called := false
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Code, called
}

func scoped(tenantID, id uuid.UUID) model.TenantScoped {
	return model.TenantScoped{ID: id, TenantID: tenantID}
}

func quietLogger() *logrus.Entry {
	l := logrus.New()
	l.SetLevel(logrus.PanicLevel)
	return logrus.NewEntry(l)
}

// ── Products ──────────────────────────────────────────────────────────────────

func TestRequireProductInScenario(t *testing.T) {
	tenant := uuid.New()
	scenA, scenB := uuid.New(), uuid.New()
	prodA := &model.Product{TenantScoped: scoped(tenant, uuid.New()), ScenarioID: scenA}
	prodB := &model.Product{TenantScoped: scoped(tenant, uuid.New()), ScenarioID: scenB}
	prodOther := &model.Product{TenantScoped: scoped(uuid.New(), uuid.New()), ScenarioID: scenA}
	mw := NewResourceScopeMiddleware(fakeProducts{prodA.ID: prodA, prodB.ID: prodB, prodOther.ID: prodOther}, nil, nil, quietLogger())

	cases := []struct {
		name     string
		scenario string
		product  string
		want     int
	}{
		{"in scenario", scenA.String(), prodA.ID.String(), 200},
		{"other scenario", scenA.String(), prodB.ID.String(), 404},
		{"other tenant", scenA.String(), prodOther.ID.String(), 404},
		{"unknown", scenA.String(), uuid.New().String(), 404},
		{"bad product id", scenA.String(), "x", 400},
		{"bad scenario id", "x", prodA.ID.String(), 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, called := serve(t, mw.RequireProductInScenario, scopedReq(tenant, map[string]string{"scenarioId": tc.scenario, "productId": tc.product}))
			assert.Equal(t, tc.want, code)
			assert.Equal(t, tc.want == 200, called)
		})
	}
}

// ── BEP ───────────────────────────────────────────────────────────────────────

func TestRequireBEPSnapshotInScenario(t *testing.T) {
	tenant := uuid.New()
	scenA, scenB := uuid.New(), uuid.New()
	snapA := &model.BEPSnapshot{TenantScoped: scoped(tenant, uuid.New()), ScenarioID: scenA}
	snapB := &model.BEPSnapshot{TenantScoped: scoped(tenant, uuid.New()), ScenarioID: scenB}
	mw := NewResourceScopeMiddleware(nil, &fakeBEP{snapshots: map[uuid.UUID]*model.BEPSnapshot{snapA.ID: snapA, snapB.ID: snapB}}, nil, quietLogger())

	code, called := serve(t, mw.RequireBEPSnapshotInScenario, scopedReq(tenant, map[string]string{"scenarioId": scenA.String(), "snapshotId": snapA.ID.String()}))
	assert.Equal(t, 200, code)
	assert.True(t, called)

	code, called = serve(t, mw.RequireBEPSnapshotInScenario, scopedReq(tenant, map[string]string{"scenarioId": scenA.String(), "snapshotId": snapB.ID.String()}))
	assert.Equal(t, 404, code)
	assert.False(t, called)

	code, _ = serve(t, mw.RequireBEPSnapshotInScenario, scopedReq(tenant, map[string]string{"scenarioId": scenA.String(), "snapshotId": uuid.New().String()}))
	assert.Equal(t, 404, code)
}

func TestRequireOptimisationPlanInSnapshot(t *testing.T) {
	tenant := uuid.New()
	snapA, snapB := uuid.New(), uuid.New()
	planA := &model.OptimisationPlan{TenantScoped: scoped(tenant, uuid.New()), SnapshotID: snapA}
	planB := &model.OptimisationPlan{TenantScoped: scoped(tenant, uuid.New()), SnapshotID: snapB}
	mw := NewResourceScopeMiddleware(nil, &fakeBEP{plans: map[uuid.UUID]*model.OptimisationPlan{planA.ID: planA, planB.ID: planB}}, nil, quietLogger())

	code, called := serve(t, mw.RequireOptimisationPlanInSnapshot, scopedReq(tenant, map[string]string{"snapshotId": snapA.String(), "optPlanId": planA.ID.String()}))
	assert.Equal(t, 200, code)
	assert.True(t, called)

	code, called = serve(t, mw.RequireOptimisationPlanInSnapshot, scopedReq(tenant, map[string]string{"snapshotId": snapA.String(), "optPlanId": planB.ID.String()}))
	assert.Equal(t, 404, code)
	assert.False(t, called)

	// The financial {planId} of the outer route must not be mistaken for the
	// optimisation plan: with no optPlanId the request is a 400, never a pass.
	code, called = serve(t, mw.RequireOptimisationPlanInSnapshot, scopedReq(tenant, map[string]string{"snapshotId": snapA.String(), "planId": planA.ID.String()}))
	assert.Equal(t, 400, code)
	assert.False(t, called)
}

// ── Cap table ─────────────────────────────────────────────────────────────────

func TestRequireCapTableEntityInScenario(t *testing.T) {
	tenant := uuid.New()
	scenA, scenB := uuid.New(), uuid.New()
	ct := &fakeCapTable{
		shareholders: map[uuid.UUID]*model.CapTableShareholder{},
		rounds:       map[uuid.UUID]*model.CapTableRound{},
		plans:        map[uuid.UUID]*model.StockOptionPlan{},
		grants:       map[uuid.UUID]*model.OptionGrant{},
		valuations:   map[uuid.UUID]*model.ValuationScenario{},
		branches:     map[uuid.UUID]*model.CapTableScenarioBranch{},
	}
	shA := &model.CapTableShareholder{TenantScoped: scoped(tenant, uuid.New()), ScenarioID: scenA}
	shB := &model.CapTableShareholder{TenantScoped: scoped(tenant, uuid.New()), ScenarioID: scenB}
	ct.shareholders[shA.ID], ct.shareholders[shB.ID] = shA, shB
	rdA := &model.CapTableRound{TenantScoped: scoped(tenant, uuid.New()), ScenarioID: scenA}
	rdB := &model.CapTableRound{TenantScoped: scoped(tenant, uuid.New()), ScenarioID: scenB}
	ct.rounds[rdA.ID], ct.rounds[rdB.ID] = rdA, rdB
	opA := &model.StockOptionPlan{TenantScoped: scoped(tenant, uuid.New()), ScenarioID: scenA}
	opB := &model.StockOptionPlan{TenantScoped: scoped(tenant, uuid.New()), ScenarioID: scenB}
	ct.plans[opA.ID], ct.plans[opB.ID] = opA, opB
	grA := &model.OptionGrant{TenantScoped: scoped(tenant, uuid.New()), PlanID: opA.ID}
	grB := &model.OptionGrant{TenantScoped: scoped(tenant, uuid.New()), PlanID: opB.ID}
	grOrphan := &model.OptionGrant{TenantScoped: scoped(tenant, uuid.New()), PlanID: uuid.New()}
	ct.grants[grA.ID], ct.grants[grB.ID], ct.grants[grOrphan.ID] = grA, grB, grOrphan
	vsA := &model.ValuationScenario{TenantScoped: scoped(tenant, uuid.New()), ScenarioID: scenA}
	vsB := &model.ValuationScenario{TenantScoped: scoped(tenant, uuid.New()), ScenarioID: scenB}
	ct.valuations[vsA.ID], ct.valuations[vsB.ID] = vsA, vsB
	brA := &model.CapTableScenarioBranch{TenantScoped: scoped(tenant, uuid.New()), ScenarioID: scenA}
	brB := &model.CapTableScenarioBranch{TenantScoped: scoped(tenant, uuid.New()), ScenarioID: scenB}
	ct.branches[brA.ID], ct.branches[brB.ID] = brA, brB

	mw := NewResourceScopeMiddleware(nil, nil, ct, quietLogger())

	cases := []struct {
		kind  CapTableEntity
		param string
		inA   uuid.UUID
		inB   uuid.UUID
	}{
		{CapTableShareholder, "id", shA.ID, shB.ID},
		{CapTableRound, "id", rdA.ID, rdB.ID},
		{CapTableRound, "roundId", rdA.ID, rdB.ID},
		{CapTableOptionPlan, "id", opA.ID, opB.ID},
		{CapTableOptionGrant, "id", grA.ID, grB.ID},
		{CapTableValuation, "id", vsA.ID, vsB.ID},
		{CapTableBranch, "id", brA.ID, brB.ID},
	}
	for _, tc := range cases {
		t.Run(string(tc.kind)+"/"+tc.param, func(t *testing.T) {
			guard := mw.RequireCapTableEntityInScenario(tc.kind, tc.param)

			code, called := serve(t, guard, scopedReq(tenant, map[string]string{"scenarioId": scenA.String(), tc.param: tc.inA.String()}))
			assert.Equal(t, 200, code, "entity in scenario")
			assert.True(t, called)

			code, called = serve(t, guard, scopedReq(tenant, map[string]string{"scenarioId": scenA.String(), tc.param: tc.inB.String()}))
			assert.Equal(t, 404, code, "entity of another scenario")
			assert.False(t, called)

			code, _ = serve(t, guard, scopedReq(tenant, map[string]string{"scenarioId": scenA.String(), tc.param: uuid.New().String()}))
			assert.Equal(t, 404, code, "unknown entity")

			code, _ = serve(t, guard, scopedReq(uuid.New(), map[string]string{"scenarioId": scenA.String(), tc.param: tc.inA.String()}))
			assert.Equal(t, 404, code, "other tenant")
		})
	}

	t.Run("grant whose plan is missing is not found", func(t *testing.T) {
		code, called := serve(t, mw.RequireCapTableEntityInScenario(CapTableOptionGrant, "id"), scopedReq(tenant, map[string]string{"scenarioId": scenA.String(), "id": grOrphan.ID.String()}))
		assert.Equal(t, 404, code)
		assert.False(t, called)
	})
}
