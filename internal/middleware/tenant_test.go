package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ── Test doubles ──────────────────────────────────────────────────────────────

var errNotFound = gorm.ErrRecordNotFound

type fakeUserRepo struct {
	users     []*model.User
	lookupErr error // returned by GetByExternalID / GetPendingInviteByEmail when set
	createErr error
	updateErr error
	created   []*model.User
	updated   []*model.User
}

func (f *fakeUserRepo) Create(u *model.User) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.users = append(f.users, u)
	f.created = append(f.created, u)
	return nil
}

func (f *fakeUserRepo) GetByID(tenantID, userID uuid.UUID) (*model.User, error) {
	for _, u := range f.users {
		if u.TenantID == tenantID && u.ID == userID {
			return u, nil
		}
	}
	return nil, errNotFound
}

func (f *fakeUserRepo) GetByExternalID(externalID string) (*model.User, error) {
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	for _, u := range f.users {
		if u.ExternalID != "" && u.ExternalID == externalID {
			return u, nil
		}
	}
	return nil, errNotFound
}

func (f *fakeUserRepo) GetByEmail(tenantID uuid.UUID, email string) (*model.User, error) {
	for _, u := range f.users {
		if u.TenantID == tenantID && u.Email == email {
			return u, nil
		}
	}
	return nil, errNotFound
}

func (f *fakeUserRepo) GetPendingInviteByEmail(email string) (*model.User, error) {
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	for _, u := range f.users {
		if u.ExternalID == "" && strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}
	return nil, errNotFound
}

func (f *fakeUserRepo) ListByTenant(tenantID uuid.UUID, offset, limit int) ([]*model.User, int64, error) {
	var out []*model.User
	for _, u := range f.users {
		if u.TenantID == tenantID {
			out = append(out, u)
		}
	}
	return out, int64(len(out)), nil
}

func (f *fakeUserRepo) Update(u *model.User) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updated = append(f.updated, u)
	return nil
}

func (f *fakeUserRepo) CountByTenant(tenantID uuid.UUID) (int64, error) {
	var n int64
	for _, u := range f.users {
		if u.TenantID == tenantID {
			n++
		}
	}
	return n, nil
}

type fakeTenantRepo struct {
	tenants map[uuid.UUID]*model.Tenant
}

func (f *fakeTenantRepo) Create(t *model.Tenant) error                          { f.tenants[t.ID] = t; return nil }
func (f *fakeTenantRepo) Update(t *model.Tenant) error                          { f.tenants[t.ID] = t; return nil }
func (f *fakeTenantRepo) GetBySlug(slug string) (*model.Tenant, error)          { return nil, errNotFound }
func (f *fakeTenantRepo) ListActive(offset, limit int) ([]*model.Tenant, error) { return nil, nil }
func (f *fakeTenantRepo) Delete(id uuid.UUID) error                             { delete(f.tenants, id); return nil }
func (f *fakeTenantRepo) GetByID(id uuid.UUID) (*model.Tenant, error) {
	if t, ok := f.tenants[id]; ok {
		return t, nil
	}
	return nil, errNotFound
}

type fakeProfiler struct {
	email, name string
	err         error
	calls       int
}

func (p *fakeProfiler) GetCurrentUserProfile(ctx context.Context) (string, string, error) {
	p.calls++
	return p.email, p.name, p.err
}

// ── Helpers ───────────────────────────────────────────────────────────────────

type captured struct {
	called   bool
	tenantID uuid.UUID
	userID   uuid.UUID
	role     string
	plan     string
}

func run(t *testing.T, mw *TenantMiddleware, ctx context.Context) (*httptest.ResponseRecorder, *captured) {
	t.Helper()
	c := &captured{}
	h := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.called = true
		c.tenantID = ctxutil.GetTenantID(r.Context())
		c.userID = ctxutil.GetUserID(r.Context())
		c.role = ctxutil.GetUserRole(r.Context())
		c.plan = ctxutil.GetUserPlan(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/plans", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w, c
}

func newMW(users *fakeUserRepo, tenants *fakeTenantRepo, profiler UserProfileFetcher, allowDefault bool) *TenantMiddleware {
	logger := logrus.NewEntry(logrus.New())
	logger.Logger.SetLevel(logrus.PanicLevel)
	return NewTenantMiddleware(users, tenants, logger, profiler, allowDefault)
}

func jwtCtx(sub, email, name string, tenantID uuid.UUID) context.Context {
	ctx := context.Background()
	ctx = ctxutil.WithUserRole(ctx, "user")
	if sub != "" {
		ctx = ctxutil.WithUserSub(ctx, sub)
	}
	if email != "" {
		ctx = ctxutil.WithUserEmail(ctx, email)
	}
	if name != "" {
		ctx = ctxutil.WithUserName(ctx, name)
	}
	if tenantID != uuid.Nil {
		ctx = ctxutil.WithTenantID(ctx, tenantID)
	}
	return ctx
}

func workspace(id uuid.UUID) *model.Tenant {
	return &model.Tenant{ID: id, Name: "ws", Type: model.TenantTypeWorkspace, IsActive: true}
}

// ── Platform-admin fast path ──────────────────────────────────────────────────

func TestTenant_PlatformAdminFastPath(t *testing.T) {
	// nil repos: any DB access on this path would nil-deref.
	mw := newMW(nil, nil, nil, false)
	ctx := ctxutil.WithUserRole(context.Background(), "admin")

	var w *httptest.ResponseRecorder
	var c *captured
	assert.NotPanics(t, func() { w, c = run(t, mw, ctx) })
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, c.called)
	assert.Equal(t, "admin", c.role, "platform admin role must be preserved")
}

// ── Existing users ────────────────────────────────────────────────────────────

func TestTenant_ExistingUser_TenantComesFromRecord(t *testing.T) {
	tenantA := uuid.New()
	u := &model.User{ID: uuid.New(), TenantID: tenantA, ExternalID: "sub-1", Email: "a@x.io", Role: "editor", Plan: "pro", IsActive: true}
	users := &fakeUserRepo{users: []*model.User{u}}
	tenants := &fakeTenantRepo{tenants: map[uuid.UUID]*model.Tenant{tenantA: workspace(tenantA)}}
	mw := newMW(users, tenants, nil, false)

	// No tenant_id claim at all — the record is authoritative.
	w, c := run(t, mw, jwtCtx("sub-1", "a@x.io", "", uuid.Nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, tenantA, c.tenantID)
	assert.Equal(t, u.ID, c.userID)
	assert.Equal(t, "editor", c.role)
	assert.Equal(t, "pro", c.plan)
	assert.Empty(t, users.created, "existing user must not be re-provisioned")
}

func TestTenant_ExistingUser_JWTTenantMismatchIsIgnored(t *testing.T) {
	tenantA, tenantB := uuid.New(), uuid.New()
	u := &model.User{ID: uuid.New(), TenantID: tenantA, ExternalID: "sub-1", Email: "a@x.io", Role: "owner", IsActive: true}
	users := &fakeUserRepo{users: []*model.User{u}}
	tenants := &fakeTenantRepo{tenants: map[uuid.UUID]*model.Tenant{tenantA: workspace(tenantA), tenantB: workspace(tenantB)}}
	mw := newMW(users, tenants, nil, false)

	// JWT claims tenant B, but the user belongs to tenant A.
	w, c := run(t, mw, jwtCtx("sub-1", "", "", tenantB))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, tenantA, c.tenantID, "a forged/foreign tenant_id claim must never override the user record")
	assert.Equal(t, "freemium", c.plan, "empty plan defaults to freemium")
}

func TestTenant_ExistingUser_NeverFallsBackToDefaultTenant(t *testing.T) {
	// Regression for the audit's critical finding: a user with no tenant_id
	// claim used to be treated as a member (with their own role) of the
	// shared default tenant. Even with the dev fallback enabled, an existing
	// record must win.
	tenantA := uuid.New()
	u := &model.User{ID: uuid.New(), TenantID: tenantA, ExternalID: "sub-1", Role: "owner", IsActive: true}
	users := &fakeUserRepo{users: []*model.User{u}}
	tenants := &fakeTenantRepo{tenants: map[uuid.UUID]*model.Tenant{tenantA: workspace(tenantA), DefaultTenantID: workspace(DefaultTenantID)}}
	mw := newMW(users, tenants, nil, true)

	w, c := run(t, mw, jwtCtx("sub-1", "", "", uuid.Nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, tenantA, c.tenantID)
	assert.NotEqual(t, DefaultTenantID, c.tenantID)
}

func TestTenant_DeactivatedUserIsRejected(t *testing.T) {
	tenantA := uuid.New()
	u := &model.User{ID: uuid.New(), TenantID: tenantA, ExternalID: "sub-1", Role: "editor", IsActive: false}
	users := &fakeUserRepo{users: []*model.User{u}}
	mw := newMW(users, &fakeTenantRepo{tenants: map[uuid.UUID]*model.Tenant{}}, nil, false)

	w, c := run(t, mw, jwtCtx("sub-1", "", "", uuid.Nil))
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.False(t, c.called)
}

func TestTenant_EnterpriseTenantPlanOverridesUserPlan(t *testing.T) {
	tenantE := uuid.New()
	u := &model.User{ID: uuid.New(), TenantID: tenantE, ExternalID: "sub-1", Role: "editor", Plan: "freemium", IsActive: true}
	users := &fakeUserRepo{users: []*model.User{u}}
	tenants := &fakeTenantRepo{tenants: map[uuid.UUID]*model.Tenant{
		tenantE: {ID: tenantE, Type: model.TenantTypeEnterprise, Plan: "enterprise", IsActive: true},
	}}
	mw := newMW(users, tenants, nil, false)

	w, c := run(t, mw, jwtCtx("sub-1", "", "", uuid.Nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "enterprise", c.plan)
}

func TestTenant_ExistingUser_ProfileRefreshedFromClaims(t *testing.T) {
	tenantA := uuid.New()
	u := &model.User{ID: uuid.New(), TenantID: tenantA, ExternalID: "sub-1", Email: "", Name: "", Role: "editor", IsActive: true}
	users := &fakeUserRepo{users: []*model.User{u}}
	mw := newMW(users, &fakeTenantRepo{tenants: map[uuid.UUID]*model.Tenant{tenantA: workspace(tenantA)}}, nil, false)

	w, _ := run(t, mw, jwtCtx("sub-1", "new@x.io", "New Name", uuid.Nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, users.updated, 1)
	assert.Equal(t, "new@x.io", u.Email)
	assert.Equal(t, "New Name", u.Name)
}

func TestTenant_ExistingUser_ProfileFetchedFromIdPWhenEmailMissing(t *testing.T) {
	tenantA := uuid.New()
	u := &model.User{ID: uuid.New(), TenantID: tenantA, ExternalID: "sub-1", Email: "", Role: "editor", IsActive: true}
	users := &fakeUserRepo{users: []*model.User{u}}
	prof := &fakeProfiler{email: "idp@x.io", name: "IdP Name"}
	mw := newMW(users, &fakeTenantRepo{tenants: map[uuid.UUID]*model.Tenant{tenantA: workspace(tenantA)}}, prof, false)

	w, _ := run(t, mw, jwtCtx("sub-1", "", "", uuid.Nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, prof.calls)
	assert.Equal(t, "idp@x.io", u.Email)
}

// ── Invitation claiming ───────────────────────────────────────────────────────

func TestTenant_PendingInviteIsClaimedOnFirstLogin(t *testing.T) {
	tenantA := uuid.New()
	invited := &model.User{ID: uuid.New(), TenantID: tenantA, ExternalID: "", Email: "Invitee@X.io", Role: "reader", IsActive: true}
	users := &fakeUserRepo{users: []*model.User{invited}}
	tenants := &fakeTenantRepo{tenants: map[uuid.UUID]*model.Tenant{tenantA: workspace(tenantA), DefaultTenantID: workspace(DefaultTenantID)}}
	mw := newMW(users, tenants, nil, true) // fallback enabled: the invite must still win

	w, c := run(t, mw, jwtCtx("sub-new", "invitee@x.io", "Inv Itee", uuid.Nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, tenantA, c.tenantID)
	assert.Equal(t, invited.ID, c.userID)
	assert.Equal(t, "reader", c.role, "invited role must be kept, not replaced by editor")
	assert.Equal(t, "sub-new", invited.ExternalID, "invite must be linked to the IdP subject")
	assert.NotNil(t, invited.JoinedAt)
	assert.Equal(t, "Inv Itee", invited.Name)
	assert.Len(t, users.updated, 1)
	assert.Empty(t, users.created, "claiming an invite must not create a second user record")
}

func TestTenant_PendingInvite_EmailResolvedViaIdPWhenClaimMissing(t *testing.T) {
	tenantA := uuid.New()
	invited := &model.User{ID: uuid.New(), TenantID: tenantA, ExternalID: "", Email: "invitee@x.io", Role: "editor", IsActive: true}
	users := &fakeUserRepo{users: []*model.User{invited}}
	prof := &fakeProfiler{email: "invitee@x.io"}
	mw := newMW(users, &fakeTenantRepo{tenants: map[uuid.UUID]*model.Tenant{tenantA: workspace(tenantA)}}, prof, false)

	w, c := run(t, mw, jwtCtx("sub-new", "", "", uuid.Nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, tenantA, c.tenantID)
	assert.Equal(t, "sub-new", invited.ExternalID)
}

func TestTenant_PendingInvite_ClaimUpdateFailureIs500(t *testing.T) {
	tenantA := uuid.New()
	invited := &model.User{ID: uuid.New(), TenantID: tenantA, ExternalID: "", Email: "invitee@x.io", Role: "editor", IsActive: true}
	users := &fakeUserRepo{users: []*model.User{invited}, updateErr: errors.New("db down")}
	mw := newMW(users, &fakeTenantRepo{tenants: map[uuid.UUID]*model.Tenant{tenantA: workspace(tenantA)}}, nil, false)

	w, c := run(t, mw, jwtCtx("sub-new", "invitee@x.io", "", uuid.Nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.False(t, c.called)
}

// ── Provisioning into a JWT-declared tenant ───────────────────────────────────

func TestTenant_ProvisionIntoJWTTenant_FirstUserIsOwner(t *testing.T) {
	tenantA := uuid.New()
	users := &fakeUserRepo{}
	tenants := &fakeTenantRepo{tenants: map[uuid.UUID]*model.Tenant{tenantA: workspace(tenantA)}}
	mw := newMW(users, tenants, nil, false)

	w, c := run(t, mw, jwtCtx("sub-new", "first@x.io", "First", tenantA))
	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, users.created, 1)
	assert.Equal(t, tenantA, c.tenantID)
	assert.Equal(t, "owner", c.role)
	assert.Equal(t, "sub-new", users.created[0].ExternalID)
	assert.Equal(t, "freemium", c.plan)
}

func TestTenant_ProvisionIntoJWTTenant_SubsequentUserIsEditor(t *testing.T) {
	tenantA := uuid.New()
	existing := &model.User{ID: uuid.New(), TenantID: tenantA, ExternalID: "sub-owner", Role: "owner", IsActive: true}
	users := &fakeUserRepo{users: []*model.User{existing}}
	tenants := &fakeTenantRepo{tenants: map[uuid.UUID]*model.Tenant{tenantA: workspace(tenantA)}}
	mw := newMW(users, tenants, nil, false)

	w, c := run(t, mw, jwtCtx("sub-new", "second@x.io", "", tenantA))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "editor", c.role)
}

func TestTenant_ProvisionIntoUnknownJWTTenantIsRejected(t *testing.T) {
	users := &fakeUserRepo{}
	tenants := &fakeTenantRepo{tenants: map[uuid.UUID]*model.Tenant{}}
	mw := newMW(users, tenants, nil, true) // even with the dev fallback on

	w, c := run(t, mw, jwtCtx("sub-new", "x@x.io", "", uuid.New()))
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.False(t, c.called)
	assert.Empty(t, users.created)
}

func TestTenant_ProvisionCreateFailureIs500(t *testing.T) {
	tenantA := uuid.New()
	users := &fakeUserRepo{createErr: errors.New("db down")}
	tenants := &fakeTenantRepo{tenants: map[uuid.UUID]*model.Tenant{tenantA: workspace(tenantA)}}
	mw := newMW(users, tenants, nil, false)

	w, c := run(t, mw, jwtCtx("sub-new", "x@x.io", "", tenantA))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.False(t, c.called)
}

// ── No resolvable tenant ──────────────────────────────────────────────────────

func TestTenant_UnknownUserWithoutTenantIsRejectedByDefault(t *testing.T) {
	users := &fakeUserRepo{}
	tenants := &fakeTenantRepo{tenants: map[uuid.UUID]*model.Tenant{DefaultTenantID: workspace(DefaultTenantID)}}
	mw := newMW(users, tenants, nil, false)

	w, c := run(t, mw, jwtCtx("sub-new", "stranger@x.io", "", uuid.Nil))
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.False(t, c.called)
	assert.Empty(t, users.created, "must not silently provision into the shared default tenant")
	assert.Contains(t, w.Body.String(), "not linked to a workspace")
}

func TestTenant_DevFallbackProvisionsIntoDefaultTenant(t *testing.T) {
	users := &fakeUserRepo{}
	tenants := &fakeTenantRepo{tenants: map[uuid.UUID]*model.Tenant{DefaultTenantID: workspace(DefaultTenantID)}}
	mw := newMW(users, tenants, nil, true)

	w, c := run(t, mw, jwtCtx("sub-new", "dev@x.io", "", uuid.Nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, DefaultTenantID, c.tenantID)
	assert.Equal(t, "owner", c.role, "first user of the (empty) default tenant becomes owner")
	require.Len(t, users.created, 1)
}

func TestTenant_TransientLookupErrorDoesNotProvision(t *testing.T) {
	tenantA := uuid.New()
	users := &fakeUserRepo{lookupErr: errors.New("connection refused")}
	tenants := &fakeTenantRepo{tenants: map[uuid.UUID]*model.Tenant{tenantA: workspace(tenantA)}}
	mw := newMW(users, tenants, nil, true)

	w, c := run(t, mw, jwtCtx("sub-1", "x@x.io", "", tenantA))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.False(t, c.called)
	assert.Empty(t, users.created, "a DB outage must not create duplicate user records")
}

func TestTenant_MissingSubjectIsUnauthorized(t *testing.T) {
	mw := newMW(&fakeUserRepo{}, &fakeTenantRepo{tenants: map[uuid.UUID]*model.Tenant{}}, nil, true)

	w, c := run(t, mw, jwtCtx("", "x@x.io", "", uuid.Nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, c.called)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func TestMaskEmail(t *testing.T) {
	assert.Equal(t, "a***@example.com", maskEmail("alice@example.com"))
	assert.Equal(t, "", maskEmail("not-an-email"))
	assert.Equal(t, "", maskEmail(""))
}
