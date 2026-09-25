package handler

// Tests for the plan scoping of the audit trail (audit finding S-H2): the
// detail endpoint must not expose a scenario the caller cannot access, and
// the list must be restricted to accessible plans for non-owners.

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"ascenda/internal/model"
	"context"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Fakes ─────────────────────────────────────────────────────────────────────

type fakeAuditRepo struct {
	entries      map[uuid.UUID]*model.AuditLog
	listedAll    bool
	listedEntity []uuid.UUID
}

func (f *fakeAuditRepo) Create(l *model.AuditLog) error { f.entries[l.ID] = l; return nil }
func (f *fakeAuditRepo) GetByID(tenantID, id uuid.UUID) (*model.AuditLog, error) {
	if e, ok := f.entries[id]; ok && e.TenantID == tenantID {
		return e, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeAuditRepo) ListByEntity(tenantID uuid.UUID, entityType string, entityID uuid.UUID) ([]*model.AuditLog, error) {
	return nil, nil
}
func (f *fakeAuditRepo) ListByTenant(tenantID uuid.UUID, offset, limit int) ([]*model.AuditLog, error) {
	f.listedAll = true
	var out []*model.AuditLog
	for _, e := range f.entries {
		if e.TenantID == tenantID {
			out = append(out, e)
		}
	}
	return out, nil
}
func (f *fakeAuditRepo) CountByTenant(tenantID uuid.UUID) (int64, error) {
	n := int64(0)
	for _, e := range f.entries {
		if e.TenantID == tenantID {
			n++
		}
	}
	return n, nil
}
func (f *fakeAuditRepo) ListByTenantAndEntities(tenantID uuid.UUID, ids []uuid.UUID, offset, limit int) ([]*model.AuditLog, error) {
	f.listedEntity = ids
	var out []*model.AuditLog
	for _, e := range f.entries {
		if e.TenantID != tenantID {
			continue
		}
		for _, id := range ids {
			if e.EntityID == id {
				out = append(out, e)
				break
			}
		}
	}
	return out, nil
}
func (f *fakeAuditRepo) CountByTenantAndEntities(tenantID uuid.UUID, ids []uuid.UUID) (int64, error) {
	rows, _ := f.ListByTenantAndEntities(tenantID, ids, 0, 1000)
	return int64(len(rows)), nil
}
func (f *fakeAuditRepo) ListByEntityID(tenantID, entityID uuid.UUID) ([]*model.AuditLog, error) {
	return nil, nil
}
func (f *fakeAuditRepo) ListByUser(tenantID, userID uuid.UUID, offset, limit int) ([]*model.AuditLog, error) {
	return nil, nil
}

type fakeCapturer struct{ captured []uuid.UUID }

func (f *fakeCapturer) CaptureScenarioData(tenantID, scenarioID uuid.UUID) (json.RawMessage, error) {
	f.captured = append(f.captured, scenarioID)
	return json.RawMessage(`{"products":[{"name":"secret"}]}`), nil
}

// fakeAccess grants access to the plans in allowed (owner → everything).
type fakeAccess struct {
	allowed  map[uuid.UUID]bool
	entities []uuid.UUID
}

func (f *fakeAccess) PlanRole(tenantID, userID uuid.UUID, tenantRole string, planID uuid.UUID) (string, bool) {
	if tenantRole == "owner" {
		return "owner", true
	}
	if f.allowed[planID] {
		return "editor", true
	}
	return "", false
}
func (f *fakeAccess) AccessibleEntityIDs(tenantID, userID uuid.UUID, tenantRole string) ([]uuid.UUID, bool, error) {
	if tenantRole == "owner" {
		return nil, true, nil
	}
	return f.entities, false, nil
}

type fakeScenarioLookup map[uuid.UUID]*model.Scenario

func (f fakeScenarioLookup) GetByID(tenantID, id uuid.UUID) (*model.Scenario, error) {
	if s, ok := f[id]; ok && s.TenantID == tenantID {
		return s, nil
	}
	return nil, errors.New("not found")
}

// ── Fixture ───────────────────────────────────────────────────────────────────

type auditFixture struct {
	h          *AuditHandler
	repo       *fakeAuditRepo
	capturer   *fakeCapturer
	tenant     uuid.UUID
	user       uuid.UUID
	planMine   uuid.UUID
	planOther  uuid.UUID
	scenMine   uuid.UUID
	scenOther  uuid.UUID
	entryMine  uuid.UUID // scenario event on scenMine
	entryOther uuid.UUID // scenario event on scenOther
	entryPlan  uuid.UUID // plan event on planOther
	entryExp   uuid.UUID // tenant-level export
	entryGone  uuid.UUID // scenario event on a deleted scenario
}

func newAuditFixture(t *testing.T) *auditFixture {
	t.Helper()
	f := &auditFixture{
		repo:      &fakeAuditRepo{entries: map[uuid.UUID]*model.AuditLog{}},
		capturer:  &fakeCapturer{},
		tenant:    uuid.New(),
		user:      uuid.New(),
		planMine:  uuid.New(),
		planOther: uuid.New(),
		scenMine:  uuid.New(),
		scenOther: uuid.New(),
	}
	scenarios := fakeScenarioLookup{
		f.scenMine:  {TenantScoped: model.TenantScoped{ID: f.scenMine, TenantID: f.tenant}, PlanID: f.planMine},
		f.scenOther: {TenantScoped: model.TenantScoped{ID: f.scenOther, TenantID: f.tenant}, PlanID: f.planOther},
	}
	add := func(entityType string, entityID uuid.UUID) uuid.UUID {
		e := &model.AuditLog{ID: uuid.New(), TenantID: f.tenant, UserID: uuid.New(), EntityType: entityType, EntityID: entityID, Action: "update"}
		f.repo.entries[e.ID] = e
		return e.ID
	}
	f.entryMine = add("product", f.scenMine)
	f.entryOther = add("product", f.scenOther)
	f.entryPlan = add("plan", f.planOther)
	f.entryExp = add("audit_export", f.tenant)
	f.entryGone = add("product", uuid.New())

	access := &fakeAccess{
		allowed:  map[uuid.UUID]bool{f.planMine: true},
		entities: []uuid.UUID{f.tenant, f.planMine, f.scenMine},
	}
	logger := logrus.NewEntry(logrus.New())
	logger.Logger.SetLevel(logrus.PanicLevel)
	f.h = NewAuditHandler(f.repo, f.capturer, access, scenarios, logger)
	return f
}

func (f *auditFixture) detail(t *testing.T, role string, entryID uuid.UUID) (*httptest.ResponseRecorder, map[string]json.RawMessage) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/"+entryID.String()+"/detail", nil)
	ctx := ctxutil.WithTenantID(req.Context(), f.tenant)
	ctx = ctxutil.WithUserID(ctx, f.user)
	ctx = ctxutil.WithUserRole(ctx, role)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("entryId", entryID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	w := httptest.NewRecorder()
	f.h.GetDetail(w, req.WithContext(ctx))

	var body map[string]json.RawMessage
	if w.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	}
	return w, body
}

func (f *auditFixture) list(t *testing.T, role string) (*httptest.ResponseRecorder, int) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit?page=0&limit=50", nil)
	ctx := ctxutil.WithTenantID(req.Context(), f.tenant)
	ctx = ctxutil.WithUserID(ctx, f.user)
	ctx = ctxutil.WithUserRole(ctx, role)
	w := httptest.NewRecorder()
	f.h.List(w, req.WithContext(ctx))

	var body struct {
		Data  []json.RawMessage `json:"data"`
		Total int               `json:"total"`
	}
	if w.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	}
	return w, body.Total
}

// ── GetDetail ─────────────────────────────────────────────────────────────────

func TestAuditDetail_MemberSeesOwnScenarioWithSnapshot(t *testing.T) {
	f := newAuditFixture(t)
	w, body := f.detail(t, "editor", f.entryMine)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, string(body["scenarioSnapshot"]), "secret")
	assert.Equal(t, []uuid.UUID{f.scenMine}, f.capturer.captured)
}

func TestAuditDetail_ForeignScenarioIsNotFoundAndNotCaptured(t *testing.T) {
	// Regression for S-H2: any tenant user could dump any scenario through the
	// audit detail endpoint.
	f := newAuditFixture(t)
	w, _ := f.detail(t, "editor", f.entryOther)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Empty(t, f.capturer.captured, "scenario data must not be captured for an inaccessible plan")
}

func TestAuditDetail_OwnerSeesEverything(t *testing.T) {
	f := newAuditFixture(t)
	w, body := f.detail(t, "owner", f.entryOther)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, string(body["scenarioSnapshot"]), "secret")
}

func TestAuditDetail_PlanEventCheckedAgainstPlan(t *testing.T) {
	f := newAuditFixture(t)
	w, _ := f.detail(t, "editor", f.entryPlan) // plan event on planOther
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Empty(t, f.capturer.captured)

	w, body := f.detail(t, "owner", f.entryPlan)
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{}`, string(body["scenarioSnapshot"]), "plan events carry no scenario snapshot")
}

func TestAuditDetail_TenantLevelExportVisibleToAll(t *testing.T) {
	f := newAuditFixture(t)
	w, body := f.detail(t, "editor", f.entryExp)
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{}`, string(body["scenarioSnapshot"]))
	assert.Empty(t, f.capturer.captured)
}

func TestAuditDetail_DeletedScenarioOwnerOnly(t *testing.T) {
	f := newAuditFixture(t)
	w, _ := f.detail(t, "editor", f.entryGone)
	assert.Equal(t, http.StatusNotFound, w.Code)

	w, body := f.detail(t, "owner", f.entryGone)
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{}`, string(body["scenarioSnapshot"]))
	assert.Empty(t, f.capturer.captured, "nothing to capture for a deleted scenario")
}

func TestAuditDetail_AdminHasNoAccess(t *testing.T) {
	f := newAuditFixture(t)
	w, _ := f.detail(t, "admin", f.entryExp)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAuditDetail_UnknownEntry(t *testing.T) {
	f := newAuditFixture(t)
	w, _ := f.detail(t, "owner", uuid.New())
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── List ──────────────────────────────────────────────────────────────────────

func TestAuditList_OwnerSeesWholeTenant(t *testing.T) {
	f := newAuditFixture(t)
	w, total := f.list(t, "owner")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 5, total)
	assert.True(t, f.repo.listedAll)
	assert.Nil(t, f.repo.listedEntity)
}

func TestAuditList_MemberSeesOnlyAccessibleEntities(t *testing.T) {
	f := newAuditFixture(t)
	w, total := f.list(t, "editor")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 2, total, "own scenario event + tenant-level export")
	assert.False(t, f.repo.listedAll, "must not use the unfiltered query")
	assert.ElementsMatch(t, []uuid.UUID{f.tenant, f.planMine, f.scenMine}, f.repo.listedEntity)
}
