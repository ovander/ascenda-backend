// Package compute — Break-Even Point (BEP) computation engine.
//
// All functions are pure (no I/O, no side effects).  They implement the
// calculation contract defined in the Ascenda BEP specification §8.
//
// Precision rules (§10):
//   - Monetary values rounded to nearest integer (€) or 1 decimal (k€).
//   - Margin and variation percentages stored to 6 decimal places internally;
//     displayed to 1–2 decimal places.
//
// Undefined-state rule (§10):
//   - When ContributionMarginPct ≤ 0, BEP is mathematically undefined
//     (return nil pointer; caller must display a message, never divide by zero).
//   - When FixedCostsTotal = 0, BEP = 0 (trivially met at any positive revenue).
package compute

import (
	"ascenda/internal/model"
	"github.com/shopspring/decimal"
)

// ─────────────────────────────────────────────────────────────────────────────
// Internal precision constants
// ─────────────────────────────────────────────────────────────────────────────

var (
	d100   = decimal.NewFromInt(100)
	d12    = decimal.NewFromInt(12)
	d10pct = decimal.NewFromFloat(10.0) // low-margin warning threshold
	dZero  = decimal.Zero
)

// ─────────────────────────────────────────────────────────────────────────────
// §8.1  Core BEP fundamentals
// ─────────────────────────────────────────────────────────────────────────────

// ComputeBEPCore derives the four primary break-even outputs from a snapshot's
// mandatory inputs plus the optional average order value.
//
//	BEP Revenue   = FixedCosts  ÷  (MarginPct / 100)
//	BEP Volume    = BEP Revenue ÷  AvgOrderValue
//	EBE           = Revenue × (MarginPct / 100) − FixedCosts
//	VarCostPerOrd = AvgOrderValue × (1 − MarginPct/100)
func ComputeBEPCore(
	fixedCostsTotal decimal.Decimal,
	contributionMarginPct decimal.Decimal,
	avgOrderValue *decimal.Decimal,
) model.BEPCoreResult {
	result := model.BEPCoreResult{}
	marginFrac := contributionMarginPct.Div(d100) // e.g. 0.60 for 60%

	// Variable cost fraction (for display)
	varCostFrac := decimal.NewFromInt(1).Sub(marginFrac)
	result.VariableCostPct = varCostFrac.Mul(d100).Round(6) // as %

	// Variable cost per order (optional)
	if avgOrderValue != nil && !avgOrderValue.IsZero() {
		vc := avgOrderValue.Mul(varCostFrac).Round(2)
		result.VariableCostPerOrder = &vc
	}

	// BEP is undefined when margin ≤ 0
	if contributionMarginPct.LessThanOrEqual(dZero) {
		result.Warnings = append(result.Warnings, model.BEPWarning{
			Code:    "margin_zero_or_negative",
			Message: "Le taux de marge sur coûts variables est nul ou négatif — le seuil de rentabilité est mathématiquement indéfini.",
		})
		return result
	}

	// Warn at margins below 10% (high sensitivity zone)
	if contributionMarginPct.LessThan(d10pct) {
		result.Warnings = append(result.Warnings, model.BEPWarning{
			Code:    "margin_below_10_pct",
			Message: "Le taux de marge est inférieur à 10 % — zone de forte sensibilité : le seuil de rentabilité est très sensible aux variations de coûts variables.",
		})
	}

	// BEP Revenue = Fixed Costs / (Margin% / 100)
	bep := fixedCostsTotal.Div(marginFrac).Round(2)
	result.BEPRevenue = &bep

	// Monthly BEP = BEP / 12
	monthly := bep.Div(d12).Round(2)
	result.MonthlyBEP = &monthly

	// BEP Volume = BEP / AvgOrderValue
	if avgOrderValue != nil && !avgOrderValue.IsZero() {
		vol := bep.Div(*avgOrderValue).Round(2)
		result.BEPVolume = &vol
	}

	return result
}

// ComputeEBE calculates EBE for a single revenue level.
//
//	EBE = Revenue × (MarginPct/100) − FixedCosts
func ComputeEBE(revenue, fixedCosts, marginPct decimal.Decimal) decimal.Decimal {
	return revenue.Mul(marginPct.Div(d100)).Sub(fixedCosts).Round(2)
}

// ─────────────────────────────────────────────────────────────────────────────
// §8.2  EBE estimation table (Sub-Module 1)
// ─────────────────────────────────────────────────────────────────────────────

// ComputeEBETable generates the 11-column EBE estimation table centred on the
// break-even revenue.  stepPct and rangePct control the column spacing
// (defaults: stepPct=10, rangePct=50 → columns at −50%,−40%,…,0%,…,+50%).
//
// Returns nil when BEP revenue is undefined (margin ≤ 0).
func ComputeEBETable(
	fixedCosts decimal.Decimal,
	marginPct decimal.Decimal,
	bepRevenue decimal.Decimal, // pre-computed from ComputeBEPCore
	stepPct decimal.Decimal,
	rangePct decimal.Decimal,
) []model.EBERow {
	if marginPct.LessThanOrEqual(dZero) {
		return nil
	}

	steps := buildVariationSteps(stepPct, rangePct)
	rows := make([]model.EBERow, len(steps))

	for i, variationPct := range steps {
		// Revenue(n) = BEP × (1 + variation/100)
		factor := decimal.NewFromInt(1).Add(variationPct.Div(d100))
		rev := bepRevenue.Mul(factor).Round(2)

		ebe := ComputeEBE(rev, fixedCosts, marginPct)
		varCosts := rev.Mul(decimal.NewFromInt(1).Sub(marginPct.Div(d100))).Round(2)
		totalCosts := fixedCosts.Add(varCosts).Round(2)

		rows[i] = model.EBERow{
			VariationPct: variationPct,
			Revenue:      rev.Round(1),
			TotalCosts:   totalCosts.Round(1),
			EBE:          ebe.Round(1),
			IsNegative:   ebe.LessThan(dZero),
			IsBEP:        variationPct.IsZero(),
		}
	}
	return rows
}

// ─────────────────────────────────────────────────────────────────────────────
// §8.2  Margin sensitivity table (Sub-Module 2)
// ─────────────────────────────────────────────────────────────────────────────

// ComputeMarginSensitivity generates the 11-column BEP-vs-margin sensitivity table.
// stepPp and rangePp are in percentage points (defaults: 5 and 25).
//
//	BEP_margin(n) = FixedCosts ÷ ((BaseMargin% + VariationPp) / 100)
//	               undefined if (BaseMargin% + VariationPp) ≤ 0
func ComputeMarginSensitivity(
	fixedCosts decimal.Decimal,
	baseMarginPct decimal.Decimal,
	stepPp decimal.Decimal,
	rangePp decimal.Decimal,
) []model.MarginSensRow {
	steps := buildVariationSteps(stepPp, rangePp)
	rows := make([]model.MarginSensRow, len(steps))

	for i, variationPp := range steps {
		effectiveMargin := baseMarginPct.Add(variationPp) // percentage points

		row := model.MarginSensRow{
			MarginVariationPp: variationPp,
			MarginPct:         effectiveMargin.Round(6),
			IsBase:            variationPp.IsZero(),
		}

		if effectiveMargin.LessThanOrEqual(dZero) {
			row.IsUndefined = true
		} else {
			bep := fixedCosts.Div(effectiveMargin.Div(d100)).Round(2)
			row.BEPRevenue = &bep
		}
		rows[i] = row
	}
	return rows
}

// ─────────────────────────────────────────────────────────────────────────────
// §8.2  Fixed-cost sensitivity table (Sub-Module 2)
// ─────────────────────────────────────────────────────────────────────────────

// ComputeFixedCostSensitivity generates the 11-column BEP-vs-fixed-cost
// sensitivity table (defaults: step=10%, range=50%).
//
//	BEP_cost(n) = FixedCosts × (1 + CostVariation%) ÷ (MarginPct / 100)
func ComputeFixedCostSensitivity(
	fixedCosts decimal.Decimal,
	marginPct decimal.Decimal,
	stepPct decimal.Decimal,
	rangePct decimal.Decimal,
) []model.FixedCostSensRow {
	if marginPct.LessThanOrEqual(dZero) {
		return nil
	}

	steps := buildVariationSteps(stepPct, rangePct)
	rows := make([]model.FixedCostSensRow, len(steps))
	marginFrac := marginPct.Div(d100)

	for i, variationPct := range steps {
		factor := decimal.NewFromInt(1).Add(variationPct.Div(d100))
		newFixed := fixedCosts.Mul(factor).Round(2)
		bep := newFixed.Div(marginFrac).Round(2)

		rows[i] = model.FixedCostSensRow{
			CostVariationPct: variationPct,
			NewFixedCosts:    newFixed.Round(1),
			MarginPct:        marginPct.Round(6),
			BEPRevenue:       bep.Round(1),
			IsBase:           variationPct.IsZero(),
		}
	}
	return rows
}

// ─────────────────────────────────────────────────────────────────────────────
// §8.3  Optimisation (Sub-Module 3)
// ─────────────────────────────────────────────────────────────────────────────

// ComputeOptimisedBEP calculates the optimised break-even point after applying
// proposed savings from an OptimisationPlan.
//
// varCostLinesDefined must be true when the snapshot has at least one
// VariableCostLine row.  When false, newTotalVarCostPerUnit is meaningless
// (the sum of an empty slice) and the margin is left unchanged at baseMarginPct.
func ComputeOptimisedBEP(
	baseFixedCosts decimal.Decimal,
	baseMarginPct decimal.Decimal,
	avgOrderValue *decimal.Decimal,
	fixedSavingsTotal decimal.Decimal,
	newTotalVarCostPerUnit decimal.Decimal, // post-savings sum of VariableCostLine amounts
	varCostLinesDefined bool, // false → no lines exist, margin stays at baseline
) model.OptimisedBEPReport {
	report := model.OptimisedBEPReport{}

	// ── Baseline BEP ─────────────────────────────────────────────────────────
	report.BaselineFixedCosts = baseFixedCosts.Round(2)
	report.BaselineMarginPct = baseMarginPct.Round(6)

	baseMarginFrac := baseMarginPct.Div(d100)
	if baseMarginPct.GreaterThan(dZero) {
		bep := baseFixedCosts.Div(baseMarginFrac).Round(2)
		report.BaselineBEPRevenue = &bep
		if avgOrderValue != nil && !avgOrderValue.IsZero() {
			vol := bep.Div(*avgOrderValue).Round(2)
			report.BaselineBEPVolume = &vol
		}
	}

	// ── Fixed cost optimisation ───────────────────────────────────────────────
	newFixed := baseFixedCosts.Sub(fixedSavingsTotal).Round(2)
	if newFixed.LessThan(dZero) {
		newFixed = dZero // savings cannot exceed current cost
	}
	deltaAbs := baseFixedCosts.Sub(newFixed).Round(2)
	var deltaPct decimal.Decimal
	if !baseFixedCosts.IsZero() {
		deltaPct = deltaAbs.Div(baseFixedCosts).Mul(d100).Round(2)
	}
	report.FixedCosts = model.OptimisedCostState{
		Current:   baseFixedCosts.Round(2),
		Optimised: newFixed,
		DeltaAbs:  deltaAbs,
		DeltaPct:  deltaPct,
	}

	// ── Variable cost / margin optimisation ───────────────────────────────────
	// New margin = (AvgOrderValue − NewVarCostPerUnit) / AvgOrderValue × 100
	// Only derivable when both an avg order value AND actual cost lines exist.
	// When no cost lines are defined, newTotalVarCostPerUnit is 0 by construction
	// (empty sum), not a user input — so we must not infer a 100 % margin from it.
	var newMarginPct decimal.Decimal
	if varCostLinesDefined && avgOrderValue != nil && !avgOrderValue.IsZero() {
		newVarPct := newTotalVarCostPerUnit.Div(*avgOrderValue).Mul(d100).Round(6)
		newMarginPct = d100.Sub(newVarPct)
	} else {
		// No avg order value, or no cost lines defined: margin unchanged
		newMarginPct = baseMarginPct
	}
	if newMarginPct.LessThan(dZero) {
		newMarginPct = dZero
		report.Warnings = append(report.Warnings, model.BEPWarning{
			Code:    "optimised_margin_negative",
			Message: "Les économies sur coûts variables proposées font passer le taux de marge en territoire négatif — vérifiez les saisies.",
		})
	}

	margDeltaAbs := newMarginPct.Sub(baseMarginPct).Round(6) // Δ pp (positive = improvement)
	report.MarginPct = model.OptimisedCostState{
		Current:   baseMarginPct.Round(6),
		Optimised: newMarginPct.Round(6),
		DeltaAbs:  margDeltaAbs, // Δ pp
		DeltaPct:  decimal.Zero, // not meaningful for percentages
	}

	// Var cost per unit state
	var baseVarCostPerUnit decimal.Decimal
	if avgOrderValue != nil && !avgOrderValue.IsZero() {
		baseVarCostPerUnit = avgOrderValue.Mul(decimal.NewFromInt(1).Sub(baseMarginPct.Div(d100))).Round(4)
	}
	vcDeltaAbs := baseVarCostPerUnit.Sub(newTotalVarCostPerUnit).Round(4)
	var vcDeltaPct decimal.Decimal
	if !baseVarCostPerUnit.IsZero() {
		vcDeltaPct = vcDeltaAbs.Div(baseVarCostPerUnit).Mul(d100).Round(2)
	}
	report.VariableCostPerUnit = model.OptimisedCostState{
		Current:   baseVarCostPerUnit,
		Optimised: newTotalVarCostPerUnit.Round(4),
		DeltaAbs:  vcDeltaAbs,
		DeltaPct:  vcDeltaPct,
	}

	// ── Combined optimised BEP ────────────────────────────────────────────────
	if newMarginPct.GreaterThan(dZero) {
		optBEP := newFixed.Div(newMarginPct.Div(d100)).Round(2)
		report.OptimisedBEPRevenue = &optBEP

		if avgOrderValue != nil && !avgOrderValue.IsZero() {
			vol := optBEP.Div(*avgOrderValue).Round(2)
			report.OptimisedBEPVolume = &vol
		}

		// Improvement deltas
		if report.BaselineBEPRevenue != nil {
			base := *report.BaselineBEPRevenue
			if !base.IsZero() {
				pct := base.Sub(optBEP).Div(base).Mul(d100).Round(2)
				report.BEPRevenueImprovementPct = &pct
			}
		}
		if report.BaselineBEPVolume != nil && report.OptimisedBEPVolume != nil {
			delta := report.BaselineBEPVolume.Sub(*report.OptimisedBEPVolume).Round(2)
			report.BEPVolumeImprovement = &delta
		}
	}

	return report
}

// ─────────────────────────────────────────────────────────────────────────────
// Top-level report assembler
// ─────────────────────────────────────────────────────────────────────────────

// ComputeBEPReport is the orchestrator called by the service layer.
// It assembles all three sub-module results into a BEPReport.
func ComputeBEPReport(
	snapshot model.BEPSnapshot,
	sensConfigs []model.SensitivityConfig,
) model.BEPReport {
	core := ComputeBEPCore(
		snapshot.FixedCostsTotal,
		snapshot.ContributionMarginPct,
		snapshot.AvgOrderValue,
	)

	// Resolve sensitivity configs (fallback to defaults if missing)
	revStep, revRange := sensDefaultsFor(sensConfigs, model.SensRevenue, decimal.NewFromInt(10), decimal.NewFromInt(50))
	margStep, margRange := sensDefaultsFor(sensConfigs, model.SensMargin, decimal.NewFromInt(5), decimal.NewFromInt(25))
	costStep, costRange := sensDefaultsFor(sensConfigs, model.SensFixedCost, decimal.NewFromInt(10), decimal.NewFromInt(50))

	var ebeTable []model.EBERow
	if core.BEPRevenue != nil {
		ebeTable = ComputeEBETable(
			snapshot.FixedCostsTotal,
			snapshot.ContributionMarginPct,
			*core.BEPRevenue,
			revStep,
			revRange,
		)
	}

	return model.BEPReport{
		SnapshotID: snapshot.ID,
		Core:       core,
		EBETable:   ebeTable,
		MarginSens: ComputeMarginSensitivity(
			snapshot.FixedCostsTotal,
			snapshot.ContributionMarginPct,
			margStep, margRange,
		),
		CostSens: ComputeFixedCostSensitivity(
			snapshot.FixedCostsTotal,
			snapshot.ContributionMarginPct,
			costStep, costRange,
		),
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ─────────────────────────────────────────────────────────────────────────────

// buildVariationSteps produces an ascending sequence of percentage variations
// centred on zero: [−range, −range+step, …, 0, …, +range].
// The number of steps = 2 × (range/step) + 1 (always odd, centre = 0).
func buildVariationSteps(step, rangeVal decimal.Decimal) []decimal.Decimal {
	if step.LessThanOrEqual(dZero) || rangeVal.LessThanOrEqual(dZero) {
		return []decimal.Decimal{dZero}
	}
	neg := rangeVal.Neg()
	var steps []decimal.Decimal
	for cur := neg; cur.LessThanOrEqual(rangeVal); cur = cur.Add(step) {
		steps = append(steps, cur)
	}
	return steps
}

// sensDefaultsFor resolves step/range from a SensitivityConfig slice, falling
// back to provided defaults when the requested type is absent.
func sensDefaultsFor(
	configs []model.SensitivityConfig,
	t model.BEPSensitivityType,
	defaultStep, defaultRange decimal.Decimal,
) (step, rangeVal decimal.Decimal) {
	for _, c := range configs {
		if c.AnalysisType == t {
			return c.StepSizePct, c.RangePct
		}
	}
	return defaultStep, defaultRange
}
