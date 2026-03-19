package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"kerplan/internal/model"
	"kerplan/internal/pkg/apierror"
	"kerplan/internal/pkg/ctxutil"
)

// MockSettingsService is a mock implementation of SettingsServicer.
type MockSettingsService struct {
	GetConfigFunc            func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.PlanConfig, error)
	UpdateConfigFunc         func(ctx context.Context, tenantID, scenarioID uuid.UUID, config *model.PlanConfig) error
	GetOpeningBalanceFunc    func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.OpeningBalance, error)
	UpdateOpeningBalanceFunc func(ctx context.Context, tenantID, scenarioID uuid.UUID, balance *model.OpeningBalance) error
	GetWCConfigFunc          func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.WorkingCapitalConfig, error)
	UpdateWCConfigFunc       func(ctx context.Context, tenantID, scenarioID uuid.UUID, wcConfig *model.WorkingCapitalConfig) error
	GetOpexPerHireFunc       func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.OpexPerHire, error)
	UpdateOpexPerHireFunc    func(ctx context.Context, tenantID, scenarioID uuid.UUID, opexPerHire *model.OpexPerHire) error
	GetCapexPerHireFunc      func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.CapexPerHire, error)
	UpdateCapexPerHireFunc   func(ctx context.Context, tenantID, scenarioID uuid.UUID, capexPerHire *model.CapexPerHire) error
}

func (m *MockSettingsService) GetConfig(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.PlanConfig, error) {
	if m.GetConfigFunc != nil {
		return m.GetConfigFunc(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("mock not implemented")
}

func (m *MockSettingsService) UpdateConfig(ctx context.Context, tenantID, scenarioID uuid.UUID, config *model.PlanConfig) error {
	if m.UpdateConfigFunc != nil {
		return m.UpdateConfigFunc(ctx, tenantID, scenarioID, config)
	}
	return apierror.Internal("mock not implemented")
}

func (m *MockSettingsService) GetOpeningBalance(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.OpeningBalance, error) {
	if m.GetOpeningBalanceFunc != nil {
		return m.GetOpeningBalanceFunc(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("mock not implemented")
}

func (m *MockSettingsService) UpdateOpeningBalance(ctx context.Context, tenantID, scenarioID uuid.UUID, balance *model.OpeningBalance) error {
	if m.UpdateOpeningBalanceFunc != nil {
		return m.UpdateOpeningBalanceFunc(ctx, tenantID, scenarioID, balance)
	}
	return apierror.Internal("mock not implemented")
}

func (m *MockSettingsService) GetWCConfig(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.WorkingCapitalConfig, error) {
	if m.GetWCConfigFunc != nil {
		return m.GetWCConfigFunc(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("mock not implemented")
}

func (m *MockSettingsService) UpdateWCConfig(ctx context.Context, tenantID, scenarioID uuid.UUID, wcConfig *model.WorkingCapitalConfig) error {
	if m.UpdateWCConfigFunc != nil {
		return m.UpdateWCConfigFunc(ctx, tenantID, scenarioID, wcConfig)
	}
	return apierror.Internal("mock not implemented")
}

func (m *MockSettingsService) GetOpexPerHire(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.OpexPerHire, error) {
	if m.GetOpexPerHireFunc != nil {
		return m.GetOpexPerHireFunc(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("mock not implemented")
}

func (m *MockSettingsService) UpdateOpexPerHire(ctx context.Context, tenantID, scenarioID uuid.UUID, opexPerHire *model.OpexPerHire) error {
	if m.UpdateOpexPerHireFunc != nil {
		return m.UpdateOpexPerHireFunc(ctx, tenantID, scenarioID, opexPerHire)
	}
	return apierror.Internal("mock not implemented")
}

func (m *MockSettingsService) GetCapexPerHire(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.CapexPerHire, error) {
	if m.GetCapexPerHireFunc != nil {
		return m.GetCapexPerHireFunc(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("mock not implemented")
}

func (m *MockSettingsService) UpdateCapexPerHire(ctx context.Context, tenantID, scenarioID uuid.UUID, capexPerHire *model.CapexPerHire) error {
	if m.UpdateCapexPerHireFunc != nil {
		return m.UpdateCapexPerHireFunc(ctx, tenantID, scenarioID, capexPerHire)
	}
	return apierror.Internal("mock not implemented")
}

// Helper to set up chi router context with URL parameters.
func withChiParams(r *http.Request, params map[string]string) *http.Request {
	chiCtx := chi.NewRouteContext()
	for k, v := range params {
		chiCtx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, chiCtx))
}

// TestGetConfig_Success tests successful retrieval of plan config.
func TestGetConfig_Success(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	tenantID := uuid.New()
	scenarioID := uuid.New()

	mockSvc := &MockSettingsService{
		GetConfigFunc: func(ctx context.Context, tid, sid uuid.UUID) (*model.PlanConfig, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return &model.PlanConfig{
				TenantScoped:          model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
				ScenarioID:            scenarioID,
				SalaryMonthsPerYear:   12,
				FirstFiscalYearMonths: 12,
				DiscountRate:          decimal.RequireFromString("0.10"),
				CorporateTaxRate:      decimal.RequireFromString("0.25"),
				Country:               "BE",
			}, nil
		},
	}

	handler := NewSettingsHandler(mockSvc, logger)
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	r := httptest.NewRequest("GET", "/settings/config", nil).WithContext(ctx)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	w := httptest.NewRecorder()

	handler.GetConfig(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var config model.PlanConfig
	err := json.Unmarshal(w.Body.Bytes(), &config)
	assert.NoError(t, err)
	assert.Equal(t, scenarioID, config.ScenarioID)
	assert.Equal(t, 12, config.SalaryMonthsPerYear)
}

// TestGetConfig_BadUUID tests invalid UUID parameter.
func TestGetConfig_BadUUID(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	tenantID := uuid.New()

	mockSvc := &MockSettingsService{}
	handler := NewSettingsHandler(mockSvc, logger)

	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	r := httptest.NewRequest("GET", "/settings/config", nil).WithContext(ctx)
	r = withChiParams(r, map[string]string{"scenarioId": "not-a-uuid"})
	w := httptest.NewRecorder()

	handler.GetConfig(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetConfig_MissingScenarioId tests missing scenarioId parameter.
func TestGetConfig_MissingScenarioId(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	tenantID := uuid.New()

	mockSvc := &MockSettingsService{}
	handler := NewSettingsHandler(mockSvc, logger)

	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	r := httptest.NewRequest("GET", "/settings/config", nil).WithContext(ctx)
	r = withChiParams(r, map[string]string{})
	w := httptest.NewRecorder()

	handler.GetConfig(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetConfig_NotFound tests when config is not found.
func TestGetConfig_NotFound(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	tenantID := uuid.New()
	scenarioID := uuid.New()

	mockSvc := &MockSettingsService{
		GetConfigFunc: func(ctx context.Context, tid, sid uuid.UUID) (*model.PlanConfig, error) {
			return nil, apierror.NotFound("config", sid.String())
		},
	}

	handler := NewSettingsHandler(mockSvc, logger)
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	r := httptest.NewRequest("GET", "/settings/config", nil).WithContext(ctx)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	w := httptest.NewRecorder()

	handler.GetConfig(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestUpdateConfig_Success tests successful config update.
func TestUpdateConfig_Success(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	tenantID := uuid.New()
	scenarioID := uuid.New()

	mockSvc := &MockSettingsService{
		UpdateConfigFunc: func(ctx context.Context, tid, sid uuid.UUID, cfg *model.PlanConfig) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			assert.Equal(t, 12, cfg.SalaryMonthsPerYear)
			return nil
		},
	}

	handler := NewSettingsHandler(mockSvc, logger)
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	configBody := model.PlanConfig{
		SalaryMonthsPerYear:   12,
		FirstFiscalYearMonths: 12,
		DiscountRate:          decimal.RequireFromString("0.10"),
		CorporateTaxRate:      decimal.RequireFromString("0.25"),
		Country:               "BE",
	}

	bodyBytes, _ := json.Marshal(configBody)
	r := httptest.NewRequest("PUT", "/settings/config", bytes.NewReader(bodyBytes)).WithContext(ctx)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateConfig(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

// TestUpdateConfig_InvalidJSON tests invalid JSON body.
func TestUpdateConfig_InvalidJSON(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	tenantID := uuid.New()
	scenarioID := uuid.New()

	mockSvc := &MockSettingsService{}
	handler := NewSettingsHandler(mockSvc, logger)

	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	r := httptest.NewRequest("PUT", "/settings/config", bytes.NewReader([]byte("invalid json"))).WithContext(ctx)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateConfig(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestUpdateConfig_BadScenarioUUID tests bad scenario UUID in update.
func TestUpdateConfig_BadScenarioUUID(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	tenantID := uuid.New()

	mockSvc := &MockSettingsService{}
	handler := NewSettingsHandler(mockSvc, logger)

	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	configBody := model.PlanConfig{SalaryMonthsPerYear: 12}
	bodyBytes, _ := json.Marshal(configBody)
	r := httptest.NewRequest("PUT", "/settings/config", bytes.NewReader(bodyBytes)).WithContext(ctx)
	r = withChiParams(r, map[string]string{"scenarioId": "bad-uuid"})
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateConfig(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetOpeningBalance_Success tests successful retrieval of opening balance.
func TestGetOpeningBalance_Success(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	tenantID := uuid.New()
	scenarioID := uuid.New()

	mockSvc := &MockSettingsService{
		GetOpeningBalanceFunc: func(ctx context.Context, tid, sid uuid.UUID) (*model.OpeningBalance, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return &model.OpeningBalance{
				TenantScoped:        model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
				ScenarioID:          scenarioID,
				NoncurrentAssets:    decimal.RequireFromString("100000"),
				ShareCapital:        decimal.RequireFromString("50000"),
				CustomerReceivables: decimal.RequireFromString("10000"),
				CashAndSecurities:   decimal.RequireFromString("5000"),
			}, nil
		},
	}

	handler := NewSettingsHandler(mockSvc, logger)
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	r := httptest.NewRequest("GET", "/settings/opening-balance", nil).WithContext(ctx)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	w := httptest.NewRecorder()

	handler.GetOpeningBalance(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var balance model.OpeningBalance
	err := json.Unmarshal(w.Body.Bytes(), &balance)
	assert.NoError(t, err)
	assert.Equal(t, scenarioID, balance.ScenarioID)
}

// TestGetOpeningBalance_MissingScenarioId tests missing scenarioId parameter.
func TestGetOpeningBalance_MissingScenarioId(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	tenantID := uuid.New()

	mockSvc := &MockSettingsService{}
	handler := NewSettingsHandler(mockSvc, logger)

	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	r := httptest.NewRequest("GET", "/settings/opening-balance", nil).WithContext(ctx)
	r = withChiParams(r, map[string]string{})
	w := httptest.NewRecorder()

	handler.GetOpeningBalance(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestUpdateOpeningBalance_Success tests successful opening balance update.
func TestUpdateOpeningBalance_Success(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	tenantID := uuid.New()
	scenarioID := uuid.New()

	mockSvc := &MockSettingsService{
		UpdateOpeningBalanceFunc: func(ctx context.Context, tid, sid uuid.UUID, bal *model.OpeningBalance) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			assert.Equal(t, decimal.RequireFromString("100000"), bal.NoncurrentAssets)
			return nil
		},
	}

	handler := NewSettingsHandler(mockSvc, logger)
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	balanceBody := model.OpeningBalance{
		NoncurrentAssets:    decimal.RequireFromString("100000"),
		ShareCapital:        decimal.RequireFromString("50000"),
		CustomerReceivables: decimal.RequireFromString("10000"),
		CashAndSecurities:   decimal.RequireFromString("5000"),
	}

	bodyBytes, _ := json.Marshal(balanceBody)
	r := httptest.NewRequest("PUT", "/settings/opening-balance", bytes.NewReader(bodyBytes)).WithContext(ctx)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateOpeningBalance(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

// TestUpdateOpeningBalance_InvalidJSON tests invalid JSON in opening balance update.
func TestUpdateOpeningBalance_InvalidJSON(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	tenantID := uuid.New()
	scenarioID := uuid.New()

	mockSvc := &MockSettingsService{}
	handler := NewSettingsHandler(mockSvc, logger)

	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	r := httptest.NewRequest("PUT", "/settings/opening-balance", io.NopCloser(bytes.NewReader([]byte("{invalid}")))).WithContext(ctx)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateOpeningBalance(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetWCConfig_Success tests successful retrieval of working capital config.
func TestGetWCConfig_Success(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	tenantID := uuid.New()
	scenarioID := uuid.New()

	mockSvc := &MockSettingsService{
		GetWCConfigFunc: func(ctx context.Context, tid, sid uuid.UUID) (*model.WorkingCapitalConfig, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return &model.WorkingCapitalConfig{
				TenantScoped:      model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
				ScenarioID:        scenarioID,
				CustomerPct0Days:  decimal.RequireFromString("0.30"),
				CustomerPct30Days: decimal.RequireFromString("0.70"),
				SupplierPct0Days:  decimal.RequireFromString("0.20"),
				SupplierPct30Days: decimal.RequireFromString("0.50"),
				SupplierPct60Days: decimal.RequireFromString("0.30"),
				InventoryPctYear1: decimal.RequireFromString("0.10"),
				InventoryPctYear2: decimal.RequireFromString("0.10"),
				InventoryPctYear3: decimal.RequireFromString("0.10"),
				InventoryPctYear4: decimal.RequireFromString("0.10"),
				InventoryPctYear5: decimal.RequireFromString("0.10"),
			}, nil
		},
	}

	handler := NewSettingsHandler(mockSvc, logger)
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	r := httptest.NewRequest("GET", "/settings/wc-config", nil).WithContext(ctx)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	w := httptest.NewRecorder()

	handler.GetWCConfig(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var wcConfig model.WorkingCapitalConfig
	err := json.Unmarshal(w.Body.Bytes(), &wcConfig)
	assert.NoError(t, err)
	assert.Equal(t, scenarioID, wcConfig.ScenarioID)
	assert.True(t, decimal.RequireFromString("0.30").Equal(wcConfig.CustomerPct0Days))
}

// TestGetWCConfig_BadUUID tests bad scenario UUID for WC config.
func TestGetWCConfig_BadUUID(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	tenantID := uuid.New()

	mockSvc := &MockSettingsService{}
	handler := NewSettingsHandler(mockSvc, logger)

	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	r := httptest.NewRequest("GET", "/settings/wc-config", nil).WithContext(ctx)
	r = withChiParams(r, map[string]string{"scenarioId": "invalid-uuid"})
	w := httptest.NewRecorder()

	handler.GetWCConfig(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestUpdateWCConfig_Success tests successful working capital config update.
func TestUpdateWCConfig_Success(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	tenantID := uuid.New()
	scenarioID := uuid.New()

	mockSvc := &MockSettingsService{
		UpdateWCConfigFunc: func(ctx context.Context, tid, sid uuid.UUID, wc *model.WorkingCapitalConfig) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			assert.True(t, decimal.RequireFromString("0.30").Equal(wc.CustomerPct0Days))
			return nil
		},
	}

	handler := NewSettingsHandler(mockSvc, logger)
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	wcBody := model.WorkingCapitalConfig{
		CustomerPct0Days:  decimal.RequireFromString("0.30"),
		CustomerPct30Days: decimal.RequireFromString("0.70"),
		SupplierPct0Days:  decimal.RequireFromString("0.20"),
		SupplierPct30Days: decimal.RequireFromString("0.50"),
		SupplierPct60Days: decimal.RequireFromString("0.30"),
		InventoryPctYear1: decimal.RequireFromString("0.10"),
		InventoryPctYear2: decimal.RequireFromString("0.10"),
		InventoryPctYear3: decimal.RequireFromString("0.10"),
		InventoryPctYear4: decimal.RequireFromString("0.10"),
		InventoryPctYear5: decimal.RequireFromString("0.10"),
	}

	bodyBytes, _ := json.Marshal(wcBody)
	r := httptest.NewRequest("PUT", "/settings/wc-config", bytes.NewReader(bodyBytes)).WithContext(ctx)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateWCConfig(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

// TestUpdateWCConfig_MissingScenarioId tests missing scenarioId in WC update.
func TestUpdateWCConfig_MissingScenarioId(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	tenantID := uuid.New()

	mockSvc := &MockSettingsService{}
	handler := NewSettingsHandler(mockSvc, logger)

	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	wcBody := model.WorkingCapitalConfig{
		CustomerPct0Days: decimal.RequireFromString("0.30"),
	}

	bodyBytes, _ := json.Marshal(wcBody)
	r := httptest.NewRequest("PUT", "/settings/wc-config", bytes.NewReader(bodyBytes)).WithContext(ctx)
	r = withChiParams(r, map[string]string{})
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateWCConfig(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestUpdateWCConfig_InvalidJSON tests invalid JSON in WC config update.
func TestUpdateWCConfig_InvalidJSON(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	tenantID := uuid.New()
	scenarioID := uuid.New()

	mockSvc := &MockSettingsService{}
	handler := NewSettingsHandler(mockSvc, logger)

	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	r := httptest.NewRequest("PUT", "/settings/wc-config", bytes.NewReader([]byte("{bad json"))).WithContext(ctx)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateWCConfig(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetOpexPerHire_Success tests successful retrieval of opex per hire settings.
func TestGetOpexPerHire_Success(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	tenantID := uuid.New()
	scenarioID := uuid.New()

	mockSvc := &MockSettingsService{
		GetOpexPerHireFunc: func(ctx context.Context, tid, sid uuid.UUID) (*model.OpexPerHire, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return &model.OpexPerHire{
				TenantScoped:          model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
				ScenarioID:            scenarioID,
				PostageTelecom:        decimal.RequireFromString("1.5"),
				TravelTransportation:  decimal.RequireFromString("3.0"),
			}, nil
		},
	}

	handler := NewSettingsHandler(mockSvc, logger)
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	r := httptest.NewRequest("GET", "/settings/opex-per-hire", nil).WithContext(ctx)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	w := httptest.NewRecorder()

	handler.GetOpexPerHire(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var opexPerHire model.OpexPerHire
	err := json.Unmarshal(w.Body.Bytes(), &opexPerHire)
	assert.NoError(t, err)
	assert.Equal(t, scenarioID, opexPerHire.ScenarioID)
}

// TestGetOpexPerHire_BadUUID tests bad scenario UUID for opex per hire.
func TestGetOpexPerHire_BadUUID(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	tenantID := uuid.New()

	mockSvc := &MockSettingsService{}
	handler := NewSettingsHandler(mockSvc, logger)

	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	r := httptest.NewRequest("GET", "/settings/opex-per-hire", nil).WithContext(ctx)
	r = withChiParams(r, map[string]string{"scenarioId": "bad-uuid"})
	w := httptest.NewRecorder()

	handler.GetOpexPerHire(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestUpdateOpexPerHire_Success tests successful opex per hire update.
func TestUpdateOpexPerHire_Success(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	tenantID := uuid.New()
	scenarioID := uuid.New()

	mockSvc := &MockSettingsService{
		UpdateOpexPerHireFunc: func(ctx context.Context, tid, sid uuid.UUID, oph *model.OpexPerHire) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			assert.True(t, decimal.RequireFromString("1.5").Equal(oph.PostageTelecom))
			return nil
		},
	}

	handler := NewSettingsHandler(mockSvc, logger)
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	body := model.OpexPerHire{
		PostageTelecom:       decimal.RequireFromString("1.5"),
		TravelTransportation: decimal.RequireFromString("3.0"),
	}

	bodyBytes, _ := json.Marshal(body)
	r := httptest.NewRequest("PUT", "/settings/opex-per-hire", bytes.NewReader(bodyBytes)).WithContext(ctx)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateOpexPerHire(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

// TestGetCapexPerHire_Success tests successful retrieval of capex per hire settings.
func TestGetCapexPerHire_Success(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	tenantID := uuid.New()
	scenarioID := uuid.New()

	mockSvc := &MockSettingsService{
		GetCapexPerHireFunc: func(ctx context.Context, tid, sid uuid.UUID) (*model.CapexPerHire, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return &model.CapexPerHire{
				TenantScoped:     model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
				ScenarioID:       scenarioID,
				FurniturePerHire: decimal.RequireFromString("2.0"),
				ITEquipPerHire:   decimal.RequireFromString("3.5"),
			}, nil
		},
	}

	handler := NewSettingsHandler(mockSvc, logger)
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	r := httptest.NewRequest("GET", "/settings/capex-per-hire", nil).WithContext(ctx)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	w := httptest.NewRecorder()

	handler.GetCapexPerHire(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var capexPerHire model.CapexPerHire
	err := json.Unmarshal(w.Body.Bytes(), &capexPerHire)
	assert.NoError(t, err)
	assert.Equal(t, scenarioID, capexPerHire.ScenarioID)
}

// TestGetCapexPerHire_BadUUID tests bad scenario UUID for capex per hire.
func TestGetCapexPerHire_BadUUID(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	tenantID := uuid.New()

	mockSvc := &MockSettingsService{}
	handler := NewSettingsHandler(mockSvc, logger)

	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	r := httptest.NewRequest("GET", "/settings/capex-per-hire", nil).WithContext(ctx)
	r = withChiParams(r, map[string]string{"scenarioId": "bad-uuid"})
	w := httptest.NewRecorder()

	handler.GetCapexPerHire(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestUpdateCapexPerHire_Success tests successful capex per hire update.
func TestUpdateCapexPerHire_Success(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	tenantID := uuid.New()
	scenarioID := uuid.New()

	mockSvc := &MockSettingsService{
		UpdateCapexPerHireFunc: func(ctx context.Context, tid, sid uuid.UUID, cph *model.CapexPerHire) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			assert.True(t, decimal.RequireFromString("2.0").Equal(cph.FurniturePerHire))
			return nil
		},
	}

	handler := NewSettingsHandler(mockSvc, logger)
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)

	body := model.CapexPerHire{
		FurniturePerHire: decimal.RequireFromString("2.0"),
		ITEquipPerHire:   decimal.RequireFromString("3.5"),
	}

	bodyBytes, _ := json.Marshal(body)
	r := httptest.NewRequest("PUT", "/settings/capex-per-hire", bytes.NewReader(bodyBytes)).WithContext(ctx)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateCapexPerHire(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

// TestHandler_MissingTenantContext tests handler behavior when tenant ID is missing from context.
func TestHandler_MissingTenantContext(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	scenarioID := uuid.New()

	mockSvc := &MockSettingsService{
		GetConfigFunc: func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.PlanConfig, error) {
			// Verify that nil UUID is passed when context is missing
			assert.Equal(t, uuid.Nil, tenantID)
			return nil, apierror.Internal("no tenant")
		},
	}

	handler := NewSettingsHandler(mockSvc, logger)
	ctx := context.Background()
	// Note: NOT setting tenant ID in context

	r := httptest.NewRequest("GET", "/settings/config", nil).WithContext(ctx)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	w := httptest.NewRecorder()

	handler.GetConfig(w, r)

	// Should return error because tenant is nil
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
