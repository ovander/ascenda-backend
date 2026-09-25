package model

import "github.com/shopspring/decimal"

// ──────────────────────────────────────────────────────────────────────────
// Annual Charts (Graphs page) — 2×2 dashboard
// ──────────────────────────────────────────────────────────────────────────

// GraphsReport is the computed annual chart data (not stored in DB).
type GraphsReport struct {
	Years           [5]int               `json:"years"`
	SalesAnalysis   GraphSalesAnalysis   `json:"salesAnalysis"`
	CostStructure   GraphCostStructure   `json:"costStructure"`
	RevProfitCash   GraphRevProfitCash   `json:"revProfitCash"`
	FinRequirements GraphFinRequirements `json:"finRequirements"`
	BalanceSheet    GraphBalanceSheet    `json:"balanceSheet"`
	HeadcountAnnual GraphHeadcountAnnual `json:"headcountAnnual"`
	PnLCascade      GraphPnLCascade      `json:"pnlCascade"`
}

type GraphSalesAnalysis struct {
	DirectSales   [5]decimal.Decimal `json:"directSales"`
	IndirectSales [5]decimal.Decimal `json:"indirectSales"`
	ExportSales   [5]decimal.Decimal `json:"exportSales"`
	DomesticSales [5]decimal.Decimal `json:"domesticSales"`
}

type GraphCostStructure struct {
	COGS         [5]decimal.Decimal `json:"cogs"`
	ExternalExp  [5]decimal.Decimal `json:"externalExp"`
	PayrollExp   [5]decimal.Decimal `json:"payrollExp"`
	Depreciation [5]decimal.Decimal `json:"depreciation"`
	OtherOpex    [5]decimal.Decimal `json:"otherOpex"`
}

type GraphRevProfitCash struct {
	Revenue        [5]decimal.Decimal `json:"revenue"`
	NetProfit      [5]decimal.Decimal `json:"netProfit"`
	CashFlow       [5]decimal.Decimal `json:"cashFlow"`
	CumulativeCash [5]decimal.Decimal `json:"cumulativeCash"`
}

type GraphFinRequirements struct {
	Requirements   [5]decimal.Decimal `json:"requirements"`
	Resources      [5]decimal.Decimal `json:"resources"`
	CumulativeCash [5]decimal.Decimal `json:"cumulativeCash"`
}

// GraphBalanceSheet holds year-end balance sheet structure (Equity / LT Debt / ST Debt).
type GraphBalanceSheet struct {
	Equity        [5]decimal.Decimal `json:"equity"`
	LongTermDebt  [5]decimal.Decimal `json:"longTermDebt"`
	ShortTermDebt [5]decimal.Decimal `json:"shortTermDebt"`
}

// GraphHeadcountAnnual holds annual FTE counts by function.
type GraphHeadcountAnnual struct {
	RnD        [5]decimal.Decimal `json:"rnd"`
	Production [5]decimal.Decimal `json:"production"`
	Sales      [5]decimal.Decimal `json:"sales"`
	GnA        [5]decimal.Decimal `json:"gna"`
	Total      [5]decimal.Decimal `json:"total"`
}

// GraphPnLCascade holds key P&L milestones for the cascade (waterfall-style) chart.
type GraphPnLCascade struct {
	Revenue     [5]decimal.Decimal `json:"revenue"`
	GrossMargin [5]decimal.Decimal `json:"grossMargin"`
	EBITDA      [5]decimal.Decimal `json:"ebitda"`
	EBIT        [5]decimal.Decimal `json:"ebit"`
	NetProfit   [5]decimal.Decimal `json:"netProfit"`
}

// ──────────────────────────────────────────────────────────────────────────
// Monthly Charts (Graphs2 page) — 2×2 dashboard, 36 months
// ──────────────────────────────────────────────────────────────────────────

// Graphs2Report is the computed monthly chart data (not stored in DB).
type Graphs2Report struct {
	Months          [36]string            `json:"months"`
	CashEquityDebt  Graph2CashEquityDebt  `json:"cashEquityDebt"`
	OperatingCash   Graph2OperatingCash   `json:"operatingCash"`
	InvoicingEBITDA Graph2InvoicingEBITDA `json:"invoicingEbitda"`
	HeadcountByFunc Graph2HeadcountByFunc `json:"headcountByFunc"`
}

type Graph2CashEquityDebt struct {
	CashBalance [36]decimal.Decimal `json:"cashBalance"`
	Equity      [36]decimal.Decimal `json:"equity"`
	TotalDebt   [36]decimal.Decimal `json:"totalDebt"`
}

type Graph2OperatingCash struct {
	Inflows  [36]decimal.Decimal `json:"inflows"`
	Outflows [36]decimal.Decimal `json:"outflows"`
	NetCash  [36]decimal.Decimal `json:"netCash"`
}

type Graph2InvoicingEBITDA struct {
	MonthlyRevenue [36]decimal.Decimal `json:"monthlyRevenue"`
	MonthlyEBITDA  [36]decimal.Decimal `json:"monthlyEbitda"`
	CumulRevenue   [36]decimal.Decimal `json:"cumulRevenue"`
}

type Graph2HeadcountByFunc struct {
	RnD        [36]decimal.Decimal `json:"rnd"`
	Production [36]decimal.Decimal `json:"production"`
	Sales      [36]decimal.Decimal `json:"sales"`
	GnA        [36]decimal.Decimal `json:"gna"`
	Total      [36]decimal.Decimal `json:"total"`
}
