package repo

import (
	"ascenda/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OrganizationRepository defines all persistence operations for organizations.
type OrganizationRepository interface {
	Create(org *model.Organization) error
	GetByID(id uuid.UUID) (*model.Organization, error)
	GetBySlug(slug string) (*model.Organization, error)
	ListAll(offset, limit int) ([]*model.Organization, error)
	CountAll() (int64, error)
	Update(org *model.Organization) error
	Delete(id uuid.UUID) error
	// ListTenants returns all tenants that belong to an organization.
	ListTenants(orgID uuid.UUID) ([]*model.Tenant, error)
	// CountActiveUsers returns the total number of active users across all
	// tenants belonging to the organization. Used for license enforcement.
	CountActiveUsers(orgID uuid.UUID) (int64, error)
}

// OrgRepo is the GORM implementation of OrganizationRepository.
type OrgRepo struct {
	db *gorm.DB
}

// NewOrgRepo creates a new OrgRepo.
func NewOrgRepo(db *gorm.DB) *OrgRepo {
	return &OrgRepo{db: db}
}

func (r *OrgRepo) Create(org *model.Organization) error {
	return r.db.Create(org).Error
}

func (r *OrgRepo) GetByID(id uuid.UUID) (*model.Organization, error) {
	var org model.Organization
	if err := r.db.Where("id = ?", id).First(&org).Error; err != nil {
		return nil, err
	}
	return &org, nil
}

func (r *OrgRepo) GetBySlug(slug string) (*model.Organization, error) {
	var org model.Organization
	if err := r.db.Where("slug = ?", slug).First(&org).Error; err != nil {
		return nil, err
	}
	return &org, nil
}

func (r *OrgRepo) ListAll(offset, limit int) ([]*model.Organization, error) {
	var orgs []*model.Organization
	err := r.db.Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&orgs).Error
	return orgs, err
}

func (r *OrgRepo) CountAll() (int64, error) {
	var count int64
	err := r.db.Model(&model.Organization{}).Count(&count).Error
	return count, err
}

func (r *OrgRepo) Update(org *model.Organization) error {
	return r.db.Save(org).Error
}

// Delete soft-deletes an organization by marking it inactive.
func (r *OrgRepo) Delete(id uuid.UUID) error {
	return r.db.Model(&model.Organization{}).
		Where("id = ?", id).
		Update("is_active", false).Error
}

func (r *OrgRepo) ListTenants(orgID uuid.UUID) ([]*model.Tenant, error) {
	var tenants []*model.Tenant
	err := r.db.Where("organization_id = ?", orgID).
		Order("name ASC").
		Find(&tenants).Error
	return tenants, err
}

// CountActiveUsers counts active users across all tenants in the organization.
func (r *OrgRepo) CountActiveUsers(orgID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&model.User{}).
		Joins("JOIN tenants ON tenants.id = users.tenant_id").
		Where("tenants.organization_id = ? AND users.is_active = true", orgID).
		Count(&count).Error
	return count, err
}

// Compile-time check.
var _ OrganizationRepository = (*OrgRepo)(nil)
