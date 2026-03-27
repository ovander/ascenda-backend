package repo

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"ascenda/internal/model"
)

// TenantRepo handles tenant data operations
type TenantRepo struct {
	db *gorm.DB
}

// NewTenantRepo creates a new TenantRepo
func NewTenantRepo(db *gorm.DB) *TenantRepo {
	return &TenantRepo{db: db}
}

// Create creates a new tenant
func (r *TenantRepo) Create(tenant *model.Tenant) error {
	return r.db.Create(tenant).Error
}

// GetByID retrieves a tenant by ID
func (r *TenantRepo) GetByID(id uuid.UUID) (*model.Tenant, error) {
	var tenant model.Tenant
	err := r.db.Where("id = ?", id).First(&tenant).Error
	if err != nil {
		return nil, err
	}
	return &tenant, nil
}

// GetBySlug retrieves a tenant by slug
func (r *TenantRepo) GetBySlug(slug string) (*model.Tenant, error) {
	var tenant model.Tenant
	err := r.db.Where("slug = ?", slug).First(&tenant).Error
	if err != nil {
		return nil, err
	}
	return &tenant, nil
}

// ListActive retrieves all active tenants with pagination
func (r *TenantRepo) ListActive(offset, limit int) ([]*model.Tenant, error) {
	var tenants []*model.Tenant
	err := r.db.Where("is_active = ?", true).
		Offset(offset).
		Limit(limit).
		Find(&tenants).Error
	return tenants, err
}

// ListAll retrieves all tenants (including inactive) with pagination, ordered by creation date.
func (r *TenantRepo) ListAll(offset, limit int) ([]*model.Tenant, error) {
	var tenants []*model.Tenant
	err := r.db.Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&tenants).Error
	return tenants, err
}

// CountAll returns the total number of tenants.
func (r *TenantRepo) CountAll() (int64, error) {
	var count int64
	err := r.db.Model(&model.Tenant{}).Count(&count).Error
	return count, err
}

// Update updates a tenant
func (r *TenantRepo) Update(tenant *model.Tenant) error {
	return r.db.Save(tenant).Error
}

// Delete soft-deletes a tenant by marking it inactive
func (r *TenantRepo) Delete(id uuid.UUID) error {
	return r.db.Model(&model.Tenant{}).Where("id = ?", id).Update("is_active", false).Error
}
