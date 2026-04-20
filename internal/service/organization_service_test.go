package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/socrate"
)

// ── in-memory mocks ────────────────────────────────────────────────────────

// mockOrgRepo ──────────────────────────────────────────────────────────────

type mockOrgRepo struct {
	orgs      map[uuid.UUID]*model.Organization
	tenants   map[uuid.UUID][]*model.Tenant // orgID → tenants
	userCount int64                         // returned by CountActiveUsers
}

func newMockOrgRepo() *mockOrgRepo {
	return &mockOrgRepo{
		orgs:    make(map[uuid.UUID]*model.Organization),
		tenants: make(map[uuid.UUID][]*model.Tenant),
	}
}

func (r *mockOrgRepo) Create(org *model.Organization) error {
	if org.ID == uuid.Nil {
		org.ID = uuid.New()
	}
	org.CreatedAt = time.Now()
	org.UpdatedAt = time.Now()
	r.orgs[org.ID] = org
	return nil
}

func (r *mockOrgRepo) GetByID(id uuid.UUID) (*model.Organization, error) {
	org, ok := r.orgs[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return org, nil
}

func (r *mockOrgRepo) GetBySlug(slug string) (*model.Organization, error) {
	for _, o := range r.orgs {
		if o.Slug == slug {
			return o, nil
		}
	}
	return nil, errors.New("not found")
}

func (r *mockOrgRepo) ListAll(offset, limit int) ([]*model.Organization, error) {
	orgs := make([]*model.Organization, 0, len(r.orgs))
	for _, o := range r.orgs {
		orgs = append(orgs, o)
	}
	if offset >= len(orgs) {
		return []*model.Organization{}, nil
	}
	end := offset + limit
	if end > len(orgs) {
		end = len(orgs)
	}
	return orgs[offset:end], nil
}

func (r *mockOrgRepo) CountAll() (int64, error) {
	return int64(len(r.orgs)), nil
}

func (r *mockOrgRepo) Update(org *model.Organization) error {
	if _, ok := r.orgs[org.ID]; !ok {
		return errors.New("not found")
	}
	r.orgs[org.ID] = org
	return nil
}

func (r *mockOrgRepo) Delete(id uuid.UUID) error {
	if org, ok := r.orgs[id]; ok {
		org.IsActive = false
		return nil
	}
	return errors.New("not found")
}

func (r *mockOrgRepo) ListTenants(orgID uuid.UUID) ([]*model.Tenant, error) {
	return r.tenants[orgID], nil
}

func (r *mockOrgRepo) CountActiveUsers(orgID uuid.UUID) (int64, error) {
	return r.userCount, nil
}

// mockOrgTenantRepo ──────────────────────────────────────────────────────

type mockOrgTenantRepo struct {
	tenants map[uuid.UUID]*model.Tenant
}

func newMockOrgTenantRepo() *mockOrgTenantRepo {
	return &mockOrgTenantRepo{tenants: make(map[uuid.UUID]*model.Tenant)}
}

func (r *mockOrgTenantRepo) Create(t *model.Tenant) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	t.CreatedAt = time.Now()
	r.tenants[t.ID] = t
	return nil
}

func (r *mockOrgTenantRepo) GetByID(id uuid.UUID) (*model.Tenant, error) {
	t, ok := r.tenants[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return t, nil
}

func (r *mockOrgTenantRepo) GetBySlug(slug string) (*model.Tenant, error) {
	for _, t := range r.tenants {
		if t.Slug == slug {
			return t, nil
		}
	}
	return nil, errors.New("not found")
}

func (r *mockOrgTenantRepo) ListActive(offset, limit int) ([]*model.Tenant, error) {
	return nil, nil
}

func (r *mockOrgTenantRepo) ListAll(offset, limit int) ([]*model.Tenant, error) {
	ts := make([]*model.Tenant, 0, len(r.tenants))
	for _, t := range r.tenants {
		ts = append(ts, t)
	}
	return ts, nil
}

func (r *mockOrgTenantRepo) CountAll() (int64, error) {
	return int64(len(r.tenants)), nil
}

func (r *mockOrgTenantRepo) Update(t *model.Tenant) error {
	if _, ok := r.tenants[t.ID]; !ok {
		return errors.New("not found")
	}
	r.tenants[t.ID] = t
	return nil
}

func (r *mockOrgTenantRepo) Delete(id uuid.UUID) error {
	delete(r.tenants, id)
	return nil
}

// mockUserRepoForOrg ───────────────────────────────────────────────────────

type mockUserRepoForOrg struct {
	users map[uuid.UUID]*model.User
}

func newMockUserRepoForOrg() *mockUserRepoForOrg {
	return &mockUserRepoForOrg{users: make(map[uuid.UUID]*model.User)}
}

func (r *mockUserRepoForOrg) Create(u *model.User) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	r.users[u.ID] = u
	return nil
}

func (r *mockUserRepoForOrg) GetByID(tenantID, userID uuid.UUID) (*model.User, error) {
	u, ok := r.users[userID]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

func (r *mockUserRepoForOrg) GetByExternalID(externalID string) (*model.User, error) {
	return nil, errors.New("not found")
}

func (r *mockUserRepoForOrg) GetByEmail(tenantID uuid.UUID, email string) (*model.User, error) {
	for _, u := range r.users {
		if u.TenantID == tenantID && u.Email == email {
			return u, nil
		}
	}
	return nil, errors.New("not found")
}

func (r *mockUserRepoForOrg) ListByTenant(tenantID uuid.UUID, offset, limit int) ([]*model.User, int64, error) {
	return nil, 0, nil
}

func (r *mockUserRepoForOrg) Update(u *model.User) error {
	r.users[u.ID] = u
	return nil
}

func (r *mockUserRepoForOrg) CountByTenant(tenantID uuid.UUID) (int64, error) {
	var count int64
	for _, u := range r.users {
		if u.TenantID == tenantID {
			count++
		}
	}
	return count, nil
}

// mockInviterForOrg ───────────────────────────────────────────────────────

type mockInviterForOrg struct {
	invitedEmails []string
	err           error
}

func (m *mockInviterForOrg) InviteUserAsService(_ context.Context, req socrate.ServiceInviteRequest) (*socrate.CreateUserResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.invitedEmails = append(m.invitedEmails, req.Email)
	return &socrate.CreateUserResult{
		UserID:    99,
		EmailSent: true,
	}, nil
}

// ── helper ────────────────────────────────────────────────────────────────

func newOrgSvc(orgRepo *mockOrgRepo, tenantRepo *mockOrgTenantRepo, userRepo *mockUserRepoForOrg, inviter SocrateInviter) *OrganizationService {
	return NewOrganizationService(orgRepo, tenantRepo, userRepo, inviter, logrus.NewEntry(logrus.New()))
}

// ── CreateOrganization ────────────────────────────────────────────────────

func TestOrgService_CreateOrganization_Success(t *testing.T) {
	orgRepo := newMockOrgRepo()
	tenantRepo := newMockOrgTenantRepo()
	userRepo := newMockUserRepoForOrg()
	inviter := &mockInviterForOrg{}

	svc := newOrgSvc(orgRepo, tenantRepo, userRepo, inviter)

	dto, err := svc.CreateOrganization(context.Background(), CreateOrganizationRequest{
		Name:                "Orange SA",
		Slug:                "orange-sa",
		Plan:                "enterprise",
		MaxUsers:            1000,
		BillingEmail:        "cfo@orange.fr",
		FirstDepartment:     "Orange Finance",
		FirstDepartmentSlug: "orange-finance",
		OwnerEmail:          "finance.admin@orange.fr",
		OwnerName:           "Jean Dupont",
	})

	require.NoError(t, err)
	assert.Equal(t, "Orange SA", dto.Name)
	assert.Equal(t, "enterprise", dto.Plan)

	// Org should be in the repo
	assert.Len(t, orgRepo.orgs, 1)

	// First department tenant should be created
	assert.Len(t, tenantRepo.tenants, 1)
	for _, t2 := range tenantRepo.tenants {
		assert.Equal(t, "orange-finance", t2.Slug)
		assert.Equal(t, model.TenantTypeEnterprise, t2.Type)
		assert.Equal(t, "enterprise", t2.Plan)
	}

	// Owner user should be in userRepo
	assert.Len(t, userRepo.users, 1)
	for _, u := range userRepo.users {
		assert.Equal(t, "owner", u.Role)
		assert.Equal(t, "finance.admin@orange.fr", u.Email)
	}

	// Invite should have been sent
	assert.Contains(t, inviter.invitedEmails, "finance.admin@orange.fr")
}

func TestOrgService_CreateOrganization_DefaultPlanIsEnterprise(t *testing.T) {
	svc := newOrgSvc(newMockOrgRepo(), newMockOrgTenantRepo(), newMockUserRepoForOrg(), nil)

	dto, err := svc.CreateOrganization(context.Background(), CreateOrganizationRequest{
		Name: "ACME", Slug: "acme",
		// Plan deliberately omitted
	})
	require.NoError(t, err)
	assert.Equal(t, "enterprise", dto.Plan)
}

func TestOrgService_CreateOrganization_InvalidPlan(t *testing.T) {
	svc := newOrgSvc(newMockOrgRepo(), newMockOrgTenantRepo(), newMockUserRepoForOrg(), nil)

	_, err := svc.CreateOrganization(context.Background(), CreateOrganizationRequest{
		Name: "ACME", Slug: "acme", Plan: "diamond",
	})
	require.Error(t, err)
}

func TestOrgService_CreateOrganization_NoOwnerEmail_SkipsInvite(t *testing.T) {
	inviter := &mockInviterForOrg{}
	svc := newOrgSvc(newMockOrgRepo(), newMockOrgTenantRepo(), newMockUserRepoForOrg(), inviter)

	_, err := svc.CreateOrganization(context.Background(), CreateOrganizationRequest{
		Name: "ACME", Slug: "acme",
		// OwnerEmail deliberately omitted
	})
	require.NoError(t, err)
	assert.Empty(t, inviter.invitedEmails) // no invite sent
}

// ── GetOrganization ───────────────────────────────────────────────────────

func TestOrgService_GetOrganization_Success(t *testing.T) {
	orgRepo := newMockOrgRepo()
	org := &model.Organization{ID: uuid.New(), Name: "Acme", Slug: "acme", Plan: "pro", IsActive: true}
	orgRepo.orgs[org.ID] = org

	svc := newOrgSvc(orgRepo, newMockOrgTenantRepo(), newMockUserRepoForOrg(), nil)
	dto, err := svc.GetOrganization(context.Background(), org.ID)

	require.NoError(t, err)
	assert.Equal(t, org.Name, dto.Name)
}

func TestOrgService_GetOrganization_NotFound(t *testing.T) {
	svc := newOrgSvc(newMockOrgRepo(), newMockOrgTenantRepo(), newMockUserRepoForOrg(), nil)
	_, err := svc.GetOrganization(context.Background(), uuid.New())
	require.Error(t, err)
}

// ── UpdateOrganization ────────────────────────────────────────────────────

func TestOrgService_UpdateOrganization_NameChange(t *testing.T) {
	orgRepo := newMockOrgRepo()
	org := &model.Organization{ID: uuid.New(), Name: "Old Name", Slug: "old", Plan: "enterprise", IsActive: true}
	orgRepo.orgs[org.ID] = org

	svc := newOrgSvc(orgRepo, newMockOrgTenantRepo(), newMockUserRepoForOrg(), nil)

	newName := "New Name"
	dto, err := svc.UpdateOrganization(context.Background(), org.ID, UpdateOrganizationRequest{Name: &newName})
	require.NoError(t, err)
	assert.Equal(t, "New Name", dto.Name)
	assert.Equal(t, "New Name", orgRepo.orgs[org.ID].Name)
}

func TestOrgService_UpdateOrganization_PlanCascadesToTenants(t *testing.T) {
	orgRepo := newMockOrgRepo()
	tenantRepo := newMockOrgTenantRepo()

	org := &model.Organization{ID: uuid.New(), Name: "ACME", Slug: "acme", Plan: "pro", IsActive: true}
	orgRepo.orgs[org.ID] = org

	// Two department tenants under this org
	t1 := &model.Tenant{ID: uuid.New(), Name: "Dept A", Slug: "dept-a", Plan: "pro", OrganizationID: &org.ID}
	t2 := &model.Tenant{ID: uuid.New(), Name: "Dept B", Slug: "dept-b", Plan: "pro", OrganizationID: &org.ID}
	tenantRepo.tenants[t1.ID] = t1
	tenantRepo.tenants[t2.ID] = t2
	orgRepo.tenants[org.ID] = []*model.Tenant{t1, t2}

	svc := newOrgSvc(orgRepo, tenantRepo, newMockUserRepoForOrg(), nil)

	newPlan := "enterprise"
	_, err := svc.UpdateOrganization(context.Background(), org.ID, UpdateOrganizationRequest{Plan: &newPlan})
	require.NoError(t, err)

	// Both tenants must have been updated to enterprise plan
	assert.Equal(t, "enterprise", tenantRepo.tenants[t1.ID].Plan)
	assert.Equal(t, "enterprise", tenantRepo.tenants[t2.ID].Plan)
}

func TestOrgService_UpdateOrganization_InvalidPlan(t *testing.T) {
	orgRepo := newMockOrgRepo()
	org := &model.Organization{ID: uuid.New(), Name: "X", Slug: "x", Plan: "pro", IsActive: true}
	orgRepo.orgs[org.ID] = org

	svc := newOrgSvc(orgRepo, newMockOrgTenantRepo(), newMockUserRepoForOrg(), nil)

	badPlan := "diamond"
	_, err := svc.UpdateOrganization(context.Background(), org.ID, UpdateOrganizationRequest{Plan: &badPlan})
	require.Error(t, err)
}

// ── DeleteOrganization ────────────────────────────────────────────────────

func TestOrgService_DeleteOrganization_Success(t *testing.T) {
	orgRepo := newMockOrgRepo()
	org := &model.Organization{ID: uuid.New(), Name: "X", Slug: "x", IsActive: true}
	orgRepo.orgs[org.ID] = org

	svc := newOrgSvc(orgRepo, newMockOrgTenantRepo(), newMockUserRepoForOrg(), nil)
	err := svc.DeleteOrganization(context.Background(), org.ID)
	require.NoError(t, err)
	assert.False(t, orgRepo.orgs[org.ID].IsActive)
}

func TestOrgService_DeleteOrganization_NotFound(t *testing.T) {
	svc := newOrgSvc(newMockOrgRepo(), newMockOrgTenantRepo(), newMockUserRepoForOrg(), nil)
	err := svc.DeleteOrganization(context.Background(), uuid.New())
	require.Error(t, err)
}

// ── CheckLicenseCapacity ──────────────────────────────────────────────────

func TestOrgService_CheckLicenseCapacity_Unlimited(t *testing.T) {
	orgRepo := newMockOrgRepo()
	org := &model.Organization{ID: uuid.New(), Name: "X", Slug: "x", MaxUsers: 0} // 0 = unlimited
	orgRepo.orgs[org.ID] = org
	orgRepo.userCount = 9999 // even with many users, unlimited means no error

	svc := newOrgSvc(orgRepo, newMockOrgTenantRepo(), newMockUserRepoForOrg(), nil)
	err := svc.CheckLicenseCapacity(context.Background(), org.ID)
	require.NoError(t, err)
}

func TestOrgService_CheckLicenseCapacity_WithinLimit(t *testing.T) {
	orgRepo := newMockOrgRepo()
	org := &model.Organization{ID: uuid.New(), Name: "X", Slug: "x", MaxUsers: 100}
	orgRepo.orgs[org.ID] = org
	orgRepo.userCount = 50 // 50 < 100 → OK

	svc := newOrgSvc(orgRepo, newMockOrgTenantRepo(), newMockUserRepoForOrg(), nil)
	err := svc.CheckLicenseCapacity(context.Background(), org.ID)
	require.NoError(t, err)
}

func TestOrgService_CheckLicenseCapacity_LimitReached(t *testing.T) {
	orgRepo := newMockOrgRepo()
	org := &model.Organization{ID: uuid.New(), Name: "X", Slug: "x", MaxUsers: 100}
	orgRepo.orgs[org.ID] = org
	orgRepo.userCount = 100 // exactly at limit → error

	svc := newOrgSvc(orgRepo, newMockOrgTenantRepo(), newMockUserRepoForOrg(), nil)
	err := svc.CheckLicenseCapacity(context.Background(), org.ID)
	require.Error(t, err)
}

// ── AddTenant ─────────────────────────────────────────────────────────────

func TestOrgService_AddTenant_Success(t *testing.T) {
	orgRepo := newMockOrgRepo()
	tenantRepo := newMockOrgTenantRepo()
	inviter := &mockInviterForOrg{}

	org := &model.Organization{ID: uuid.New(), Name: "ACME", Slug: "acme", Plan: "enterprise", IsActive: true}
	orgRepo.orgs[org.ID] = org

	svc := newOrgSvc(orgRepo, tenantRepo, newMockUserRepoForOrg(), inviter)

	dto, err := svc.AddTenant(context.Background(), org.ID, AddOrgTenantRequest{
		Name:       "ACME Finance",
		Slug:       "acme-finance",
		OwnerEmail: "cfo@acme.com",
		OwnerName:  "Alice CFO",
	})
	require.NoError(t, err)
	assert.Equal(t, "acme-finance", dto.Slug)
	assert.Equal(t, "enterprise", dto.Plan) // inherits org plan

	// Tenant created in repo
	assert.Len(t, tenantRepo.tenants, 1)
	for _, ten := range tenantRepo.tenants {
		assert.Equal(t, model.TenantTypeEnterprise, ten.Type)
		assert.Equal(t, &org.ID, ten.OrganizationID)
	}

	// Invite sent
	assert.Contains(t, inviter.invitedEmails, "cfo@acme.com")
}

func TestOrgService_AddTenant_OrgNotFound(t *testing.T) {
	svc := newOrgSvc(newMockOrgRepo(), newMockOrgTenantRepo(), newMockUserRepoForOrg(), nil)
	_, err := svc.AddTenant(context.Background(), uuid.New(), AddOrgTenantRequest{Name: "X", Slug: "x"})
	require.Error(t, err)
}

func TestOrgService_AddTenant_InactiveOrgRejected(t *testing.T) {
	orgRepo := newMockOrgRepo()
	org := &model.Organization{ID: uuid.New(), Name: "Dead", Slug: "dead", IsActive: false}
	orgRepo.orgs[org.ID] = org

	svc := newOrgSvc(orgRepo, newMockOrgTenantRepo(), newMockUserRepoForOrg(), nil)
	_, err := svc.AddTenant(context.Background(), org.ID, AddOrgTenantRequest{Name: "X", Slug: "x"})
	require.Error(t, err)
}

// ── ListOrganizations ─────────────────────────────────────────────────────

func TestOrgService_ListOrganizations_Pagination(t *testing.T) {
	orgRepo := newMockOrgRepo()
	for i := 0; i < 5; i++ {
		o := &model.Organization{ID: uuid.New(), Name: "Org", Slug: uuid.New().String(), IsActive: true}
		orgRepo.orgs[o.ID] = o
	}

	svc := newOrgSvc(orgRepo, newMockOrgTenantRepo(), newMockUserRepoForOrg(), nil)
	resp, err := svc.ListOrganizations(context.Background(), 1, 3)
	require.NoError(t, err)
	assert.Equal(t, int64(5), resp.TotalCount)
	assert.Len(t, resp.Organizations, 3)
}
