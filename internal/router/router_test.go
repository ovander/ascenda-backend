package router

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

// ── Stub handler that returns 200 with handler name ───────────────

func stubHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Handler", name)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"handler":"` + name + `"}`))
	}
}

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

func buildTestRouter(planMemberRepo repo.PlanMemberRepository) *chi.Mux {
	logger := logrus.NewEntry(logrus.New())

	// Create real RBAC and PlanAccess middleware
	rbacMW := middleware.NewRBACMiddleware(logger)
	planAccessMW := middleware.NewPlanAccessMiddleware(planMemberRepo, logger)

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

			// Tenant management (requires manage:tenant)
			r.Route("/tenant", func(r chi.Router) {
				r.Use(rbacMW.RequirePermission(middleware.PermManageTenant))
				r.Get("/", stubHandler("GetTenant"))
				r.Put("/", stubHandler("UpdateTenant"))
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
							r.Get("/", stubHandler("GetScenario"))

							r.Route("/products", func(r chi.Router) {
								r.Get("/", stubHandler("ListProducts"))
								r.With(planAccessMW.RequirePlanEdit).Post("/", stubHandler("CreateProduct"))
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
	r := buildTestRouter(newMockPlanMemberRepo())

	t.Run("GET /users/me returns 200 for any role", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", "/api/v1/users/me", "user"))
		assert.Equal(t, 200, w.Code)
		assert.Equal(t, "GetMe", w.Header().Get("X-Handler"))
	})
}

func TestRouterUserManagementRBAC(t *testing.T) {
	r := buildTestRouter(newMockPlanMemberRepo())

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
	r := buildTestRouter(newMockPlanMemberRepo())

	t.Run("owner can access tenant", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", "/api/v1/tenant/", "owner"))
		assert.Equal(t, 200, w.Code)
	})

	t.Run("admin CANNOT access tenant", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", "/api/v1/tenant/", "admin"))
		assert.Equal(t, 403, w.Code)
	})

	t.Run("user CANNOT access tenant", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", "/api/v1/tenant/", "user"))
		assert.Equal(t, 403, w.Code)
	})
}

func TestRouterPlanListAccessible(t *testing.T) {
	r := buildTestRouter(newMockPlanMemberRepo())

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
	r := buildTestRouter(newMockPlanMemberRepo())

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
		r := buildTestRouter(repo)
		// No plan_member entry — owner bypasses

		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", "/api/v1/plans/"+planID.String()+"/", "owner"))
		assert.Equal(t, 200, w.Code)
		assert.Equal(t, "GetPlan", w.Header().Get("X-Handler"))
	})

	t.Run("admin blocked from plan access", func(t *testing.T) {
		repo := newMockPlanMemberRepo()
		r := buildTestRouter(repo)

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
		r := buildTestRouter(repo)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", "/api/v1/plans/"+planID.String()+"/", "user"))
		assert.Equal(t, 200, w.Code)
	})

	t.Run("user without membership denied", func(t *testing.T) {
		repo := newMockPlanMemberRepo()
		r := buildTestRouter(repo)

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
		r := buildTestRouter(repo)

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
		r := buildTestRouter(repo)

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
		r := buildTestRouter(repo)

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
		r := buildTestRouter(repo)

		path := "/api/v1/plans/" + planID.String() + "/scenarios/"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", path, "owner"))
		assert.Equal(t, 200, w.Code)
		assert.Equal(t, "ListScenarios", w.Header().Get("X-Handler"))
	})

	t.Run("owner can get scenario", func(t *testing.T) {
		repo := newMockPlanMemberRepo()
		r := buildTestRouter(repo)

		path := "/api/v1/plans/" + planID.String() + "/scenarios/" + scenarioID.String() + "/"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", path, "owner"))
		assert.Equal(t, 200, w.Code)
		assert.Equal(t, "GetScenario", w.Header().Get("X-Handler"))
	})

	t.Run("owner can list products under scenario", func(t *testing.T) {
		repo := newMockPlanMemberRepo()
		r := buildTestRouter(repo)

		path := "/api/v1/plans/" + planID.String() + "/scenarios/" + scenarioID.String() + "/products/"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("GET", path, "owner"))
		assert.Equal(t, 200, w.Code)
		assert.Equal(t, "ListProducts", w.Header().Get("X-Handler"))
	})

	t.Run("owner can get settings config", func(t *testing.T) {
		repo := newMockPlanMemberRepo()
		r := buildTestRouter(repo)

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
		r := buildTestRouter(repo)

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
		r := buildTestRouter(repo)

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
		r := buildTestRouter(repo)

		path := "/api/v1/plans/" + planID.String() + "/scenarios/" + scenarioID.String() + "/products/"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newReq("POST", path, "user"))
		assert.Equal(t, 403, w.Code)
	})
}

func TestRouterNonExistentRoute(t *testing.T) {
	r := buildTestRouter(newMockPlanMemberRepo())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newReq("GET", "/api/v1/nonexistent", "owner"))
	// Chi returns 404 for unmatched routes (or 405 for wrong method)
	assert.True(t, w.Code == 404 || w.Code == 405)
}

func TestRouterAdminCannotAccessBusinessRoutes(t *testing.T) {
	planID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	r := buildTestRouter(newMockPlanMemberRepo())

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
