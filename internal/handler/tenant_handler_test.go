package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"kerplan/internal/pkg/ctxutil"
)

func TestTenantHandlerGet(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	handler := NewTenantHandler(logger)

	tenantID := uuid.New()
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	r := httptest.NewRequest("GET", "/tenant", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.Get(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var tenant TenantDTO
	err := json.Unmarshal(w.Body.Bytes(), &tenant)
	assert.NoError(t, err)
	assert.Equal(t, tenantID.String(), tenant.ID)
	assert.Equal(t, "Tenant", tenant.Name)
	assert.Equal(t, "tenant@example.com", tenant.Email)
}

func TestTenantHandlerGetWithMissingTenantID(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	handler := NewTenantHandler(logger)

	// Create request without tenant context
	r := httptest.NewRequest("GET", "/tenant", nil)
	w := httptest.NewRecorder()

	handler.Get(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var tenant TenantDTO
	err := json.Unmarshal(w.Body.Bytes(), &tenant)
	assert.NoError(t, err)
	// nil UUID converts to "00000000-0000-0000-0000-000000000000"
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", tenant.ID)
}

func TestTenantHandlerUpdateWithAdmin(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	handler := NewTenantHandler(logger)

	tenantID := uuid.New()
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)
	ctx = ctxutil.WithUserRole(ctx, "admin")

	reqBody := `{"name": "Updated Tenant Name"}`
	body := io.NopCloser(bytes.NewBufferString(reqBody))
	r := httptest.NewRequest("PUT", "/tenant", body).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.Update(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestTenantHandlerUpdateWithOwner(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	handler := NewTenantHandler(logger)

	tenantID := uuid.New()
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)
	ctx = ctxutil.WithUserRole(ctx, "owner")

	reqBody := `{"name": "Updated Tenant Name"}`
	body := io.NopCloser(bytes.NewBufferString(reqBody))
	r := httptest.NewRequest("PUT", "/tenant", body).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.Update(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestTenantHandlerUpdateDeniesViewer(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	handler := NewTenantHandler(logger)

	tenantID := uuid.New()
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)
	ctx = ctxutil.WithUserRole(ctx, "viewer")

	reqBody := `{"name": "Updated Tenant Name"}`
	body := io.NopCloser(bytes.NewBufferString(reqBody))
	r := httptest.NewRequest("PUT", "/tenant", body).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.Update(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTenantHandlerUpdateDeniesCollaborator(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	handler := NewTenantHandler(logger)

	tenantID := uuid.New()
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)
	ctx = ctxutil.WithUserRole(ctx, "collaborator")

	reqBody := `{"name": "Updated Tenant Name"}`
	body := io.NopCloser(bytes.NewBufferString(reqBody))
	r := httptest.NewRequest("PUT", "/tenant", body).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.Update(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTenantHandlerUpdateInvalidJSON(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	handler := NewTenantHandler(logger)

	tenantID := uuid.New()
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)
	ctx = ctxutil.WithUserRole(ctx, "admin")

	body := io.NopCloser(bytes.NewBufferString(`{invalid json}`))
	r := httptest.NewRequest("PUT", "/tenant", body).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.Update(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
