package compute

import (
	"math"
	"sort"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ─────────────────────────────────────────────────────────────────────────────
// Public entry points
// ─────────────────────────────────────────────────────────────────────────────

// ComputeCapTable is the top-level orchestrator for all three sub-modules.
func ComputeCapTable(
	company model.CapTableCompany,
	shareholders []model.CapTableShareholder,
	shareClasses []model.CapTableShareClass,
	rounds []model.CapTableRound,
	plans []model.StockOptionPlan,
	grants []model.OptionGrant,
	valuationScenarios []model.ValuationScenario,
) model.CapTableReport {
	ingefi := ComputeIngeFi(company, shareholders, shareClasses, rounds, grants, plans)
	stockOpt := ComputeStockOptions(company, shareholders, rounds, plans, grants, ingefi)

	var valoResults []model.ValuationScenarioResult
	for _, vs := range valuationScenarios {
		valoResults = append(valoResults, ComputeValuation(vs))
	}

	// Collect all warnings from sub-modules
	var warnings []model.ValidationWarning
	warnings = append(warnings, ingefi.Warnings...)

	return model.CapTableReport{
		IngeFi:      ingefi.Report,
		StockOption: stockOpt,
		FastValo:    model.FastValoReport{Scenarios: valoResults},
		Warnings:    warnings,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// IngéFi — cap table evolution
// ─────────────────────────────────────────────────────────────────────────────

// ingeFiResult bundles the public report and internal warnings.
type ingeFiResult struct {
	Report   model.IngeFiReport
	Warnings []model.ValidationWarning
}

// ComputeIngeFi produces the full cap table evolution from founding to the last round.
func ComputeIngeFi(
	company model.CapTableCompany,
	shareholders []model.CapTableShareholder,
	shareClasses []model.CapTableShareClass,
	rounds []model.CapTableRound,
	grants []model.OptionGrant,
	plans []model.StockOptionPlan,
) ingeFiResult {
	var warnings []model.ValidationWarning

	// Sort rounds by phase number.
	sorted := make([]model.CapTableRound, len(rounds))
	copy(sorted, rounds)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].PhaseNumber < sorted[j].PhaseNumber })

	// Build shareholder map for O(1) lookup.
	shMap := make(map[uuid.UUID]model.CapTableShareholder, len(shareholders))
	for _, sh := range shareholders {
		shMap[sh.ID] = sh
	}

	// Current share counts per shareholder (mutable state across phases).
	shares := make(map[uuid.UUID]int64, len(shareholders))
	for _, sh := range shareholders {
		shares[sh.ID] = sh.InitShares
	}

	// Cumulative split coefficient (starts at 1.0).
	cumulativeSplit := decimal.NewFromInt(1)

	// Nominal value tracks adjustments due to splits.
	nominalCents := company.NominalValueCents

	// Phase 0 — founding state.
	phase0 := buildPhase(0, "Création", company.NominalValueCents, decimal.NewFromInt(1),
		decimal.Zero, decimal.Zero, decimal.Zero,
		shares, shMap, nil, nil, nil)
	phases := []model.IngeFiPhase{phase0}

	// Compute outstanding options per shareholder, per round (for fully-diluted %).
	outstandingByRound := buildOutstandingByRound(grants, plans, sorted)

	// Process each round in order.
	for _, rnd := range sorted {
		// ── 1. Apply stock split ──────────────────────────────────────────────
		split := rnd.SplitCoefficient
		if split.IsZero() {
			split = decimal.NewFromInt(1)
		}
		if !split.Equal(decimal.NewFromInt(1)) {
			cumulativeSplit = cumulativeSplit.Mul(split)
			nominalCents = int64(math.Round(float64(nominalCents) / split.InexactFloat64()))
			for id, cnt := range shares {
				shares[id] = int64(math.Round(float64(cnt) * split.InexactFloat64()))
			}
		}

		totalBefore := sumShares(shares)

		// ── 2. Derive valuation ───────────────────────────────────────────────
		var preMoney, postMoney decimal.Decimal
		if rnd.PreMoneyValuationK != nil {
			preMoney = *rnd.PreMoneyValuationK
			postMoney = preMoney.Add(rnd.AmountRaisedK)
		} else if !rnd.PctGranted.IsZero() {
			// PostMoney = AmountRaised / (PctGranted / 100)
			postMoney = rnd.AmountRaisedK.Div(rnd.PctGranted.Div(decimal.NewFromInt(100)))
			preMoney = postMoney.Sub(rnd.AmountRaisedK)
		}

		// ── 3. Issue new shares ───────────────────────────────────────────────
		// Find the investor shareholder for this round (by class type match).
		var investorID uuid.UUID
		for _, sh := range shareholders {
			if string(sh.ClassType) == string(rnd.ShareClassType) && sh.Type == model.ShareholderInvestor {
				investorID = sh.ID
				break
			}
		}
		if rnd.NewSharesCreated > 0 && investorID != uuid.Nil {
			shares[investorID] += rnd.NewSharesCreated
		}

		totalAfter := sumShares(shares)

		// ── 4. Share price & emission premium ─────────────────────────────────
		var sharePrice decimal.Decimal
		if totalAfter > 0 && !postMoney.IsZero() {
			sharePrice = postMoney.Div(decimal.NewFromInt(totalAfter))
		}

		nominalK := decimal.NewFromInt(nominalCents).Div(decimal.NewFromInt(100000)) // cents → k currency
		emPremPerShare := sharePrice.Sub(nominalK)
		if emPremPerShare.IsNegative() {
			emPremPerShare = decimal.Zero
		}
		totalEmPrem := emPremPerShare.Mul(decimal.NewFromInt(rnd.NewSharesCreated))
		reinteg := decimal.Zero
		if rnd.EmissionPremiumReintegK != nil {
			reinteg = *rnd.EmissionPremiumReintegK
		}

		// ── 5. Goodwill ───────────────────────────────────────────────────────
		goodwill := decimal.Zero
		if rnd.BookEquityK != nil && !postMoney.IsZero() {
			goodwill = postMoney.Sub(*rnd.BookEquityK)
		}

		// ── 6. Build per-shareholder positions ────────────────────────────────
		outstandingThisRound := outstandingByRound[rnd.ID]

		phase := buildPhase(
			rnd.PhaseNumber, rnd.Label,
			nominalCents, split,
			preMoney, postMoney, rnd.AmountRaisedK,
			shares, shMap,
			outstandingThisRound,
			&rnd,
			&model.EmissionPremiumDetail{
				PerShareK:            emPremPerShare.Round(6),
				TotalK:               totalEmPrem.Round(2),
				ReintegratedK:        reinteg.Round(2),
				AdjustedNominalCents: nominalCents,
			},
		)
		phase.SharePriceEur = sharePrice.Round(6)
		phase.Goodwill = goodwill.Round(2)
		phase.TotalSharesBefore = totalBefore
		phase.TotalSharesAfter = totalAfter

		// Valuation growth vs previous phase.
		if len(phases) > 0 {
			prev := phases[len(phases)-1]
			if !prev.PostMoneyValuationK.IsZero() {
				phase.ValuationGrowthPct = postMoney.Sub(prev.PostMoneyValuationK).
					Div(prev.PostMoneyValuationK).Round(4)
				phase.ValuationMultiple = postMoney.Div(prev.PostMoneyValuationK).Round(4)
			}
		}

		// ── 7. Warnings ───────────────────────────────────────────────────────
		// Down-round detection.
		if len(phases) > 0 && !postMoney.IsZero() {
			prev := phases[len(phases)-1]
			if postMoney.LessThan(prev.PostMoneyValuationK) {
				warnings = append(warnings, model.ValidationWarning{
					Severity: "warning",
					Sheet:    "captable",
					Row:      rnd.Label,
					Message:  "Down-round detected: post-money valuation is lower than previous round",
				})
			}
		}
		// Founder dilution alert.
		founderAlert := company.FounderAlertPct
		for _, pos := range phase.Positions {
			if shMap[pos.ShareholderID].Type == model.ShareholderFounder {
				displayPct := pos.PctBasicAfter.Mul(decimal.NewFromInt(100))
				if displayPct.LessThan(founderAlert) {
					warnings = append(warnings, model.ValidationWarning{
						Severity: "warning",
						Sheet:    "captable",
						Row:      pos.Name,
						Message:  "Founder ownership fell below alert threshold",
					})
				}
			}
		}

		phases = append(phases, phase)
		_ = cumulativeSplit // used for split-adjusted nominal tracking
	}

	// ── Build matrix and waterfall ────────────────────────────────────────────
	matrix := buildMatrix(shareholders, phases)
	waterfall := buildWaterfall(shareholders, phases)

	// ── Build charts ──────────────────────────────────────────────────────────
	charts := buildIngeFiCharts(shareholders, phases)

	return ingeFiResult{
		Report: model.IngeFiReport{
			Phases:    phases,
			Matrix:    matrix,
			Waterfall: waterfall,
			Charts:    charts,
		},
		Warnings: warnings,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// FastStockOption — option lifecycle
// ─────────────────────────────────────────────────────────────────────────────

// ComputeStockOptions produces the full option lifecycle report.
func ComputeStockOptions(
	company model.CapTableCompany,
	shareholders []model.CapTableShareholder,
	rounds []model.CapTableRound,
	plans []model.StockOptionPlan,
	grants []model.OptionGrant,
	ingefi ingeFiResult,
) model.StockOptionReport {
	// Plan summaries.
	planSummaries := make([]model.StockOptionPlanSummary, 0, len(plans))
	for _, plan := range plans {
		// Aggregate grants for this plan.
		var totalGranted, totalExercised, totalCancelled int64
		for _, g := range grants {
			if g.PlanID == plan.ID {
				totalGranted += g.OptionsGranted
				totalExercised += g.OptionsExercised
				totalCancelled += g.OptionsCancelled
			}
		}
		outstanding := totalGranted - totalExercised - totalCancelled

		eligible := plan.IsEligibleBSPCE
		planSummaries = append(planSummaries, model.StockOptionPlanSummary{
			PlanID:          plan.ID,
			PlanLabel:       plan.PlanLabel,
			Instrument:      plan.Instrument,
			ExercisePrice:   plan.ExercisePrice,
			Voted:           plan.OptionsVoted,
			Attributed:      totalGranted,
			Exercised:       totalExercised,
			Cancelled:       totalCancelled,
			Reserve:         plan.OptionsVoted - totalGranted,
			Outstanding:     outstanding,
			IsEligibleBSPCE: eligible,
		})
	}

	// Per-phase beneficiary breakdown.
	shMap := make(map[uuid.UUID]model.CapTableShareholder, len(shareholders))
	for _, sh := range shareholders {
		shMap[sh.ID] = sh
	}

	byPhase := make([]model.StockOptionPhaseReport, 0, len(ingefi.Report.Phases))
	for _, phase := range ingefi.Report.Phases {
		totalShares := int64(0)
		for _, pos := range phase.Positions {
			totalShares += pos.SharesAfterRound
		}

		// Total outstanding options at this phase.
		var totalOutstanding int64
		for _, ps := range planSummaries {
			totalOutstanding += ps.Outstanding
		}

		var founders, investors []model.StockOptionBeneficiaryRow
		var totRow model.StockOptionBeneficiaryRow

		for _, pos := range phase.Positions {
			sh := shMap[pos.ShareholderID]

			// Outstanding options for this beneficiary.
			var personalOptions int64
			for _, g := range grants {
				if g.ShareholderID == pos.ShareholderID {
					personalOptions += g.OptionsGranted - g.OptionsExercised - g.OptionsCancelled
				}
			}

			var pctBasic, pctFD decimal.Decimal
			if totalShares > 0 {
				pctBasic = decimal.NewFromInt(pos.SharesAfterRound).
					Div(decimal.NewFromInt(totalShares)).Round(4)
			}
			if totalShares+totalOutstanding > 0 {
				pctFD = decimal.NewFromInt(pos.SharesAfterRound + personalOptions).
					Div(decimal.NewFromInt(totalShares + totalOutstanding)).Round(4)
			}

			impliedValue := decimal.Zero
			if !phase.SharePriceEur.IsZero() {
				impliedValue = phase.SharePriceEur.Mul(decimal.NewFromInt(pos.SharesAfterRound)).Round(2)
			}

			row := model.StockOptionBeneficiaryRow{
				Name:             sh.Name,
				SharesBefore:     pos.SharesBeforeSplit,
				OptionsGranted:   personalOptions,
				OptionsExercised: 0, // cumulative not tracked per phase here
				SharesAfter:      pos.SharesAfterRound,
				PctBasic:         pctBasic,
				PctFullyDiluted:  pctFD,
				ImpliedValueK:    impliedValue,
				SharesPostSplit:  pos.SharesAfterSplit,
			}

			totRow.SharesBefore += row.SharesBefore
			totRow.SharesAfter += row.SharesAfter
			totRow.PctBasic = totRow.PctBasic.Add(pctBasic)
			totRow.PctFullyDiluted = totRow.PctFullyDiluted.Add(pctFD)
			totRow.ImpliedValueK = totRow.ImpliedValueK.Add(impliedValue)

			switch sh.Type {
			case model.ShareholderFounder, model.ShareholderAssociate:
				founders = append(founders, row)
			default:
				investors = append(investors, row)
			}
		}

		byPhase = append(byPhase, model.StockOptionPhaseReport{
			PhaseNumber: phase.PhaseNumber,
			Label:       phase.Label,
			Founders:    founders,
			Investors:   investors,
			Totals:      totRow,
		})
	}

	// Charts.
	gauges := make([]model.ChartGaugeEntry, 0, len(planSummaries))
	for _, ps := range planSummaries {
		var pctUsed decimal.Decimal
		if ps.Voted > 0 {
			pctUsed = decimal.NewFromInt(ps.Attributed).
				Div(decimal.NewFromInt(ps.Voted)).Round(4)
		}
		gauges = append(gauges, model.ChartGaugeEntry{
			PlanLabel: ps.PlanLabel,
			Total:     ps.Voted,
			Used:      ps.Attributed,
			PctUsed:   pctUsed,
		})
	}

	var fdComparison []model.ChartDataPoint
	for _, phase := range ingefi.Report.Phases {
		if len(phase.Positions) > 0 {
			fdComparison = append(fdComparison, model.ChartDataPoint{
				Label: phase.Label,
				Value: phase.Positions[0].PctFullyDiluted, // first founder as reference
			})
		}
	}

	return model.StockOptionReport{
		Plans:   planSummaries,
		ByPhase: byPhase,
		Charts: model.StockOptionCharts{
			PoolUtilisationGauge:   gauges,
			FullyDilutedComparison: fdComparison,
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// FastValo — company valuation solver
// ─────────────────────────────────────────────────────────────────────────────

// ComputeValuation solves one FastValo scenario. All formulas per §7.5.
func ComputeValuation(vs model.ValuationScenario) model.ValuationScenarioResult {
	result := model.ValuationScenarioResult{
		ScenarioID: vs.ID,
		CalcType:   vs.CalcType,
		Label:      vs.Label,
		Language:   vs.DisplayLanguage,
		Labels:     buildValoLabels(vs.CalcType, vs.DisplayLanguage),
	}

	switch vs.CalcType {

	case model.ValScenMultipleToIRR:
		// A: (Multiple + Horizon) → IRR
		// IRR = multiple^(1/horizon) − 1
		if vs.MoneyMultiple != nil && vs.HorizonYears != nil {
			multiple := vs.MoneyMultiple.InexactFloat64()
			years := vs.HorizonYears.InexactFloat64()
			if years > 0 && multiple > 0 {
				irr := math.Pow(multiple, 1.0/years) - 1
				irrDec := decimal.NewFromFloat(irr).Round(4)
				multDec := decimal.NewFromFloat(multiple).Round(4)
				result.IRRPct = &irrDec
				result.MoneyMultiple = &multDec
			}
		}

	case model.ValScenInvestmentToTerminal:
		// B: (Investment + Annual yield) → Terminal Value + Multiple
		// Terminal = Investment × (1 + yield)^years
		if vs.InvestmentK != nil && vs.AnnualYieldPct != nil && vs.HorizonYears != nil {
			inv := vs.InvestmentK.InexactFloat64()
			yield := vs.AnnualYieldPct.InexactFloat64() / 100.0
			years := vs.HorizonYears.InexactFloat64()
			terminal := inv * math.Pow(1+yield, years)
			mult := terminal / inv
			termDec := decimal.NewFromFloat(terminal).Round(2)
			multDec := decimal.NewFromFloat(mult).Round(4)
			result.TerminalValueK = &termDec
			result.MoneyMultiple = &multDec
		}

	case model.ValScenInvestmentPctToIRR:
		// C: (Investment + Final% + Exit value) → IRR + NPV + Investor share
		if vs.InvestmentK != nil && vs.FinalInvestorPct != nil && vs.ExitCompanyValueK != nil && vs.HorizonYears != nil {
			inv := vs.InvestmentK.InexactFloat64()
			pct := vs.FinalInvestorPct.InexactFloat64() / 100.0
			exitVal := vs.ExitCompanyValueK.InexactFloat64()
			years := vs.HorizonYears.InexactFloat64()

			investorShare := exitVal * pct
			mult := investorShare / inv
			irr := math.Pow(mult, 1.0/years) - 1

			shareDec := decimal.NewFromFloat(investorShare).Round(2)
			multDec := decimal.NewFromFloat(mult).Round(4)
			irrDec := decimal.NewFromFloat(irr).Round(4)
			result.InvestorShareK = &shareDec
			result.MoneyMultiple = &multDec
			result.IRRPct = &irrDec

			// NPV
			discRate := 0.10 // default
			if vs.DiscountRatePct != nil {
				discRate = vs.DiscountRatePct.InexactFloat64() / 100.0
			}
			npv := investorShare / math.Pow(1+discRate, years)
			npvDec := decimal.NewFromFloat(npv).Round(2)
			result.NPVK = &npvDec
		}

	case model.ValScenNewMoneyToPremoney:
		// D: (New money + Post-money %) → Pre/Post-money Valuation
		if vs.NewMoneyK != nil && vs.FinalInvestorPct != nil {
			newMoney := vs.NewMoneyK.InexactFloat64()
			pct := vs.FinalInvestorPct.InexactFloat64() / 100.0
			if pct > 0 {
				postMoney := newMoney / pct
				preMoney := postMoney - newMoney
				postDec := decimal.NewFromFloat(postMoney).Round(2)
				preDec := decimal.NewFromFloat(preMoney).Round(2)
				result.PostMoneyK = &postDec
				result.PreMoneyK = &preDec
			}
		}
	}

	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ─────────────────────────────────────────────────────────────────────────────

// sumShares returns the total share count across all holders.
func sumShares(shares map[uuid.UUID]int64) int64 {
	var total int64
	for _, cnt := range shares {
		total += cnt
	}
	return total
}

// buildOutstandingByRound pre-computes outstanding option counts per shareholder
// indexed by the round after which they apply (cumulative — grants accumulate forward).
func buildOutstandingByRound(
	grants []model.OptionGrant,
	plans []model.StockOptionPlan,
	sortedRounds []model.CapTableRound,
) map[uuid.UUID]map[uuid.UUID]int64 {
	// planRoundID maps plan ID → round ID it was created at.
	planRoundID := make(map[uuid.UUID]uuid.UUID, len(plans))
	for _, p := range plans {
		if p.RoundID != nil {
			planRoundID[p.ID] = *p.RoundID
		}
	}

	// outstanding[roundID][shareholderID] = outstanding options at that phase.
	outstanding := make(map[uuid.UUID]map[uuid.UUID]int64)
	for _, rnd := range sortedRounds {
		outstanding[rnd.ID] = make(map[uuid.UUID]int64)
	}

	// Accumulate grants round-by-round (grants carry forward to later rounds).
	cum := make(map[uuid.UUID]int64) // shareholderID → cumulative outstanding
	for _, rnd := range sortedRounds {
		// Add grants that were created at this round (or before).
		for _, g := range grants {
			planRnd, ok := planRoundID[g.PlanID]
			if ok && planRnd == rnd.ID {
				o := g.OptionsGranted - g.OptionsExercised - g.OptionsCancelled
				if o > 0 {
					cum[g.ShareholderID] += o
				}
			}
		}
		// Copy current cumulative state into this round's snapshot.
		for shID, cnt := range cum {
			outstanding[rnd.ID][shID] = cnt
		}
	}

	return outstanding
}

// buildPhase creates an IngeFiPhase snapshot for the given state.
func buildPhase(
	phaseNumber int,
	label string,
	nominalCents int64,
	split decimal.Decimal,
	preMoney, postMoney, amountRaised decimal.Decimal,
	shares map[uuid.UUID]int64,
	shMap map[uuid.UUID]model.CapTableShareholder,
	outstanding map[uuid.UUID]int64, // may be nil for phase 0
	rnd *model.CapTableRound,
	emPrem *model.EmissionPremiumDetail,
) model.IngeFiPhase {
	totalShares := sumShares(shares)

	// Compute total outstanding options.
	var totalOutstanding int64
	for _, cnt := range outstanding {
		totalOutstanding += cnt
	}

	var positions []model.IngeFiShareholderPosition
	for shID, cnt := range shares {
		sh, ok := shMap[shID]
		if !ok {
			continue
		}

		pctBasicBefore := decimal.Zero
		pctBasicAfter := decimal.Zero
		if totalShares > 0 {
			pctBasicAfter = decimal.NewFromInt(cnt).
				Div(decimal.NewFromInt(totalShares)).Round(4)
		}

		pctFD := pctBasicAfter
		if totalShares+totalOutstanding > 0 {
			personalOut := outstanding[shID]
			pctFD = decimal.NewFromInt(cnt + personalOut).
				Div(decimal.NewFromInt(totalShares + totalOutstanding)).Round(4)
		}

		// Determine shares before split from previous shares count.
		// For phase 0, before == after (no split yet).
		splitF := split.InexactFloat64()
		var beforeSplit int64
		if splitF != 0 && splitF != 1 {
			beforeSplit = int64(math.Round(float64(cnt) / splitF))
		} else {
			beforeSplit = cnt
		}

		var newReceived int64
		if rnd != nil && sh.ClassType == rnd.ShareClassType && sh.Type == model.ShareholderInvestor {
			newReceived = rnd.NewSharesCreated
		}

		impliedValue := decimal.Zero
		var sharePrice decimal.Decimal
		if totalShares > 0 && !postMoney.IsZero() {
			sharePrice = postMoney.Div(decimal.NewFromInt(totalShares))
			impliedValue = sharePrice.Mul(decimal.NewFromInt(cnt)).Round(2)
		}

		positions = append(positions, model.IngeFiShareholderPosition{
			ShareholderID:     shID,
			Name:              sh.Name,
			Type:              sh.Type,
			ClassType:         sh.ClassType,
			SharesBeforeSplit: beforeSplit,
			SharesAfterSplit:  cnt,
			NewSharesReceived: newReceived,
			SharesAfterRound:  cnt,
			PctBasicBefore:    pctBasicBefore,
			PctBasicAfter:     pctBasicAfter,
			PctFullyDiluted:   pctFD,
			DilutionDelta:     pctBasicBefore.Sub(pctBasicAfter).Round(4),
			ImpliedValueK:     impliedValue,
			AmountInvestedK:   sh.InvestedAmountK,
		})
	}

	// Sort positions by shareholder type then name for deterministic output.
	sort.Slice(positions, func(i, j int) bool {
		if positions[i].Type != positions[j].Type {
			return positions[i].Type < positions[j].Type
		}
		return positions[i].Name < positions[j].Name
	})

	phase := model.IngeFiPhase{
		PhaseNumber:         phaseNumber,
		Label:               label,
		TotalSharesBefore:   totalShares,
		TotalSharesAfter:    totalShares,
		PreMoneyValuationK:  preMoney.Round(2),
		PostMoneyValuationK: postMoney.Round(2),
		AmountRaisedK:       amountRaised.Round(2),
		NominalValueCents:   nominalCents,
		SplitCoefficient:    split,
		Positions:           positions,
	}

	if emPrem != nil {
		phase.EmissionPremium = *emPrem
	} else {
		phase.EmissionPremium = model.EmissionPremiumDetail{
			AdjustedNominalCents: nominalCents,
		}
	}

	return phase
}

// buildMatrix creates the shareholder × phase ownership grid.
func buildMatrix(shareholders []model.CapTableShareholder, phases []model.IngeFiPhase) model.CapTableMatrix {
	columns := make([]string, len(phases))
	for i, p := range phases {
		columns[i] = p.Label
	}

	// Build per-shareholder rows.
	posMap := make(map[uuid.UUID][]model.CapTableMatrixCell, len(shareholders))

	for _, phase := range phases {
		found := make(map[uuid.UUID]bool)
		for _, pos := range phase.Positions {
			posMap[pos.ShareholderID] = append(posMap[pos.ShareholderID], model.CapTableMatrixCell{
				Shares:          pos.SharesAfterRound,
				PctBasic:        pos.PctBasicAfter,
				PctFullyDiluted: pos.PctFullyDiluted,
				ImpliedValueK:   pos.ImpliedValueK,
			})
			found[pos.ShareholderID] = true
		}
		// Zero cells for shareholders not in this phase.
		for _, sh := range shareholders {
			if !found[sh.ID] {
				posMap[sh.ID] = append(posMap[sh.ID], model.CapTableMatrixCell{})
			}
		}
	}

	rows := make([]model.CapTableMatrixRow, 0, len(shareholders))
	for _, sh := range shareholders {
		rows = append(rows, model.CapTableMatrixRow{
			ShareholderName: sh.Name,
			Type:            sh.Type,
			Cells:           posMap[sh.ID],
		})
	}

	// Build totals per phase column.
	totals := make([]model.CapTableMatrixTotalsRow, len(phases))
	for i, phase := range phases {
		var totalShares, totalPools int64
		var totalValue decimal.Decimal
		for _, pos := range phase.Positions {
			totalShares += pos.SharesAfterRound
			if pos.Type == model.ShareholderESOPPool {
				totalPools += pos.SharesAfterRound
			}
			totalValue = totalValue.Add(pos.ImpliedValueK)
		}
		totals[i] = model.CapTableMatrixTotalsRow{
			TotalShares:          totalShares,
			TotalSharesPlusPools: totalShares + totalPools,
			TotalValueK:          totalValue.Round(2),
		}
	}

	return model.CapTableMatrix{
		ColumnLabels: columns,
		Rows:         rows,
		Totals:       totals,
	}
}

// buildWaterfall creates the dilution waterfall for each holder across all phases.
func buildWaterfall(shareholders []model.CapTableShareholder, phases []model.IngeFiPhase) model.DilutionWaterfall {
	holders := make([]model.DilutionWaterfallHolder, 0, len(shareholders))

	for _, sh := range shareholders {
		pcts := make([]decimal.Decimal, 0, len(phases))
		var initialPct, finalPct decimal.Decimal

		for _, phase := range phases {
			pct := decimal.Zero
			for _, pos := range phase.Positions {
				if pos.ShareholderID == sh.ID {
					pct = pos.PctBasicAfter
					break
				}
			}
			pcts = append(pcts, pct)
		}

		if len(pcts) > 0 {
			initialPct = pcts[0]
			finalPct = pcts[len(pcts)-1]
		}

		holders = append(holders, model.DilutionWaterfallHolder{
			Name:          sh.Name,
			Type:          sh.Type,
			InitialPct:    initialPct,
			PctByPhase:    pcts,
			FinalPct:      finalPct,
			TotalDilution: initialPct.Sub(finalPct).Round(4),
		})
	}

	return model.DilutionWaterfall{Holders: holders}
}

// buildIngeFiCharts assembles chart data bundles for the IngéFi report.
func buildIngeFiCharts(shareholders []model.CapTableShareholder, phases []model.IngeFiPhase) model.IngeFiCharts {
	// Ownership stacked bar — one bar per phase, one segment per shareholder.
	stackedBars := make([]model.ChartStackedBar, 0, len(phases))
	for _, phase := range phases {
		segments := make([]model.ChartSlice, 0, len(phase.Positions))
		for _, pos := range phase.Positions {
			segments = append(segments, model.ChartSlice{
				Label: pos.Name,
				Value: pos.PctBasicAfter.Mul(decimal.NewFromInt(100)).Round(2),
			})
		}
		stackedBars = append(stackedBars, model.ChartStackedBar{
			Label:    phase.Label,
			Segments: segments,
		})
	}

	// Valuation timeline — pre and post money per phase.
	var valTimeline []model.ChartDataPoint
	for _, phase := range phases {
		valTimeline = append(valTimeline, model.ChartDataPoint{
			Label: phase.Label + " pre",
			Value: phase.PreMoneyValuationK,
		})
		valTimeline = append(valTimeline, model.ChartDataPoint{
			Label: phase.Label + " post",
			Value: phase.PostMoneyValuationK,
		})
	}

	// Dilution funnel — first founder's ownership % per phase.
	var dilutionFunnel []model.ChartDataPoint
	for _, phase := range phases {
		for _, pos := range phase.Positions {
			for _, sh := range shareholders {
				if sh.ID == pos.ShareholderID && sh.Type == model.ShareholderFounder {
					dilutionFunnel = append(dilutionFunnel, model.ChartDataPoint{
						Label: phase.Label,
						Value: pos.PctBasicAfter.Mul(decimal.NewFromInt(100)).Round(2),
					})
					goto nextPhase
				}
			}
		}
	nextPhase:
	}

	return model.IngeFiCharts{
		OwnershipStackedBar: stackedBars,
		ValuationTimeline:   valTimeline,
		DilutionFunnel:      dilutionFunnel,
	}
}

// buildValoLabels returns bilingual labels for a FastValo scenario type.
func buildValoLabels(calcType model.ValuationScenarioType, language string) map[string]string {
	labels := map[model.ValuationScenarioType]map[string]string{
		model.ValScenMultipleToIRR: {
			"fr_title": "Scénario A — Multiple → TRI",
			"en_title": "Scenario A — Multiple → IRR",
			"fr_desc":  "Calcule le TRI annuel à partir du multiple et de l'horizon",
			"en_desc":  "Computes annual IRR from money multiple and exit horizon",
		},
		model.ValScenInvestmentToTerminal: {
			"fr_title": "Scénario B — Investissement → Valeur terminale",
			"en_title": "Scenario B — Investment → Terminal Value",
			"fr_desc":  "Calcule la valeur terminale à partir du rendement annuel",
			"en_desc":  "Computes terminal value from annual yield",
		},
		model.ValScenInvestmentPctToIRR: {
			"fr_title": "Scénario C — Investissement + % → TRI + VAN",
			"en_title": "Scenario C — Investment + % → IRR + NPV",
			"fr_desc":  "Calcule le TRI et la VAN à partir de l'investissement et du pourcentage final",
			"en_desc":  "Computes IRR and NPV from investment amount and final ownership %",
		},
		model.ValScenNewMoneyToPremoney: {
			"fr_title": "Scénario D — Nouveaux fonds → Valorisation",
			"en_title": "Scenario D — New Money → Valuation",
			"fr_desc":  "Calcule la valorisation pré et post-money",
			"en_desc":  "Computes pre and post-money valuation",
		},
	}

	if m, ok := labels[calcType]; ok {
		return m
	}
	return map[string]string{
		"fr_title": string(calcType),
		"en_title": string(calcType),
	}
}
