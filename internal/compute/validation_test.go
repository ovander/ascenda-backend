package compute

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"ascenda/internal/model"
)

func TestValidateOutput(t *testing.T) {
	tests := []struct {
		name                  string
		output                model.FullPlanOutput
		expectedWarningCount  int
		expectedMinSeverity   string
	}{
		{
			name: "balanced balance sheet",
			output: model.FullPlanOutput{
				BSheet: model.BSheetReport{
					Detailed: model.BSheetDetailed{
						Assets: model.BSheetDetailedAssets{
							TotalAssets: [6]decimal.Decimal{
								decimal.NewFromInt(100000), decimal.NewFromInt(110000), decimal.NewFromInt(120000),
								decimal.NewFromInt(130000), decimal.NewFromInt(140000), decimal.NewFromInt(150000),
							},
							Cash: [6]decimal.Decimal{
								decimal.NewFromInt(10000), decimal.NewFromInt(15000), decimal.NewFromInt(20000),
								decimal.NewFromInt(25000), decimal.NewFromInt(30000), decimal.NewFromInt(35000),
							},
						},
						Liabilities: model.BSheetDetailedLiabilities{
							TotalLiabilities: [6]decimal.Decimal{
								decimal.NewFromInt(100000), decimal.NewFromInt(110000), decimal.NewFromInt(120000),
								decimal.NewFromInt(130000), decimal.NewFromInt(140000), decimal.NewFromInt(150000),
							},
						},
					},
					Equity: [6]decimal.Decimal{
						decimal.NewFromInt(50000), decimal.NewFromInt(55000), decimal.NewFromInt(60000),
						decimal.NewFromInt(65000), decimal.NewFromInt(70000), decimal.NewFromInt(75000),
					},
				},
				Revenue: model.ConsolidatedRevenue{
					Totals: [5]model.ConsolidatedRevenueYear{
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
					},
				},
				WCR: model.WCRReport{
					Summary: model.WCRSummary{
						BasicWCR: [5]decimal.Decimal{
							decimal.NewFromInt(10000), decimal.NewFromInt(10000), decimal.NewFromInt(10000),
							decimal.NewFromInt(10000), decimal.NewFromInt(10000),
						},
					},
				},
				Capex: model.CapexSummary{
					Totals: model.CapexTotals{
						TotalDepreciation: [5]decimal.Decimal{
							decimal.NewFromInt(5000), decimal.NewFromInt(5000), decimal.NewFromInt(5000),
							decimal.NewFromInt(5000), decimal.NewFromInt(5000),
						},
						NetAssets: [6]decimal.Decimal{
							decimal.NewFromInt(50000), decimal.NewFromInt(45000), decimal.NewFromInt(40000),
							decimal.NewFromInt(35000), decimal.NewFromInt(30000), decimal.NewFromInt(25000),
						},
					},
				},
				PnL: model.PnlReport{
					Years: [5]model.PnlYear{
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
					},
				},
				FiPlan: model.FiplanReport{
					Plan: model.FiplanPlan{
						Balance: model.FiplanBalance{
							CumulativeCash: [5]decimal.Decimal{
								decimal.NewFromInt(10000), decimal.NewFromInt(15000), decimal.NewFromInt(20000),
								decimal.NewFromInt(25000), decimal.NewFromInt(30000),
							},
						},
					},
					Warning: [5]bool{false, false, false, false, false},
				},
			},
			expectedWarningCount: 0,
		},
		{
			name: "unbalanced balance sheet",
			output: model.FullPlanOutput{
				BSheet: model.BSheetReport{
					Detailed: model.BSheetDetailed{
						Assets: model.BSheetDetailedAssets{
							TotalAssets: [6]decimal.Decimal{
								decimal.NewFromInt(100000), decimal.NewFromInt(110000), decimal.NewFromInt(120000),
								decimal.NewFromInt(130000), decimal.NewFromInt(140000), decimal.NewFromInt(150000),
							},
							Cash: [6]decimal.Decimal{
								decimal.NewFromInt(10000), decimal.NewFromInt(15000), decimal.NewFromInt(20000),
								decimal.NewFromInt(25000), decimal.NewFromInt(30000), decimal.NewFromInt(35000),
							},
						},
						Liabilities: model.BSheetDetailedLiabilities{
							TotalLiabilities: [6]decimal.Decimal{
								decimal.NewFromInt(50000), decimal.NewFromInt(55000), decimal.NewFromInt(60000),
								decimal.NewFromInt(65000), decimal.NewFromInt(70000), decimal.NewFromInt(75000),
							},
						},
					},
					Equity: [6]decimal.Decimal{
						decimal.NewFromInt(50000), decimal.NewFromInt(55000), decimal.NewFromInt(60000),
						decimal.NewFromInt(65000), decimal.NewFromInt(70000), decimal.NewFromInt(75000),
					},
				},
				Revenue: model.ConsolidatedRevenue{
					Totals: [5]model.ConsolidatedRevenueYear{
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
					},
				},
				WCR: model.WCRReport{
					Summary: model.WCRSummary{
						BasicWCR: [5]decimal.Decimal{
							decimal.NewFromInt(10000), decimal.NewFromInt(10000), decimal.NewFromInt(10000),
							decimal.NewFromInt(10000), decimal.NewFromInt(10000),
						},
					},
				},
				Capex: model.CapexSummary{
					Totals: model.CapexTotals{
						TotalDepreciation: [5]decimal.Decimal{
							decimal.NewFromInt(5000), decimal.NewFromInt(5000), decimal.NewFromInt(5000),
							decimal.NewFromInt(5000), decimal.NewFromInt(5000),
						},
						NetAssets: [6]decimal.Decimal{
							decimal.NewFromInt(50000), decimal.NewFromInt(45000), decimal.NewFromInt(40000),
							decimal.NewFromInt(35000), decimal.NewFromInt(30000), decimal.NewFromInt(25000),
						},
					},
				},
				PnL: model.PnlReport{
					Years: [5]model.PnlYear{
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
					},
				},
				FiPlan: model.FiplanReport{
					Plan: model.FiplanPlan{
						Balance: model.FiplanBalance{
							CumulativeCash: [5]decimal.Decimal{
								decimal.NewFromInt(10000), decimal.NewFromInt(15000), decimal.NewFromInt(20000),
								decimal.NewFromInt(25000), decimal.NewFromInt(30000),
							},
						},
					},
					Warning: [5]bool{false, false, false, false, false},
				},
			},
			expectedWarningCount: 6, // Balance sheet imbalance in all 6 years
		},
		{
			name: "negative cash balance in BSheet",
			output: model.FullPlanOutput{
				BSheet: model.BSheetReport{
					Detailed: model.BSheetDetailed{
						Assets: model.BSheetDetailedAssets{
							TotalAssets: [6]decimal.Decimal{
								decimal.NewFromInt(100000), decimal.NewFromInt(110000), decimal.NewFromInt(120000),
								decimal.NewFromInt(130000), decimal.NewFromInt(140000), decimal.NewFromInt(150000),
							},
							Cash: [6]decimal.Decimal{
								decimal.NewFromInt(10000), decimal.NewFromInt(-5000), decimal.NewFromInt(20000),
								decimal.NewFromInt(25000), decimal.NewFromInt(30000), decimal.NewFromInt(35000),
							},
						},
						Liabilities: model.BSheetDetailedLiabilities{
							TotalLiabilities: [6]decimal.Decimal{
								decimal.NewFromInt(100000), decimal.NewFromInt(110000), decimal.NewFromInt(120000),
								decimal.NewFromInt(130000), decimal.NewFromInt(140000), decimal.NewFromInt(150000),
							},
						},
					},
					Equity: [6]decimal.Decimal{
						decimal.NewFromInt(50000), decimal.NewFromInt(55000), decimal.NewFromInt(60000),
						decimal.NewFromInt(65000), decimal.NewFromInt(70000), decimal.NewFromInt(75000),
					},
				},
				Revenue: model.ConsolidatedRevenue{
					Totals: [5]model.ConsolidatedRevenueYear{
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
					},
				},
				WCR: model.WCRReport{
					Summary: model.WCRSummary{
						BasicWCR: [5]decimal.Decimal{
							decimal.NewFromInt(10000), decimal.NewFromInt(10000), decimal.NewFromInt(10000),
							decimal.NewFromInt(10000), decimal.NewFromInt(10000),
						},
					},
				},
				Capex: model.CapexSummary{
					Totals: model.CapexTotals{
						TotalDepreciation: [5]decimal.Decimal{
							decimal.NewFromInt(5000), decimal.NewFromInt(5000), decimal.NewFromInt(5000),
							decimal.NewFromInt(5000), decimal.NewFromInt(5000),
						},
						NetAssets: [6]decimal.Decimal{
							decimal.NewFromInt(50000), decimal.NewFromInt(45000), decimal.NewFromInt(40000),
							decimal.NewFromInt(35000), decimal.NewFromInt(30000), decimal.NewFromInt(25000),
						},
					},
				},
				PnL: model.PnlReport{
					Years: [5]model.PnlYear{
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
					},
				},
				FiPlan: model.FiplanReport{
					Plan: model.FiplanPlan{
						Balance: model.FiplanBalance{
							CumulativeCash: [5]decimal.Decimal{
								decimal.NewFromInt(10000), decimal.NewFromInt(15000), decimal.NewFromInt(20000),
								decimal.NewFromInt(25000), decimal.NewFromInt(30000),
							},
						},
					},
					Warning: [5]bool{false, false, false, false, false},
				},
			},
			expectedWarningCount: 1, // Negative cash in BSheet year 1
		},
		{
			name: "negative cumulative cash in FiPlan",
			output: model.FullPlanOutput{
				BSheet: model.BSheetReport{
					Detailed: model.BSheetDetailed{
						Assets: model.BSheetDetailedAssets{
							TotalAssets: [6]decimal.Decimal{
								decimal.NewFromInt(100000), decimal.NewFromInt(110000), decimal.NewFromInt(120000),
								decimal.NewFromInt(130000), decimal.NewFromInt(140000), decimal.NewFromInt(150000),
							},
							Cash: [6]decimal.Decimal{
								decimal.NewFromInt(10000), decimal.NewFromInt(15000), decimal.NewFromInt(20000),
								decimal.NewFromInt(25000), decimal.NewFromInt(30000), decimal.NewFromInt(35000),
							},
						},
						Liabilities: model.BSheetDetailedLiabilities{
							TotalLiabilities: [6]decimal.Decimal{
								decimal.NewFromInt(100000), decimal.NewFromInt(110000), decimal.NewFromInt(120000),
								decimal.NewFromInt(130000), decimal.NewFromInt(140000), decimal.NewFromInt(150000),
							},
						},
					},
					Equity: [6]decimal.Decimal{
						decimal.NewFromInt(50000), decimal.NewFromInt(55000), decimal.NewFromInt(60000),
						decimal.NewFromInt(65000), decimal.NewFromInt(70000), decimal.NewFromInt(75000),
					},
				},
				Revenue: model.ConsolidatedRevenue{
					Totals: [5]model.ConsolidatedRevenueYear{
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
					},
				},
				WCR: model.WCRReport{
					Summary: model.WCRSummary{
						BasicWCR: [5]decimal.Decimal{
							decimal.NewFromInt(10000), decimal.NewFromInt(10000), decimal.NewFromInt(10000),
							decimal.NewFromInt(10000), decimal.NewFromInt(10000),
						},
					},
				},
				Capex: model.CapexSummary{
					Totals: model.CapexTotals{
						TotalDepreciation: [5]decimal.Decimal{
							decimal.NewFromInt(5000), decimal.NewFromInt(5000), decimal.NewFromInt(5000),
							decimal.NewFromInt(5000), decimal.NewFromInt(5000),
						},
						NetAssets: [6]decimal.Decimal{
							decimal.NewFromInt(50000), decimal.NewFromInt(45000), decimal.NewFromInt(40000),
							decimal.NewFromInt(35000), decimal.NewFromInt(30000), decimal.NewFromInt(25000),
						},
					},
				},
				PnL: model.PnlReport{
					Years: [5]model.PnlYear{
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
					},
				},
				FiPlan: model.FiplanReport{
					Plan: model.FiplanPlan{
						Balance: model.FiplanBalance{
							CumulativeCash: [5]decimal.Decimal{
								decimal.NewFromInt(10000), decimal.NewFromInt(-5000), decimal.NewFromInt(20000),
								decimal.NewFromInt(25000), decimal.NewFromInt(30000),
							},
						},
					},
					Warning: [5]bool{false, false, false, false, false},
				},
			},
			expectedWarningCount: 1, // Negative cumulative cash in year 2
		},
		{
			name: "negative WCR",
			output: model.FullPlanOutput{
				BSheet: model.BSheetReport{
					Detailed: model.BSheetDetailed{
						Assets: model.BSheetDetailedAssets{
							TotalAssets: [6]decimal.Decimal{
								decimal.NewFromInt(100000), decimal.NewFromInt(110000), decimal.NewFromInt(120000),
								decimal.NewFromInt(130000), decimal.NewFromInt(140000), decimal.NewFromInt(150000),
							},
							Cash: [6]decimal.Decimal{
								decimal.NewFromInt(10000), decimal.NewFromInt(15000), decimal.NewFromInt(20000),
								decimal.NewFromInt(25000), decimal.NewFromInt(30000), decimal.NewFromInt(35000),
							},
						},
						Liabilities: model.BSheetDetailedLiabilities{
							TotalLiabilities: [6]decimal.Decimal{
								decimal.NewFromInt(100000), decimal.NewFromInt(110000), decimal.NewFromInt(120000),
								decimal.NewFromInt(130000), decimal.NewFromInt(140000), decimal.NewFromInt(150000),
							},
						},
					},
					Equity: [6]decimal.Decimal{
						decimal.NewFromInt(50000), decimal.NewFromInt(55000), decimal.NewFromInt(60000),
						decimal.NewFromInt(65000), decimal.NewFromInt(70000), decimal.NewFromInt(75000),
					},
				},
				Revenue: model.ConsolidatedRevenue{
					Totals: [5]model.ConsolidatedRevenueYear{
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
					},
				},
				WCR: model.WCRReport{
					Summary: model.WCRSummary{
						BasicWCR: [5]decimal.Decimal{
							decimal.NewFromInt(-5000), decimal.NewFromInt(-5000), decimal.NewFromInt(-5000),
							decimal.NewFromInt(-5000), decimal.NewFromInt(-5000),
						},
					},
				},
				Capex: model.CapexSummary{
					Totals: model.CapexTotals{
						TotalDepreciation: [5]decimal.Decimal{
							decimal.NewFromInt(5000), decimal.NewFromInt(5000), decimal.NewFromInt(5000),
							decimal.NewFromInt(5000), decimal.NewFromInt(5000),
						},
						NetAssets: [6]decimal.Decimal{
							decimal.NewFromInt(50000), decimal.NewFromInt(45000), decimal.NewFromInt(40000),
							decimal.NewFromInt(35000), decimal.NewFromInt(30000), decimal.NewFromInt(25000),
						},
					},
				},
				PnL: model.PnlReport{
					Years: [5]model.PnlYear{
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
					},
				},
				FiPlan: model.FiplanReport{
					Plan: model.FiplanPlan{
						Balance: model.FiplanBalance{
							CumulativeCash: [5]decimal.Decimal{
								decimal.NewFromInt(10000), decimal.NewFromInt(15000), decimal.NewFromInt(20000),
								decimal.NewFromInt(25000), decimal.NewFromInt(30000),
							},
						},
					},
					Warning: [5]bool{false, false, false, false, false},
				},
			},
			expectedWarningCount: 5, // Negative WCR in all 5 years (info level)
		},
		{
			name: "negative revenue",
			output: model.FullPlanOutput{
				BSheet: model.BSheetReport{
					Detailed: model.BSheetDetailed{
						Assets: model.BSheetDetailedAssets{
							TotalAssets: [6]decimal.Decimal{
								decimal.NewFromInt(100000), decimal.NewFromInt(110000), decimal.NewFromInt(120000),
								decimal.NewFromInt(130000), decimal.NewFromInt(140000), decimal.NewFromInt(150000),
							},
							Cash: [6]decimal.Decimal{
								decimal.NewFromInt(10000), decimal.NewFromInt(15000), decimal.NewFromInt(20000),
								decimal.NewFromInt(25000), decimal.NewFromInt(30000), decimal.NewFromInt(35000),
							},
						},
						Liabilities: model.BSheetDetailedLiabilities{
							TotalLiabilities: [6]decimal.Decimal{
								decimal.NewFromInt(100000), decimal.NewFromInt(110000), decimal.NewFromInt(120000),
								decimal.NewFromInt(130000), decimal.NewFromInt(140000), decimal.NewFromInt(150000),
							},
						},
					},
					Equity: [6]decimal.Decimal{
						decimal.NewFromInt(50000), decimal.NewFromInt(55000), decimal.NewFromInt(60000),
						decimal.NewFromInt(65000), decimal.NewFromInt(70000), decimal.NewFromInt(75000),
					},
				},
				Revenue: model.ConsolidatedRevenue{
					Totals: [5]model.ConsolidatedRevenueYear{
						{TotalTurnover: decimal.NewFromInt(-10000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
					},
				},
				WCR: model.WCRReport{
					Summary: model.WCRSummary{
						BasicWCR: [5]decimal.Decimal{
							decimal.NewFromInt(10000), decimal.NewFromInt(10000), decimal.NewFromInt(10000),
							decimal.NewFromInt(10000), decimal.NewFromInt(10000),
						},
					},
				},
				Capex: model.CapexSummary{
					Totals: model.CapexTotals{
						TotalDepreciation: [5]decimal.Decimal{
							decimal.NewFromInt(5000), decimal.NewFromInt(5000), decimal.NewFromInt(5000),
							decimal.NewFromInt(5000), decimal.NewFromInt(5000),
						},
						NetAssets: [6]decimal.Decimal{
							decimal.NewFromInt(50000), decimal.NewFromInt(45000), decimal.NewFromInt(40000),
							decimal.NewFromInt(35000), decimal.NewFromInt(30000), decimal.NewFromInt(25000),
						},
					},
				},
				PnL: model.PnlReport{
					Years: [5]model.PnlYear{
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
					},
				},
				FiPlan: model.FiplanReport{
					Plan: model.FiplanPlan{
						Balance: model.FiplanBalance{
							CumulativeCash: [5]decimal.Decimal{
								decimal.NewFromInt(10000), decimal.NewFromInt(15000), decimal.NewFromInt(20000),
								decimal.NewFromInt(25000), decimal.NewFromInt(30000),
							},
						},
					},
					Warning: [5]bool{false, false, false, false, false},
				},
			},
			expectedWarningCount: 1, // Negative revenue (error severity)
		},
		{
			name: "unusual gross margin",
			output: model.FullPlanOutput{
				BSheet: model.BSheetReport{
					Detailed: model.BSheetDetailed{
						Assets: model.BSheetDetailedAssets{
							TotalAssets: [6]decimal.Decimal{
								decimal.NewFromInt(100000), decimal.NewFromInt(110000), decimal.NewFromInt(120000),
								decimal.NewFromInt(130000), decimal.NewFromInt(140000), decimal.NewFromInt(150000),
							},
							Cash: [6]decimal.Decimal{
								decimal.NewFromInt(10000), decimal.NewFromInt(15000), decimal.NewFromInt(20000),
								decimal.NewFromInt(25000), decimal.NewFromInt(30000), decimal.NewFromInt(35000),
							},
						},
						Liabilities: model.BSheetDetailedLiabilities{
							TotalLiabilities: [6]decimal.Decimal{
								decimal.NewFromInt(100000), decimal.NewFromInt(110000), decimal.NewFromInt(120000),
								decimal.NewFromInt(130000), decimal.NewFromInt(140000), decimal.NewFromInt(150000),
							},
						},
					},
					Equity: [6]decimal.Decimal{
						decimal.NewFromInt(50000), decimal.NewFromInt(55000), decimal.NewFromInt(60000),
						decimal.NewFromInt(65000), decimal.NewFromInt(70000), decimal.NewFromInt(75000),
					},
				},
				Revenue: model.ConsolidatedRevenue{
					Totals: [5]model.ConsolidatedRevenueYear{
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("1.5")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
					},
				},
				WCR: model.WCRReport{
					Summary: model.WCRSummary{
						BasicWCR: [5]decimal.Decimal{
							decimal.NewFromInt(10000), decimal.NewFromInt(10000), decimal.NewFromInt(10000),
							decimal.NewFromInt(10000), decimal.NewFromInt(10000),
						},
					},
				},
				Capex: model.CapexSummary{
					Totals: model.CapexTotals{
						TotalDepreciation: [5]decimal.Decimal{
							decimal.NewFromInt(5000), decimal.NewFromInt(5000), decimal.NewFromInt(5000),
							decimal.NewFromInt(5000), decimal.NewFromInt(5000),
						},
						NetAssets: [6]decimal.Decimal{
							decimal.NewFromInt(50000), decimal.NewFromInt(45000), decimal.NewFromInt(40000),
							decimal.NewFromInt(35000), decimal.NewFromInt(30000), decimal.NewFromInt(25000),
						},
					},
				},
				PnL: model.PnlReport{
					Years: [5]model.PnlYear{
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
					},
				},
				FiPlan: model.FiplanReport{
					Plan: model.FiplanPlan{
						Balance: model.FiplanBalance{
							CumulativeCash: [5]decimal.Decimal{
								decimal.NewFromInt(10000), decimal.NewFromInt(15000), decimal.NewFromInt(20000),
								decimal.NewFromInt(25000), decimal.NewFromInt(30000),
							},
						},
					},
					Warning: [5]bool{false, false, false, false, false},
				},
			},
			expectedWarningCount: 1, // Unusual gross margin in year 1
		},
		{
			name: "depreciation exceeds net assets",
			output: model.FullPlanOutput{
				BSheet: model.BSheetReport{
					Detailed: model.BSheetDetailed{
						Assets: model.BSheetDetailedAssets{
							TotalAssets: [6]decimal.Decimal{
								decimal.NewFromInt(100000), decimal.NewFromInt(110000), decimal.NewFromInt(120000),
								decimal.NewFromInt(130000), decimal.NewFromInt(140000), decimal.NewFromInt(150000),
							},
							Cash: [6]decimal.Decimal{
								decimal.NewFromInt(10000), decimal.NewFromInt(15000), decimal.NewFromInt(20000),
								decimal.NewFromInt(25000), decimal.NewFromInt(30000), decimal.NewFromInt(35000),
							},
						},
						Liabilities: model.BSheetDetailedLiabilities{
							TotalLiabilities: [6]decimal.Decimal{
								decimal.NewFromInt(100000), decimal.NewFromInt(110000), decimal.NewFromInt(120000),
								decimal.NewFromInt(130000), decimal.NewFromInt(140000), decimal.NewFromInt(150000),
							},
						},
					},
					Equity: [6]decimal.Decimal{
						decimal.NewFromInt(50000), decimal.NewFromInt(55000), decimal.NewFromInt(60000),
						decimal.NewFromInt(65000), decimal.NewFromInt(70000), decimal.NewFromInt(75000),
					},
				},
				Revenue: model.ConsolidatedRevenue{
					Totals: [5]model.ConsolidatedRevenueYear{
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
					},
				},
				WCR: model.WCRReport{
					Summary: model.WCRSummary{
						BasicWCR: [5]decimal.Decimal{
							decimal.NewFromInt(10000), decimal.NewFromInt(10000), decimal.NewFromInt(10000),
							decimal.NewFromInt(10000), decimal.NewFromInt(10000),
						},
					},
				},
				Capex: model.CapexSummary{
					Totals: model.CapexTotals{
						TotalDepreciation: [5]decimal.Decimal{
							decimal.NewFromInt(60000), decimal.NewFromInt(5000), decimal.NewFromInt(5000),
							decimal.NewFromInt(5000), decimal.NewFromInt(5000),
						},
						NetAssets: [6]decimal.Decimal{
							decimal.NewFromInt(50000), decimal.NewFromInt(45000), decimal.NewFromInt(40000),
							decimal.NewFromInt(35000), decimal.NewFromInt(30000), decimal.NewFromInt(25000),
						},
					},
				},
				PnL: model.PnlReport{
					Years: [5]model.PnlYear{
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
					},
				},
				FiPlan: model.FiplanReport{
					Plan: model.FiplanPlan{
						Balance: model.FiplanBalance{
							CumulativeCash: [5]decimal.Decimal{
								decimal.NewFromInt(10000), decimal.NewFromInt(15000), decimal.NewFromInt(20000),
								decimal.NewFromInt(25000), decimal.NewFromInt(30000),
							},
						},
					},
					Warning: [5]bool{false, false, false, false, false},
				},
			},
			expectedWarningCount: 1, // Depreciation exceeds net assets in year 1 (info level)
		},
		{
			name: "negative EBITDA",
			output: model.FullPlanOutput{
				BSheet: model.BSheetReport{
					Detailed: model.BSheetDetailed{
						Assets: model.BSheetDetailedAssets{
							TotalAssets: [6]decimal.Decimal{
								decimal.NewFromInt(100000), decimal.NewFromInt(110000), decimal.NewFromInt(120000),
								decimal.NewFromInt(130000), decimal.NewFromInt(140000), decimal.NewFromInt(150000),
							},
							Cash: [6]decimal.Decimal{
								decimal.NewFromInt(10000), decimal.NewFromInt(15000), decimal.NewFromInt(20000),
								decimal.NewFromInt(25000), decimal.NewFromInt(30000), decimal.NewFromInt(35000),
							},
						},
						Liabilities: model.BSheetDetailedLiabilities{
							TotalLiabilities: [6]decimal.Decimal{
								decimal.NewFromInt(100000), decimal.NewFromInt(110000), decimal.NewFromInt(120000),
								decimal.NewFromInt(130000), decimal.NewFromInt(140000), decimal.NewFromInt(150000),
							},
						},
					},
					Equity: [6]decimal.Decimal{
						decimal.NewFromInt(50000), decimal.NewFromInt(55000), decimal.NewFromInt(60000),
						decimal.NewFromInt(65000), decimal.NewFromInt(70000), decimal.NewFromInt(75000),
					},
				},
				Revenue: model.ConsolidatedRevenue{
					Totals: [5]model.ConsolidatedRevenueYear{
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
					},
				},
				WCR: model.WCRReport{
					Summary: model.WCRSummary{
						BasicWCR: [5]decimal.Decimal{
							decimal.NewFromInt(10000), decimal.NewFromInt(10000), decimal.NewFromInt(10000),
							decimal.NewFromInt(10000), decimal.NewFromInt(10000),
						},
					},
				},
				Capex: model.CapexSummary{
					Totals: model.CapexTotals{
						TotalDepreciation: [5]decimal.Decimal{
							decimal.NewFromInt(5000), decimal.NewFromInt(5000), decimal.NewFromInt(5000),
							decimal.NewFromInt(5000), decimal.NewFromInt(5000),
						},
						NetAssets: [6]decimal.Decimal{
							decimal.NewFromInt(50000), decimal.NewFromInt(45000), decimal.NewFromInt(40000),
							decimal.NewFromInt(35000), decimal.NewFromInt(30000), decimal.NewFromInt(25000),
						},
					},
				},
				PnL: model.PnlReport{
					Years: [5]model.PnlYear{
						{EBITDA: decimal.NewFromInt(-10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
					},
				},
				FiPlan: model.FiplanReport{
					Plan: model.FiplanPlan{
						Balance: model.FiplanBalance{
							CumulativeCash: [5]decimal.Decimal{
								decimal.NewFromInt(10000), decimal.NewFromInt(15000), decimal.NewFromInt(20000),
								decimal.NewFromInt(25000), decimal.NewFromInt(30000),
							},
						},
					},
					Warning: [5]bool{false, false, false, false, false},
				},
			},
			expectedWarningCount: 1, // Negative EBITDA in year 1
		},
		{
			name: "negative equity",
			output: model.FullPlanOutput{
				BSheet: model.BSheetReport{
					Detailed: model.BSheetDetailed{
						Assets: model.BSheetDetailedAssets{
							TotalAssets: [6]decimal.Decimal{
								decimal.NewFromInt(100000), decimal.NewFromInt(110000), decimal.NewFromInt(120000),
								decimal.NewFromInt(130000), decimal.NewFromInt(140000), decimal.NewFromInt(150000),
							},
							Cash: [6]decimal.Decimal{
								decimal.NewFromInt(10000), decimal.NewFromInt(15000), decimal.NewFromInt(20000),
								decimal.NewFromInt(25000), decimal.NewFromInt(30000), decimal.NewFromInt(35000),
							},
						},
						Liabilities: model.BSheetDetailedLiabilities{
							TotalLiabilities: [6]decimal.Decimal{
								decimal.NewFromInt(100000), decimal.NewFromInt(110000), decimal.NewFromInt(120000),
								decimal.NewFromInt(130000), decimal.NewFromInt(140000), decimal.NewFromInt(150000),
							},
						},
					},
					Equity: [6]decimal.Decimal{
						decimal.NewFromInt(-10000), decimal.NewFromInt(55000), decimal.NewFromInt(60000),
						decimal.NewFromInt(65000), decimal.NewFromInt(70000), decimal.NewFromInt(75000),
					},
				},
				Revenue: model.ConsolidatedRevenue{
					Totals: [5]model.ConsolidatedRevenueYear{
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
					},
				},
				WCR: model.WCRReport{
					Summary: model.WCRSummary{
						BasicWCR: [5]decimal.Decimal{
							decimal.NewFromInt(10000), decimal.NewFromInt(10000), decimal.NewFromInt(10000),
							decimal.NewFromInt(10000), decimal.NewFromInt(10000),
						},
					},
				},
				Capex: model.CapexSummary{
					Totals: model.CapexTotals{
						TotalDepreciation: [5]decimal.Decimal{
							decimal.NewFromInt(5000), decimal.NewFromInt(5000), decimal.NewFromInt(5000),
							decimal.NewFromInt(5000), decimal.NewFromInt(5000),
						},
						NetAssets: [6]decimal.Decimal{
							decimal.NewFromInt(50000), decimal.NewFromInt(45000), decimal.NewFromInt(40000),
							decimal.NewFromInt(35000), decimal.NewFromInt(30000), decimal.NewFromInt(25000),
						},
					},
				},
				PnL: model.PnlReport{
					Years: [5]model.PnlYear{
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
					},
				},
				FiPlan: model.FiplanReport{
					Plan: model.FiplanPlan{
						Balance: model.FiplanBalance{
							CumulativeCash: [5]decimal.Decimal{
								decimal.NewFromInt(10000), decimal.NewFromInt(15000), decimal.NewFromInt(20000),
								decimal.NewFromInt(25000), decimal.NewFromInt(30000),
							},
						},
					},
					Warning: [5]bool{false, false, false, false, false},
				},
			},
			expectedWarningCount: 1, // Negative equity in year 0
		},
		{
			name: "fiplan warning flag",
			output: model.FullPlanOutput{
				BSheet: model.BSheetReport{
					Detailed: model.BSheetDetailed{
						Assets: model.BSheetDetailedAssets{
							TotalAssets: [6]decimal.Decimal{
								decimal.NewFromInt(100000), decimal.NewFromInt(110000), decimal.NewFromInt(120000),
								decimal.NewFromInt(130000), decimal.NewFromInt(140000), decimal.NewFromInt(150000),
							},
							Cash: [6]decimal.Decimal{
								decimal.NewFromInt(10000), decimal.NewFromInt(15000), decimal.NewFromInt(20000),
								decimal.NewFromInt(25000), decimal.NewFromInt(30000), decimal.NewFromInt(35000),
							},
						},
						Liabilities: model.BSheetDetailedLiabilities{
							TotalLiabilities: [6]decimal.Decimal{
								decimal.NewFromInt(100000), decimal.NewFromInt(110000), decimal.NewFromInt(120000),
								decimal.NewFromInt(130000), decimal.NewFromInt(140000), decimal.NewFromInt(150000),
							},
						},
					},
					Equity: [6]decimal.Decimal{
						decimal.NewFromInt(50000), decimal.NewFromInt(55000), decimal.NewFromInt(60000),
						decimal.NewFromInt(65000), decimal.NewFromInt(70000), decimal.NewFromInt(75000),
					},
				},
				Revenue: model.ConsolidatedRevenue{
					Totals: [5]model.ConsolidatedRevenueYear{
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
						{TotalTurnover: decimal.NewFromInt(100000), GrossMarginPct: decimal.RequireFromString("0.4")},
					},
				},
				WCR: model.WCRReport{
					Summary: model.WCRSummary{
						BasicWCR: [5]decimal.Decimal{
							decimal.NewFromInt(10000), decimal.NewFromInt(10000), decimal.NewFromInt(10000),
							decimal.NewFromInt(10000), decimal.NewFromInt(10000),
						},
					},
				},
				Capex: model.CapexSummary{
					Totals: model.CapexTotals{
						TotalDepreciation: [5]decimal.Decimal{
							decimal.NewFromInt(5000), decimal.NewFromInt(5000), decimal.NewFromInt(5000),
							decimal.NewFromInt(5000), decimal.NewFromInt(5000),
						},
						NetAssets: [6]decimal.Decimal{
							decimal.NewFromInt(50000), decimal.NewFromInt(45000), decimal.NewFromInt(40000),
							decimal.NewFromInt(35000), decimal.NewFromInt(30000), decimal.NewFromInt(25000),
						},
					},
				},
				PnL: model.PnlReport{
					Years: [5]model.PnlYear{
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
						{EBITDA: decimal.NewFromInt(10000)},
					},
				},
				FiPlan: model.FiplanReport{
					Plan: model.FiplanPlan{
						Balance: model.FiplanBalance{
							CumulativeCash: [5]decimal.Decimal{
								decimal.NewFromInt(10000), decimal.NewFromInt(15000), decimal.NewFromInt(20000),
								decimal.NewFromInt(25000), decimal.NewFromInt(30000),
							},
						},
					},
					Warning: [5]bool{false, true, false, false, false},
				},
			},
			expectedWarningCount: 1, // FiPlan warning in year 2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := ValidateOutput(tt.output)

			assert.Equal(t, tt.expectedWarningCount, len(warnings),
				"Expected %d warnings but got %d", tt.expectedWarningCount, len(warnings))

			if len(warnings) > 0 && tt.expectedMinSeverity != "" {
				for _, w := range warnings {
					assert.Equal(t, tt.expectedMinSeverity, w.Severity)
				}
			}
		})
	}
}
