//go:build integration

package repo_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"ascenda/internal/model"
	"ascenda/internal/repo"
)

// ── Create ────────────────────────────────────────────────────────────────────

func TestScenarioRepo_Create_Succeeds(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)
	plan := makePlan(t, db, tenant.ID, owner.ID)

	r := repo.NewScenarioRepo(db)
	scenario := &model.Scenario{
		TenantScoped: model.TenantScoped{TenantID: tenant.ID},
		PlanID:       plan.ID,
		Name:         "Base Case",
	}

	err := r.Create(scenario)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, scenario.ID, "Create must populate the ID")
	assert.False(t, scenario.CreatedAt.IsZero(), "Create must set CreatedAt")
}

func TestScenarioRepo_Create_AssignsUUIDWhenNilProvided(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)
	plan := makePlan(t, db, tenant.ID, owner.ID)

	r := repo.NewScenarioRepo(db)
	scenario := &model.Scenario{
		TenantScoped: model.TenantScoped{TenantID: tenant.ID},
		PlanID:       plan.ID,
		Name:         "Bull Case",
		// ID is uuid.Nil — BeforeCreate hook must supply one
	}

	require.NoError(t, r.Create(scenario))
	assert.NotEqual(t, uuid.Nil, scenario.ID)
}

// ── GetByID ───────────────────────────────────────────────────────────────────

func TestScenarioRepo_GetByID_ReturnsCorrectScenario(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)
	plan := makePlan(t, db, tenant.ID, owner.ID)
	created := makeScenario(t, db, tenant.ID, plan.ID)

	r := repo.NewScenarioRepo(db)
	got, err := r.GetByID(tenant.ID, created.ID)

	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, created.Name, got.Name)
	assert.Equal(t, plan.ID, got.PlanID)
}

func TestScenarioRepo_GetByID_ReturnsNotFoundForWrongTenant(t *testing.T) {
	db := testDB(t)
	tenantA := makeTenant(t, db)
	tenantB := makeTenant(t, db)
	ownerA := makeUser(t, db, tenantA.ID)
	plan := makePlan(t, db, tenantA.ID, ownerA.ID)
	scenario := makeScenario(t, db, tenantA.ID, plan.ID)

	r := repo.NewScenarioRepo(db)
	_, err := r.GetByID(tenantB.ID, scenario.ID)

	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestScenarioRepo_GetByID_ReturnsNotFoundForUnknownID(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)

	r := repo.NewScenarioRepo(db)
	_, err := r.GetByID(tenant.ID, uuid.New())

	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ── ListByPlan ────────────────────────────────────────────────────────────────

func TestScenarioRepo_ListByPlan_ReturnsScenariosForPlanOnly(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)
	planA := makePlan(t, db, tenant.ID, owner.ID)
	planB := makePlan(t, db, tenant.ID, owner.ID)

	makeScenario(t, db, tenant.ID, planA.ID)
	makeScenario(t, db, tenant.ID, planA.ID)
	makeScenario(t, db, tenant.ID, planB.ID) // must not appear in planA results

	r := repo.NewScenarioRepo(db)
	scenarios, err := r.ListByPlan(tenant.ID, planA.ID)

	require.NoError(t, err)
	assert.Len(t, scenarios, 2)
	for _, s := range scenarios {
		assert.Equal(t, planA.ID, s.PlanID)
		assert.Equal(t, tenant.ID, s.TenantID)
	}
}

func TestScenarioRepo_ListByPlan_DoesNotLeakAcrossTenants(t *testing.T) {
	db := testDB(t)
	tenantA := makeTenant(t, db)
	tenantB := makeTenant(t, db)
	ownerA := makeUser(t, db, tenantA.ID)
	ownerB := makeUser(t, db, tenantB.ID)
	planA := makePlan(t, db, tenantA.ID, ownerA.ID)
	planB := makePlan(t, db, tenantB.ID, ownerB.ID)

	makeScenario(t, db, tenantA.ID, planA.ID)
	makeScenario(t, db, tenantB.ID, planB.ID)

	r := repo.NewScenarioRepo(db)
	// Query tenantB's context for planA's ID — must return nothing
	scenarios, err := r.ListByPlan(tenantB.ID, planA.ID)

	require.NoError(t, err)
	assert.Empty(t, scenarios)
}

func TestScenarioRepo_ListByPlan_EmptyWhenNoScenarios(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)
	plan := makePlan(t, db, tenant.ID, owner.ID)

	r := repo.NewScenarioRepo(db)
	scenarios, err := r.ListByPlan(tenant.ID, plan.ID)

	require.NoError(t, err)
	assert.Empty(t, scenarios)
}

func TestScenarioRepo_ListByPlan_OrderedByCreatedAtAsc(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)
	plan := makePlan(t, db, tenant.ID, owner.ID)

	for i := 0; i < 3; i++ {
		makeScenario(t, db, tenant.ID, plan.ID)
	}

	r := repo.NewScenarioRepo(db)
	scenarios, err := r.ListByPlan(tenant.ID, plan.ID)
	require.NoError(t, err)
	require.Len(t, scenarios, 3)

	// ListByPlan orders ASC — each entry must not pre-date the previous one
	for i := 1; i < len(scenarios); i++ {
		assert.False(
			t,
			scenarios[i].CreatedAt.Before(scenarios[i-1].CreatedAt),
			"scenarios[%d] should not be older than scenarios[%d]", i, i-1,
		)
	}
}

// ── Update ────────────────────────────────────────────────────────────────────

func TestScenarioRepo_Update_PersistsChanges(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)
	plan := makePlan(t, db, tenant.ID, owner.ID)
	scenario := makeScenario(t, db, tenant.ID, plan.ID)

	scenario.Name = "Bear Case"
	scenario.Description = "pessimistic assumptions"

	r := repo.NewScenarioRepo(db)
	require.NoError(t, r.Update(scenario))

	fetched, err := r.GetByID(tenant.ID, scenario.ID)
	require.NoError(t, err)
	assert.Equal(t, "Bear Case", fetched.Name)
	assert.Equal(t, "pessimistic assumptions", fetched.Description)
}

func TestScenarioRepo_Update_DoesNotAffectSiblingScenarios(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)
	plan := makePlan(t, db, tenant.ID, owner.ID)
	scenarioA := makeScenario(t, db, tenant.ID, plan.ID)
	scenarioB := makeScenario(t, db, tenant.ID, plan.ID)
	originalBName := scenarioB.Name

	scenarioA.Name = "Updated A"
	r := repo.NewScenarioRepo(db)
	require.NoError(t, r.Update(scenarioA))

	fetchedB, err := r.GetByID(tenant.ID, scenarioB.ID)
	require.NoError(t, err)
	assert.Equal(t, originalBName, fetchedB.Name)
}

// ── Delete ────────────────────────────────────────────────────────────────────

func TestScenarioRepo_Delete_RemovesScenario(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)
	plan := makePlan(t, db, tenant.ID, owner.ID)
	scenario := makeScenario(t, db, tenant.ID, plan.ID)

	r := repo.NewScenarioRepo(db)
	require.NoError(t, r.Delete(tenant.ID, scenario.ID))

	_, err := r.GetByID(tenant.ID, scenario.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestScenarioRepo_Delete_DoesNotRemoveSiblingScenarios(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)
	plan := makePlan(t, db, tenant.ID, owner.ID)
	scenarioA := makeScenario(t, db, tenant.ID, plan.ID)
	scenarioB := makeScenario(t, db, tenant.ID, plan.ID)

	r := repo.NewScenarioRepo(db)
	require.NoError(t, r.Delete(tenant.ID, scenarioA.ID))

	// Sibling B must still exist
	fetched, err := r.GetByID(tenant.ID, scenarioB.ID)
	require.NoError(t, err)
	assert.Equal(t, scenarioB.ID, fetched.ID)
}

func TestScenarioRepo_Delete_IgnoresWrongTenant(t *testing.T) {
	db := testDB(t)
	tenantA := makeTenant(t, db)
	tenantB := makeTenant(t, db)
	ownerA := makeUser(t, db, tenantA.ID)
	plan := makePlan(t, db, tenantA.ID, ownerA.ID)
	scenario := makeScenario(t, db, tenantA.ID, plan.ID)

	r := repo.NewScenarioRepo(db)
	// Wrong-tenant delete must return ErrRecordNotFound (translated to 404 by the HTTP handler)
	require.ErrorIs(t, r.Delete(tenantB.ID, scenario.ID), gorm.ErrRecordNotFound)

	fetched, err := r.GetByID(tenantA.ID, scenario.ID)
	require.NoError(t, err)
	assert.Equal(t, scenario.ID, fetched.ID)
}
