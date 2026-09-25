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

// ── mockOpexService ───────────────────────────────────────────────────────────

type mockOpexService struct {
	listEntriesFn   func(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.OpexManualEntry, error)
	updateEntriesFn func(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.OpexManualEntry) error
	getSummaryFn    func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.OpexSummary, error)
}

func (m *mockOpexService) ListEntries(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.OpexManualEntry, error) {
	if m.listEntriesFn != nil {
		return m.listEntriesFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("list entries not implemented")
}

func (m *mockOpexService) UpdateEntries(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.OpexManualEntry) error {
	if m.updateEntriesFn != nil {
		return m.updateEntriesFn(ctx, tenantID, scenarioID, entries)
	}
	return apierror.Internal("update entries not implemented")
}

func (m *mockOpexService) GetSummary(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.OpexSummary, error) {
	if m.getSummaryFn != nil {
		return m.getSummaryFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("get summary not implemented")
}

// ── fixtures ──────────────────────────────────────────────────────────────────

func newOpexHandler(svc *mockOpexService) *OpexHandler {
	return NewOpexHandler(svc, logrus.NewEntry(logrus.New()))
}

// ── ListEntries ───────────────────────────────────────────────────────────────

func TestOpexHandler_ListEntries_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	entries := []model.OpexManualEntry{
		{
			LineID:    model.OpexLineID("rent"),
			YearIndex: 1,
			Amount:    decimal.NewFromInt(10000),
		},
	}

	svc := &mockOpexService{
		listEntriesFn: func(_ context.Context, tid, sid uuid.UUID) ([]model.OpexManualEntry, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return entries, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newOpexHandler(svc).ListEntries(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 1)
}

func TestOpexHandler_ListEntries_BadScenarioUUID(t *testing.T) {
	svc := &mockOpexService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newOpexHandler(svc).ListEntries(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOpexHandler_ListEntries_ServiceError(t *testing.T) {
	svc := &mockOpexService{
		listEntriesFn: func(_ context.Context, _, _ uuid.UUID) ([]model.OpexManualEntry, error) {
			return nil, apierror.NotFound("scenario", "x")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newOpexHandler(svc).ListEntries(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── UpdateEntries ─────────────────────────────────────────────────────────────

func TestOpexHandler_UpdateEntries_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	entries := []model.OpexManualEntry{
		{
			LineID:    model.OpexLineID("utilities"),
			YearIndex: 1,
			Amount:    decimal.NewFromInt(5000),
		},
	}

	svc := &mockOpexService{
		updateEntriesFn: func(_ context.Context, tid, sid uuid.UUID, ee []model.OpexManualEntry) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			assert.Equal(t, 1, len(ee))
			assert.Equal(t, model.OpexLineID("utilities"), ee[0].LineID)
			return nil
		},
	}

	body, _ := json.Marshal(entries)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newOpexHandler(svc).UpdateEntries(w, r)

	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestOpexHandler_UpdateEntries_BadScenarioUUID(t *testing.T) {
	svc := &mockOpexService{}
	body, _ := json.Marshal([]model.OpexManualEntry{})
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newOpexHandler(svc).UpdateEntries(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOpexHandler_UpdateEntries_InvalidBody(t *testing.T) {
	svc := &mockOpexService{}
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader([]byte("invalid json")))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newOpexHandler(svc).UpdateEntries(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOpexHandler_UpdateEntries_ServiceError(t *testing.T) {
	svc := &mockOpexService{
		updateEntriesFn: func(_ context.Context, _, _ uuid.UUID, _ []model.OpexManualEntry) error {
			return apierror.NotFound("scenario", "x")
		},
	}

	entries := []model.OpexManualEntry{}
	body, _ := json.Marshal(entries)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newOpexHandler(svc).UpdateEntries(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── GetSummary ────────────────────────────────────────────────────────────────

func TestOpexHandler_GetSummary_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	summary := &model.OpexSummary{}

	svc := &mockOpexService{
		getSummaryFn: func(_ context.Context, tid, sid uuid.UUID) (*model.OpexSummary, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return summary, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newOpexHandler(svc).GetSummary(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

func TestOpexHandler_GetSummary_BadScenarioUUID(t *testing.T) {
	svc := &mockOpexService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newOpexHandler(svc).GetSummary(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOpexHandler_GetSummary_ServiceError(t *testing.T) {
	svc := &mockOpexService{
		getSummaryFn: func(_ context.Context, _, _ uuid.UUID) (*model.OpexSummary, error) {
			return nil, apierror.NotFound("scenario", "x")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newOpexHandler(svc).GetSummary(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
