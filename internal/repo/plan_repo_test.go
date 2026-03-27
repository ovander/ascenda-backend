//go:build integration

package repo_test

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"ascenda/internal/model"
	"ascenda/internal/repo"
)

// ── Create ────────────────────────────────────────────────────────────────────

func TestPlanRepo_Create_Succeeds(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)

	r := repo.NewPlanRepo(db)
	plan := &model.BusinessPlan{
		TenantScoped: model.TenantScoped{TenantID: tenant.ID},
		Name:         "My First Plan",
		Description:  "description",
		Status:       "draft",
		CreatedBy:    owner.ID,
	}

	err := r.Create(plan)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, plan.ID, "Create must populate the plan ID")
	assert.False(t, plan.CreatedAt.IsZero(), "Create must set CreatedAt")
}

func TestPlanRepo_Create_AssignsUUIDWhenNilProvided(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)

	r := repo.NewPlanRepo(db)
	plan := &model.BusinessPlan{
		TenantScoped: model.TenantScoped{TenantID: tenant.ID},
		Name:         "Auto-ID Plan",
		Status:       "draft",
		CreatedBy:    owner.ID,
		// ID intentionally left as uuid.Nil
	}

	require.NoError(t, r.Create(plan))
	assert.NotEqual(t, uuid.Nil, plan.ID)
}

// ── GetByID ───────────────────────────────────────────────────────────────────

func TestPlanRepo_GetByID_ReturnsCorrectPlan(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)
	created := makePlan(t, db, tenant.ID, owner.ID)

	r := repo.NewPlanRepo(db)
	got, err := r.GetByID(tenant.ID, created.ID)

	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, created.Name, got.Name)
	assert.Equal(t, tenant.ID, got.TenantID)
}

func TestPlanRepo_GetByID_ReturnsNotFoundForWrongTenant(t *testing.T) {
	db := testDB(t)
	tenantA := makeTenant(t, db)
	tenantB := makeTenant(t, db)
	ownerA := makeUser(t, db, tenantA.ID)
	plan := makePlan(t, db, tenantA.ID, ownerA.ID)

	r := repo.NewPlanRepo(db)
	_, err := r.GetByID(tenantB.ID, plan.ID)

	require.Error(t, err, "tenant B must not be able to retrieve tenant A's plan")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestPlanRepo_GetByID_ReturnsNotFoundForUnknownID(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)

	r := repo.NewPlanRepo(db)
	_, err := r.GetByID(tenant.ID, uuid.New())

	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ── ListByTenant ──────────────────────────────────────────────────────────────

func TestPlanRepo_ListByTenant_ReturnsTenantPlansOnly(t *testing.T) {
	db := testDB(t)
	tenantA := makeTenant(t, db)
	tenantB := makeTenant(t, db)
	ownerA := makeUser(t, db, tenantA.ID)
	ownerB := makeUser(t, db, tenantB.ID)

	// 3 plans for A, 1 for B
	for i := 0; i < 3; i++ {
		makePlan(t, db, tenantA.ID, ownerA.ID)
	}
	makePlan(t, db, tenantB.ID, ownerB.ID)

	r := repo.NewPlanRepo(db)
	plans, err := r.ListByTenant(tenantA.ID, 0, 50)

	require.NoError(t, err)
	assert.Len(t, plans, 3)
	for _, p := range plans {
		assert.Equal(t, tenantA.ID, p.TenantID, "all returned plans must belong to tenant A")
	}
}

func TestPlanRepo_ListByTenant_PaginationOffsetAndLimit(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)

	for i := 0; i < 5; i++ {
		makePlan(t, db, tenant.ID, owner.ID)
	}

	r := repo.NewPlanRepo(db)

	// Page 1: first 3
	page1, err := r.ListByTenant(tenant.ID, 0, 3)
	require.NoError(t, err)
	assert.Len(t, page1, 3)

	// Page 2: remaining 2
	page2, err := r.ListByTenant(tenant.ID, 3, 3)
	require.NoError(t, err)
	assert.Len(t, page2, 2)

	// No overlap between pages
	ids1 := make(map[uuid.UUID]bool)
	for _, p := range page1 {
		ids1[p.ID] = true
	}
	for _, p := range page2 {
		assert.False(t, ids1[p.ID], "page 2 must not contain records from page 1")
	}
}

func TestPlanRepo_ListByTenant_EmptyWhenNoPlans(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)

	r := repo.NewPlanRepo(db)
	plans, err := r.ListByTenant(tenant.ID, 0, 50)

	require.NoError(t, err)
	assert.Empty(t, plans)
}

// ── CountByTenant ─────────────────────────────────────────────────────────────

// CountByTenant is a regression test for the pagination bug fixed in the
// remediation sprint: the count must come from a SELECT COUNT(*) query, not
// from len(page), so it reflects the true total even when pagination limits
// the page size.
func TestPlanRepo_CountByTenant_ReturnsAccurateTotal(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)

	for i := 0; i < 4; i++ {
		makePlan(t, db, tenant.ID, owner.ID)
	}

	r := repo.NewPlanRepo(db)
	count, err := r.CountByTenant(tenant.ID)

	require.NoError(t, err)
	assert.Equal(t, int64(4), count)
}

func TestPlanRepo_CountByTenant_CountDoesNotLeakAcrossTenants(t *testing.T) {
	db := testDB(t)
	tenantA := makeTenant(t, db)
	tenantB := makeTenant(t, db)
	ownerA := makeUser(t, db, tenantA.ID)
	ownerB := makeUser(t, db, tenantB.ID)

	makePlan(t, db, tenantA.ID, ownerA.ID)
	makePlan(t, db, tenantA.ID, ownerA.ID)
	makePlan(t, db, tenantB.ID, ownerB.ID) // must not be counted for A

	r := repo.NewPlanRepo(db)
	count, err := r.CountByTenant(tenantA.ID)

	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestPlanRepo_CountByTenant_ReturnsZeroForNewTenant(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)

	r := repo.NewPlanRepo(db)
	count, err := r.CountByTenant(tenant.ID)

	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

// CountByTenant must stay consistent with ListByTenant even across page
// boundaries — this is the exact scenario the pagination bug broke.
func TestPlanRepo_CountByTenant_ConsistentWithListAcrossPages(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)

	const total = 7
	for i := 0; i < total; i++ {
		makePlan(t, db, tenant.ID, owner.ID)
	}

	r := repo.NewPlanRepo(db)

	count, err := r.CountByTenant(tenant.ID)
	require.NoError(t, err)

	// Fetch only the first page (size 3) and confirm count still equals total
	page, err := r.ListByTenant(tenant.ID, 0, 3)
	require.NoError(t, err)

	assert.Equal(t, int64(total), count, "count must equal the DB total, not len(page)")
	assert.Len(t, page, 3, "page must respect the limit")
}

// ── Update ────────────────────────────────────────────────────────────────────

func TestPlanRepo_Update_PersistsChanges(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)
	plan := makePlan(t, db, tenant.ID, owner.ID)

	plan.Name = "Renamed Plan"
	plan.Status = "review"

	r := repo.NewPlanRepo(db)
	require.NoError(t, r.Update(plan))

	fetched, err := r.GetByID(tenant.ID, plan.ID)
	require.NoError(t, err)
	assert.Equal(t, "Renamed Plan", fetched.Name)
	assert.Equal(t, "review", fetched.Status)
}

func TestPlanRepo_Update_DoesNotAffectOtherPlans(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)
	planA := makePlan(t, db, tenant.ID, owner.ID)
	planB := makePlan(t, db, tenant.ID, owner.ID)
	originalBName := planB.Name

	planA.Name = "Updated A"
	r := repo.NewPlanRepo(db)
	require.NoError(t, r.Update(planA))

	fetchedB, err := r.GetByID(tenant.ID, planB.ID)
	require.NoError(t, err)
	assert.Equal(t, originalBName, fetchedB.Name, "updating plan A must not affect plan B")
}

// ── Delete ────────────────────────────────────────────────────────────────────

func TestPlanRepo_Delete_RemovesPlan(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)
	plan := makePlan(t, db, tenant.ID, owner.ID)

	r := repo.NewPlanRepo(db)
	require.NoError(t, r.Delete(tenant.ID, plan.ID))

	_, err := r.GetByID(tenant.ID, plan.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestPlanRepo_Delete_IgnoresWrongTenant(t *testing.T) {
	db := testDB(t)
	tenantA := makeTenant(t, db)
	tenantB := makeTenant(t, db)
	ownerA := makeUser(t, db, tenantA.ID)
	plan := makePlan(t, db, tenantA.ID, ownerA.ID)

	r := repo.NewPlanRepo(db)
	// Wrong-tenant delete must return ErrRecordNotFound (translated to 404 by the HTTP handler)
	require.ErrorIs(t, r.Delete(tenantB.ID, plan.ID), gorm.ErrRecordNotFound)

	// Plan must still exist under tenant A
	fetched, err := r.GetByID(tenantA.ID, plan.ID)
	require.NoError(t, err)
	assert.Equal(t, plan.ID, fetched.ID)
}

// ── PurgeDemoPlans ────────────────────────────────────────────────────────────

func TestPlanRepo_PurgeDemoPlans_DeletesDemoPlansAndScenarios(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)

	demo := makeDemoPlan(t, db, tenant.ID, owner.ID)
	// Attach a scenario to the demo plan so we can verify cascading removal
	makeScenario(t, db, tenant.ID, demo.ID)

	r := repo.NewPlanRepo(db)
	require.NoError(t, r.PurgeDemoPlans(tenant.ID))

	// Demo plan gone
	_, err := r.GetByID(tenant.ID, demo.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "demo plan must be removed")

	// Scenario gone
	var count int64
	db.Model(&model.Scenario{}).Where("plan_id = ?", demo.ID).Count(&count)
	assert.Equal(t, int64(0), count, "scenarios belonging to the demo plan must be purged")
}

func TestPlanRepo_PurgeDemoPlans_PreservesNonDemoPlans(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)

	real := makePlan(t, db, tenant.ID, owner.ID)
	makeDemoPlan(t, db, tenant.ID, owner.ID)

	r := repo.NewPlanRepo(db)
	require.NoError(t, r.PurgeDemoPlans(tenant.ID))

	// Real plan untouched
	fetched, err := r.GetByID(tenant.ID, real.ID)
	require.NoError(t, err)
	assert.Equal(t, real.ID, fetched.ID)
}

func TestPlanRepo_PurgeDemoPlans_IsIdempotent(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)
	makeDemoPlan(t, db, tenant.ID, owner.ID)

	r := repo.NewPlanRepo(db)
	require.NoError(t, r.PurgeDemoPlans(tenant.ID))
	// Second call must not error even though there is nothing left to purge
	require.NoError(t, r.PurgeDemoPlans(tenant.ID))
}

func TestPlanRepo_PurgeDemoPlans_DoesNotLeakAcrossTenants(t *testing.T) {
	db := testDB(t)
	tenantA := makeTenant(t, db)
	tenantB := makeTenant(t, db)
	ownerA := makeUser(t, db, tenantA.ID)
	ownerB := makeUser(t, db, tenantB.ID)

	makeDemoPlan(t, db, tenantA.ID, ownerA.ID)
	demoB := makeDemoPlan(t, db, tenantB.ID, ownerB.ID)

	r := repo.NewPlanRepo(db)
	require.NoError(t, r.PurgeDemoPlans(tenantA.ID))

	// Tenant B's demo plan is untouched
	fetched, err := r.GetByID(tenantB.ID, demoB.ID)
	require.NoError(t, err)
	assert.Equal(t, demoB.ID, fetched.ID)
}

// ── ListByTenant ordering ─────────────────────────────────────────────────────

func TestPlanRepo_ListByTenant_OrderedByCreatedAtDesc(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeUser(t, db, tenant.ID)

	// Create plans and collect their IDs in insertion order
	var insertedIDs []uuid.UUID
	for i := 0; i < 3; i++ {
		p := makePlan(t, db, tenant.ID, owner.ID)
		// Give each plan a slightly different name to identify it
		p.Name = fmt.Sprintf("Plan %02d", i)
		require.NoError(t, db.Save(p).Error)
		insertedIDs = append(insertedIDs, p.ID)
	}

	r := repo.NewPlanRepo(db)
	plans, err := r.ListByTenant(tenant.ID, 0, 50)
	require.NoError(t, err)
	require.Len(t, plans, 3)

	// Verify descending order: each plan's CreatedAt >= the next one's
	for i := 1; i < len(plans); i++ {
		assert.False(
			t,
			plans[i-1].CreatedAt.Before(plans[i].CreatedAt),
			"plans[%d].CreatedAt should be >= plans[%d].CreatedAt", i-1, i,
		)
	}
}
