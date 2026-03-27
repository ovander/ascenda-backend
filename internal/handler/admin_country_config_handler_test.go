package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ascenda/internal/model"
	"ascenda/internal/pkg/apierror"
	"ascenda/internal/service"
)

// ── stub service ─────────────────────────────────────────────────────────────

type stubCountryConfigSvc struct {
	listFn          func() ([]*model.CountryRateConfig, error)
	getByCodeFn     func(code string) (*model.CountryRateConfig, error)
	updateFn        func(code string, req service.UpdateCountryRateConfigRequest) (*model.CountryRateConfig, error)
	resetToDefaultFn func(code string) (*model.CountryRateConfig, error)
}

func (s *stubCountryConfigSvc) List() ([]*model.CountryRateConfig, error) {
	if s.listFn != nil {
		return s.listFn()
	}
	return nil, apierror.Internal("list not implemented")
}

func (s *stubCountryConfigSvc) GetByCode(code string) (*model.CountryRateConfig, error) {
	if s.getByCodeFn != nil {
		return s.getByCodeFn(code)
	}
	return nil, apierror.Internal("getByCode not implemented")
}

func (s *stubCountryConfigSvc) Update(code string, req service.UpdateCountryRateConfigRequest) (*model.CountryRateConfig, error) {
	if s.updateFn != nil {
		return s.updateFn(code, req)
	}
	return nil, apierror.Internal("update not implemented")
}

func (s *stubCountryConfigSvc) ResetToDefault(code string) (*model.CountryRateConfig, error) {
	if s.resetToDefaultFn != nil {
		return s.resetToDefaultFn(code)
	}
	return nil, apierror.Internal("resetToDefault not implemented")
}

// ── helpers ───────────────────────────────────────────────────────────────────

// compile-time check: stub satisfies the handler interface.
var _ CountryRateConfigSvc = (*stubCountryConfigSvc)(nil)

func newCountryConfigHandler(svc *stubCountryConfigSvc) *AdminCountryConfigHandler {
	return NewAdminCountryConfigHandler(svc, logrus.NewEntry(logrus.New()))
}

func frConfig() *model.CountryRateConfig {
	return &model.CountryRateConfig{
		CountryCode:      "FR",
		CountryName:      "France",
		CorporateTaxRate: decimal.NewFromFloat(0.25),
		VATRate:          decimal.NewFromFloat(0.20),
		EmployerTaxRate:  decimal.NewFromFloat(0.45),
		MLTInterestRate:  decimal.NewFromFloat(0.03),
		Language:         "fr",
		CurrencySymbol:   "€",
	}
}

// routeWith sets chi URL params on the request context.
func routeWith(r *http.Request, params map[string]string) *http.Request {
	return withChiParams(r, params)
}

// ── List ─────────────────────────────────────────────────────────────────────

func TestAdminCountryConfig_List_Success(t *testing.T) {
	svc := &stubCountryConfigSvc{
		listFn: func() ([]*model.CountryRateConfig, error) {
			return []*model.CountryRateConfig{frConfig()}, nil
		},
	}
	h := newCountryConfigHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/admin/country-configs", nil)
	w := httptest.NewRecorder()
	h.List(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp []model.CountryRateConfig
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Len(t, resp, 1)
	assert.Equal(t, "FR", resp[0].CountryCode)
}

func TestAdminCountryConfig_List_ServiceError(t *testing.T) {
	svc := &stubCountryConfigSvc{
		listFn: func() ([]*model.CountryRateConfig, error) {
			return nil, apierror.Internal("db down")
		},
	}
	h := newCountryConfigHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/admin/country-configs", nil)
	w := httptest.NewRecorder()
	h.List(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ── Get ──────────────────────────────────────────────────────────────────────

func TestAdminCountryConfig_Get_Success(t *testing.T) {
	svc := &stubCountryConfigSvc{
		getByCodeFn: func(code string) (*model.CountryRateConfig, error) {
			assert.Equal(t, "FR", code)
			return frConfig(), nil
		},
	}
	h := newCountryConfigHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/admin/country-configs/FR", nil)
	r = routeWith(r, map[string]string{"code": "FR"})
	w := httptest.NewRecorder()
	h.Get(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp model.CountryRateConfig
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "FR", resp.CountryCode)
}

func TestAdminCountryConfig_Get_UppercasesCode(t *testing.T) {
	called := ""
	svc := &stubCountryConfigSvc{
		getByCodeFn: func(code string) (*model.CountryRateConfig, error) {
			called = code
			return frConfig(), nil
		},
	}
	h := newCountryConfigHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/admin/country-configs/fr", nil)
	r = routeWith(r, map[string]string{"code": "fr"})
	w := httptest.NewRecorder()
	h.Get(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "FR", called, "code should be uppercased before service call")
}

func TestAdminCountryConfig_Get_NotFound(t *testing.T) {
	svc := &stubCountryConfigSvc{
		getByCodeFn: func(code string) (*model.CountryRateConfig, error) {
			return nil, apierror.NotFound("country config", code)
		},
	}
	h := newCountryConfigHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/admin/country-configs/ZZ", nil)
	r = routeWith(r, map[string]string{"code": "ZZ"})
	w := httptest.NewRecorder()
	h.Get(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── Update ───────────────────────────────────────────────────────────────────

func TestAdminCountryConfig_Update_Success(t *testing.T) {
	newRate := decimal.NewFromFloat(0.30)
	var gotReq service.UpdateCountryRateConfigRequest
	svc := &stubCountryConfigSvc{
		updateFn: func(code string, req service.UpdateCountryRateConfigRequest) (*model.CountryRateConfig, error) {
			assert.Equal(t, "FR", code)
			gotReq = req
			cfg := frConfig()
			cfg.CorporateTaxRate = newRate
			return cfg, nil
		},
	}
	h := newCountryConfigHandler(svc)

	body, _ := json.Marshal(map[string]any{"corporateTaxRate": newRate})
	r := httptest.NewRequest(http.MethodPut, "/admin/country-configs/FR", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = routeWith(r, map[string]string{"code": "FR"})
	w := httptest.NewRecorder()
	h.Update(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, gotReq.CorporateTaxRate)
	assert.True(t, gotReq.CorporateTaxRate.Equal(newRate))
}

func TestAdminCountryConfig_Update_InvalidJSON(t *testing.T) {
	svc := &stubCountryConfigSvc{}
	h := newCountryConfigHandler(svc)

	r := httptest.NewRequest(http.MethodPut, "/admin/country-configs/FR", bytes.NewReader([]byte("not-json")))
	r.Header.Set("Content-Type", "application/json")
	r = routeWith(r, map[string]string{"code": "FR"})
	w := httptest.NewRecorder()
	h.Update(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminCountryConfig_Update_ServiceError(t *testing.T) {
	svc := &stubCountryConfigSvc{
		updateFn: func(code string, req service.UpdateCountryRateConfigRequest) (*model.CountryRateConfig, error) {
			return nil, apierror.NotFound("country config", code)
		},
	}
	h := newCountryConfigHandler(svc)

	body, _ := json.Marshal(map[string]any{})
	r := httptest.NewRequest(http.MethodPut, "/admin/country-configs/ZZ", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = routeWith(r, map[string]string{"code": "ZZ"})
	w := httptest.NewRecorder()
	h.Update(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── Reset ─────────────────────────────────────────────────────────────────────

func TestAdminCountryConfig_Reset_Success(t *testing.T) {
	svc := &stubCountryConfigSvc{
		resetToDefaultFn: func(code string) (*model.CountryRateConfig, error) {
			assert.Equal(t, "FR", code)
			return frConfig(), nil
		},
	}
	h := newCountryConfigHandler(svc)

	r := httptest.NewRequest(http.MethodPost, "/admin/country-configs/FR/reset", nil)
	r = routeWith(r, map[string]string{"code": "FR"})
	w := httptest.NewRecorder()
	h.Reset(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp model.CountryRateConfig
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "FR", resp.CountryCode)
}

func TestAdminCountryConfig_Reset_NotFound(t *testing.T) {
	svc := &stubCountryConfigSvc{
		resetToDefaultFn: func(code string) (*model.CountryRateConfig, error) {
			return nil, apierror.NotFound("country config", code)
		},
	}
	h := newCountryConfigHandler(svc)

	r := httptest.NewRequest(http.MethodPost, "/admin/country-configs/ZZ/reset", nil)
	r = routeWith(r, map[string]string{"code": "ZZ"})
	w := httptest.NewRecorder()
	h.Reset(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
