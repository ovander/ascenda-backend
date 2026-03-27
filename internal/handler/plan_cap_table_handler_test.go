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
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"ascenda/internal/model"
	"ascenda/internal/pkg/ctxutil"
)

// ─────────────────────────────────────────────────────────────────────────────
// Mock PlanShareholderRepository
// ─────────────────────────────────────────────────────────────────────────────

type mockPlanShareholderRepo struct {
	listByPlanFunc func(tenantID, planID uuid.UUID) ([]*model.PlanShareholder, error)
	getByIDFunc    func(tenantID, id uuid.UUID) (*model.PlanShareholder, error)
	createFunc     func(sh *model.PlanShareholder) error
	updateFunc     func(sh *model.PlanShareholder) error
	deleteFunc     func(tenantID, id uuid.UUID) error
}

func (m *mockPlanShareholderRepo) ListByPlan(tenantID, planID uuid.UUID) ([]*model.PlanShareholder, error) {
	if m.listByPlanFunc != nil {
		return m.listByPlanFunc(tenantID, planID)
	}
	return nil, nil
}

func (m *mockPlanShareholderRepo) GetByID(tenantID, id uuid.UUID) (*model.PlanShareholder, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(tenantID, id)
	}
	return nil, errors.New("not found")
}

func (m *mockPlanShareholderRepo) Create(sh *model.PlanShareholder) error {
	if m.createFunc != nil {
		return m.createFunc(sh)
	}
	return nil
}

func (m *mockPlanShareholderRepo) Update(sh *model.PlanShareholder) error {
	if m.updateFunc != nil {
		return m.updateFunc(sh)
	}
	return nil
}

func (m *mockPlanShareholderRepo) Delete(tenantID, id uuid.UUID) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(tenantID, id)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func newPlanCapTableHandler(repo *mockPlanShareholderRepo) *PlanCapTableHandler {
	return NewPlanCapTableHandler(repo, logrus.NewEntry(logrus.New()))
}

// planCapTableReq builds a request with tenantID and {planId} in chi context.
func planCapTableReq(method, path string, body interface{}, planID, tenantID uuid.UUID) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body) //nolint:errcheck
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	ctx := ctxutil.WithTenantID(req.Context(), tenantID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("planId", planID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

	return req.WithContext(ctx)
}

// planCapTableReqWithID also sets the {id} URL param (for shareholder update/delete).
func planCapTableReqWithID(method, path string, body interface{}, planID, tenantID, id uuid.UUID) *http.Request {
	req := planCapTableReq(method, path, body, planID, tenantID)
	rctx := chi.RouteContext(req.Context())
	rctx.URLParams.Add("id", id.String())
	return req
}

// ─────────────────────────────────────────────────────────────────────────────
// GetSummary
// ─────────────────────────────────────────────────────────────────────────────

func TestPlanCapTable_GetSummary_EmptyList(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()

	repo := &mockPlanShareholderRepo{
		listByPlanFunc: func(tid, pid uuid.UUID) ([]*model.PlanShareholder, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, planID, pid)
			return []*model.PlanShareholder{}, nil
		},
	}

	h := newPlanCapTableHandler(repo)
	req := planCapTableReq(http.MethodGet, "/cap-table", nil, planID, tenantID)
	w := httptest.NewRecorder()
	h.GetSummary(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result planCapTableSummary
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, planID.String(), result.PlanID)
	assert.Equal(t, int64(0), result.TotalShares)
	assert.True(t, result.TotalInvested.IsZero())
	assert.Empty(t, result.Shareholders)
}

func TestPlanCapTable_GetSummary_WithShareholders(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()

	sh1 := &model.PlanShareholder{
		Name:           "Alice",
		Type:           model.ShareholderFounder,
		Shares:         10000,
		OwnershipPct:   decimal.NewFromFloat(60.0),
		InvestedAmount: decimal.NewFromFloat(0),
	}
	sh2 := &model.PlanShareholder{
		Name:           "Venture Co",
		Type:           model.ShareholderInvestor,
		Shares:         6000,
		OwnershipPct:   decimal.NewFromFloat(40.0),
		InvestedAmount: decimal.NewFromFloat(500000),
	}

	repo := &mockPlanShareholderRepo{
		listByPlanFunc: func(tid, pid uuid.UUID) ([]*model.PlanShareholder, error) {
			return []*model.PlanShareholder{sh1, sh2}, nil
		},
	}

	h := newPlanCapTableHandler(repo)
	req := planCapTableReq(http.MethodGet, "/cap-table", nil, planID, tenantID)
	w := httptest.NewRecorder()
	h.GetSummary(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result planCapTableSummary
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, planID.String(), result.PlanID)
	assert.Equal(t, int64(16000), result.TotalShares)
	assert.Equal(t, "500000", result.TotalInvested.String())
	assert.Len(t, result.Shareholders, 2)
}

func TestPlanCapTable_GetSummary_RepoError(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()

	repo := &mockPlanShareholderRepo{
		listByPlanFunc: func(tid, pid uuid.UUID) ([]*model.PlanShareholder, error) {
			return nil, errors.New("db connection lost")
		},
	}

	h := newPlanCapTableHandler(repo)
	req := planCapTableReq(http.MethodGet, "/cap-table", nil, planID, tenantID)
	w := httptest.NewRecorder()
	h.GetSummary(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPlanCapTable_GetSummary_InvalidPlanID(t *testing.T) {
	h := newPlanCapTableHandler(&mockPlanShareholderRepo{})

	req := httptest.NewRequest(http.MethodGet, "/cap-table", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("planId", "not-a-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.GetSummary(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ─────────────────────────────────────────────────────────────────────────────
// CreateShareholder
// ─────────────────────────────────────────────────────────────────────────────

func TestPlanCapTable_CreateShareholder_OK(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()

	var captured *model.PlanShareholder
	repo := &mockPlanShareholderRepo{
		createFunc: func(sh *model.PlanShareholder) error {
			captured = sh
			sh.TenantScoped.ID = uuid.New() // simulate DB-generated ID
			return nil
		},
	}

	h := newPlanCapTableHandler(repo)
	body := map[string]interface{}{
		"name":           "Alice",
		"type":           "founder",
		"shares":         10000,
		"ownershipPct":   "60.00",
		"investedAmount": "0",
		"notes":          "Co-founder",
	}
	req := planCapTableReq(http.MethodPost, "/cap-table/shareholders", body, planID, tenantID)
	w := httptest.NewRecorder()
	h.CreateShareholder(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	require.NotNil(t, captured)
	assert.Equal(t, planID, captured.PlanID)
	assert.Equal(t, tenantID, captured.TenantID)
	assert.Equal(t, "Alice", captured.Name)
	assert.Equal(t, model.ShareholderFounder, captured.Type)
	assert.Equal(t, int64(10000), captured.Shares)
	assert.Equal(t, "Co-founder", captured.Notes)

	// Response body should include the created shareholder
	var result model.PlanShareholder
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "Alice", result.Name)
}

func TestPlanCapTable_CreateShareholder_InvalidBody(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()

	h := newPlanCapTableHandler(&mockPlanShareholderRepo{})
	req := planCapTableReq(http.MethodPost, "/cap-table/shareholders", nil, planID, tenantID)
	// Overwrite body with invalid JSON
	req.Body = http.NoBody
	w := httptest.NewRecorder()
	h.CreateShareholder(w, req)

	// EOF / missing body → bad request or internal depending on validator
	assert.True(t, w.Code >= 400 && w.Code < 500, "expected 4xx, got %d", w.Code)
}

func TestPlanCapTable_CreateShareholder_RepoError(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()

	repo := &mockPlanShareholderRepo{
		createFunc: func(sh *model.PlanShareholder) error {
			return errors.New("insert failed")
		},
	}

	h := newPlanCapTableHandler(repo)
	body := map[string]interface{}{
		"name":           "Bob",
		"type":           "investor",
		"shares":         5000,
		"ownershipPct":   "33.33",
		"investedAmount": "250000",
	}
	req := planCapTableReq(http.MethodPost, "/cap-table/shareholders", body, planID, tenantID)
	w := httptest.NewRecorder()
	h.CreateShareholder(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPlanCapTable_CreateShareholder_InvalidPlanID(t *testing.T) {
	h := newPlanCapTableHandler(&mockPlanShareholderRepo{})

	req := httptest.NewRequest(http.MethodPost, "/cap-table/shareholders", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("planId", "bad-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.CreateShareholder(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ─────────────────────────────────────────────────────────────────────────────
// UpdateShareholder
// ─────────────────────────────────────────────────────────────────────────────

func TestPlanCapTable_UpdateShareholder_OK(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()
	shareholderID := uuid.New()

	existing := &model.PlanShareholder{
		Name:           "Alice",
		Type:           model.ShareholderFounder,
		Shares:         10000,
		OwnershipPct:   decimal.NewFromFloat(60.0),
		InvestedAmount: decimal.NewFromFloat(0),
	}
	existing.TenantScoped.ID = shareholderID
	existing.TenantID = tenantID

	repo := &mockPlanShareholderRepo{
		getByIDFunc: func(tid, id uuid.UUID) (*model.PlanShareholder, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, shareholderID, id)
			return existing, nil
		},
		updateFunc: func(sh *model.PlanShareholder) error {
			assert.Equal(t, "Alice Updated", sh.Name)
			assert.Equal(t, int64(12000), sh.Shares)
			return nil
		},
	}

	h := newPlanCapTableHandler(repo)
	body := map[string]interface{}{
		"name":           "Alice Updated",
		"type":           "founder",
		"shares":         12000,
		"ownershipPct":   "60.00",
		"investedAmount": "0",
	}
	req := planCapTableReqWithID(http.MethodPut, "/cap-table/shareholders/"+shareholderID.String(), body, planID, tenantID, shareholderID)
	w := httptest.NewRecorder()
	h.UpdateShareholder(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestPlanCapTable_UpdateShareholder_NotFound(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()
	shareholderID := uuid.New()

	repo := &mockPlanShareholderRepo{
		getByIDFunc: func(tid, id uuid.UUID) (*model.PlanShareholder, error) {
			return nil, errors.New("not found")
		},
	}

	h := newPlanCapTableHandler(repo)
	body := map[string]interface{}{
		"name":           "Ghost",
		"type":           "investor",
		"shares":         1000,
		"ownershipPct":   "5.0",
		"investedAmount": "100000",
	}
	req := planCapTableReqWithID(http.MethodPut, "/cap-table/shareholders/"+shareholderID.String(), body, planID, tenantID, shareholderID)
	w := httptest.NewRecorder()
	h.UpdateShareholder(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPlanCapTable_UpdateShareholder_InvalidID(t *testing.T) {
	h := newPlanCapTableHandler(&mockPlanShareholderRepo{})

	req := httptest.NewRequest(http.MethodPut, "/cap-table/shareholders/bad-uuid", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "bad-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.UpdateShareholder(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPlanCapTable_UpdateShareholder_RepoSaveError(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()
	shareholderID := uuid.New()

	existing := &model.PlanShareholder{Name: "Alice"}
	existing.TenantScoped.ID = shareholderID
	existing.TenantID = tenantID

	repo := &mockPlanShareholderRepo{
		getByIDFunc: func(_, _ uuid.UUID) (*model.PlanShareholder, error) {
			return existing, nil
		},
		updateFunc: func(sh *model.PlanShareholder) error {
			return errors.New("constraint violation")
		},
	}

	h := newPlanCapTableHandler(repo)
	body := map[string]interface{}{
		"name":           "Alice",
		"type":           "founder",
		"shares":         10000,
		"ownershipPct":   "60.0",
		"investedAmount": "0",
	}
	req := planCapTableReqWithID(http.MethodPut, "/cap-table/shareholders/"+shareholderID.String(), body, planID, tenantID, shareholderID)
	w := httptest.NewRecorder()
	h.UpdateShareholder(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ─────────────────────────────────────────────────────────────────────────────
// DeleteShareholder
// ─────────────────────────────────────────────────────────────────────────────

func TestPlanCapTable_DeleteShareholder_OK(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()
	shareholderID := uuid.New()

	deleted := false
	repo := &mockPlanShareholderRepo{
		deleteFunc: func(tid, id uuid.UUID) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, shareholderID, id)
			deleted = true
			return nil
		},
	}

	h := newPlanCapTableHandler(repo)
	req := planCapTableReqWithID(http.MethodDelete, "/cap-table/shareholders/"+shareholderID.String(), nil, planID, tenantID, shareholderID)
	w := httptest.NewRecorder()
	h.DeleteShareholder(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, deleted)
}

func TestPlanCapTable_DeleteShareholder_InvalidID(t *testing.T) {
	h := newPlanCapTableHandler(&mockPlanShareholderRepo{})

	req := httptest.NewRequest(http.MethodDelete, "/cap-table/shareholders/bad", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "not-a-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.DeleteShareholder(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPlanCapTable_DeleteShareholder_RepoError(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()
	shareholderID := uuid.New()

	repo := &mockPlanShareholderRepo{
		deleteFunc: func(_, _ uuid.UUID) error {
			return errors.New("foreign key constraint")
		},
	}

	h := newPlanCapTableHandler(repo)
	req := planCapTableReqWithID(http.MethodDelete, "/cap-table/shareholders/"+shareholderID.String(), nil, planID, tenantID, shareholderID)
	w := httptest.NewRecorder()
	h.DeleteShareholder(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestPlanCapTable_DeleteShareholder_WrongTenant404 verifies that when the
// repo returns gorm.ErrRecordNotFound (RowsAffected == 0 — the shareholder ID
// either does not exist or belongs to a different tenant), the handler returns
// HTTP 404 instead of 500.
func TestPlanCapTable_DeleteShareholder_WrongTenant404(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()
	shareholderID := uuid.New()

	repo := &mockPlanShareholderRepo{
		deleteFunc: func(tid, id uuid.UUID) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, shareholderID, id)
			return gorm.ErrRecordNotFound // simulates wrong-tenant or missing row
		},
	}

	h := newPlanCapTableHandler(repo)
	req := planCapTableReqWithID(http.MethodDelete, "/cap-table/shareholders/"+shareholderID.String(), nil, planID, tenantID, shareholderID)
	w := httptest.NewRecorder()
	h.DeleteShareholder(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
