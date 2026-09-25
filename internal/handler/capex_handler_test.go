package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── mockCapexService ──────────────────────────────────────────────────────────

type mockCapexService struct {
	listEntriesFn   func(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CapexEntry, error)
	updateEntriesFn func(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.CapexEntry) error
	getSummaryFn    func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.CapexSummary, error)
}

func (m *mockCapexService) ListEntries(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CapexEntry, error) {
	if m.listEntriesFn != nil {
		return m.listEntriesFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("list entries not implemented")
}

func (m *mockCapexService) UpdateEntries(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.CapexEntry) error {
	if m.updateEntriesFn != nil {
		return m.updateEntriesFn(ctx, tenantID, scenarioID, entries)
	}
	return apierror.Internal("update entries not implemented")
}

func (m *mockCapexService) GetSummary(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.CapexSummary, error) {
	if m.getSummaryFn != nil {
		return m.getSummaryFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("get summary not implemented")
}

// ── fixtures ──────────────────────────────────────────────────────────────────

func newCapexHandler(svc *mockCapexService) *CapexHandler {
	return NewCapexHandler(svc, logrus.NewEntry(logrus.New()))
}

// ── ListEntries ───────────────────────────────────────────────────────────────

func TestCapexHandler_ListEntries_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	entries := []model.CapexEntry{
		{
			Category: model.AssetCategory("Equipment"),
			Amount:   decimal.NewFromInt(50000),
		},
	}

	svc := &mockCapexService{
		listEntriesFn: func(_ context.Context, tid, sid uuid.UUID) ([]model.CapexEntry, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return entries, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newCapexHandler(svc).ListEntries(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 1)
}

func TestCapexHandler_ListEntries_BadScenarioUUID(t *testing.T) {
	svc := &mockCapexService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newCapexHandler(svc).ListEntries(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCapexHandler_ListEntries_ServiceError(t *testing.T) {
	svc := &mockCapexService{
		listEntriesFn: func(_ context.Context, _, _ uuid.UUID) ([]model.CapexEntry, error) {
			return nil, apierror.NotFound("scenario", "x")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newCapexHandler(svc).ListEntries(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── UpdateEntries ─────────────────────────────────────────────────────────────

func TestCapexHandler_UpdateEntries_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	entries := []model.CapexEntry{
		{
			Category: model.AssetCategory("Equipment"),
			Amount:   decimal.NewFromInt(75000),
		},
	}

	svc := &mockCapexService{
		updateEntriesFn: func(_ context.Context, tid, sid uuid.UUID, ee []model.CapexEntry) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			assert.Equal(t, 1, len(ee))
			assert.Equal(t, model.AssetCategory("Equipment"), ee[0].Category)
			return nil
		},
	}

	body, _ := json.Marshal(entries)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newCapexHandler(svc).UpdateEntries(w, r)

	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestCapexHandler_UpdateEntries_BadScenarioUUID(t *testing.T) {
	svc := &mockCapexService{}
	body, _ := json.Marshal([]model.CapexEntry{})
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newCapexHandler(svc).UpdateEntries(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCapexHandler_UpdateEntries_InvalidBody(t *testing.T) {
	svc := &mockCapexService{}
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader([]byte("invalid json")))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newCapexHandler(svc).UpdateEntries(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCapexHandler_UpdateEntries_ServiceError(t *testing.T) {
	svc := &mockCapexService{
		updateEntriesFn: func(_ context.Context, _, _ uuid.UUID, _ []model.CapexEntry) error {
			return apierror.NotFound("scenario", "x")
		},
	}

	entries := []model.CapexEntry{}
	body, _ := json.Marshal(entries)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newCapexHandler(svc).UpdateEntries(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── GetSummary ────────────────────────────────────────────────────────────────

func TestCapexHandler_GetSummary_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	summary := &model.CapexSummary{}

	svc := &mockCapexService{
		getSummaryFn: func(_ context.Context, tid, sid uuid.UUID) (*model.CapexSummary, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return summary, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newCapexHandler(svc).GetSummary(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

func TestCapexHandler_GetSummary_BadScenarioUUID(t *testing.T) {
	svc := &mockCapexService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newCapexHandler(svc).GetSummary(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCapexHandler_GetSummary_ServiceError(t *testing.T) {
	svc := &mockCapexService{
		getSummaryFn: func(_ context.Context, _, _ uuid.UUID) (*model.CapexSummary, error) {
			return nil, apierror.NotFound("scenario", "x")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newCapexHandler(svc).GetSummary(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
