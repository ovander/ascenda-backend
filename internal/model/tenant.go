package model

import (
	"time"

	"github.com/google/uuid"
)

// Tenant represents a multi-tenant organization
type Tenant struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name      string    `gorm:"type:varchar(255);not null" json:"name"`
	Slug      string    `gorm:"type:varchar(100);uniqueIndex" json:"slug"`
	IsActive  bool      `gorm:"not null;default:true" json:"isActive"`
	Tier      string    `gorm:"type:varchar(50);not null;default:'free'" json:"tier"`                   // free|starter|pro|enterprise
	MaxPlans  int       `gorm:"not null;default:3" json:"maxPlans"`
	MaxUsers  int       `gorm:"not null;default:5" json:"maxUsers"`
	AICredits int       `gorm:"not null;default:0" json:"aiCredits"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

// TableName specifies the table name for Tenant
func (Tenant) TableName() string {
	return "tenants"
}

// User represents a user within a tenant
type User struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"-"`
	ExternalID string     `gorm:"type:varchar(255)" json:"-"` // Socrate sub, empty until first login
	Email      string     `gorm:"type:varchar(255);not null" json:"email"`
	Name       string     `gorm:"type:varchar(255)" json:"name"`
	Role       string     `gorm:"type:varchar(50);not null;default:'user'" json:"role"` // owner|admin|user
	IsActive   bool       `gorm:"not null;default:true" json:"isActive"`
	InvitedBy  *uuid.UUID `gorm:"type:uuid" json:"invitedBy,omitempty"`
	JoinedAt   *time.Time `json:"joinedAt,omitempty"`
	CreatedAt  time.Time  `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt  time.Time  `gorm:"autoUpdateTime" json:"updatedAt"`
}

// TableName specifies the table name for User
func (User) TableName() string {
	return "users"
}

// PlanMember represents a user's access to a specific plan
type PlanMember struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID  uuid.UUID `gorm:"type:uuid;not null;index" json:"-"`
	PlanID    uuid.UUID `gorm:"type:uuid;not null;index" json:"planId"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index" json:"userId"`
	Role      string    `gorm:"type:varchar(50);not null" json:"role"` // editor|viewer
	GrantedBy uuid.UUID `gorm:"type:uuid;not null" json:"grantedBy"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

// TableName specifies the table name for PlanMember
func (PlanMember) TableName() string {
	return "plan_members"
}
