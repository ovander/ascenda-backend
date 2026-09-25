package compute

import (
	"ascenda/internal/model"

	"github.com/shopspring/decimal"
)

// ComputePnl computes the full P&L statement according to the new model types.
// wcr and fiplan are optional (pass zero values for the first-pass in the engine);
// after Layer 3 the engine calls this a second time with the real WCR and FiPlan.
func ComputePnl(
	pnlEntries []model.PnlManualEntry,
	revenue model.ConsolidatedRevenue,
	staff model.StaffPayrollSummary,
	capex model.CapexSummary,
	opex model.OpexSummary,
	fiplan model.FiplanReport,
	wcr model.WCRReport,
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

	startYear := config.ForecastStart.Year()
	if startYear < 2000 {
		startYear = 2024 // safe fallback if forecastStart not set
	}

	// yearIndex is 0-based (0..4) throughout the loop — used as array index
	// and as the key into the PnlManualEntry map (same 0-based convention as
	// opex and capex manual entries).
	// PnlYear.YearIndex in the output is stored as 1-based (1..5), consistent
	// with the staffPayroll output and the revenue yearIndex field.
	for yearIndex := 0; yearIndex < 5; yearIndex++ {
		year := &model.PnlYear{
			Year:      startYear + yearIndex,
			YearIndex: yearIndex + 1, // 1-based in output, matching revenue/staff convention
		}

		// 1. Sales
		year.Sales = revenue.Totals[yearIndex].TotalTurnover

		// 2. Export Sales Memo (informational)
		year.ExportSalesMemo = revenue.Totals[yearIndex].EuropeExportSales

		// 3. Capitalized Production (manual entry)
		year.CapitalizedProduction = getAmount(model.PnlCapitalizedProd, yearIndex)

		// 4. Stored Production — inventory change (WCR inventory at end of year
		//    minus inventory at end of previous year / opening balance for year 0).
		//    A positive change (inventory grew) is operating revenue; negative means
		//    inventory drawdown and reduces operating revenue.
		if yearIndex == 0 {
			year.StoredProduction = wcr.Inventory.InventoryValue[0].Sub(wcr.Inventory.InitialInventory)
		} else {
			year.StoredProduction = wcr.Inventory.InventoryValue[yearIndex].Sub(wcr.Inventory.InventoryValue[yearIndex-1])
		}

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

		// 16. Grants and Other Revenue — non-repayable subsidies and other grants
		//    recognised as operating income in the year they are received.
		//    Subsidies + OtherGrants from FiPlan resources (RepayableGrants are
		//    treated as debt and excluded from P&L revenue).
		year.GrantsOtherRevenue = fiplan.Plan.Resources.Subsidies[yearIndex].
			Add(fiplan.Plan.Resources.OtherGrants[yearIndex])

		// 17. Other Operating Expenses (manual entry)
		year.OtherOperatingExp = getAmount(model.PnlOtherOperatingExp, yearIndex)

		// 18. EBIT
		year.EBIT = year.EBITDA.
			Sub(year.Depreciation).
			Sub(year.Impairment).
			Add(year.GrantsOtherRevenue).
			Sub(year.OtherOperatingExp)

		// 19. Financial Revenues — interest earned on positive cash balances.
		year.FinancialRevenues = decimal.Zero // reserved for future: cash × InterestOnPositiveCash

		// 20. Financial Expenses — interest on outstanding MLT debt (from FiPlan).
		year.FinancialExpenses = fiplan.LoanInterest[yearIndex]

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
		Years:           chartYears,
		EBITDAPositive:  chartEBITDAPositive,
		EBITDANegative:  chartEBITDANegative,
		OtherOpex:       chartOtherOpex,
		PayrollExpenses: chartPayroll,
	}

	return result
}
