package compute

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ascenda/internal/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test fixtures (matching EBE+ reference example: ProgiTop SA)
//
//   Fixed costs   = 1 440 000 €   (1 440 k€)
//   Margin %      = 60 %
//   BEP Revenue   = 1 440 000 / 0.60 = 2 400 000 € (2 400 k€)
//   Avg order     = 15 000 €  → BEP Volume = 2 400 000 / 15 000 = 160 orders
// ─────────────────────────────────────────────────────────────────────────────

func progitopFixed() decimal.Decimal    { return decimal.NewFromInt(1_440_000) }
func progitopMargin() decimal.Decimal   { return decimal.NewFromInt(60) }
func progitopAvgOrder() *decimal.Decimal {
	v := decimal.NewFromInt(15_000)
	return &v
}

// ─────────────────────────────────────────────────────────────────────────────
// §1  ComputeBEPCore — primary outputs
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeBEPCore_ProgiTop(t *testing.T) {
	result := ComputeBEPCore(progitopFixed(), progitopMargin(), progitopAvgOrder())

	require.NotNil(t, result.BEPRevenue, "BEP revenue should be defined")
	assert.Equal(t, "2400000", result.BEPRevenue.StringFixed(0), "BEP revenue = 2 400 000 €")

	require.NotNil(t, result.BEPVolume, "BEP volume should be defined")
	assert.Equal(t, "160", result.BEPVolume.StringFixed(0), "BEP volume = 160 orders")

	require.NotNil(t, result.MonthlyBEP, "monthly BEP should be defined")
	assert.Equal(t, "200000.00", result.MonthlyBEP.StringFixed(2), "monthly BEP = 2 400 000 / 12")

	assert.Equal(t, "40", result.VariableCostPct.StringFixed(0), "variable cost % = 100 - 60 = 40%")

	require.NotNil(t, result.VariableCostPerOrder)
	assert.Equal(t, "6000.00", result.VariableCostPerOrder.StringFixed(2), "var cost/order = 15000 × 0.40 = 6 000 €")

	assert.Empty(t, result.Warnings, "no warnings for healthy inputs")
}

func TestComputeBEPCore_ZeroMargin_Undefined(t *testing.T) {
	result := ComputeBEPCore(progitopFixed(), decimal.Zero, nil)

	assert.Nil(t, result.BEPRevenue, "BEP undefined when margin = 0")
	assert.Nil(t, result.BEPVolume, "volume undefined when margin = 0")
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "margin_zero_or_negative", result.Warnings[0].Code)
}

func TestComputeBEPCore_NegativeMargin_Undefined(t *testing.T) {
	result := ComputeBEPCore(progitopFixed(), decimal.NewFromInt(-5), nil)

	assert.Nil(t, result.BEPRevenue)
	assert.Equal(t, "margin_zero_or_negative", result.Warnings[0].Code)
}

func TestComputeBEPCore_LowMarginWarning(t *testing.T) {
	// 5% margin — valid but in the high-sensitivity zone
	result := ComputeBEPCore(decimal.NewFromInt(100_000), decimal.NewFromInt(5), nil)

	require.NotNil(t, result.BEPRevenue, "BEP is defined (margin > 0)")
	assert.Equal(t, "2000000.00", result.BEPRevenue.StringFixed(2))
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "margin_below_10_pct", result.Warnings[0].Code)
}

func TestComputeBEPCore_ZeroFixedCosts(t *testing.T) {
	// BEP = 0 / 0.60 = 0  (trivially met)
	result := ComputeBEPCore(decimal.Zero, progitopMargin(), progitopAvgOrder())

	require.NotNil(t, result.BEPRevenue)
	assert.True(t, result.BEPRevenue.IsZero(), "BEP = 0 when fixed costs = 0")
	assert.Empty(t, result.Warnings)
}

func TestComputeBEPCore_NoAvgOrder_VolumeNil(t *testing.T) {
	result := ComputeBEPCore(progitopFixed(), progitopMargin(), nil)

	assert.NotNil(t, result.BEPRevenue, "BEP revenue still computed")
	assert.Nil(t, result.BEPVolume, "volume undefined without avg order value")
	assert.Nil(t, result.VariableCostPerOrder)
}

// ─────────────────────────────────────────────────────────────────────────────
// §2  ComputeEBE — single-point EBE
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeEBE_AtBEP_IsZero(t *testing.T) {
	// At BEP revenue, EBE must equal zero by definition
	bep := decimal.NewFromInt(2_400_000)
	ebe := ComputeEBE(bep, progitopFixed(), progitopMargin())
	assert.True(t, ebe.IsZero(), "EBE at BEP = 0")
}

func TestComputeEBE_AboveBEP_Positive(t *testing.T) {
	// Revenue = BEP + 10% = 2 640 000 → EBE = 2 640 000 × 0.60 − 1 440 000 = 144 000
	rev := decimal.NewFromInt(2_640_000)
	ebe := ComputeEBE(rev, progitopFixed(), progitopMargin())
	assert.Equal(t, "144000.00", ebe.StringFixed(2))
}

func TestComputeEBE_BelowBEP_Negative(t *testing.T) {
	// Revenue = BEP − 10% = 2 160 000 → EBE = 2 160 000 × 0.60 − 1 440 000 = −144 000
	rev := decimal.NewFromInt(2_160_000)
	ebe := ComputeEBE(rev, progitopFixed(), progitopMargin())
	assert.Equal(t, "-144000.00", ebe.StringFixed(2))
}

// ─────────────────────────────────────────────────────────────────────────────
// §3  ComputeEBETable — 11-column sensitivity table
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeEBETable_ProgiTop_Columns(t *testing.T) {
	bep := decimal.NewFromInt(2_400_000)
	step := decimal.NewFromInt(10)
	rang := decimal.NewFromInt(50)

	rows := ComputeEBETable(progitopFixed(), progitopMargin(), bep, step, rang)

	require.Len(t, rows, 11, "11 columns for ±50% at 10% step")

	// Centre column: variation = 0, EBE = 0 (base €)
	centre := rows[5]
	assert.True(t, centre.IsBEP)
	assert.True(t, centre.EBE.IsZero(), "EBE at BEP = 0")
	assert.False(t, centre.IsNegative)

	// First column: −50% → Revenue = 1 200 000 €, EBE < 0
	first := rows[0]
	assert.Equal(t, "-50", first.VariationPct.StringFixed(0))
	assert.True(t, first.IsNegative, "loss zone at −50%")
	assert.Equal(t, "1200000.0", first.Revenue.StringFixed(1)) // base €

	// Last column: +50% → EBE > 0
	last := rows[10]
	assert.Equal(t, "50", last.VariationPct.StringFixed(0))
	assert.False(t, last.IsNegative, "profit zone at +50%")
}

func TestComputeEBETable_NilWhenMarginZero(t *testing.T) {
	rows := ComputeEBETable(progitopFixed(), decimal.Zero,
		decimal.NewFromInt(2_400_000), decimal.NewFromInt(10), decimal.NewFromInt(50))
	assert.Nil(t, rows)
}

func TestComputeEBETable_CentreColumnIsExactlyZeroEBE(t *testing.T) {
	// The EBE at the BEP column must be exactly zero regardless of step/range config.
	for _, step := range []int{5, 10, 20} {
		bep := decimal.NewFromInt(2_400_000)
		rows := ComputeEBETable(progitopFixed(), progitopMargin(), bep,
			decimal.NewFromInt(int64(step)), decimal.NewFromInt(int64(step*5)))
		centre := findBEPRow(t, rows)
		assert.True(t, centre.EBE.IsZero(),
			"EBE at BEP must be exactly 0 for step=%d%%", step)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// §4  ComputeMarginSensitivity — BEP vs margin
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeMarginSensitivity_ProgiTop(t *testing.T) {
	rows := ComputeMarginSensitivity(progitopFixed(), progitopMargin(),
		decimal.NewFromInt(5), decimal.NewFromInt(25))

	require.Len(t, rows, 11, "11 columns")

	// Base column (0 variation): BEP should equal the reference 2 400 000 €
	base := findBaseMarginRow(t, rows)
	require.NotNil(t, base.BEPRevenue)
	assert.Equal(t, "2400000.0", base.BEPRevenue.StringFixed(1), "BEP at base = 2 400 000 €")

	// First column: margin = 60 − 25 = 35% → BEP = 1 440 000 / 0.35 ≈ 4 114 285 €
	first := rows[0]
	assert.Equal(t, "-25", first.MarginVariationPp.StringFixed(0))
	require.NotNil(t, first.BEPRevenue)
	// At 35% margin: BEP = 1440000/0.35 ≈ 4 114 285 €
	assert.True(t, first.BEPRevenue.GreaterThan(decimal.NewFromInt(4_000_000)),
		"lower margin → higher BEP")

	// Last column: margin = 85% → BEP < baseline
	last := rows[10]
	require.NotNil(t, last.BEPRevenue)
	assert.True(t, last.BEPRevenue.LessThan(decimal.NewFromInt(2_000_000)),
		"higher margin → lower BEP")
}

func TestComputeMarginSensitivity_UndefinedWhenMarginDropsBelowZero(t *testing.T) {
	// Base margin = 20%, range = 25 pp → leftmost col = 20 − 25 = −5% (undefined)
	rows := ComputeMarginSensitivity(progitopFixed(), decimal.NewFromInt(20),
		decimal.NewFromInt(5), decimal.NewFromInt(25))
	require.Len(t, rows, 11)
	// First column: margin = −5% → undefined
	assert.True(t, rows[0].IsUndefined)
	assert.Nil(t, rows[0].BEPRevenue)
}

func TestComputeMarginSensitivity_HigherMarginReducesBEP(t *testing.T) {
	// Non-regression: BEP is monotonically decreasing as margin increases.
	rows := ComputeMarginSensitivity(progitopFixed(), decimal.NewFromInt(50),
		decimal.NewFromInt(5), decimal.NewFromInt(20))
	lastBEP := (*decimal.Decimal)(nil)
	for _, row := range rows {
		if row.IsUndefined || row.BEPRevenue == nil {
			continue
		}
		if lastBEP != nil {
			assert.True(t, row.BEPRevenue.LessThanOrEqual(*lastBEP),
				"BEP should decrease as margin increases (col %s)", row.MarginPct)
		}
		lastBEP = row.BEPRevenue
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// §5  ComputeFixedCostSensitivity — BEP vs fixed costs
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeFixedCostSensitivity_ProgiTop(t *testing.T) {
	rows := ComputeFixedCostSensitivity(progitopFixed(), progitopMargin(),
		decimal.NewFromInt(10), decimal.NewFromInt(50))

	require.Len(t, rows, 11)

	base := findBaseFixedCostRow(t, rows)
	assert.Equal(t, "2400000.0", base.BEPRevenue.StringFixed(1), "BEP at base = 2 400 000 €")

	// Proportionality: 20% cost reduction → 20% BEP reduction (linear)
	// Find −20% column
	for _, row := range rows {
		if row.CostVariationPct.Equal(decimal.NewFromInt(-20)) {
			// Expected: BEP = 1440000 × 0.80 / 0.60 = 1920000 €
			assert.Equal(t, "1920000.0", row.BEPRevenue.StringFixed(1),
				"20%% cost reduction → BEP drops 20%%")
		}
	}
}

func TestComputeFixedCostSensitivity_LinearRelationship(t *testing.T) {
	// Non-regression: BEP is proportional to fixed costs (holding margin constant).
	rows := ComputeFixedCostSensitivity(progitopFixed(), progitopMargin(),
		decimal.NewFromInt(10), decimal.NewFromInt(50))

	var baseline decimal.Decimal
	for _, row := range rows {
		if row.IsBase {
			baseline = row.BEPRevenue
		}
	}
	require.False(t, baseline.IsZero())

	for _, row := range rows {
		// BEP(n) = Baseline × (1 + variation/100)
		expectedFactor := decimal.NewFromInt(1).Add(row.CostVariationPct.Div(d100))
		expected := baseline.Mul(expectedFactor).Round(1)
		assert.True(t,
			row.BEPRevenue.Sub(expected).Abs().LessThan(decimal.NewFromFloat(0.2)),
			"BEP linear to fixed costs at variation=%s%%", row.CostVariationPct,
		)
	}
}

func TestComputeFixedCostSensitivity_NilWhenMarginZero(t *testing.T) {
	rows := ComputeFixedCostSensitivity(progitopFixed(), decimal.Zero,
		decimal.NewFromInt(10), decimal.NewFromInt(50))
	assert.Nil(t, rows)
}

// ─────────────────────────────────────────────────────────────────────────────
// §6  ComputeOptimisedBEP — Optimisation Planner (§8.3)
// ─────────────────────────────────────────────────────────────────────────────

// ProgiTop optimisation example from the EBE+ reference model:
//   Fixed cost reduction: −180 k€ → new fixed = 1 260 k€
//   Variable cost reduction: margin 60% → 70%
//   New BEP = 1 260 000 / 0.70 = 1 800 000 €  (−25%)
//   Volume improvement: 160 − 120 = 40 orders

func TestComputeOptimisedBEP_ProgiTopExample(t *testing.T) {
	fixedSavings := decimal.NewFromInt(180_000) // −180 k€ fixed cost reduction
	// Margin goes from 60% to 70% via variable cost reduction.
	// avg order = 15 000, original var cost/unit = 6 000
	// new var cost/unit such that new margin = 70%: var = 15000 × 0.30 = 4 500
	newVarCostPerUnit := decimal.NewFromInt(4_500)

	report := ComputeOptimisedBEP(
		progitopFixed(), progitopMargin(), progitopAvgOrder(),
		fixedSavings, newVarCostPerUnit, true,
	)

	// Fixed cost state
	assert.Equal(t, "1440000.00", report.FixedCosts.Current.StringFixed(2))
	assert.Equal(t, "1260000.00", report.FixedCosts.Optimised.StringFixed(2))
	assert.Equal(t, "180000.00", report.FixedCosts.DeltaAbs.StringFixed(2))
	assert.Equal(t, "12.50", report.FixedCosts.DeltaPct.StringFixed(2))

	// New margin = (15000 − 4500) / 15000 × 100 = 70%
	assert.Equal(t, "70.000000", report.MarginPct.Optimised.StringFixed(6))

	// Optimised BEP = 1 260 000 / 0.70 = 1 800 000 €
	require.NotNil(t, report.OptimisedBEPRevenue)
	assert.Equal(t, "1800000.00", report.OptimisedBEPRevenue.StringFixed(2))

	// BEP improvement % = (2 400 000 − 1 800 000) / 2 400 000 = 25%
	require.NotNil(t, report.BEPRevenueImprovementPct)
	assert.Equal(t, "25.00", report.BEPRevenueImprovementPct.StringFixed(2))

	// Volume: 2 400 000 / 15 000 = 160; optimised: 1 800 000 / 15 000 = 120 → Δ = 40
	require.NotNil(t, report.OptimisedBEPVolume)
	assert.Equal(t, "120.00", report.OptimisedBEPVolume.StringFixed(2))
	require.NotNil(t, report.BEPVolumeImprovement)
	assert.Equal(t, "40.00", report.BEPVolumeImprovement.StringFixed(2))

	assert.Empty(t, report.Warnings)
}

func TestComputeOptimisedBEP_NoVariableSavings_OnlyFixedReduction(t *testing.T) {
	// Only fixed cost reduction; variable costs unchanged.
	// Margin remains 60%, new fixed = 1 440 000 − 144 000 = 1 296 000
	// New BEP = 1 296 000 / 0.60 = 2 160 000 (−10%)
	fixedSavings := decimal.NewFromInt(144_000)
	// Baseline var cost/unit = 15000 × 0.40 = 6000 (unchanged)
	sameVarCost := decimal.NewFromInt(6_000)

	report := ComputeOptimisedBEP(progitopFixed(), progitopMargin(), progitopAvgOrder(),
		fixedSavings, sameVarCost, true)

	require.NotNil(t, report.OptimisedBEPRevenue)
	assert.Equal(t, "2160000.00", report.OptimisedBEPRevenue.StringFixed(2))
	require.NotNil(t, report.BEPRevenueImprovementPct)
	assert.Equal(t, "10.00", report.BEPRevenueImprovementPct.StringFixed(2))
}

func TestComputeOptimisedBEP_ZeroSavings_Unchanged(t *testing.T) {
	// No savings applied → optimised = baseline
	sameVarCost := decimal.NewFromInt(6_000) // baseline var cost/unit
	report := ComputeOptimisedBEP(progitopFixed(), progitopMargin(), progitopAvgOrder(),
		decimal.Zero, sameVarCost, true)

	require.NotNil(t, report.OptimisedBEPRevenue)
	require.NotNil(t, report.BaselineBEPRevenue)
	assert.Equal(t, report.BaselineBEPRevenue.StringFixed(2), report.OptimisedBEPRevenue.StringFixed(2),
		"no savings → optimised BEP = baseline BEP")
}

func TestComputeOptimisedBEP_NegativeOptimisedMarginWarning(t *testing.T) {
	// Variable cost "savings" that push the new var cost above avg order value
	// → new margin would go negative → engine clamps to 0 and emits a warning.
	hugeSaving := decimal.NewFromInt(20_000) // new var cost per unit = 20 000 > avg order 15 000
	report := ComputeOptimisedBEP(progitopFixed(), progitopMargin(), progitopAvgOrder(),
		decimal.Zero, hugeSaving, true)

	require.Len(t, report.Warnings, 1)
	assert.Equal(t, "optimised_margin_negative", report.Warnings[0].Code)
}

func TestComputeOptimisedBEP_NoAvgOrderValue(t *testing.T) {
	// Without avg order value, BEP volume is nil but BEP revenue is still computed.
	report := ComputeOptimisedBEP(progitopFixed(), progitopMargin(), nil,
		decimal.NewFromInt(100_000), decimal.Zero, true)

	assert.Nil(t, report.OptimisedBEPVolume)
	assert.Nil(t, report.BaselineBEPVolume)
	require.NotNil(t, report.OptimisedBEPRevenue)
}

// TestComputeOptimisedBEP_NoVarCostLines_MarginUnchanged is the regression test
// for the bug where a snapshot with an AvgOrderValue but no VariableCostLine rows
// produced an optimised margin of 100% (because newTotalVarCostPerUnit = 0 from
// an empty sum was mistaken for "all variable costs eliminated").
// With varCostLinesDefined = false the margin must stay at baseMarginPct.
func TestComputeOptimisedBEP_NoVarCostLines_MarginUnchanged(t *testing.T) {
	baseMargin := decimal.NewFromFloat(43.04)
	avgOrder := decimal.NewFromInt(1620)

	// varCostLinesDefined = false → newTotalVarCostPerUnit is irrelevant
	report := ComputeOptimisedBEP(
		decimal.NewFromInt(580_000),
		baseMargin,
		&avgOrder,
		decimal.Zero,  // no fixed savings either
		decimal.Zero,  // empty sum from no lines
		false,         // ← no lines defined
	)

	// Margin must equal baseline, not jump to 100%
	assert.True(t, baseMargin.Equal(report.MarginPct.Optimised),
		"margin must stay at %.6f when no var cost lines defined, got %s",
		baseMargin, report.MarginPct.Optimised)
	// BEP revenue must equal baseline BEP (savings = 0)
	require.NotNil(t, report.OptimisedBEPRevenue)
	require.NotNil(t, report.BaselineBEPRevenue)
	assert.Equal(t, report.BaselineBEPRevenue.StringFixed(2), report.OptimisedBEPRevenue.StringFixed(2),
		"no savings + no lines → optimised BEP = baseline BEP")
	// Improvement % must be zero (or nil)
	if report.BEPRevenueImprovementPct != nil {
		assert.True(t, report.BEPRevenueImprovementPct.IsZero(),
			"improvement must be 0 when nothing changed")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// §7  buildVariationSteps — helper
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildVariationSteps_DefaultConfig(t *testing.T) {
	steps := buildVariationSteps(decimal.NewFromInt(10), decimal.NewFromInt(50))
	require.Len(t, steps, 11, "±50%% at 10%% step = 11 columns")
	assert.Equal(t, "-50", steps[0].StringFixed(0))
	assert.Equal(t, "0", steps[5].StringFixed(0))
	assert.Equal(t, "50", steps[10].StringFixed(0))
}

func TestBuildVariationSteps_MarginConfig(t *testing.T) {
	steps := buildVariationSteps(decimal.NewFromInt(5), decimal.NewFromInt(25))
	require.Len(t, steps, 11, "±25 pp at 5 pp step = 11 columns")
	assert.Equal(t, "-25", steps[0].StringFixed(0))
	assert.Equal(t, "0", steps[5].StringFixed(0))
	assert.Equal(t, "25", steps[10].StringFixed(0))
}

func TestBuildVariationSteps_ZeroStepReturnsOnlyBase(t *testing.T) {
	steps := buildVariationSteps(decimal.Zero, decimal.NewFromInt(50))
	require.Len(t, steps, 1)
	assert.True(t, steps[0].IsZero())
}

// ─────────────────────────────────────────────────────────────────────────────
// §8  ComputeBEPReport — full orchestration
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeBEPReport_AllTablesPopulated(t *testing.T) {
	snap := model.BEPSnapshot{
		FixedCostsTotal:       progitopFixed(),
		ContributionMarginPct: progitopMargin(),
		AvgOrderValue:         progitopAvgOrder(),
	}
	report := ComputeBEPReport(snap, nil) // nil → use defaults

	assert.NotNil(t, report.Core.BEPRevenue)
	assert.Len(t, report.EBETable, 11)
	assert.Len(t, report.MarginSens, 11)
	assert.Len(t, report.CostSens, 11)
}

func TestComputeBEPReport_CustomSensitivityConfig(t *testing.T) {
	snap := model.BEPSnapshot{
		FixedCostsTotal:       progitopFixed(),
		ContributionMarginPct: progitopMargin(),
	}
	// Custom: 20% step, ±40% range → 5 columns (−40, −20, 0, +20, +40)
	configs := []model.SensitivityConfig{
		{
			AnalysisType: model.SensRevenue,
			StepSizePct:  decimal.NewFromInt(20),
			RangePct:     decimal.NewFromInt(40),
		},
	}
	report := ComputeBEPReport(snap, configs)

	assert.Len(t, report.EBETable, 5, "custom step/range produces 5 columns")
	assert.Len(t, report.MarginSens, 11, "defaults used when config absent for type")
}

func TestComputeBEPReport_UndefinedBEP_EmptyEBETable(t *testing.T) {
	snap := model.BEPSnapshot{
		FixedCostsTotal:       progitopFixed(),
		ContributionMarginPct: decimal.Zero, // BEP undefined
	}
	report := ComputeBEPReport(snap, nil)

	assert.Nil(t, report.Core.BEPRevenue)
	assert.Nil(t, report.EBETable, "no EBE table when BEP undefined")
}

// ─────────────────────────────────────────────────────────────────────────────
// §9  Non-regression tests
// ─────────────────────────────────────────────────────────────────────────────

// NR-01: EBE at BEP is always exactly zero, regardless of fixed costs / margin.
func TestNR_EBEAtBEPAlwaysZero(t *testing.T) {
	cases := []struct {
		fixed  decimal.Decimal
		margin decimal.Decimal
	}{
		{decimal.NewFromInt(100_000), decimal.NewFromInt(25)},
		{decimal.NewFromInt(5_000_000), decimal.NewFromFloat(72.5)},
		{decimal.NewFromInt(1), decimal.NewFromInt(100)},
	}
	for _, c := range cases {
		core := ComputeBEPCore(c.fixed, c.margin, nil)
		require.NotNil(t, core.BEPRevenue, "BEP should be defined for positive margin")
		ebe := ComputeEBE(*core.BEPRevenue, c.fixed, c.margin)
		// Allow rounding error of ≤ 0.01 €
		assert.True(t, ebe.Abs().LessThanOrEqual(decimal.NewFromFloat(0.01)),
			"EBE at BEP must be ≈ 0 for fixed=%s margin=%s%%", c.fixed, c.margin)
	}
}

// NR-02: Margin sensitivity BEP is monotonically decreasing as margin increases.
func TestNR_MarginSens_Monotonic(t *testing.T) {
	rows := ComputeMarginSensitivity(
		decimal.NewFromInt(1_000_000),
		decimal.NewFromInt(50),
		decimal.NewFromInt(5),
		decimal.NewFromInt(20),
	)
	var prev *decimal.Decimal
	for _, row := range rows {
		if row.IsUndefined || row.BEPRevenue == nil {
			continue
		}
		if prev != nil {
			assert.True(t, row.BEPRevenue.LessThanOrEqual(*prev),
				"BEP must decrease as margin increases: got %s after %s",
				row.BEPRevenue, *prev)
		}
		prev = row.BEPRevenue
	}
}

// NR-03: Fixed-cost sensitivity BEP is proportional (linear) to cost variation.
func TestNR_FixedCostSens_Linear(t *testing.T) {
	fixed := decimal.NewFromInt(800_000)
	margin := decimal.NewFromInt(40)
	rows := ComputeFixedCostSensitivity(fixed, margin,
		decimal.NewFromInt(10), decimal.NewFromInt(50))

	var baseline decimal.Decimal
	for _, row := range rows {
		if row.IsBase {
			baseline = row.BEPRevenue
		}
	}

	for _, row := range rows {
		expectedFactor := decimal.NewFromInt(1).Add(row.CostVariationPct.Div(d100))
		expected := baseline.Mul(expectedFactor).Round(1)
		diff := row.BEPRevenue.Sub(expected).Abs()
		assert.True(t, diff.LessThanOrEqual(decimal.NewFromFloat(0.2)),
			"BEP(cost variation=%s) should be proportional: expected %s got %s",
			row.CostVariationPct, expected, row.BEPRevenue)
	}
}

// NR-04: Empty EBE table columns exactly 11 for default config.
func TestNR_EBETable_Always11Columns(t *testing.T) {
	bep := decimal.NewFromInt(2_000_000)
	rows := ComputeEBETable(decimal.NewFromInt(1_200_000), decimal.NewFromInt(60),
		bep, decimal.NewFromInt(10), decimal.NewFromInt(50))
	assert.Len(t, rows, 11)
}

// NR-05: optimised BEP improvement % ≥ 0 when savings ≥ 0.
func TestNR_OptimisedBEP_ImprovementNonNegative(t *testing.T) {
	report := ComputeOptimisedBEP(
		decimal.NewFromInt(1_000_000), decimal.NewFromInt(55), progitopAvgOrder(),
		decimal.NewFromInt(50_000), // positive fixed saving
		decimal.NewFromInt(6_000),  // same var cost as baseline
		true,
	)
	if report.BEPRevenueImprovementPct != nil {
		assert.True(t, report.BEPRevenueImprovementPct.GreaterThanOrEqual(decimal.Zero),
			"improvement %% should be ≥ 0 when savings ≥ 0")
	}
}

// NR-06: Savings exceeding fixed costs are clamped to zero (new fixed ≥ 0).
func TestNR_FixedSavingsClamped(t *testing.T) {
	// Fixed savings > total fixed costs → new fixed costs should be 0, not negative.
	hugeSavings := decimal.NewFromInt(99_000_000)
	report := ComputeOptimisedBEP(
		progitopFixed(), progitopMargin(), nil,
		hugeSavings, decimal.Zero, false,
	)
	assert.True(t, report.FixedCosts.Optimised.GreaterThanOrEqual(decimal.Zero),
		"optimised fixed costs must be ≥ 0")
}

// NR-07: EBE table output values are in base € (no /1000 applied).
func TestNR_EBEOutput_IsBaseEur(t *testing.T) {
	// Fixed costs 2 400 000 € (base), margin 40 %
	fixedCosts := decimal.NewFromInt(2_400_000)
	marginPct := decimal.NewFromInt(40)
	bepRevenue := fixedCosts.Div(marginPct.Div(decimal.NewFromInt(100))) // 6 000 000 €

	rows := ComputeEBETable(fixedCosts, marginPct, bepRevenue,
		decimal.NewFromInt(10), decimal.NewFromInt(50))

	// Find the BEP row (variation = 0%)
	var bepRow *model.EBERow
	for i := range rows {
		if rows[i].IsBEP {
			bepRow = &rows[i]
			break
		}
	}
	if bepRow == nil {
		t.Fatal("no BEP row found")
	}
	// Revenue should be ~6 000 000 € (base €, NOT 6 000 k€)
	assert.True(t,
		bepRow.Revenue.GreaterThanOrEqual(decimal.NewFromInt(5_000_000)),
		"BEP revenue should be in base € (≥ 5 000 000), got %s", bepRow.Revenue.String(),
	)
}

// ─────────────────────────────────────────────────────────────────────────────
// Test helpers
// ─────────────────────────────────────────────────────────────────────────────

func findBEPRow(t *testing.T, rows []model.EBERow) model.EBERow {
	t.Helper()
	for _, r := range rows {
		if r.IsBEP {
			return r
		}
	}
	t.Fatal("no BEP row found in EBE table")
	return model.EBERow{}
}

func findBaseMarginRow(t *testing.T, rows []model.MarginSensRow) model.MarginSensRow {
	t.Helper()
	for _, r := range rows {
		if r.IsBase {
			return r
		}
	}
	t.Fatal("no base row in margin sensitivity table")
	return model.MarginSensRow{}
}

func findBaseFixedCostRow(t *testing.T, rows []model.FixedCostSensRow) model.FixedCostSensRow {
	t.Helper()
	for _, r := range rows {
		if r.IsBase {
			return r
		}
	}
	t.Fatal("no base row in fixed-cost sensitivity table")
	return model.FixedCostSensRow{}
}
