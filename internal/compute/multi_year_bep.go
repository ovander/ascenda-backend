// Package compute — Multi-Year Break-Even computation.
//
// ComputeMultiYearBEP derives a 5-year break-even picture from the full plan
// output without any database interaction.  It answers two questions:
//
//  1. Annual BEP: for each year, what net-sales revenue is needed to cover
//     that year's fixed costs at the plan's contribution margin?
//     Does the planned revenue exceed it?
//
//  2. Cumulative BEP (payback): in which year (and estimated month) does the
//     running sum of annual EBEs first turn non-negative?
package compute

import (
	"ascenda/internal/model"

	"github.com/shopspring/decimal"
)

// ComputeMultiYearBEP derives the full 5-year break-even report from a plan.
//
// Derivation rules (consistent with ImportFromPlan):
//   - FixedCosts[y]          = Payroll[y].TotalPayroll + Opex.GrandTotal[y]
//   - ContributionMarginPct[y] = Revenue.Totals[y].GrossMarginPct × 100  (ratio → %)
//   - BEPRevenue[y]          = FixedCosts[y] ÷ (MarginPct[y] / 100)
//   - AnnualEBE[y]           = Revenue[y] × GrossMarginPct[y] − FixedCosts[y]
//   - CumulativeEBE[y]       = Σ AnnualEBE[0..y]
func ComputeMultiYearBEP(plan *model.FullPlanOutput) model.MultiYearBEPReport {
	var report model.MultiYearBEPReport

	cumulative := decimal.Zero
	firstAnnualSet := false
	cumulativeBEPSet := false

	for i := 0; i < 5; i++ {
		rev := plan.Revenue.Totals[i]
		payroll := plan.Payroll.Payroll[i].TotalPayroll
		opex := plan.Opex.GrandTotal[i]
		fixedCosts := payroll.Add(opex)

		// GrossMarginPct is stored as a ratio (0–1); convert to a percentage (0–100).
		marginPct := rev.GrossMarginPct.Mul(d100)

		// Annual EBE = Revenue × MarginRatio − FixedCosts
		annualEBE := rev.TotalTurnover.Mul(rev.GrossMarginPct).Sub(fixedCosts)
		prevCumulative := cumulative
		cumulative = cumulative.Add(annualEBE)

		row := model.MultiYearBEPRow{
			Year:                  i + 1,
			FixedCosts:            fixedCosts,
			Revenue:               rev.TotalTurnover,
			ContributionMarginPct: marginPct,
			AnnualEBE:             annualEBE,
			CumulativeEBE:         cumulative,
		}

		// BEP Revenue (undefined when margin ≤ 0)
		if marginPct.GreaterThan(decimal.Zero) {
			bepRev := fixedCosts.Div(marginPct.Div(d100))
			row.BEPRevenue = &bepRev
			row.RevenueAboveBEP = rev.TotalTurnover.GreaterThanOrEqual(bepRev)
		} else {
			row.BEPRevenueUndefined = true
		}

		// First year with positive annual EBE (annual break-even reached)
		if !firstAnnualSet && annualEBE.GreaterThan(decimal.Zero) {
			row.IsFirstAnnualBEP = true
			y := i + 1
			report.FirstProfitableYear = &y
			firstAnnualSet = true
		}

		// Cumulative BEP crossover: running sum first turns ≥ 0
		if !cumulativeBEPSet && cumulative.GreaterThanOrEqual(decimal.Zero) {
			row.IsCumulativeBEPCrossover = true
			y := i + 1
			report.CumulativeBEPYear = &y

			// Interpolate the month within this year at which crossover occurs.
			// If prevCumulative < 0 and annualEBE > 0:
			//   crossover fraction t = |prevCumulative| / annualEBE
			//   estimated month = ceil(t × 12), clamped to [1, 12]
			if i > 0 && prevCumulative.LessThan(decimal.Zero) && annualEBE.GreaterThan(decimal.Zero) {
				t := prevCumulative.Neg().Div(annualEBE)
				rawMonth := t.Mul(decimal.NewFromInt(12)).Ceil().IntPart()
				if rawMonth < 1 {
					rawMonth = 1
				}
				if rawMonth > 12 {
					rawMonth = 12
				}
				m := int(rawMonth)
				report.CumulativeBEPMonth = &m
			} else {
				// Crossed over in year 1 (started positive from month 1)
				m := 1
				report.CumulativeBEPMonth = &m
			}
			cumulativeBEPSet = true
		}

		report.Years[i] = row
	}

	report.TotalCumulativeEBE = cumulative
	return report
}
