package model

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// BudgetLineID for monthly budget overrides.
type BudgetLineID string

const (
	// Revenue section
	BudgetSalesRevenue    BudgetLineID = "sales_revenue"
	BudgetOtherRevenue    BudgetLineID = "other_revenue"
	BudgetTotalRevenue    BudgetLineID = "total_revenue"
	// COGS section
	BudgetRawMaterials    BudgetLineID = "raw_materials"
	BudgetSubcontracting  BudgetLineID = "subcontracting"
	BudgetDirectLabor     BudgetLineID = "direct_labor"
	BudgetTotalCOGS       BudgetLineID = "total_cogs"
	// Gross Margin
	BudgetGrossMargin     BudgetLineID = "gross_margin"
	// External Expenses
	BudgetRentExpenses    BudgetLineID = "rent_expenses"
	BudgetLeasingExpenses BudgetLineID = "leasing_expenses"
	BudgetProfFees        BudgetLineID = "professional_fees"
	BudgetRoyalties       BudgetLineID = "royalties"
	BudgetTravelExpenses  BudgetLineID = "travel_expenses"
	BudgetMarketingExp    BudgetLineID = "marketing_expenses"
	BudgetHRExpenses      BudgetLineID = "hr_expenses"
	BudgetTotalExternal   BudgetLineID = "total_external"
	// Staff Costs
	BudgetPayroll         BudgetLineID = "payroll"
	BudgetIncentives      BudgetLineID = "incentives"
	BudgetTotalStaff      BudgetLineID = "total_staff"
	// Taxes
	BudgetTaxesDuties     BudgetLineID = "taxes_duties"
	// EBITDA
	BudgetEBITDA          BudgetLineID = "ebitda"
	// Depreciation
	BudgetDepreciation    BudgetLineID = "depreciation"
	// EBIT
	BudgetEBIT            BudgetLineID = "ebit"
	// Financial
	BudgetFinancialIncome BudgetLineID = "financial_income"
	BudgetFinancialExp    BudgetLineID = "financial_expense"
	// Net
	BudgetPreTaxProfit    BudgetLineID = "pretax_profit"
	BudgetCorporateTax    BudgetLineID = "corporate_tax"
	BudgetNetProfit       BudgetLineID = "net_profit"
)

// BudgetMonthlyOverride represents monthly budget overrides (sparse storage).
type BudgetMonthlyOverride struct {
	TenantScoped
	ScenarioID uuid.UUID       `gorm:"type:uuid;not null;index" json:"scenarioId"`
	LineID     BudgetLineID    `gorm:"type:varchar(100);not null" json:"lineId"`
	YearIndex  int             `gorm:"not null" json:"yearIndex"` // 1-5
	Month      int             `gorm:"not null" json:"month"`     // 1-12
	Amount     decimal.Decimal `gorm:"type:numeric(15,2)" json:"amount"`
}

func (BudgetMonthlyOverride) TableName() string {
	return "budget_monthly_overrides"
}

// Budget1Report is the computed detailed monthly budget (not stored in DB).
type Budget1Report struct {
	YearIndex int                `json:"yearIndex"`
	Rows      []BudgetMonthlyRow `json:"rows"`
}

type BudgetMonthlyRow struct {
	LineID           BudgetLineID        `json:"lineId"`
	Label            string              `json:"label"`
	Section          string              `json:"section"`
	Annual           decimal.Decimal     `json:"annual"`
	Monthly          [12]decimal.Decimal `json:"months"`
	YearTotal        decimal.Decimal     `json:"annualTotal"`
	IsTotal          bool                `json:"isTotal"`
	DistributionRule DistributionRule    `json:"distributionRule"`
}

// Budget2Report is the computed summarised budget (not stored in DB).
type Budget2Report struct {
	Quarterly   Budget2View `json:"quarterly"`
	SemiAnnual  Budget2View `json:"semiAnnual"`
	ByFunction  Budget2View `json:"byFunction"`
	ByCostType  Budget2View `json:"byCostType"`
}

type Budget2View struct {
	Label   string              `json:"label"`
	Columns []string            `json:"columns"`
	Rows    []Budget2Row        `json:"rows"`
}

type Budget2Row struct {
	LineID      BudgetLineID      `json:"lineId"`
	Label       string            `json:"label"`
	Values      []decimal.Decimal `json:"values"`
	IsTotal     bool              `json:"isTotal"`
	IsAggregate bool              `json:"isAggregate"` // alias of IsTotal for frontend GridRow mapping
}
