package model

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// OpexSubcategory groups lines into the 7 sections of the Opex sheet.
type OpexSubcategory string

const (
	OpexSubPremises     OpexSubcategory = "premises"
	OpexSubLeasing      OpexSubcategory = "leasing"
	OpexSubProfessional OpexSubcategory = "professional"
	OpexSubRoyalties    OpexSubcategory = "royalties"
	OpexSubTravel       OpexSubcategory = "travel"
	OpexSubMarketing    OpexSubcategory = "marketing"
	OpexSubHR           OpexSubcategory = "hr"
)

// OpexLineID identifies each of the 22 expense lines.
type OpexLineID string

const (
	LinePropertyRentals        OpexLineID = "property_rentals"
	LinePostageTelecom         OpexLineID = "postage_telecom"
	LineSuppliesPurchases      OpexLineID = "supplies_purchases"
	LineStudiesDocumentation   OpexLineID = "studies_documentation"
	LineInsuranceCosts         OpexLineID = "insurance_costs"
	LineLeasingMovable         OpexLineID = "leasing_movable"
	LineLeasingRealEstate      OpexLineID = "leasing_real_estate"
	LineMaintenanceRepairs     OpexLineID = "maintenance_repairs"
	LineProfessionalFees       OpexLineID = "professional_fees"
	LineExternalStaffRnD       OpexLineID = "external_staff_rnd"
	LineRoyaltyPatents         OpexLineID = "royalty_patents"
	LineRoyaltyTrademarks      OpexLineID = "royalty_trademarks"
	LineTravelTransport        OpexLineID = "travel_transport"
	LineMissionRepresentation  OpexLineID = "mission_representation"
	LineDesignCreation         OpexLineID = "design_creation"
	LineAdvertisingComms       OpexLineID = "advertising_comms"
	LineTradeShows             OpexLineID = "trade_shows"
	LineTechCostsWebHosting    OpexLineID = "tech_costs_web"
	LinePromotionMerchandising OpexLineID = "promotion_merchandising"
	LineRecruitmentTraining    OpexLineID = "recruitment_training"
	LineOtherExpenses          OpexLineID = "other_expenses"
)

// OpexLineConfig defines the behavior of each line.
type OpexLineConfig struct {
	ID            OpexLineID
	Subcategory   OpexSubcategory
	SortOrder     int
	IsUserInput   bool
	CostDriver    string // "per_capita", "pct_of_sales", "special_rent", "pct_of_payroll", "manual"
	SettingsField string // which OpexPerHire field drives this line
}

// AllOpexLines in display order with their configuration.
var AllOpexLines = []OpexLineConfig{
	{LinePropertyRentals, OpexSubPremises, 1, false, "special_rent", "PropertyRentals"},
	{LinePostageTelecom, OpexSubPremises, 2, false, "per_capita", "PostageTelecom"},
	{LineSuppliesPurchases, OpexSubPremises, 3, false, "per_capita", "SuppliesPurchases"},
	{LineStudiesDocumentation, OpexSubPremises, 4, false, "per_capita", "StudiesDocumentation"},
	{LineInsuranceCosts, OpexSubPremises, 5, false, "pct_of_sales", "InsuranceCostsPctSales"},
	{LineLeasingMovable, OpexSubLeasing, 6, true, "manual", ""},
	{LineLeasingRealEstate, OpexSubLeasing, 7, true, "manual", ""},
	{LineMaintenanceRepairs, OpexSubLeasing, 8, true, "manual", ""},
	{LineProfessionalFees, OpexSubProfessional, 9, true, "manual", ""},
	{LineExternalStaffRnD, OpexSubProfessional, 10, true, "manual", ""},
	{LineRoyaltyPatents, OpexSubRoyalties, 11, false, "pct_of_sales", "RoyaltyPaymentsPctSales"},
	{LineRoyaltyTrademarks, OpexSubRoyalties, 12, true, "manual", ""},
	{LineTravelTransport, OpexSubTravel, 13, false, "per_capita", "TravelTransportation"},
	{LineMissionRepresentation, OpexSubTravel, 14, false, "per_capita", "MissionRepresentation"},
	{LineDesignCreation, OpexSubMarketing, 15, true, "manual", ""},
	{LineAdvertisingComms, OpexSubMarketing, 16, true, "manual", ""},
	{LineTradeShows, OpexSubMarketing, 17, true, "manual", ""},
	{LineTechCostsWebHosting, OpexSubMarketing, 18, true, "manual", ""},
	{LinePromotionMerchandising, OpexSubMarketing, 19, true, "manual", ""},
	{LineRecruitmentTraining, OpexSubHR, 20, false, "pct_of_payroll", "RecruitTrainingPctPayroll"},
	{LineOtherExpenses, OpexSubHR, 21, true, "manual", ""},
}

// UserInputOpexLines returns only the 12 lines that need DB storage.
func UserInputOpexLines() []OpexLineConfig {
	var result []OpexLineConfig
	for _, l := range AllOpexLines {
		if l.IsUserInput {
			result = append(result, l)
		}
	}
	return result
}

// OpexManualEntry represents manual opex line entries (12 user-input lines only).
type OpexManualEntry struct {
	TenantScoped
	ScenarioID uuid.UUID       `gorm:"type:uuid;not null;index" json:"scenarioId"`
	LineID     OpexLineID      `gorm:"type:varchar(100);not null" json:"lineId"`
	YearIndex  int             `gorm:"not null" json:"yearIndex"`
	Amount     decimal.Decimal `gorm:"type:numeric(15,2)" json:"amount"`
}

func (OpexManualEntry) TableName() string {
	return "opex_manual_entries"
}

// OpexSummary is the computed output for the Opex sheet (not stored in DB).
type OpexSummary struct {
	Subcategories []OpexSubcategoryResult `json:"subcategories"`
	GrandTotal    [5]decimal.Decimal      `json:"grandTotal"`
}

type OpexSubcategoryResult struct {
	Subcategory OpexSubcategory    `json:"subcategory"`
	Lines       []OpexLineResult   `json:"lines"`
	Subtotal    [5]decimal.Decimal `json:"subtotal"`
}

type OpexLineResult struct {
	LineID     OpexLineID         `json:"lineId"`
	IsUserInput bool              `json:"isUserInput"`
	CostDriver string             `json:"costDriver"`
	Years      [5]decimal.Decimal `json:"years"`
}
