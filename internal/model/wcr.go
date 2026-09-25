package model

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// WCRLineID for working capital requirement user inputs.
type WCRLineID string

const (
	WCRPrepaidExpenses  WCRLineID = "prepaid_expenses"
	WCRDeferredRevenue  WCRLineID = "deferred_revenue"
	WCRTaxReceivables   WCRLineID = "tax_receivables"
	WCROtherAdjustPlus  WCRLineID = "other_adjust_plus"
	WCROtherAdjustMinus WCRLineID = "other_adjust_minus"
)

var AllWCRInputLines = []WCRLineID{
	WCRPrepaidExpenses, WCRDeferredRevenue, WCRTaxReceivables,
	WCROtherAdjustPlus, WCROtherAdjustMinus,
}

// WCREntry stores user-input WCR adjustment lines (5 lines × 5 years = 25 rows).
type WCREntry struct {
	TenantScoped
	ScenarioID uuid.UUID       `gorm:"type:uuid;not null;index" json:"scenarioId"`
	LineID     WCRLineID       `gorm:"type:varchar(100);not null" json:"lineId"`
	YearIndex  int             `gorm:"type:integer;not null" json:"yearIndex"`
	Amount     decimal.Decimal `gorm:"type:numeric(15,2);not null;default:0" json:"amount"`
}

func (WCREntry) TableName() string {
	return "wcr_entries"
}

// WCRConfigSnapshot captures the payment-timing configuration used for this
// computation. Included in WCRReport so audit trails and the UI can show the
// exact pct/days distribution without a separate settings API call.
type WCRConfigSnapshot struct {
	// Tranche delay buckets (always [0, 30, 60, 90, 120] days)
	Days [5]int `json:"days"`
	// Customer allocation weights per bucket (sums to 1)
	CustomerPcts [5]decimal.Decimal `json:"customerPcts"`
	// Supplier allocation weights per bucket (sums to 1)
	SupplierPcts [5]decimal.Decimal `json:"supplierPcts"`
	// Inventory % of COGS, one per year
	InventoryPcts [5]decimal.Decimal `json:"inventoryPcts"`
}

// WCRReport is the computed output (not stored in DB).
type WCRReport struct {
	VATRate      decimal.Decimal `json:"vatRate"`
	Customers    WCRCustomers    `json:"customers"`
	Inventory    WCRInventory    `json:"inventory"`
	Suppliers    WCRSuppliers    `json:"suppliers"`
	Summary      WCRSummary      `json:"summary"`
	FiscalSocial WCRFiscalSocial `json:"fiscalSocial"`
	Adjustments  WCRAdjustments  `json:"adjustments"`
	Adjusted     WCRAdjusted     `json:"adjusted"`
	Charts       WCRCharts       `json:"charts"`
	// EffectiveDSO = Σ(customerPct_k × days_k) — the weighted-average customer
	// collection delay in days implied by the tranche configuration.
	EffectiveDSO decimal.Decimal `json:"effectiveDso"`
	// EffectiveDPO = Σ(supplierPct_k × days_k) — the weighted-average supplier
	// payment delay in days implied by the tranche configuration.
	EffectiveDPO decimal.Decimal `json:"effectiveDpo"`
	// ConfigSnapshot embeds the pct/days distribution for full auditability.
	ConfigSnapshot WCRConfigSnapshot `json:"configSnapshot"`
}

type WCRCustomers struct {
	SalesExclVAT     [5]decimal.Decimal    `json:"salesExclVat"`
	ExportExclVAT    [5]decimal.Decimal    `json:"exportExclVat"`
	SalesInclTax     [5]decimal.Decimal    `json:"salesInclTax"`
	Tranches         [5][5]decimal.Decimal `json:"tranches"`
	TotalCustomers   [5]decimal.Decimal    `json:"totalCustomers"`
	InitialTradeRecv decimal.Decimal       `json:"initialTradeRecv"`
}

type WCRInventory struct {
	COGSBase         [5]decimal.Decimal `json:"cogsBase"`
	InventoryPct     [5]decimal.Decimal `json:"inventoryPct"`
	InventoryValue   [5]decimal.Decimal `json:"inventoryValue"`
	InitialInventory decimal.Decimal    `json:"initialInventory"`
}

type WCRSuppliers struct {
	COGSExclVAT     [5]decimal.Decimal    `json:"cogsExclVat"`
	ExternalExclVAT [5]decimal.Decimal    `json:"externalExclVat"`
	CapexExclVAT    [5]decimal.Decimal    `json:"capexExclVat"`
	TotalInclVAT    [5]decimal.Decimal    `json:"totalInclVat"`
	Tranches        [5][5]decimal.Decimal `json:"tranches"`
	TotalSuppliers  [5]decimal.Decimal    `json:"totalSuppliers"`
	InitialTradePay decimal.Decimal       `json:"initialTradePay"`
}

type WCRSummary struct {
	CustomerWCR  [5]decimal.Decimal `json:"customerWcr"`
	InventoryWCR [5]decimal.Decimal `json:"inventoryWcr"`
	SupplierWCR  [5]decimal.Decimal `json:"supplierWcr"`
	BasicWCR     [5]decimal.Decimal `json:"basicWcr"`
	BasicWCRDays [5]decimal.Decimal `json:"basicWcrDays"`
	InitialWCR   decimal.Decimal    `json:"initialWcr"`
	WCRChange    [5]decimal.Decimal `json:"wcrChange"`
}

type WCRFiscalSocial struct {
	VATCollected       [5]decimal.Decimal `json:"vatCollected"`
	VATDeductible      [5]decimal.Decimal `json:"vatDeductible"`
	NetVATPayable      [5]decimal.Decimal `json:"netVatPayable"`
	VATDaysOutstanding [5]decimal.Decimal `json:"vatDaysOutstanding"`
	VATLiability       [5]decimal.Decimal `json:"vatLiability"`
	EmployerCharges    [5]decimal.Decimal `json:"employerCharges"`
	EmployeeCharges    [5]decimal.Decimal `json:"employeeCharges"`
	TotalSocialCharges [5]decimal.Decimal `json:"totalSocialCharges"`
	SocialDaysOutstand [5]decimal.Decimal `json:"socialDaysOutstand"`
	SocialLiability    [5]decimal.Decimal `json:"socialLiability"`
	CorporateTaxLiab   [5]decimal.Decimal `json:"corporateTaxLiab"`
	TotalFiscalSocial  [5]decimal.Decimal `json:"totalFiscalSocial"`
}

type WCRAdjustments struct {
	PrepaidExpenses  [5]decimal.Decimal `json:"prepaidExpenses"`
	DeferredRevenue  [5]decimal.Decimal `json:"deferredRevenue"`
	TaxReceivables   [5]decimal.Decimal `json:"taxReceivables"`
	OtherAdjustPlus  [5]decimal.Decimal `json:"otherAdjustPlus"`
	OtherAdjustMinus [5]decimal.Decimal `json:"otherAdjustMinus"`
	NetAdjustment    [5]decimal.Decimal `json:"netAdjustment"`
}

type WCRAdjusted struct {
	AdjustedWCR       [5]decimal.Decimal `json:"adjustedWcr"`
	AdjustedWCRDays   [5]decimal.Decimal `json:"adjustedWcrDays"`
	AdjustedWCRChange [5]decimal.Decimal `json:"adjustedWcrChange"`
	InitialAdjWCR     decimal.Decimal    `json:"initialAdjWcr"`
}

type WCRCharts struct {
	Years           [5]int             `json:"years"`
	CustomerWCR     [5]decimal.Decimal `json:"customerWcr"`
	InventoryWCR    [5]decimal.Decimal `json:"inventoryWcr"`
	SupplierWCR     [5]decimal.Decimal `json:"supplierWcr"`
	FiscalSocialWCR [5]decimal.Decimal `json:"fiscalSocialWcr"`
	WCRChange       [5]decimal.Decimal `json:"wcrChange"`
}
