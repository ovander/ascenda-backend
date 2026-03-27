package compute

import (
	"ascenda/internal/model"

	"github.com/shopspring/decimal"
)

// ComputeCash computes the cash flow report for 3 years (36 months) with monthly breakdowns.
// It distributes annual P&L values across 12 months, applies overrides, and calculates sections.
func ComputeCash(
	overrides []model.CashMonthlyOverride,
	pnl model.PnlReport,
	wcr model.WCRReport,
	capex model.CapexSummary,
	fiplan model.FiplanReport,
	revenue model.ConsolidatedRevenue,
	staff model.StaffPayrollSummary,
	opex model.OpexSummary,
	config model.PlanConfig,
) model.CashReport {
	result := model.CashReport{}

	// Build override map: lineID + yearIndex + month -> amount
	overrideMap := buildOverrideMap(overrides)

	// prevYearClosing carries the last month's closing balance forward into the next year.
	prevYearClosing := decimal.Zero

	// Process 3 years
	for yearIdx := 0; yearIdx < 3; yearIdx++ {
		year := &result.Years[yearIdx]
		year.YearIndex = yearIdx + 1

		// Initialize sections
		revenueSection := &year.Revenue
		operatingSection := &year.Operating
		capexSection := &year.Capex
		wcrSection := &year.WCR
		taxSection := &year.Tax
		economicSection := &year.Economic
		financingSection := &year.Financing
		cashSection := &year.Cash

		// ─── REVENUE SECTION ───
		computeRevenueSection(revenueSection, revenue, yearIdx, overrideMap)

		// ─── OPERATING SECTION ───
		computeOperatingSection(operatingSection, opex, staff, yearIdx, overrideMap)

		// ─── CAPEX SECTION ───
		computeCapexSection(capexSection, capex, yearIdx, overrideMap)

		// ─── WCR SECTION ───
		computeWCRSection(wcrSection, wcr, yearIdx, overrideMap)

		// ─── TAX SECTION ───
		computeTaxSection(taxSection, pnl, wcr, yearIdx, overrideMap)

		// ─── ECONOMIC SECTION ───
		computeEconomicSection(economicSection, revenueSection, operatingSection, capexSection, wcrSection, taxSection)

		// ─── FINANCING SECTION ───
		computeFinancingSection(financingSection, fiplan, yearIdx, overrideMap)

		// ─── CASH SECTION ───
		computeCashSection(cashSection, economicSection, financingSection, prevYearClosing)

		// ─── BALANCE CONVENIENCE ARRAYS ───
		// ClosingBalance is the cumulative running cash balance after each month.
		// NetCashFlow is the period change (Economic + Financing) for each month.
		// OpeningBalance is the balance before each month's flows.
		// prevYearClosing carries year N closing into year N+1 opening month 1.
		year.ClosingBalance = cashSection.Total
		for m := 0; m < 12; m++ {
			if m == 0 {
				year.OpeningBalance[m] = prevYearClosing
			} else {
				year.OpeningBalance[m] = cashSection.Total[m-1]
			}
			year.NetCashFlow[m] = year.ClosingBalance[m].Sub(year.OpeningBalance[m])
		}
		// Carry closing balance of this year forward to next year's opening month.
		prevYearClosing = cashSection.Total[11]
	}

	// Schedules (can remain nil for now)
	result.Schedules = model.CashSchedules{}

	return result
}

// buildOverrideMap creates a map for fast lookup of overrides by line + year + month
func buildOverrideMap(overrides []model.CashMonthlyOverride) map[model.CashLineID]map[int]map[int]decimal.Decimal {
	m := make(map[model.CashLineID]map[int]map[int]decimal.Decimal)
	for _, o := range overrides {
		if _, ok := m[o.LineID]; !ok {
			m[o.LineID] = make(map[int]map[int]decimal.Decimal)
		}
		if _, ok := m[o.LineID][o.YearIndex]; !ok {
			m[o.LineID][o.YearIndex] = make(map[int]decimal.Decimal)
		}
		m[o.LineID][o.YearIndex][o.Month] = o.Amount
	}
	return m
}

// getOverride retrieves an override amount, or returns zero if not found
func getOverride(overrideMap map[model.CashLineID]map[int]map[int]decimal.Decimal, lineID model.CashLineID, yearIdx int, month int) decimal.Decimal {
	if yearMap, ok := overrideMap[lineID]; ok {
		if monthMap, ok := yearMap[yearIdx]; ok {
			if amount, ok := monthMap[month]; ok {
				return amount
			}
		}
	}
	return decimal.NewFromInt(-1) // sentinel: -1 means no override
}

// distributeAnnual distributes an annual value across 12 months using the specified rule.
// Returns the monthly distribution (or nil if overridden entirely).
func distributeAnnual(annual decimal.Decimal, rule model.DistributionRule) [12]decimal.Decimal {
	var monthly [12]decimal.Decimal
	switch rule {
	case model.DistEvenSpread:
		monthlyAmount := SafeDiv(annual, decimal.NewFromInt(12))
		for i := 0; i < 12; i++ {
			monthly[i] = monthlyAmount
		}
	case model.DistLumpM1:
		monthly[0] = annual
		for i := 1; i < 12; i++ {
			monthly[i] = decimal.Zero
		}
	default:
		// Default to even spread
		monthlyAmount := SafeDiv(annual, decimal.NewFromInt(12))
		for i := 0; i < 12; i++ {
			monthly[i] = monthlyAmount
		}
	}
	return monthly
}

// applyOverridesAndComputeTotal applies overrides to monthly values and computes annual sum
func applyOverridesAndComputeTotal(monthly [12]decimal.Decimal, overrideMap map[model.CashLineID]map[int]map[int]decimal.Decimal, lineID model.CashLineID, yearIdx int) ([12]decimal.Decimal, decimal.Decimal) {
	result := monthly
	for month := 1; month <= 12; month++ {
		override := getOverride(overrideMap, lineID, yearIdx, month)
		if override.GreaterThanOrEqual(decimal.Zero) { // not sentinel -1
			result[month-1] = override
		}
	}
	// Compute annual as sum of monthly
	annual := decimal.Zero
	for i := 0; i < 12; i++ {
		annual = annual.Add(result[i])
	}
	return result, annual
}

// ─────────────────────────────────────────────────────────────────────────
// SECTION COMPUTATIONS
// ─────────────────────────────────────────────────────────────────────────

// computeRevenueSection builds the Revenue section from ConsolidatedRevenue
func computeRevenueSection(section *model.CashSection, revenue model.ConsolidatedRevenue, yearIdx int, overrideMap map[model.CashLineID]map[int]map[int]decimal.Decimal) {
	var lines []model.CashLine

	// Sales Revenue line (even_spread by default)
	if yearIdx < len(revenue.Totals) {
		annualSales := revenue.Totals[yearIdx].TotalTurnover
		monthly := distributeAnnual(annualSales, model.DistEvenSpread)
		monthly, annual := applyOverridesAndComputeTotal(monthly, overrideMap, model.CashOtherRevenues, yearIdx)

		lines = append(lines, model.CashLine{
			LineID:           model.CashOtherRevenues,
			Label:            "Sales Revenue",
			Monthly:          monthly,
			Annual:           annual,
			DistributionRule: model.DistEvenSpread,
		})
	}

	section.Lines = lines
	computeSectionTotal(section)
}

// computeOperatingSection builds the Operating section from OpexSummary and StaffPayrollSummary
func computeOperatingSection(section *model.CashSection, opex model.OpexSummary, staff model.StaffPayrollSummary, yearIdx int, overrideMap map[model.CashLineID]map[int]map[int]decimal.Decimal) {
	var lines []model.CashLine

	// Map opex subcategories to CashLineIDs
	opexLineMap := map[string]model.CashLineID{
		"premises":     model.CashOpexRentTelecom,
		"leasing":      model.CashOpexLeasing,
		"professional": model.CashOpexFees,
		"royalties":    model.CashOpexRoyalties,
		"travel":       model.CashOpexTravel,
		"marketing":    model.CashOpexAdvertising,
		"hr":           model.CashOpexOther,
	}

	// Add opex subcategories
	for _, sub := range opex.Subcategories {
		if yearIdx < len(sub.Subtotal) {
			lineID := opexLineMap[string(sub.Subcategory)]
			annualAmount := sub.Subtotal[yearIdx]
			monthly := distributeAnnual(annualAmount, model.DistEvenSpread)
			monthly, annual := applyOverridesAndComputeTotal(monthly, overrideMap, lineID, yearIdx)

			lines = append(lines, model.CashLine{
				LineID:           lineID,
				Label:            string(sub.Subcategory),
				Monthly:          monthly,
				Annual:           annual,
				DistributionRule: model.DistEvenSpread,
			})
		}
	}

	// Add payroll (even spread)
	if yearIdx < len(staff.Payroll) {
		totalPayroll := staff.Payroll[yearIdx].TotalPayroll
		monthly := distributeAnnual(totalPayroll, model.DistEvenSpread)
		monthly, annual := applyOverridesAndComputeTotal(monthly, overrideMap, model.CashHeadcountSub, yearIdx)

		lines = append(lines, model.CashLine{
			LineID:           model.CashHeadcountSub,
			Label:            "Payroll",
			Monthly:          monthly,
			Annual:           annual,
			DistributionRule: model.DistEvenSpread,
		})
	}

	section.Lines = lines
	computeSectionTotal(section)
}

// computeCapexSection builds the Capex section from CapexSummary
func computeCapexSection(section *model.CashSection, capex model.CapexSummary, yearIdx int, overrideMap map[model.CashLineID]map[int]map[int]decimal.Decimal) {
	var lines []model.CashLine

	// Map asset categories to CashLineIDs (all 14 categories covered)
	capexLineMap := map[model.AssetCategory]model.CashLineID{
		// Tangible – land & buildings
		model.AssetLand:              model.CashCapexLandBuilding,
		model.AssetBuildings:         model.CashCapexLandBuilding,
		// Intangible – patents, R&D, setup
		model.AssetIntangibleBusiness: model.CashCapexPatentsRD,
		model.AssetSetupExpenses:      model.CashCapexPatentsRD,
		model.AssetPatentsTrademarks:  model.CashCapexPatentsRD,
		model.AssetRnDExpenses:        model.CashCapexPatentsRD,
		model.AssetOtherIntangible:    model.CashCapexPatentsRD,
		// Physical prototypes & equipment
		model.AssetPrototypes:         model.CashCapexPrototypes,
		model.AssetEquipmentTools:     model.CashCapexPrototypes,
		// IT, furniture, vehicles & other tangibles
		model.AssetOfficeFurniture:    model.CashCapexITVehicles,
		model.AssetComputerHWSW:       model.CashCapexITVehicles,
		model.AssetVehicles:           model.CashCapexITVehicles,
		model.AssetOtherTangible:      model.CashCapexITVehicles,
		// Financial investments → land/building bucket (closest capital allocation)
		model.AssetFinancial:          model.CashCapexLandBuilding,
	}

	// Add capex categories (lump_m1 by default)
	for _, inv := range capex.Investments {
		if yearIdx < len(inv.Years) {
			lineID := capexLineMap[inv.Category]
			annualAmount := inv.Years[yearIdx]
			monthly := distributeAnnual(annualAmount, model.DistLumpM1)
			monthly, annual := applyOverridesAndComputeTotal(monthly, overrideMap, lineID, yearIdx)

			lines = append(lines, model.CashLine{
				LineID:           lineID,
				Label:            string(inv.Category),
				Monthly:          monthly,
				Annual:           annual,
				DistributionRule: model.DistLumpM1,
			})
		}
	}

	section.Lines = lines
	computeSectionTotal(section)
}

// computeWCRSection builds the WCR (Working Capital Requirement) section
func computeWCRSection(section *model.CashSection, wcr model.WCRReport, yearIdx int, overrideMap map[model.CashLineID]map[int]map[int]decimal.Decimal) {
	var lines []model.CashLine

	// WCR is typically calculated from tranches (collections and payments)
	// For now, simplified: assume negligible or zero monthly impacts
	// In a full implementation, would distribute collections/payments per tranche timing

	section.Lines = lines
	computeSectionTotal(section)
}

// computeTaxSection builds the Tax section from PnlReport and WCRReport
func computeTaxSection(section *model.CashSection, pnl model.PnlReport, wcr model.WCRReport, yearIdx int, overrideMap map[model.CashLineID]map[int]map[int]decimal.Decimal) {
	var lines []model.CashLine

	// Corporate tax line (typically lump_m1 or manual)
	if yearIdx < len(pnl.Years) {
		annualTax := pnl.Years[yearIdx].CorporateTax
		monthly := distributeAnnual(annualTax, model.DistLumpM1)
		monthly, annual := applyOverridesAndComputeTotal(monthly, overrideMap, model.CashCorporateTax, yearIdx)

		lines = append(lines, model.CashLine{
			LineID:           model.CashCorporateTax,
			Label:            "Corporate Tax",
			Monthly:          monthly,
			Annual:           annual,
			DistributionRule: model.DistLumpM1,
		})
	}

	// VAT payments (from VAT schedule if available)
	// For now, simplified: zero VAT
	// In full implementation, would use wcr.Collected and wcr.NetPayable

	section.Lines = lines
	computeSectionTotal(section)
}

// computeEconomicSection builds Economic = Revenue - Operating - Capex - WCR - Tax
func computeEconomicSection(economic *model.CashSection, revenue *model.CashSection, operating *model.CashSection, capex *model.CashSection, wcr *model.CashSection, tax *model.CashSection) {
	// Economic is a subtotal: sum monthly from Revenue - Operating - Capex - WCR - Tax
	var economic12 [12]decimal.Decimal

	for m := 0; m < 12; m++ {
		val := decimal.Zero
		// Add revenue
		for _, line := range revenue.Lines {
			val = val.Add(line.Monthly[m])
		}
		// Subtract operating
		for _, line := range operating.Lines {
			val = val.Sub(line.Monthly[m])
		}
		// Subtract capex
		for _, line := range capex.Lines {
			val = val.Sub(line.Monthly[m])
		}
		// Subtract wcr
		for _, line := range wcr.Lines {
			val = val.Sub(line.Monthly[m])
		}
		// Subtract tax
		for _, line := range tax.Lines {
			val = val.Sub(line.Monthly[m])
		}
		economic12[m] = val
	}

	// Create a single line representing economic subtotal
	annual := decimal.Zero
	for i := 0; i < 12; i++ {
		annual = annual.Add(economic12[i])
	}

	economic.Lines = []model.CashLine{
		{
			LineID:           model.CashCurrentAccount, // economic subtotal; no dedicated cash constant exists
			Label:            "Economic Subtotal",
			Monthly:          economic12,
			Annual:           annual,
			DistributionRule: model.DistFromSchedule,
		},
	}
	economic.Total = economic12
}

// computeFinancingSection builds the Financing section from FiplanReport
func computeFinancingSection(section *model.CashSection, fiplan model.FiplanReport, yearIdx int, overrideMap map[model.CashLineID]map[int]map[int]decimal.Decimal) {
	var lines []model.CashLine

	// Capital increase (lump_m1 typically)
	if yearIdx < len(fiplan.Plan.Resources.CapitalIncrease) {
		annualCapitalIncrease := fiplan.Plan.Resources.CapitalIncrease[yearIdx]
		monthly := distributeAnnual(annualCapitalIncrease, model.DistLumpM1)
		monthly, annual := applyOverridesAndComputeTotal(monthly, overrideMap, model.CashCapitalIncrease, yearIdx)

		lines = append(lines, model.CashLine{
			LineID:           model.CashCapitalIncrease,
			Label:            "Capital Increase",
			Monthly:          monthly,
			Annual:           annual,
			DistributionRule: model.DistLumpM1,
		})
	}

	// LT Loans (lump_m1 typically)
	if yearIdx < len(fiplan.Plan.Resources.LTLoans) {
		annualLoans := fiplan.Plan.Resources.LTLoans[yearIdx]
		monthly := distributeAnnual(annualLoans, model.DistLumpM1)
		monthly, annual := applyOverridesAndComputeTotal(monthly, overrideMap, model.CashLTLoans, yearIdx)

		lines = append(lines, model.CashLine{
			LineID:           model.CashLTLoans,
			Label:            "LT Loans",
			Monthly:          monthly,
			Annual:           annual,
			DistributionRule: model.DistLumpM1,
		})
	}

	// Loan repayment (typically lump_m1 or scheduled)
	if yearIdx < len(fiplan.Plan.Requirements.LoanRepayments) {
		annualRepayment := fiplan.Plan.Requirements.LoanRepayments[yearIdx]
		monthly := distributeAnnual(annualRepayment, model.DistLumpM1)
		monthly, annual := applyOverridesAndComputeTotal(monthly, overrideMap, model.CashLoanRepayment, yearIdx)

		lines = append(lines, model.CashLine{
			LineID:           model.CashLoanRepayment,
			Label:            "Loan Repayment",
			Monthly:          monthly,
			Annual:           annual,
			DistributionRule: model.DistLumpM1,
		})
	}

	// Dividends (lump_m1 typically)
	if yearIdx < len(fiplan.Plan.Requirements.Dividends) {
		annualDividends := fiplan.Plan.Requirements.Dividends[yearIdx]
		monthly := distributeAnnual(annualDividends, model.DistLumpM1)
		monthly, annual := applyOverridesAndComputeTotal(monthly, overrideMap, model.CashDividends, yearIdx)

		lines = append(lines, model.CashLine{
			LineID:           model.CashDividends,
			Label:            "Dividends",
			Monthly:          monthly,
			Annual:           annual,
			DistributionRule: model.DistLumpM1,
		})
	}

	section.Lines = lines
	computeSectionTotal(section)
}

// computeCashSection builds the Cash section as cumulative balance from Economic + Financing.
// openingBalance is the closing cash from the previous year (0 for year 1).
func computeCashSection(section *model.CashSection, economic *model.CashSection, financing *model.CashSection, openingBalance decimal.Decimal) {
	var cashMonthly [12]decimal.Decimal
	runningBalance := openingBalance

	for m := 0; m < 12; m++ {
		// Monthly change: Economic + Financing
		monthlyEconomic := decimal.Zero
		for _, line := range economic.Lines {
			monthlyEconomic = monthlyEconomic.Add(line.Monthly[m])
		}

		monthlyFinancing := decimal.Zero
		for _, line := range financing.Lines {
			monthlyFinancing = monthlyFinancing.Add(line.Monthly[m])
		}

		monthlyChange := monthlyEconomic.Add(monthlyFinancing)
		runningBalance = runningBalance.Add(monthlyChange)
		cashMonthly[m] = runningBalance
	}

	section.Lines = []model.CashLine{
		{
			LineID:           model.CashCurrentAccount,
			Label:            "Cash Balance",
			Monthly:          cashMonthly,
			Annual:           cashMonthly[11],
			DistributionRule: model.DistFromSchedule,
		},
	}
	section.Total = cashMonthly
}

// computeSectionTotal computes the Total[12] array as the sum of all lines' monthly values
func computeSectionTotal(section *model.CashSection) {
	var total [12]decimal.Decimal
	for _, line := range section.Lines {
		for m := 0; m < 12; m++ {
			total[m] = total[m].Add(line.Monthly[m])
		}
	}
	section.Total = total
}
