package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"kerplan/internal/event"
	"kerplan/internal/model"
	"kerplan/internal/pkg/apierror"
	"kerplan/internal/repo"
)

// ProductService orchestrates product CRUD and computation.
type ProductService struct {
	productRepo   repo.ProductRepository
	reportService *ReportService
	emitter       *event.Emitter
	logger        *logrus.Entry
}

// NewProductService creates a new ProductService.
func NewProductService(productRepo repo.ProductRepository, reportService *ReportService, emitter *event.Emitter, logger *logrus.Entry) *ProductService {
	return &ProductService{
		productRepo:   productRepo,
		reportService: reportService,
		emitter:       emitter,
		logger:        logger,
	}
}

// ListProducts lists all products for a scenario.
func (s *ProductService) ListProducts(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]*model.Product, error) {
	products, err := s.productRepo.ListProductsByScenario(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to list products")
		return nil, apierror.Internal("failed to list products")
	}

	return products, nil
}

// CreateProduct creates a new product.
func (s *ProductService) CreateProduct(ctx context.Context, tenantID, scenarioID uuid.UUID, product *model.Product) error {
	product.ID = uuid.New()
	product.TenantID = tenantID
	product.ScenarioID = scenarioID

	if err := s.productRepo.CreateProduct(product); err != nil {
		s.logger.WithError(err).Error("failed to create product")
		return apierror.Internal("failed to create product")
	}

	s.logger.WithField("product_id", product.ID).Info("product created")
	s.emitter.Publish(event.Event{Type: event.DataChanged, TenantID: tenantID, ScenarioID: scenarioID, EntityType: "product", EntityID: product.ID, Action: event.ActionCreate})
	return nil
}

// GetProduct retrieves a product by ID.
func (s *ProductService) GetProduct(ctx context.Context, tenantID, productID uuid.UUID) (*model.Product, error) {
	product, err := s.productRepo.GetByID(tenantID, productID)
	if err != nil || product == nil || product.TenantID != tenantID {
		s.logger.WithError(err).Warn("product not found")
		return nil, apierror.NotFound("product", productID.String())
	}

	return product, nil
}

// UpdateProduct updates a product.
func (s *ProductService) UpdateProduct(ctx context.Context, tenantID, productID uuid.UUID, product *model.Product) error {
	existing, err := s.GetProduct(ctx, tenantID, productID)
	if err != nil {
		return err
	}

	existing.Name = product.Name
	existing.SortOrder = product.SortOrder
	existing.DirectCostVariability = product.DirectCostVariability
	existing.ExternalChargeVariability = product.ExternalChargeVariability
	existing.TaxVariability = product.TaxVariability
	existing.StaffVariability = product.StaffVariability
	existing.DepreciationVariability = product.DepreciationVariability

	if err := s.productRepo.UpdateProduct(existing); err != nil {
		s.logger.WithError(err).Error("failed to update product")
		return apierror.Internal("failed to update product")
	}

	s.logger.WithField("product_id", productID).Info("product updated")
	s.emitter.Publish(event.Event{Type: event.DataChanged, TenantID: tenantID, EntityType: "product", EntityID: productID, Action: event.ActionUpdate})
	return nil
}

// DeleteProduct removes a product.
func (s *ProductService) DeleteProduct(ctx context.Context, tenantID, productID uuid.UUID) error {
	if _, err := s.GetProduct(ctx, tenantID, productID); err != nil {
		return err
	}

	if err := s.productRepo.DeleteProduct(tenantID, productID); err != nil {
		s.logger.WithError(err).Error("failed to delete product")
		return apierror.Internal("failed to delete product")
	}

	s.logger.WithField("product_id", productID).Info("product deleted")
	s.emitter.Publish(event.Event{Type: event.DataChanged, TenantID: tenantID, EntityType: "product", EntityID: productID, Action: event.ActionDelete})
	return nil
}

// GetAssumptions retrieves assumptions for a product.
func (s *ProductService) GetAssumptions(ctx context.Context, tenantID, productID uuid.UUID) ([]model.ProductAssumption, error) {
	ptrAssumptions, err := s.productRepo.GetAssumptionsByProduct(tenantID, productID)
	if err != nil {
		s.logger.WithError(err).Error("failed to get assumptions")
		return nil, apierror.Internal("failed to get assumptions")
	}

	assumptions := make([]model.ProductAssumption, len(ptrAssumptions))
	for i, a := range ptrAssumptions {
		assumptions[i] = *a
	}
	return assumptions, nil
}

// UpdateAssumptions updates or creates product assumptions.
func (s *ProductService) UpdateAssumptions(ctx context.Context, tenantID, scenarioID, productID uuid.UUID, assumptions []model.ProductAssumption) error {
	for i := range assumptions {
		if assumptions[i].ID == uuid.Nil {
			assumptions[i].ID = uuid.New()
		}
		assumptions[i].TenantID = tenantID
		assumptions[i].ProductID = productID
	}

	if err := s.productRepo.BatchUpsertAssumptions(tenantID, scenarioID, assumptions); err != nil {
		s.logger.WithError(err).Error("failed to update assumptions")
		return apierror.Internal("failed to update assumptions")
	}

	s.logger.WithField("product_id", productID).Info("assumptions updated")
	s.emitter.Publish(event.Event{Type: event.DataChanged, TenantID: tenantID, ScenarioID: scenarioID, EntityType: "product_assumptions", EntityID: productID, Action: event.ActionUpdate})
	return nil
}

// GetVolumes retrieves sales volumes for a product.
func (s *ProductService) GetVolumes(ctx context.Context, tenantID, productID uuid.UUID) ([]model.ProductSalesVolume, error) {
	ptrVolumes, err := s.productRepo.GetVolumesByProduct(tenantID, productID)
	if err != nil {
		s.logger.WithError(err).Error("failed to get volumes")
		return nil, apierror.Internal("failed to get volumes")
	}

	volumes := make([]model.ProductSalesVolume, len(ptrVolumes))
	for i, v := range ptrVolumes {
		volumes[i] = *v
	}
	return volumes, nil
}

// UpdateVolumes updates or creates sales volumes.
func (s *ProductService) UpdateVolumes(ctx context.Context, tenantID, scenarioID, productID uuid.UUID, volumes []model.ProductSalesVolume) error {
	for i := range volumes {
		if volumes[i].ID == uuid.Nil {
			volumes[i].ID = uuid.New()
		}
		volumes[i].TenantID = tenantID
		volumes[i].ProductID = productID
	}

	if err := s.productRepo.BatchUpsertVolumes(tenantID, scenarioID, volumes); err != nil {
		s.logger.WithError(err).Error("failed to update volumes")
		return apierror.Internal("failed to update volumes")
	}

	s.logger.WithField("product_id", productID).Info("volumes updated")
	s.emitter.Publish(event.Event{Type: event.DataChanged, TenantID: tenantID, ScenarioID: scenarioID, EntityType: "product_volumes", EntityID: productID, Action: event.ActionUpdate})
	return nil
}

// GetMargins retrieves distributor margins for a product.
func (s *ProductService) GetMargins(ctx context.Context, tenantID, productID uuid.UUID) ([]model.ProductDistributorMargin, error) {
	ptrMargins, err := s.productRepo.GetMarginsByProduct(tenantID, productID)
	if err != nil {
		s.logger.WithError(err).Error("failed to get margins")
		return nil, apierror.Internal("failed to get margins")
	}

	margins := make([]model.ProductDistributorMargin, len(ptrMargins))
	for i, m := range ptrMargins {
		margins[i] = *m
	}
	return margins, nil
}

// UpdateMargins updates or creates distributor margins.
func (s *ProductService) UpdateMargins(ctx context.Context, tenantID, scenarioID, productID uuid.UUID, margins []model.ProductDistributorMargin) error {
	for i := range margins {
		if margins[i].ID == uuid.Nil {
			margins[i].ID = uuid.New()
		}
		margins[i].TenantID = tenantID
		margins[i].ProductID = productID
	}

	if err := s.productRepo.BatchUpsertMargins(tenantID, scenarioID, margins); err != nil {
		s.logger.WithError(err).Error("failed to update margins")
		return apierror.Internal("failed to update margins")
	}

	s.logger.WithField("product_id", productID).Info("margins updated")
	s.emitter.Publish(event.Event{Type: event.DataChanged, TenantID: tenantID, ScenarioID: scenarioID, EntityType: "product_margins", EntityID: productID, Action: event.ActionUpdate})
	return nil
}

// GetConsolidatedRevenue delegates to ReportService for revenue computation.
func (s *ProductService) GetConsolidatedRevenue(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.ConsolidatedRevenue, error) {
	fullReport, err := s.reportService.GetFullReport(ctx, tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to compute consolidated revenue")
		return nil, apierror.Internal("failed to compute consolidated revenue")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("consolidated revenue computed")
	return &fullReport.Revenue, nil
}
