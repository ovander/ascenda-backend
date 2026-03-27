package repo

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"ascenda/internal/model"
)

// AuditRepo handles audit log data operations
type AuditRepo struct {
	db *gorm.DB
}

// NewAuditRepo creates a new AuditRepo
func NewAuditRepo(db *gorm.DB) *AuditRepo {
	return &AuditRepo{db: db}
}

// Create creates a new audit log entry
func (r *AuditRepo) Create(auditLog *model.AuditLog) error {
	return r.db.Create(auditLog).Error
}

// ListByEntity retrieves audit logs for a specific entity
func (r *AuditRepo) ListByEntity(tenantID uuid.UUID, entityType string, entityID uuid.UUID) ([]*model.AuditLog, error) {
	var logs []*model.AuditLog
	err := r.db.Where("tenant_id = ? AND entity_type = ? AND entity_id = ?", tenantID, entityType, entityID).
		Order("created_at DESC").
		Find(&logs).Error
	return logs, err
}

// ListByTenant retrieves all audit logs for a tenant with pagination
func (r *AuditRepo) ListByTenant(tenantID uuid.UUID, offset, limit int) ([]*model.AuditLog, error) {
	var logs []*model.AuditLog
	err := r.db.Where("tenant_id = ?", tenantID).
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&logs).Error
	return logs, err
}

// CountByTenant returns the total number of audit log entries for a tenant.
func (r *AuditRepo) CountByTenant(tenantID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&model.AuditLog{}).
		Where("tenant_id = ?", tenantID).
		Count(&count).Error
	return count, err
}

// GetByID retrieves a single audit log entry by its primary key within a tenant.
func (r *AuditRepo) GetByID(tenantID, entryID uuid.UUID) (*model.AuditLog, error) {
	var log model.AuditLog
	err := r.db.Where("id = ? AND tenant_id = ?", entryID, tenantID).First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// ListByEntityID retrieves all audit logs for a given entity_id regardless of entity_type.
// Used to build the full audit trail for a scenario (where entity_id = scenario UUID).
func (r *AuditRepo) ListByEntityID(tenantID, entityID uuid.UUID) ([]*model.AuditLog, error) {
	var logs []*model.AuditLog
	err := r.db.Where("tenant_id = ? AND entity_id = ?", tenantID, entityID).
		Order("created_at ASC").
		Find(&logs).Error
	return logs, err
}

// ListByUser retrieves all audit logs created by a specific user
func (r *AuditRepo) ListByUser(tenantID, userID uuid.UUID, offset, limit int) ([]*model.AuditLog, error) {
	var logs []*model.AuditLog
	err := r.db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&logs).Error
	return logs, err
}
