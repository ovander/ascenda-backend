package model

import (
	"github.com/google/uuid"
)

// BusinessPlan represents a business plan with multiple scenarios
type BusinessPlan struct {
	TenantScoped
	Name        string    `gorm:"type:varchar(255);not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Status      string    `gorm:"type:varchar(50);not null;default:'draft'" json:"status"` // draft, review, approved, archived
	CreatedBy   uuid.UUID `gorm:"type:uuid;not null" json:"createdBy"`
	// IsDemo marks plans that ship as built-in demos; they cannot be deleted by users.
	IsDemo bool `gorm:"not null;default:false" json:"isDemo"`
}

// TableName specifies the table name for BusinessPlan
func (BusinessPlan) TableName() string {
	return "business_plans"
}

// Scenario represents a what-if scenario within a plan
type Scenario struct {
	TenantScoped
	PlanID      uuid.UUID `gorm:"type:uuid;not null;index" json:"planId"`
	Name        string    `gorm:"type:varchar(255);not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	IsDefault   bool      `gorm:"not null;default:false" json:"isDefault"`
}

// TableName specifies the table name for Scenario
func (Scenario) TableName() string {
	return "scenarios"
}
