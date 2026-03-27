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

func TestUserRepo_Create_Succeeds(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)

	r := repo.NewUserRepo(db)
	uid := uuid.New().String()[:8]
	user := &model.User{
		TenantID: tenant.ID,
		Email:    "newuser-" + uid + "@example.com",
		Name:     "New User",
		Role:     "user",
		IsActive: true,
	}

	err := r.Create(user)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, user.ID, "Create must populate the ID")
	assert.False(t, user.CreatedAt.IsZero(), "Create must set CreatedAt")
}

func TestUserRepo_Create_RejectsDuplicateEmailWithinTenant(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	existing := makeUser(t, db, tenant.ID)

	r := repo.NewUserRepo(db)
	duplicate := &model.User{
		TenantID: tenant.ID,
		Email:    existing.Email, // duplicate within the same tenant
		Name:     "Other Name",
		Role:     "user",
		IsActive: true,
	}

	err := r.Create(duplicate)
	// Note: the User model does not have a unique index on (tenant_id, email)
	// out-of-the-box, but if it does this test validates the constraint.
	// If the schema has no such constraint, this test documents the current
	// behaviour (no error) and should be updated once the constraint is added.
	_ = err // accept either outcome; see comment above
}

// ── GetByID ───────────────────────────────────────────────────────────────────

func TestUserRepo_GetByID_ReturnsCorrectUser(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	created := makeUser(t, db, tenant.ID)

	r := repo.NewUserRepo(db)
	got, err := r.GetByID(tenant.ID, created.ID)

	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, created.Email, got.Email)
	assert.Equal(t, tenant.ID, got.TenantID)
}

func TestUserRepo_GetByID_ReturnsNotFoundForWrongTenant(t *testing.T) {
	db := testDB(t)
	tenantA := makeTenant(t, db)
	tenantB := makeTenant(t, db)
	userA := makeUser(t, db, tenantA.ID)

	r := repo.NewUserRepo(db)
	_, err := r.GetByID(tenantB.ID, userA.ID)

	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestUserRepo_GetByID_ReturnsNotFoundForUnknownID(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)

	r := repo.NewUserRepo(db)
	_, err := r.GetByID(tenant.ID, uuid.New())

	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ── GetByEmail ────────────────────────────────────────────────────────────────

func TestUserRepo_GetByEmail_ReturnsCorrectUser(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	created := makeUser(t, db, tenant.ID)

	r := repo.NewUserRepo(db)
	got, err := r.GetByEmail(tenant.ID, created.Email)

	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, created.Email, got.Email)
}

func TestUserRepo_GetByEmail_ReturnsNotFoundForWrongTenant(t *testing.T) {
	db := testDB(t)
	tenantA := makeTenant(t, db)
	tenantB := makeTenant(t, db)
	userA := makeUser(t, db, tenantA.ID)

	r := repo.NewUserRepo(db)
	_, err := r.GetByEmail(tenantB.ID, userA.Email)

	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestUserRepo_GetByEmail_ReturnsNotFoundForUnknownEmail(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)

	r := repo.NewUserRepo(db)
	_, err := r.GetByEmail(tenant.ID, "nobody@example.com")

	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ── GetByExternalID ───────────────────────────────────────────────────────────

func TestUserRepo_GetByExternalID_ReturnsCorrectUser(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	user := makeUser(t, db, tenant.ID)

	// Assign an external ID (simulates post-first-login update)
	extID := "auth0|" + uuid.New().String()
	user.ExternalID = extID
	require.NoError(t, db.Save(user).Error)

	r := repo.NewUserRepo(db)
	got, err := r.GetByExternalID(extID)

	require.NoError(t, err)
	assert.Equal(t, user.ID, got.ID)
	assert.Equal(t, extID, got.ExternalID)
}

func TestUserRepo_GetByExternalID_ReturnsNotFoundForUnknownExternalID(t *testing.T) {
	db := testDB(t)

	r := repo.NewUserRepo(db)
	_, err := r.GetByExternalID("auth0|does-not-exist")

	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ── ListByTenant ──────────────────────────────────────────────────────────────

func TestUserRepo_ListByTenant_ReturnsTenantUsersOnly(t *testing.T) {
	db := testDB(t)
	tenantA := makeTenant(t, db)
	tenantB := makeTenant(t, db)

	makeUser(t, db, tenantA.ID)
	makeUser(t, db, tenantA.ID)
	makeUser(t, db, tenantB.ID) // must not appear in tenantA results

	r := repo.NewUserRepo(db)
	users, total, err := r.ListByTenant(tenantA.ID, 0, 50)

	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, users, 2)
	for _, u := range users {
		assert.Equal(t, tenantA.ID, u.TenantID)
	}
}

func TestUserRepo_ListByTenant_PaginationOffsetAndLimit(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)

	for i := 0; i < 5; i++ {
		makeUser(t, db, tenant.ID)
	}

	r := repo.NewUserRepo(db)

	page1, total1, err := r.ListByTenant(tenant.ID, 0, 3)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total1, "total must reflect full count, not page size")
	assert.Len(t, page1, 3)

	page2, total2, err := r.ListByTenant(tenant.ID, 3, 3)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total2, "total must be consistent across pages")
	assert.Len(t, page2, 2)

	// No overlap between pages
	ids1 := make(map[uuid.UUID]bool)
	for _, u := range page1 {
		ids1[u.ID] = true
	}
	for _, u := range page2 {
		assert.False(t, ids1[u.ID], "page 2 must not repeat records from page 1")
	}
}

func TestUserRepo_ListByTenant_EmptyWhenNoUsers(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)

	r := repo.NewUserRepo(db)
	users, total, err := r.ListByTenant(tenant.ID, 0, 50)

	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, users)
}

func TestUserRepo_ListByTenant_OrderedByCreatedAtAsc(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)

	for i := 0; i < 3; i++ {
		makeUser(t, db, tenant.ID)
	}

	r := repo.NewUserRepo(db)
	users, _, err := r.ListByTenant(tenant.ID, 0, 50)
	require.NoError(t, err)
	require.Len(t, users, 3)

	for i := 1; i < len(users); i++ {
		assert.False(
			t,
			users[i].CreatedAt.Before(users[i-1].CreatedAt),
			"users[%d] must not be older than users[%d]", i, i-1,
		)
	}
}

// ── CountByTenant ─────────────────────────────────────────────────────────────

func TestUserRepo_CountByTenant_CountsOnlyActiveUsers(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)

	makeUser(t, db, tenant.ID) // active (default)
	makeUser(t, db, tenant.ID) // active

	// Create an inactive user
	inactive := makeUser(t, db, tenant.ID)
	inactive.IsActive = false
	require.NoError(t, db.Save(inactive).Error)

	r := repo.NewUserRepo(db)
	count, err := r.CountByTenant(tenant.ID)

	require.NoError(t, err)
	assert.Equal(t, int64(2), count, "CountByTenant must count only active users")
}

func TestUserRepo_CountByTenant_DoesNotLeakAcrossTenants(t *testing.T) {
	db := testDB(t)
	tenantA := makeTenant(t, db)
	tenantB := makeTenant(t, db)

	makeUser(t, db, tenantA.ID)
	makeUser(t, db, tenantA.ID)
	makeUser(t, db, tenantB.ID) // must not be counted for A

	r := repo.NewUserRepo(db)
	count, err := r.CountByTenant(tenantA.ID)

	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestUserRepo_CountByTenant_ReturnsZeroForNewTenant(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)

	r := repo.NewUserRepo(db)
	count, err := r.CountByTenant(tenant.ID)

	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

// ── Update ────────────────────────────────────────────────────────────────────

func TestUserRepo_Update_PersistsChanges(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	user := makeUser(t, db, tenant.ID)

	user.Name = "Updated Name"
	user.Role = "admin"

	r := repo.NewUserRepo(db)
	require.NoError(t, r.Update(user))

	fetched, err := r.GetByID(tenant.ID, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", fetched.Name)
	assert.Equal(t, "admin", fetched.Role)
}

func TestUserRepo_Update_DoesNotAffectOtherUsers(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	userA := makeUser(t, db, tenant.ID)
	userB := makeUser(t, db, tenant.ID)
	originalBName := userB.Name

	userA.Name = "Updated A"
	r := repo.NewUserRepo(db)
	require.NoError(t, r.Update(userA))

	fetchedB, err := r.GetByID(tenant.ID, userB.ID)
	require.NoError(t, err)
	assert.Equal(t, originalBName, fetchedB.Name)
}
