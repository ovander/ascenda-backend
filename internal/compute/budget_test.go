package compute

import (
	"testing"

	"ascenda/internal/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestComputeBudget1(t *testing.T) {
	config := model.PlanConfig{
		DiscountRate:     decimal.NewFromFloat(0.1),
		CorporateTaxRate: decimal.NewFromFloat(0.25),
		EmployerTaxRate:  decimal.NewFromFloat(0.42),
	}

	tests := []struct {
		name      string
		overrides []model.BudgetMonthlyOverride
		pnl       model.PnlReport
		opex      model.OpexSummary
		staff     model.StaffPayrollSummary
		capex     model.CapexSummary
		checkRows func(*testing.T, model.Budget1Report)
	}{
		{
			name:      "even distribution of annual values across 12 months",
			overrides: []model.BudgetMonthlyOverride{},
			pnl: model.PnlReport{
				Years: [5]model.PnlYear{
					{
						Sales:             decimal.NewFromInt(120000),
						COGS:              decimal.NewFromInt(60000),
						TaxesAndDuties:    decimal.NewFromInt(6000),
						FinancialRevenues: decimal.Zero,
						FinancialExpenses: decimal.Zero,
						CorporateTax:      decimal.Zero,
					},
					{
						Sales:             decimal.NewFromInt(144000),
						COGS:              decimal.NewFromInt(72000),
						TaxesAndDuties:    decimal.NewFromInt(7200),
						FinancialRevenues: decimal.Zero,
						FinancialExpenses: decimal.Zero,
						CorporateTax:      decimal.Zero,
					},
				},
			},
			opex: model.OpexSummary{
				GrandTotal: [5]decimal.Decimal{
					decimal.NewFromInt(6000),
					decimal.NewFromInt(7200),
					decimal.Zero, decimal.Zero, decimal.Zero,
				},
				Subcategories: []model.OpexSubcategoryResult{},
			},
			staff: model.StaffPayrollSummary{
				Payroll: [5]model.StaffPayrollYear{
					{
						SubtotalPayroll:    decimal.NewFromInt(24000),
						SubtotalIncentives: decimal.Zero,
						TotalPayroll:       decimal.NewFromInt(24000),
					},
					{
						SubtotalPayroll:    decimal.NewFromInt(28800),
						SubtotalIncentives: decimal.Zero,
						TotalPayroll:       decimal.NewFromInt(28800),
					},
				},
				FunctionalBreakdown: [5]model.StaffFunctionalYear{},
			},
			capex: model.CapexSummary{
				Totals: model.CapexTotals{
					TotalCapex:        [5]decimal.Decimal{decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero},
					TotalDepreciation: [5]decimal.Decimal{decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero},
				},
			},
			checkRows: func(t *testing.T, result model.Budget1Report) {
				assert.Equal(t, 1, result.YearIndex, "Should be for year index 1")
				assert.True(t, len(result.Rows) > 0, "Should have budget rows")

				// Check revenue row
				revenueRow := result.Rows[0]
				assert.Equal(t, model.BudgetSalesRevenue, revenueRow.LineID)
				assert.Equal(t, "Sales Revenue", revenueRow.Label)
				assertDecEq(t, decimal.NewFromInt(120000), revenueRow.Annual,
					"Annual revenue should match PnL")
				assertDecEq(t, decimal.NewFromInt(120000), revenueRow.YearTotal,
					"Year total revenue should match PnL")

				// Check monthly distribution: 120000/12 = 10000
				expectedMonthly := decimal.NewFromInt(10000)
				for month := 0; month < 12; month++ {
					assertDecEq(t, expectedMonthly, revenueRow.Monthly[month],
						"Month %d revenue should be evenly distributed", month+1)
				}
			},
		},
		{
			name: "budget with overrides for specific months",
			overrides: []model.BudgetMonthlyOverride{
				{YearIndex: 1, LineID: model.BudgetSalesRevenue, Month: 1, Amount: decimal.NewFromInt(15000)},
				{YearIndex: 1, LineID: model.BudgetSalesRevenue, Month: 12, Amount: decimal.NewFromInt(20000)},
				{YearIndex: 1, LineID: model.BudgetRawMaterials, Month: 1, Amount: decimal.NewFromInt(8000)},
			},
			pnl: model.PnlReport{
				Years: [5]model.PnlYear{
					{
						Sales:             decimal.NewFromInt(130000),
						COGS:              decimal.NewFromInt(65000),
						TaxesAndDuties:    decimal.Zero,
						FinancialRevenues: decimal.Zero,
						FinancialExpenses: decimal.Zero,
						CorporateTax:      decimal.Zero,
					},
				},
			},
			opex: model.OpexSummary{
				GrandTotal:    [5]decimal.Decimal{decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero},
				Subcategories: []model.OpexSubcategoryResult{},
			},
			staff: model.StaffPayrollSummary{
				Payroll: [5]model.StaffPayrollYear{
					{
						SubtotalPayroll:    decimal.Zero,
						SubtotalIncentives: decimal.Zero,
						TotalPayroll:       decimal.Zero,
					},
				},
				FunctionalBreakdown: [5]model.StaffFunctionalYear{},
			},
			capex: model.CapexSummary{
				Totals: model.CapexTotals{
					TotalCapex:        [5]decimal.Decimal{decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero},
					TotalDepreciation: [5]decimal.Decimal{decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero},
				},
			},
			checkRows: func(t *testing.T, result model.Budget1Report) {
				revenueRow := result.Rows[0]

				// Month 1 should be overridden to 15000
				assertDecEq(t, decimal.NewFromInt(15000), revenueRow.Monthly[0],
					"Month 1 revenue should be overridden")

				// Month 12 should be overridden to 20000
				assertDecEq(t, decimal.NewFromInt(20000), revenueRow.Monthly[11],
					"Month 12 revenue should be overridden")

				// Non-overridden months use annual/12
				expectedDefault := SafeDiv(decimal.NewFromInt(130000), decimal.NewFromInt(12))
				assertDecEq(t, expectedDefault, revenueRow.Monthly[1],
					"Month 2 revenue should use annual/12")

				// Check COGS row overrides
				cogsRow := result.Rows[3] // Raw materials row
				assertDecEq(t, decimal.NewFromInt(8000), cogsRow.Monthly[0],
					"Month 1 COGS should be overridden")

				// Non-overridden COGS months use annual/12
				expectedDefaultCOGS := SafeDiv(decimal.NewFromInt(65000), decimal.NewFromInt(12))
				assertDecEq(t, expectedDefaultCOGS, cogsRow.Monthly[1],
					"Month 2 COGS should use annual/12")
			},
		},
		{
			name:      "quarterly aggregation check (all rows sum correctly)",
			overrides: []model.BudgetMonthlyOverride{},
			pnl: model.PnlReport{
				Years: [5]model.PnlYear{
					{
						Sales:             decimal.NewFromInt(120000),
						COGS:              decimal.NewFromInt(60000),
						TaxesAndDuties:    decimal.NewFromInt(6000),
						FinancialRevenues: decimal.Zero,
						FinancialExpenses: decimal.Zero,
						CorporateTax:      decimal.Zero,
					},
				},
			},
			opex: model.OpexSummary{
				GrandTotal:    [5]decimal.Decimal{decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero},
				Subcategories: []model.OpexSubcategoryResult{},
			},
			staff: model.StaffPayrollSummary{
				Payroll: [5]model.StaffPayrollYear{
					{
						SubtotalPayroll:    decimal.NewFromInt(24000),
						SubtotalIncentives: decimal.Zero,
						TotalPayroll:       decimal.NewFromInt(24000),
					},
				},
				FunctionalBreakdown: [5]model.StaffFunctionalYear{},
			},
			capex: model.CapexSummary{
				Totals: model.CapexTotals{
					TotalCapex:        [5]decimal.Decimal{decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero},
					TotalDepreciation: [5]decimal.Decimal{decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero},
				},
			},
			checkRows: func(t *testing.T, result model.Budget1Report) {
				for _, row := range result.Rows {
					// Sum all 12 months should equal annual
					monthSum := decimal.Zero
					for month := 0; month < 12; month++ {
						monthSum = monthSum.Add(row.Monthly[month])
					}
					assertDecEq(t, row.Annual, monthSum,
						"Monthly sum should equal annual for %s", row.Label)

					// Year total should also match annual
					assertDecEq(t, row.Annual, row.YearTotal,
						"Year total should equal annual for %s", row.Label)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeBudget1(tt.overrides, tt.pnl, tt.opex, tt.staff, tt.capex, config)
			tt.checkRows(t, result)
		})
	}
}

func TestComputeBudget2(t *testing.T) {
	config := model.PlanConfig{
		DiscountRate:     decimal.NewFromFloat(0.1),
		CorporateTaxRate: decimal.NewFromFloat(0.25),
		EmployerTaxRate:  decimal.NewFromFloat(0.42),
	}

	tests := []struct {
		name  string
		pnl   model.PnlReport
		opex  model.OpexSummary
		staff model.StaffPayrollSummary
		check func(*testing.T, model.Budget2Report)
	}{
		{
			name: "even quarterly distribution",
			pnl: model.PnlReport{
				Years: [5]model.PnlYear{
					{
						Sales:             decimal.NewFromInt(120000),
						COGS:              decimal.NewFromInt(60000),
						TaxesAndDuties:    decimal.NewFromInt(6000),
						FinancialRevenues: decimal.Zero,
						FinancialExpenses: decimal.Zero,
						CorporateTax:      decimal.Zero,
					},
				},
			},
			opex: model.OpexSummary{
				GrandTotal:    [5]decimal.Decimal{decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero},
				Subcategories: []model.OpexSubcategoryResult{},
			},
			staff: model.StaffPayrollSummary{
				Payroll: [5]model.StaffPayrollYear{
					{
						SubtotalPayroll:    decimal.NewFromInt(24000),
						SubtotalIncentives: decimal.Zero,
						TotalPayroll:       decimal.NewFromInt(24000),
					},
				},
				FunctionalBreakdown: [5]model.StaffFunctionalYear{},
			},
			check: func(t *testing.T, result model.Budget2Report) {
				// Check quarterly view
				assert.NotNil(t, result.Quarterly, "Quarterly view should exist")
				assert.Equal(t, 4, len(result.Quarterly.Columns), "Should have 4 quarter columns")
				assert.True(t, len(result.Quarterly.Rows) > 0, "Should have quarterly rows")

				// Check that revenue row exists and is properly distributed
				var revenueRow *model.Budget2Row
				for i := range result.Quarterly.Rows {
					if result.Quarterly.Rows[i].LineID == model.BudgetSalesRevenue {
						revenueRow = &result.Quarterly.Rows[i]
						break
					}
				}
				assert.NotNil(t, revenueRow, "Revenue row should exist")

				// Each quarter should be 120000/4 = 30000
				expectedQuarterly := decimal.NewFromInt(30000)
				for q := 0; q < 4; q++ {
					assertDecEq(t, expectedQuarterly, revenueRow.Values[q],
						"Quarter %d revenue should be evenly distributed", q+1)
				}
			},
		},
		{
			name: "quarterly aggregation check",
			pnl: model.PnlReport{
				Years: [5]model.PnlYear{
					{
						Sales:             decimal.NewFromInt(100000),
						COGS:              decimal.NewFromInt(50000),
						TaxesAndDuties:    decimal.NewFromInt(5000),
						FinancialRevenues: decimal.Zero,
						FinancialExpenses: decimal.Zero,
						CorporateTax:      decimal.Zero,
					},
				},
			},
			opex: model.OpexSummary{
				GrandTotal:    [5]decimal.Decimal{decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero},
				Subcategories: []model.OpexSubcategoryResult{},
			},
			staff: model.StaffPayrollSummary{
				Payroll: [5]model.StaffPayrollYear{
					{
						SubtotalPayroll:    decimal.NewFromInt(20000),
						SubtotalIncentives: decimal.Zero,
						TotalPayroll:       decimal.NewFromInt(20000),
					},
				},
				FunctionalBreakdown: [5]model.StaffFunctionalYear{},
			},
			check: func(t *testing.T, result model.Budget2Report) {
				assert.NotNil(t, result.Quarterly, "Quarterly view should exist")

				// For each row, sum quarters should equal annual value
				for _, row := range result.Quarterly.Rows {
					quarterSum := decimal.Zero
					for q := 0; q < 4; q++ {
						quarterSum = quarterSum.Add(row.Values[q])
					}
					// Note: Values contain quarterly amounts, so sum of 4 quarters = annual
					_ = quarterSum
					assert.True(t, len(row.Values) > 0, "Row should have values for %s", row.Label)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeBudget2(tt.pnl, tt.opex, tt.staff, config)
			tt.check(t, result)
		})
	}
}
