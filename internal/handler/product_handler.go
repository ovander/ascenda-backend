package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"kerplan/internal/dto"
	"kerplan/internal/model"
	"kerplan/internal/pkg/ctxutil"
	"kerplan/internal/service"
)

// ProductHandler handles product operations.
type ProductHandler struct {
	svc    *service.ProductService
	logger *logrus.Entry
}

// NewProductHandler creates a new ProductHandler.
func NewProductHandler(svc *service.ProductService, logger *logrus.Entry) *ProductHandler {
	return &ProductHandler{
		svc:    svc,
		logger: logger,
	}
}

// List returns all products for a scenario.
func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	products, err := h.svc.ListProducts(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.ProductsFromPtrs(products))
}

// CreateProductRequest represents a product creation request.
type CreateProductRequest struct {
	Name string `json:"name" validate:"required"`
}

// Create creates a new product.
func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateProductRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, err)
		return
	}

	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	product := &model.Product{
		Name: req.Name,
	}

	if err := h.svc.CreateProduct(r.Context(), tenantID, scenarioID, product); err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, dto.ProductFromModel(*product))
}

// Get retrieves a product by ID.
func (h *ProductHandler) Get(w http.ResponseWriter, r *http.Request) {
	productID, err := parseUUIDParam(chi.URLParam(r, "productId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	product, err := h.svc.GetProduct(r.Context(), tenantID, productID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.ProductFromModel(*product))
}

// UpdateProductRequest represents a product update request.
type UpdateProductRequest struct {
	Name                      string          `json:"name"`
	DirectCostVariability     decimal.Decimal `json:"directCostVariability"`
	ExternalChargeVariability decimal.Decimal `json:"externalChargeVariability"`
	TaxVariability            decimal.Decimal `json:"taxVariability"`
	StaffVariability          decimal.Decimal `json:"staffVariability"`
	DepreciationVariability   decimal.Decimal `json:"depreciationVariability"`
}

// Update updates a product.
func (h *ProductHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req UpdateProductRequest
	if err := decodeJSON(r, &req); err != nil {
		handleError(w, err)
		return
	}

	productID, err := parseUUIDParam(chi.URLParam(r, "productId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	product := &model.Product{
		Name:                      req.Name,
		DirectCostVariability:     req.DirectCostVariability,
		ExternalChargeVariability: req.ExternalChargeVariability,
		TaxVariability:            req.TaxVariability,
		StaffVariability:          req.StaffVariability,
		DepreciationVariability:   req.DepreciationVariability,
	}

	if err := h.svc.UpdateProduct(r.Context(), tenantID, productID, product); err != nil {
		handleError(w, err)
		return
	}

	respondNoContent(w)
}

// Delete removes a product.
func (h *ProductHandler) Delete(w http.ResponseWriter, r *http.Request) {
	productID, err := parseUUIDParam(chi.URLParam(r, "productId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.DeleteProduct(r.Context(), tenantID, productID); err != nil {
		handleError(w, err)
		return
	}

	respondNoContent(w)
}

// GetAssumptions retrieves product assumptions.
func (h *ProductHandler) GetAssumptions(w http.ResponseWriter, r *http.Request) {
	productID, err := parseUUIDParam(chi.URLParam(r, "productId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	assumptions, err := h.svc.GetAssumptions(r.Context(), tenantID, productID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.AssumptionsFromModels(assumptions))
}

// UpdateAssumptions updates product assumptions.
func (h *ProductHandler) UpdateAssumptions(w http.ResponseWriter, r *http.Request) {
	productID, err := parseUUIDParam(chi.URLParam(r, "productId"))
	if err != nil {
		handleError(w, err)
		return
	}

	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	var assumptions []model.ProductAssumption
	if err := decodeAndValidate(r, &assumptions); err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateAssumptions(r.Context(), tenantID, scenarioID, productID, assumptions); err != nil {
		handleError(w, err)
		return
	}

	respondNoContent(w)
}

// GetVolumes retrieves product sales volumes.
func (h *ProductHandler) GetVolumes(w http.ResponseWriter, r *http.Request) {
	productID, err := parseUUIDParam(chi.URLParam(r, "productId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	volumes, err := h.svc.GetVolumes(r.Context(), tenantID, productID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.VolumesFromModels(volumes))
}

// UpdateVolumes updates product sales volumes.
func (h *ProductHandler) UpdateVolumes(w http.ResponseWriter, r *http.Request) {
	productID, err := parseUUIDParam(chi.URLParam(r, "productId"))
	if err != nil {
		handleError(w, err)
		return
	}

	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	var volumes []model.ProductSalesVolume
	if err := decodeAndValidate(r, &volumes); err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateVolumes(r.Context(), tenantID, scenarioID, productID, volumes); err != nil {
		handleError(w, err)
		return
	}

	respondNoContent(w)
}

// GetMargins retrieves distributor margins.
func (h *ProductHandler) GetMargins(w http.ResponseWriter, r *http.Request) {
	productID, err := parseUUIDParam(chi.URLParam(r, "productId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	margins, err := h.svc.GetMargins(r.Context(), tenantID, productID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.MarginsFromModels(margins))
}

// UpdateMargins updates distributor margins.
func (h *ProductHandler) UpdateMargins(w http.ResponseWriter, r *http.Request) {
	productID, err := parseUUIDParam(chi.URLParam(r, "productId"))
	if err != nil {
		handleError(w, err)
		return
	}

	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	var margins []model.ProductDistributorMargin
	if err := decodeAndValidate(r, &margins); err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateMargins(r.Context(), tenantID, scenarioID, productID, margins); err != nil {
		handleError(w, err)
		return
	}

	respondNoContent(w)
}

// GetConsolidatedRevenue returns consolidated revenue metrics.
func (h *ProductHandler) GetConsolidatedRevenue(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	revenue, err := h.svc.GetConsolidatedRevenue(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, revenue)
}
