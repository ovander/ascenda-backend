package compute

import (
	"testing"

	"ascenda/internal/model"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// buildPlan constructs a minimal FullPlanOutput for testing.
// payrolls, opexes, turnoverAmounts, and marginRatios are all indexed 0–4 (year 1–5).
// All monetary values are in € (full euros, consistent with production model).
func buildPlan(
	payrolls [5]int64,
	opexes [5]int64,
	turnoverAmounts [5]int64,
	marginRatios [5]float64, // 0–1, e.g. 0.40 for 40%
) *model.FullPlanOutput {
	plan := &model.FullPlanOutput{}
	for i := 0; i < 5; i++ {
		plan.Payroll.Payroll[i].TotalPayroll = decimal.NewFromInt(payrolls[i])
		plan.Opex.GrandTotal[i] = decimal.NewFromInt(opexes[i])
		plan.Revenue.Totals[i].TotalTurnover = decimal.NewFromInt(turnoverAmounts[i])
		plan.Revenue.Totals[i].GrossMarginPct = decimal.NewFromFloat(marginRatios[i])
	}
	return plan
}

// ─────────────────────────────────────────────────────────────────────────────
// §1  Annual BEP — per-year outputs
// ─────────────────────────────────────────────────────────────────────────────

// Reference plan for most tests:
//
//	Year 1: FixedCosts = 600 k€, Revenue = 1 000 k€, Margin = 40 % → BEP = 1 500 k€  → LOSS
//	Year 2: FixedCosts = 700 k€, Revenue = 2 000 k€, Margin = 42 % → BEP = 1 667 k€  → PROFIT
//	Year 3: FixedCosts = 750 k€, Revenue = 2 500 k€, Margin = 44 % → BEP = 1 705 k€  → PROFIT
//	Year 4: FixedCosts = 800 k€, Revenue = 3 000 k€, Margin = 46 % → BEP = 1 739 k€  → PROFIT
//	Year 5: FixedCosts = 850 k€, Revenue = 3 500 k€, Margin = 48 % → BEP = 1 771 k€  → PROFIT
func refPlan() *model.FullPlanOutput {
	return buildPlan(
		[5]int64{400_000, 480_000, 510_000, 540_000, 570_000},           // payroll
		[5]int64{200_000, 220_000, 240_000, 260_000, 280_000},           // opex
		[5]int64{1_000_000, 2_000_000, 2_500_000, 3_000_000, 3_500_000}, // turnover
		[5]float64{0.40, 0.42, 0.44, 0.46, 0.48},                        // margin ratios
	)
}

func TestComputeMultiYearBEP_ReturnsCorrectFixedCosts(t *testing.T) {
	report := ComputeMultiYearBEP(refPlan())

	// Year 1: payroll 400k + opex 200k = 600k
	assert.Equal(t, "600000", report.Years[0].FixedCosts.StringFixed(0))
	// Year 5: payroll 570k + opex 280k = 850k
	assert.Equal(t, "850000", report.Years[4].FixedCosts.StringFixed(0))
}

func TestComputeMultiYearBEP_ContributionMarginPct_IsPercentage(t *testing.T) {
	report := ComputeMultiYearBEP(refPlan())

	// GrossMarginPct 0.40 ratio → ContributionMarginPct 40 %
	assert.Equal(t, "40.00", report.Years[0].ContributionMarginPct.StringFixed(2))
	// Year 5: 0.48 → 48 %
	assert.Equal(t, "48.00", report.Years[4].ContributionMarginPct.StringFixed(2))
}

func TestComputeMultiYearBEP_BEPRevenue_YearOne(t *testing.T) {
	report := ComputeMultiYearBEP(refPlan())

	// Year 1: BEP = 600 000 / 0.40 = 1 500 000
	require.NotNil(t, report.Years[0].BEPRevenue)
	assert.Equal(t, "1500000.00", report.Years[0].BEPRevenue.StringFixed(2))
}

func TestComputeMultiYearBEP_RevenueAboveBEP_CorrectFlags(t *testing.T) {
	report := ComputeMultiYearBEP(refPlan())

	// Year 1: Revenue 1 000 000 < BEP 1 500 000 → below
	assert.False(t, report.Years[0].RevenueAboveBEP, "year 1 should be below BEP")
	// Year 2: Revenue 2 000 000 > BEP ~1 667 000 → above
	assert.True(t, report.Years[1].RevenueAboveBEP, "year 2 should be above BEP")
}

// ─────────────────────────────────────────────────────────────────────────────
// §2  Annual EBE
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeMultiYearBEP_AnnualEBE_YearOne_IsNegative(t *testing.T) {
	report := ComputeMultiYearBEP(refPlan())

	// Year 1 EBE = 1 000 000 × 0.40 − 600 000 = 400 000 − 600 000 = −200 000
	assert.Equal(t, "-200000.00", report.Years[0].AnnualEBE.StringFixed(2))
}

func TestComputeMultiYearBEP_AnnualEBE_YearTwo_IsPositive(t *testing.T) {
	report := ComputeMultiYearBEP(refPlan())

	// Year 2 EBE = 2 000 000 × 0.42 − 700 000 = 840 000 − 700 000 = 140 000
	assert.Equal(t, "140000.00", report.Years[1].AnnualEBE.StringFixed(2))
}

// ─────────────────────────────────────────────────────────────────────────────
// §3  First profitable year
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeMultiYearBEP_FirstProfitableYear_IsYear2(t *testing.T) {
	report := ComputeMultiYearBEP(refPlan())

	require.NotNil(t, report.FirstProfitableYear, "should find a first profitable year")
	assert.Equal(t, 2, *report.FirstProfitableYear)
}

func TestComputeMultiYearBEP_FirstProfitableYear_FlagSetOnRow(t *testing.T) {
	report := ComputeMultiYearBEP(refPlan())

	assert.False(t, report.Years[0].IsFirstAnnualBEP, "year 1 must not be flagged as first BEP")
	assert.True(t, report.Years[1].IsFirstAnnualBEP, "year 2 must be flagged as first annual BEP")
	assert.False(t, report.Years[2].IsFirstAnnualBEP, "year 3 must not be flagged again")
}

func TestComputeMultiYearBEP_NeverProfitable_NilFirstYear(t *testing.T) {
	// All years lose money: margin 20%, turnover 500k, fixed 200k → EBE = -100k/yr
	plan := buildPlan(
		[5]int64{150_000, 150_000, 150_000, 150_000, 150_000},
		[5]int64{50_000, 50_000, 50_000, 50_000, 50_000},
		[5]int64{500_000, 500_000, 500_000, 500_000, 500_000},
		[5]float64{0.20, 0.20, 0.20, 0.20, 0.20},
	)
	report := ComputeMultiYearBEP(plan)

	assert.Nil(t, report.FirstProfitableYear, "no profitable year within 5 years")
	assert.Nil(t, report.CumulativeBEPYear, "no cumulative BEP within 5 years")
	assert.True(t, report.TotalCumulativeEBE.LessThan(decimal.Zero), "total EBE must be negative")
}

// ─────────────────────────────────────────────────────────────────────────────
// §4  Cumulative BEP (payback)
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeMultiYearBEP_CumulativeEBE_Accumulation(t *testing.T) {
	report := ComputeMultiYearBEP(refPlan())

	// Year 1: cumEBE = −200 000
	assert.Equal(t, "-200000.00", report.Years[0].CumulativeEBE.StringFixed(2))
	// Year 2: cumEBE = −200 000 + 140 000 = −60 000
	assert.Equal(t, "-60000.00", report.Years[1].CumulativeEBE.StringFixed(2))
	// Year 3 EBE = 2 500 000 × 0.44 − 750 000 = 1 100 000 − 750 000 = 350 000
	// Year 3 cumEBE = −60 000 + 350 000 = 290 000
	assert.Equal(t, "290000.00", report.Years[2].CumulativeEBE.StringFixed(2))
}

func TestComputeMultiYearBEP_CumulativeBEPYear_IsYear3(t *testing.T) {
	report := ComputeMultiYearBEP(refPlan())

	require.NotNil(t, report.CumulativeBEPYear)
	assert.Equal(t, 3, *report.CumulativeBEPYear, "cumulative BEP should be reached in year 3")
}

func TestComputeMultiYearBEP_CumulativeBEPRow_FlaggedCorrectly(t *testing.T) {
	report := ComputeMultiYearBEP(refPlan())

	assert.False(t, report.Years[0].IsCumulativeBEPCrossover)
	assert.False(t, report.Years[1].IsCumulativeBEPCrossover)
	assert.True(t, report.Years[2].IsCumulativeBEPCrossover, "year 3 row must carry the crossover flag")
	assert.False(t, report.Years[3].IsCumulativeBEPCrossover)
}

func TestComputeMultiYearBEP_CumulativeBEPMonth_Interpolated(t *testing.T) {
	// Year 3 crosses over: prevCumEBE = −60 000, year3EBE = 350 000
	// t = 60 000 / 350 000 ≈ 0.171 → month = ceil(0.171 × 12) = ceil(2.06) = 3
	report := ComputeMultiYearBEP(refPlan())

	require.NotNil(t, report.CumulativeBEPMonth)
	assert.Equal(t, 3, *report.CumulativeBEPMonth, "crossover estimated at month 3 of year 3")
}

// ─────────────────────────────────────────────────────────────────────────────
// §5  Edge cases
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeMultiYearBEP_ZeroMargin_BEPUndefined(t *testing.T) {
	plan := buildPlan(
		[5]int64{100_000, 100_000, 100_000, 100_000, 100_000},
		[5]int64{50_000, 50_000, 50_000, 50_000, 50_000},
		[5]int64{500_000, 500_000, 500_000, 500_000, 500_000},
		[5]float64{0, 0, 0, 0, 0},
	)
	report := ComputeMultiYearBEP(plan)

	for i := 0; i < 5; i++ {
		assert.Nil(t, report.Years[i].BEPRevenue, "BEP must be nil when margin = 0")
		assert.True(t, report.Years[i].BEPRevenueUndefined)
	}
}

func TestComputeMultiYearBEP_ProfitableFromYear1_CumulativeBEPYear1(t *testing.T) {
	// High margin, low fixed costs — profitable from day one
	plan := buildPlan(
		[5]int64{100_000, 110_000, 120_000, 130_000, 140_000},
		[5]int64{50_000, 55_000, 60_000, 65_000, 70_000},
		[5]int64{1_000_000, 1_200_000, 1_400_000, 1_600_000, 1_800_000},
		[5]float64{0.70, 0.70, 0.70, 0.70, 0.70},
	)
	report := ComputeMultiYearBEP(plan)

	// Year 1 EBE = 1 000 000 × 0.70 − 150 000 = 700 000 − 150 000 = 550 000
	assert.True(t, report.Years[0].AnnualEBE.GreaterThan(decimal.Zero))
	require.NotNil(t, report.FirstProfitableYear)
	assert.Equal(t, 1, *report.FirstProfitableYear)

	require.NotNil(t, report.CumulativeBEPYear)
	assert.Equal(t, 1, *report.CumulativeBEPYear)
	require.NotNil(t, report.CumulativeBEPMonth)
	assert.Equal(t, 1, *report.CumulativeBEPMonth)
}

func TestComputeMultiYearBEP_TotalCumulativeEBE_MatchesLastRow(t *testing.T) {
	report := ComputeMultiYearBEP(refPlan())

	lastRow := report.Years[4]
	assert.True(t, report.TotalCumulativeEBE.Equal(lastRow.CumulativeEBE),
		"TotalCumulativeEBE must equal year-5 cumulative EBE")
}

func TestComputeMultiYearBEP_YearLabels_Are1Based(t *testing.T) {
	report := ComputeMultiYearBEP(refPlan())

	for i := 0; i < 5; i++ {
		assert.Equal(t, i+1, report.Years[i].Year)
	}
}
