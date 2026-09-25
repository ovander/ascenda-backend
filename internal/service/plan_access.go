package service

import (
	"fmt"

	"ascenda/internal/repo"
	"github.com/google/uuid"
)

// PlanAccessResolver is the single place that decides which plan-level role a
// tenant user has on a plan. It is used by PlanAccessMiddleware for URL-scoped
// routes and by handlers that must scope tenant-wide data (the audit trail)
// to what the caller may see.
//
// Rule, in order:
//   - tenant role "owner" → "owner" on every plan of the tenant;
//   - tenant role "admin" → no plan access (platform operators are not tenant users);
//   - an explicit plan_members row → its role ("editor" | "viewer");
//   - a demo plan → "editor" ("viewer" for the read-only tenant role "reader");
//   - otherwise no access.
type PlanAccessResolver struct {
	planRepo     repo.PlanRepository
	memberRepo   repo.PlanMemberRepository
	scenarioRepo repo.ScenarioRepository
}

// NewPlanAccessResolver creates a PlanAccessResolver. scenarioRepo is only
// needed by AccessibleEntityIDs and may be nil when that method is not used.
func NewPlanAccessResolver(planRepo repo.PlanRepository, memberRepo repo.PlanMemberRepository, scenarioRepo repo.ScenarioRepository) *PlanAccessResolver {
	return &PlanAccessResolver{planRepo: planRepo, memberRepo: memberRepo, scenarioRepo: scenarioRepo}
}

// PlanRole returns the caller's plan-level role on planID and whether they
// have any access at all.
func (r *PlanAccessResolver) PlanRole(tenantID, userID uuid.UUID, tenantRole string, planID uuid.UUID) (string, bool) {
	switch tenantRole {
	case "owner":
		return "owner", true
	case "admin":
		return "", false
	}

	if member, err := r.memberRepo.GetByPlanAndUser(tenantID, planID, userID); err == nil && member != nil {
		return member.Role, true
	}

	if plan, err := r.planRepo.GetByID(tenantID, planID); err == nil && plan != nil && plan.IsDemo {
		if tenantRole == "reader" {
			return "viewer", true
		}
		return "editor", true
	}

	return "", false
}

// AccessibleEntityIDs returns the set of IDs that tenant-wide, entity-keyed
// data (such as audit rows) may be filtered on for the caller: the IDs of
// every plan they can access, the IDs of those plans' scenarios, and the
// tenant ID itself (tenant-level events such as audit exports).
//
// all=true means the caller may see everything in the tenant (owner) and ids
// is nil; callers should then skip the filter entirely.
func (r *PlanAccessResolver) AccessibleEntityIDs(tenantID, userID uuid.UUID, tenantRole string) (ids []uuid.UUID, all bool, err error) {
	switch tenantRole {
	case "owner":
		return nil, true, nil
	case "admin":
		return nil, false, nil
	}
	if r.scenarioRepo == nil {
		return nil, false, fmt.Errorf("plan access resolver: scenario repository not configured")
	}

	planIDs := map[uuid.UUID]struct{}{}

	members, err := r.memberRepo.ListByUser(tenantID, userID)
	if err != nil {
		return nil, false, fmt.Errorf("list plan memberships: %w", err)
	}
	for _, m := range members {
		planIDs[m.PlanID] = struct{}{}
	}

	// Demo plans are readable by every tenant user. Tenants hold a handful of
	// plans, so a bounded page is enough to find the demo ones.
	plans, err := r.planRepo.ListByTenant(tenantID, 0, 1000)
	if err != nil {
		return nil, false, fmt.Errorf("list tenant plans: %w", err)
	}
	for _, p := range plans {
		if p.IsDemo {
			planIDs[p.ID] = struct{}{}
		}
	}

	ids = append(ids, tenantID)
	for planID := range planIDs {
		ids = append(ids, planID)
		scenarios, err := r.scenarioRepo.ListByPlan(tenantID, planID)
		if err != nil {
			return nil, false, fmt.Errorf("list scenarios of plan %s: %w", planID, err)
		}
		for _, s := range scenarios {
			ids = append(ids, s.ID)
		}
	}
	return ids, false, nil
}
