package model

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// PlanShareholder is the simplified, plan-level shareholder record for the
// Pro-tier cap table dashboard. Unlike the full scenario-level cap table
// (CapTableShareholder), this model is scoped to a plan rather than a scenario
// and tracks only the headline equity data needed for the ownership summary.
type PlanShareholder struct {
	TenantScoped
	PlanID         uuid.UUID       `gorm:"type:uuid;not null;index" json:"planId"`
	Name           string          `gorm:"type:varchar(255);not null" json:"name"`
	Type           ShareholderType `gorm:"type:varchar(30);not null" json:"type"`
	Shares         int64           `gorm:"not null;default:0" json:"shares"`
	OwnershipPct   decimal.Decimal `gorm:"type:numeric(10,4);default:0" json:"ownershipPct"`
	InvestedAmount decimal.Decimal `gorm:"type:numeric(15,2);default:0" json:"investedAmount"`
	Notes          string          `gorm:"type:text" json:"notes,omitempty"`
}

// TableName specifies the table name for GORM.
func (PlanShareholder) TableName() string { return "plan_shareholders" }
