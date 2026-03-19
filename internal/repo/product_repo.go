package repo

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"kerplan/internal/model"
)

// ProductRepo handles product and product-related data operations
type ProductRepo struct {
	db *gorm.DB
}

// NewProductRepo creates a new ProductRepo
func NewProductRepo(db *gorm.DB) *ProductRepo {
	return &ProductRepo{db: db}
}

// CreateProduct creates a new product
func (r *ProductRepo) CreateProduct(product *model.Product) error {
	return r.db.Create(product).Error
}

// GetByID retrieves a product by ID with tenant filtering
func (r *ProductRepo) GetByID(tenantID, productID uuid.UUID) (*model.Product, error) {
	var product model.Product
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, productID).First(&product).Error
	if err != nil {
		return nil, err
	}
	return &product, nil
}

// ListProductsByScenario retrieves all products for a scenario
func (r *ProductRepo) ListProductsByScenario(tenantID, scenarioID uuid.UUID) ([]*model.Product, error) {
	var products []*model.Product
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Order("sort_order, created_at").
		Find(&products).Error
	return products, err
}

// UpdateProduct updates a product
func (r *ProductRepo) UpdateProduct(product *model.Product) error {
	return r.db.Save(product).Error
}

// DeleteProduct deletes a product
func (r *ProductRepo) DeleteProduct(tenantID, productID uuid.UUID) error {
	return r.db.Where("tenant_id = ? AND id = ?", tenantID, productID).Delete(&model.Product{}).Error
}

// BatchUpsertAssumptions creates or updates product assumptions
func (r *ProductRepo) BatchUpsertAssumptions(tenantID, scenarioID uuid.UUID, assumptions []model.ProductAssumption) error {
	for i := range assumptions {
		assumptions[i].TenantID = tenantID
	}
	return r.db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&assumptions).Error
}

// GetAssumptionsByProduct retrieves all assumptions for a product
func (r *ProductRepo) GetAssumptionsByProduct(tenantID, productID uuid.UUID) ([]*model.ProductAssumption, error) {
	var assumptions []*model.ProductAssumption
	err := r.db.Where("tenant_id = ? AND product_id = ?", tenantID, productID).
		Order("year").
		Find(&assumptions).Error
	return assumptions, err
}

// BatchUpsertVolumes creates or updates sales volumes
func (r *ProductRepo) BatchUpsertVolumes(tenantID, scenarioID uuid.UUID, volumes []model.ProductSalesVolume) error {
	for i := range volumes {
		volumes[i].TenantID = tenantID
	}
	return r.db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&volumes).Error
}

// GetVolumesByProduct retrieves all volumes for a product
func (r *ProductRepo) GetVolumesByProduct(tenantID, productID uuid.UUID) ([]*model.ProductSalesVolume, error) {
	var volumes []*model.ProductSalesVolume
	err := r.db.Where("tenant_id = ? AND product_id = ?", tenantID, productID).
		Order("year, zone_index").
		Find(&volumes).Error
	return volumes, err
}

// BatchUpsertMargins creates or updates distributor margins
func (r *ProductRepo) BatchUpsertMargins(tenantID, scenarioID uuid.UUID, margins []model.ProductDistributorMargin) error {
	for i := range margins {
		margins[i].TenantID = tenantID
	}
	return r.db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&margins).Error
}

// GetMarginsByProduct retrieves all margins for a product
func (r *ProductRepo) GetMarginsByProduct(tenantID, productID uuid.UUID) ([]*model.ProductDistributorMargin, error) {
	var margins []*model.ProductDistributorMargin
	err := r.db.Where("tenant_id = ? AND product_id = ?", tenantID, productID).
		Order("year").
		Find(&margins).Error
	return margins, err
}

// GetAssumptionsByScenario retrieves all assumptions for all products in a scenario (batch).
func (r *ProductRepo) GetAssumptionsByScenario(tenantID, scenarioID uuid.UUID) ([]*model.ProductAssumption, error) {
	var assumptions []*model.ProductAssumption
	err := r.db.Where("tenant_id = ? AND product_id IN (SELECT id FROM products WHERE scenario_id = ?)", tenantID, scenarioID).
		Order("product_id, year").
		Find(&assumptions).Error
	return assumptions, err
}

// GetVolumesByScenario retrieves all volumes for all products in a scenario (batch).
func (r *ProductRepo) GetVolumesByScenario(tenantID, scenarioID uuid.UUID) ([]*model.ProductSalesVolume, error) {
	var volumes []*model.ProductSalesVolume
	err := r.db.Where("tenant_id = ? AND product_id IN (SELECT id FROM products WHERE scenario_id = ?)", tenantID, scenarioID).
		Order("product_id, year, zone_index").
		Find(&volumes).Error
	return volumes, err
}

// GetMarginsByScenario retrieves all margins for all products in a scenario (batch).
func (r *ProductRepo) GetMarginsByScenario(tenantID, scenarioID uuid.UUID) ([]*model.ProductDistributorMargin, error) {
	var margins []*model.ProductDistributorMargin
	err := r.db.Where("tenant_id = ? AND product_id IN (SELECT id FROM products WHERE scenario_id = ?)", tenantID, scenarioID).
		Order("product_id, year").
		Find(&margins).Error
	return margins, err
}
