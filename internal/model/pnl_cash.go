package model

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// PnlCashLineID for the Anglo-Saxon functional P&L.
type PnlCashLineID string

const (
	PnlCashMiscSalesCosts PnlCashLineID = "misc_sales_costs"
)

// PnlCashEntry stores the single user-input line for the Anglo-Saxon P&L.
type PnlCashEntry struct {
	TenantScoped
	ScenarioID uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:uix_pnl_cash_entries" json:"scenarioId"`
	LineID     PnlCashLineID   `gorm:"type:varchar(100);not null;uniqueIndex:uix_pnl_cash_entries" json:"lineId"`
	YearIndex  int             `gorm:"column:year;not null;uniqueIndex:uix_pnl_cash_entries" json:"yearIndex"` // DB column: year
	Amount     decimal.Decimal `gorm:"type:numeric(15,2)" json:"amount"`
}

func (PnlCashEntry) TableName() string {
	return "pnl_cash_entries"
}

// PnlCashReport is the computed Anglo-Saxon P&L output (not stored in DB).
type PnlCashReport struct {
	Years     [5]PnlCashYear   `json:"years"`
	ChartData PnlCashChartData `json:"chartData"`
}

type PnlCashYear struct {
	Year              int             `json:"year"`
	YearIndex         int             `json:"yearIndex"`
	Sales             decimal.Decimal `json:"sales"`
	CostOfSales       decimal.Decimal `json:"costOfSales"`
	GrossMargin       decimal.Decimal `json:"grossMargin"`
	RDPayroll         decimal.Decimal `json:"rdPayroll"`
	OutsourcedRD      decimal.Decimal `json:"outsourcedRd"`
	RoyaltiesMisc     decimal.Decimal `json:"royaltiesMisc"`
	SalesPayroll      decimal.Decimal `json:"salesPayroll"`
	AdvertisingPromo  decimal.Decimal `json:"advertisingPromo"`
	MiscSalesCosts    decimal.Decimal `json:"miscSalesCosts"`
	GAPayroll         decimal.Decimal `json:"gaPayroll"`
	InsuranceRent     decimal.Decimal `json:"insuranceRent"`
	LeasedEquip       decimal.Decimal `json:"leasedEquip"`
	LegalConsulting   decimal.Decimal `json:"legalConsulting"`
	TravelMisc        decimal.Decimal `json:"travelMisc"`
	Depreciation      decimal.Decimal `json:"depreciation"`
	EBIT              decimal.Decimal `json:"ebit"`
	InterestExpense   decimal.Decimal `json:"interestExpense"`
	Subsidies         decimal.Decimal `json:"subsidies"`
	TaxesIncurred     decimal.Decimal `json:"taxesIncurred"`
	NetProfit         decimal.Decimal `json:"netProfit"`
	// Percentage columns
	SalesPct          decimal.Decimal `json:"salesPct"`
}

type PnlCashChartData struct {
	Years          [5]int             `json:"years"`
	CostOfSales    [5]decimal.Decimal `json:"costOfSales"`
	RDProduction   [5]decimal.Decimal `json:"rdProduction"`
	SalesMarketing [5]decimal.Decimal `json:"salesMarketing"`
	GeneralAdmin   [5]decimal.Decimal `json:"generalAdmin"`
	EBITNegative   [5]decimal.Decimal `json:"ebitNegative"`
	EBITPositive   [5]decimal.Decimal `json:"ebitPositive"`
}
