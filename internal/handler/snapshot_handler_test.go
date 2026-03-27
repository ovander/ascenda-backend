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
	"ascenda/internal/model"
	"ascenda/internal/pkg/apierror"
	"ascenda/internal/pkg/ctxutil"
	"ascenda/internal/pkg/pagination"
	"ascenda/internal/service"
)

// NOTE: SnapshotHandler uses a concrete *service.SnapshotService type.
// SnapshotService does not have a corresponding interface, so unit testing with mocks
// is not possible. These tests focus on input validation and UUID parsing, which return
// early errors before touching the service.

// ── mockSnapshotService ────────────────────────────────────────────────────────
// A minimal mock for testing paths that do call the service (after param validation).
// This is a best-effort mock that may be incomplete; integration tests are preferred.

type mockSnapshotService struct {
	listFn         func(ctx context.Context, tenantID, scenarioID uuid.UUID, params pagination.Params) ([]model.PlanSnapshot, int64, error)
	createFn       func(ctx context.Context, tenantID, scenarioID uuid.UUID, label, description string) (*model.PlanSnapshot, error)
	getFn          func(ctx context.Context, tenantID, snapshotID uuid.UUID) (*model.PlanSnapshot, error)
	getDataFn      func(ctx context.Context, tenantID, snapshotID uuid.UUID) (json.RawMessage, error)
	restoreFn      func(ctx context.Context, tenantID, snapshotID uuid.UUID) error
	cloneFn        func(ctx context.Context, tenantID, snapshotID, newScenarioID uuid.UUID) error
	diffFn         func(ctx context.Context, tenantID, snapshot1ID, snapshot2ID uuid.UUID) (map[string]interface{}, error)
	deleteFn       func(ctx context.Context, tenantID, snapshotID uuid.UUID) error
}

func (m *mockSnapshotService) List(ctx context.Context, tenantID, scenarioID uuid.UUID, params pagination.Params) ([]model.PlanSnapshot, int64, error) {
	if m.listFn != nil {
		return m.listFn(ctx, tenantID, scenarioID, params)
	}
	return nil, 0, apierror.Internal("list not implemented")
}

func (m *mockSnapshotService) Create(ctx context.Context, tenantID, scenarioID uuid.UUID, label, description string) (*model.PlanSnapshot, error) {
	if m.createFn != nil {
		return m.createFn(ctx, tenantID, scenarioID, label, description)
	}
	return nil, apierror.Internal("create not implemented")
}

func (m *mockSnapshotService) Get(ctx context.Context, tenantID, snapshotID uuid.UUID) (*model.PlanSnapshot, error) {
	if m.getFn != nil {
		return m.getFn(ctx, tenantID, snapshotID)
	}
	return nil, apierror.Internal("get not implemented")
}

func (m *mockSnapshotService) GetData(ctx context.Context, tenantID, snapshotID uuid.UUID) (json.RawMessage, error) {
	if m.getDataFn != nil {
		return m.getDataFn(ctx, tenantID, snapshotID)
	}
	return nil, apierror.Internal("get data not implemented")
}

func (m *mockSnapshotService) Restore(ctx context.Context, tenantID, snapshotID uuid.UUID) error {
	if m.restoreFn != nil {
		return m.restoreFn(ctx, tenantID, snapshotID)
	}
	return apierror.Internal("restore not implemented")
}

func (m *mockSnapshotService) CloneToScenario(ctx context.Context, tenantID, snapshotID, newScenarioID uuid.UUID) error {
	if m.cloneFn != nil {
		return m.cloneFn(ctx, tenantID, snapshotID, newScenarioID)
	}
	return apierror.Internal("clone not implemented")
}

func (m *mockSnapshotService) Diff(ctx context.Context, tenantID, snapshot1ID, snapshot2ID uuid.UUID) (map[string]interface{}, error) {
	if m.diffFn != nil {
		return m.diffFn(ctx, tenantID, snapshot1ID, snapshot2ID)
	}
	return nil, apierror.Internal("diff not implemented")
}

func (m *mockSnapshotService) Delete(ctx context.Context, tenantID, snapshotID uuid.UUID) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, tenantID, snapshotID)
	}
	return apierror.Internal("delete not implemented")
}

// ── fixtures ───────────────────────────────────────────────────────────────────

func makeSnapshot(tenantID, scenarioID uuid.UUID, label, description string) *model.PlanSnapshot {
	s := &model.PlanSnapshot{
		TenantScoped: model.TenantScoped{
			ID:       uuid.New(),
			TenantID: tenantID,
		},
		ScenarioID:  scenarioID,
		Version:     1,
		Label:       label,
		Description: description,
		Data:        json.RawMessage(`{}`),
		CreatedBy:   uuid.New(),
	}
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	return s
}

func newSnapshotHandler(svc *mockSnapshotService) *SnapshotHandler {
	return NewSnapshotHandler((*service.SnapshotService)(nil), logrus.NewEntry(logrus.New()))
}

// ── List ───────────────────────────────────────────────────────────────────────

func TestSnapshotHandler_List_BadScenarioUUID(t *testing.T) {
	svc := &mockSnapshotService{}
	r := httptest.NewRequest(http.MethodGet, "/?page=0&limit=10", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newSnapshotHandler(svc).List(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── Create ─────────────────────────────────────────────────────────────────────

func TestSnapshotHandler_Create_MissingLabel(t *testing.T) {
	svc := &mockSnapshotService{}
	body, _ := json.Marshal(map[string]string{"description": "no label provided"})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newSnapshotHandler(svc).Create(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestSnapshotHandler_Create_BadScenarioUUID(t *testing.T) {
	svc := &mockSnapshotService{}
	body, _ := json.Marshal(CreateSnapshotRequest{Label: "V1"})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newSnapshotHandler(svc).Create(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── Get ────────────────────────────────────────────────────────────────────────

func TestSnapshotHandler_Get_BadSnapshotUUID(t *testing.T) {
	svc := &mockSnapshotService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"snapshotId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newSnapshotHandler(svc).Get(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── GetData ────────────────────────────────────────────────────────────────────

func TestSnapshotHandler_GetData_BadSnapshotUUID(t *testing.T) {
	svc := &mockSnapshotService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"snapshotId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newSnapshotHandler(svc).GetData(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── Restore ────────────────────────────────────────────────────────────────────

func TestSnapshotHandler_Restore_BadSnapshotUUID(t *testing.T) {
	svc := &mockSnapshotService{}
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r = withChiParams(r, map[string]string{"snapshotId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newSnapshotHandler(svc).Restore(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── Clone ──────────────────────────────────────────────────────────────────────

func TestSnapshotHandler_Clone_MissingNewScenarioID(t *testing.T) {
	svc := &mockSnapshotService{}
	body, _ := json.Marshal(map[string]string{})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"snapshotId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newSnapshotHandler(svc).Clone(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestSnapshotHandler_Clone_BadSnapshotUUID(t *testing.T) {
	svc := &mockSnapshotService{}
	body, _ := json.Marshal(CloneSnapshotRequest{NewScenarioID: uuid.New().String()})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"snapshotId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newSnapshotHandler(svc).Clone(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSnapshotHandler_Clone_BadNewScenarioID(t *testing.T) {
	svc := &mockSnapshotService{}
	body, _ := json.Marshal(CloneSnapshotRequest{NewScenarioID: "bad"})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"snapshotId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newSnapshotHandler(svc).Clone(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── Diff ───────────────────────────────────────────────────────────────────────

func TestSnapshotHandler_Diff_BadSnapshot1UUID(t *testing.T) {
	svc := &mockSnapshotService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"snapshot1Id": "bad", "snapshot2Id": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newSnapshotHandler(svc).Diff(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSnapshotHandler_Diff_BadSnapshot2UUID(t *testing.T) {
	svc := &mockSnapshotService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"snapshot1Id": uuid.New().String(), "snapshot2Id": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newSnapshotHandler(svc).Diff(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── Delete ─────────────────────────────────────────────────────────────────────

func TestSnapshotHandler_Delete_BadSnapshotUUID(t *testing.T) {
	svc := &mockSnapshotService{}
	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r = withChiParams(r, map[string]string{"snapshotId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newSnapshotHandler(svc).Delete(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
