package repo

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"kerplan/internal/model"
)

type UserRepo struct {
	db *gorm.DB
}

func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(user *model.User) error {
	return r.db.Create(user).Error
}

func (r *UserRepo) GetByID(tenantID, userID uuid.UUID) (*model.User, error) {
	var user model.User
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, userID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) GetByExternalID(externalID string) (*model.User, error) {
	var user model.User
	err := r.db.Where("external_id = ?", externalID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) GetByEmail(tenantID uuid.UUID, email string) (*model.User, error) {
	var user model.User
	err := r.db.Where("tenant_id = ? AND email = ?", tenantID, email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) ListByTenant(tenantID uuid.UUID, offset, limit int) ([]*model.User, int64, error) {
	var users []*model.User
	var total int64
	r.db.Model(&model.User{}).Where("tenant_id = ?", tenantID).Count(&total)
	err := r.db.Where("tenant_id = ?", tenantID).
		Order("created_at ASC").
		Offset(offset).Limit(limit).
		Find(&users).Error
	return users, total, err
}

func (r *UserRepo) Update(user *model.User) error {
	return r.db.Save(user).Error
}

func (r *UserRepo) CountByTenant(tenantID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&model.User{}).Where("tenant_id = ? AND is_active = ?", tenantID, true).Count(&count).Error
	return count, err
}
