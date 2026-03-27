package compute

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"ascenda/internal/model"
)

func TestComputeProductRevenue(t *testing.T) {
	config := model.PlanConfig{
		CorporateTaxRate:   decimal.NewFromFloat(0.25),
		EmployerTaxRate: decimal.NewFromFloat(0.42),
		IncentiveCap:       decimal.NewFromFloat(0.15),
	}

	scenarioID := uuid.New()
	productAID := uuid.New()
	productBID := uuid.New()
	productCID := uuid.New()

	tests := []struct {
		name     string
		product  model.Product
		bundle   ProductInputBundle
		config   model.PlanConfig
		expected struct {
			turnover   [5]decimal.Decimal
			cogs       [5]decimal.Decimal
			grossMargin [5]decimal.Decimal
		}
	}{
		{
			name: "single product with uniform direct volumes",
			product: model.Product{
				Name:       "Product A",
				ScenarioID: scenarioID,
			},
			bundle: ProductInputBundle{
				// Assumptions for all 5 years: BaseUnitPrice=100, costs=50
				Assumptions: [MaxYears]model.ProductAssumption{
					{
						ProductID:        productAID,
						YearIndex:        1,
						BaseUnitPrice:    decimal.NewFromInt(100),
						RawMaterialCost:  decimal.NewFromInt(50),
						RoyaltiesCost:    decimal.Zero,
						LogisticsCost:    decimal.Zero,
						CostCoefficient:  decimal.NewFromInt(1),
						PriceCoefficient: decimal.NewFromInt(1),
					},
					{
						ProductID:        productAID,
						YearIndex:        2,
						BaseUnitPrice:    decimal.NewFromInt(100),
						RawMaterialCost:  decimal.NewFromInt(50),
						RoyaltiesCost:    decimal.Zero,
						LogisticsCost:    decimal.Zero,
						CostCoefficient:  decimal.NewFromInt(1),
						PriceCoefficient: decimal.NewFromInt(1),
					},
					{
						ProductID:        productAID,
						YearIndex:        3,
						BaseUnitPrice:    decimal.NewFromInt(100),
						RawMaterialCost:  decimal.NewFromInt(50),
						RoyaltiesCost:    decimal.Zero,
						LogisticsCost:    decimal.Zero,
						CostCoefficient:  decimal.NewFromInt(1),
						PriceCoefficient: decimal.NewFromInt(1),
					},
					{
						ProductID:        productAID,
						YearIndex:        4,
						BaseUnitPrice:    decimal.NewFromInt(100),
						RawMaterialCost:  decimal.NewFromInt(50),
						RoyaltiesCost:    decimal.Zero,
						LogisticsCost:    decimal.Zero,
						CostCoefficient:  decimal.NewFromInt(1),
						PriceCoefficient: decimal.NewFromInt(1),
					},
					{
						ProductID:        productAID,
						YearIndex:        5,
						BaseUnitPrice:    decimal.NewFromInt(100),
						RawMaterialCost:  decimal.NewFromInt(50),
						RoyaltiesCost:    decimal.Zero,
						LogisticsCost:    decimal.Zero,
						CostCoefficient:  decimal.NewFromInt(1),
						PriceCoefficient: decimal.NewFromInt(1),
					},
				},
				// Volumes: 1000 units direct per zone per year (3000 total per year)
				Volumes: []model.ProductSalesVolume{
					// Year 1
					{ProductID: productAID, YearIndex: 1, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 1000},
					{ProductID: productAID, YearIndex: 1, Zone: model.ZoneEurope, Channel: model.ChannelDirect, UnitsSold: 1000},
					{ProductID: productAID, YearIndex: 1, Zone: model.ZoneExport, Channel: model.ChannelDirect, UnitsSold: 1000},
					// Year 2
					{ProductID: productAID, YearIndex: 2, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 1000},
					{ProductID: productAID, YearIndex: 2, Zone: model.ZoneEurope, Channel: model.ChannelDirect, UnitsSold: 1000},
					{ProductID: productAID, YearIndex: 2, Zone: model.ZoneExport, Channel: model.ChannelDirect, UnitsSold: 1000},
					// Year 3
					{ProductID: productAID, YearIndex: 3, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 1000},
					{ProductID: productAID, YearIndex: 3, Zone: model.ZoneEurope, Channel: model.ChannelDirect, UnitsSold: 1000},
					{ProductID: productAID, YearIndex: 3, Zone: model.ZoneExport, Channel: model.ChannelDirect, UnitsSold: 1000},
					// Year 4
					{ProductID: productAID, YearIndex: 4, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 1000},
					{ProductID: productAID, YearIndex: 4, Zone: model.ZoneEurope, Channel: model.ChannelDirect, UnitsSold: 1000},
					{ProductID: productAID, YearIndex: 4, Zone: model.ZoneExport, Channel: model.ChannelDirect, UnitsSold: 1000},
					// Year 5
					{ProductID: productAID, YearIndex: 5, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 1000},
					{ProductID: productAID, YearIndex: 5, Zone: model.ZoneEurope, Channel: model.ChannelDirect, UnitsSold: 1000},
					{ProductID: productAID, YearIndex: 5, Zone: model.ZoneExport, Channel: model.ChannelDirect, UnitsSold: 1000},
				},
				// No distributor margins for direct-only sales
				Margins: []model.ProductDistributorMargin{},
			},
			config: config,
			expected: struct {
				turnover   [5]decimal.Decimal
				cogs       [5]decimal.Decimal
				grossMargin [5]decimal.Decimal
			}{
				// 3000 units * 100 = 300 000 € turnover per year
				// 3000 units * 50  = 150 000 € COGS per year
				// grossMargin      = 150 000 €
				turnover: [5]decimal.Decimal{
					decimal.NewFromInt(300000), decimal.NewFromInt(300000), decimal.NewFromInt(300000),
					decimal.NewFromInt(300000), decimal.NewFromInt(300000),
				},
				cogs: [5]decimal.Decimal{
					decimal.NewFromInt(150000), decimal.NewFromInt(150000), decimal.NewFromInt(150000),
					decimal.NewFromInt(150000), decimal.NewFromInt(150000),
				},
				grossMargin: [5]decimal.Decimal{
					decimal.NewFromInt(150000), decimal.NewFromInt(150000), decimal.NewFromInt(150000),
					decimal.NewFromInt(150000), decimal.NewFromInt(150000),
				},
			},
		},
		{
			name: "zero volumes",
			product: model.Product{
				Name:       "Product B",
				ScenarioID: scenarioID,
			},
			bundle: ProductInputBundle{
				Assumptions: [MaxYears]model.ProductAssumption{
					{
						ProductID:        productBID,
						YearIndex:        1,
						BaseUnitPrice:    decimal.NewFromInt(100),
						RawMaterialCost:  decimal.NewFromInt(50),
						RoyaltiesCost:    decimal.Zero,
						LogisticsCost:    decimal.Zero,
						CostCoefficient:  decimal.NewFromInt(1),
						PriceCoefficient: decimal.NewFromInt(1),
					},
					{
						ProductID:        productBID,
						YearIndex:        2,
						BaseUnitPrice:    decimal.NewFromInt(100),
						RawMaterialCost:  decimal.NewFromInt(50),
						RoyaltiesCost:    decimal.Zero,
						LogisticsCost:    decimal.Zero,
						CostCoefficient:  decimal.NewFromInt(1),
						PriceCoefficient: decimal.NewFromInt(1),
					},
					{
						ProductID:        productBID,
						YearIndex:        3,
						BaseUnitPrice:    decimal.NewFromInt(100),
						RawMaterialCost:  decimal.NewFromInt(50),
						RoyaltiesCost:    decimal.Zero,
						LogisticsCost:    decimal.Zero,
						CostCoefficient:  decimal.NewFromInt(1),
						PriceCoefficient: decimal.NewFromInt(1),
					},
					{
						ProductID:        productBID,
						YearIndex:        4,
						BaseUnitPrice:    decimal.NewFromInt(100),
						RawMaterialCost:  decimal.NewFromInt(50),
						RoyaltiesCost:    decimal.Zero,
						LogisticsCost:    decimal.Zero,
						CostCoefficient:  decimal.NewFromInt(1),
						PriceCoefficient: decimal.NewFromInt(1),
					},
					{
						ProductID:        productBID,
						YearIndex:        5,
						BaseUnitPrice:    decimal.NewFromInt(100),
						RawMaterialCost:  decimal.NewFromInt(50),
						RoyaltiesCost:    decimal.Zero,
						LogisticsCost:    decimal.Zero,
						CostCoefficient:  decimal.NewFromInt(1),
						PriceCoefficient: decimal.NewFromInt(1),
					},
				},
				Volumes: []model.ProductSalesVolume{},
				Margins:  []model.ProductDistributorMargin{},
			},
			config: config,
			expected: struct {
				turnover   [5]decimal.Decimal
				cogs       [5]decimal.Decimal
				grossMargin [5]decimal.Decimal
			}{
				turnover: [5]decimal.Decimal{
					decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
				},
				cogs: [5]decimal.Decimal{
					decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
				},
				grossMargin: [5]decimal.Decimal{
					decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
				},
			},
		},
		{
			name: "multiple zones with different volumes and indirect channel with distributor margins",
			product: model.Product{
				Name:       "Product C",
				ScenarioID: scenarioID,
			},
			bundle: ProductInputBundle{
				Assumptions: [MaxYears]model.ProductAssumption{
					{
						ProductID:        productCID,
						YearIndex:        1,
						BaseUnitPrice:    decimal.NewFromInt(100),
						RawMaterialCost:  decimal.NewFromInt(50),
						RoyaltiesCost:    decimal.Zero,
						LogisticsCost:    decimal.Zero,
						CostCoefficient:  decimal.NewFromInt(1),
						PriceCoefficient: decimal.NewFromInt(1),
					},
					{
						ProductID:        productCID,
						YearIndex:        2,
						BaseUnitPrice:    decimal.NewFromInt(100),
						RawMaterialCost:  decimal.NewFromInt(50),
						RoyaltiesCost:    decimal.Zero,
						LogisticsCost:    decimal.Zero,
						CostCoefficient:  decimal.NewFromInt(1),
						PriceCoefficient: decimal.NewFromInt(1),
					},
					{
						ProductID:        productCID,
						YearIndex:        3,
						BaseUnitPrice:    decimal.NewFromInt(100),
						RawMaterialCost:  decimal.NewFromInt(50),
						RoyaltiesCost:    decimal.Zero,
						LogisticsCost:    decimal.Zero,
						CostCoefficient:  decimal.NewFromInt(1),
						PriceCoefficient: decimal.NewFromInt(1),
					},
					{
						ProductID:        productCID,
						YearIndex:        4,
						BaseUnitPrice:    decimal.NewFromInt(100),
						RawMaterialCost:  decimal.NewFromInt(50),
						RoyaltiesCost:    decimal.Zero,
						LogisticsCost:    decimal.Zero,
						CostCoefficient:  decimal.NewFromInt(1),
						PriceCoefficient: decimal.NewFromInt(1),
					},
					{
						ProductID:        productCID,
						YearIndex:        5,
						BaseUnitPrice:    decimal.NewFromInt(100),
						RawMaterialCost:  decimal.NewFromInt(50),
						RoyaltiesCost:    decimal.Zero,
						LogisticsCost:    decimal.Zero,
						CostCoefficient:  decimal.NewFromInt(1),
						PriceCoefficient: decimal.NewFromInt(1),
					},
				},
				// Volumes with different zones per year
				Volumes: []model.ProductSalesVolume{
					// Year 1: France=50, Europe=30, Export=20 (total direct=100)
					{ProductID: productCID, YearIndex: 1, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 50},
					{ProductID: productCID, YearIndex: 1, Zone: model.ZoneEurope, Channel: model.ChannelDirect, UnitsSold: 30},
					{ProductID: productCID, YearIndex: 1, Zone: model.ZoneExport, Channel: model.ChannelDirect, UnitsSold: 20},
					// Year 2: France=60, Europe=40, Export=30 (total direct=130)
					{ProductID: productCID, YearIndex: 2, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 60},
					{ProductID: productCID, YearIndex: 2, Zone: model.ZoneEurope, Channel: model.ChannelDirect, UnitsSold: 40},
					{ProductID: productCID, YearIndex: 2, Zone: model.ZoneExport, Channel: model.ChannelDirect, UnitsSold: 30},
					// Year 3: France=70, Europe=50, Export=40 (total direct=160)
					{ProductID: productCID, YearIndex: 3, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 70},
					{ProductID: productCID, YearIndex: 3, Zone: model.ZoneEurope, Channel: model.ChannelDirect, UnitsSold: 50},
					{ProductID: productCID, YearIndex: 3, Zone: model.ZoneExport, Channel: model.ChannelDirect, UnitsSold: 40},
					// Year 4: France=80, Europe=60, Export=50 (total direct=190)
					{ProductID: productCID, YearIndex: 4, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 80},
					{ProductID: productCID, YearIndex: 4, Zone: model.ZoneEurope, Channel: model.ChannelDirect, UnitsSold: 60},
					{ProductID: productCID, YearIndex: 4, Zone: model.ZoneExport, Channel: model.ChannelDirect, UnitsSold: 50},
					// Year 5: France=90, Europe=70, Export=60 (total direct=220)
					{ProductID: productCID, YearIndex: 5, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 90},
					{ProductID: productCID, YearIndex: 5, Zone: model.ZoneEurope, Channel: model.ChannelDirect, UnitsSold: 70},
					{ProductID: productCID, YearIndex: 5, Zone: model.ZoneExport, Channel: model.ChannelDirect, UnitsSold: 60},
				},
				// No margins needed for direct-only sales
				Margins: []model.ProductDistributorMargin{},
			},
			config: config,
			expected: struct {
				turnover   [5]decimal.Decimal
				cogs       [5]decimal.Decimal
				grossMargin [5]decimal.Decimal
			}{
				// Year 1: 100 units * 100 = 10000, 100 * 50 = 5000, margin = 5000
				// Year 2: 130 units * 100 = 13000, 130 * 50 = 6500, margin = 6500
				// Year 3: 160 units * 100 = 16000, 160 * 50 = 8000, margin = 8000
				// Year 4: 190 units * 100 = 19000, 190 * 50 = 9500, margin = 9500
				// Year 5: 220 units * 100 = 22000, 220 * 50 = 11000, margin = 11000
				turnover: [5]decimal.Decimal{
					decimal.NewFromInt(10000), decimal.NewFromInt(13000),
					decimal.NewFromInt(16000), decimal.NewFromInt(19000), decimal.NewFromInt(22000),
				},
				cogs: [5]decimal.Decimal{
					decimal.NewFromInt(5000), decimal.NewFromFloat(6500),
					decimal.NewFromInt(8000), decimal.NewFromFloat(9500), decimal.NewFromInt(11000),
				},
				grossMargin: [5]decimal.Decimal{
					decimal.NewFromInt(5000), decimal.NewFromFloat(6500),
					decimal.NewFromInt(8000), decimal.NewFromFloat(9500), decimal.NewFromInt(11000),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeProductRevenue(tt.product, tt.bundle, tt.config)

			for year := 0; year < MaxYears; year++ {
				assertDecEq(t, tt.expected.turnover[year], result.Years[year].Turnover,
					"Turnover mismatch in year %d", year+1)
				assertDecEq(t, tt.expected.cogs[year], result.Years[year].COGS,
					"COGS mismatch in year %d", year+1)
				assertDecEq(t, tt.expected.grossMargin[year], result.Years[year].GrossMargin,
					"Gross margin mismatch in year %d", year+1)
			}
		})
	}
}

func TestComputeConsolidatedRevenue(t *testing.T) {
	config := model.PlanConfig{
		CorporateTaxRate:   decimal.NewFromFloat(0.25),
		EmployerTaxRate: decimal.NewFromFloat(0.42),
		IncentiveCap:       decimal.NewFromFloat(0.15),
	}

	scenarioID := uuid.New()
	product1ID := uuid.New()
	product2ID := uuid.New()

	tests := []struct {
		name              string
		products          []model.Product
		bundles           []ProductInputBundle
		expectedTotalTurnover [5]decimal.Decimal
		expectedTotalCOGS     [5]decimal.Decimal
	}{
		{
			name: "multiple products consolidated",
			products: []model.Product{
				{Name: "Product 1", ScenarioID: scenarioID},
				{Name: "Product 2", ScenarioID: scenarioID},
			},
			bundles: []ProductInputBundle{
				{
					// Product 1: BaseUnitPrice=100, Cost=50, 100 units/year
					Assumptions: [MaxYears]model.ProductAssumption{
						{
							ProductID:        product1ID,
							YearIndex:        1,
							BaseUnitPrice:    decimal.NewFromInt(100),
							RawMaterialCost:  decimal.NewFromInt(50),
							RoyaltiesCost:    decimal.Zero,
							LogisticsCost:    decimal.Zero,
							CostCoefficient:  decimal.NewFromInt(1),
							PriceCoefficient: decimal.NewFromInt(1),
						},
						{
							ProductID:        product1ID,
							YearIndex:        2,
							BaseUnitPrice:    decimal.NewFromInt(100),
							RawMaterialCost:  decimal.NewFromInt(50),
							RoyaltiesCost:    decimal.Zero,
							LogisticsCost:    decimal.Zero,
							CostCoefficient:  decimal.NewFromInt(1),
							PriceCoefficient: decimal.NewFromInt(1),
						},
						{
							ProductID:        product1ID,
							YearIndex:        3,
							BaseUnitPrice:    decimal.NewFromInt(100),
							RawMaterialCost:  decimal.NewFromInt(50),
							RoyaltiesCost:    decimal.Zero,
							LogisticsCost:    decimal.Zero,
							CostCoefficient:  decimal.NewFromInt(1),
							PriceCoefficient: decimal.NewFromInt(1),
						},
						{
							ProductID:        product1ID,
							YearIndex:        4,
							BaseUnitPrice:    decimal.NewFromInt(100),
							RawMaterialCost:  decimal.NewFromInt(50),
							RoyaltiesCost:    decimal.Zero,
							LogisticsCost:    decimal.Zero,
							CostCoefficient:  decimal.NewFromInt(1),
							PriceCoefficient: decimal.NewFromInt(1),
						},
						{
							ProductID:        product1ID,
							YearIndex:        5,
							BaseUnitPrice:    decimal.NewFromInt(100),
							RawMaterialCost:  decimal.NewFromInt(50),
							RoyaltiesCost:    decimal.Zero,
							LogisticsCost:    decimal.Zero,
							CostCoefficient:  decimal.NewFromInt(1),
							PriceCoefficient: decimal.NewFromInt(1),
						},
					},
					Volumes: []model.ProductSalesVolume{
						// 100 units per year in France direct
						{ProductID: product1ID, YearIndex: 1, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 100},
						{ProductID: product1ID, YearIndex: 2, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 100},
						{ProductID: product1ID, YearIndex: 3, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 100},
						{ProductID: product1ID, YearIndex: 4, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 100},
						{ProductID: product1ID, YearIndex: 5, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 100},
					},
					Margins: []model.ProductDistributorMargin{},
				},
				{
					// Product 2: BaseUnitPrice=200, Cost=100, 50 units/year
					Assumptions: [MaxYears]model.ProductAssumption{
						{
							ProductID:        product2ID,
							YearIndex:        1,
							BaseUnitPrice:    decimal.NewFromInt(200),
							RawMaterialCost:  decimal.NewFromInt(100),
							RoyaltiesCost:    decimal.Zero,
							LogisticsCost:    decimal.Zero,
							CostCoefficient:  decimal.NewFromInt(1),
							PriceCoefficient: decimal.NewFromInt(1),
						},
						{
							ProductID:        product2ID,
							YearIndex:        2,
							BaseUnitPrice:    decimal.NewFromInt(200),
							RawMaterialCost:  decimal.NewFromInt(100),
							RoyaltiesCost:    decimal.Zero,
							LogisticsCost:    decimal.Zero,
							CostCoefficient:  decimal.NewFromInt(1),
							PriceCoefficient: decimal.NewFromInt(1),
						},
						{
							ProductID:        product2ID,
							YearIndex:        3,
							BaseUnitPrice:    decimal.NewFromInt(200),
							RawMaterialCost:  decimal.NewFromInt(100),
							RoyaltiesCost:    decimal.Zero,
							LogisticsCost:    decimal.Zero,
							CostCoefficient:  decimal.NewFromInt(1),
							PriceCoefficient: decimal.NewFromInt(1),
						},
						{
							ProductID:        product2ID,
							YearIndex:        4,
							BaseUnitPrice:    decimal.NewFromInt(200),
							RawMaterialCost:  decimal.NewFromInt(100),
							RoyaltiesCost:    decimal.Zero,
							LogisticsCost:    decimal.Zero,
							CostCoefficient:  decimal.NewFromInt(1),
							PriceCoefficient: decimal.NewFromInt(1),
						},
						{
							ProductID:        product2ID,
							YearIndex:        5,
							BaseUnitPrice:    decimal.NewFromInt(200),
							RawMaterialCost:  decimal.NewFromInt(100),
							RoyaltiesCost:    decimal.Zero,
							LogisticsCost:    decimal.Zero,
							CostCoefficient:  decimal.NewFromInt(1),
							PriceCoefficient: decimal.NewFromInt(1),
						},
					},
					Volumes: []model.ProductSalesVolume{
						// 50 units per year in France direct
						{ProductID: product2ID, YearIndex: 1, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 50},
						{ProductID: product2ID, YearIndex: 2, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 50},
						{ProductID: product2ID, YearIndex: 3, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 50},
						{ProductID: product2ID, YearIndex: 4, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 50},
						{ProductID: product2ID, YearIndex: 5, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 50},
					},
					Margins: []model.ProductDistributorMargin{},
				},
			},
			expectedTotalTurnover: [5]decimal.Decimal{
				// Product 1: 100 * 100 = 10000 per year
				// Product 2: 50 * 200 = 10000 per year
				// Total: 20000
				decimal.NewFromInt(20000), decimal.NewFromInt(20000), decimal.NewFromInt(20000),
				decimal.NewFromInt(20000), decimal.NewFromInt(20000),
			},
			expectedTotalCOGS: [5]decimal.Decimal{
				// Product 1: 100 * 50 = 5000 per year
				// Product 2: 50 * 100 = 5000 per year
				// Total: 10000
				decimal.NewFromInt(10000), decimal.NewFromInt(10000), decimal.NewFromInt(10000),
				decimal.NewFromInt(10000), decimal.NewFromInt(10000),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeConsolidatedRevenue(tt.products, tt.bundles, config)

			for year := 0; year < MaxYears; year++ {
				assertDecEq(t, tt.expectedTotalTurnover[year], result.Totals[year].TotalTurnover,
					"Total turnover mismatch in year %d", year+1)
				assertDecEq(t, tt.expectedTotalCOGS[year], result.Totals[year].TotalCOGS,
					"Total COGS mismatch in year %d", year+1)
			}
		})
	}
}

// TestProductRevenueCalendarYear is a regression test for the bug where
// ProductRevenueYear.Year and ConsolidatedRevenueYear.Year were always 0
// instead of being populated with the actual calendar year derived from
// config.ForecastStart.
func TestProductRevenueCalendarYear(t *testing.T) {
	forecastStart := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := model.PlanConfig{
		ForecastStart:         forecastStart,
		SalaryMonthsPerYear:   12,
		FirstFiscalYearMonths: 12,
	}

	product := model.Product{
		Name:       "Test Product",
		ScenarioID: uuid.New(),
	}

	// Minimal bundle — no volumes means zero revenue, but Year fields are set regardless.
	bundle := ProductInputBundle{
		Assumptions: [MaxYears]model.ProductAssumption{
			{YearIndex: 1, BaseUnitPrice: decimal.NewFromInt(100), CostCoefficient: decimal.NewFromInt(1), PriceCoefficient: decimal.NewFromInt(1)},
			{YearIndex: 2, BaseUnitPrice: decimal.NewFromInt(100), CostCoefficient: decimal.NewFromInt(1), PriceCoefficient: decimal.NewFromInt(1)},
			{YearIndex: 3, BaseUnitPrice: decimal.NewFromInt(100), CostCoefficient: decimal.NewFromInt(1), PriceCoefficient: decimal.NewFromInt(1)},
			{YearIndex: 4, BaseUnitPrice: decimal.NewFromInt(100), CostCoefficient: decimal.NewFromInt(1), PriceCoefficient: decimal.NewFromInt(1)},
			{YearIndex: 5, BaseUnitPrice: decimal.NewFromInt(100), CostCoefficient: decimal.NewFromInt(1), PriceCoefficient: decimal.NewFromInt(1)},
		},
	}

	// Per-product Year field
	result := ComputeProductRevenue(product, bundle, cfg)
	expectedYears := [MaxYears]int{2025, 2026, 2027, 2028, 2029}
	for i := 0; i < MaxYears; i++ {
		assert.Equal(t, expectedYears[i], result.Years[i].Year,
			"ProductRevenueYear[%d].Year should be calendar year %d", i, expectedYears[i])
		assert.Equal(t, i+1, result.Years[i].YearIndex,
			"ProductRevenueYear[%d].YearIndex should be 1-based index %d", i, i+1)
	}

	// Consolidated Year field
	consolidated := ComputeConsolidatedRevenue([]model.Product{product}, []ProductInputBundle{bundle}, cfg)
	for i := 0; i < MaxYears; i++ {
		assert.Equal(t, expectedYears[i], consolidated.Totals[i].Year,
			"ConsolidatedRevenueYear[%d].Year should be calendar year %d", i, expectedYears[i])
	}
}
