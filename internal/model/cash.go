package model

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// CashLineID for monthly cash flow overrides.
type CashLineID string

const (
	CashOtherRevenues      CashLineID = "other_revenues"
	CashOpexRentTelecom    CashLineID = "opex_rent_telecom"
	CashOpexLeasing        CashLineID = "opex_leasing"
	CashOpexFees           CashLineID = "opex_fees"
	CashOpexRoyalties      CashLineID = "opex_royalties"
	CashOpexTravel         CashLineID = "opex_travel"
	CashOpexAdvertising    CashLineID = "opex_advertising"
	CashOpexOther          CashLineID = "opex_other"
	CashCapexLandBuilding  CashLineID = "capex_land_building"
	CashCapexPatentsRD     CashLineID = "capex_patents_rd"
	CashCapexPrototypes    CashLineID = "capex_prototypes"
	CashCapexITVehicles    CashLineID = "capex_it_vehicles"
	CashCorporateTax       CashLineID = "corporate_tax"
	CashVATPayments        CashLineID = "vat_payments"
	CashCapitalIncrease    CashLineID = "capital_increase"
	CashDividends          CashLineID = "dividends"
	CashCurrentAccount     CashLineID = "current_account"
	CashLTLoans            CashLineID = "lt_loans"
	CashRepayableGrants    CashLineID = "repayable_grants"
	CashLoanRepayment      CashLineID = "loan_repayment"
	CashGrantRepayment     CashLineID = "grant_repayment"
	CashDiscountingExpense CashLineID = "discounting_expense"
	CashOverdraftInterest  CashLineID = "overdraft_interest"
	CashUnitsDirectSub     CashLineID = "units_direct"
	CashUnitsIndirectSub   CashLineID = "units_indirect"
	CashHeadcountSub       CashLineID = "headcount"
	CashIncentivesSub      CashLineID = "incentives"
)

// DistributionRule controls how annual values are distributed across months.
type DistributionRule string

const (
	DistEvenSpread   DistributionRule = "even_spread"
	DistLumpM1       DistributionRule = "lump_m1"
	DistFromSchedule DistributionRule = "from_schedule"
	DistManualOnly   DistributionRule = "manual_only"
)

// CashMonthlyOverride represents monthly cash overrides for specific years (sparse storage).
type CashMonthlyOverride struct {
	TenantScoped
	ScenarioID uuid.UUID       `gorm:"type:uuid;not null;index" json:"scenarioId"`
	LineID     CashLineID      `gorm:"type:varchar(100);not null" json:"lineId"`
	YearIndex  int             `gorm:"not null" json:"yearIndex"`          // 1-3
	Month      int             `gorm:"not null" json:"month"`              // 1-12
	SubIndex   int             `gorm:"not null;default:0" json:"subIndex"` // product/role index
	Amount     decimal.Decimal `gorm:"type:numeric(15,2)" json:"amount"`
}

func (CashMonthlyOverride) TableName() string {
	return "cash_monthly_overrides"
}

// CashReport is the computed 36-month cash flow output (not stored in DB).
type CashReport struct {
	Years     [3]CashYear   `json:"years"`
	Schedules CashSchedules `json:"schedules"`
}

type CashYear struct {
	YearIndex int         `json:"yearIndex"`
	Revenue   CashSection `json:"revenue"`
	Operating CashSection `json:"operating"`
	Capex     CashSection `json:"capex"`
	WCR       CashSection `json:"wcr"`
	Tax       CashSection `json:"tax"`
	Economic  CashSection `json:"economic"`
	Financing CashSection `json:"financing"`
	Cash      CashSection `json:"cash"`
	// Convenience arrays consumed by the frontend cash view
	NetCashFlow    [12]decimal.Decimal `json:"netCashFlow"`
	OpeningBalance [12]decimal.Decimal `json:"openingBalance"`
	ClosingBalance [12]decimal.Decimal `json:"closingBalance"`
}

type CashSection struct {
	Lines []CashLine          `json:"lines"`
	Total [12]decimal.Decimal `json:"total"`
}

type CashLine struct {
	LineID           CashLineID          `json:"lineId"`
	Label            string              `json:"label"`
	Monthly          [12]decimal.Decimal `json:"months"`
	Annual           decimal.Decimal     `json:"annualTotal"`
	DistributionRule DistributionRule    `json:"distributionRule"`
}

type CashSchedules struct {
	UnitSchedule    *CashUnitSchedule    `json:"unitSchedule,omitempty"`
	StaffSchedule   *CashStaffSchedule   `json:"staffSchedule,omitempty"`
	PaymentSchedule *CashPaymentSchedule `json:"paymentSchedule,omitempty"`
	VATSchedule     *CashVATSchedule     `json:"vatSchedule,omitempty"`
}

type CashUnitSchedule struct {
	DirectUnits   [][12]int64       `json:"directUnits"`
	IndirectUnits [][12]int64       `json:"indirectUnits"`
	UnitRevenue   []decimal.Decimal `json:"unitRevenue"`
	UnitCOGS      []decimal.Decimal `json:"unitCogs"`
}

type CashStaffSchedule struct {
	Headcount     [][12]decimal.Decimal `json:"headcount"`
	GrossSalary   []decimal.Decimal     `json:"grossSalary"`
	NetSalaries   [12]decimal.Decimal   `json:"netSalaries"`
	SocialCharges [12]decimal.Decimal   `json:"socialCharges"`
	Incentives    [12]decimal.Decimal   `json:"incentives"`
	TotalPayroll  [12]decimal.Decimal   `json:"totalPayroll"`
}

type CashPaymentSchedule struct {
	CollectionByTranche [5][12]decimal.Decimal `json:"collectionByTranche"`
	COGSInclVAT         [12]decimal.Decimal    `json:"cogsInclVat"`
	PaymentByTranche    [5][12]decimal.Decimal `json:"paymentByTranche"`
	TotalPayments       [12]decimal.Decimal    `json:"totalPayments"`
}

type CashVATSchedule struct {
	Collected        [12]decimal.Decimal `json:"collected"`
	DeductibleOpex   [12]decimal.Decimal `json:"deductibleOpex"`
	DeductibleCapex  [12]decimal.Decimal `json:"deductibleCapex"`
	NetPayable       [12]decimal.Decimal `json:"netPayable"`
	EffectivePayment [12]decimal.Decimal `json:"effectivePayment"`
}
