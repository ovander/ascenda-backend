package middleware

import (
	"net/http"

	"ascenda/internal/repo"
	"ascenda/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"
)

// PlanAccessMiddleware checks plan-level permissions via plan_members table and
// binds nested resources (scenarios) to the plan named in the URL.
type PlanAccessMiddleware struct {
	resolver     *service.PlanAccessResolver
	scenarioRepo repo.ScenarioRepository
	logger       *logrus.Entry
}

func NewPlanAccessMiddleware(planRepo repo.PlanRepository, planMemberRepo repo.PlanMemberRepository, scenarioRepo repo.ScenarioRepository, logger *logrus.Entry) *PlanAccessMiddleware {
	return &PlanAccessMiddleware{
		resolver:     service.NewPlanAccessResolver(planRepo, planMemberRepo, scenarioRepo),
		scenarioRepo: scenarioRepo,
		logger:       logger,
	}
}

// writeJSONError writes a minimal JSON error body in the API's envelope format.
func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(`{"error":{"code":"` + code + `","message":"` + message + `"}}`)) //nolint:errcheck
}

// RequirePlanAccess checks if user has any access (editor or viewer) to the plan.
// Owner bypasses this check entirely.
//
// The plan-level role injected into the context comes from
// service.PlanAccessResolver.PlanRole (owner → member role → demo sandbox);
// callers without any access are rejected with 403.
func (m *PlanAccessMiddleware) RequirePlanAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		role := ctxutil.GetUserRole(ctx)

		// Owner has access to all plans
		if role == "owner" {
			next.ServeHTTP(w, r)
			return
		}

		// Admin has no plan access at all
		if role == "admin" {
			writeJSONError(w, http.StatusForbidden, "forbidden", "admin role cannot access plans")
			return
		}

		tenantID := ctxutil.GetTenantID(ctx)
		userID := ctxutil.GetUserID(ctx)

		// Extract planId from URL (could be {id} or {planId} depending on route)
		planIDStr := chi.URLParam(r, "planId")
		if planIDStr == "" {
			planIDStr = chi.URLParam(r, "id")
		}
		if planIDStr == "" {
			// No plan ID in URL - this is a list endpoint, let it through
			// The handler will filter by accessible plans
			next.ServeHTTP(w, r)
			return
		}

		planID, err := uuid.Parse(planIDStr)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "bad_request", "invalid plan ID")
			return
		}

		// The plan-level role is set explicitly rather than leaking the tenant
		// role through; RequireScenarioInPlan keeps a demo plan from being used
		// as a gateway to other plans' scenarios (audit finding S-H1).
		planRole, ok := m.resolver.PlanRole(tenantID, userID, role, planID)
		if !ok {
			m.logger.WithField("user_id", userID).WithField("plan_id", planID).Warn("plan access denied")
			writeJSONError(w, http.StatusForbidden, "forbidden", "no access to this plan")
			return
		}
		ctx = ctxutil.WithUserRole(ctx, planRole)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireScenarioInPlan verifies that the {scenarioId} in the URL belongs to
// the {planId} in the URL (within the caller's tenant). Without this check a
// user with access to any plan could read or write every scenario in the
// tenant by pairing their plan ID with a foreign scenario ID, because the
// data services are keyed on (tenant, scenario) only.
//
// Mount it on the "/{scenarioId}" subrouter, after RequirePlanAccess.
// A mismatch is reported as 404 so that scenario IDs from other plans are not
// confirmed to exist.
func (m *PlanAccessMiddleware) RequireScenarioInPlan(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		planID, err := uuid.Parse(chi.URLParam(r, "planId"))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "bad_request", "invalid plan ID")
			return
		}
		scenarioID, err := uuid.Parse(chi.URLParam(r, "scenarioId"))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "bad_request", "invalid scenario ID")
			return
		}

		tenantID := ctxutil.GetTenantID(ctx)
		scenario, err := m.scenarioRepo.GetByID(tenantID, scenarioID)
		if err != nil || scenario == nil {
			writeJSONError(w, http.StatusNotFound, "not_found", "scenario not found")
			return
		}
		if scenario.PlanID != planID {
			m.logger.WithFields(logrus.Fields{
				"user_id":       ctxutil.GetUserID(ctx),
				"plan_id":       planID,
				"scenario_id":   scenarioID,
				"scenario_plan": scenario.PlanID,
			}).Warn("scenario does not belong to plan in URL — access denied")
			writeJSONError(w, http.StatusNotFound, "not_found", "scenario not found")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequirePlanEdit checks if user has editor access to the plan.
// Used for PUT/POST/DELETE on plan data.
func (m *PlanAccessMiddleware) RequirePlanEdit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		role := ctxutil.GetUserRole(ctx)

		// After RequirePlanAccess, the role is set to the plan-level role
		// Owner keeps their "owner" role which always has edit access
		if role == "owner" || role == "editor" {
			next.ServeHTTP(w, r)
			return
		}

		writeJSONError(w, http.StatusForbidden, "forbidden", "editor access required")
	})
}
