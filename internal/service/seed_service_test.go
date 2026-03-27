package service

// Tests for the seed service data definitions.
//
// SeedService itself cannot be unit-tested without a real DB (it holds a
// concrete *repo.RepoBundle), but the plan-definition functions —
// saasDemo(), hardwareDemo(), consultingDemo(), demoPlanDefs() — are pure
// and contain all the business logic worth validating:
//
//   - Each driver product has the correct typed params (validates the
//     type assertions in createDemoProduct will fire).
//   - Units = FTE × workingDays × utilization for consulting products
//     (catches drift between the inline comments and the actual values).
//   - COGS per billable day ≈ employer cost ÷ billable days, within ±€10
//     (validates the cost-model comments and seed numbers are consistent).
//   - WCR, FiPlan, and incentive-rate fields are set on the right plans.

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"ascenda/internal/model"
)

// ── demoPlanDefs ──────────────────────────────────────────────────────────────

func TestDemoPlanDefs_ReturnsThreePlans(t *testing.T) {
	defs := demoPlanDefs()
	assert.Len(t, defs, 3, "expected SaaS, Hardware, and Consulting demo plans")
}

func TestDemoPlanDefs_AllHaveProducts(t *testing.T) {
	for _, def := range demoPlanDefs() {
		assert.NotEmpty(t, def.products,
			"plan %q should have at least one product", def.name)
	}
}

func TestDemoPlanDefs_AllHaveHeadcountsAndSalaries(t *testing.T) {
	for _, def := range demoPlanDefs() {
		assert.NotEmpty(t, def.headcounts,
			"plan %q: headcounts must be defined", def.name)
		assert.NotEmpty(t, def.salaries,
			"plan %q: salaries must be defined", def.name)
	}
}

func TestDemoPlanDefs_HeadcountAndSalaryCategoriesMatch(t *testing.T) {
	for _, def := range demoPlanDefs() {
		for cat := range def.headcounts {
			_, ok := def.salaries[cat]
			assert.True(t, ok,
				"plan %q: category %q has a headcount but no salary row", def.name, cat)
		}
	}
}

func TestDemoPlanDefs_AllHaveCapexAndOpex(t *testing.T) {
	for _, def := range demoPlanDefs() {
		assert.NotEmpty(t, def.capex,
			"plan %q: capex entries must be defined", def.name)
		assert.NotEmpty(t, def.opex,
			"plan %q: opex entries must be defined", def.name)
	}
}

// ── SaaS demo ─────────────────────────────────────────────────────────────────

func TestSaaSDemo_HasSaaSDriverProduct(t *testing.T) {
	def := saasDemo()
	found := false
	for _, pd := range def.products {
		if pd.driverType == model.DriverSaaS {
			found = true
			_, ok := pd.driverParams.(model.SaaSParams)
			assert.True(t, ok,
				"SaaS product %q: driverParams must be SaaSParams", pd.name)
		}
	}
	assert.True(t, found, "saasDemo must contain at least one DriverSaaS product")
}

func TestSaaSDemo_SaaSParamsActiveUsersMatchUnits(t *testing.T) {
	def := saasDemo()
	for _, pd := range def.products {
		if pd.driverType != model.DriverSaaS {
			continue
		}
		sp := pd.driverParams.(model.SaaSParams)
		for i := 0; i < 5; i++ {
			assert.Equal(t, int64(sp.ActiveUsers[i]), pd.units[i],
				"SaaS product %q Y%d: activeUsers should equal units (they drive volume)",
				pd.name, i+1)
		}
	}
}

func TestSaaSDemo_NoWCROverride(t *testing.T) {
	def := saasDemo()
	assert.True(t, def.wcCustomer30Pct.IsZero() && def.wcCustomer60Pct.IsZero(),
		"saasDemo should not override WCR — B2B defaults (DSO 45 d) apply")
}

func TestSaaSDemo_NoFiplanEntries(t *testing.T) {
	def := saasDemo()
	assert.Empty(t, def.fiplanEntries,
		"saasDemo should not seed financing plan entries")
}

func TestSaaSDemo_NoIncentiveRates(t *testing.T) {
	def := saasDemo()
	assert.Empty(t, def.incentiveRates,
		"saasDemo should not define incentive rates")
}

// ── Hardware demo ─────────────────────────────────────────────────────────────

func TestHardwareDemo_HasIndustryDriverProduct(t *testing.T) {
	def := hardwareDemo()
	found := false
	for _, pd := range def.products {
		if pd.driverType == model.DriverIndustry {
			found = true
			_, ok := pd.driverParams.(model.IndustryParams)
			assert.True(t, ok,
				"Industry product %q: driverParams must be IndustryParams", pd.name)
		}
	}
	assert.True(t, found, "hardwareDemo must contain at least one DriverIndustry product")
}

func TestHardwareDemo_NoWCROverride(t *testing.T) {
	def := hardwareDemo()
	assert.True(t, def.wcCustomer30Pct.IsZero() && def.wcCustomer60Pct.IsZero(),
		"hardwareDemo should not override WCR — B2B defaults apply")
}

func TestHardwareDemo_NoFiplanEntries(t *testing.T) {
	def := hardwareDemo()
	assert.Empty(t, def.fiplanEntries,
		"hardwareDemo should not seed financing plan entries")
}

func TestHardwareDemo_NoIncentiveRates(t *testing.T) {
	def := hardwareDemo()
	assert.Empty(t, def.incentiveRates,
		"hardwareDemo should not define incentive rates")
}

// ── Consulting demo — structure ───────────────────────────────────────────────

func TestConsultingDemo_AllProductsHaveConsultingParams(t *testing.T) {
	def := consultingDemo()
	assert.NotEmpty(t, def.products, "consultingDemo must have products")
	for _, pd := range def.products {
		assert.Equal(t, model.DriverConsulting, pd.driverType,
			"consulting product %q must use DriverConsulting", pd.name)
		_, ok := pd.driverParams.(model.ConsultingParams)
		assert.True(t, ok,
			"consulting product %q: driverParams must be ConsultingParams", pd.name)
	}
}

func TestConsultingDemo_WCRIs60DayDSO(t *testing.T) {
	def := consultingDemo()
	assert.True(t, def.wcCustomer30Pct.IsZero(),
		"consulting: customerPct30Days must be 0 (full 60-day DSO)")
	assert.True(t, decimal.NewFromFloat(1).Equal(def.wcCustomer60Pct),
		"consulting: customerPct60Days must be 1 (full 60-day DSO)")
}

func TestConsultingDemo_HasFiplanEntries(t *testing.T) {
	def := consultingDemo()
	assert.NotEmpty(t, def.fiplanEntries,
		"consultingDemo must seed financing plan entries")

	lineIDs := make(map[model.FiplanLineID]bool, len(def.fiplanEntries))
	for _, fe := range def.fiplanEntries {
		lineIDs[fe.lineID] = true
	}
	assert.True(t, lineIDs[model.FiplanCapitalIncrease],
		"financing plan must include a capital-increase line")
	assert.True(t, lineIDs[model.FiplanLTLoans],
		"financing plan must include a long-term-loan line")
	assert.True(t, lineIDs[model.FiplanDividends],
		"financing plan must include a dividend line")
}

func TestConsultingDemo_FiplanCapitalInY1Only(t *testing.T) {
	def := consultingDemo()
	for _, fe := range def.fiplanEntries {
		if fe.lineID != model.FiplanCapitalIncrease {
			continue
		}
		assert.False(t, fe.amounts[0].IsZero(),
			"capital increase must be > 0 in Y1")
		for i := 1; i < 5; i++ {
			assert.True(t, fe.amounts[i].IsZero(),
				"capital increase must be 0 in Y%d (one-off injection)", i+1)
		}
	}
}

func TestConsultingDemo_FiplanDividendsStartFromY3(t *testing.T) {
	def := consultingDemo()
	for _, fe := range def.fiplanEntries {
		if fe.lineID != model.FiplanDividends {
			continue
		}
		assert.True(t, fe.amounts[0].IsZero(), "dividends must be 0 in Y1")
		assert.True(t, fe.amounts[1].IsZero(), "dividends must be 0 in Y2")
		assert.False(t, fe.amounts[2].IsZero(), "dividends must be > 0 from Y3")
	}
}

func TestConsultingDemo_HasIncentiveRates(t *testing.T) {
	def := consultingDemo()
	assert.NotEmpty(t, def.incentiveRates,
		"consultingDemo must define variable-pay rates")

	salesRate, hasSales := def.incentiveRates[model.CategorySalesTeam]
	assert.True(t, hasSales, "sales team must have an incentive rate")
	assert.False(t, salesRate.IsZero(), "sales team incentive rate must be > 0")

	execRate, hasExec := def.incentiveRates[model.CategoryExecutiveTeam]
	assert.True(t, hasExec, "executive team must have an incentive rate")
	assert.True(t, execRate.GreaterThan(salesRate),
		"executive bonus rate (%s) should exceed sales rate (%s)",
		execRate.StringFixed(2), salesRate.StringFixed(2))
}

func TestConsultingDemo_IncentiveRatesForAllHeadcountCategories(t *testing.T) {
	def := consultingDemo()
	for cat := range def.headcounts {
		_, ok := def.incentiveRates[cat]
		assert.True(t, ok,
			"category %q has headcount but no incentive rate", cat)
	}
}

// ── Consulting demo — HR and Finance headcounts ───────────────────────────────

func TestConsultingDemo_HasHRAndFinanceHeadcounts(t *testing.T) {
	def := consultingDemo()

	_, hasHR := def.headcounts[model.CategoryHR]
	assert.True(t, hasHR, "consultingDemo must include HR headcount rows")

	_, hasFin := def.headcounts[model.CategoryFinance]
	assert.True(t, hasFin, "consultingDemo must include Finance headcount rows")
}

func TestConsultingDemo_HRAppearsInY3(t *testing.T) {
	def := consultingDemo()
	hr := def.headcounts[model.CategoryHR]
	assert.True(t, hr[0].IsZero(), "HR Y1 must be 0 (firm too small)")
	assert.True(t, hr[1].IsZero(), "HR Y2 must be 0 (firm too small)")
	assert.False(t, hr[2].IsZero(), "HR Y3 must be > 0 (first hire)")
}

func TestConsultingDemo_FinanceAppearsInY4(t *testing.T) {
	def := consultingDemo()
	fin := def.headcounts[model.CategoryFinance]
	assert.True(t, fin[0].IsZero(), "Finance Y1 must be 0")
	assert.True(t, fin[1].IsZero(), "Finance Y2 must be 0")
	assert.True(t, fin[2].IsZero(), "Finance Y3 must be 0")
	assert.False(t, fin[3].IsZero(), "Finance Y4 must be > 0 (first hire)")
}

// ── Consulting demo — data-consistency checks ─────────────────────────────────

// TestConsultingDemo_BillableDaysMatchUnits verifies that the unit volumes
// stored in the seed data exactly equal FTE × workingDays × utilizationRate.
// Any mismatch between the inline comments and the actual field values is caught here.
func TestConsultingDemo_BillableDaysMatchUnits(t *testing.T) {
	def := consultingDemo()
	for _, pd := range def.products {
		cp := pd.driverParams.(model.ConsultingParams)
		for i := 0; i < 5; i++ {
			wd := decimal.NewFromInt(int64(cp.WorkingDays))
			billable := cp.Headcount[i].Mul(wd).Mul(cp.UtilizationRate[i])
			expected := billable.Truncate(0).IntPart()
			assert.Equal(t, expected, pd.units[i],
				"product %q Y%d: units=%d should equal trunc(FTE×workingDays×utilization)=%d",
				pd.name, i+1, pd.units[i], expected)
		}
	}
}

// TestConsultingDemo_COGSMatchesEmployerCostPerBillableDay verifies that the
// COGS (cost per billable day) stored in the seed data is consistent with the
// formula: (FTE × monthlyGross × 12 × employerCharges) / billableDays.
// A ±€10 tolerance accounts for rounding to whole euros in the seed values.
func TestConsultingDemo_COGSMatchesEmployerCostPerBillableDay(t *testing.T) {
	def := consultingDemo()
	tolerance := decimal.NewFromFloat(10)

	for _, pd := range def.products {
		cp := pd.driverParams.(model.ConsultingParams)
		for i := 0; i < 5; i++ {
			wd := decimal.NewFromInt(int64(cp.WorkingDays))
			billableDays := cp.Headcount[i].Mul(wd).Mul(cp.UtilizationRate[i])
			if billableDays.IsZero() {
				continue
			}
			annualEmployerCost := cp.Headcount[i].
				Mul(cp.MonthlyGross[i]).
				Mul(decimal.NewFromInt(12)).
				Mul(cp.EmployerCharges)
			expectedCOGS := annualEmployerCost.Div(billableDays)
			diff := expectedCOGS.Sub(pd.cogs[i]).Abs()
			assert.True(t, diff.LessThanOrEqual(tolerance),
				"product %q Y%d: COGS/day %s should be ≈ employerCost/billableDay %s (diff %s)",
				pd.name, i+1,
				pd.cogs[i].StringFixed(0),
				expectedCOGS.StringFixed(0),
				diff.StringFixed(0))
		}
	}
}

// TestConsultingDemo_PricesExceedCOGS checks that day-rate billing prices are
// always above the COGS per day — a sanity check on gross margin sign.
func TestConsultingDemo_PricesExceedCOGS(t *testing.T) {
	def := consultingDemo()
	for _, pd := range def.products {
		for i := 0; i < 5; i++ {
			assert.True(t, pd.prices[i].GreaterThan(pd.cogs[i]),
				"product %q Y%d: price %s must exceed COGS %s (positive gross margin)",
				pd.name, i+1, pd.prices[i].StringFixed(0), pd.cogs[i].StringFixed(0))
		}
	}
}

// TestConsultingDemo_PricesIncreaseOverTime checks that billing rates grow
// each year — a basic coherence test for the 5-year projection.
func TestConsultingDemo_PricesIncreaseOverTime(t *testing.T) {
	def := consultingDemo()
	for _, pd := range def.products {
		for i := 1; i < 5; i++ {
			assert.True(t, pd.prices[i].GreaterThanOrEqual(pd.prices[i-1]),
				"product %q: price Y%d (%s) should be ≥ Y%d (%s)",
				pd.name, i+1, pd.prices[i].StringFixed(0),
				i, pd.prices[i-1].StringFixed(0))
		}
	}
}
