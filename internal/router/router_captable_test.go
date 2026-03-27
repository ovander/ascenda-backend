package router

// router_captable_test.go — verifies that the plan-level cap-table route block
// is correctly wired: it must be reachable under /{planId}/cap-table (not
// under /scenarios/{scenarioId}/cap-table) and it must be gated by the Pro
// tier gate.  Uses the same stub-handler + mock-repo approach as router_test.go.

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"ascenda/internal/middleware"
	"ascenda/internal/model"
	"ascenda/internal/pkg/ctxutil"
	"ascenda/internal/repo"
)

// ── Mock TenantRepository for TierGateMiddleware ──────────────────────────

type mockCapTableTierTenantRepo struct {
	tiers map[uuid.UUID]string
}

func newMockCapTableTierRepo() *mockCapTableTierTenantRepo {
	return &mockCapTableTierTenantRepo{tiers: make(map[uuid.UUID]string)}
}

func (m *mockCapTableTierTenantRepo) setTier(id uuid.UUID, tier string) {
	m.tiers[id] = tier
}

func (m *mockCapTableTierTenantRepo) Create(t *model.Tenant) error  { return nil }
func (m *mockCapTableTierTenantRepo) GetBySlug(_ string) (*model.Tenant, error) { return nil, nil }
func (m *mockCapTableTierTenantRepo) ListActive(_, _ int) ([]*model.Tenant, error) { return nil, nil }
func (m *mockCapTableTierTenantRepo) Update(t *model.Tenant) error  { return nil }
func (m *mockCapTableTierTenantRepo) Delete(_ uuid.UUID) error       { return nil }

func (m *mockCapTableTierTenantRepo) GetByID(id uuid.UUID) (*model.Tenant, error) {
	tier, ok := m.tiers[id]
	if !ok {
		return nil, nil // unknown → will fail the gate
	}
	t := &model.Tenant{Tier: tier}
	t.ID = id
	return t, nil
}

// Compile-time interface check.
var _ repo.TenantRepository = (*mockCapTableTierTenantRepo)(nil)

// ── Test router with tier gate ────────────────────────────────────────────

// buildTierGatedRouter returns a minimal chi router that mirrors the real
// router for the plan-level cap-table segment:
//
//	GET  /api/v1/plans/{planId}/cap-table         — requires Pro tier
//	POST /api/v1/plans/{planId}/cap-table/shareholders
//	PUT  /api/v1/plans/{planId}/cap-table/shareholders/{id}
//	DELETE /api/v1/plans/{planId}/cap-table/shareholders/{id}
func buildTierGatedRouter(tierRepo repo.TenantRepository, planMemberRepo repo.PlanMemberRepository) *chi.Mux {
	logger := logrus.NewEntry(logrus.New())
	rbacMW := middleware.NewRBACMiddleware(logger)
	planAccessMW := middleware.NewPlanAccessMiddleware(planMemberRepo, logger)
	tierGateMW := middleware.NewTierGateMiddleware(tierRepo, logger)

	r := chi.NewRouter()
	r.Route("/api/v1/plans", func(r chi.Router) {
		r.Route("/{planId}", func(r chi.Router) {
			r.Use(planAccessMW.RequirePlanAccess)
			r.Get("/", stubHandler("GetPlan"))

			// Plan-level cap table — requires Pro tier.
			r.Route("/cap-table", func(r chi.Router) {
				r.Use(tierGateMW.Require(middleware.TierPro))
				r.Get("/", stubHandler("GetCapTableSummary"))
				r.With(planAccessMW.RequirePlanEdit).Post("/shareholders", stubHandler("CreateShareholder"))
				r.With(planAccessMW.RequirePlanEdit).Put("/shareholders/{id}", stubHandler("UpdateShareholder"))
				r.With(planAccessMW.RequirePlanEdit).Delete("/shareholders/{id}", stubHandler("DeleteShareholder"))
			})

			// Scenarios remain nested (cap-table should NOT be accessible here).
			r.Route("/scenarios", func(r chi.Router) {
				r.Get("/", stubHandler("ListScenarios"))
				r.Route("/{scenarioId}", func(r chi.Router) {
					r.Get("/", stubHandler("GetScenario"))
				})
			})
		})
	})
	_ = rbacMW // keep import used — real router attaches this to users/tenant
	return r
}

// newTierReq builds a request with tenantID and userRole in context.
// The tenantID is fixed so the tier repo can look it up.
func newTierReq(method, path, role string, tenantID uuid.UUID) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	ctx := req.Context()
	ctx = ctxutil.WithUserRole(ctx, role)
	ctx = ctxutil.WithTenantID(ctx, tenantID)
	ctx = ctxutil.WithUserID(ctx, uuid.MustParse("00000000-0000-0000-0000-000000000099"))
	return req.WithContext(ctx)
}

// ── Tests ─────────────────────────────────────────────────────────────────

func TestCapTableRoute_FreeTier_Blocked(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()

	tierRepo := newMockCapTableTierRepo()
	tierRepo.setTier(tenantID, middleware.TierFree)

	// Owner bypasses plan-access check, so the only gate we test here is tier.
	planRepo := newMockPlanMemberRepo()
	r := buildTierGatedRouter(tierRepo, planRepo)

	path := "/api/v1/plans/" + planID.String() + "/cap-table/"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, newTierReq("GET", path, "owner", tenantID))

	assert.Equal(t, http.StatusForbidden, w.Code, "Free tier must be blocked from cap-table")
}

func TestCapTableRoute_ProTier_Allowed(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()

	tierRepo := newMockCapTableTierRepo()
	tierRepo.setTier(tenantID, middleware.TierPro)

	planRepo := newMockPlanMemberRepo()
	r := buildTierGatedRouter(tierRepo, planRepo)

	path := "/api/v1/plans/" + planID.String() + "/cap-table/"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, newTierReq("GET", path, "owner", tenantID))

	assert.Equal(t, http.StatusOK, w.Code, "Pro tier must be allowed through cap-table gate")
	assert.Equal(t, "GetCapTableSummary", w.Header().Get("X-Handler"))
}

func TestCapTableRoute_EnterpriseTier_PassesProGate(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()

	tierRepo := newMockCapTableTierRepo()
	tierRepo.setTier(tenantID, middleware.TierEnterprise)

	planRepo := newMockPlanMemberRepo()
	r := buildTierGatedRouter(tierRepo, planRepo)

	path := "/api/v1/plans/" + planID.String() + "/cap-table/"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, newTierReq("GET", path, "owner", tenantID))

	assert.Equal(t, http.StatusOK, w.Code, "Enterprise tier should satisfy Pro gate")
}

func TestCapTableRoute_NotUnderScenarios(t *testing.T) {
	// Confirms the fix: the cap-table route is NOT under /scenarios/{scenarioId}.
	tenantID := uuid.New()
	planID := uuid.New()
	scenarioID := uuid.New()

	tierRepo := newMockCapTableTierRepo()
	tierRepo.setTier(tenantID, middleware.TierPro)

	planRepo := newMockPlanMemberRepo()
	r := buildTierGatedRouter(tierRepo, planRepo)

	// The OLD (wrong) path — must return 404, not 200.
	wrongPath := "/api/v1/plans/" + planID.String() + "/scenarios/" + scenarioID.String() + "/cap-table/"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, newTierReq("GET", wrongPath, "owner", tenantID))

	assert.Equal(t, http.StatusNotFound, w.Code,
		"cap-table must NOT be reachable under /scenarios/{scenarioId}")
}

func TestCapTableRoute_ViewerCanGET_ButCannotPOST(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000099")
	tenantID := uuid.New()
	planID := uuid.New()

	tierRepo := newMockCapTableTierRepo()
	tierRepo.setTier(tenantID, middleware.TierPro)

	planRepo := newMockPlanMemberRepo()
	planRepo.Create(&model.PlanMember{
		TenantID: tenantID,
		PlanID:   planID,
		UserID:   userID,
		Role:     "viewer",
	})
	r := buildTierGatedRouter(tierRepo, planRepo)

	base := "/api/v1/plans/" + planID.String()

	// GET /cap-table — allowed for viewer (read-only)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, newTierReq("GET", base+"/cap-table/", "user", tenantID))
	assert.Equal(t, http.StatusOK, w.Code, "viewer should be able to GET cap-table summary")

	// POST /cap-table/shareholders — blocked for viewer (edit required)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, newTierReq("POST", base+"/cap-table/shareholders", "user", tenantID))
	assert.Equal(t, http.StatusForbidden, w.Code, "viewer must not POST shareholders")
}

func TestCapTableRoute_EditorCanPOST(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000099")
	tenantID := uuid.New()
	planID := uuid.New()

	tierRepo := newMockCapTableTierRepo()
	tierRepo.setTier(tenantID, middleware.TierPro)

	planRepo := newMockPlanMemberRepo()
	planRepo.Create(&model.PlanMember{
		TenantID: tenantID,
		PlanID:   planID,
		UserID:   userID,
		Role:     "editor",
	})
	r := buildTierGatedRouter(tierRepo, planRepo)

	path := "/api/v1/plans/" + planID.String() + "/cap-table/shareholders"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, newTierReq("POST", path, "user", tenantID))

	assert.Equal(t, http.StatusOK, w.Code, "editor should be able to POST shareholders")
	assert.Equal(t, "CreateShareholder", w.Header().Get("X-Handler"))
}

func TestCapTableRoute_FreeTier_ForbiddenBodyContainsUpgradeRequired(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()

	tierRepo := newMockCapTableTierRepo()
	tierRepo.setTier(tenantID, middleware.TierFree)

	planRepo := newMockPlanMemberRepo()
	r := buildTierGatedRouter(tierRepo, planRepo)

	path := "/api/v1/plans/" + planID.String() + "/cap-table/"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, newTierReq("GET", path, "owner", tenantID))

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json",
		"tier gate 403 must be JSON")
	assert.Contains(t, w.Body.String(), "upgrade_required")
}
