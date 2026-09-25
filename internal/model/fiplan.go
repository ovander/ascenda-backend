package model

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// FiplanLineID constants for financial planning user inputs.
type FiplanLineID string

const (
	FiplanDividends             FiplanLineID = "dividends"
	FiplanGrantRepayments       FiplanLineID = "grant_repayments"
	FiplanCapitalIncrease       FiplanLineID = "capital_increase"
	FiplanCurrentAccountContrib FiplanLineID = "current_account_contrib"
	FiplanSubsidies             FiplanLineID = "subsidies"
	FiplanOtherGrants           FiplanLineID = "other_grants"
	FiplanRepayableGrants       FiplanLineID = "repayable_grants"
	FiplanLTLoans               FiplanLineID = "lt_loans"
	FiplanAssetSales            FiplanLineID = "asset_sales"
	FiplanDisposalGainsLosses   FiplanLineID = "disposal_gains_losses"
)

// AllFiplanInputLines lists all 10 user-input lines.
var AllFiplanInputLines = []FiplanLineID{
	FiplanDividends, FiplanGrantRepayments, FiplanCapitalIncrease,
	FiplanCurrentAccountContrib, FiplanSubsidies, FiplanOtherGrants,
	FiplanRepayableGrants, FiplanLTLoans, FiplanAssetSales,
	FiplanDisposalGainsLosses,
}

// FiplanEntry represents financial planning entries (10 user-input lines × 5 years).
type FiplanEntry struct {
	TenantScoped
	ScenarioID uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:uix_fiplan_entries" json:"scenarioId"`
	LineID     FiplanLineID    `gorm:"type:varchar(100);not null;uniqueIndex:uix_fiplan_entries" json:"lineId"`
	YearIndex  int             `gorm:"column:year;not null;uniqueIndex:uix_fiplan_entries" json:"yearIndex"` // DB column: year
	Amount     decimal.Decimal `gorm:"type:numeric(15,2)" json:"amount"`

	// Cap Table link — set when this capital_increase entry was pushed from a cap table round.
	// Nullable; non-capital_increase lines always have these nil/empty.
	CapTableRoundID    *uuid.UUID `gorm:"type:uuid;column:cap_table_round_id"           json:"capTableRoundId,omitempty"`
	CapTableRoundLabel string     `gorm:"type:varchar(100);column:cap_table_round_label" json:"capTableRoundLabel,omitempty"`
}

func (FiplanEntry) TableName() string {
	return "fiplan_entries"
}

// FiplanReport is the computed output (not stored in DB).
type FiplanReport struct {
	Plan     FiplanPlan     `json:"plan"`
	CashFlow FiplanCashFlow `json:"cashFlow"`
	Warning  [5]bool        `json:"warning"`

	// LoanInterest[y] is the interest charge on the outstanding debt balance for
	// year y (0-based). It feeds directly into P&L FinancialExpenses once
	// FiPlan is available (Layer-3 recompute in the engine).
	LoanInterest [5]decimal.Decimal `json:"loanInterest"`
}

type FiplanPlan struct {
	Requirements FiplanRequirements `json:"requirements"`
	Resources    FiplanResources    `json:"resources"`
	Balance      FiplanBalance      `json:"balance"`
}

type FiplanRequirements struct {
	Capex            [5]decimal.Decimal `json:"capex"`
	Dividends        [5]decimal.Decimal `json:"dividends"`
	NegativeCashFlow [5]decimal.Decimal `json:"negativeCashFlow"`
	WCRChange        [5]decimal.Decimal `json:"wcrChange"`
	LoanRepayments   [5]decimal.Decimal `json:"loanRepayments"`
	GrantRepayments  [5]decimal.Decimal `json:"grantRepayments"`
	Total            [5]decimal.Decimal `json:"total"`
}

type FiplanResources struct {
	CapitalIncrease    [5]decimal.Decimal `json:"capitalIncrease"`
	CurrentAccountCont [5]decimal.Decimal `json:"currentAccountCont"`
	PositiveCashFlow   [5]decimal.Decimal `json:"positiveCashFlow"`
	Subsidies          [5]decimal.Decimal `json:"subsidies"`
	OtherGrants        [5]decimal.Decimal `json:"otherGrants"`
	RepayableGrants    [5]decimal.Decimal `json:"repayableGrants"`
	LTLoans            [5]decimal.Decimal `json:"ltLoans"`
	AssetSales         [5]decimal.Decimal `json:"assetSales"`
	Total              [5]decimal.Decimal `json:"total"`
}

type FiplanBalance struct {
	AnnualBalance   [5]decimal.Decimal `json:"annualBalance"`
	CumulativeCash  [5]decimal.Decimal `json:"cumulativeCash"`
	BSheetCashCheck [5]decimal.Decimal `json:"bsheetCashCheck"`
	InitialCash     decimal.Decimal    `json:"initialCash"`
}

type FiplanCashFlow struct {
	Operating CashFlowOperating `json:"operating"`
	Investing CashFlowInvesting `json:"investing"`
	Financing CashFlowFinancing `json:"financing"`
	Summary   CashFlowSummary   `json:"summary"`
}

type CashFlowOperating struct {
	NetProfit        [5]decimal.Decimal `json:"netProfit"`
	Depreciation     [5]decimal.Decimal `json:"depreciation"`
	DisposalGainLoss [5]decimal.Decimal `json:"disposalGainLoss"`
	CashFlowCAF      [5]decimal.Decimal `json:"cashFlowCaf"`
	WCRChange        [5]decimal.Decimal `json:"wcrChange"`
	OperatingFlows   [5]decimal.Decimal `json:"operatingFlows"`
}

type CashFlowInvesting struct {
	CapexOutflow    [5]decimal.Decimal `json:"capexOutflow"`
	AssetDisposals  [5]decimal.Decimal `json:"assetDisposals"`
	InvestmentFlows [5]decimal.Decimal `json:"investmentFlows"`
}

type CashFlowFinancing struct {
	CapitalIncrease     [5]decimal.Decimal `json:"capitalIncrease"`
	CurrentAccountCont  [5]decimal.Decimal `json:"currentAccountCont"`
	NewLoansAndGrants   [5]decimal.Decimal `json:"newLoansAndGrants"`
	Dividends           [5]decimal.Decimal `json:"dividends"`
	LoanGrantRepayments [5]decimal.Decimal `json:"loanGrantRepayments"`
	FinancingFlows      [5]decimal.Decimal `json:"financingFlows"`
}

type CashFlowSummary struct {
	ChangeInCash   [5]decimal.Decimal `json:"changeInCash"`
	CumulativeCash [5]decimal.Decimal `json:"cumulativeCash"`
	InitialCash    decimal.Decimal    `json:"initialCash"`
}
