package compute

import (
	"testing"

	"github.com/shopspring/decimal"
	"kerplan/internal/model"
)

func TestComputeCapexSummary(t *testing.T) {
	config := model.PlanConfig{
		CorporateTaxRate:   decimal.NewFromFloat(0.25),
		EmployerTaxRate: decimal.NewFromFloat(0.42),
		IncentiveCap:       decimal.NewFromFloat(0.15),
	}

	tests := []struct {
		name            string
		entries         []model.CapexEntry
		expectedInvYear [5]decimal.Decimal
		expectedDepYear [5]decimal.Decimal
	}{
		{
			name: "single investment per year with AssetEquipmentTools",
			entries: []model.CapexEntry{
				{Category: model.AssetEquipmentTools, YearIndex: 0, Amount: decimal.NewFromInt(10000)},
				{Category: model.AssetEquipmentTools, YearIndex: 1, Amount: decimal.NewFromInt(10000)},
				{Category: model.AssetEquipmentTools, YearIndex: 2, Amount: decimal.NewFromInt(10000)},
				{Category: model.AssetEquipmentTools, YearIndex: 3, Amount: decimal.NewFromInt(10000)},
				{Category: model.AssetEquipmentTools, YearIndex: 4, Amount: decimal.NewFromInt(10000)},
			},
			expectedInvYear: [5]decimal.Decimal{
				decimal.NewFromInt(10000), decimal.NewFromInt(10000), decimal.NewFromInt(10000),
				decimal.NewFromInt(10000), decimal.NewFromInt(10000),
			},
			// AssetEquipmentTools useful life = 5 years
			// Year 0 investment: 10000/5 = 2000 per year starting from year 1
			// Year 0 depreciation: 0
			// Year 1 depreciation: 2000 (from Y0 investment)
			// Year 2 depreciation: 2000 + 2000 = 4000 (from Y0 and Y1)
			// Year 3 depreciation: 2000 + 2000 + 2000 = 6000
			// Year 4 depreciation: 2000 + 2000 + 2000 + 2000 = 8000
			expectedDepYear: [5]decimal.Decimal{
				decimal.Zero, decimal.NewFromInt(2000), decimal.NewFromInt(4000),
				decimal.NewFromInt(6000), decimal.NewFromInt(8000),
			},
		},
		{
			name: "zero investments",
			entries: []model.CapexEntry{
				{Category: model.AssetEquipmentTools, YearIndex: 0, Amount: decimal.Zero},
				{Category: model.AssetEquipmentTools, YearIndex: 1, Amount: decimal.Zero},
				{Category: model.AssetEquipmentTools, YearIndex: 2, Amount: decimal.Zero},
				{Category: model.AssetEquipmentTools, YearIndex: 3, Amount: decimal.Zero},
				{Category: model.AssetEquipmentTools, YearIndex: 4, Amount: decimal.Zero},
			},
			expectedInvYear: [5]decimal.Decimal{
				decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
			},
			expectedDepYear: [5]decimal.Decimal{
				decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
			},
		},
		{
			name: "non-depreciable assets (Land)",
			entries: []model.CapexEntry{
				{Category: model.AssetLand, YearIndex: 0, Amount: decimal.NewFromInt(5000)},
				{Category: model.AssetLand, YearIndex: 1, Amount: decimal.NewFromInt(5000)},
				{Category: model.AssetLand, YearIndex: 2, Amount: decimal.NewFromInt(5000)},
				{Category: model.AssetLand, YearIndex: 3, Amount: decimal.NewFromInt(5000)},
				{Category: model.AssetLand, YearIndex: 4, Amount: decimal.NewFromInt(5000)},
			},
			expectedInvYear: [5]decimal.Decimal{
				decimal.NewFromInt(5000), decimal.NewFromInt(5000), decimal.NewFromInt(5000),
				decimal.NewFromInt(5000), decimal.NewFromInt(5000),
			},
			// Land is non-depreciable
			expectedDepYear: [5]decimal.Decimal{
				decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
			},
		},
		{
			name: "multiple categories with new life values",
			entries: []model.CapexEntry{
				// Buildings (life=10)
				{Category: model.AssetBuildings, YearIndex: 0, Amount: decimal.NewFromInt(50000)},
				{Category: model.AssetBuildings, YearIndex: 1, Amount: decimal.NewFromInt(50000)},
				// AssetEquipmentTools (life=5)
				{Category: model.AssetEquipmentTools, YearIndex: 0, Amount: decimal.NewFromInt(20000)},
				{Category: model.AssetEquipmentTools, YearIndex: 1, Amount: decimal.NewFromInt(20000)},
				// Vehicles (life=5)
				{Category: model.AssetVehicles, YearIndex: 0, Amount: decimal.NewFromInt(10000)},
				{Category: model.AssetVehicles, YearIndex: 1, Amount: decimal.NewFromInt(10000)},
			},
			expectedInvYear: [5]decimal.Decimal{
				decimal.NewFromInt(80000), decimal.NewFromInt(80000), decimal.Zero,
				decimal.Zero, decimal.Zero,
			},
			// Depreciation starts the year AFTER investment
			// Year 0: no depreciation (investments made in Y0 start depreciating in Y1)
			// Year 1: Y0 assets: Buildings 50000/10=5000, AssetEquipmentTools 20000/5=4000, Vehicles 10000/5=2000 = 11000
			// Year 2: Y0 assets (11000) + Y1 assets (5000+4000+2000=11000) = 22000
			// Year 3-4: Same = 22000
			expectedDepYear: [5]decimal.Decimal{
				decimal.Zero, decimal.NewFromInt(11000), decimal.NewFromInt(22000),
				decimal.NewFromInt(22000), decimal.NewFromInt(22000),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeCapexSummary(tt.entries, config)

			for year := 0; year < 5; year++ {
				assertDecEq(t, tt.expectedInvYear[year], result.Totals.TotalCapex[year],
					"Total investment mismatch in year %d", year)
				assertDecEq(t, tt.expectedDepYear[year], result.Totals.TotalDepreciation[year],
					"Total depreciation mismatch in year %d", year)
			}

			// Verify net assets computation: NA[0] = 0, NA[i] = NA[i-1] + Inv[i] - Dep[i]
			assertDecEq(t, decimal.Zero, result.Totals.NetAssets[0], "NetAssets[0] should be zero")
			for year := 0; year < 5; year++ {
				expected := result.Totals.NetAssets[year].Add(result.Totals.TotalCapex[year]).Sub(result.Totals.TotalDepreciation[year])
				assertDecEq(t, expected, result.Totals.NetAssets[year+1],
					"Net assets mismatch in year %d", year)
			}
		})
	}
}

func TestDepreciationWithNewDefaults(t *testing.T) {
	config := model.PlanConfig{
		CorporateTaxRate:   decimal.NewFromFloat(0.25),
		EmployerTaxRate: decimal.NewFromFloat(0.42),
		IncentiveCap:       decimal.NewFromFloat(0.15),
	}

	tests := []struct {
		name            string
		entries         []model.CapexEntry
		expectedDepYear [5]decimal.Decimal
		description     string
	}{
		{
			name: "AssetBuildings 10-year depreciation",
			entries: []model.CapexEntry{
				{Category: model.AssetBuildings, YearIndex: 0, Amount: decimal.NewFromInt(50000)},
			},
			expectedDepYear: [5]decimal.Decimal{
				decimal.Zero,
				decimal.NewFromInt(5000), // 50000/10 = 5000 per year
				decimal.NewFromInt(5000),
				decimal.NewFromInt(5000),
				decimal.NewFromInt(5000),
			},
			description: "Building with 10-year life depreciates at 5000/year",
		},
		{
			name: "AssetEquipmentTools 5-year depreciation",
			entries: []model.CapexEntry{
				{Category: model.AssetEquipmentTools, YearIndex: 0, Amount: decimal.NewFromInt(25000)},
			},
			expectedDepYear: [5]decimal.Decimal{
				decimal.Zero,
				decimal.NewFromInt(5000), // 25000/5 = 5000 per year
				decimal.NewFromInt(5000),
				decimal.NewFromInt(5000),
				decimal.NewFromInt(5000),
			},
			description: "Equipment with 5-year life depreciates at 5000/year",
		},
		{
			name: "AssetVehicles 5-year depreciation",
			entries: []model.CapexEntry{
				{Category: model.AssetVehicles, YearIndex: 1, Amount: decimal.NewFromInt(30000)},
			},
			expectedDepYear: [5]decimal.Decimal{
				decimal.Zero,
				decimal.Zero,
				decimal.NewFromInt(6000), // 30000/5 = 6000 per year, starts year 2 (after YearIndex 1)
				decimal.NewFromInt(6000),
				decimal.NewFromInt(6000),
			},
			description: "Vehicle investment in year 2, 5-year depreciation starting year 3",
		},
		{
			name: "cumulative depreciation from multiple investments",
			entries: []model.CapexEntry{
				{Category: model.AssetEquipmentTools, YearIndex: 0, Amount: decimal.NewFromInt(10000)},
				{Category: model.AssetEquipmentTools, YearIndex: 1, Amount: decimal.NewFromInt(10000)},
				{Category: model.AssetEquipmentTools, YearIndex: 2, Amount: decimal.NewFromInt(10000)},
			},
			expectedDepYear: [5]decimal.Decimal{
				decimal.Zero,
				decimal.NewFromInt(2000), // Only Y0 investment: 10000/5
				decimal.NewFromInt(4000), // Y0 and Y1: 2000 + 2000
				decimal.NewFromInt(6000), // Y0, Y1, Y2: 2000 + 2000 + 2000
				decimal.NewFromInt(6000), // Same 3 investments still depreciating: 2000 + 2000 + 2000
			},
			description: "Staggered investments accumulate depreciation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeCapexSummary(tt.entries, config)

			for year := 0; year < 5; year++ {
				assertDecEq(t, tt.expectedDepYear[year], result.Totals.TotalDepreciation[year],
					"Depreciation mismatch in year %d (%s)", year, tt.description)
			}
		})
	}
}
