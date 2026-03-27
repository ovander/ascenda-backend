package dto

import (
	"encoding/json"

	"ascenda/internal/model"

	"github.com/shopspring/decimal"
)

// ProductResponse is the API representation of a product.
type ProductResponse struct {
	ID                        string          `json:"id"`
	ScenarioID                string          `json:"scenarioId"`
	Name                      string          `json:"name"`
	ProductType               string          `json:"productType"`
	SortOrder                 int             `json:"sortOrder"`
	DirectCostVariability     decimal.Decimal `json:"directCostVariability"`
	ExternalChargeVariability decimal.Decimal `json:"externalChargeVariability"`
	TaxVariability            decimal.Decimal `json:"taxVariability"`
	StaffVariability          decimal.Decimal `json:"staffVariability"`
	DepreciationVariability   decimal.Decimal `json:"depreciationVariability"`
	// Business Driver Framework — Phase 1
	DriverType   string          `json:"driverType"`
	DriverParams json.RawMessage `json:"driverParams,omitempty"`
	Timestamps
}

// ProductFromModel converts a model.Product to a ProductResponse.
func ProductFromModel(p model.Product) ProductResponse {
	return ProductResponse{
		ID:                        p.ID.String(),
		ScenarioID:                p.ScenarioID.String(),
		Name:                      p.Name,
		ProductType:               string(p.ProductType),
		SortOrder:                 p.SortOrder,
		DirectCostVariability:     p.DirectCostVariability,
		ExternalChargeVariability: p.ExternalChargeVariability,
		TaxVariability:            p.TaxVariability,
		StaffVariability:          p.StaffVariability,
		DepreciationVariability:   p.DepreciationVariability,
		DriverType:                string(p.DriverType),
		DriverParams:              p.DriverParams,
		Timestamps: Timestamps{
			CreatedAt: p.CreatedAt,
			UpdatedAt: p.UpdatedAt,
		},
	}
}

// ProductsFromModels converts a slice of model.Product to ProductResponse DTOs.
func ProductsFromModels(products []model.Product) []ProductResponse {
	out := make([]ProductResponse, len(products))
	for i, p := range products {
		out[i] = ProductFromModel(p)
	}
	return out
}

// ProductsFromPtrs converts a slice of *model.Product to ProductResponse DTOs.
func ProductsFromPtrs(products []*model.Product) []ProductResponse {
	out := make([]ProductResponse, len(products))
	for i, p := range products {
		out[i] = ProductFromModel(*p)
	}
	return out
}

// AssumptionResponse is the API representation of a product assumption.
type AssumptionResponse struct {
	ID               string          `json:"id"`
	ProductID        string          `json:"productId"`
	YearIndex        int             `json:"yearIndex"`
	RawMaterialCost  decimal.Decimal `json:"rawMaterialCost"`
	RoyaltiesCost    decimal.Decimal `json:"royaltiesCost"`
	LogisticsCost    decimal.Decimal `json:"logisticsCost"`
	CostCoefficient  decimal.Decimal `json:"costCoefficient"`
	PriceCoefficient decimal.Decimal `json:"priceCoefficient"`
	BaseUnitPrice    decimal.Decimal `json:"baseUnitPrice"`
	IsOverridden     bool            `json:"isOverridden"`
	Timestamps
}

// AssumptionFromModel converts a model.ProductAssumption to an AssumptionResponse.
func AssumptionFromModel(a model.ProductAssumption) AssumptionResponse {
	return AssumptionResponse{
		ID:               a.ID.String(),
		ProductID:        a.ProductID.String(),
		YearIndex:        a.YearIndex,
		RawMaterialCost:  a.RawMaterialCost,
		RoyaltiesCost:    a.RoyaltiesCost,
		LogisticsCost:    a.LogisticsCost,
		CostCoefficient:  a.CostCoefficient,
		PriceCoefficient: a.PriceCoefficient,
		BaseUnitPrice:    a.BaseUnitPrice,
		IsOverridden:     a.IsOverridden,
		Timestamps:       Timestamps{CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt},
	}
}

// AssumptionsFromModels converts a slice of model.ProductAssumption to DTOs.
func AssumptionsFromModels(assumptions []model.ProductAssumption) []AssumptionResponse {
	out := make([]AssumptionResponse, len(assumptions))
	for i, a := range assumptions {
		out[i] = AssumptionFromModel(a)
	}
	return out
}

// VolumeResponse is the API representation of a product sales volume.
type VolumeResponse struct {
	ID        string `json:"id"`
	ProductID string `json:"productId"`
	YearIndex int    `json:"yearIndex"`
	Zone      string `json:"zone"`
	Channel   string `json:"channel"`
	UnitsSold int64  `json:"unitsSold"`
	Timestamps
}

// VolumeFromModel converts a model.ProductSalesVolume to a VolumeResponse.
func VolumeFromModel(v model.ProductSalesVolume) VolumeResponse {
	return VolumeResponse{
		ID:         v.ID.String(),
		ProductID:  v.ProductID.String(),
		YearIndex:  v.YearIndex,
		Zone:       string(v.Zone),
		Channel:    string(v.Channel),
		UnitsSold:  v.UnitsSold,
		Timestamps: Timestamps{CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt},
	}
}

// VolumesFromModels converts a slice of model.ProductSalesVolume to DTOs.
func VolumesFromModels(volumes []model.ProductSalesVolume) []VolumeResponse {
	out := make([]VolumeResponse, len(volumes))
	for i, v := range volumes {
		out[i] = VolumeFromModel(v)
	}
	return out
}

// MarginResponse is the API representation of a product distributor margin.
type MarginResponse struct {
	ID            string          `json:"id"`
	ProductID     string          `json:"productId"`
	YearIndex     int             `json:"yearIndex"`
	Zone          string          `json:"zone"`
	MarginPercent decimal.Decimal `json:"marginPercent"`
	IsOverridden  bool            `json:"isOverridden"`
	Timestamps
}

// MarginFromModel converts a model.ProductDistributorMargin to a MarginResponse.
func MarginFromModel(m model.ProductDistributorMargin) MarginResponse {
	return MarginResponse{
		ID:            m.ID.String(),
		ProductID:     m.ProductID.String(),
		YearIndex:     m.YearIndex,
		Zone:          string(m.Zone),
		MarginPercent: m.MarginPercent,
		IsOverridden:  m.IsOverridden,
		Timestamps:    Timestamps{CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt},
	}
}

// MarginsFromModels converts a slice of model.ProductDistributorMargin to DTOs.
func MarginsFromModels(margins []model.ProductDistributorMargin) []MarginResponse {
	out := make([]MarginResponse, len(margins))
	for i, m := range margins {
		out[i] = MarginFromModel(m)
	}
	return out
}
