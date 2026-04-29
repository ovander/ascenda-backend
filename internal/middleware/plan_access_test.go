package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/ctxutil"
)

// ── Mock PlanRepo ─────────────────────────────────────────────────

type mockPlanRepo struct {
	plans map[string]*model.BusinessPlan
}

func newMockPlanRepo() *mockPlanRepo {
	return &mockPlanRepo{plans: make(map[string]*model.BusinessPlan)}
}

func (m *mockPlanRepo) add(plan *model.BusinessPlan) {
	m.plans[plan.ID.String()] = plan
}

func (m *mockPlanRepo) Create(plan *model.BusinessPlan) error { return nil }

func (m *mockPlanRepo) GetByID(tenantID, planID uuid.UUID) (*model.BusinessPlan, error) {
	return m.plans[planID.String()], nil
}

func (m *mockPlanRepo) ListByTenant(tenantID uuid.UUID, page, limit int) ([]*model.BusinessPlan, error) {
	return nil, nil
}

func (m *mockPlanRepo) Update(plan *model.BusinessPlan) error { return nil }

func (m *mockPlanRepo) Delete(tenantID, planID uuid.UUID) error { return nil }

func (m *mockPlanRepo) PurgeDemoPlans(tenantID uuid.UUID) error { return nil }

func (m *mockPlanRepo) CountByTenant(tenantID uuid.UUID) (int64, error) { return 0, nil }

// ── Mock PlanMemberRepo ───────────────────────────────────────────

type mockPlanMemberRepo struct {
	members map[string]*model.PlanMember
}

func newMockPlanMemberRepo() *mockPlanMemberRepo {
	return &mockPlanMemberRepo{members: make(map[string]*model.PlanMember)}
}

func (m *mockPlanMemberRepo) Create(member *model.PlanMember) error {
	key := member.PlanID.String() + ":" + member.UserID.String()
	m.members[key] = member
	return nil
}

func (m *mockPlanMemberRepo) GetByPlanAndUser(tenantID, planID, userID uuid.UUID) (*model.PlanMember, error) {
	key := planID.String() + ":" + userID.String()
	member := m.members[key]
	if member == nil {
		return nil, nil
	}
	return member, nil
}

func (m *mockPlanMemberRepo) ListByPlan(tenantID, planID uuid.UUID) ([]*model.PlanMember, error) {
	return nil, nil
}

func (m *mockPlanMemberRepo) ListByUser(tenantID, userID uuid.UUID) ([]*model.PlanMember, error) {
	return nil, nil
}

func (m *mockPlanMemberRepo) Update(member *model.PlanMember) error { return nil }

func (m *mockPlanMemberRepo) Delete(tenantID, planID, userID uuid.UUID) error { return nil }

// ── Helper to build request with chi params ───────────────────────

func newPlanRequest(role string, tenantID, userID, planID uuid.UUID) *http.Request {
	req := httptest.NewRequest("GET", "/plans/"+planID.String(), nil)
	ctx := req.Context()
	ctx = ctxutil.WithUserRole(ctx, role)
	ctx = ctxutil.WithTenantID(ctx, tenantID)
	ctx = ctxutil.WithUserID(ctx, userID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("planId", planID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

	return req.WithContext(ctx)
}

// ── Tests ─────────────────────────────────────────────────────────

func TestPlanAccessOwnerBypass(t *testing.T) {
	planRepo := newMockPlanRepo()
	memberRepo := newMockPlanMemberRepo()
	mw := NewPlanAccessMiddleware(planRepo, memberRepo, logrus.NewEntry(logrus.New()))

	tenantID := uuid.New()
	userID := uuid.New()
	planID := uuid.New()
	// No plan_members entry — owner bypasses

	nextCalled := false
	handler := mw.RequirePlanAccess(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := newPlanRequest("owner", tenantID, userID, planID)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPlanAccessAdminDenied(t *testing.T) {
	planRepo := newMockPlanRepo()
	memberRepo := newMockPlanMemberRepo()
	mw := NewPlanAccessMiddleware(planRepo, memberRepo, logrus.NewEntry(logrus.New()))

	tenantID := uuid.New()
	userID := uuid.New()
	planID := uuid.New()

	handler := mw.RequirePlanAccess(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := newPlanRequest("admin", tenantID, userID, planID)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestPlanAccessUserWithMembership(t *testing.T) {
	planRepo := newMockPlanRepo()
	memberRepo := newMockPlanMemberRepo()
	mw := NewPlanAccessMiddleware(planRepo, memberRepo, logrus.NewEntry(logrus.New()))

	tenantID := uuid.New()
	userID := uuid.New()
	planID := uuid.New()

	// Grant editor access
	memberRepo.Create(&model.PlanMember{
		TenantID: tenantID,
		PlanID:   planID,
		UserID:   userID,
		Role:     "editor",
	})

	var capturedRole string
	handler := mw.RequirePlanAccess(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRole = ctxutil.GetUserRole(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := newPlanRequest("user", tenantID, userID, planID)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "editor", capturedRole, "plan-level role should be injected into context")
}

func TestPlanAccessUserWithViewerRole(t *testing.T) {
	planRepo := newMockPlanRepo()
	memberRepo := newMockPlanMemberRepo()
	mw := NewPlanAccessMiddleware(planRepo, memberRepo, logrus.NewEntry(logrus.New()))

	tenantID := uuid.New()
	userID := uuid.New()
	planID := uuid.New()

	memberRepo.Create(&model.PlanMember{
		TenantID: tenantID,
		PlanID:   planID,
		UserID:   userID,
		Role:     "viewer",
	})

	var capturedRole string
	handler := mw.RequirePlanAccess(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRole = ctxutil.GetUserRole(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := newPlanRequest("user", tenantID, userID, planID)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "viewer", capturedRole)
}

func TestPlanAccessUserWithoutMembership(t *testing.T) {
	planRepo := newMockPlanRepo()
	memberRepo := newMockPlanMemberRepo()
	mw := NewPlanAccessMiddleware(planRepo, memberRepo, logrus.NewEntry(logrus.New()))

	tenantID := uuid.New()
	userID := uuid.New()
	planID := uuid.New()
	// No membership

	handler := mw.RequirePlanAccess(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := newPlanRequest("user", tenantID, userID, planID)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestPlanAccessDemoPlanBypassesForAnyRole(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	planID := uuid.New()

	planRepo := newMockPlanRepo()
	planRepo.add(&model.BusinessPlan{
		TenantScoped: model.TenantScoped{ID: planID, TenantID: tenantID},
		IsDemo:       true,
	})
	memberRepo := newMockPlanMemberRepo()
	// No membership entry — demo plans are open to all authenticated users
	mw := NewPlanAccessMiddleware(planRepo, memberRepo, logrus.NewEntry(logrus.New()))

	for _, role := range []string{"editor", "reader", "user"} {
		t.Run("demo plan accessible by "+role, func(t *testing.T) {
			nextCalled := false
			handler := mw.RequirePlanAccess(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			}))

			req := newPlanRequest(role, tenantID, userID, planID)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.True(t, nextCalled, "demo plan should bypass membership check for role: "+role)
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestPlanEditOwnerAllowed(t *testing.T) {
	planRepo := newMockPlanRepo()
	memberRepo := newMockPlanMemberRepo()
	mw := NewPlanAccessMiddleware(planRepo, memberRepo, logrus.NewEntry(logrus.New()))

	handler := mw.RequirePlanEdit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("PUT", "/plans/x", nil)
	ctx := ctxutil.WithUserRole(req.Context(), "owner")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPlanEditEditorAllowed(t *testing.T) {
	planRepo := newMockPlanRepo()
	memberRepo := newMockPlanMemberRepo()
	mw := NewPlanAccessMiddleware(planRepo, memberRepo, logrus.NewEntry(logrus.New()))

	handler := mw.RequirePlanEdit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("PUT", "/plans/x", nil)
	ctx := ctxutil.WithUserRole(req.Context(), "editor")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPlanEditViewerDenied(t *testing.T) {
	planRepo := newMockPlanRepo()
	memberRepo := newMockPlanMemberRepo()
	mw := NewPlanAccessMiddleware(planRepo, memberRepo, logrus.NewEntry(logrus.New()))

	handler := mw.RequirePlanEdit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := httptest.NewRequest("PUT", "/plans/x", nil)
	ctx := ctxutil.WithUserRole(req.Context(), "viewer")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestPlanAccessNoPlanIdPassesThrough(t *testing.T) {
	planRepo := newMockPlanRepo()
	memberRepo := newMockPlanMemberRepo()
	mw := NewPlanAccessMiddleware(planRepo, memberRepo, logrus.NewEntry(logrus.New()))

	nextCalled := false
	handler := mw.RequirePlanAccess(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	// Request without planId param (e.g., GET /plans/ list endpoint)
	req := httptest.NewRequest("GET", "/plans/", nil)
	ctx := ctxutil.WithUserRole(req.Context(), "user")
	ctx = ctxutil.WithTenantID(ctx, uuid.New())
	ctx = ctxutil.WithUserID(ctx, uuid.New())
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, nextCalled, "list endpoint should pass through without plan ID check")
}
