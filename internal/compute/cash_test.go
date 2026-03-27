package compute

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"ascenda/internal/model"
)

func TestComputeCash(t *testing.T) {
	config := model.PlanConfig{
		DiscountRate:     decimal.NewFromFloat(0.1),
		CorporateTaxRate: decimal.NewFromFloat(0.25),
		EmployerTaxRate:  decimal.NewFromFloat(0.42),
	}

	tests := []struct {
		name         string
		overrides    []model.CashMonthlyOverride
		pnl          model.PnlReport
		revenue      model.ConsolidatedRevenue
		staff        model.StaffPayrollSummary
		capex        model.CapexSummary
		opex         model.OpexSummary
		wcr          model.WCRReport
		fiplan       model.FiplanReport
		checkMonthly func(*testing.T, model.CashReport)
	}{
		{
			name:      "basic monthly cash flow from annual data",
			overrides: []model.CashMonthlyOverride{},
			pnl: model.PnlReport{
				Years: [5]model.PnlYear{
					{
						Year:           1,
						YearIndex:      0,
						Sales:          decimal.NewFromInt(120000),
						ExternalExpenses: decimal.NewFromInt(60000),
						PayrollExpenses: decimal.NewFromInt(34080),
						Depreciation:   decimal.Zero,
						CorporateTax:   decimal.Zero,
						NetProfit:      decimal.NewFromInt(25920),
					},
					{
						Year:           2,
						YearIndex:      1,
						Sales:          decimal.NewFromInt(144000),
						ExternalExpenses: decimal.NewFromInt(72000),
						PayrollExpenses: decimal.NewFromInt(40896),
						Depreciation:   decimal.Zero,
						CorporateTax:   decimal.Zero,
						NetProfit:      decimal.NewFromInt(31104),
					},
					{
						Year:           3,
						YearIndex:      2,
						Sales:          decimal.NewFromInt(172800),
						ExternalExpenses: decimal.NewFromInt(86400),
						PayrollExpenses: decimal.NewFromInt(49075),
						Depreciation:   decimal.Zero,
						CorporateTax:   decimal.Zero,
						NetProfit:      decimal.NewFromInt(37325),
					},
					{},
					{},
				},
			},
			revenue: model.ConsolidatedRevenue{
				Totals: [5]model.ConsolidatedRevenueYear{
					{Year: 1, TotalTurnover: decimal.NewFromInt(120000)},
					{Year: 2, TotalTurnover: decimal.NewFromInt(144000)},
					{Year: 3, TotalTurnover: decimal.NewFromInt(172800)},
					{},
					{},
				},
			},
			staff: model.StaffPayrollSummary{
				Payroll: [5]model.StaffPayrollYear{
					{
						Year:           1,
						YearIndex:      0,
						SubtotalPayroll: decimal.NewFromInt(24000),
						TotalPayroll:    decimal.NewFromInt(34080),
					},
					{
						Year:           2,
						YearIndex:      1,
						SubtotalPayroll: decimal.NewFromInt(28800),
						TotalPayroll:    decimal.NewFromInt(40896),
					},
					{
						Year:           3,
						YearIndex:      2,
						SubtotalPayroll: decimal.NewFromInt(34560),
						TotalPayroll:    decimal.NewFromInt(49075),
					},
					{},
					{},
				},
				EmployerTaxRate: decimal.NewFromFloat(0.42),
			},
			capex: model.CapexSummary{
				Totals: model.CapexTotals{
					TotalCapex: [5]decimal.Decimal{
						decimal.NewFromInt(30000),
						decimal.NewFromInt(15000),
						decimal.NewFromInt(10000),
						decimal.Zero,
						decimal.Zero,
					},
				},
			},
			opex: model.OpexSummary{
				GrandTotal: [5]decimal.Decimal{
					decimal.NewFromInt(60000),
					decimal.NewFromInt(72000),
					decimal.NewFromInt(86400),
					decimal.Zero,
					decimal.Zero,
				},
			},
			wcr:    model.WCRReport{},
			fiplan: model.FiplanReport{},
			checkMonthly: func(t *testing.T, result model.CashReport) {
				// Check year 1, month 1
				year1 := result.Years[0]
				assert.Equal(t, 1, year1.YearIndex)
				assert.Equal(t, 1, len(year1.Revenue.Lines), "Revenue section should have 1 line (Sales Revenue)")

				// Check section totals for first month (Total is [12]decimal.Decimal fixed array)
				assert.Equal(t, 12, len(year1.Revenue.Total))
				expectedRevenue := decimal.NewFromInt(10000)
				assertDecEq(t, expectedRevenue, year1.Revenue.Total[0],
					"Year 1 Month 1 revenue total should be monthly revenue")

				// Check that all 12 months are processed for each year
				assert.Equal(t, 3, len(result.Years), "Should have 3 years of data")
				for yearIdx := 0; yearIdx < 3; yearIdx++ {
					assert.Equal(t, yearIdx+1, result.Years[yearIdx].YearIndex,
						"Year index mismatch for year %d", yearIdx)
					assert.Equal(t, 12, len(result.Years[yearIdx].Revenue.Total),
						"Each year should have 12 months of revenue")
				}
			},
		},
		{
			name:      "empty overrides uses default distribution",
			overrides: []model.CashMonthlyOverride{},
			pnl: model.PnlReport{
				Years: [5]model.PnlYear{
					{
						Year:           1,
						YearIndex:      0,
						Sales:          decimal.NewFromInt(12000),
						ExternalExpenses: decimal.NewFromInt(6000),
						PayrollExpenses: decimal.NewFromInt(3408),
						NetProfit:       decimal.NewFromInt(2592),
					},
					{}, {}, {}, {},
				},
			},
			revenue: model.ConsolidatedRevenue{
				Totals: [5]model.ConsolidatedRevenueYear{
					{Year: 1, TotalTurnover: decimal.NewFromInt(12000)},
					{}, {}, {}, {},
				},
			},
			staff: model.StaffPayrollSummary{
				Payroll: [5]model.StaffPayrollYear{
					{
						Year:           1,
						YearIndex:      0,
						SubtotalPayroll: decimal.NewFromInt(2400),
						TotalPayroll:    decimal.NewFromInt(3408),
					},
					{}, {}, {}, {},
				},
				EmployerTaxRate: decimal.NewFromFloat(0.42),
			},
			capex: model.CapexSummary{
				Totals: model.CapexTotals{
					TotalCapex: [5]decimal.Decimal{
						decimal.NewFromInt(3000),
						decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
					},
				},
			},
			opex: model.OpexSummary{
				GrandTotal: [5]decimal.Decimal{
					decimal.NewFromInt(6000),
					decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
				},
			},
			wcr:    model.WCRReport{},
			fiplan: model.FiplanReport{},
			checkMonthly: func(t *testing.T, result model.CashReport) {
				// With even distribution: 12000/12 = 1000 per month
				expectedMonthlyRevenue := decimal.NewFromInt(1000)

				year1 := result.Years[0]
				for month := 0; month < 12; month++ {
					assertDecEq(t, expectedMonthlyRevenue, year1.Revenue.Total[month],
						"Month %d should have even revenue distribution", month+1)
				}
			},
		},
		{
			name: "with overrides replacing specific months",
			overrides: []model.CashMonthlyOverride{
				{LineID: model.CashOtherRevenues, YearIndex: 0, Month: 1, Amount: decimal.NewFromInt(15000)},
				{LineID: model.CashOtherRevenues, YearIndex: 0, Month: 6, Amount: decimal.NewFromInt(12000)},
				{LineID: model.CashOtherRevenues, YearIndex: 0, Month: 12, Amount: decimal.NewFromInt(20000)},
			},
			pnl: model.PnlReport{
				Years: [5]model.PnlYear{
					{
						Year:           1,
						YearIndex:      0,
						Sales:          decimal.NewFromInt(120000),
						ExternalExpenses: decimal.NewFromInt(60000),
						PayrollExpenses: decimal.NewFromInt(3408),
						NetProfit:       decimal.NewFromInt(56592),
					},
					{}, {}, {}, {},
				},
			},
			revenue: model.ConsolidatedRevenue{
				Totals: [5]model.ConsolidatedRevenueYear{
					{Year: 1, TotalTurnover: decimal.NewFromInt(120000)},
					{}, {}, {}, {},
				},
			},
			staff: model.StaffPayrollSummary{
				Payroll: [5]model.StaffPayrollYear{
					{
						Year:           1,
						YearIndex:      0,
						SubtotalPayroll: decimal.NewFromInt(2400),
						TotalPayroll:    decimal.NewFromInt(3408),
					},
					{}, {}, {}, {},
				},
				EmployerTaxRate: decimal.NewFromFloat(0.42),
			},
			capex: model.CapexSummary{
				Totals: model.CapexTotals{
					TotalCapex: [5]decimal.Decimal{
						decimal.NewFromInt(1200),
						decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
					},
				},
			},
			opex: model.OpexSummary{
				GrandTotal: [5]decimal.Decimal{
					decimal.NewFromInt(60000),
					decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
				},
			},
			wcr:    model.WCRReport{},
			fiplan: model.FiplanReport{},
			checkMonthly: func(t *testing.T, result model.CashReport) {
				year1 := result.Years[0]

				// Check that sections exist and have monthly data
				assert.Equal(t, 12, len(year1.Revenue.Total),
					"Should have 12 months of revenue data")
				assert.Equal(t, 12, len(year1.Operating.Total),
					"Should have 12 months of operating data")
				assert.Equal(t, 12, len(year1.Capex.Total),
					"Should have 12 months of capex data")

				// Revenue section should contain lines including OtherRevenues
				foundOtherRevenues := false
				for _, line := range year1.Revenue.Lines {
					if line.LineID == model.CashOtherRevenues {
						foundOtherRevenues = true
						// Check overridden months
						assert.True(t, line.Monthly[0].Equal(decimal.NewFromInt(15000)),
							"Month 1 should be overridden to 15000")
						assert.True(t, line.Monthly[5].Equal(decimal.NewFromInt(12000)),
							"Month 6 should be overridden to 12000")
						assert.True(t, line.Monthly[11].Equal(decimal.NewFromInt(20000)),
							"Month 12 should be overridden to 20000")
						break
					}
				}
				assert.True(t, foundOtherRevenues,
					"Should find OtherRevenues line in revenue section")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeCash(tt.overrides, tt.pnl, tt.wcr, tt.capex, tt.fiplan, tt.revenue, tt.staff, tt.opex, config)
			tt.checkMonthly(t, result)
		})
	}
}
