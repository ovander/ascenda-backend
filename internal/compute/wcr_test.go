package compute

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"ascenda/internal/model"
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

// TestWCRInvariants enforces all structural identity relationships for every year.
// These cover the anomaly report items 5.2 (no invariant enforcement).
func TestWCRInvariants(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	tol := decimal.NewFromFloat(0.01) // 1-cent tolerance for rounding

	config := model.PlanConfig{
		TenantScoped:     model.TenantScoped{TenantID: tenantID},
		ScenarioID:       scenarioID,
		CorporateTaxRate: decimal.NewFromFloat(0.25),
		EmployerTaxRate:  decimal.NewFromFloat(0.42),
		VATRate:          decimal.NewFromFloat(0.2),
	}

	// Non-zero opening balance to exercise year-0 WCRChange formula
	openBal := model.OpeningBalance{
		TenantScoped:        model.TenantScoped{TenantID: tenantID},
		ScenarioID:          scenarioID,
		CustomerReceivables: decimal.NewFromFloat(5000),
		Inventories:         decimal.NewFromFloat(1000),
		SupplierPayables:    decimal.NewFromFloat(2000),
	}

	wcCfg := model.WorkingCapitalConfig{
		TenantScoped:      model.TenantScoped{TenantID: tenantID},
		ScenarioID:        scenarioID,
		CustomerPct0Days:  decimal.NewFromFloat(0.1),
		CustomerPct30Days: decimal.NewFromFloat(0.4),
		CustomerPct60Days: decimal.NewFromFloat(0.3),
		CustomerPct90Days: decimal.NewFromFloat(0.1),
		// CustomerPct120Days = 1 - 0.1 - 0.4 - 0.3 - 0.1 = 0.1
		SupplierPct0Days:  decimal.NewFromFloat(0.2),
		SupplierPct30Days: decimal.NewFromFloat(0.5),
		SupplierPct60Days: decimal.NewFromFloat(0.2),
		SupplierPct90Days: decimal.NewFromFloat(0.1),
		// SupplierPct120Days = 0
		InventoryPctYear1: decimal.NewFromFloat(0.15),
		InventoryPctYear2: decimal.NewFromFloat(0.15),
		InventoryPctYear3: decimal.NewFromFloat(0.15),
		InventoryPctYear4: decimal.NewFromFloat(0.15),
		InventoryPctYear5: decimal.NewFromFloat(0.15),
	}

	rev := model.ConsolidatedRevenue{
		ScenarioID: scenarioID,
		Totals: [5]model.ConsolidatedRevenueYear{
			{TotalTurnover: decimal.NewFromInt(400_000), TotalCOGS: decimal.NewFromInt(120_000)},
			{TotalTurnover: decimal.NewFromInt(480_000), TotalCOGS: decimal.NewFromInt(144_000)},
			{TotalTurnover: decimal.NewFromInt(576_000), TotalCOGS: decimal.NewFromInt(172_800)},
			{TotalTurnover: decimal.NewFromInt(691_200), TotalCOGS: decimal.NewFromInt(207_360)},
			{TotalTurnover: decimal.NewFromInt(829_440), TotalCOGS: decimal.NewFromInt(248_832)},
		},
	}

	opex := model.OpexSummary{
		GrandTotal: [5]decimal.Decimal{
			decimal.NewFromInt(80_000),
			decimal.NewFromInt(96_000),
			decimal.NewFromInt(115_200),
			decimal.NewFromInt(138_240),
			decimal.NewFromInt(165_888),
		},
	}

	staff := model.StaffPayrollSummary{
		ScenarioID: scenarioID,
		Payroll: [5]model.StaffPayrollYear{
			{SubtotalPayroll: decimal.NewFromInt(200_000)},
			{SubtotalPayroll: decimal.NewFromInt(240_000)},
			{SubtotalPayroll: decimal.NewFromInt(288_000)},
			{SubtotalPayroll: decimal.NewFromInt(345_600)},
			{SubtotalPayroll: decimal.NewFromInt(414_720)},
		},
	}

	pnl := model.PnlReport{
		Years: [5]model.PnlYear{
			{PreTaxEarnings: decimal.NewFromInt(50_000)},
			{PreTaxEarnings: decimal.NewFromInt(70_000)},
			{PreTaxEarnings: decimal.NewFromInt(95_000)},
			{PreTaxEarnings: decimal.NewFromInt(130_000)},
			{PreTaxEarnings: decimal.NewFromInt(180_000)},
		},
	}

	capex := model.CapexSummary{
		Totals: model.CapexTotals{
			TotalCapex: [5]decimal.Decimal{
				decimal.NewFromInt(30_000),
				decimal.NewFromInt(25_000),
				decimal.NewFromInt(20_000),
				decimal.NewFromInt(15_000),
				decimal.NewFromInt(10_000),
			},
		},
	}

	r := ComputeWCR([]model.WCREntry{}, rev, opex, staff, wcCfg, openBal, capex, pnl, config)

	// Precompute expected initialWCR
	initialWCR := openBal.CustomerReceivables.Add(openBal.Inventories).Sub(openBal.SupplierPayables)

	for y := 0; y < MaxYears; y++ {
		// ── Identity 1: BasicWCR = CustomerWCR + InventoryWCR − SupplierWCR ──────
		expectedBasic := r.Summary.CustomerWCR[y].
			Add(r.Summary.InventoryWCR[y]).
			Sub(r.Summary.SupplierWCR[y])
		assertDecEqApprox(t, expectedBasic, r.Summary.BasicWCR[y], tol,
			"year %d: basicWcr = customerWcr + inventoryWcr − supplierWcr", y+1)

		// ── Identity 2: AdjustedWCR = BasicWCR + TotalFiscalSocial + NetAdjustment ──
		expectedAdj := r.Summary.BasicWCR[y].
			Add(r.FiscalSocial.TotalFiscalSocial[y]).
			Add(r.Adjustments.NetAdjustment[y])
		assertDecEqApprox(t, expectedAdj, r.Adjusted.AdjustedWCR[y], tol,
			"year %d: adjustedWcr = basicWcr + totalFiscalSocial + netAdjustment", y+1)

		// ── Identity 3: NetVATPayable = VATCollected − VATDeductible ─────────────
		expectedNet := r.FiscalSocial.VATCollected[y].Sub(r.FiscalSocial.VATDeductible[y])
		assertDecEqApprox(t, expectedNet, r.FiscalSocial.NetVATPayable[y], tol,
			"year %d: netVatPayable = vatCollected − vatDeductible", y+1)

		// ── Identity 4: TotalSocialCharges = EmployerCharges + EmployeeCharges ───
		expectedSocial := r.FiscalSocial.EmployerCharges[y].Add(r.FiscalSocial.EmployeeCharges[y])
		assertDecEqApprox(t, expectedSocial, r.FiscalSocial.TotalSocialCharges[y], tol,
			"year %d: totalSocialCharges = employerCharges + employeeCharges", y+1)

		// ── Identity 5: TotalFiscalSocial = VATLiability + SocialLiability + CorpTax ──
		expectedFiscal := r.FiscalSocial.VATLiability[y].
			Add(r.FiscalSocial.SocialLiability[y]).
			Add(r.FiscalSocial.CorporateTaxLiab[y])
		assertDecEqApprox(t, expectedFiscal, r.FiscalSocial.TotalFiscalSocial[y], tol,
			"year %d: totalFiscalSocial = vatLiability + socialLiability + corporateTaxLiab", y+1)

		// ── Identity 6: Sum of customer tranches = TotalCustomers ────────────────
		trancheSum := decimal.Zero
		for k := 0; k < 5; k++ {
			trancheSum = trancheSum.Add(r.Customers.Tranches[y][k])
		}
		assertDecEqApprox(t, trancheSum, r.Customers.TotalCustomers[y], tol,
			"year %d: sum(customerTranches) = totalCustomers", y+1)

		// ── Identity 7: Sum of supplier tranches = TotalSuppliers ────────────────
		supplierSum := decimal.Zero
		for k := 0; k < 5; k++ {
			supplierSum = supplierSum.Add(r.Suppliers.Tranches[y][k])
		}
		assertDecEqApprox(t, supplierSum, r.Suppliers.TotalSuppliers[y], tol,
			"year %d: sum(supplierTranches) = totalSuppliers", y+1)

		// ── Identity 8: WCRChange temporal continuity ────────────────────────────
		if y == 0 {
			// WCRChange[0] = BasicWCR[0] − initialWCR (opening balance)
			expectedChange0 := r.Summary.BasicWCR[0].Sub(initialWCR)
			assertDecEqApprox(t, expectedChange0, r.Summary.WCRChange[0], tol,
				"year 1: wcrChange[0] = basicWcr[0] − initialWcr")
		} else {
			// WCRChange[y] = BasicWCR[y] − BasicWCR[y-1]
			expectedChangeY := r.Summary.BasicWCR[y].Sub(r.Summary.BasicWCR[y-1])
			assertDecEqApprox(t, expectedChangeY, r.Summary.WCRChange[y], tol,
				"year %d: wcrChange[y] = basicWcr[y] − basicWcr[y-1]", y+1)
		}

		// ── Identity 9: InventoryValue = COGSBase × InventoryPct ─────────────────
		expectedInv := r.Inventory.COGSBase[y].Mul(r.Inventory.InventoryPct[y])
		assertDecEqApprox(t, expectedInv, r.Inventory.InventoryValue[y], tol,
			"year %d: inventoryValue = cogsBase × inventoryPct", y+1)
	}

	// ── Identity 10: InitialWCR = CustomerReceivables + Inventories − SupplierPayables ──
	assertDecEqApprox(t, initialWCR, r.Summary.InitialWCR, tol,
		"initialWcr = customerReceivables + inventories − supplierPayables")
	assertDecEqApprox(t, initialWCR, r.Adjusted.InitialAdjWCR, tol,
		"initialAdjWcr = initialWcr (no opening fiscal-social)")
}

// ── Fix-3: WCR uses surplus inventory for manufacturing scenarios ─────────────

// TestWCRSurplusInventoryOverridesCOGSPct verifies that when ConsolidatedRevenue
// carries TotalSurplusInventory (from DriverIndustry products with UnitsProduced),
// ComputeWCR uses that value for Inventory.InventoryValue instead of TotalCOGS×invPct.
func TestWCRSurplusInventoryOverridesCOGSPct(t *testing.T) {
	_ = uuid.New() // keep uuid import used

	wcConfig := model.WorkingCapitalConfig{
		InventoryPctYear1: decimal.NewFromFloat(0.20), // 20% of COGS — ignored when surplus set
		InventoryPctYear2: decimal.NewFromFloat(0.20),
		InventoryPctYear3: decimal.NewFromFloat(0.20),
		InventoryPctYear4: decimal.NewFromFloat(0.20),
		InventoryPctYear5: decimal.NewFromFloat(0.20),
		CustomerPct0Days:  decimal.NewFromInt(1),
		SupplierPct0Days:  decimal.NewFromInt(1),
	}

	// Scenario: year-0 COGS = 100k, invPct = 20% → default inventory = 20k.
	// But the manufacturing surplus is 35k (more finished-goods stock than the % would give).
	revenue := model.ConsolidatedRevenue{
		Totals: [5]model.ConsolidatedRevenueYear{
			{TotalTurnover: decimal.NewFromInt(200_000), TotalCOGS: decimal.NewFromInt(100_000),
				TotalSurplusInventory: decimal.NewFromInt(35_000)},
			{TotalTurnover: decimal.NewFromInt(200_000), TotalCOGS: decimal.NewFromInt(100_000),
				TotalSurplusInventory: decimal.NewFromInt(35_000)},
			{TotalTurnover: decimal.NewFromInt(200_000), TotalCOGS: decimal.NewFromInt(100_000),
				TotalSurplusInventory: decimal.Zero}, // draw-down cleared inventory
			{TotalTurnover: decimal.NewFromInt(200_000), TotalCOGS: decimal.NewFromInt(100_000)},
			{TotalTurnover: decimal.NewFromInt(200_000), TotalCOGS: decimal.NewFromInt(100_000)},
		},
	}

	config := model.PlanConfig{
		VATRate: decimal.NewFromFloat(0.20),
	}

	result := ComputeWCR(nil, revenue, model.OpexSummary{}, model.StaffPayrollSummary{},
		wcConfig, model.OpeningBalance{}, model.CapexSummary{}, model.PnlReport{}, config)

	// Years 0 and 1: surplus inventory overrides COGS×% (35k vs 20k)
	assert.True(t, decimal.NewFromInt(35_000).Equal(result.Inventory.InventoryValue[0]),
		"Year 0: should use TotalSurplusInventory (35k), not COGS×20%% (20k)")
	assert.True(t, decimal.NewFromInt(35_000).Equal(result.Inventory.InventoryValue[1]),
		"Year 1: should use TotalSurplusInventory (35k), not COGS×20%% (20k)")

	// Year 2: surplus is zero → fall back to COGS×invPct = 100k × 20% = 20k
	assert.True(t, decimal.NewFromInt(20_000).Equal(result.Inventory.InventoryValue[2]),
		"Year 2: TotalSurplusInventory=0, should fall back to COGS×invPct (20k)")

	// Years 3 and 4: no surplus set → standard COGS×invPct
	assert.True(t, decimal.NewFromInt(20_000).Equal(result.Inventory.InventoryValue[3]),
		"Year 3: COGS×invPct = 20k")
	assert.True(t, decimal.NewFromInt(20_000).Equal(result.Inventory.InventoryValue[4]),
		"Year 4: COGS×invPct = 20k")
}
