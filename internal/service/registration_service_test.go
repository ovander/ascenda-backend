package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/socrate"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── mock SocrateRegistrar ─────────────────────────────────────────────────────

type mockSocrateRegistrar struct {
	nextID  uint
	failOn  string // fail when this email is seen
	failErr error
}

func (m *mockSocrateRegistrar) RegisterUser(_ context.Context, req socrate.CreateUserRequest) (*socrate.CreateUserResult, error) {
	if m.failOn != "" && req.Email == m.failOn {
		return nil, m.failErr
	}
	m.nextID++
	return &socrate.CreateUserResult{UserID: m.nextID}, nil
}

func (m *mockSocrateRegistrar) CreateUser(_ context.Context, req socrate.CreateUserRequest) (*socrate.CreateUserResult, error) {
	return m.RegisterUser(context.Background(), req)
}

// ── helper ────────────────────────────────────────────────────────────────────

func newTestRegistrationService(registrar SocrateRegistrar) (*RegistrationService, *MockUserRepo, *mockAdminTenantRepo) {
	userRepo := NewMockUserRepo()
	tenantRepo := newMockAdminTenantRepo()
	logger := logrus.NewEntry(logrus.New())
	// seedService = nil (nil-guarded in Register, seeding skipped in unit tests)
	svc := NewRegistrationService(registrar, userRepo, tenantRepo, nil, logger)
	return svc, userRepo, tenantRepo
}

func validRegisterReq() RegisterRequest {
	return RegisterRequest{
		FirstName:   "Alice",
		LastName:    "Smith",
		CompanyName: "Acme Corp",
		Email:       "alice@acme.com",
		Country:     "BE",
	}
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestRegistrationService_HappyPath(t *testing.T) {
	reg := &mockSocrateRegistrar{}
	svc, userRepo, tenantRepo := newTestRegistrationService(reg)

	result, err := svc.Register(context.Background(), validRegisterReq())
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Message)

	// One tenant and one user should have been created.
	assert.Len(t, tenantRepo.tenants, 1)
	assert.NotEmpty(t, userRepo.users)

	// Check tenant defaults.
	for _, tenant := range tenantRepo.tenants {
		assert.True(t, tenant.IsActive)
		assert.NotEmpty(t, tenant.Slug)
	}
}

func TestRegistrationService_NoSocrate_StillCreatesLocalRecords(t *testing.T) {
	// nil registrar → graceful degradation (dev mode).
	svc, userRepo, tenantRepo := newTestRegistrationService(nil)

	_, err := svc.Register(context.Background(), validRegisterReq())
	require.NoError(t, err)
	assert.Len(t, tenantRepo.tenants, 1)
	assert.NotEmpty(t, userRepo.users)
}

func TestRegistrationService_UserRole_IsOwner(t *testing.T) {
	reg := &mockSocrateRegistrar{}
	svc, userRepo, _ := newTestRegistrationService(reg)

	_, _ = svc.Register(context.Background(), validRegisterReq()) //nolint:errcheck

	for _, u := range userRepo.users {
		if u.Email == "alice@acme.com" {
			assert.Equal(t, "owner", u.Role)
			assert.True(t, u.IsActive)
			return
		}
	}
	t.Fatal("user not found in repo")
}

func TestRegistrationService_SlugBuiltFromCompanyName(t *testing.T) {
	svc, _, tenantRepo := newTestRegistrationService(nil)
	req := validRegisterReq()
	req.CompanyName = "Hello World SAS"

	_, _ = svc.Register(context.Background(), req) //nolint:errcheck

	for _, tenant := range tenantRepo.tenants {
		assert.Contains(t, tenant.Slug, "hello-world-sas")
	}
}

func TestRegistrationService_InvalidCountry_Returns400(t *testing.T) {
	svc, _, _ := newTestRegistrationService(nil)
	req := validRegisterReq()
	req.Country = "ZZ" // not in the allow-list

	_, err := svc.Register(context.Background(), req)
	require.Error(t, err)
	appErr, ok := err.(*apierror.AppError)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, appErr.StatusCode)
}

func TestRegistrationService_CountryCaseInsensitive(t *testing.T) {
	svc, _, _ := newTestRegistrationService(nil)
	req := validRegisterReq()
	req.Country = "be" // lowercase should be accepted

	_, err := svc.Register(context.Background(), req)
	require.NoError(t, err)
}

func TestRegistrationService_SocrateAlreadyExists_Returns409(t *testing.T) {
	reg := &mockSocrateRegistrar{
		failOn:  "alice@acme.com",
		failErr: socrate.ErrUserAlreadyExists,
	}
	svc, _, _ := newTestRegistrationService(reg)

	_, err := svc.Register(context.Background(), validRegisterReq())
	require.Error(t, err)
	appErr, ok := err.(*apierror.AppError)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, appErr.StatusCode)
}

func TestRegistrationService_SocrateInternalError_Returns500(t *testing.T) {
	reg := &mockSocrateRegistrar{
		failOn:  "alice@acme.com",
		failErr: apierror.Internal("idp error"),
	}
	svc, _, _ := newTestRegistrationService(reg)

	_, err := svc.Register(context.Background(), validRegisterReq())
	require.Error(t, err)
	appErr, ok := err.(*apierror.AppError)
	require.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, appErr.StatusCode)
}

func TestRegistrationService_ValidCountries(t *testing.T) {
	countries := []string{"BE", "FR", "LU", "NL", "DE", "GB", "CH", "ES", "IT", "PT", "IE", "US", "CA"}
	for _, cc := range countries {
		t.Run(cc, func(t *testing.T) {
			svc, _, _ := newTestRegistrationService(nil)
			req := validRegisterReq()
			req.Country = cc
			_, err := svc.Register(context.Background(), req)
			assert.NoError(t, err, "country %s should be accepted", cc)
		})
	}
}

// ── buildSlug unit tests ──────────────────────────────────────────────────────

func TestBuildSlug_Basic(t *testing.T) {
	s := buildSlug("Acme Corp.")
	// Should start with acme-corp and end with an 8-char suffix after a hyphen.
	assert.Contains(t, s, "acme-corp-")
}

func TestBuildSlug_SpecialCharsStripped(t *testing.T) {
	s := buildSlug("Foo & Bar! Inc.")
	assert.Contains(t, s, "foo-bar-inc-")
}

func TestBuildSlug_EmptyName_Fallback(t *testing.T) {
	s := buildSlug("   ")
	assert.Contains(t, s, "company-")
}

func TestBuildSlug_Uniqueness(t *testing.T) {
	// Same input should produce different slugs (UUID suffix differs each call).
	s1 := buildSlug("Same Name")
	s2 := buildSlug("Same Name")
	assert.NotEqual(t, s1, s2)
}
