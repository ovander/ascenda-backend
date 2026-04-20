package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"ascenda/internal/dto"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
)

// mockTenantService implements service.TenantServicer for handler unit tests.
// It decouples the handler tests from the service and repository implementations.
type mockTenantService struct {
	tenant *model.Tenant
	err    error
}

func (m *mockTenantService) GetTenant(_ context.Context, _ uuid.UUID) (*model.Tenant, error) {
	if m.err != nil {
		return nil, apierror.NotFound("tenant", "mock")
	}
	return m.tenant, nil
}

func (m *mockTenantService) UpdateTenant(_ context.Context, _ uuid.UUID, name string) error {
	return m.err
}

func newTestTenantHandler(svc *mockTenantService) *TenantHandler {
	return NewTenantHandler(svc, logrus.NewEntry(logrus.StandardLogger()))
}

func TestTenantHandlerGet(t *testing.T) {
	tenantID := uuid.New()
	svc := &mockTenantService{tenant: &model.Tenant{
		ID:       tenantID,
		Name:     "Acme",
		Slug:     "acme",
		IsActive: true,
	}}
	handler := newTestTenantHandler(svc)

	ctx := ctxutil.WithTenantID(context.Background(), tenantID)
	r := httptest.NewRequest("GET", "/tenant", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.Get(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp dto.TenantResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, tenantID.String(), resp.ID)
	assert.Equal(t, "Acme", resp.Name)
	assert.Equal(t, "acme", resp.Slug)
	assert.True(t, resp.IsActive)
}

func TestTenantHandlerGetRepoError(t *testing.T) {
	tenantID := uuid.New()
	svc := &mockTenantService{err: errors.New("db error")}
	handler := newTestTenantHandler(svc)

	ctx := ctxutil.WithTenantID(context.Background(), tenantID)
	r := httptest.NewRequest("GET", "/tenant", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.Get(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTenantHandlerUpdateWithAdmin(t *testing.T) {
	svc := &mockTenantService{}
	handler := newTestTenantHandler(svc)

	tenantID := uuid.New()
	ctx := ctxutil.WithTenantID(context.Background(), tenantID)
	ctx = ctxutil.WithUserRole(ctx, "admin")

	body := io.NopCloser(bytes.NewBufferString(`{"name": "Updated Tenant Name"}`))
	r := httptest.NewRequest("PUT", "/tenant", body).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.Update(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestTenantHandlerUpdateWithOwner(t *testing.T) {
	svc := &mockTenantService{}
	handler := newTestTenantHandler(svc)

	tenantID := uuid.New()
	ctx := ctxutil.WithTenantID(context.Background(), tenantID)
	ctx = ctxutil.WithUserRole(ctx, "owner")

	body := io.NopCloser(bytes.NewBufferString(`{"name": "Updated Tenant Name"}`))
	r := httptest.NewRequest("PUT", "/tenant", body).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.Update(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestTenantHandlerUpdateDeniesViewer(t *testing.T) {
	svc := &mockTenantService{}
	handler := newTestTenantHandler(svc)

	tenantID := uuid.New()
	ctx := ctxutil.WithTenantID(context.Background(), tenantID)
	ctx = ctxutil.WithUserRole(ctx, "viewer")

	body := io.NopCloser(bytes.NewBufferString(`{"name": "Updated Tenant Name"}`))
	r := httptest.NewRequest("PUT", "/tenant", body).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.Update(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTenantHandlerUpdateDeniesCollaborator(t *testing.T) {
	svc := &mockTenantService{}
	handler := newTestTenantHandler(svc)

	tenantID := uuid.New()
	ctx := ctxutil.WithTenantID(context.Background(), tenantID)
	ctx = ctxutil.WithUserRole(ctx, "collaborator")

	body := io.NopCloser(bytes.NewBufferString(`{"name": "Updated Tenant Name"}`))
	r := httptest.NewRequest("PUT", "/tenant", body).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.Update(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTenantHandlerUpdateInvalidJSON(t *testing.T) {
	svc := &mockTenantService{}
	handler := newTestTenantHandler(svc)

	tenantID := uuid.New()
	ctx := ctxutil.WithTenantID(context.Background(), tenantID)
	ctx = ctxutil.WithUserRole(ctx, "admin")

	body := io.NopCloser(bytes.NewBufferString(`{invalid json}`))
	r := httptest.NewRequest("PUT", "/tenant", body).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.Update(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
