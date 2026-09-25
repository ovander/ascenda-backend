package service

// SensitivityEngine — in-memory perturbation engine for scenario sensitivity analysis.
//
// Design rules:
//  1. Zero DB calls: the engine clones an already-loaded FullPlanInput in memory
//     and calls the compute engine directly.  All DB access is handled by the
//     caller via PlanComputeOrchestrator.LoadInputs.
//  2. Typed levers: each EconomicLever is a named constant — unknown strings are
//     rejected at entry rather than silently misrouted.
//  3. Performance limits:
//     - Maximum N ≤ 10 perturbations per call (maxSensitivityPerturbations).
//     - Context-bounded execution via sensitivityTimeout.
//     - Early stop: 3 consecutive zero-delta results on the same lever halt
//       evaluation for that lever.

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"ascenda/internal/compute"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

// ──────────────────────────────────────────────────────────────────────────────
// Constants
// ──────────────────────────────────────────────────────────────────────────────

const (
	// maxSensitivityPerturbations caps the number of perturbations accepted
	// per Run call.  Prevents runaway compute time.
	maxSensitivityPerturbations = 10

	// sensitivityTimeout is the per-Run context deadline.  Intentionally
	// distinct from the report timeout (separate concerns).
	sensitivityTimeout = 15 * time.Second

	// fragility signal thresholds — normalised stddev of viability deltas.
	fragilityStable    = 20
	fragilitySensitive = 50
)

// ──────────────────────────────────────────────────────────────────────────────
// Public types
// ──────────────────────────────────────────────────────────────────────────────

// EconomicLever is a validated string identifying which input dimension
// to perturb.  Unknown lever strings are rejected by Run.
type EconomicLever string

const (
	// LeverRevenueScale scales all product unit prices by (1 + Delta).
	LeverRevenueScale EconomicLever = "revenue_scale"

	// LeverHeadcountScale scales all staff FTE values by (1 + Delta).
	LeverHeadcountScale EconomicLever = "headcount_scale"

	// LeverCOGSScale scales all product raw-material costs by (1 + Delta).
	LeverCOGSScale EconomicLever = "cogs_scale"
)

// Perturbation describes a single lever adjustment to apply to the baseline.
type Perturbation struct {
	// Lever identifies which economic dimension to perturb.
	Lever EconomicLever `json:"lever"`

	// Delta is the fractional change: +0.10 = +10%, −0.20 = −20%.
	// The effective factor applied to the target field is (1 + Delta).
	Delta decimal.Decimal `json:"delta"`
}

// SensitivityResult holds the outcome of one perturbation.
type SensitivityResult struct {
	Lever          EconomicLever   `json:"lever"`
	Delta          decimal.Decimal `json:"delta"`
	ViabilityScore int             `json:"viability_score"`
	ViabilityDelta int             `json:"viability_delta"` // result − baseline
}

// SensitivityReport is the top-level output of SensitivityEngine.Run.
type SensitivityReport struct {
	Results         []SensitivityResult `json:"results"`
	FragilityScore  int                 `json:"fragility_score"`  // 0–100 normalised stddev
	FragilitySignal string              `json:"fragility_signal"` // stable | sensitive | fragile
}

// ──────────────────────────────────────────────────────────────────────────────
// SensitivityEngine
// ──────────────────────────────────────────────────────────────────────────────

// SensitivityEngine performs in-memory perturbation analysis on a scenario.
type SensitivityEngine struct {
	orchestrator *PlanComputeOrchestrator
	logger       *logrus.Entry
}

// NewSensitivityEngine creates a SensitivityEngine.
func NewSensitivityEngine(orchestrator *PlanComputeOrchestrator, logger *logrus.Entry) *SensitivityEngine {
	return &SensitivityEngine{orchestrator: orchestrator, logger: logger}
}

// Run performs sensitivity analysis for a scenario.
//
// Validation errors (unknown lever, N > 10) are returned before any DB access.
// The context is bounded by sensitivityTimeout; early cancellation is respected
// at each perturbation boundary.
func (e *SensitivityEngine) Run(
	ctx context.Context,
	tenantID, scenarioID uuid.UUID,
	perturbations []Perturbation,
) (*SensitivityReport, error) {
	// ── Validate first — no DB calls below this line ──────────────────────
	if len(perturbations) > maxSensitivityPerturbations {
		return nil, apierror.BadRequest(fmt.Sprintf(
			"too many perturbations: %d provided, max %d allowed",
			len(perturbations), maxSensitivityPerturbations,
		))
	}
	for _, p := range perturbations {
		if !isValidLever(p.Lever) {
			return nil, apierror.BadRequest(fmt.Sprintf("unknown lever: %q", p.Lever))
		}
	}

	// ── Load inputs (single DB round-trip) ───────────────────────────────
	ctx, cancel := context.WithTimeout(ctx, sensitivityTimeout)
	defer cancel()

	input, err := e.orchestrator.LoadInputs(tenantID, scenarioID)
	if err != nil {
		return nil, err
	}

	return runPerturbations(e.orchestrator, *input, perturbations, ctx)
}

// runPerturbations executes perturbations against a pre-loaded FullPlanInput.
// It is intentionally package-private rather than a method so that tests can
// call it directly with an in-memory input — zero DB round-trips required.
func runPerturbations(
	orch *PlanComputeOrchestrator,
	input compute.FullPlanInput,
	perturbations []Perturbation,
	ctx context.Context,
) (*SensitivityReport, error) {
	// ── Baseline ──────────────────────────────────────────────────────────
	baselineOutput := orch.ComputeFromInput(input)
	baselineProj := computeProjections(baselineOutput)
	baselineScore := computeViability(baselineProj).Score

	// ── Perturbation loop ─────────────────────────────────────────────────
	var results []SensitivityResult
	// Track consecutive zero-delta count per lever for early stop.
	consecutiveZeros := make(map[EconomicLever]int)

	for _, p := range perturbations {
		// Context check: respect cancellation / timeout.
		if ctx != nil {
			select {
			case <-ctx.Done():
				break
			default:
			}
		}

		// Early stop: 3 consecutive zero-delta results on this lever.
		if consecutiveZeros[p.Lever] >= 3 {
			continue
		}

		cloned := cloneFullPlanInput(input)
		factor := decimal.NewFromInt(1).Add(p.Delta)
		perturbed := applyPerturbation(cloned, p.Lever, factor)

		pertOutput := orch.ComputeFromInput(perturbed)
		pertProj := computeProjections(pertOutput)
		pertScore := computeViability(pertProj).Score
		delta := pertScore - baselineScore

		results = append(results, SensitivityResult{
			Lever:          p.Lever,
			Delta:          p.Delta,
			ViabilityScore: pertScore,
			ViabilityDelta: delta,
		})

		if delta == 0 {
			consecutiveZeros[p.Lever]++
		} else {
			consecutiveZeros[p.Lever] = 0
		}
	}

	fragScore, fragSignal := computeFragility(results)
	return &SensitivityReport{
		Results:         results,
		FragilityScore:  fragScore,
		FragilitySignal: fragSignal,
	}, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Lever validation + dispatch
// ──────────────────────────────────────────────────────────────────────────────

// isValidLever returns true for recognised EconomicLever values.
func isValidLever(l EconomicLever) bool {
	switch l {
	case LeverRevenueScale, LeverHeadcountScale, LeverCOGSScale:
		return true
	}
	return false
}

// applyPerturbation dispatches to the appropriate perturbation function.
// The input is assumed to be a clone (safe to mutate).
func applyPerturbation(input compute.FullPlanInput, lever EconomicLever, factor decimal.Decimal) compute.FullPlanInput {
	switch lever {
	case LeverRevenueScale:
		return perturbRevenueScale(input, factor)
	case LeverHeadcountScale:
		return perturbHeadcountScale(input, factor)
	case LeverCOGSScale:
		return perturbCOGSScale(input, factor)
	}
	return input
}

// ──────────────────────────────────────────────────────────────────────────────
// Perturbation functions — pure, no I/O, operate on cloned inputs
// ──────────────────────────────────────────────────────────────────────────────

// perturbRevenueScale multiplies BaseUnitPrice in every product-year assumption
// by factor.  Revenue scales proportionally with unit price.
func perturbRevenueScale(input compute.FullPlanInput, factor decimal.Decimal) compute.FullPlanInput {
	for i := range input.ProductData {
		for y := range input.ProductData[i].Assumptions {
			input.ProductData[i].Assumptions[y].BaseUnitPrice =
				input.ProductData[i].Assumptions[y].BaseUnitPrice.Mul(factor)
		}
	}
	return input
}

// perturbHeadcountScale multiplies every StaffHeadcount.FTE by factor.
// Higher FTE → higher payroll → lower viability / cash health.
func perturbHeadcountScale(input compute.FullPlanInput, factor decimal.Decimal) compute.FullPlanInput {
	for i := range input.Headcounts {
		input.Headcounts[i].FTE = input.Headcounts[i].FTE.Mul(factor)
	}
	return input
}

// perturbCOGSScale multiplies RawMaterialCost in every product-year assumption
// by factor.  Higher COGS → lower gross margin → lower viability.
func perturbCOGSScale(input compute.FullPlanInput, factor decimal.Decimal) compute.FullPlanInput {
	for i := range input.ProductData {
		for y := range input.ProductData[i].Assumptions {
			input.ProductData[i].Assumptions[y].RawMaterialCost =
				input.ProductData[i].Assumptions[y].RawMaterialCost.Mul(factor)
		}
	}
	return input
}

// ──────────────────────────────────────────────────────────────────────────────
// Fragility
// ──────────────────────────────────────────────────────────────────────────────

// computeFragility derives a fragility score and signal from the distribution of
// viability deltas.
//
//	FragilityScore = clamp(int(stddev(viabilityDeltas)), 0, 100)
//
// This is directly interpretable: a score of 30 means perturbations shift
// viability by ±30 points on average.
//
//	0–20  → "stable"    (perturbations barely move viability)
//	21–50 → "sensitive"
//	51+   → "fragile"
//
// Guard: fewer than 2 results → score=0, signal="stable" (no variance by definition).
func computeFragility(results []SensitivityResult) (score int, signal string) {
	if len(results) < 2 {
		return 0, "stable"
	}

	deltas := make([]float64, len(results))
	var sum float64
	for i, r := range results {
		d := float64(r.ViabilityDelta)
		deltas[i] = d
		sum += d
	}

	mean := sum / float64(len(deltas))
	var variance float64
	for _, d := range deltas {
		diff := d - mean
		variance += diff * diff
	}
	variance /= float64(len(deltas))

	rawScore := int(math.Sqrt(variance))
	if rawScore < 0 {
		rawScore = 0
	}
	if rawScore > 100 {
		rawScore = 100
	}

	var sig string
	switch {
	case rawScore <= fragilityStable:
		sig = "stable"
	case rawScore <= fragilitySensitive:
		sig = "sensitive"
	default:
		sig = "fragile"
	}
	return rawScore, sig
}

// ──────────────────────────────────────────────────────────────────────────────
// Deep clone
// ──────────────────────────────────────────────────────────────────────────────

// cloneFullPlanInput produces a deep copy of a FullPlanInput such that no
// slice in the clone shares a backing array with the original.
//
// Value-type fields (Config, OpeningBalance, WCConfig, …) are copied by the
// struct assignment.  Slices require explicit allocation + copy to prevent
// aliasing.
func cloneFullPlanInput(input compute.FullPlanInput) compute.FullPlanInput {
	clone := input // copies all value-type fields

	// Products
	clone.Products = cloneSlice(input.Products)

	// ProductData — ProductInputBundle contains two inner slices (Volumes,
	// Margins) that must also be deep-copied.
	if input.ProductData != nil {
		clone.ProductData = make([]compute.ProductInputBundle, len(input.ProductData))
		for i, b := range input.ProductData {
			bundle := b // copies fixed arrays (Assumptions, SurplusInventoryValue)
			bundle.Volumes = cloneSlice(b.Volumes)
			bundle.Margins = cloneSlice(b.Margins)
			clone.ProductData[i] = bundle
		}
	}

	// Staff
	clone.Headcounts = cloneSlice(input.Headcounts)
	clone.Salaries = cloneSlice(input.Salaries)
	clone.Incentives = cloneSlice(input.Incentives)

	// Capex / Opex
	clone.CapexEntries = cloneSlice(input.CapexEntries)
	clone.OpexEntries = cloneSlice(input.OpexEntries)

	// P&L / Finance
	clone.PnlEntries = cloneSlice(input.PnlEntries)
	clone.FiplanEntries = cloneSlice(input.FiplanEntries)
	clone.PnlCashEntries = cloneSlice(input.PnlCashEntries)
	clone.WCREntries = cloneSlice(input.WCREntries)

	// Monthly overrides
	clone.CashOverrides = cloneSlice(input.CashOverrides)
	clone.BudgetOverrides = cloneSlice(input.BudgetOverrides)

	return clone
}

// cloneSlice creates a shallow copy of a slice.  For slices of plain value
// types (structs without embedded pointer or slice fields), this is sufficient
// to prevent aliasing between original and clone.
func cloneSlice[T any](s []T) []T {
	if s == nil {
		return nil
	}
	c := make([]T, len(s))
	copy(c, s)
	return c
}

// ──────────────────────────────────────────────────────────────────────────────
// ComputeSensitivityLevers — AI narration helper
// ──────────────────────────────────────────────────────────────────────────────

// aiLeverDef maps each economic lever to the human-readable name and driver type
// label used in AI narration context.
type aiLeverDef struct {
	lever      EconomicLever
	name       string
	driverType string
}

var aiLevers = []aiLeverDef{
	{LeverRevenueScale, "Revenue (unit price)", "revenue"},
	{LeverHeadcountScale, "Headcount (FTE)", "cost"},
	{LeverCOGSScale, "Cost of Goods Sold", "cost"},
}

// ComputeSensitivityLevers runs each economic lever at +10% and -10% against
// the baseline, measuring EBITDA impact.  The result is a slice of
// SensitivityLever values sorted by absolute EBITDA impact descending — ready
// to be injected into NarrationContext.SensitivityLevers before calling the AI.
//
// The computation is entirely in-memory (no extra DB calls beyond LoadInputs).
func (e *SensitivityEngine) ComputeSensitivityLevers(
	ctx context.Context,
	tenantID, scenarioID uuid.UUID,
) ([]SensitivityLever, error) {
	ctx, cancel := context.WithTimeout(ctx, sensitivityTimeout)
	defer cancel()

	input, err := e.orchestrator.LoadInputs(tenantID, scenarioID)
	if err != nil {
		return nil, err
	}

	return computeSensitivityLeversFromInput(e.orchestrator, *input, ctx), nil
}

// computeSensitivityLeversFromInput is the package-private core of
// ComputeSensitivityLevers.  It accepts a pre-loaded FullPlanInput so that
// tests can call it directly — zero DB round-trips required.
func computeSensitivityLeversFromInput(
	orch *PlanComputeOrchestrator,
	input compute.FullPlanInput,
	ctx context.Context,
) []SensitivityLever {
	// Baseline sums across all forecast years.
	baseOutput := orch.ComputeFromInput(input)
	baseEBITDA := sumDecimalArray(baseOutput.Ratios.Profitability.EBITDA[:])
	baseRevenue := sumDecimalArray(baseOutput.Ratios.Sales.Sales[:])

	deltas := []decimal.Decimal{
		decimal.NewFromFloat(0.10),
		decimal.NewFromFloat(-0.10),
	}

	// leverResult holds the best (largest absolute EBITDA impact) result seen
	// for one lever across both +10 % and -10 % perturbations.
	type leverResult struct {
		def           aiLeverDef
		baseValue     decimal.Decimal
		stressValue   decimal.Decimal
		revenueImpact decimal.Decimal
		ebitdaImpact  decimal.Decimal
	}
	bestByLever := make(map[EconomicLever]leverResult, len(aiLevers))

	for _, def := range aiLevers {
		for _, delta := range deltas {
			if ctx != nil {
				select {
				case <-ctx.Done():
					goto done
				default:
				}
			}

			cloned := cloneFullPlanInput(input)
			factor := decimal.NewFromInt(1).Add(delta)
			perturbed := applyPerturbation(cloned, def.lever, factor)

			pertOutput := orch.ComputeFromInput(perturbed)
			pertEBITDA := sumDecimalArray(pertOutput.Ratios.Profitability.EBITDA[:])
			pertRevenue := sumDecimalArray(pertOutput.Ratios.Sales.Sales[:])

			ebitdaImpact := pertEBITDA.Sub(baseEBITDA)
			revenueImpact := pertRevenue.Sub(baseRevenue)

			existing, seen := bestByLever[def.lever]
			if !seen || ebitdaImpact.Abs().GreaterThan(existing.ebitdaImpact.Abs()) {
				var baseVal, stressVal decimal.Decimal
				if def.lever == LeverRevenueScale {
					baseVal, stressVal = baseRevenue, pertRevenue
				} else {
					baseVal, stressVal = baseEBITDA, pertEBITDA
				}
				bestByLever[def.lever] = leverResult{
					def:           def,
					baseValue:     baseVal,
					stressValue:   stressVal,
					revenueImpact: revenueImpact,
					ebitdaImpact:  ebitdaImpact,
				}
			}
		}
	}

done:
	levers := make([]SensitivityLever, 0, len(bestByLever))
	for _, r := range bestByLever {
		levers = append(levers, SensitivityLever{
			LeverName:     r.def.name,
			DriverType:    r.def.driverType,
			BaseValue:     r.baseValue.InexactFloat64(),
			StressValue:   r.stressValue.InexactFloat64(),
			RevenueImpact: r.revenueImpact.InexactFloat64(),
			EBITDAImpact:  r.ebitdaImpact.InexactFloat64(),
		})
	}
	sort.Slice(levers, func(i, j int) bool {
		return math.Abs(levers[i].EBITDAImpact) > math.Abs(levers[j].EBITDAImpact)
	})
	return levers
}

// sumDecimalArray sums a slice of decimal.Decimal values.
func sumDecimalArray(vals []decimal.Decimal) decimal.Decimal {
	var total decimal.Decimal
	for _, v := range vals {
		total = total.Add(v)
	}
	return total
}
