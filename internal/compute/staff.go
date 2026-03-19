package compute

import (
	"kerplan/internal/model"

	"github.com/shopspring/decimal"
)

// ComputeStaffPayroll computes the complete Staff sheet output including headcount summary,
// annual payroll per category, and functional breakdown.
func ComputeStaffPayroll(
	headcounts []model.StaffHeadcount,
	salaries []model.StaffSalary,
	incentives []model.StaffIncentive,
	config model.PlanConfig,
) model.StaffPayrollSummary {
	result := model.StaffPayrollSummary{
		SalaryMonthsPerYear:   config.SalaryMonthsPerYear,
		FirstFiscalYearMonths: config.FirstFiscalYearMonths,
		EmployerTaxRate:       config.EmployerTaxRate,
	}

	// Build lookup maps for O(1) access
	// headcountMap[category][yearIndex] = FTE
	headcountMap := make(map[model.StaffCategory]map[int]decimal.Decimal)
	for _, hc := range headcounts {
		if _, ok := headcountMap[hc.Category]; !ok {
			headcountMap[hc.Category] = make(map[int]decimal.Decimal)
		}
		headcountMap[hc.Category][hc.YearIndex] = hc.FTE
	}

	// salaryMap[category][yearIndex] = MonthlyGrossSalary
	salaryMap := make(map[model.StaffCategory]map[int]decimal.Decimal)
	for _, s := range salaries {
		if _, ok := salaryMap[s.Category]; !ok {
			salaryMap[s.Category] = make(map[int]decimal.Decimal)
		}
		salaryMap[s.Category][s.YearIndex] = s.MonthlyGrossSalary
	}

	// incentiveMap[yearIndex] = (IncentivePct, SpecificIncentives)
	incentiveMap := make(map[int]model.StaffIncentive)
	for _, inc := range incentives {
		incentiveMap[inc.YearIndex] = inc
	}

	// Compute headcount summary
	result.Headcount = computeHeadcountSummary(headcountMap, config.PreviousStaff)

	// Compute annual payroll and functional breakdown for each year
	for yearIndex := 1; yearIndex <= 5; yearIndex++ {
		yearPayroll := model.StaffPayrollYear{
			Year:      config.ForecastStart.Year() + yearIndex - 1,
			YearIndex: yearIndex,
		}

		// Months for this year
		var months int
		if yearIndex == 1 {
			months = config.FirstFiscalYearMonths
		} else {
			months = config.SalaryMonthsPerYear
		}
		if months == 0 {
			months = 12
		}

		fyAdj := SafeDiv(decimal.NewFromInt(int64(months)), decimal.NewFromInt(12))

		// Build per-category payroll and accumulate subtotals
		var subtotalPayroll decimal.Decimal
		functionalAmounts := make(map[model.StaffFunction]decimal.Decimal)

		for _, category := range model.AllCategories {
			fte := decimal.Zero
			if fteLookup, ok := headcountMap[category][yearIndex]; ok {
				fte = fteLookup
			}

			monthlySalary := decimal.Zero
			if msLookup, ok := salaryMap[category][yearIndex]; ok {
				monthlySalary = msLookup
			}

			// AnnualPayroll = FTE × MonthlySalary × (1 + EmployerTaxRate) × Months / 12
			annualPayroll := fte.
				Mul(monthlySalary).
				Mul(decimal.NewFromInt(1).Add(config.EmployerTaxRate)).
				Mul(decimal.NewFromInt(int64(months))).
				Div(decimal.NewFromInt(12))

			categoryPayroll := model.StaffCategoryPayroll{
				Category:      category,
				Function:      model.CategoryFunction[category],
				FTE:           fte,
				MonthlySalary: monthlySalary,
				EmployerRate:  config.EmployerTaxRate,
				Months:        months,
				FYAdjustment:  fyAdj,
				AnnualPayroll: annualPayroll,
			}

			yearPayroll.Categories = append(yearPayroll.Categories, categoryPayroll)
			subtotalPayroll = subtotalPayroll.Add(annualPayroll)

			// Accumulate by function for breakdown
			fn := model.CategoryFunction[category]
			functionalAmounts[fn] = functionalAmounts[fn].Add(annualPayroll)
		}

		yearPayroll.SubtotalPayroll = subtotalPayroll

		// Incentive calculations
		incentive, ok := incentiveMap[yearIndex]
		if !ok {
			incentive = model.StaffIncentive{
				IncentivePct:       decimal.Zero,
				SpecificIncentives: decimal.Zero,
			}
		}

		yearPayroll.IncentivePct = incentive.IncentivePct

		// IncentiveAmount = SubtotalPayroll × min(IncentivePct, IncentiveCap)
		cappedIncentivePct := incentive.IncentivePct
		if cappedIncentivePct.GreaterThan(config.IncentiveCap) {
			cappedIncentivePct = config.IncentiveCap
		}
		yearPayroll.IncentiveAmount = subtotalPayroll.Mul(cappedIncentivePct)
		yearPayroll.SpecificIncentives = incentive.SpecificIncentives
		yearPayroll.SubtotalIncentives = yearPayroll.IncentiveAmount.Add(yearPayroll.SpecificIncentives)
		yearPayroll.TotalPayroll = subtotalPayroll.Add(yearPayroll.SubtotalIncentives)

		result.Payroll[yearIndex-1] = yearPayroll

		// Build functional breakdown for this year
		functionalYear := model.StaffFunctionalYear{
			Year:      config.ForecastStart.Year() + yearIndex - 1,
			YearIndex: yearIndex,
			RnD:       functionalAmounts[model.FunctionRnD],
			Production: functionalAmounts[model.FunctionProduction],
			Sales:      functionalAmounts[model.FunctionSales],
			GnA:        functionalAmounts[model.FunctionGnA],
		}

		// For year 3 (index 2), compute percentages of total payroll
		if yearIndex == 3 {
			totalPayroll := yearPayroll.TotalPayroll
			if !totalPayroll.IsZero() {
				rndPct := SafeDiv(functionalAmounts[model.FunctionRnD], totalPayroll)
				prodPct := SafeDiv(functionalAmounts[model.FunctionProduction], totalPayroll)
				salesPct := SafeDiv(functionalAmounts[model.FunctionSales], totalPayroll)
				gnaPct := SafeDiv(functionalAmounts[model.FunctionGnA], totalPayroll)
				functionalYear.RnDPctYear3 = &rndPct
				functionalYear.ProductionPctYear3 = &prodPct
				functionalYear.SalesPctYear3 = &salesPct
				functionalYear.GnAPctYear3 = &gnaPct
			}
		}

		result.FunctionalBreakdown[yearIndex-1] = functionalYear
	}

	return result
}

// computeHeadcountSummary builds the headcount summary section (§2).
func computeHeadcountSummary(
	headcountMap map[model.StaffCategory]map[int]decimal.Decimal,
	previousStaff int,
) model.StaffHeadcountSummary {
	summary := model.StaffHeadcountSummary{
		Categories: make([]model.StaffCategoryHeadcount, 0, len(model.AllCategories)),
	}

	for _, category := range model.AllCategories {
		catHeadcount := model.StaffCategoryHeadcount{
			Category: category,
			Function: model.CategoryFunction[category],
		}

		for yearIndex := 1; yearIndex <= 5; yearIndex++ {
			if fteLookup, ok := headcountMap[category][yearIndex]; ok {
				catHeadcount.FTE[yearIndex-1] = fteLookup
			} else {
				catHeadcount.FTE[yearIndex-1] = decimal.Zero
			}
			summary.TotalStaff[yearIndex-1] = summary.TotalStaff[yearIndex-1].Add(catHeadcount.FTE[yearIndex-1])
		}

		summary.Categories = append(summary.Categories, catHeadcount)
	}

	// Compute recruits: Year 1 = TotalStaff[0] - PreviousStaff, Year N = TotalStaff[N] - TotalStaff[N-1]
	prevTotal := decimal.NewFromInt(int64(previousStaff))
	for yearIndex := 0; yearIndex < 5; yearIndex++ {
		if yearIndex == 0 {
			summary.Recruits[yearIndex] = summary.TotalStaff[yearIndex].Sub(prevTotal)
		} else {
			summary.Recruits[yearIndex] = summary.TotalStaff[yearIndex].Sub(summary.TotalStaff[yearIndex-1])
		}
	}

	return summary
}
