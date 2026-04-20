package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ascenda/internal/dto"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
)

// ── mockScenarioService ───────────────────────────────────────────────────────

type mockScenarioService struct {
	listFn   func(ctx context.Context, tenantID, planID uuid.UUID) ([]model.Scenario, error)
	getFn    func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.Scenario, error)
	createFn func(ctx context.Context, tenantID, planID uuid.UUID, name, description string) (*model.Scenario, error)
	updateFn func(ctx context.Context, tenantID, scenarioID uuid.UUID, name, description string) error
	deleteFn func(ctx context.Context, tenantID, scenarioID uuid.UUID) error
	cloneFn  func(ctx context.Context, tenantID, scenarioID uuid.UUID, newName string) (*model.Scenario, error)
}

func (m *mockScenarioService) ListScenarios(ctx context.Context, tenantID, planID uuid.UUID) ([]model.Scenario, error) {
	if m.listFn != nil {
		return m.listFn(ctx, tenantID, planID)
	}
	return nil, apierror.Internal("list not implemented")
}

func (m *mockScenarioService) GetScenario(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.Scenario, error) {
	if m.getFn != nil {
		return m.getFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("get not implemented")
}

func (m *mockScenarioService) CreateScenario(ctx context.Context, tenantID, planID uuid.UUID, name, description string) (*model.Scenario, error) {
	if m.createFn != nil {
		return m.createFn(ctx, tenantID, planID, name, description)
	}
	return nil, apierror.Internal("create not implemented")
}

func (m *mockScenarioService) UpdateScenario(ctx context.Context, tenantID, scenarioID uuid.UUID, name, description string) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, tenantID, scenarioID, name, description)
	}
	return apierror.Internal("update not implemented")
}

func (m *mockScenarioService) DeleteScenario(ctx context.Context, tenantID, scenarioID uuid.UUID) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, tenantID, scenarioID)
	}
	return apierror.Internal("delete not implemented")
}

func (m *mockScenarioService) CloneScenario(ctx context.Context, tenantID, scenarioID uuid.UUID, newName string) (*model.Scenario, error) {
	if m.cloneFn != nil {
		return m.cloneFn(ctx, tenantID, scenarioID, newName)
	}
	return nil, apierror.Internal("clone not implemented")
}

// ── fixtures ──────────────────────────────────────────────────────────────────

func makeScenario(tenantID, planID uuid.UUID, name, description string) *model.Scenario {
	s := &model.Scenario{
		TenantScoped: model.TenantScoped{
			ID:       uuid.New(),
			TenantID: tenantID,
		},
		PlanID:      planID,
		Name:        name,
		Description: description,
		IsDefault:   false,
	}
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	return s
}

func newScenarioHandler(svc ScenarioServicer) *ScenarioHandler {
	return NewScenarioHandler(svc, logrus.NewEntry(logrus.New()))
}

// ── Create ────────────────────────────────────────────────────────────────────

func TestScenarioHandler_Create_Success(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()
	want := makeScenario(tenantID, planID, "Base Case", "Conservative assumptions")

	svc := &mockScenarioService{
		createFn: func(_ context.Context, tid, pid uuid.UUID, name, desc string) (*model.Scenario, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, planID, pid)
			assert.Equal(t, "Base Case", name)
			assert.Equal(t, "Conservative assumptions", desc)
			return want, nil
		},
	}

	body, _ := json.Marshal(CreateScenarioRequest{Name: "Base Case", Description: "Conservative assumptions"})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	ctx := ctxutil.WithTenantID(r.Context(), tenantID)
	r = r.WithContext(ctx)
	// Inject planId via chi URL param
	r = withChiParams(r, map[string]string{"planId": planID.String()})
	w := httptest.NewRecorder()

	newScenarioHandler(svc).Create(w, r)

	require.Equal(t, http.StatusCreated, w.Code)
	var got dto.ScenarioResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, want.Name, got.Name)
	assert.Equal(t, "Conservative assumptions", got.Description)
}

func TestScenarioHandler_Create_MissingName(t *testing.T) {
	svc := &mockScenarioService{}
	body, _ := json.Marshal(map[string]string{"description": "no name provided"})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"planId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newScenarioHandler(svc).Create(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestScenarioHandler_Create_EmptyDescriptionAllowed(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()
	want := makeScenario(tenantID, planID, "Optimistic", "")

	svc := &mockScenarioService{
		createFn: func(_ context.Context, _, _ uuid.UUID, _, desc string) (*model.Scenario, error) {
			assert.Equal(t, "", desc, "empty description should be passed through")
			return want, nil
		},
	}

	body, _ := json.Marshal(CreateScenarioRequest{Name: "Optimistic"})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"planId": planID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newScenarioHandler(svc).Create(w, r)

	require.Equal(t, http.StatusCreated, w.Code)
}

func TestScenarioHandler_Create_BadPlanUUID(t *testing.T) {
	svc := &mockScenarioService{}
	body, _ := json.Marshal(CreateScenarioRequest{Name: "X"})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"planId": "not-a-uuid"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newScenarioHandler(svc).Create(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestScenarioHandler_Create_ServiceError(t *testing.T) {
	svc := &mockScenarioService{
		createFn: func(_ context.Context, _, _ uuid.UUID, _, _ string) (*model.Scenario, error) {
			return nil, apierror.NotFound("plan", "missing")
		},
	}

	body, _ := json.Marshal(CreateScenarioRequest{Name: "X"})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"planId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newScenarioHandler(svc).Create(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── Update ────────────────────────────────────────────────────────────────────

func TestScenarioHandler_Update_NameAndDescription(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	var capturedName, capturedDesc string
	svc := &mockScenarioService{
		updateFn: func(_ context.Context, _, sid uuid.UUID, name, desc string) error {
			assert.Equal(t, scenarioID, sid)
			capturedName = name
			capturedDesc = desc
			return nil
		},
	}

	body, _ := json.Marshal(UpdateScenarioRequest{Name: "Revised Base", Description: "Updated assumptions"})
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newScenarioHandler(svc).Update(w, r)

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "Revised Base", capturedName)
	assert.Equal(t, "Updated assumptions", capturedDesc)
}

func TestScenarioHandler_Update_DescriptionOnlyPreservesName(t *testing.T) {
	svc := &mockScenarioService{
		updateFn: func(_ context.Context, _, _ uuid.UUID, name, desc string) error {
			// Service receives whatever the handler passes — empty name means "don't change"
			assert.Equal(t, "", name)
			assert.Equal(t, "New description only", desc)
			return nil
		},
	}

	body, _ := json.Marshal(UpdateScenarioRequest{Description: "New description only"})
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newScenarioHandler(svc).Update(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestScenarioHandler_Update_BadScenarioUUID(t *testing.T) {
	svc := &mockScenarioService{}
	body, _ := json.Marshal(UpdateScenarioRequest{Name: "X"})
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newScenarioHandler(svc).Update(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestScenarioHandler_Update_ServiceError(t *testing.T) {
	svc := &mockScenarioService{
		updateFn: func(_ context.Context, _, _ uuid.UUID, _, _ string) error {
			return apierror.NotFound("scenario", "gone")
		},
	}

	body, _ := json.Marshal(UpdateScenarioRequest{Name: "X"})
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newScenarioHandler(svc).Update(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── List ──────────────────────────────────────────────────────────────────────

func TestScenarioHandler_List_Success(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()
	sc1 := makeScenario(tenantID, planID, "Base", "")
	sc2 := makeScenario(tenantID, planID, "Optimistic", "Best case")

	svc := &mockScenarioService{
		listFn: func(_ context.Context, tid, pid uuid.UUID) ([]model.Scenario, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, planID, pid)
			return []model.Scenario{*sc1, *sc2}, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"planId": planID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newScenarioHandler(svc).List(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []dto.ScenarioResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 2)
	assert.Equal(t, "Base", got[0].Name)
	assert.Equal(t, "Optimistic", got[1].Name)
	assert.Equal(t, "Best case", got[1].Description)
}

func TestScenarioHandler_List_Empty(t *testing.T) {
	svc := &mockScenarioService{
		listFn: func(_ context.Context, _, _ uuid.UUID) ([]model.Scenario, error) {
			return []model.Scenario{}, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"planId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newScenarioHandler(svc).List(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []dto.ScenarioResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Empty(t, got)
}

func TestScenarioHandler_List_BadPlanUUID(t *testing.T) {
	svc := &mockScenarioService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"planId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newScenarioHandler(svc).List(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── Get ───────────────────────────────────────────────────────────────────────

func TestScenarioHandler_Get_Success(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()
	sc := makeScenario(tenantID, planID, "Base", "Base description")

	svc := &mockScenarioService{
		getFn: func(_ context.Context, _, sid uuid.UUID) (*model.Scenario, error) {
			assert.Equal(t, sc.ID, sid)
			return sc, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": sc.ID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newScenarioHandler(svc).Get(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got dto.ScenarioResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, sc.Name, got.Name)
	assert.Equal(t, "Base description", got.Description)
}

func TestScenarioHandler_Get_NotFound(t *testing.T) {
	svc := &mockScenarioService{
		getFn: func(_ context.Context, _, _ uuid.UUID) (*model.Scenario, error) {
			return nil, apierror.NotFound("scenario", "x")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newScenarioHandler(svc).Get(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── Delete ────────────────────────────────────────────────────────────────────

func TestScenarioHandler_Delete_Success(t *testing.T) {
	scenarioID := uuid.New()
	called := false
	svc := &mockScenarioService{
		deleteFn: func(_ context.Context, _, sid uuid.UUID) error {
			assert.Equal(t, scenarioID, sid)
			called = true
			return nil
		},
	}

	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newScenarioHandler(svc).Delete(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, called)
}

func TestScenarioHandler_Delete_NotFound(t *testing.T) {
	svc := &mockScenarioService{
		deleteFn: func(_ context.Context, _, _ uuid.UUID) error {
			return apierror.NotFound("scenario", "x")
		},
	}

	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newScenarioHandler(svc).Delete(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── Clone ─────────────────────────────────────────────────────────────────────

func TestScenarioHandler_Clone_Success(t *testing.T) {
	tenantID := uuid.New()
	srcID := uuid.New()
	cloned := makeScenario(tenantID, uuid.New(), "Base Copy", "")

	svc := &mockScenarioService{
		cloneFn: func(_ context.Context, _, sid uuid.UUID, newName string) (*model.Scenario, error) {
			assert.Equal(t, srcID, sid)
			assert.Equal(t, "Base Copy", newName)
			return cloned, nil
		},
	}

	body, _ := json.Marshal(CloneScenarioRequest{Name: "Base Copy"})
	r := httptest.NewRequest(http.MethodPost, "/clone", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": srcID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newScenarioHandler(svc).Clone(w, r)

	require.Equal(t, http.StatusCreated, w.Code)
	var got dto.ScenarioResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "Base Copy", got.Name)
}

func TestScenarioHandler_Clone_MissingName(t *testing.T) {
	svc := &mockScenarioService{}
	body, _ := json.Marshal(map[string]string{})
	r := httptest.NewRequest(http.MethodPost, "/clone", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newScenarioHandler(svc).Clone(w, r)

	// Name is required in CloneScenarioRequest — expect validation error
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// ── JSON response shape ───────────────────────────────────────────────────────

func TestScenarioHandler_Create_ResponseContainsDescriptionField(t *testing.T) {
	// Guard: the JSON response must always include the description field,
	// even when it is empty, so frontend destructuring never sees undefined.
	tenantID := uuid.New()
	sc := makeScenario(tenantID, uuid.New(), "X", "")

	svc := &mockScenarioService{
		createFn: func(_ context.Context, _, _ uuid.UUID, _, _ string) (*model.Scenario, error) {
			return sc, nil
		},
	}

	body, _ := json.Marshal(CreateScenarioRequest{Name: "X"})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"planId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newScenarioHandler(svc).Create(w, r)

	require.Equal(t, http.StatusCreated, w.Code)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	_, hasDesc := raw["description"]
	assert.True(t, hasDesc, "response JSON must contain description key")
}

// ── Path parsing fallback ─────────────────────────────────────────────────────

func TestScenarioHandler_ExtractPlanID_FallbackPathParsing(t *testing.T) {
	// When chi params are absent, extractPlanID falls back to URL path parsing.
	planID := uuid.New()

	r := httptest.NewRequest(http.MethodGet, "/api/v1/plans/"+planID.String()+"/scenarios", nil)
	// No chi params injected — relies on path fallback.
	extracted := extractPlanID(r)
	assert.Equal(t, planID.String(), extracted)
}

func TestScenarioHandler_ExtractPlanID_ChiParamPreferred(t *testing.T) {
	planID := uuid.New()
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("planId", planID.String())
	r := httptest.NewRequest(http.MethodGet, "/", nil).
		WithContext(context.WithValue(context.Background(), chi.RouteCtxKey, chiCtx))
	assert.Equal(t, planID.String(), extractPlanID(r))
}
