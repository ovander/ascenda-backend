package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ascenda/internal/service"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── mockAdminUserService ──────────────────────────────────────────────────

type mockAdminUserService struct {
	listUsersFn          func(ctx context.Context, search string, page, pageSize int) (*service.AdminUserListResponse, error)
	getUserFn            func(ctx context.Context, socrateID string) (*service.AdminUserDTO, error)
	createUserFn         func(ctx context.Context, req service.CreateUserRequest) (*service.AdminUserDTO, error)
	updateUserFn         func(ctx context.Context, socrateID string, req service.UpdateUserRequest) (*service.AdminUserDTO, error)
	deleteUserFn         func(ctx context.Context, socrateID string) error
	resendVerificationFn func(ctx context.Context, socrateID string) error
	resetPasswordFn      func(ctx context.Context, socrateID string) error
	listTenantsFn        func(ctx context.Context, page, pageSize int) (*service.AdminTenantListResponse, error)
	getTenantFn          func(ctx context.Context, id uuid.UUID) (*service.AdminTenantDTO, error)
	createTenantFn       func(ctx context.Context, req service.CreateTenantRequest) (*service.AdminTenantDTO, error)
	updateTenantFn       func(ctx context.Context, id uuid.UUID, req service.UpdateTenantRequest) (*service.AdminTenantDTO, error)
}

func (m *mockAdminUserService) ListUsers(ctx context.Context, search string, page, pageSize int) (*service.AdminUserListResponse, error) {
	if m.listUsersFn != nil {
		return m.listUsersFn(ctx, search, page, pageSize)
	}
	return nil, apierror.Internal("list users not implemented")
}

func (m *mockAdminUserService) GetUser(ctx context.Context, socrateID string) (*service.AdminUserDTO, error) {
	if m.getUserFn != nil {
		return m.getUserFn(ctx, socrateID)
	}
	return nil, apierror.Internal("get user not implemented")
}

func (m *mockAdminUserService) CreateUser(ctx context.Context, req service.CreateUserRequest) (*service.AdminUserDTO, error) {
	if m.createUserFn != nil {
		return m.createUserFn(ctx, req)
	}
	return nil, apierror.Internal("create user not implemented")
}

func (m *mockAdminUserService) UpdateUser(ctx context.Context, socrateID string, req service.UpdateUserRequest) (*service.AdminUserDTO, error) {
	if m.updateUserFn != nil {
		return m.updateUserFn(ctx, socrateID, req)
	}
	return nil, apierror.Internal("update user not implemented")
}

func (m *mockAdminUserService) DeleteUser(ctx context.Context, socrateID string) error {
	if m.deleteUserFn != nil {
		return m.deleteUserFn(ctx, socrateID)
	}
	return apierror.Internal("delete user not implemented")
}

func (m *mockAdminUserService) ResendVerification(ctx context.Context, socrateID string) error {
	if m.resendVerificationFn != nil {
		return m.resendVerificationFn(ctx, socrateID)
	}
	return apierror.Internal("resend verification not implemented")
}

func (m *mockAdminUserService) ResetPassword(ctx context.Context, socrateID string) error {
	if m.resetPasswordFn != nil {
		return m.resetPasswordFn(ctx, socrateID)
	}
	return apierror.Internal("reset password not implemented")
}

func (m *mockAdminUserService) ListTenants(ctx context.Context, page, pageSize int) (*service.AdminTenantListResponse, error) {
	if m.listTenantsFn != nil {
		return m.listTenantsFn(ctx, page, pageSize)
	}
	return nil, apierror.Internal("list tenants not implemented")
}

func (m *mockAdminUserService) GetTenant(ctx context.Context, id uuid.UUID) (*service.AdminTenantDTO, error) {
	if m.getTenantFn != nil {
		return m.getTenantFn(ctx, id)
	}
	return nil, apierror.Internal("get tenant not implemented")
}

func (m *mockAdminUserService) CreateTenant(ctx context.Context, req service.CreateTenantRequest) (*service.AdminTenantDTO, error) {
	if m.createTenantFn != nil {
		return m.createTenantFn(ctx, req)
	}
	return nil, apierror.Internal("create tenant not implemented")
}

func (m *mockAdminUserService) UpdateTenant(ctx context.Context, id uuid.UUID, req service.UpdateTenantRequest) (*service.AdminTenantDTO, error) {
	if m.updateTenantFn != nil {
		return m.updateTenantFn(ctx, id, req)
	}
	return nil, apierror.Internal("update tenant not implemented")
}

// ── fixtures ──────────────────────────────────────────────────────────────

func newAdminUserHandler(svc *mockAdminUserService) *AdminUserHandler {
	return NewAdminUserHandler(svc, logrus.NewEntry(logrus.New()))
}

// ── ListUsers ─────────────────────────────────────────────────────────────

func TestAdminUserHandler_ListUsers_Success(t *testing.T) {
	result := &service.AdminUserListResponse{
		Users:      []service.AdminUserDTO{},
		TotalCount: 0,
		Page:       1,
		PageSize:   20,
	}

	svc := &mockAdminUserService{
		listUsersFn: func(_ context.Context, search string, page, pageSize int) (*service.AdminUserListResponse, error) {
			assert.Equal(t, "", search)
			assert.Equal(t, 1, page)
			assert.Equal(t, 20, pageSize)
			return result, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/?page=1&pageSize=20", nil)
	r.Header.Set("Authorization", "Bearer token123")
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).ListUsers(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got service.AdminUserListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, int64(0), got.TotalCount)
}

func TestAdminUserHandler_ListUsers_PageSizeMax(t *testing.T) {
	result := &service.AdminUserListResponse{
		Users:      []service.AdminUserDTO{},
		TotalCount: 0,
		Page:       1,
		PageSize:   100,
	}

	svc := &mockAdminUserService{
		listUsersFn: func(_ context.Context, _ string, page, pageSize int) (*service.AdminUserListResponse, error) {
			// pageSize > 100 should be capped to 100
			assert.Equal(t, 100, pageSize)
			return result, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/?pageSize=999", nil)
	r.Header.Set("Authorization", "Bearer token123")
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).ListUsers(w, r)

	require.Equal(t, http.StatusOK, w.Code)
}

// ── GetUser ───────────────────────────────────────────────────────────────

func TestAdminUserHandler_GetUser_Success(t *testing.T) {
	userID := "12345"
	user := &service.AdminUserDTO{
		SocrateID: 12345,
		Email:     "user@example.com",
		Name:      "John Doe",
		IsActive:  true,
	}

	svc := &mockAdminUserService{
		getUserFn: func(_ context.Context, sid string) (*service.AdminUserDTO, error) {
			assert.Equal(t, userID, sid)
			return user, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer token123")
	r = withChiParams(r, map[string]string{"userId": userID})
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).GetUser(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got service.AdminUserDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "user@example.com", got.Email)
}

func TestAdminUserHandler_GetUser_MissingUserId(t *testing.T) {
	svc := &mockAdminUserService{}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer token123")
	r = withChiParams(r, map[string]string{"userId": ""})
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).GetUser(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminUserHandler_GetUser_NotFound(t *testing.T) {
	svc := &mockAdminUserService{
		getUserFn: func(_ context.Context, _ string) (*service.AdminUserDTO, error) {
			return nil, apierror.NotFound("user", "xyz")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer token123")
	r = withChiParams(r, map[string]string{"userId": "xyz"})
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).GetUser(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── CreateUser ────────────────────────────────────────────────────────────

func TestAdminUserHandler_CreateUser_Success(t *testing.T) {
	user := &service.AdminUserDTO{
		SocrateID: 999,
		Email:     "newuser@example.com",
		Name:      "New User",
		IsActive:  true,
	}

	svc := &mockAdminUserService{
		createUserFn: func(_ context.Context, req service.CreateUserRequest) (*service.AdminUserDTO, error) {
			assert.Equal(t, "newuser@example.com", req.Email)
			assert.Equal(t, "New User", req.FullName)
			return user, nil
		},
	}

	req := service.CreateUserRequest{
		Email:    "newuser@example.com",
		FullName: "New User",
		Role:     "user",
	}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer token123")
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).CreateUser(w, r)

	require.Equal(t, http.StatusCreated, w.Code)
	var got service.AdminUserDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "newuser@example.com", got.Email)
}

func TestAdminUserHandler_CreateUser_InvalidBody(t *testing.T) {
	svc := &mockAdminUserService{}

	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("invalid json")))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer token123")
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).CreateUser(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminUserHandler_CreateUser_MissingEmail(t *testing.T) {
	svc := &mockAdminUserService{}

	// Missing email field
	req := service.CreateUserRequest{
		FullName: "New User",
	}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer token123")
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).CreateUser(w, r)

	// Validation should fail (email is required)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// ── UpdateUser ────────────────────────────────────────────────────────────

func TestAdminUserHandler_UpdateUser_Success(t *testing.T) {
	userID := "42"
	user := &service.AdminUserDTO{
		SocrateID: 42,
		Email:     "user@example.com",
		Name:      "Updated Name",
		IsActive:  true,
	}

	svc := &mockAdminUserService{
		updateUserFn: func(_ context.Context, sid string, req service.UpdateUserRequest) (*service.AdminUserDTO, error) {
			assert.Equal(t, userID, sid)
			return user, nil
		},
	}

	fullName := "Updated Name"
	req := service.UpdateUserRequest{
		FullName: &fullName,
	}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer token123")
	r = withChiParams(r, map[string]string{"userId": userID})
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).UpdateUser(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got service.AdminUserDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "Updated Name", got.Name)
}

func TestAdminUserHandler_UpdateUser_MissingUserId(t *testing.T) {
	svc := &mockAdminUserService{}

	req := service.UpdateUserRequest{}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer token123")
	r = withChiParams(r, map[string]string{"userId": ""})
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).UpdateUser(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminUserHandler_UpdateUser_InvalidBody(t *testing.T) {
	svc := &mockAdminUserService{}

	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader([]byte("invalid")))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer token123")
	r = withChiParams(r, map[string]string{"userId": "42"})
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).UpdateUser(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── DeleteUser ────────────────────────────────────────────────────────────

func TestAdminUserHandler_DeleteUser_Success(t *testing.T) {
	userID := "42"
	called := false

	svc := &mockAdminUserService{
		deleteUserFn: func(_ context.Context, sid string) error {
			assert.Equal(t, userID, sid)
			called = true
			return nil
		},
	}

	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r.Header.Set("Authorization", "Bearer token123")
	r = withChiParams(r, map[string]string{"userId": userID})
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).DeleteUser(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, called)
}

func TestAdminUserHandler_DeleteUser_MissingUserId(t *testing.T) {
	svc := &mockAdminUserService{}

	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r.Header.Set("Authorization", "Bearer token123")
	r = withChiParams(r, map[string]string{"userId": ""})
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).DeleteUser(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── ResendVerification ────────────────────────────────────────────────────

func TestAdminUserHandler_ResendVerification_Success(t *testing.T) {
	userID := "42"
	called := false

	svc := &mockAdminUserService{
		resendVerificationFn: func(_ context.Context, sid string) error {
			assert.Equal(t, userID, sid)
			called = true
			return nil
		},
	}

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Authorization", "Bearer token123")
	r = withChiParams(r, map[string]string{"userId": userID})
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).ResendVerification(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, called)
}

func TestAdminUserHandler_ResendVerification_MissingUserId(t *testing.T) {
	svc := &mockAdminUserService{}

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Authorization", "Bearer token123")
	r = withChiParams(r, map[string]string{"userId": ""})
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).ResendVerification(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── ResetPassword ─────────────────────────────────────────────────────────

func TestAdminUserHandler_ResetPassword_Success(t *testing.T) {
	userID := "42"
	called := false

	svc := &mockAdminUserService{
		resetPasswordFn: func(_ context.Context, sid string) error {
			assert.Equal(t, userID, sid)
			called = true
			return nil
		},
	}

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Authorization", "Bearer token123")
	r = withChiParams(r, map[string]string{"userId": userID})
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).ResetPassword(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, called)
}

func TestAdminUserHandler_ResetPassword_MissingUserId(t *testing.T) {
	svc := &mockAdminUserService{}

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Authorization", "Bearer token123")
	r = withChiParams(r, map[string]string{"userId": ""})
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).ResetPassword(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── ListTenants ───────────────────────────────────────────────────────────

func TestAdminUserHandler_ListTenants_Success(t *testing.T) {
	result := &service.AdminTenantListResponse{
		Tenants:    []service.AdminTenantDTO{},
		TotalCount: 0,
		Page:       1,
		PageSize:   20,
	}

	svc := &mockAdminUserService{
		listTenantsFn: func(_ context.Context, page, pageSize int) (*service.AdminTenantListResponse, error) {
			assert.Equal(t, 1, page)
			assert.Equal(t, 20, pageSize)
			return result, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/?page=1&pageSize=20", nil)
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).ListTenants(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got service.AdminTenantListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, int64(0), got.TotalCount)
}

func TestAdminUserHandler_ListTenants_PageSizeMax(t *testing.T) {
	result := &service.AdminTenantListResponse{
		Tenants:    []service.AdminTenantDTO{},
		TotalCount: 0,
		Page:       1,
		PageSize:   100,
	}

	svc := &mockAdminUserService{
		listTenantsFn: func(_ context.Context, page, pageSize int) (*service.AdminTenantListResponse, error) {
			// pageSize > 100 should be capped to 100
			assert.Equal(t, 100, pageSize)
			return result, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/?pageSize=999", nil)
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).ListTenants(w, r)

	require.Equal(t, http.StatusOK, w.Code)
}

// ── GetTenant ─────────────────────────────────────────────────────────────

func TestAdminUserHandler_GetTenant_Success(t *testing.T) {
	tenantID := uuid.New()
	tenant := &service.AdminTenantDTO{
		ID:   tenantID.String(),
		Name: "ACME Corp",
		Slug: "acme",
	}

	svc := &mockAdminUserService{
		getTenantFn: func(_ context.Context, id uuid.UUID) (*service.AdminTenantDTO, error) {
			assert.Equal(t, tenantID, id)
			return tenant, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"tenantId": tenantID.String()})
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).GetTenant(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got service.AdminTenantDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "ACME Corp", got.Name)
}

func TestAdminUserHandler_GetTenant_BadUUID(t *testing.T) {
	svc := &mockAdminUserService{}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"tenantId": "not-a-uuid"})
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).GetTenant(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminUserHandler_GetTenant_NotFound(t *testing.T) {
	tenantID := uuid.New()

	svc := &mockAdminUserService{
		getTenantFn: func(_ context.Context, _ uuid.UUID) (*service.AdminTenantDTO, error) {
			return nil, apierror.NotFound("tenant", tenantID.String())
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"tenantId": tenantID.String()})
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).GetTenant(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── CreateTenant ──────────────────────────────────────────────────────────

func TestAdminUserHandler_CreateTenant_Success(t *testing.T) {
	tenant := &service.AdminTenantDTO{
		ID:   uuid.New().String(),
		Name: "New Tenant",
		Slug: "new-tenant",
	}

	svc := &mockAdminUserService{
		createTenantFn: func(_ context.Context, req service.CreateTenantRequest) (*service.AdminTenantDTO, error) {
			assert.Equal(t, "New Tenant", req.Name)
			assert.Equal(t, "new-tenant", req.Slug)
			return tenant, nil
		},
	}

	req := service.CreateTenantRequest{
		Name: "New Tenant",
		Slug: "new-tenant",
	}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).CreateTenant(w, r)

	require.Equal(t, http.StatusCreated, w.Code)
	var got service.AdminTenantDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "New Tenant", got.Name)
}

func TestAdminUserHandler_CreateTenant_InvalidBody(t *testing.T) {
	svc := &mockAdminUserService{}

	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("invalid")))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).CreateTenant(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminUserHandler_CreateTenant_MissingName(t *testing.T) {
	svc := &mockAdminUserService{}

	// Missing name field
	req := service.CreateTenantRequest{
		Slug: "new-tenant",
	}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).CreateTenant(w, r)

	// Validation should fail (name is required)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// ── UpdateTenant ──────────────────────────────────────────────────────────

func TestAdminUserHandler_UpdateTenant_Success(t *testing.T) {
	tenantID := uuid.New()
	tenant := &service.AdminTenantDTO{
		ID:   tenantID.String(),
		Name: "Updated Tenant",
		Slug: "new-tenant",
	}

	svc := &mockAdminUserService{
		updateTenantFn: func(_ context.Context, id uuid.UUID, req service.UpdateTenantRequest) (*service.AdminTenantDTO, error) {
			assert.Equal(t, tenantID, id)
			return tenant, nil
		},
	}

	name := "Updated Tenant"
	req := service.UpdateTenantRequest{
		Name: &name,
	}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"tenantId": tenantID.String()})
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).UpdateTenant(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got service.AdminTenantDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "Updated Tenant", got.Name)
}

func TestAdminUserHandler_UpdateTenant_BadUUID(t *testing.T) {
	svc := &mockAdminUserService{}

	req := service.UpdateTenantRequest{}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"tenantId": "invalid"})
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).UpdateTenant(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminUserHandler_UpdateTenant_InvalidBody(t *testing.T) {
	tenantID := uuid.New()

	svc := &mockAdminUserService{}

	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader([]byte("bad json")))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"tenantId": tenantID.String()})
	w := httptest.NewRecorder()

	newAdminUserHandler(svc).UpdateTenant(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
