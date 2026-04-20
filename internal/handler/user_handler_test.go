package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/ctxutil"
)

// mockUserService is a test double for UserServiceIface.
type mockUserService struct{}

func (m *mockUserService) ListUsers(_ context.Context, _ uuid.UUID, _, _ int) ([]*model.User, int64, error) {
	return []*model.User{}, 0, nil
}
func (m *mockUserService) InviteUser(_ context.Context, _ uuid.UUID, _ uuid.UUID, email, _ string, role string) (*model.User, error) {
	return &model.User{ID: uuid.New(), Email: email, Role: role, IsActive: true}, nil
}
func (m *mockUserService) UpdateRole(_ context.Context, _ uuid.UUID, _ uuid.UUID, _, _ string) error {
	return nil
}
func (m *mockUserService) DeactivateUser(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}
func (m *mockUserService) ReactivateUser(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}
func (m *mockUserService) DeleteUser(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}

// newTestUserHandler creates a handler with a mock service.
func newTestUserHandler() *UserHandler {
	return NewUserHandler(&mockUserService{}, logrus.NewEntry(logrus.New()))
}

func TestUserHandlerGetMe(t *testing.T) {
	handler := newTestUserHandler()

	userID := uuid.New()
	tenantID := uuid.New()
	role := "admin"

	ctx := context.Background()
	ctx = ctxutil.WithUserID(ctx, userID)
	ctx = ctxutil.WithTenantID(ctx, tenantID)
	ctx = ctxutil.WithUserRole(ctx, role)

	r := httptest.NewRequest("GET", "/me", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.GetMe(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var user UserDTO
	err := json.Unmarshal(w.Body.Bytes(), &user)
	assert.NoError(t, err)
	assert.Equal(t, userID.String(), user.ID)
	assert.Equal(t, role, user.Role)
	assert.Empty(t, user.Name)
}

func TestUserHandlerGetMeWithEmailAndName(t *testing.T) {
	handler := newTestUserHandler()

	userID := uuid.New()
	ctx := context.Background()
	ctx = ctxutil.WithUserID(ctx, userID)
	ctx = ctxutil.WithUserEmail(ctx, "test@ascenda.io")
	ctx = ctxutil.WithUserName(ctx, "Test User")
	ctx = ctxutil.WithUserRole(ctx, "owner")

	r := httptest.NewRequest("GET", "/me", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.GetMe(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var user UserDTO
	json.Unmarshal(w.Body.Bytes(), &user)
	assert.Equal(t, "test@ascenda.io", user.Email)
	assert.Equal(t, "Test User", user.Name)
	assert.Equal(t, "owner", user.Role)
}

func TestUserHandlerGetMeDefaultsToEditor(t *testing.T) {
	handler := newTestUserHandler()

	r := httptest.NewRequest("GET", "/me", nil)
	w := httptest.NewRecorder()

	handler.GetMe(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var user UserDTO
	json.Unmarshal(w.Body.Bytes(), &user)
	assert.Equal(t, "editor", user.Role)
	assert.Equal(t, uuid.Nil.String(), user.ID)
}

func TestUserHandlerListAllowsAdmin(t *testing.T) {
	handler := newTestUserHandler()

	tenantID := uuid.New()
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)
	ctx = ctxutil.WithUserRole(ctx, "admin")

	r := httptest.NewRequest("GET", "/users", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.List(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUserHandlerListAllowsOwner(t *testing.T) {
	handler := newTestUserHandler()

	tenantID := uuid.New()
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)
	ctx = ctxutil.WithUserRole(ctx, "owner")

	r := httptest.NewRequest("GET", "/users", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.List(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUserHandlerListDeniesUser(t *testing.T) {
	handler := newTestUserHandler()

	tenantID := uuid.New()
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)
	ctx = ctxutil.WithUserRole(ctx, "user")

	r := httptest.NewRequest("GET", "/users", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.List(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUserHandlerInviteWithValidData(t *testing.T) {
	handler := newTestUserHandler()

	tenantID := uuid.New()
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)
	ctx = ctxutil.WithUserRole(ctx, "admin")

	reqBody := `{"email": "newuser@example.com", "role": "editor"}`
	body := io.NopCloser(bytes.NewBufferString(reqBody))
	r := httptest.NewRequest("POST", "/users/invite", body).WithContext(ctx)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Invite(w, r)

	assert.Equal(t, http.StatusCreated, w.Code)

	var result map[string]string
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Equal(t, "newuser@example.com", result["email"])
	assert.Equal(t, "editor", result["role"])
}

func TestUserHandlerInviteDeniesUser(t *testing.T) {
	handler := newTestUserHandler()

	tenantID := uuid.New()
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)
	ctx = ctxutil.WithUserRole(ctx, "user")

	reqBody := `{"email": "newuser@example.com", "role": "editor"}`
	body := io.NopCloser(bytes.NewBufferString(reqBody))
	r := httptest.NewRequest("POST", "/users/invite", body).WithContext(ctx)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Invite(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUserHandlerInviteInvalidEmail(t *testing.T) {
	handler := newTestUserHandler()

	tenantID := uuid.New()
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)
	ctx = ctxutil.WithUserRole(ctx, "admin")

	reqBody := `{"email": "notanemail", "role": "editor"}`
	body := io.NopCloser(bytes.NewBufferString(reqBody))
	r := httptest.NewRequest("POST", "/users/invite", body).WithContext(ctx)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Invite(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestUserHandlerUpdateRoleInvalidUUID(t *testing.T) {
	handler := newTestUserHandler()

	tenantID := uuid.New()
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)
	ctx = ctxutil.WithUserRole(ctx, "admin")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("userId", "invalid-uuid")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

	reqBody := `{"role": "editor"}`
	body := io.NopCloser(bytes.NewBufferString(reqBody))
	r := httptest.NewRequest("PUT", "/users/invalid-uuid/role", body).WithContext(ctx)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateRole(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUserHandlerDeactivateInvalidUUID(t *testing.T) {
	handler := newTestUserHandler()

	tenantID := uuid.New()
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("userId", "invalid-uuid")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

	r := httptest.NewRequest("POST", "/users/invalid-uuid/deactivate", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.Deactivate(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUserHandlerUpdateMe(t *testing.T) {
	handler := newTestUserHandler()

	userID := uuid.New()
	ctx := context.Background()
	ctx = ctxutil.WithUserID(ctx, userID)

	reqBody := `{"name": "Updated Name"}`
	body := io.NopCloser(bytes.NewBufferString(reqBody))
	r := httptest.NewRequest("PUT", "/me", body).WithContext(ctx)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateMe(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
}
