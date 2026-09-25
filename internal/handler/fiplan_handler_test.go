package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ascenda/internal/dto"
	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── mockFiplanService ─────────────────────────────────────────────────────

type mockFiplanService struct {
	listEntriesFn                func(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.FiplanEntry, error)
	updateEntriesFn              func(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.FiplanEntry) error
	getReportFn                  func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.FiplanReport, error)
	getGrantsForPnLFn            func(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.FiplanEntry, error)
	getCapitalIncreaseEntryFn    func(ctx context.Context, tenantID, scenarioID uuid.UUID, yearIndex int) (*model.FiplanEntry, error)
	upsertCapitalIncreaseEntryFn func(ctx context.Context, tenantID, scenarioID uuid.UUID, yearIndex int, amount decimal.Decimal, roundID uuid.UUID, roundLabel string) error
}

func (m *mockFiplanService) ListEntries(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.FiplanEntry, error) {
	if m.listEntriesFn != nil {
		return m.listEntriesFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("list entries not implemented")
}

func (m *mockFiplanService) UpdateEntries(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.FiplanEntry) error {
	if m.updateEntriesFn != nil {
		return m.updateEntriesFn(ctx, tenantID, scenarioID, entries)
	}
	return apierror.Internal("update entries not implemented")
}

func (m *mockFiplanService) GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.FiplanReport, error) {
	if m.getReportFn != nil {
		return m.getReportFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("get report not implemented")
}

func (m *mockFiplanService) GetGrantsForPnL(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.FiplanEntry, error) {
	if m.getGrantsForPnLFn != nil {
		return m.getGrantsForPnLFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("get grants for pnl not implemented")
}

func (m *mockFiplanService) GetCapitalIncreaseEntry(ctx context.Context, tenantID, scenarioID uuid.UUID, yearIndex int) (*model.FiplanEntry, error) {
	if m.getCapitalIncreaseEntryFn != nil {
		return m.getCapitalIncreaseEntryFn(ctx, tenantID, scenarioID, yearIndex)
	}
	return nil, apierror.Internal("get capital increase entry not implemented")
}

func (m *mockFiplanService) UpsertCapitalIncreaseEntry(ctx context.Context, tenantID, scenarioID uuid.UUID, yearIndex int, amount decimal.Decimal, roundID uuid.UUID, roundLabel string) error {
	if m.upsertCapitalIncreaseEntryFn != nil {
		return m.upsertCapitalIncreaseEntryFn(ctx, tenantID, scenarioID, yearIndex, amount, roundID, roundLabel)
	}
	return apierror.Internal("upsert capital increase entry not implemented")
}

// ── mockCapTableRoundLinker ────────────────────────────────────────────────

type mockCapTableRoundLinker struct {
	createRoundFn func(ctx context.Context, rnd *model.CapTableRound) error
}

func (m *mockCapTableRoundLinker) CreateRound(ctx context.Context, rnd *model.CapTableRound) error {
	if m.createRoundFn != nil {
		return m.createRoundFn(ctx, rnd)
	}
	return apierror.Internal("create round not implemented")
}

// ── Helper ────────────────────────────────────────────────────────────────

func newFiplanHandler(svc *mockFiplanService, roundLinker capTableRoundLinker) *FiplanHandler {
	return NewFiplanHandler(svc, roundLinker, logrus.NewEntry(logrus.New()))
}

// ── ListEntries ───────────────────────────────────────────────────────────

func TestFiplanHandler_ListEntries_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	entry1 := model.FiplanEntry{
		LineID:    model.FiplanCapitalIncrease,
		YearIndex: 1,
		Amount:    decimal.NewFromInt(100000),
	}
	entry2 := model.FiplanEntry{
		LineID:    model.FiplanLTLoans,
		YearIndex: 2,
		Amount:    decimal.NewFromInt(50000),
	}

	svc := &mockFiplanService{
		listEntriesFn: func(_ context.Context, tid, sid uuid.UUID) ([]model.FiplanEntry, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return []model.FiplanEntry{entry1, entry2}, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newFiplanHandler(svc, nil).ListEntries(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []dto.FiplanEntryResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 2)
}

func TestFiplanHandler_ListEntries_BadScenarioUUID(t *testing.T) {
	svc := &mockFiplanService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "not-a-uuid"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newFiplanHandler(svc, nil).ListEntries(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFiplanHandler_ListEntries_ServiceError(t *testing.T) {
	svc := &mockFiplanService{
		listEntriesFn: func(_ context.Context, _, _ uuid.UUID) ([]model.FiplanEntry, error) {
			return nil, apierror.NotFound("scenario", "missing")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newFiplanHandler(svc, nil).ListEntries(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── UpdateEntries ─────────────────────────────────────────────────────────

func TestFiplanHandler_UpdateEntries_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	var capturedEntries []model.FiplanEntry
	svc := &mockFiplanService{
		updateEntriesFn: func(_ context.Context, tid, sid uuid.UUID, entries []model.FiplanEntry) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			capturedEntries = entries
			return nil
		},
	}

	entries := []model.FiplanEntry{
		{LineID: model.FiplanCapitalIncrease, YearIndex: 1, Amount: decimal.NewFromInt(100000)},
	}
	body, _ := json.Marshal(entries)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newFiplanHandler(svc, nil).UpdateEntries(w, r)

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Len(t, capturedEntries, 1)
}

func TestFiplanHandler_UpdateEntries_BadScenarioUUID(t *testing.T) {
	svc := &mockFiplanService{}
	entries := []model.FiplanEntry{{LineID: model.FiplanCapitalIncrease, YearIndex: 1, Amount: decimal.NewFromInt(100000)}}
	body, _ := json.Marshal(entries)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newFiplanHandler(svc, nil).UpdateEntries(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFiplanHandler_UpdateEntries_InvalidBody(t *testing.T) {
	svc := &mockFiplanService{}
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader([]byte("invalid json")))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newFiplanHandler(svc, nil).UpdateEntries(w, r)

	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusUnprocessableEntity)
}

func TestFiplanHandler_UpdateEntries_ServiceError(t *testing.T) {
	svc := &mockFiplanService{
		updateEntriesFn: func(_ context.Context, _, _ uuid.UUID, _ []model.FiplanEntry) error {
			return apierror.Internal("database error")
		},
	}

	entries := []model.FiplanEntry{{LineID: model.FiplanCapitalIncrease, YearIndex: 1, Amount: decimal.NewFromInt(100000)}}
	body, _ := json.Marshal(entries)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newFiplanHandler(svc, nil).UpdateEntries(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ── GetReport ─────────────────────────────────────────────────────────────

func TestFiplanHandler_GetReport_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	expectedReport := &model.FiplanReport{}

	svc := &mockFiplanService{
		getReportFn: func(_ context.Context, tid, sid uuid.UUID) (*model.FiplanReport, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return expectedReport, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newFiplanHandler(svc, nil).GetReport(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

func TestFiplanHandler_GetReport_BadScenarioUUID(t *testing.T) {
	svc := &mockFiplanService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newFiplanHandler(svc, nil).GetReport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFiplanHandler_GetReport_ServiceError(t *testing.T) {
	svc := &mockFiplanService{
		getReportFn: func(_ context.Context, _, _ uuid.UUID) (*model.FiplanReport, error) {
			return nil, apierror.NotFound("scenario", "missing")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newFiplanHandler(svc, nil).GetReport(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── GetGrantsForPnL ───────────────────────────────────────────────────────

func TestFiplanHandler_GetGrantsForPnL_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	grants := []model.FiplanEntry{{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID}}

	svc := &mockFiplanService{
		getGrantsForPnLFn: func(_ context.Context, tid, sid uuid.UUID) ([]model.FiplanEntry, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return grants, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newFiplanHandler(svc, nil).GetGrantsForPnL(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got, 1)
}

func TestFiplanHandler_GetGrantsForPnL_BadScenarioUUID(t *testing.T) {
	svc := &mockFiplanService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newFiplanHandler(svc, nil).GetGrantsForPnL(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFiplanHandler_GetGrantsForPnL_ServiceError(t *testing.T) {
	svc := &mockFiplanService{
		getGrantsForPnLFn: func(_ context.Context, _, _ uuid.UUID) ([]model.FiplanEntry, error) {
			return nil, apierror.Internal("calculation error")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newFiplanHandler(svc, nil).GetGrantsForPnL(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ── CreateRoundFromCapitalIncrease ────────────────────────────────────────

func TestFiplanHandler_CreateRoundFromCapitalIncrease_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	roundID := uuid.New()

	fiplanEntry := &model.FiplanEntry{
		Amount: decimal.NewFromInt(1000000), // 1M euro
	}

	var capturedRound *model.CapTableRound
	svc := &mockFiplanService{
		getCapitalIncreaseEntryFn: func(_ context.Context, tid, sid uuid.UUID, yi int) (*model.FiplanEntry, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			assert.Equal(t, 0, yi)
			return fiplanEntry, nil
		},
		upsertCapitalIncreaseEntryFn: func(_ context.Context, tid, sid uuid.UUID, yi int, amt decimal.Decimal, rid uuid.UUID, rl string) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			assert.Equal(t, 0, yi)
			return nil
		},
	}

	linker := &mockCapTableRoundLinker{
		createRoundFn: func(_ context.Context, rnd *model.CapTableRound) error {
			rnd.ID = roundID
			capturedRound = rnd
			return nil
		},
	}

	body, _ := json.Marshal(map[string]interface{}{
		"label":          "Series A",
		"shareClassType": "preferred_a",
		"phaseNumber":    1,
		"sortOrder":      1,
	})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{
		"scenarioId": scenarioID.String(),
		"yearIndex":  "0",
	})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newFiplanHandler(svc, linker).CreateRoundFromCapitalIncrease(w, r)

	require.Equal(t, http.StatusCreated, w.Code)
	assert.NotNil(t, capturedRound)
	assert.Equal(t, "Series A", capturedRound.Label)
}

func TestFiplanHandler_CreateRoundFromCapitalIncrease_NoRoundLinker(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	svc := &mockFiplanService{}

	body, _ := json.Marshal(map[string]interface{}{
		"label": "Series A",
	})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{
		"scenarioId": scenarioID.String(),
		"yearIndex":  "0",
	})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newFiplanHandler(svc, nil).CreateRoundFromCapitalIncrease(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestFiplanHandler_CreateRoundFromCapitalIncrease_BadScenarioUUID(t *testing.T) {
	svc := &mockFiplanService{}
	linker := &mockCapTableRoundLinker{}

	body, _ := json.Marshal(map[string]interface{}{"label": "Series A"})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{
		"scenarioId": "bad",
		"yearIndex":  "0",
	})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newFiplanHandler(svc, linker).CreateRoundFromCapitalIncrease(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFiplanHandler_CreateRoundFromCapitalIncrease_BadYearIndex(t *testing.T) {
	svc := &mockFiplanService{}
	linker := &mockCapTableRoundLinker{}

	body, _ := json.Marshal(map[string]interface{}{"label": "Series A"})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{
		"scenarioId": uuid.New().String(),
		"yearIndex":  "not-a-number",
	})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newFiplanHandler(svc, linker).CreateRoundFromCapitalIncrease(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFiplanHandler_CreateRoundFromCapitalIncrease_YearIndexOutOfRange(t *testing.T) {
	for _, yearIdx := range []string{"-1", "5", "10"} {
		svc := &mockFiplanService{}
		linker := &mockCapTableRoundLinker{}

		body, _ := json.Marshal(map[string]interface{}{"label": "Series A"})
		r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r = withChiParams(r, map[string]string{
			"scenarioId": uuid.New().String(),
			"yearIndex":  yearIdx,
		})
		r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
		w := httptest.NewRecorder()

		newFiplanHandler(svc, linker).CreateRoundFromCapitalIncrease(w, r)

		assert.Equal(t, http.StatusBadRequest, w.Code, "year index "+yearIdx+" should be rejected")
	}
}

func TestFiplanHandler_CreateRoundFromCapitalIncrease_MissingLabel(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	svc := &mockFiplanService{}
	linker := &mockCapTableRoundLinker{}

	body, _ := json.Marshal(map[string]interface{}{
		"shareClassType": "preferred_a",
	})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{
		"scenarioId": scenarioID.String(),
		"yearIndex":  "0",
	})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newFiplanHandler(svc, linker).CreateRoundFromCapitalIncrease(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFiplanHandler_CreateRoundFromCapitalIncrease_ZeroAmount(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	fiplanEntry := &model.FiplanEntry{
		Amount: decimal.Zero,
	}

	svc := &mockFiplanService{
		getCapitalIncreaseEntryFn: func(_ context.Context, _, _ uuid.UUID, _ int) (*model.FiplanEntry, error) {
			return fiplanEntry, nil
		},
	}
	linker := &mockCapTableRoundLinker{}

	body, _ := json.Marshal(map[string]interface{}{"label": "Series A"})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{
		"scenarioId": scenarioID.String(),
		"yearIndex":  "0",
	})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newFiplanHandler(svc, linker).CreateRoundFromCapitalIncrease(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFiplanHandler_CreateRoundFromCapitalIncrease_AlreadyLinked(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	roundID := uuid.New()

	fiplanEntry := &model.FiplanEntry{
		Amount:          decimal.NewFromInt(1000000),
		CapTableRoundID: &roundID,
	}

	svc := &mockFiplanService{
		getCapitalIncreaseEntryFn: func(_ context.Context, _, _ uuid.UUID, _ int) (*model.FiplanEntry, error) {
			return fiplanEntry, nil
		},
	}
	linker := &mockCapTableRoundLinker{}

	body, _ := json.Marshal(map[string]interface{}{"label": "Series A"})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{
		"scenarioId": scenarioID.String(),
		"yearIndex":  "0",
	})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newFiplanHandler(svc, linker).CreateRoundFromCapitalIncrease(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFiplanHandler_CreateRoundFromCapitalIncrease_ServiceError(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	svc := &mockFiplanService{
		getCapitalIncreaseEntryFn: func(_ context.Context, _, _ uuid.UUID, _ int) (*model.FiplanEntry, error) {
			return nil, apierror.NotFound("entry", "not found")
		},
	}
	linker := &mockCapTableRoundLinker{}

	body, _ := json.Marshal(map[string]interface{}{"label": "Series A"})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{
		"scenarioId": scenarioID.String(),
		"yearIndex":  "0",
	})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newFiplanHandler(svc, linker).CreateRoundFromCapitalIncrease(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestFiplanHandler_CreateRoundFromCapitalIncrease_InvalidBody(t *testing.T) {
	svc := &mockFiplanService{}
	linker := &mockCapTableRoundLinker{}

	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("invalid json")))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{
		"scenarioId": uuid.New().String(),
		"yearIndex":  "0",
	})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newFiplanHandler(svc, linker).CreateRoundFromCapitalIncrease(w, r)

	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusUnprocessableEntity)
}
