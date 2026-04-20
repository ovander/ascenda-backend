package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/apierror"
	"ascenda/internal/service"
)

// CountryRateConfigSvc is the interface the handler requires.
type CountryRateConfigSvc interface {
	List() ([]*model.CountryRateConfig, error)
	GetByCode(code string) (*model.CountryRateConfig, error)
	Create(req service.CreateCountryRateConfigRequest) (*model.CountryRateConfig, error)
	Update(code string, req service.UpdateCountryRateConfigRequest) (*model.CountryRateConfig, error)
	ResetToDefault(code string) (*model.CountryRateConfig, error)
}

// AdminCountryConfigHandler exposes platform-admin CRUD for country rate configs.
type AdminCountryConfigHandler struct {
	svc    CountryRateConfigSvc
	logger *logrus.Entry
}

func NewAdminCountryConfigHandler(svc CountryRateConfigSvc, logger *logrus.Entry) *AdminCountryConfigHandler {
	return &AdminCountryConfigHandler{svc: svc, logger: logger}
}

// List handles GET /api/v1/admin/country-configs
func (h *AdminCountryConfigHandler) List(w http.ResponseWriter, r *http.Request) {
	configs, err := h.svc.List()
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, configs)
}

// Get handles GET /api/v1/admin/country-configs/{code}
func (h *AdminCountryConfigHandler) Get(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(chi.URLParam(r, "code"))
	if code == "" {
		handleError(w, r, apierror.BadRequest("country code is required"))
		return
	}
	cfg, err := h.svc.GetByCode(code)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, cfg)
}

// Create handles POST /api/v1/admin/country-configs
func (h *AdminCountryConfigHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req service.CreateCountryRateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handleError(w, r, apierror.BadRequest("invalid request body"))
		return
	}
	cfg, err := h.svc.Create(req)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, cfg)
}

// Update handles PUT /api/v1/admin/country-configs/{code}
func (h *AdminCountryConfigHandler) Update(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(chi.URLParam(r, "code"))
	if code == "" {
		handleError(w, r, apierror.BadRequest("country code is required"))
		return
	}
	var req service.UpdateCountryRateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handleError(w, r, apierror.BadRequest("invalid request body"))
		return
	}
	cfg, err := h.svc.Update(code, req)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, cfg)
}

// Reset handles POST /api/v1/admin/country-configs/{code}/reset
// Restores rates to the built-in defaults.
func (h *AdminCountryConfigHandler) Reset(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(chi.URLParam(r, "code"))
	if code == "" {
		handleError(w, r, apierror.BadRequest("country code is required"))
		return
	}
	cfg, err := h.svc.ResetToDefault(code)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, cfg)
}
