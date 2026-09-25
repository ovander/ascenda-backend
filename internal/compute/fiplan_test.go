package compute

import (
	"testing"
	"time"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestComputeFiplan(t *testing.T) {
	scenarioID := uuid.New()

	config := model.PlanConfig{
		ScenarioID:       scenarioID,
		DiscountRate:     decimal.NewFromFloat(0.1),
		CorporateTaxRate: decimal.NewFromFloat(0.25),
		EmployerTaxRate:  decimal.NewFromFloat(0.42),
		MLTLoanTermYears: 5,
		ForecastStart:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	pnlReport := model.PnlReport{
		Years: [5]model.PnlYear{
			{YearIndex: 0, Year: 2025, NetProfit: decimal.NewFromInt(100000), Depreciation: decimal.NewFromInt(5000)},
			{YearIndex: 1, Year: 2026, NetProfit: decimal.NewFromInt(120000), Depreciation: decimal.NewFromInt(5000)},
			{YearIndex: 2, Year: 2027, NetProfit: decimal.NewFromInt(150000), Depreciation: decimal.NewFromInt(5000)},
			{YearIndex: 3, Year: 2028, NetProfit: decimal.NewFromInt(180000), Depreciation: decimal.NewFromInt(5000)},
			{YearIndex: 4, Year: 2029, NetProfit: decimal.NewFromInt(200000), Depreciation: decimal.NewFromInt(5000)},
		},
	}

	wcr := model.WCRReport{
		Summary: model.WCRSummary{
			WCRChange: [5]decimal.Decimal{
				decimal.Zero, decimal.NewFromInt(5000), decimal.NewFromInt(3000),
				decimal.NewFromInt(2000), decimal.Zero,
			},
		},
	}

	openingBal := model.OpeningBalance{
		ScenarioID:        scenarioID,
		CashAndSecurities: decimal.NewFromInt(50000),
	}

	tests := []struct {
		name        string
		entries     []model.FiplanEntry
		capex       model.CapexSummary
		checkFiplan func(*testing.T, model.FiplanReport)
	}{
		{
			name: "sources and uses balancing",
			entries: []model.FiplanEntry{
				// Year 0 (index 0) sources
				{LineID: model.FiplanCapitalIncrease, YearIndex: 1, Amount: decimal.NewFromInt(30000)},
				{LineID: model.FiplanLTLoans, YearIndex: 1, Amount: decimal.NewFromInt(20000)},
				// Year 0 uses
				{LineID: model.FiplanDividends, YearIndex: 1, Amount: decimal.NewFromInt(5000)},
				// Year 1 (index 1) sources
				{LineID: model.FiplanCapitalIncrease, YearIndex: 2, Amount: decimal.NewFromInt(10000)},
				{LineID: model.FiplanAssetSales, YearIndex: 2, Amount: decimal.NewFromInt(15000)},
				// Year 1 uses
				{LineID: model.FiplanDividends, YearIndex: 2, Amount: decimal.NewFromInt(10000)},
			},
			capex: model.CapexSummary{
				Totals: model.CapexTotals{
					TotalCapex: [5]decimal.Decimal{
						decimal.NewFromInt(40000),
						decimal.NewFromInt(25000),
						decimal.Zero, decimal.Zero, decimal.Zero,
					},
				},
			},
			checkFiplan: func(t *testing.T, result model.FiplanReport) {
				// Year 0 requirements: capex + dividends
				expectedYear0Reqs := decimal.NewFromInt(40000).Add(decimal.NewFromInt(5000))
				assertDecEq(t, expectedYear0Reqs, result.Plan.Requirements.Total[0],
					"Year 0 requirements should include capex and dividends")

				// Year 0 resources: capital increase + loans
				expectedYear0Res := decimal.NewFromInt(30000).Add(decimal.NewFromInt(20000))
				assertDecEq(t, expectedYear0Res, result.Plan.Resources.Total[0],
					"Year 0 resources should be capital increase + loans")

				// Year 1 requirements: capex + dividends + wcr change + loan repayment
				// (20k loan drawn in Year 0, 5-yr term → 20k/5 = 4k instalment from Year 1)
				expectedYear1Reqs := decimal.NewFromInt(25000).
					Add(decimal.NewFromInt(10000)).
					Add(decimal.NewFromInt(5000)).
					Add(decimal.NewFromInt(4000))
				assertDecEq(t, expectedYear1Reqs, result.Plan.Requirements.Total[1],
					"Year 1 requirements should include capex, dividends, WCR change, and loan repayment")

				// Year 1 resources: capital + loans + asset sales
				expectedYear1Res := decimal.NewFromInt(10000).
					Add(decimal.Zero). // no new loans in this test
					Add(decimal.NewFromInt(15000))
				assertDecEq(t, expectedYear1Res, result.Plan.Resources.Total[1],
					"Year 1 resources should be capital + asset sales")
			},
		},
		{
			name: "cumulative cash flow calculation",
			entries: []model.FiplanEntry{
				{LineID: model.FiplanCapitalIncrease, YearIndex: 1, Amount: decimal.NewFromInt(100000)},
				{LineID: model.FiplanCapitalIncrease, YearIndex: 2, Amount: decimal.NewFromInt(50000)},
			},
			capex: model.CapexSummary{
				Totals: model.CapexTotals{
					TotalCapex: [5]decimal.Decimal{
						decimal.NewFromInt(30000),
						decimal.NewFromInt(20000),
						decimal.NewFromInt(15000),
						decimal.NewFromInt(10000),
						decimal.Zero,
					},
				},
			},
			checkFiplan: func(t *testing.T, result model.FiplanReport) {
				// Year 0 balance should be positive (operating cash flow exists from PnL)
				if result.Plan.Balance.AnnualBalance[0].IsNegative() {
					t.Errorf("Year 0 balance should be non-negative, got %v", result.Plan.Balance.AnnualBalance[0])
				}

				// Year 0 cumulative should equal balance + opening cash
				expectedYear0Cum := result.Plan.Balance.AnnualBalance[0].Add(result.Plan.Balance.InitialCash)
				assertDecEq(t, expectedYear0Cum, result.Plan.Balance.CumulativeCash[0],
					"Year 0 cumulative should be balance + opening cash")

				// Year 1 cumulative should be previous cumulative + year 1 balance
				expectedYear1Cum := result.Plan.Balance.CumulativeCash[0].Add(result.Plan.Balance.AnnualBalance[1])
				assertDecEq(t, expectedYear1Cum, result.Plan.Balance.CumulativeCash[1],
					"Year 1 cumulative should be previous cumulative + balance")
			},
		},
		{
			name:    "empty entries with capex only",
			entries: []model.FiplanEntry{},
			capex: model.CapexSummary{
				Totals: model.CapexTotals{
					TotalCapex: [5]decimal.Decimal{
						decimal.NewFromInt(50000),
						decimal.NewFromInt(40000),
						decimal.NewFromInt(30000),
						decimal.NewFromInt(20000),
						decimal.NewFromInt(10000),
					},
				},
			},
			checkFiplan: func(t *testing.T, result model.FiplanReport) {
				// Requirements should have capex values
				for year := 0; year < 5; year++ {
					if result.Plan.Requirements.Capex[year].IsZero() && year < 5 {
						t.Errorf("Year %d capex should not be zero, got %v", year, result.Plan.Requirements.Capex[year])
					}
				}

				// With no external sources, resources may be lower than requirements
				// This tests the balance calculation logic
				for year := 0; year < 5; year++ {
					expectedBalance := result.Plan.Resources.Total[year].Sub(result.Plan.Requirements.Total[year])
					assertDecEq(t, expectedBalance, result.Plan.Balance.AnnualBalance[year],
						"Year %d balance should be resources minus requirements", year)
				}
			},
		},
		{
			name: "multiple source types aggregation",
			entries: []model.FiplanEntry{
				{LineID: model.FiplanCapitalIncrease, YearIndex: 1, Amount: decimal.NewFromInt(50000)},
				{LineID: model.FiplanCurrentAccountContrib, YearIndex: 1, Amount: decimal.NewFromInt(20000)},
				{LineID: model.FiplanSubsidies, YearIndex: 1, Amount: decimal.NewFromInt(30000)},
				{LineID: model.FiplanOtherGrants, YearIndex: 1, Amount: decimal.NewFromInt(15000)},
				{LineID: model.FiplanRepayableGrants, YearIndex: 1, Amount: decimal.NewFromInt(25000)},
				{LineID: model.FiplanLTLoans, YearIndex: 1, Amount: decimal.NewFromInt(40000)},
				{LineID: model.FiplanDividends, YearIndex: 1, Amount: decimal.NewFromInt(10000)},
				{LineID: model.FiplanGrantRepayments, YearIndex: 1, Amount: decimal.NewFromInt(5000)},
			},
			capex: model.CapexSummary{
				Totals: model.CapexTotals{
					TotalCapex: [5]decimal.Decimal{
						decimal.NewFromInt(60000),
						decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
					},
				},
			},
			checkFiplan: func(t *testing.T, result model.FiplanReport) {
				// Total resources: 50k + 20k + 30k + 15k + 25k + 40k = 180k
				expectedResources := decimal.NewFromInt(50000).
					Add(decimal.NewFromInt(20000)).
					Add(decimal.NewFromInt(30000)).
					Add(decimal.NewFromInt(15000)).
					Add(decimal.NewFromInt(25000)).
					Add(decimal.NewFromInt(40000))
				assertDecEq(t, expectedResources, result.Plan.Resources.Total[0],
					"Resources should be sum of all resource lines")

				// Total requirements: 60k (capex) + 10k (dividends) + 5k (grant repay) = 75k
				expectedRequirements := decimal.NewFromInt(60000).
					Add(decimal.NewFromInt(10000)).
					Add(decimal.NewFromInt(5000))
				assertDecEq(t, expectedRequirements, result.Plan.Requirements.Total[0],
					"Requirements should be sum of capex and use lines")

				// Balance should be resources - requirements
				expectedBalance := expectedResources.Sub(expectedRequirements)
				assertDecEq(t, expectedBalance, result.Plan.Balance.AnnualBalance[0],
					"Balance should be resources minus requirements")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeFiplan(tt.entries, tt.capex, pnlReport, wcr, openingBal, config)
			tt.checkFiplan(t, result)
		})
	}
}

// TestFiplanNegativeCAF verifies the CAF split logic introduced to fix the
// NegativeCashFlow placeholder bug:
//   - When pnl.CashFlow < 0 → Requirements.NegativeCashFlow = |cashFlow|,
//     Resources.PositiveCashFlow = 0
//   - When pnl.CashFlow >= 0 → Resources.PositiveCashFlow = cashFlow,
//     Requirements.NegativeCashFlow = 0
//
// It also verifies that the cross-statement identity (annualBalance ==
// changeInCash) still holds in the negative-CAF scenario.
func TestFiplanNegativeCAF(t *testing.T) {
	scenarioID := uuid.New()
	config := model.PlanConfig{ScenarioID: scenarioID}
	wcr := model.WCRReport{}
	capex := model.CapexSummary{}
	openingBal := model.OpeningBalance{ScenarioID: scenarioID, CashAndSecurities: decimal.NewFromInt(100000)}

	t.Run("negative cashflow goes into NegativeCashFlow requirement", func(t *testing.T) {
		negativeCF := decimal.NewFromInt(-50000)
		pnl := model.PnlReport{
			Years: [5]model.PnlYear{
				{NetProfit: decimal.NewFromInt(-55000), Depreciation: decimal.NewFromInt(5000), CashFlow: negativeCF},
			},
		}
		result := ComputeFiplan(nil, capex, pnl, wcr, openingBal, config)

		assertDecEq(t, negativeCF.Abs(), result.Plan.Requirements.NegativeCashFlow[0],
			"NegativeCashFlow should equal |cashFlow| when cashFlow is negative")
		assertDecEq(t, decimal.Zero, result.Plan.Resources.PositiveCashFlow[0],
			"PositiveCashFlow should be zero when cashFlow is negative")
	})

	t.Run("positive cashflow goes into PositiveCashFlow resource", func(t *testing.T) {
		positiveCF := decimal.NewFromInt(80000)
		pnl := model.PnlReport{
			Years: [5]model.PnlYear{
				{NetProfit: decimal.NewFromInt(75000), Depreciation: decimal.NewFromInt(5000), CashFlow: positiveCF},
			},
		}
		result := ComputeFiplan(nil, capex, pnl, wcr, openingBal, config)

		assertDecEq(t, positiveCF, result.Plan.Resources.PositiveCashFlow[0],
			"PositiveCashFlow should equal cashFlow when cashFlow is positive")
		assertDecEq(t, decimal.Zero, result.Plan.Requirements.NegativeCashFlow[0],
			"NegativeCashFlow should be zero when cashFlow is positive")
	})

	t.Run("zero cashflow produces no requirement and no resource", func(t *testing.T) {
		pnl := model.PnlReport{}
		result := ComputeFiplan(nil, capex, pnl, wcr, openingBal, config)

		assertDecEq(t, decimal.Zero, result.Plan.Requirements.NegativeCashFlow[0],
			"NegativeCashFlow should be zero when cashFlow is zero")
		assertDecEq(t, decimal.Zero, result.Plan.Resources.PositiveCashFlow[0],
			"PositiveCashFlow should be zero when cashFlow is zero")
	})

	t.Run("annualBalance equals changeInCash when CAF is negative (cross-statement identity)", func(t *testing.T) {
		// Year 0: loss-making (CashFlow = -110,194), CAPEX = 35,000, WCR = -57,771
		// mirrors the real-world Consulting Firm Demo scenario from the bug report.
		pnl := model.PnlReport{
			Years: [5]model.PnlYear{
				{
					NetProfit:    decimal.NewFromInt(-115194),
					Depreciation: decimal.NewFromInt(5000),
					CashFlow:     decimal.NewFromInt(-110194),
				},
			},
		}
		capexWithData := model.CapexSummary{
			Totals: model.CapexTotals{
				TotalCapex:        [5]decimal.Decimal{decimal.NewFromInt(35000)},
				TotalDepreciation: [5]decimal.Decimal{decimal.NewFromInt(5000)},
			},
		}
		wcrWithData := model.WCRReport{
			Summary: model.WCRSummary{
				WCRChange: [5]decimal.Decimal{decimal.NewFromInt(-57771)},
			},
		}
		entries := []model.FiplanEntry{
			{LineID: model.FiplanCapitalIncrease, YearIndex: 1, Amount: decimal.NewFromInt(200000)},
		}
		result := ComputeFiplan(entries, capexWithData, pnl, wcrWithData, openingBal, config)

		annualBalance := result.Plan.Balance.AnnualBalance[0]
		changeInCash := result.CashFlow.Summary.ChangeInCash[0]
		if !annualBalance.Equal(changeInCash) {
			t.Errorf("annualBalance (%s) != changeInCash (%s) — cross-statement identity broken for negative CAF",
				annualBalance, changeInCash)
		}
	})

	t.Run("mixed years: negative then positive CAF", func(t *testing.T) {
		// Keep depreciation = 0 so PnlYear.CashFlow == NetProfit, which is also what
		// the cash flow statement computes (CashFlowCAF = NetProfit + capex.TotalDepreciation,
		// and capex is empty here). Mixing a non-zero PnlYear.Depreciation with an empty
		// capex would produce a false cross-statement mismatch.
		pnl := model.PnlReport{
			Years: [5]model.PnlYear{
				{NetProfit: decimal.NewFromInt(-15000), CashFlow: decimal.NewFromInt(-15000)},
				{NetProfit: decimal.NewFromInt(35000), CashFlow: decimal.NewFromInt(35000)},
				{NetProfit: decimal.Zero, CashFlow: decimal.Zero},
			},
		}
		result := ComputeFiplan(nil, capex, pnl, wcr, openingBal, config)

		// Year 0: negative → NegativeCashFlow
		assertDecEq(t, decimal.NewFromInt(15000), result.Plan.Requirements.NegativeCashFlow[0],
			"year 0 NegativeCashFlow should be 15,000")
		assertDecEq(t, decimal.Zero, result.Plan.Resources.PositiveCashFlow[0],
			"year 0 PositiveCashFlow should be 0")

		// Year 1: positive → PositiveCashFlow
		assertDecEq(t, decimal.Zero, result.Plan.Requirements.NegativeCashFlow[1],
			"year 1 NegativeCashFlow should be 0")
		assertDecEq(t, decimal.NewFromInt(35000), result.Plan.Resources.PositiveCashFlow[1],
			"year 1 PositiveCashFlow should be 35,000")

		// Year 2: zero → both zero
		assertDecEq(t, decimal.Zero, result.Plan.Requirements.NegativeCashFlow[2],
			"year 2 NegativeCashFlow should be 0")
		assertDecEq(t, decimal.Zero, result.Plan.Resources.PositiveCashFlow[2],
			"year 2 PositiveCashFlow should be 0")

		// Cross-statement identity must hold for all three years
		for year := 0; year < 3; year++ {
			if !result.Plan.Balance.AnnualBalance[year].Equal(result.CashFlow.Summary.ChangeInCash[year]) {
				t.Errorf("year %d: annualBalance (%s) != changeInCash (%s)",
					year, result.Plan.Balance.AnnualBalance[year], result.CashFlow.Summary.ChangeInCash[year])
			}
		}
	})
}

// TestFiplanCrossValidation enforces the two key inter-statement identities:
//
//  1. Cash Flow Identity (§4.2):
//     plan.balance.annualBalance == cashFlow.summary.changeInCash
//     Both are the net cash change for the year, derived independently:
//     - Plan path:       resources.total − requirements.total
//     - Cash Flow path:  operatingFlows + investmentFlows + financingFlows
//
//  2. Operating Flow Decomposition (§4.5):
//     cashFlow.operating.operatingFlows == cashFlowCAF − wcrChange
//
// These tests guarantee that the two derivation paths can never silently diverge.
func TestFiplanCrossValidation(t *testing.T) {
	scenarioID := uuid.New()

	config := model.PlanConfig{
		ScenarioID:       scenarioID,
		CorporateTaxRate: decimal.NewFromFloat(0.25),
		EmployerTaxRate:  decimal.NewFromFloat(0.42),
	}

	// Use non-round WCR values to exercise the rounding path (repeating decimals from ÷365)
	wcrReport := model.WCRReport{
		Summary: model.WCRSummary{
			WCRChange: [5]decimal.Decimal{
				decimal.NewFromFloat(1000.333333333),
				decimal.NewFromFloat(500.666666666),
				decimal.NewFromFloat(250.123456789),
				decimal.NewFromFloat(125.987654321),
				decimal.Zero,
			},
		},
	}

	pnlReport := model.PnlReport{
		Years: [5]model.PnlYear{
			{NetProfit: decimal.NewFromInt(50000), Depreciation: decimal.NewFromInt(3000), CashFlow: decimal.NewFromInt(53000)},
			{NetProfit: decimal.NewFromInt(60000), Depreciation: decimal.NewFromInt(3000), CashFlow: decimal.NewFromInt(63000)},
			{NetProfit: decimal.NewFromInt(70000), Depreciation: decimal.NewFromInt(3000), CashFlow: decimal.NewFromInt(73000)},
			{NetProfit: decimal.NewFromInt(80000), Depreciation: decimal.NewFromInt(3000), CashFlow: decimal.NewFromInt(83000)},
			{NetProfit: decimal.NewFromInt(90000), Depreciation: decimal.NewFromInt(3000), CashFlow: decimal.NewFromInt(93000)},
		},
	}

	capex := model.CapexSummary{
		Totals: model.CapexTotals{
			TotalCapex:        [5]decimal.Decimal{decimal.NewFromInt(10000), decimal.NewFromInt(8000), decimal.NewFromInt(6000), decimal.NewFromInt(4000), decimal.NewFromInt(2000)},
			TotalDepreciation: [5]decimal.Decimal{decimal.NewFromInt(3000), decimal.NewFromInt(3000), decimal.NewFromInt(3000), decimal.NewFromInt(3000), decimal.NewFromInt(3000)},
		},
	}

	entries := []model.FiplanEntry{
		{LineID: model.FiplanCapitalIncrease, YearIndex: 1, Amount: decimal.NewFromInt(20000)},
		{LineID: model.FiplanLTLoans, YearIndex: 1, Amount: decimal.NewFromInt(15000)},
		{LineID: model.FiplanDividends, YearIndex: 3, Amount: decimal.NewFromInt(5000)},
	}

	openingBal := model.OpeningBalance{
		ScenarioID:        scenarioID,
		CashAndSecurities: decimal.NewFromInt(10000),
	}

	result := ComputeFiplan(entries, capex, pnlReport, wcrReport, openingBal, config)

	for year := 0; year < 5; year++ {
		// Identity 1: annualBalance == changeInCash
		annualBalance := result.Plan.Balance.AnnualBalance[year]
		changeInCash := result.CashFlow.Summary.ChangeInCash[year]
		if !annualBalance.Equal(changeInCash) {
			t.Errorf("year %d: plan.annualBalance (%s) != cashFlow.changeInCash (%s) — cash flow identity violated",
				year, annualBalance, changeInCash)
		}

		// Identity 2: operatingFlows == cashFlowCAF - wcrChange
		caf := result.CashFlow.Operating.CashFlowCAF[year]
		wcrChange := result.CashFlow.Operating.WCRChange[year]
		operatingFlows := result.CashFlow.Operating.OperatingFlows[year]
		expectedOperating := caf.Sub(wcrChange)
		if !operatingFlows.Equal(expectedOperating) {
			t.Errorf("year %d: operatingFlows (%s) != CAF (%s) - wcrChange (%s) — operating decomposition violated",
				year, operatingFlows, caf, wcrChange)
		}
	}
}

// ── Fix-1: new LT loan repayment schedule ────────────────────────────────────

// TestFiplanNewLoanRepaymentSchedule verifies that a new LT loan drawn in Year 1
// is repaid in instalments over the loan term, reducing the outstanding balance
// (and therefore interest) in subsequent years.  Prior to the fix, new loans
// accumulated without being repaid, causing infinite interest growth.
func TestFiplanNewLoanRepaymentSchedule(t *testing.T) {
	scenarioID := uuid.New()

	config := model.PlanConfig{
		ScenarioID:       scenarioID,
		MLTInterestRate:  decimal.NewFromFloat(0.05), // 5 %
		MLTLoanTermYears: 5,
		ForecastStart:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	openBal := model.OpeningBalance{
		ScenarioID:        scenarioID,
		CashAndSecurities: decimal.Zero,
		LoansAndDebt:      decimal.Zero, // no pre-existing debt
	}

	// Draw a 100 k loan in Year 1 of the forecast (computation yearIndex 1).
	// FiplanEntry.YearIndex uses 1-based numbering: YearIndex=2 → getEntry(..., yearIndex+1=2)
	// resolves at computation yearIndex=1, which is the second forecast year.
	entries := []model.FiplanEntry{
		{LineID: model.FiplanLTLoans, YearIndex: 2, Amount: decimal.NewFromInt(100_000)},
	}

	emptyPnl := model.PnlReport{}
	emptyWCR := model.WCRReport{}
	emptyCapex := model.CapexSummary{}

	result := ComputeFiplan(entries, emptyCapex, emptyPnl, emptyWCR, openBal, config)

	// Year 0 (before any loan draw): no interest (balance = 0)
	assertDecEq(t, decimal.Zero, result.LoanInterest[0], "Year 0 interest should be zero — no debt yet")

	// Year 1: loan drawn *after* interest is computed, so still no interest this year
	assertDecEq(t, decimal.Zero, result.LoanInterest[1], "Year 1 interest should be zero — loan drawn end of period")

	// Year 2: balance = 100k drawn − 0 (no repayment on opening, new loan repayments start Y2)
	// First instalment of 100k/5 = 20k is due in Year 2, so balance at start of Year 2 = 100k.
	expectedInterestY2 := decimal.NewFromInt(100_000).Mul(decimal.NewFromFloat(0.05)) // 5 000
	assertDecEq(t, expectedInterestY2, result.LoanInterest[2], "Year 2 interest should be 5% of 100k")

	// Year 3: balance after Y2 repayment = 100k − 20k = 80k.
	expectedInterestY3 := decimal.NewFromInt(80_000).Mul(decimal.NewFromFloat(0.05)) // 4 000
	assertDecEq(t, expectedInterestY3, result.LoanInterest[3], "Year 3 interest should be 5% of 80k")

	// Year 4: balance after Y3 repayment = 80k − 20k = 60k.
	expectedInterestY4 := decimal.NewFromInt(60_000).Mul(decimal.NewFromFloat(0.05)) // 3 000
	assertDecEq(t, expectedInterestY4, result.LoanInterest[4], "Year 4 interest should be 5% of 60k")

	// The repayment schedule should reflect the instalment (100k / 5 = 20k) in Years 2-5.
	// Within the 5-year plan window we see repayments for Y2, Y3, Y4 (indices 2, 3, 4).
	expectedInstalment := decimal.NewFromInt(20_000)
	assertDecEq(t, decimal.Zero, result.Plan.Requirements.LoanRepayments[0], "Year 0: no repayment")
	assertDecEq(t, decimal.Zero, result.Plan.Requirements.LoanRepayments[1], "Year 1: no repayment (draw year)")
	assertDecEq(t, expectedInstalment, result.Plan.Requirements.LoanRepayments[2], "Year 2: first instalment")
	assertDecEq(t, expectedInstalment, result.Plan.Requirements.LoanRepayments[3], "Year 3: second instalment")
	assertDecEq(t, expectedInstalment, result.Plan.Requirements.LoanRepayments[4], "Year 4: third instalment")
}

// TestFiplanOpeningDebtRepayment verifies that opening-balance debt is repaid
// evenly across loanTerm years and that interest shrinks as principal is paid down.
func TestFiplanOpeningDebtRepayment(t *testing.T) {
	scenarioID := uuid.New()

	config := model.PlanConfig{
		ScenarioID:       scenarioID,
		MLTInterestRate:  decimal.NewFromFloat(0.05), // 5 %
		MLTLoanTermYears: 5,
		ForecastStart:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	openBal := model.OpeningBalance{
		ScenarioID:   scenarioID,
		LoansAndDebt: decimal.NewFromInt(100_000),
	}

	result := ComputeFiplan(nil, model.CapexSummary{}, model.PnlReport{}, model.WCRReport{}, openBal, config)

	// Principal repayment = 100k / 5 = 20k each year
	repayment := decimal.NewFromInt(20_000)
	for y := 0; y < 5; y++ {
		assertDecEq(t, repayment, result.Plan.Requirements.LoanRepayments[y],
			"Year %d: opening debt repayment should be 20k", y)
	}

	// Interest[y] = balance_at_start_of_year × 5%
	// Y0: 100k → 5 000 ; Y1: 80k → 4 000 ; ... Y4: 20k → 1 000
	expectedInterests := []int64{5_000, 4_000, 3_000, 2_000, 1_000}
	for y, expInt := range expectedInterests {
		assertDecEq(t, decimal.NewFromInt(expInt), result.LoanInterest[y],
			"Year %d: interest should be %d", y, expInt)
	}
}

// ── Fix-2: grants not double-counted in cash flow ─────────────────────────────

// TestFiplanGrantsNotInFinancingCashFlow verifies that non-repayable subsidies and
// other grants are NOT included in the Cash Flow Financing section.  They flow
// through P&L as operating income (GrantsOtherRevenue), and including them in
// financing flows would double-count them.
func TestFiplanGrantsNotInFinancingCashFlow(t *testing.T) {
	scenarioID := uuid.New()

	config := model.PlanConfig{
		ScenarioID:    scenarioID,
		ForecastStart: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	entries := []model.FiplanEntry{
		{LineID: model.FiplanSubsidies, YearIndex: 1, Amount: decimal.NewFromInt(30_000)},
		{LineID: model.FiplanOtherGrants, YearIndex: 1, Amount: decimal.NewFromInt(20_000)},
		{LineID: model.FiplanRepayableGrants, YearIndex: 1, Amount: decimal.NewFromInt(10_000)},
		{LineID: model.FiplanLTLoans, YearIndex: 1, Amount: decimal.NewFromInt(50_000)},
	}

	result := ComputeFiplan(entries, model.CapexSummary{}, model.PnlReport{}, model.WCRReport{},
		model.OpeningBalance{ScenarioID: scenarioID}, config)

	// NewLoansAndGrants should only contain repayable financing: LT loans + repayable grants
	expectedFinancing := decimal.NewFromInt(50_000).Add(decimal.NewFromInt(10_000)) // 60k
	assertDecEq(t, expectedFinancing, result.CashFlow.Financing.NewLoansAndGrants[0],
		"Financing NewLoansAndGrants should be LTLoans + RepayableGrants only (60k), not 110k")

	// Subsidies and OtherGrants should still appear in Resources (for the financing plan table)
	assertDecEq(t, decimal.NewFromInt(30_000), result.Plan.Resources.Subsidies[0],
		"Subsidies should be in Resources")
	assertDecEq(t, decimal.NewFromInt(20_000), result.Plan.Resources.OtherGrants[0],
		"OtherGrants should be in Resources")
}
