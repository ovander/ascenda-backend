package repo

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"ascenda/internal/model"
)

// PlanShareholderRepository defines the interface for plan-level shareholder data.
type PlanShareholderRepository interface {
	ListByPlan(tenantID, planID uuid.UUID) ([]*model.PlanShareholder, error)
	GetByID(tenantID, id uuid.UUID) (*model.PlanShareholder, error)
	Create(sh *model.PlanShareholder) error
	Update(sh *model.PlanShareholder) error
	Delete(tenantID, id uuid.UUID) error
}

// PlanShareholderRepo is the GORM implementation.
type PlanShareholderRepo struct {
	db *gorm.DB
}

// NewPlanShareholderRepo creates a new PlanShareholderRepo.
func NewPlanShareholderRepo(db *gorm.DB) *PlanShareholderRepo {
	return &PlanShareholderRepo{db: db}
}

// Compile-time interface check.
var _ PlanShareholderRepository = (*PlanShareholderRepo)(nil)

func (r *PlanShareholderRepo) ListByPlan(tenantID, planID uuid.UUID) ([]*model.PlanShareholder, error) {
	var list []*model.PlanShareholder
	err := r.db.Where("tenant_id = ? AND plan_id = ?", tenantID, planID).
		Order("created_at asc").
		Find(&list).Error
	return list, err
}

func (r *PlanShareholderRepo) GetByID(tenantID, id uuid.UUID) (*model.PlanShareholder, error) {
	var sh model.PlanShareholder
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&sh).Error
	if err != nil {
		return nil, err
	}
	return &sh, nil
}

func (r *PlanShareholderRepo) Create(sh *model.PlanShareholder) error {
	return r.db.Create(sh).Error
}

func (r *PlanShareholderRepo) Update(sh *model.PlanShareholder) error {
	return r.db.Save(sh).Error
}

func (r *PlanShareholderRepo) Delete(tenantID, id uuid.UUID) error {
	result := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&model.PlanShareholder{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
