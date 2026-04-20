package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/ctxutil"
)

// ─────────────────────────────────────────────────────────────────────────────
// Mock PlanMemberRepository
// ─────────────────────────────────────────────────────────────────────────────

type mockPlanMemberRepo struct {
	createFunc          func(m *model.PlanMember) error
	getByPlanAndUserFunc func(tenantID, planID, userID uuid.UUID) (*model.PlanMember, error)
	listByPlanFunc      func(tenantID, planID uuid.UUID) ([]*model.PlanMember, error)
	listByUserFunc      func(tenantID, userID uuid.UUID) ([]*model.PlanMember, error)
	updateFunc          func(m *model.PlanMember) error
	deleteFunc          func(tenantID, planID, userID uuid.UUID) error
}

func (m *mockPlanMemberRepo) Create(member *model.PlanMember) error {
	if m.createFunc != nil {
		return m.createFunc(member)
	}
	return nil
}

func (m *mockPlanMemberRepo) GetByPlanAndUser(tenantID, planID, userID uuid.UUID) (*model.PlanMember, error) {
	if m.getByPlanAndUserFunc != nil {
		return m.getByPlanAndUserFunc(tenantID, planID, userID)
	}
	return nil, errors.New("not found")
}

func (m *mockPlanMemberRepo) ListByPlan(tenantID, planID uuid.UUID) ([]*model.PlanMember, error) {
	if m.listByPlanFunc != nil {
		return m.listByPlanFunc(tenantID, planID)
	}
	return nil, nil
}

func (m *mockPlanMemberRepo) ListByUser(tenantID, userID uuid.UUID) ([]*model.PlanMember, error) {
	if m.listByUserFunc != nil {
		return m.listByUserFunc(tenantID, userID)
	}
	return nil, nil
}

func (m *mockPlanMemberRepo) Update(member *model.PlanMember) error {
	if m.updateFunc != nil {
		return m.updateFunc(member)
	}
	return nil
}

func (m *mockPlanMemberRepo) Delete(tenantID, planID, userID uuid.UUID) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(tenantID, planID, userID)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func newPlanMemberHandler(repo *mockPlanMemberRepo) *PlanMemberHandler {
	return NewPlanMemberHandler(repo, logrus.NewEntry(logrus.New()))
}

func planMemberReq(method, path string, body interface{}, planID, tenantID uuid.UUID) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body) //nolint:errcheck
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	ctx := ctxutil.WithTenantID(req.Context(), tenantID)
	// Set a non-freemium plan so the handler's plan-sharing guard doesn't block.
	ctx = ctxutil.WithUserPlan(ctx, "pro")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("planId", planID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

	return req.WithContext(ctx)
}

func planMemberReqWithUser(method, path string, body interface{}, planID, tenantID, userID uuid.UUID) *http.Request {
	req := planMemberReq(method, path, body, planID, tenantID)
	rctx := chi.RouteContext(req.Context())
	rctx.URLParams.Add("userId", userID.String())
	return req
}

// ─────────────────────────────────────────────────────────────────────────────
// List
// ─────────────────────────────────────────────────────────────────────────────

func TestPlanMember_List_OK(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()
	userID := uuid.New()

	member := &model.PlanMember{
		ID:       uuid.New(),
		TenantID: tenantID,
		PlanID:   planID,
		UserID:   userID,
		Role:     "editor",
	}

	repo := &mockPlanMemberRepo{
		listByPlanFunc: func(tid, pid uuid.UUID) ([]*model.PlanMember, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, planID, pid)
			return []*model.PlanMember{member}, nil
		},
	}

	h := newPlanMemberHandler(repo)
	req := planMemberReq(http.MethodGet, "/members", nil, planID, tenantID)
	w := httptest.NewRecorder()
	h.List(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result []PlanMemberDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.Len(t, result, 1)
	assert.Equal(t, userID.String(), result[0].UserID)
	assert.Equal(t, "editor", result[0].Role)
}

// ─────────────────────────────────────────────────────────────────────────────
// Grant
// ─────────────────────────────────────────────────────────────────────────────

func TestPlanMember_Grant_OK(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()
	userID := uuid.New()

	var created *model.PlanMember
	repo := &mockPlanMemberRepo{
		getByPlanAndUserFunc: func(_, _, _ uuid.UUID) (*model.PlanMember, error) {
			return nil, errors.New("not found") // not yet a member
		},
		createFunc: func(m *model.PlanMember) error {
			created = m
			return nil
		},
	}

	h := newPlanMemberHandler(repo)
	body := map[string]interface{}{
		"userId": userID.String(),
		"role":   "editor",
	}
	req := planMemberReq(http.MethodPost, "/members", body, planID, tenantID)
	w := httptest.NewRecorder()
	h.Grant(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	require.NotNil(t, created)
	assert.Equal(t, userID, created.UserID)
	assert.Equal(t, "editor", created.Role)
}

func TestPlanMember_Grant_InvalidRole(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()

	h := newPlanMemberHandler(&mockPlanMemberRepo{})
	body := map[string]interface{}{
		"userId": uuid.New().String(),
		"role":   "superadmin", // invalid
	}
	req := planMemberReq(http.MethodPost, "/members", body, planID, tenantID)
	w := httptest.NewRecorder()
	h.Grant(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPlanMember_Grant_FreemiumBlocked(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()

	h := newPlanMemberHandler(&mockPlanMemberRepo{})
	body := map[string]interface{}{
		"userId": uuid.New().String(),
		"role":   "editor",
	}
	// Override the default "pro" plan set by planMemberReq with freemium.
	req := planMemberReq(http.MethodPost, "/members", body, planID, tenantID)
	req = req.WithContext(ctxutil.WithUserPlan(req.Context(), "freemium"))
	w := httptest.NewRecorder()
	h.Grant(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestPlanMember_Grant_AlreadyMember(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()
	userID := uuid.New()

	existing := &model.PlanMember{UserID: userID, Role: "viewer"}
	repo := &mockPlanMemberRepo{
		getByPlanAndUserFunc: func(_, _, _ uuid.UUID) (*model.PlanMember, error) {
			return existing, nil
		},
	}

	h := newPlanMemberHandler(repo)
	body := map[string]interface{}{
		"userId": userID.String(),
		"role":   "editor",
	}
	req := planMemberReq(http.MethodPost, "/members", body, planID, tenantID)
	w := httptest.NewRecorder()
	h.Grant(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

// ─────────────────────────────────────────────────────────────────────────────
// Revoke — the key tests for P1-3
// ─────────────────────────────────────────────────────────────────────────────

func TestPlanMember_Revoke_OK(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()
	userID := uuid.New()

	deleted := false
	repo := &mockPlanMemberRepo{
		deleteFunc: func(tid, pid, uid uuid.UUID) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, planID, pid)
			assert.Equal(t, userID, uid)
			deleted = true
			return nil
		},
	}

	h := newPlanMemberHandler(repo)
	req := planMemberReqWithUser(http.MethodDelete, "/members/"+userID.String(), nil, planID, tenantID, userID)
	w := httptest.NewRecorder()
	h.Revoke(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, deleted)
}

// TestPlanMember_Revoke_WrongTenant404 verifies that when the repo returns
// gorm.ErrRecordNotFound (RowsAffected == 0 — the (planID, userID) pair does
// not exist for this tenant), the handler emits HTTP 404 rather than 500.
func TestPlanMember_Revoke_WrongTenant404(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()
	userID := uuid.New()

	repo := &mockPlanMemberRepo{
		deleteFunc: func(_, _, _ uuid.UUID) error {
			return gorm.ErrRecordNotFound
		},
	}

	h := newPlanMemberHandler(repo)
	req := planMemberReqWithUser(http.MethodDelete, "/members/"+userID.String(), nil, planID, tenantID, userID)
	w := httptest.NewRecorder()
	h.Revoke(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPlanMember_Revoke_DBError(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()
	userID := uuid.New()

	repo := &mockPlanMemberRepo{
		deleteFunc: func(_, _, _ uuid.UUID) error {
			return errors.New("db timeout")
		},
	}

	h := newPlanMemberHandler(repo)
	req := planMemberReqWithUser(http.MethodDelete, "/members/"+userID.String(), nil, planID, tenantID, userID)
	w := httptest.NewRecorder()
	h.Revoke(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
