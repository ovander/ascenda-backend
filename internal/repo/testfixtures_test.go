//go:build integration

package repo_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"ascenda/internal/model"
)

// ── Tenant ────────────────────────────────────────────────────────────────────

// makeTenant creates a uniquely-named active tenant in the test database and
// returns it.  The slug is randomised so parallel/sequential tests don't
// collide on the unique index.
func makeTenant(t *testing.T, db *gorm.DB) *model.Tenant {
	t.Helper()
	uid := uuid.New().String()[:8]
	tenant := &model.Tenant{
		ID:       uuid.New(),
		Name:     "Tenant-" + uid,
		Slug:     "tenant-" + uid,
		IsActive: true,
		Tier:     "free",
		MaxPlans: 10,
		MaxUsers: 10,
	}
	require.NoError(t, db.Create(tenant).Error, "makeTenant: create")
	return tenant
}

// ── User ──────────────────────────────────────────────────────────────────────

// makeUser creates a uniquely-named active user under the given tenant.
func makeUser(t *testing.T, db *gorm.DB, tenantID uuid.UUID) *model.User {
	t.Helper()
	uid := uuid.New().String()[:8]
	user := &model.User{
		TenantID: tenantID,
		Email:    "user-" + uid + "@example.com",
		Name:     "User " + uid,
		Role:     "user",
		IsActive: true,
	}
	require.NoError(t, db.Create(user).Error, "makeUser: create")
	return user
}

// makeOwner is a convenience wrapper that creates a user with the "owner" role.
func makeOwner(t *testing.T, db *gorm.DB, tenantID uuid.UUID) *model.User {
	t.Helper()
	u := makeUser(t, db, tenantID)
	u.Role = "owner"
	require.NoError(t, db.Save(u).Error, "makeOwner: update role")
	return u
}

// ── BusinessPlan ──────────────────────────────────────────────────────────────

// makePlan creates a draft BusinessPlan under the given tenant, attributed to
// the given user.
func makePlan(t *testing.T, db *gorm.DB, tenantID, createdBy uuid.UUID) *model.BusinessPlan {
	t.Helper()
	plan := &model.BusinessPlan{
		TenantScoped: model.TenantScoped{TenantID: tenantID},
		Name:         "Plan-" + uuid.New().String()[:8],
		Description:  "test plan",
		Status:       "draft",
		CreatedBy:    createdBy,
	}
	require.NoError(t, db.Create(plan).Error, "makePlan: create")
	return plan
}

// makeDemoPlan creates a plan marked as a demo (is_demo=true).
func makeDemoPlan(t *testing.T, db *gorm.DB, tenantID, createdBy uuid.UUID) *model.BusinessPlan {
	t.Helper()
	plan := &model.BusinessPlan{
		TenantScoped: model.TenantScoped{TenantID: tenantID},
		Name:         "Demo-" + uuid.New().String()[:8],
		Status:       "draft",
		CreatedBy:    createdBy,
		IsDemo:       true,
	}
	require.NoError(t, db.Create(plan).Error, "makeDemoPlan: create")
	return plan
}

// ── Scenario ──────────────────────────────────────────────────────────────────

// makeScenario creates a Scenario under the given plan.
func makeScenario(t *testing.T, db *gorm.DB, tenantID, planID uuid.UUID) *model.Scenario {
	t.Helper()
	scenario := &model.Scenario{
		TenantScoped: model.TenantScoped{TenantID: tenantID},
		PlanID:       planID,
		Name:         "Scenario-" + uuid.New().String()[:8],
		Description:  "test scenario",
	}
	require.NoError(t, db.Create(scenario).Error, "makeScenario: create")
	return scenario
}
