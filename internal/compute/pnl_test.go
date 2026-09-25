package compute

import (
	"testing"
	"time"

	"ascenda/internal/model"
	"github.com/shopspring/decimal"
)

func TestComputePnl(t *testing.T) {
	config := model.PlanConfig{
		CorporateTaxRate: decimal.NewFromFloat(0.25),
		EmployerTaxRate:  decimal.NewFromFloat(0.42),
		IncentiveCap:     decimal.NewFromFloat(0.15),
	}

	tests := []struct {
		name              string
		pnlEntries        []model.PnlManualEntry
		revenue           model.ConsolidatedRevenue
		staff             model.StaffPayrollSummary
		capex             model.CapexSummary
		opex              model.OpexSummary
		expectedEBIT      [5]decimal.Decimal
		expectedNetProfit [5]decimal.Decimal
	}{
		{
			name: "basic P&L with positive earnings",
			// YearIndex is 0-based (0..4) in storage, matching opex/capex manual entry convention.
			pnlEntries: []model.PnlManualEntry{
				{YearIndex: 0, LineID: model.PnlOtherOperatingExp, Amount: decimal.NewFromInt(5000)},
				{YearIndex: 1, LineID: model.PnlOtherOperatingExp, Amount: decimal.NewFromInt(5000)},
				{YearIndex: 2, LineID: model.PnlOtherOperatingExp, Amount: decimal.NewFromInt(5000)},
				{YearIndex: 3, LineID: model.PnlOtherOperatingExp, Amount: decimal.NewFromInt(5000)},
				{YearIndex: 4, LineID: model.PnlOtherOperatingExp, Amount: decimal.NewFromInt(5000)},
			},
			revenue: model.ConsolidatedRevenue{
				Totals: [5]model.ConsolidatedRevenueYear{
					{
						TotalTurnover:    decimal.NewFromInt(100000),
						TotalCOGS:        decimal.NewFromInt(40000),
						TotalGrossMargin: decimal.NewFromInt(60000),
						GrossMarginPct:   decimal.NewFromFloat(0.6),
					},
					{
						TotalTurnover:    decimal.NewFromInt(100000),
						TotalCOGS:        decimal.NewFromInt(40000),
						TotalGrossMargin: decimal.NewFromInt(60000),
						GrossMarginPct:   decimal.NewFromFloat(0.6),
					},
					{
						TotalTurnover:    decimal.NewFromInt(100000),
						TotalCOGS:        decimal.NewFromInt(40000),
						TotalGrossMargin: decimal.NewFromInt(60000),
						GrossMarginPct:   decimal.NewFromFloat(0.6),
					},
					{
						TotalTurnover:    decimal.NewFromInt(100000),
						TotalCOGS:        decimal.NewFromInt(40000),
						TotalGrossMargin: decimal.NewFromInt(60000),
						GrossMarginPct:   decimal.NewFromFloat(0.6),
					},
					{
						TotalTurnover:    decimal.NewFromInt(100000),
						TotalCOGS:        decimal.NewFromInt(40000),
						TotalGrossMargin: decimal.NewFromInt(60000),
						GrossMarginPct:   decimal.NewFromFloat(0.6),
					},
				},
			},
			staff: model.StaffPayrollSummary{
				Payroll: [5]model.StaffPayrollYear{
					{
						SubtotalPayroll: decimal.NewFromInt(20000),
						TotalPayroll:    decimal.NewFromInt(20000),
						IncentiveAmount: decimal.NewFromInt(2000),
					},
					{
						SubtotalPayroll: decimal.NewFromInt(20000),
						TotalPayroll:    decimal.NewFromInt(20000),
						IncentiveAmount: decimal.NewFromInt(2000),
					},
					{
						SubtotalPayroll: decimal.NewFromInt(20000),
						TotalPayroll:    decimal.NewFromInt(20000),
						IncentiveAmount: decimal.NewFromInt(2000),
					},
					{
						SubtotalPayroll: decimal.NewFromInt(20000),
						TotalPayroll:    decimal.NewFromInt(20000),
						IncentiveAmount: decimal.NewFromInt(2000),
					},
					{
						SubtotalPayroll: decimal.NewFromInt(20000),
						TotalPayroll:    decimal.NewFromInt(20000),
						IncentiveAmount: decimal.NewFromInt(2000),
					},
				},
			},
			capex: model.CapexSummary{
				Totals: model.CapexTotals{
					TotalDepreciation: [5]decimal.Decimal{
						decimal.Zero, decimal.NewFromInt(1000), decimal.NewFromInt(1000),
						decimal.NewFromInt(1000), decimal.NewFromInt(1000),
					},
				},
			},
			opex: model.OpexSummary{
				GrandTotal: [5]decimal.Decimal{
					decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
				},
			},
			// Calculation for each year:
			// Sales = 100000, COGS = 40000, OtherOpEx = 5000, Payroll = 20000
			// AddedValue = Sales - COGS - OtherOpEx = 100000 - 40000 - 5000 = 55000
			// EBITDA = AddedValue - Payroll = 55000 - 20000 = 35000
			// Year 1: EBIT = 35000 - 0 = 35000, NetProfit = 35000 - (35000 * 0.25) = 26250
			// Year 2-5: EBIT = 35000 - 1000 = 34000, NetProfit = 34000 - (34000 * 0.25) = 25500
			expectedEBIT: [5]decimal.Decimal{
				decimal.NewFromInt(35000), decimal.NewFromInt(34000), decimal.NewFromInt(34000),
				decimal.NewFromInt(34000), decimal.NewFromInt(34000),
			},
			expectedNetProfit: [5]decimal.Decimal{
				decimal.NewFromInt(26250), decimal.NewFromInt(25500), decimal.NewFromInt(25500),
				decimal.NewFromInt(25500), decimal.NewFromInt(25500),
			},
		},
		{
			name: "loss carryforward scenario",
			// YearIndex is 0-based (0..4) in storage, matching opex/capex manual entry convention.
			pnlEntries: []model.PnlManualEntry{
				{YearIndex: 0, LineID: model.PnlOtherOperatingExp, Amount: decimal.NewFromInt(100000)},
				{YearIndex: 1, LineID: model.PnlOtherOperatingExp, Amount: decimal.NewFromInt(10000)},
				{YearIndex: 2, LineID: model.PnlOtherOperatingExp, Amount: decimal.NewFromInt(5000)},
				{YearIndex: 3, LineID: model.PnlOtherOperatingExp, Amount: decimal.NewFromInt(5000)},
				{YearIndex: 4, LineID: model.PnlOtherOperatingExp, Amount: decimal.NewFromInt(5000)},
			},
			revenue: model.ConsolidatedRevenue{
				Totals: [5]model.ConsolidatedRevenueYear{
					{
						TotalTurnover:    decimal.NewFromInt(50000),
						TotalCOGS:        decimal.NewFromInt(20000),
						TotalGrossMargin: decimal.NewFromInt(30000),
					},
					{
						TotalTurnover:    decimal.NewFromInt(50000),
						TotalCOGS:        decimal.NewFromInt(20000),
						TotalGrossMargin: decimal.NewFromInt(30000),
					},
					{
						TotalTurnover:    decimal.NewFromInt(50000),
						TotalCOGS:        decimal.NewFromInt(20000),
						TotalGrossMargin: decimal.NewFromInt(30000),
					},
					{
						TotalTurnover:    decimal.NewFromInt(50000),
						TotalCOGS:        decimal.NewFromInt(20000),
						TotalGrossMargin: decimal.NewFromInt(30000),
					},
					{
						TotalTurnover:    decimal.NewFromInt(50000),
						TotalCOGS:        decimal.NewFromInt(20000),
						TotalGrossMargin: decimal.NewFromInt(30000),
					},
				},
			},
			staff: model.StaffPayrollSummary{
				Payroll: [5]model.StaffPayrollYear{
					{
						SubtotalPayroll: decimal.NewFromInt(10000),
						TotalPayroll:    decimal.NewFromInt(10000),
						IncentiveAmount: decimal.NewFromInt(1000),
					},
					{
						SubtotalPayroll: decimal.NewFromInt(10000),
						TotalPayroll:    decimal.NewFromInt(10000),
						IncentiveAmount: decimal.NewFromInt(1000),
					},
					{
						SubtotalPayroll: decimal.NewFromInt(10000),
						TotalPayroll:    decimal.NewFromInt(10000),
						IncentiveAmount: decimal.NewFromInt(1000),
					},
					{
						SubtotalPayroll: decimal.NewFromInt(10000),
						TotalPayroll:    decimal.NewFromInt(10000),
						IncentiveAmount: decimal.NewFromInt(1000),
					},
					{
						SubtotalPayroll: decimal.NewFromInt(10000),
						TotalPayroll:    decimal.NewFromInt(10000),
						IncentiveAmount: decimal.NewFromInt(1000),
					},
				},
			},
			capex: model.CapexSummary{
				Totals: model.CapexTotals{
					TotalDepreciation: [5]decimal.Decimal{
						decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
					},
				},
			},
			opex: model.OpexSummary{
				GrandTotal: [5]decimal.Decimal{
					decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
				},
			},
			// Calculation for each year:
			// Sales = 50000, COGS = 20000, Payroll = 10000
			// AddedValue = 50000 - 20000 = 30000
			// Year 1: OtherOpEx = 100000, EBITDA = 30000 - 10000 = 20000, EBIT = 20000 - 100000 = -80000
			// Year 2: OtherOpEx = 10000, EBITDA = 30000 - 10000 = 20000, EBIT = 20000 - 10000 = 10000
			// Year 3-5: OtherOpEx = 5000, EBITDA = 30000 - 10000 = 20000, EBIT = 20000 - 5000 = 15000
			// For loss scenario, NetProfit = EBIT (no tax when negative)
			expectedEBIT: [5]decimal.Decimal{
				decimal.NewFromInt(-80000), decimal.NewFromInt(10000), decimal.NewFromInt(15000),
				decimal.NewFromInt(15000), decimal.NewFromInt(15000),
			},
			expectedNetProfit: [5]decimal.Decimal{
				decimal.NewFromInt(-80000),
				decimal.NewFromInt(7500),  // 10000 - (10000 * 0.25)
				decimal.NewFromInt(11250), // 15000 - (15000 * 0.25)
				decimal.NewFromInt(11250), // 15000 - (15000 * 0.25)
				decimal.NewFromInt(11250), // 15000 - (15000 * 0.25)
			},
		},
		{
			name:       "all zero inputs",
			pnlEntries: []model.PnlManualEntry{},
			revenue: model.ConsolidatedRevenue{
				Totals: [5]model.ConsolidatedRevenueYear{
					{TotalTurnover: decimal.Zero, TotalCOGS: decimal.Zero},
					{TotalTurnover: decimal.Zero, TotalCOGS: decimal.Zero},
					{TotalTurnover: decimal.Zero, TotalCOGS: decimal.Zero},
					{TotalTurnover: decimal.Zero, TotalCOGS: decimal.Zero},
					{TotalTurnover: decimal.Zero, TotalCOGS: decimal.Zero},
				},
			},
			staff: model.StaffPayrollSummary{
				Payroll: [5]model.StaffPayrollYear{
					{TotalPayroll: decimal.Zero},
					{TotalPayroll: decimal.Zero},
					{TotalPayroll: decimal.Zero},
					{TotalPayroll: decimal.Zero},
					{TotalPayroll: decimal.Zero},
				},
			},
			capex: model.CapexSummary{
				Totals: model.CapexTotals{
					TotalDepreciation: [5]decimal.Decimal{
						decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
					},
				},
			},
			opex: model.OpexSummary{
				GrandTotal: [5]decimal.Decimal{
					decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
				},
			},
			expectedEBIT: [5]decimal.Decimal{
				decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
			},
			expectedNetProfit: [5]decimal.Decimal{
				decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputePnl(tt.pnlEntries, tt.revenue, tt.staff, tt.capex, tt.opex, model.FiplanReport{}, model.WCRReport{}, config)

			for year := 0; year < 5; year++ {
				if len(tt.expectedEBIT) > 0 && tt.expectedEBIT[year] != decimal.Zero {
					assertDecEq(t, tt.expectedEBIT[year], result.Years[year].EBIT,
						"EBIT mismatch in year %d", year+1)
				}

				assertDecEq(t, tt.expectedNetProfit[year], result.Years[year].NetProfit,
					"Net profit mismatch in year %d", year+1)
			}
		})
	}
}

// ── Fix-1: FiPlan loan interest wired to P&L Financial Expenses ──────────────

// TestPnlFinancialExpensesFromFiplan verifies that FinancialExpenses (P&L line 20)
// is populated from fiplan.LoanInterest and correctly reduces PreTaxEarnings.
func TestPnlFinancialExpensesFromFiplan(t *testing.T) {
	config := model.PlanConfig{
		CorporateTaxRate: decimal.NewFromFloat(0.25),
		ForecastStart:    time.Now(),
	}

	// Minimal revenue to give EBIT = 200 000 per year (before financial charges).
	revenue := model.ConsolidatedRevenue{
		Totals: [5]model.ConsolidatedRevenueYear{
			{TotalTurnover: decimal.NewFromInt(200_000)},
			{TotalTurnover: decimal.NewFromInt(200_000)},
			{TotalTurnover: decimal.NewFromInt(200_000)},
			{TotalTurnover: decimal.NewFromInt(200_000)},
			{TotalTurnover: decimal.NewFromInt(200_000)},
		},
	}

	// Simulate FiPlan with decreasing loan interest (debt is being paid down).
	fiplan := model.FiplanReport{
		LoanInterest: [5]decimal.Decimal{
			decimal.NewFromInt(5_000),
			decimal.NewFromInt(4_000),
			decimal.NewFromInt(3_000),
			decimal.NewFromInt(2_000),
			decimal.NewFromInt(1_000),
		},
	}

	result := ComputePnl(nil, revenue, model.StaffPayrollSummary{}, model.CapexSummary{},
		model.OpexSummary{}, fiplan, model.WCRReport{}, config)

	expectedExpenses := []decimal.Decimal{
		decimal.NewFromInt(5_000),
		decimal.NewFromInt(4_000),
		decimal.NewFromInt(3_000),
		decimal.NewFromInt(2_000),
		decimal.NewFromInt(1_000),
	}
	for y := 0; y < 5; y++ {
		assertDecEq(t, expectedExpenses[y], result.Years[y].FinancialExpenses,
			"Year %d: FinancialExpenses should equal FiPlan.LoanInterest[%d]", y, y)
		// PreTaxEarnings = EBIT + FinancialRevenues - FinancialExpenses
		// EBIT ≈ Sales (no payroll, opex, depn in this minimal test)
		expectedPTE := result.Years[y].EBIT.Sub(expectedExpenses[y])
		assertDecEq(t, expectedPTE, result.Years[y].PreTaxEarnings,
			"Year %d: PreTaxEarnings must be reduced by FinancialExpenses", y)
	}
}

// ── Fix-2: FiPlan subsidies / other grants wired to P&L GrantsOtherRevenue ──

// TestPnlGrantsOtherRevenueFromFiplan verifies that GrantsOtherRevenue (P&L line 16)
// is populated from fiplan.Plan.Resources.Subsidies + OtherGrants, and that
// RepayableGrants (financing, not P&L income) are excluded.
func TestPnlGrantsOtherRevenueFromFiplan(t *testing.T) {
	config := model.PlanConfig{
		CorporateTaxRate: decimal.NewFromFloat(0.25),
		ForecastStart:    time.Now(),
	}

	fiplan := model.FiplanReport{
		Plan: model.FiplanPlan{
			Resources: model.FiplanResources{
				Subsidies:   [5]decimal.Decimal{decimal.NewFromInt(20_000), decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero},
				OtherGrants: [5]decimal.Decimal{decimal.NewFromInt(10_000), decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero},
				// RepayableGrants must NOT appear in GrantsOtherRevenue
				RepayableGrants: [5]decimal.Decimal{decimal.NewFromInt(50_000), decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero},
			},
		},
	}

	result := ComputePnl(nil, model.ConsolidatedRevenue{}, model.StaffPayrollSummary{},
		model.CapexSummary{}, model.OpexSummary{}, fiplan, model.WCRReport{}, config)

	// Year 0: 20k subsidies + 10k other grants = 30k total
	assertDecEq(t, decimal.NewFromInt(30_000), result.Years[0].GrantsOtherRevenue,
		"Year 0 GrantsOtherRevenue should be Subsidies + OtherGrants = 30k")

	// RepayableGrants must not be included
	assertDecEq(t, decimal.NewFromInt(30_000), result.Years[0].GrantsOtherRevenue,
		"RepayableGrants (50k) must not inflate GrantsOtherRevenue")

	// Years 1-4: no grants, should be zero
	for y := 1; y < 5; y++ {
		assertDecEq(t, decimal.Zero, result.Years[y].GrantsOtherRevenue,
			"Year %d: GrantsOtherRevenue should be zero when no grants entered", y)
	}
}

// ── Fix-3: WCR inventory change wired to P&L Stored Production ───────────────

// TestPnlStoredProductionFromWCR verifies that StoredProduction (P&L line 4)
// reflects the annual change in WCR inventory values.  A growing inventory
// creates positive StoredProduction (operating revenue); a draw-down is negative.
func TestPnlStoredProductionFromWCR(t *testing.T) {
	config := model.PlanConfig{
		CorporateTaxRate: decimal.NewFromFloat(0.25),
		ForecastStart:    time.Now(),
	}

	wcr := model.WCRReport{
		Inventory: model.WCRInventory{
			InitialInventory: decimal.NewFromInt(10_000), // opening balance inventory
			InventoryValue: [5]decimal.Decimal{
				decimal.NewFromInt(15_000), // Y1: inventory grew by 5k
				decimal.NewFromInt(20_000), // Y2: grew by 5k
				decimal.NewFromInt(18_000), // Y3: draw-down of 2k
				decimal.NewFromInt(18_000), // Y4: no change
				decimal.NewFromInt(16_000), // Y5: draw-down of 2k
			},
		},
	}

	result := ComputePnl(nil, model.ConsolidatedRevenue{}, model.StaffPayrollSummary{},
		model.CapexSummary{}, model.OpexSummary{}, model.FiplanReport{}, wcr, config)

	expected := []decimal.Decimal{
		decimal.NewFromInt(5_000),  // Y0: 15k - 10k (initial)
		decimal.NewFromInt(5_000),  // Y1: 20k - 15k
		decimal.NewFromInt(-2_000), // Y2: 18k - 20k
		decimal.Zero,               // Y3: 18k - 18k
		decimal.NewFromInt(-2_000), // Y4: 16k - 18k
	}
	for y, exp := range expected {
		assertDecEq(t, exp, result.Years[y].StoredProduction,
			"Year %d: StoredProduction mismatch", y)
	}
}
