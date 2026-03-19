package compute

import (
	"kerplan/internal/model"

	"github.com/shopspring/decimal"
)

// ComputePnl computes the full P&L statement according to the new model types
func ComputePnl(
	pnlEntries []model.PnlManualEntry,
	revenue model.ConsolidatedRevenue,
	staff model.StaffPayrollSummary,
	capex model.CapexSummary,
	opex model.OpexSummary,
	fiplan model.FiplanReport,
	config model.PlanConfig,
) model.PnlReport {
	result := model.PnlReport{}

	// Build lookup map: lineID/yearIndex -> amount
	entryMap := make(map[model.PnlLineID]map[int]decimal.Decimal)
	for _, entry := range pnlEntries {
		if _, ok := entryMap[entry.LineID]; !ok {
			entryMap[entry.LineID] = make(map[int]decimal.Decimal)
		}
		entryMap[entry.LineID][entry.YearIndex] = entry.Amount
	}

	// Helper to get value from map or use zero
	getAmount := func(lineID model.PnlLineID, yearIndex int) decimal.Decimal {
		if yearVals, ok := entryMap[lineID]; ok {
			if val, ok := yearVals[yearIndex]; ok {
				return val
			}
		}
		return decimal.Zero
	}

	// Helper for safe division
	safeDiv := func(numerator, denominator decimal.Decimal) decimal.Decimal {
		if denominator.IsZero() {
			return decimal.Zero
		}
		return numerator.Div(denominator)
	}

	// Compute 5 years of P&L data
	chartYears := [5]int{}
	chartEBITDAPositive := [5]decimal.Decimal{}
	chartEBITDANegative := [5]decimal.Decimal{}
	chartOtherOpex := [5]decimal.Decimal{}
	chartPayroll := [5]decimal.Decimal{}

	for yearIndex := 0; yearIndex < 5; yearIndex++ {
		year := &model.PnlYear{
			Year:      2024 + yearIndex,
			YearIndex: yearIndex,
		}

		// 1. Sales
		year.Sales = revenue.Totals[yearIndex].TotalTurnover

		// 2. Export Sales Memo (informational)
		year.ExportSalesMemo = revenue.Totals[yearIndex].EuropeExportSales

		// 3. Capitalized Production (manual entry)
		year.CapitalizedProduction = getAmount(model.PnlCapitalizedProd, yearIndex)

		// 4. Stored Production (from WCR, for now 0)
		year.StoredProduction = decimal.Zero

		// 5. Total Operating Revenue
		year.TotalOperatingRevenue = year.Sales.
			Add(year.CapitalizedProduction).
			Add(year.StoredProduction)

		// 6. COGS
		year.COGS = revenue.Totals[yearIndex].TotalCOGS

		// 7. Inventory Change (from WCR, for now 0)
		year.InventoryChange = decimal.Zero

		// 8. External Expenses
		year.ExternalExpenses = opex.GrandTotal[yearIndex]

		// 9. Total Consumption
		year.TotalConsumption = year.COGS.
			Add(year.InventoryChange).
			Add(year.ExternalExpenses)

		// 10. Added Value
		year.AddedValue = year.TotalOperatingRevenue.Sub(year.TotalConsumption)

		// 11. Taxes and Duties
		year.TaxesAndDuties = year.Sales.Mul(config.TaxesAndDutiesRate)

		// 12. Payroll Expenses
		year.PayrollExpenses = staff.Payroll[yearIndex].TotalPayroll

		// 13. EBITDA
		year.EBITDA = year.AddedValue.
			Sub(year.TaxesAndDuties).
			Sub(year.PayrollExpenses)

		// 14. Depreciation
		year.Depreciation = capex.Totals.TotalDepreciation[yearIndex]

		// 15. Impairment (manual entry)
		year.Impairment = getAmount(model.PnlImpairment, yearIndex)

		// 16. Grants and Other Revenue (from fiplan, for now 0)
		year.GrantsOtherRevenue = decimal.Zero

		// 17. Other Operating Expenses (manual entry)
		year.OtherOperatingExp = getAmount(model.PnlOtherOperatingExp, yearIndex)

		// 18. EBIT
		year.EBIT = year.EBITDA.
			Sub(year.Depreciation).
			Sub(year.Impairment).
			Add(year.GrantsOtherRevenue).
			Sub(year.OtherOperatingExp)

		// 19. Financial Revenues (from fiplan, for now 0)
		year.FinancialRevenues = decimal.Zero

		// 20. Financial Expenses (from fiplan, for now 0)
		year.FinancialExpenses = decimal.Zero

		// 21. Pre-tax Earnings
		year.PreTaxEarnings = year.EBIT.
			Add(year.FinancialRevenues).
			Sub(year.FinancialExpenses)

		// 22. Extraordinary Income (manual entry)
		year.ExtraordinaryIncome = getAmount(model.PnlExtraordinaryIncome, yearIndex)

		// 23. Extraordinary Expense (manual entry)
		year.ExtraordinaryExpense = getAmount(model.PnlExtraordinaryExpense, yearIndex)

		// 24. Employee Participation (manual entry)
		year.EmployeeParticipation = getAmount(model.PnlEmployeeParticipation, yearIndex)

		// Taxable base for corporate tax calculation
		taxableBase := year.PreTaxEarnings.
			Add(year.ExtraordinaryIncome).
			Sub(year.ExtraordinaryExpense).
			Sub(year.EmployeeParticipation)

		// 25. Corporate Tax (only on positive taxable base)
		if taxableBase.GreaterThan(decimal.Zero) {
			year.CorporateTax = taxableBase.Mul(config.CorporateTaxRate)
		} else {
			year.CorporateTax = decimal.Zero
		}

		// 26. Tax Credits (from MultiYearAdjustment, for now 0)
		year.TaxCredits = decimal.Zero

		// 27. Net Profit
		year.NetProfit = year.PreTaxEarnings.
			Add(year.ExtraordinaryIncome).
			Sub(year.ExtraordinaryExpense).
			Sub(year.EmployeeParticipation).
			Sub(year.CorporateTax).
			Add(year.TaxCredits)

		// 28. Staff Headcount
		year.StaffHeadcount = staff.Headcount.TotalStaff[yearIndex]

		// 29. Cash Flow
		year.CashFlow = year.NetProfit.
			Add(year.Depreciation).
			Add(year.Impairment)

		// 30. Percentage of Sales
		year.PctOfSales = safeDiv(year.NetProfit, year.Sales)

		result.Years[yearIndex] = *year

		// Chart data
		chartYears[yearIndex] = year.Year
		if year.EBITDA.GreaterThan(decimal.Zero) {
			chartEBITDAPositive[yearIndex] = year.EBITDA
		} else {
			chartEBITDAPositive[yearIndex] = decimal.Zero
		}
		if year.EBITDA.LessThan(decimal.Zero) {
			chartEBITDANegative[yearIndex] = year.EBITDA
		} else {
			chartEBITDANegative[yearIndex] = decimal.Zero
		}
		chartOtherOpex[yearIndex] = year.ExternalExpenses
		chartPayroll[yearIndex] = year.PayrollExpenses
	}

	result.ChartData = model.PnlChartData{
		Years:              chartYears,
		EBITDAPositive:     chartEBITDAPositive,
		EBITDANegative:     chartEBITDANegative,
		OtherOpex:          chartOtherOpex,
		PayrollExpenses:    chartPayroll,
	}

	return result
}
