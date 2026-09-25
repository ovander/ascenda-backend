package service

import (
	"errors"
	"testing"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Minimal fakes ─────────────────────────────────────────────────────────────

type fakePlanRepoForAccess struct {
	plans map[uuid.UUID]*model.BusinessPlan
}

func (f *fakePlanRepoForAccess) Create(p *model.BusinessPlan) error { f.plans[p.ID] = p; return nil }
func (f *fakePlanRepoForAccess) GetByID(tenantID, planID uuid.UUID) (*model.BusinessPlan, error) {
	if p, ok := f.plans[planID]; ok && p.TenantID == tenantID {
		return p, nil
	}
	return nil, errors.New("not found")
}
func (f *fakePlanRepoForAccess) ListByTenant(tenantID uuid.UUID, offset, limit int) ([]*model.BusinessPlan, error) {
	var out []*model.BusinessPlan
	for _, p := range f.plans {
		if p.TenantID == tenantID {
			out = append(out, p)
		}
	}
	return out, nil
}
func (f *fakePlanRepoForAccess) CountByTenant(tenantID uuid.UUID) (int64, error) { return 0, nil }
func (f *fakePlanRepoForAccess) Update(p *model.BusinessPlan) error              { return nil }
func (f *fakePlanRepoForAccess) Delete(tenantID, planID uuid.UUID) error         { return nil }
func (f *fakePlanRepoForAccess) PurgeDemoPlans(tenantID uuid.UUID) error         { return nil }

type fakeMemberRepoForAccess struct {
	members []*model.PlanMember
}

func (f *fakeMemberRepoForAccess) Create(m *model.PlanMember) error {
	f.members = append(f.members, m)
	return nil
}
func (f *fakeMemberRepoForAccess) GetByPlanAndUser(tenantID, planID, userID uuid.UUID) (*model.PlanMember, error) {
	for _, m := range f.members {
		if m.TenantID == tenantID && m.PlanID == planID && m.UserID == userID {
			return m, nil
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeMemberRepoForAccess) ListByPlan(tenantID, planID uuid.UUID) ([]*model.PlanMember, error) {
	return nil, nil
}
func (f *fakeMemberRepoForAccess) ListByUser(tenantID, userID uuid.UUID) ([]*model.PlanMember, error) {
	var out []*model.PlanMember
	for _, m := range f.members {
		if m.TenantID == tenantID && m.UserID == userID {
			out = append(out, m)
		}
	}
	return out, nil
}
func (f *fakeMemberRepoForAccess) Update(m *model.PlanMember) error                { return nil }
func (f *fakeMemberRepoForAccess) Delete(tenantID, planID, userID uuid.UUID) error { return nil }

type accessFixture struct {
	resolver *PlanAccessResolver
	tenant   uuid.UUID
	user     uuid.UUID
	planMine uuid.UUID // user is editor
	planView uuid.UUID // user is viewer
	planDemo uuid.UUID // demo, no membership
	planNone uuid.UUID // no access
	scenMine uuid.UUID
	scenView uuid.UUID
	scenDemo uuid.UUID
	scenNone uuid.UUID
}

func newAccessFixture(t *testing.T) *accessFixture {
	t.Helper()
	f := &accessFixture{tenant: uuid.New(), user: uuid.New()}
	plans := &fakePlanRepoForAccess{plans: map[uuid.UUID]*model.BusinessPlan{}}
	members := &fakeMemberRepoForAccess{}
	scenarios := newInMemScenarioRepo()

	mkPlan := func(demo bool) uuid.UUID {
		p := &model.BusinessPlan{TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: f.tenant}, IsDemo: demo}
		plans.plans[p.ID] = p
		return p.ID
	}
	mkScen := func(planID uuid.UUID) uuid.UUID {
		s := &model.Scenario{TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: f.tenant}, PlanID: planID}
		require.NoError(t, scenarios.Create(s))
		return s.ID
	}
	f.planMine, f.planView, f.planDemo, f.planNone = mkPlan(false), mkPlan(false), mkPlan(true), mkPlan(false)
	f.scenMine, f.scenView, f.scenDemo, f.scenNone = mkScen(f.planMine), mkScen(f.planView), mkScen(f.planDemo), mkScen(f.planNone)
	members.members = []*model.PlanMember{
		{TenantID: f.tenant, PlanID: f.planMine, UserID: f.user, Role: "editor"},
		{TenantID: f.tenant, PlanID: f.planView, UserID: f.user, Role: "viewer"},
		{TenantID: f.tenant, PlanID: f.planNone, UserID: uuid.New(), Role: "editor"}, // someone else
	}
	f.resolver = NewPlanAccessResolver(plans, members, scenarios)
	return f
}

// ── PlanRole ──────────────────────────────────────────────────────────────────

func TestPlanAccessResolver_PlanRole(t *testing.T) {
	f := newAccessFixture(t)

	cases := []struct {
		name       string
		tenantRole string
		plan       uuid.UUID
		wantRole   string
		wantOK     bool
	}{
		{"owner on any plan", "owner", f.planNone, "owner", true},
		{"admin has no plan access", "admin", f.planMine, "", false},
		{"editor membership", "editor", f.planMine, "editor", true},
		{"viewer membership", "editor", f.planView, "viewer", true},
		{"demo plan without membership", "editor", f.planDemo, "editor", true},
		{"demo plan for reader", "reader", f.planDemo, "viewer", true},
		{"no membership", "editor", f.planNone, "", false},
		{"unknown plan", "editor", uuid.New(), "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			role, ok := f.resolver.PlanRole(f.tenant, f.user, tc.tenantRole, tc.plan)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantRole, role)
		})
	}

	t.Run("other tenant sees nothing", func(t *testing.T) {
		_, ok := f.resolver.PlanRole(uuid.New(), f.user, "editor", f.planMine)
		assert.False(t, ok)
	})
}

// ── AccessibleEntityIDs ───────────────────────────────────────────────────────

func TestPlanAccessResolver_AccessibleEntityIDs(t *testing.T) {
	f := newAccessFixture(t)

	t.Run("owner sees everything", func(t *testing.T) {
		ids, all, err := f.resolver.AccessibleEntityIDs(f.tenant, f.user, "owner")
		require.NoError(t, err)
		assert.True(t, all)
		assert.Nil(t, ids)
	})

	t.Run("admin sees nothing", func(t *testing.T) {
		ids, all, err := f.resolver.AccessibleEntityIDs(f.tenant, f.user, "admin")
		require.NoError(t, err)
		assert.False(t, all)
		assert.Empty(t, ids)
	})

	t.Run("member sees own plans, demo plans, their scenarios and the tenant", func(t *testing.T) {
		ids, all, err := f.resolver.AccessibleEntityIDs(f.tenant, f.user, "editor")
		require.NoError(t, err)
		assert.False(t, all)
		assert.ElementsMatch(t, []uuid.UUID{
			f.tenant,
			f.planMine, f.scenMine,
			f.planView, f.scenView,
			f.planDemo, f.scenDemo,
		}, ids)
		assert.NotContains(t, ids, f.planNone)
		assert.NotContains(t, ids, f.scenNone)
	})

	t.Run("user without memberships still sees demo plans and the tenant", func(t *testing.T) {
		ids, _, err := f.resolver.AccessibleEntityIDs(f.tenant, uuid.New(), "editor")
		require.NoError(t, err)
		assert.ElementsMatch(t, []uuid.UUID{f.tenant, f.planDemo, f.scenDemo}, ids)
	})

	t.Run("scenario repo required", func(t *testing.T) {
		r := NewPlanAccessResolver(&fakePlanRepoForAccess{plans: map[uuid.UUID]*model.BusinessPlan{}}, &fakeMemberRepoForAccess{}, nil)
		_, _, err := r.AccessibleEntityIDs(f.tenant, f.user, "editor")
		assert.Error(t, err)
	})
}
