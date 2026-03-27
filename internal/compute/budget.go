package compute

import (
	"ascenda/internal/model"

	"github.com/shopspring/decimal"
)

// ComputeBudget1 computes detailed monthly budget report for year 1 (YearIndex=1).
// Distributes annual values evenly across 12 months and applies overrides.
func ComputeBudget1(
	overrides []model.BudgetMonthlyOverride,
	pnl model.PnlReport,
	opex model.OpexSummary,
	staff model.StaffPayrollSummary,
	capex model.CapexSummary,
	config model.PlanConfig,
) model.Budget1Report {
	result := model.Budget1Report{
		YearIndex: 1,
		Rows:      []model.BudgetMonthlyRow{},
	}

	// Build override map: lineID/month -> amount (for year 1)
	overrideMap := make(map[model.BudgetLineID]map[int]decimal.Decimal)
	for _, override := range overrides {
		if override.YearIndex != 1 {
			continue
		}
		if _, ok := overrideMap[override.LineID]; !ok {
			overrideMap[override.LineID] = make(map[int]decimal.Decimal)
		}
		overrideMap[override.LineID][override.Month] = override.Amount
	}

	// Year 1 index (0-based)
	yearIdx := 0

	// currentSection tracks which section header to stamp on each row.
	currentSection := ""

	// Helper to create a row with monthly distribution
	addRow := func(lineID model.BudgetLineID, label string, annual decimal.Decimal, isTotal bool) {
		row := model.BudgetMonthlyRow{
			LineID:           lineID,
			Label:            label,
			Section:          currentSection,
			Annual:           annual,
			IsTotal:          isTotal,
			DistributionRule: model.DistEvenSpread,
		}

		// Distribute evenly across 12 months
		monthlyAmount := SafeDiv(annual, decimal.NewFromInt(int64(MonthsPerYear)))

		// Apply overrides and compute monthly values
		for month := 1; month <= MonthsPerYear; month++ {
			if monthOverrides, ok := overrideMap[lineID]; ok {
				if override, ok := monthOverrides[month]; ok {
					row.Monthly[month-1] = override
					continue
				}
			}
			row.Monthly[month-1] = monthlyAmount
		}

		// YearTotal = sum of monthly values
		row.YearTotal = decimal.Zero
		for month := 0; month < MonthsPerYear; month++ {
			row.YearTotal = row.YearTotal.Add(row.Monthly[month])
		}

		result.Rows = append(result.Rows, row)
	}

	// ─────────────────────────────────────────────────────────────
	// Revenue Section
	// ─────────────────────────────────────────────────────────────
	currentSection = "Revenue"
	salesRevenue := pnl.Years[yearIdx].Sales
	addRow(model.BudgetSalesRevenue, "Sales Revenue", salesRevenue, false)

	otherRevenue := decimal.Zero // Could be populated from pnl.Years[yearIdx].GrantsOtherRevenue if desired
	addRow(model.BudgetOtherRevenue, "Other Revenue", otherRevenue, false)

	totalRevenue := salesRevenue.Add(otherRevenue)
	addRow(model.BudgetTotalRevenue, "Total Revenue", totalRevenue, true)

	// ─────────────────────────────────────────────────────────────
	// COGS Section
	// ─────────────────────────────────────────────────────────────
	currentSection = "COGS"
	rawMaterials := pnl.Years[yearIdx].COGS
	addRow(model.BudgetRawMaterials, "Raw Materials", rawMaterials, false)

	subcontracting := decimal.Zero // Not in current P&L
	addRow(model.BudgetSubcontracting, "Subcontracting", subcontracting, false)

	directLabor := decimal.Zero // Could be populated from production payroll if available
	addRow(model.BudgetDirectLabor, "Direct Labor", directLabor, false)

	totalCOGS := rawMaterials.Add(subcontracting).Add(directLabor)
	addRow(model.BudgetTotalCOGS, "Total COGS", totalCOGS, true)

	// Gross Margin
	grossMargin := totalRevenue.Sub(totalCOGS)
	addRow(model.BudgetGrossMargin, "Gross Margin", grossMargin, true)

	// ─────────────────────────────────────────────────────────────
	// External Expenses (from OpexSummary)
	// ─────────────────────────────────────────────────────────────
	currentSection = "External Expenses"
	rentExpenses := getOpexSubcategoryTotal(opex, model.OpexSubPremises, yearIdx)
	addRow(model.BudgetRentExpenses, "Rent Expenses", rentExpenses, false)

	leasingExpenses := getOpexSubcategoryTotal(opex, model.OpexSubLeasing, yearIdx)
	addRow(model.BudgetLeasingExpenses, "Leasing Expenses", leasingExpenses, false)

	profFees := getOpexSubcategoryTotal(opex, model.OpexSubProfessional, yearIdx)
	addRow(model.BudgetProfFees, "Professional Fees", profFees, false)

	royalties := getOpexSubcategoryTotal(opex, model.OpexSubRoyalties, yearIdx)
	addRow(model.BudgetRoyalties, "Royalties", royalties, false)

	travelExpenses := getOpexSubcategoryTotal(opex, model.OpexSubTravel, yearIdx)
	addRow(model.BudgetTravelExpenses, "Travel Expenses", travelExpenses, false)

	marketingExp := getOpexSubcategoryTotal(opex, model.OpexSubMarketing, yearIdx)
	addRow(model.BudgetMarketingExp, "Marketing Expenses", marketingExp, false)

	hrExpenses := getOpexSubcategoryTotal(opex, model.OpexSubHR, yearIdx)
	addRow(model.BudgetHRExpenses, "HR Expenses", hrExpenses, false)

	totalExternal := rentExpenses.Add(leasingExpenses).Add(profFees).Add(royalties).
		Add(travelExpenses).Add(marketingExp).Add(hrExpenses)
	addRow(model.BudgetTotalExternal, "Total External Expenses", totalExternal, true)

	// ─────────────────────────────────────────────────────────────
	// Staff Costs (from StaffPayrollSummary)
	// ─────────────────────────────────────────────────────────────
	currentSection = "Staff Costs"
	payroll := staff.Payroll[yearIdx].SubtotalPayroll
	addRow(model.BudgetPayroll, "Payroll", payroll, false)

	incentives := staff.Payroll[yearIdx].SubtotalIncentives
	addRow(model.BudgetIncentives, "Incentives", incentives, false)

	totalStaff := payroll.Add(incentives)
	addRow(model.BudgetTotalStaff, "Total Staff Costs", totalStaff, true)

	// ─────────────────────────────────────────────────────────────
	// Taxes & Depreciation
	// ─────────────────────────────────────────────────────────────
	currentSection = "Taxes & Depreciation"
	taxesDuties := pnl.Years[yearIdx].TaxesAndDuties
	addRow(model.BudgetTaxesDuties, "Taxes and Duties", taxesDuties, false)

	// EBITDA = GrossMargin - TotalExternal - TotalStaff - TaxesDuties
	ebitda := grossMargin.Sub(totalExternal).Sub(totalStaff).Sub(taxesDuties)
	addRow(model.BudgetEBITDA, "EBITDA", ebitda, true)

	// Depreciation (from CapexSummary)
	depreciation := capex.Totals.TotalDepreciation[yearIdx]
	addRow(model.BudgetDepreciation, "Depreciation", depreciation, false)

	// EBIT = EBITDA - Depreciation
	ebit := ebitda.Sub(depreciation)
	addRow(model.BudgetEBIT, "EBIT", ebit, true)

	// ─────────────────────────────────────────────────────────────
	// Financial Items & Net Profit
	// ─────────────────────────────────────────────────────────────
	currentSection = "Financial & Net Profit"
	financialIncome := pnl.Years[yearIdx].FinancialRevenues
	addRow(model.BudgetFinancialIncome, "Financial Income", financialIncome, false)

	financialExp := pnl.Years[yearIdx].FinancialExpenses
	addRow(model.BudgetFinancialExp, "Financial Expenses", financialExp, false)

	// PreTaxProfit = EBIT + FinancialIncome - FinancialExp
	preTaxProfit := ebit.Add(financialIncome).Sub(financialExp)
	addRow(model.BudgetPreTaxProfit, "Pre-Tax Profit", preTaxProfit, true)

	// Corporate Tax
	corporateTax := pnl.Years[yearIdx].CorporateTax
	addRow(model.BudgetCorporateTax, "Corporate Tax", corporateTax, false)

	// Net Profit = PreTaxProfit - CorporateTax
	netProfit := preTaxProfit.Sub(corporateTax)
	addRow(model.BudgetNetProfit, "Net Profit", netProfit, true)

	return result
}

// ComputeBudget2 computes summary budget views (quarterly, semi-annual, by function, by cost type).
func ComputeBudget2(
	pnl model.PnlReport,
	opex model.OpexSummary,
	staff model.StaffPayrollSummary,
	config model.PlanConfig,
) model.Budget2Report {
	result := model.Budget2Report{}
	yearIdx := 0

	// Helper to create a view with multiple columns
	addView := func(label string, columns []string, rows []model.Budget2Row) model.Budget2View {
		return model.Budget2View{
			Label:   label,
			Columns: columns,
			Rows:    rows,
		}
	}

	// Build budget2 rows for year 1 with all line items
	allRows := buildAllBudgetRows(pnl, opex, staff, yearIdx)

	// ─────────────────────────────────────────────────────────────
	// Quarterly View (4 columns: Q1-Q4)
	// ─────────────────────────────────────────────────────────────
	quarterlyRows := []model.Budget2Row{}
	for _, row := range allRows {
		values := make([]decimal.Decimal, 4)
		// Distribute annual value into 4 quarters
		annual := decimal.Zero
		if len(row.Values) > 0 {
			annual = row.Values[0]
		}
		quarterly := SafeDiv(annual, decimal.NewFromInt(4))
		for q := 0; q < 4; q++ {
			values[q] = quarterly
		}
		quarterlyRows = append(quarterlyRows, model.Budget2Row{
			LineID:      row.LineID,
			Label:       row.Label,
			Values:      values,
			IsTotal:     row.IsTotal,
			IsAggregate: row.IsTotal,
		})
	}
	result.Quarterly = addView("Quarterly", []string{"Q1", "Q2", "Q3", "Q4"}, quarterlyRows)

	// ─────────────────────────────────────────────────────────────
	// Semi-Annual View (2 columns: H1, H2)
	// ─────────────────────────────────────────────────────────────
	semiAnnualRows := []model.Budget2Row{}
	for _, row := range allRows {
		values := make([]decimal.Decimal, 2)
		// Distribute annual value into 2 half-years
		annual := decimal.Zero
		if len(row.Values) > 0 {
			annual = row.Values[0]
		}
		semiAnnual := SafeDiv(annual, decimal.NewFromInt(2))
		for h := 0; h < 2; h++ {
			values[h] = semiAnnual
		}
		semiAnnualRows = append(semiAnnualRows, model.Budget2Row{
			LineID:  row.LineID,
			Label:   row.Label,
			Values:  values,
			IsTotal: row.IsTotal,
		})
	}
	result.SemiAnnual = addView("Semi-Annual", []string{"H1", "H2"}, semiAnnualRows)

	// ─────────────────────────────────────────────────────────────
	// By Function View (R&D, Production, Sales, G&A)
	// ─────────────────────────────────────────────────────────────
	funcRows := buildByFunctionRows(staff, yearIdx)
	result.ByFunction = addView("By Function", []string{"R&D", "Production", "Sales", "G&A"}, funcRows)

	// ─────────────────────────────────────────────────────────────
	// By Cost Type View (Fixed vs Variable)
	// ─────────────────────────────────────────────────────────────
	costTypeRows := buildByCostTypeRows(pnl, opex, staff, yearIdx)
	result.ByCostType = addView("By Cost Type", []string{"Fixed", "Variable"}, costTypeRows)

	return result
}

// Helper: Get opex subcategory total for a specific year index
func getOpexSubcategoryTotal(opex model.OpexSummary, subcat model.OpexSubcategory, yearIdx int) decimal.Decimal {
	for _, sub := range opex.Subcategories {
		if sub.Subcategory == subcat {
			return sub.Subtotal[yearIdx]
		}
	}
	return decimal.Zero
}

// Helper: Build all budget rows for Budget2
func buildAllBudgetRows(pnl model.PnlReport, opex model.OpexSummary, staff model.StaffPayrollSummary, yearIdx int) []model.Budget2Row {
	rows := []model.Budget2Row{}

	salesRevenue := pnl.Years[yearIdx].Sales
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetSalesRevenue,
		Label:   "Sales Revenue",
		Values:  []decimal.Decimal{salesRevenue},
		IsTotal: false,
	})

	otherRevenue := decimal.Zero
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetOtherRevenue,
		Label:   "Other Revenue",
		Values:  []decimal.Decimal{otherRevenue},
		IsTotal: false,
	})

	totalRevenue := salesRevenue.Add(otherRevenue)
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetTotalRevenue,
		Label:   "Total Revenue",
		Values:  []decimal.Decimal{totalRevenue},
		IsTotal: true,
	})

	rawMaterials := pnl.Years[yearIdx].COGS
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetRawMaterials,
		Label:   "Raw Materials",
		Values:  []decimal.Decimal{rawMaterials},
		IsTotal: false,
	})

	subcontracting := decimal.Zero
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetSubcontracting,
		Label:   "Subcontracting",
		Values:  []decimal.Decimal{subcontracting},
		IsTotal: false,
	})

	directLabor := decimal.Zero
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetDirectLabor,
		Label:   "Direct Labor",
		Values:  []decimal.Decimal{directLabor},
		IsTotal: false,
	})

	totalCOGS := rawMaterials.Add(subcontracting).Add(directLabor)
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetTotalCOGS,
		Label:   "Total COGS",
		Values:  []decimal.Decimal{totalCOGS},
		IsTotal: true,
	})

	grossMargin := totalRevenue.Sub(totalCOGS)
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetGrossMargin,
		Label:   "Gross Margin",
		Values:  []decimal.Decimal{grossMargin},
		IsTotal: true,
	})

	rentExpenses := getOpexSubcategoryTotal(opex, model.OpexSubPremises, yearIdx)
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetRentExpenses,
		Label:   "Rent Expenses",
		Values:  []decimal.Decimal{rentExpenses},
		IsTotal: false,
	})

	leasingExpenses := getOpexSubcategoryTotal(opex, model.OpexSubLeasing, yearIdx)
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetLeasingExpenses,
		Label:   "Leasing Expenses",
		Values:  []decimal.Decimal{leasingExpenses},
		IsTotal: false,
	})

	profFees := getOpexSubcategoryTotal(opex, model.OpexSubProfessional, yearIdx)
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetProfFees,
		Label:   "Professional Fees",
		Values:  []decimal.Decimal{profFees},
		IsTotal: false,
	})

	royalties := getOpexSubcategoryTotal(opex, model.OpexSubRoyalties, yearIdx)
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetRoyalties,
		Label:   "Royalties",
		Values:  []decimal.Decimal{royalties},
		IsTotal: false,
	})

	travelExpenses := getOpexSubcategoryTotal(opex, model.OpexSubTravel, yearIdx)
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetTravelExpenses,
		Label:   "Travel Expenses",
		Values:  []decimal.Decimal{travelExpenses},
		IsTotal: false,
	})

	marketingExp := getOpexSubcategoryTotal(opex, model.OpexSubMarketing, yearIdx)
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetMarketingExp,
		Label:   "Marketing Expenses",
		Values:  []decimal.Decimal{marketingExp},
		IsTotal: false,
	})

	hrExpenses := getOpexSubcategoryTotal(opex, model.OpexSubHR, yearIdx)
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetHRExpenses,
		Label:   "HR Expenses",
		Values:  []decimal.Decimal{hrExpenses},
		IsTotal: false,
	})

	totalExternal := rentExpenses.Add(leasingExpenses).Add(profFees).Add(royalties).
		Add(travelExpenses).Add(marketingExp).Add(hrExpenses)
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetTotalExternal,
		Label:   "Total External Expenses",
		Values:  []decimal.Decimal{totalExternal},
		IsTotal: true,
	})

	payroll := staff.Payroll[yearIdx].SubtotalPayroll
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetPayroll,
		Label:   "Payroll",
		Values:  []decimal.Decimal{payroll},
		IsTotal: false,
	})

	incentives := staff.Payroll[yearIdx].SubtotalIncentives
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetIncentives,
		Label:   "Incentives",
		Values:  []decimal.Decimal{incentives},
		IsTotal: false,
	})

	totalStaff := payroll.Add(incentives)
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetTotalStaff,
		Label:   "Total Staff Costs",
		Values:  []decimal.Decimal{totalStaff},
		IsTotal: true,
	})

	taxesDuties := pnl.Years[yearIdx].TaxesAndDuties
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetTaxesDuties,
		Label:   "Taxes and Duties",
		Values:  []decimal.Decimal{taxesDuties},
		IsTotal: false,
	})

	ebitda := grossMargin.Sub(totalExternal).Sub(totalStaff).Sub(taxesDuties)
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetEBITDA,
		Label:   "EBITDA",
		Values:  []decimal.Decimal{ebitda},
		IsTotal: true,
	})

	depreciation := decimal.Zero // Will be populated from capex if available
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetDepreciation,
		Label:   "Depreciation",
		Values:  []decimal.Decimal{depreciation},
		IsTotal: false,
	})

	ebit := ebitda.Sub(depreciation)
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetEBIT,
		Label:   "EBIT",
		Values:  []decimal.Decimal{ebit},
		IsTotal: true,
	})

	financialIncome := pnl.Years[yearIdx].FinancialRevenues
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetFinancialIncome,
		Label:   "Financial Income",
		Values:  []decimal.Decimal{financialIncome},
		IsTotal: false,
	})

	financialExp := pnl.Years[yearIdx].FinancialExpenses
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetFinancialExp,
		Label:   "Financial Expenses",
		Values:  []decimal.Decimal{financialExp},
		IsTotal: false,
	})

	preTaxProfit := ebit.Add(financialIncome).Sub(financialExp)
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetPreTaxProfit,
		Label:   "Pre-Tax Profit",
		Values:  []decimal.Decimal{preTaxProfit},
		IsTotal: true,
	})

	corporateTax := pnl.Years[yearIdx].CorporateTax
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetCorporateTax,
		Label:   "Corporate Tax",
		Values:  []decimal.Decimal{corporateTax},
		IsTotal: false,
	})

	netProfit := preTaxProfit.Sub(corporateTax)
	rows = append(rows, model.Budget2Row{
		LineID:  model.BudgetNetProfit,
		Label:   "Net Profit",
		Values:  []decimal.Decimal{netProfit},
		IsTotal: true,
	})

	return rows
}

// Helper: Build by-function breakdown (R&D, Production, Sales, G&A)
func buildByFunctionRows(staff model.StaffPayrollSummary, yearIdx int) []model.Budget2Row {
	funcYear := staff.FunctionalBreakdown[yearIdx]

	rows := []model.Budget2Row{
		{
			LineID:  model.BudgetPayroll,
			Label:   "Payroll by Function",
			Values:  []decimal.Decimal{funcYear.RnD, funcYear.Production, funcYear.Sales, funcYear.GnA},
			IsTotal: false,
		},
	}

	return rows
}

// Helper: Build by-cost-type breakdown (Fixed vs Variable)
func buildByCostTypeRows(pnl model.PnlReport, opex model.OpexSummary, staff model.StaffPayrollSummary, yearIdx int) []model.Budget2Row {
	// Simplified split: payroll is typically fixed, opex split is more nuanced
	// This is a basic allocation - adjust based on actual business logic

	totalPayroll := staff.Payroll[yearIdx].TotalPayroll
	totalOpex := opex.GrandTotal[yearIdx]

	// Assume 70% of opex is fixed, 30% variable (adjustable based on business model)
	fixedOpex := totalOpex.Mul(decimal.RequireFromString("0.7"))
	variableOpex := totalOpex.Mul(decimal.RequireFromString("0.3"))

	fixedCosts := totalPayroll.Add(fixedOpex)
	variableCosts := variableOpex

	rows := []model.Budget2Row{
		{
			LineID:  model.BudgetPayroll,
			Label:   "Costs by Type",
			Values:  []decimal.Decimal{fixedCosts, variableCosts},
			IsTotal: false,
		},
	}

	return rows
}
