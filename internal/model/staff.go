package model

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ─────────────────────────────────────────────────────────────────────────
// 1. Staff Functions and Categories
// ─────────────────────────────────────────────────────────────────────────

// StaffFunction groups categories into the 4 P&L functions.
type StaffFunction string

const (
	FunctionRnD        StaffFunction = "rnd"
	FunctionProduction StaffFunction = "production"
	FunctionSales      StaffFunction = "sales"
	FunctionGnA        StaffFunction = "gna" // General & Administrative
)

// StaffCategory maps to the staff rows in the Staff sheet.
type StaffCategory string

const (
	// R&D (2 rows)
	CategoryRnDEngineers StaffCategory = "rnd_engineers"
	CategoryRnDProduct   StaffCategory = "rnd_product" // Product Management

	// Production (2 rows)
	CategoryProdEngineers   StaffCategory = "prod_engineers"
	CategoryProdTechnicians StaffCategory = "prod_technicians"

	// Sales & Marketing (3 rows)
	CategorySalesTeam           StaffCategory = "sales_team"
	CategoryMarketingTeam       StaffCategory = "marketing_team"
	CategoryCustomerSuccess     StaffCategory = "sales_customer_success" // Customer Success / Account Management

	// G&A (6 rows)
	CategoryAdminManagers   StaffCategory = "admin_managers"
	CategoryAdminAssistants StaffCategory = "admin_assistants"
	CategoryExecutiveTeam   StaffCategory = "executive_team"
	CategoryFinance         StaffCategory = "gna_finance"  // Finance & Accounting
	CategoryHR              StaffCategory = "gna_hr"       // HR & People Ops
	CategoryIT              StaffCategory = "gna_it"       // IT & Infrastructure
)

// CategoryFunction maps each category to its parent function.
var CategoryFunction = map[StaffCategory]StaffFunction{
	CategoryRnDEngineers:    FunctionRnD,
	CategoryRnDProduct:      FunctionRnD,
	CategoryProdEngineers:   FunctionProduction,
	CategoryProdTechnicians: FunctionProduction,
	CategorySalesTeam:       FunctionSales,
	CategoryMarketingTeam:   FunctionSales,
	CategoryCustomerSuccess: FunctionSales,
	CategoryAdminManagers:   FunctionGnA,
	CategoryAdminAssistants: FunctionGnA,
	CategoryExecutiveTeam:   FunctionGnA,
	CategoryFinance:         FunctionGnA,
	CategoryHR:              FunctionGnA,
	CategoryIT:              FunctionGnA,
}

// AllCategories in display order (matches spreadsheet row order).
var AllCategories = []StaffCategory{
	// R&D
	CategoryRnDEngineers,
	CategoryRnDProduct,
	// Production
	CategoryProdEngineers,
	CategoryProdTechnicians,
	// Sales & Marketing
	CategorySalesTeam,
	CategoryMarketingTeam,
	CategoryCustomerSuccess,
	// G&A
	CategoryAdminManagers,
	CategoryAdminAssistants,
	CategoryExecutiveTeam,
	CategoryFinance,
	CategoryHR,
	CategoryIT,
}

// ─────────────────────────────────────────────────────────────────────────
// 2. Input Models (stored in DB)
// ─────────────────────────────────────────────────────────────────────────

// StaffHeadcount stores the FTE count per category per year.
// Maps to Staff rows 10–17 (columns C–G).
// 8 categories × 5 years = 40 rows per scenario.
type StaffHeadcount struct {
	TenantScoped
	ScenarioID   uuid.UUID     `gorm:"type:uuid;not null;uniqueIndex:uix_staff_headcounts" json:"scenarioId"`
	Category     StaffCategory `gorm:"type:varchar(50);not null;uniqueIndex:uix_staff_headcounts" json:"category"`
	YearIndex    int           `gorm:"column:year;not null;uniqueIndex:uix_staff_headcounts" json:"yearIndex"` // 1–5, DB column: year
	FTE          decimal.Decimal `gorm:"type:numeric(8,2)" json:"fte"`
	IsOverridden bool          `gorm:"not null;default:false" json:"isOverridden"`
}

// TableName specifies the table name for StaffHeadcount
func (StaffHeadcount) TableName() string {
	return "staff_headcounts"
}

// StaffSalary stores the average monthly gross salary per category per year.
// Maps to Staff cols I–M, rows 10–17.
// 8 categories × 5 years = 40 rows per scenario.
type StaffSalary struct {
	TenantScoped
	ScenarioID         uuid.UUID     `gorm:"type:uuid;not null;uniqueIndex:uix_staff_salaries" json:"scenarioId"`
	Category           StaffCategory `gorm:"type:varchar(50);not null;uniqueIndex:uix_staff_salaries" json:"category"`
	YearIndex          int           `gorm:"column:year;not null;uniqueIndex:uix_staff_salaries" json:"yearIndex"` // 1–5, DB column: year
	MonthlyGrossSalary decimal.Decimal `gorm:"type:numeric(12,4)" json:"monthlyGrossSalary"`
	AnnualIncreasePct  decimal.Decimal `gorm:"type:numeric(8,4)" json:"annualIncreasePct"`
	IsOverridden       bool          `gorm:"not null;default:false" json:"isOverridden"`
}

// TableName specifies the table name for StaffSalary
func (StaffSalary) TableName() string {
	return "staff_salaries"
}

// StaffIncentive stores the two incentive inputs per year.
// Maps to Staff rows 38 (% incentive) and 40 (specific incentive).
// 5 years = 5 rows per scenario.
type StaffIncentive struct {
	TenantScoped
	ScenarioID         uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:uix_staff_incentives" json:"scenarioId"`
	YearIndex          int             `gorm:"column:year;not null;uniqueIndex:uix_staff_incentives" json:"yearIndex"` // 1–5, DB column: year
	IncentivePct       decimal.Decimal `gorm:"type:numeric(8,4)" json:"incentivePct"`
	SpecificIncentives decimal.Decimal `gorm:"type:numeric(12,4)" json:"specificIncentives"`
	IsOverridden       bool            `gorm:"not null;default:false" json:"isOverridden"`
}

// TableName specifies the table name for StaffIncentive
func (StaffIncentive) TableName() string {
	return "staff_incentives"
}

// ─────────────────────────────────────────────────────────────────────────
// 3. Computed Output Types (NOT stored in DB)
// ─────────────────────────────────────────────────────────────────────────

// StaffPayrollSummary is the COMPUTED output for the Staff sheet.
// Not stored in DB — generated by compute/staff.go.
// Returned by GET /api/v1/plans/{planID}/scenarios/{scenID}/staff/summary
type StaffPayrollSummary struct {
	ScenarioID uuid.UUID `json:"scenarioId"`

	// Headcount summary (§2)
	Headcount StaffHeadcountSummary `json:"headcount"`

	// Annual payroll per category + totals (§4)
	Payroll [5]StaffPayrollYear `json:"payroll"`

	// Functional breakdown (§5)
	FunctionalBreakdown [5]StaffFunctionalYear `json:"functionalBreakdown"`

	// Settings references (for display)
	SalaryMonthsPerYear   int             `json:"salaryMonthsPerYear"`
	FirstFiscalYearMonths int             `json:"firstFiscalYearMonths"`
	EmployerTaxRate       decimal.Decimal `json:"employerTaxRate"`
}

// StaffHeadcountSummary — computed totals from §2.
type StaffHeadcountSummary struct {
	// Per category × per year (the grid as entered)
	Categories []StaffCategoryHeadcount `json:"categories"`

	// Row 18: Total staff per year
	TotalStaff [5]decimal.Decimal `json:"totalStaff"`

	// Row 19: Number of recruits (net new hires)
	// Year 1 = TotalStaff[0] - PlanConfig.PreviousStaff
	// Year N = TotalStaff[N] - TotalStaff[N-1]
	Recruits [5]decimal.Decimal `json:"recruits"`
}

// StaffCategoryHeadcount — one category's headcount across 5 years.
type StaffCategoryHeadcount struct {
	Category StaffCategory      `json:"category"`
	Function StaffFunction      `json:"function"`
	FTE      [5]decimal.Decimal `json:"fte"` // per year
}

// StaffPayrollYear — computed payroll for one year (§4).
type StaffPayrollYear struct {
	Year      int `json:"year"`
	YearIndex int `json:"yearIndex"`

	// Per-category payroll (rows 24–31)
	// Formula: FTE × MonthlySalary × (1 + EmployerTaxRate/100) × Months × FY_Adj
	Categories []StaffCategoryPayroll `json:"categories"`

	// Row 33: Sub-total payroll excl. incentives
	SubtotalPayroll decimal.Decimal `json:"subtotalPayroll"`

	// Row 38: Incentive % (from StaffIncentive input)
	IncentivePct decimal.Decimal `json:"incentivePct"`

	// Row 39: Incentive amount = SubtotalPayroll × IncentivePct  (IncentivePct is a fraction, e.g. 0.10 = 10%)
	IncentiveAmount decimal.Decimal `json:"incentiveAmount"`

	// Row 40: Specific incentives (from StaffIncentive input)
	SpecificIncentives decimal.Decimal `json:"specificIncentives"`

	// Row 42: Sub-total incentives = IncentiveAmount + SpecificIncentives
	SubtotalIncentives decimal.Decimal `json:"subtotalIncentives"`

	// Row 45: TOTAL PAYROLL = SubtotalPayroll + SubtotalIncentives
	TotalPayroll decimal.Decimal `json:"totalPayroll"`
}

// StaffCategoryPayroll — one category's annual payroll breakdown.
type StaffCategoryPayroll struct {
	Category      StaffCategory   `json:"category"`
	Function      StaffFunction   `json:"function"`
	FTE           decimal.Decimal `json:"fte"`
	MonthlySalary decimal.Decimal `json:"monthlySalary"`
	EmployerRate  decimal.Decimal `json:"employerRate"`
	Months        int             `json:"months"`       // C57 or C58 for Yr1
	FYAdjustment  decimal.Decimal `json:"fyAdjustment"`
	AnnualPayroll decimal.Decimal `json:"annualPayroll"` // the computed result
}

// StaffFunctionalYear — §5 breakdown for one year.
type StaffFunctionalYear struct {
	Year      int `json:"year"`
	YearIndex int `json:"yearIndex"`

	RnD        decimal.Decimal `json:"rnd"`        // Row 50: sum of RnD categories
	Production decimal.Decimal `json:"production"` // Row 51: sum of Production categories
	Sales      decimal.Decimal `json:"sales"`      // Row 52: sum of Sales categories
	GnA        decimal.Decimal `json:"gna"`        // Row 53: sum of G&A categories

	// % of Year 3 total (col I in spreadsheet)
	RnDPctYear3        *decimal.Decimal `json:"rndPctYear3,omitempty"`
	ProductionPctYear3 *decimal.Decimal `json:"productionPctYear3,omitempty"`
	SalesPctYear3      *decimal.Decimal `json:"salesPctYear3,omitempty"`
	GnAPctYear3        *decimal.Decimal `json:"gnaPctYear3,omitempty"`
}
