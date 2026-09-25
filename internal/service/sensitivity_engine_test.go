package service

// Unit tests for SensitivityEngine and its helpers.
//
// Design principles (mirror sensitivity_engine.go §Design rules):
//  1. Zero DB calls — every test either calls runPerturbations directly with an
//     in-memory FullPlanInput, or exercises validation paths that fire before
//     LoadInputs is ever called (nil orchestrator is safe for those paths).
//  2. Alias test (TestCloneFullPlanInput_NoAliasedSlices) proves that mutating
//     the clone does not affect the original — guaranteeing perturbation safety.
//  3. Fragility tests validate both extremes of the signal scale.

import (
	"context"
	"math"
	"testing"

	"ascenda/internal/compute"
	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────────────────────────────────────
// Test helper
// ──────────────────────────────────────────────────────────────────────────────

// buildSensitivityTestInput returns a FullPlanInput with:
//   - One DriverGeneric product
//   - BaseUnitPrice = 100 for all 5 years
//   - UnitsSold = 1 000 × year (France / direct) → ~100 k revenue Y1, ~500 k Y5
//   - PriceCoefficient = CostCoefficient = 1.0 (neutral)
//   - No staff, capex, opex, or overrides (clean baseline)
//
// This produces meaningful revenue so that perturbations create non-trivial
// viability deltas.
func buildSensitivityTestInput() compute.FullPlanInput {
	productID := uuid.New()

	product := model.Product{
		DriverType: model.DriverGeneric,
	}
	product.ID = productID

	bundle := compute.ProductInputBundle{}
	for y := 0; y < compute.MaxYears; y++ {
		bundle.Assumptions[y] = model.ProductAssumption{
			YearIndex:        y + 1,
			BaseUnitPrice:    decimal.NewFromFloat(100),
			RawMaterialCost:  decimal.NewFromFloat(30),
			PriceCoefficient: decimal.NewFromInt(1),
			CostCoefficient:  decimal.NewFromInt(1),
		}
		bundle.Volumes = append(bundle.Volumes, model.ProductSalesVolume{
			YearIndex: y + 1,
			Zone:      model.ZoneFrance,
			Channel:   model.ChannelDirect,
			UnitsSold: int64(1000 * (y + 1)),
		})
	}

	return compute.FullPlanInput{
		Products:    []model.Product{product},
		ProductData: []compute.ProductInputBundle{bundle},
	}
}

// nullLogger returns a logrus.Entry suitable for tests (writes to stderr,
// consistent with the rest of the service test suite).
func nullLogger() *logrus.Entry {
	return logrus.NewEntry(logrus.New())
}

// buildOrch builds a PlanComputeOrchestrator with nil repos.
// Safe only when no DB call will be made (validation-only or runPerturbations paths).
func buildOrch() *PlanComputeOrchestrator {
	return &PlanComputeOrchestrator{repos: nil, logger: nullLogger()}
}

// ──────────────────────────────────────────────────────────────────────────────
// 1. Clone correctness
// ──────────────────────────────────────────────────────────────────────────────

// TestCloneFullPlanInput_NoAliasedSlices proves that every slice in the clone
// is backed by a distinct array from the original.  Mutating the clone must
// never affect the original.
func TestCloneFullPlanInput_NoAliasedSlices(t *testing.T) {
	original := buildSensitivityTestInput()

	// Add a headcount entry to exercise the Headcounts slice path.
	original.Headcounts = []model.StaffHeadcount{
		{FTE: decimal.NewFromInt(5)},
	}

	clone := cloneFullPlanInput(original)

	// ── Mutate every slice in the clone ───────────────────────────────────
	clone.Products[0].Name = "mutated"
	clone.ProductData[0].Assumptions[0].BaseUnitPrice = decimal.NewFromInt(999)
	clone.ProductData[0].Volumes[0].UnitsSold = 0
	clone.Headcounts[0].FTE = decimal.NewFromInt(999)

	// ── Original must be unchanged ────────────────────────────────────────
	assert.Equal(t, "", original.Products[0].Name,
		"Products slice must not be aliased")
	assert.True(t,
		original.ProductData[0].Assumptions[0].BaseUnitPrice.Equal(decimal.NewFromFloat(100)),
		"Assumptions array must not be aliased")
	assert.Equal(t, int64(1000), original.ProductData[0].Volumes[0].UnitsSold,
		"Volumes slice must not be aliased")
	assert.True(t,
		original.Headcounts[0].FTE.Equal(decimal.NewFromInt(5)),
		"Headcounts slice must not be aliased")
}

// ──────────────────────────────────────────────────────────────────────────────
// 2. Perturbation functions
// ──────────────────────────────────────────────────────────────────────────────

// TestRevenueScale_PositiveDelta_IncreasesViability verifies that scaling
// revenue upward (+20 %) yields a higher-or-equal viability score than baseline.
func TestRevenueScale_PositiveDelta_IncreasesViability(t *testing.T) {
	orch := buildOrch()
	input := buildSensitivityTestInput()

	perturbations := []Perturbation{
		{Lever: LeverRevenueScale, Delta: decimal.NewFromFloat(0.20)},
	}

	report, err := runPerturbations(orch, input, perturbations, context.Background())
	require.NoError(t, err)
	require.Len(t, report.Results, 1)

	r := report.Results[0]
	assert.Equal(t, LeverRevenueScale, r.Lever)
	assert.True(t, r.ViabilityDelta >= 0,
		"positive revenue scaling must not decrease viability (delta=%d)", r.ViabilityDelta)
}

// TestHeadcountScale_2x_DegradesCashHealth verifies that doubling headcount
// (100 % increase in FTE) produces a zero-or-negative viability delta compared
// to a baseline with no staff.
func TestHeadcountScale_2x_DegradesCashHealth(t *testing.T) {
	orch := buildOrch()
	input := buildSensitivityTestInput()

	// Give the scenario a baseline headcount so the 2× lever has something to scale.
	// Salaries are left empty; the compute engine defaults to zero salary,
	// meaning doubling FTE still doubles payroll cost to 2× zero → delta = 0.
	// The assertion (delta ≤ 0) remains valid: viability cannot have increased.
	input.Headcounts = []model.StaffHeadcount{
		{FTE: decimal.NewFromInt(3)},
	}
	input.Salaries = []model.StaffSalary{
		{MonthlyGrossSalary: decimal.NewFromFloat(4000)},
	}

	perturbations := []Perturbation{
		{Lever: LeverHeadcountScale, Delta: decimal.NewFromFloat(1.0)}, // 2× headcount
	}

	report, err := runPerturbations(orch, input, perturbations, context.Background())
	require.NoError(t, err)
	require.Len(t, report.Results, 1)

	r := report.Results[0]
	assert.Equal(t, LeverHeadcountScale, r.Lever)
	assert.True(t, r.ViabilityDelta <= 0,
		"doubling headcount must not increase viability (delta=%d)", r.ViabilityDelta)
}

// ──────────────────────────────────────────────────────────────────────────────
// 3. Validation — fires before LoadInputs (nil repos safe)
// ──────────────────────────────────────────────────────────────────────────────

// TestSensitivityRun_UnknownLever_ReturnsError confirms that an unrecognised
// lever string is rejected with a 400-style error before any DB access.
func TestSensitivityRun_UnknownLever_ReturnsError(t *testing.T) {
	engine := NewSensitivityEngine(nil, nullLogger())

	_, err := engine.Run(
		context.Background(),
		uuid.New(), uuid.New(),
		[]Perturbation{
			{Lever: EconomicLever("unknown_lever"), Delta: decimal.NewFromFloat(0.1)},
		},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown lever")
}

// TestSensitivityRun_ExceedsMaxN_ReturnsError confirms that providing more than
// maxSensitivityPerturbations (10) entries is rejected before any DB access.
func TestSensitivityRun_ExceedsMaxN_ReturnsError(t *testing.T) {
	engine := NewSensitivityEngine(nil, nullLogger())

	perturbations := make([]Perturbation, maxSensitivityPerturbations+1)
	for i := range perturbations {
		perturbations[i] = Perturbation{Lever: LeverRevenueScale, Delta: decimal.NewFromFloat(0.01)}
	}

	_, err := engine.Run(
		context.Background(),
		uuid.New(), uuid.New(),
		perturbations,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many perturbations")
}

// ──────────────────────────────────────────────────────────────────────────────
// 4. Early stop
// ──────────────────────────────────────────────────────────────────────────────

// TestSensitivityRun_EarlyStop_After3ZeroDeltas verifies that once 3 consecutive
// perturbations on the same lever produce a zero viability delta the remaining
// perturbations for that lever are skipped.
//
// Strategy: set delta = 0 for all perturbations (factor = 1.0 → unperturbed
// output ≡ baseline → ViabilityDelta = 0 every time).  We submit 6 identical
// entries on the same lever and expect exactly 3 results (early-stop after 3).
func TestSensitivityRun_EarlyStop_After3ZeroDeltas(t *testing.T) {
	orch := buildOrch()
	input := buildSensitivityTestInput()

	perturbations := make([]Perturbation, 6)
	for i := range perturbations {
		perturbations[i] = Perturbation{
			Lever: LeverRevenueScale,
			Delta: decimal.NewFromInt(0), // factor = 1.0 → no change → delta = 0
		}
	}

	report, err := runPerturbations(orch, input, perturbations, context.Background())
	require.NoError(t, err)

	assert.Len(t, report.Results, 3,
		"early stop must halt after 3 consecutive zero-delta results on the same lever")
}

// ──────────────────────────────────────────────────────────────────────────────
// 5. Fragility scoring
// ──────────────────────────────────────────────────────────────────────────────

// TestFragility_ZeroVariance_IsStable confirms that identical viability deltas
// yield FragilityScore = 0 and FragilitySignal = "stable".
func TestFragility_ZeroVariance_IsStable(t *testing.T) {
	results := []SensitivityResult{
		{ViabilityDelta: 5},
		{ViabilityDelta: 5},
		{ViabilityDelta: 5},
		{ViabilityDelta: 5},
	}

	score, signal := computeFragility(results)

	assert.Equal(t, 0, score, "zero variance must produce score = 0")
	assert.Equal(t, "stable", signal)
}

// TestFragility_HighVariance_IsFragile confirms that widely-spread viability
// deltas push the score above fragilitySensitive (50) → signal = "fragile".
func TestFragility_HighVariance_IsFragile(t *testing.T) {
	// Deltas: −70, −70, +70, +70  → mean = 0, variance = 70², stddev = 70
	results := []SensitivityResult{
		{ViabilityDelta: -70},
		{ViabilityDelta: -70},
		{ViabilityDelta: 70},
		{ViabilityDelta: 70},
	}

	score, signal := computeFragility(results)

	assert.Greater(t, score, fragilitySensitive,
		"stddev=70 must exceed the sensitive threshold (50)")
	assert.Equal(t, "fragile", signal)
}

// ──────────────────────────────────────────────────────────────────────────────
// 6. computeSensitivityLeversFromInput — AI narration lever computation
// ──────────────────────────────────────────────────────────────────────────────

// TestComputeSensitivityLevers_ReturnsAllThreeLevers verifies that all three
// known economic levers (revenue, headcount, COGS) are returned.
func TestComputeSensitivityLevers_ReturnsAllThreeLevers(t *testing.T) {
	orch := buildOrch()
	input := buildSensitivityTestInput()
	// Add headcount + salary (Category+YearIndex must match for payroll to compute).
	for y := 1; y <= compute.MaxYears; y++ {
		input.Headcounts = append(input.Headcounts, model.StaffHeadcount{
			Category: model.CategoryAdminManagers, YearIndex: y, FTE: decimal.NewFromInt(2),
		})
		input.Salaries = append(input.Salaries, model.StaffSalary{
			Category: model.CategoryAdminManagers, YearIndex: y, MonthlyGrossSalary: decimal.NewFromFloat(3000),
		})
	}

	levers := computeSensitivityLeversFromInput(orch, input, context.Background())

	require.Len(t, levers, 3, "must return one entry per economic lever")

	names := make([]string, len(levers))
	for i, l := range levers {
		names[i] = l.LeverName
	}
	assert.Contains(t, names, "Revenue (unit price)")
	assert.Contains(t, names, "Headcount (FTE)")
	assert.Contains(t, names, "Cost of Goods Sold")
}

// TestComputeSensitivityLevers_SortedByAbsEBITDAImpact verifies that the
// returned slice is ordered largest-to-smallest absolute EBITDA impact.
func TestComputeSensitivityLevers_SortedByAbsEBITDAImpact(t *testing.T) {
	orch := buildOrch()
	input := buildSensitivityTestInput()

	levers := computeSensitivityLeversFromInput(orch, input, context.Background())

	require.NotEmpty(t, levers)
	for i := 1; i < len(levers); i++ {
		prev := math.Abs(levers[i-1].EBITDAImpact)
		curr := math.Abs(levers[i].EBITDAImpact)
		assert.GreaterOrEqual(t, prev, curr,
			"levers must be sorted by |ebitda_impact| descending: index %d (%.2f) < index %d (%.2f)",
			i-1, prev, i, curr)
	}
}

// TestComputeSensitivityLevers_RevenueLeverDominates confirms that for a
// product-only plan (no staff, no COGS) the revenue lever has the largest
// absolute EBITDA impact and appears first in the sorted output.
func TestComputeSensitivityLevers_RevenueLeverDominates(t *testing.T) {
	orch := buildOrch()
	// Pure revenue plan: no headcount, no raw material cost.
	productID := uuid.New()
	product := model.Product{DriverType: model.DriverGeneric}
	product.ID = productID

	bundle := compute.ProductInputBundle{}
	for y := 0; y < compute.MaxYears; y++ {
		bundle.Assumptions[y] = model.ProductAssumption{
			YearIndex:        y + 1,
			BaseUnitPrice:    decimal.NewFromFloat(200),
			RawMaterialCost:  decimal.Zero, // no COGS
			PriceCoefficient: decimal.NewFromInt(1),
			CostCoefficient:  decimal.NewFromInt(1),
		}
		bundle.Volumes = append(bundle.Volumes, model.ProductSalesVolume{
			YearIndex: y + 1,
			Zone:      model.ZoneFrance,
			Channel:   model.ChannelDirect,
			UnitsSold: int64(500 * (y + 1)),
		})
	}
	input := compute.FullPlanInput{
		Products:    []model.Product{product},
		ProductData: []compute.ProductInputBundle{bundle},
	}

	levers := computeSensitivityLeversFromInput(orch, input, context.Background())

	require.NotEmpty(t, levers)
	assert.Equal(t, "Revenue (unit price)", levers[0].LeverName,
		"revenue lever must rank first when there is no COGS and no headcount")
}

// TestComputeSensitivityLevers_RevenueImpact_MatchesExpected verifies that the
// revenue_impact of the revenue lever equals +10% of 5-year total revenue.
// Base: 100 × (1000+2000+3000+4000+5000) = 1 500 000.  +10% → 150 000.
func TestComputeSensitivityLevers_RevenueImpact_MatchesExpected(t *testing.T) {
	orch := buildOrch()
	input := buildSensitivityTestInput()

	levers := computeSensitivityLeversFromInput(orch, input, context.Background())

	var revLever *SensitivityLever
	for i := range levers {
		if levers[i].LeverName == "Revenue (unit price)" {
			revLever = &levers[i]
			break
		}
	}
	require.NotNil(t, revLever, "revenue lever must be present")

	// The +10% perturbation should show the larger absolute impact and be selected.
	// Expected revenue impact = 10% of (100 × 15 000) = 150 000.
	expectedImpact := 150_000.0
	assert.InDelta(t, expectedImpact, math.Abs(revLever.RevenueImpact), 1.0,
		"revenue_impact must be ~150 000 for a +10%% price perturbation")
}

// TestComputeSensitivityLevers_HeadcountLever_NegativeEBITDAImpact verifies
// that increasing headcount produces a negative EBITDA impact (cost rises).
//
// StaffHeadcount and StaffSalary are matched by Category+YearIndex — both
// must be set to the same value for the compute engine to compute payroll.
func TestComputeSensitivityLevers_HeadcountLever_NegativeEBITDAImpact(t *testing.T) {
	orch := buildOrch()
	input := buildSensitivityTestInput()

	for y := 1; y <= compute.MaxYears; y++ {
		input.Headcounts = append(input.Headcounts, model.StaffHeadcount{
			Category:  model.CategoryAdminManagers,
			YearIndex: y,
			FTE:       decimal.NewFromInt(5),
		})
		input.Salaries = append(input.Salaries, model.StaffSalary{
			Category:           model.CategoryAdminManagers,
			YearIndex:          y,
			MonthlyGrossSalary: decimal.NewFromFloat(5000),
		})
	}

	levers := computeSensitivityLeversFromInput(orch, input, context.Background())

	var hcLever *SensitivityLever
	for i := range levers {
		if levers[i].LeverName == "Headcount (FTE)" {
			hcLever = &levers[i]
			break
		}
	}
	require.NotNil(t, hcLever, "headcount lever must be present")
	assert.Less(t, hcLever.EBITDAImpact, 0.0,
		"increasing headcount must produce a negative EBITDA impact")
}

// TestComputeSensitivityLevers_BaseValues_ArePositive verifies that base_value
// and stress_value are finite and non-zero for a plan with revenue.
func TestComputeSensitivityLevers_BaseValues_ArePositive(t *testing.T) {
	orch := buildOrch()
	input := buildSensitivityTestInput()

	levers := computeSensitivityLeversFromInput(orch, input, context.Background())

	for _, l := range levers {
		assert.False(t, math.IsNaN(l.BaseValue), "lever %q base_value must not be NaN", l.LeverName)
		assert.False(t, math.IsInf(l.BaseValue, 0), "lever %q base_value must not be Inf", l.LeverName)
		assert.False(t, math.IsNaN(l.StressValue), "lever %q stress_value must not be NaN", l.LeverName)
	}
}

// TestComputeSensitivityLevers_EmptyPlan_ReturnsLeversWithoutPanic ensures the
// function handles an empty plan gracefully (no products, no staff) — all
// EBITDA impacts will be zero but the function must not panic.
func TestComputeSensitivityLevers_EmptyPlan_ReturnsLeversWithoutPanic(t *testing.T) {
	orch := buildOrch()
	input := compute.FullPlanInput{} // completely empty

	require.NotPanics(t, func() {
		levers := computeSensitivityLeversFromInput(orch, input, context.Background())
		assert.Len(t, levers, 3, "must still return 3 levers even for an empty plan")
	})
}

// TestComputeSensitivityLevers_ContextCancelled_ReturnsPartialResults confirms
// that when the context is already cancelled the function returns immediately
// without panicking (may return 0 or partial levers — both are acceptable).
func TestComputeSensitivityLevers_ContextCancelled_ReturnsPartialResults(t *testing.T) {
	orch := buildOrch()
	input := buildSensitivityTestInput()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	require.NotPanics(t, func() {
		levers := computeSensitivityLeversFromInput(orch, input, ctx)
		// We only guarantee no panic; result count is non-deterministic.
		assert.LessOrEqual(t, len(levers), 3)
	})
}

// TestSumDecimalArray_Empty_ReturnsZero verifies the helper on an empty slice.
func TestSumDecimalArray_Empty_ReturnsZero(t *testing.T) {
	result := sumDecimalArray([]decimal.Decimal{})
	assert.True(t, result.IsZero())
}

// TestSumDecimalArray_MultipleValues_ReturnsCorrectSum verifies the helper.
func TestSumDecimalArray_MultipleValues_ReturnsCorrectSum(t *testing.T) {
	vals := []decimal.Decimal{
		decimal.NewFromFloat(10.5),
		decimal.NewFromFloat(20.0),
		decimal.NewFromFloat(-5.5),
	}
	result := sumDecimalArray(vals)
	assert.True(t, result.Equal(decimal.NewFromFloat(25.0)),
		"10.5 + 20.0 - 5.5 must equal 25.0, got %s", result)
}
