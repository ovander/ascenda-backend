package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"ascenda/internal/dto"
	"ascenda/internal/model"
	"ascenda/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"
)

// ProductServicer is the narrow interface the ProductHandler depends on.
// Using an interface instead of the concrete *service.ProductService makes the
// handler independently unit-testable without standing up a full service graph.
type ProductServicer interface {
	ListProducts(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]*model.Product, error)
	CreateProduct(ctx context.Context, tenantID, scenarioID uuid.UUID, product *model.Product) error
	GetProduct(ctx context.Context, tenantID, productID uuid.UUID) (*model.Product, error)
	UpdateProduct(ctx context.Context, tenantID, productID uuid.UUID, product *model.Product) error
	DeleteProduct(ctx context.Context, tenantID, productID uuid.UUID) error
	GetAssumptions(ctx context.Context, tenantID, productID uuid.UUID) ([]model.ProductAssumption, error)
	UpdateAssumptions(ctx context.Context, tenantID, scenarioID, productID uuid.UUID, assumptions []model.ProductAssumption) error
	GetVolumes(ctx context.Context, tenantID, productID uuid.UUID) ([]model.ProductSalesVolume, error)
	UpdateVolumes(ctx context.Context, tenantID, scenarioID, productID uuid.UUID, volumes []model.ProductSalesVolume) error
	GetMargins(ctx context.Context, tenantID, productID uuid.UUID) ([]model.ProductDistributorMargin, error)
	UpdateMargins(ctx context.Context, tenantID, scenarioID, productID uuid.UUID, margins []model.ProductDistributorMargin) error
	GetDerivedBundle(ctx context.Context, tenantID, productID uuid.UUID) (*service.DerivedBundleResult, error)
	GetRevenueByProduct(ctx context.Context, tenantID, scenarioID, productID uuid.UUID) (*model.ProductRevenueSummary, error)
	GetConsolidatedRevenue(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.ConsolidatedRevenue, error)
}

// ProductHandler handles product operations.
type ProductHandler struct {
	svc    ProductServicer
	logger *logrus.Entry
}

// NewProductHandler creates a new ProductHandler.
func NewProductHandler(svc ProductServicer, logger *logrus.Entry) *ProductHandler {
	return &ProductHandler{
		svc:    svc,
		logger: logger,
	}
}

// List returns all products for a scenario.
func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	products, err := h.svc.ListProducts(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.ProductsFromPtrs(products))
}

// CreateProductRequest represents a product creation request.
type CreateProductRequest struct {
	Name         string          `json:"name"         validate:"required"`
	ProductType  string          `json:"productType"`
	DriverType   string          `json:"driverType"`
	DriverParams json.RawMessage `json:"driverParams"`
}

// Create creates a new product.
func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateProductRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	pType := model.ProductTypeProduct
	if req.ProductType == string(model.ProductTypeService) {
		pType = model.ProductTypeService
	}

	dType := model.DriverGeneric
	if req.DriverType != "" {
		dType = model.DriverType(req.DriverType)
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	product := &model.Product{
		Name:         req.Name,
		ProductType:  pType,
		DriverType:   dType,
		DriverParams: req.DriverParams,
	}

	if err := h.svc.CreateProduct(r.Context(), tenantID, scenarioID, product); err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusCreated, dto.ProductFromModel(*product))
}

// Get retrieves a product by ID.
func (h *ProductHandler) Get(w http.ResponseWriter, r *http.Request) {
	productID, err := parseUUIDParam(chi.URLParam(r, "productId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	product, err := h.svc.GetProduct(r.Context(), tenantID, productID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.ProductFromModel(*product))
}

// UpdateProductRequest represents a product update request.
type UpdateProductRequest struct {
	Name         string          `json:"name"`
	DriverType   string          `json:"driverType"`
	DriverParams json.RawMessage `json:"driverParams"`
}

// Update updates a product.
func (h *ProductHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req UpdateProductRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	productID, err := parseUUIDParam(chi.URLParam(r, "productId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	// Pass the driver type as-is; an empty string means "don't change the
	// existing value" — the service layer guards against overwriting with empty.
	dType := model.DriverType(req.DriverType)

	tenantID := ctxutil.GetTenantID(r.Context())
	product := &model.Product{
		Name:         req.Name,
		DriverType:   dType,
		DriverParams: req.DriverParams,
	}

	if err := h.svc.UpdateProduct(r.Context(), tenantID, productID, product); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// Delete removes a product.
func (h *ProductHandler) Delete(w http.ResponseWriter, r *http.Request) {
	productID, err := parseUUIDParam(chi.URLParam(r, "productId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.DeleteProduct(r.Context(), tenantID, productID); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// GetAssumptions retrieves product assumptions.
func (h *ProductHandler) GetAssumptions(w http.ResponseWriter, r *http.Request) {
	productID, err := parseUUIDParam(chi.URLParam(r, "productId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	assumptions, err := h.svc.GetAssumptions(r.Context(), tenantID, productID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.AssumptionsFromModels(assumptions))
}

// UpdateAssumptions updates product assumptions.
func (h *ProductHandler) UpdateAssumptions(w http.ResponseWriter, r *http.Request) {
	productID, err := parseUUIDParam(chi.URLParam(r, "productId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	var assumptions []model.ProductAssumption
	if err := decodeAndValidate(r, &assumptions); err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateAssumptions(r.Context(), tenantID, scenarioID, productID, assumptions); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// GetVolumes retrieves product sales volumes.
func (h *ProductHandler) GetVolumes(w http.ResponseWriter, r *http.Request) {
	productID, err := parseUUIDParam(chi.URLParam(r, "productId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	volumes, err := h.svc.GetVolumes(r.Context(), tenantID, productID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.VolumesFromModels(volumes))
}

// UpdateVolumes updates product sales volumes.
func (h *ProductHandler) UpdateVolumes(w http.ResponseWriter, r *http.Request) {
	productID, err := parseUUIDParam(chi.URLParam(r, "productId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	var volumes []model.ProductSalesVolume
	if err := decodeAndValidate(r, &volumes); err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateVolumes(r.Context(), tenantID, scenarioID, productID, volumes); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// GetMargins retrieves distributor margins.
func (h *ProductHandler) GetMargins(w http.ResponseWriter, r *http.Request) {
	productID, err := parseUUIDParam(chi.URLParam(r, "productId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	margins, err := h.svc.GetMargins(r.Context(), tenantID, productID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.MarginsFromModels(margins))
}

// UpdateMargins updates distributor margins.
func (h *ProductHandler) UpdateMargins(w http.ResponseWriter, r *http.Request) {
	productID, err := parseUUIDParam(chi.URLParam(r, "productId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	var margins []model.ProductDistributorMargin
	if err := decodeAndValidate(r, &margins); err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateMargins(r.Context(), tenantID, scenarioID, productID, margins); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// GetDerivedBundle returns the driver-computed volumes and assumptions for a
// typed-driver product.  For generic products the stored rows are returned
// unchanged so the frontend can always use this endpoint uniformly.
func (h *ProductHandler) GetDerivedBundle(w http.ResponseWriter, r *http.Request) {
	productID, err := parseUUIDParam(chi.URLParam(r, "productId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	result, err := h.svc.GetDerivedBundle(r.Context(), tenantID, productID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// GetRevenueByProduct returns the revenue summary for a single product.
func (h *ProductHandler) GetRevenueByProduct(w http.ResponseWriter, r *http.Request) {
	productID, err := parseUUIDParam(chi.URLParam(r, "productId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	summary, err := h.svc.GetRevenueByProduct(r.Context(), tenantID, scenarioID, productID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, summary)
}

// GetConsolidatedRevenue returns consolidated revenue metrics.
func (h *ProductHandler) GetConsolidatedRevenue(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	revenue, err := h.svc.GetConsolidatedRevenue(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, revenue)
}
