package repo

import (
	"ascenda/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PlanMemberRepo struct {
	db *gorm.DB
}

func NewPlanMemberRepo(db *gorm.DB) *PlanMemberRepo {
	return &PlanMemberRepo{db: db}
}

func (r *PlanMemberRepo) Create(member *model.PlanMember) error {
	return r.db.Create(member).Error
}

func (r *PlanMemberRepo) GetByPlanAndUser(tenantID, planID, userID uuid.UUID) (*model.PlanMember, error) {
	var member model.PlanMember
	err := r.db.Where("tenant_id = ? AND plan_id = ? AND user_id = ?", tenantID, planID, userID).First(&member).Error
	if err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *PlanMemberRepo) ListByPlan(tenantID, planID uuid.UUID) ([]*model.PlanMember, error) {
	var members []*model.PlanMember
	err := r.db.Where("tenant_id = ? AND plan_id = ?", tenantID, planID).Find(&members).Error
	return members, err
}

func (r *PlanMemberRepo) ListByUser(tenantID, userID uuid.UUID) ([]*model.PlanMember, error) {
	var members []*model.PlanMember
	err := r.db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).Find(&members).Error
	return members, err
}

func (r *PlanMemberRepo) Update(member *model.PlanMember) error {
	return r.db.Save(member).Error
}

func (r *PlanMemberRepo) Delete(tenantID, planID, userID uuid.UUID) error {
	result := r.db.Where("tenant_id = ? AND plan_id = ? AND user_id = ?", tenantID, planID, userID).
		Delete(&model.PlanMember{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
