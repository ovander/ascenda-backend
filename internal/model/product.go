package model

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// FlexInt64 is an int64 that can be unmarshalled from either a JSON number or a
// JSON string (e.g. "10" or 10).  The frontend may store integer driver-param
// arrays as string arrays when patchArr coerces values with String(); this type
// keeps the backend tolerant of both representations so old saved data still works.
type FlexInt64 int64

func (f *FlexInt64) UnmarshalJSON(data []byte) error {
	// Fast path: plain JSON number
	var n int64
	if err := json.Unmarshal(data, &n); err == nil {
		*f = FlexInt64(n)
		return nil
	}
	// Fallback: quoted string like "10" or "10.0"
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	s = strings.TrimSpace(s)
	if fv, err := strconv.ParseFloat(s, 64); err == nil {
		*f = FlexInt64(int64(math.Round(fv)))
		return nil
	}
	iv, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*f = FlexInt64(iv)
	return nil
}

func (f FlexInt64) MarshalJSON() ([]byte, error) {
	return json.Marshal(int64(f))
}

// DriverType enumerates the supported business driver frameworks.
type DriverType string

const (
	DriverGeneric      DriverType = "generic"
	DriverSaaS         DriverType = "saas"
	DriverConsulting   DriverType = "consulting"
	DriverIndustry     DriverType = "industry"
	DriverMarketplace  DriverType = "marketplace"
	DriverMedia        DriverType = "media"
	DriverSessionBased DriverType = "session_based"
)

// ---------------------------------------------------------------------------
// Typed parameter structs — one per driver type.
// These are stored as JSONB in Product.DriverParams.
// ---------------------------------------------------------------------------

// ConsultingParams drives revenue/COGS from headcount × utilisation × daily rate.
//
//	BillableDays[y] = Headcount[y] × WorkingDays × UtilizationRate[y]
//	Revenue[y]      = BillableDays[y] × DailyRate[y]          (stored as BaseUnitPrice in ProductAssumption)
//	COGS[y]         = BillableDays[y] × CostPerDay[y]         (stored as RawMaterialCost in ProductAssumption)
type ConsultingParams struct {
	// Headcount[0..4] — number of billable FTEs per year (Y1..Y5).
	Headcount [5]decimal.Decimal `json:"headcount"`
	// WorkingDays — billable working days available per FTE per year (e.g. 220).
	WorkingDays int `json:"workingDays"`
	// UtilizationRate[0..4] — fraction of working days that are billable (0–1).
	UtilizationRate [5]decimal.Decimal `json:"utilizationRate"`
	// MonthlyGross[0..4] — average monthly gross salary per FTE per year (€).
	MonthlyGross [5]decimal.Decimal `json:"monthlyGross"`
	// EmployerCharges — employer charge factor on top of gross (e.g. 1.45 for France).
	EmployerCharges decimal.Decimal `json:"employerCharges"`
}

// SaaSParams drives revenue/COGS from active-user count × monthly fee.
//
//	Revenue[y] = ActiveUsers[y] × MonthlyFee[y] × 12
//	COGS[y]    = ActiveUsers[y] × (InfraCostPPU[y] + SupportCostPPU[y]) × 12
type SaaSParams struct {
	// ActiveUsers[0..4] — paying active users at year-end per year.
	ActiveUsers [5]FlexInt64 `json:"activeUsers"`
	// MonthlyFee[0..4] — average monthly revenue per user (€).
	MonthlyFee [5]decimal.Decimal `json:"monthlyFee"`
	// InfraCostPPU[0..4] — monthly infrastructure cost per user (€).
	InfraCostPPU [5]decimal.Decimal `json:"infraCostPerUser"`
	// SupportCostPPU[0..4] — monthly support / CS cost per user (€).
	SupportCostPPU [5]decimal.Decimal `json:"supportCostPerUser"`
	// ChurnRate[0..4] — optional monthly churn rate (0–1) for cohort modelling.
	ChurnRate [5]decimal.Decimal `json:"churnRate,omitempty"`
	// ExpansionRate[0..4] — optional net revenue expansion rate (0–1).
	ExpansionRate [5]decimal.Decimal `json:"expansionRate,omitempty"`
}

// IndustryParams drives revenue/COGS from manufactured / shipped unit volume.
//
//	Revenue[y] = Units[y] × UnitPrice[y]   (UnitPrice from ProductAssumption.BaseUnitPrice)
//	COGS[y]    = Units[y] × UnitCost[y]    (UnitCost from ProductAssumption total)
type IndustryParams struct {
	// ProductionCapacity[0..4] — max units producible per year (0 = unconstrained).
	ProductionCapacity [5]FlexInt64 `json:"productionCapacity"`
	// ScrapRate[0..4] — fraction of production lost to defects (0–1).
	ScrapRate [5]decimal.Decimal `json:"scrapRate"`
	// SetupCost[0..4] — fixed per-batch or tooling setup cost (€/year).
	SetupCost [5]decimal.Decimal `json:"setupCost"`
	// UnitsProduced[0..4] — actual units manufactured per year (0 = same as UnitsSold,
	// i.e. produce-to-order; no finished-goods inventory build-up).
	// When UnitsProduced[y] > UnitsSold[y] the surplus creates finished-goods
	// inventory whose cost feeds P&L StoredProduction and the WCR Inventory line.
	// When UnitsProduced[y] < UnitsSold[y] the inventory drawdown reduces
	// StoredProduction (negative contribution to P&L operating revenue).
	UnitsProduced [5]FlexInt64 `json:"unitsProduced"`
}

// MarketplaceParams drives revenue/COGS from transaction volume × take rate.
//
//	Revenue[y] = Transactions[y] × GMV_PerTx[y] × TakeRate[y]
//	COGS[y]    = Transactions[y] × PaymentCost[y] + FixedInfraCost[y]
type MarketplaceParams struct {
	// Transactions[0..4] — number of transactions per year.
	Transactions [5]FlexInt64 `json:"transactions"`
	// GMVPerTransaction[0..4] — average gross merchandise value per transaction (€).
	GMVPerTransaction [5]decimal.Decimal `json:"gmvPerTransaction"`
	// TakeRate[0..4] — platform commission on GMV (0–1).
	TakeRate [5]decimal.Decimal `json:"takeRate"`
	// PaymentCost[0..4] — variable payment processing cost per transaction (€).
	PaymentCost [5]decimal.Decimal `json:"paymentCost"`
	// FixedInfraCost[0..4] — fixed annual infrastructure cost (€).
	FixedInfraCost [5]decimal.Decimal `json:"fixedInfraCost"`
}

// MediaParams drives revenue/COGS from audience size × CPM / fill rate.
//
//	Revenue[y] = (Impressions[y] / 1000) × CPM[y] × FillRate[y]
//	COGS[y]    = ContentCost[y] + (Impressions[y] × DeliveryCostPerImpression[y])
type MediaParams struct {
	// Impressions[0..4] — annual served impressions / views.
	Impressions [5]FlexInt64 `json:"impressions"`
	// CPM[0..4] — cost per mille (revenue per 1 000 impressions, €).
	CPM [5]decimal.Decimal `json:"cpm"`
	// FillRate[0..4] — fraction of impressions monetised (0–1).
	FillRate [5]decimal.Decimal `json:"fillRate"`
	// ContentCost[0..4] — fixed annual content production / licensing cost (€).
	ContentCost [5]decimal.Decimal `json:"contentCost"`
	// DeliveryCostPerImpression[0..4] — variable CDN / delivery cost per impression (€).
	DeliveryCostPerImpression [5]decimal.Decimal `json:"deliveryCostPerImpression"`
}

// SessionBasedParams drives revenue/COGS from training or event sessions.
//
// Economic model:
//
//	ActualSessions[y]      = min(Sessions[y], TrainerCount[y] × SessionsPerTrainer[y] × UtilizationRate[y])
//	RealizedParticipants[y] = ParticipantsPerSession[y] × FillRate[y]
//	UnitPrice[y]            = RealizedParticipants[y] × PricePerParticipant[y]
//	UnitCost[y]             = TrainerCostPerSession[y] + VariableCostPerParticipant[y] × RealizedParticipants[y]
//	Volume[y]               = ActualSessions[y]           (integer sessions, capacity-clamped)
type SessionBasedParams struct {
	// Sessions[0..4] — planned number of sessions per year (before capacity clamping).
	Sessions [5]FlexInt64 `json:"sessions"`
	// ParticipantsPerSession[0..4] — design capacity: max participants per session.
	ParticipantsPerSession [5]decimal.Decimal `json:"participantsPerSession"`
	// FillRate[0..4] — fraction of session capacity that is actually filled (0–1).
	FillRate [5]decimal.Decimal `json:"fillRate"`
	// PricePerParticipant[0..4] — revenue charged per attending participant (€).
	PricePerParticipant [5]decimal.Decimal `json:"pricePerParticipant"`
	// TrainerCount[0..4] — number of trainers/facilitators available per year.
	TrainerCount [5]decimal.Decimal `json:"trainerCount"`
	// SessionsPerTrainer[0..4] — maximum sessions one trainer can deliver per year.
	SessionsPerTrainer [5]FlexInt64 `json:"sessionsPerTrainer"`
	// UtilizationRate[0..4] — fraction of trainer capacity actually scheduled (0–1).
	UtilizationRate [5]decimal.Decimal `json:"utilizationRate"`
	// TrainerCostPerSession[0..4] — fixed cost per session (trainer fee, venue, etc.) (€).
	TrainerCostPerSession [5]decimal.Decimal `json:"trainerCostPerSession"`
	// VariableCostPerParticipant[0..4] — variable cost per attending participant (€).
	VariableCostPerParticipant [5]decimal.Decimal `json:"variableCostPerParticipant"`
}

// SalesChannel enumeration for product sales channels
type SalesChannel string

const (
	ChannelDirect   SalesChannel = "direct"
	ChannelIndirect SalesChannel = "indirect"
)

// GeoZone enumeration for geographic zones
type GeoZone string

const (
	ZoneFrance GeoZone = "france"
	ZoneEurope GeoZone = "europe"
	ZoneExport GeoZone = "export"
)

// ProductType enumeration
type ProductType string

const (
	ProductTypeProduct ProductType = "product"
	ProductTypeService ProductType = "service"
)

// Product represents a product or service line
type Product struct {
	TenantScoped
	ScenarioID                uuid.UUID       `gorm:"type:uuid;not null;index" json:"scenarioId"`
	Name                      string          `gorm:"type:varchar(255);not null" json:"name"`
	ProductType               ProductType     `gorm:"type:varchar(50);not null;default:'product'" json:"productType"`
	SortOrder                 int             `gorm:"not null;default:0" json:"sortOrder"`
	DirectCostVariability     decimal.Decimal `gorm:"type:numeric(5,4)" json:"directCostVariability"`
	ExternalChargeVariability decimal.Decimal `gorm:"type:numeric(5,4)" json:"externalChargeVariability"`
	TaxVariability            decimal.Decimal `gorm:"type:numeric(5,4)" json:"taxVariability"`
	StaffVariability          decimal.Decimal `gorm:"type:numeric(5,4)" json:"staffVariability"`
	DepreciationVariability   decimal.Decimal `gorm:"type:numeric(5,4)" json:"depreciationVariability"`

	// Business Driver Framework — Phase 1
	// DriverType selects which compute formula drives Volume → Revenue / COGS.
	// Defaults to "generic" so all existing products are fully backward-compatible.
	DriverType DriverType `gorm:"type:varchar(50);not null;default:'generic'" json:"driverType"`
	// DriverParams holds the typed parameter struct for the selected driver, serialised
	// as JSONB.  Null for generic products that use manual ProductAssumption inputs.
	DriverParams json.RawMessage `gorm:"type:jsonb"                                  json:"driverParams,omitempty"`
}

// TableName specifies the table name for Product
func (Product) TableName() string {
	return "products"
}

// ProductAssumption stores cost and pricing inputs per product per year.
// Maps to P1 rows 24–35 (Key Assumptions section).
type ProductAssumption struct {
	TenantScoped
	ProductID        uuid.UUID       `gorm:"type:uuid;not null;index" json:"productId"`
	YearIndex        int             `gorm:"column:year;not null" json:"yearIndex"` // 1–5
	RawMaterialCost  decimal.Decimal `gorm:"type:numeric(15,4)" json:"rawMaterialCost"`
	RoyaltiesCost    decimal.Decimal `gorm:"type:numeric(15,4)" json:"royaltiesCost"`
	LogisticsCost    decimal.Decimal `gorm:"type:numeric(15,4)" json:"logisticsCost"`
	CostCoefficient  decimal.Decimal `gorm:"type:numeric(8,4);default:1.0" json:"costCoefficient"`
	PriceCoefficient decimal.Decimal `gorm:"type:numeric(8,4);default:1.0" json:"priceCoefficient"`
	BaseUnitPrice    decimal.Decimal `gorm:"type:numeric(15,4)" json:"baseUnitPrice"`
	IsOverridden     bool            `gorm:"not null;default:false" json:"isOverridden"`
}

// TableName specifies the table name for ProductAssumption
func (ProductAssumption) TableName() string {
	return "product_assumptions"
}

// ProductAssumptionComputed wraps ProductAssumption with computed fields.
// Not stored in DB — returned by the service.
type ProductAssumptionComputed struct {
	ProductAssumption

	// TotalUnitCost = (RawMaterialCost + RoyaltiesCost + LogisticsCost) × CostCoefficient
	TotalUnitCost decimal.Decimal `json:"totalUnitCost"`

	// FinalUnitPrice = BaseUnitPrice × PriceCoefficient
	FinalUnitPrice decimal.Decimal `json:"finalUnitPrice"`
}

// ProductSalesVolume stores sales volume input for a product × zone × channel × year combination.
// Maps to P1 rows 36–71 (Direct and Indirect Sales sections).
type ProductSalesVolume struct {
	TenantScoped
	ProductID uuid.UUID    `gorm:"type:uuid;not null;index" json:"productId"`
	YearIndex int          `gorm:"column:year;not null" json:"yearIndex"` // 1–5
	Zone      GeoZone      `gorm:"type:varchar(20);not null" json:"zone"`
	Channel   SalesChannel `gorm:"type:varchar(20);not null" json:"channel"`
	UnitsSold int64        `gorm:"not null;default:0" json:"unitsSold"`
}

// TableName specifies the table name for ProductSalesVolume
func (ProductSalesVolume) TableName() string {
	return "product_sales_volumes"
}

// ProductDistributorMargin stores the distributor margin % per zone per year.
// Maps to P1 rows 49–71 (Indirect Sales section).
type ProductDistributorMargin struct {
	TenantScoped
	ProductID     uuid.UUID       `gorm:"type:uuid;not null;index" json:"productId"`
	YearIndex     int             `gorm:"column:year;not null" json:"yearIndex"` // 1–5
	Zone          GeoZone         `gorm:"type:varchar(20);not null" json:"zone"`
	MarginPercent decimal.Decimal `gorm:"type:numeric(8,4)" json:"marginPercent"`
	IsOverridden  bool            `gorm:"not null;default:false" json:"isOverridden"`
}

// TableName specifies the table name for ProductDistributorMargin
func (ProductDistributorMargin) TableName() string {
	return "product_distributor_margins"
}

// ProductRevenueSummary is the computed output for one product across all years.
// Matches P1 rows 5–20 (Revenue Summary section).
// Not stored in DB — generated by the compute engine.
type ProductRevenueSummary struct {
	ProductID   uuid.UUID             `json:"productId"`
	ProductName string                `json:"productName"`
	Years       [5]ProductRevenueYear `json:"years"`
	// SurplusInventoryValue[y] is non-zero only for DriverIndustry products
	// where UnitsProduced[y] != UnitsSold[y].  It holds the cumulative finished-
	// goods inventory value (in plan currency) at end of year y, computed as:
	//   openingInventory + (UnitsProduced - UnitsSold) × effectiveUnitCost
	// A negative value means inventory has been drawn down below zero, which is
	// clamped to zero in the WCR layer (cannot sell what hasn't been produced).
	SurplusInventoryValue [5]decimal.Decimal `json:"surplusInventoryValue"`
}

// ProductRevenueYear contains annual revenue data for a product.
type ProductRevenueYear struct {
	Year      int `json:"year"`      // calendar year (e.g. 2025)
	YearIndex int `json:"yearIndex"` // 1–5

	// Row 5: Total Turnover [k]
	Turnover decimal.Decimal `json:"turnover"`

	// Row 6: Europe & Export Sales [k]
	EuropeExportSales decimal.Decimal `json:"europeExportSales"`

	// Row 7: Direct Sales [k]
	DirectSalesTotal decimal.Decimal `json:"directSalesTotal"`

	// Rows 9–15: Unit Sales breakdowns
	TotalUnitSales     int64 `json:"totalUnitSales"`     // Row 9
	FranceUnitSales    int64 `json:"franceUnitSales"`    // Row 10
	EuropeUnitSales    int64 `json:"europeUnitSales"`    // Row 11
	ExportUnitSales    int64 `json:"exportUnitSales"`    // Row 12
	CumulatedUnitSales int64 `json:"cumulatedUnitSales"` // Row 13
	DirectUnitSales    int64 `json:"directUnitSales"`    // Row 14
	IndirectUnitSales  int64 `json:"indirectUnitSales"`  // Row 15

	// Row 17: Cost of Goods Sold [k]
	COGS decimal.Decimal `json:"cogs"`

	// Rows 19–20: Gross Margin
	GrossMargin    decimal.Decimal `json:"grossMargin"`    // [k]
	GrossMarginPct decimal.Decimal `json:"grossMarginPct"` // %

	// Per-zone detail (for grid display)
	Zones [3]ProductRevenueZoneYear `json:"zones"`

	// Percentage columns (H, I in spreadsheet)
	PctOfYear1Turnover *decimal.Decimal `json:"pctOfYear1Turnover,omitempty"` // Col H
	PctOfYear3Turnover *decimal.Decimal `json:"pctOfYear3Turnover,omitempty"` // Col I
}

// ProductRevenueZoneYear contains per-zone revenue breakdown for a product in a single year.
type ProductRevenueZoneYear struct {
	Zone GeoZone `json:"zone"`

	// Direct channel
	DirectUnitsSold int64           `json:"directUnitsSold"`
	DirectUnitPrice decimal.Decimal `json:"directUnitPrice"` // final price = base × priceCoeff
	DirectRevenue   decimal.Decimal `json:"directRevenue"`   // [k]

	// Indirect channel (distributors)
	IndirectUnitsSold     int64           `json:"indirectUnitsSold"`
	EndUserPrice          decimal.Decimal `json:"endUserPrice"`          // same as direct price
	DistributorMarginPct  decimal.Decimal `json:"distributorMarginPct"`  // from ProductDistributorMargin
	CustDistributorCoeff  decimal.Decimal `json:"custDistributorCoeff"`  // 1 / (1 - margin%)
	DistributorPrice      decimal.Decimal `json:"distributorPrice"`      // end-user price × (1 - margin%)
	IndirectRevenue       decimal.Decimal `json:"indirectRevenue"`       // [k]
	CompanyGrossMarginPct decimal.Decimal `json:"companyGrossMarginPct"` // (distrib price - unit cost) / distrib price
}

// ConsolidatedRevenue aggregates all products' revenues into a company-level view.
// Matches the "Revenues" sheet in the spreadsheet.
// Not stored in DB — generated by the compute engine.
type ConsolidatedRevenue struct {
	ScenarioID uuid.UUID                  `json:"scenarioId"`
	Products   []ProductRevenueSummary    `json:"products"`
	Totals     [5]ConsolidatedRevenueYear `json:"totals"`
}

// ConsolidatedRevenueYear contains aggregated annual revenue totals across all products.
type ConsolidatedRevenueYear struct {
	Year               int             `json:"year"`
	TotalTurnover      decimal.Decimal `json:"totalTurnover"` // sum of all products
	TotalDirectSales   decimal.Decimal `json:"totalDirectSales"`
	TotalIndirectSales decimal.Decimal `json:"totalIndirectSales"`
	EuropeExportSales  decimal.Decimal `json:"europeExportSales"`
	TotalCOGS          decimal.Decimal `json:"totalCogs"`
	TotalGrossMargin   decimal.Decimal `json:"totalGrossMargin"`
	GrossMarginPct     decimal.Decimal `json:"grossMarginPct"`
	TotalUnitSales     int64           `json:"totalUnitSales"`
	// TotalSurplusInventory is the sum of SurplusInventoryValue across all
	// DriverIndustry products.  When non-zero the WCR compute uses this value
	// for the Inventory line instead of TotalCOGS × inventoryPct, which would
	// understate or overstate finished-goods stock for manufacturing scenarios.
	TotalSurplusInventory decimal.Decimal `json:"totalSurplusInventory"`
}

// ProductSegmentRow represents a segment breakdown row for reporting.
type ProductSegmentRow struct {
	Label  string                 `json:"label"`
	Values [5]ProductSegmentValue `json:"values"`
}

// ProductSegmentValue holds a single segment value with unit and percent.
type ProductSegmentValue struct {
	Value   decimal.Decimal `json:"value"`
	Percent decimal.Decimal `json:"percent"`
}

// ProductUnitsRow represents a units breakdown row for reporting.
type ProductUnitsRow struct {
	Label  string               `json:"label"`
	Values [5]ProductUnitsValue `json:"values"`
}

// ProductUnitsValue holds unit sales data with cumulative tracking.
type ProductUnitsValue struct {
	Units      int64           `json:"units"`
	Cumulative int64           `json:"cumulative"`
	Percent    decimal.Decimal `json:"percent"`
}
