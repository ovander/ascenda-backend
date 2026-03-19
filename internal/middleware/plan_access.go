package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"kerplan/internal/pkg/ctxutil"
	"kerplan/internal/repo"
)

// PlanAccessMiddleware checks plan-level permissions via plan_members table.
type PlanAccessMiddleware struct {
	planMemberRepo repo.PlanMemberRepository
	logger         *logrus.Entry
}

func NewPlanAccessMiddleware(planMemberRepo repo.PlanMemberRepository, logger *logrus.Entry) *PlanAccessMiddleware {
	return &PlanAccessMiddleware{
		planMemberRepo: planMemberRepo,
		logger:         logger,
	}
}

// RequirePlanAccess checks if user has any access (editor or viewer) to the plan.
// Owner bypasses this check entirely.
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
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":{"code":"forbidden","message":"admin role cannot access plans"}}`))
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
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":{"code":"bad_request","message":"invalid plan ID"}}`))
			return
		}

		member, err := m.planMemberRepo.GetByPlanAndUser(tenantID, planID, userID)
		if err != nil || member == nil {
			m.logger.WithField("user_id", userID).WithField("plan_id", planID).Warn("plan access denied")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":{"code":"forbidden","message":"no access to this plan"}}`))
			return
		}

		// Inject the plan-level role into context so handlers can check it
		ctx = ctxutil.WithUserRole(ctx, member.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
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

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":{"code":"forbidden","message":"editor access required"}}`))
	})
}
