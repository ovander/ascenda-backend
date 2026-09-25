package compute

import (
	"testing"
	"time"

	"ascenda/internal/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestComputePnlCash(t *testing.T) {
	config := model.PlanConfig{
		DiscountRate:     decimal.NewFromFloat(0.1),
		CorporateTaxRate: decimal.NewFromFloat(0.25),
		EmployerTaxRate:  decimal.NewFromFloat(0.42),
		ForecastStart:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		PreviousStaff:    0,
	}

	tests := []struct {
		name     string
		entries  []model.PnlCashEntry
		revenue  model.ConsolidatedRevenue
		opex     model.OpexSummary
		staff    model.StaffPayrollSummary
		capex    model.CapexSummary
		pnl      model.PnlReport
		checkPnl func(*testing.T, model.PnlCashReport)
	}{
		{
			name:    "basic simplified PnL with complete data",
			entries: []model.PnlCashEntry{},
			revenue: model.ConsolidatedRevenue{
				Totals: [5]model.ConsolidatedRevenueYear{
					{
						Year:             2025,
						TotalTurnover:    decimal.NewFromInt(100000),
						TotalCOGS:        decimal.NewFromInt(60000),
						TotalGrossMargin: decimal.NewFromInt(40000),
					},
					{
						Year:             2026,
						TotalTurnover:    decimal.NewFromInt(120000),
						TotalCOGS:        decimal.NewFromInt(72000),
						TotalGrossMargin: decimal.NewFromInt(48000),
					},
					{
						Year:             2027,
						TotalTurnover:    decimal.NewFromInt(144000),
						TotalCOGS:        decimal.NewFromInt(86400),
						TotalGrossMargin: decimal.NewFromInt(57600),
					},
					{
						Year:             2028,
						TotalTurnover:    decimal.NewFromInt(172800),
						TotalCOGS:        decimal.NewFromInt(103680),
						TotalGrossMargin: decimal.NewFromInt(69120),
					},
					{
						Year:             2029,
						TotalTurnover:    decimal.NewFromInt(207360),
						TotalCOGS:        decimal.NewFromInt(124416),
						TotalGrossMargin: decimal.NewFromInt(82944),
					},
				},
			},
			opex: model.OpexSummary{
				Subcategories: []model.OpexSubcategoryResult{
					{
						Subcategory: model.OpexSubPremises,
						Lines: []model.OpexLineResult{
							{
								LineID:      model.LineExternalStaffRnD,
								IsUserInput: true,
								CostDriver:  "manual",
								Years: [5]decimal.Decimal{
									decimal.NewFromInt(5000),
									decimal.NewFromInt(6000),
									decimal.NewFromInt(7200),
									decimal.NewFromInt(8640),
									decimal.NewFromInt(10368),
								},
							},
						},
					},
					{
						Subcategory: model.OpexSubMarketing,
						Lines: []model.OpexLineResult{
							{
								LineID:      model.LineAdvertisingComms,
								IsUserInput: true,
								CostDriver:  "manual",
								Years: [5]decimal.Decimal{
									decimal.NewFromInt(3000),
									decimal.NewFromInt(3600),
									decimal.NewFromInt(4320),
									decimal.NewFromInt(5184),
									decimal.NewFromInt(6221),
								},
							},
						},
					},
					{
						Subcategory: model.OpexSubHR,
						Lines: []model.OpexLineResult{
							{
								LineID:      model.LineRecruitmentTraining,
								IsUserInput: false,
								CostDriver:  "pct_of_payroll",
								Years: [5]decimal.Decimal{
									decimal.NewFromInt(2000),
									decimal.NewFromInt(2400),
									decimal.NewFromInt(2880),
									decimal.NewFromInt(3456),
									decimal.NewFromInt(4147),
								},
							},
						},
					},
				},
				GrandTotal: [5]decimal.Decimal{
					decimal.NewFromInt(10000),
					decimal.NewFromInt(12000),
					decimal.NewFromInt(14400),
					decimal.NewFromInt(17280),
					decimal.NewFromInt(20736),
				},
			},
			staff: model.StaffPayrollSummary{
				Payroll: [5]model.StaffPayrollYear{
					{
						Year:            2025,
						YearIndex:       0,
						SubtotalPayroll: decimal.NewFromInt(30000),
						TotalPayroll:    decimal.NewFromInt(30000),
					},
					{
						Year:            2026,
						YearIndex:       1,
						SubtotalPayroll: decimal.NewFromInt(36000),
						TotalPayroll:    decimal.NewFromInt(36000),
					},
					{
						Year:            2027,
						YearIndex:       2,
						SubtotalPayroll: decimal.NewFromInt(43200),
						TotalPayroll:    decimal.NewFromInt(43200),
					},
					{
						Year:            2028,
						YearIndex:       3,
						SubtotalPayroll: decimal.NewFromInt(51840),
						TotalPayroll:    decimal.NewFromInt(51840),
					},
					{
						Year:            2029,
						YearIndex:       4,
						SubtotalPayroll: decimal.NewFromInt(62208),
						TotalPayroll:    decimal.NewFromInt(62208),
					},
				},
				FunctionalBreakdown: [5]model.StaffFunctionalYear{
					{
						Year:      2025,
						YearIndex: 0,
						RnD:       decimal.NewFromInt(5000),
						Sales:     decimal.NewFromInt(10000),
						GnA:       decimal.NewFromInt(15000),
					},
					{
						Year:      2026,
						YearIndex: 1,
						RnD:       decimal.NewFromInt(6000),
						Sales:     decimal.NewFromInt(12000),
						GnA:       decimal.NewFromInt(18000),
					},
					{
						Year:      2027,
						YearIndex: 2,
						RnD:       decimal.NewFromInt(7200),
						Sales:     decimal.NewFromInt(14400),
						GnA:       decimal.NewFromInt(21600),
					},
					{
						Year:      2028,
						YearIndex: 3,
						RnD:       decimal.NewFromInt(8640),
						Sales:     decimal.NewFromInt(17280),
						GnA:       decimal.NewFromInt(25920),
					},
					{
						Year:      2029,
						YearIndex: 4,
						RnD:       decimal.NewFromInt(10368),
						Sales:     decimal.NewFromInt(20736),
						GnA:       decimal.NewFromInt(31104),
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
			pnl: model.PnlReport{
				Years: [5]model.PnlYear{
					{
						Year:               2025,
						YearIndex:          0,
						FinancialExpenses:  decimal.Zero,
						GrantsOtherRevenue: decimal.Zero,
						CorporateTax:       decimal.Zero,
					},
					{
						Year:               2026,
						YearIndex:          1,
						FinancialExpenses:  decimal.Zero,
						GrantsOtherRevenue: decimal.Zero,
						CorporateTax:       decimal.Zero,
					},
					{
						Year:               2027,
						YearIndex:          2,
						FinancialExpenses:  decimal.Zero,
						GrantsOtherRevenue: decimal.Zero,
						CorporateTax:       decimal.Zero,
					},
					{
						Year:               2028,
						YearIndex:          3,
						FinancialExpenses:  decimal.Zero,
						GrantsOtherRevenue: decimal.Zero,
						CorporateTax:       decimal.Zero,
					},
					{
						Year:               2029,
						YearIndex:          4,
						FinancialExpenses:  decimal.Zero,
						GrantsOtherRevenue: decimal.Zero,
						CorporateTax:       decimal.Zero,
					},
				},
			},
			checkPnl: func(t *testing.T, result model.PnlCashReport) {
				// Year 1 checks
				assertDecEq(t, decimal.NewFromInt(100000), result.Years[0].Sales,
					"Sales should match input")
				assertDecEq(t, decimal.NewFromInt(40000), result.Years[0].GrossMargin,
					"Gross margin should match input")

				// Check functional breakdown from opex and staff
				assertDecEq(t, decimal.NewFromInt(5000), result.Years[0].RDPayroll,
					"R&D payroll should match functional breakdown")
				assertDecEq(t, decimal.NewFromInt(10000), result.Years[0].SalesPayroll,
					"Sales payroll should match functional breakdown")
				assertDecEq(t, decimal.NewFromInt(15000), result.Years[0].GAPayroll,
					"G&A payroll should match functional breakdown")

				// EBIT = GrossMargin - TotalExpenses
				// TotalExpenses = RDPayroll(5000) + OutsourcedRD(5000) + RoyaltiesMisc(0)
				//              + SalesPayroll(10000) + AdvertisingPromo(3000) + MiscSalesCosts(0)
				//              + GAPayroll(15000) + InsuranceRent(0) + LeasedEquip(0)
				//              + LegalConsulting(0) + TravelMisc(2000) + Depreciation(0) = 40000
				// EBIT = 40000 - 40000 = 0
				expectedEBIT := decimal.NewFromInt(40000).
					Sub(decimal.NewFromInt(5000)).  // RDPayroll
					Sub(decimal.NewFromInt(5000)).  // OutsourcedRD
					Sub(decimal.NewFromInt(10000)). // SalesPayroll
					Sub(decimal.NewFromInt(3000)).  // AdvertisingPromo
					Sub(decimal.NewFromInt(15000)). // GAPayroll
					Sub(decimal.NewFromInt(2000))   // TravelMisc (recruitment_training)
				assertDecEq(t, expectedEBIT, result.Years[0].EBIT,
					"EBIT calculation mismatch")

				// Net Profit = EBIT (no financing or taxes in this scenario)
				assertDecEq(t, result.Years[0].EBIT, result.Years[0].NetProfit,
					"Net profit should equal EBIT in cash view")

				// Year 2 checks
				assertDecEq(t, decimal.NewFromInt(120000), result.Years[1].Sales,
					"Year 2 sales should match input")
				assertDecEq(t, decimal.NewFromInt(48000), result.Years[1].GrossMargin,
					"Year 2 gross margin should match input")

				// EBIT = GrossMargin - TotalExpenses
				// TotalExpenses = RDPayroll(6000) + OutsourcedRD(6000) + RoyaltiesMisc(0)
				//              + SalesPayroll(12000) + AdvertisingPromo(3600) + MiscSalesCosts(0)
				//              + GAPayroll(18000) + InsuranceRent(0) + LeasedEquip(0)
				//              + LegalConsulting(0) + TravelMisc(2400) + Depreciation(0) = 48000
				// EBIT = 48000 - 48000 = 0
				expectedEBIT2 := decimal.NewFromInt(48000).
					Sub(decimal.NewFromInt(6000)).  // RDPayroll
					Sub(decimal.NewFromInt(6000)).  // OutsourcedRD
					Sub(decimal.NewFromInt(12000)). // SalesPayroll
					Sub(decimal.NewFromInt(3600)).  // AdvertisingPromo
					Sub(decimal.NewFromInt(18000)). // GAPayroll
					Sub(decimal.NewFromInt(2400))   // TravelMisc (recruitment_training)
				assertDecEq(t, expectedEBIT2, result.Years[1].EBIT,
					"Year 2 EBIT calculation mismatch")
			},
		},
		{
			name:    "zero inputs",
			entries: []model.PnlCashEntry{},
			revenue: model.ConsolidatedRevenue{
				Totals: [5]model.ConsolidatedRevenueYear{
					{Year: 2025},
					{Year: 2026},
					{Year: 2027},
					{Year: 2028},
					{Year: 2029},
				},
			},
			opex: model.OpexSummary{
				Subcategories: []model.OpexSubcategoryResult{},
				GrandTotal: [5]decimal.Decimal{
					decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
				},
			},
			staff: model.StaffPayrollSummary{
				Payroll: [5]model.StaffPayrollYear{
					{Year: 2025, YearIndex: 0, TotalPayroll: decimal.Zero},
					{Year: 2026, YearIndex: 1, TotalPayroll: decimal.Zero},
					{Year: 2027, YearIndex: 2, TotalPayroll: decimal.Zero},
					{Year: 2028, YearIndex: 3, TotalPayroll: decimal.Zero},
					{Year: 2029, YearIndex: 4, TotalPayroll: decimal.Zero},
				},
				FunctionalBreakdown: [5]model.StaffFunctionalYear{
					{Year: 2025, YearIndex: 0},
					{Year: 2026, YearIndex: 1},
					{Year: 2027, YearIndex: 2},
					{Year: 2028, YearIndex: 3},
					{Year: 2029, YearIndex: 4},
				},
			},
			capex: model.CapexSummary{
				Totals: model.CapexTotals{
					TotalDepreciation: [5]decimal.Decimal{
						decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
					},
				},
			},
			pnl: model.PnlReport{
				Years: [5]model.PnlYear{
					{Year: 2025, YearIndex: 0},
					{Year: 2026, YearIndex: 1},
					{Year: 2027, YearIndex: 2},
					{Year: 2028, YearIndex: 3},
					{Year: 2029, YearIndex: 4},
				},
			},
			checkPnl: func(t *testing.T, result model.PnlCashReport) {
				// All values should be zero
				for year := 0; year < 5; year++ {
					assertDecEq(t, decimal.Zero, result.Years[year].Sales,
						"Sales year %d should be zero", year+1)
					assertDecEq(t, decimal.Zero, result.Years[year].GrossMargin,
						"Gross margin year %d should be zero", year+1)
					assertDecEq(t, decimal.Zero, result.Years[year].RDPayroll,
						"R&D payroll year %d should be zero", year+1)
					assertDecEq(t, decimal.Zero, result.Years[year].SalesPayroll,
						"Sales payroll year %d should be zero", year+1)
					assertDecEq(t, decimal.Zero, result.Years[year].GAPayroll,
						"G&A payroll year %d should be zero", year+1)
					assertDecEq(t, decimal.Zero, result.Years[year].EBIT,
						"EBIT year %d should be zero", year+1)
					assertDecEq(t, decimal.Zero, result.Years[year].NetProfit,
						"Net profit year %d should be zero", year+1)
				}
			},
		},
		{
			name:    "with opex subcategories",
			entries: []model.PnlCashEntry{},
			revenue: model.ConsolidatedRevenue{
				Totals: [5]model.ConsolidatedRevenueYear{
					{
						Year:             2025,
						TotalTurnover:    decimal.NewFromInt(200000),
						TotalCOGS:        decimal.NewFromInt(100000),
						TotalGrossMargin: decimal.NewFromInt(100000),
					},
					{
						Year:             2026,
						TotalTurnover:    decimal.NewFromInt(240000),
						TotalCOGS:        decimal.NewFromInt(120000),
						TotalGrossMargin: decimal.NewFromInt(120000),
					},
					{Year: 2027},
					{Year: 2028},
					{Year: 2029},
				},
			},
			opex: model.OpexSummary{
				Subcategories: []model.OpexSubcategoryResult{
					{
						Subcategory: model.OpexSubProfessional,
						Lines: []model.OpexLineResult{
							{
								LineID:      model.LineExternalStaffRnD,
								IsUserInput: true,
								CostDriver:  "manual",
								Years: [5]decimal.Decimal{
									decimal.NewFromInt(15000),
									decimal.NewFromInt(18000),
									decimal.Zero, decimal.Zero, decimal.Zero,
								},
							},
						},
					},
					{
						Subcategory: model.OpexSubMarketing,
						Lines: []model.OpexLineResult{
							{
								LineID:      model.LineAdvertisingComms,
								IsUserInput: true,
								CostDriver:  "manual",
								Years: [5]decimal.Decimal{
									decimal.NewFromInt(8000),
									decimal.NewFromInt(9600),
									decimal.Zero, decimal.Zero, decimal.Zero,
								},
							},
						},
					},
					{
						Subcategory: model.OpexSubHR,
						Lines: []model.OpexLineResult{
							{
								LineID:      model.LineRecruitmentTraining,
								IsUserInput: false,
								CostDriver:  "pct_of_payroll",
								Years: [5]decimal.Decimal{
									decimal.NewFromInt(12000),
									decimal.NewFromInt(14400),
									decimal.Zero, decimal.Zero, decimal.Zero,
								},
							},
						},
					},
				},
				GrandTotal: [5]decimal.Decimal{
					decimal.NewFromInt(35000),
					decimal.NewFromInt(42000),
					decimal.Zero, decimal.Zero, decimal.Zero,
				},
			},
			staff: model.StaffPayrollSummary{
				Payroll: [5]model.StaffPayrollYear{
					{
						Year:            2025,
						YearIndex:       0,
						SubtotalPayroll: decimal.NewFromInt(40000),
						TotalPayroll:    decimal.NewFromInt(40000),
					},
					{
						Year:            2026,
						YearIndex:       1,
						SubtotalPayroll: decimal.NewFromInt(48000),
						TotalPayroll:    decimal.NewFromInt(48000),
					},
					{Year: 2027, YearIndex: 2},
					{Year: 2028, YearIndex: 3},
					{Year: 2029, YearIndex: 4},
				},
				FunctionalBreakdown: [5]model.StaffFunctionalYear{
					{
						Year:      2025,
						YearIndex: 0,
						RnD:       decimal.NewFromInt(15000),
						Sales:     decimal.NewFromInt(8000),
						GnA:       decimal.NewFromInt(17000),
					},
					{
						Year:      2026,
						YearIndex: 1,
						RnD:       decimal.NewFromInt(18000),
						Sales:     decimal.NewFromInt(9600),
						GnA:       decimal.NewFromInt(20400),
					},
					{Year: 2027, YearIndex: 2},
					{Year: 2028, YearIndex: 3},
					{Year: 2029, YearIndex: 4},
				},
			},
			capex: model.CapexSummary{
				Totals: model.CapexTotals{
					TotalDepreciation: [5]decimal.Decimal{
						decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
					},
				},
			},
			pnl: model.PnlReport{
				Years: [5]model.PnlYear{
					{Year: 2025, YearIndex: 0},
					{Year: 2026, YearIndex: 1},
					{Year: 2027, YearIndex: 2},
					{Year: 2028, YearIndex: 3},
					{Year: 2029, YearIndex: 4},
				},
			},
			checkPnl: func(t *testing.T, result model.PnlCashReport) {
				// Year 1 checks
				assertDecEq(t, decimal.NewFromInt(200000), result.Years[0].Sales,
					"Sales should match")
				assertDecEq(t, decimal.NewFromInt(100000), result.Years[0].GrossMargin,
					"Gross margin should match")

				// Check functional breakdown is captured
				assertDecEq(t, decimal.NewFromInt(15000), result.Years[0].RDPayroll,
					"R&D should include all R&D from functional breakdown")
				assertDecEq(t, decimal.NewFromInt(8000), result.Years[0].SalesPayroll,
					"Sales should include all Sales from functional breakdown")
				assertDecEq(t, decimal.NewFromInt(17000), result.Years[0].GAPayroll,
					"G&A should include all G&A from functional breakdown")

				// EBIT = GrossMargin - TotalExpenses
				// TotalExpenses = RDPayroll(15000) + OutsourcedRD(15000) + RoyaltiesMisc(0)
				//              + SalesPayroll(8000) + AdvertisingPromo(8000) + MiscSalesCosts(0)
				//              + GAPayroll(17000) + InsuranceRent(0) + LeasedEquip(0)
				//              + LegalConsulting(0) + TravelMisc(12000) + Depreciation(0) = 75000
				// EBIT = 100000 - 75000 = 25000
				expectedEBIT := decimal.NewFromInt(100000).
					Sub(decimal.NewFromInt(15000)). // RDPayroll
					Sub(decimal.NewFromInt(15000)). // OutsourcedRD
					Sub(decimal.NewFromInt(8000)).  // SalesPayroll
					Sub(decimal.NewFromInt(8000)).  // AdvertisingPromo
					Sub(decimal.NewFromInt(17000)). // GAPayroll
					Sub(decimal.NewFromInt(12000))  // TravelMisc (recruitment_training)
				assertDecEq(t, expectedEBIT, result.Years[0].EBIT,
					"EBIT should be GrossMargin - TotalExpenses")

				assertDecEq(t, result.Years[0].EBIT, result.Years[0].NetProfit,
					"Net profit should equal EBIT")

				// Year 2 should follow same pattern
				// EBIT = GrossMargin - TotalExpenses
				// TotalExpenses = RDPayroll(18000) + OutsourcedRD(18000) + RoyaltiesMisc(0)
				//              + SalesPayroll(9600) + AdvertisingPromo(9600) + MiscSalesCosts(0)
				//              + GAPayroll(20400) + InsuranceRent(0) + LeasedEquip(0)
				//              + LegalConsulting(0) + TravelMisc(14400) + Depreciation(0) = 90000
				// EBIT = 120000 - 90000 = 30000
				expectedEBIT2 := decimal.NewFromInt(120000).
					Sub(decimal.NewFromInt(18000)). // RDPayroll
					Sub(decimal.NewFromInt(18000)). // OutsourcedRD
					Sub(decimal.NewFromInt(9600)).  // SalesPayroll
					Sub(decimal.NewFromInt(9600)).  // AdvertisingPromo
					Sub(decimal.NewFromInt(20400)). // GAPayroll
					Sub(decimal.NewFromInt(14400))  // TravelMisc (recruitment_training)
				assertDecEq(t, expectedEBIT2, result.Years[1].EBIT,
					"Year 2 EBIT calculation mismatch")
			},
		},
		{
			name:    "high profitability scenario",
			entries: []model.PnlCashEntry{},
			revenue: model.ConsolidatedRevenue{
				Totals: [5]model.ConsolidatedRevenueYear{
					{
						Year:             2025,
						TotalTurnover:    decimal.NewFromInt(500000),
						TotalCOGS:        decimal.NewFromInt(200000),
						TotalGrossMargin: decimal.NewFromInt(300000),
					},
					{
						Year:             2026,
						TotalTurnover:    decimal.NewFromInt(600000),
						TotalCOGS:        decimal.NewFromInt(240000),
						TotalGrossMargin: decimal.NewFromInt(360000),
					},
					{
						Year:             2027,
						TotalTurnover:    decimal.NewFromInt(720000),
						TotalCOGS:        decimal.NewFromInt(288000),
						TotalGrossMargin: decimal.NewFromInt(432000),
					},
					{Year: 2028},
					{Year: 2029},
				},
			},
			opex: model.OpexSummary{
				Subcategories: []model.OpexSubcategoryResult{
					{
						Subcategory: model.OpexSubProfessional,
						Lines: []model.OpexLineResult{
							{
								LineID:      model.LineExternalStaffRnD,
								IsUserInput: true,
								CostDriver:  "manual",
								Years: [5]decimal.Decimal{
									decimal.NewFromInt(20000),
									decimal.NewFromInt(24000),
									decimal.NewFromInt(28800),
									decimal.Zero, decimal.Zero,
								},
							},
						},
					},
					{
						Subcategory: model.OpexSubMarketing,
						Lines: []model.OpexLineResult{
							{
								LineID:      model.LineAdvertisingComms,
								IsUserInput: true,
								CostDriver:  "manual",
								Years: [5]decimal.Decimal{
									decimal.NewFromInt(15000),
									decimal.NewFromInt(18000),
									decimal.NewFromInt(21600),
									decimal.Zero, decimal.Zero,
								},
							},
						},
					},
					{
						Subcategory: model.OpexSubHR,
						Lines: []model.OpexLineResult{
							{
								LineID:      model.LineRecruitmentTraining,
								IsUserInput: false,
								CostDriver:  "pct_of_payroll",
								Years: [5]decimal.Decimal{
									decimal.NewFromInt(25000),
									decimal.NewFromInt(30000),
									decimal.NewFromInt(36000),
									decimal.Zero, decimal.Zero,
								},
							},
						},
					},
				},
				GrandTotal: [5]decimal.Decimal{
					decimal.NewFromInt(60000),
					decimal.NewFromInt(72000),
					decimal.NewFromInt(86400),
					decimal.Zero, decimal.Zero,
				},
			},
			staff: model.StaffPayrollSummary{
				Payroll: [5]model.StaffPayrollYear{
					{
						Year:            2025,
						YearIndex:       0,
						SubtotalPayroll: decimal.NewFromInt(100000),
						TotalPayroll:    decimal.NewFromInt(100000),
					},
					{
						Year:            2026,
						YearIndex:       1,
						SubtotalPayroll: decimal.NewFromInt(120000),
						TotalPayroll:    decimal.NewFromInt(120000),
					},
					{
						Year:            2027,
						YearIndex:       2,
						SubtotalPayroll: decimal.NewFromInt(144000),
						TotalPayroll:    decimal.NewFromInt(144000),
					},
					{Year: 2028, YearIndex: 3},
					{Year: 2029, YearIndex: 4},
				},
				FunctionalBreakdown: [5]model.StaffFunctionalYear{
					{
						Year:      2025,
						YearIndex: 0,
						RnD:       decimal.NewFromInt(20000),
						Sales:     decimal.NewFromInt(15000),
						GnA:       decimal.NewFromInt(65000),
					},
					{
						Year:      2026,
						YearIndex: 1,
						RnD:       decimal.NewFromInt(24000),
						Sales:     decimal.NewFromInt(18000),
						GnA:       decimal.NewFromInt(78000),
					},
					{
						Year:      2027,
						YearIndex: 2,
						RnD:       decimal.NewFromInt(28800),
						Sales:     decimal.NewFromInt(21600),
						GnA:       decimal.NewFromInt(93600),
					},
					{Year: 2028, YearIndex: 3},
					{Year: 2029, YearIndex: 4},
				},
			},
			capex: model.CapexSummary{
				Totals: model.CapexTotals{
					TotalDepreciation: [5]decimal.Decimal{
						decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
					},
				},
			},
			pnl: model.PnlReport{
				Years: [5]model.PnlYear{
					{Year: 2025, YearIndex: 0},
					{Year: 2026, YearIndex: 1},
					{Year: 2027, YearIndex: 2},
					{Year: 2028, YearIndex: 3},
					{Year: 2029, YearIndex: 4},
				},
			},
			checkPnl: func(t *testing.T, result model.PnlCashReport) {
				// Year 1: high profitability
				// EBIT = GrossMargin - TotalExpenses
				// TotalExpenses = RDPayroll(20000) + OutsourcedRD(20000) + RoyaltiesMisc(0)
				//              + SalesPayroll(15000) + AdvertisingPromo(15000) + MiscSalesCosts(0)
				//              + GAPayroll(65000) + InsuranceRent(0) + LeasedEquip(0)
				//              + LegalConsulting(0) + TravelMisc(25000) + Depreciation(0) = 160000
				// EBIT = 300000 - 160000 = 140000
				expectedEBIT := decimal.NewFromInt(300000).
					Sub(decimal.NewFromInt(20000)). // RDPayroll
					Sub(decimal.NewFromInt(20000)). // OutsourcedRD
					Sub(decimal.NewFromInt(15000)). // SalesPayroll
					Sub(decimal.NewFromInt(15000)). // AdvertisingPromo
					Sub(decimal.NewFromInt(65000)). // GAPayroll
					Sub(decimal.NewFromInt(25000))  // TravelMisc (recruitment_training)
				assertDecEq(t, decimal.NewFromInt(140000), expectedEBIT,
					"Manual calculation check")
				assertDecEq(t, expectedEBIT, result.Years[0].EBIT,
					"Year 1 EBIT should be 140000")

				// Year 2: maintaining profitability
				// EBIT = GrossMargin - TotalExpenses
				// TotalExpenses = RDPayroll(24000) + OutsourcedRD(24000) + RoyaltiesMisc(0)
				//              + SalesPayroll(18000) + AdvertisingPromo(18000) + MiscSalesCosts(0)
				//              + GAPayroll(78000) + InsuranceRent(0) + LeasedEquip(0)
				//              + LegalConsulting(0) + TravelMisc(30000) + Depreciation(0) = 192000
				// EBIT = 360000 - 192000 = 168000
				expectedEBIT2 := decimal.NewFromInt(360000).
					Sub(decimal.NewFromInt(24000)). // RDPayroll
					Sub(decimal.NewFromInt(24000)). // OutsourcedRD
					Sub(decimal.NewFromInt(18000)). // SalesPayroll
					Sub(decimal.NewFromInt(18000)). // AdvertisingPromo
					Sub(decimal.NewFromInt(78000)). // GAPayroll
					Sub(decimal.NewFromInt(30000))  // TravelMisc (recruitment_training)
				assertDecEq(t, expectedEBIT2, result.Years[1].EBIT,
					"Year 2 EBIT should be 168000")

				// Verify positive profitability throughout
				for year := 0; year < 3; year++ {
					assert.True(t, result.Years[year].EBIT.GreaterThan(decimal.Zero),
						"EBIT year %d should be positive", year+1)
					assert.True(t, result.Years[year].NetProfit.GreaterThan(decimal.Zero),
						"Net profit year %d should be positive", year+1)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputePnlCash(tt.entries, tt.revenue, tt.opex, tt.staff, tt.capex, tt.pnl, config)
			tt.checkPnl(t, result)
		})
	}
}
