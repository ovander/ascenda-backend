package compute

import (
	"ascenda/internal/model"
	"fmt"

	"github.com/shopspring/decimal"
)

// ValidateOutput performs cross-checks on the full plan output
func ValidateOutput(out model.FullPlanOutput) []model.ValidationWarning {
	warnings := []model.ValidationWarning{}

	// Tolerance for balance sheet rounding errors
	tolerance := ParseDecimal(0.01)

	// Check 1: Balance sheet balance (Assets = Liabilities for each year 0-5)
	for year := 0; year < 6; year++ {
		assets := out.BSheet.Detailed.Assets.TotalAssets[year]
		liabilities := out.BSheet.Detailed.Liabilities.TotalLiabilities[year]

		diff := assets.Sub(liabilities).Abs()
		if diff.GreaterThan(tolerance) {
			warnings = append(warnings, model.ValidationWarning{
				Severity: "warning",
				Message:  fmt.Sprintf("Balance sheet imbalance in year %d: Assets %s vs Liabilities %s (diff: %s)", year, assets.StringFixed(2), liabilities.StringFixed(2), diff.StringFixed(2)),
			})
		}
	}

	// Check 2: Negative cash warnings (from both BSheet and FiPlan)
	for year := 0; year < 6; year++ {
		if out.BSheet.Detailed.Assets.Cash[year].IsNegative() {
			warnings = append(warnings, model.ValidationWarning{
				Severity: "warning",
				Message:  fmt.Sprintf("Negative cash balance in year %d: %s", year, out.BSheet.Detailed.Assets.Cash[year].StringFixed(2)),
			})
		}
	}

	for year := 0; year < 5; year++ {
		if out.FiPlan.Plan.Balance.CumulativeCash[year].IsNegative() {
			warnings = append(warnings, model.ValidationWarning{
				Severity: "warning",
				Message:  fmt.Sprintf("Negative cumulative cash in year %d: %s", year+1, out.FiPlan.Plan.Balance.CumulativeCash[year].StringFixed(2)),
			})
		}
	}

	// Check 3: WCR coherence (negative WCR is informational)
	for year := 0; year < 5; year++ {
		if out.WCR.Summary.BasicWCR[year].IsNegative() {
			warnings = append(warnings, model.ValidationWarning{
				Severity: "info",
				Message:  fmt.Sprintf("Negative basic WCR in year %d: %s (may indicate favorable payment terms)", year+1, out.WCR.Summary.BasicWCR[year].StringFixed(2)),
			})
		}
	}

	// Check 4: Revenue consistency (TotalTurnover should be non-negative)
	for year := 0; year < 5; year++ {
		if out.Revenue.Totals[year].TotalTurnover.IsNegative() {
			warnings = append(warnings, model.ValidationWarning{
				Severity: "error",
				Message:  fmt.Sprintf("Negative revenue (TotalTurnover) in year %d: %s", year+1, out.Revenue.Totals[year].TotalTurnover.StringFixed(2)),
			})
		}
	}

	// Check 5: Gross margin check (should be between 0 and 1)
	for year := 0; year < 5; year++ {
		gmPct := out.Revenue.Totals[year].GrossMarginPct
		if gmPct.GreaterThan(decimal.NewFromInt(1)) || gmPct.LessThan(decimal.Zero) {
			warnings = append(warnings, model.ValidationWarning{
				Severity: "warning",
				Message:  fmt.Sprintf("Unusual gross margin percentage in year %d: %s (should be between 0 and 1)", year+1, gmPct.StringFixed(4)),
			})
		}
	}

	// Check 6: Capex depreciation check (depreciation should not exceed net assets)
	for year := 0; year < 5; year++ {
		if out.Capex.Totals.TotalDepreciation[year].GreaterThan(out.Capex.Totals.NetAssets[year]) {
			warnings = append(warnings, model.ValidationWarning{
				Severity: "info",
				Message:  fmt.Sprintf("Depreciation exceeds net assets in year %d: Depreciation %s vs NetAssets %s", year+1, out.Capex.Totals.TotalDepreciation[year].StringFixed(2), out.Capex.Totals.NetAssets[year].StringFixed(2)),
			})
		}
	}

	// Check 7: EBITDA negative warning
	for year := 0; year < 5; year++ {
		if out.PnL.Years[year].EBITDA.IsNegative() {
			warnings = append(warnings, model.ValidationWarning{
				Severity: "warning",
				Message:  fmt.Sprintf("Negative EBITDA in year %d: %s", year+1, out.PnL.Years[year].EBITDA.StringFixed(2)),
			})
		}
	}

	// Check 8: Equity negative warning
	for year := 0; year < 6; year++ {
		if out.BSheet.Equity[year].IsNegative() {
			warnings = append(warnings, model.ValidationWarning{
				Severity: "warning",
				Message:  fmt.Sprintf("Negative equity in year %d: %s", year, out.BSheet.Equity[year].StringFixed(2)),
			})
		}
	}

	// Check 9: FiPlan warnings
	for year := 0; year < 5; year++ {
		if out.FiPlan.Warning[year] {
			warnings = append(warnings, model.ValidationWarning{
				Severity: "warning",
				Message:  fmt.Sprintf("Financial plan warning flag set for year %d", year+1),
			})
		}
	}

	return warnings
}
