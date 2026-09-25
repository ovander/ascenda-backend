package model

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// PnlLineID constants for French P&L format
type PnlLineID string

const (
	PnlSales                 PnlLineID = "sales"
	PnlExportSalesMemo       PnlLineID = "export_sales_memo"
	PnlCapitalizedProd       PnlLineID = "capitalized_production"
	PnlStoredProd            PnlLineID = "stored_production"
	PnlTotalOperatingRev     PnlLineID = "total_operating_rev"
	PnlCOGS                  PnlLineID = "cogs"
	PnlInventoryChange       PnlLineID = "inventory_change"
	PnlExternalExpenses      PnlLineID = "external_expenses"
	PnlTotalConsumption      PnlLineID = "total_consumption"
	PnlAddedValue            PnlLineID = "added_value"
	PnlTaxesAndDuties        PnlLineID = "taxes_and_duties"
	PnlPayrollExpenses       PnlLineID = "payroll_expenses"
	PnlEBITDA                PnlLineID = "ebitda"
	PnlDepreciation          PnlLineID = "depreciation"
	PnlImpairment            PnlLineID = "impairment"
	PnlGrantsOtherRevenue    PnlLineID = "grants_other_revenue"
	PnlOtherOperatingExp     PnlLineID = "other_operating_exp"
	PnlEBIT                  PnlLineID = "ebit"
	PnlFinancialRevenues     PnlLineID = "financial_revenues"
	PnlFinancialExpenses     PnlLineID = "financial_expenses"
	PnlPreTaxEarnings        PnlLineID = "pre_tax_earnings"
	PnlExtraordinaryIncome   PnlLineID = "extraordinary_income"
	PnlExtraordinaryExpense  PnlLineID = "extraordinary_expense"
	PnlEmployeeParticipation PnlLineID = "employee_participation"
	PnlCorporateTax          PnlLineID = "corporate_tax"
	PnlTaxCredits            PnlLineID = "tax_credits"
	PnlNetProfit             PnlLineID = "net_profit"
	PnlStaffHeadcount        PnlLineID = "staff_headcount"
	PnlCashFlow              PnlLineID = "cash_flow"
)

// UserInputPnlLines defines the 6 lines that users can manually enter.
var UserInputPnlLines = []PnlLineID{
	PnlCapitalizedProd,
	PnlImpairment,
	PnlOtherOperatingExp,
	PnlExtraordinaryIncome,
	PnlExtraordinaryExpense,
	PnlEmployeeParticipation,
}

// PnlManualEntry represents manual P&L line entries.
type PnlManualEntry struct {
	TenantScoped
	ScenarioID uuid.UUID       `gorm:"type:uuid;not null;index" json:"scenarioId"`
	LineID     PnlLineID       `gorm:"type:varchar(100);not null" json:"lineId"`
	YearIndex  int             `gorm:"not null" json:"yearIndex"`
	Amount     decimal.Decimal `gorm:"type:numeric(15,2)" json:"amount"`
}

func (PnlManualEntry) TableName() string {
	return "pnl_manual_entries"
}

// PnlReport is the computed French P&L output (not stored in DB).
type PnlReport struct {
	Years     [5]PnlYear   `json:"years"`
	ChartData PnlChartData `json:"chartData"`
}

type PnlYear struct {
	Year                  int             `json:"year"`
	YearIndex             int             `json:"yearIndex"`
	Sales                 decimal.Decimal `json:"sales"`
	ExportSalesMemo       decimal.Decimal `json:"exportSalesMemo"`
	CapitalizedProduction decimal.Decimal `json:"capitalizedProduction"`
	StoredProduction      decimal.Decimal `json:"storedProduction"`
	TotalOperatingRevenue decimal.Decimal `json:"totalOperatingRevenue"`
	COGS                  decimal.Decimal `json:"cogs"`
	InventoryChange       decimal.Decimal `json:"inventoryChange"`
	ExternalExpenses      decimal.Decimal `json:"externalExpenses"`
	TotalConsumption      decimal.Decimal `json:"totalConsumption"`
	AddedValue            decimal.Decimal `json:"addedValue"`
	TaxesAndDuties        decimal.Decimal `json:"taxesAndDuties"`
	PayrollExpenses       decimal.Decimal `json:"payrollExpenses"`
	EBITDA                decimal.Decimal `json:"ebitda"`
	Depreciation          decimal.Decimal `json:"depreciation"`
	Impairment            decimal.Decimal `json:"impairment"`
	GrantsOtherRevenue    decimal.Decimal `json:"grantsOtherRevenue"`
	OtherOperatingExp     decimal.Decimal `json:"otherOperatingExp"`
	EBIT                  decimal.Decimal `json:"ebit"`
	FinancialRevenues     decimal.Decimal `json:"financialRevenues"`
	FinancialExpenses     decimal.Decimal `json:"financialExpenses"`
	PreTaxEarnings        decimal.Decimal `json:"preTaxEarnings"`
	ExtraordinaryIncome   decimal.Decimal `json:"extraordinaryIncome"`
	ExtraordinaryExpense  decimal.Decimal `json:"extraordinaryExpense"`
	EmployeeParticipation decimal.Decimal `json:"employeeParticipation"`
	CorporateTax          decimal.Decimal `json:"corporateTax"`
	TaxCredits            decimal.Decimal `json:"taxCredits"`
	NetProfit             decimal.Decimal `json:"netProfit"`
	StaffHeadcount        decimal.Decimal `json:"staffHeadcount"`
	CashFlow              decimal.Decimal `json:"cashFlow"`
	// Percentage column
	PctOfSales decimal.Decimal `json:"pctOfSales"`
}

type PnlChartData struct {
	Years           [5]int             `json:"years"`
	EBITDAPositive  [5]decimal.Decimal `json:"ebitdaPositive"`
	EBITDANegative  [5]decimal.Decimal `json:"ebitdaNegative"`
	OtherOpex       [5]decimal.Decimal `json:"otherOpex"`
	PayrollExpenses [5]decimal.Decimal `json:"payrollExpenses"`
}
