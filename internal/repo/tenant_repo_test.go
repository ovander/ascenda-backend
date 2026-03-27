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

func TestTenantRepo_Create_Succeeds(t *testing.T) {
	db := testDB(t)

	r := repo.NewTenantRepo(db)
	uid := uuid.New().String()[:8]
	tenant := &model.Tenant{
		ID:       uuid.New(),
		Name:     "Acme-" + uid,
		Slug:     "acme-" + uid,
		IsActive: true,
		Tier:     "free",
	}

	err := r.Create(tenant)

	require.NoError(t, err)
	assert.False(t, tenant.CreatedAt.IsZero(), "Create must set CreatedAt")
}

func TestTenantRepo_Create_RejectsNonUniqueSlug(t *testing.T) {
	db := testDB(t)
	existing := makeTenant(t, db)

	r := repo.NewTenantRepo(db)
	duplicate := &model.Tenant{
		ID:       uuid.New(),
		Name:     "Other Name",
		Slug:     existing.Slug, // same slug — unique index must reject this
		IsActive: true,
		Tier:     "free",
	}

	err := r.Create(duplicate)
	assert.Error(t, err, "duplicate slug must be rejected by the unique index")
}

// ── GetByID ───────────────────────────────────────────────────────────────────

func TestTenantRepo_GetByID_ReturnsCorrectTenant(t *testing.T) {
	db := testDB(t)
	created := makeTenant(t, db)

	r := repo.NewTenantRepo(db)
	got, err := r.GetByID(created.ID)

	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, created.Name, got.Name)
	assert.Equal(t, created.Slug, got.Slug)
}

func TestTenantRepo_GetByID_ReturnsNotFoundForUnknownID(t *testing.T) {
	db := testDB(t)

	r := repo.NewTenantRepo(db)
	_, err := r.GetByID(uuid.New())

	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ── GetBySlug ─────────────────────────────────────────────────────────────────

func TestTenantRepo_GetBySlug_ReturnsCorrectTenant(t *testing.T) {
	db := testDB(t)
	created := makeTenant(t, db)

	r := repo.NewTenantRepo(db)
	got, err := r.GetBySlug(created.Slug)

	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, created.Slug, got.Slug)
}

func TestTenantRepo_GetBySlug_ReturnsNotFoundForUnknownSlug(t *testing.T) {
	db := testDB(t)

	r := repo.NewTenantRepo(db)
	_, err := r.GetBySlug("slug-that-does-not-exist")

	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ── ListActive ────────────────────────────────────────────────────────────────

func TestTenantRepo_ListActive_ReturnsOnlyActiveTenants(t *testing.T) {
	db := testDB(t)
	active1 := makeTenant(t, db) // is_active = true by default
	active2 := makeTenant(t, db)

	// Create an inactive tenant by soft-deleting through the repo
	inactive := makeTenant(t, db)
	r := repo.NewTenantRepo(db)
	require.NoError(t, r.Delete(inactive.ID))

	tenants, err := r.ListActive(0, 50)

	require.NoError(t, err)
	ids := make(map[uuid.UUID]bool)
	for _, tn := range tenants {
		ids[tn.ID] = true
	}
	assert.True(t, ids[active1.ID], "active tenant 1 must appear in ListActive")
	assert.True(t, ids[active2.ID], "active tenant 2 must appear in ListActive")
	assert.False(t, ids[inactive.ID], "inactive tenant must not appear in ListActive")
}

func TestTenantRepo_ListActive_PaginationOffsetAndLimit(t *testing.T) {
	db := testDB(t)
	// Create 5 active tenants in this test's transaction
	for i := 0; i < 5; i++ {
		makeTenant(t, db)
	}

	r := repo.NewTenantRepo(db)

	page1, err := r.ListActive(0, 3)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(page1), 3, "page 1 must respect limit of 3")

	page2, err := r.ListActive(3, 3)
	require.NoError(t, err)

	ids1 := make(map[uuid.UUID]bool)
	for _, tn := range page1 {
		ids1[tn.ID] = true
	}
	for _, tn := range page2 {
		assert.False(t, ids1[tn.ID], "page 2 must not repeat records from page 1")
	}
}

// ── ListAll (AdminTenantRepository) ──────────────────────────────────────────

func TestTenantRepo_ListAll_IncludesInactiveTenants(t *testing.T) {
	db := testDB(t)
	active := makeTenant(t, db)
	inactive := makeTenant(t, db)

	r := repo.NewTenantRepo(db)
	require.NoError(t, r.Delete(inactive.ID)) // soft-delete via repo

	all, err := r.ListAll(0, 100)
	require.NoError(t, err)

	ids := make(map[uuid.UUID]bool)
	for _, tn := range all {
		ids[tn.ID] = true
	}
	assert.True(t, ids[active.ID], "ListAll must include active tenant")
	assert.True(t, ids[inactive.ID], "ListAll must include inactive (soft-deleted) tenant")
}

// ── CountAll (AdminTenantRepository) ─────────────────────────────────────────

func TestTenantRepo_CountAll_ReflectsAllTenants(t *testing.T) {
	db := testDB(t)

	r := repo.NewTenantRepo(db)

	// Baseline count before creating anything in this transaction
	before, err := r.CountAll()
	require.NoError(t, err)

	makeTenant(t, db)
	makeTenant(t, db)

	after, err := r.CountAll()
	require.NoError(t, err)

	assert.Equal(t, before+2, after, "CountAll must increase by 2 after creating 2 tenants")
}

func TestTenantRepo_CountAll_IncludesInactiveTenants(t *testing.T) {
	db := testDB(t)
	inactive := makeTenant(t, db)

	r := repo.NewTenantRepo(db)
	require.NoError(t, r.Delete(inactive.ID))

	before, err := r.CountAll()
	require.NoError(t, err)

	makeTenant(t, db)

	after, err := r.CountAll()
	require.NoError(t, err)

	assert.Equal(t, before+1, after, "CountAll must count even after soft-deleting a tenant")
}

// ── Update ────────────────────────────────────────────────────────────────────

func TestTenantRepo_Update_PersistsNameChange(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	tenant.Name = "Renamed Corp"
	tenant.Tier = "pro"

	r := repo.NewTenantRepo(db)
	require.NoError(t, r.Update(tenant))

	fetched, err := r.GetByID(tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, "Renamed Corp", fetched.Name)
	assert.Equal(t, "pro", fetched.Tier)
}

func TestTenantRepo_Update_DoesNotAffectOtherTenants(t *testing.T) {
	db := testDB(t)
	tenantA := makeTenant(t, db)
	tenantB := makeTenant(t, db)
	originalBName := tenantB.Name

	tenantA.Name = "Updated A"
	r := repo.NewTenantRepo(db)
	require.NoError(t, r.Update(tenantA))

	fetchedB, err := r.GetByID(tenantB.ID)
	require.NoError(t, err)
	assert.Equal(t, originalBName, fetchedB.Name)
}

// ── Delete (soft-delete) ──────────────────────────────────────────────────────

func TestTenantRepo_Delete_MarksTenantInactive(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	require.True(t, tenant.IsActive)

	r := repo.NewTenantRepo(db)
	require.NoError(t, r.Delete(tenant.ID))

	// The row still exists — GetByID must find it (no hard delete)
	fetched, err := r.GetByID(tenant.ID)
	require.NoError(t, err)
	assert.False(t, fetched.IsActive, "Delete must set is_active = false")
}

func TestTenantRepo_Delete_IsIdempotent(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)

	r := repo.NewTenantRepo(db)
	require.NoError(t, r.Delete(tenant.ID))
	// Second delete on an already-inactive tenant must not error
	require.NoError(t, r.Delete(tenant.ID))
}

func TestTenantRepo_Delete_UnknownIDDoesNotError(t *testing.T) {
	db := testDB(t)

	r := repo.NewTenantRepo(db)
	// GORM's Update with WHERE on a non-existent row succeeds with 0 rows affected
	err := r.Delete(uuid.New())
	assert.NoError(t, err)
}
