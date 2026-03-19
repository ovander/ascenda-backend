package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// PlanConfig stores plan-level configuration settings
type PlanConfig struct {
	TenantScoped
	ScenarioID              uuid.UUID       `gorm:"type:uuid;not null;index" json:"scenarioId"`
	// §1: General Configuration
	Language                string          `gorm:"type:varchar(10);default:'fr'" json:"language"`
	CompanyName             string          `gorm:"type:varchar(255)" json:"companyName"`
	ForecastStart           time.Time       `gorm:"not null" json:"forecastStart"`
	// §5: Key Rates & Parameters
	PreviousStaff           int             `gorm:"not null;default:0" json:"previousStaff"`
	PriorYearTurnover       decimal.Decimal `gorm:"type:numeric(15,2)" json:"priorYearTurnover"`
	AvgDistributorDiscount  decimal.Decimal `gorm:"type:numeric(5,4)" json:"avgDistributorDiscount"`
	MLTInterestRate         decimal.Decimal `gorm:"type:numeric(5,4)" json:"mltInterestRate"`
	DiscountedSalesPct      decimal.Decimal `gorm:"type:numeric(5,4)" json:"discountedSalesPct"`
	BillsDiscountRate       decimal.Decimal `gorm:"type:numeric(5,4)" json:"billsDiscountRate"`
	AvgBillTermMonths       int             `gorm:"not null;default:3" json:"avgBillTermMonths"`
	VATRate                 decimal.Decimal `gorm:"type:numeric(5,4)" json:"vatRate"`
	CorporateTaxRate        decimal.Decimal `gorm:"type:numeric(5,4)" json:"corporateTaxRate"`
	TaxesAndDutiesRate      decimal.Decimal `gorm:"type:numeric(5,4)" json:"taxesAndDutiesRate"`
	InterestOnPositiveCash  decimal.Decimal `gorm:"type:numeric(5,4)" json:"interestOnPositiveCash"`
	MLTLoanTermYears        int             `gorm:"not null;default:5" json:"mltLoanTermYears"`
	CurrencySymbol          string          `gorm:"type:varchar(10);default:'€'" json:"currencySymbol"`
	// §7: Payroll & Fiscal Year
	SalaryMonthsPerYear     int             `gorm:"not null;default:12" json:"salaryMonthsPerYear"`
	EmployerTaxRate         decimal.Decimal `gorm:"type:numeric(5,4)" json:"employerTaxRate"`
	FirstFiscalYearMonths   int             `gorm:"not null;default:12" json:"firstFiscalYearMonths"`
	DiscountRate            decimal.Decimal `gorm:"type:numeric(5,4)" json:"discountRate"`
	IncentiveCap            decimal.Decimal `gorm:"type:numeric(5,4)" json:"incentiveCap"`
	Country                 string          `gorm:"type:varchar(50);default:'BE'" json:"country"`
}

// TableName specifies the table name for PlanConfig
func (PlanConfig) TableName() string {
	return "plan_configs"
}

// OpeningBalance stores the opening balance sheet for a scenario
type OpeningBalance struct {
	TenantScoped
	ScenarioID          uuid.UUID       `gorm:"type:uuid;not null;index" json:"scenarioId"`
	// Assets
	NoncurrentAssets    decimal.Decimal `gorm:"type:numeric(15,2)" json:"noncurrentAssets"`
	Inventories         decimal.Decimal `gorm:"type:numeric(15,2)" json:"inventories"`
	CustomerReceivables decimal.Decimal `gorm:"type:numeric(15,2)" json:"customerReceivables"`
	CashAndSecurities   decimal.Decimal `gorm:"type:numeric(15,2)" json:"cashAndSecurities"`
	// Liabilities
	ShareCapital        decimal.Decimal `gorm:"type:numeric(15,2)" json:"shareCapital"`
	RetainedEarnings    decimal.Decimal `gorm:"type:numeric(15,2)" json:"retainedEarnings"`
	LoansAndDebt        decimal.Decimal `gorm:"type:numeric(15,2)" json:"loansAndDebt"`
	SupplierPayables    decimal.Decimal `gorm:"type:numeric(15,2)" json:"supplierPayables"`
	SocialAndTaxDebts   decimal.Decimal `gorm:"type:numeric(15,2)" json:"socialAndTaxDebts"`
}

// TableName specifies the table name for OpeningBalance
func (OpeningBalance) TableName() string {
	return "opening_balances"
}

// WorkingCapitalConfig stores working capital assumptions for a scenario
type WorkingCapitalConfig struct {
	TenantScoped
	ScenarioID        uuid.UUID       `gorm:"type:uuid;not null;index" json:"scenarioId"`
	// Customer payment distribution (must sum to ≤100%)
	CustomerPct0Days  decimal.Decimal `gorm:"type:numeric(5,4)" json:"customerPct0Days"`
	CustomerPct30Days decimal.Decimal `gorm:"type:numeric(5,4)" json:"customerPct30Days"`
	CustomerPct60Days decimal.Decimal `gorm:"type:numeric(5,4)" json:"customerPct60Days"`
	CustomerPct90Days decimal.Decimal `gorm:"type:numeric(5,4)" json:"customerPct90Days"`
	// Supplier payment distribution (must sum to ≤100%)
	SupplierPct0Days  decimal.Decimal `gorm:"type:numeric(5,4)" json:"supplierPct0Days"`
	SupplierPct30Days decimal.Decimal `gorm:"type:numeric(5,4)" json:"supplierPct30Days"`
	SupplierPct60Days decimal.Decimal `gorm:"type:numeric(5,4)" json:"supplierPct60Days"`
	SupplierPct90Days decimal.Decimal `gorm:"type:numeric(5,4)" json:"supplierPct90Days"`
	// Inventory as % of sales at production cost per year
	InventoryPctYear1 decimal.Decimal `gorm:"type:numeric(5,4)" json:"inventoryPctYear1"`
	InventoryPctYear2 decimal.Decimal `gorm:"type:numeric(5,4)" json:"inventoryPctYear2"`
	InventoryPctYear3 decimal.Decimal `gorm:"type:numeric(5,4)" json:"inventoryPctYear3"`
	InventoryPctYear4 decimal.Decimal `gorm:"type:numeric(5,4)" json:"inventoryPctYear4"`
	InventoryPctYear5 decimal.Decimal `gorm:"type:numeric(5,4)" json:"inventoryPctYear5"`
}

// TableName specifies the table name for WorkingCapitalConfig
func (WorkingCapitalConfig) TableName() string {
	return "working_capital_configs"
}

// OpexPerHire maps to Settings §8 (operating expense ratios per employee).
// One row per scenario.
type OpexPerHire struct {
	TenantScoped
	ScenarioID              uuid.UUID       `gorm:"type:uuid;not null;index" json:"scenarioId"`
	PropertyRentals         decimal.Decimal `gorm:"type:numeric(12,4)" json:"propertyRentals"`          // D64 k, 1st year base
	PostageTelecom          decimal.Decimal `gorm:"type:numeric(12,4)" json:"postageTelecom"`           // D65 k/person/year
	SuppliesPurchases       decimal.Decimal `gorm:"type:numeric(12,4)" json:"suppliesPurchases"`        // D66 k/person/year
	StudiesDocumentation    decimal.Decimal `gorm:"type:numeric(12,4)" json:"studiesDocumentation"`     // D67 k/person/year
	InsuranceCostsPctSales  decimal.Decimal `gorm:"type:numeric(5,4)" json:"insuranceCostsPctSales"`    // D68 % of sales
	RoyaltyPaymentsPctSales decimal.Decimal `gorm:"type:numeric(5,4)" json:"royaltyPaymentsPctSales"`   // J65 % of sales
	TravelTransportation    decimal.Decimal `gorm:"type:numeric(12,4)" json:"travelTransportation"`     // J66 k/person/year
	MissionRepresentation   decimal.Decimal `gorm:"type:numeric(12,4)" json:"missionRepresentation"`    // J67 k/person/year
	RecruitTrainingPctPayroll decimal.Decimal `gorm:"type:numeric(5,4)" json:"recruitTrainingPctPayroll"` // J68 % of payroll
}

// TableName specifies the table name for OpexPerHire
func (OpexPerHire) TableName() string {
	return "opex_per_hire"
}

// CapexPerHire maps to Settings §8 (capital expenditure per new hire).
// One row per scenario.
type CapexPerHire struct {
	TenantScoped
	ScenarioID       uuid.UUID       `gorm:"type:uuid;not null;index" json:"scenarioId"`
	FurniturePerHire decimal.Decimal `gorm:"type:numeric(12,4)" json:"furniturePerHire"` // J60 k
	ITEquipPerHire   decimal.Decimal `gorm:"type:numeric(12,4)" json:"itEquipPerHire"`   // J61 k
}

// TableName specifies the table name for CapexPerHire
func (CapexPerHire) TableName() string {
	return "capex_per_hire"
}

// MultiYearAdjustment maps to Settings §6 (rows 53–57).
// One row per scenario per year (5 rows per scenario).
type MultiYearAdjustment struct {
	TenantScoped
	ScenarioID           uuid.UUID       `gorm:"type:uuid;not null;index" json:"scenarioId"`
	YearIndex            int             `gorm:"not null" json:"yearIndex"` // 1–5
	PreviousDepreciation decimal.Decimal `gorm:"type:numeric(15,2)" json:"previousDepreciation"` // D55:H55 k
	PotentialTaxCredits  decimal.Decimal `gorm:"type:numeric(15,2)" json:"potentialTaxCredits"`  // D56:H56 k
}

// TableName specifies the table name for MultiYearAdjustment
func (MultiYearAdjustment) TableName() string {
	return "multi_year_adjustments"
}

// ──────────────────────────────────────────────────────────────────────────
// Computed wrapper types (not stored in DB)
// ──────────────────────────────────────────────────────────────────────────

// PlanConfigComputed wraps PlanConfig with computed fields
type PlanConfigComputed struct {
	PlanConfig
	CurrentDate    time.Time       `json:"currentDate"`
	FirstCivilYear int             `json:"firstCivilYear"`
	OverdraftRate  decimal.Decimal `json:"overdraftRate"` // MLTInterestRate + 3%
	YearHeaders    [5]int          `json:"yearHeaders"`   // [2025, 2026, 2027, 2028, 2029]
	UnitLabel      string          `json:"unitLabel"`     // "k€", "k$", etc.
}

// OpeningBalanceComputed wraps OpeningBalance with computed fields
type OpeningBalanceComputed struct {
	OpeningBalance
	TotalAssets      decimal.Decimal `json:"totalAssets"`
	TotalLiabilities decimal.Decimal `json:"totalLiabilities"`
	OtherPayables    decimal.Decimal `json:"otherPayables"` // balancing plug
}

// WorkingCapitalComputed wraps WorkingCapitalConfig with computed 120-day tranches
type WorkingCapitalComputed struct {
	WorkingCapitalConfig
	CustomerPct120Days decimal.Decimal `json:"customerPct120Days"`
	SupplierPct120Days decimal.Decimal `json:"supplierPct120Days"`
}

// MultiYearAdjustmentComputed wraps MultiYearAdjustment with computed fields
type MultiYearAdjustmentComputed struct {
	MultiYearAdjustment
	Year                 int             `json:"year"`                 // calendar year
	Deleveraging         decimal.Decimal `json:"deleveraging"`         // SLN
	CorporateTaxEstimate decimal.Decimal `json:"corporateTaxEstimate"` // from P&L
}
