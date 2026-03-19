package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"kerplan/internal/event"
	"kerplan/internal/model"
)

// ── Mock implementations ──────────────────────────────────────────

type MockUserRepo struct {
	users map[string]*model.User // key: tenantID:userID or externalID
}

func NewMockUserRepo() *MockUserRepo {
	return &MockUserRepo{users: make(map[string]*model.User)}
}

func (m *MockUserRepo) Create(user *model.User) error {
	m.users[user.TenantID.String()+":"+user.ID.String()] = user
	if user.ExternalID != "" {
		m.users["ext:"+user.ExternalID] = user
	}
	if user.Email != "" {
		m.users[user.TenantID.String()+":email:"+user.Email] = user
	}
	return nil
}

func (m *MockUserRepo) GetByID(tenantID, userID uuid.UUID) (*model.User, error) {
	return m.users[tenantID.String()+":"+userID.String()], nil
}

func (m *MockUserRepo) GetByExternalID(externalID string) (*model.User, error) {
	u := m.users["ext:"+externalID]
	if u == nil {
		return nil, &notFoundErr{}
	}
	return u, nil
}

func (m *MockUserRepo) GetByEmail(tenantID uuid.UUID, email string) (*model.User, error) {
	return m.users[tenantID.String()+":email:"+email], nil
}

func (m *MockUserRepo) ListByTenant(tenantID uuid.UUID, offset, limit int) ([]*model.User, int64, error) {
	var result []*model.User
	for _, u := range m.users {
		if u.TenantID == tenantID {
			// Deduplicate (user may be stored under multiple keys)
			found := false
			for _, r := range result {
				if r.ID == u.ID {
					found = true
					break
				}
			}
			if !found {
				result = append(result, u)
			}
		}
	}
	return result, int64(len(result)), nil
}

func (m *MockUserRepo) Update(user *model.User) error {
	m.users[user.TenantID.String()+":"+user.ID.String()] = user
	if user.ExternalID != "" {
		m.users["ext:"+user.ExternalID] = user
	}
	return nil
}

func (m *MockUserRepo) CountByTenant(tenantID uuid.UUID) (int64, error) {
	count := int64(0)
	seen := make(map[uuid.UUID]bool)
	for _, u := range m.users {
		if u.TenantID == tenantID && u.IsActive && !seen[u.ID] {
			count++
			seen[u.ID] = true
		}
	}
	return count, nil
}

type notFoundErr struct{}

func (e *notFoundErr) Error() string { return "not found" }

// MockTenantRepo for UserService tests
type MockTenantRepoForUsers struct {
	tenants map[uuid.UUID]*model.Tenant
}

func NewMockTenantRepoForUsers() *MockTenantRepoForUsers {
	return &MockTenantRepoForUsers{tenants: make(map[uuid.UUID]*model.Tenant)}
}

func (m *MockTenantRepoForUsers) Create(t *model.Tenant) error {
	m.tenants[t.ID] = t
	return nil
}
func (m *MockTenantRepoForUsers) GetByID(id uuid.UUID) (*model.Tenant, error) {
	return m.tenants[id], nil
}
func (m *MockTenantRepoForUsers) GetBySlug(slug string) (*model.Tenant, error) { return nil, nil }
func (m *MockTenantRepoForUsers) ListActive(offset, limit int) ([]*model.Tenant, error) {
	return nil, nil
}
func (m *MockTenantRepoForUsers) Update(t *model.Tenant) error  { return nil }
func (m *MockTenantRepoForUsers) Delete(id uuid.UUID) error     { return nil }

func newTestUserService() (*UserService, *MockUserRepo, *MockTenantRepoForUsers) {
	userRepo := NewMockUserRepo()
	tenantRepo := NewMockTenantRepoForUsers()
	logger := logrus.NewEntry(logrus.New())
	emitter := event.NewEmitter(logger)

	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	tenantRepo.Create(&model.Tenant{
		ID:       tenantID,
		Name:     "Test Tenant",
		Slug:     "test",
		IsActive: true,
		MaxUsers: 10,
		MaxPlans: 5,
		Tier:     "free",
	})

	svc := NewUserService(userRepo, tenantRepo, emitter, logger)
	return svc, userRepo, tenantRepo
}

// ── Tests ─────────────────────────────────────────────────────────

func TestGetOrCreateUserFirstUserBecomesOwner(t *testing.T) {
	svc, _, _ := newTestUserService()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	user, err := svc.GetOrCreateUser(context.Background(), tenantID, "socrate-1", "first@test.com", "First User")

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "owner", user.Role)
	assert.Equal(t, "first@test.com", user.Email)
	assert.NotNil(t, user.JoinedAt)
}

func TestGetOrCreateUserSubsequentUserBecomesViewer(t *testing.T) {
	svc, _, _ := newTestUserService()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	// First user = owner
	svc.GetOrCreateUser(context.Background(), tenantID, "socrate-1", "first@test.com", "First")

	// Second user = viewer
	user, err := svc.GetOrCreateUser(context.Background(), tenantID, "socrate-2", "second@test.com", "Second")

	assert.NoError(t, err)
	assert.Equal(t, "user", user.Role)
}

func TestGetOrCreateUserReturnsExistingOnSecondLogin(t *testing.T) {
	svc, _, _ := newTestUserService()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	user1, _ := svc.GetOrCreateUser(context.Background(), tenantID, "socrate-1", "user@test.com", "User")
	user2, _ := svc.GetOrCreateUser(context.Background(), tenantID, "socrate-1", "user@test.com", "User")

	assert.Equal(t, user1.ID, user2.ID)
}

func TestGetOrCreateUserClaimsPendingInvite(t *testing.T) {
	svc, userRepo, _ := newTestUserService()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	// Create first user so next one isn't auto-owner
	svc.GetOrCreateUser(context.Background(), tenantID, "socrate-owner", "owner@test.com", "Owner")

	// Create a pending invite (no ExternalID)
	invitedBy := uuid.New()
	invite := &model.User{
		ID:        uuid.New(),
		TenantID:  tenantID,
		Email:     "invited@test.com",
		Role:      "editor",
		IsActive:  true,
		InvitedBy: &invitedBy,
	}
	userRepo.Create(invite)

	// Now the invited user logs in
	user, err := svc.GetOrCreateUser(context.Background(), tenantID, "socrate-invited", "invited@test.com", "Invited User")

	assert.NoError(t, err)
	assert.Equal(t, invite.ID, user.ID)
	assert.Equal(t, "editor", user.Role) // Gets the invited role
	assert.Equal(t, "socrate-invited", user.ExternalID)
	assert.NotNil(t, user.JoinedAt)
}

func TestInviteUserSuccess(t *testing.T) {
	svc, _, _ := newTestUserService()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	invitedBy := uuid.New()

	user, err := svc.InviteUser(context.Background(), tenantID, invitedBy, "new@test.com", "editor")

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "new@test.com", user.Email)
	assert.Equal(t, "editor", user.Role)
	assert.Equal(t, &invitedBy, user.InvitedBy)
	assert.Nil(t, user.JoinedAt) // Not yet joined
}

func TestInviteUserRejectsInvalidRole(t *testing.T) {
	svc, _, _ := newTestUserService()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	_, err := svc.InviteUser(context.Background(), tenantID, uuid.New(), "new@test.com", "owner")

	assert.Error(t, err)
}

func TestInviteUserRejectsDuplicateEmail(t *testing.T) {
	svc, _, _ := newTestUserService()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	svc.InviteUser(context.Background(), tenantID, uuid.New(), "dup@test.com", "viewer")
	_, err := svc.InviteUser(context.Background(), tenantID, uuid.New(), "dup@test.com", "editor")

	assert.Error(t, err)
}

func TestUpdateRoleSuccess(t *testing.T) {
	svc, userRepo, _ := newTestUserService()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	user := &model.User{
		ID: uuid.New(), TenantID: tenantID, Email: "user@test.com",
		Role: "viewer", IsActive: true,
	}
	userRepo.Create(user)

	err := svc.UpdateRole(context.Background(), tenantID, user.ID, "owner", "editor")
	assert.NoError(t, err)

	updated, _ := userRepo.GetByID(tenantID, user.ID)
	assert.Equal(t, "editor", updated.Role)
}

func TestUpdateRoleCannotChangeOwner(t *testing.T) {
	svc, userRepo, _ := newTestUserService()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	owner := &model.User{
		ID: uuid.New(), TenantID: tenantID, Email: "owner@test.com",
		Role: "owner", IsActive: true,
	}
	userRepo.Create(owner)

	err := svc.UpdateRole(context.Background(), tenantID, owner.ID, "owner", "editor")
	assert.Error(t, err)
}

func TestUpdateRoleCannotPromoteToOwner(t *testing.T) {
	svc, userRepo, _ := newTestUserService()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	user := &model.User{
		ID: uuid.New(), TenantID: tenantID, Email: "user@test.com",
		Role: "editor", IsActive: true,
	}
	userRepo.Create(user)

	err := svc.UpdateRole(context.Background(), tenantID, user.ID, "owner", "owner")
	assert.Error(t, err)
}

func TestOnlyOwnerCanPromoteToAdmin(t *testing.T) {
	svc, userRepo, _ := newTestUserService()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	user := &model.User{
		ID: uuid.New(), TenantID: tenantID, Email: "user@test.com",
		Role: "editor", IsActive: true,
	}
	userRepo.Create(user)

	// Admin caller cannot promote to admin
	err := svc.UpdateRole(context.Background(), tenantID, user.ID, "admin", "admin")
	assert.Error(t, err)

	// Owner caller can promote to admin
	err = svc.UpdateRole(context.Background(), tenantID, user.ID, "owner", "admin")
	assert.NoError(t, err)
}

func TestDeactivateUser(t *testing.T) {
	svc, userRepo, _ := newTestUserService()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	user := &model.User{
		ID: uuid.New(), TenantID: tenantID, Email: "user@test.com",
		Role: "editor", IsActive: true,
	}
	userRepo.Create(user)

	err := svc.DeactivateUser(context.Background(), tenantID, user.ID)
	assert.NoError(t, err)

	updated, _ := userRepo.GetByID(tenantID, user.ID)
	assert.False(t, updated.IsActive)
}

func TestCannotDeactivateOwner(t *testing.T) {
	svc, userRepo, _ := newTestUserService()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	owner := &model.User{
		ID: uuid.New(), TenantID: tenantID, Email: "owner@test.com",
		Role: "owner", IsActive: true,
	}
	userRepo.Create(owner)

	err := svc.DeactivateUser(context.Background(), tenantID, owner.ID)
	assert.Error(t, err)
}

func TestReactivateUser(t *testing.T) {
	svc, userRepo, _ := newTestUserService()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	user := &model.User{
		ID: uuid.New(), TenantID: tenantID, Email: "user@test.com",
		Role: "editor", IsActive: false,
	}
	userRepo.Create(user)

	err := svc.ReactivateUser(context.Background(), tenantID, user.ID)
	assert.NoError(t, err)

	updated, _ := userRepo.GetByID(tenantID, user.ID)
	assert.True(t, updated.IsActive)
}

func TestTransferOwnership(t *testing.T) {
	svc, userRepo, _ := newTestUserService()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	owner := &model.User{
		ID: uuid.New(), TenantID: tenantID, Role: "owner", IsActive: true,
	}
	admin := &model.User{
		ID: uuid.New(), TenantID: tenantID, Role: "admin", IsActive: true,
	}
	userRepo.Create(owner)
	userRepo.Create(admin)

	err := svc.TransferOwnership(context.Background(), tenantID, owner.ID, admin.ID)
	assert.NoError(t, err)

	updatedOwner, _ := userRepo.GetByID(tenantID, owner.ID)
	updatedAdmin, _ := userRepo.GetByID(tenantID, admin.ID)
	assert.Equal(t, "admin", updatedOwner.Role)
	assert.Equal(t, "owner", updatedAdmin.Role)
}

func TestTransferOwnershipOnlyToAdmin(t *testing.T) {
	svc, userRepo, _ := newTestUserService()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	owner := &model.User{
		ID: uuid.New(), TenantID: tenantID, Role: "owner", IsActive: true,
	}
	editor := &model.User{
		ID: uuid.New(), TenantID: tenantID, Role: "editor", IsActive: true,
	}
	userRepo.Create(owner)
	userRepo.Create(editor)

	err := svc.TransferOwnership(context.Background(), tenantID, owner.ID, editor.ID)
	assert.Error(t, err) // Cannot transfer to non-admin
}

func TestListUsers(t *testing.T) {
	svc, userRepo, _ := newTestUserService()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	now := time.Now()
	userRepo.Create(&model.User{
		ID: uuid.New(), TenantID: tenantID, Email: "a@test.com", Role: "owner", IsActive: true, JoinedAt: &now,
	})
	userRepo.Create(&model.User{
		ID: uuid.New(), TenantID: tenantID, Email: "b@test.com", Role: "editor", IsActive: true, JoinedAt: &now,
	})

	users, total, err := svc.ListUsers(context.Background(), tenantID, 0, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, users, 2)
}
