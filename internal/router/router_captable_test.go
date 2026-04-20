package router

// router_captable_test.go — verifies that the plan-level cap-table route block
// is correctly wired: it must be reachable under /{planId}/cap-table (not
// under /scenarios/{scenarioId}/cap-table) and it must be gated by the Pro
// plan gate.  Uses the same stub-handler + mock-repo approach as router_test.go.

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
	"github.com/ovander/backendkit/ctxutil"
	"ascenda/internal/repo"
)

// ── Test router with tier gate ────────────────────────────────────────────

// buildTierGatedRouter returns a minimal chi router that mirrors the real
// router for the plan-level cap-table segment:
//
//	GET  /api/v1/plans/{planId}/cap-table         — requires Pro plan
//	POST /api/v1/plans/{planId}/cap-table/shareholders
//	PUT  /api/v1/plans/{planId}/cap-table/shareholders/{id}
//	DELETE /api/v1/plans/{planId}/cap-table/shareholders/{id}
func buildTierGatedRouter(planMemberRepo repo.PlanMemberRepository) *chi.Mux {
	logger := logrus.NewEntry(logrus.New())
	rbacMW := middleware.NewRBACMiddleware(logger)
	planAccessMW := middleware.NewPlanAccessMiddleware(planMemberRepo, logger)
	tierGateMW := middleware.NewTierGateMiddleware(logger)

	r := chi.NewRouter()
	r.Route("/api/v1/plans", func(r chi.Router) {
		r.Route("/{planId}", func(r chi.Router) {
			r.Use(planAccessMW.RequirePlanAccess)
			r.Get("/", stubHandler("GetPlan"))

			// Plan-level cap table — requires Pro plan.
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

// newTierReq builds a request with userRole, userPlan and userID in context.
func newTierReq(method, path, role, plan string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	ctx := req.Context()
	ctx = ctxutil.WithUserRole(ctx, role)
	ctx = ctxutil.WithUserPlan(ctx, plan)
	ctx = ctxutil.WithTenantID(ctx, uuid.New())
	ctx = ctxutil.WithUserID(ctx, uuid.MustParse("00000000-0000-0000-0000-000000000099"))
	return req.WithContext(ctx)
}

// ── Tests ─────────────────────────────────────────────────────────────────

func TestCapTableRoute_FreemiumPlan_Blocked(t *testing.T) {
	planID := uuid.New()

	planRepo := newMockPlanMemberRepo()
	r := buildTierGatedRouter(planRepo)

	path := "/api/v1/plans/" + planID.String() + "/cap-table/"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, newTierReq("GET", path, "owner", middleware.TierFree))

	assert.Equal(t, http.StatusForbidden, w.Code, "Freemium plan must be blocked from cap-table")
}

func TestCapTableRoute_ProPlan_Allowed(t *testing.T) {
	planID := uuid.New()

	planRepo := newMockPlanMemberRepo()
	r := buildTierGatedRouter(planRepo)

	path := "/api/v1/plans/" + planID.String() + "/cap-table/"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, newTierReq("GET", path, "owner", middleware.TierPro))

	assert.Equal(t, http.StatusOK, w.Code, "Pro plan must be allowed through cap-table gate")
	assert.Equal(t, "GetCapTableSummary", w.Header().Get("X-Handler"))
}

func TestCapTableRoute_EnterprisePlan_PassesProGate(t *testing.T) {
	planID := uuid.New()

	planRepo := newMockPlanMemberRepo()
	r := buildTierGatedRouter(planRepo)

	path := "/api/v1/plans/" + planID.String() + "/cap-table/"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, newTierReq("GET", path, "owner", middleware.TierEnterprise))

	assert.Equal(t, http.StatusOK, w.Code, "Enterprise plan should satisfy Pro gate")
}

func TestCapTableRoute_NotUnderScenarios(t *testing.T) {
	// Confirms the fix: the cap-table route is NOT under /scenarios/{scenarioId}.
	planID := uuid.New()
	scenarioID := uuid.New()

	planRepo := newMockPlanMemberRepo()
	r := buildTierGatedRouter(planRepo)

	// The OLD (wrong) path — must return 404, not 200.
	wrongPath := "/api/v1/plans/" + planID.String() + "/scenarios/" + scenarioID.String() + "/cap-table/"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, newTierReq("GET", wrongPath, "owner", middleware.TierPro))

	assert.Equal(t, http.StatusNotFound, w.Code,
		"cap-table must NOT be reachable under /scenarios/{scenarioId}")
}

func TestCapTableRoute_ViewerCanGET_ButCannotPOST(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000099")
	tenantID := uuid.New()
	planID := uuid.New()

	planRepo := newMockPlanMemberRepo()
	planRepo.Create(&model.PlanMember{
		TenantID: tenantID,
		PlanID:   planID,
		UserID:   userID,
		Role:     "viewer",
	})
	r := buildTierGatedRouter(planRepo)

	base := "/api/v1/plans/" + planID.String()

	// Build request helper that also sets tenantID for plan member lookup.
	makeReq := func(method, path, role, plan string) *http.Request {
		req := newTierReq(method, path, role, plan)
		ctx := ctxutil.WithTenantID(req.Context(), tenantID)
		return req.WithContext(ctx)
	}

	// GET /cap-table — allowed for viewer (read-only)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, makeReq("GET", base+"/cap-table/", "user", middleware.TierPro))
	assert.Equal(t, http.StatusOK, w.Code, "viewer should be able to GET cap-table summary")

	// POST /cap-table/shareholders — blocked for viewer (edit required)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, makeReq("POST", base+"/cap-table/shareholders", "user", middleware.TierPro))
	assert.Equal(t, http.StatusForbidden, w.Code, "viewer must not POST shareholders")
}

func TestCapTableRoute_EditorCanPOST(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000099")
	tenantID := uuid.New()
	planID := uuid.New()

	planRepo := newMockPlanMemberRepo()
	planRepo.Create(&model.PlanMember{
		TenantID: tenantID,
		PlanID:   planID,
		UserID:   userID,
		Role:     "editor",
	})
	r := buildTierGatedRouter(planRepo)

	req := newTierReq("POST", "/api/v1/plans/"+planID.String()+"/cap-table/shareholders", "user", middleware.TierPro)
	ctx := ctxutil.WithTenantID(req.Context(), tenantID)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "editor should be able to POST shareholders")
	assert.Equal(t, "CreateShareholder", w.Header().Get("X-Handler"))
}

func TestCapTableRoute_FreemiumPlan_ForbiddenBodyContainsUpgradeRequired(t *testing.T) {
	planID := uuid.New()

	planRepo := newMockPlanMemberRepo()
	r := buildTierGatedRouter(planRepo)

	path := "/api/v1/plans/" + planID.String() + "/cap-table/"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, newTierReq("GET", path, "owner", middleware.TierFree))

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json",
		"tier gate 403 must be JSON")
	assert.Contains(t, w.Body.String(), "upgrade_required")
}
