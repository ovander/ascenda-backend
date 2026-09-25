package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ascenda/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── mockOrgService ─────────────────────────────────────────────────────────

type mockOrgService struct {
	listOrgsFn    func(ctx context.Context, page, pageSize int) (*service.OrgListResponse, error)
	getOrgFn      func(ctx context.Context, id uuid.UUID) (*service.OrgDTO, error)
	createOrgFn   func(ctx context.Context, req service.CreateOrganizationRequest) (*service.OrgDTO, error)
	updateOrgFn   func(ctx context.Context, id uuid.UUID, req service.UpdateOrganizationRequest) (*service.OrgDTO, error)
	deleteOrgFn   func(ctx context.Context, id uuid.UUID) error
	addTenantFn   func(ctx context.Context, orgID uuid.UUID, req service.AddOrgTenantRequest) (*service.OrgTenantDTO, error)
	listTenantsFn func(ctx context.Context, orgID uuid.UUID) ([]service.OrgTenantDTO, error)
}

func (m *mockOrgService) ListOrganizations(ctx context.Context, page, pageSize int) (*service.OrgListResponse, error) {
	if m.listOrgsFn != nil {
		return m.listOrgsFn(ctx, page, pageSize)
	}
	return nil, apierror.Internal("not implemented")
}

func (m *mockOrgService) GetOrganization(ctx context.Context, id uuid.UUID) (*service.OrgDTO, error) {
	if m.getOrgFn != nil {
		return m.getOrgFn(ctx, id)
	}
	return nil, apierror.Internal("not implemented")
}

func (m *mockOrgService) CreateOrganization(ctx context.Context, req service.CreateOrganizationRequest) (*service.OrgDTO, error) {
	if m.createOrgFn != nil {
		return m.createOrgFn(ctx, req)
	}
	return nil, apierror.Internal("not implemented")
}

func (m *mockOrgService) UpdateOrganization(ctx context.Context, id uuid.UUID, req service.UpdateOrganizationRequest) (*service.OrgDTO, error) {
	if m.updateOrgFn != nil {
		return m.updateOrgFn(ctx, id, req)
	}
	return nil, apierror.Internal("not implemented")
}

func (m *mockOrgService) DeleteOrganization(ctx context.Context, id uuid.UUID) error {
	if m.deleteOrgFn != nil {
		return m.deleteOrgFn(ctx, id)
	}
	return apierror.Internal("not implemented")
}

func (m *mockOrgService) AddTenant(ctx context.Context, orgID uuid.UUID, req service.AddOrgTenantRequest) (*service.OrgTenantDTO, error) {
	if m.addTenantFn != nil {
		return m.addTenantFn(ctx, orgID, req)
	}
	return nil, apierror.Internal("not implemented")
}

func (m *mockOrgService) ListTenants(ctx context.Context, orgID uuid.UUID) ([]service.OrgTenantDTO, error) {
	if m.listTenantsFn != nil {
		return m.listTenantsFn(ctx, orgID)
	}
	return nil, apierror.Internal("not implemented")
}

// ── fixtures ──────────────────────────────────────────────────────────────

func newAdminOrgHandler(svc *mockOrgService) *AdminOrgHandler {
	return NewAdminOrgHandler(svc, logrus.NewEntry(logrus.New()))
}

func orgFixture() service.OrgDTO {
	return service.OrgDTO{
		ID:           uuid.New().String(),
		Name:         "Orange SA",
		Slug:         "orange-sa",
		Plan:         "enterprise",
		MaxUsers:     1000,
		BillingEmail: "cfo@orange.fr",
		IsActive:     true,
		TenantCount:  22,
		UserCount:    850,
	}
}

func tenantDTOFixture() service.OrgTenantDTO {
	return service.OrgTenantDTO{
		ID:       uuid.New().String(),
		Name:     "Orange Finance",
		Slug:     "orange-finance",
		Plan:     "enterprise",
		IsActive: true,
	}
}

// setURLParam injects a chi URL param into the request context.
func setURLParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// ── ListOrganizations ─────────────────────────────────────────────────────

func TestAdminOrgHandler_ListOrganizations_Success(t *testing.T) {
	org := orgFixture()
	svc := &mockOrgService{
		listOrgsFn: func(_ context.Context, page, pageSize int) (*service.OrgListResponse, error) {
			assert.Equal(t, 1, page)
			assert.Equal(t, 20, pageSize)
			return &service.OrgListResponse{
				Organizations: []service.OrgDTO{org},
				TotalCount:    1,
				Page:          1,
				PageSize:      20,
			}, nil
		},
	}

	h := newAdminOrgHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organizations", nil)
	w := httptest.NewRecorder()
	h.ListOrganizations(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body service.OrgListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, int64(1), body.TotalCount)
	assert.Equal(t, org.Name, body.Organizations[0].Name)
}

func TestAdminOrgHandler_ListOrganizations_ServiceError(t *testing.T) {
	svc := &mockOrgService{
		listOrgsFn: func(_ context.Context, _, _ int) (*service.OrgListResponse, error) {
			return nil, apierror.Internal("db down")
		},
	}
	h := newAdminOrgHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organizations", nil)
	w := httptest.NewRecorder()
	h.ListOrganizations(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ── GetOrganization ───────────────────────────────────────────────────────

func TestAdminOrgHandler_GetOrganization_Success(t *testing.T) {
	org := orgFixture()
	orgID, _ := uuid.Parse(org.ID)
	svc := &mockOrgService{
		getOrgFn: func(_ context.Context, id uuid.UUID) (*service.OrgDTO, error) {
			assert.Equal(t, orgID, id)
			return &org, nil
		},
	}
	h := newAdminOrgHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organizations/"+org.ID, nil)
	req = setURLParam(req, "orgId", org.ID)
	w := httptest.NewRecorder()
	h.GetOrganization(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body service.OrgDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, org.Name, body.Name)
}

func TestAdminOrgHandler_GetOrganization_InvalidID(t *testing.T) {
	h := newAdminOrgHandler(&mockOrgService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organizations/not-a-uuid", nil)
	req = setURLParam(req, "orgId", "not-a-uuid")
	w := httptest.NewRecorder()
	h.GetOrganization(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminOrgHandler_GetOrganization_NotFound(t *testing.T) {
	id := uuid.New()
	svc := &mockOrgService{
		getOrgFn: func(_ context.Context, _ uuid.UUID) (*service.OrgDTO, error) {
			return nil, apierror.NotFound("organization", id.String())
		},
	}
	h := newAdminOrgHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organizations/"+id.String(), nil)
	req = setURLParam(req, "orgId", id.String())
	w := httptest.NewRecorder()
	h.GetOrganization(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── CreateOrganization ────────────────────────────────────────────────────

func TestAdminOrgHandler_CreateOrganization_Success(t *testing.T) {
	var gotReq service.CreateOrganizationRequest
	created := orgFixture()
	svc := &mockOrgService{
		createOrgFn: func(_ context.Context, req service.CreateOrganizationRequest) (*service.OrgDTO, error) {
			gotReq = req
			return &created, nil
		},
	}

	body := service.CreateOrganizationRequest{
		Name:         "Orange SA",
		Slug:         "orange-sa",
		Plan:         "enterprise",
		MaxUsers:     1000,
		BillingEmail: "cfo@orange.fr",
		OwnerEmail:   "admin@orange.fr",
		OwnerName:    "Jean Dupont",
	}
	b, _ := json.Marshal(body)

	h := newAdminOrgHandler(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/organizations", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateOrganization(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "orange-sa", gotReq.Slug)
	assert.Equal(t, "enterprise", gotReq.Plan)
	assert.Equal(t, "admin@orange.fr", gotReq.OwnerEmail)
}

func TestAdminOrgHandler_CreateOrganization_Conflict(t *testing.T) {
	svc := &mockOrgService{
		createOrgFn: func(_ context.Context, _ service.CreateOrganizationRequest) (*service.OrgDTO, error) {
			return nil, apierror.Conflict("slug already exists")
		},
	}
	body := service.CreateOrganizationRequest{Name: "X", Slug: "x"}
	b, _ := json.Marshal(body)
	h := newAdminOrgHandler(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/organizations", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateOrganization(w, req)
	assert.Equal(t, http.StatusConflict, w.Code)
}

// ── UpdateOrganization ────────────────────────────────────────────────────

func TestAdminOrgHandler_UpdateOrganization_Success(t *testing.T) {
	org := orgFixture()
	orgID, _ := uuid.Parse(org.ID)
	newName := "Orange Group"
	svc := &mockOrgService{
		updateOrgFn: func(_ context.Context, id uuid.UUID, req service.UpdateOrganizationRequest) (*service.OrgDTO, error) {
			assert.Equal(t, orgID, id)
			assert.Equal(t, newName, *req.Name)
			org.Name = newName
			return &org, nil
		},
	}

	body := service.UpdateOrganizationRequest{Name: &newName}
	b, _ := json.Marshal(body)
	h := newAdminOrgHandler(svc)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/organizations/"+org.ID, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = setURLParam(req, "orgId", org.ID)
	w := httptest.NewRecorder()
	h.UpdateOrganization(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp service.OrgDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, newName, resp.Name)
}

func TestAdminOrgHandler_UpdateOrganization_NotFound(t *testing.T) {
	id := uuid.New()
	svc := &mockOrgService{
		updateOrgFn: func(_ context.Context, _ uuid.UUID, _ service.UpdateOrganizationRequest) (*service.OrgDTO, error) {
			return nil, apierror.NotFound("organization", id.String())
		},
	}
	name := "X"
	body := service.UpdateOrganizationRequest{Name: &name}
	b, _ := json.Marshal(body)
	h := newAdminOrgHandler(svc)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/organizations/"+id.String(), bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = setURLParam(req, "orgId", id.String())
	w := httptest.NewRecorder()
	h.UpdateOrganization(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── DeleteOrganization ────────────────────────────────────────────────────

func TestAdminOrgHandler_DeleteOrganization_Success(t *testing.T) {
	id := uuid.New()
	deleted := false
	svc := &mockOrgService{
		deleteOrgFn: func(_ context.Context, orgID uuid.UUID) error {
			assert.Equal(t, id, orgID)
			deleted = true
			return nil
		},
	}
	h := newAdminOrgHandler(svc)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/organizations/"+id.String(), nil)
	req = setURLParam(req, "orgId", id.String())
	w := httptest.NewRecorder()
	h.DeleteOrganization(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, deleted)
}

func TestAdminOrgHandler_DeleteOrganization_InvalidID(t *testing.T) {
	h := newAdminOrgHandler(&mockOrgService{})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/organizations/bad-id", nil)
	req = setURLParam(req, "orgId", "bad-id")
	w := httptest.NewRecorder()
	h.DeleteOrganization(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── AddOrgTenant ──────────────────────────────────────────────────────────

func TestAdminOrgHandler_AddOrgTenant_Success(t *testing.T) {
	orgID := uuid.New()
	dept := tenantDTOFixture()
	svc := &mockOrgService{
		addTenantFn: func(_ context.Context, id uuid.UUID, req service.AddOrgTenantRequest) (*service.OrgTenantDTO, error) {
			assert.Equal(t, orgID, id)
			assert.Equal(t, "Orange Finance", req.Name)
			return &dept, nil
		},
	}
	body := service.AddOrgTenantRequest{Name: "Orange Finance", Slug: "orange-finance", OwnerEmail: "cfo.finance@orange.fr"}
	b, _ := json.Marshal(body)
	h := newAdminOrgHandler(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/organizations/"+orgID.String()+"/tenants", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = setURLParam(req, "orgId", orgID.String())
	w := httptest.NewRecorder()
	h.AddOrgTenant(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp service.OrgTenantDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, dept.Name, resp.Name)
}

func TestAdminOrgHandler_AddOrgTenant_OrgNotFound(t *testing.T) {
	orgID := uuid.New()
	svc := &mockOrgService{
		addTenantFn: func(_ context.Context, _ uuid.UUID, _ service.AddOrgTenantRequest) (*service.OrgTenantDTO, error) {
			return nil, apierror.NotFound("organization", orgID.String())
		},
	}
	body := service.AddOrgTenantRequest{Name: "X", Slug: "x"}
	b, _ := json.Marshal(body)
	h := newAdminOrgHandler(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/organizations/"+orgID.String()+"/tenants", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = setURLParam(req, "orgId", orgID.String())
	w := httptest.NewRecorder()
	h.AddOrgTenant(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── ListOrgTenants ────────────────────────────────────────────────────────

func TestAdminOrgHandler_ListOrgTenants_Success(t *testing.T) {
	orgID := uuid.New()
	dept := tenantDTOFixture()
	svc := &mockOrgService{
		listTenantsFn: func(_ context.Context, id uuid.UUID) ([]service.OrgTenantDTO, error) {
			assert.Equal(t, orgID, id)
			return []service.OrgTenantDTO{dept}, nil
		},
	}
	h := newAdminOrgHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organizations/"+orgID.String()+"/tenants", nil)
	req = setURLParam(req, "orgId", orgID.String())
	w := httptest.NewRecorder()
	h.ListOrgTenants(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string][]service.OrgTenantDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Len(t, body["tenants"], 1)
	assert.Equal(t, dept.Name, body["tenants"][0].Name)
}

func TestAdminOrgHandler_ListOrgTenants_InvalidID(t *testing.T) {
	h := newAdminOrgHandler(&mockOrgService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organizations/bad-id/tenants", nil)
	req = setURLParam(req, "orgId", "bad-id")
	w := httptest.NewRecorder()
	h.ListOrgTenants(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
