package compute

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"kerplan/internal/model"
)

func TestComputeWCR(t *testing.T) {
	scenarioID := uuid.New()
	tenantID := uuid.New()

	config := model.PlanConfig{
		TenantScoped:     model.TenantScoped{TenantID: tenantID},
		ScenarioID:       scenarioID,
		DiscountRate:     decimal.NewFromFloat(0.1),
		CorporateTaxRate: decimal.NewFromFloat(0.25),
		EmployerTaxRate:  decimal.NewFromFloat(0.42),
		VATRate:          decimal.NewFromFloat(0.21),
	}

	wcConfig := model.WorkingCapitalConfig{
		TenantScoped:          model.TenantScoped{TenantID: tenantID},
		ScenarioID:            scenarioID,
		CustomerPct0Days:      decimal.NewFromFloat(0.1),
		CustomerPct30Days:     decimal.NewFromFloat(0.6),
		CustomerPct60Days:     decimal.NewFromFloat(0.2),
		CustomerPct90Days:     decimal.NewFromFloat(0.1),
		SupplierPct0Days:      decimal.NewFromFloat(0.2),
		SupplierPct30Days:     decimal.NewFromFloat(0.5),
		SupplierPct60Days:     decimal.NewFromFloat(0.2),
		SupplierPct90Days:     decimal.NewFromFloat(0.1),
		InventoryPctYear1:     decimal.NewFromFloat(0.1),
		InventoryPctYear2:     decimal.NewFromFloat(0.1),
		InventoryPctYear3:     decimal.NewFromFloat(0.1),
		InventoryPctYear4:     decimal.NewFromFloat(0.1),
		InventoryPctYear5:     decimal.NewFromFloat(0.1),
	}

	openingBal := model.OpeningBalance{
		TenantScoped:         model.TenantScoped{TenantID: tenantID},
		ScenarioID:           scenarioID,
		CustomerReceivables:  decimal.Zero,
		Inventories:          decimal.Zero,
		SupplierPayables:     decimal.Zero,
		SocialAndTaxDebts:    decimal.Zero,
	}

	capex := model.CapexSummary{}

	pnl := model.PnlReport{}

	tests := []struct {
		name       string
		entries    []model.WCREntry
		revenue    model.ConsolidatedRevenue
		opex       model.OpexSummary
		staff      model.StaffPayrollSummary
		checkWCR   func(*testing.T, model.WCRReport)
	}{
		{
			name:    "standard WCR calculation with computed values",
			entries: []model.WCREntry{},
			revenue: model.ConsolidatedRevenue{
				ScenarioID: scenarioID,
				Totals: [5]model.ConsolidatedRevenueYear{
					{
						TotalTurnover: decimal.NewFromInt(365000),
						TotalCOGS:     decimal.NewFromInt(182500),
					},
					{
						TotalTurnover: decimal.NewFromInt(438000),
						TotalCOGS:     decimal.NewFromInt(219000),
					},
					{
						TotalTurnover: decimal.NewFromInt(525600),
						TotalCOGS:     decimal.NewFromInt(262800),
					},
					{
						TotalTurnover: decimal.NewFromInt(630720),
						TotalCOGS:     decimal.NewFromInt(315360),
					},
					{
						TotalTurnover: decimal.NewFromInt(756864),
						TotalCOGS:     decimal.NewFromInt(378432),
					},
				},
			},
			opex: model.OpexSummary{
				GrandTotal: [5]decimal.Decimal{
					decimal.NewFromInt(73000),
					decimal.NewFromInt(87600),
					decimal.NewFromInt(105120),
					decimal.NewFromInt(126144),
					decimal.NewFromInt(151373),
				},
			},
			staff: model.StaffPayrollSummary{
				ScenarioID: scenarioID,
				Payroll: [5]model.StaffPayrollYear{
					{TotalPayroll: decimal.NewFromInt(100000)},
					{TotalPayroll: decimal.NewFromInt(120000)},
					{TotalPayroll: decimal.NewFromInt(144000)},
					{TotalPayroll: decimal.NewFromInt(172800)},
					{TotalPayroll: decimal.NewFromInt(207360)},
				},
			},
			checkWCR: func(t *testing.T, result model.WCRReport) {
				// Verify VAT rate is set
				assert.Equal(t, decimal.NewFromFloat(0.21), result.VATRate,
					"VAT rate should be set from config")

				// Verify basic WCR structure is populated
				assert.NotEmpty(t, result.Summary.CustomerWCR,
					"Customer WCR should be populated")
				assert.NotEmpty(t, result.Summary.SupplierWCR,
					"Supplier WCR should be populated")

				// Year 1 (index 0): Customer WCR based on sales and payment terms
				// With 60% at 30 days: 365000 * 0.6 * 30 / 360 = 18250
				assert.True(t, result.Summary.CustomerWCR[0].GreaterThan(decimal.Zero),
					"Year 1 customer WCR should be positive")

				// Year 1: Supplier WCR and inventory should be calculated
				assert.True(t, result.Summary.SupplierWCR[0].GreaterThanOrEqual(decimal.Zero),
					"Year 1 supplier WCR should be non-negative")
				assert.True(t, result.Inventory.InventoryValue[0].GreaterThan(decimal.Zero),
					"Year 1 inventory should be positive given 10% rate on 365k")

				// Basic WCR = CustomerWCR + InventoryValue - SupplierWCR
				expectedBasicWCR := result.Summary.CustomerWCR[0].
					Add(result.Inventory.InventoryValue[0]).
					Sub(result.Summary.SupplierWCR[0])
				assertDecEqApprox(t, expectedBasicWCR, result.Summary.BasicWCR[0], decimal.NewFromFloat(1),
					"Year 1 basic WCR should be customers + inventory - suppliers")

				// WCR Change for year 1 should be positive (initial WCR)
				assert.True(t, result.Summary.WCRChange[0].GreaterThan(decimal.Zero),
					"Year 1 WCR change should be initial WCR (positive)")

				// WCR Change for year 2 should reflect growth
				expectedChange2 := result.Summary.BasicWCR[1].Sub(result.Summary.BasicWCR[0])
				// Allow small tolerance for rounding
				diff := expectedChange2.Sub(result.Summary.WCRChange[1]).Abs()
				assert.True(t, diff.LessThanOrEqual(decimal.NewFromFloat(1)),
					"Year 2 WCR change should approximate difference from year 1")
			},
		},
		{
			name:    "zero revenue (no WCR)",
			entries: []model.WCREntry{},
			revenue: model.ConsolidatedRevenue{
				ScenarioID: scenarioID,
				Totals: [5]model.ConsolidatedRevenueYear{
					{TotalTurnover: decimal.Zero},
					{TotalTurnover: decimal.Zero},
					{TotalTurnover: decimal.Zero},
					{TotalTurnover: decimal.Zero},
					{TotalTurnover: decimal.Zero},
				},
			},
			opex: model.OpexSummary{
				GrandTotal: [5]decimal.Decimal{
					decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
				},
			},
			staff: model.StaffPayrollSummary{
				ScenarioID: scenarioID,
				Payroll: [5]model.StaffPayrollYear{
					{TotalPayroll: decimal.Zero},
					{TotalPayroll: decimal.Zero},
					{TotalPayroll: decimal.Zero},
					{TotalPayroll: decimal.Zero},
					{TotalPayroll: decimal.Zero},
				},
			},
			checkWCR: func(t *testing.T, result model.WCRReport) {
				// All WCR components should be zero
				for year := 0; year < 5; year++ {
					assertDecEq(t, decimal.Zero, result.Summary.CustomerWCR[year],
						"Customer WCR year %d should be zero", year+1)
					assertDecEq(t, decimal.Zero, result.Summary.SupplierWCR[year],
						"Supplier WCR year %d should be zero", year+1)
					assertDecEq(t, decimal.Zero, result.Inventory.InventoryValue[year],
						"Inventory WCR year %d should be zero", year+1)
					assertDecEq(t, decimal.Zero, result.Summary.BasicWCR[year],
						"Total WCR year %d should be zero", year+1)
				}
			},
		},
		{
			name: "manual overrides for adjustment lines",
			entries: []model.WCREntry{
				{LineID: model.WCRPrepaidExpenses, YearIndex: 0, Amount: decimal.NewFromInt(5000)},
				{LineID: model.WCRDeferredRevenue, YearIndex: 0, Amount: decimal.NewFromInt(-3000)},
				{LineID: model.WCRPrepaidExpenses, YearIndex: 1, Amount: decimal.NewFromInt(6000)},
			},
			revenue: model.ConsolidatedRevenue{
				ScenarioID: scenarioID,
				Totals: [5]model.ConsolidatedRevenueYear{
					{TotalTurnover: decimal.NewFromInt(365000)},
					{TotalTurnover: decimal.NewFromInt(438000)},
					{TotalTurnover: decimal.Zero},
					{TotalTurnover: decimal.Zero},
					{TotalTurnover: decimal.Zero},
				},
			},
			opex: model.OpexSummary{
				GrandTotal: [5]decimal.Decimal{
					decimal.NewFromInt(73000),
					decimal.NewFromInt(87600),
					decimal.Zero, decimal.Zero, decimal.Zero,
				},
			},
			staff: model.StaffPayrollSummary{
				ScenarioID: scenarioID,
				Payroll: [5]model.StaffPayrollYear{
					{TotalPayroll: decimal.NewFromInt(100000)},
					{TotalPayroll: decimal.NewFromInt(120000)},
					{TotalPayroll: decimal.Zero},
					{TotalPayroll: decimal.Zero},
					{TotalPayroll: decimal.Zero},
				},
			},
			checkWCR: func(t *testing.T, result model.WCRReport) {
				// Year 1: Prepaid expenses override should be applied
				assertDecEq(t, decimal.NewFromInt(5000), result.Adjustments.PrepaidExpenses[0],
					"Year 1 prepaid expenses should be overridden")

				// Year 1: Deferred revenue override should be applied
				assertDecEq(t, decimal.NewFromInt(-3000), result.Adjustments.DeferredRevenue[0],
					"Year 1 deferred revenue should be overridden")

				// Year 2: Prepaid expenses override should be applied
				assertDecEq(t, decimal.NewFromInt(6000), result.Adjustments.PrepaidExpenses[1],
					"Year 2 prepaid expenses should be overridden")

				// Net adjustment should sum the individual adjustments
				expectedNetAdj0 := decimal.NewFromInt(5000).Add(decimal.NewFromInt(-3000))
				assertDecEq(t, expectedNetAdj0, result.Adjustments.NetAdjustment[0],
					"Year 1 net adjustment should sum prepaid and deferred")
			},
		},
		{
			name:    "WCR reflects year-over-year growth",
			entries: []model.WCREntry{},
			revenue: model.ConsolidatedRevenue{
				ScenarioID: scenarioID,
				Totals: [5]model.ConsolidatedRevenueYear{
					{TotalTurnover: decimal.NewFromInt(100000)},
					{TotalTurnover: decimal.NewFromInt(120000)},
					{TotalTurnover: decimal.NewFromInt(144000)},
					{TotalTurnover: decimal.Zero},
					{TotalTurnover: decimal.Zero},
				},
			},
			opex: model.OpexSummary{
				GrandTotal: [5]decimal.Decimal{
					decimal.NewFromInt(20000),
					decimal.NewFromInt(24000),
					decimal.NewFromInt(28800),
					decimal.Zero, decimal.Zero,
				},
			},
			staff: model.StaffPayrollSummary{
				ScenarioID: scenarioID,
				Payroll: [5]model.StaffPayrollYear{
					{TotalPayroll: decimal.NewFromInt(40000)},
					{TotalPayroll: decimal.NewFromInt(48000)},
					{TotalPayroll: decimal.NewFromInt(57600)},
					{TotalPayroll: decimal.Zero},
					{TotalPayroll: decimal.Zero},
				},
			},
			checkWCR: func(t *testing.T, result model.WCRReport) {
				// Year 1 change should equal initial basic WCR
				assertDecEqApprox(t, result.Summary.BasicWCR[0], result.Summary.WCRChange[0], decimal.NewFromFloat(1),
					"Year 1 WCR change should equal initial basic WCR")

				// Year 2 change should reflect growth from year 1 to year 2
				expectedChange2 := result.Summary.BasicWCR[1].Sub(result.Summary.BasicWCR[0])
				diff := expectedChange2.Sub(result.Summary.WCRChange[1]).Abs()
				assert.True(t, diff.LessThanOrEqual(decimal.NewFromFloat(1)),
					"Year 2 WCR change should approximate difference")

				// Year 3 change should reflect growth from year 2 to year 3
				expectedChange3 := result.Summary.BasicWCR[2].Sub(result.Summary.BasicWCR[1])
				diff = expectedChange3.Sub(result.Summary.WCRChange[2]).Abs()
				assert.True(t, diff.LessThanOrEqual(decimal.NewFromFloat(1)),
					"Year 3 WCR change should approximate difference")

				// Basic WCR should increase with revenue growth
				assert.True(t, result.Summary.BasicWCR[1].GreaterThanOrEqual(result.Summary.BasicWCR[0]),
					"WCR should increase or stay same as revenue increases")
				assert.True(t, result.Summary.BasicWCR[2].GreaterThanOrEqual(result.Summary.BasicWCR[1]),
					"WCR should increase or stay same as revenue increases")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeWCR(tt.entries, tt.revenue, tt.opex, tt.staff, wcConfig, openingBal, capex, pnl, config)
			tt.checkWCR(t, result)
		})
	}
}
