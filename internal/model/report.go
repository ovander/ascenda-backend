package model

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Report represents a persisted computed report for a scenario.
type Report struct {
	TenantScoped
	ScenarioID uuid.UUID       `gorm:"type:uuid;not null;index" json:"scenarioId"`
	Data       json.RawMessage `gorm:"type:jsonb;not null" json:"data"`
}

// TableName specifies the table name for Report.
func (Report) TableName() string {
	return "reports"
}

// ──────────────────────────────────────────────────────────────────────────
// Balance Sheet (4 views per BIZPREV_BSHEET_MODELS.md)
// ──────────────────────────────────────────────────────────────────────────

// BSheetReport is the computed balance sheet output (not stored in DB).
type BSheetReport struct {
	Detailed       BSheetDetailed       `json:"detailed"`
	Condensed      BSheetCondensed      `json:"condensed"`
	Analysis       BSheetAnalysis       `json:"analysis"`
	Capital        BSheetCapital        `json:"capital"`
	WorkingCapital BSheetWorkingCapital `json:"workingCapital"`
	Equity         [6]decimal.Decimal   `json:"equity"`
	Charts         BSheetCharts         `json:"charts"`
	NCAWarning     [6]bool              `json:"ncaWarning"`
}

type BSheetDetailed struct {
	Assets      BSheetDetailedAssets      `json:"assets"`
	Liabilities BSheetDetailedLiabilities `json:"liabilities"`
}

type BSheetDetailedAssets struct {
	NoncurrentAssets  [6]decimal.Decimal `json:"noncurrentAssets"`
	Inventory         [6]decimal.Decimal `json:"inventory"`
	AccountsReceivable [6]decimal.Decimal `json:"accountsReceivable"`
	Cash              [6]decimal.Decimal `json:"cash"`
	TotalAssets       [6]decimal.Decimal `json:"totalAssets"`
}

type BSheetDetailedLiabilities struct {
	ShareCapital    [6]decimal.Decimal `json:"shareCapital"`
	NetProfit       [6]decimal.Decimal `json:"netProfit"`
	RetainedEarnings [6]decimal.Decimal `json:"retainedEarnings"`
	LongTermDebt    [6]decimal.Decimal `json:"longTermDebt"`
	TradePayables   [6]decimal.Decimal `json:"tradePayables"`
	SocialTaxDebts  [6]decimal.Decimal `json:"socialTaxDebts"`
	OtherPayables   [6]decimal.Decimal `json:"otherPayables"`
	TotalLiabilities [6]decimal.Decimal `json:"totalLiabilities"`
}

type BSheetCondensed struct {
	Assets      BSheetCondensedAssets      `json:"assets"`
	Liabilities BSheetCondensedLiabilities `json:"liabilities"`
}

type BSheetCondensedAssets struct {
	NoncurrentAssets [6]decimal.Decimal `json:"noncurrentAssets"`
	CurrentAssets    [6]decimal.Decimal `json:"currentAssets"`
	Cash             [6]decimal.Decimal `json:"cash"`
	Total            [6]decimal.Decimal `json:"total"`
}

type BSheetCondensedLiabilities struct {
	Equity       [6]decimal.Decimal `json:"equity"`
	LongTermDebt [6]decimal.Decimal `json:"longTermDebt"`
	ShortTermDebt [6]decimal.Decimal `json:"shortTermDebt"`
	Total        [6]decimal.Decimal `json:"total"`
}

type BSheetAnalysis struct {
	Equity          [6]decimal.Decimal `json:"equity"`
	LongTermDebt    [6]decimal.Decimal `json:"longTermDebt"`
	PermanentCapital [6]decimal.Decimal `json:"permanentCapital"`
	ShortTermDebt   [6]decimal.Decimal `json:"shortTermDebt"`
	TotalSources    [6]decimal.Decimal `json:"totalSources"`
	NoncurrentAssets [6]decimal.Decimal `json:"noncurrentAssets"`
	CurrentAssets   [6]decimal.Decimal `json:"currentAssets"`
	Cash            [6]decimal.Decimal `json:"cash"`
	TotalUses       [6]decimal.Decimal `json:"totalUses"`
	WorkingCapital  [6]decimal.Decimal `json:"workingCapital"`
	WCR             [6]decimal.Decimal `json:"wcr"`
	WCMinusWCR      [6]decimal.Decimal `json:"wcMinusWcr"`
	NetDebt         [6]decimal.Decimal `json:"netDebt"`
}

type BSheetCapital struct {
	Employed [6]decimal.Decimal `json:"employed"`
	Invested [6]decimal.Decimal `json:"invested"`
}

type BSheetWorkingCapital struct {
	Employed [6]decimal.Decimal `json:"employed"`
	Invested [6]decimal.Decimal `json:"invested"`
}

type BSheetCharts struct {
	Years            [6]int             `json:"years"`
	AssetStructure   [6]decimal.Decimal `json:"assetStructure"`
	LiabilityStructure [6]decimal.Decimal `json:"liabilityStructure"`
	CapitalEmployed  [6]decimal.Decimal `json:"capitalEmployed"`
	CapitalInvested  [6]decimal.Decimal `json:"capitalInvested"`
	WCEmployed       [6]decimal.Decimal `json:"wcEmployed"`
	WCInvested       [6]decimal.Decimal `json:"wcInvested"`
}

// ──────────────────────────────────────────────────────────────────────────
// Ratios (per BIZPREV_RATIOS_MODELS.md)
// ──────────────────────────────────────────────────────────────────────────

// RatiosReport is the computed financial ratios output (not stored in DB).
type RatiosReport struct {
	Years          [5]int              `json:"years"`
	ScalingFactor  decimal.Decimal     `json:"scalingFactor"`
	Sales          RatiosSalesMargins  `json:"sales"`
	Operational    RatiosOperational   `json:"operational"`
	Profitability  RatiosProfitability `json:"profitability"`
	EquityLeverage RatiosEquityLeverage `json:"equityLeverage"`
	Valuation      RatiosValuation     `json:"valuation"`
	Charts         RatiosCharts        `json:"charts"`
}

type RatiosSalesMargins struct {
	Sales          [5]decimal.Decimal `json:"sales"`
	GrowthRate     [5]decimal.Decimal `json:"growthRate"`
	ExportSales    [5]decimal.Decimal `json:"exportSales"`
	ExportPct      [5]decimal.Decimal `json:"exportPct"`
	COGS           [5]decimal.Decimal `json:"cogs"`
	COGSPct        [5]decimal.Decimal `json:"cogsPct"`
	GrossMarginPct [5]decimal.Decimal `json:"grossMarginPct"`
}

type RatiosOperational struct {
	StaffHeadcount   [5]decimal.Decimal `json:"staffHeadcount"`
	SalesPerStaff    [5]decimal.Decimal `json:"salesPerStaff"`
	PayrollExpenses  [5]decimal.Decimal `json:"payrollExpenses"`
	PayrollPct       [5]decimal.Decimal `json:"payrollPct"`
	CapitalExpenditure [5]decimal.Decimal `json:"capitalExpenditure"`
	CapexPct         [5]decimal.Decimal `json:"capexPct"`
	Depreciation     [5]decimal.Decimal `json:"depreciation"`
	ExternalExpenses [5]decimal.Decimal `json:"externalExpenses"`
	AdvertisingPromo [5]decimal.Decimal `json:"advertisingPromo"`
	AdPromoPct       [5]decimal.Decimal `json:"adPromoPct"`
}

type RatiosProfitability struct {
	AddedValue      [5]decimal.Decimal `json:"addedValue"`
	AddedValuePct   [5]decimal.Decimal `json:"addedValuePct"`
	EBITDA          [5]decimal.Decimal `json:"ebitda"`
	EBITDAPct       [5]decimal.Decimal `json:"ebitdaPct"`
	NetProfit       [5]decimal.Decimal `json:"netProfit"`
	NetProfitPct    [5]decimal.Decimal `json:"netProfitPct"`
	// CashFlow = NetProfit + Depreciation  (accounting / operating cash flow)
	CashFlow        [5]decimal.Decimal `json:"cashFlow"`
	CashFlowPct     [5]decimal.Decimal `json:"cashFlowPct"`
	// FreeCashFlow = CashFlow − CapEx − ΔWCR  (investable free cash flow)
	FreeCashFlow    [5]decimal.Decimal `json:"freeCashFlow"`
	FreeCashFlowPct [5]decimal.Decimal `json:"freeCashFlowPct"`
	CashAtEOY       [5]decimal.Decimal `json:"cashAtEoy"`
}

type RatiosEquityLeverage struct {
	InitialEquity     decimal.Decimal    `json:"initialEquity"`
	CapitalIncrease   [5]decimal.Decimal `json:"capitalIncrease"`
	TotalEquityEOY    [5]decimal.Decimal `json:"totalEquityEoy"`
	NetProfitMinusCap [5]decimal.Decimal `json:"netProfitMinusCap"`
	// FinancialReturn = NetProfit / TotalEquityEOY
	FinancialReturn   [5]decimal.Decimal `json:"financialReturn"`
	// TotalAssets: denominator for EquityToAssets; included for audit traceability
	TotalAssets       [5]decimal.Decimal `json:"totalAssets"`
	// EquityToAssets = TotalEquityEOY / TotalAssets
	EquityToAssets    [5]decimal.Decimal `json:"equityToAssets"`
	LTLoans           [5]decimal.Decimal `json:"ltLoans"`
	LTLoansToEquity   [5]decimal.Decimal `json:"ltLoansToEquity"`
	CashFlowToLoans   [5]decimal.Decimal `json:"cashFlowToLoans"`
	FinExpToEBITDA    [5]decimal.Decimal `json:"finExpToEbitda"`
	// WCR: working capital requirement in currency (base for WCRRotationDays)
	WCR               [5]decimal.Decimal `json:"wcr"`
	// WCRRotationDays = WCR / Sales × 365
	WCRRotationDays   [5]decimal.Decimal `json:"wcrRotationDays"`
}

type RatiosValuation struct {
	DiscountRate decimal.Decimal `json:"discountRate"`
	// NPV = Σ FreeCashFlow[y] / (1+r)^(y+1)  for y = 0..4
	NPV             decimal.Decimal `json:"npv"`
	IRR             decimal.Decimal `json:"irr"`
	IRRValid        bool            `json:"irrValid"`
	// TerminalValue = FreeCashFlow[4] / max(discountRate, 5%)  discounted to today
	// Represents perpetuity value of steady-state FCF beyond year 5
	TerminalValue   decimal.Decimal `json:"terminalValue"`
	// DiscountedValue = NPV + TerminalValue  (enterprise value estimate)
	DiscountedValue decimal.Decimal `json:"discountedValue"`
	// PEMultiple = DiscountedValue / NetProfit[4]  (DCF-implied earnings multiple)
	PEMultiple      decimal.Decimal `json:"peMultiple"`
}

type RatiosCharts struct {
	Summary         interface{} `json:"summary"`
	Waterfall       interface{} `json:"waterfall"`
	RevenueByProduct interface{} `json:"revenueByProduct"`
	CashFlowComponents interface{} `json:"cashFlowComponents"`
	ProfitabilityChart interface{} `json:"profitabilityChart"`
}

// ──────────────────────────────────────────────────────────────────────────
// FullPlanOutput — master aggregation of all computed reports
// ──────────────────────────────────────────────────────────────────────────

// FullPlanOutput represents the complete plan output with all reports.
type FullPlanOutput struct {
	Revenue  ConsolidatedRevenue `json:"revenue"`
	Payroll  StaffPayrollSummary `json:"payroll"`
	Capex    CapexSummary        `json:"capex"`
	Opex     OpexSummary         `json:"opex"`
	PnL      PnlReport           `json:"pnl"`
	FiPlan   FiplanReport        `json:"fiplan"`
	PnlCash  PnlCashReport       `json:"pnlCash"`
	BSheet   BSheetReport        `json:"bsheet"`
	Ratios   RatiosReport        `json:"ratios"`
	WCR      WCRReport           `json:"wcr"`
	Cash     CashReport          `json:"cash"`
	Budget1  Budget1Report       `json:"budget1"`
	Budget2  Budget2Report       `json:"budget2"`
	Graphs   GraphsReport        `json:"graphs"`
	Graphs2  Graphs2Report       `json:"graphs2"`
	Warnings []ValidationWarning `json:"warnings"`
}
