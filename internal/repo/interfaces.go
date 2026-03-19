package repo

import (
	"github.com/google/uuid"
	"kerplan/internal/model"
)

// PlanRepository defines the interface for business plan data operations
type PlanRepository interface {
	Create(plan *model.BusinessPlan) error
	GetByID(tenantID, planID uuid.UUID) (*model.BusinessPlan, error)
	ListByTenant(tenantID uuid.UUID, offset, limit int) ([]*model.BusinessPlan, error)
	Update(plan *model.BusinessPlan) error
	Delete(tenantID, planID uuid.UUID) error
}

// ScenarioRepository defines the interface for scenario data operations
type ScenarioRepository interface {
	Create(scenario *model.Scenario) error
	GetByID(tenantID, scenarioID uuid.UUID) (*model.Scenario, error)
	ListByPlan(tenantID, planID uuid.UUID) ([]*model.Scenario, error)
	Update(scenario *model.Scenario) error
	Delete(tenantID, scenarioID uuid.UUID) error
}

// TenantRepository defines the interface for tenant data operations
type TenantRepository interface {
	Create(tenant *model.Tenant) error
	GetByID(id uuid.UUID) (*model.Tenant, error)
	GetBySlug(slug string) (*model.Tenant, error)
	ListActive(offset, limit int) ([]*model.Tenant, error)
	Update(tenant *model.Tenant) error
	Delete(id uuid.UUID) error
}

// UserRepository defines the interface for user data operations
type UserRepository interface {
	Create(user *model.User) error
	GetByID(tenantID, userID uuid.UUID) (*model.User, error)
	GetByExternalID(externalID string) (*model.User, error)
	GetByEmail(tenantID uuid.UUID, email string) (*model.User, error)
	ListByTenant(tenantID uuid.UUID, offset, limit int) ([]*model.User, int64, error)
	Update(user *model.User) error
	CountByTenant(tenantID uuid.UUID) (int64, error)
}

// SettingsRepository defines the interface for settings data operations
type SettingsRepository interface {
	GetConfig(tenantID, scenarioID uuid.UUID) (*model.PlanConfig, error)
	UpsertConfig(config *model.PlanConfig) error
	GetOpeningBalance(tenantID, scenarioID uuid.UUID) (*model.OpeningBalance, error)
	UpsertOpeningBalance(balance *model.OpeningBalance) error
	GetWCConfig(tenantID, scenarioID uuid.UUID) (*model.WorkingCapitalConfig, error)
	UpsertWCConfig(config *model.WorkingCapitalConfig) error
	GetOpexPerHire(tenantID, scenarioID uuid.UUID) (*model.OpexPerHire, error)
	UpsertOpexPerHire(opexPerHire *model.OpexPerHire) error
	GetCapexPerHire(tenantID, scenarioID uuid.UUID) (*model.CapexPerHire, error)
	UpsertCapexPerHire(capexPerHire *model.CapexPerHire) error
	ListMultiYearAdjustments(tenantID, scenarioID uuid.UUID) ([]*model.MultiYearAdjustment, error)
	BatchUpsertMultiYearAdjustments(tenantID, scenarioID uuid.UUID, adjustments []model.MultiYearAdjustment) error
}

// ProductRepository defines the interface for product data operations
type ProductRepository interface {
	CreateProduct(product *model.Product) error
	GetByID(tenantID, productID uuid.UUID) (*model.Product, error)
	ListProductsByScenario(tenantID, scenarioID uuid.UUID) ([]*model.Product, error)
	UpdateProduct(product *model.Product) error
	DeleteProduct(tenantID, productID uuid.UUID) error
	BatchUpsertAssumptions(tenantID, scenarioID uuid.UUID, assumptions []model.ProductAssumption) error
	GetAssumptionsByProduct(tenantID, productID uuid.UUID) ([]*model.ProductAssumption, error)
	BatchUpsertVolumes(tenantID, scenarioID uuid.UUID, volumes []model.ProductSalesVolume) error
	GetVolumesByProduct(tenantID, productID uuid.UUID) ([]*model.ProductSalesVolume, error)
	BatchUpsertMargins(tenantID, scenarioID uuid.UUID, margins []model.ProductDistributorMargin) error
	GetMarginsByProduct(tenantID, productID uuid.UUID) ([]*model.ProductDistributorMargin, error)
	// Batch-fetch methods (scenario-level, eliminate N+1)
	GetAssumptionsByScenario(tenantID, scenarioID uuid.UUID) ([]*model.ProductAssumption, error)
	GetVolumesByScenario(tenantID, scenarioID uuid.UUID) ([]*model.ProductSalesVolume, error)
	GetMarginsByScenario(tenantID, scenarioID uuid.UUID) ([]*model.ProductDistributorMargin, error)
}

// StaffRepository defines the interface for staff data operations
type StaffRepository interface {
	ListHeadcountsByScenario(tenantID, scenarioID uuid.UUID) ([]*model.StaffHeadcount, error)
	BatchUpsertHeadcounts(tenantID, scenarioID uuid.UUID, headcounts []model.StaffHeadcount) error
	ListSalariesByScenario(tenantID, scenarioID uuid.UUID) ([]*model.StaffSalary, error)
	BatchUpsertSalaries(tenantID, scenarioID uuid.UUID, salaries []model.StaffSalary) error
	ListIncentivesByScenario(tenantID, scenarioID uuid.UUID) ([]*model.StaffIncentive, error)
	BatchUpsertIncentives(tenantID, scenarioID uuid.UUID, incentives []model.StaffIncentive) error
}

// CapexRepository defines the interface for capex data operations
type CapexRepository interface {
	ListByScenario(tenantID, scenarioID uuid.UUID) ([]*model.CapexEntry, error)
	BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.CapexEntry) error
}

// OpexRepository defines the interface for opex data operations
type OpexRepository interface {
	ListByScenario(tenantID, scenarioID uuid.UUID) ([]*model.OpexManualEntry, error)
	BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.OpexManualEntry) error
}

// PnLRepository defines the interface for P&L data operations
type PnLRepository interface {
	ListByScenario(tenantID, scenarioID uuid.UUID) ([]*model.PnlManualEntry, error)
	BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.PnlManualEntry) error
}

// FiplanRepository defines the interface for fiplan data operations
type FiplanRepository interface {
	ListByScenario(tenantID, scenarioID uuid.UUID) ([]*model.FiplanEntry, error)
	BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.FiplanEntry) error
}

// PnlCashRepository defines the interface for P&L cash data operations
type PnlCashRepository interface {
	ListByScenario(tenantID, scenarioID uuid.UUID) ([]*model.PnlCashEntry, error)
	BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.PnlCashEntry) error
}

// WCRRepository defines the interface for WCR data operations
type WCRRepository interface {
	ListByScenario(tenantID, scenarioID uuid.UUID) ([]*model.WCREntry, error)
	BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.WCREntry) error
}

// CashRepository defines the interface for cash data operations
type CashRepository interface {
	ListByScenario(tenantID, scenarioID uuid.UUID) ([]*model.CashMonthlyOverride, error)
	BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.CashMonthlyOverride) error
}

// BudgetRepository defines the interface for budget data operations
type BudgetRepository interface {
	ListByScenario(tenantID, scenarioID uuid.UUID, year int) ([]*model.BudgetMonthlyOverride, error)
	ListAllByScenario(tenantID, scenarioID uuid.UUID) ([]*model.BudgetMonthlyOverride, error)
	BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.BudgetMonthlyOverride) error
}

// SnapshotRepository defines the interface for snapshot data operations
type SnapshotRepository interface {
	Create(snapshot *model.PlanSnapshot) error
	GetByID(tenantID, snapshotID uuid.UUID) (*model.PlanSnapshot, error)
	ListByScenario(tenantID, scenarioID uuid.UUID, offset, limit int) ([]*model.PlanSnapshot, error)
	ListByPlan(tenantID, planID uuid.UUID, offset, limit int) ([]*model.PlanSnapshot, error)
	Delete(tenantID, snapshotID uuid.UUID) error
}

// AuditRepository defines the interface for audit log operations
type AuditRepository interface {
	Create(auditLog *model.AuditLog) error
	ListByEntity(tenantID uuid.UUID, entityType string, entityID uuid.UUID) ([]*model.AuditLog, error)
	ListByTenant(tenantID uuid.UUID, offset, limit int) ([]*model.AuditLog, error)
	ListByUser(tenantID, userID uuid.UUID, offset, limit int) ([]*model.AuditLog, error)
}

// PlanMemberRepository defines the interface for plan membership operations
type PlanMemberRepository interface {
	Create(member *model.PlanMember) error
	GetByPlanAndUser(tenantID, planID, userID uuid.UUID) (*model.PlanMember, error)
	ListByPlan(tenantID, planID uuid.UUID) ([]*model.PlanMember, error)
	ListByUser(tenantID, userID uuid.UUID) ([]*model.PlanMember, error)
	Update(member *model.PlanMember) error
	Delete(tenantID, planID, userID uuid.UUID) error
}

// Compile-time interface compliance checks
var (
	_ PlanRepository        = (*PlanRepo)(nil)
	_ ScenarioRepository    = (*ScenarioRepo)(nil)
	_ TenantRepository      = (*TenantRepo)(nil)
	_ UserRepository        = (*UserRepo)(nil)
	_ SettingsRepository    = (*SettingsRepo)(nil)
	_ ProductRepository     = (*ProductRepo)(nil)
	_ StaffRepository       = (*StaffRepo)(nil)
	_ CapexRepository       = (*CapexRepo)(nil)
	_ OpexRepository        = (*OpexRepo)(nil)
	_ PnLRepository         = (*PnLRepo)(nil)
	_ FiplanRepository      = (*FiplanRepo)(nil)
	_ PnlCashRepository     = (*PnlCashRepo)(nil)
	_ WCRRepository         = (*WCRRepo)(nil)
	_ CashRepository        = (*CashRepo)(nil)
	_ BudgetRepository      = (*BudgetRepo)(nil)
	_ SnapshotRepository    = (*SnapshotRepo)(nil)
	_ AuditRepository       = (*AuditRepo)(nil)
	_ PlanMemberRepository  = (*PlanMemberRepo)(nil)
)
