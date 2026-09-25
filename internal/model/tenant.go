package model

import (
	"time"

	"github.com/google/uuid"
)

// TenantType distinguishes self-service workspaces from org-provisioned departments.
type TenantType string

const (
	TenantTypeWorkspace  TenantType = "workspace"  // auto-created on self-service registration
	TenantTypeEnterprise TenantType = "enterprise" // provisioned by Ascenda admin under an Organization
)

// Organization is the billing/administrative umbrella for enterprise clients.
// One org groups N tenants (departments). Workspace tenants have no org.
type Organization struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name         string    `gorm:"type:varchar(255);not null" json:"name"`
	Slug         string    `gorm:"type:varchar(100);uniqueIndex" json:"slug"`
	Plan         string    `gorm:"type:varchar(50);not null;default:'enterprise'" json:"plan"` // freemium|pro|enterprise
	MaxUsers     int       `gorm:"not null;default:0" json:"maxUsers"`                         // 0 = unlimited
	BillingEmail string    `gorm:"type:varchar(255)" json:"billingEmail,omitempty"`
	Domain       string    `gorm:"type:varchar(255)" json:"domain,omitempty"` // future: auto-join by email domain
	IsActive     bool      `gorm:"not null;default:true" json:"isActive"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

// TableName specifies the table name for Organization.
func (Organization) TableName() string {
	return "organizations"
}

// Tenant represents a multi-tenant organization (workspace) or an enterprise department.
type Tenant struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrganizationID *uuid.UUID `gorm:"type:uuid;index" json:"organizationId,omitempty"` // nil for workspace tenants
	Type           TenantType `gorm:"type:varchar(50);not null;default:'workspace'" json:"type"`
	Plan           string     `gorm:"type:varchar(50);not null;default:'freemium'" json:"plan"` // effective plan; governs tier gating for enterprise tenants
	Name           string     `gorm:"type:varchar(255);not null" json:"name"`
	Slug           string     `gorm:"type:varchar(100);uniqueIndex" json:"slug"`
	IsActive       bool       `gorm:"not null;default:true" json:"isActive"`
	AICredits      int        `gorm:"not null;default:0" json:"aiCredits"`
	CreatedAt      time.Time  `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt      time.Time  `gorm:"autoUpdateTime" json:"updatedAt"`
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
	Role       string     `gorm:"type:varchar(50);not null;default:'user'" json:"role"`     // owner|user (Ascenda tenant role)
	Plan       string     `gorm:"type:varchar(50);not null;default:'freemium'" json:"plan"` // freemium|pro|enterprise (workspace plan; ignored for enterprise tenant members — tenant.Plan governs)
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
