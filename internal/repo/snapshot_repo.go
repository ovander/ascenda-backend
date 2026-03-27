package repo

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"ascenda/internal/model"
)

// SnapshotRepo handles plan snapshot data operations
type SnapshotRepo struct {
	db *gorm.DB
}

// NewSnapshotRepo creates a new SnapshotRepo
func NewSnapshotRepo(db *gorm.DB) *SnapshotRepo {
	return &SnapshotRepo{db: db}
}

// Create creates a new plan snapshot
func (r *SnapshotRepo) Create(snapshot *model.PlanSnapshot) error {
	return r.db.Create(snapshot).Error
}

// GetByID retrieves a snapshot by ID with tenant filtering
func (r *SnapshotRepo) GetByID(tenantID, snapshotID uuid.UUID) (*model.PlanSnapshot, error) {
	var snapshot model.PlanSnapshot
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, snapshotID).First(&snapshot).Error
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

// ListByScenario retrieves all snapshots for a scenario
func (r *SnapshotRepo) ListByScenario(tenantID, scenarioID uuid.UUID, offset, limit int) ([]*model.PlanSnapshot, error) {
	var snapshots []*model.PlanSnapshot
	err := r.db.Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&snapshots).Error
	return snapshots, err
}

// ListByPlan retrieves all snapshots for a plan
func (r *SnapshotRepo) ListByPlan(tenantID, planID uuid.UUID, offset, limit int) ([]*model.PlanSnapshot, error) {
	var snapshots []*model.PlanSnapshot
	err := r.db.Where("tenant_id = ? AND plan_id = ?", tenantID, planID).
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&snapshots).Error
	return snapshots, err
}

// Delete deletes a snapshot.
// Returns gorm.ErrRecordNotFound when no row matching both tenant_id and id is
// found so the handler can return HTTP 404.
func (r *SnapshotRepo) Delete(tenantID, snapshotID uuid.UUID) error {
	result := r.db.Where("tenant_id = ? AND id = ?", tenantID, snapshotID).Delete(&model.PlanSnapshot{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
