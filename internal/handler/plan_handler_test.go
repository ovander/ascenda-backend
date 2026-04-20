package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ascenda/internal/dto"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/backendkit/pagination"
	"ascenda/internal/service"
)

// ── mockPlanService ───────────────────────────────────────────────────────────

type mockPlanService struct {
	listFn                 func(ctx context.Context, tenantID uuid.UUID, params pagination.Params) ([]model.BusinessPlan, int64, error)
	createFn               func(ctx context.Context, tenantID, createdBy uuid.UUID, name, description, country string) (*model.BusinessPlan, error)
	getPlanFn              func(ctx context.Context, tenantID, planID uuid.UUID) (*model.BusinessPlan, error)
	updateFn               func(ctx context.Context, tenantID, planID uuid.UUID, name, description, status string) error
	deleteFn               func(ctx context.Context, tenantID, planID uuid.UUID) error
	transitionFn           func(ctx context.Context, tenantID, planID uuid.UUID, newStatus, callerRole string) error
	getPlanImpactFn        func(ctx context.Context, tenantID, planID uuid.UUID) (*service.PlanImpact, error)
	getScenarioImpactFn    func(ctx context.Context, tenantID, planID, scenarioID uuid.UUID) (*service.ScenarioImpact, error)
	listScenariosFn        func(ctx context.Context, tenantID, planID uuid.UUID) ([]model.Scenario, error)
}

func (m *mockPlanService) ListPlans(ctx context.Context, tenantID uuid.UUID, params pagination.Params) ([]model.BusinessPlan, int64, error) {
	if m.listFn != nil {
		return m.listFn(ctx, tenantID, params)
	}
	return nil, 0, apierror.Internal("list not implemented")
}

func (m *mockPlanService) CreatePlan(ctx context.Context, tenantID, createdBy uuid.UUID, name, description, country string) (*model.BusinessPlan, error) {
	if m.createFn != nil {
		return m.createFn(ctx, tenantID, createdBy, name, description, country)
	}
	return nil, apierror.Internal("create not implemented")
}

func (m *mockPlanService) GetPlan(ctx context.Context, tenantID, planID uuid.UUID) (*model.BusinessPlan, error) {
	if m.getPlanFn != nil {
		return m.getPlanFn(ctx, tenantID, planID)
	}
	return nil, apierror.Internal("get not implemented")
}

func (m *mockPlanService) UpdatePlan(ctx context.Context, tenantID, planID uuid.UUID, name, description, status string) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, tenantID, planID, name, description, status)
	}
	return apierror.Internal("update not implemented")
}

func (m *mockPlanService) DeletePlan(ctx context.Context, tenantID, planID uuid.UUID) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, tenantID, planID)
	}
	return apierror.Internal("delete not implemented")
}

func (m *mockPlanService) TransitionPlanStatus(ctx context.Context, tenantID, planID uuid.UUID, newStatus, callerRole string) error {
	if m.transitionFn != nil {
		return m.transitionFn(ctx, tenantID, planID, newStatus, callerRole)
	}
	return apierror.Internal("transition not implemented")
}

func (m *mockPlanService) GetPlanImpact(ctx context.Context, tenantID, planID uuid.UUID) (*service.PlanImpact, error) {
	if m.getPlanImpactFn != nil {
		return m.getPlanImpactFn(ctx, tenantID, planID)
	}
	return nil, apierror.Internal("get impact not implemented")
}

func (m *mockPlanService) GetScenarioImpact(ctx context.Context, tenantID, planID, scenarioID uuid.UUID) (*service.ScenarioImpact, error) {
	if m.getScenarioImpactFn != nil {
		return m.getScenarioImpactFn(ctx, tenantID, planID, scenarioID)
	}
	return nil, apierror.Internal("get scenario impact not implemented")
}

func (m *mockPlanService) ListScenarios(ctx context.Context, tenantID, planID uuid.UUID) ([]model.Scenario, error) {
	if m.listScenariosFn != nil {
		return m.listScenariosFn(ctx, tenantID, planID)
	}
	return nil, apierror.Internal("list scenarios not implemented")
}

// ── fixtures ──────────────────────────────────────────────────────────────────

func makePlan(tenantID uuid.UUID, name, description, status string) *model.BusinessPlan {
	p := &model.BusinessPlan{
		TenantScoped: model.TenantScoped{
			ID:       uuid.New(),
			TenantID: tenantID,
		},
		Name:        name,
		Description: description,
		Status:      status,
		CreatedBy:   uuid.New(),
		IsDemo:      false,
	}
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	return p
}

func newPlanHandler(svc service.PlanServicer) *PlanHandler {
	return NewPlanHandler(svc, nil, logrus.NewEntry(logrus.New()))
}

// ── List ───────────────────────────────────────────────────────────────────────

func TestPlanHandler_List_Success(t *testing.T) {
	tenantID := uuid.New()
	plan1 := makePlan(tenantID, "Plan 1", "Description 1", "draft")
	plan2 := makePlan(tenantID, "Plan 2", "Description 2", "review")

	svc := &mockPlanService{
		listFn: func(_ context.Context, tid uuid.UUID, params pagination.Params) ([]model.BusinessPlan, int64, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, 0, params.Offset)
			assert.Equal(t, 10, params.PerPage)
			return []model.BusinessPlan{*plan1, *plan2}, 2, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/?page=0&limit=10", nil)
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newPlanHandler(svc).List(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got dto.PagedResponse[dto.PlanResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, int64(2), got.Total)
}

func TestPlanHandler_List_MissingTenantContext(t *testing.T) {
	svc := &mockPlanService{}
	r := httptest.NewRequest(http.MethodGet, "/?page=0&limit=10", nil)
	// No tenant context
	w := httptest.NewRecorder()

	newPlanHandler(svc).List(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ── Create ─────────────────────────────────────────────────────────────────────

func TestPlanHandler_Create_Success(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	want := makePlan(tenantID, "My Plan", "A new plan", "draft")

	svc := &mockPlanService{
		createFn: func(_ context.Context, tid, uid uuid.UUID, name, desc, country string) (*model.BusinessPlan, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, userID, uid)
			assert.Equal(t, "My Plan", name)
			assert.Equal(t, "A new plan", desc)
			assert.Equal(t, "BE", country) // default country
			return want, nil
		},
	}

	body, _ := json.Marshal(CreatePlanRequest{Name: "My Plan", Description: "A new plan", Country: "BE"})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	ctx := ctxutil.WithTenantID(r.Context(), tenantID)
	ctx = ctxutil.WithUserID(ctx, userID)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	newPlanHandler(svc).Create(w, r)

	require.Equal(t, http.StatusCreated, w.Code)
	var got dto.PlanResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "My Plan", got.Name)
}

func TestPlanHandler_Create_MissingName(t *testing.T) {
	svc := &mockPlanService{}
	body, _ := json.Marshal(map[string]string{"description": "no name provided"})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPlanHandler(svc).Create(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestPlanHandler_Create_MissingTenantContext(t *testing.T) {
	svc := &mockPlanService{}
	body, _ := json.Marshal(CreatePlanRequest{Name: "Plan"})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	// No tenant context
	w := httptest.NewRecorder()

	newPlanHandler(svc).Create(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ── Get ────────────────────────────────────────────────────────────────────────

func TestPlanHandler_Get_Success(t *testing.T) {
	tenantID := uuid.New()
	plan := makePlan(tenantID, "Base Plan", "A base plan", "draft")

	svc := &mockPlanService{
		getPlanFn: func(_ context.Context, tid, planID uuid.UUID) (*model.BusinessPlan, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, plan.ID, planID)
			return plan, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"planId": plan.ID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newPlanHandler(svc).Get(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got dto.PlanResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "Base Plan", got.Name)
}

func TestPlanHandler_Get_BadPlanUUID(t *testing.T) {
	svc := &mockPlanService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"planId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPlanHandler(svc).Get(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPlanHandler_Get_NotFound(t *testing.T) {
	svc := &mockPlanService{
		getPlanFn: func(_ context.Context, _, _ uuid.UUID) (*model.BusinessPlan, error) {
			return nil, apierror.NotFound("plan", "missing")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"planId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPlanHandler(svc).Get(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── Update ─────────────────────────────────────────────────────────────────────

func TestPlanHandler_Update_Success(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()

	svc := &mockPlanService{
		updateFn: func(_ context.Context, tid, pid uuid.UUID, name, desc, status string) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, planID, pid)
			assert.Equal(t, "Updated Plan", name)
			assert.Equal(t, "Updated desc", desc)
			return nil
		},
	}

	body, _ := json.Marshal(UpdatePlanRequest{Name: "Updated Plan", Description: "Updated desc"})
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"planId": planID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newPlanHandler(svc).Update(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestPlanHandler_Update_BadPlanUUID(t *testing.T) {
	svc := &mockPlanService{}
	body, _ := json.Marshal(UpdatePlanRequest{Name: "X"})
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"planId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPlanHandler(svc).Update(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── Delete ─────────────────────────────────────────────────────────────────────

func TestPlanHandler_Delete_Success(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()
	called := false

	svc := &mockPlanService{
		deleteFn: func(_ context.Context, tid, pid uuid.UUID) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, planID, pid)
			called = true
			return nil
		},
	}

	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r = withChiParams(r, map[string]string{"planId": planID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newPlanHandler(svc).Delete(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, called)
}

func TestPlanHandler_Delete_BadPlanUUID(t *testing.T) {
	svc := &mockPlanService{}
	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r = withChiParams(r, map[string]string{"planId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newPlanHandler(svc).Delete(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── Lock ───────────────────────────────────────────────────────────────────────

func TestPlanHandler_Lock_Success(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()

	svc := &mockPlanService{
		transitionFn: func(_ context.Context, tid, pid uuid.UUID, newStatus, role string) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, planID, pid)
			assert.Equal(t, "approved", newStatus)
			return nil
		},
	}

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r = withChiParams(r, map[string]string{"planId": planID.String()})
	ctx := ctxutil.WithTenantID(r.Context(), tenantID)
	ctx = ctxutil.WithUserRole(ctx, "owner")
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	newPlanHandler(svc).Lock(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestPlanHandler_Lock_ServiceError(t *testing.T) {
	svc := &mockPlanService{
		transitionFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID, _, _ string) error {
			return apierror.Forbidden("only the plan owner can lock or archive a plan")
		},
	}

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r = withChiParams(r, map[string]string{"planId": uuid.New().String()})
	ctx := ctxutil.WithTenantID(r.Context(), uuid.New())
	ctx = ctxutil.WithUserRole(ctx, "user")
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	newPlanHandler(svc).Lock(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ── GetImpact ──────────────────────────────────────────────────────────────────

func TestPlanHandler_GetImpact_Success(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()
	impact := &service.PlanImpact{
		PlanID:        planID.String(),
		PlanName:      "Plan",
		PlanStatus:    "draft",
		ScenarioCount: 2,
		IsDemo:        false,
		CanDelete:     true,
	}

	svc := &mockPlanService{
		getPlanImpactFn: func(_ context.Context, tid, pid uuid.UUID) (*service.PlanImpact, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, planID, pid)
			return impact, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"planId": planID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newPlanHandler(svc).GetImpact(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got service.PlanImpact
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, planID.String(), got.PlanID)
	assert.Equal(t, 2, got.ScenarioCount)
	assert.True(t, got.CanDelete)
}
