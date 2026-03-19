package compute

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"kerplan/internal/model"
)

func TestComputeFiplan(t *testing.T) {
	scenarioID := uuid.New()

	config := model.PlanConfig{
		ScenarioID:       scenarioID,
		DiscountRate:     decimal.NewFromFloat(0.1),
		CorporateTaxRate: decimal.NewFromFloat(0.25),
		EmployerTaxRate:  decimal.NewFromFloat(0.42),
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
		ScenarioID:      scenarioID,
		CashAndSecurities: decimal.NewFromInt(50000),
	}

	tests := []struct {
		name         string
		entries      []model.FiplanEntry
		capex        model.CapexSummary
		checkFiplan  func(*testing.T, model.FiplanReport)
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

				// Year 1 requirements: capex + dividends + wcr change
				expectedYear1Reqs := decimal.NewFromInt(25000).
					Add(decimal.NewFromInt(10000)).
					Add(decimal.NewFromInt(5000))
				assertDecEq(t, expectedYear1Reqs, result.Plan.Requirements.Total[1],
					"Year 1 requirements should include capex, dividends, and WCR change")

				// Year 1 resources: capital + loans + asset sales
				expectedYear1Res := decimal.NewFromInt(10000).
					Add(decimal.Zero).  // no new loans in this test
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
