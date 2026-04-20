package service

// Unit tests for the ScenarioAnalysisService pure logic functions.
//
// These tests operate entirely on in-memory model structs — no database,
// no mocks, no network.  Each test covers a single logical unit:
//
//   - computeProjections  (revenue5Y, ebitdaPeak, breakEvenMonth, cashMin)
//   - computeViability    (score + classification)
//   - detectRisks         (cash_gap, late_profitability, no_profitability, unrealistic_growth)
//   - identifyDrivers     (hiring, pricing, burn rate, leverage)
//   - extractTrends       (revenue + cash series)

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"ascenda/internal/model"
)

// ──────────────────────────────────────────────────────────────────────────────
// Test helpers
// ──────────────────────────────────────────────────────────────────────────────

func dec(f float64) decimal.Decimal { return decimal.NewFromFloat(f) }

// assertDecEq asserts two decimal.Decimal values are numerically equal.
// Using assert.Equal on decimals compares internal representation (mantissa +
// exponent) which can differ for the same numeric value after arithmetic
// operations — e.g. NewFromFloat(1500) vs. sum of NewFromFloat(100..500).
func assertDecEq(t *testing.T, expected, actual decimal.Decimal, msgAndArgs ...interface{}) {
	t.Helper()
	if !expected.Equal(actual) {
		assert.Fail(t,
			"decimal mismatch",
			"expected %s, got %s — %v", expected.String(), actual.String(), msgAndArgs,
		)
	}
}

// makePnL builds a PnlReport with the given per-year sales, EBITDA, and net
// profit values.  Indices outside len(sales) are left at zero.
func makePnL(sales, ebitda, netProfit, payroll, cogs, cashFlow [5]float64) model.PnlReport {
	var pnl model.PnlReport
	for i := 0; i < 5; i++ {
		pnl.Years[i] = model.PnlYear{
			Year:            i + 1,
			YearIndex:       i,
			Sales:           dec(sales[i]),
			EBITDA:          dec(ebitda[i]),
			NetProfit:       dec(netProfit[i]),
			PayrollExpenses: dec(payroll[i]),
			COGS:            dec(cogs[i]),
			CashFlow:        dec(cashFlow[i]),
		}
	}
	return pnl
}

// makeCash builds a CashReport with the given monthly closing balances for up
// to 3 years.  balances must have length ≤ 36.
func makeCash(balances []float64) model.CashReport {
	var cash model.CashReport
	for i, b := range balances {
		yr := i / 12
		mo := i % 12
		if yr < 3 {
			cash.Years[yr].ClosingBalance[mo] = dec(b)
		}
	}
	return cash
}

// makeCashWithEconomic builds a CashReport where Economic.Total[m] is set from
// the provided monthly slice (up to 36 entries) and ClosingBalance is derived
// as the running cumulative sum.  This mirrors what the compute engine produces
// for the economic cash section and is the correct data source for
// computeBreakEvenV2 phase 1.
func makeCashWithEconomic(monthly []float64) model.CashReport {
	var cash model.CashReport
	runningSum := 0.0
	for i, v := range monthly {
		if i >= 36 {
			break
		}
		yr := i / 12
		mo := i % 12
		cash.Years[yr].Economic.Total[mo] = dec(v)
		runningSum += v
		cash.Years[yr].ClosingBalance[mo] = dec(runningSum)
	}
	return cash
}

// makeOutput is a convenience wrapper for tests that need a full FullPlanOutput.
func makeOutput(pnl model.PnlReport, cash model.CashReport) *model.FullPlanOutput {
	return &model.FullPlanOutput{PnL: pnl, Cash: cash}
}

// ──────────────────────────────────────────────────────────────────────────────
// computeRevenue5Y
// ──────────────────────────────────────────────────────────────────────────────

func TestComputeRevenue5Y_SumsAllYears(t *testing.T) {
	pnl := makePnL(
		[5]float64{100, 200, 300, 400, 500},
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
	)
	assertDecEq(t, dec(1500), computeRevenue5Y(pnl))
}

func TestComputeRevenue5Y_AllZero(t *testing.T) {
	pnl := makePnL(
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
	)
	assert.True(t, computeRevenue5Y(pnl).IsZero())
}

// ──────────────────────────────────────────────────────────────────────────────
// computeEbitdaPeak
// ──────────────────────────────────────────────────────────────────────────────

func TestComputeEbitdaPeak_ReturnsMax(t *testing.T) {
	pnl := makePnL(
		[5]float64{},
		[5]float64{-100, -50, 0, 80, 120},
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
	)
	assertDecEq(t, dec(120), computeEbitdaPeak(pnl))
}

func TestComputeEbitdaPeak_AllNegative(t *testing.T) {
	pnl := makePnL(
		[5]float64{},
		[5]float64{-200, -150, -100, -50, -10},
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
	)
	assertDecEq(t, dec(-10), computeEbitdaPeak(pnl))
}

// ──────────────────────────────────────────────────────────────────────────────
// computeCashMin
// ──────────────────────────────────────────────────────────────────────────────

func TestComputeCashMin_FindsLowest(t *testing.T) {
	// Plant a negative spike at month 15 (year 2, month 3)
	balances := make([]float64, 36)
	for i := range balances {
		balances[i] = 100
	}
	balances[14] = -250 // month 15 (0-indexed 14)
	cash := makeCash(balances)
	assertDecEq(t, dec(-250), computeCashMin(cash))
}

func TestComputeCashMin_AlwaysPositive(t *testing.T) {
	balances := make([]float64, 36)
	for i := range balances {
		balances[i] = float64(i + 10)
	}
	cash := makeCash(balances)
	assert.True(t, computeCashMin(cash).GreaterThan(decimal.Zero))
}

// ──────────────────────────────────────────────────────────────────────────────
// computeBreakEvenV2 — Phase 1: economic cash (months 1–36)
// ──────────────────────────────────────────────────────────────────────────────

func TestBreakEven_ReachedViaEconomicCash(t *testing.T) {
	// Months 1–12: -100 each → cumulative = -1200 at month 12.
	// Month 13: +2000 → cumulative = -1200 + 2000 = 800 > 0 → BEP at month 13.
	monthly := make([]float64, 36)
	for i := range monthly {
		monthly[i] = -100
	}
	monthly[12] = 2000 // month 13 (0-indexed 12)

	cash := makeCashWithEconomic(monthly)
	pnl := makePnL([5]float64{}, [5]float64{}, [5]float64{}, [5]float64{}, [5]float64{}, [5]float64{})

	m, status, method := computeBreakEvenV2(pnl, cash)

	assert.Equal(t, 13, m)
	assert.Equal(t, BEPReached, status)
	assert.Equal(t, "economic_cash", method)
}

// ──────────────────────────────────────────────────────────────────────────────
// computeBreakEvenV2 — Phase 2: PnL approximation (months 37–60)
// ──────────────────────────────────────────────────────────────────────────────

func TestBreakEven_ReachedViaPnLApprox_Months37to60(t *testing.T) {
	// Months 1–36: -100 each → cumulative at month 36 = -3600.
	// PnL year 4 (index 3) CashFlow = +12000 → monthly = +1000.
	// Month 37: -3600 + 1000 = -2600; month 40: -3600 + 4000 = 400 > 0 → BEP at month 40.
	monthly := make([]float64, 36)
	for i := range monthly {
		monthly[i] = -100
	}
	cash := makeCashWithEconomic(monthly)

	// Zero CashFlow for years 4-5 except year 4 (index 3) = +12000.
	// slope of last 12 = 0 (all flat), so phase 3 does not trigger.
	pnl := makePnL(
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{0, 0, 0, 12000, 0}, // cashFlow
	)

	m, status, method := computeBreakEvenV2(pnl, cash)

	assert.Equal(t, 40, m)
	assert.Equal(t, BEPReached, status)
	assert.Equal(t, "pnl_approx", method)
}

// ──────────────────────────────────────────────────────────────────────────────
// computeBreakEvenV2 — Phase 3: approaching detection
// ──────────────────────────────────────────────────────────────────────────────

func TestBreakEven_Approaching_ConvergingNegative(t *testing.T) {
	// Months 1–24: -500 each → deep negative.
	// Months 25–36: -550, -450, -350, -250, -150, -50, 50, 150, 250, 350, 450, 550
	//   → slope = 100 (verified: mean=0, cov = 100*143 = 14300, slope = 14300/143 = 100).
	// cumulativeAt36 = 24*(-500) + 0 = -12000.
	// Revenue5Y = 60 000 → avgMonthlyRevenue = 1000.
	// Guard: slope(100) >= 0.01*1000(10) ✓ AND slope(100) >= 100 ✓ → approaching.
	monthly := make([]float64, 36)
	for i := 0; i < 24; i++ {
		monthly[i] = -500
	}
	// months 25–36 (indices 24–35): increasing by 100 each step.
	for i := 0; i < 12; i++ {
		monthly[24+i] = -550 + float64(i)*100
	}

	cash := makeCashWithEconomic(monthly)
	// PnL years 4–5 CashFlow = 0 so phase 2 leaves cumulative negative.
	pnl := makePnL(
		[5]float64{12000, 12000, 12000, 12000, 12000}, // revenue → Revenue5Y = 60 000
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{0, 0, 0, 0, 0}, // cashFlow years 4-5 = 0
	)

	_, status, method := computeBreakEvenV2(pnl, cash)

	assert.Equal(t, BEPApproaching, status)
	assert.Equal(t, "projected", method)
}

func TestBreakEven_NotReached_FlatNegative(t *testing.T) {
	// All 36 months: -100 each → slope of last 12 = 0 → fails absolute guard.
	monthly := make([]float64, 36)
	for i := range monthly {
		monthly[i] = -100
	}
	cash := makeCashWithEconomic(monthly)
	pnl := makePnL([5]float64{}, [5]float64{}, [5]float64{}, [5]float64{}, [5]float64{}, [5]float64{})

	m, status, _ := computeBreakEvenV2(pnl, cash)

	assert.Equal(t, 0, m)
	assert.Equal(t, BEPNotReached, status)
}

func TestBreakEven_NotReached_DivergingNegative(t *testing.T) {
	// Months 25–36 get worse each step → negative slope.
	monthly := make([]float64, 36)
	for i := 0; i < 24; i++ {
		monthly[i] = -100
	}
	for i := 0; i < 12; i++ {
		monthly[24+i] = -float64(i+1) * 100 // -100, -200, ..., -1200
	}
	cash := makeCashWithEconomic(monthly)
	pnl := makePnL([5]float64{}, [5]float64{}, [5]float64{}, [5]float64{}, [5]float64{}, [5]float64{})

	m, status, _ := computeBreakEvenV2(pnl, cash)

	assert.Equal(t, 0, m)
	assert.Equal(t, BEPNotReached, status)
}

// ──────────────────────────────────────────────────────────────────────────────
// computeBreakEvenV2 — dual slope guard edge cases
// ──────────────────────────────────────────────────────────────────────────────

func TestBreakEven_SlopeBelowAbsoluteThreshold_IsNotReached(t *testing.T) {
	// last12 = [-275, -225, ..., 275] → slope = 50 (< minAbsoluteBEPSlope=100) → not_reached.
	// Symmetric series: yi = 50*(i-5.5) for i=0..11 →
	//   cov = 50*Σ(i-5.5)² = 50*143 = 7150 → slope = 7150/143 = 50.
	monthly := make([]float64, 36)
	for i := 0; i < 24; i++ {
		monthly[i] = -1000
	}
	for i := 0; i < 12; i++ {
		monthly[24+i] = 50.0 * (float64(i) - 5.5) // -275, -225, ..., 275
	}
	cash := makeCashWithEconomic(monthly)
	// Revenue5Y = 600 000 → avgMonthlyRevenue = 10 000 → proportional floor = 100.
	// slope(50) < 100 → fails absolute guard → not_reached regardless of proportional.
	pnl := makePnL(
		[5]float64{120000, 120000, 120000, 120000, 120000},
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
	)

	m, status, _ := computeBreakEvenV2(pnl, cash)

	assert.Equal(t, 0, m)
	assert.Equal(t, BEPNotReached, status)
}

func TestBreakEven_SlopeMeetsBothGuards_IsApproaching(t *testing.T) {
	// last12 = [-1100, -900, ..., 1100] → slope = 200 (> 100 absolute, > proportional for Revenue5Y=60k).
	// yi = 200*(i-5.5): -1100, -900, -700, -500, -300, -100, 100, 300, 500, 700, 900, 1100
	// cov = 200*143 = 28600 → slope = 200.
	monthly := make([]float64, 36)
	for i := 0; i < 24; i++ {
		monthly[i] = -5000
	}
	for i := 0; i < 12; i++ {
		monthly[24+i] = 200.0 * (float64(i) - 5.5) // -1100..1100
	}
	cash := makeCashWithEconomic(monthly)
	pnl := makePnL(
		[5]float64{12000, 12000, 12000, 12000, 12000}, // Revenue5Y = 60 000
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
	)

	projMonth, status, method := computeBreakEvenV2(pnl, cash)

	assert.Equal(t, BEPApproaching, status)
	assert.Equal(t, "projected", method)
	assert.Greater(t, projMonth, 36) // must be beyond the 36-month economic window
}

// ──────────────────────────────────────────────────────────────────────────────
// computeProjections — FundingRequired / FundingUrgency
// ──────────────────────────────────────────────────────────────────────────────

func TestFundingRequired_NegativeCash(t *testing.T) {
	// Month 18 (year index 1, month index 5) dips to -500.
	balances := make([]float64, 36)
	for i := range balances {
		balances[i] = 10
	}
	balances[17] = -500 // month 18 (0-indexed 17) is the minimum

	cash := makeCash(balances)
	pnl := makePnL([5]float64{}, [5]float64{}, [5]float64{}, [5]float64{}, [5]float64{}, [5]float64{})
	output := makeOutput(pnl, cash)
	p := computeProjections(output)

	assertDecEq(t, dec(500), p.FundingRequired)
	assert.Equal(t, 18, p.FundingMonth)
	assert.Equal(t, fundingUrgencyNearTerm, p.FundingUrgency) // 18 <= 18 → near_term
}

func TestFundingRequired_PositiveCash_IsZero(t *testing.T) {
	balances := make([]float64, 36)
	for i := range balances {
		balances[i] = float64(i + 100)
	}
	cash := makeCash(balances)
	pnl := makePnL([5]float64{}, [5]float64{}, [5]float64{}, [5]float64{}, [5]float64{}, [5]float64{})
	output := makeOutput(pnl, cash)
	p := computeProjections(output)

	assert.True(t, p.FundingRequired.IsZero())
	assert.Equal(t, 0, p.FundingMonth)
	assert.Equal(t, "", p.FundingUrgency)
}

// ──────────────────────────────────────────────────────────────────────────────
// fundingUrgencyFromMonth
// ──────────────────────────────────────────────────────────────────────────────

func TestFundingUrgency_Immediate(t *testing.T) {
	assert.Equal(t, fundingUrgencyImmediate, fundingUrgencyFromMonth(1))
	assert.Equal(t, fundingUrgencyImmediate, fundingUrgencyFromMonth(3))
	assert.Equal(t, fundingUrgencyImmediate, fundingUrgencyFromMonth(6))
}

func TestFundingUrgency_NearTerm(t *testing.T) {
	assert.Equal(t, fundingUrgencyNearTerm, fundingUrgencyFromMonth(7))
	assert.Equal(t, fundingUrgencyNearTerm, fundingUrgencyFromMonth(12))
	assert.Equal(t, fundingUrgencyNearTerm, fundingUrgencyFromMonth(18))
}

func TestFundingUrgency_LongTerm(t *testing.T) {
	assert.Equal(t, fundingUrgencyLongTerm, fundingUrgencyFromMonth(19))
	assert.Equal(t, fundingUrgencyLongTerm, fundingUrgencyFromMonth(24))
	assert.Equal(t, fundingUrgencyLongTerm, fundingUrgencyFromMonth(36))
}

// ──────────────────────────────────────────────────────────────────────────────
// linearSlope12
// ──────────────────────────────────────────────────────────────────────────────

func TestLinearSlope12_PerfectlyLinear(t *testing.T) {
	// y = 50*x → slope should be exactly 50.
	var y [12]float64
	for i := range y {
		y[i] = 50.0 * float64(i)
	}
	slope := linearSlope12(y)
	assert.InDelta(t, 50.0, slope, 0.001)
}

func TestLinearSlope12_Flat(t *testing.T) {
	var y [12]float64
	for i := range y {
		y[i] = -200
	}
	slope := linearSlope12(y)
	assert.InDelta(t, 0.0, slope, 0.001)
}

func TestLinearSlope12_Negative(t *testing.T) {
	// Decreasing series: slope should be negative.
	var y [12]float64
	for i := range y {
		y[i] = float64(11-i) * 100 // 1100, 1000, 900, ...
	}
	slope := linearSlope12(y)
	assert.Less(t, slope, 0.0)
}

// ──────────────────────────────────────────────────────────────────────────────
// computeViability
// ──────────────────────────────────────────────────────────────────────────────

func TestComputeViability_StrongScenario(t *testing.T) {
	// Revenue5Y=500K, CashMin=60K (ratio=+12% ≥ cashHealthCeiling → 30 pts),
	// BEP month 12 (≈20.3 pts), EbitdaMarginY5=30% (clamped 20 pts),
	// MaxYoYGrowthPct=40 (9 pts), positive cash → burnControl 10 pts.
	// Expected total ≈ 89 → strong.
	p := ProjectionResult{
		Revenue5Y:       dec(500_000),
		EbitdaPeak:      dec(125_000),
		BreakEvenMonth:  12,
		CashMin:         dec(60_000),
		EbitdaMarginY5:  dec(0.30),
		MaxYoYGrowthPct: 40.0,
	}
	v := computeViability(p)
	assert.GreaterOrEqual(t, v.Score, viabilityStrong)
	assert.Equal(t, "strong", v.Status)
}

func TestComputeViability_RiskyScenario(t *testing.T) {
	// Worst case: no revenue, cash at absolute floor (−200K), EBITDA never
	// positive, BEP never reached.
	// scoreCashHealth(−200K, 0) = 0  (absolute floor)
	// scoreBreakEven(0) = 0
	// scoreEbitdaMargin(neg peak, 0) = 0
	// scoreRevenueRealism(0, 0) = 0  (zero revenue)
	// scoreBurnControl(−200K, 0) = 0  (absolute floor)
	// Total = 0 → risky.
	p := ProjectionResult{
		Revenue5Y:       dec(0),
		EbitdaPeak:      dec(-50_000),
		BreakEvenMonth:  0,
		CashMin:         dec(-200_000),
		EbitdaMarginY5:  dec(0),
		MaxYoYGrowthPct: 0,
	}
	v := computeViability(p)
	assert.LessOrEqual(t, v.Score, 30)
	assert.Equal(t, "risky", v.Status)
}

func TestComputeViability_ModerateScenario(t *testing.T) {
	// Revenue5Y=800 (below minMeaningfulRevenue → absolute branch), CashMin=−20,
	// BEP=36, EbitdaPeak=50, margin=0, maxYoY=0.
	// scoreCashHealth(−20, 800) abs ≈ 24.0
	// scoreBreakEven(36) ≈ 10.2
	// scoreEbitdaMargin(50, 0) = 10  (peak >0 but margin=0 → baseline only)
	// scoreRevenueRealism(0, 800) = 15
	// scoreBurnControl(−20, 800) abs ≈ 10.0
	// Total ≈ 69 → moderate (≥50 and <80).
	p := ProjectionResult{
		Revenue5Y:       dec(800),
		EbitdaPeak:      dec(50),
		BreakEvenMonth:  36,
		CashMin:         dec(-20),
		EbitdaMarginY5:  dec(0),
		MaxYoYGrowthPct: 0,
	}
	v := computeViability(p)
	assert.GreaterOrEqual(t, v.Score, viabilityModerate)
	assert.Less(t, v.Score, viabilityStrong)
	assert.Equal(t, "moderate", v.Status)
}

func TestClassifyViability_Thresholds(t *testing.T) {
	assert.Equal(t, "strong", classifyViability(80))
	assert.Equal(t, "strong", classifyViability(100))
	assert.Equal(t, "moderate", classifyViability(50))
	assert.Equal(t, "moderate", classifyViability(79))
	assert.Equal(t, "risky", classifyViability(49))
	assert.Equal(t, "risky", classifyViability(0))
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 2 — scoreCashHealth
// ──────────────────────────────────────────────────────────────────────────────

func TestScoreCashHealth_RelativeBranch_Floor(t *testing.T) {
	// ratio = cashMin/rev5Y = −50K/100K = −0.50 (exactly at cashHealthFloor) → 0 pts.
	score := scoreCashHealth(dec(-50_000), dec(100_000))
	assert.InDelta(t, 0.0, score, 0.01)
}

func TestScoreCashHealth_RelativeBranch_Ceiling(t *testing.T) {
	// ratio = 10K/100K = +0.10 (exactly at cashHealthCeiling) → 30 pts.
	score := scoreCashHealth(dec(10_000), dec(100_000))
	assert.InDelta(t, 30.0, score, 0.01)
}

func TestScoreCashHealth_RelativeBranch_AboveCeiling(t *testing.T) {
	// ratio > ceiling — score must be clamped to 30, not exceed it.
	score := scoreCashHealth(dec(100_000), dec(100_000))
	assert.InDelta(t, 30.0, score, 0.01)
}

func TestScoreCashHealth_RelativeBranch_Midpoint(t *testing.T) {
	// ratio = −20K/100K = −0.20 (midpoint between −0.50 and +0.10).
	// score = 30 × (−0.20 − (−0.50)) / (0.10 − (−0.50)) = 30 × 0.30/0.60 = 15.
	score := scoreCashHealth(dec(-20_000), dec(100_000))
	assert.InDelta(t, 15.0, score, 0.01)
}

func TestScoreCashHealth_AbsoluteBranch_LowRevenue(t *testing.T) {
	// Revenue5Y = 5000 < minMeaningfulRevenue → absolute branch.
	// cashMin = −100K → score = 30×(−100K−(−200K))/(50K−(−200K)) = 30×100K/250K = 12.
	score := scoreCashHealth(dec(-100_000), dec(5_000))
	assert.InDelta(t, 12.0, score, 0.01)
}

func TestScoreCashHealth_AbsoluteBranch_Floor(t *testing.T) {
	// cashMin = −200K at absolute floor → 0 pts.
	score := scoreCashHealth(dec(-200_000), dec(0))
	assert.InDelta(t, 0.0, score, 0.01)
}

func TestScoreCashHealth_AbsoluteBranch_Ceiling(t *testing.T) {
	// cashMin = +50K at absolute ceiling → 30 pts.
	score := scoreCashHealth(dec(50_000), dec(0))
	assert.InDelta(t, 30.0, score, 0.01)
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 2 — scoreBreakEven
// ──────────────────────────────────────────────────────────────────────────────

func TestScoreBreakEven_NeverReached(t *testing.T) {
	// bep = 0 (not_reached or approaching with no month) → always 0.
	assert.InDelta(t, 0.0, scoreBreakEven(0), 0.001)
}

func TestScoreBreakEven_Month1(t *testing.T) {
	// bep = 1 (earliest possible) → maximum 25 pts.
	// 25*(60−1)/(60−1) = 25.
	assert.InDelta(t, 25.0, scoreBreakEven(1), 0.001)
}

func TestScoreBreakEven_Month60(t *testing.T) {
	// bep = 60 (last month of horizon) → 0 pts.
	// 25*(60−60)/59 = 0.
	assert.InDelta(t, 0.0, scoreBreakEven(60), 0.001)
}

func TestScoreBreakEven_BeyondHorizon(t *testing.T) {
	// bep > 60 (approaching projection beyond horizon) → clamped to 0.
	assert.InDelta(t, 0.0, scoreBreakEven(90), 0.001)
}

func TestScoreBreakEven_Month30(t *testing.T) {
	// bep = 30 → 25*(60−30)/59 = 25*30/59 ≈ 12.71.
	assert.InDelta(t, 25.0*30.0/59.0, scoreBreakEven(30), 0.01)
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 2 — scoreEbitdaMargin
// ──────────────────────────────────────────────────────────────────────────────

func TestScoreEbitdaMargin_NegativePeak(t *testing.T) {
	// ebitdaPeak ≤ 0 → always 0 regardless of margin.
	assert.InDelta(t, 0.0, scoreEbitdaMargin(dec(-1), dec(0.30)), 0.001)
	assert.InDelta(t, 0.0, scoreEbitdaMargin(dec(0), dec(0.50)), 0.001)
}

func TestScoreEbitdaMargin_ZeroMargin(t *testing.T) {
	// ebitdaPeak > 0, margin = 0 → baseline = 10.
	// score = 10 + 10*0/0.25 = 10.
	assert.InDelta(t, 10.0, scoreEbitdaMargin(dec(1_000), dec(0)), 0.001)
}

func TestScoreEbitdaMargin_AnchorMargin(t *testing.T) {
	// margin = ebitdaMarginAnchor (0.25) → 10 + 10*0.25/0.25 = 20 pts (maximum).
	assert.InDelta(t, 20.0, scoreEbitdaMargin(dec(1_000), dec(0.25)), 0.001)
}

func TestScoreEbitdaMargin_AboveAnchor(t *testing.T) {
	// margin > 0.25 → score would exceed 20, must clamp to 20.
	assert.InDelta(t, 20.0, scoreEbitdaMargin(dec(1_000), dec(0.50)), 0.001)
}

func TestScoreEbitdaMargin_NegativeMargin(t *testing.T) {
	// ebitdaPeak > 0 (e.g. first year positive), but y5 margin = −25%.
	// score = 10 + 10*(−0.25)/0.25 = 10 − 10 = 0.
	assert.InDelta(t, 0.0, scoreEbitdaMargin(dec(1_000), dec(-0.25)), 0.001)
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 2 — scoreBurnControl
// ──────────────────────────────────────────────────────────────────────────────

func TestScoreBurnControl_PositiveCash_Relative(t *testing.T) {
	// cashMin ≥ 0 in relative branch → burnRatio ≤ 0 → clamped to 10.
	assert.InDelta(t, 10.0, scoreBurnControl(dec(5_000), dec(100_000)), 0.001)
}

func TestScoreBurnControl_AtCeiling_Relative(t *testing.T) {
	// burnRatio = 20K/100K = 0.20 (exactly at burnControlCeiling) → 0 pts.
	assert.InDelta(t, 0.0, scoreBurnControl(dec(-20_000), dec(100_000)), 0.001)
}

func TestScoreBurnControl_AbsoluteBranch_PositiveCash(t *testing.T) {
	// cashMin ≥ 0, revenue < minMeaningfulRevenue → 10 pts.
	assert.InDelta(t, 10.0, scoreBurnControl(dec(0), dec(500)), 0.001)
}

func TestScoreBurnControl_AbsoluteBranch_AtFloor(t *testing.T) {
	// cashMin = burnControlAbsFloor (−50K) → 0 pts.
	assert.InDelta(t, 0.0, scoreBurnControl(dec(-50_000), dec(0)), 0.001)
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 2 — continuous distinction (demonstrates v2 > v1)
// ──────────────────────────────────────────────────────────────────────────────

func TestComputeViability_ContinuousDistinction(t *testing.T) {
	// Two scenarios with identical BEP (month 24) and identical EBITDA/growth,
	// but Scenario A has positive cash while Scenario B has deeply negative cash.
	// v2 scoring should produce materially different scores because cash health
	// and burn control both react to cashMin continuously.
	base := ProjectionResult{
		BreakEvenMonth:  24,
		EbitdaPeak:      dec(50_000),
		EbitdaMarginY5:  dec(0.20),
		MaxYoYGrowthPct: 50.0,
		Revenue5Y:       dec(200_000),
	}

	// Scenario A: healthy cash — cashMin well above relative ceiling.
	scenarioA := base
	scenarioA.CashMin = dec(20_000) // ratio=+10% → max cash health + max burn control

	// Scenario B: stressed cash — deeply negative, burnRatio exceeds ceiling.
	scenarioB := base
	scenarioB.CashMin = dec(-80_000) // ratio=−40%, burnRatio=40% → both near floor

	vA := computeViability(scenarioA)
	vB := computeViability(scenarioB)

	// Both share the same BEP so v1 (binary) would treat them identically.
	// v2 must produce a score gap of at least 30 points.
	assert.Greater(t, vA.Score, vB.Score)
	assert.GreaterOrEqual(t, vA.Score-vB.Score, 30,
		"v2 scoring should materially differentiate scenarios with identical BEP but different cash health")
}

// ──────────────────────────────────────────────────────────────────────────────
// detectRisks
// ──────────────────────────────────────────────────────────────────────────────

func TestDetectRisks_CashGap(t *testing.T) {
	pnl := makePnL(
		[5]float64{100, 200, 300, 400, 500},
		[5]float64{50, 80, 100, 120, 150},
		[5]float64{10, 20, 30, 40, 50},
		[5]float64{},
		[5]float64{},
		[5]float64{},
	)
	p := ProjectionResult{
		Revenue5Y:      dec(1500),
		EbitdaPeak:     dec(150),
		BreakEvenMonth: 6,
		CashMin:        dec(-300), // negative cash
	}
	risks := detectRisks(makeOutput(pnl, model.CashReport{}), p)
	types := riskTypes(risks)
	assert.Contains(t, types, "cash_gap")
	cashRisk := findRisk(risks, "cash_gap")
	assert.NotNil(t, cashRisk)
	assert.Equal(t, "high", cashRisk.Severity)
}

func TestDetectRisks_LateProfitability(t *testing.T) {
	pnl := makePnL(
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
	)
	p := ProjectionResult{
		BreakEvenMonth: 36, // beyond 24 months
		CashMin:        dec(10),
		EbitdaPeak:     dec(50),
		Revenue5Y:      dec(500),
	}
	risks := detectRisks(makeOutput(pnl, model.CashReport{}), p)
	r := findRisk(risks, "late_profitability")
	assert.NotNil(t, r)
	assert.Equal(t, "medium", r.Severity)
	assert.Equal(t, 36, *r.When)
}

func TestDetectRisks_NeverProfitable(t *testing.T) {
	pnl := makePnL(
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
	)
	p := ProjectionResult{
		BreakEvenMonth: 0, // never
		CashMin:        dec(-50),
		EbitdaPeak:     dec(-10),
	}
	risks := detectRisks(makeOutput(pnl, model.CashReport{}), p)
	// Expect late_profitability (high) + no_profitability + cash_gap
	assert.Contains(t, riskTypes(risks), "late_profitability")
	lp := findRisk(risks, "late_profitability")
	assert.Equal(t, "high", lp.Severity)
}

func TestDetectRisks_NoProfitability(t *testing.T) {
	pnl := makePnL(
		[5]float64{},
		[5]float64{-100, -80, -60, -40, -20}, // always negative EBITDA
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
	)
	p := ProjectionResult{
		EbitdaPeak:     dec(-20),
		BreakEvenMonth: 0,
		CashMin:        dec(10),
	}
	risks := detectRisks(makeOutput(pnl, model.CashReport{}), p)
	r := findRisk(risks, "no_profitability")
	assert.NotNil(t, r)
	assert.Equal(t, "high", r.Severity)
}

func TestDetectRisks_UnrealisticGrowth(t *testing.T) {
	// Revenue triples each year — growth rate > 100 %.
	// detectRisks reads MaxYoYGrowthPct from the ProjectionResult (Sprint 3
	// eliminates the internal double-call); must be set explicitly.
	pnl := makePnL(
		[5]float64{100, 300, 900, 2700, 8100},
		[5]float64{10, 30, 90, 270, 810},
		[5]float64{5, 15, 45, 135, 405},
		[5]float64{},
		[5]float64{},
		[5]float64{},
	)
	p := ProjectionResult{
		Revenue5Y:       dec(12100),
		EbitdaPeak:      dec(810),
		BreakEvenMonth:  3,
		CashMin:         dec(10),
		MaxYoYGrowthPct: 200.0, // year 1→2: +200 % (computed externally)
	}
	risks := detectRisks(makeOutput(pnl, model.CashReport{}), p)
	r := findRisk(risks, "unrealistic_growth")
	assert.NotNil(t, r)
	assert.Equal(t, "medium", r.Severity)
}

func TestDetectRisks_HealthyScenario_NoRisks(t *testing.T) {
	// Conservative, sustainable growth with positive cash.
	// MaxYoYGrowthPct = 0 (not set) → 0 ≤ 100 so unrealistic_growth not fired.
	pnl := makePnL(
		[5]float64{100, 115, 130, 150, 170},
		[5]float64{10, 20, 30, 40, 50},
		[5]float64{5, 10, 15, 20, 25},
		[5]float64{},
		[5]float64{},
		[5]float64{},
	)
	p := ProjectionResult{
		Revenue5Y:      dec(665),
		EbitdaPeak:     dec(50),
		BreakEvenMonth: 6,
		CashMin:        dec(20),
	}
	risks := detectRisks(makeOutput(pnl, model.CashReport{}), p)
	assert.Empty(t, risks)
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 3 — urgencyFromMonth
// ──────────────────────────────────────────────────────────────────────────────

func TestUrgencyDerivation_Immediate(t *testing.T) {
	assert.Equal(t, fundingUrgencyImmediate, urgencyFromMonth(intPtr(1)))
	assert.Equal(t, fundingUrgencyImmediate, urgencyFromMonth(intPtr(3)))
	assert.Equal(t, fundingUrgencyImmediate, urgencyFromMonth(intPtr(6)))
}

func TestUrgencyDerivation_NearTerm(t *testing.T) {
	assert.Equal(t, fundingUrgencyNearTerm, urgencyFromMonth(intPtr(7)))
	assert.Equal(t, fundingUrgencyNearTerm, urgencyFromMonth(intPtr(12)))
	assert.Equal(t, fundingUrgencyNearTerm, urgencyFromMonth(intPtr(18)))
}

func TestUrgencyDerivation_LongTerm(t *testing.T) {
	assert.Equal(t, fundingUrgencyLongTerm, urgencyFromMonth(intPtr(19)))
	assert.Equal(t, fundingUrgencyLongTerm, urgencyFromMonth(intPtr(60)))
}

func TestUrgencyDerivation_NilWhen(t *testing.T) {
	// nil pointer — no timing information — returns empty string.
	// Callers that require a non-empty urgency assign it explicitly in detectRisks.
	assert.Equal(t, "", urgencyFromMonth(nil))
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 3 — cash_gap risk When population
// ──────────────────────────────────────────────────────────────────────────────

func TestCashGap_HasWhen_Populated(t *testing.T) {
	// CashMinMonth = 10 → cash_gap risk should have When = 10.
	p := ProjectionResult{
		CashMin:      dec(-500),
		CashMinMonth: 10,
	}
	risks := detectRisks(makeOutput(makePnL([5]float64{}, [5]float64{}, [5]float64{}, [5]float64{}, [5]float64{}, [5]float64{}), model.CashReport{}), p)
	r := findRisk(risks, "cash_gap")
	assert.NotNil(t, r)
	assert.NotNil(t, r.When)
	assert.Equal(t, 10, *r.When)
	assert.NotEmpty(t, r.Urgency)
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 3 — high_leverage risk
// ──────────────────────────────────────────────────────────────────────────────

func makeOutputWithLeverage(netDebt5, fcf5 float64) *model.FullPlanOutput {
	var output model.FullPlanOutput
	output.BSheet.Analysis.NetDebt[4] = dec(netDebt5)
	output.Ratios.Profitability.FreeCashFlow[4] = dec(fcf5)
	return &output
}

func TestDetectRisks_HighLeverage_HighSeverity_NegativeFCF(t *testing.T) {
	// NetDebt > 0, FCF ≤ 0 → cannot service debt → high severity.
	output := makeOutputWithLeverage(500_000, -10_000)
	p := ProjectionResult{
		EbitdaPeak: dec(100_000),
		CashMin:    dec(10),
	}
	risks := detectRisks(output, p)
	r := findRisk(risks, "high_leverage")
	assert.NotNil(t, r)
	assert.Equal(t, "high", r.Severity)
	assert.NotEmpty(t, r.Urgency)
}

func TestDetectRisks_HighLeverage_MediumSeverity_RatioExceeded(t *testing.T) {
	// NetDebt = 400K, FCF = 100K → ratio = 4 > highLeverageRatio (3) → medium.
	output := makeOutputWithLeverage(400_000, 100_000)
	p := ProjectionResult{
		EbitdaPeak: dec(100_000),
		CashMin:    dec(10),
	}
	risks := detectRisks(output, p)
	r := findRisk(risks, "high_leverage")
	assert.NotNil(t, r)
	assert.Equal(t, "medium", r.Severity)
}

func TestDetectRisks_HighLeverage_NotEmitted_RatioBelowThreshold(t *testing.T) {
	// NetDebt = 100K, FCF = 100K → ratio = 1 ≤ highLeverageRatio (3) → not emitted.
	output := makeOutputWithLeverage(100_000, 100_000)
	p := ProjectionResult{
		EbitdaPeak: dec(100_000),
		CashMin:    dec(10),
	}
	risks := detectRisks(output, p)
	assert.Nil(t, findRisk(risks, "high_leverage"))
}

func TestDetectRisks_HighLeverage_NotEmitted_NetDebtZero(t *testing.T) {
	// NetDebt = 0 → no leverage risk regardless of FCF.
	output := makeOutputWithLeverage(0, 50_000)
	p := ProjectionResult{
		EbitdaPeak: dec(100_000),
		CashMin:    dec(10),
	}
	risks := detectRisks(output, p)
	assert.Nil(t, findRisk(risks, "high_leverage"))
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 3 — computeRiskSummary
// ──────────────────────────────────────────────────────────────────────────────

func TestRiskSummary_NoRisks_IsNone(t *testing.T) {
	rs := computeRiskSummary([]Risk{})
	assert.Equal(t, riskSummaryNone, rs.GlobalRiskLevel)
	assert.Equal(t, 0, rs.RiskScore)
	assert.Equal(t, 0, rs.RiskCount)
}

func TestRiskSummary_OneLowRisk_IsLow(t *testing.T) {
	rs := computeRiskSummary([]Risk{{Severity: "low"}})
	assert.Equal(t, riskSummaryLow, rs.GlobalRiskLevel)
	assert.Equal(t, 1, rs.RiskScore)
}

func TestRiskSummary_OneMediumRisk_IsLow(t *testing.T) {
	// score = 2 → still "low" (range 1–2)
	rs := computeRiskSummary([]Risk{{Severity: "medium"}})
	assert.Equal(t, riskSummaryLow, rs.GlobalRiskLevel)
	assert.Equal(t, 2, rs.RiskScore)
}

func TestRiskSummary_ThreeMediumRisks_IsElevated(t *testing.T) {
	// score = 3×2 = 6 → "elevated" (range 6–8)
	risks := []Risk{{Severity: "medium"}, {Severity: "medium"}, {Severity: "medium"}}
	rs := computeRiskSummary(risks)
	assert.Equal(t, riskSummaryElevated, rs.GlobalRiskLevel)
	assert.Equal(t, 6, rs.RiskScore)
	assert.Equal(t, 3, rs.RiskCount)
}

func TestRiskSummary_ThreeHighRisks_IsCritical(t *testing.T) {
	// score = 3×3 = 9 → "critical" (≥9)
	risks := []Risk{{Severity: "high"}, {Severity: "high"}, {Severity: "high"}}
	rs := computeRiskSummary(risks)
	assert.Equal(t, riskSummaryCritical, rs.GlobalRiskLevel)
	assert.Equal(t, 9, rs.RiskScore)
}

func TestRiskSummary_MixedRisks_Moderate(t *testing.T) {
	// 1 medium + 1 low = 2+1 = 3 → "moderate" (range 3–5)
	risks := []Risk{{Severity: "medium"}, {Severity: "low"}}
	rs := computeRiskSummary(risks)
	assert.Equal(t, riskSummaryModerate, rs.GlobalRiskLevel)
	assert.Equal(t, 3, rs.RiskScore)
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 3 — viability cap under critical risk
// ──────────────────────────────────────────────────────────────────────────────

func TestViabilityCap_StrongScoreWithCriticalRisk_IsCappedAtModerate(t *testing.T) {
	// A scenario whose raw viability score would be "strong" (≥80) but whose
	// GlobalRiskLevel is "critical" must be capped to ≤60 and classified
	// "moderate" at most.  The cap is applied in AnalyzeScenario after
	// computeRiskSummary — we test it directly on the cap logic here.
	rawScore := 85
	riskSummary := RiskSummary{GlobalRiskLevel: riskSummaryCritical}

	if riskSummary.GlobalRiskLevel == riskSummaryCritical && rawScore > viabilityCriticalCap {
		rawScore = viabilityCriticalCap
	}
	status := classifyViability(rawScore)

	assert.Equal(t, viabilityCriticalCap, rawScore)
	assert.NotEqual(t, "strong", status,
		"strong + critical is a product contradiction — must be prevented by cap")
	assert.Equal(t, "moderate", status)
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 3 — all emitted risks have non-empty Urgency
// ──────────────────────────────────────────────────────────────────────────────

func TestDetectRisks_AllRisksHaveNonEmptyUrgency(t *testing.T) {
	// A scenario that triggers all currently detectable risk types should have
	// non-empty Urgency on every emitted risk.
	output := makeOutputWithLeverage(500_000, -1) // triggers high_leverage
	// Add PnL for no_profitability
	output.PnL = makePnL(
		[5]float64{100, 300, 900, 2700, 8100},
		[5]float64{-50, -40, -30, -20, -10}, // always negative EBITDA
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
	)
	p := ProjectionResult{
		CashMin:         dec(-1_000),
		CashMinMonth:    4,
		BreakEvenMonth:  0, // never reached
		EbitdaPeak:      dec(-10),
		MaxYoYGrowthPct: 200.0, // triggers unrealistic_growth
	}
	risks := detectRisks(output, p)
	assert.NotEmpty(t, risks)
	for _, r := range risks {
		assert.NotEmpty(t, r.Urgency,
			"every risk must have a non-empty Urgency; got empty for type=%s", r.Type)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// identifyDrivers
// ──────────────────────────────────────────────────────────────────────────────

func TestIdentifyDrivers_HiringTooAggressive(t *testing.T) {
	// Payroll = 70 % of sales in year 1
	pnl := makePnL(
		[5]float64{100, 120, 0, 0, 0},
		[5]float64{},
		[5]float64{},
		[5]float64{70, 84, 0, 0, 0}, // payroll
		[5]float64{20, 0, 0, 0, 0},  // COGS → gross margin 80 %
		[5]float64{-30, 0, 0, 0, 0},
	)
	drivers := identifyDrivers(makeOutput(pnl, model.CashReport{}))
	names := driverNames(drivers)
	assert.Contains(t, names, "Hiring too aggressive")
}

func TestIdentifyDrivers_PricingTooLow(t *testing.T) {
	// COGS = 90 % of sales → gross margin 10 %
	pnl := makePnL(
		[5]float64{100, 120, 0, 0, 0},
		[5]float64{},
		[5]float64{},
		[5]float64{20, 0, 0, 0, 0},   // payroll (reasonable)
		[5]float64{90, 108, 0, 0, 0}, // COGS — high
		[5]float64{-10, 0, 0, 0, 0},
	)
	drivers := identifyDrivers(makeOutput(pnl, model.CashReport{}))
	names := driverNames(drivers)
	assert.Contains(t, names, "Pricing too low")
}

func TestIdentifyDrivers_StrongLeverage(t *testing.T) {
	// EBITDA margin 40 % in year 5
	pnl := makePnL(
		[5]float64{100, 200, 400, 700, 1000},
		[5]float64{10, 40, 120, 210, 400}, // year 5 EBITDA margin = 40%
		[5]float64{5, 20, 60, 105, 200},
		[5]float64{20, 40, 80, 140, 200},  // payroll ≤ 20%
		[5]float64{30, 60, 120, 210, 300}, // COGS
		[5]float64{-10, 0, 10, 30, 80},
	)
	drivers := identifyDrivers(makeOutput(pnl, model.CashReport{}))
	names := driverNames(drivers)
	assert.Contains(t, names, "Strong operating leverage")
}

func TestIdentifyDrivers_EfficientScenario_NoDrivers(t *testing.T) {
	// Balanced scenario: payroll ~25 %, margin ~60 %, reasonable burn
	pnl := makePnL(
		[5]float64{100, 120, 150, 180, 220},
		[5]float64{20, 30, 45, 60, 80},
		[5]float64{10, 15, 22, 30, 40},
		[5]float64{25, 30, 37, 45, 55},   // payroll
		[5]float64{40, 48, 60, 72, 88},   // COGS → margin ~60%
		[5]float64{-15, 5, 15, 25, 40},
	)
	drivers := identifyDrivers(makeOutput(pnl, model.CashReport{}))
	// No negative drivers expected; possibly "Strong operating leverage" in year 5
	// (EBITDA margin = 80/220 ≈ 36 % > 25 %)
	for _, d := range drivers {
		assert.NotEqual(t, "Hiring too aggressive", d.Name)
		assert.NotEqual(t, "Pricing too low", d.Name)
		assert.NotEqual(t, "Inefficient growth model", d.Name)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 4 test helpers
// ──────────────────────────────────────────────────────────────────────────────

// makeCashWithOperating builds a CashReport where year-1 Operating.Total is
// set from the provided 12-month slice (all remaining months zero).
func makeCashWithOperating(monthlyOperating [12]float64) model.CashReport {
	var cash model.CashReport
	for m, v := range monthlyOperating {
		cash.Years[0].Operating.Total[m] = dec(v)
	}
	return cash
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 4 — rankDrivers
// ──────────────────────────────────────────────────────────────────────────────

func TestRankDrivers_MaxThree(t *testing.T) {
	// 5 candidates of equal priority — top 3 only should be returned.
	candidates := []candidateDriver{
		{Driver: Driver{Name: "A", RootCause: true}, priority: 2},
		{Driver: Driver{Name: "B", RootCause: true}, priority: 2},
		{Driver: Driver{Name: "C", RootCause: true}, priority: 2},
		{Driver: Driver{Name: "D", RootCause: true}, priority: 2},
		{Driver: Driver{Name: "E", RootCause: true}, priority: 2},
	}
	result := rankDrivers(candidates)
	assert.Len(t, result, 3)
}

func TestRankDrivers_RootCauseBeforeSymptomOnTie(t *testing.T) {
	// Two candidates with the same priority — root cause must precede symptom.
	candidates := []candidateDriver{
		{Driver: Driver{Name: "Symptom", RootCause: false}, priority: 2},
		{Driver: Driver{Name: "RootCause", RootCause: true}, priority: 2},
	}
	result := rankDrivers(candidates)
	assert.Len(t, result, 2)
	assert.Equal(t, "RootCause", result[0].Name)
	assert.Equal(t, "Symptom", result[1].Name)
}

func TestRankDrivers_HigherPriorityFirst(t *testing.T) {
	// Higher priority must precede lower, regardless of root cause.
	candidates := []candidateDriver{
		{Driver: Driver{Name: "Low", RootCause: true}, priority: 1},
		{Driver: Driver{Name: "High", RootCause: false}, priority: 3},
	}
	result := rankDrivers(candidates)
	assert.Equal(t, "High", result[0].Name)
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 4 — hiring driver trajectory
// ──────────────────────────────────────────────────────────────────────────────

func TestHiringDriver_WorseningTrajectory_HighImpact(t *testing.T) {
	// Year 1: payroll/sales = 60 % (above threshold).
	// Year 3: payroll/sales = 70 % (worsening) → impact = "high".
	pnl := makePnL(
		[5]float64{100, 110, 120, 0, 0},
		[5]float64{},
		[5]float64{},
		[5]float64{60, 66, 84, 0, 0}, // payroll: 60%/year1, 70%/year3
		[5]float64{20, 22, 24, 0, 0},
		[5]float64{-40, 0, 0, 0, 0},
	)
	drivers := identifyDrivers(makeOutput(pnl, model.CashReport{}))
	d := findDriver(drivers, "Hiring too aggressive")
	assert.NotNil(t, d)
	assert.Equal(t, "high", d.Impact)
	assert.True(t, d.RootCause)
}

func TestHiringDriver_ImprovingTrajectory_MediumImpact(t *testing.T) {
	// Year 1: payroll/sales = 60 % (above threshold).
	// Year 3: payroll/sales = 45 % (improving) → impact = "medium".
	pnl := makePnL(
		[5]float64{100, 120, 200, 0, 0},
		[5]float64{},
		[5]float64{},
		[5]float64{60, 66, 90, 0, 0}, // payroll: 60%/year1, 45%/year3
		[5]float64{20, 22, 30, 0, 0},
		[5]float64{-40, 0, 0, 0, 0},
	)
	drivers := identifyDrivers(makeOutput(pnl, model.CashReport{}))
	d := findDriver(drivers, "Hiring too aggressive")
	assert.NotNil(t, d)
	assert.Equal(t, "medium", d.Impact)
	assert.True(t, d.RootCause)
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 4 — cash-contribution driver
// ──────────────────────────────────────────────────────────────────────────────

func TestCashContributionDriver_HighPayrollFraction_HiringNotEmitted(t *testing.T) {
	// All values in the same currency units as makePnL (integers ~100 scale).
	// Year 1: payroll=80, sales=200 → hiringRatio=40% ≤ 50% → hiring NOT emitted.
	// Operating: 12 months × (−10) → operatingOut=120.
	// payroll/operatingOut = 80/120 ≈ 66.7% > 60% → cash-contribution fires.
	pnl := makePnL(
		[5]float64{200, 220, 0, 0, 0},
		[5]float64{},
		[5]float64{},
		[5]float64{80, 88, 0, 0, 0}, // payroll = 80 → 40% of sales
		[5]float64{40, 44, 0, 0, 0},
		[5]float64{-60, 0, 0, 0, 0},
	)
	var operating [12]float64
	for i := range operating {
		operating[i] = -10 // 12 months × −10 → operatingOut=120
	}
	cash := makeCashWithOperating(operating)
	drivers := identifyDrivers(makeOutput(pnl, cash))

	names := driverNames(drivers)
	assert.Contains(t, names, "Cash contribution: payroll dominant")
	assert.NotContains(t, names, "Hiring too aggressive")

	d := findDriver(drivers, "Cash contribution: payroll dominant")
	assert.NotNil(t, d)
	assert.True(t, d.RootCause)
}

func TestCashContributionDriver_Suppressed_WhenHiringAlreadyEmitted(t *testing.T) {
	// Year 1: payroll=80, sales=100 → hiringRatio=80% > 50% → hiring IS emitted.
	// Same operating values: payroll fraction would be > 60 %, but cash-contribution
	// must be suppressed to avoid duplicating the root-cause signal.
	pnl := makePnL(
		[5]float64{100, 110, 0, 0, 0},
		[5]float64{},
		[5]float64{},
		[5]float64{80, 88, 0, 0, 0}, // payroll → 80% of year-1 sales
		[5]float64{10, 11, 0, 0, 0},
		[5]float64{-70, 0, 0, 0, 0},
	)
	var operating [12]float64
	for i := range operating {
		operating[i] = -10 // operatingOut=120; payroll/operatingOut = 80/120 > 0.60
	}
	cash := makeCashWithOperating(operating)
	drivers := identifyDrivers(makeOutput(pnl, cash))

	names := driverNames(drivers)
	assert.Contains(t, names, "Hiring too aggressive")
	assert.NotContains(t, names, "Cash contribution: payroll dominant",
		"cash-contribution driver must be suppressed when hiring driver already emitted")
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 4 — root cause classification on existing drivers
// ──────────────────────────────────────────────────────────────────────────────

func TestDriver_PricingTooLow_IsRootCause(t *testing.T) {
	pnl := makePnL(
		[5]float64{100, 120, 0, 0, 0},
		[5]float64{},
		[5]float64{},
		[5]float64{20, 0, 0, 0, 0},
		[5]float64{90, 108, 0, 0, 0},
		[5]float64{-10, 0, 0, 0, 0},
	)
	drivers := identifyDrivers(makeOutput(pnl, model.CashReport{}))
	d := findDriver(drivers, "Pricing too low")
	assert.NotNil(t, d)
	assert.True(t, d.RootCause, "pricing too low is a root cause, not a symptom")
}

func TestDriver_InefficientGrowthModel_IsNotRootCause(t *testing.T) {
	// High burn (>50%) + slow growth (<20%)
	pnl := makePnL(
		[5]float64{100, 110, 0, 0, 0}, // growth = 10% < 20%
		[5]float64{},
		[5]float64{},
		[5]float64{20, 22, 0, 0, 0},   // payroll — 20% of sales
		[5]float64{20, 22, 0, 0, 0},   // COGS — margin 60%
		[5]float64{-60, 0, 0, 0, 0},   // cashflow burn = -60% of sales
	)
	drivers := identifyDrivers(makeOutput(pnl, model.CashReport{}))
	d := findDriver(drivers, "Inefficient growth model")
	assert.NotNil(t, d)
	assert.False(t, d.RootCause, "inefficient growth model is a symptom, not a root cause")
}

// ──────────────────────────────────────────────────────────────────────────────
// extractTrends
// ──────────────────────────────────────────────────────────────────────────────

func TestExtractRevenueTrend_Returns5Points(t *testing.T) {
	pnl := makePnL(
		[5]float64{100, 200, 300, 400, 500},
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
	)
	series := extractRevenueTrend(pnl)
	assert.Len(t, series, 5)
	assertDecEq(t, dec(100), series[0])
	assertDecEq(t, dec(500), series[4])
}

func TestExtractCashTrend_Returns36Points(t *testing.T) {
	balances := make([]float64, 36)
	for i := range balances {
		balances[i] = float64(i * 10)
	}
	cash := makeCash(balances)
	series := extractCashTrend(cash)
	assert.Len(t, series, 36)
	assertDecEq(t, dec(0), series[0])
	assertDecEq(t, dec(350), series[35])
}

// ──────────────────────────────────────────────────────────────────────────────
// maxYoYGrowthPct
// ──────────────────────────────────────────────────────────────────────────────

func TestMaxYoYGrowthPct_TripleRevenue(t *testing.T) {
	pnl := makePnL(
		[5]float64{100, 300, 600, 900, 1000},
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
	)
	// Year 1→2: +200 %, year 2→3: +100 %, year 3→4: +50 %, year 4→5: +11 %
	g := maxYoYGrowthPct(pnl)
	assert.InDelta(t, 200.0, g, 0.01)
}

func TestMaxYoYGrowthPct_ZeroBaseYear(t *testing.T) {
	pnl := makePnL(
		[5]float64{0, 100, 200, 300, 400},
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
		[5]float64{},
	)
	// Year 1 base is 0 — skip; year 2→3: +100 %, etc.
	g := maxYoYGrowthPct(pnl)
	assert.InDelta(t, 100.0, g, 0.01)
}

// ──────────────────────────────────────────────────────────────────────────────
// Test helpers (internal to this file)
// ──────────────────────────────────────────────────────────────────────────────

func riskTypes(risks []Risk) []string {
	types := make([]string, len(risks))
	for i, r := range risks {
		types[i] = r.Type
	}
	return types
}

func findRisk(risks []Risk, riskType string) *Risk {
	for i := range risks {
		if risks[i].Type == riskType {
			return &risks[i]
		}
	}
	return nil
}

func driverNames(drivers []Driver) []string {
	names := make([]string, len(drivers))
	for i, d := range drivers {
		names[i] = d.Name
	}
	return names
}

func findDriver(drivers []Driver, name string) *Driver {
	for i := range drivers {
		if drivers[i].Name == name {
			return &drivers[i]
		}
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 5 test helpers
// ──────────────────────────────────────────────────────────────────────────────

// makeCashWithRevenue builds a CashReport where Revenue.Total[m] is set from
// the provided slice (up to 36 entries, 0-indexed left to right).
func makeCashWithRevenue(monthly []float64) model.CashReport {
	var cash model.CashReport
	for i, v := range monthly {
		if i >= 36 {
			break
		}
		cash.Years[i/12].Revenue.Total[i%12] = dec(v)
	}
	return cash
}

// makeCashWithNetCashFlow builds a CashReport where NetCashFlow[m] is set from
// the provided slice (up to 36 entries, 0-indexed).
func makeCashWithNetCashFlow(monthly []float64) model.CashReport {
	var cash model.CashReport
	for i, v := range monthly {
		if i >= 36 {
			break
		}
		cash.Years[i/12].NetCashFlow[i%12] = dec(v)
	}
	return cash
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 5 — computeRevenueCAGR
// ──────────────────────────────────────────────────────────────────────────────

func TestRevenueCAGR_ZeroBase_ReturnsZero(t *testing.T) {
	// Year 1 sales = 0 → base undefined; CAGR must return 0.
	pnl := makePnL(
		[5]float64{0, 100, 200, 300, 500},
		[5]float64{}, [5]float64{}, [5]float64{}, [5]float64{}, [5]float64{},
	)
	assertDecEq(t, dec(0), computeRevenueCAGR(pnl))
}

func TestRevenueCAGR_DoubleEvery4Years(t *testing.T) {
	// Year1=100, Year5=200: CAGR = 2^0.25 - 1 ≈ 18.9 %.
	pnl := makePnL(
		[5]float64{100, 130, 150, 170, 200},
		[5]float64{}, [5]float64{}, [5]float64{}, [5]float64{}, [5]float64{},
	)
	cagr, _ := computeRevenueCAGR(pnl).Float64()
	// 2^0.25 - 1 = 0.18921...
	assert.InDelta(t, 0.1892, cagr, 0.001)
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 5 — computeBurnRateMonthly
// ──────────────────────────────────────────────────────────────────────────────

func TestBurnRate_NoNegativeMonths_ReturnsZero(t *testing.T) {
	// All Economic.Total entries are zero or positive — no burn to average.
	var cash model.CashReport
	for y := 0; y < 3; y++ {
		for m := 0; m < 12; m++ {
			cash.Years[y].Economic.Total[m] = dec(float64(y*100 + m*10))
		}
	}
	assertDecEq(t, dec(0), computeBurnRateMonthly(cash))
}

func TestBurnRate_AllNegative_ReturnsAverage(t *testing.T) {
	// 36 months all at −1000 → average burn = 1000.
	var cash model.CashReport
	for y := 0; y < 3; y++ {
		for m := 0; m < 12; m++ {
			cash.Years[y].Economic.Total[m] = dec(-1000)
		}
	}
	assertDecEq(t, dec(1000), computeBurnRateMonthly(cash))
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 5 — computeRevenueVolatility
// ──────────────────────────────────────────────────────────────────────────────

func TestVolatility_FlatRevenue_ReturnsZero(t *testing.T) {
	// All five years have identical sales — stddev = 0, CV = 0.
	pnl := makePnL(
		[5]float64{1000, 1000, 1000, 1000, 1000},
		[5]float64{}, [5]float64{}, [5]float64{}, [5]float64{}, [5]float64{},
	)
	assertDecEq(t, dec(0), computeRevenueVolatility(pnl))
}

func TestVolatility_HighDispersion_ReturnsHighCV(t *testing.T) {
	// Year 1 is very small vs year 5 → high coefficient of variation.
	pnl := makePnL(
		[5]float64{10, 100, 500, 1000, 2000},
		[5]float64{}, [5]float64{}, [5]float64{}, [5]float64{}, [5]float64{},
	)
	cv, _ := computeRevenueVolatility(pnl).Float64()
	assert.Greater(t, cv, 0.5, "high revenue dispersion should yield CV > 0.5")
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 5 — computeInflectionMonth
// ──────────────────────────────────────────────────────────────────────────────

func TestInflectionMonth_NegativeThenPositive_ReturnsMonth(t *testing.T) {
	// Months 0–5 negative, month 6 positive → inflection at month 7 (1-indexed).
	var cash model.CashReport
	for m := 0; m < 6; m++ {
		cash.Years[0].NetCashFlow[m] = dec(-500)
	}
	cash.Years[0].NetCashFlow[6] = dec(200)
	assert.Equal(t, 7, computeInflectionMonth(cash))
}

func TestInflectionMonth_AlwaysNegative_ReturnsZero(t *testing.T) {
	// 36 months all negative — cash flow never inflects to positive.
	var cash model.CashReport
	for y := 0; y < 3; y++ {
		for m := 0; m < 12; m++ {
			cash.Years[y].NetCashFlow[m] = dec(-100)
		}
	}
	assert.Equal(t, 0, computeInflectionMonth(cash))
}

func TestInflectionMonth_AlwaysPositive_ReturnsZero(t *testing.T) {
	// Cash flow never goes negative — no inflection to detect.
	var cash model.CashReport
	for m := 0; m < 12; m++ {
		cash.Years[0].NetCashFlow[m] = dec(300)
	}
	assert.Equal(t, 0, computeInflectionMonth(cash))
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 5 — computeTimeToScale
// ──────────────────────────────────────────────────────────────────────────────

func TestTimeToScale_MonotonicAcceleration_ReturnsOne(t *testing.T) {
	// Revenue at months 0,1,2,3: 10_000, 10_500, 11_100, 12_000.
	// rate[1]=5.0%, rate[2]=5.7%, rate[3]=8.1%: rate[2]>rate[1] AND rate[3]>rate[2].
	// Both revenue guard (>5000) and variation guard (>2%) pass.
	// Earliest qualifying i=2 → TimeToScale = 2-1 = 1.
	rev := make([]float64, 36)
	rev[0], rev[1], rev[2], rev[3] = 10_000, 10_500, 11_100, 12_000
	for i := 4; i < 36; i++ {
		rev[i] = 12_000
	}
	cash := makeCashWithRevenue(rev)
	assert.Equal(t, 1, computeTimeToScale(cash))
}

func TestTimeToScale_NeverAccelerates_ReturnsZero(t *testing.T) {
	// Arithmetic progression: constant absolute increment → growth rate
	// monotonically decreases.  No two consecutive growth-rate increases ever occur.
	rev := make([]float64, 36)
	for i := range rev {
		rev[i] = float64(10_000 + i*1_000)
	}
	cash := makeCashWithRevenue(rev)
	assert.Equal(t, 0, computeTimeToScale(cash))
}

func TestTimeToScale_RevenueBelowMinThreshold_ReturnsZero(t *testing.T) {
	// Revenue accelerates (growth rate increasing) but all values < minScaleRevenue.
	// Revenue guard prevents a qualifying signal → must return 0.
	rev := make([]float64, 36)
	rev[0], rev[1], rev[2], rev[3] = 100, 105, 111, 120
	for i := 4; i < 36; i++ {
		rev[i] = 120
	}
	cash := makeCashWithRevenue(rev)
	assert.Equal(t, 0, computeTimeToScale(cash))
}

func TestTimeToScale_VariationBelowMinThreshold_ReturnsZero(t *testing.T) {
	// Revenue > 5000 but month-over-month changes < minScaleVariation (2%).
	// Variation guard prevents a qualifying signal → must return 0.
	rev := make([]float64, 36)
	// Sub-1% changes: 6000, 6006, 6013, 6021 — growth rates ≈ 0.1%, clearly < 2%.
	rev[0], rev[1], rev[2], rev[3] = 6_000, 6_006, 6_013, 6_021
	for i := 4; i < 36; i++ {
		rev[i] = 6_021
	}
	cash := makeCashWithRevenue(rev)
	assert.Equal(t, 0, computeTimeToScale(cash))
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 5 — computeTrendSignal
// ──────────────────────────────────────────────────────────────────────────────

func TestTrendSignal_HighVolatility_OverridesImproving(t *testing.T) {
	// Strong CAGR + InflectionMonth present → would otherwise be "improving".
	// RevenueVolatility > 0.50 must override and return "volatile".
	insights := TrendInsights{
		RevenueCAGR:       dec(0.25),  // > cagrImprovingFloor (0.10) → improving signal
		BurnRateMonthly:   dec(5_000),
		RevenueVolatility: dec(0.65),  // > volatilityThreshold (0.50) → volatile override
		InflectionMonth:   12,         // burn-decreasing proxy is true
		TimeToScale:       6,
	}
	assert.Equal(t, "volatile", computeTrendSignal(insights))
}

func TestTrendSignal_ImprovingConditions_ReturnsImproving(t *testing.T) {
	// CAGR > 0.10 AND burn decreasing (InflectionMonth > 0), low volatility.
	insights := TrendInsights{
		RevenueCAGR:       dec(0.20),
		BurnRateMonthly:   dec(3_000),
		RevenueVolatility: dec(0.10),
		InflectionMonth:   10,
	}
	assert.Equal(t, "improving", computeTrendSignal(insights))
}

func TestTrendSignal_NegativeCAGR_ReturnsDeteriorating(t *testing.T) {
	insights := TrendInsights{
		RevenueCAGR:       dec(-0.05), // negative CAGR
		BurnRateMonthly:   dec(0),
		RevenueVolatility: dec(0.10),
		InflectionMonth:   0,
	}
	assert.Equal(t, "deteriorating", computeTrendSignal(insights))
}

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 5 — acceptance: InflectionMonth vs TimeToScale are distinct
// ──────────────────────────────────────────────────────────────────────────────

// ──────────────────────────────────────────────────────────────────────────────
// Sprint 6 — buildHighlights / buildHeadline
// ──────────────────────────────────────────────────────────────────────────────

// makeResult constructs a minimal ScenarioAnalysisResult for headline tests.
func makeResult(status, globalRisk, bepStatus string, bepMonth, score, riskCount int, drivers []Driver, risks []Risk) *ScenarioAnalysisResult {
	return &ScenarioAnalysisResult{
		Viability: ViabilityResult{Score: score, Status: status},
		Projections: ProjectionResult{
			BreakEvenMonth:  bepMonth,
			BreakEvenStatus: bepStatus,
		},
		RiskSummary: RiskSummary{GlobalRiskLevel: globalRisk, RiskCount: riskCount},
		Drivers:     drivers,
		Risks:       risks,
	}
}

func TestHeadline_AllSevenBranches(t *testing.T) {
	// Branch 1: strong + none/low → "Healthy scenario: …"
	r1 := makeResult("strong", "none", BEPReached, 18, 85, 0,
		[]Driver{{Name: "Strong operating leverage", Impact: "high", RootCause: true}},
		nil,
	)
	h1 := buildHeadline(r1)
	assert.Contains(t, h1, "Healthy scenario:")
	assert.Contains(t, h1, "month 18")

	// Branch 2: strong + moderate → "Strong viability (N/100) but …"
	month5 := 5
	r2 := makeResult("strong", "moderate", BEPReached, 10, 82, 1,
		nil,
		[]Risk{{Type: "cash_gap", Severity: "medium", When: &month5, Message: "Cash drops."}},
	)
	h2 := buildHeadline(r2)
	assert.Contains(t, h2, "Strong viability (82/100)")
	assert.Contains(t, h2, "cash_gap")

	// Branch 3: moderate + root cause driver → "[Driver] limits growth: …"
	r3 := makeResult("moderate", "low", BEPReached, 24, 62, 1,
		[]Driver{{Name: "Pricing too low", Impact: "high", RootCause: true, Effect: "margin is low"}},
		[]Risk{{Type: "late_profitability", Severity: "medium", Message: "Late BEP."}},
	)
	h3 := buildHeadline(r3)
	assert.Contains(t, h3, "Pricing too low limits growth")
	assert.Contains(t, h3, "month 24")

	// Branch 4: moderate + no root cause → "Moderate scenario: …"
	r4 := makeResult("moderate", "low", BEPReached, 30, 55, 2,
		[]Driver{{Name: "Inefficient growth model", Impact: "medium", RootCause: false}},
		nil,
	)
	h4 := buildHeadline(r4)
	assert.Contains(t, h4, "Moderate scenario:")
	assert.Contains(t, h4, "risk(s) identified")

	// Branch 5: risky + critical → "Critical: …"
	r5 := makeResult("risky", "critical", BEPNotReached, 0, 25, 4, nil, nil)
	h5 := buildHeadline(r5)
	assert.Contains(t, h5, "Critical:")
	assert.Contains(t, h5, "compounding risks")

	// Branch 6: risky + approaching → "Approaching viability but …"
	r6 := makeResult("risky", "elevated", BEPApproaching, 52, 35, 2, nil, nil)
	h6 := buildHeadline(r6)
	assert.Contains(t, h6, "Approaching viability")
	assert.Contains(t, h6, "month 52")

	// Branch 7: risky + not_reached → "Not fundable …"
	r7 := makeResult("risky", "elevated", BEPNotReached, 0, 28, 3, nil, nil)
	h7 := buildHeadline(r7)
	assert.Contains(t, h7, "Not fundable")

	// All 7 headlines must be distinct.
	all := []string{h1, h2, h3, h4, h5, h6, h7}
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			assert.NotEqual(t, all[i], all[j],
				"branches %d and %d must produce distinct headlines", i+1, j+1)
		}
	}
}

func TestBuildHighlights_MaxTwoStrengths(t *testing.T) {
	// 3 high-impact root-cause drivers — buildHighlights must cap Strengths at 2.
	result := &ScenarioAnalysisResult{
		Viability:   ViabilityResult{Score: 85, Status: "strong"},
		Projections: ProjectionResult{BreakEvenMonth: 12, BreakEvenStatus: BEPReached},
		RiskSummary: RiskSummary{GlobalRiskLevel: "none"},
		Drivers: []Driver{
			{Name: "Pricing too low", Impact: "high", RootCause: true, Effect: "e1"},
			{Name: "Strong operating leverage", Impact: "high", RootCause: true, Effect: "e2"},
			{Name: "Hiring too aggressive", Impact: "high", RootCause: true, Effect: "e3"},
		},
	}
	h := buildHighlights(result)
	assert.LessOrEqual(t, len(h.Strengths), 2, "Strengths must be capped at 2")
}

func TestBuildHighlights_FundingRequiredInWeaknesses_WhenNegativeCash(t *testing.T) {
	// cash_gap risk (from negative cash) should appear in Weaknesses.
	cashMsg := "Cash position drops to -50000 at month 8 — the scenario may require additional financing."
	result := &ScenarioAnalysisResult{
		Viability:   ViabilityResult{Score: 40, Status: "risky"},
		Projections: ProjectionResult{BreakEvenMonth: 0, BreakEvenStatus: BEPNotReached, CashMin: dec(-50_000)},
		RiskSummary: RiskSummary{GlobalRiskLevel: "elevated", RiskCount: 1},
		Risks: []Risk{
			{Type: "cash_gap", Severity: "high", Message: cashMsg},
		},
	}
	h := buildHighlights(result)
	assert.Contains(t, h.Weaknesses, cashMsg)
	assert.Contains(t, h.TopRisks, cashMsg)
}

func TestBuildHighlights_NoRisks_FallbackUsed(t *testing.T) {
	result := &ScenarioAnalysisResult{
		Viability:   ViabilityResult{Score: 75, Status: "strong"},
		Projections: ProjectionResult{BreakEvenMonth: 10, BreakEvenStatus: BEPReached},
		RiskSummary: RiskSummary{GlobalRiskLevel: "none"},
		Risks:       nil, // no risks
	}
	h := buildHighlights(result)
	assert.NotEmpty(t, h.Weaknesses, "Weaknesses must never be empty")
	assert.NotEmpty(t, h.TopRisks, "TopRisks must never be empty")
	assert.Contains(t, h.Weaknesses[0], "No structural risks")
	assert.Contains(t, h.TopRisks[0], "No risks flagged")
}

func TestBuildHighlights_NoDrivers_FallbackUsed(t *testing.T) {
	result := &ScenarioAnalysisResult{
		Viability:   ViabilityResult{Score: 65, Status: "moderate"},
		Projections: ProjectionResult{BreakEvenMonth: 20, BreakEvenStatus: BEPReached},
		RiskSummary: RiskSummary{GlobalRiskLevel: "low"},
		Drivers:     nil, // no drivers
	}
	h := buildHighlights(result)
	assert.NotEmpty(t, h.TopDrivers, "TopDrivers must never be empty")
	assert.Contains(t, h.TopDrivers[0], "No dominant signal")
}

func TestBuildHighlights_NoStrengths_FallbackUsed(t *testing.T) {
	// No high-impact root-cause drivers, negative cash, negative EBITDA peak
	// → all strength sources absent → fallback string must be used.
	result := &ScenarioAnalysisResult{
		Viability:   ViabilityResult{Score: 30, Status: "risky"},
		Projections: ProjectionResult{CashMin: dec(-10_000), EbitdaPeak: dec(-5_000)},
		RiskSummary: RiskSummary{GlobalRiskLevel: "critical"},
		Drivers:     []Driver{{Name: "Inefficient growth model", Impact: "medium", RootCause: false}},
	}
	h := buildHighlights(result)
	assert.NotEmpty(t, h.Strengths, "Strengths must never be empty")
	assert.Contains(t, h.Strengths[0], "No dominant positive signal")
}

func TestBuildHighlights_NeverCallsDecimalArithmetic(t *testing.T) {
	// Structural test: buildHighlights must compile and produce a non-empty
	// Highlights when given a valid ScenarioAnalysisResult.  The function's
	// decimal-arithmetic constraint is enforced by code inspection: only
	// .IsNegative() and .IsPositive() (comparison methods, not arithmetic)
	// are used inside buildHighlights.
	result := &ScenarioAnalysisResult{
		Viability:   ViabilityResult{Score: 60, Status: "moderate"},
		Projections: ProjectionResult{BreakEvenMonth: 18, BreakEvenStatus: BEPReached, CashMin: dec(5_000), EbitdaPeak: dec(20_000)},
		RiskSummary: RiskSummary{GlobalRiskLevel: "low"},
		Drivers:     []Driver{{Name: "Pricing too low", Impact: "high", RootCause: true, Effect: "gross margin is low"}},
		Risks:       []Risk{{Type: "late_profitability", Severity: "medium", Message: "BEP at month 18."}},
	}
	h := buildHighlights(result)
	// All four slices must be non-empty (confirmed by the function contract).
	assert.NotEmpty(t, h.Headline)
	assert.NotEmpty(t, h.Strengths)
	assert.NotEmpty(t, h.Weaknesses)
	assert.NotEmpty(t, h.TopRisks)
	assert.NotEmpty(t, h.TopDrivers)
}

func TestComputeInsights_InflectionDistinctFromTimeToScale(t *testing.T) {
	// NetCashFlow inflects early (month 2) but revenue acceleration starts later
	// (month 9).  This proves the two metrics measure different phenomena.
	var cash model.CashReport

	// NetCashFlow: month 0 negative, months 1+ positive → InflectionMonth = 2.
	cash.Years[0].NetCashFlow[0] = dec(-1_000)
	for m := 1; m < 12; m++ {
		cash.Years[0].NetCashFlow[m] = dec(500)
	}
	for y := 1; y < 3; y++ {
		for m := 0; m < 12; m++ {
			cash.Years[y].NetCashFlow[m] = dec(500)
		}
	}

	// Revenue.Total: flat at 6000 for months 0–9, then 6000, 6200, 6500.
	// rate[10]=(6200-6000)/6000≈3.3%, rate[11]=(6500-6200)/6200≈4.8%.
	// rate[10] > rate[9]=0 AND rate[11] > rate[10], both months above 5000 and >2%.
	// → TimeToScale = 10 - 1 = 9.
	for m := 0; m < 12; m++ {
		cash.Years[0].Revenue.Total[m] = dec(6_000)
	}
	// Overwrite months 10 and 11 (within year 0).
	cash.Years[0].Revenue.Total[10] = dec(6_200)
	cash.Years[0].Revenue.Total[11] = dec(6_500)

	inflection := computeInflectionMonth(cash)
	timeToScale := computeTimeToScale(cash)

	assert.Equal(t, 2, inflection, "NetCashFlow inflects at month 2")
	assert.Equal(t, 9, timeToScale, "Revenue acceleration detected starting at month 9")
	assert.NotEqual(t, inflection, timeToScale,
		"InflectionMonth and TimeToScale measure different phenomena and must differ in this scenario")
}
