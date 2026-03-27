package model

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// AssetCategory maps to the 14 rows (rows 15–28) in the Capex sheet.
type AssetCategory string

const (
	AssetLand               AssetCategory = "land"
	AssetIntangibleBusiness AssetCategory = "intangible_business"
	AssetFinancial          AssetCategory = "financial"
	AssetBuildings          AssetCategory = "buildings"
	AssetSetupExpenses      AssetCategory = "setup_expenses"
	AssetPatentsTrademarks  AssetCategory = "patents_trademarks"
	AssetRnDExpenses        AssetCategory = "rnd_expenses"
	AssetOtherIntangible    AssetCategory = "other_intangible"
	AssetPrototypes         AssetCategory = "prototypes"
	AssetEquipmentTools     AssetCategory = "equipment_tools"
	AssetOfficeFurniture    AssetCategory = "office_furniture"
	AssetComputerHWSW       AssetCategory = "computer_hw_sw"
	AssetVehicles           AssetCategory = "vehicles"
	AssetOtherTangible      AssetCategory = "other_tangible"
)

var AllAssetCategories = []AssetCategory{
	AssetLand, AssetIntangibleBusiness, AssetBuildings, AssetSetupExpenses,
	AssetPatentsTrademarks, AssetRnDExpenses, AssetOtherIntangible, AssetPrototypes,
	AssetEquipmentTools, AssetOfficeFurniture, AssetComputerHWSW, AssetVehicles,
	AssetOtherTangible, AssetFinancial,
}

var NonDepreciableCategories = map[AssetCategory]bool{
	AssetLand: true, AssetIntangibleBusiness: true, AssetFinancial: true,
}

var StaffLinkedCategories = map[AssetCategory]bool{
	AssetOfficeFurniture: true, AssetComputerHWSW: true,
}

var DefaultDepreciationYears = map[AssetCategory]int{
	AssetBuildings: 10, AssetSetupExpenses: 5, AssetPatentsTrademarks: 20,
	AssetRnDExpenses: 4, AssetOtherIntangible: 4, AssetPrototypes: 5,
	AssetEquipmentTools: 5, AssetOfficeFurniture: 5, AssetComputerHWSW: 3,
	AssetVehicles: 5, AssetOtherTangible: 4,
}

// CapexEntry represents a capital expenditure entry per category per year.
type CapexEntry struct {
	TenantScoped
	ScenarioID        uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:uix_capex_entries" json:"scenarioId"`
	Category          AssetCategory   `gorm:"type:varchar(50);not null;uniqueIndex:uix_capex_entries" json:"category"`
	YearIndex         int             `gorm:"not null;uniqueIndex:uix_capex_entries" json:"yearIndex"`
	Amount            decimal.Decimal `gorm:"type:numeric(15,2)" json:"amount"`
	DepreciationYears int             `gorm:"not null" json:"depreciationYears"`
	IsManualOverride  bool            `gorm:"not null;default:false" json:"isManualOverride"`
}

func (CapexEntry) TableName() string {
	return "capex_entries"
}

// CapexSummary is the computed output for the Capex sheet (not stored in DB).
type CapexSummary struct {
	Investments         []CapexCategoryRow   `json:"investments"`
	Totals              CapexTotals          `json:"totals"`
	DepreciationSchedule []CapexDepreciationRow `json:"depreciationSchedule"`
}

type CapexCategoryRow struct {
	Category        AssetCategory      `json:"category"`
	IsDepreciable   bool               `json:"isDepreciable"`
	IsStaffLinked   bool               `json:"isStaffLinked"`
	DepreciationYears int              `json:"depreciationYears"`
	Years           [5]decimal.Decimal `json:"years"`
}

type CapexTotals struct {
	TotalCapex          [5]decimal.Decimal `json:"totalCapex"`
	PreviousNetAssets   decimal.Decimal    `json:"previousNetAssets"`
	PriorDepreciation   [5]decimal.Decimal `json:"priorDepreciation"`
	TotalDepreciation   [5]decimal.Decimal `json:"totalDepreciation"`
	NetAssets           [6]decimal.Decimal `json:"netAssets"`
}

type CapexDepreciationRow struct {
	Category          AssetCategory      `json:"category"`
	DepreciationYears int                `json:"depreciationYears"`
	Years             [5]decimal.Decimal `json:"years"`
}
