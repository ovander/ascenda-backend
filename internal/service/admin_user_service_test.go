package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/socrate"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Mock Socrate client ───────────────────────────────────────────────────────
// mockSocrateClient satisfies SocrateUserManager.
// Socrate only knows two roles per app: "admin" or "user".

type mockSocrateClient struct {
	users  map[string]*socrate.User
	nextID uint
	err    error // if set, all calls return this error
}

func newMockSocrateClient() *mockSocrateClient {
	return &mockSocrateClient{
		users:  make(map[string]*socrate.User),
		nextID: 1,
	}
}

func (m *mockSocrateClient) addUser(u socrate.User) {
	key := fmt.Sprintf("%d", u.ID)
	m.users[key] = &u
}

func (m *mockSocrateClient) ListUsers(_ context.Context, search string, page, pageSize int) (*socrate.UserListResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	var all []socrate.User
	for _, u := range m.users {
		if search == "" || strContains(u.Email, search) || strContains(u.Name, search) {
			all = append(all, *u)
		}
	}
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > len(all) {
		start = len(all)
	}
	if end > len(all) {
		end = len(all)
	}
	return &socrate.UserListResponse{
		Users:      all[start:end],
		TotalCount: int64(len(all)),
		Page:       page,
		PageSize:   pageSize,
	}, nil
}

func (m *mockSocrateClient) GetUser(_ context.Context, userID string) (*socrate.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.users[userID], nil
}

func (m *mockSocrateClient) CreateUser(_ context.Context, req socrate.CreateUserRequest) (*socrate.CreateUserResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, u := range m.users {
		if u.Email == req.Email {
			return nil, socrate.ErrUserAlreadyExists
		}
	}
	u := &socrate.User{
		ID:        m.nextID,
		Email:     req.Email,
		Name:      req.Name,
		Role:      req.Role,
		CreatedAt: time.Now(),
	}
	m.nextID++
	m.users[fmt.Sprintf("%d", u.ID)] = u
	return &socrate.CreateUserResult{UserID: u.ID, Role: u.Role}, nil
}

func (m *mockSocrateClient) UpdateUserRole(_ context.Context, userID string, role string) error {
	if m.err != nil {
		return m.err
	}
	u, ok := m.users[userID]
	if !ok {
		return errors.New("not found")
	}
	u.Role = role
	return nil
}

func (m *mockSocrateClient) DeleteUser(_ context.Context, userID string) error {
	if m.err != nil {
		return m.err
	}
	delete(m.users, userID)
	return nil
}

func (m *mockSocrateClient) ResendVerification(_ context.Context, _ string) error {
	return m.err
}

func (m *mockSocrateClient) ForcePasswordReset(_ context.Context, _ string) error {
	return m.err
}

// strContains is a simple substring helper used only by the mock.
func strContains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(s) < len(sub) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ── Mock user repo ────────────────────────────────────────────────────────────

type mockAdminUserRepo struct {
	byExternalID map[string]*model.User
	byID         map[uuid.UUID]*model.User
}

func newMockAdminUserRepo() *mockAdminUserRepo {
	return &mockAdminUserRepo{
		byExternalID: make(map[string]*model.User),
		byID:         make(map[uuid.UUID]*model.User),
	}
}

func (m *mockAdminUserRepo) addUser(u *model.User) {
	m.byID[u.ID] = u
	if u.ExternalID != "" {
		m.byExternalID[u.ExternalID] = u
	}
}

// Satisfy repo.UserRepository
func (m *mockAdminUserRepo) Create(u *model.User) error {
	m.byID[u.ID] = u
	if u.ExternalID != "" {
		m.byExternalID[u.ExternalID] = u
	}
	return nil
}
func (m *mockAdminUserRepo) GetByID(_ uuid.UUID, id uuid.UUID) (*model.User, error) {
	u, ok := m.byID[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}
func (m *mockAdminUserRepo) GetPendingInviteByEmail(email string) (*model.User, error) {
	return nil, errors.New("not found")
}
func (m *mockAdminUserRepo) GetByExternalID(extID string) (*model.User, error) {
	u, ok := m.byExternalID[extID]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}
func (m *mockAdminUserRepo) GetByEmail(_ uuid.UUID, email string) (*model.User, error) {
	for _, u := range m.byID {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, errors.New("not found")
}
func (m *mockAdminUserRepo) ListByTenant(_ uuid.UUID, _, _ int) ([]*model.User, int64, error) {
	return nil, 0, nil
}
func (m *mockAdminUserRepo) Update(u *model.User) error {
	m.byID[u.ID] = u
	if u.ExternalID != "" {
		m.byExternalID[u.ExternalID] = u
	}
	return nil
}
func (m *mockAdminUserRepo) CountByTenant(_ uuid.UUID) (int64, error) {
	return 0, nil
}

// ── Mock admin tenant repo ────────────────────────────────────────────────────

type mockAdminTenantRepo struct {
	tenants map[uuid.UUID]*model.Tenant
}

func newMockAdminTenantRepo() *mockAdminTenantRepo {
	return &mockAdminTenantRepo{tenants: make(map[uuid.UUID]*model.Tenant)}
}

func (m *mockAdminTenantRepo) Create(t *model.Tenant) error {
	m.tenants[t.ID] = t
	return nil
}
func (m *mockAdminTenantRepo) GetByID(id uuid.UUID) (*model.Tenant, error) {
	t, ok := m.tenants[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return t, nil
}
func (m *mockAdminTenantRepo) GetBySlug(slug string) (*model.Tenant, error) {
	for _, t := range m.tenants {
		if t.Slug == slug {
			return t, nil
		}
	}
	return nil, errors.New("not found")
}
func (m *mockAdminTenantRepo) ListActive(offset, limit int) ([]*model.Tenant, error) {
	return nil, nil
}
func (m *mockAdminTenantRepo) ListAll(offset, limit int) ([]*model.Tenant, error) {
	var all []*model.Tenant
	for _, t := range m.tenants {
		all = append(all, t)
	}
	if offset >= len(all) {
		return []*model.Tenant{}, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], nil
}
func (m *mockAdminTenantRepo) CountAll() (int64, error) {
	return int64(len(m.tenants)), nil
}
func (m *mockAdminTenantRepo) Update(t *model.Tenant) error {
	m.tenants[t.ID] = t
	return nil
}
func (m *mockAdminTenantRepo) Delete(id uuid.UUID) error {
	delete(m.tenants, id)
	return nil
}

// ── Test helpers ──────────────────────────────────────────────────────────────

func newTestAdminUserService(sc SocrateUserManager) (*AdminUserService, *mockAdminUserRepo, *mockAdminTenantRepo) {
	userRepo := newMockAdminUserRepo()
	tenantRepo := newMockAdminTenantRepo()
	logger := logrus.NewEntry(logrus.New())
	svc := NewAdminUserService(sc, userRepo, tenantRepo, logger)
	return svc, userRepo, tenantRepo
}

// ── Socrate-proxy user tests ──────────────────────────────────────────────────

func TestAdminUserService_NoSocrateClient(t *testing.T) {
	svc, _, _ := newTestAdminUserService(nil)
	ctx := context.Background()

	_, err := svc.ListUsers(ctx, "", 1, 10)
	assert.Error(t, err, "ListUsers should fail without Socrate")

	_, err = svc.GetUser(ctx, "1")
	assert.Error(t, err, "GetUser should fail without Socrate")

	_, err = svc.CreateUser(ctx, CreateUserRequest{Email: "a@b.com", FullName: "Alice"})
	assert.Error(t, err, "CreateUser should fail without Socrate")

	err = svc.DeleteUser(ctx, "1")
	assert.Error(t, err, "DeleteUser should fail without Socrate")

	err = svc.ResendVerification(ctx, "1")
	assert.Error(t, err, "ResendVerification should fail without Socrate")

	err = svc.ResetPassword(ctx, "1")
	assert.Error(t, err, "ResetPassword should fail without Socrate")
}

func TestAdminUserService_ListUsers_Empty(t *testing.T) {
	sc := newMockSocrateClient()
	svc, _, _ := newTestAdminUserService(sc)

	result, err := svc.ListUsers(context.Background(), "", 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.TotalCount)
	assert.Empty(t, result.Users)
}

func TestAdminUserService_ListUsers_WithData(t *testing.T) {
	sc := newMockSocrateClient()
	sc.addUser(socrate.User{ID: 1, Email: "alice@example.com", Name: "Alice", Role: "user"})
	sc.addUser(socrate.User{ID: 2, Email: "bob@example.com", Name: "Bob", Role: "admin"})

	svc, _, _ := newTestAdminUserService(sc)

	result, err := svc.ListUsers(context.Background(), "", 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.TotalCount)
	assert.Len(t, result.Users, 2)
}

func TestAdminUserService_ListUsers_Search(t *testing.T) {
	sc := newMockSocrateClient()
	sc.addUser(socrate.User{ID: 1, Email: "alice@example.com", Name: "Alice"})
	sc.addUser(socrate.User{ID: 2, Email: "bob@example.com", Name: "Bob"})

	svc, _, _ := newTestAdminUserService(sc)

	result, err := svc.ListUsers(context.Background(), "alice", 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.TotalCount)
	assert.Equal(t, "alice@example.com", result.Users[0].Email)
}

func TestAdminUserService_ListUsers_EnrichesWithAscendaData(t *testing.T) {
	sc := newMockSocrateClient()
	sc.addUser(socrate.User{ID: 5, Email: "owner@example.com", Name: "Owner", Role: "user"})

	svc, userRepo, tenantRepo := newTestAdminUserService(sc)

	// Tenant and matching Ascenda user record
	tenantID := uuid.New()
	tenantRepo.Create(&model.Tenant{ID: tenantID, Name: "Acme Corp", Slug: "acme", IsActive: true})
	ascendaUser := &model.User{
		ID:         uuid.New(),
		TenantID:   tenantID,
		ExternalID: "5", // matches Socrate ID 5
		Email:      "owner@example.com",
		Role:       "owner",
		IsActive:   true,
	}
	userRepo.addUser(ascendaUser)

	result, err := svc.ListUsers(context.Background(), "", 1, 10)
	require.NoError(t, err)
	require.Len(t, result.Users, 1)

	dto := result.Users[0]
	assert.Equal(t, "user", dto.Role) // Socrate role
	require.NotNil(t, dto.AscendaRole)
	assert.Equal(t, "owner", *dto.AscendaRole) // Ascenda local role
	require.NotNil(t, dto.TenantName)
	assert.Equal(t, "Acme Corp", *dto.TenantName)
}

func TestAdminUserService_ListUsers_SocrateError(t *testing.T) {
	sc := newMockSocrateClient()
	sc.err = errors.New("socrate unavailable")
	svc, _, _ := newTestAdminUserService(sc)

	_, err := svc.ListUsers(context.Background(), "", 1, 10)
	assert.Error(t, err)
}

func TestAdminUserService_GetUser_Found(t *testing.T) {
	sc := newMockSocrateClient()
	sc.addUser(socrate.User{ID: 3, Email: "carol@example.com", Name: "Carol", Role: "user"})
	svc, _, _ := newTestAdminUserService(sc)

	dto, err := svc.GetUser(context.Background(), "3")
	require.NoError(t, err)
	assert.Equal(t, "carol@example.com", dto.Email)
	assert.Equal(t, uint(3), dto.SocrateID)
}

func TestAdminUserService_GetUser_NotFound(t *testing.T) {
	sc := newMockSocrateClient()
	svc, _, _ := newTestAdminUserService(sc)

	dto, err := svc.GetUser(context.Background(), "999")
	assert.Error(t, err)
	assert.Nil(t, dto)
}

func TestAdminUserService_CreateUser_Success(t *testing.T) {
	sc := newMockSocrateClient()
	svc, _, _ := newTestAdminUserService(sc)

	dto, err := svc.CreateUser(context.Background(), CreateUserRequest{
		Email:    "new@example.com",
		FullName: "New User",
		Role:     "user",
	})
	require.NoError(t, err)
	assert.Equal(t, "new@example.com", dto.Email)
	assert.Equal(t, "user", dto.Role)
}

func TestAdminUserService_CreateUser_AdminRole(t *testing.T) {
	sc := newMockSocrateClient()
	svc, _, _ := newTestAdminUserService(sc)

	dto, err := svc.CreateUser(context.Background(), CreateUserRequest{
		Email:    "admin@example.com",
		FullName: "Admin User",
		Role:     "admin",
	})
	require.NoError(t, err)
	assert.Equal(t, "admin", dto.Role)
}

func TestAdminUserService_CreateUser_DefaultsToUserRole(t *testing.T) {
	sc := newMockSocrateClient()
	svc, _, _ := newTestAdminUserService(sc)

	// No role specified → should default to "user"
	dto, err := svc.CreateUser(context.Background(), CreateUserRequest{
		Email:    "notrole@example.com",
		FullName: "No Role",
	})
	require.NoError(t, err)
	assert.Equal(t, "user", dto.Role)
}

func TestAdminUserService_CreateUser_InvalidSocrateRole(t *testing.T) {
	sc := newMockSocrateClient()
	svc, _, _ := newTestAdminUserService(sc)

	// Ascenda roles like "owner" are NOT valid Socrate roles
	_, err := svc.CreateUser(context.Background(), CreateUserRequest{
		Email:    "owner@example.com",
		FullName: "Owner User",
		Role:     "owner",
	})
	assert.Error(t, err, "should reject Ascenda-only role 'owner'")

	_, err = svc.CreateUser(context.Background(), CreateUserRequest{
		Email:    "viewer@example.com",
		FullName: "Viewer User",
		Role:     "viewer",
	})
	assert.Error(t, err, "should reject plan-level role 'viewer'")
}

func TestAdminUserService_CreateUser_DuplicateEmail(t *testing.T) {
	sc := newMockSocrateClient()
	sc.addUser(socrate.User{ID: 1, Email: "dup@example.com"})
	svc, _, _ := newTestAdminUserService(sc)

	_, err := svc.CreateUser(context.Background(), CreateUserRequest{
		Email:    "dup@example.com",
		FullName: "Duplicate",
		Role:     "user",
	})
	assert.Error(t, err)
}

func TestAdminUserService_UpdateUser_FullName(t *testing.T) {
	sc := newMockSocrateClient()
	sc.addUser(socrate.User{ID: 7, Email: "dave@example.com", Name: "Dave", Role: "user"})
	svc, _, _ := newTestAdminUserService(sc)

	newName := "David"
	dto, err := svc.UpdateUser(context.Background(), "7", UpdateUserRequest{FullName: &newName})
	require.NoError(t, err)
	assert.Equal(t, "David", dto.Name)
}

func TestAdminUserService_UpdateUser_PromoteToAdmin(t *testing.T) {
	sc := newMockSocrateClient()
	sc.addUser(socrate.User{ID: 8, Email: "user@example.com", Name: "User", Role: "user"})
	svc, _, _ := newTestAdminUserService(sc)

	adminRole := "admin"
	dto, err := svc.UpdateUser(context.Background(), "8", UpdateUserRequest{Role: &adminRole})
	require.NoError(t, err)
	assert.Equal(t, "admin", dto.Role)
}

func TestAdminUserService_UpdateUser_InvalidRole(t *testing.T) {
	sc := newMockSocrateClient()
	sc.addUser(socrate.User{ID: 9, Email: "test@example.com", Name: "Test", Role: "user"})
	svc, _, _ := newTestAdminUserService(sc)

	ownerRole := "owner"
	_, err := svc.UpdateUser(context.Background(), "9", UpdateUserRequest{Role: &ownerRole})
	assert.Error(t, err, "should reject Ascenda-only role 'owner'")
}

func TestAdminUserService_UpdateUser_Plan_UpgradesToPro(t *testing.T) {
	sc := newMockSocrateClient()
	sc.addUser(socrate.User{ID: 20, Email: "alice@example.com", Name: "Alice", Role: "user"})
	svc, userRepo, tenantRepo := newTestAdminUserService(sc)

	// Add a local Ascenda user record linked via ExternalID
	tenantID := uuid.New()
	tenantRepo.Create(&model.Tenant{ID: tenantID, Name: "ACME", Slug: "acme"})
	localUser := &model.User{
		ID:         uuid.New(),
		TenantID:   tenantID,
		ExternalID: "20",
		Email:      "alice@example.com",
		Name:       "Alice",
		Plan:       "freemium",
		Role:       "user",
		IsActive:   true,
	}
	userRepo.addUser(localUser)

	proPlan := "pro"
	dto, err := svc.UpdateUser(context.Background(), "20", UpdateUserRequest{Plan: &proPlan})
	require.NoError(t, err)
	// Plan is a local-only field → service returns nil DTO (handler sends 204).
	assert.Nil(t, dto, "plan-only change should return nil DTO")

	// Verify the local DB record was mutated
	updated, _ := userRepo.GetByExternalID("20")
	assert.Equal(t, "pro", updated.Plan, "local user record should be persisted as pro")
}

func TestAdminUserService_UpdateUser_Plan_DowngradesToFreemium(t *testing.T) {
	sc := newMockSocrateClient()
	sc.addUser(socrate.User{ID: 21, Email: "bob@example.com", Name: "Bob", Role: "user"})
	svc, userRepo, tenantRepo := newTestAdminUserService(sc)

	tenantID := uuid.New()
	tenantRepo.Create(&model.Tenant{ID: tenantID, Name: "Startup", Slug: "startup"})
	localUser := &model.User{
		ID:         uuid.New(),
		TenantID:   tenantID,
		ExternalID: "21",
		Email:      "bob@example.com",
		Name:       "Bob",
		Plan:       "enterprise",
		Role:       "user",
		IsActive:   true,
	}
	userRepo.addUser(localUser)

	freemiumPlan := "freemium"
	dto, err := svc.UpdateUser(context.Background(), "21", UpdateUserRequest{Plan: &freemiumPlan})
	require.NoError(t, err)
	assert.Nil(t, dto, "plan-only change should return nil DTO")

	updated, _ := userRepo.GetByExternalID("21")
	assert.Equal(t, "freemium", updated.Plan)
}

func TestAdminUserService_UpdateUser_Plan_InvalidValue(t *testing.T) {
	sc := newMockSocrateClient()
	sc.addUser(socrate.User{ID: 22, Email: "carol@example.com", Name: "Carol", Role: "user"})
	svc, _, _ := newTestAdminUserService(sc)

	badPlan := "premium_plus"
	_, err := svc.UpdateUser(context.Background(), "22", UpdateUserRequest{Plan: &badPlan})
	assert.Error(t, err, "should reject unknown plan value")
	assert.Contains(t, err.Error(), "invalid plan")
}

func TestAdminUserService_UpdateUser_Plan_NoLocalRecord_IsNoop(t *testing.T) {
	// If there is no local user record (e.g. user never logged in), plan update
	// should succeed without error — Socrate update goes through fine.
	sc := newMockSocrateClient()
	sc.addUser(socrate.User{ID: 23, Email: "ghost@example.com", Name: "Ghost", Role: "user"})
	svc, _, _ := newTestAdminUserService(sc)

	proPlan := "pro"
	dto, err := svc.UpdateUser(context.Background(), "23", UpdateUserRequest{Plan: &proPlan})
	require.NoError(t, err)
	// Plan is local-only → nil DTO regardless of whether a local record exists.
	assert.Nil(t, dto, "plan-only change should return nil DTO")
}

func TestAdminUserService_UpdateUser_Plan_CombinedWithFullName(t *testing.T) {
	sc := newMockSocrateClient()
	sc.addUser(socrate.User{ID: 24, Email: "diana@example.com", Name: "Diana", Role: "user"})
	svc, userRepo, tenantRepo := newTestAdminUserService(sc)

	tenantID := uuid.New()
	tenantRepo.Create(&model.Tenant{ID: tenantID, Name: "Corp", Slug: "corp"})
	localUser := &model.User{
		ID:         uuid.New(),
		TenantID:   tenantID,
		ExternalID: "24",
		Email:      "diana@example.com",
		Name:       "Diana",
		Plan:       "freemium",
		Role:       "user",
		IsActive:   true,
	}
	userRepo.addUser(localUser)

	newName := "Diana Prince"
	proPlan := "pro"
	dto, err := svc.UpdateUser(context.Background(), "24", UpdateUserRequest{
		FullName: &newName,
		Plan:     &proPlan,
	})
	require.NoError(t, err)
	assert.Equal(t, "Diana Prince", dto.Name)
	require.NotNil(t, dto.Plan)
	assert.Equal(t, "pro", *dto.Plan)

	updated, _ := userRepo.GetByExternalID("24")
	assert.Equal(t, "pro", updated.Plan)
}

func TestAdminUserService_UpdateUser_AscendaRole_ChangesToOwner(t *testing.T) {
	sc := newMockSocrateClient()
	sc.addUser(socrate.User{ID: 25, Email: "eve@example.com", Name: "Eve", Role: "user"})
	svc, userRepo, tenantRepo := newTestAdminUserService(sc)

	tenantID := uuid.New()
	tenantRepo.Create(&model.Tenant{ID: tenantID, Name: "Org", Slug: "org"})
	localUser := &model.User{
		ID:         uuid.New(),
		TenantID:   tenantID,
		ExternalID: "25",
		Email:      "eve@example.com",
		Name:       "Eve",
		Plan:       "freemium",
		Role:       "user",
		IsActive:   true,
	}
	userRepo.addUser(localUser)

	ownerRole := "owner"
	dto, err := svc.UpdateUser(context.Background(), "25", UpdateUserRequest{AscendaRole: &ownerRole})
	require.NoError(t, err)
	// AscendaRole is local-only → nil DTO (handler sends 204).
	assert.Nil(t, dto, "ascendaRole-only change should return nil DTO")

	updated, _ := userRepo.GetByExternalID("25")
	assert.Equal(t, "owner", updated.Role, "local role should be persisted as owner")
}

func TestAdminUserService_UpdateUser_AscendaRole_InvalidValue(t *testing.T) {
	sc := newMockSocrateClient()
	sc.addUser(socrate.User{ID: 26, Email: "frank@example.com", Name: "Frank", Role: "user"})
	svc, _, _ := newTestAdminUserService(sc)

	badRole := "superadmin"
	_, err := svc.UpdateUser(context.Background(), "26", UpdateUserRequest{AscendaRole: &badRole})
	assert.Error(t, err, "should reject unknown Ascenda role")
	assert.Contains(t, err.Error(), "invalid Ascenda role")
}

func TestAdminUserService_UpdateUser_PlanOnly_DoesNotCallSocrateUpdate(t *testing.T) {
	// When only Plan is changed (no FullName / Role), the service fetches the
	// user via GetUser (not UpdateUser) so it never sends an empty-body PUT to Socrate.
	sc := newMockSocrateClient()
	sc.addUser(socrate.User{ID: 27, Email: "grace@example.com", Name: "Grace", Role: "user"})
	svc, userRepo, tenantRepo := newTestAdminUserService(sc)

	tenantID := uuid.New()
	tenantRepo.Create(&model.Tenant{ID: tenantID, Name: "Solo", Slug: "solo"})
	userRepo.addUser(&model.User{
		ID:         uuid.New(),
		TenantID:   tenantID,
		ExternalID: "27",
		Email:      "grace@example.com",
		Name:       "Grace",
		Plan:       "freemium",
		Role:       "user",
		IsActive:   true,
	})

	proPlan := "pro"
	dto, err := svc.UpdateUser(context.Background(), "27", UpdateUserRequest{Plan: &proPlan})
	require.NoError(t, err)
	// No Socrate call at all — service returns nil DTO (handler sends 204).
	assert.Nil(t, dto, "plan-only change must not return a DTO")

	updated, _ := userRepo.GetByExternalID("27")
	assert.Equal(t, "pro", updated.Plan)
}

func TestAdminUserService_DeleteUser_RemovesFromSocrate(t *testing.T) {
	sc := newMockSocrateClient()
	sc.addUser(socrate.User{ID: 10, Email: "del@example.com"})
	svc, _, _ := newTestAdminUserService(sc)

	err := svc.DeleteUser(context.Background(), "10")
	require.NoError(t, err)
	assert.Nil(t, sc.users["10"], "user should be removed from Socrate mock")
}

func TestAdminUserService_DeleteUser_DeactivatesAscendaRecord(t *testing.T) {
	sc := newMockSocrateClient()
	sc.addUser(socrate.User{ID: 11, Email: "deact@example.com"})
	svc, userRepo, _ := newTestAdminUserService(sc)

	kpUser := &model.User{
		ID:         uuid.New(),
		TenantID:   uuid.New(),
		ExternalID: "11",
		Email:      "deact@example.com",
		IsActive:   true,
	}
	userRepo.addUser(kpUser)

	err := svc.DeleteUser(context.Background(), "11")
	require.NoError(t, err)

	// Ascenda record should be deactivated
	updated, _ := userRepo.GetByExternalID("11")
	assert.False(t, updated.IsActive)
}

func TestAdminUserService_ResendVerification(t *testing.T) {
	sc := newMockSocrateClient()
	svc, _, _ := newTestAdminUserService(sc)

	err := svc.ResendVerification(context.Background(), "1")
	assert.NoError(t, err)
}

func TestAdminUserService_ResetPassword(t *testing.T) {
	sc := newMockSocrateClient()
	svc, _, _ := newTestAdminUserService(sc)

	err := svc.ResetPassword(context.Background(), "1")
	assert.NoError(t, err)
}

// ── Tenant management tests ───────────────────────────────────────────────────

func TestAdminUserService_ListTenants_Empty(t *testing.T) {
	svc, _, _ := newTestAdminUserService(nil)

	result, err := svc.ListTenants(context.Background(), 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.TotalCount)
	assert.Empty(t, result.Tenants)
}

func TestAdminUserService_ListTenants_WithData(t *testing.T) {
	svc, _, tenantRepo := newTestAdminUserService(nil)
	tenantRepo.Create(&model.Tenant{ID: uuid.New(), Name: "Acme", Slug: "acme", IsActive: true})
	tenantRepo.Create(&model.Tenant{ID: uuid.New(), Name: "Beta Corp", Slug: "beta", IsActive: false})

	result, err := svc.ListTenants(context.Background(), 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.TotalCount)
	assert.Len(t, result.Tenants, 2)
}

func TestAdminUserService_GetTenant_Found(t *testing.T) {
	svc, _, tenantRepo := newTestAdminUserService(nil)
	id := uuid.New()
	tenantRepo.Create(&model.Tenant{ID: id, Name: "FindMe", Slug: "find-me", IsActive: true})

	dto, err := svc.GetTenant(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "FindMe", dto.Name)
	assert.Equal(t, "find-me", dto.Slug)
}

func TestAdminUserService_GetTenant_NotFound(t *testing.T) {
	svc, _, _ := newTestAdminUserService(nil)

	_, err := svc.GetTenant(context.Background(), uuid.New())
	assert.Error(t, err)
}

func TestAdminUserService_CreateTenant_Defaults(t *testing.T) {
	svc, _, _ := newTestAdminUserService(nil)

	dto, err := svc.CreateTenant(context.Background(), CreateTenantRequest{
		Name: "New Co",
		Slug: "new-co",
	})
	require.NoError(t, err)
	assert.Equal(t, "New Co", dto.Name)
	assert.True(t, dto.IsActive)
}

func TestAdminUserService_CreateTenant_CustomValues(t *testing.T) {
	svc, _, _ := newTestAdminUserService(nil)

	dto, err := svc.CreateTenant(context.Background(), CreateTenantRequest{
		Name: "Enterprise Co",
		Slug: "enterprise",
	})
	require.NoError(t, err)
	assert.Equal(t, "Enterprise Co", dto.Name)
	assert.Equal(t, "enterprise", dto.Slug)
}

func TestAdminUserService_UpdateTenant(t *testing.T) {
	svc, _, tenantRepo := newTestAdminUserService(nil)
	id := uuid.New()
	tenantRepo.Create(&model.Tenant{ID: id, Name: "Old Name", Slug: "old", IsActive: true})

	newName := "New Name"
	dto, err := svc.UpdateTenant(context.Background(), id, UpdateTenantRequest{
		Name: &newName,
	})
	require.NoError(t, err)
	assert.Equal(t, "New Name", dto.Name)
}

func TestAdminUserService_UpdateTenant_NotFound(t *testing.T) {
	svc, _, _ := newTestAdminUserService(nil)

	name := "Ghost"
	_, err := svc.UpdateTenant(context.Background(), uuid.New(), UpdateTenantRequest{Name: &name})
	assert.Error(t, err)
}

func TestAdminUserService_UpdateTenant_Deactivate(t *testing.T) {
	svc, _, tenantRepo := newTestAdminUserService(nil)
	id := uuid.New()
	tenantRepo.Create(&model.Tenant{ID: id, Name: "Active", Slug: "active", IsActive: true})

	inactive := false
	dto, err := svc.UpdateTenant(context.Background(), id, UpdateTenantRequest{IsActive: &inactive})
	require.NoError(t, err)
	assert.False(t, dto.IsActive)
}

// ── Role boundary tests ───────────────────────────────────────────────────────
// These document the relationship between Socrate roles ("admin"/"user") and
// Ascenda local roles ("admin"/"owner"/"user"):
//   - Socrate "admin" → Ascenda ascendaRole "admin" (derived, not stored)
//   - Socrate "user"  → Ascenda ascendaRole from local DB (owner, user, …)

func TestSocrateRoleIsNotAscendaRole(t *testing.T) {
	sc := newMockSocrateClient()
	// A Socrate "admin" with no local Ascenda record: ascendaRole stays nil.
	sc.addUser(socrate.User{ID: 20, Email: "socrateadmin@example.com", Role: "admin"})
	svc, _, _ := newTestAdminUserService(sc)

	dto, err := svc.GetUser(context.Background(), "20")
	require.NoError(t, err)
	assert.Equal(t, "admin", dto.Role, "Socrate role is preserved")
	assert.Nil(t, dto.AscendaRole, "no Ascenda record means no ascendaRole enrichment")
}

func TestSocrateAdminWithLocalRecord_AscendaRoleIsAdmin(t *testing.T) {
	sc := newMockSocrateClient()
	sc.addUser(socrate.User{ID: 21, Email: "platformadmin@example.com", Role: "admin"})
	svc, userRepo, tenantRepo := newTestAdminUserService(sc)

	tenantID := uuid.New()
	tenantRepo.Create(&model.Tenant{ID: tenantID, Name: "Test Corp", Slug: "testcorp", IsActive: true})
	userRepo.addUser(&model.User{
		ID:         uuid.New(),
		TenantID:   tenantID,
		ExternalID: "21",
		Email:      "platformadmin@example.com",
		Role:       "user", // stored value is irrelevant for Socrate admins
		IsActive:   true,
	})

	dto, err := svc.GetUser(context.Background(), "21")
	require.NoError(t, err)
	assert.Equal(t, "admin", dto.Role, "Socrate role preserved")
	require.NotNil(t, dto.AscendaRole)
	assert.Equal(t, "admin", *dto.AscendaRole, "Socrate admin is always Ascenda admin, regardless of stored local role")
}
