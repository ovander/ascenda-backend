package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func init() {
	// Configure shopspring/decimal to marshal as JSON number (not string).
	decimal.MarshalJSONWithoutQuotes = true
}

// TenantScoped is embedded in every data model for multi-tenancy.
type TenantScoped struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID  uuid.UUID `gorm:"type:uuid;not null;index" json:"-"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

// BeforeCreate hook ensures UUID and tenant_id are set.
func (b *TenantScoped) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

// ValidationWarning represents a cross-check warning from the compute engine.
type ValidationWarning struct {
	Severity string `json:"severity"` // "error", "warning", "info"
	Sheet    string `json:"sheet"`    // "pnl", "bsheet", "budget1", etc.
	Row      string `json:"row"`      // Row reference
	Message  string `json:"message"`
}
