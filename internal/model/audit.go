package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AuditLog represents an audit trail entry for entity changes
type AuditLog struct {
	ID         uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID   uuid.UUID       `gorm:"type:uuid;not null;index" json:"tenantId"`
	UserID     uuid.UUID       `gorm:"type:uuid;not null" json:"userId"`
	EntityType string          `gorm:"type:varchar(100);not null;index" json:"entityType"`
	EntityID   uuid.UUID       `gorm:"type:uuid;not null;index" json:"entityId"`
	Action     string          `gorm:"type:varchar(50);not null" json:"action"` // create, update, delete
	Changes    json.RawMessage `gorm:"type:jsonb" json:"changes"`
	CreatedAt  time.Time       `gorm:"autoCreateTime;index" json:"createdAt"`
}

// TableName specifies the table name for AuditLog
func (AuditLog) TableName() string {
	return "audit_logs"
}
