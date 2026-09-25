package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"ascenda/internal/middleware"
	"ascenda/internal/model"
	"ascenda/internal/repo"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// ── Stub handler that returns 200 with handler name ───────────────

func stubHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Handler", name)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"handler":"` + name + `"}`))
	}
}

// ── Mock PlanRepo ─────────────────────────────────────────────────

type mockPlanRepo struct {
	plans map[string]*model.BusinessPlan
}

func newMockPlanRepo() *mockPlanRepo {
	return &mockPlanRepo{plans: make(map[string]*model.BusinessPlan)}
}

func (m *mockPlanRepo) Create(plan *model.BusinessPlan) error { return nil }
func (m *mockPlanRepo) GetByID(tenantID, planID uuid.UUID) (*model.BusinessPlan, error) {
	return m.plans[planID.String()], nil
}
func (m *mockPlanRepo) ListByTenant(tenantID uuid.UUID, page, limit int) ([]*model.BusinessPlan, error) {
	return nil, nil
}
func (m *mockPlanRepo) Update(plan *model.BusinessPlan) error           { return nil }
func (m *mockPlanRepo) Delete(tenantID, planID uuid.UUID) error         { return nil }
func (m *mockPlanRepo) PurgeDemoPlans(tenantID uuid.UUID) error         { return nil }
func (m *mockPlanRepo) CountByTenant(tenantID uuid.UUID) (int64, error) { return 0, nil }

var _ repo.PlanRepository = (*mockPlanRepo)(nil)

// ── Mock PlanMemberRepo ───────────────────────────────────────────

// mockScenarioRepo satisfies repo.ScenarioRepository. Scenarios added via add()
// are returned by GetByID; RequireScenarioInPlan uses it to bind {scenarioId}
// to {planId}.
type mockScenarioRepo struct {
	scenarios map[uuid.UUID]*model.Scenario
}

func newMockScenarioRepo() *mockScenarioRepo {
	return &mockScenarioRepo{scenarios: make(map[uuid.UUID]*model.Scenario)}
}

func (m *mockScenarioRepo) add(s *model.Scenario) { m.scenarios[s.ID] = s }

func (m *mockScenarioRepo) Create(s *model.Scenario) error { m.scenarios[s.ID] = s; return nil }
func (m *mockScenarioRepo) GetByID(tenantID, scenarioID uuid.UUID) (*model.Scenario, error) {
	if s, ok := m.scenarios[scenarioID]; ok && s.TenantID == tenantID {
		return s, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockScenarioRepo) ListByPlan(tenantID, planID uuid.UUID) ([]*model.Scenario, error) {
	return nil, nil
}
func (m *mockScenarioRepo) Update(s *model.Scenario) error              { return nil }
func (m *mockScenarioRepo) Delete(tenantID, scenarioID uuid.UUID) error { return nil }

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
	return m.members[key], nil
}
func (m *mockPlanMemberRepo) ListByPlan(tenantID, planID uuid.UUID) ([]*model.PlanMember, error) {
	return nil, nil
}
func (m *mockPlanMemberRepo) ListByUser(tenantID, userID uuid.UUID) ([]*model.PlanMember, error) {
	return nil, nil
}
func (m *mockPlanMemberRepo) Update(member *model.PlanMember) error { return nil }
func (m *mockPlanMemberRepo) Delete(tenantID, planID, userID uuid.UUID) error {
	return nil
}

// Verify interface compliance
var _ repo.PlanMemberRepository = (*mockPlanMemberRepo)(nil)

// ── Passthrough auth middleware (skips JWT validation) ─────────────

func passthroughAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

// ── Passthrough tenant middleware (skips DB) ───────────────────────

func passthroughTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

// ── Build test router ─────────────────────────────────────────────

// defaultScenarios registers the scenario the nested-route tests use
// (22222222-… in plan 11111111-…, tenant 00000000-…-0001), mirroring what the
// real ScenarioRepo would return.
func defaultScenarios() *mockScenarioRepo {
	sr := newMockScenarioRepo()
	sr.add(&model.Scenario{
		TenantScoped: model.TenantScoped{
			ID:       uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			TenantID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		},
		PlanID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
	})
	return sr
}

// fakeProductLookup satisfies middleware.ProductLookup. Product 44444444-…
// belongs to the default scenario 22222222-…; product 55555555-… belongs to a
// scenario of another plan.
type fakeProductLookup struct{}

func (fakeProductLookup) GetByID(tenantID, productID uuid.UUID) (*model.Product, error) {
	tid := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	switch productID {
	case uuid.MustParse("44444444-4444-4444-4444-444444444444"):
		return &model.Product{TenantScoped: model.TenantScoped{ID: productID, TenantID: tid}, ScenarioID: uuid.MustParse("22222222-2222-2222-2222-222222222222")}, nil
	case uuid.MustParse("55555555-5555-5555-5555-555555555555"):
		return &model.Product{TenantScoped: model.TenantScoped{ID: productID, TenantID: tid}, ScenarioID: uuid.MustParse("33333333-3333-3333-3333-333333333333")}, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func buildTestRouter(planRepo repo.PlanRepository, planMemberRepo repo.PlanMemberRepository, scenarioRepo repo.ScenarioRepository) *chi.Mux {
	logger := logrus.NewEntry(logrus.New())
	scopeMW := middleware.NewResourceScopeMiddleware(fakeProductLookup{}, nil, nil, logger)

	// Create real RBAC and PlanAccess middleware
	rbacMW := middleware.NewRBACMiddleware(logger)
	planAccessMW := middleware.NewPlanAccessMiddleware(planRepo, planMemberRepo, scenarioRepo, logger)

	r := chi.NewRouter()

	// Simplified version of the real router — same route structure,
	// but with passthrough auth/tenant and stub handlers.
	r.Route("/api/v1", func(r chi.Router) {
		// Simulate auth + tenant middleware injecting role into context
		// (tests set role via request context)

		// User profile (no tenant required)
		r.Get("/users/me", stubHandler("GetMe"))
		r.Put("/users/me", stubHandler("UpdateMe"))

		// Tenant-required routes
		r.Group(func(r chi.Router) {
			// User management (requires manage:users)
			r.Route("/users", func(r chi.Router) {
				r.Use(rbacMW.RequirePermission(middleware.PermManageUsers))
				r.Get("/", stubHandler("ListUsers"))
				r.Post("/invite", stubHandler("InviteUser"))
			})

			// Tenant routes — GET open to all authenticated users, PUT requires manage:tenant
			r.Route("/tenant", func(r chi.Router) {
				r.Get("/", stubHandler("GetTenant"))
				r.With(rbacMW.RequirePermission(middleware.PermManageTenant)).Put("/", stubHandler("UpdateTenant"))
			})

			// Plan routes
			r.Route("/plans", func(r chi.Router) {
				r.Get("/", stubHandler("ListPlans"))
				r.With(rbacMW.RequirePermission(middleware.PermManagePlan)).Post("/", stubHandler("CreatePlan"))

				r.Route("/{planId}", func(r chi.Router) {
					r.Use(planAccessMW.RequirePlanAccess)
					r.Get("/", stubHandler("GetPlan"))
					r.With(planAccessMW.RequirePlanEdit).Put("/", stubHandler("UpdatePlan"))

					r.Route("/scenarios", func(r chi.Router) {
						r.Get("/", stubHandler("ListScenarios"))
						r.With(planAccessMW.RequirePlanEdit).Post("/", stubHandler("CreateScenario"))

						r.Route("/{scenarioId}", func(r chi.Router) {
							r.Use(planAccessMW.RequireScenarioInPlan)
							r.Get("/", stubHandler("GetScenario"))

							r.Route("/products", func(r chi.Router) {
								r.Get("/", stubHandler("ListProducts"))
								r.With(planAccessMW.RequirePlanEdit).Post("/", stubHandler("CreateProduct"))
								// Guarded single-product routes, as in the real router.
								r.Route("/{productId}", func(r chi.Router) {
									r.Use(scopeMW.RequireProductInScenario)
									r.Get("/", stubHandler("GetProduct"))
									r.With(planAccessMW.RequirePlanEdit).Put("/", stubHandler("UpdateProduct"))
								})
							})

							r.Route("/settings", func(r chi.Router) {
								r.Get("/config", stubHandler("GetConfig"))
								r.With(planAccessMW.RequirePlanEdit).Put("/config", stubHandler("UpdateConfig"))
							})
						})
					})
				})
			})
		})
	})

	return r
}

// ── Helper to create request with role in context ─────────────────

func newReq(method, path, role string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	ctx := req.Context()
	ctx = ctxutil.WithUserRole(ctx, role)
	ctx = ctxutil.WithTenantID(ctx, uuid.MustParse("00000000-0000-0000-0000-000000000001"))
	ctx = ctxutil.WithUserID(ctx, uuid.MustParse("00000000-0000-0000-0000-000000000099"))
	return req.WithContext(ctx)
}

// ── Tests ─────────────────────────────────────────────────────────

func TestRouterHealthAndAuth(t *testing.T) {
	r := buildTestRouter(newMockPlanRepo(), newMockPlanMemberRepo(), defaultScenarios())

	t.Run("GET /users/me returns 200 for any role", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", "/api/v1/users/me", "user"))
		assert.Equal(t, 200, w.Code)
		assert.Equal(t, "GetMe", w.Header().Get("X-Handler"))
	})
}

func TestRouterUserManagementRBAC(t *testing.T) {
	r := buildTestRouter(newMockPlanRepo(), newMockPlanMemberRepo(), defaultScenarios())

	t.Run("owner can list users", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", "/api/v1/users/", "owner"))
		assert.Equal(t, 200, w.Code)
	})

	t.Run("admin CANNOT list tenant users (no PermManageUsers)", func(t *testing.T) {
		// platform admin has PermPlatformAdmin only — tenant-scoped user list is owner-only
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", "/api/v1/users/", "admin"))
		assert.Equal(t, 403, w.Code)
	})

	t.Run("user CANNOT list users", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", "/api/v1/users/", "user"))
		assert.Equal(t, 403, w.Code)
	})
}

func TestRouterTenantRBAC(t *testing.T) {
	r := buildTestRouter(newMockPlanRepo(), newMockPlanMemberRepo(), defaultScenarios())

	// GET /tenant/ — readable by all authenticated users (needed for tier gating).
	for _, role := range []string{"owner", "editor", "reader", "admin", "user"} {
		role := role
		t.Run("GET tenant accessible by "+role, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, newReq("GET", "/api/v1/tenant/", role))
			assert.Equal(t, 200, w.Code)
		})
	}

	// PUT /tenant/ — requires manage:tenant (owner only).
	t.Run("owner can PUT tenant", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("PUT", "/api/v1/tenant/", "owner"))
		assert.Equal(t, 200, w.Code)
	})

	for _, role := range []string{"editor", "reader", "admin", "user"} {
		role := role
		t.Run(role+" CANNOT PUT tenant", func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, newReq("PUT", "/api/v1/tenant/", role))
			assert.Equal(t, 403, w.Code)
		})
	}
}

func TestRouterPlanListAccessible(t *testing.T) {
	r := buildTestRouter(newMockPlanRepo(), newMockPlanMemberRepo(), defaultScenarios())

	t.Run("any authenticated user can list plans", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", "/api/v1/plans/", "user"))
		assert.Equal(t, 200, w.Code)
	})

	t.Run("owner can list plans", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", "/api/v1/plans/", "owner"))
		assert.Equal(t, 200, w.Code)
	})
}

func TestRouterPlanCreateRBAC(t *testing.T) {
	r := buildTestRouter(newMockPlanRepo(), newMockPlanMemberRepo(), defaultScenarios())

	t.Run("owner can create plan", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("POST", "/api/v1/plans/", "owner"))
		assert.Equal(t, 200, w.Code)
	})

	t.Run("user CANNOT create plan (needs PermManagePlan)", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("POST", "/api/v1/plans/", "user"))
		assert.Equal(t, 403, w.Code)
	})

	t.Run("admin CANNOT create plan", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("POST", "/api/v1/plans/", "admin"))
		assert.Equal(t, 403, w.Code)
	})
}

func TestRouterPlanAccessMiddleware(t *testing.T) {
	planID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000099")
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	t.Run("owner bypasses plan access check", func(t *testing.T) {
		repo := newMockPlanMemberRepo()
		r := buildTestRouter(newMockPlanRepo(), repo, defaultScenarios())
		// No plan_member entry — owner bypasses

		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", "/api/v1/plans/"+planID.String()+"/", "owner"))
		assert.Equal(t, 200, w.Code)
		assert.Equal(t, "GetPlan", w.Header().Get("X-Handler"))
	})

	t.Run("admin blocked from plan access", func(t *testing.T) {
		repo := newMockPlanMemberRepo()
		r := buildTestRouter(newMockPlanRepo(), repo, defaultScenarios())

		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", "/api/v1/plans/"+planID.String()+"/", "admin"))
		assert.Equal(t, 403, w.Code)
	})

	t.Run("user with editor membership can access plan", func(t *testing.T) {
		repo := newMockPlanMemberRepo()
		repo.Create(&model.PlanMember{
			TenantID: tenantID,
			PlanID:   planID,
			UserID:   userID,
			Role:     "editor",
		})
		r := buildTestRouter(newMockPlanRepo(), repo, defaultScenarios())

		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", "/api/v1/plans/"+planID.String()+"/", "user"))
		assert.Equal(t, 200, w.Code)
	})

	t.Run("user without membership denied", func(t *testing.T) {
		repo := newMockPlanMemberRepo()
		r := buildTestRouter(newMockPlanRepo(), repo, defaultScenarios())

		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", "/api/v1/plans/"+planID.String()+"/", "user"))
		assert.Equal(t, 403, w.Code)
	})

	t.Run("user with viewer membership can GET plan", func(t *testing.T) {
		repo := newMockPlanMemberRepo()
		repo.Create(&model.PlanMember{
			TenantID: tenantID,
			PlanID:   planID,
			UserID:   userID,
			Role:     "viewer",
		})
		r := buildTestRouter(newMockPlanRepo(), repo, defaultScenarios())

		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", "/api/v1/plans/"+planID.String()+"/", "user"))
		assert.Equal(t, 200, w.Code)
	})

	t.Run("viewer CANNOT PUT plan (edit)", func(t *testing.T) {
		repo := newMockPlanMemberRepo()
		repo.Create(&model.PlanMember{
			TenantID: tenantID,
			PlanID:   planID,
			UserID:   userID,
			Role:     "viewer",
		})
		r := buildTestRouter(newMockPlanRepo(), repo, defaultScenarios())

		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("PUT", "/api/v1/plans/"+planID.String()+"/", "user"))
		assert.Equal(t, 403, w.Code)
	})

	t.Run("editor CAN PUT plan (edit)", func(t *testing.T) {
		repo := newMockPlanMemberRepo()
		repo.Create(&model.PlanMember{
			TenantID: tenantID,
			PlanID:   planID,
			UserID:   userID,
			Role:     "editor",
		})
		r := buildTestRouter(newMockPlanRepo(), repo, defaultScenarios())

		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("PUT", "/api/v1/plans/"+planID.String()+"/", "user"))
		assert.Equal(t, 200, w.Code)
	})
}

func TestRouterNestedScenarioRoutes(t *testing.T) {
	planID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	scenarioID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	t.Run("owner can list scenarios", func(t *testing.T) {
		repo := newMockPlanMemberRepo()
		r := buildTestRouter(newMockPlanRepo(), repo, defaultScenarios())

		path := "/api/v1/plans/" + planID.String() + "/scenarios/"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", path, "owner"))
		assert.Equal(t, 200, w.Code)
		assert.Equal(t, "ListScenarios", w.Header().Get("X-Handler"))
	})

	t.Run("owner can get scenario", func(t *testing.T) {
		repo := newMockPlanMemberRepo()
		r := buildTestRouter(newMockPlanRepo(), repo, defaultScenarios())

		path := "/api/v1/plans/" + planID.String() + "/scenarios/" + scenarioID.String() + "/"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", path, "owner"))
		assert.Equal(t, 200, w.Code)
		assert.Equal(t, "GetScenario", w.Header().Get("X-Handler"))
	})

	t.Run("owner can list products under scenario", func(t *testing.T) {
		repo := newMockPlanMemberRepo()
		r := buildTestRouter(newMockPlanRepo(), repo, defaultScenarios())

		path := "/api/v1/plans/" + planID.String() + "/scenarios/" + scenarioID.String() + "/products/"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", path, "owner"))
		assert.Equal(t, 200, w.Code)
		assert.Equal(t, "ListProducts", w.Header().Get("X-Handler"))
	})

	t.Run("owner can get settings config", func(t *testing.T) {
		repo := newMockPlanMemberRepo()
		r := buildTestRouter(newMockPlanRepo(), repo, defaultScenarios())

		path := "/api/v1/plans/" + planID.String() + "/scenarios/" + scenarioID.String() + "/settings/config"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", path, "owner"))
		assert.Equal(t, 200, w.Code)
		assert.Equal(t, "GetConfig", w.Header().Get("X-Handler"))
	})

	t.Run("viewer can GET settings but not PUT", func(t *testing.T) {
		userID := uuid.MustParse("00000000-0000-0000-0000-000000000099")
		tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

		repo := newMockPlanMemberRepo()
		repo.Create(&model.PlanMember{
			TenantID: tenantID, PlanID: planID, UserID: userID, Role: "viewer",
		})
		r := buildTestRouter(newMockPlanRepo(), repo, defaultScenarios())

		basePath := "/api/v1/plans/" + planID.String() + "/scenarios/" + scenarioID.String()

		// GET works
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", basePath+"/settings/config", "user"))
		assert.Equal(t, 200, w.Code)

		// PUT blocked
		w = httptest.NewRecorder()
		r.ServeHTTP(w, newReq("PUT", basePath+"/settings/config", "user"))
		assert.Equal(t, 403, w.Code)
	})

	t.Run("editor can POST product", func(t *testing.T) {
		userID := uuid.MustParse("00000000-0000-0000-0000-000000000099")
		tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

		repo := newMockPlanMemberRepo()
		repo.Create(&model.PlanMember{
			TenantID: tenantID, PlanID: planID, UserID: userID, Role: "editor",
		})
		r := buildTestRouter(newMockPlanRepo(), repo, defaultScenarios())

		path := "/api/v1/plans/" + planID.String() + "/scenarios/" + scenarioID.String() + "/products/"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("POST", path, "user"))
		assert.Equal(t, 200, w.Code)
		assert.Equal(t, "CreateProduct", w.Header().Get("X-Handler"))
	})

	t.Run("viewer CANNOT POST product", func(t *testing.T) {
		userID := uuid.MustParse("00000000-0000-0000-0000-000000000099")
		tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

		repo := newMockPlanMemberRepo()
		repo.Create(&model.PlanMember{
			TenantID: tenantID, PlanID: planID, UserID: userID, Role: "viewer",
		})
		r := buildTestRouter(newMockPlanRepo(), repo, defaultScenarios())

		path := "/api/v1/plans/" + planID.String() + "/scenarios/" + scenarioID.String() + "/products/"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("POST", path, "user"))
		assert.Equal(t, 403, w.Code)
	})
}

// TestRouterScenarioMustBelongToPlan covers audit finding S-H1: a scenario ID
// from another plan (or a nonexistent one) must not be reachable through a plan
// the caller has access to — even for the tenant owner.
func TestRouterScenarioMustBelongToPlan(t *testing.T) {
	planID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	otherPlan := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	foreignScenario := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	scenarios := defaultScenarios()
	scenarios.add(&model.Scenario{
		TenantScoped: model.TenantScoped{ID: foreignScenario, TenantID: uuid.MustParse("00000000-0000-0000-0000-000000000001")},
		PlanID:       otherPlan,
	})
	r := buildTestRouter(newMockPlanRepo(), newMockPlanMemberRepo(), scenarios)

	for _, path := range []string{
		"/api/v1/plans/" + planID.String() + "/scenarios/" + foreignScenario.String() + "/",
		"/api/v1/plans/" + planID.String() + "/scenarios/" + foreignScenario.String() + "/products/",
		"/api/v1/plans/" + planID.String() + "/scenarios/" + foreignScenario.String() + "/settings/config",
		"/api/v1/plans/" + planID.String() + "/scenarios/" + uuid.New().String() + "/",
	} {
		t.Run("owner blocked from "+path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, newReq("GET", path, "owner"))
			assert.Equal(t, 404, w.Code)
			assert.Empty(t, w.Header().Get("X-Handler"), "handler must not run")
		})
	}
}

// TestRouterProductMustBelongToScenario: a product ID from another scenario is
// not reachable through a scenario the caller can access (sub-resource scoping).
func TestRouterProductMustBelongToScenario(t *testing.T) {
	base := "/api/v1/plans/11111111-1111-1111-1111-111111111111/scenarios/22222222-2222-2222-2222-222222222222/products/"
	r := buildTestRouter(newMockPlanRepo(), newMockPlanMemberRepo(), defaultScenarios())

	t.Run("product in scenario is served", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", base+"44444444-4444-4444-4444-444444444444/", "owner"))
		assert.Equal(t, 200, w.Code)
		assert.Equal(t, "GetProduct", w.Header().Get("X-Handler"))
	})
	for name, id := range map[string]string{
		"product of another scenario": "55555555-5555-5555-5555-555555555555",
		"unknown product":             uuid.New().String(),
	} {
		t.Run(name, func(t *testing.T) {
			for _, method := range []string{"GET", "PUT"} {
				w := httptest.NewRecorder()
				r.ServeHTTP(w, newReq(method, base+id+"/", "owner"))
				assert.Equal(t, 404, w.Code, method)
				assert.Empty(t, w.Header().Get("X-Handler"), "handler must not run")
			}
		})
	}
}

func TestRouterNonExistentRoute(t *testing.T) {
	r := buildTestRouter(newMockPlanRepo(), newMockPlanMemberRepo(), defaultScenarios())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newReq("GET", "/api/v1/nonexistent", "owner"))
	// Chi returns 404 for unmatched routes (or 405 for wrong method)
	assert.True(t, w.Code == 404 || w.Code == 405)
}

func TestRouterAdminCannotAccessBusinessRoutes(t *testing.T) {
	planID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	r := buildTestRouter(newMockPlanRepo(), newMockPlanMemberRepo(), defaultScenarios())

	routes := []string{
		"/api/v1/plans/" + planID.String() + "/",
		"/api/v1/plans/" + planID.String() + "/scenarios/",
	}

	for _, path := range routes {
		t.Run("admin blocked from "+path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, newReq("GET", path, "admin"))
			assert.Equal(t, 403, w.Code, "admin should be blocked from %s", path)
		})
	}
}
