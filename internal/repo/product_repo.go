package repo

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"ascenda/internal/model"
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

// DeleteProduct deletes a product and all its child rows (assumptions, volumes, margins)
// in a single transaction so the compute engine never sees orphaned data.
func (r *ProductRepo) DeleteProduct(tenantID, productID uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tenant_id = ? AND product_id = ?", tenantID, productID).
			Delete(&model.ProductAssumption{}).Error; err != nil {
			return err
		}
		if err := tx.Where("tenant_id = ? AND product_id = ?", tenantID, productID).
			Delete(&model.ProductSalesVolume{}).Error; err != nil {
			return err
		}
		if err := tx.Where("tenant_id = ? AND product_id = ?", tenantID, productID).
			Delete(&model.ProductDistributorMargin{}).Error; err != nil {
			return err
		}
		return tx.Where("tenant_id = ? AND id = ?", tenantID, productID).
			Delete(&model.Product{}).Error
	})
}

// BatchUpsertAssumptions replaces all assumptions for the product in a single transaction.
// The frontend always sends the full set, so delete-then-insert is correct and idempotent.
func (r *ProductRepo) BatchUpsertAssumptions(tenantID, scenarioID uuid.UUID, assumptions []model.ProductAssumption) error {
	if len(assumptions) == 0 {
		return nil
	}
	productID := assumptions[0].ProductID
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tenant_id = ? AND product_id = ?", tenantID, productID).
			Delete(&model.ProductAssumption{}).Error; err != nil {
			return err
		}
		return tx.Create(&assumptions).Error
	})
}

// GetAssumptionsByProduct retrieves all assumptions for a product
func (r *ProductRepo) GetAssumptionsByProduct(tenantID, productID uuid.UUID) ([]*model.ProductAssumption, error) {
	var assumptions []*model.ProductAssumption
	err := r.db.Where("tenant_id = ? AND product_id = ?", tenantID, productID).
		Order("year").
		Find(&assumptions).Error
	return assumptions, err
}

// BatchUpsertVolumes replaces all volumes for the product in a single transaction.
func (r *ProductRepo) BatchUpsertVolumes(tenantID, scenarioID uuid.UUID, volumes []model.ProductSalesVolume) error {
	if len(volumes) == 0 {
		return nil
	}
	productID := volumes[0].ProductID
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tenant_id = ? AND product_id = ?", tenantID, productID).
			Delete(&model.ProductSalesVolume{}).Error; err != nil {
			return err
		}
		return tx.Create(&volumes).Error
	})
}

// GetVolumesByProduct retrieves all volumes for a product
func (r *ProductRepo) GetVolumesByProduct(tenantID, productID uuid.UUID) ([]*model.ProductSalesVolume, error) {
	var volumes []*model.ProductSalesVolume
	err := r.db.Where("tenant_id = ? AND product_id = ?", tenantID, productID).
		Order("year, zone, channel").
		Find(&volumes).Error
	return volumes, err
}

// BatchUpsertMargins replaces all margins for the product in a single transaction.
func (r *ProductRepo) BatchUpsertMargins(tenantID, scenarioID uuid.UUID, margins []model.ProductDistributorMargin) error {
	if len(margins) == 0 {
		return nil
	}
	productID := margins[0].ProductID
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tenant_id = ? AND product_id = ?", tenantID, productID).
			Delete(&model.ProductDistributorMargin{}).Error; err != nil {
			return err
		}
		return tx.Create(&margins).Error
	})
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
		Order("product_id, year, zone, channel").
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
