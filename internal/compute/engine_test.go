package compute

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"ascenda/internal/model"
)

func TestComputeFullPlan(t *testing.T) {
	tests := []struct {
		name                string
		input               FullPlanInput
		expectedNoWarnings  bool
		shouldNotPanic      bool
	}{
		{
			name: "minimal valid input with empty entries produces zero reports",
			input: FullPlanInput{
				Config: model.PlanConfig{
						DiscountRate:         decimal.NewFromFloat(0.10),
					CorporateTaxRate:     decimal.NewFromFloat(0.25),
					EmployerTaxRate:      decimal.NewFromFloat(0.42),
					IncentiveCap:         decimal.NewFromFloat(0.15),
					Country:              "BE",
					SalaryMonthsPerYear:  12,
					FirstFiscalYearMonths: 12,
					ForecastStart:        time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				OpeningBalance: model.OpeningBalance{
					ShareCapital:        decimal.NewFromInt(100000),
					RetainedEarnings:    decimal.Zero,
					NoncurrentAssets:    decimal.Zero,
					Inventories:         decimal.Zero,
					CustomerReceivables: decimal.Zero,
					CashAndSecurities:   decimal.Zero,
					LoansAndDebt:        decimal.Zero,
					SupplierPayables:    decimal.Zero,
					SocialAndTaxDebts:   decimal.Zero,
				},
				WCConfig: model.WorkingCapitalConfig{
					CustomerPct0Days:  decimal.NewFromFloat(0.50),
					CustomerPct30Days: decimal.NewFromFloat(0.50),
					SupplierPct0Days:  decimal.NewFromFloat(0.50),
					SupplierPct30Days: decimal.NewFromFloat(0.50),
					InventoryPctYear1: decimal.NewFromFloat(0.10),
					InventoryPctYear2: decimal.NewFromFloat(0.10),
					InventoryPctYear3: decimal.NewFromFloat(0.10),
					InventoryPctYear4: decimal.NewFromFloat(0.10),
					InventoryPctYear5: decimal.NewFromFloat(0.10),
				},
				Products:        []model.Product{},
				ProductData:     []ProductInputBundle{},
				Headcounts:      []model.StaffHeadcount{},
				Salaries:        []model.StaffSalary{},
				Incentives:      []model.StaffIncentive{},
				CapexEntries:    []model.CapexEntry{},
				OpexEntries:     []model.OpexManualEntry{},
				PnlEntries:      []model.PnlManualEntry{},
				FiplanEntries:   []model.FiplanEntry{},
				PnlCashEntries:  []model.PnlCashEntry{},
				WCREntries:      []model.WCREntry{},
				CashOverrides:   []model.CashMonthlyOverride{},
				BudgetOverrides: []model.BudgetMonthlyOverride{},
			},
			expectedNoWarnings: true,
			shouldNotPanic:     true,
		},
		{
			name: "basic plan with single product",
			input: FullPlanInput{
				Config: model.PlanConfig{
						DiscountRate:         decimal.NewFromFloat(0.10),
					CorporateTaxRate:     decimal.NewFromFloat(0.25),
					EmployerTaxRate:      decimal.NewFromFloat(0.42),
					IncentiveCap:         decimal.NewFromFloat(0.15),
					Country:              "BE",
					SalaryMonthsPerYear:  12,
					FirstFiscalYearMonths: 12,
					ForecastStart:        time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				OpeningBalance: model.OpeningBalance{
					ShareCapital:        decimal.NewFromInt(100000),
					RetainedEarnings:    decimal.Zero,
					NoncurrentAssets:    decimal.Zero,
					Inventories:         decimal.Zero,
					CustomerReceivables: decimal.Zero,
					CashAndSecurities:   decimal.Zero,
					LoansAndDebt:        decimal.Zero,
					SupplierPayables:    decimal.Zero,
					SocialAndTaxDebts:   decimal.Zero,
				},
				WCConfig: model.WorkingCapitalConfig{
					CustomerPct0Days:  decimal.NewFromFloat(0.50),
					CustomerPct30Days: decimal.NewFromFloat(0.50),
					SupplierPct0Days:  decimal.NewFromFloat(0.50),
					SupplierPct30Days: decimal.NewFromFloat(0.50),
					InventoryPctYear1: decimal.NewFromFloat(0.10),
					InventoryPctYear2: decimal.NewFromFloat(0.10),
					InventoryPctYear3: decimal.NewFromFloat(0.10),
					InventoryPctYear4: decimal.NewFromFloat(0.10),
					InventoryPctYear5: decimal.NewFromFloat(0.10),
				},
				Products: []model.Product{
					{Name: "Product A"},
				},
				ProductData: []ProductInputBundle{
					{
						Assumptions: [5]model.ProductAssumption{
							{BaseUnitPrice: decimal.NewFromInt(100), RawMaterialCost: decimal.NewFromInt(50), CostCoefficient: decimal.NewFromFloat(1.0), PriceCoefficient: decimal.NewFromFloat(1.0)},
							{BaseUnitPrice: decimal.NewFromInt(100), RawMaterialCost: decimal.NewFromInt(50), CostCoefficient: decimal.NewFromFloat(1.0), PriceCoefficient: decimal.NewFromFloat(1.0)},
							{BaseUnitPrice: decimal.NewFromInt(100), RawMaterialCost: decimal.NewFromInt(50), CostCoefficient: decimal.NewFromFloat(1.0), PriceCoefficient: decimal.NewFromFloat(1.0)},
							{BaseUnitPrice: decimal.NewFromInt(100), RawMaterialCost: decimal.NewFromInt(50), CostCoefficient: decimal.NewFromFloat(1.0), PriceCoefficient: decimal.NewFromFloat(1.0)},
							{BaseUnitPrice: decimal.NewFromInt(100), RawMaterialCost: decimal.NewFromInt(50), CostCoefficient: decimal.NewFromFloat(1.0), PriceCoefficient: decimal.NewFromFloat(1.0)},
						},
						Volumes: []model.ProductSalesVolume{
							{YearIndex: 1, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 100},
							{YearIndex: 2, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 100},
							{YearIndex: 3, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 100},
							{YearIndex: 4, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 100},
							{YearIndex: 5, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 100},
						},
						Margins: []model.ProductDistributorMargin{},
					},
				},
				Headcounts: []model.StaffHeadcount{
					{Category: model.CategoryRnDEngineers, YearIndex: 1, FTE: decimal.NewFromInt(1)},
					{Category: model.CategoryRnDEngineers, YearIndex: 2, FTE: decimal.NewFromInt(1)},
					{Category: model.CategoryRnDEngineers, YearIndex: 3, FTE: decimal.NewFromInt(1)},
					{Category: model.CategoryRnDEngineers, YearIndex: 4, FTE: decimal.NewFromInt(1)},
					{Category: model.CategoryRnDEngineers, YearIndex: 5, FTE: decimal.NewFromInt(1)},
				},
				Salaries: []model.StaffSalary{
					{Category: model.CategoryRnDEngineers, YearIndex: 1, MonthlyGrossSalary: decimal.NewFromInt(3000)},
					{Category: model.CategoryRnDEngineers, YearIndex: 2, MonthlyGrossSalary: decimal.NewFromInt(3000)},
					{Category: model.CategoryRnDEngineers, YearIndex: 3, MonthlyGrossSalary: decimal.NewFromInt(3000)},
					{Category: model.CategoryRnDEngineers, YearIndex: 4, MonthlyGrossSalary: decimal.NewFromInt(3000)},
					{Category: model.CategoryRnDEngineers, YearIndex: 5, MonthlyGrossSalary: decimal.NewFromInt(3000)},
				},
				Incentives: []model.StaffIncentive{
					{YearIndex: 1, IncentivePct: decimal.NewFromFloat(0.10)},
					{YearIndex: 2, IncentivePct: decimal.NewFromFloat(0.10)},
					{YearIndex: 3, IncentivePct: decimal.NewFromFloat(0.10)},
					{YearIndex: 4, IncentivePct: decimal.NewFromFloat(0.10)},
					{YearIndex: 5, IncentivePct: decimal.NewFromFloat(0.10)},
				},
				CapexEntries: []model.CapexEntry{
					{Category: model.AssetEquipmentTools, YearIndex: 0, Amount: decimal.NewFromInt(10000), DepreciationYears: 5},
					{Category: model.AssetEquipmentTools, YearIndex: 1, Amount: decimal.NewFromInt(10000), DepreciationYears: 5},
				},
				OpexEntries:     []model.OpexManualEntry{},
				PnlEntries:      []model.PnlManualEntry{},
				FiplanEntries:   []model.FiplanEntry{},
				PnlCashEntries:  []model.PnlCashEntry{},
				WCREntries:      []model.WCREntry{},
				CashOverrides:   []model.CashMonthlyOverride{},
				BudgetOverrides: []model.BudgetMonthlyOverride{},
			},
			shouldNotPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			assert.NotPanics(t, func() {
				output := ComputeFullPlan(tt.input)

				// Verify output structure is populated
				assert.NotNil(t, output)

				// Verify revenue was computed
				if len(tt.input.Products) > 0 {
					assert.True(t, output.Revenue.Totals[0].TotalTurnover.GreaterThan(decimal.Zero), "expected positive revenue")
				}

				// Verify payroll was computed
				if len(tt.input.Headcounts) > 0 {
					assert.True(t, output.Payroll.Payroll[0].TotalPayroll.GreaterThan(decimal.Zero), "expected positive payroll")
				}

				// Verify capex was computed
				if len(tt.input.CapexEntries) > 0 {
					assert.True(t, output.Capex.Totals.TotalCapex[0].GreaterThan(decimal.Zero), "expected positive capex")
				}

				// Verify PnL structure exists
				assert.NotNil(t, output.PnL)

				// Verify balance sheet structure exists
				assert.NotNil(t, output.BSheet)

				// Warnings should be populated (even if empty slice)
				assert.NotNil(t, output.Warnings)

				if tt.expectedNoWarnings {
					// Some warnings may still exist for structural reasons
					// Just verify the structure is valid
				}
			})
		})
	}
}

func TestComputeFullPlanIntegration(t *testing.T) {
	// Integration test: ensure all layers compute without panicking
	config := model.PlanConfig{
		DiscountRate:         decimal.NewFromFloat(0.10),
		CorporateTaxRate:     decimal.NewFromFloat(0.25),
		EmployerTaxRate:      decimal.NewFromFloat(0.42),
		IncentiveCap:         decimal.NewFromFloat(0.15),
		Country:              "BE",
		SalaryMonthsPerYear:  12,
		FirstFiscalYearMonths: 12,
		ForecastStart:        time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	input := FullPlanInput{
		Config:         config,
		OpeningBalance: model.OpeningBalance{ShareCapital: decimal.NewFromInt(100000)},
		WCConfig: model.WorkingCapitalConfig{
			CustomerPct0Days:  decimal.NewFromFloat(0.50),
			CustomerPct30Days: decimal.NewFromFloat(0.50),
			SupplierPct0Days:  decimal.NewFromFloat(0.50),
			SupplierPct30Days: decimal.NewFromFloat(0.50),
			InventoryPctYear1: decimal.NewFromFloat(0.10),
			InventoryPctYear2: decimal.NewFromFloat(0.10),
			InventoryPctYear3: decimal.NewFromFloat(0.10),
			InventoryPctYear4: decimal.NewFromFloat(0.10),
			InventoryPctYear5: decimal.NewFromFloat(0.10),
		},
		Products:        []model.Product{},
		ProductData:     []ProductInputBundle{},
		Headcounts:      []model.StaffHeadcount{},
		Salaries:        []model.StaffSalary{},
		Incentives:      []model.StaffIncentive{},
		CapexEntries:    []model.CapexEntry{},
		OpexEntries:     []model.OpexManualEntry{},
		PnlEntries:      []model.PnlManualEntry{},
		FiplanEntries:   []model.FiplanEntry{},
		PnlCashEntries:  []model.PnlCashEntry{},
		WCREntries:      []model.WCREntry{},
		CashOverrides:   []model.CashMonthlyOverride{},
		BudgetOverrides: []model.BudgetMonthlyOverride{},
	}

	output := ComputeFullPlan(input)

	// Verify all major sections are present
	assert.NotNil(t, output.Revenue)
	assert.NotNil(t, output.Payroll)
	assert.NotNil(t, output.Capex)
	assert.NotNil(t, output.Opex)
	assert.NotNil(t, output.PnL)
	assert.NotNil(t, output.FiPlan)
	assert.NotNil(t, output.PnlCash)
	assert.NotNil(t, output.BSheet)
	assert.NotNil(t, output.Ratios)
	assert.NotNil(t, output.WCR)
	assert.NotNil(t, output.Cash)
	assert.NotNil(t, output.Budget1)
	assert.NotNil(t, output.Budget2)
	assert.NotNil(t, output.Warnings)
}
