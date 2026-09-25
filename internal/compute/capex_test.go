package compute

import (
	"testing"

	"ascenda/internal/model"
	"github.com/shopspring/decimal"
)

func TestComputeCapexSummary(t *testing.T) {
	config := model.PlanConfig{
		CorporateTaxRate: decimal.NewFromFloat(0.25),
		EmployerTaxRate:  decimal.NewFromFloat(0.42),
		IncentiveCap:     decimal.NewFromFloat(0.15),
	}

	tests := []struct {
		name            string
		entries         []model.CapexEntry
		expectedInvYear [5]decimal.Decimal
		expectedDepYear [5]decimal.Decimal
	}{
		{
			// YearIndex is 1-based (1–5). Amounts are in k€ (the displayed unit)
			// and used as-is by the compute layer — no unit conversion.
			name: "single investment per year with AssetEquipmentTools",
			entries: []model.CapexEntry{
				{Category: model.AssetEquipmentTools, YearIndex: 1, Amount: decimal.NewFromInt(10000)},
				{Category: model.AssetEquipmentTools, YearIndex: 2, Amount: decimal.NewFromInt(10000)},
				{Category: model.AssetEquipmentTools, YearIndex: 3, Amount: decimal.NewFromInt(10000)},
				{Category: model.AssetEquipmentTools, YearIndex: 4, Amount: decimal.NewFromInt(10000)},
				{Category: model.AssetEquipmentTools, YearIndex: 5, Amount: decimal.NewFromInt(10000)},
			},
			expectedInvYear: [5]decimal.Decimal{
				decimal.NewFromInt(10000), decimal.NewFromInt(10000), decimal.NewFromInt(10000),
				decimal.NewFromInt(10000), decimal.NewFromInt(10000),
			},
			// AssetEquipmentTools useful life = 5 years.
			// investYear 0 (YearIndex 1): 10000/5 = 2000/yr, dep at idx 1,2,3,4
			// investYear 1 (YearIndex 2): 2000/yr, dep at idx 2,3,4
			// investYear 2 (YearIndex 3): 2000/yr, dep at idx 3,4
			// investYear 3 (YearIndex 4): 2000/yr, dep at idx 4
			// investYear 4 (YearIndex 5): no room within 5-yr horizon
			// idx: 0→0  1→2000  2→4000  3→6000  4→8000
			expectedDepYear: [5]decimal.Decimal{
				decimal.Zero, decimal.NewFromInt(2000), decimal.NewFromInt(4000),
				decimal.NewFromInt(6000), decimal.NewFromInt(8000),
			},
		},
		{
			name: "zero investments",
			entries: []model.CapexEntry{
				{Category: model.AssetEquipmentTools, YearIndex: 1, Amount: decimal.Zero},
				{Category: model.AssetEquipmentTools, YearIndex: 2, Amount: decimal.Zero},
				{Category: model.AssetEquipmentTools, YearIndex: 3, Amount: decimal.Zero},
				{Category: model.AssetEquipmentTools, YearIndex: 4, Amount: decimal.Zero},
				{Category: model.AssetEquipmentTools, YearIndex: 5, Amount: decimal.Zero},
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
				{Category: model.AssetLand, YearIndex: 1, Amount: decimal.NewFromInt(5000)},
				{Category: model.AssetLand, YearIndex: 2, Amount: decimal.NewFromInt(5000)},
				{Category: model.AssetLand, YearIndex: 3, Amount: decimal.NewFromInt(5000)},
				{Category: model.AssetLand, YearIndex: 4, Amount: decimal.NewFromInt(5000)},
				{Category: model.AssetLand, YearIndex: 5, Amount: decimal.NewFromInt(5000)},
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
				{Category: model.AssetBuildings, YearIndex: 1, Amount: decimal.NewFromInt(50000)},
				{Category: model.AssetBuildings, YearIndex: 2, Amount: decimal.NewFromInt(50000)},
				// Equipment (life=5)
				{Category: model.AssetEquipmentTools, YearIndex: 1, Amount: decimal.NewFromInt(20000)},
				{Category: model.AssetEquipmentTools, YearIndex: 2, Amount: decimal.NewFromInt(20000)},
				// Vehicles (life=5)
				{Category: model.AssetVehicles, YearIndex: 1, Amount: decimal.NewFromInt(10000)},
				{Category: model.AssetVehicles, YearIndex: 2, Amount: decimal.NewFromInt(10000)},
			},
			expectedInvYear: [5]decimal.Decimal{
				decimal.NewFromInt(80000), decimal.NewFromInt(80000), decimal.Zero,
				decimal.Zero, decimal.Zero,
			},
			// Depreciation starts the year AFTER investment.
			// idx 0 (Y1): 0
			// idx 1 (Y2): Y1 assets: Buildings 50000/10=5000, Equipment 20000/5=4000, Vehicles 10000/5=2000 = 11000
			// idx 2 (Y3): Y1(11000) + Y2(5000+4000+2000=11000) = 22000
			// idx 3-4: same 3 investments still within horizon → 22000
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

			// Verify net assets: NA[0] = 0, NA[i] = NA[i-1] + Inv[i] - Dep[i]
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
		CorporateTaxRate: decimal.NewFromFloat(0.25),
		EmployerTaxRate:  decimal.NewFromFloat(0.42),
		IncentiveCap:     decimal.NewFromFloat(0.15),
	}

	tests := []struct {
		name            string
		entries         []model.CapexEntry
		expectedDepYear [5]decimal.Decimal
		description     string
	}{
		{
			// 50 000 k€; 10-year life → 5 000/year.
			// investYear=0; dep at idx 1,2,3,4 (4 visible years in a 5-yr plan).
			name: "AssetBuildings 10-year depreciation",
			entries: []model.CapexEntry{
				{Category: model.AssetBuildings, YearIndex: 1, Amount: decimal.NewFromInt(50000)},
			},
			expectedDepYear: [5]decimal.Decimal{
				decimal.Zero,
				decimal.NewFromInt(5000), // 50000/10 = 5000/year
				decimal.NewFromInt(5000),
				decimal.NewFromInt(5000),
				decimal.NewFromInt(5000),
			},
			description: "Building with 10-year life depreciates at 5000/year",
		},
		{
			// 25 000 k€; 5-year life → 5 000/year.
			// investYear=0; dep at idx 1,2,3,4.
			name: "AssetEquipmentTools 5-year depreciation",
			entries: []model.CapexEntry{
				{Category: model.AssetEquipmentTools, YearIndex: 1, Amount: decimal.NewFromInt(25000)},
			},
			expectedDepYear: [5]decimal.Decimal{
				decimal.Zero,
				decimal.NewFromInt(5000), // 25000/5 = 5000/year
				decimal.NewFromInt(5000),
				decimal.NewFromInt(5000),
				decimal.NewFromInt(5000),
			},
			description: "Equipment with 5-year life depreciates at 5000/year",
		},
		{
			// 30 000 k€; 5-year life → 6 000/year.
			// Investment in Y2 (YearIndex:2, investYear=1); dep at idx 2,3,4.
			name: "AssetVehicles 5-year depreciation",
			entries: []model.CapexEntry{
				{Category: model.AssetVehicles, YearIndex: 2, Amount: decimal.NewFromInt(30000)},
			},
			expectedDepYear: [5]decimal.Decimal{
				decimal.Zero,
				decimal.Zero,
				decimal.NewFromInt(6000), // 30000/5 = 6000/year, starts year after purchase
				decimal.NewFromInt(6000),
				decimal.NewFromInt(6000),
			},
			description: "Vehicle investment in Y2, 5-year depreciation starting Y3",
		},
		{
			// Three staggered equipment investments of 10 000 k€ each.
			// investYear 0: 2000/yr at idx 1,2,3,4
			// investYear 1: 2000/yr at idx 2,3,4
			// investYear 2: 2000/yr at idx 3,4
			name: "cumulative depreciation from multiple investments",
			entries: []model.CapexEntry{
				{Category: model.AssetEquipmentTools, YearIndex: 1, Amount: decimal.NewFromInt(10000)},
				{Category: model.AssetEquipmentTools, YearIndex: 2, Amount: decimal.NewFromInt(10000)},
				{Category: model.AssetEquipmentTools, YearIndex: 3, Amount: decimal.NewFromInt(10000)},
			},
			expectedDepYear: [5]decimal.Decimal{
				decimal.Zero,
				decimal.NewFromInt(2000), // Only Y1 investment: 10000/5
				decimal.NewFromInt(4000), // Y1 + Y2: 2000+2000
				decimal.NewFromInt(6000), // Y1 + Y2 + Y3: 2000+2000+2000
				decimal.NewFromInt(6000), // Same 3 investments still depreciating
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
